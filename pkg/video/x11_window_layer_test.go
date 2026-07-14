package video

import (
	"strings"
	"testing"

	"github.com/BurntSushi/xgb/xproto"
)

func TestDecideNativeWindowLayerPolicyForWaylandWithDisplay(t *testing.T) {
	// What: 模拟用户当前这种 Wayland 会话但同时存在 XWayland DISPLAY 的环境。
	// Why: 这正是“HUD 走 Wayland，ffplay 需要单独压层”的目标场景，必须锁住策略不会回退成不可控。
	policy := decideNativeWindowLayerPolicy("wayland", ":1")

	if !policy.hasDisplay {
		t.Fatalf("expected display to be available")
	}
	if !policy.enableX11Stacking {
		t.Fatalf("expected x11 stacking to be enabled")
	}
	if !policy.forceSDLX11 {
		t.Fatalf("expected ffplay to be forced onto x11 backend")
	}
}

func TestDecideNativeWindowLayerPolicyForPureWaylandWithoutDisplay(t *testing.T) {
	// What: 模拟纯 Wayland 且没有 DISPLAY 的环境。
	// Why: 这类环境无法对独立 ffplay 窗口做 X11 层级控制，必须明确降级而不是误报“已处理”。
	policy := decideNativeWindowLayerPolicy("wayland", "")

	if policy.hasDisplay {
		t.Fatalf("expected display to be unavailable")
	}
	if policy.enableX11Stacking {
		t.Fatalf("expected x11 stacking to be disabled")
	}
	if policy.forceSDLX11 {
		t.Fatalf("expected no x11 forcing without display")
	}
}

func TestDecideNativeWindowLayerPolicyForX11Session(t *testing.T) {
	// What: 模拟标准 X11 会话。
	// Why: 在纯 X11 场景下应当继续启用窗口下压，但没有必要再额外强制 SDL backend。
	policy := decideNativeWindowLayerPolicy("x11", ":0")

	if !policy.hasDisplay {
		t.Fatalf("expected display to be available")
	}
	if !policy.enableX11Stacking {
		t.Fatalf("expected x11 stacking to stay enabled")
	}
	if policy.forceSDLX11 {
		t.Fatalf("expected no extra x11 forcing in x11 session")
	}
}

func TestVideoWindowLayoutFromGeometryPreservesAbsoluteOffset(t *testing.T) {
	// What: 构造一份落在副屏上的 HUD 绝对几何信息。
	// Why: 多屏问题的核心回归点就是 Left/Top 不能再被偷偷归零，否则视频层会重新跑回主屏左上角。
	layout, err := videoWindowLayoutFromGeometry(x11WindowGeometry{
		Left:   1920,
		Top:    48,
		Width:  2560,
		Height: 1440,
	})
	if err != nil {
		t.Fatalf("unexpected geometry conversion error: %v", err)
	}

	if layout.Left != 1920 || layout.Top != 48 {
		t.Fatalf("expected absolute offset to be preserved, got left=%d top=%d", layout.Left, layout.Top)
	}
	if layout.Width != 2560 || layout.Height != 1440 {
		t.Fatalf("unexpected layout size width=%d height=%d", layout.Width, layout.Height)
	}
}

func TestVideoWindowLayoutFromGeometryRejectsInvalidSize(t *testing.T) {
	// What: 构造一份缺失逻辑尺寸的窗口几何。
	// Why: 若这里放过零尺寸布局，ffplay 会以错误参数起窗，后续层级和多屏逻辑都会直接失效。
	_, err := videoWindowLayoutFromGeometry(x11WindowGeometry{
		Left:   1920,
		Top:    0,
		Width:  0,
		Height: 1080,
	})
	if err == nil {
		t.Fatalf("expected invalid geometry to be rejected")
	}
}

