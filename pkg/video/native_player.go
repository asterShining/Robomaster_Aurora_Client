package video

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// What: 本地原生视频层只保留极小缓冲。
	// Why: 用户目标是“新鲜度优先”，因此这里刻意不做长队列，宁可主动丢旧帧也不允许画面排队累积。
	nativeFrameQueueSize = 2

	// What: official/custom 两条视频链使用不同的 ffplay 原生窗口标题。
	// Why: 双视频常驻后，X11/XWayland 选窗必须能稳定区分主画面与右上角 PiP，不能再让两条链路共享同一个标题。
	nativeOfficialVideoWindowTitle = "RoboMaster Native Video Layer"
	nativeCustomVideoWindowTitle   = "RoboMaster Native Video Layer Custom"

	// What: 单帧在 Go 侧本地队列里的最大允许停留时间。
	// Why: 这里不能再像上一版那样压得过低，否则 ffplay 首次起流或系统偶发调度抖动时会把本来还能显示的帧误判成“过时帧”，直接造成首画面更慢和持续掉帧。
	maxQueuedFrameAge = 120 * time.Millisecond

	// What: 单次向 ffplay stdin 写一帧的慢阻塞告警阈值。
	// Why: 现场实机上 pipe 写入偶发阻塞并不等价于播放器已经彻底失效；这一版先只记录慢写现象而不再立刻重启，避免 ffplay 被自杀式拉起又打死。
	slowWriteWarnDuration = 250 * time.Millisecond

	// What: 原生窗口层级收敛时的重试次数。
	// Why: ffplay 子窗口在不同桌面环境里的真正 map 时机并不稳定，必须给窗口管理器一点时间完成注册，否则第一次抓窗大概率扑空。
	windowLayerSettleRetryCount = 24

	// What: 每次等待原生窗口出现在窗口管理器中的轮询间隔。
	// Why: 这里不能打得太密，否则会平白增加 X11 请求噪声；也不能太稀，否则 HUD 和视频层在启动阶段会明显错位更久。
	windowLayerSettleRetryInterval = 120 * time.Millisecond

	// What: 当前 ffplay 进程已经开始稳定出帧后，窗口层仍允许继续软等待的最长期限。
	// Why: Wayland/XWayland 下 ffplay 往往要等到首帧附近才真正映射顶层窗；这里必须给出缓冲，避免把迟到建窗误判成 runtime_error。
	windowLayerErrorPromotionAfterPresent = 2 * time.Second

	// What: 判定“当前进程仍在连续呈现”的最近成功写帧新鲜度阈值。
	// Why: 只有在视频还在持续流向 ffplay 时，贴窗失败才值得升级成硬错误；若播放器早已停帧，就不该继续把锅甩给窗口层。
	windowLayerPresentFreshnessThreshold = 800 * time.Millisecond
)

type queuedFrame struct {
	data       []byte
	enqueuedAt time.Time
}

// VideoWindowLayout 描述原生视频窗应占据的屏幕区域。
// What: 将左上角位置、窗口尺寸和当前 HUD 顶层窗 ID 一起作为播放器布局快照传递。
// Why: ffplay 现在不再使用真正的 fullscreen 层级，而是用无边框普通窗铺满 HUD 所在区域；若没有 HUD 窗身份，后续 sibling 压层与重映射比较都无法稳定工作。
type VideoWindowLayout struct {
	Left        int
	Top         int
	Width       int
	Height      int
	HUDWindowID uint32
	// What: 指定当前视频窗在堆叠顺序里应直接位于哪个窗口下方。
	// Why: 双视频常驻后必须固定成 `HUD > PiP > Main`；仅知道 HUDWindowID 已不足以表达主窗需要压在 PiP 下方这一层级关系。
	StackAboveWindowID uint32
}

// PlayerSnapshot 是前端状态栏和上层调度用到的只读统计快照。
// What: 统一收口本地播放器运行态与窗口层贴合状态。
// Why: App 层需要周期性把这些指标桥接到前端，但不应该直接窥探播放器内部锁和进程句柄。
type PlayerSnapshot struct {
	DecoderFPS       float64
	PresentFPS       float64
	DecodeDropFrames uint64
	CorruptFrames    uint64
	DecoderResets    uint64
	LastInputAt      int64
	LastPresentAt    int64
	WindowLayerReady bool
	WindowLayerErr   string
	WindowID         uint32
}

type ffplayLaunchMode struct {
	label    string
	hwaccel  string
	software bool
}

type ffplayLaunchAttempt struct {
	mode ffplayLaunchMode
	args []string
	env  []string
}

type ffplayLaunchResult struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr io.ReadCloser
	mode   ffplayLaunchMode
}

