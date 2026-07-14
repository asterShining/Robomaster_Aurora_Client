package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"rm-aurora/pkg/config"
	"rm-aurora/pkg/rmcp"
	"rm-aurora/pkg/video"
)

// fakeVideoPlayer 是供 App 单测使用的最小播放器替身。
// What: 用纯内存计数器模拟 Stop、EnqueueFrame、Snapshot 和 Layout。
// Why: 当前回归点集中在“切源时是否停对了播放器、是否把帧投给了正确来源的播放器”，没必要在单测里真的拉起 ffplay 子进程。
type fakeVideoPlayer struct {
	stopCalls      int
	enqueuedFrames [][]byte
	snapshot       video.PlayerSnapshot
	layout         video.VideoWindowLayout
}

func buildTestHEVCNAL(nalType byte, payload ...byte) []byte {
	nal := []byte{0x00, 0x00, 0x00, 0x01, nalType << 1, 0x01}
	return append(nal, payload...)
}

func buildRecoverableHEVCFrame() []byte {
	frame := append([]byte{}, buildTestHEVCNAL(32, 0xaa, 0xbb)...)
	frame = append(frame, buildTestHEVCNAL(33, 0xcc, 0xdd)...)
	frame = append(frame, buildTestHEVCNAL(34, 0xee, 0xff)...)
	frame = append(frame, buildTestHEVCNAL(19, 0x11, 0x22, 0x33)...)
	return frame
}

func buildPredictedHEVCFrame() []byte {
	return buildTestHEVCNAL(1, 0x44, 0x55, 0x66)
}

func bridgeRawCustomStreamForTest(stream []byte, inputChunkSizes []int, outputChunkBytes int) [][]byte {
	if len(stream) == 0 || outputChunkBytes <= 0 {
		return nil
	}
	if len(inputChunkSizes) == 0 {
		inputChunkSizes = []int{outputChunkBytes}
	}

	var (
		pending    []byte
		blockIndex int
		offset     int
		outputs    [][]byte
	)

	for offset < len(stream) {
		chunkBytes := inputChunkSizes[blockIndex%len(inputChunkSizes)]
		if chunkBytes <= 0 {
			chunkBytes = outputChunkBytes
		}
		if chunkBytes > len(stream)-offset {
			chunkBytes = len(stream) - offset
		}

		pending = append(pending, stream[offset:offset+chunkBytes]...)
		offset += chunkBytes
		blockIndex++

		for len(pending) >= outputChunkBytes {
			outputs = append(outputs, append([]byte(nil), pending[:outputChunkBytes]...))
			pending = append([]byte(nil), pending[outputChunkBytes:]...)
		}
	}

	return outputs
}

// Stop 记录一次停止调用。
// What: 仅累计 stop 调用次数。
// Why: 这条计数就是本轮 custom -> official 回归的核心断言，必须能明确看见旧播放器有没有被停掉。
func (p *fakeVideoPlayer) Stop() {
	p.stopCalls++
}

// EnqueueFrame 记录一次帧投递。
// What: 复制输入帧并保存到切片里。
// Why: 测试要验证帧是否被投进了正确播放器，必须保留一份独立副本，避免后续调用方复用底层切片污染断言。
func (p *fakeVideoPlayer) EnqueueFrame(frame []byte) {
	copiedFrame := append([]byte(nil), frame...)
	p.enqueuedFrames = append(p.enqueuedFrames, copiedFrame)
}

// Snapshot 返回预设播放器快照。
// What: 让测试自由控制 LastPresentAt、FPS 等运行态。
// Why: buildVideoStatePayload 要区分“活动源真实 live”与“旧播放器残留导致的假 live”，这里必须可注入时间戳。
func (p *fakeVideoPlayer) Snapshot() video.PlayerSnapshot {
	return p.snapshot
}

// Layout 返回预设窗口布局。
// What: 为布局同步相关调用提供稳定返回值。
// Why: 只要接口契约完整，测试就可以在不依赖真实 ffplay 的情况下编译通过。
func (p *fakeVideoPlayer) Layout() video.VideoWindowLayout {
	return p.layout
}

func (p *fakeVideoPlayer) UpdateLayout(layout video.VideoWindowLayout) error {
	p.layout = layout
	return nil
}

func TestBuildVideoStatePayloadDefaultsToWaitingSource(t *testing.T) {
	// What: 在没有 UDP、没有播放器、没有 MQTT 的最小场景下构造 App。
	// Why: 这条用例锁住“冷启动时前端必须拿到一份可渲染的默认视频状态”，避免后续回归成空对象。
	app := &App{}

	payload := app.buildVideoStatePayload()

	if payload.BackendConnected {
		t.Fatalf("backend should be disconnected by default")
	}
	if payload.VideoConnected {
		t.Fatalf("video should be disconnected by default")
	}
	if payload.ControlLinkConnected {
		t.Fatalf("control link should be disconnected by default")
	}
	if payload.PIPSource != videoSourceCustom {
		t.Fatalf("unexpected default pip source: %s", payload.PIPSource)
	}
	if payload.DisplayState != videoDisplayStateWaitingSource {
		t.Fatalf("unexpected default display state: %s", payload.DisplayState)
	}
	if payload.Message != "等待官方 H.265 视频源（本机 UDP :3334 尚未收到任何首包）" {
		t.Fatalf("unexpected default message: %s", payload.Message)
	}
}

func TestLoadConfiguredUIBootstrapRestoresVideoSourceAndRobotIdentity(t *testing.T) {
	// What: 将用户配置目录切到临时目录，并写入一份同时包含 videoSource 与 mqttRobotIdentity 的快照。
	// Why: 本轮 local custom E2E 依赖后端在 frontend hydrate 前就先拿到这两个字段，否则会先按 official 启动再切源。
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.SaveClientConfigJSON(`{"version":4,"uiPanel":{"videoSource":"custom","mqttRobotIdentity":"red_hero"}}`); err != nil {
		t.Fatalf("save bootstrap config failed: %v", err)
	}

	bootstrap := loadConfiguredUIBootstrap()
	if !bootstrap.hasVideoSource || bootstrap.videoSource != videoSourceCustom {
		t.Fatalf("unexpected bootstrap video source: has=%t value=%s", bootstrap.hasVideoSource, bootstrap.videoSource)
	}
	if !bootstrap.hasMQTTRobotIdentity || bootstrap.mqttRobotIdentity != mqttHeroIdentityRedHero {
		t.Fatalf("unexpected bootstrap robot identity: has=%t value=%s", bootstrap.hasMQTTRobotIdentity, bootstrap.mqttRobotIdentity)
	}
}

func TestLoadConfiguredUIBootstrapForcesOfficialVideoForNonHero(t *testing.T) {
	// What: 非英雄身份即使旧配置保存了 custom，也必须在启动期归一化为 official。
	// Why: 其他兵种没有自定义视频源，不能让应用冷启动后卡在等待 0x0310 的黑幕。
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.SaveClientConfigJSON(`{"version":4,"uiPanel":{"videoSource":"custom","mqttRobotIdentity":"red_infantry_4"}}`); err != nil {
		t.Fatalf("save bootstrap config failed: %v", err)
	}

	bootstrap := loadConfiguredUIBootstrap()
	if !bootstrap.hasMQTTRobotIdentity || bootstrap.mqttRobotIdentity != "red_infantry_4" {
		t.Fatalf("unexpected bootstrap robot identity: has=%t value=%s", bootstrap.hasMQTTRobotIdentity, bootstrap.mqttRobotIdentity)
	}
	if !bootstrap.hasVideoSource || bootstrap.videoSource != videoSourceOfficial {
		t.Fatalf("non-hero custom config should normalize to official, has=%t value=%s", bootstrap.hasVideoSource, bootstrap.videoSource)
	}
}