func TestBuildVideoWindowStateAtomsOmitsGlobalAboveState(t *testing.T) {
	// What: 构造一组最小 atom 句柄并生成视频窗状态列表。
	// Why: 这条用例要锁住“视频窗不再携带 ABOVE 状态”，防止后续回归又把 ffplay 抬成桌面高层窗。
	atoms := x11AtomSet{
		netWMStateAbove:       11,
		netWMStateSkipTaskbar: 22,
		netWMStateSkipPager:   33,
	}

	stateAtoms := buildVideoWindowStateAtoms(atoms)
	if len(stateAtoms) != 2 {
		t.Fatalf("unexpected state atom count: %d", len(stateAtoms))
	}
	if stateAtoms[0] != atoms.netWMStateSkipTaskbar {
		t.Fatalf("unexpected first state atom: %d", stateAtoms[0])
	}
	if stateAtoms[1] != atoms.netWMStateSkipPager {
		t.Fatalf("unexpected second state atom: %d", stateAtoms[1])
	}
	for _, atom := range stateAtoms {
		if atom == atoms.netWMStateAbove {
			t.Fatalf("video window state should not include global ABOVE atom")
		}
	}
}

func TestWindowGeometryMatchesLayoutAllowsSmallPositionDrift(t *testing.T) {
	// What: 构造一份仅有极小 left/top 漂移的窗口回读结果。
	// Why: 当前实机回归正是窗口管理器给 ffplay 加了几个像素的补偿；这类轻微漂移不应再触发 geometry mismatch 黑幕。
	layout := VideoWindowLayout{
		Left:   100,
		Top:    200,
		Width:  1920,
		Height: 1080,
	}

	if !windowGeometryMatchesLayout(x11WindowGeometry{
		Left:   108,
		Top:    193,
		Width:  1920,
		Height: 1080,
	}, layout) {
		t.Fatalf("expected small position drift to be tolerated")
	}
}

func TestWindowGeometryMatchesLayoutAllowsSmallSizeDrift(t *testing.T) {
	// What: 构造一份仅有极小 width/height 漂移的窗口回读结果。
	// Why: 某些 WM/XWayland 会对无边框窗做设备像素比取整；只要仍覆盖 HUD 主区域，就不应继续阻断视频显示。
	layout := VideoWindowLayout{
		Left:   0,
		Top:    0,
		Width:  2560,
		Height: 1440,
	}

	if !windowGeometryMatchesLayout(x11WindowGeometry{
		Left:   0,
		Top:    0,
		Width:  2576,
		Height: 1425,
	}, layout) {
		t.Fatalf("expected small size drift to be tolerated")
	}
}

func TestWindowGeometryMatchesLayoutRejectsLargeDrift(t *testing.T) {
	// What: 构造一份明显偏离目标 HUD 区域的窗口回读结果。
	// Why: 修复目标只是放过轻微抖动，不是放弃校验；真正跑偏到错误区域时仍必须继续报错阻断。
	layout := VideoWindowLayout{
		Left:   300,
		Top:    120,
		Width:  1920,
		Height: 1080,
	}

	if windowGeometryMatchesLayout(x11WindowGeometry{
		Left:   320,
		Top:    120,
		Width:  1920,
		Height: 1080,
	}, layout) {
		t.Fatalf("expected large drift to be rejected")
	}
}

func TestSelectLargestVisibleWindowByPIDAndTitlePrefersLargestMappedWindow(t *testing.T) {
	// What: 构造同 PID、同标题下的多个候选顶层窗。
	// Why: 当前修复要求 HUD/ffplay 必须优先选真正的主窗，而不是被较小的辅助窗或未映射窗口抢走身份。
	candidate, err := selectLargestVisibleWindowByPIDAndTitle([]x11WindowCandidate{
		{
			windowID: 0x11,
			pid:      4242,
			pidKnown: true,
			title:    "RoboMaster 2026 Custom Client",
			geometry: x11WindowGeometry{Left: 0, Top: 0, Width: 960, Height: 540},
			mapState: byte(xproto.MapStateViewable),
		},
		{
			windowID: 0x22,
			pid:      4242,
			pidKnown: true,
			title:    "RoboMaster 2026 Custom Client",
			geometry: x11WindowGeometry{Left: 0, Top: 0, Width: 1920, Height: 1080},
			mapState: byte(xproto.MapStateViewable),
		},
		{
			windowID: 0x33,
			pid:      4242,
			pidKnown: true,
			title:    "RoboMaster 2026 Custom Client",
			geometry: x11WindowGeometry{Left: 0, Top: 0, Width: 3840, Height: 2160},
			mapState: byte(xproto.MapStateUnviewable),
		},
	}, 4242, "RoboMaster 2026 Custom Client")
	if err != nil {
		t.Fatalf("unexpected selection error: %v", err)
	}

	if candidate.windowID != 0x22 {
		t.Fatalf("expected largest mapped window to be selected, got=0x%08x", uint32(candidate.windowID))
	}
}