// NativePlayer 负责托管 ffplay 子进程，并把完整 HEVC 帧通过 stdin 输送给原生视频层。
// Why: 让解码、显示和 HUD 彻底解耦，优先保证低延迟和高帧率。
type NativePlayer struct {
	ffplayPath string

	// What: 输入格式字符串，传递给 ffplay 的 -f 参数。
	// Why: 官方源是 HEVC（hevc），自定义源现在重新走 H.264（h264），必须参数化而不是硬编码。
	inputFormat string
	windowTitle string
	sourceName  string

	framesCh chan queuedFrame
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once

	processMu         sync.Mutex
	cmd               *exec.Cmd
	stdin             io.WriteCloser
	layout            VideoWindowLayout
	windowLayerPolicy nativeWindowLayerPolicy
	windowSearchMode  nativeVideoWindowSearchMode
	onUnexpectedExit  func()

	statsMu            sync.Mutex
	decoderWindowStart time.Time
	decoderWindowCount uint64
	decoderFPS         float64
	presentWindowStart time.Time
	presentWindowCount uint64
	presentFPS         float64
	lastInputAt        int64
	lastPresentAt      int64
	firstPresentAt     int64
	windowLayerReady   bool
	windowLayerErr     string
	windowID           uint32
	decoderMode        string

	decodeDropFrames uint64
	corruptFrames    uint64
	decoderResets    uint64
}

// NewNativePlayer 创建原生视频层托管器（默认 HEVC 格式，用于官方源）。
// What: 在启动时显式探测 ffplay 可执行文件。
// Why: 一旦比赛机缺少 ffplay，问题应在启动阶段直接暴露，而不是等收到首帧后才发现整条链路不可用。
func NewNativePlayer(hudWindowTitle string) (*NativePlayer, error) {
	return NewOfficialNativePlayer(hudWindowTitle)
}

// NewNativePlayerWithFormat 创建指定输入格式的原生视频层托管器。
// What: 允许调用方显式指定 ffplay -f 参数（如 "hevc" 或 "h264"）。
// Why: 官方源走 H.265，自定义源走原始 H.264 字节流，两条链路用同一套 ffplay 托管器但格式不同。
func NewNativePlayerWithFormat(_ string, inputFormat string) (*NativePlayer, error) {
	if strings.EqualFold(strings.TrimSpace(inputFormat), "h264") {
		return NewCustomNativePlayer("")
	}
	return NewOfficialNativePlayer("")
}

// NewOfficialNativePlayer 创建官方视频源专用的原生播放器。
// What: 固定 official 的输入格式、窗口标题与搜窗策略。
// Why: 主画面与 PiP 并存后，官方源必须有自己独立的窗口身份，不能再与 custom 共享同一套发现规则。
func NewOfficialNativePlayer(_ string) (*NativePlayer, error) {
	return newNativePlayer("official", "hevc", nativeOfficialVideoWindowTitle, nativeVideoWindowSearchModeOfficial)
}

// NewCustomNativePlayer 创建自定义视频源专用的原生播放器。
// What: 固定 custom 的输入格式、窗口标题与搜窗策略。
// Why: 自定义 0x0310 H.264 链路需要与官方源完全隔离，避免 PiP 选窗和写流互相串扰。
func NewCustomNativePlayer(_ string) (*NativePlayer, error) {
	return newNativePlayer("custom", "h264", nativeCustomVideoWindowTitle, nativeVideoWindowSearchModeCustom)
}

func newNativePlayer(sourceName string, inputFormat string, windowTitle string, searchMode nativeVideoWindowSearchMode) (*NativePlayer, error) {
	ffplayPath, err := exec.LookPath("ffplay")
	if err != nil {
		return nil, err
	}

	if inputFormat == "" {
		inputFormat = "hevc"
	}

	return &NativePlayer{
		ffplayPath:        ffplayPath,
		inputFormat:       inputFormat,
		windowTitle:       windowTitle,
		sourceName:        sourceName,
		framesCh:          make(chan queuedFrame, nativeFrameQueueSize),
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
		windowLayerPolicy: detectNativeWindowLayerPolicy(),
		windowSearchMode:  searchMode,
	}, nil
}

// SetUnexpectedExitHook 注册一个在 ffplay 意外退出后触发的回调。
// What: 允许上层在播放器进程自退时同步重置与该流绑定的状态机。
// Why: 对 raw H.264 这类需要等待关键帧重同步的链路来说，显示进程一旦重启，继续沿用旧同步态只会不断把中途 P 帧灌进新进程。
func (p *NativePlayer) SetUnexpectedExitHook(hook func()) {
	p.processMu.Lock()
	p.onUnexpectedExit = hook
	p.processMu.Unlock()
}

