package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"rm-aurora/pkg/config"
	"rm-aurora/pkg/network"
	"rm-aurora/pkg/rmcp"
	"rm-aurora/pkg/video"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// videoRuntimePlayer 描述 App 层真正依赖的最小播放器能力。
// What: 将 Stop、EnqueueFrame、Snapshot 和 Layout 这四个必需能力收口成接口。
// Why: App 只关心"停播放器、喂帧、取状态、读布局"，抽成接口后既能继续接真实 NativePlayer，也能在单测里放入假播放器锁住切源时序。
type videoRuntimePlayer interface {
	Stop()
	EnqueueFrame(frame []byte)
	Snapshot() video.PlayerSnapshot
	Layout() video.VideoWindowLayout
	UpdateLayout(layout video.VideoWindowLayout) error
}

type mqttBrokerConfig struct {
	host string
	port int
}

type configuredUIBootstrap struct {
	mqttRobotIdentity    string
	hasMQTTRobotIdentity bool
	videoSource          string
	hasVideoSource       bool
}

type officialVideoConfig struct {
	port     int
	disabled bool
}

type videoRuntimeState struct {
	officialPlayer           videoRuntimePlayer
	customPlayer             videoRuntimePlayer
	officialPlayerErr        string
	customPlayerErr          string
	activeVideoSource        string
	lastCustomByteBlockAt    int64
	officialSyncReady        bool
	officialBootstrapReady   bool
	officialBootstrapPending bool
	domReady                 bool
}

type videoRateSample struct {
	at                time.Time
	officialPackets   uint64
	officialFrames    uint64
	officialDrops     uint64
	customBlocks      uint64
	customAUs         uint64
	customDrops       uint64
	customCorrupt     uint64
	officialPacketHz  float64
	officialFrameHz   float64
	officialDropRate  float64
	customBlockHz     float64
	customAUFPS       float64
	customDropRate    float64
	customCorruptRate float64
	lastDiagnosticLog time.Time
}

// App 结构体维系 Wails 后台与业务层的核心逻辑。
// What: 将视频、控制、比赛态桥接统一收口在一个生命周期对象里。
// Why: Wails 的启动、关闭和前端绑定都围绕同一个实例展开，集中管理最稳妥。
type App struct {
	ctx context.Context

	udpRx *network.UDPReceiver
	mqtt  *network.MQTTClient

	// What: 统一缓存裁判系统里和血量显示直接相关的最小状态快照。
	// Why: 基地血量、全队血量、本机实时血量和血量上限来自不同 topic，必须在后端先拼成一份稳定视图，再发给前端。
	rmStateMu sync.RWMutex

	mqttClientID        uint16
	selfSide            string
	selfRelativeRobotID uint32
	mqttBrokerHost      string
	mqttBrokerPort      int
	gameStatus          *rmcp.GameStatus
	globalUnitStatus    *rmcp.GlobalUnitStatus
	robotDynamicStatus  *rmcp.RobotDynamicStatus
	robotStaticStatus   map[uint32]*rmcp.RobotStaticStatus

	// What: 原生视频层会在 OnDomReady 后才真正启动。
	// Why: 这意味着 startup、状态广播和 UDP 回调会与播放器初始化并发发生，因此这里必须用读写锁保护播放器句柄与错误态，避免窗口层重排后出现数据竞争。
	videoMu sync.RWMutex

	// What: 将原生视频层的启动与停止流程串行化。
	// Why: `domReady`、前端初始化切源和运行时窗口重建都可能并发碰到播放器生命周期；若不额外串行，这几条链路会各自误判"当前没有播放器"，从而同时拉起多个 ffplay 进程。
	videoLifecycleMu sync.Mutex
	videoRateMu      sync.Mutex
	videoRateSample  videoRateSample

	officialPlayer    videoRuntimePlayer
	customPlayer      videoRuntimePlayer
	officialPlayerErr string
	customPlayerErr   string
	activeVideoSource string
	videoDOMReady     bool

	// What: 记录最近一次 0x0310 自定义字节块到达时间。
	// Why: 自定义源等待态、在线态和黑幕提示都依赖这份"底层原始数据是否还在来"的时间基准。
	lastCustomByteBlockAt    int64
	customByteBlockCount     uint64
	customStreamNeedsResync  bool
	officialBootstrapFrame   []byte
	officialBootstrapPending bool

	// What: 自定义 H.264 访问单元重组器。
	// Why: 0x0310 只负责转发 300B 原始字节块，不保留帧边界；客户端必须在这里把任意分块恢复成完整 AU 后再交给 ffplay。
	h264Reassembler *video.H264Reassembler
	hevcSyncGate    *video.HEVCSyncGate

	// What: 记录当前官方视频 UDP 监听配置。
	// Why: 本地 custom-only 联调需要明确跳过官方 :3334 绑定，否则测试会被现有图传进程的端口占用直接打死。
	officialVideoPort     int
	officialVideoDisabled bool
}

type videoStatePayload struct {
	BackendConnected       bool    `json:"backend_connected"`
	ControlLinkConnected   bool    `json:"control_link_connected"`
	VideoConnected         bool    `json:"video_connected"`
	ActiveSource           string  `json:"active_source"`
	PIPSource              string  `json:"pip_source"`
	OfficialAvailable      bool    `json:"official_available"`
	CustomAvailable        bool    `json:"custom_available"`
	OfficialVideoConnected bool    `json:"official_video_connected"`
	CustomVideoConnected   bool    `json:"custom_video_connected"`
	DisplayState           string  `json:"display_state"`
	OfficialDisplayState   string  `json:"official_display_state"`
	CustomDisplayState     string  `json:"custom_display_state"`
	DecoderFPS             float64 `json:"decoder_fps"`
	PresentFPS             float64 `json:"present_fps"`
	LatencyMs              int64   `json:"latency_ms"`
	OfficialLatencyMs      int64   `json:"official_latency_ms"`
	CustomLatencyMs        int64   `json:"custom_latency_ms"`
	OfficialPacketRateHz   float64 `json:"official_packet_rate_hz"`
	OfficialFrameRateHz    float64 `json:"official_frame_rate_hz"`
	OfficialDropRate       float64 `json:"official_drop_rate"`
	CustomBlockRateHz      float64 `json:"custom_block_rate_hz"`
	CustomAUFPS            float64 `json:"custom_au_fps"`
	CustomDropRate         float64 `json:"custom_drop_rate"`
	CustomCorruptRate      float64 `json:"custom_corrupt_rate"`
	CustomH264NALTotal     uint64  `json:"custom_h264_nal_total"`
	CustomH264IDR          uint64  `json:"custom_h264_idr"`
	CustomH264SPS          uint64  `json:"custom_h264_sps"`
	CustomH264PPS          uint64  `json:"custom_h264_pps"`
	CustomH264NoStartCode  uint64  `json:"custom_h264_no_start_code"`
	CustomH264Synced       bool    `json:"custom_h264_synced"`
	UDPDropFrames          uint64  `json:"udp_drop_frames"`
	DecodeDropFrames       uint64  `json:"decode_drop_frames"`
	CorruptFrames          uint64  `json:"corrupt_frames"`
	DecoderResets          uint64  `json:"decoder_resets"`
	HeaderOrder            string  `json:"header_order"`
	Message                string  `json:"message"`
	OfficialMessage        string  `json:"official_message"`
	CustomMessage          string  `json:"custom_message"`
	UpdatedAt              int64   `json:"updated_at"`
}

// What: 统一定义视频状态广播周期与窗口恢复参数。
// Why: 双源视频切换后，官方源状态、幕布提示和原生窗口重建都依赖这组稳定常量，不能继续散落魔法数字。
const (
	defaultMQTTBrokerHost                 = "192.168.12.1"
	defaultMQTTBrokerPort                 = 3333
	defaultOfficialVideoPort              = 3334
	defaultMQTTClientID            uint16 = 101
	baseMaxHealth                         = 5000
	videoStateEmitInterval                = 200 * time.Millisecond
	videoConnectedThreshold               = 800 * time.Millisecond
	customVideoConnectedThreshold         = 4 * time.Second
	videoSourceActiveThreshold            = 1500 * time.Millisecond
	customStreamGapResyncThreshold        = videoConnectedThreshold
	videoWindowLayoutRetryCount           = 6
	videoWindowLayoutRetryInterval        = 120 * time.Millisecond
	videoWindowLayoutSyncInterval         = 2 * time.Second
	mqttClientIDProbeTimeout              = 1200 * time.Millisecond
)

const (
	videoDisplayStateLive          = "live"
	videoDisplayStateWaitingSource = "waiting_source"
	videoDisplayStateWaitingFrame  = "waiting_frame"
	videoDisplayStateResyncing     = "resyncing"
	videoDisplayStateStalled       = "stalled"
	videoDisplayStateRuntimeError  = "runtime_error"
)

const (
	teamSideRed  = "red"
	teamSideBlue = "blue"
)

const (
	videoSourceOfficial = "official"
	videoSourceCustom   = "custom"
)

const (
	mqttHeroIdentityRedHero  = "red_hero"
	mqttHeroIdentityBlueHero = "blue_hero"
)

var mqttRobotIdentityClientIDs = map[string]uint16{
	mqttHeroIdentityRedHero:  1,
	"red_engineer":           2,
	"red_infantry_3":         3,
	"red_infantry_4":         4,
	"red_infantry_5":         5,
	"red_drone":              6,
	"red_sentry":             7,
	mqttHeroIdentityBlueHero: 101,
	"blue_engineer":          102,
	"blue_infantry_3":        103,
	"blue_infantry_4":        104,
	"blue_infantry_5":        105,
	"blue_drone":             106,
	"blue_sentry":            107,
}

var mqttRobotIdentityAliases = map[string]string{
	"red_infantry":   "red_infantry_3",
	"blue_infantry":  "blue_infantry_3",
	"red_infantry3":  "red_infantry_3",
	"red_infantry4":  "red_infantry_4",
	"red_infantry5":  "red_infantry_5",
	"blue_infantry3": "blue_infantry_3",
	"blue_infantry4": "blue_infantry_4",
	"blue_infantry5": "blue_infantry_5",
}

// robotIDToIdentityLabel 将客户端 ID 反向映射为稳定的字符串标签。
// What: 覆盖红蓝双方全部机器人与固定实体 ID，未匹配时回退到数值字符串。
// Why: 前端设置面板需要一个可读列表但后端只认 ID；这里提供最小双向桥。
func robotIDToIdentityLabel(clientID uint16) string {
	switch clientID {
	case 1:
		return mqttHeroIdentityRedHero
	case 2:
		return "red_engineer"
	case 3:
		return "red_infantry_3"
	case 4:
		return "red_infantry_4"
	case 5:
		return "red_infantry_5"
	case 6:
		return "red_drone"
	case 7:
		return "red_sentry"
	case 8:
		return "red_dart"
	case 9:
		return "red_radar"
	case 10:
		return "red_outpost"
	case 11:
		return "red_base"
	case 101:
		return mqttHeroIdentityBlueHero
	case 102:
		return "blue_engineer"
	case 103:
		return "blue_infantry_3"
	case 104:
		return "blue_infantry_4"
	case 105:
		return "blue_infantry_5"
	case 106:
		return "blue_drone"
	case 107:
		return "blue_sentry"
	case 108:
		return "blue_dart"
	case 109:
		return "blue_radar"
	case 110:
		return "blue_outpost"
	case 111:
		return "blue_base"
	default:
		return strconv.FormatUint(uint64(clientID), 10)
	}
}

// robotIDLabelForDisplay 将身份标签转为人可读中文。
func robotIDLabelForDisplay(clientID uint16) string {
	if clientID >= 100 {
		switch clientID % 100 {
		case 1:
			return "蓝方英雄"
		case 2:
			return "蓝方工程"
		case 3, 4, 5:
			return "蓝方步兵"
		case 6:
			return "蓝方空中"
		case 7:
			return "蓝方哨兵"
		case 8:
			return "蓝方飞镖"
		case 9:
			return "蓝方雷达"
		case 10:
			return "蓝方前哨"
		case 11:
			return "蓝方基地"
		default:
			return fmt.Sprintf("蓝方 #%d", clientID)
		}
	}
	switch clientID {
	case 1:
		return "红方英雄"
	case 2:
		return "红方工程"
	case 3, 4, 5:
		return "红方步兵"
	case 6:
		return "红方空中"
	case 7:
		return "红方哨兵"
	case 8:
		return "红方飞镖"
	case 9:
		return "红方雷达"
	case 10:
		return "红方前哨"
	case 11:
		return "红方基地"
	default:
		return fmt.Sprintf("红方 #%d", clientID)
	}
}

// mqttClientIDProbeCandidates 返回当前可探测的全体机器人 clientID 候选列表。
// What: 按固定顺序生成红蓝双方完整 ID 候选，方便纯数字 ID 场景下自动回退。
func mqttClientIDProbeCandidates() []uint16 {
	ids := make([]uint16, 0, 22)
	for i := uint16(1); i <= 11; i++ {
		ids = append(ids, i)
	}
	for i := uint16(101); i <= 111; i++ {
		ids = append(ids, i)
	}
	return ids
}

var officialRobotOrder = []uint32{1, 2, 3, 4, 7}

// NewApp 创建新 App。
func NewApp() *App {
	return &App{
		robotStaticStatus:     make(map[uint32]*rmcp.RobotStaticStatus),
		mqttBrokerHost:        defaultMQTTBrokerHost,
		mqttBrokerPort:        defaultMQTTBrokerPort,
		activeVideoSource:     videoSourceOfficial,
		lastCustomByteBlockAt: 0,
		h264Reassembler:       video.NewH264Reassembler(),
		hevcSyncGate:          video.NewHEVCSyncGate(),
		officialVideoPort:     defaultOfficialVideoPort,
	}
}

// resolveMQTTBrokerConfig 解析当前进程实际应连接的 MQTT broker 地址。
// What: 允许测试态通过环境变量覆盖 host/port，同时保留正式环境默认目标。
// Why: 单机仿真必须把客户端指向本地 broker，但比赛机默认行为不能因此改变。
func resolveMQTTBrokerConfig() mqttBrokerConfig {
	config := mqttBrokerConfig{
		host: defaultMQTTBrokerHost,
		port: defaultMQTTBrokerPort,
	}

	if rawHost := strings.TrimSpace(os.Getenv("RM_MQTT_HOST")); rawHost != "" {
		config.host = rawHost
	}

	rawPort := strings.TrimSpace(os.Getenv("RM_MQTT_PORT"))
	if rawPort == "" {
		return config
	}

	parsedPort, err := strconv.Atoi(rawPort)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		log.Printf("[Warning] invalid RM_MQTT_PORT=%q, fallback to %d", rawPort, defaultMQTTBrokerPort)
		return config
	}

	config.port = parsedPort
	return config
}

