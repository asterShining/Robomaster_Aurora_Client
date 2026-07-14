package video

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

// nativeWindowLayerPolicy 描述当前桌面环境下，ffplay 独立窗口该如何被管理。
// What: 把 Wayland/X11 的分支判断收口成一份只读策略。
// Why: 这样播放器启动路径只关心“要不要强制走 X11、要不要做层级控制”，不用把环境判断散在多个函数里。
type nativeWindowLayerPolicy struct {
	sessionType       string
	hasDisplay        bool
	enableX11Stacking bool
	forceSDLX11       bool
	reason            string
}

// x11WindowGeometry 描述某个 X11 顶层窗在桌面根窗口坐标系里的绝对几何信息。
// What: 将窗口左上角绝对坐标与逻辑尺寸收口到一个结构里。
// Why: 多屏环境下，GTK/Wails 返回的窗口坐标可能只是“相对当前屏”，而 ffplay 需要的是整个桌面空间里的绝对坐标。
type x11WindowGeometry struct {
	Left   int
	Top    int
	Width  int
	Height int
}

// x11WindowCandidate 描述当前桌面上一个可候选的顶层窗。
// What: 将窗口 ID、PID、标题、几何和 map 状态收口到同一结构。
// Why: HUD 与 ffplay 的选窗规则现在要求同时比较“是不是同一进程”“标题是否精确命中”“是否真的可见”“面积谁更大”，分散在多个局部变量里很容易漏判。
type x11WindowCandidate struct {
	windowID xproto.Window
	pid      uint32
	pidKnown bool
	title    string
	wmClass  string
	source   string
	geometry x11WindowGeometry
	mapState byte
}

type nativeVideoWindowSearchMode string

const (
	nativeVideoWindowSearchModeOfficial nativeVideoWindowSearchMode = "official"
	nativeVideoWindowSearchModeCustom   nativeVideoWindowSearchMode = "custom"
)

const (
	// windowGeometryPositionTolerancePx 定义窗口左上角位置允许的最大漂移。
	// What: 给 left/top 回读比较留出一个极小容差。
	// Why: XWayland/WM 在首次映射后经常会引入 1-8px 的边框补偿或对齐抖动；若继续要求逐像素完全一致，就会把其实已经能正常出画的 ffplay 误判成失败。
	windowGeometryPositionTolerancePx = 8

	// windowGeometrySizeTolerancePx 定义窗口宽高允许的最大漂移。
	// What: 给 width/height 回读比较留出略大于位置的容差。
	// Why: 某些窗口管理器会在无边框窗上额外做缩放取整或设备像素比修正，尺寸偏差通常略大于位置偏差；这里必须单独放宽，避免轻微 rounding 被当成严重错位。
	windowGeometrySizeTolerancePx = 16

	// nativeWindowSearchPositionTolerancePx 定义候选视频窗在“搜索阶段”允许偏离目标布局的位置阈值。
	// What: 给还未完成 sibling 压层前的原生视频窗一个比最终几何校验更宽的 left/top 搜索容差。
	// Why: 运行时真正的问题是“窗口找不到”，而不是“窗口回读不够严格”；搜索阶段必须允许较宽的初始偏差，才能把迟到映射或被 WM 轻微挪动的 ffplay 窗抓出来再二次贴合。
	nativeWindowSearchPositionTolerancePx = 96

	// nativeWindowSearchSizeTolerancePx 定义候选视频窗在“搜索阶段”允许偏离目标布局的尺寸阈值。
	// What: 给尚未完成贴合的视频窗一个比最终几何校验更宽的 width/height 搜索容差。
	// Why: 某些桌面环境会先给 SDL/XWayland 窗一个中间尺寸，再在后续重排中收敛到目标值；若搜索阶段也要求最终精度，就会把正确窗口提前排除掉。
	nativeWindowSearchSizeTolerancePx = 192

	// nativeWindowSearchLogLimit 限制单次窗口诊断日志里最多展开的候选窗数量。
	// What: 只记录最有价值的前若干个候选窗摘要。
	// Why: 整个 X11 窗树里可能有大量无关窗口；若把所有候选一股脑打进日志，现场排障反而会被噪声淹没。
	nativeWindowSearchLogLimit = 8
)

// x11AtomSet 收口本轮窗口控制会用到的 EWMH/X11 atoms。
// What: 把多次复用的 atom 句柄缓存到一起。
// Why: 原生窗口下压会连续查询 client list、读取 PID/标题并发送 WM_STATE 请求，若每一步都重新字符串查 atom，会让启动阶段更慢更脆。
type x11AtomSet struct {
	netClientList         xproto.Atom
	netClientListStacking xproto.Atom
	netWMName             xproto.Atom
	netWMPid              xproto.Atom
	netWMState            xproto.Atom
	netWMStateAbove       xproto.Atom
	netWMStateBelow       xproto.Atom
	netWMStateSkipTaskbar xproto.Atom
	netWMStateSkipPager   xproto.Atom
	utf8String            xproto.Atom
}

// detectNativeWindowLayerPolicy 读取当前进程环境，推导播放器原生窗口策略。
// What: 将会话类型、DISPLAY 暴露情况与 ffplay 期望 backend 一次性归一化。
// Why: 用户当前就是在 Wayland 会话里跑 HUD，若这里不明确策略，后面很容易继续拿 X11 语义去猜纯 Wayland 行为。
func detectNativeWindowLayerPolicy() nativeWindowLayerPolicy {
	return decideNativeWindowLayerPolicy(
		os.Getenv("XDG_SESSION_TYPE"),
		os.Getenv("DISPLAY"),
	)
}

// decideNativeWindowLayerPolicy 根据给定环境值生成窗口策略。
// What: 纯函数版本的策略推导逻辑。
// Why: 这样既方便单元测试，也避免测试时依赖宿主机真实会话环境。
func decideNativeWindowLayerPolicy(sessionType string, display string) nativeWindowLayerPolicy {
	policy := nativeWindowLayerPolicy{
		sessionType: strings.ToLower(strings.TrimSpace(sessionType)),
		hasDisplay:  strings.TrimSpace(display) != "",
	}

	if !policy.hasDisplay {
		// What: 没有 DISPLAY 时直接判定为无法做 X11 层级控制。
		// Why: 当前后端的窗口下压能力完全建立在 X11/XWayland 之上，纯 Wayland 且无 DISPLAY 的场景必须坦诚降级。
		policy.reason = "当前会话未暴露 DISPLAY，ffplay 只能作为普通窗口运行，无法稳定压在 HUD 主窗下方"
		return policy
	}

	policy.enableX11Stacking = true

	if policy.sessionType == "wayland" {
		// What: Wayland 但同时存在 DISPLAY 时，强制 ffplay 走 XWayland。
		// Why: 这样后端才能通过 X11/EWMH 把视频窗锁到 HUD 主窗正下方；否则 ffplay 若走原生 Wayland，单独窗口层级几乎不可控。
		policy.forceSDLX11 = true
		policy.reason = "检测到 Wayland + XWayland，将强制 ffplay 使用 X11 backend，并在映射后压到 HUD 主窗下方"
		return policy
	}

	// What: 其它存在 DISPLAY 的场景默认按 X11 栈处理。
	// Why: 无论是原生 X11 还是 XWayland，后端都可以用同一套 EWMH 请求把视频层收敛到 HUD 主窗下方。
	policy.reason = "检测到可用 X11 显示链，ffplay 启动后会主动压到 HUD 主窗下方"
	return policy
}