// Start 启动受控 ffplay 进程与写入循环。
// What: 在起播放器前先锁定视频窗几何参数。
// Why: 只有让 ffplay 和透明 HUD 共用同一块屏幕区域，原生视频层才会稳定待在 HUD 后面而不是抢走最上层。
func (p *NativePlayer) Start(layout VideoWindowLayout) error {
	if layout.Width <= 0 || layout.Height <= 0 {
		return fmt.Errorf("invalid video window layout width=%d height=%d", layout.Width, layout.Height)
	}

	// What: 在启动前先把本机桌面环境与播放器窗口策略记进日志。
	// Why: 用户现在同时踩到 Wayland/XWayland 与独立原生窗层级问题，若这里不直说后端准备怎么处理，后续排障会非常盲。
	log.Printf(
		"[video] %s native window policy session=%s display=%t manage_x11=%t force_sdl_x11=%t reason=%s",
		p.sourceName,
		p.windowLayerPolicy.sessionType,
		p.windowLayerPolicy.hasDisplay,
		p.windowLayerPolicy.enableX11Stacking,
		p.windowLayerPolicy.forceSDLX11,
		p.windowLayerPolicy.reason,
	)

	p.processMu.Lock()
	p.layout = layout
	p.processMu.Unlock()

	if err := p.ensureProcess(); err != nil {
		return err
	}

	go p.writerLoop()
	return nil
}

// UpdateLayout 在运行时刷新当前原生视频窗布局。
// What: 原子更新布局参数，并在进程仍存活时异步重贴窗口层。
// Why: 双视频常驻下的主次互换不能靠 Stop+Start 完成，只能在同一 ffplay 进程上改几何与堆叠关系。
func (p *NativePlayer) UpdateLayout(layout VideoWindowLayout) error {
	if layout.Width <= 0 || layout.Height <= 0 {
		return fmt.Errorf("invalid video window layout width=%d height=%d", layout.Width, layout.Height)
	}

	p.processMu.Lock()
	previousLayout := p.layout
	p.layout = layout
	pid := 0
	if p.cmd != nil && p.cmd.Process != nil {
		pid = p.cmd.Process.Pid
	}
	p.processMu.Unlock()

	if pid > 0 && videoWindowLayoutRequiresProcessRestart(previousLayout, layout, !p.windowLayerPolicy.enableX11Stacking) {
		return p.restartProcess("video window size changed")
	}

	if pid > 0 && p.windowLayerPolicy.enableX11Stacking {
		go p.applyX11WindowLayer(pid)
	}

	return nil
}

// Stop 停止播放器并等待写入循环退出。
// What: 通过单次关闭语义收口停止流程。
// Why: App 退出路径和异常恢复路径都可能触发 Stop，必须避免重复 close 通道导致 panic。
func (p *NativePlayer) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopCh)

		// What: 在关闭协程前先主动杀掉 ffplay 子进程。
		// Why: 这样可以立即解除 stdin 写阻塞，避免 shutdown 被卡在外部解码进程上。
		p.processMu.Lock()
		p.stopProcessLocked()
		p.processMu.Unlock()
	})

	<-p.doneCh
}

// EnqueueFrame 将完整 HEVC 帧投递给原生播放器。
// What: 采用“保最新帧”的非阻塞入队策略。
// Why: 比赛图传最怕的是越积越慢，因此队列满时必须立刻淘汰旧帧，而不是等待消费者慢慢赶上。
func (p *NativePlayer) EnqueueFrame(frame []byte) {
	if len(frame) == 0 {
		return
	}

	select {
	case <-p.stopCh:
		return
	default:
	}

	item := queuedFrame{
		data:       frame,
		enqueuedAt: time.Now(),
	}

	select {
	case p.framesCh <- item:
		return
	default:
	}

	select {
	case <-p.framesCh:
		dropCount := atomic.AddUint64(&p.decodeDropFrames, 1)
		if dropCount <= 5 || dropCount%60 == 0 {
			log.Printf("[video] Drop stale queued frame to keep freshness (drop_count=%d)", dropCount)
		}
	default:
	}

	select {
	case p.framesCh <- item:
	default:
		dropCount := atomic.AddUint64(&p.decodeDropFrames, 1)
		if dropCount <= 5 || dropCount%60 == 0 {
			log.Printf("[video] Drop latest frame because player queue is still blocked (drop_count=%d)", dropCount)
		}
	}
}

// Snapshot 导出一份线程安全的播放器统计快照。
// What: 将内部浮点 FPS、时间戳和原子计数统一打包。
// Why: 避免 App 层为了拼前端状态去直接摸内部锁和进程状态，降低耦合。
func (p *NativePlayer) Snapshot() PlayerSnapshot {
	p.statsMu.Lock()
	snapshot := PlayerSnapshot{
		DecoderFPS:       p.decoderFPS,
		PresentFPS:       p.presentFPS,
		LastInputAt:      p.lastInputAt,
		LastPresentAt:    p.lastPresentAt,
		WindowLayerReady: p.windowLayerReady,
		WindowLayerErr:   p.windowLayerErr,
		WindowID:         p.windowID,
	}
	p.statsMu.Unlock()

	snapshot.DecodeDropFrames = atomic.LoadUint64(&p.decodeDropFrames)
	snapshot.CorruptFrames = atomic.LoadUint64(&p.corruptFrames)
	snapshot.DecoderResets = atomic.LoadUint64(&p.decoderResets)
	return snapshot
}