// normalizeMQTTRobotIdentity 将外部传入的 MQTT 身份字符串归一化。
// What: 接受常用机器人命名身份或纯数字 clientID；非法输入回退到空字符串。
// Why: 用户可以从前端设置页选择任意机器人身份，不再只限于红/蓝英雄。
func normalizeMQTTRobotIdentity(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return ""
	}
	if alias, ok := mqttRobotIdentityAliases[normalized]; ok {
		return alias
	}
	if _, ok := mqttRobotIdentityClientIDs[normalized]; ok {
		return normalized
	}
	if id, err := strconv.ParseUint(normalized, 10, 16); err == nil {
		label := robotIDToIdentityLabel(uint16(id))
		if _, ok := mqttRobotIdentityClientIDs[label]; ok {
			return label
		}
	}
	return ""
}

// normalizeMQTTHeroIdentity 保留旧命名入口，内部走新的机器人身份归一化。
func normalizeMQTTHeroIdentity(raw string) string {
	return normalizeMQTTRobotIdentity(raw)
}

// mqttRobotIdentityToClientID 将身份字符串映射为裁判系统要求的机器人 clientID。
// What: 支持常用机器人命名身份或纯数字 clientID。
// Why: 现在客户端允许选择任何机器人身份，映射必须能覆盖全范围并安全回退。
func mqttRobotIdentityToClientID(identity string) uint16 {
	normalizedIdentity := normalizeMQTTRobotIdentity(identity)
	if clientID, ok := mqttRobotIdentityClientIDs[normalizedIdentity]; ok {
		return clientID
	}
	if parsed, err := strconv.ParseUint(strings.TrimSpace(identity), 10, 16); err == nil {
		label := robotIDToIdentityLabel(uint16(parsed))
		if _, ok := mqttRobotIdentityClientIDs[label]; ok {
			return uint16(parsed)
		}
	}
	return 0
}

func isHeroMQTTRobotIdentity(identity string) bool {
	normalizedIdentity := normalizeMQTTRobotIdentity(identity)
	return normalizedIdentity == mqttHeroIdentityRedHero || normalizedIdentity == mqttHeroIdentityBlueHero
}

func isHeroMQTTClientID(clientID uint16) bool {
	return clientID == 1 || clientID == 101
}

func (a *App) allowsCustomVideoSource() bool {
	a.rmStateMu.RLock()
	clientID := a.mqttClientID
	a.rmStateMu.RUnlock()

	// What: 启动早期还没有绑定身份时允许 custom 初始化。
	// Why: 后续 configureRMIdentity 会再次按真实身份收敛，避免旧配置在身份尚未落地前被误判。
	if clientID == 0 {
		return true
	}
	return isHeroMQTTClientID(clientID)
}

func normalizeVideoSourceForMQTTRobotIdentity(source string, identity string) string {
	normalizedSource := normalizeVideoSource(source)
	if normalizedSource == "" {
		return ""
	}
	if normalizedSource == videoSourceCustom && !isHeroMQTTRobotIdentity(identity) {
		return videoSourceOfficial
	}
	return normalizedSource
}

// mqttHeroIdentityToClientID 保留旧命名入口，内部走新的机器人身份映射。
func mqttHeroIdentityToClientID(identity string) uint16 {
	return mqttRobotIdentityToClientID(identity)
}

// loadConfiguredUIBootstrap 读取后端启动阶段必须立即生效的 UI 配置。
// What: 从本地配置里提取 mqttRobotIdentity/mqttHeroIdentity 与 videoSource。
// Why: 这两项都必须在 frontend hydrate 前就决定后端行为，否则会先连错 broker 身份或先拉起错误的视频源。
func loadConfiguredUIBootstrap() configuredUIBootstrap {
	rawConfigJSON, err := config.LoadClientConfigJSON()
	if err != nil {
		log.Printf("[Warning] load UI bootstrap config failed: %v", err)
		return configuredUIBootstrap{}
	}

	// What: 只声明本次真正关心的最小配置结构。
	// Why: 后端启动期只需要极少数直接影响连接与视频源的字段，没必要把整个 UI 配置结构重复搬一遍。
	var snapshot struct {
		UIPanel struct {
			MQTTRobotIdentity string `json:"mqttRobotIdentity"`
			MQTTHeroIdentity  string `json:"mqttHeroIdentity"`
			VideoSource       string `json:"videoSource"`
		} `json:"uiPanel"`
	}

	if err := json.Unmarshal([]byte(rawConfigJSON), &snapshot); err != nil {
		log.Printf("[Warning] parse UI bootstrap config failed: %v", err)
		return configuredUIBootstrap{}
	}

	bootstrap := configuredUIBootstrap{}
	rawIdentity := snapshot.UIPanel.MQTTRobotIdentity
	if strings.TrimSpace(rawIdentity) == "" {
		rawIdentity = snapshot.UIPanel.MQTTHeroIdentity
	}
	normalizedIdentity := normalizeMQTTRobotIdentity(rawIdentity)
	if normalizedIdentity != "" {
		bootstrap.mqttRobotIdentity = normalizedIdentity
		bootstrap.hasMQTTRobotIdentity = true
	}

	normalizedSource := normalizeVideoSourceForMQTTRobotIdentity(snapshot.UIPanel.VideoSource, normalizedIdentity)
	if normalizedSource != "" {
		bootstrap.videoSource = normalizedSource
		bootstrap.hasVideoSource = true
	}

	return bootstrap
}

// resolveOfficialVideoConfig 解析当前进程是否需要拉起官方 UDP 视频链路。
// What: 允许测试态通过环境变量关闭官方 UDP，或覆写监听端口。
// Why: 本地 custom-only 联调只关心 0x0310，自定义链路必须能在已有 :3334 占用的机器上独立跑起来。
func resolveOfficialVideoConfig() officialVideoConfig {
	config := officialVideoConfig{
		port:     defaultOfficialVideoPort,
		disabled: false,
	}

	switch strings.ToLower(strings.TrimSpace(os.Getenv("RM_DISABLE_OFFICIAL_UDP"))) {
	case "1", "true", "yes", "on":
		config.disabled = true
	}

	rawPort := strings.TrimSpace(os.Getenv("RM_OFFICIAL_UDP_PORT"))
	if rawPort == "" {
		return config
	}

	parsedPort, err := strconv.Atoi(rawPort)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		log.Printf("[Warning] invalid RM_OFFICIAL_UDP_PORT=%q, fallback to %d", rawPort, defaultOfficialVideoPort)
		return config
	}

	config.port = parsedPort
	return config
}

// resolveMQTTClientID 探测当前裁判系统 broker 真正接受的机器人 clientID。
// What: 先按当前默认值尝试，失败后再在官方机器人 ID 列表里自动回退。
// Why: 现场最容易出现"程序写死了蓝方英雄，但当前接的是红方英雄"的错位；若不在启动期自动识别，MQTT 会一直 pending 且表面上看不出是 ID 错。
func resolveMQTTClientID(serverIP string, port int, preferredID uint16) uint16 {
	// What: 调用网络层最小握手探测。
	// Why: 只有 broker 自己的 CONNACK 才能最终裁定当前 robot ID 是否被接受。
	resolvedClientID, err := network.DetectAcceptedMQTTClientID(
		serverIP,
		port,
		preferredID,
		mqttClientIDProbeCandidates(),
		mqttClientIDProbeTimeout,
	)
	if err != nil {
		// What: 探测失败时回退到既有默认值。
		// Why: 这样至少保持现有行为不退化，同时把错误原因明确抛到日志里。
		log.Printf("[Warning] MQTT clientID probe failed, fallback to %d: %v", preferredID, err)
		return preferredID
	}

	if resolvedClientID != preferredID {
		// What: 只有探测结果与默认值不一致时才额外打印切换日志。
		// Why: 这样能把"为什么现场突然不是 101 而是 1"解释清楚，又不会在默认值本来正确时制造噪声。
		log.Printf("[network] MQTT clientID auto-switched from %d to %d based on broker CONNACK", preferredID, resolvedClientID)
	}

	return resolvedClientID
}

// resetIdentityScopedRMState 清空与当前本机身份直接绑定的裁判系统缓存。
// What: 在切换红/蓝英雄身份时清掉本机动态状态与静态状态缓存。
// Why: 这些缓存都带有"上一台机器人"的语义，不清理就会把旧身份的数据短暂带到新身份界面上。
func (a *App) resetIdentityScopedRMState() {
	a.rmStateMu.Lock()
	a.robotDynamicStatus = nil
	a.robotStaticStatus = make(map[uint32]*rmcp.RobotStaticStatus)
	a.rmStateMu.Unlock()
}

// bindMQTTClient 按给定机器人 clientID 创建并挂载 MQTT 长连接与订阅。
// What: 将"建连接 + 保存句柄 + 补订阅"收口到一条路径。
// Why: 启动期与运行时切换红/蓝英雄都需要完全一致的 MQTT 行为，不能让两条链路再各自维护一份订阅清单。
func (a *App) bindMQTTClient(clientID uint16) error {
	if a.mqtt != nil {
		a.mqtt.Close()
	}
	a.mqtt = nil

	// What: 用最终生效的机器人 clientID 建立 MQTT 长连接。
	// Why: 只有这里真正连上的身份与 configureRMIdentity 一致，后续 topic 数据与本机阵营映射才不会再次错位。
	mqtt, err := network.NewMQTTClient(a.mqttBrokerHost, a.mqttBrokerPort, clientID)
	if err != nil {
		return err
	}

	a.mqtt = mqtt
	if a.mqtt == nil {
		return nil
	}

	// What: 订阅比赛全局状态并桥接到顶部比赛元信息。
	// Why: 比分、阶段、局次和倒计时都必须来自官方 GameStatus，不能再由前端拼假值。
	if err := a.mqtt.SubscribeGameStatus(a.onGameStatus); err != nil {
		log.Printf("[Warning] GameStatus subscribe failed: %v", err)
	}

	// What: 订阅机器人动态状态并桥接到统一战斗态事件。
	// Why: 一键买弹必须依赖 can_remote_ammo 实时门控，不能靠前端猜测当前是否允许补给。
	if err := a.mqtt.SubscribeRobotDynamicStatus(a.onRobotDynamicStatus); err != nil {
		log.Printf("[Warning] RobotDynamicStatus subscribe failed: %v", err)
	}

	// What: 订阅基地与全队血量同步状态并桥接到顶部战况事件。
	// Why: 基地血条、双方编组血量都以这条 topic 为准，继续靠模拟值会持续覆盖真实数据。
	if err := a.mqtt.SubscribeGlobalUnitStatus(a.onGlobalUnitStatus); err != nil {
		log.Printf("[Warning] GlobalUnitStatus subscribe failed: %v", err)
	}

	if err := a.mqtt.SubscribeGlobalLogisticsStatus(a.onGlobalLogisticsStatus); err != nil {
		log.Printf("[Warning] GlobalLogisticsStatus subscribe failed: %v", err)
	}

	if err := a.mqtt.SubscribeGlobalSpecialMechanism(a.onGlobalSpecialMechanism); err != nil {
		log.Printf("[Warning] GlobalSpecialMechanism subscribe failed: %v", err)
	}

	if err := a.mqtt.SubscribeEvent(a.onRefereeEvent); err != nil {
		log.Printf("[Warning] Event subscribe failed: %v", err)
	}

	// What: 订阅机器人固定属性并缓存各机器人 max_health。
	// Why: 本机血量上限和每个机器人血量上限都来自这里，若不订阅就无法和 UI 做真实对齐。
	if err := a.mqtt.SubscribeRobotStaticStatus(a.onRobotStaticStatus); err != nil {
		log.Printf("[Warning] RobotStaticStatus subscribe failed: %v", err)
	}

	if err := a.mqtt.SubscribeRobotModuleStatus(a.onRobotModuleStatus); err != nil {
		log.Printf("[Warning] RobotModuleStatus subscribe failed: %v", err)
	}

	if err := a.mqtt.SubscribeRobotPosition(a.onRobotPosition); err != nil {
		log.Printf("[Warning] RobotPosition subscribe failed: %v", err)
	}

	if err := a.mqtt.SubscribeRadarInfo(a.onRadarInfo); err != nil {
		log.Printf("[Warning] RadarInfoToClient subscribe failed: %v", err)
	}

	if err := a.mqtt.SubscribeBuff(a.onBuff); err != nil {
		log.Printf("[Warning] Buff subscribe failed: %v", err)
	}

	if err := a.mqtt.SubscribeDeployModeStatus(a.onDeployModeStatus); err != nil {
		log.Printf("[Warning] DeployModeStatusSync subscribe failed: %v", err)
	}

	// What: 订阅机器人 0x0310 对应的自定义字节流。
	// Why: 自定义视频源已经建立在这条链路上，切换英雄身份后这条订阅也必须跟着恢复。
	if err := a.mqtt.SubscribeCustomByteBlock(a.onCustomByteBlock); err != nil {
		log.Printf("[Warning] CustomByteBlock subscribe failed: %v", err)
	}

	return nil
}

// configureRMIdentity 根据 MQTT clientID 固化本机阵营与相对机器人编号。
// What: 在 startup 早期就把本机身份解析成后续桥接会频繁复用的字段。
// Why: GlobalUnitStatus 的"己方/敌方"语义必须先落到 red/blue，不能等每次发事件时再临时猜。
func (a *App) configureRMIdentity(clientID uint16) {
	a.rmStateMu.Lock()
	a.mqttClientID = clientID
	a.selfSide = resolveTeamSideFromClientID(clientID)
	a.selfRelativeRobotID = resolveRelativeRobotID(uint32(clientID))
	if a.robotStaticStatus == nil {
		a.robotStaticStatus = make(map[uint32]*rmcp.RobotStaticStatus)
	}
	a.rmStateMu.Unlock()
}

// resolveTeamSideFromClientID 将裁判系统 clientID 归一化成 red/blue。
// What: 以协议附录的绝对 ID 规则区分红蓝双方。
// Why: 只有先知道本机属于哪一侧，才能把"己方基地血量"稳定映射到前端固定的 red/blue 结构。
func resolveTeamSideFromClientID(clientID uint16) string {
	if clientID >= 100 {
		return teamSideBlue
	}
	return teamSideRed
}