// ResolveX11WindowLayout 解析给定顶层窗在桌面坐标系中的绝对布局。
// What: 按 PID 与窗口标题锁定目标窗，并将其几何信息与顶层窗 ID 一起转换为播放器可直接复用的绝对布局。
// Why: 官方视频层必须和 HUD 主窗严格落在同一块显示区域，多屏下若继续依赖相对坐标或丢掉 HUD 窗身份，ffplay 就会跑到错误屏幕或压在错误窗口下。
func ResolveX11WindowLayout(pid int, title string) (VideoWindowLayout, error) {
	if pid <= 0 {
		return VideoWindowLayout{}, fmt.Errorf("invalid pid: %d", pid)
	}

	policy := detectNativeWindowLayerPolicy()
	if !policy.enableX11Stacking {
		// What: 没有可用 X11 显示链时直接返回不可控错误。
		// Why: 当前官方视频层的几何解析与层级控制都建立在 X11/XWayland 之上，继续盲启只会回到“层级不可控”的老问题。
		return VideoWindowLayout{}, fmt.Errorf("x11 window control unavailable: %s", policy.reason)
	}

	conn, err := xgb.NewConn()
	if err != nil {
		return VideoWindowLayout{}, err
	}
	defer conn.Close()

	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)
	if screen == nil {
		return VideoWindowLayout{}, fmt.Errorf("x11 default screen unavailable")
	}

	atoms, err := internX11Atoms(conn)
	if err != nil {
		return VideoWindowLayout{}, err
	}

	// What: 先按 PID 与标题定位真正的 HUD 顶层窗。
	// Why: 同一进程后续还会拉起 ffplay 子窗，若只看 PID 或只看标题，窗口匹配都可能被误导。
	windowCandidate, err := findWindowByPIDAndTitle(conn, screen.Root, atoms, uint32(pid), title)
	if err != nil {
		return VideoWindowLayout{}, err
	}

	layout, err := videoWindowLayoutFromGeometry(windowCandidate.geometry)
	if err != nil {
		return VideoWindowLayout{}, err
	}
	layout.HUDWindowID = uint32(windowCandidate.windowID)
	return layout, nil
}

// videoWindowLayoutFromGeometry 将 X11 绝对几何信息转换为播放器布局。
// What: 把 root 坐标系里的窗口位置与逻辑尺寸原样映射到 ffplay 布局。
// Why: 这一步必须严格保留绝对 Left/Top；一旦偷偷归零，多屏下视频层就会重新跑回主屏左上角。
func videoWindowLayoutFromGeometry(geometry x11WindowGeometry) (VideoWindowLayout, error) {
	if geometry.Width <= 0 || geometry.Height <= 0 {
		return VideoWindowLayout{}, fmt.Errorf("x11 window geometry invalid width=%d height=%d", geometry.Width, geometry.Height)
	}

	return VideoWindowLayout{
		Left:   geometry.Left,
		Top:    geometry.Top,
		Width:  geometry.Width,
		Height: geometry.Height,
	}, nil
}

// applyX11WindowLayer 等待 ffplay 顶层窗出现，并把它压到 HUD 窗口正下方。
// What: 在独立 goroutine 里轮询查找视频窗与 HUD 主窗，再发送基于 sibling 的局部重排请求。
// Why: 用户现在要的是“视频只停留在本客户端 HUD 下方”，而不是把 ffplay 抬成整个桌面的高层窗。
func (p *NativePlayer) applyX11WindowLayer(pid int) {
	if pid <= 0 {
		return
	}

	if !p.windowLayerPolicy.enableX11Stacking {
		return
	}

	conn, err := xgb.NewConn()
	if err != nil {
		log.Printf("[Warning] [video] Open X11 display failed, skip native window layering: %v", err)
		return
	}
	defer conn.Close()

	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)
	if screen == nil {
		log.Printf("[Warning] [video] X11 default screen unavailable, skip native window layering")
		return
	}

	atoms, err := internX11Atoms(conn)
	if err != nil {
		log.Printf("[Warning] [video] Resolve X11 atoms failed, skip native window layering: %v", err)
		return
	}

	root := screen.Root
	lastLoggedHardErr := ""

	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		// What: 每次重试前都先确认当前 goroutine 盯的还是这一轮 ffplay 进程。
		// Why: 播放器重启后旧 goroutine 可能尚未退出；若不先做代际校验，它就会把过期结论覆盖到新进程状态上。
		if !p.isCurrentProcessPID(pid) {
			return
		}

		targetLayout := p.Layout()
		hudWindowID := xproto.Window(targetLayout.HUDWindowID)
		if hudWindowID == 0 {
			layerErr := "原生视频窗未能贴合 HUD，已阻止透出桌面: HUD 主窗身份缺失"
			p.setWindowLayerState(0, false, layerErr)
			if lastLoggedHardErr != layerErr {
				log.Printf("[Warning] [video] %s", layerErr)
				lastLoggedHardErr = layerErr
			}
			return
		}

		searchResult, err := applyX11WindowLayerOnce(conn, root, atoms, uint32(pid), p.windowTitle, p.windowSearchMode, hudWindowID, targetLayout)
		if err == nil {
			// What: 成功贴合后再次确认当前进程代际，然后才写回 ready。
			// Why: 这样可以避免“旧进程刚成功贴窗，但新进程已经启动”的极端竞态把新进程误判成 ready。
			if !p.isCurrentProcessPID(pid) {
				return
			}

			log.Printf(
				"[video] Native window stacked directly below HUD pid=%d window=0x%08x hud=0x%08x strategy=%s source=%s",
				pid,
				uint32(searchResult.candidate.windowID),
				uint32(hudWindowID),
				searchResult.strategy,
				searchResult.candidate.source,
			)
			p.setWindowLayerState(uint32(searchResult.candidate.windowID), true, "")
			return
		}

		// What: 失败后仅在“当前进程已经稳定出帧一段时间”的前提下才升级成硬错误。
		// Why: ffplay 首帧前往往根本还没有可抓取顶层窗；这段时间应该继续停留在等待态，而不是提前把前端打成 runtime_error。
		layerErr := p.currentWindowLayerRuntimeError(err, time.Now())
		if !p.isCurrentProcessPID(pid) {
			return
		}
		p.setWindowLayerState(uint32(searchResult.candidate.windowID), false, layerErr)
		if layerErr != "" && layerErr != lastLoggedHardErr {
			log.Printf("[Warning] [video] Failed to stack ffplay window below HUD pid=%d: %v", pid, err)
			logWindowSearchDiagnostics(pid, searchResult, targetLayout, err)
			lastLoggedHardErr = layerErr
		}
		if layerErr == "" {
			lastLoggedHardErr = ""
		}

		time.Sleep(windowLayerSettleRetryInterval)
	}
}