// Layout 导出播放器当前绑定的窗口布局。
// What: 在持锁状态下复制当前原生视频层的绝对几何参数。
// Why: App 层需要判断 HUD 是否已经切到另一块屏幕，若不提供这份快照，就无法安全决定是否要重建 ffplay。
func (p *NativePlayer) Layout() VideoWindowLayout {
	p.processMu.Lock()
	layout := p.layout
	p.processMu.Unlock()
	return layout
}

func videoWindowLayoutRequiresProcessRestart(previous VideoWindowLayout, next VideoWindowLayout, requireProcessResize bool) bool {
	if previous.Width <= 0 || previous.Height <= 0 || next.Width <= 0 || next.Height <= 0 {
		return false
	}
	if previous.Width == next.Width && previous.Height == next.Height {
		return false
	}
	if requireProcessResize {
		return true
	}
	return isMajorVideoWindowRoleChange(previous, next)
}

func isMajorVideoWindowRoleChange(previous VideoWindowLayout, next VideoWindowLayout) bool {
	previousArea := int64(previous.Width) * int64(previous.Height)
	nextArea := int64(next.Width) * int64(next.Height)
	if previousArea <= 0 || nextArea <= 0 {
		return false
	}
	smallerArea := previousArea
	largerArea := nextArea
	if smallerArea > largerArea {
		smallerArea, largerArea = largerArea, smallerArea
	}

	// What: 主画面和 PiP 之间的大面积互换直接重启 ffplay，而不是只走 X11 resize。
	// Why: XWayland/SDL 对 1920x1080 <-> 小窗这种角色切换经常只移动不改尺寸，会留下 HUD 几何异常遮罩。
	if largerArea >= smallerArea*4 {
		return true
	}

	previousWide := previous.Width >= previous.Height*2
	nextWide := next.Width >= next.Height*2
	previousSquare := previous.Width*4 <= previous.Height*5 && previous.Height*4 <= previous.Width*5
	nextSquare := next.Width*4 <= next.Height*5 && next.Height*4 <= next.Width*5
	return (previousWide && nextSquare) || (previousSquare && nextWide)
}

// writerLoop 串行消费完整视频帧并写入 ffplay stdin。
// What: 将所有阻塞型 I/O 收口到单协程。
// Why: 这样上层 UDP 组帧和 App 生命周期管理都不需要承担写 pipe 的阻塞风险。
func (p *NativePlayer) writerLoop() {
	defer close(p.doneCh)

	for {
		select {
		case <-p.stopCh:
			return
		case item := <-p.framesCh:
			if len(item.data) == 0 {
				continue
			}

			// What: 对完整帧做最小必要的 Annex-B 合法性探测。
			// Why: official/custom 现在都走 Annex-B 字节流；若连起始码都缺失，继续把脏帧交给播放器只会放大雪花屏和错帧污染。
			if !looksLikeAnnexBFrame(item.data) {
				corruptCount := atomic.AddUint64(&p.corruptFrames, 1)
				if corruptCount <= 5 || corruptCount%60 == 0 {
					log.Printf("[video] Drop suspicious HEVC frame without Annex-B start code (count=%d size=%d)", corruptCount, len(item.data))
				}
				continue
			}

			// What: 在真正写入前先判断当前帧是否已经在本地排队过久。
			// Why: freshness-first 模式下，旧帧宁可只在 Go 侧本地丢弃，也不要再像上一版那样通过频繁重启 ffplay 来“清 backlog”，否则播放器会反复黑屏重启。
			queueAge := time.Since(item.enqueuedAt)
			if queueAge > maxQueuedFrameAge {
				dropCount := atomic.AddUint64(&p.decodeDropFrames, 1)
				if dropCount <= 5 || dropCount%60 == 0 {
					log.Printf("[video] Drop stale frame before ffplay write (drop_count=%d queue_age=%s)", dropCount, queueAge.Round(time.Millisecond))
				}
				continue
			}

			now := time.Now()
			p.noteDecoderFrame(now)

			writeStart := time.Now()

			// What: 首次写入失败时先尝试重建 ffplay 进程，再重放当前帧一次。
			// Why: 比赛机上最常见的故障是子进程退出或 pipe 断裂，直接放弃当前帧会让恢复速度更慢。
			if err := p.writeFrame(item.data); err != nil {
				log.Printf("[video] %s write frame to ffplay failed: %v", p.sourceName, err)
				if restartErr := p.restartProcess("stdin write failure"); restartErr != nil {
					dropCount := atomic.AddUint64(&p.decodeDropFrames, 1)
					if dropCount <= 5 || dropCount%60 == 0 {
						log.Printf("[video] Drop frame because ffplay restart failed (drop_count=%d err=%v)", dropCount, restartErr)
					}
					continue
				}

				dropCount := atomic.AddUint64(&p.decodeDropFrames, 1)
				if dropCount <= 5 || dropCount%60 == 0 {
					log.Printf("[video] Drop current frame after %s ffplay restart; wait for stream sync gate (drop_count=%d)", p.sourceName, dropCount)
				}
				continue
			}

			writeDuration := time.Since(writeStart)
			p.notePresentFrame(time.Now())

			// What: 写 pipe 成功后仍然保留慢阻塞观测。
			// Why: 这类日志可以继续帮助判断本机解码或渲染是否跟不上，但这里绝不能再因为一次慢写就自杀式重启播放器，否则实际效果只会比单纯卡顿更差。
			if writeDuration > slowWriteWarnDuration && len(p.framesCh) > 0 {
				log.Printf("[video] %s ffplay stdin drain is slow, keep newest frames only (write_block=%s queued=%d)", p.sourceName, writeDuration.Round(time.Millisecond), len(p.framesCh))
			}
		}
	}
}