func TestSelectLargestVisibleWindowByPIDAndTitleRequiresExactTitle(t *testing.T) {
	// What: 构造同 PID 下的“空标题辅助窗”和“精确标题主窗”。
	// Why: 本轮修复的关键就是禁止 HUD 再回退到“同 PID 但标题未就绪的任意窗”。
	candidate, err := selectLargestVisibleWindowByPIDAndTitle([]x11WindowCandidate{
		{
			windowID: 0x44,
			pid:      5150,
			pidKnown: true,
			title:    "",
			geometry: x11WindowGeometry{Left: 0, Top: 0, Width: 2560, Height: 1440},
			mapState: byte(xproto.MapStateViewable),
		},
		{
			windowID: 0x55,
			pid:      5150,
			pidKnown: true,
			title:    "RoboMaster Native Video Layer",
			geometry: x11WindowGeometry{Left: 10, Top: 20, Width: 1280, Height: 720},
			mapState: byte(xproto.MapStateViewable),
		},
	}, 5150, "RoboMaster Native Video Layer")
	if err != nil {
		t.Fatalf("unexpected selection error: %v", err)
	}

	if candidate.windowID != 0x55 {
		t.Fatalf("expected exact-title window to win over empty-title fallback, got=0x%08x", uint32(candidate.windowID))
	}
}

func TestSelectNativeVideoWindowCandidateFallsBackToExactTitleWhenPIDMissing(t *testing.T) {
	// What: 构造一个标题已经就绪、但 `_NET_WM_PID` 尚未可读的 ffplay 候选窗。
	// Why: Wayland/XWayland 下最常见的时序问题之一就是标题先出来而 PID 还没暴露；这时如果继续死卡 PID，真实视频窗就会被漏掉。
	targetLayout := VideoWindowLayout{
		Left:   40,
		Top:    60,
		Width:  1600,
		Height: 900,
	}

	candidate, strategy, err := selectNativeVideoWindowCandidate([]x11WindowCandidate{
		{
			windowID: 0x61,
			pidKnown: false,
			title:    "RoboMaster Native Video Layer",
			wmClass:  "ffplay/ffplay",
			source:   "query-tree-descendant",
			geometry: x11WindowGeometry{Left: 48, Top: 64, Width: 1600, Height: 900},
			mapState: byte(xproto.MapStateViewable),
		},
	}, 9001, "RoboMaster Native Video Layer", targetLayout)
	if err != nil {
		t.Fatalf("unexpected exact-title fallback error: %v", err)
	}
	if strategy != "exact-title-fallback" {
		t.Fatalf("unexpected fallback strategy: %s", strategy)
	}
	if candidate.windowID != 0x61 {
		t.Fatalf("unexpected exact-title fallback candidate: 0x%08x", uint32(candidate.windowID))
	}
}

func TestSelectNativeVideoWindowCandidateExactTitleFallbackRejectsWrongPIDResidual(t *testing.T) {
	// What: 构造一个“标题正确但 PID 属于旧进程”的残留窗，以及一个“标题正确但 PID 缺失”的当前候选窗。
	// Why: 运行时最危险的误判之一就是旧 ffplay 残留窗继续留在桌面上；标题回退必须优先接受 PID 缺失的当前候选，而不是误抓旧 PID 窗口。
	targetLayout := VideoWindowLayout{
		Left:   40,
		Top:    60,
		Width:  1600,
		Height: 900,
	}

	candidate, strategy, err := selectNativeVideoWindowCandidate([]x11WindowCandidate{
		{
			windowID: 0x62,
			pid:      7001,
			pidKnown: true,
			title:    "RoboMaster Native Video Layer",
			wmClass:  "ffplay/ffplay",
			source:   "ewmh-stacking",
			geometry: x11WindowGeometry{Left: 40, Top: 60, Width: 1600, Height: 900},
			mapState: byte(xproto.MapStateViewable),
		},
		{
			windowID: 0x63,
			pidKnown: false,
			title:    "RoboMaster Native Video Layer",
			wmClass:  "ffplay/ffplay",
			source:   "query-tree-descendant",
			geometry: x11WindowGeometry{Left: 48, Top: 64, Width: 1600, Height: 900},
			mapState: byte(xproto.MapStateViewable),
		},
	}, 9001, "RoboMaster Native Video Layer", targetLayout)
	if err != nil {
		t.Fatalf("unexpected exact-title fallback error with residual window: %v", err)
	}
	if strategy != "exact-title-fallback" {
		t.Fatalf("unexpected fallback strategy with residual window: %s", strategy)
	}
	if candidate.windowID != 0x63 {
		t.Fatalf("expected PID-missing current candidate to win over residual wrong-PID title match, got=0x%08x", uint32(candidate.windowID))
	}
}