// x11WindowSearchResult 收口一次原生视频窗搜索得到的关键结果。
// What: 同时保存最终命中的候选窗、搜索策略名和本轮收集到的候选窗摘要。
// Why: 贴窗失败时需要把“为什么没选中”和“现场到底有哪些窗”一起打进日志，仅靠单个 error 文案不足以支撑排障。
type x11WindowSearchResult struct {
	candidate  x11WindowCandidate
	candidates []x11WindowCandidate
	strategy   string
}

// applyX11WindowLayerOnce 对当前 ffplay 进程执行一次完整的贴窗尝试。
// What: 单次完成“找视频窗”“对齐到 HUD”“回读几何校验”三个动作，并以 error 表达失败原因。
// Why: 持续监督循环需要把“单次尝试”与“是否升级成硬错误”分离，否则每次失败都会被过早固化成终态异常。
func applyX11WindowLayerOnce(conn *xgb.Conn, root xproto.Window, atoms x11AtomSet, pid uint32, title string, searchMode nativeVideoWindowSearchMode, hudWindowID xproto.Window, targetLayout VideoWindowLayout) (x11WindowSearchResult, error) {
	// What: 先按“严格命中 + 保守回退”的搜索策略锁定最可能的 ffplay 原生窗。
	// Why: 当前真正的问题是窗口发现阶段把候选窗漏掉；只有把搜索结果显式收口，后续贴层失败时才能知道究竟是“没找到”还是“找错了”。
	searchResult, err := findNativeVideoWindow(conn, root, atoms, pid, title, searchMode, targetLayout)
	if err != nil {
		return searchResult, err
	}

	if hudWindowID == searchResult.candidate.windowID {
		// What: HUD 与视频若意外命中同一个顶层窗，直接视为失败。
		// Why: 这种情况下 sibling 重排已经失去意义，继续下发只会把错误状态进一步放大。
		return searchResult, fmt.Errorf("hud window unexpectedly matched video window 0x%08x", uint32(searchResult.candidate.windowID))
	}

	// What: 先把视频窗压到 HUD 正下方并对齐目标几何。
	// Why: 用户要的是”HUD 下面只能看到视频”，因此层级和位置两个条件必须同时满足，缺一不可。
	if err := applyWindowBelowHUD(conn, root, searchResult.candidate.windowID, hudWindowID, targetLayout, atoms); err != nil {
		// What: 叠层失败时回退检查几何是否已到位。
		// Why: XWayland 下 KWin 可能拒绝 ConfigureWindow 的 sibling 叠层请求 (BadMatch)，
		// 但 ffplay 启动参数已经设好了窗口位置和尺寸；若几何无误，视频实际已在渲染，不应继续反复报错。
		actualGeometry, geoErr := readAbsoluteWindowGeometry(conn, root, searchResult.candidate.windowID)
		if geoErr != nil {
			return searchResult, err
		}
		if windowGeometryMatchesLayout(actualGeometry, targetLayout) {
			log.Printf(
				"[video] Window stacking failed but geometry matches - accepting as ready (window=0x%08x reason=%v)",
				uint32(searchResult.candidate.windowID),
				err,
			)
			return searchResult, nil
		}
		return searchResult, err
	}

	// What: 请求发出后立刻回读视频窗真实几何。
	// Why: 某些窗口管理器会接受 sibling 请求却忽略坐标修正；只有回读校验过，前端才能安全撤掉遮幕。
	actualGeometry, err := readAbsoluteWindowGeometry(conn, root, searchResult.candidate.windowID)
	if err != nil {
		return searchResult, err
	}

	if windowGeometryMatchesLayout(actualGeometry, targetLayout) {
		return searchResult, nil
	}

	deltaLeft := absInt(actualGeometry.Left - targetLayout.Left)
	deltaTop := absInt(actualGeometry.Top - targetLayout.Top)
	deltaWidth := absInt(actualGeometry.Width - targetLayout.Width)
	deltaHeight := absInt(actualGeometry.Height - targetLayout.Height)
	return searchResult, fmt.Errorf(
		"video window geometry mismatch actual=(%d,%d,%d,%d) target=(%d,%d,%d,%d) delta=(left=%d top=%d width=%d height=%d) tolerance=(pos=%d size=%d)",
		actualGeometry.Left,
		actualGeometry.Top,
		actualGeometry.Width,
		actualGeometry.Height,
		targetLayout.Left,
		targetLayout.Top,
		targetLayout.Width,
		targetLayout.Height,
		deltaLeft,
		deltaTop,
		deltaWidth,
		deltaHeight,
		windowGeometryPositionTolerancePx,
		windowGeometrySizeTolerancePx,
	)
}

// internX11Atoms 解析当前窗口控制会用到的一组 atoms。
// What: 一次性向 X server 请求所有关键 atom。
// Why: 这样既减少往返次数，也避免调用路径里反复写同样的字符串常量。
func internX11Atoms(conn *xgb.Conn) (x11AtomSet, error) {
	netClientList, err := internAtom(conn, "_NET_CLIENT_LIST")
	if err != nil {
		return x11AtomSet{}, err
	}

	netClientListStacking, err := internAtom(conn, "_NET_CLIENT_LIST_STACKING")
	if err != nil {
		return x11AtomSet{}, err
	}

	netWMName, err := internAtom(conn, "_NET_WM_NAME")
	if err != nil {
		return x11AtomSet{}, err
	}

	netWMPid, err := internAtom(conn, "_NET_WM_PID")
	if err != nil {
		return x11AtomSet{}, err
	}

	netWMState, err := internAtom(conn, "_NET_WM_STATE")
	if err != nil {
		return x11AtomSet{}, err
	}

	netWMStateAbove, err := internAtom(conn, "_NET_WM_STATE_ABOVE")
	if err != nil {
		return x11AtomSet{}, err
	}

	netWMStateBelow, err := internAtom(conn, "_NET_WM_STATE_BELOW")
	if err != nil {
		return x11AtomSet{}, err
	}

	netWMStateSkipTaskbar, err := internAtom(conn, "_NET_WM_STATE_SKIP_TASKBAR")
	if err != nil {
		return x11AtomSet{}, err
	}

	netWMStateSkipPager, err := internAtom(conn, "_NET_WM_STATE_SKIP_PAGER")
	if err != nil {
		return x11AtomSet{}, err
	}

	utf8String, err := internAtom(conn, "UTF8_STRING")
	if err != nil {
		return x11AtomSet{}, err
	}

	return x11AtomSet{
		netClientList:         netClientList,
		netClientListStacking: netClientListStacking,
		netWMName:             netWMName,
		netWMPid:              netWMPid,
		netWMState:            netWMState,
		netWMStateAbove:       netWMStateAbove,
		netWMStateBelow:       netWMStateBelow,
		netWMStateSkipTaskbar: netWMStateSkipTaskbar,
		netWMStateSkipPager:   netWMStateSkipPager,
		utf8String:            utf8String,
	}, nil
}

// internAtom 将 atom 名称解析为当前 X server 上的数值句柄。
// What: 对单个 atom 做一次 checked 查询。
// Why: 窗口管理器交互完全依赖 atom，任意一个拿不到都说明当前 X11 环境不完整，应当尽早退出而不是半吊子继续操作。
func internAtom(conn *xgb.Conn, name string) (xproto.Atom, error) {
	reply, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
	if err != nil {
		return xproto.AtomNone, err
	}
	return reply.Atom, nil
}