func TestLoadConfiguredUIBootstrapKeepsLegacyHeroIdentity(t *testing.T) {
	// What: 旧配置只有 mqttHeroIdentity 时仍可恢复。
	// Why: 现场升级不能要求用户清空旧配置才能继续连接原英雄身份。
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.SaveClientConfigJSON(`{"version":3,"uiPanel":{"videoSource":"official","mqttHeroIdentity":"red_hero"}}`); err != nil {
		t.Fatalf("save legacy bootstrap config failed: %v", err)
	}

	bootstrap := loadConfiguredUIBootstrap()
	if !bootstrap.hasMQTTRobotIdentity || bootstrap.mqttRobotIdentity != mqttHeroIdentityRedHero {
		t.Fatalf("unexpected legacy bootstrap robot identity: has=%t value=%s", bootstrap.hasMQTTRobotIdentity, bootstrap.mqttRobotIdentity)
	}
}

func TestMQTTRobotIdentityMappingSupportsCommonRobotIDs(t *testing.T) {
	cases := []struct {
		identity string
		clientID uint16
	}{
		{mqttHeroIdentityRedHero, 1},
		{"red_engineer", 2},
		{"red_infantry_3", 3},
		{"red_infantry_4", 4},
		{"red_infantry_5", 5},
		{"red_drone", 6},
		{"red_sentry", 7},
		{mqttHeroIdentityBlueHero, 101},
		{"blue_engineer", 102},
		{"blue_infantry_3", 103},
		{"blue_infantry_4", 104},
		{"blue_infantry_5", 105},
		{"blue_drone", 106},
		{"blue_sentry", 107},
		{"red_infantry", 3},
		{"104", 104},
	}

	for _, tc := range cases {
		normalized := normalizeMQTTRobotIdentity(tc.identity)
		if normalized == "" {
			t.Fatalf("identity %q should normalize", tc.identity)
		}
		if got := mqttRobotIdentityToClientID(normalized); got != tc.clientID {
			t.Fatalf("identity %q mapped to %d, want %d", tc.identity, got, tc.clientID)
		}
		if got := robotIDToIdentityLabel(tc.clientID); mqttRobotIdentityToClientID(got) != tc.clientID {
			t.Fatalf("clientID %d resolved to non-roundtrippable identity %q", tc.clientID, got)
		}
	}
}

func TestResolveOfficialVideoConfigSupportsTestOverrides(t *testing.T) {
	// What: 同时覆盖官方视频禁用开关与监听端口。
	// Why: 本地 custom-only 联调必须能跳过 :3334，又要保留需要时的端口覆写能力。
	t.Setenv("RM_DISABLE_OFFICIAL_UDP", "1")
	t.Setenv("RM_OFFICIAL_UDP_PORT", "43334")

	config := resolveOfficialVideoConfig()
	if !config.disabled {
		t.Fatalf("expected official video config to be disabled")
	}
	if config.port != 43334 {
		t.Fatalf("unexpected overridden official video port: %d", config.port)
	}
}

func TestBuildPiPVideoWindowLayoutMatchesDesktopFrameSlots(t *testing.T) {
	// What: 桌面 1920x1080 下分别计算 official 与 custom PiP 原生窗口尺寸。
	// Why: 前端框体是 29vw/560px 与 24vw/420px；后端尺寸偏小会让视频无法铺满小窗。
	hudLayout := video.VideoWindowLayout{Width: 1920, Height: 1080, HUDWindowID: 7}

	officialLayout := buildPiPVideoWindowLayout(hudLayout, videoSourceOfficial)
	if officialLayout.Width != 557 || officialLayout.Height != 313 {
		t.Fatalf("unexpected official desktop pip size: %dx%d", officialLayout.Width, officialLayout.Height)
	}
	if officialLayout.Left != 1339 || officialLayout.Top != 24 {
		t.Fatalf("unexpected official desktop pip position: left=%d top=%d", officialLayout.Left, officialLayout.Top)
	}

	customLayout := buildPiPVideoWindowLayout(hudLayout, videoSourceCustom)
	if customLayout.Width != 420 || customLayout.Height != 420 {
		t.Fatalf("unexpected custom desktop pip size: %dx%d", customLayout.Width, customLayout.Height)
	}
	if customLayout.Left != 1476 || customLayout.Top != 24 {
		t.Fatalf("unexpected custom desktop pip position: left=%d top=%d", customLayout.Left, customLayout.Top)
	}
}

func TestBuildPiPVideoWindowLayoutMatchesMobileFrameSlots(t *testing.T) {
	// What: 窄屏下按前端 media query 的 min(46vw,300px) / min(38vw,220px) 计算 PiP。
	// Why: 现场窗口缩放或外接屏变更时，小窗视频仍必须贴合前端框体。
	hudLayout := video.VideoWindowLayout{Width: 900, Height: 600, HUDWindowID: 7}

	officialLayout := buildPiPVideoWindowLayout(hudLayout, videoSourceOfficial)
	if officialLayout.Width != 300 || officialLayout.Height != 169 {
		t.Fatalf("unexpected official mobile pip size: %dx%d", officialLayout.Width, officialLayout.Height)
	}
	if officialLayout.Left != 588 || officialLayout.Top != 12 {
		t.Fatalf("unexpected official mobile pip position: left=%d top=%d", officialLayout.Left, officialLayout.Top)
	}

	customLayout := buildPiPVideoWindowLayout(hudLayout, videoSourceCustom)
	if customLayout.Width != 220 || customLayout.Height != 220 {
		t.Fatalf("unexpected custom mobile pip size: %dx%d", customLayout.Width, customLayout.Height)
	}
	if customLayout.Left != 668 || customLayout.Top != 12 {
		t.Fatalf("unexpected custom mobile pip position: left=%d top=%d", customLayout.Left, customLayout.Top)
	}
}

func TestQuitAppRequiresRuntimeContext(t *testing.T) {
	// What: 在没有 Wails runtime context 的最小场景下直接请求退出。
	// Why: 前端开发态不会注入真实 runtime；这里必须稳定返回错误，避免退出确认弹层落成后在调试环境里变成静默无反应。
	app := NewApp()

	if err := app.QuitApp(); err == nil {
		t.Fatalf("expected quit without runtime context to fail")
	}
}

func TestResolveTeamSideFromClientID(t *testing.T) {
	// What: 分别验证红方与蓝方机器人 ID 的阵营归属。
	// Why: MQTT clientID 自动切换后，阵营映射不能再隐含“永远是蓝方英雄”的历史假设。
	if side := resolveTeamSideFromClientID(1); side != teamSideRed {
		t.Fatalf("unexpected side for red hero: %s", side)
	}
	if side := resolveTeamSideFromClientID(101); side != teamSideBlue {
		t.Fatalf("unexpected side for blue hero: %s", side)
	}
}

func TestBuildVideoStatePayloadPrefersPlayerError(t *testing.T) {
	// What: 人为注入原生视频层启动错误。
	// Why: 前端状态条需要优先看到“播放器不可用”而不是被笼统的“等待视频源”覆盖掉。
	app := &App{
		officialPlayerErr: "原生视频层不可用: ffplay missing",
	}

	payload := app.buildVideoStatePayload()

	if payload.DisplayState != videoDisplayStateRuntimeError {
		t.Fatalf("unexpected player error display state: %s", payload.DisplayState)
	}
	if payload.Message != app.officialPlayerErr {
		t.Fatalf("unexpected player error message: got=%s want=%s", payload.Message, app.officialPlayerErr)
	}
}