// resolveRelativeRobotID 将协议绝对机器人 ID 归一化成前端使用的相对编号。
// What: 把 101/102/103/104/107 这类蓝方绝对 ID 还原为 1/2/3/4/7。
// Why: 前端队伍卡和默认占位都以相对编号合并，后端若直接发绝对 ID 会打乱现有合并逻辑。
func resolveRelativeRobotID(absoluteRobotID uint32) uint32 {
	if absoluteRobotID >= 100 {
		return absoluteRobotID % 100
	}
	return absoluteRobotID
}

// resolveAbsoluteRobotID 根据 red/blue 阵营和相对编号恢复协议绝对 ID。
// What: 将前端队伍槽位编号映射回裁判系统消息里的真实机器人 ID。
// Why: RobotStaticStatus 缓存按绝对 ID 建索引，构建 match-state 时必须能从槽位反查到缓存项。
func resolveAbsoluteRobotID(side string, relativeRobotID uint32) uint32 {
	if side == teamSideBlue {
		return relativeRobotID + 100
	}
	return relativeRobotID
}

// resolveOpponentSide 返回给定阵营的对侧阵营。
// What: 将己方/敌方映射统一收口。
// Why: GlobalUnitStatus 里 allied/enemy 是相对概念，拼 red/blue 视图时必须先固定两侧关系。
func resolveOpponentSide(side string) string {
	if side == teamSideBlue {
		return teamSideRed
	}
	return teamSideBlue
}

// resolveRoleByRobotID 将相对机器人编号映射为角色字符串。
// What: 保持后端发出的 role 与前端既有枚举严格一致。
// Why: 这样可以继续复用前端现有 role -> 中文标签映射，而不必再开一套新字段。
func resolveRoleByRobotID(robotID uint32) string {
	if robotID == 1 {
		return "hero"
	}
	if robotID == 2 {
		return "engineer"
	}
	if robotID == 3 || robotID == 4 {
		return "infantry"
	}
	if robotID == 7 {
		return "sentry"
	}
	return "unknown"
}

// cacheRobotDynamicStatus 原子更新最新本机动态状态。
// What: 把 RobotDynamicStatus 收到的实时值收口到共享缓存。
// Why: 本机状态卡与顶部编组都要复用这份 10Hz 数据，不能让两个 emit 链各自持有不同快照。
func (a *App) cacheRobotDynamicStatus(status *rmcp.RobotDynamicStatus) {
	if status == nil {
		return
	}

	a.rmStateMu.Lock()
	a.robotDynamicStatus = status
	a.rmStateMu.Unlock()
}

// cacheGameStatus 原子更新最新比赛元信息状态。
// What: 把比分、阶段、局次与倒计时集中缓存下来。
// Why: 顶部中间条要严格按官方 GameStatus 渲染，不能再依赖前端假字段兜底。
func (a *App) cacheGameStatus(status *rmcp.GameStatus) {
	if status == nil {
		return
	}

	a.rmStateMu.Lock()
	a.gameStatus = status
	a.rmStateMu.Unlock()
}

// cacheGlobalUnitStatus 原子更新最新全局血量状态。
// What: 把基地与双方编组血量集中缓存下来。
// Why: RobotStaticStatus 到达时需要重用上一帧的全局血量，不能要求两个 topic 同时到达才更新 UI。
func (a *App) cacheGlobalUnitStatus(status *rmcp.GlobalUnitStatus) {
	if status == nil {
		return
	}

	a.rmStateMu.Lock()
	a.globalUnitStatus = status
	a.rmStateMu.Unlock()
}

// cacheRobotStaticStatus 原子更新按绝对 robot_id 索引的固定属性缓存。
// What: 将 max_health、连接状态等低频信息按机器人维度持久化。
// Why: 队伍卡每个槽位的血量上限都依赖这张表，不能只保留"最近一条静态状态"。
func (a *App) cacheRobotStaticStatus(status *rmcp.RobotStaticStatus) {
	if status == nil {
		return
	}

	a.rmStateMu.Lock()
	if a.robotStaticStatus == nil {
		a.robotStaticStatus = make(map[uint32]*rmcp.RobotStaticStatus)
	}

	absoluteRobotID := status.GetRobotId()
	if absoluteRobotID == 0 {
		// What: 少数异常回包若未显式携带 robot_id，则退回当前客户端绑定的绝对机器人 ID。
		// Why: 至少保证本机 max_health 不会因为一个缺字段回包而彻底丢失。
		absoluteRobotID = uint32(a.mqttClientID)
	}
	a.robotStaticStatus[absoluteRobotID] = status
	a.rmStateMu.Unlock()
}

// buildCombatStatePayload 由缓存组合前端统一 combat-state 事件。
// What: 将本机实时血量、血量上限和现有火控字段拼成一份 snake_case payload。
// Why: 前端已经围绕 combat-state 做单一状态源收敛，后端应继续沿用同一入口而不是再造新事件名。
func (a *App) buildCombatStatePayload() (map[string]interface{}, bool) {
	a.rmStateMu.RLock()
	defer a.rmStateMu.RUnlock()

	if a.robotDynamicStatus == nil {
		return nil, false
	}

	payload := map[string]interface{}{
		"hp":                          a.robotDynamicStatus.GetCurrentHealth(),
		"ammo":                        a.robotDynamicStatus.GetRemainingAmmo(),
		"heat":                        a.robotDynamicStatus.GetCurrentHeat(),
		"last_projectile_fire_rate":   a.robotDynamicStatus.GetLastProjectileFireRate(),
		"current_chassis_energy":      a.robotDynamicStatus.GetCurrentChassisEnergy(),
		"current_buffer_energy":       a.robotDynamicStatus.GetCurrentBufferEnergy(),
		"current_experience":          a.robotDynamicStatus.GetCurrentExperience(),
		"experience_for_upgrade":      a.robotDynamicStatus.GetExperienceForUpgrade(),
		"total_projectiles_fired":     a.robotDynamicStatus.GetTotalProjectilesFired(),
		"is_out_of_combat":            a.robotDynamicStatus.GetIsOutOfCombat(),
		"out_of_combat_countdown_sec": a.robotDynamicStatus.GetOutOfCombatCountdown(),
		"can_remote_heal":             a.robotDynamicStatus.GetCanRemoteHeal(),
		"can_remote_ammo":             a.robotDynamicStatus.GetCanRemoteAmmo(),
		"updated_at":                  time.Now().UnixMilli(),
	}

	if selfStaticStatus := a.robotStaticStatus[uint32(a.mqttClientID)]; selfStaticStatus != nil && selfStaticStatus.GetMaxHealth() > 0 {
		// What: 仅在拿到真实 max_health 后才写入 max_hp。
		// Why: 若这里回退到默认值，会把前端已经缓存的正确上限反向冲掉。
		payload["max_hp"] = selfStaticStatus.GetMaxHealth()
	}

	return payload, true
}

// buildMatchStatePayload 由缓存组合前端统一 match-state 事件。
// What: 同时桥接官方比赛元信息与红蓝血量编组。
// Why: 顶部中间条和顶部两侧编组来自不同 topic，后端必须允许它们分批到达，而不是要求所有字段一次性齐全。
func (a *App) buildMatchStatePayload() (map[string]interface{}, bool) {
	a.rmStateMu.RLock()
	defer a.rmStateMu.RUnlock()

	payload := map[string]interface{}{
		"updated_at": time.Now().UnixMilli(),
	}

	if a.gameStatus != nil {
		// What: 一旦拿到 GameStatus，就原样桥接官方字段。
		// Why: 前端顶部中间条需要直接消费这些字段做真实阶段/比分展示，不能再让后端改写成自造 round_label/economy。
		payload["game_status_ready"] = true
		payload["current_round"] = a.gameStatus.GetCurrentRound()
		payload["total_rounds"] = a.gameStatus.GetTotalRounds()
		payload["red_score"] = a.gameStatus.GetRedScore()
		payload["blue_score"] = a.gameStatus.GetBlueScore()
		payload["current_stage"] = a.gameStatus.GetCurrentStage()
		payload["stage_countdown_sec"] = a.gameStatus.GetStageCountdownSec()
		payload["stage_elapsed_sec"] = a.gameStatus.GetStageElapsedSec()
		payload["is_paused"] = a.gameStatus.GetIsPaused()
	}

	if a.globalUnitStatus != nil {
		selfSide := a.selfSide
		if selfSide == "" {
			selfSide = resolveTeamSideFromClientID(a.mqttClientID)
		}
		opponentSide := resolveOpponentSide(selfSide)

		allyHealth := make([]uint32, 0, len(officialRobotOrder))
		enemyHealth := make([]uint32, 0, len(officialRobotOrder))
		for index, hp := range a.globalUnitStatus.GetRobotHealth() {
			if index < len(officialRobotOrder) {
				allyHealth = append(allyHealth, hp)
				continue
			}
			if index < len(officialRobotOrder)*2 {
				enemyHealth = append(enemyHealth, hp)
			}
		}

		redBaseHP := a.globalUnitStatus.GetBaseHealth()
		redBaseStatus := a.globalUnitStatus.GetBaseStatus()
		redBaseShield := a.globalUnitStatus.GetBaseShield()
		redOutpostHP := a.globalUnitStatus.GetOutpostHealth()
		redOutpostStatus := a.globalUnitStatus.GetOutpostStatus()
		redTotalDamage := a.globalUnitStatus.GetTotalDamageAlly()
		blueBaseHP := a.globalUnitStatus.GetEnemyBaseHealth()
		blueBaseStatus := a.globalUnitStatus.GetEnemyBaseStatus()
		blueBaseShield := a.globalUnitStatus.GetEnemyBaseShield()
		blueOutpostHP := a.globalUnitStatus.GetEnemyOutpostHealth()
		blueOutpostStatus := a.globalUnitStatus.GetEnemyOutpostStatus()
		blueTotalDamage := a.globalUnitStatus.GetTotalDamageEnemy()
		redUnits := a.buildTeamUnitsLocked(selfSide, allyHealth)
		blueUnits := a.buildTeamUnitsLocked(opponentSide, enemyHealth)
		if selfSide == teamSideBlue {
			redBaseHP = a.globalUnitStatus.GetEnemyBaseHealth()
			redBaseStatus = a.globalUnitStatus.GetEnemyBaseStatus()
			redBaseShield = a.globalUnitStatus.GetEnemyBaseShield()
			redOutpostHP = a.globalUnitStatus.GetEnemyOutpostHealth()
			redOutpostStatus = a.globalUnitStatus.GetEnemyOutpostStatus()
			redTotalDamage = a.globalUnitStatus.GetTotalDamageEnemy()
			blueBaseHP = a.globalUnitStatus.GetBaseHealth()
			blueBaseStatus = a.globalUnitStatus.GetBaseStatus()
			blueBaseShield = a.globalUnitStatus.GetBaseShield()
			blueOutpostHP = a.globalUnitStatus.GetOutpostHealth()
			blueOutpostStatus = a.globalUnitStatus.GetOutpostStatus()
			blueTotalDamage = a.globalUnitStatus.GetTotalDamageAlly()
			redUnits = a.buildTeamUnitsLocked(teamSideRed, enemyHealth)
			blueUnits = a.buildTeamUnitsLocked(teamSideBlue, allyHealth)
		}

		// What: 红蓝基地与编组仍然沿用前端已稳定消费的 red/blue 结构。
		// Why: 用户已经确认过顶部基地血条与队伍卡布局，这一层只需要改真实数据来源，不应再把前端结构一起打碎。
		payload["global_status_ready"] = true
		payload["red"] = map[string]interface{}{
			"base_hp":        redBaseHP,
			"base_max_hp":    baseMaxHealth,
			"base_status":    redBaseStatus,
			"base_shield":    redBaseShield,
			"outpost_hp":     redOutpostHP,
			"outpost_status": redOutpostStatus,
			"total_damage":   redTotalDamage,
			"units":          redUnits,
		}
		payload["blue"] = map[string]interface{}{
			"base_hp":        blueBaseHP,
			"base_max_hp":    baseMaxHealth,
			"base_status":    blueBaseStatus,
			"base_shield":    blueBaseShield,
			"outpost_hp":     blueOutpostHP,
			"outpost_status": blueOutpostStatus,
			"total_damage":   blueTotalDamage,
			"units":          blueUnits,
		}
	}

	_, hasGameStatus := payload["game_status_ready"]
	_, hasGlobalUnitStatus := payload["red"]
	if !hasGameStatus && !hasGlobalUnitStatus {
		return nil, false
	}

	return payload, true
}

// buildTeamUnitsLocked 在持有 rmStateMu 读锁期间构建单侧队伍卡 payload。
// What: 按协议固定顺序生成 1/2/3/4/7 五个槽位的血量视图。
// Why: 前端 store 是按 robot_id 稳定合并的，只要顺序和 robot_id 固定，UI 就不会抖动或串位。
func (a *App) buildTeamUnitsLocked(side string, healthValues []uint32) []map[string]interface{} {
	units := make([]map[string]interface{}, 0, len(officialRobotOrder))
	prefix := "R"
	if side == teamSideBlue {
		prefix = "B"
	}

	for index, relativeRobotID := range officialRobotOrder {
		hp := uint32(0)
		if index < len(healthValues) {
			hp = healthValues[index]
		}

		absoluteRobotID := resolveAbsoluteRobotID(side, relativeRobotID)
		if absoluteRobotID == uint32(a.mqttClientID) && a.robotDynamicStatus != nil {
			// What: 本机槽位优先使用 10Hz 的 RobotDynamicStatus 覆盖 1Hz 的全局血量。
			// Why: 这样顶部编组与本机状态卡能保持同一拍，不会出现"本机卡已掉血但顶部队伍卡还停在上一秒"的错位。
			hp = a.robotDynamicStatus.GetCurrentHealth()
		}

		unitPayload := map[string]interface{}{
			"robot_id":   relativeRobotID,
			"role":       resolveRoleByRobotID(relativeRobotID),
			"hp":         hp,
			"avatar_key": fmt.Sprintf("%s%d", prefix, relativeRobotID),
			"online":     true,
		}

		if staticStatus := a.robotStaticStatus[absoluteRobotID]; staticStatus != nil {
			if staticStatus.GetMaxHealth() > 0 {
				// What: 只在静态状态确实给到 max_health 时才下发 max_hp。
				// Why: 这样前端才能把"缺字段"理解成"不覆盖旧值"，而不是被无意义默认值洗掉。
				unitPayload["max_hp"] = staticStatus.GetMaxHealth()
			}

			// What: online 表示链路和上场状态是否允许该槽位被视为可用。
			// Why: 机器人即便战亡也可能仍在线，因此不能把 alive_state 直接误当成离线判据。
			unitPayload["online"] = staticStatus.GetConnectionState() == 1 && staticStatus.GetFieldState() == 0
		}

		units = append(units, unitPayload)
	}

	return units
}