func TestSelectNativeVideoWindowCandidateFallsBackToPIDGeometryWhenTitleMissing(t *testing.T) {
	// What: 构造一个 PID 已匹配、几何也接近目标，但标题仍为空的候选窗。
	// Why: SDL/XWayland 有可能先创建可见窗，再稍后补标题；此时 PID + 几何已经足够把正确视频窗从同屏其他窗口里收敛出来。
	targetLayout := VideoWindowLayout{
		Left:   100,
		Top:    120,
		Width:  1280,
		Height: 720,
	}

	candidate, strategy, err := selectNativeVideoWindowCandidate([]x11WindowCandidate{
		{
			windowID: 0x71,
			pid:      4242,
			pidKnown: true,
			title:    "",
			wmClass:  "",
			source:   "query-tree-descendant",
			geometry: x11WindowGeometry{Left: 110, Top: 126, Width: 1280, Height: 720},
			mapState: byte(xproto.MapStateViewable),
		},
		{
			windowID: 0x72,
			pid:      5151,
			pidKnown: true,
			title:    "",
			wmClass:  "",
			source:   "root-child",
			geometry: x11WindowGeometry{Left: 900, Top: 600, Width: 1280, Height: 720},
			mapState: byte(xproto.MapStateViewable),
		},
	}, 4242, "RoboMaster Native Video Layer", targetLayout)
	if err != nil {
		t.Fatalf("unexpected pid-geometry fallback error: %v", err)
	}
	if strategy != "pid-geometry-fallback" {
		t.Fatalf("unexpected fallback strategy: %s", strategy)
	}
	if candidate.windowID != 0x71 {
		t.Fatalf("unexpected pid-geometry fallback candidate: 0x%08x", uint32(candidate.windowID))
	}
}

func TestSelectNativeVideoWindowCandidateFallsBackToClassGeometryWhenPIDMissing(t *testing.T) {
	// What: 构造一个既没有标题也没有 PID，但 `WM_CLASS=ffplay` 且几何已接近目标布局的候选窗。
	// Why: 这条路径就是最后的保守兜底；当 XWayland 只先暴露 class 与几何时，窗口发现器仍应能把唯一合理的 ffplay 窗抓出来。
	targetLayout := VideoWindowLayout{
		Left:   10,
		Top:    20,
		Width:  1920,
		Height: 1080,
	}

	candidate, strategy, err := selectNativeVideoWindowCandidate([]x11WindowCandidate{
		{
			windowID: 0x81,
			pidKnown: false,
			title:    "",
			wmClass:  "ffplay/ffplay",
			source:   "query-tree-descendant",
			geometry: x11WindowGeometry{Left: 20, Top: 24, Width: 1910, Height: 1080},
			mapState: byte(xproto.MapStateViewable),
		},
		{
			windowID: 0x82,
			pidKnown: false,
			title:    "",
			wmClass:  "vlc/vlc",
			source:   "root-child",
			geometry: x11WindowGeometry{Left: 18, Top: 20, Width: 1918, Height: 1080},
			mapState: byte(xproto.MapStateViewable),
		},
	}, 7777, "RoboMaster Native Video Layer", targetLayout)
	if err != nil {
		t.Fatalf("unexpected class-geometry fallback error: %v", err)
	}
	if strategy != "class-geometry-fallback" {
		t.Fatalf("unexpected fallback strategy: %s", strategy)
	}
	if candidate.windowID != 0x81 {
		t.Fatalf("unexpected class-geometry fallback candidate: 0x%08x", uint32(candidate.windowID))
	}
}