func TestBuildVideoStatePayloadReportsOfficialVideoDisabled(t *testing.T) {
	// What: 构造官方 UDP 已在测试态禁用的场景。
	// Why: 用户若误停在 official 源，前端必须直说“当前只测 custom 链路”，不能继续给出误导性的等待首包提示。
	app := NewApp()
	app.officialVideoDisabled = true

	payload := app.buildVideoStatePayload()
	if payload.DisplayState != videoDisplayStateWaitingSource {
		t.Fatalf("unexpected display state when official udp is disabled: %s", payload.DisplayState)
	}
	if payload.Message != "官方 UDP 图传已在测试态禁用，当前仅验证 custom 0x0310 链路" {
		t.Fatalf("unexpected disabled-official message: %s", payload.Message)
	}
	if payload.OfficialDisplayState != videoDisplayStateWaitingSource {
		t.Fatalf("unexpected official display state when disabled: %s", payload.OfficialDisplayState)
	}
}

func TestIsRecentTimestamp(t *testing.T) {
	// What: 构造一个仍在活跃窗口内的毫秒时间戳。
	// Why: 视频包、完整帧和本地呈现都依赖这套活跃判断，必须有最小测试锁住边界。
	recentTimestamp := time.Now().Add(-300 * time.Millisecond).UnixMilli()

	if !isRecentTimestamp(recentTimestamp, time.Second) {
		t.Fatalf("recent timestamp should be treated as active")
	}
	if isRecentTimestamp(recentTimestamp, 100*time.Millisecond) {
		t.Fatalf("recent timestamp should not pass an overly small threshold")
	}
}

func TestAgeFromTimestamp(t *testing.T) {
	// What: 取一个 200ms 前的时间戳并计算年龄。
	// Why: 前端连接态直接展示该毫秒值，不能出现负数或明显失真。
	timestamp := time.Now().Add(-200 * time.Millisecond).UnixMilli()

	age := ageFromTimestamp(timestamp)
	if age < 100 || age > 1000 {
		t.Fatalf("unexpected timestamp age: %d", age)
	}
	if ageFromTimestamp(0) != 0 {
		t.Fatalf("zero timestamp should map to zero age")
	}
}

func TestVideoWindowLayoutsEqualTreatsAbsoluteOffsetChangeAsDifferent(t *testing.T) {
	// What: 构造两份尺寸相同但绝对 Left 不同的视频布局。
	// Why: 多屏切换时最容易被忽略的就是“尺寸没变但屏幕变了”，这条用例专门锁住偏移变化也必须触发重建。
	leftLayout := video.VideoWindowLayout{
		Left:   0,
		Top:    0,
		Width:  1920,
		Height: 1080,
	}
	rightLayout := video.VideoWindowLayout{
		Left:   1920,
		Top:    0,
		Width:  1920,
		Height: 1080,
	}

	if videoWindowLayoutsEqual(leftLayout, rightLayout) {
		t.Fatalf("expected absolute offset change to be treated as different layout")
	}
}

func TestVideoWindowLayoutsEqualTreatsHUDWindowIdentityChangeAsDifferent(t *testing.T) {
	// What: 构造两份几何完全一致、但 HUD 顶层窗身份不同的布局。
	// Why: Wayland/XWayland 下 HUD 可能在运行时重映射成新的顶层窗；若这里忽略窗口身份，ffplay 会继续压在旧窗下面。
	leftLayout := video.VideoWindowLayout{
		Left:        0,
		Top:         0,
		Width:       1920,
		Height:      1080,
		HUDWindowID: 0x100001,
	}
	rightLayout := video.VideoWindowLayout{
		Left:        0,
		Top:         0,
		Width:       1920,
		Height:      1080,
		HUDWindowID: 0x100002,
	}

	if videoWindowLayoutsEqual(leftLayout, rightLayout) {
		t.Fatalf("expected HUD window identity change to be treated as different layout")
	}
}

func TestSetActiveVideoSourceSwitchesRuntimeStateWithoutPlayer(t *testing.T) {
	// What: 在没有 ctx、没有 ffplay 的最小场景下切换 official/custom。
	// Why: 源切换的运行时状态不应依赖真实播放器存在，否则前端面板与后端状态机会继续发生错位。
	app := NewApp()
	app.configureRMIdentity(1)

	if err := app.SetActiveVideoSource(videoSourceCustom); err != nil {
		t.Fatalf("switch to custom failed: %v", err)
	}

	state := app.getVideoRuntimeState()
	if state.activeVideoSource != videoSourceCustom {
		t.Fatalf("unexpected active source after custom switch: %s", state.activeVideoSource)
	}
	if state.officialPlayer != nil || state.customPlayer != nil {
		t.Fatalf("players should stay empty without real runtime")
	}

	if err := app.SetActiveVideoSource(videoSourceOfficial); err != nil {
		t.Fatalf("switch back to official failed: %v", err)
	}

	state = app.getVideoRuntimeState()
	if state.activeVideoSource != videoSourceOfficial {
		t.Fatalf("unexpected active source after official switch: %s", state.activeVideoSource)
	}
	if state.officialPlayer != nil || state.customPlayer != nil {
		t.Fatalf("players should stay empty after switching back without real runtime")
	}
}

func TestSetActiveVideoSourceRejectsCustomForNonHeroIdentity(t *testing.T) {
	// What: 非英雄身份下请求 custom 会被后端兜底为 official。
	// Why: 旧前端或异常配置不能绕过 UI 禁用态，让其他兵种进入等待自定义视频的错误主画面。
	app := NewApp()
	app.configureRMIdentity(104)

	if err := app.SetActiveVideoSource(videoSourceCustom); err != nil {
		t.Fatalf("switch to custom should be normalized, got error: %v", err)
	}

	state := app.getVideoRuntimeState()
	if state.activeVideoSource != videoSourceOfficial {
		t.Fatalf("non-hero custom switch should normalize to official, got %s", state.activeVideoSource)
	}
}

func TestSetMQTTRobotIdentityForcesOfficialWhenLeavingHero(t *testing.T) {
	// What: 从英雄 custom 主画面切到步兵身份时自动回 official。
	// Why: 身份切换是现场最快捷的操作路径，不能让上一个英雄的 custom 选择残留到其他兵种。
	app := NewApp()
	app.configureRMIdentity(1)
	app.setActiveVideoSource(videoSourceCustom)

	if err := app.SetMQTTRobotIdentity("blue_infantry_4"); err != nil {
		t.Fatalf("switch robot identity failed: %v", err)
	}

	state := app.getVideoRuntimeState()
	if state.activeVideoSource != videoSourceOfficial {
		t.Fatalf("non-hero identity should force official, got %s", state.activeVideoSource)
	}
}

func TestSetActiveVideoSourceKeepsPersistentPlayersRunning(t *testing.T) {
	// What: 在内存里挂上 official/custom 两个常驻播放器，再切换主画面。
	// Why: 双视频方案要求主次互换只改布局，不允许再通过 Stop+Start 杀掉另一条链路。
	app := NewApp()
	officialPlayer := &fakeVideoPlayer{}
	customPlayer := &fakeVideoPlayer{}
	app.setActiveVideoSource(videoSourceOfficial)
	app.setVideoPlayerState(videoSourceOfficial, officialPlayer, "")
	app.setVideoPlayerState(videoSourceCustom, customPlayer, "")

	if err := app.SetActiveVideoSource(videoSourceCustom); err != nil {
		t.Fatalf("switch to custom failed: %v", err)
	}

	state := app.getVideoRuntimeState()
	if officialPlayer.stopCalls != 0 {
		t.Fatalf("official player should stay running, got stopCalls=%d", officialPlayer.stopCalls)
	}
	if customPlayer.stopCalls != 0 {
		t.Fatalf("custom player should stay running, got stopCalls=%d", customPlayer.stopCalls)
	}
	if state.officialPlayer != officialPlayer {
		t.Fatalf("official player should remain attached")
	}
	if state.customPlayer != customPlayer {
		t.Fatalf("custom player should remain attached")
	}
	if state.activeVideoSource != videoSourceCustom {
		t.Fatalf("unexpected active source after switch: %s", state.activeVideoSource)
	}
}