// ensureProcess 保证 ffplay 子进程当前可写。
// What: 仅在句柄缺失时才重新创建进程。
// Why: 正常帧路径必须尽量短，不能每帧都重新触碰进程创建逻辑。
func (p *NativePlayer) ensureProcess() error {
	p.processMu.Lock()
	defer p.processMu.Unlock()

	if p.stdin != nil && p.cmd != nil {
		return nil
	}

	return p.startProcessLocked()
}

// restartProcess 重启 ffplay 子进程。
// What: 将“杀旧进程 + 起新进程”的动作封装为单个临界区。
// Why: 避免 writerLoop 和 wait goroutine 在切换句柄时互相踩踏。
func (p *NativePlayer) restartProcess(reason string) error {
	p.processMu.Lock()
	defer p.processMu.Unlock()

	select {
	case <-p.stopCh:
		return io.ErrClosedPipe
	default:
	}

	resetCount := atomic.AddUint64(&p.decoderResets, 1)
	log.Printf("[video] Restart %s ffplay due to %s (reset_count=%d)", p.sourceName, reason, resetCount)

	if p.onUnexpectedExit != nil {
		p.onUnexpectedExit()
	}

	p.stopProcessLocked()
	return p.startProcessLocked()
}

// startProcessLocked 在持锁状态下启动 ffplay。
// What: 统一收口 ffplay 的低延迟参数。
// Why: 只有在进程级把缓冲、探测时间和同步策略压到最低，才能尽量逼近官方图传的实时性。
func (p *NativePlayer) startProcessLocked() error {
	if p.ffplayPath == "" {
		return errors.New("ffplay binary not found")
	}
	if p.layout.Width <= 0 || p.layout.Height <= 0 {
		return fmt.Errorf("video window layout not ready width=%d height=%d", p.layout.Width, p.layout.Height)
	}

	// What: 每次重新拉起 ffplay 前都先清空上一轮窗口贴合状态。
	// Why: restart 可能发生在上一轮已经 live 之后；若这里不先打回“待贴合”，前端会短暂误以为新窗口已经可安全显示。
	p.resetWindowLayerStateForNewProcess()

	launchAttempts := buildFFplayLaunchAttempts(p.layout, p.inputFormat, p.windowTitle, p.windowLayerPolicy.forceSDLX11)
	launchResult, err := launchFFplayWithFallback(launchAttempts, p.startFFplayProcess)
	if err != nil {
		return err
	}

	p.cmd = launchResult.cmd
	p.stdin = launchResult.stdin

	go p.watchProcessExit(launchResult.cmd)
	go p.watchProcessStderr(launchResult.stderr)

	// What: 进程拉起后异步执行原生窗口下压。
	// Why: 这个动作必须等 ffplay 真正创建出顶层窗后才能做，不能阻塞首帧写入链路。
	if p.windowLayerPolicy.enableX11Stacking {
		go p.applyX11WindowLayer(launchResult.cmd.Process.Pid)
	} else {
		// What: 在无需额外 X11 控层的场景下，直接把窗口层状态视为就绪。
		// Why: 否则上层 video-state 会永远卡在“等待贴合”，即使本机根本不需要 sibling 压层。
		p.setWindowLayerState(0, true, "")
	}

	log.Printf(
		"[video] %s ffplay started mode=%s title=%q pid=%d left=%d top=%d width=%d height=%d",
		p.sourceName,
		launchResult.mode.label,
		p.windowTitle,
		launchResult.cmd.Process.Pid,
		p.layout.Left,
		p.layout.Top,
		p.layout.Width,
		p.layout.Height,
	)
	return nil
}