// setVideoPlayerState 原子替换指定来源的原生视频层句柄与错误态。
// What: 把 official/custom 的播放器句柄和错误文案都收口到同一入口。
// Why: 双视频常驻后，两路运行态必须各自独立维护，不能再共用单一 videoPlayer 槽位。
func (a *App) setVideoPlayerState(source string, player videoRuntimePlayer, playerErr string) {
	a.videoMu.Lock()
	switch source {
	case videoSourceCustom:
		a.customPlayer = player
		a.customPlayerErr = playerErr
	default:
		a.officialPlayer = player
		a.officialPlayerErr = playerErr
	}
	a.videoMu.Unlock()
}

// setVideoDOMReady 记录当前 HUD 主窗是否已经进入可解析布局的阶段。
// What: 在 domReady 后再允许双播放器启动与重贴层。
// Why: startup 时虽然已经拿到 ctx，但 HUD 顶层窗几何与身份仍未稳定，过早拉起原生视频层只会把错误状态永久化。
func (a *App) setVideoDOMReady(ready bool) {
	a.videoMu.Lock()
	a.videoDOMReady = ready
	a.videoMu.Unlock()
}

// getVideoRuntimeState 导出当前双视频运行时快照。
// What: 在一个读锁里同时读取 official/custom 播放器、错误态、当前主源和 DOM readiness。
// Why: 双视频布局同步和状态广播都依赖同一拍快照；若拆成多次读取，主次关系和窗口层状态很容易互相错位。
func (a *App) getVideoRuntimeState() videoRuntimeState {
	a.videoMu.RLock()
	state := videoRuntimeState{
		officialPlayer:           a.officialPlayer,
		customPlayer:             a.customPlayer,
		officialPlayerErr:        a.officialPlayerErr,
		customPlayerErr:          a.customPlayerErr,
		activeVideoSource:        a.activeVideoSource,
		lastCustomByteBlockAt:    a.lastCustomByteBlockAt,
		officialBootstrapReady:   len(a.officialBootstrapFrame) > 0,
		officialBootstrapPending: a.officialBootstrapPending,
		domReady:                 a.videoDOMReady,
	}
	a.videoMu.RUnlock()
	if a.hevcSyncGate != nil {
		state.officialSyncReady = a.hevcSyncGate.IsSynced()
	}
	return state
}

// normalizeVideoSource 将外部视频源字符串归一化为固定枚举。
// What: 统一收口前端与后端传入的 source 值。
// Why: Wails 前端、配置文件和后端默认值都要共享同一套取值，不然切源时很容易因大小写或空格导致状态失效。
func normalizeVideoSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", videoSourceOfficial:
		return videoSourceOfficial
	case videoSourceCustom:
		return videoSourceCustom
	default:
		return ""
	}
}

func otherVideoSource(source string) string {
	if normalizeVideoSource(source) == videoSourceCustom {
		return videoSourceOfficial
	}
	return videoSourceCustom
}

// setActiveVideoSource 原子更新当前视频源。
// What: 仅负责写入当前源枚举。
// Why: 源切换和状态广播都会读这个字段，必须在同一把锁下更新，避免前端短时间读到旧源与新提示的错配。
func (a *App) setActiveVideoSource(source string) {
	a.videoMu.Lock()
	a.activeVideoSource = source
	a.videoMu.Unlock()
}

// noteCustomByteBlockReceipt 记录最近一次 0x0310 自定义数据到达时间，并返回上一包时间。
// What: 每次收到有效 CustomByteBlock 都刷新最后活跃时间。
// Why: 自定义源状态需要同时反映"链路上是否还在来数据"和"播放器是否已出画"；上一包时间还用于断流恢复时主动等待下一个 IDR。
func (a *App) noteCustomByteBlockReceipt(timestamp int64) int64 {
	a.videoMu.Lock()
	previous := a.lastCustomByteBlockAt
	a.lastCustomByteBlockAt = timestamp
	a.videoMu.Unlock()
	return previous
}

func (a *App) markCustomStreamNeedsResync() {
	a.videoMu.Lock()
	a.customStreamNeedsResync = true
	a.videoMu.Unlock()
}

func (a *App) consumeCustomStreamNeedsResync() bool {
	a.videoMu.Lock()
	needsResync := a.customStreamNeedsResync
	a.customStreamNeedsResync = false
	a.videoMu.Unlock()
	return needsResync
}

func (a *App) setOfficialBootstrapFrame(frame []byte) {
	if len(frame) == 0 {
		return
	}

	copiedFrame := append([]byte(nil), frame...)
	a.videoMu.Lock()
	a.officialBootstrapFrame = copiedFrame
	a.videoMu.Unlock()
}

func (a *App) markOfficialBootstrapPending() {
	a.videoMu.Lock()
	a.officialBootstrapPending = true
	a.videoMu.Unlock()
}

func (a *App) resetOfficialStreamForPlayerRestart() {
	if a.hevcSyncGate != nil {
		a.hevcSyncGate.Reset()
	}

	a.videoMu.Lock()
	a.officialBootstrapFrame = nil
	a.officialBootstrapPending = true
	a.videoMu.Unlock()

	log.Printf("[official-video] reset HEVC sync gate because official ffplay restarted; wait for next VPS/SPS/PPS+IRAP")
}

func (a *App) consumeOfficialBootstrapFrame(currentFrame []byte) ([]byte, bool) {
	a.videoMu.Lock()
	defer a.videoMu.Unlock()

	if !a.officialBootstrapPending || len(a.officialBootstrapFrame) == 0 {
		return nil, false
	}

	bootstrapFrame := append([]byte(nil), a.officialBootstrapFrame...)
	a.officialBootstrapPending = false
	return bootstrapFrame, bytes.Equal(bootstrapFrame, currentFrame)
}

// ensureHUDWindowForeground 重申主 HUD 窗口的全屏置顶状态。
// What: 在拉起、重建或关闭任意原生视频层前后，重复确认 Wails 主窗口仍停在最上层。
// Why: Wayland/XWayland 混合环境下，独立 ffplay 窗创建瞬间仍可能短暂抢前台，这里必须继续补一层保险。
func (a *App) ensureHUDWindowForeground() {
	if a.ctx == nil {
		return
	}
	select {
	case <-a.ctx.Done():
		return
	default:
	}

	runtime.WindowShow(a.ctx)
	runtime.WindowFullscreen(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)

	go func(appCtx context.Context) {
		ticker := time.NewTicker(180 * time.Millisecond)
		defer ticker.Stop()

		for attempt := 0; attempt < 4; attempt++ {
			select {
			case <-appCtx.Done():
				return
			case <-ticker.C:
				runtime.WindowShow(appCtx)
				runtime.WindowFullscreen(appCtx)
				runtime.WindowSetAlwaysOnTop(appCtx, true)
			}
		}
	}(a.ctx)
}