func TestSelectNativeVideoWindowCandidateRejectsUnviewableFallbacks(t *testing.T) {
	// What: 构造一个标题命中但仍未进入 Viewable 状态的候选窗。
	// Why: 回退策略只应该放宽属性条件，不能放弃“窗口必须真正可见”这个底线，否则还没 map 完成的壳层也会被误判成视频窗。
	targetLayout := VideoWindowLayout{
		Left:   0,
		Top:    0,
		Width:  800,
		Height: 600,
	}

	_, _, err := selectNativeVideoWindowCandidate([]x11WindowCandidate{
		{
			windowID: 0x91,
			pidKnown: false,
			title:    "RoboMaster Native Video Layer",
			wmClass:  "ffplay/ffplay",
			source:   "query-tree-descendant",
			geometry: x11WindowGeometry{Left: 0, Top: 0, Width: 800, Height: 600},
			mapState: byte(xproto.MapStateUnviewable),
		},
	}, 9999, "RoboMaster Native Video Layer", targetLayout)
	if err == nil {
		t.Fatalf("expected unviewable fallback candidate to be rejected")
	}
	if !strings.Contains(err.Error(), "none are viewable yet") {
		t.Fatalf("unexpected unviewable fallback error: %v", err)
	}
}

func TestSelectNativeVideoWindowCandidateOfficialFallbackExcludesCustomPiPTitle(t *testing.T) {
	targetLayout := VideoWindowLayout{
		Left:   0,
		Top:    0,
		Width:  1920,
		Height: 1080,
	}

	candidate, strategy, err := selectNativeVideoWindowCandidate([]x11WindowCandidate{
		{
			windowID: 0xa1,
			pidKnown: false,
			title:    nativeCustomVideoWindowTitle,
			wmClass:  "ffplay/ffplay",
			source:   "query-tree-descendant",
			geometry: x11WindowGeometry{Left: 8, Top: 10, Width: 1918, Height: 1078},
			mapState: byte(xproto.MapStateViewable),
		},
		{
			windowID: 0xa2,
			pidKnown: false,
			title:    "",
			wmClass:  "ffplay/ffplay",
			source:   "query-tree-descendant",
			geometry: x11WindowGeometry{Left: 6, Top: 6, Width: 1916, Height: 1076},
			mapState: byte(xproto.MapStateViewable),
		},
	}, 9999, nativeOfficialVideoWindowTitle, targetLayout)
	if err != nil {
		t.Fatalf("unexpected official fallback error: %v", err)
	}
	if strategy != "class-geometry-fallback" {
		t.Fatalf("unexpected official fallback strategy: %s", strategy)
	}
	if candidate.windowID != 0xa2 {
		t.Fatalf("official fallback should exclude custom titled candidate, got=0x%08x", uint32(candidate.windowID))
	}
}

func TestSelectNativeVideoWindowCandidateCustomSearchRejectsClassOnlyFallback(t *testing.T) {
	targetLayout := VideoWindowLayout{
		Left:   40,
		Top:    60,
		Width:  1600,
		Height: 900,
	}

	_, _, err := selectNativeVideoWindowCandidateWithMode([]x11WindowCandidate{
		{
			windowID: 0xb1,
			pidKnown: false,
			title:    "",
			wmClass:  "ffplay/ffplay",
			source:   "query-tree-descendant",
			geometry: x11WindowGeometry{Left: 40, Top: 60, Width: 1600, Height: 900},
			mapState: byte(xproto.MapStateViewable),
		},
	}, 7001, nativeCustomVideoWindowTitle, targetLayout, nativeVideoWindowSearchModeCustom)
	if err == nil {
		t.Fatalf("expected custom search mode to reject class-only fallback")
	}
	if !strings.Contains(err.Error(), "ffplay-class") && !strings.Contains(err.Error(), "geometry-nearby") {
		t.Fatalf("unexpected custom search error: %v", err)
	}
}