// findWindowByPIDAndTitle 在当前桌面顶层窗列表里找出“PID 与标题都精确命中”的可见主窗。
// What: 先枚举顶层候选，再用统一的“精确标题 + Viewable + 最大面积”规则做最终选择。
// Why: HUD 现在和若干 GTK/Wails 辅助窗共处同一进程，继续使用“同 PID 就先认”会反复命中错误窗口。
func findWindowByPIDAndTitle(conn *xgb.Conn, root xproto.Window, atoms x11AtomSet, pid uint32, title string) (x11WindowCandidate, error) {
	windowCandidates, err := collectX11WindowCandidates(conn, root, atoms)
	if err != nil {
		return x11WindowCandidate{}, err
	}

	return selectLargestVisibleWindowByPIDAndTitle(windowCandidates, pid, title)
}

// findNativeVideoWindow 为 ffplay 原生视频层搜索最可信的候选窗。
// What: 先把当前 X11 可读到的顶层窗与后代窗统一收集，再按“严格命中 -> 保守回退”的顺序挑出最佳候选。
// Why: 当前运行时问题不是“完全没有窗口”，而是“窗口发现条件过窄把真实 ffplay 窗漏掉”；必须把搜索与选择解耦，才能在不放弃安全性的前提下补回回退路径。
func findNativeVideoWindow(conn *xgb.Conn, root xproto.Window, atoms x11AtomSet, pid uint32, title string, searchMode nativeVideoWindowSearchMode, targetLayout VideoWindowLayout) (x11WindowSearchResult, error) {
	windowCandidates, err := collectX11WindowCandidates(conn, root, atoms)
	if err != nil {
		return x11WindowSearchResult{}, err
	}

	windowCandidate, strategy, err := selectNativeVideoWindowCandidateWithMode(windowCandidates, pid, title, targetLayout, searchMode)
	return x11WindowSearchResult{
		candidate:  windowCandidate,
		candidates: windowCandidates,
		strategy:   strategy,
	}, err
}

// x11WindowRef 记录一个待解析窗口及其发现来源。
// What: 将窗口 ID 和“它是从哪条枚举路径被发现的”一起保存。
// Why: 同一个窗口既可能出现在 `_NET_CLIENT_LIST`，也可能只在递归 `QueryTree` 里出现；排障时必须知道命中来源，才能判断问题卡在 WM 接管层还是树形枚举层。
type x11WindowRef struct {
	windowID xproto.Window
	source   string
}

// collectX11WindowCandidates 枚举当前桌面上可读到的窗口候选并补齐属性快照。
// What: 同时收集顶层窗和其后代窗，再读取 PID、标题、WM_CLASS、mapState 与绝对几何。
// Why: Wayland + XWayland 下 ffplay 真实可操作的窗口不一定直接出现在 `_NET_CLIENT_LIST*`；若不把后代窗一起纳入候选，实际存在的 SDL/XWayland 窗仍会被漏掉。
func collectX11WindowCandidates(conn *xgb.Conn, root xproto.Window, atoms x11AtomSet) ([]x11WindowCandidate, error) {
	windowRefs, err := collectX11WindowRefs(conn, root, atoms)
	if err != nil {
		return nil, err
	}

	windowCandidates := make([]x11WindowCandidate, 0, len(windowRefs))
	for _, windowRef := range windowRefs {
		windowPID, pidKnown, err := readWindowPID(conn, windowRef.windowID, atoms.netWMPid)
		if err != nil {
			continue
		}

		windowTitle, err := readWindowTitle(conn, windowRef.windowID, atoms)
		if err != nil {
			continue
		}

		windowWMClass, _, err := readWindowWMClass(conn, windowRef.windowID)
		if err != nil {
			continue
		}

		mapState, err := readWindowMapState(conn, windowRef.windowID)
		if err != nil {
			continue
		}

		geometry, err := readAbsoluteWindowGeometry(conn, root, windowRef.windowID)
		if err != nil {
			continue
		}

		windowCandidates = append(windowCandidates, x11WindowCandidate{
			windowID: windowRef.windowID,
			pid:      windowPID,
			pidKnown: pidKnown,
			title:    strings.TrimSpace(windowTitle),
			wmClass:  strings.TrimSpace(windowWMClass),
			source:   windowRef.source,
			geometry: geometry,
			mapState: mapState,
		})
	}

	return windowCandidates, nil
}

// collectX11WindowRefs 收集当前桌面上可进一步读取属性的窗口 ID。
// What: 先从 `_NET_CLIENT_LIST_STACKING` / `_NET_CLIENT_LIST` / root 直接子窗拿顶层引用，再递归展开其后代。
// Why: 某些桌面环境只把真正可操作的视频窗挂在顶层窗下面；若只扫顶层列表，ffplay 的实际 XWayland 子窗仍然会消失在搜索视野之外。
func collectX11WindowRefs(conn *xgb.Conn, root xproto.Window, atoms x11AtomSet) ([]x11WindowRef, error) {
	windowRefs := make([]x11WindowRef, 0, 32)
	seenWindowRefs := make(map[xproto.Window]struct{}, 32)

	// What: 用统一 add 函数保证每个窗口只记录一次，且保留最早的来源标签。
	// Why: 同一个窗口可能同时出现在 stacking list、client list 和 QueryTree 里；若不先去重，后续候选摘要会被重复信息淹没。
	addWindowRef := func(windowID xproto.Window, source string) bool {
		if windowID == 0 {
			return false
		}
		if _, duplicated := seenWindowRefs[windowID]; duplicated {
			return false
		}

		seenWindowRefs[windowID] = struct{}{}
		windowRefs = append(windowRefs, x11WindowRef{
			windowID: windowID,
			source:   source,
		})
		return true
	}

	stackingList, err := readWindowListProperty(conn, root, atoms.netClientListStacking)
	if err == nil {
		for _, windowID := range stackingList {
			addWindowRef(windowID, "ewmh-stacking")
		}
	}

	clientList, err := readWindowListProperty(conn, root, atoms.netClientList)
	if err == nil {
		for _, windowID := range clientList {
			addWindowRef(windowID, "ewmh-client-list")
		}
	}

	rootTreeReply, rootTreeErr := xproto.QueryTree(conn, root).Reply()
	if rootTreeErr == nil && rootTreeReply != nil {
		for _, windowID := range rootTreeReply.Children {
			addWindowRef(windowID, "root-child")
		}
	}

	if len(windowRefs) == 0 && rootTreeErr != nil {
		return nil, rootTreeErr
	}

	queue := append([]x11WindowRef(nil), windowRefs...)
	for index := 0; index < len(queue); index++ {
		parentRef := queue[index]
		treeReply, err := xproto.QueryTree(conn, parentRef.windowID).Reply()
		if err != nil || treeReply == nil {
			continue
		}

		for _, childWindowID := range treeReply.Children {
			// What: 统一把后代窗标记成 QueryTree 路径，并继续向下展开。
			// Why: 一旦 ffplay 真正的可操作窗藏在更深层，就必须完整递归到底，不能只停在一层 child。
			if !addWindowRef(childWindowID, "query-tree-descendant") {
				continue
			}
			queue = append(queue, x11WindowRef{
				windowID: childWindowID,
				source:   "query-tree-descendant",
			})
		}
	}

	return windowRefs, nil
}