func clampInt(value int, min int, max int) int {
	if max < min {
		return min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func calculatePiPVideoSlot(hudLayout video.VideoWindowLayout, source string) (int, int, int) {
	isCustom := source == videoSourceCustom
	margin := clampInt(int(math.Round(float64(hudLayout.Width)*0.014)), 12, 24)
	if hudLayout.Width <= 980 {
		margin = 12
	}

	aspectWidth := 16.0
	aspectHeight := 9.0
	width := clampInt(int(math.Round(float64(hudLayout.Width)*0.29)), 320, 560)
	if isCustom {
		aspectWidth = 1
		aspectHeight = 1
		width = clampInt(int(math.Round(float64(hudLayout.Width)*0.24)), 260, 420)
	}
	if hudLayout.Width <= 980 {
		width = int(math.Round(float64(hudLayout.Width) * 0.46))
		if isCustom {
			width = int(math.Round(float64(hudLayout.Width) * 0.38))
		}
		width = clampInt(width, 1, 300)
		if isCustom {
			width = clampInt(width, 1, 220)
		}
	}

	maxAvailableWidth := clampInt(hudLayout.Width-margin*2, 1, hudLayout.Width)
	maxAvailableHeight := clampInt(hudLayout.Height-margin*2, 1, hudLayout.Height)
	width = clampInt(width, 1, maxAvailableWidth)
	height := int(math.Round(float64(width) * aspectHeight / aspectWidth))
	if height > maxAvailableHeight {
		height = maxAvailableHeight
		width = int(math.Round(float64(height) * aspectWidth / aspectHeight))
	}
	width = clampInt(width, 1, maxAvailableWidth)
	height = clampInt(height, 1, maxAvailableHeight)

	return margin, width, height
}

func buildPiPVideoWindowLayout(hudLayout video.VideoWindowLayout, source string) video.VideoWindowLayout {
	margin, width, height := calculatePiPVideoSlot(hudLayout, source)

	return video.VideoWindowLayout{
		Left:               hudLayout.Left + hudLayout.Width - margin - width,
		Top:                hudLayout.Top + margin,
		Width:              width,
		Height:             height,
		HUDWindowID:        hudLayout.HUDWindowID,
		StackAboveWindowID: hudLayout.HUDWindowID,
	}
}

func buildDualVideoWindowLayouts(hudLayout video.VideoWindowLayout, activeSource string, officialWindowID uint32, customWindowID uint32) (video.VideoWindowLayout, video.VideoWindowLayout) {
	officialLayout := hudLayout
	customLayout := hudLayout

	switch normalizeVideoSource(activeSource) {
	case videoSourceCustom:
		officialLayout = buildPiPVideoWindowLayout(hudLayout, videoSourceOfficial)
		customLayout.StackAboveWindowID = officialWindowID
		if customLayout.StackAboveWindowID == 0 {
			customLayout.StackAboveWindowID = hudLayout.HUDWindowID
		}
	default:
		customLayout = buildPiPVideoWindowLayout(hudLayout, videoSourceCustom)
		officialLayout.StackAboveWindowID = customWindowID
		if officialLayout.StackAboveWindowID == 0 {
			officialLayout.StackAboveWindowID = hudLayout.HUDWindowID
		}
	}

	return officialLayout, customLayout
}

func shouldRunOfficialPlayer(state videoRuntimeState, officialVideoDisabled bool) bool {
	if officialVideoDisabled {
		return false
	}
	return state.officialSyncReady || state.officialBootstrapReady || state.officialPlayer != nil
}

func shouldRunCustomPlayer(state videoRuntimeState) bool {
	return normalizeVideoSource(state.activeVideoSource) == videoSourceCustom ||
		isRecentTimestamp(state.lastCustomByteBlockAt, videoSourceActiveThreshold)
}

// shouldEnqueueOfficialFrame 判断当前官方 H.265 帧是否需要送入 official ffplay。
// What: official 主画面和 PiP 都持续接收完整已同步 HEVC 帧。
// Why: PiP 也必须保持真实连续预览；只抽关键帧会让右上角看似在线但画面不流畅，且会掩盖 official 链路真实延迟。
func shouldEnqueueOfficialFrame(state videoRuntimeState, syncDecision video.HEVCSyncDecision) bool {
	_ = state
	_ = syncDecision
	return true
}

func deltaUint64(current uint64, previous uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}

func ratio(numerator uint64, denominator uint64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func roundRate(value float64) float64 {
	return math.Round(value*100) / 100
}

func (a *App) updateVideoRateSample(udpStats network.UDPReceiverStats, officialStats video.PlayerSnapshot, customStats video.PlayerSnapshot, customDiagnostics video.H264Diagnostics, customBlockCount uint64) videoRateSample {
	now := time.Now()
	officialDrops := udpStats.IncompleteFrameDrops + udpStats.TimeoutFrameDrops + udpStats.InvalidFrameDrops + officialStats.DecodeDropFrames
	customDrops := customDiagnostics.DropsOut + customStats.DecodeDropFrames
	customCorrupt := customStats.CorruptFrames

	a.videoRateMu.Lock()
	defer a.videoRateMu.Unlock()

	if a.videoRateSample.at.IsZero() {
		a.videoRateSample.at = now
		a.videoRateSample.officialPackets = udpStats.PacketCount
		a.videoRateSample.officialFrames = udpStats.FrameCount
		a.videoRateSample.officialDrops = officialDrops
		a.videoRateSample.customBlocks = customBlockCount
		a.videoRateSample.customAUs = customDiagnostics.FramesOut
		a.videoRateSample.customDrops = customDrops
		a.videoRateSample.customCorrupt = customCorrupt
		return a.videoRateSample
	}

	elapsed := now.Sub(a.videoRateSample.at)
	if elapsed < 500*time.Millisecond {
		return a.videoRateSample
	}

	elapsedSeconds := elapsed.Seconds()
	officialPacketDelta := deltaUint64(udpStats.PacketCount, a.videoRateSample.officialPackets)
	officialFrameDelta := deltaUint64(udpStats.FrameCount, a.videoRateSample.officialFrames)
	officialDropDelta := deltaUint64(officialDrops, a.videoRateSample.officialDrops)
	customBlockDelta := deltaUint64(customBlockCount, a.videoRateSample.customBlocks)
	customAUDelta := deltaUint64(customDiagnostics.FramesOut, a.videoRateSample.customAUs)
	customDropDelta := deltaUint64(customDrops, a.videoRateSample.customDrops)
	customCorruptDelta := deltaUint64(customCorrupt, a.videoRateSample.customCorrupt)

	a.videoRateSample.at = now
	a.videoRateSample.officialPackets = udpStats.PacketCount
	a.videoRateSample.officialFrames = udpStats.FrameCount
	a.videoRateSample.officialDrops = officialDrops
	a.videoRateSample.customBlocks = customBlockCount
	a.videoRateSample.customAUs = customDiagnostics.FramesOut
	a.videoRateSample.customDrops = customDrops
	a.videoRateSample.customCorrupt = customCorrupt
	a.videoRateSample.officialPacketHz = roundRate(float64(officialPacketDelta) / elapsedSeconds)
	a.videoRateSample.officialFrameHz = roundRate(float64(officialFrameDelta) / elapsedSeconds)
	a.videoRateSample.officialDropRate = roundRate(ratio(officialDropDelta, officialFrameDelta+officialDropDelta))
	a.videoRateSample.customBlockHz = roundRate(float64(customBlockDelta) / elapsedSeconds)
	a.videoRateSample.customAUFPS = roundRate(float64(customAUDelta) / elapsedSeconds)
	a.videoRateSample.customDropRate = roundRate(ratio(customDropDelta, customAUDelta+customDropDelta))
	a.videoRateSample.customCorruptRate = roundRate(ratio(customCorruptDelta, customAUDelta+customCorruptDelta))
	return a.videoRateSample
}

func (a *App) createVideoPlayer(source string) (*video.NativePlayer, error) {
	switch source {
	case videoSourceCustom:
		if a.h264Reassembler != nil {
			a.h264Reassembler.Reset()
		}
		player, err := video.NewCustomNativePlayer(appWindowTitle)
		if err != nil {
			return nil, err
		}
		player.SetUnexpectedExitHook(func() {
			a.markCustomStreamNeedsResync()
			a.emitVideoStateSnapshot()
		})
		return player, nil
	default:
		player, err := video.NewOfficialNativePlayer(appWindowTitle)
		if err != nil {
			return nil, err
		}
		player.SetUnexpectedExitHook(func() {
			a.resetOfficialStreamForPlayerRestart()
			a.emitVideoStateSnapshot()
		})
		return player, nil
	}
}

func (a *App) syncVideoPlayers() {
	a.videoLifecycleMu.Lock()
	defer a.videoLifecycleMu.Unlock()

	state := a.getVideoRuntimeState()
	if !state.domReady || a.ctx == nil {
		return
	}

	nextHUDLayout, err := a.resolveVideoWindowLayout()
	if err != nil {
		return
	}

	runOfficialPlayer := shouldRunOfficialPlayer(state, a.officialVideoDisabled)
	runCustomPlayer := shouldRunCustomPlayer(state)

	officialWindowID := uint32(0)
	if state.officialPlayer != nil {
		officialWindowID = state.officialPlayer.Snapshot().WindowID
	}
	customWindowID := uint32(0)
	if state.customPlayer != nil {
		customWindowID = state.customPlayer.Snapshot().WindowID
	}

	officialLayout, customLayout := buildDualVideoWindowLayouts(nextHUDLayout, state.activeVideoSource, officialWindowID, customWindowID)

	if !runOfficialPlayer {
		if state.officialPlayer != nil {
			log.Printf("[video] stop official player because official source is not runnable")
			state.officialPlayer.Stop()
		}
		a.videoMu.Lock()
		a.officialBootstrapPending = false
		a.videoMu.Unlock()
		a.setVideoPlayerState(videoSourceOfficial, nil, "")
	} else {
		if state.officialPlayer == nil {
			if !state.officialSyncReady && !state.officialBootstrapReady {
				a.setVideoPlayerState(videoSourceOfficial, nil, "")
				goto syncCustomPlayer
			}

			player, createErr := a.createVideoPlayer(videoSourceOfficial)
			if createErr != nil {
				playerErr := fmt.Sprintf("原生视频层不可用: %v", createErr)
				a.setVideoPlayerState(videoSourceOfficial, nil, playerErr)
			} else if startErr := player.Start(officialLayout); startErr != nil {
				playerErr := fmt.Sprintf("原生视频层启动失败: %v", startErr)
				a.setVideoPlayerState(videoSourceOfficial, nil, playerErr)
			} else {
				a.markOfficialBootstrapPending()
				a.setVideoPlayerState(videoSourceOfficial, player, "")
				log.Printf("[video] official player attached to runtime")
			}
		} else if !videoWindowLayoutsEqual(state.officialPlayer.Layout(), officialLayout) {
			if err := state.officialPlayer.UpdateLayout(officialLayout); err == nil {
				a.setVideoPlayerState(videoSourceOfficial, state.officialPlayer, "")
			}
		}
	}

syncCustomPlayer:
	if !runCustomPlayer {
		if state.customPlayer != nil {
			log.Printf("[video] stop custom player while official priority path is active")
			state.customPlayer.Stop()
		}
		a.setVideoPlayerState(videoSourceCustom, nil, "")
	} else if state.customPlayer == nil {
		player, createErr := a.createVideoPlayer(videoSourceCustom)
		if createErr != nil {
			playerErr := fmt.Sprintf("自定义视频层不可用: %v", createErr)
			a.setVideoPlayerState(videoSourceCustom, nil, playerErr)
		} else if startErr := player.Start(customLayout); startErr != nil {
			playerErr := fmt.Sprintf("自定义视频层启动失败: %v", startErr)
			a.setVideoPlayerState(videoSourceCustom, nil, playerErr)
		} else {
			a.setVideoPlayerState(videoSourceCustom, player, "")
			log.Printf("[video] custom player attached to runtime")
		}
	} else if !videoWindowLayoutsEqual(state.customPlayer.Layout(), customLayout) {
		if err := state.customPlayer.UpdateLayout(customLayout); err == nil {
			a.setVideoPlayerState(videoSourceCustom, state.customPlayer, "")
		}
	}

	// What: 第二次按最新窗口 ID 重算布局锚点。
	// Why: PiP 窗口第一次启动时其 X11 ID 要等 ffplay 真正建窗后才可见，主画面必须在拿到这个 ID 后再补一次 `Main < PiP` 的层级关系。
	state = a.getVideoRuntimeState()
	if state.officialPlayer != nil {
		officialWindowID = state.officialPlayer.Snapshot().WindowID
	}
	if state.customPlayer != nil {
		customWindowID = state.customPlayer.Snapshot().WindowID
	}
	runOfficialPlayer = shouldRunOfficialPlayer(state, a.officialVideoDisabled)
	runCustomPlayer = shouldRunCustomPlayer(state)
	officialLayout, customLayout = buildDualVideoWindowLayouts(nextHUDLayout, state.activeVideoSource, officialWindowID, customWindowID)
	if state.officialPlayer != nil && !videoWindowLayoutsEqual(state.officialPlayer.Layout(), officialLayout) {
		_ = state.officialPlayer.UpdateLayout(officialLayout)
	}
	if runCustomPlayer && state.customPlayer != nil && !videoWindowLayoutsEqual(state.customPlayer.Layout(), customLayout) {
		_ = state.customPlayer.UpdateLayout(customLayout)
	}

	a.ensureHUDWindowForeground()
}

func (a *App) stopAllVideoPlayers() {
	a.videoLifecycleMu.Lock()
	defer a.videoLifecycleMu.Unlock()

	state := a.getVideoRuntimeState()
	if state.officialPlayer != nil {
		log.Printf("[video] stop official player")
		state.officialPlayer.Stop()
	}
	if state.customPlayer != nil {
		log.Printf("[video] stop custom player")
		state.customPlayer.Stop()
	}

	a.setVideoPlayerState(videoSourceOfficial, nil, "")
	a.setVideoPlayerState(videoSourceCustom, nil, "")
}

// startup 钩子，完成各类底层服务的初始化。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("[App] Starting RM 2026 Custom Client Backend...")

	// What: 优先读取用户在客户端里明确选择的机器人身份。
	// Why: 当前用户已经要求在客户端里显式选择阵营与机器人身份；一旦有显式配置，就必须优先尊重这个选择而不是继续后台猜测。
	configuredUIBootstrap := loadConfiguredUIBootstrap()
	if configuredUIBootstrap.hasVideoSource {
		// What: 在 DOM ready 前就落下本地保存的视频源选择。
		// Why: 若仍先按 official 起播放器，再等前端 hydrate 切到 custom，本地联调会额外引入一轮错误的官方源初始化。
		a.setActiveVideoSource(configuredUIBootstrap.videoSource)
		log.Printf("[video] configured active video source=%s", configuredUIBootstrap.videoSource)
	}

	brokerConfig := resolveMQTTBrokerConfig()
	a.mqttBrokerHost = brokerConfig.host
	a.mqttBrokerPort = brokerConfig.port
	log.Printf("[network] MQTT broker target %s:%d", a.mqttBrokerHost, a.mqttBrokerPort)

	resolvedMQTTClientID := defaultMQTTClientID
	if configuredUIBootstrap.hasMQTTRobotIdentity {
		// What: 已有显式选择时直接落到对应 clientID。
		// Why: 用户既然已经在客户端里做了明确选择，后端就不应该再跨到另一侧机器人去自动回退。
		resolvedMQTTClientID = mqttRobotIdentityToClientID(configuredUIBootstrap.mqttRobotIdentity)
		log.Printf("[network] MQTT robot identity configured as %s client_id=%d", configuredUIBootstrap.mqttRobotIdentity, resolvedMQTTClientID)
	} else {
		// What: 历史配置还没有新选择项时，临时保留 broker 自动探测兜底。
		// Why: 这样老配置用户升级后仍可直接起得来，不会因为还没点过新设置而再次卡死在错误 clientID 上。
		resolvedMQTTClientID = resolveMQTTClientID(a.mqttBrokerHost, a.mqttBrokerPort, defaultMQTTClientID)
	}

	// What: 按探测后的最终 clientID 固化本机阵营与相对机器人编号。
	// Why: 后续所有己方/敌方的血量映射都依赖这份身份信息，必须与最终实际连接的 broker 身份严格一致。
	a.configureRMIdentity(resolvedMQTTClientID)
	a.resetIdentityScopedRMState()

	// 1. 初始化 MQTT 裁判系统长连接。
	// What: 继续保留原有控制链和比赛态链路。
	// Why: 本轮重构聚焦视频层，输入和比赛态不应该被一并打乱。
	if err := a.bindMQTTClient(resolvedMQTTClientID); err != nil {
		log.Printf("[Warning] MQTT Connect Failed, retrying in background: %v", err)
	}

	officialVideoConfig := resolveOfficialVideoConfig()
	a.officialVideoPort = officialVideoConfig.port
	a.officialVideoDisabled = officialVideoConfig.disabled

	// 2. 启动 UDP 图传缓冲合并器。
	// What: 正式环境继续在 Go 层完成官方 UDP 分片组帧；测试态可显式跳过。
	// Why: 本地 custom-only 联调只验证 0x0310 自定义链路，不能再被官方 :3334 端口占用强制打断。
	if a.officialVideoDisabled {
		log.Printf("[network] Official UDP receiver disabled by RM_DISABLE_OFFICIAL_UDP=1")
	} else {
		udpRx, err := network.NewUDPReceiver(a.officialVideoPort, a.onH265FrameReady)
		if err != nil {
			log.Fatalf("Fatal: Cannot bind UDP %d: %v", a.officialVideoPort, err)
		}
		a.udpRx = udpRx
		a.udpRx.Start()
	}

	// 3. 启动周期性视频状态广播。
	go a.emitVideoStateLoop()

	// What: 额外起一条低频布局巡检循环。
	// Why: 多屏环境下 HUD 所在显示器可能在运行时发生变化；无论当前跑的是 official 还是 custom，只要独立视频窗不跟着重建，就会继续停在旧屏或旧偏移。
	go a.syncActiveVideoWindowLayoutLoop()
}

// domReady 在前端 DOM 与主窗口可用后启动原生视频层。
// What: 把 Wails 透明 HUD 的置顶、全屏和尺寸读取都收口到真正可获取窗口几何信息的阶段。
// Why: 只有先让 HUD 窗口稳定占住最上层，再启动 ffplay 无边框窗，才能避免"视频大屏在前、UI 被盖住"的层级反转。
func (a *App) domReady(ctx context.Context) {
	if a.ctx == nil && ctx != nil {
		a.ctx = ctx
	}

	a.setVideoDOMReady(true)
	a.ensureHUDWindowForeground()
	a.syncVideoPlayers()
}

// resolveVideoWindowLayout 推导原生视频窗应占据的屏幕区域。
// What: 直接从 X11 根窗口坐标系解析当前 HUD 主窗的绝对几何信息。
// Why: 多屏下 GTK/Wails 的窗口位置可能只是"相对当前屏"的局部坐标；只有绝对坐标才能让 ffplay 跟 HUD 稳定落在同一块显示区域。
func (a *App) resolveVideoWindowLayout() (video.VideoWindowLayout, error) {
	var lastErr error

	for attempt := 1; attempt <= videoWindowLayoutRetryCount; attempt++ {
		layout, err := video.ResolveX11WindowLayout(os.Getpid(), appWindowTitle)
		if err == nil {
			log.Printf(
				"[video] Use HUD absolute layout left=%d top=%d width=%d height=%d hud=0x%08x",
				layout.Left,
				layout.Top,
				layout.Width,
				layout.Height,
				layout.HUDWindowID,
			)
			return layout, nil
		}

		lastErr = err

		// What: 每次重试前都再强调一次 HUD 主窗的显示状态。
		// Why: DOM ready、切源和窗口管理器重排可能让 HUD 顶层窗暂时还没完全 map 到 client list，这里先把它催熟再去抓坐标更稳。
		a.ensureHUDWindowForeground()
		time.Sleep(videoWindowLayoutRetryInterval)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown layout resolution failure")
	}

	return video.VideoWindowLayout{}, fmt.Errorf("resolve HUD absolute layout failed: %w", lastErr)
}

// videoWindowLayoutsEqual 判断两份视频布局是否完全一致。
// What: 同时比较绝对坐标、逻辑尺寸和当前 HUD 顶层窗身份。
// Why: 多屏切换或窗口重映射时，即便尺寸与坐标都没变，只要 HUD 顶层窗 ID 变了，ffplay 仍可能压在旧窗下面，必须重建。
func videoWindowLayoutsEqual(left video.VideoWindowLayout, right video.VideoWindowLayout) bool {
	return left.Left == right.Left &&
		left.Top == right.Top &&
		left.Width == right.Width &&
		left.Height == right.Height &&
		left.HUDWindowID == right.HUDWindowID &&
		left.StackAboveWindowID == right.StackAboveWindowID
}

// syncActiveVideoWindowLayoutLoop 周期性收敛双视频层与 HUD 布局。
// What: 低频轮询 HUD 绝对布局，并同步 official/custom 两条常驻窗口的几何与堆叠关系。
// Why: 主画面与 PiP 都是独立原生窗；只校正活动源会让另一条链路在换屏或交换主次后继续停在错误位置。
func (a *App) syncActiveVideoWindowLayoutLoop() {
	if a.ctx == nil {
		return
	}

	ticker := time.NewTicker(videoWindowLayoutSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.syncVideoPlayers()
		}
	}
}

// simulateRobotStateLoop 模拟裁判系统数据变化并通知前端 Vue。
func (a *App) simulateRobotStateLoop() {
	ticker := time.NewTicker(500 * time.Millisecond) // 500ms 更新一次 UI
	defer ticker.Stop()

	// 初始化模拟状态。
	hp := 1200
	ammo := 100
	redBaseHP := 5000
	blueBaseHP := 5000

	robotIDs := []int{1, 2, 3, 4, 7}
	robotMaxHP := map[int]int{
		1: 350,
		2: 320,
		3: 300,
		4: 300,
		7: 600,
	}
	redUnitHP := map[int]int{
		1: 350,
		2: 320,
		3: 300,
		4: 300,
		7: 600,
	}
	blueUnitHP := map[int]int{
		1: 350,
		2: 320,
		3: 300,
		4: 300,
		7: 600,
	}

	resolveRole := func(robotID int) string {
		if robotID == 1 {
			return "hero"
		}
		if robotID == 2 {
			return "engineer"
		}
		if robotID == 3 || robotID == 4 {
			return "infantry"
		}
		if robotID == 7 {
			return "sentry"
		}
		return "unknown"
	}

	makeUnits := func(prefix string, hpMap map[int]int) []map[string]interface{} {
		units := make([]map[string]interface{}, 0, len(robotIDs))
		for _, robotID := range robotIDs {
			units = append(units, map[string]interface{}{
				"robot_id":   robotID,
				"role":       resolveRole(robotID),
				"hp":         hpMap[robotID],
				"max_hp":     robotMaxHP[robotID],
				"avatar_key": fmt.Sprintf("%s%d", prefix, robotID),
				"online":     true,
			})
		}
		return units
	}

	for {
		select {
		case <-a.ctx.Done():
			return // App 退出时结束循环
		case <-ticker.C:
			// 模拟受到攻击，血量随机减少 0-50。
			hp -= int(time.Now().UnixNano() % 50)
			if hp <= 0 {
				hp = 1200 // 模拟复活被重置装甲
			}

			// 模拟持续射击，弹药递减。
			ammo -= 1
			if ammo <= 0 {
				ammo = 100 // 模拟重新补充弹药
			}

			redBaseHP -= int(time.Now().UnixNano() % 20)
			blueBaseHP -= int(time.Now().UnixNano() % 17)
			if redBaseHP <= 0 {
				redBaseHP = 5000
			}
			if blueBaseHP <= 0 {
				blueBaseHP = 5000
			}

			for _, robotID := range robotIDs {
				redUnitHP[robotID] -= int((time.Now().UnixNano() + int64(robotID)) % 6)
				blueUnitHP[robotID] -= int((time.Now().UnixNano() + int64(robotID*3)) % 6)
				if redUnitHP[robotID] <= 0 {
					redUnitHP[robotID] = robotMaxHP[robotID]
				}
				if blueUnitHP[robotID] <= 0 {
					blueUnitHP[robotID] = robotMaxHP[robotID]
				}
			}

			// What: 通过 Wails 的事件广播机制，将状态以 JSON 对象形式推给前端 App.vue。
			// Why: 为本地演示补上一位可远程买弹标志，让控制链在无真实裁判系统时也能即时联调。
			runtime.EventsEmit(a.ctx, "robot-state", map[string]interface{}{
				"hp":              hp,
				"max_hp":          1200,
				"ammo":            ammo,
				"max_ammo":        100,
				"can_remote_ammo": ammo <= 80,
				"updated_at":      time.Now().UnixMilli(),
			})

			// What: 推送顶部红蓝基地与机器人编组数据。
			// Why: 让前端顶部组件直接使用统一比赛态数据源，避免继续写死占位值。
			runtime.EventsEmit(a.ctx, "match-state", map[string]interface{}{
				"red": map[string]interface{}{
					"base_hp":     redBaseHP,
					"base_max_hp": 5000,
					"units":       makeUnits("R", redUnitHP),
				},
				"blue": map[string]interface{}{
					"base_hp":     blueBaseHP,
					"base_max_hp": 5000,
					"units":       makeUnits("B", blueUnitHP),
				},
				"updated_at": time.Now().UnixMilli(),
			})
		}
	}
}

// shutdown 负责释放底层资源。
func (a *App) shutdown(ctx context.Context) {
	log.Println("[App] Shutting down...")

	// What: 先停 UDP 收包。
	// Why: 这样可以先阻断新帧进入，避免关闭视频层时仍有新数据并发灌入。
	if a.udpRx != nil {
		a.udpRx.Stop()
	}

	// What: 再停原生视频层。
	// Why: 确保 ffplay 子进程和 stdin 写协程在应用退出前被完整回收。
	a.stopAllVideoPlayers()

	// What: 最后关闭 MQTT。
	// Why: 控制链属于低频资源，放在最后收尾不会反向阻塞视频链退出。
	if a.mqtt != nil {
		a.mqtt.Close()
	}
}

// onH265FrameReady 在 UDP 组完一帧 H.265 数据后立刻触发。
func (a *App) onH265FrameReady(h265Frame []byte) {
	if len(h265Frame) == 0 {
		return
	}

	syncDecision := video.HEVCSyncDecision{Pass: true}
	if a.hevcSyncGate != nil {
		syncDecision = a.hevcSyncGate.Observe(h265Frame)
	}

	if !syncDecision.Pass {
		if syncDecision.DropCount <= 5 || syncDecision.DropCount%60 == 0 {
			log.Printf(
				"[official-video] drop pre-sync HEVC frame has_vps=%t has_sps=%t has_pps=%t has_irap=%t drop_count=%d",
				syncDecision.HasVPS,
				syncDecision.HasSPS,
				syncDecision.HasPPS,
				syncDecision.HasIRAP,
				syncDecision.DropCount,
			)
		}
		return
	}

	if syncDecision.Recoverable {
		a.setOfficialBootstrapFrame(h265Frame)
	}

	state := a.getVideoRuntimeState()
	if state.officialPlayer == nil {
		if syncDecision.Recoverable {
			a.syncVideoPlayers()
			state = a.getVideoRuntimeState()
		}
		if state.officialPlayer == nil {
			return
		}
	}

	if !state.officialBootstrapPending && syncDecision.Recoverable {
		a.markOfficialBootstrapPending()
		state = a.getVideoRuntimeState()
	}

	if state.officialPlayer == nil {
		return
	}

	bootstrapFrame, skipCurrentFrame := a.consumeOfficialBootstrapFrame(h265Frame)
	if len(bootstrapFrame) > 0 {
		state.officialPlayer.EnqueueFrame(bootstrapFrame)
		if skipCurrentFrame {
			return
		}
	}

	if !shouldEnqueueOfficialFrame(state, syncDecision) {
		return
	}

	// What: 组帧成功后直接交给原生播放器队列。
	state.officialPlayer.EnqueueFrame(h265Frame)
}

// emitVideoStateLoop 周期性向前端发送视频链路健康度与关键指标。
func (a *App) emitVideoStateLoop() {
	ticker := time.NewTicker(videoStateEmitInterval)
	defer ticker.Stop()

	a.emitVideoStateSnapshot()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.emitVideoStateSnapshot()
		}
	}
}