func TestBuildVideoStatePayloadExposesPiPSourceAndPerSourceState(t *testing.T) {
	// What: 构造 official/custom 两条链都已出画的双常驻场景。
	// Why: 前端现在既要按 active_source 渲染主画面，又要按 pip_source 独立渲染右上角 PiP，占位协议必须一次带齐两路状态。
	app := NewApp()
	app.setActiveVideoSource(videoSourceOfficial)
	now := time.Now().UnixMilli()
	app.setVideoPlayerState(videoSourceOfficial, &fakeVideoPlayer{
		snapshot: video.PlayerSnapshot{
			LastPresentAt:    now,
			PresentFPS:       60,
			DecoderFPS:       60,
			WindowLayerReady: true,
		},
	}, "")
	app.noteCustomByteBlockReceipt(now)
	app.setVideoPlayerState(videoSourceCustom, &fakeVideoPlayer{
		snapshot: video.PlayerSnapshot{
			LastPresentAt:    now,
			PresentFPS:       60,
			DecoderFPS:       60,
			WindowLayerReady: true,
		},
	}, "")

	payload := app.buildVideoStatePayload()
	if payload.ActiveSource != videoSourceOfficial {
		t.Fatalf("unexpected active source: %s", payload.ActiveSource)
	}
	if payload.PIPSource != videoSourceCustom {
		t.Fatalf("unexpected pip source: %s", payload.PIPSource)
	}
	if payload.CustomDisplayState != videoDisplayStateLive || !payload.CustomVideoConnected {
		t.Fatalf("expected custom pip to report live")
	}
	if payload.OfficialDisplayState == videoDisplayStateRuntimeError {
		t.Fatalf("official main should stay in waiting/resync path without injected UDP, got=%s", payload.OfficialDisplayState)
	}
}

func TestBuildVideoStatePayloadCustomSourceWaitsForWindowLayerReady(t *testing.T) {
	// What: 构造“自定义码流已在 ffplay 中出画，但窗口层尚未贴合 HUD”的场景。
	// Why: 这正是现场会漏出桌面的危险过渡态；前端必须继续保留黑幕，而不是误判成 live。
	app := NewApp()
	now := time.Now().UnixMilli()
	app.setActiveVideoSource(videoSourceCustom)
	app.noteCustomByteBlockReceipt(now)
	app.setVideoPlayerState(videoSourceCustom, &fakeVideoPlayer{
		snapshot: video.PlayerSnapshot{
			LastPresentAt:    now,
			PresentFPS:       60,
			DecoderFPS:       60,
			WindowLayerReady: false,
		},
	}, "")

	payload := app.buildVideoStatePayload()
	if payload.DisplayState != videoDisplayStateWaitingFrame {
		t.Fatalf("unexpected display state while window layer is pending: %s", payload.DisplayState)
	}
	if payload.VideoConnected {
		t.Fatalf("window layer pending should not report video connected")
	}
	if payload.Message != "自定义视频窗正在贴合 HUD" {
		t.Fatalf("unexpected window-layer pending message: %s", payload.Message)
	}
}

func TestBuildVideoStatePayloadPromotesWindowLayerError(t *testing.T) {
	// What: 构造“播放器还在当前活动源上，但窗口层校验已经失败”的场景。
	// Why: 用户现在看到的是 HUD 下漏桌面；只要窗口层失败，就必须优先进入 runtime_error，而不是继续输出等待态。
	app := NewApp()
	app.setActiveVideoSource(videoSourceOfficial)
	app.setVideoPlayerState(videoSourceOfficial, &fakeVideoPlayer{
		snapshot: video.PlayerSnapshot{
			LastPresentAt:    time.Now().UnixMilli(),
			PresentFPS:       60,
			DecoderFPS:       60,
			WindowLayerReady: false,
			WindowLayerErr:   "原生视频窗未能贴合 HUD，已阻止透出桌面: geometry mismatch",
		},
	}, "")

	payload := app.buildVideoStatePayload()
	if payload.DisplayState != videoDisplayStateRuntimeError {
		t.Fatalf("unexpected display state for window layer error: %s", payload.DisplayState)
	}
	if payload.Message != "原生视频窗未能贴合 HUD，已阻止透出桌面: geometry mismatch" {
		t.Fatalf("unexpected window layer error message: %s", payload.Message)
	}
}

func TestOnH265FrameReadyRoutesOnlyToOfficialPlayer(t *testing.T) {
	// What: 构造 official/custom 双播放器并把主画面切到 custom。
	// Why: 两路常驻后，官方 H.265 帧必须永远只进 official 播放器，不能再受当前主次关系影响。
	officialFrame := buildRecoverableHEVCFrame()

	app := NewApp()
	officialPlayer := &fakeVideoPlayer{}
	customPlayer := &fakeVideoPlayer{}
	app.setActiveVideoSource(videoSourceCustom)
	app.setVideoPlayerState(videoSourceOfficial, officialPlayer, "")
	app.setVideoPlayerState(videoSourceCustom, customPlayer, "")
	app.onH265FrameReady(officialFrame)
	if len(officialPlayer.enqueuedFrames) != 1 {
		t.Fatalf("official frame should be enqueued into official player, got=%d", len(officialPlayer.enqueuedFrames))
	}
	if len(customPlayer.enqueuedFrames) != 0 {
		t.Fatalf("official frame should not be enqueued into custom player")
	}
	if !bytes.Equal(officialPlayer.enqueuedFrames[0], officialFrame) {
		t.Fatalf("unexpected enqueued official frame payload")
	}
}

func TestOnH265FrameReadyFeedsContinuousOfficialPIPFrames(t *testing.T) {
	// What: 在 custom 主画面场景下连续注入官方关键帧和普通 P 帧。
	// Why: 双屏同时显示时 official PiP 必须接收完整连续帧流；只抽关键帧会让小窗卡顿并掩盖真实链路延迟。
	app := NewApp()
	officialPlayer := &fakeVideoPlayer{}
	app.setActiveVideoSource(videoSourceCustom)
	app.setVideoPlayerState(videoSourceOfficial, officialPlayer, "")

	app.onH265FrameReady(buildRecoverableHEVCFrame())
	if len(officialPlayer.enqueuedFrames) != 1 {
		t.Fatalf("recoverable official frame should pass immediately, got=%d", len(officialPlayer.enqueuedFrames))
	}

	app.onH265FrameReady(buildPredictedHEVCFrame())
	if len(officialPlayer.enqueuedFrames) != 2 {
		t.Fatalf("official PiP predicted frame should be kept continuous, got=%d", len(officialPlayer.enqueuedFrames))
	}

	app.setActiveVideoSource(videoSourceOfficial)
	app.onH265FrameReady(buildPredictedHEVCFrame())
	if len(officialPlayer.enqueuedFrames) != 3 {
		t.Fatalf("official main source should not be throttled, got=%d", len(officialPlayer.enqueuedFrames))
	}
}

func TestOnH265FrameReadyCachesBootstrapUntilOfficialPlayerExists(t *testing.T) {
	// What: 先在 official 播放器尚未挂入时收到一张可恢复 HEVC 帧，再在下一张普通帧到来前挂入播放器。
	// Why: 双常驻启动后，官方首个关键帧可能早于 official ffplay 可用时刻；这张帧必须被缓存并在播放器就绪后作为 bootstrap 回放。
	app := NewApp()
	bootstrapFrame := buildRecoverableHEVCFrame()
	nextFrame := buildPredictedHEVCFrame()

	app.onH265FrameReady(bootstrapFrame)

	officialPlayer := &fakeVideoPlayer{}
	app.setVideoPlayerState(videoSourceOfficial, officialPlayer, "")
	app.markOfficialBootstrapPending()
	app.onH265FrameReady(nextFrame)

	if len(officialPlayer.enqueuedFrames) != 2 {
		t.Fatalf("expected bootstrap + next frame to be enqueued, got=%d", len(officialPlayer.enqueuedFrames))
	}
	if !bytes.Equal(officialPlayer.enqueuedFrames[0], bootstrapFrame) {
		t.Fatalf("unexpected first bootstrap frame payload")
	}
	if !bytes.Equal(officialPlayer.enqueuedFrames[1], nextFrame) {
		t.Fatalf("unexpected second frame payload")
	}
}