// selectNativeVideoWindowCandidate 从候选窗里挑出最可信的 ffplay 视频窗。
// What: 依次尝试“严格 PID+标题”“标题回退”“PID+class 回退”“PID+几何回退”“class+几何回退”。
// Why: 用户当前故障的根本是运行时属性并不总能一次性全部到位；必须把强匹配和弱匹配分层，优先吃最可信的证据，再在必要时保守降级。
func selectNativeVideoWindowCandidate(candidates []x11WindowCandidate, pid uint32, title string, targetLayout VideoWindowLayout) (x11WindowCandidate, string, error) {
	return selectNativeVideoWindowCandidateWithMode(candidates, pid, title, targetLayout, nativeVideoWindowSearchModeOfficial)
}

func selectNativeVideoWindowCandidateWithMode(candidates []x11WindowCandidate, pid uint32, title string, targetLayout VideoWindowLayout, searchMode nativeVideoWindowSearchMode) (x11WindowCandidate, string, error) {
	windowCandidate, err := selectLargestVisibleWindowByPIDAndTitle(candidates, pid, title)
	if err == nil {
		return windowCandidate, "exact-pid-title", nil
	}

	if searchMode == nativeVideoWindowSearchModeOfficial {
		if windowCandidate, ok := selectClosestVisibleWindowCandidate(candidates, func(candidate x11WindowCandidate) bool {
			return !candidateTitleExplicitlyExcluded(candidate, searchMode) &&
				strings.TrimSpace(candidate.title) == strings.TrimSpace(title) &&
				(!candidate.pidKnown || candidate.pid == pid)
		}, targetLayout); ok {
			return windowCandidate, "exact-title-fallback", nil
		}
	}

	if searchMode == nativeVideoWindowSearchModeOfficial {
		if windowCandidate, ok := selectClosestVisibleWindowCandidate(candidates, func(candidate x11WindowCandidate) bool {
			return !candidateTitleExplicitlyExcluded(candidate, searchMode) &&
				candidate.pidKnown &&
				candidate.pid == pid &&
				candidateHasFFplayClass(candidate)
		}, targetLayout); ok {
			return windowCandidate, "pid-class-fallback", nil
		}
	}

	if windowCandidate, ok := selectClosestVisibleWindowCandidate(candidates, func(candidate x11WindowCandidate) bool {
		return !candidateTitleExplicitlyExcluded(candidate, searchMode) &&
			candidate.pidKnown &&
			candidate.pid == pid &&
			windowGeometryLooksLikeSearchTarget(candidate.geometry, targetLayout)
	}, targetLayout); ok {
		return windowCandidate, "pid-geometry-fallback", nil
	}

	if searchMode == nativeVideoWindowSearchModeOfficial {
		if windowCandidate, ok := selectClosestVisibleWindowCandidate(candidates, func(candidate x11WindowCandidate) bool {
			return !candidateTitleExplicitlyExcluded(candidate, searchMode) &&
				candidateHasFFplayClass(candidate) &&
				windowGeometryLooksLikeSearchTarget(candidate.geometry, targetLayout)
		}, targetLayout); ok {
			return windowCandidate, "class-geometry-fallback", nil
		}
	}

	return x11WindowCandidate{}, "", buildNativeVideoWindowSelectionError(candidates, pid, title, targetLayout)
}

func candidateTitleExplicitlyExcluded(candidate x11WindowCandidate, searchMode nativeVideoWindowSearchMode) bool {
	title := strings.TrimSpace(candidate.title)
	if title == "" {
		return false
	}

	switch searchMode {
	case nativeVideoWindowSearchModeOfficial:
		return title == nativeCustomVideoWindowTitle
	case nativeVideoWindowSearchModeCustom:
		return title == nativeOfficialVideoWindowTitle
	default:
		return false
	}
}

// selectClosestVisibleWindowCandidate 在满足给定谓词的候选窗里选出最接近目标布局的那个。
// What: 仅从可见、尺寸有效的候选窗里选择“几何距离最小；若并列则面积最大”的一个。
// Why: 保守回退阶段不能再依赖单一属性完全命中，因此需要用“最像目标视频窗”的几何特征做最终收敛。
func selectClosestVisibleWindowCandidate(candidates []x11WindowCandidate, predicate func(candidate x11WindowCandidate) bool, targetLayout VideoWindowLayout) (x11WindowCandidate, bool) {
	var best x11WindowCandidate
	bestFound := false
	bestDistance := int64(0)
	bestArea := int64(0)

	for _, candidate := range candidates {
		if !isUsableVideoWindowCandidate(candidate) {
			continue
		}
		if !predicate(candidate) {
			continue
		}

		distance := windowGeometryDistance(candidate.geometry, targetLayout)
		area := int64(candidate.geometry.Width) * int64(candidate.geometry.Height)
		if !bestFound || distance < bestDistance || (distance == bestDistance && area > bestArea) {
			best = candidate
			bestFound = true
			bestDistance = distance
			bestArea = area
		}
	}

	return best, bestFound
}

// isUsableVideoWindowCandidate 判断候选窗是否具备进入视频层筛选的最小条件。
// What: 统一要求窗口已 Viewable 且几何尺寸有效。
// Why: 无论是严格匹配还是保守回退，都不能把隐藏窗、零尺寸壳层或尚未完成 map 的过渡窗当成最终视频窗目标。
func isUsableVideoWindowCandidate(candidate x11WindowCandidate) bool {
	return candidate.mapState == byte(xproto.MapStateViewable) &&
		candidate.geometry.Width > 0 &&
		candidate.geometry.Height > 0
}

// candidateHasFFplayClass 判断候选窗的 WM_CLASS 是否指向 ffplay。
// What: 用不区分大小写的子串匹配识别 `ffplay` 类名。
// Why: XWayland 下 SDL/ffplay 的 instance/class 写法可能是 `ffplay`、`FFplay` 或组合字符串；这里只关心它是不是 ffplay 系窗口，而不是要求精确格式。
func candidateHasFFplayClass(candidate x11WindowCandidate) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(candidate.wmClass)), "ffplay")
}

// windowGeometryLooksLikeSearchTarget 判断候选窗在搜索阶段是否已经足够接近目标布局。
// What: 使用比最终贴合校验更宽的 left/top/width/height 搜索容差。
// Why: 这一步的目的不是确认“已经完美贴好”，而是从一堆窗口里先把“最像要找的视频窗”的候选收敛出来。
func windowGeometryLooksLikeSearchTarget(geometry x11WindowGeometry, layout VideoWindowLayout) bool {
	return absInt(geometry.Left-layout.Left) <= nativeWindowSearchPositionTolerancePx &&
		absInt(geometry.Top-layout.Top) <= nativeWindowSearchPositionTolerancePx &&
		absInt(geometry.Width-layout.Width) <= nativeWindowSearchSizeTolerancePx &&
		absInt(geometry.Height-layout.Height) <= nativeWindowSearchSizeTolerancePx
}