// emitVideoStateSnapshot 组装并广播一次当前视频状态。
// What: 将内部快照桥接为前端可直接消费的 snake_case payload。
// Why: 这样前端只需做类型适配和显示，不必知道后端如何组合 UDP 与本地播放器统计。
func (a *App) emitVideoStateSnapshot() {
	if a.ctx == nil {
		return
	}

	payload := a.buildVideoStatePayload()
	a.logVideoDiagnostics(payload)
	runtime.EventsEmit(a.ctx, "video-state", payload)
}

// buildVideoStatePayload 组合 UDP 接收器与原生视频层的状态。
// What: 将"UDP 是否活着""完整帧是否组出来""本地播放器是否还在呈现"拆开表达。
// Why: 这样前端和排障日志才能准确区分问题到底发生在协议层、组帧层还是显示层。
func (a *App) buildVideoStatePayload() videoStatePayload {
	officialVideoPort := a.officialVideoPort
	if officialVideoPort == 0 {
		officialVideoPort = defaultOfficialVideoPort
	}

	udpStats := network.UDPReceiverStats{}
	if a.udpRx != nil {
		udpStats = a.udpRx.StatsSnapshot()
	}

	state := a.getVideoRuntimeState()
	activeSource := normalizeVideoSource(state.activeVideoSource)
	lastCustomByteBlockAt := state.lastCustomByteBlockAt
	customReassemblyDrops := uint64(0)
	customDiagnostics := video.H264Diagnostics{}
	if a.h264Reassembler != nil {
		customReassemblyDrops = a.h264Reassembler.DropsOut()
		customDiagnostics = a.h264Reassembler.Diagnostics()
	}
	customBlockCount := atomic.LoadUint64(&a.customByteBlockCount)

	officialStats := video.PlayerSnapshot{}
	if state.officialPlayer != nil {
		officialStats = state.officialPlayer.Snapshot()
	}
	customStats := video.PlayerSnapshot{}
	if state.customPlayer != nil {
		customStats = state.customPlayer.Snapshot()
	}
	rateSample := a.updateVideoRateSample(udpStats, officialStats, customStats, customDiagnostics, customBlockCount)

	officialPlayerErr := state.officialPlayerErr
	if officialPlayerErr == "" && officialStats.WindowLayerErr != "" {
		officialPlayerErr = officialStats.WindowLayerErr
	}
	customPlayerErr := state.customPlayerErr
	if customPlayerErr == "" && customStats.WindowLayerErr != "" {
		customPlayerErr = customStats.WindowLayerErr
	}

	officialAvailable := isRecentTimestamp(udpStats.LastPacketAt, videoSourceActiveThreshold)
	customAvailable := isRecentTimestamp(lastCustomByteBlockAt, videoSourceActiveThreshold)
	if a.officialVideoDisabled {
		officialAvailable = false
	}
	frameReady := isRecentTimestamp(udpStats.LastFrameAt, videoSourceActiveThreshold)
	officialPresenting := state.officialPlayer != nil && isRecentTimestamp(officialStats.LastPresentAt, videoConnectedThreshold)
	customPresenting := state.customPlayer != nil && isRecentTimestamp(customStats.LastPresentAt, customVideoConnectedThreshold)
	officialVideoConnected := officialPresenting && officialStats.WindowLayerReady
	customVideoConnected := customPresenting && customStats.WindowLayerReady
	udpDropFrames := udpStats.IncompleteFrameDrops + udpStats.TimeoutFrameDrops + udpStats.InvalidFrameDrops

	officialLatencyMs := ageFromTimestamp(officialStats.LastPresentAt)
	if officialLatencyMs == 0 && frameReady {
		officialLatencyMs = ageFromTimestamp(udpStats.LastFrameAt)
	}
	if officialLatencyMs == 0 && officialAvailable {
		officialLatencyMs = ageFromTimestamp(udpStats.LastPacketAt)
	}
	customLatencyMs := ageFromTimestamp(customStats.LastPresentAt)
	if customLatencyMs == 0 {
		customLatencyMs = ageFromTimestamp(lastCustomByteBlockAt)
	}
	officialRuntimeErrVisible := officialPlayerErr != "" &&
		(udpStats.FrameCount == 0 || (state.officialSyncReady && !state.officialBootstrapPending))

	officialDisplayState := videoDisplayStateLive
	officialMessage := "原生低延迟视频链路正常"
	switch {
	case officialRuntimeErrVisible:
		officialDisplayState = videoDisplayStateRuntimeError
		officialMessage = officialPlayerErr
	case a.officialVideoDisabled:
		officialDisplayState = videoDisplayStateWaitingSource
		officialMessage = "官方 UDP 图传已在测试态禁用，当前仅验证 custom 0x0310 链路"
	case udpStats.PacketCount == 0:
		officialDisplayState = videoDisplayStateWaitingSource
		officialMessage = fmt.Sprintf("等待官方 H.265 视频源（本机 UDP :%d 尚未收到任何首包）", officialVideoPort)
	case !officialAvailable:
		officialDisplayState = videoDisplayStateStalled
		officialMessage = "官方 H.265 视频源已中断，最近未收到新的 UDP 视频包"
	case udpStats.FrameCount == 0:
		officialDisplayState = videoDisplayStateWaitingFrame
		officialMessage = "已收到 UDP 视频包，等待首个完整 HEVC 帧"
	case !state.officialSyncReady:
		officialDisplayState = videoDisplayStateWaitingFrame
		officialMessage = "已收到官方 HEVC 帧，等待参数帧/关键帧"
	case !frameReady:
		officialDisplayState = videoDisplayStateResyncing
		officialMessage = "UDP 仍在收包，但完整帧暂未稳定"
	case state.officialPlayer == nil:
		officialDisplayState = videoDisplayStateResyncing
		officialMessage = "官方视频等待原生播放器启动"
	case state.officialBootstrapPending:
		officialDisplayState = videoDisplayStateResyncing
		officialMessage = "官方视频等待参数帧/关键帧恢复解码"
	case officialPresenting && !officialStats.WindowLayerReady:
		officialDisplayState = videoDisplayStateResyncing
		officialMessage = "原生视频窗正在贴合 HUD"
	case !officialVideoConnected:
		officialDisplayState = videoDisplayStateResyncing
		officialMessage = "原生视频层等待关键帧或重同步"
	case udpDropFrames > 0 || officialStats.DecodeDropFrames > 0 || officialStats.CorruptFrames > 0:
		officialMessage = fmt.Sprintf(
			"低延迟链路工作中，UDP丢帧=%d 本地丢帧=%d 坏帧=%d",
			udpDropFrames,
			officialStats.DecodeDropFrames,
			officialStats.CorruptFrames,
		)
	}

	customDisplayState := videoDisplayStateWaitingSource
	customMessage := "等待 0x0310 自定义视频数据"
	switch {
	case customPlayerErr != "":
		customDisplayState = videoDisplayStateRuntimeError
		customMessage = customPlayerErr
	case lastCustomByteBlockAt == 0:
		customDisplayState = videoDisplayStateWaitingSource
		customMessage = "等待 0x0310 自定义视频数据"
	case !customAvailable:
		customDisplayState = videoDisplayStateStalled
		customMessage = "自定义 H.264 视频源已中断，最近未收到新的 0x0310 数据"
	case customPresenting && !customStats.WindowLayerReady:
		customDisplayState = videoDisplayStateWaitingFrame
		customMessage = "自定义视频窗正在贴合 HUD"
	case customVideoConnected:
		customDisplayState = videoDisplayStateLive
		customMessage = "自定义 H.264 视频链路正常"
	case customDiagnostics.NALTotal == 0 && customDiagnostics.NoStartCode > 0:
		customDisplayState = videoDisplayStateWaitingFrame
		customMessage = "已收到 0x0310，但暂未识别到 Annex-B H.264 起始码"
	case customDiagnostics.NALTotal > 0 && (customDiagnostics.SPS == 0 || customDiagnostics.PPS == 0 || customDiagnostics.IDR == 0):
		customDisplayState = videoDisplayStateWaitingFrame
		customMessage = fmt.Sprintf(
			"已收到 H.264 NAL，等待 SPS/PPS/IDR sps=%d pps=%d idr=%d",
			customDiagnostics.SPS,
			customDiagnostics.PPS,
			customDiagnostics.IDR,
		)
	case customDiagnostics.Synced:
		customDisplayState = videoDisplayStateResyncing
		customMessage = "自定义 H.264 链路在线，低码率关键帧重同步中"
	default:
		customDisplayState = videoDisplayStateResyncing
		customMessage = "已收到 0x0310 视频数据，等待关键帧恢复出画"
	}

	pipSource := otherVideoSource(activeSource)
	backendConnected := officialAvailable
	videoConnected := officialVideoConnected
	displayState := officialDisplayState
	decoderFPS := officialStats.DecoderFPS
	presentFPS := officialStats.PresentFPS
	latencyMs := officialLatencyMs
	decodeDropFrames := officialStats.DecodeDropFrames
	corruptFrames := officialStats.CorruptFrames
	decoderResets := officialStats.DecoderResets
	headerOrder := udpStats.HeaderOrder
	message := officialMessage
	currentUDPDropFrames := udpDropFrames

	if activeSource == videoSourceCustom {
		backendConnected = customAvailable
		videoConnected = customVideoConnected
		displayState = customDisplayState
		decoderFPS = customStats.DecoderFPS
		presentFPS = customStats.PresentFPS
		latencyMs = customLatencyMs
		decodeDropFrames = customStats.DecodeDropFrames + customReassemblyDrops
		corruptFrames = customStats.CorruptFrames
		decoderResets = customStats.DecoderResets
		headerOrder = "custom"
		message = customMessage
		currentUDPDropFrames = 0
	}

	return videoStatePayload{
		BackendConnected:       backendConnected,
		ControlLinkConnected:   a.mqtt != nil && a.mqtt.IsConnected(),
		VideoConnected:         videoConnected,
		ActiveSource:           activeSource,
		PIPSource:              pipSource,
		OfficialAvailable:      officialAvailable,
		CustomAvailable:        customAvailable,
		OfficialVideoConnected: officialVideoConnected,
		CustomVideoConnected:   customVideoConnected,
		DisplayState:           displayState,
		OfficialDisplayState:   officialDisplayState,
		CustomDisplayState:     customDisplayState,
		DecoderFPS:             decoderFPS,
		PresentFPS:             presentFPS,
		LatencyMs:              latencyMs,
		OfficialLatencyMs:      officialLatencyMs,
		CustomLatencyMs:        customLatencyMs,
		OfficialPacketRateHz:   rateSample.officialPacketHz,
		OfficialFrameRateHz:    rateSample.officialFrameHz,
		OfficialDropRate:       rateSample.officialDropRate,
		CustomBlockRateHz:      rateSample.customBlockHz,
		CustomAUFPS:            rateSample.customAUFPS,
		CustomDropRate:         rateSample.customDropRate,
		CustomCorruptRate:      rateSample.customCorruptRate,
		CustomH264NALTotal:     customDiagnostics.NALTotal,
		CustomH264IDR:          customDiagnostics.IDR,
		CustomH264SPS:          customDiagnostics.SPS,
		CustomH264PPS:          customDiagnostics.PPS,
		CustomH264NoStartCode:  customDiagnostics.NoStartCode,
		CustomH264Synced:       customDiagnostics.Synced,
		UDPDropFrames:          currentUDPDropFrames,
		DecodeDropFrames:       decodeDropFrames,
		CorruptFrames:          corruptFrames,
		DecoderResets:          decoderResets,
		HeaderOrder:            headerOrder,
		Message:                message,
		OfficialMessage:        officialMessage,
		CustomMessage:          customMessage,
		UpdatedAt:              time.Now().UnixMilli(),
	}
}