func TestOnH265FrameReadyDropsPreSyncPredictedFrames(t *testing.T) {
	// What: 在 official 仍未同步时注入一张只有普通 slice 的 HEVC 帧。
	// Why: 这条回归锁住“中途 P 帧不能再直接喂给 official ffplay”，否则会重新触发 PPS id out of range。
	app := NewApp()
	officialPlayer := &fakeVideoPlayer{}
	app.setVideoPlayerState(videoSourceOfficial, officialPlayer, "")

	app.onH265FrameReady(buildPredictedHEVCFrame())

	if len(officialPlayer.enqueuedFrames) != 0 {
		t.Fatalf("pre-sync predicted frame should not be enqueued into official player")
	}
}

func TestResetOfficialStreamForPlayerRestartRequiresFreshBootstrap(t *testing.T) {
	// What: 先让官方链路同步并出一帧，再模拟 ffplay 因 PiP 尺寸变化重启。
	// Why: 重启后的播放器不能继续吃中途 P 帧，必须等下一张参数集齐全的 IRAP 才能恢复。
	app := NewApp()
	officialPlayer := &fakeVideoPlayer{}
	app.setVideoPlayerState(videoSourceOfficial, officialPlayer, "")

	firstBootstrap := buildRecoverableHEVCFrame()
	app.onH265FrameReady(firstBootstrap)
	if len(officialPlayer.enqueuedFrames) != 1 {
		t.Fatalf("expected initial bootstrap frame to be enqueued, got=%d", len(officialPlayer.enqueuedFrames))
	}
	if !app.hevcSyncGate.IsSynced() {
		t.Fatalf("official sync gate should be synced after recoverable frame")
	}

	app.resetOfficialStreamForPlayerRestart()
	app.onH265FrameReady(buildPredictedHEVCFrame())
	if len(officialPlayer.enqueuedFrames) != 1 {
		t.Fatalf("post-restart predicted frame should be dropped, got=%d", len(officialPlayer.enqueuedFrames))
	}

	secondBootstrap := buildRecoverableHEVCFrame()
	app.onH265FrameReady(secondBootstrap)
	if len(officialPlayer.enqueuedFrames) != 2 {
		t.Fatalf("expected fresh bootstrap after restart, got=%d", len(officialPlayer.enqueuedFrames))
	}
	if !bytes.Equal(officialPlayer.enqueuedFrames[1], secondBootstrap) {
		t.Fatalf("unexpected post-restart bootstrap payload")
	}
}

func TestOnCustomByteBlockRoutesOnlyToCustomPlayer(t *testing.T) {
	// What: 通过“上位机原始 H.264 流 -> C 板 regroup 成 300B 0x0310 块 -> 客户端 onCustomByteBlock”完整注入双播放器场景。
	// Why: custom AU 现在必须无条件只流向 custom 播放器，即便当前主画面仍停在 official；首个同步块还要保护 IDR 不被同块后续 AU 挤出小队列。
	sps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x64, 0x00, 0x1f}, bytes.Repeat([]byte{0x21}, 24)...)
	pps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xee, 0x06, 0xf2}, bytes.Repeat([]byte{0x32}, 12)...)
	idr := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x84}, bytes.Repeat([]byte{0x43}, 196)...)
	pSlice1 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x54}, 174)...)
	pSlice2 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x65}, 154)...)
	pSlice3 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x76}, 138)...)
	stream := append(append(append(append(append([]byte{}, sps...), pps...), idr...), pSlice1...), pSlice2...)
	stream = append(stream, pSlice3...)

	blocks := bridgeRawCustomStreamForTest(stream, []int{64, 64, 44, 61, 87, 108, 72, 96}, 300)
	if len(blocks) < 2 {
		t.Fatalf("expected at least two regrouped 0x0310 blocks, got=%d", len(blocks))
	}

	app := NewApp()
	officialPlayer := &fakeVideoPlayer{}
	customPlayer := &fakeVideoPlayer{}
	app.setActiveVideoSource(videoSourceOfficial)
	app.setVideoPlayerState(videoSourceOfficial, officialPlayer, "")
	app.setVideoPlayerState(videoSourceCustom, customPlayer, "")
	for _, block := range blocks {
		app.onCustomByteBlock(&rmcp.CustomByteBlock{Data: block})
	}
	if len(officialPlayer.enqueuedFrames) != 0 {
		t.Fatalf("custom frame should not be enqueued into official player")
	}
	if len(customPlayer.enqueuedFrames) != 1 {
		t.Fatalf("bootstrap burst should enqueue only first recoverable custom AU, got=%d", len(customPlayer.enqueuedFrames))
	}

	expectedIDRAU := append(append(append([]byte{}, sps...), pps...), idr...)
	if !bytes.Equal(customPlayer.enqueuedFrames[0], expectedIDRAU) {
		t.Fatalf("unexpected first enqueued custom AU")
	}
	if app.h264Reassembler.FramesOut() < 2 {
		t.Fatalf("expected reassembler to recover and drain post-bootstrap AUs, got=%d", app.h264Reassembler.FramesOut())
	}
}

func TestOnCustomByteBlockKeepsDiagnosingWhenCustomPlayerStopped(t *testing.T) {
	// What: 模拟官方主画面优先策略下 custom ffplay 没有启动，但 0x0310 仍持续到达。
	// Why: 为了保护官方低延迟，客户端会停掉备用 custom 显示进程；这不能让 H.264 协议诊断也一起失效。
	sps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x64, 0x00, 0x1f}, bytes.Repeat([]byte{0x21}, 24)...)
	pps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xee, 0x06, 0xf2}, bytes.Repeat([]byte{0x32}, 12)...)
	idr := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x84}, bytes.Repeat([]byte{0x43}, 196)...)
	pSlice1 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x54}, 174)...)
	pSlice2 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x65}, 154)...)
	stream := append(append(append(append([]byte{}, sps...), pps...), idr...), pSlice1...)
	stream = append(stream, pSlice2...)

	app := NewApp()
	app.setActiveVideoSource(videoSourceOfficial)
	for _, block := range bridgeRawCustomStreamForTest(stream, []int{80, 64, 112, 90}, 300) {
		app.onCustomByteBlock(&rmcp.CustomByteBlock{Data: block})
	}

	diag := app.h264Reassembler.Diagnostics()
	if diag.NALTotal == 0 || diag.SPS == 0 || diag.PPS == 0 || diag.IDR == 0 {
		t.Fatalf("expected custom h264 diagnostics without custom player, got=%+v", diag)
	}

	payload := app.buildVideoStatePayload()
	if !payload.CustomAvailable {
		t.Fatalf("custom source should still be marked available while player is stopped")
	}
	if payload.CustomH264NALTotal == 0 || payload.CustomH264IDR == 0 {
		t.Fatalf("expected h264 diagnostics in payload, got nal=%d idr=%d", payload.CustomH264NALTotal, payload.CustomH264IDR)
	}
}