// windowGeometryDistance 计算候选窗与目标布局的几何距离。
// What: 将 left/top/width/height 的绝对差值合并成一个单调距离。
// Why: 保守回退阶段需要在多个“可能是视频窗”的候选之间选出最像目标布局的那个，这里提供稳定、可比较的排序依据。
func windowGeometryDistance(geometry x11WindowGeometry, layout VideoWindowLayout) int64 {
	return int64(absInt(geometry.Left-layout.Left) +
		absInt(geometry.Top-layout.Top) +
		absInt(geometry.Width-layout.Width) +
		absInt(geometry.Height-layout.Height))
}

// buildNativeVideoWindowSelectionError 根据候选窗分布生成更具体的“没选中”原因。
// What: 区分“完全没有可读候选”“有候选但都不可见”“有相关候选但不满足 PID/标题/class/几何条件”等不同失败层级。
// Why: 当前现场日志里只有一句 `not found yet`，信息量太低；排障需要知道问题到底卡在树枚举、窗口映射，还是属性尚未就绪。
func buildNativeVideoWindowSelectionError(candidates []x11WindowCandidate, pid uint32, title string, targetLayout VideoWindowLayout) error {
	if len(candidates) == 0 {
		return fmt.Errorf("x11 window discovery yielded no readable candidates for pid=%d title=%q", pid, title)
	}

	viewableCount := 0
	titleMatchCount := 0
	pidMatchCount := 0
	ffplayClassCount := 0
	searchGeometryCount := 0

	for _, candidate := range candidates {
		if candidate.mapState == byte(xproto.MapStateViewable) {
			viewableCount++
		}
		if strings.TrimSpace(candidate.title) == strings.TrimSpace(title) {
			titleMatchCount++
		}
		if candidate.pidKnown && candidate.pid == pid {
			pidMatchCount++
		}
		if candidateHasFFplayClass(candidate) {
			ffplayClassCount++
		}
		if windowGeometryLooksLikeSearchTarget(candidate.geometry, targetLayout) {
			searchGeometryCount++
		}
	}

	if viewableCount == 0 {
		return fmt.Errorf("x11 window candidates found but none are viewable yet for pid=%d title=%q", pid, title)
	}
	if titleMatchCount > 0 {
		return fmt.Errorf("x11 title-matched window candidates found but none are currently usable for pid=%d title=%q", pid, title)
	}
	if pidMatchCount > 0 {
		return fmt.Errorf("x11 pid-matched window candidates found but none are currently usable for pid=%d title=%q", pid, title)
	}
	if ffplayClassCount > 0 {
		return fmt.Errorf("x11 ffplay-class window candidates found but none are close enough to target layout for pid=%d title=%q", pid, title)
	}
	if searchGeometryCount > 0 {
		return fmt.Errorf("x11 geometry-nearby window candidates found but none matched pid/title/class for pid=%d title=%q", pid, title)
	}
	return fmt.Errorf("x11 window candidates found but no candidate matched pid=%d title=%q", pid, title)
}

// logWindowSearchDiagnostics 打印一次压缩后的窗口搜索诊断摘要。
// What: 将目标布局、最终命中的候选窗和本轮前若干个候选窗概要一起写入日志。
// Why: 真正排障时最需要的是“本轮到底看到了哪些窗以及为什么没选它们”，而不是只看最终一条抽象错误文案。
func logWindowSearchDiagnostics(pid int, searchResult x11WindowSearchResult, targetLayout VideoWindowLayout, reason error) {
	selectedSummary := "none"
	if searchResult.candidate.windowID != 0 {
		selectedSummary = describeWindowCandidate(searchResult.candidate, targetLayout)
	}

	log.Printf(
		"[Warning] [video] Window search diagnostics pid=%d target=(%d,%d,%d,%d) strategy=%s selected=%s candidates=%s reason=%v",
		pid,
		targetLayout.Left,
		targetLayout.Top,
		targetLayout.Width,
		targetLayout.Height,
		searchResult.strategy,
		selectedSummary,
		summarizeWindowCandidates(searchResult.candidates, targetLayout, nativeWindowSearchLogLimit),
		reason,
	)
}

// summarizeWindowCandidates 生成候选窗列表的压缩摘要。
// What: 只展开有限个最有诊断价值的候选窗，并在必要时标记剩余数量。
// Why: 现场窗口数可能很多；日志必须可读，不能为了完整性把所有无关窗全打印出来。
func summarizeWindowCandidates(candidates []x11WindowCandidate, targetLayout VideoWindowLayout, limit int) string {
	if len(candidates) == 0 {
		return "none"
	}

	summaries := make([]string, 0, limit)
	for _, candidate := range candidates {
		if candidate.windowID == 0 {
			continue
		}
		if candidate.title == "" && candidate.wmClass == "" && !candidate.pidKnown {
			continue
		}

		summaries = append(summaries, describeWindowCandidate(candidate, targetLayout))
		if len(summaries) >= limit {
			break
		}
	}

	if len(summaries) == 0 {
		for _, candidate := range candidates {
			summaries = append(summaries, describeWindowCandidate(candidate, targetLayout))
			if len(summaries) >= limit {
				break
			}
		}
	}

	if len(candidates) > len(summaries) {
		summaries = append(summaries, fmt.Sprintf("...+%d more", len(candidates)-len(summaries)))
	}

	return strings.Join(summaries, " | ")
}

// describeWindowCandidate 将单个候选窗编码成单行摘要。
// What: 输出窗口 ID、PID、标题、WM_CLASS、mapState、来源、几何和与目标布局的距离。
// Why: 排障时需要快速比较不同候选窗到底哪里不同，这里把最关键的筛选维度都固定到一行里。
func describeWindowCandidate(candidate x11WindowCandidate, targetLayout VideoWindowLayout) string {
	pidLabel := "?"
	if candidate.pidKnown {
		pidLabel = fmt.Sprintf("%d", candidate.pid)
	}

	return fmt.Sprintf(
		"0x%08x src=%s pid=%s title=%q class=%q map=%s geom=(%d,%d,%d,%d) dist=%d",
		uint32(candidate.windowID),
		candidate.source,
		pidLabel,
		candidate.title,
		candidate.wmClass,
		describeMapState(candidate.mapState),
		candidate.geometry.Left,
		candidate.geometry.Top,
		candidate.geometry.Width,
		candidate.geometry.Height,
		windowGeometryDistance(candidate.geometry, targetLayout),
	)
}

// describeMapState 将 X11 的 map_state 编码成人类可读的字符串。
// What: 把 `Unmapped` / `Unviewable` / `Viewable` 三种常见状态转成稳定文本。
// Why: 当前运行时搜索失败的一个关键分叉点就是“窗口到底存在但未 viewable，还是压根没被发现”；日志里必须直接看得出来。
func describeMapState(mapState byte) string {
	switch mapState {
	case byte(xproto.MapStateUnmapped):
		return "unmapped"
	case byte(xproto.MapStateUnviewable):
		return "unviewable"
	case byte(xproto.MapStateViewable):
		return "viewable"
	default:
		return fmt.Sprintf("unknown(%d)", mapState)
	}
}