func (a *App) logVideoDiagnostics(payload videoStatePayload) {
	now := time.Now()

	a.videoRateMu.Lock()
	if !a.videoRateSample.lastDiagnosticLog.IsZero() && now.Sub(a.videoRateSample.lastDiagnosticLog) < time.Second {
		a.videoRateMu.Unlock()
		return
	}
	a.videoRateSample.lastDiagnosticLog = now
	a.videoRateMu.Unlock()

	log.Printf(
		"[video-diag] active=%s pip=%s present_fps=%.2f official(pkt=%.2fhz frame=%.2fhz drop=%.2f latency=%dms connected=%t) custom(block=%.2fhz au=%.2ffps drop=%.2f corrupt=%.2f latency=%dms synced=%t connected=%t)",
		payload.ActiveSource,
		payload.PIPSource,
		payload.PresentFPS,
		payload.OfficialPacketRateHz,
		payload.OfficialFrameRateHz,
		payload.OfficialDropRate,
		payload.OfficialLatencyMs,
		payload.OfficialVideoConnected,
		payload.CustomBlockRateHz,
		payload.CustomAUFPS,
		payload.CustomDropRate,
		payload.CustomCorruptRate,
		payload.CustomLatencyMs,
		payload.CustomH264Synced,
		payload.CustomVideoConnected,
	)
}

// isRecentTimestamp 判断某个毫秒时间戳是否仍在"活跃窗口"内。
// What: 把连接态判断统一收口到一个小函数。
// Why: 视频包、完整帧和本地呈现都要用同一套"最近是否活着"的标准，不能各自写一套时间比较。
func isRecentTimestamp(timestamp int64, threshold time.Duration) bool {
	if timestamp <= 0 {
		return false
	}

	return time.Since(time.UnixMilli(timestamp)) <= threshold
}

// ageFromTimestamp 计算距离最近一次活动的年龄。
// What: 将时间戳统一转换为前端可直接显示的毫秒值。
// Why: 前端连接条只关心当前链路"多久没动了"，不需要重复参与 Go 侧时间换算。
func ageFromTimestamp(timestamp int64) int64 {
	if timestamp <= 0 {
		return 0
	}

	age := time.Since(time.UnixMilli(timestamp)).Milliseconds()
	if age < 0 {
		return 0
	}
	return age
}

// onGlobalUnitStatus 处理基地与全队血量同步。
// What: 先更新全局缓存，再把它翻译成前端固定的 red/blue match-state。
// Why: 顶部基地血条和双方编组都只认 match-state，不能把协议里的 allied/enemy 直接暴露给前端。
func (a *App) onGlobalUnitStatus(status *rmcp.GlobalUnitStatus) {
	if status == nil || a.ctx == nil {
		return
	}

	a.cacheGlobalUnitStatus(status)

	matchStatePayload, ok := a.buildMatchStatePayload()
	if !ok {
		return
	}

	runtime.EventsEmit(a.ctx, "match-state", matchStatePayload)
}

// onGameStatus 处理比赛全局元信息。
// What: 先缓存官方 GameStatus，再桥接到统一 match-state。
// Why: 顶部中间条要与官方字段 1:1 对齐，不能继续依赖前端自造局次、比分和倒计时。
func (a *App) onGameStatus(status *rmcp.GameStatus) {
	if status == nil || a.ctx == nil {
		return
	}

	a.cacheGameStatus(status)

	matchStatePayload, ok := a.buildMatchStatePayload()
	if !ok {
		return
	}

	runtime.EventsEmit(a.ctx, "match-state", matchStatePayload)
}

func (a *App) onRobotDynamicStatus(status *rmcp.RobotDynamicStatus) {
	if status == nil || a.ctx == nil {
		return
	}

	a.cacheRobotDynamicStatus(status)

	// What: 将 RobotDynamicStatus 映射到前端统一战斗态事件。
	// Why: 前端已经围绕 combat-state 做归一化，继续复用这条链路可避免再造第二套 store 入口。
	combatStatePayload, ok := a.buildCombatStatePayload()
	if ok {
		runtime.EventsEmit(a.ctx, "combat-state", combatStatePayload)
	}

	// What: 同步刷新顶部编组里的本机槽位血量。
	// Why: GlobalUnitStatus 仅 1Hz，而本机血量是 10Hz；若这里不顺手刷新，顶部本机会明显落后一拍。
	matchStatePayload, ok := a.buildMatchStatePayload()
	if ok {
		runtime.EventsEmit(a.ctx, "match-state", matchStatePayload)
	}
}

// onRobotStaticStatus 处理机器人固定属性与配置。
// What: 缓存 max_health/连接状态后，按需刷新 combat-state 与 match-state。
// Why: 血量上限属于低频静态数据，但一旦到达就应立刻修正 UI，不必再等下一帧别的 topic。
func (a *App) onRobotStaticStatus(status *rmcp.RobotStaticStatus) {
	if status == nil || a.ctx == nil {
		return
	}

	a.cacheRobotStaticStatus(status)

	absoluteRobotID := status.GetRobotId()
	if absoluteRobotID == 0 {
		// What: 若静态状态包缺失 robot_id，则按本机绝对机器人 ID 兜底。
		// Why: 至少保证本机血量上限一旦被收到，就能立刻刷新本机状态卡。
		absoluteRobotID = uint32(a.mqttClientID)
	}

	payload := map[string]interface{}{
		"connection_state":           status.GetConnectionState(),
		"field_state":                status.GetFieldState(),
		"alive_state":                status.GetAliveState(),
		"robot_id":                   absoluteRobotID,
		"robot_type":                 status.GetRobotType(),
		"performance_system_shooter": status.GetPerformanceSystemShooter(),
		"performance_system_chassis": status.GetPerformanceSystemChassis(),
		"level":                      status.GetLevel(),
		"max_health":                 status.GetMaxHealth(),
		"max_heat":                   status.GetMaxHeat(),
		"heat_cooldown_rate":         status.GetHeatCooldownRate(),
		"max_power":                  status.GetMaxPower(),
		"max_buffer_energy":          status.GetMaxBufferEnergy(),
		"max_chassis_energy":         status.GetMaxChassisEnergy(),
		"self":                       absoluteRobotID == uint32(a.mqttClientID),
		"updated_at":                 time.Now().UnixMilli(),
	}
	runtime.EventsEmit(a.ctx, "robot-static-status", payload)

	if absoluteRobotID == uint32(a.mqttClientID) {
		combatStatePayload, ok := a.buildCombatStatePayload()
		if ok {
			runtime.EventsEmit(a.ctx, "combat-state", combatStatePayload)
		}
	}

	matchStatePayload, ok := a.buildMatchStatePayload()
	if ok {
		runtime.EventsEmit(a.ctx, "match-state", matchStatePayload)
	}
}

func (a *App) onGlobalLogisticsStatus(status *rmcp.GlobalLogisticsStatus) {
	if status == nil || a.ctx == nil {
		return
	}

	runtime.EventsEmit(a.ctx, "global-logistics-status", map[string]interface{}{
		"remaining_economy":      status.GetRemainingEconomy(),
		"total_economy_obtained": status.GetTotalEconomyObtained(),
		"tech_level":             status.GetTechLevel(),
		"encryption_level":       status.GetEncryptionLevel(),
		"updated_at":             time.Now().UnixMilli(),
	})
}

func (a *App) onGlobalSpecialMechanism(status *rmcp.GlobalSpecialMechanism) {
	if status == nil || a.ctx == nil {
		return
	}

	mechanisms := make([]map[string]interface{}, 0, len(status.GetMechanismId()))
	for index, mechanismID := range status.GetMechanismId() {
		timeSec := int32(0)
		if index < len(status.GetMechanismTimeSec()) {
			timeSec = status.GetMechanismTimeSec()[index]
		}
		mechanisms = append(mechanisms, map[string]interface{}{
			"mechanism_id": mechanismID,
			"time_sec":     timeSec,
		})
	}

	runtime.EventsEmit(a.ctx, "global-special-mechanism", map[string]interface{}{
		"mechanisms": mechanisms,
		"updated_at": time.Now().UnixMilli(),
	})
}

func (a *App) onRefereeEvent(event *rmcp.Event) {
	if event == nil || a.ctx == nil {
		return
	}

	now := time.Now().UnixMilli()
	eventID := event.GetEventId()
	param := event.GetParam()
	runtime.EventsEmit(a.ctx, "referee-event", map[string]interface{}{
		"event_id":   eventID,
		"param":      param,
		"updated_at": now,
	})

	zone := strings.ToLower(strings.TrimSpace(param))
	switch zone {
	case "front", "back", "left", "right":
		runtime.EventsEmit(a.ctx, "armor-hit", map[string]interface{}{
			"event_id":  eventID,
			"zone":      zone,
			"param":     param,
			"timestamp": now,
		})
	}
}

func (a *App) onRobotModuleStatus(status *rmcp.RobotModuleStatus) {
	if status == nil || a.ctx == nil {
		return
	}

	runtime.EventsEmit(a.ctx, "robot-module-status", map[string]interface{}{
		"power_manager":          status.GetPowerManager(),
		"rfid":                   status.GetRfid(),
		"light_strip":            status.GetLightStrip(),
		"small_shooter":          status.GetSmallShooter(),
		"big_shooter":            status.GetBigShooter(),
		"uwb":                    status.GetUwb(),
		"armor":                  status.GetArmor(),
		"video_transmission":     status.GetVideoTransmission(),
		"capacitor":              status.GetCapacitor(),
		"main_controller":        status.GetMainController(),
		"laser_detection_module": status.GetLaserDetectionModule(),
		"updated_at":             time.Now().UnixMilli(),
	})
}

func (a *App) onRobotPosition(status *rmcp.RobotPosition) {
	if status == nil || a.ctx == nil {
		return
	}

	runtime.EventsEmit(a.ctx, "robot-position", map[string]interface{}{
		"x":          status.GetX(),
		"y":          status.GetY(),
		"z":          status.GetZ(),
		"yaw":        status.GetYaw(),
		"updated_at": time.Now().UnixMilli(),
	})
}

func (a *App) onRadarInfo(status *rmcp.RadarInfoToClient) {
	if status == nil || a.ctx == nil {
		return
	}

	runtime.EventsEmit(a.ctx, "radar-info", map[string]interface{}{
		"target_robot_id": status.GetTargetRobotId(),
		"target_pos_x":    status.GetTargetPosX(),
		"target_pos_y":    status.GetTargetPosY(),
		"torward_angle":   status.GetTorwardAngle(),
		"is_high_light":   status.GetIsHighLight(),
		"updated_at":      time.Now().UnixMilli(),
	})
}

func (a *App) onBuff(status *rmcp.Buff) {
	if status == nil || a.ctx == nil {
		return
	}

	runtime.EventsEmit(a.ctx, "buff-state", map[string]interface{}{
		"robot_id":       status.GetRobotId(),
		"buff_type":      status.GetBuffType(),
		"buff_level":     status.GetBuffLevel(),
		"buff_max_time":  status.GetBuffMaxTime(),
		"buff_left_time": status.GetBuffLeftTime(),
		"updated_at":     time.Now().UnixMilli(),
	})
}