func TestOnCustomByteBlockResyncsAfterInputGap(t *testing.T) {
	// What: 模拟 MQTT/官方链路中断一段时间后，只先恢复普通 P 帧，再恢复下一组参数集与 IDR。
	// Why: 断流期间解码参考链已经断裂，客户端必须丢弃恢复瞬间的旧参考 P 帧，避免 ffplay 短暂炸色块。
	sps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x64, 0x00, 0x1f}, bytes.Repeat([]byte{0x21}, 24)...)
	pps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xee, 0x06, 0xf2}, bytes.Repeat([]byte{0x32}, 12)...)
	idr := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x84}, bytes.Repeat([]byte{0x43}, 80)...)
	pSlice1 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x54}, 32)...)
	pSlice2 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x65}, 32)...)
	pSlice3 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x76}, 32)...)

	app := NewApp()
	player := &fakeVideoPlayer{}
	app.setActiveVideoSource(videoSourceCustom)
	app.setVideoPlayerState(videoSourceCustom, player, "")

	initialStream := append(append(append(append([]byte{}, sps...), pps...), idr...), pSlice1...)
	initialStream = append(initialStream, pSlice2...)
	app.onCustomByteBlock(&rmcp.CustomByteBlock{Data: initialStream})
	if len(player.enqueuedFrames) != 1 {
		t.Fatalf("expected initial recoverable IDR only, got=%d", len(player.enqueuedFrames))
	}
	if !app.h264Reassembler.Diagnostics().Synced {
		t.Fatalf("custom stream should be synced after initial IDR")
	}

	app.noteCustomByteBlockReceipt(time.Now().Add(-customStreamGapResyncThreshold - 100*time.Millisecond).UnixMilli())
	app.onCustomByteBlock(&rmcp.CustomByteBlock{Data: append(append([]byte{}, pSlice1...), pSlice2...)})
	if len(player.enqueuedFrames) != 1 {
		t.Fatalf("post-gap P frame should be dropped until next IDR, got=%d", len(player.enqueuedFrames))
	}
	if app.h264Reassembler.Diagnostics().Synced {
		t.Fatalf("custom stream should wait for a fresh IDR after input gap")
	}

	recoverStream := append(append(append(append([]byte{}, sps...), pps...), idr...), pSlice3...)
	recoverStream = append(recoverStream, pSlice1...)
	app.onCustomByteBlock(&rmcp.CustomByteBlock{Data: recoverStream})
	if len(player.enqueuedFrames) != 2 {
		t.Fatalf("expected fresh IDR after gap to recover output, got=%d", len(player.enqueuedFrames))
	}
}

func TestBuildVideoStatePayloadSeparatesOfficialAndCustomLatency(t *testing.T) {
	now := time.Now()
	app := NewApp()
	app.setActiveVideoSource(videoSourceCustom)
	app.noteCustomByteBlockReceipt(now.Add(-2 * time.Second).UnixMilli())
	app.setVideoPlayerState(videoSourceOfficial, &fakeVideoPlayer{
		snapshot: video.PlayerSnapshot{
			LastPresentAt:    now.Add(-20 * time.Millisecond).UnixMilli(),
			WindowLayerReady: true,
		},
	}, "")

	payload := app.buildVideoStatePayload()
	if payload.OfficialLatencyMs <= 0 || payload.OfficialLatencyMs > 200 {
		t.Fatalf("unexpected official latency: %d", payload.OfficialLatencyMs)
	}
	if payload.CustomLatencyMs < 1500 {
		t.Fatalf("custom latency should reflect stale custom block age, got=%d", payload.CustomLatencyMs)
	}
	if payload.LatencyMs != payload.CustomLatencyMs {
		t.Fatalf("active latency should stay tied to custom source, got active=%d custom=%d", payload.LatencyMs, payload.CustomLatencyMs)
	}
}

func TestOnCustomByteBlockMarksCustomSourceAvailableBeforeKeyframe(t *testing.T) {
	// What: 构造一包已经到达客户端、但内容还不足以重组成完整 AU 的 0x0310 数据。
	// Why: 现场最常见的联调阶段正是“链路已通，但还在等首个关键帧”；前端必须明确显示“已收到数据，等待关键帧”，不能误报成完全断流。
	app := NewApp()
	player := &fakeVideoPlayer{}
	app.setActiveVideoSource(videoSourceCustom)
	app.setVideoPlayerState(videoSourceCustom, player, "")

	incompleteBlock := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x55}, 294)...)
	app.onCustomByteBlock(&rmcp.CustomByteBlock{Data: incompleteBlock})

	if len(player.enqueuedFrames) != 0 {
		t.Fatalf("incomplete pre-sync block should not enqueue decoded AU")
	}

	payload := app.buildVideoStatePayload()
	if !payload.CustomAvailable {
		t.Fatalf("custom source should become available immediately after receiving 0x0310 data")
	}
	if payload.DisplayState != videoDisplayStateResyncing {
		t.Fatalf("unexpected display state before keyframe: %s", payload.DisplayState)
	}
	if payload.Message != "已收到 0x0310 视频数据，等待关键帧恢复出画" {
		t.Fatalf("unexpected pre-keyframe message: %s", payload.Message)
	}
}

func TestGetClientConfigInjectsRuntimeMQTTRobotIdentityWhenConfigMissing(t *testing.T) {
	// What: 把用户配置目录切到测试临时目录并写入一份不含 mqttRobotIdentity 的旧配置。
	// Why: 真实问题就发生在历史配置升级后缺字段，前端 hydrate 会误把运行时红方身份覆盖回默认蓝方。
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.SaveClientConfigJSON(`{"version":3,"uiPanel":{"videoSource":"official"}}`); err != nil {
		t.Fatalf("save legacy config failed: %v", err)
	}

	app := NewApp()
	app.configureRMIdentity(1)

	configJSON, err := app.GetClientConfig()
	if err != nil {
		t.Fatalf("get client config failed: %v", err)
	}

	var snapshot struct {
		UIPanel struct {
			VideoSource       string `json:"videoSource"`
			MQTTRobotIdentity string `json:"mqttRobotIdentity"`
			MQTTHeroIdentity  string `json:"mqttHeroIdentity"`
		} `json:"uiPanel"`
	}

	// What: 解析后端返回的配置快照。
	// Why: 需要同时确认“原字段仍保留”和“缺失的 mqttRobotIdentity 已被按运行时真实身份补齐”。
	if err := json.Unmarshal([]byte(configJSON), &snapshot); err != nil {
		t.Fatalf("parse returned config failed: %v", err)
	}
	if snapshot.UIPanel.VideoSource != "official" {
		t.Fatalf("unexpected videoSource after injection: %s", snapshot.UIPanel.VideoSource)
	}
	if snapshot.UIPanel.MQTTRobotIdentity != mqttHeroIdentityRedHero {
		t.Fatalf("unexpected injected mqttRobotIdentity: %s", snapshot.UIPanel.MQTTRobotIdentity)
	}
	if snapshot.UIPanel.MQTTHeroIdentity != mqttHeroIdentityRedHero {
		t.Fatalf("unexpected mirrored mqttHeroIdentity: %s", snapshot.UIPanel.MQTTHeroIdentity)
	}
}