// selectLargestVisibleWindowByPIDAndTitle 从候选顶层窗里挑出真正的主窗。
// What: 只接受 PID 与标题都精确命中的 Viewable 窗口，并在多候选时优先选择面积最大的那个。
// Why: Wails/GTK 与 SDL 在同一 PID 下可能同时暴露多个顶层窗；主 HUD/视频窗几乎总是那一个面积最大的可见窗，不能再依赖“先找到谁就算谁”。
func selectLargestVisibleWindowByPIDAndTitle(candidates []x11WindowCandidate, pid uint32, title string) (x11WindowCandidate, error) {
	var best x11WindowCandidate
	bestArea := int64(-1)

	for _, candidate := range candidates {
		// What: 只接受目标进程自己的窗口。
		// Why: client list 里会混入整桌面所有顶层窗，不先卡 PID，后续标题再像也没有意义。
		if !candidate.pidKnown || candidate.pid != pid {
			continue
		}

		// What: 标题必须与调用方期望值完全一致。
		// Why: 本轮修复的关键点就是禁止 HUD 再回退到“同 PID 的任意辅助窗”；标题不一致直接排除最稳。
		if strings.TrimSpace(candidate.title) != strings.TrimSpace(title) {
			continue
		}

		// What: 只接受窗口管理器已经标记为可见的顶层窗。
		// Why: 未 map 完成或已隐藏的窗口即便 PID/标题命中，也不能作为几何与层级控制目标。
		if candidate.mapState != byte(xproto.MapStateViewable) {
			continue
		}

		// What: 过滤掉零尺寸和异常尺寸候选。
		// Why: 这些候选通常是过渡态辅助窗或尚未完成布局的壳层，拿来驱动 ffplay 只会把视频挤到错误位置。
		if candidate.geometry.Width <= 0 || candidate.geometry.Height <= 0 {
			continue
		}

		area := int64(candidate.geometry.Width) * int64(candidate.geometry.Height)
		if area > bestArea {
			best = candidate
			bestArea = area
		}
	}

	if bestArea < 0 {
		return x11WindowCandidate{}, fmt.Errorf("x11 window for pid=%d title=%q not found yet", pid, title)
	}
	return best, nil
}

// readWindowListProperty 读取 root 上保存的一组顶层窗 ID。
// What: 从 EWMH 的 `_NET_CLIENT_LIST*` 属性里恢复窗口数组。
// Why: 这是定位被窗口管理器接管的顶层窗最直接的入口，比递归扫树更轻。
func readWindowListProperty(conn *xgb.Conn, window xproto.Window, property xproto.Atom) ([]xproto.Window, error) {
	reply, err := xproto.GetProperty(conn, false, window, property, xproto.AtomWindow, 0, 4096).Reply()
	if err != nil {
		return nil, err
	}

	if reply == nil || reply.Format != 32 || len(reply.Value) == 0 {
		return nil, nil
	}

	result := make([]xproto.Window, 0, len(reply.Value)/4)
	for offset := 0; offset+4 <= len(reply.Value); offset += 4 {
		result = append(result, xproto.Window(xgb.Get32(reply.Value[offset:])))
	}
	return result, nil
}

// readWindowPID 读取窗口声明的 `_NET_WM_PID`。
// What: 从窗口属性里提取创建该窗口的进程 PID。
// Why: 仅靠标题定位窗口在 ffplay 高频重启时不可靠，PID 是区分“当前实例”和“上一轮残留窗”的关键。
func readWindowPID(conn *xgb.Conn, window xproto.Window, property xproto.Atom) (uint32, bool, error) {
	reply, err := xproto.GetProperty(conn, false, window, property, xproto.AtomCardinal, 0, 1).Reply()
	if err != nil {
		return 0, false, err
	}

	if reply == nil || reply.Format != 32 || len(reply.Value) < 4 {
		return 0, false, nil
	}

	return xgb.Get32(reply.Value), true, nil
}

// readWindowWMClass 读取窗口声明的 `WM_CLASS`。
// What: 将 X11 原始 `instance\0class\0` 字节串解析成单个可读字符串。
// Why: ffplay/XWayland 在标题或 PID 尚未稳定时，`WM_CLASS` 往往更早可见；若这里不单独解析，就无法建立 `ffplay` 类名回退链路。
func readWindowWMClass(conn *xgb.Conn, window xproto.Window) (string, bool, error) {
	reply, err := xproto.GetProperty(conn, false, window, xproto.AtomWmClass, xproto.AtomString, 0, 1024).Reply()
	if err != nil {
		return "", false, err
	}

	if reply == nil || len(reply.Value) == 0 {
		return "", false, nil
	}

	classParts := make([]string, 0, 2)
	for _, rawPart := range bytes.Split(bytes.TrimRight(reply.Value, "\x00"), []byte{0}) {
		part := strings.TrimSpace(string(rawPart))
		if part == "" {
			continue
		}
		classParts = append(classParts, part)
	}

	if len(classParts) == 0 {
		return "", false, nil
	}

	return strings.Join(classParts, "/"), true, nil
}

// readWindowMapState 读取当前顶层窗的可见状态。
// What: 从 `GetWindowAttributes` 中提取 map_state。
// Why: 只有真正 Viewable 的顶层窗才适合作为 HUD 或 ffplay 主窗目标，隐藏窗和尚未映射完成的窗都必须提前排掉。
func readWindowMapState(conn *xgb.Conn, window xproto.Window) (byte, error) {
	reply, err := xproto.GetWindowAttributes(conn, window).Reply()
	if err != nil {
		return 0, err
	}
	if reply == nil {
		return 0, fmt.Errorf("x11 window attributes unavailable for 0x%08x", uint32(window))
	}
	return reply.MapState, nil
}

// readAbsoluteWindowGeometry 读取目标窗在 root 坐标系中的绝对位置与逻辑尺寸。
// What: 同时查询窗口自身几何尺寸和相对 root 的平移结果。
// Why: 某些 WM 会对顶层窗做 reparent，单独拿 `GetGeometry` 的 X/Y 不能稳定代表桌面绝对坐标，必须显式换算到 root 坐标系。
func readAbsoluteWindowGeometry(conn *xgb.Conn, root xproto.Window, window xproto.Window) (x11WindowGeometry, error) {
	geometryReply, err := xproto.GetGeometry(conn, xproto.Drawable(window)).Reply()
	if err != nil {
		return x11WindowGeometry{}, err
	}

	translateReply, err := xproto.TranslateCoordinates(conn, window, root, 0, 0).Reply()
	if err != nil {
		return x11WindowGeometry{}, err
	}

	return x11WindowGeometry{
		Left:   int(translateReply.DstX),
		Top:    int(translateReply.DstY),
		Width:  int(geometryReply.Width),
		Height: int(geometryReply.Height),
	}, nil
}

// readWindowTitle 读取窗口标题。
// What: 先读 `_NET_WM_NAME`，失败时再回退到传统 `WM_NAME`。
// Why: 新旧窗口管理器和 SDL 后端对标题属性的写法并不一致，双路径兼容更稳。
func readWindowTitle(conn *xgb.Conn, window xproto.Window, atoms x11AtomSet) (string, error) {
	title, ok, err := readWindowStringProperty(conn, window, atoms.netWMName, atoms.utf8String)
	if err == nil && ok {
		return title, nil
	}

	title, ok, err = readWindowStringProperty(conn, window, xproto.AtomWmName, xproto.AtomString)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return title, nil
}