func (a *App) onDeployModeStatus(status *rmcp.DeployModeStatusSync) {
	if status == nil || a.ctx == nil {
		return
	}

	runtime.EventsEmit(a.ctx, "deploy-mode-status", map[string]interface{}{
		"status":     status.GetStatus(),
		"updated_at": time.Now().UnixMilli(),
	})
}

// onCustomByteBlock 处理机器人 0x0310 对应的自定义视频字节块。
// What: 将连续到达的 300B 原始 H.264 字节块重组成完整 AU，并按顺序送入 custom 原生播放器。
// Why: 官方 0x0310 只保证 300B 负载转发，不保留帧边界；若这里不做访问单元级重组，ffplay 只能持续收到坏 slice。
func (a *App) onCustomByteBlock(block *rmcp.CustomByteBlock) {
	if block == nil || len(block.GetData()) == 0 {
		return
	}

	now := time.Now().UnixMilli()
	previousBlockAt := a.noteCustomByteBlockReceipt(now)
	blockData := block.GetData()
	blockCount := atomic.AddUint64(&a.customByteBlockCount, 1)
	if blockCount <= 5 || blockCount%100 == 0 {
		log.Printf("[custom-video] CustomByteBlock received count=%d bytes=%d", blockCount, len(blockData))
	}

	if a.h264Reassembler == nil {
		return
	}

	state := a.getVideoRuntimeState()
	if state.customPlayer == nil {
		a.syncVideoPlayers()
		state = a.getVideoRuntimeState()
	}
	if a.consumeCustomStreamNeedsResync() {
		log.Printf("[custom-video] resync H.264 reassembler because custom ffplay restarted; preserve cached SPS/PPS and wait for next IDR")
		a.h264Reassembler.ResyncPreservingParameterSets()
	}
	if previousBlockAt > 0 && time.Duration(now-previousBlockAt)*time.Millisecond > customStreamGapResyncThreshold {
		log.Printf("[custom-video] reset H.264 reassembler after %.0fms input gap; wait for next SPS/PPS+IDR",
			float64(now-previousBlockAt))
		a.h264Reassembler.Reset()
	}

	// What: 每次收到 0x0310 块后都持续排空当前已经完成的 AU。
	// Why: 单个 300B block 可能一次跨过多个 NAL/AU 边界；若不在同一回调里连续取空，后续完整帧会被无意义地卡到下一个网络包才送出。
	decodedFrames := 0
	droppedBootstrapBurstFrames := 0
	customWasSynced := a.h264Reassembler.Diagnostics().Synced
	for frame := a.h264Reassembler.Feed(blockData); frame != nil; frame = a.h264Reassembler.Feed(nil) {
		if !customWasSynced && decodedFrames > 0 {
			droppedBootstrapBurstFrames++
			continue
		}
		if state.customPlayer != nil {
			state.customPlayer.EnqueueFrame(frame)
		}
		decodedFrames++
	}
	if droppedBootstrapBurstFrames > 0 {
		log.Printf("[custom-video] drop %d post-bootstrap AU(s) from same block; preserve first recoverable IDR", droppedBootstrapBurstFrames)
	}

	// What: 只在首批成功出画和稳定运行阶段降频打印 custom 源解码日志。
	// Why: 用户当前没有整条实物链路，这里必须提供"客户端已经真的从 0x0310 恢复出可解码 AU"的直接证据，同时避免每帧刷屏。
	if decodedFrames > 0 {
		totalFramesOut := a.h264Reassembler.FramesOut()
		if totalFramesOut <= 3 || (totalFramesOut%30) == 0 {
			log.Printf(
				"[custom-video] 0x0310 block accepted bytes=%d decoded_au=%d total_au=%d drop_au=%d",
				len(blockData),
				decodedFrames,
				totalFramesOut,
				a.h264Reassembler.DropsOut(),
			)
		}
	} else if blockCount <= 5 || blockCount%100 == 0 {
		diag := a.h264Reassembler.Diagnostics()
		log.Printf(
			"[custom-video] waiting H.264 sync blocks=%d nal=%d sps=%d pps=%d idr=%d no_start=%d synced=%t drop_au=%d",
			blockCount,
			diag.NALTotal,
			diag.SPS,
			diag.PPS,
			diag.IDR,
			diag.NoStartCode,
			diag.Synced,
			diag.DropsOut,
		)
	}

	// What: 若当前就停在 custom 源，则收到首包后立刻推一次状态快照。
	// Why: 这样前端无论把 custom 放在主画面还是右上角 PiP，都能立刻把占位态切到"已收到数据/正在出画"。
	if a.ctx != nil {
		a.emitVideoStateSnapshot()
	}
}

// SetActiveVideoSource 切换当前激活的视频源。
// What: 对外暴露 official/custom 二选一切换入口，并在需要时切换对应的原生视频层。
// Why: 这次精简的核心就是把客户端收敛到双源视频模型，切源必须真正落到后端而不能只停留在前端配置字段。
func (a *App) SetActiveVideoSource(source string) error {
	normalizedSource := normalizeVideoSource(source)
	if normalizedSource == "" {
		return fmt.Errorf("unsupported video source: %s", source)
	}
	if normalizedSource == videoSourceCustom && !a.allowsCustomVideoSource() {
		normalizedSource = videoSourceOfficial
	}

	state := a.getVideoRuntimeState()
	if state.activeVideoSource == normalizedSource {
		a.syncVideoPlayers()
		a.emitVideoStateSnapshot()
		return nil
	}

	// What: 这里只切换"谁是主画面"，播放器生命周期交给 syncVideoPlayers 统一收敛。
	// Why: official 主画面必须保持最低延迟，切回官方时要允许后端停掉 custom ffplay，避免备用链路继续抢解码和窗口管理资源。
	log.Printf("[video] switch active video source %s -> %s", state.activeVideoSource, normalizedSource)
	a.setActiveVideoSource(normalizedSource)
	a.syncVideoPlayers()
	a.emitVideoStateSnapshot()
	return nil
}

// SetMQTTRobotIdentity 切换当前客户端使用的机器人 MQTT 身份。
// What: 对外暴露给前端设置面板的运行时切换入口。
// Why: 用户已经明确要求进入客户端后可选择其他机器人，选择后不能只改本地配置文件，必须立即作用到后端 MQTT 身份。
func (a *App) SetMQTTRobotIdentity(identity string) error {
	normalizedIdentity := normalizeMQTTRobotIdentity(identity)
	if normalizedIdentity == "" {
		return fmt.Errorf("unsupported mqtt robot identity: %s", identity)
	}

	nextMQTTClientID := mqttRobotIdentityToClientID(normalizedIdentity)
	if nextMQTTClientID == 0 {
		return fmt.Errorf("invalid mqtt robot identity mapping: %s", normalizedIdentity)
	}

	if a.mqttClientID == nextMQTTClientID && a.mqtt != nil {
		if !isHeroMQTTClientID(nextMQTTClientID) {
			a.setActiveVideoSource(videoSourceOfficial)
			a.syncVideoPlayers()
			a.emitVideoStateSnapshot()
		}
		return nil
	}

	// What: 先切换本机身份与缓存，再重连 MQTT。
	// Why: 只有这样后续收到的 RobotDynamicStatus / RobotStaticStatus 才会按新的机器人身份解释，不会继续沿用旧阵营与旧本机机器人编号。
	a.configureRMIdentity(nextMQTTClientID)
	a.resetIdentityScopedRMState()

	if err := a.bindMQTTClient(nextMQTTClientID); err != nil {
		return err
	}
	if !isHeroMQTTClientID(nextMQTTClientID) {
		a.setActiveVideoSource(videoSourceOfficial)
		a.syncVideoPlayers()
	}

	// What: 身份切换后立即推一次视频/链路状态快照。
	// Why: 这样设置面板和状态卡能立刻看到控制链是否恢复，不需要再额外等下一轮心跳。
	a.emitVideoStateSnapshot()
	return nil
}

// SetMQTTHeroIdentity 保留旧 Wails 绑定入口，内部委托到机器人身份切换。
func (a *App) SetMQTTHeroIdentity(identity string) error {
	return a.SetMQTTRobotIdentity(identity)
}

// currentMQTTRobotIdentity 返回当前运行时实际绑定的 MQTT 机器人身份。
// What: 将当前生效的 mqttClientID 还原成前端设置面板使用的机器人身份枚举。
// Why: 历史配置文件可能还没有 mqttHeroIdentity 字段；前端启动时若拿不到运行时真实身份，就会回退成默认蓝方并把刚探测成功的红方连接再次覆盖掉。
func (a *App) currentMQTTRobotIdentity() string {
	a.rmStateMu.RLock()
	currentClientID := a.mqttClientID
	a.rmStateMu.RUnlock()

	return normalizeMQTTRobotIdentity(resolveMQTTHeroIdentityFromClientID(currentClientID))
}

func (a *App) currentMQTTHeroIdentity() string {
	return a.currentMQTTRobotIdentity()
}

// resolveMQTTHeroIdentityFromClientID 将 MQTT clientID 反向映射为身份标签。
// What: 支持所有已知机器人 ID 映射；未匹配时返回空字符串。
// Why: 前端设置页需要从运行时真实 clientID 还原用户可选身份，不能再只返回红/蓝英雄两个值。
func resolveMQTTHeroIdentityFromClientID(clientID uint16) string {
	return robotIDToIdentityLabel(clientID)
}

// injectRuntimeMQTTRobotIdentityIntoConfig 在返回前端的配置 JSON 中补齐运行时 MQTT 身份。
// What: 当持久化配置还没升级出 mqttRobotIdentity 字段时，把当前运行时真实身份补进返回给前端的 JSON。
// Why: 前端 hydrate 会把后端返回配置再次同步回运行时；若这里不补齐，旧配置用户会在启动后立刻把自动探测出的红方身份误覆盖回蓝方。
func injectRuntimeMQTTRobotIdentityIntoConfig(rawConfigJSON string, runtimeIdentity string) (string, error) {
	normalizedRuntimeIdentity := normalizeMQTTRobotIdentity(runtimeIdentity)
	if normalizedRuntimeIdentity == "" {
		return rawConfigJSON, nil
	}

	var decoded any

	// What: 先把原始 JSON 解成通用对象树。
	// Why: 当前只需要最小补丁一个字段，没必要在后端复制整份前端配置结构定义。
	if err := json.Unmarshal([]byte(rawConfigJSON), &decoded); err != nil {
		return "", err
	}

	rootObject, ok := decoded.(map[string]any)
	if !ok || rootObject == nil {
		// What: 若历史配置根节点不是对象，则直接回退到新的最小对象。
		// Why: GetClientConfig 只服务前端恢复配置；比起把异常结构继续透传，让前端拿到可用对象更安全。
		rootObject = make(map[string]any)
	}

	uiPanelObject, ok := rootObject["uiPanel"].(map[string]any)
	if !ok || uiPanelObject == nil {
		// What: 缺少 uiPanel 节点时现场补一层对象。
		// Why: mqttRobotIdentity 就挂在 uiPanel 下，不能要求旧配置文件必须已经带这个层级。
		uiPanelObject = make(map[string]any)
		rootObject["uiPanel"] = uiPanelObject
	}

	configuredRobotIdentity, _ := uiPanelObject["mqttRobotIdentity"].(string)
	normalizedConfiguredRobotIdentity := normalizeMQTTRobotIdentity(configuredRobotIdentity)
	if normalizedConfiguredRobotIdentity != "" {
		// What: 只在缺字段或字段非法时才注入运行时值。
		// Why: 用户一旦在设置面板里做过显式选择，就必须优先尊重配置，而不是被当前运行时临时值反向覆盖。
		uiPanelObject["mqttRobotIdentity"] = normalizedConfiguredRobotIdentity
		uiPanelObject["mqttHeroIdentity"] = normalizedConfiguredRobotIdentity
	} else {
		configuredHeroIdentity, _ := uiPanelObject["mqttHeroIdentity"].(string)
		normalizedConfiguredHeroIdentity := normalizeMQTTRobotIdentity(configuredHeroIdentity)
		if normalizedConfiguredHeroIdentity != "" {
			uiPanelObject["mqttRobotIdentity"] = normalizedConfiguredHeroIdentity
			uiPanelObject["mqttHeroIdentity"] = normalizedConfiguredHeroIdentity
		} else {
			uiPanelObject["mqttRobotIdentity"] = normalizedRuntimeIdentity
			uiPanelObject["mqttHeroIdentity"] = normalizedRuntimeIdentity
		}
	}

	patchedJSON, err := json.Marshal(rootObject)
	if err != nil {
		return "", err
	}

	return string(patchedJSON), nil
}

func injectRuntimeMQTTHeroIdentityIntoConfig(rawConfigJSON string, runtimeIdentity string) (string, error) {
	return injectRuntimeMQTTRobotIdentityIntoConfig(rawConfigJSON, runtimeIdentity)
}

// SaveClientConfig 保存前端 UI/控制配置。
// What: 暴露给 Wails 前端的配置落盘接口。
// Why: 设置页"保存并应用"必须具备后端确认链路，避免仅前端内存生效导致误判。
func (a *App) SaveClientConfig(configJSON string) error {
	return config.SaveClientConfigJSON(configJSON)
}

// GetClientConfig 获取已保存的 UI/控制配置。
// What: 暴露给前端启动期读取配置的接口，并在必要时补齐运行时 MQTT 身份。
// Why: 旧配置文件没有 mqttHeroIdentity 字段时，前端若只读到默认值，就会把启动期已经探测成功的真实英雄身份再次覆盖掉。
func (a *App) GetClientConfig() (string, error) {
	rawConfigJSON, err := config.LoadClientConfigJSON()
	if err != nil {
		return "", err
	}

	patchedConfigJSON, err := injectRuntimeMQTTRobotIdentityIntoConfig(rawConfigJSON, a.currentMQTTRobotIdentity())
	if err != nil {
		return "", err
	}

	return patchedConfigJSON, nil
}

// QuitApp 请求当前 Wails 应用退出。
// What: 暴露给前端退出确认弹层的最小退出接口。
// Why: 退出动作必须由后端持有的 runtime context 触发，前端自己无法安全关闭原生 HUD 窗口与后台服务。
func (a *App) QuitApp() error {
	if a.ctx == nil {
		// What: 在 runtime context 尚未可用时直接返回错误。
		// Why: 这通常意味着前端跑在纯开发调试环境里；比起静默失败，明确告诉前端"当前无法退出"更容易排障。
		return fmt.Errorf("runtime context not ready")
	}

	// What: 通过 Wails runtime 发起正式退出。
	// Why: 这样可以确保 shutdown 钩子、原生视频层回收与 MQTT/UDP 资源释放都沿既有生命周期执行。
	runtime.Quit(a.ctx)
	return nil
}