func TestGetClientConfigPreservesExplicitMQTTRobotIdentity(t *testing.T) {
	// What: 写入一份已经显式保存蓝方步兵 4 的配置。
	// Why: 用户在客户端里主动选择过身份后，后端返回配置时必须尊重该显式值，不能被当前运行时临时身份覆盖。
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.SaveClientConfigJSON(`{"version":4,"uiPanel":{"videoSource":"official","mqttRobotIdentity":"blue_infantry_4","mqttHeroIdentity":"blue_infantry_4"}}`); err != nil {
		t.Fatalf("save explicit config failed: %v", err)
	}

	app := NewApp()
	app.configureRMIdentity(1)

	configJSON, err := app.GetClientConfig()
	if err != nil {
		t.Fatalf("get client config failed: %v", err)
	}

	var snapshot struct {
		UIPanel struct {
			MQTTRobotIdentity string `json:"mqttRobotIdentity"`
			MQTTHeroIdentity  string `json:"mqttHeroIdentity"`
		} `json:"uiPanel"`
	}

	// What: 解析返回配置并只检查身份字段。
	// Why: 这条用例重点锁住“显式配置优先级高于运行时注入”的行为。
	if err := json.Unmarshal([]byte(configJSON), &snapshot); err != nil {
		t.Fatalf("parse returned config failed: %v", err)
	}
	if snapshot.UIPanel.MQTTRobotIdentity != "blue_infantry_4" {
		t.Fatalf("explicit mqttRobotIdentity should be preserved, got=%s", snapshot.UIPanel.MQTTRobotIdentity)
	}
	if snapshot.UIPanel.MQTTHeroIdentity != "blue_infantry_4" {
		t.Fatalf("mirrored mqttHeroIdentity should be preserved, got=%s", snapshot.UIPanel.MQTTHeroIdentity)
	}
}

func TestResolveMQTTBrokerConfigUsesEnvironmentOverride(t *testing.T) {
	t.Setenv("RM_MQTT_HOST", "127.0.0.1")
	t.Setenv("RM_MQTT_PORT", "3333")

	config := resolveMQTTBrokerConfig()
	if config.host != "127.0.0.1" {
		t.Fatalf("unexpected mqtt host override: %s", config.host)
	}
	if config.port != 3333 {
		t.Fatalf("unexpected mqtt port override: %d", config.port)
	}
}

func TestResolveMQTTBrokerConfigRejectsInvalidPort(t *testing.T) {
	t.Setenv("RM_MQTT_HOST", "127.0.0.1")
	t.Setenv("RM_MQTT_PORT", "70000")

	config := resolveMQTTBrokerConfig()
	if config.host != "127.0.0.1" {
		t.Fatalf("unexpected mqtt host after invalid port fallback: %s", config.host)
	}
	if config.port != defaultMQTTBrokerPort {
		t.Fatalf("unexpected mqtt port fallback: %d", config.port)
	}
}

// extractSidePayload 从 buildMatchStatePayload 的匿名 map 中取出单侧数据。
// What: 把测试里的类型断言收口到一处。
// Why: 这样每条用例都可以聚焦“映射结果是否正确”，不用重复写样板断言。
func extractSidePayload(t *testing.T, payload map[string]interface{}, side string) map[string]interface{} {
	t.Helper()

	rawSide, ok := payload[side]
	if !ok {
		t.Fatalf("missing %s side payload", side)
	}

	sidePayload, ok := rawSide.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected %s side payload type: %T", side, rawSide)
	}

	return sidePayload
}

// extractUnitPayload 从单侧 units 列表中取指定索引项。
// What: 把队伍卡测试里的类型断言与边界检查统一收口。
// Why: 顶部编组映射要同时校验 robot_id、hp、max_hp 和 online，拆成小辅助函数后可读性更高。
func extractUnitPayload(t *testing.T, sidePayload map[string]interface{}, index int) map[string]interface{} {
	t.Helper()

	rawUnits, ok := sidePayload["units"]
	if !ok {
		t.Fatalf("missing units payload")
	}

	units, ok := rawUnits.([]map[string]interface{})
	if !ok {
		t.Fatalf("unexpected units payload type: %T", rawUnits)
	}

	if index < 0 || index >= len(units) {
		t.Fatalf("unit index out of range: %d", index)
	}

	return units[index]
}

func TestBuildCombatStatePayloadUsesDynamicHealthAndStaticMaxHealth(t *testing.T) {
	// What: 构造一份本机动态血量与静态上限同时到位的缓存。
	// Why: 这条用例锁住“hp 来自动态状态、max_hp 来自静态状态”的核心对齐关系。
	app := NewApp()
	app.configureRMIdentity(101)
	app.cacheRobotDynamicStatus(&rmcp.RobotDynamicStatus{
		CurrentHealth: 118,
		RemainingAmmo: 42,
		CurrentHeat:   13.5,
		CanRemoteAmmo: true,
	})
	app.cacheRobotStaticStatus(&rmcp.RobotStaticStatus{
		RobotId:   101,
		MaxHealth: 250,
	})

	payload, ok := app.buildCombatStatePayload()
	if !ok {
		t.Fatalf("combat payload should be built")
	}
	if payload["hp"] != uint32(118) {
		t.Fatalf("unexpected hp: %#v", payload["hp"])
	}
	if payload["max_hp"] != uint32(250) {
		t.Fatalf("unexpected max_hp: %#v", payload["max_hp"])
	}
	if payload["ammo"] != uint32(42) {
		t.Fatalf("unexpected ammo: %#v", payload["ammo"])
	}
	if payload["can_remote_ammo"] != true {
		t.Fatalf("unexpected can_remote_ammo: %#v", payload["can_remote_ammo"])
	}
}