// stopProcessLocked 在持锁状态下终止当前 ffplay。
// What: 先断 stdin，再杀进程，再清空本地句柄。
// Why: 这样写入协程在下一次 ensureProcess 时就一定会看到“无进程”状态，不会继续向旧 pipe 写数据。
func (p *NativePlayer) stopProcessLocked() {
	if p.stdin != nil {
		_ = p.stdin.Close()
		p.stdin = nil
	}

	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}

	p.cmd = nil
	p.resetWindowLayerStateForNewProcess()
}

// watchProcessExit 等待 ffplay 子进程退出并回收句柄。
// What: 在后台补齐 Wait 调用。
// Why: 不这样做会留下僵尸进程，同时上层也无法感知子进程是否已经崩掉。
func (p *NativePlayer) watchProcessExit(cmd *exec.Cmd) {
	err := cmd.Wait()

	p.processMu.Lock()
	wasCurrent := p.cmd == cmd
	hook := p.onUnexpectedExit
	if wasCurrent {
		p.cmd = nil
		p.stdin = nil
	}
	p.processMu.Unlock()

	if !wasCurrent {
		return
	}

	select {
	case <-p.stopCh:
		return
	default:
	}

	if hook != nil {
		hook()
	}

	if err != nil {
		// What: 将“ffplay 已异常退出”同步写回窗口层状态。
		// Why: 若后续一段时间内都没有新帧触发重建，上层也必须立即知道当前原生视频层已不可用。
		p.setWindowLayerState(0, false, "原生视频进程已退出，等待重建")
		log.Printf("[video] %s ffplay exited unexpectedly: %v", p.sourceName, err)
		return
	}

	p.setWindowLayerState(0, false, "")
	log.Printf("[video] %s ffplay exited", p.sourceName)
}

// watchProcessStderr 读取 ffplay 的错误输出。
// What: 将底层解码器的报错汇总到统一日志。
// Why: 现场排查花屏、坏帧和关键帧丢失时，这些原生日志往往是唯一直接证据。
func (p *NativePlayer) watchProcessStderr(stderr io.ReadCloser) {
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// What: 只对明显的解码异常关键字累计坏帧计数。
		// Why: 该统计只用于暴露链路健康度，不能把普通提示信息也算进坏帧。
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "error") ||
			strings.Contains(lowerLine, "invalid") ||
			strings.Contains(lowerLine, "corrupt") ||
			strings.Contains(lowerLine, "missing") {
			atomic.AddUint64(&p.corruptFrames, 1)
		}

		log.Printf("[video][%s][ffplay] %s", p.sourceName, line)
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-p.stopCh:
			return
		default:
			log.Printf("[video] read ffplay stderr failed: %v", err)
		}
	}
}

// writeFrame 将一帧完整 HEVC 访问单元写入 ffplay stdin。
// What: 使用完整写入语义，避免 pipe 短写把同一帧拆成半帧残片。
// Why: 一旦短写后直接返回，播放器收到的就是被截断的坏帧，雪花和解码异常会立刻放大。
func (p *NativePlayer) writeFrame(frame []byte) error {
	if err := p.ensureProcess(); err != nil {
		return err
	}

	p.processMu.Lock()
	stdin := p.stdin
	p.processMu.Unlock()

	if stdin == nil {
		return io.ErrClosedPipe
	}

	return writeAll(stdin, frame)
}

// noteDecoderFrame 记录进入播放器写入阶段的帧速。
// What: 以 1 秒为窗口近似统计“进入原生视频层的帧率”。
// Why: 这比单纯统计 UDP 收包更接近真实后端能处理的节奏。
func (p *NativePlayer) noteDecoderFrame(now time.Time) {
	p.statsMu.Lock()
	defer p.statsMu.Unlock()

	p.lastInputAt = now.UnixMilli()

	if p.decoderWindowStart.IsZero() {
		p.decoderWindowStart = now
	}

	p.decoderWindowCount++
	if elapsed := now.Sub(p.decoderWindowStart); elapsed >= time.Second {
		p.decoderFPS = float64(p.decoderWindowCount) / elapsed.Seconds()
		p.decoderWindowStart = now
		p.decoderWindowCount = 0
	}
}

// notePresentFrame 记录成功写入 ffplay 的帧速。
// What: 以同样的窗口统计“真正送达原生显示层的帧率”。
// Why: 该指标能直接暴露 pipe 阻塞、子进程退出和本地重启造成的呈现损失。
func (p *NativePlayer) notePresentFrame(now time.Time) {
	p.statsMu.Lock()
	defer p.statsMu.Unlock()

	p.lastPresentAt = now.UnixMilli()
	if p.firstPresentAt == 0 {
		// What: 当前 ffplay 进程第一次成功收到完整帧时，记录这次“首帧送达”的绝对时间。
		// Why: 只有知道本进程真正开始出帧的时刻，窗口层才能区分“首帧前的正常等待”与“稳定出帧后的异常贴窗失败”。
		p.firstPresentAt = p.lastPresentAt
	}

	if p.presentWindowStart.IsZero() {
		p.presentWindowStart = now
	}

	p.presentWindowCount++
	if elapsed := now.Sub(p.presentWindowStart); elapsed >= time.Second {
		p.presentFPS = float64(p.presentWindowCount) / elapsed.Seconds()
		p.presentWindowStart = now
		p.presentWindowCount = 0
	}
}