// readWindowStringProperty 把任意字符串属性解成 Go string。
// What: 统一处理 X11 属性里常见的结尾空字节。
// Why: 若不在这里去掉尾随 `\x00`，标题匹配会被这些不可见字符反复误伤。
func readWindowStringProperty(conn *xgb.Conn, window xproto.Window, property xproto.Atom, propertyType xproto.Atom) (string, bool, error) {
	reply, err := xproto.GetProperty(conn, false, window, property, propertyType, 0, 1024).Reply()
	if err != nil {
		return "", false, err
	}

	if reply == nil || len(reply.Value) == 0 {
		return "", false, nil
	}

	return strings.TrimSpace(string(bytes.TrimRight(reply.Value, "\x00"))), true, nil
}

// buildVideoWindowStateAtoms 构造视频窗应保留的 `_NET_WM_STATE` 列表。
// What: 只保留 skip taskbar / pager 这类不会抬高全局层级的状态。
// Why: 当前需求已经从“视频盖住普通桌面程序”改成“视频只停在 HUD 下方”，因此这里必须明确排除 `_NET_WM_STATE_ABOVE`。
func buildVideoWindowStateAtoms(atoms x11AtomSet) []xproto.Atom {
	return []xproto.Atom{
		atoms.netWMStateSkipTaskbar,
		atoms.netWMStateSkipPager,
	}
}

// windowGeometryMatchesLayout 判断 X11 回读几何是否已经贴合目标布局。
// What: 逐项比较 left/top/width/height，但允许极小的窗口管理器抖动容差。
// Why: 当前现场回归就是 ffplay 明明已经出画，却因为 XWayland/WM 的微小几何修正被判成 geometry mismatch；这里必须把“轻微漂移”和“真正跑偏”区分开。
func windowGeometryMatchesLayout(geometry x11WindowGeometry, layout VideoWindowLayout) bool {
	return absInt(geometry.Left-layout.Left) <= windowGeometryPositionTolerancePx &&
		absInt(geometry.Top-layout.Top) <= windowGeometryPositionTolerancePx &&
		absInt(geometry.Width-layout.Width) <= windowGeometrySizeTolerancePx &&
		absInt(geometry.Height-layout.Height) <= windowGeometrySizeTolerancePx
}

// absInt 返回整数绝对值。
// What: 将几何回读差值统一转成非负数。
// Why: 位置和尺寸既可能向正方向漂，也可能向负方向漂；只有先归一成绝对值，容差判断与日志才不会漏掉半边情况。
func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// x11SignedIntToUint32 将 X11 允许的有符号坐标编码成 ConfigureWindow 需要的 32bit 值。
// What: 对 left/top 做显式 int32 收窄后再转 uint32。
// Why: X11 协议里的坐标字段本质上是有符号量，直接从 Go int 强转可能在 32/64 位边界上留下歧义。
func x11SignedIntToUint32(value int) uint32 {
	return uint32(int32(value))
}

// applyWindowBelowHUD 把目标视频窗贴到 HUD 布局范围内，并压到 HUD 窗口正下方。
// What: 先清掉会抬高全局层级的 EWMH 状态，再同步下发几何修正与 sibling+below 请求。
// Why: 只有同时控制“窗在哪里”和“窗压在谁下面”，才能稳定满足“HUD > 视频 > 背景”这一顺序。
func applyWindowBelowHUD(conn *xgb.Conn, root xproto.Window, window xproto.Window, hudWindow xproto.Window, layout VideoWindowLayout, atoms x11AtomSet) error {
	anchorWindow := hudWindow
	if layout.StackAboveWindowID != 0 {
		anchorWindow = xproto.Window(layout.StackAboveWindowID)
	}

	stateAtoms := buildVideoWindowStateAtoms(atoms)

	stateBytes := make([]byte, len(stateAtoms)*4)
	for index, atom := range stateAtoms {
		// What: 按 32bit atom 列表重建 `_NET_WM_STATE` 属性体。
		// Why: 有些 WM 在首次接管窗口时会直接读取属性快照，因此这里不能只发 client message，而不把“不要 ABOVE”落实到属性本体上。
		xgb.Put32(stateBytes[index*4:], uint32(atom))
	}

	if err := xproto.ChangePropertyChecked(
		conn,
		xproto.PropModeReplace,
		window,
		atoms.netWMState,
		xproto.AtomAtom,
		32,
		uint32(len(stateAtoms)),
		stateBytes,
	).Check(); err != nil {
		return err
	}

	if err := sendWMStateMessage(conn, root, window, atoms.netWMState, atoms.netWMStateAbove, 0); err != nil {
		return err
	}
	if err := sendWMStateMessage(conn, root, window, atoms.netWMState, atoms.netWMStateBelow, 0); err != nil {
		return err
	}
	if err := sendWMStateMessage(conn, root, window, atoms.netWMState, atoms.netWMStateSkipTaskbar, 1); err != nil {
		return err
	}
	if err := sendWMStateMessage(conn, root, window, atoms.netWMState, atoms.netWMStateSkipPager, 1); err != nil {
		return err
	}

	if err := xproto.ConfigureWindowChecked(
		conn,
		window,
		xproto.ConfigWindowX|
			xproto.ConfigWindowY|
			xproto.ConfigWindowWidth|
			xproto.ConfigWindowHeight|
			xproto.ConfigWindowSibling|
			xproto.ConfigWindowStackMode,
		[]uint32{
			x11SignedIntToUint32(layout.Left),
			x11SignedIntToUint32(layout.Top),
			uint32(layout.Width),
			uint32(layout.Height),
			uint32(anchorWindow),
			uint32(xproto.StackModeBelow),
		},
	).Check(); err != nil {
		return err
	}

	// What: 主动与 X server 做一次同步。
	// Why: 这样可以尽快把“移除 ABOVE + 压到 HUD 下方”这组请求真正冲到服务器侧，而不是拖到后续别的请求里一起发送。
	conn.Sync()
	return nil
}

// sendWMStateMessage 向窗口管理器发送 `_NET_WM_STATE` 请求。
// What: 用标准 EWMH client message 增删单个窗口状态。
// Why: 直接改属性并不能保证所有 WM 都立即响应；真正可靠的做法仍然是通知 root window 上的窗口管理器。
func sendWMStateMessage(conn *xgb.Conn, root xproto.Window, window xproto.Window, stateProperty xproto.Atom, targetState xproto.Atom, action uint32) error {
	event := xproto.ClientMessageEvent{
		Format: 32,
		Window: window,
		Type:   stateProperty,
		Data: xproto.ClientMessageDataUnionData32New([]uint32{
			action,
			uint32(targetState),
			0,
			1,
			0,
		}),
	}

	return xproto.SendEventChecked(
		conn,
		false,
		root,
		xproto.EventMaskSubstructureRedirect|xproto.EventMaskSubstructureNotify,
		string(event.Bytes()),
	).Check()
}