func TestBuildMatchStatePayloadMapsBlueClientPerspective(t *testing.T) {
	// What: 用蓝方英雄 clientID 构造 allied/enemy 视角的全局血量缓存。
	// Why: 真实比赛里 GlobalUnitStatus 的 base_health 永远代表“己方”，蓝方客户端若不重排就会把红蓝显示反；同时还要确认官方 GameStatus 字段会和血量编组一起下发。
	app := NewApp()
	app.configureRMIdentity(101)
	app.cacheGameStatus(&rmcp.GameStatus{
		CurrentRound:      2,
		TotalRounds:       5,
		RedScore:          1,
		BlueScore:         3,
		CurrentStage:      4,
		StageCountdownSec: 123,
		StageElapsedSec:   177,
		IsPaused:          false,
	})
	app.cacheGlobalUnitStatus(&rmcp.GlobalUnitStatus{
		BaseHealth:         4800,
		BaseStatus:         1,
		BaseShield:         220,
		OutpostHealth:      1500,
		OutpostStatus:      2,
		EnemyBaseHealth:    4300,
		EnemyBaseStatus:    3,
		EnemyBaseShield:    180,
		EnemyOutpostHealth: 1200,
		EnemyOutpostStatus: 4,
		RobotHealth:        []uint32{110, 120, 130, 140, 170, 210, 220, 230, 240, 270},
		TotalDamageAlly:    3600,
		TotalDamageEnemy:   2900,
	})
	app.cacheRobotDynamicStatus(&rmcp.RobotDynamicStatus{
		CurrentHealth: 115,
	})
	app.cacheRobotStaticStatus(&rmcp.RobotStaticStatus{
		RobotId:         101,
		MaxHealth:       250,
		ConnectionState: 1,
		FieldState:      0,
	})
	app.cacheRobotStaticStatus(&rmcp.RobotStaticStatus{
		RobotId:         1,
		MaxHealth:       450,
		ConnectionState: 1,
		FieldState:      0,
	})

	payload, ok := app.buildMatchStatePayload()
	if !ok {
		t.Fatalf("match payload should be built")
	}

	redSide := extractSidePayload(t, payload, "red")
	blueSide := extractSidePayload(t, payload, "blue")
	if redSide["base_hp"] != uint32(4300) {
		t.Fatalf("unexpected red base hp: %#v", redSide["base_hp"])
	}
	if blueSide["base_hp"] != uint32(4800) {
		t.Fatalf("unexpected blue base hp: %#v", blueSide["base_hp"])
	}
	if redSide["base_max_hp"] != baseMaxHealth {
		t.Fatalf("unexpected red base max hp: %#v", redSide["base_max_hp"])
	}
	if blueSide["base_max_hp"] != baseMaxHealth {
		t.Fatalf("unexpected blue base max hp: %#v", blueSide["base_max_hp"])
	}
	if payload["global_status_ready"] != true {
		t.Fatalf("unexpected global_status_ready: %#v", payload["global_status_ready"])
	}
	if redSide["outpost_hp"] != uint32(1200) {
		t.Fatalf("unexpected red outpost hp: %#v", redSide["outpost_hp"])
	}
	if blueSide["outpost_hp"] != uint32(1500) {
		t.Fatalf("unexpected blue outpost hp: %#v", blueSide["outpost_hp"])
	}
	if redSide["total_damage"] != uint32(2900) {
		t.Fatalf("unexpected red total damage: %#v", redSide["total_damage"])
	}
	if blueSide["total_damage"] != uint32(3600) {
		t.Fatalf("unexpected blue total damage: %#v", blueSide["total_damage"])
	}
	if payload["game_status_ready"] != true {
		t.Fatalf("unexpected game_status_ready: %#v", payload["game_status_ready"])
	}
	if payload["current_round"] != uint32(2) {
		t.Fatalf("unexpected current_round: %#v", payload["current_round"])
	}
	if payload["total_rounds"] != uint32(5) {
		t.Fatalf("unexpected total_rounds: %#v", payload["total_rounds"])
	}
	if payload["red_score"] != uint32(1) {
		t.Fatalf("unexpected red_score: %#v", payload["red_score"])
	}
	if payload["blue_score"] != uint32(3) {
		t.Fatalf("unexpected blue_score: %#v", payload["blue_score"])
	}
	if payload["current_stage"] != uint32(4) {
		t.Fatalf("unexpected current_stage: %#v", payload["current_stage"])
	}
	if payload["stage_countdown_sec"] != int32(123) {
		t.Fatalf("unexpected stage_countdown_sec: %#v", payload["stage_countdown_sec"])
	}
	if payload["stage_elapsed_sec"] != int32(177) {
		t.Fatalf("unexpected stage_elapsed_sec: %#v", payload["stage_elapsed_sec"])
	}
	if payload["is_paused"] != false {
		t.Fatalf("unexpected is_paused: %#v", payload["is_paused"])
	}

	redHero := extractUnitPayload(t, redSide, 0)
	blueHero := extractUnitPayload(t, blueSide, 0)
	if redHero["robot_id"] != uint32(1) {
		t.Fatalf("unexpected red hero robot_id: %#v", redHero["robot_id"])
	}
	if redHero["hp"] != uint32(210) {
		t.Fatalf("unexpected red hero hp: %#v", redHero["hp"])
	}
	if redHero["max_hp"] != uint32(450) {
		t.Fatalf("unexpected red hero max_hp: %#v", redHero["max_hp"])
	}
	if blueHero["hp"] != uint32(115) {
		t.Fatalf("unexpected blue hero hp: %#v", blueHero["hp"])
	}
	if blueHero["max_hp"] != uint32(250) {
		t.Fatalf("unexpected blue hero max_hp: %#v", blueHero["max_hp"])
	}
}

func TestBuildMatchStatePayloadMapsRedClientPerspective(t *testing.T) {
	// What: 用红方英雄 clientID 构造 allied/enemy 视角的全局血量缓存。
	// Why: 这条用例锁住红方视角下不应额外翻转映射，防止修蓝方时把红方链路顺手改坏。
	app := NewApp()
	app.configureRMIdentity(1)
	app.cacheGlobalUnitStatus(&rmcp.GlobalUnitStatus{
		BaseHealth:         4600,
		BaseStatus:         5,
		BaseShield:         300,
		OutpostHealth:      1800,
		OutpostStatus:      6,
		EnemyBaseHealth:    4200,
		EnemyBaseStatus:    7,
		EnemyBaseShield:    140,
		EnemyOutpostHealth: 1600,
		EnemyOutpostStatus: 8,
		RobotHealth:        []uint32{101, 102, 103, 104, 107, 201, 202, 203, 204, 207},
		TotalDamageAlly:    5100,
		TotalDamageEnemy:   4300,
	})

	payload, ok := app.buildMatchStatePayload()
	if !ok {
		t.Fatalf("match payload should be built")
	}

	redSide := extractSidePayload(t, payload, "red")
	blueSide := extractSidePayload(t, payload, "blue")
	if redSide["base_hp"] != uint32(4600) {
		t.Fatalf("unexpected red base hp: %#v", redSide["base_hp"])
	}
	if blueSide["base_hp"] != uint32(4200) {
		t.Fatalf("unexpected blue base hp: %#v", blueSide["base_hp"])
	}
	if redSide["outpost_hp"] != uint32(1800) {
		t.Fatalf("unexpected red outpost hp: %#v", redSide["outpost_hp"])
	}
	if blueSide["outpost_hp"] != uint32(1600) {
		t.Fatalf("unexpected blue outpost hp: %#v", blueSide["outpost_hp"])
	}
	if redSide["total_damage"] != uint32(5100) {
		t.Fatalf("unexpected red total damage: %#v", redSide["total_damage"])
	}
	if blueSide["total_damage"] != uint32(4300) {
		t.Fatalf("unexpected blue total damage: %#v", blueSide["total_damage"])
	}

	redHero := extractUnitPayload(t, redSide, 0)
	blueHero := extractUnitPayload(t, blueSide, 0)
	if redHero["hp"] != uint32(101) {
		t.Fatalf("unexpected red hero hp: %#v", redHero["hp"])
	}
	if blueHero["hp"] != uint32(201) {
		t.Fatalf("unexpected blue hero hp: %#v", blueHero["hp"])
	}
}

func TestBuildMatchStatePayloadSupportsGameStatusOnly(t *testing.T) {
	// What: 只缓存官方 GameStatus，不放任何基地或编组血量。
	// Why: 顶部中间条必须能在双方血量 topic 尚未到达前先显示真实局次、比分和阶段，不能再因为缺少 GlobalUnitStatus 整体失效。
	app := NewApp()
	app.cacheGameStatus(&rmcp.GameStatus{
		CurrentRound:      1,
		TotalRounds:       3,
		RedScore:          0,
		BlueScore:         2,
		CurrentStage:      3,
		StageCountdownSec: 5,
		StageElapsedSec:   10,
		IsPaused:          true,
	})

	payload, ok := app.buildMatchStatePayload()
	if !ok {
		t.Fatalf("match payload should be built from game status only")
	}
	if payload["game_status_ready"] != true {
		t.Fatalf("unexpected game_status_ready: %#v", payload["game_status_ready"])
	}
	if payload["current_round"] != uint32(1) {
		t.Fatalf("unexpected current_round: %#v", payload["current_round"])
	}
	if payload["total_rounds"] != uint32(3) {
		t.Fatalf("unexpected total_rounds: %#v", payload["total_rounds"])
	}
	if payload["red_score"] != uint32(0) {
		t.Fatalf("unexpected red_score: %#v", payload["red_score"])
	}
	if payload["blue_score"] != uint32(2) {
		t.Fatalf("unexpected blue_score: %#v", payload["blue_score"])
	}
	if payload["current_stage"] != uint32(3) {
		t.Fatalf("unexpected current_stage: %#v", payload["current_stage"])
	}
	if payload["stage_countdown_sec"] != int32(5) {
		t.Fatalf("unexpected stage_countdown_sec: %#v", payload["stage_countdown_sec"])
	}
	if payload["stage_elapsed_sec"] != int32(10) {
		t.Fatalf("unexpected stage_elapsed_sec: %#v", payload["stage_elapsed_sec"])
	}
	if payload["is_paused"] != true {
		t.Fatalf("unexpected is_paused: %#v", payload["is_paused"])
	}
	if _, exists := payload["red"]; exists {
		t.Fatalf("unexpected red side payload when global status is absent")
	}
	if _, exists := payload["blue"]; exists {
		t.Fatalf("unexpected blue side payload when global status is absent")
	}
}