// setWindowLayerState 记录当前原生窗口是否已经安全贴合到 HUD。
// What: 统一写入“已就绪/未就绪”和最近一次窗口层错误文案。
// Why: buildVideoStatePayload 需要把“有帧输出”和“窗口层贴合成功”分开判断，避免视频窗跑偏时前端继续透出桌面。
func (p *NativePlayer) setWindowLayerState(windowID uint32, ready bool, errMessage string) {
	p.statsMu.Lock()
	p.windowID = windowID
	p.windowLayerReady = ready
	p.windowLayerErr = strings.TrimSpace(errMessage)
	p.statsMu.Unlock()
}

// resetWindowLayerStateForNewProcess 为新一轮 ffplay 进程重置窗口层跟踪状态。
// What: 同时清空贴合就绪态、硬错误文案和当前进程首帧时间。
// Why: 旧进程遗留的窗口层结论绝不能污染新进程；否则新 ffplay 还没真正起窗时，前端就会继续沿用上一轮 live 或 error。
func (p *NativePlayer) resetWindowLayerStateForNewProcess() {
	p.statsMu.Lock()
	p.windowID = 0
	p.windowLayerReady = false
	p.windowLayerErr = ""
	p.firstPresentAt = 0
	p.decoderMode = ""
	p.statsMu.Unlock()
}

func buildFFplayLaunchModes() []ffplayLaunchMode {
	return []ffplayLaunchMode{
		{
			label:    "software",
			software: true,
		},
	}
}

func buildFFplayArgs(layout VideoWindowLayout, inputFormat string, windowTitle string, mode ffplayLaunchMode) []string {
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-avioflags", "direct",
		"-fflags", "discardcorrupt+flush_packets+nobuffer",
		"-framedrop",
		"-sync", "ext",
		"-fpsprobesize", "0",
		"-probesize", "32",
		"-analyzeduration", "0",
		"-max_delay", "0",
		"-use_wallclock_as_timestamps", "1",
		"-threads", "1",
		"-vf", "setpts=0",
		"-fast",
		"-an",
	}

	if mode.hwaccel != "" {
		args = append(args, "-hwaccel", mode.hwaccel)
	}

	args = append(
		args,
		"-window_title", windowTitle,
		"-noborder",
		"-left", strconv.Itoa(layout.Left),
		"-top", strconv.Itoa(layout.Top),
		"-x", strconv.Itoa(layout.Width),
		"-y", strconv.Itoa(layout.Height),
		"-f", inputFormat,
		"-i", "pipe:0",
	)

	return args
}

func buildFFplayEnv(layout VideoWindowLayout, forceSDLX11 bool) []string {
	env := append(
		os.Environ(),
		fmt.Sprintf("SDL_VIDEO_WINDOW_POS=%d,%d", layout.Left, layout.Top),
		"SDL_VIDEO_CENTERED=0",
		"SDL_RENDER_VSYNC=0",
	)

	if forceSDLX11 {
		env = append(env, "SDL_VIDEODRIVER=x11")
	}

	return env
}

func buildFFplayLaunchAttempts(layout VideoWindowLayout, inputFormat string, windowTitle string, forceSDLX11 bool) []ffplayLaunchAttempt {
	modes := buildFFplayLaunchModes()
	attempts := make([]ffplayLaunchAttempt, 0, len(modes))
	for _, mode := range modes {
		attempts = append(attempts, ffplayLaunchAttempt{
			mode: mode,
			args: buildFFplayArgs(layout, inputFormat, windowTitle, mode),
			env:  buildFFplayEnv(layout, forceSDLX11),
		})
	}
	return attempts
}

type ffplayProcessFactory func(args []string, env []string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, error)

func launchFFplayWithFallback(attempts []ffplayLaunchAttempt, factory ffplayProcessFactory) (ffplayLaunchResult, error) {
	if len(attempts) == 0 {
		return ffplayLaunchResult{}, errors.New("no ffplay launch attempts configured")
	}

	var errorsOut []string
	for _, attempt := range attempts {
		cmd, stdin, stderr, err := factory(attempt.args, attempt.env)
		if err == nil {
			return ffplayLaunchResult{
				cmd:    cmd,
				stdin:  stdin,
				stderr: stderr,
				mode:   attempt.mode,
			}, nil
		}
		errorsOut = append(errorsOut, fmt.Sprintf("%s: %v", attempt.mode.label, err))
	}

	return ffplayLaunchResult{}, fmt.Errorf("launch ffplay failed after fallback attempts (%s)", strings.Join(errorsOut, "; "))
}

func (p *NativePlayer) startFFplayProcess(args []string, env []string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, error) {
	cmd := exec.Command(p.ffplayPath, args...)
	cmd.Stdout = io.Discard
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, nil, nil, err
	}

	return cmd, stdin, stderr, nil
}

// windowLayerTimingSnapshot 导出窗口层错误升级所需的最小时间快照。
// What: 只读取当前进程首帧时间与最近一次成功呈现时间。
// Why: 窗口层错误要不要升级成硬错误，只取决于“是否已经稳定出帧”和“最近是不是还在继续出帧”。
func (p *NativePlayer) windowLayerTimingSnapshot() (int64, int64) {
	p.statsMu.Lock()
	firstPresentAt := p.firstPresentAt
	lastPresentAt := p.lastPresentAt
	p.statsMu.Unlock()
	return firstPresentAt, lastPresentAt
}

// currentWindowLayerRuntimeError 结合当前进程的时间快照，决定本次贴窗失败是否应升级成硬错误。
// What: 将当前错误与进程出帧时间统一交给纯函数判定。
// Why: 这样窗口层监督协程可以专注于“持续重试”，而错误升级门槛则能被单元测试单独锁住。
func (p *NativePlayer) currentWindowLayerRuntimeError(err error, now time.Time) string {
	firstPresentAt, lastPresentAt := p.windowLayerTimingSnapshot()
	return buildWindowLayerRuntimeError(err, firstPresentAt, lastPresentAt, now)
}

// isCurrentProcessPID 判断给定 PID 是否仍对应当前 NativePlayer 正在托管的 ffplay 进程。
// What: 在持锁状态下比对当前进程句柄与 PID。
// Why: 贴窗 goroutine 可能晚于进程重启返回；若没有代际校验，旧 goroutine 就可能把上一轮失败态回写给新进程。
func (p *NativePlayer) isCurrentProcessPID(pid int) bool {
	p.processMu.Lock()
	defer p.processMu.Unlock()

	if pid <= 0 || p.cmd == nil || p.cmd.Process == nil {
		return false
	}

	return p.cmd.Process.Pid == pid
}

// shouldPromoteWindowLayerError 判断当前贴窗失败是否应升级成前端可见的硬错误。
// What: 仅当当前进程已经稳定出帧一段时间，且最近仍在持续呈现时才返回 true。
// Why: 这条门槛就是为了把“首帧前/首帧后的迟到建窗”留在等待态，只把“稳定出帧后仍贴不上 HUD”视作真正异常。
func shouldPromoteWindowLayerError(firstPresentAt int64, lastPresentAt int64, now time.Time) bool {
	if firstPresentAt <= 0 || lastPresentAt <= 0 {
		return false
	}

	firstPresentTime := time.UnixMilli(firstPresentAt)
	lastPresentTime := time.UnixMilli(lastPresentAt)

	if now.Sub(firstPresentTime) < windowLayerErrorPromotionAfterPresent {
		return false
	}

	if now.Sub(lastPresentTime) > windowLayerPresentFreshnessThreshold {
		return false
	}

	return true
}

// buildWindowLayerRuntimeError 根据当前进程的呈现时序构造可见错误文案。
// What: 软失败阶段返回空串，只有满足升级门槛时才生成统一硬错误。
// Why: App 层已经把 “WindowLayerReady=false + WindowLayerErr=”” 作为“继续等待贴合 HUD”的语义，这里必须把软失败和硬失败彻底拆开。
func buildWindowLayerRuntimeError(err error, firstPresentAt int64, lastPresentAt int64, now time.Time) string {
	if err == nil {
		return ""
	}

	if !shouldPromoteWindowLayerError(firstPresentAt, lastPresentAt, now) {
		return ""
	}

	return fmt.Sprintf("原生视频窗未能贴合 HUD，已阻止透出桌面: %v", err)
}

// looksLikeAnnexBFrame 判断完整帧里是否至少存在一个 Annex-B 起始码。
// What: 这里只做最小必要校验，不尝试完整解析 NAL。
// Why: 校验太重会反向增加 CPU 延迟，而完全不校验又会把明显坏帧直接灌进显示链路。
func looksLikeAnnexBFrame(frame []byte) bool {
	for index := 0; index+2 < len(frame); index++ {
		if frame[index] != 0x00 || frame[index+1] != 0x00 {
			continue
		}
		if frame[index+2] == 0x01 {
			return true
		}
		if index+3 < len(frame) && frame[index+2] == 0x00 && frame[index+3] == 0x01 {
			return true
		}
	}
	return false
}

// writeAll 保证缓冲区被完整写入目标 writer。
// What: 将 pipe 的潜在短写循环收口在单函数内。
// Why: 这样上层帧写逻辑只需处理“成功 / 失败”，不用每次都手写短写恢复分支。
func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written <= 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}
