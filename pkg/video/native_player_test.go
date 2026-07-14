package video

import (
	"errors"
	"io"
	"os/exec"
	"testing"
	"time"
)

type noopWriteCloser struct{}

func (noopWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (noopWriteCloser) Close() error {
	return nil
}

func TestBuildWindowLayerRuntimeErrorSuppressesFailureBeforeFirstPresent(t *testing.T) {
	// What: 构造“窗口层当前失败，但当前 ffplay 进程还从未成功收到任何一帧”的场景。
	// Why: Wayland/XWayland 下 ffplay 经常要到首帧附近才真正创建顶层窗；这段时间必须保持等待态，不能提前把前端打成 runtime_error。
	message := buildWindowLayerRuntimeError(
		errors.New("x11 window for pid=42 title=\"RoboMaster Native Video Layer\" not found yet"),
		0,
		0,
		time.UnixMilli(10_000),
	)

	if message != "" {
		t.Fatalf("expected soft waiting before first present, got=%q", message)
	}
}

func TestBuildWindowLayerRuntimeErrorSuppressesFailureDuringPresentGrace(t *testing.T) {
	// What: 构造“当前进程已经开始出帧，但距离首帧仍处于缓冲窗口内”的场景。
	// Why: 这正是 ffplay 迟到建窗最常见的区间；只要仍在 grace period 内，就应继续等待贴窗，而不是升级成硬错误。
	now := time.UnixMilli(20_000)
	firstPresentAt := now.Add(-windowLayerErrorPromotionAfterPresent + 300*time.Millisecond).UnixMilli()
	lastPresentAt := now.Add(-120 * time.Millisecond).UnixMilli()

	message := buildWindowLayerRuntimeError(
		errors.New("x11 window for pid=84 title=\"RoboMaster Native Video Layer\" not found yet"),
		firstPresentAt,
		lastPresentAt,
		now,
	)

	if message != "" {
		t.Fatalf("expected soft waiting during present grace, got=%q", message)
	}
}

func TestBuildWindowLayerRuntimeErrorPromotesStablePresentFailure(t *testing.T) {
	// What: 构造“当前进程已经稳定出帧足够久，且最近仍在持续呈现，但窗口层依旧无法贴合”的场景。
	// Why: 只有这种状态才应该被视为真正异常；否则前端就无法把‘已经出画但仍漏桌面’明确暴露给用户。
	now := time.UnixMilli(30_000)
	firstPresentAt := now.Add(-windowLayerErrorPromotionAfterPresent - 500*time.Millisecond).UnixMilli()
	lastPresentAt := now.Add(-120 * time.Millisecond).UnixMilli()

	message := buildWindowLayerRuntimeError(
		errors.New("video window geometry mismatch actual=(0,0,100,100) target=(10,10,200,200)"),
		firstPresentAt,
		lastPresentAt,
		now,
	)

	expected := "原生视频窗未能贴合 HUD，已阻止透出桌面: video window geometry mismatch actual=(0,0,100,100) target=(10,10,200,200)"
	if message != expected {
		t.Fatalf("unexpected promoted window-layer error: got=%q want=%q", message, expected)
	}
}

func TestBuildWindowLayerRuntimeErrorSuppressesFailureAfterPresentBecomesStale(t *testing.T) {
	// What: 构造“当前进程曾经出过帧，但最近已经明显停帧”的场景。
	// Why: 一旦呈现已经不新鲜，真实问题更可能是播放器停摆或链路断流；这时继续把错误定性成贴窗失败会误导排障。
	now := time.UnixMilli(40_000)
	firstPresentAt := now.Add(-windowLayerErrorPromotionAfterPresent - time.Second).UnixMilli()
	lastPresentAt := now.Add(-windowLayerPresentFreshnessThreshold - 200*time.Millisecond).UnixMilli()

	message := buildWindowLayerRuntimeError(
		errors.New("x11 window for pid=128 title=\"RoboMaster Native Video Layer\" not found yet"),
		firstPresentAt,
		lastPresentAt,
		now,
	)

	if message != "" {
		t.Fatalf("expected stale present to suppress window-layer hard error, got=%q", message)
	}
}

func TestBuildFFplayLaunchAttemptsIncludeMuteAndHardwareFallback(t *testing.T) {
	layout := VideoWindowLayout{
		Left:   32,
		Top:    24,
		Width:  1280,
		Height: 720,
	}

	attempts := buildFFplayLaunchAttempts(layout, "hevc", nativeOfficialVideoWindowTitle, true)
	if len(attempts) != 1 {
		t.Fatalf("unexpected launch attempt count: %d", len(attempts))
	}

	firstArgs := attempts[0].args

	if !containsSequence(firstArgs, "-an") {
		t.Fatalf("expected ffplay args to include -an for mute")
	}
	if containsSequence(firstArgs, "-hwaccel", "auto") {
		t.Fatalf("software fallback should not keep hwaccel flag")
	}
	if !containsSequence(firstArgs, "-window_title", nativeOfficialVideoWindowTitle) {
		t.Fatalf("expected official window title in launch args")
	}
	if !containsSequence(attempts[0].env, "SDL_VIDEODRIVER=x11") {
		t.Fatalf("expected x11 SDL backend to be injected when requested")
	}
}

func TestVideoWindowLayoutRequiresProcessRestartForMajorRoleSwap(t *testing.T) {
	// What: 区分普通位置/层级重贴和主画面/PiP 尺寸切换。
	// Why: X11/XWayland 下普通 resize 可以直接 ConfigureWindow，但主副视频互换必须用新的 -x/-y 参数重启 ffplay 才稳定。
	fullscreen := VideoWindowLayout{
		Left:               1920,
		Top:                0,
		Width:              1920,
		Height:             1080,
		HUDWindowID:        0x1,
		StackAboveWindowID: 0x2,
	}
	moved := fullscreen
	moved.Left = 1932
	moved.Top = 24
	moved.StackAboveWindowID = 0x3
	smallResize := moved
	smallResize.Width = 1880
	smallResize.Height = 1058
	pip := moved
	pip.Width = 420
	pip.Height = 236

	if videoWindowLayoutRequiresProcessRestart(fullscreen, moved, true) {
		t.Fatalf("position and stacking changes should not require ffplay restart")
	}
	if !videoWindowLayoutRequiresProcessRestart(fullscreen, pip, true) {
		t.Fatalf("main/PiP size changes should restart ffplay when native resize is unavailable")
	}
	if videoWindowLayoutRequiresProcessRestart(fullscreen, smallResize, false) {
		t.Fatalf("small size changes should still use X11 resize without ffplay restart")
	}
	if !videoWindowLayoutRequiresProcessRestart(fullscreen, pip, false) {
		t.Fatalf("main/PiP size changes should restart ffplay even when native resize is available")
	}
}

func TestLaunchFFplayWithFallbackRetriesSoftwareMode(t *testing.T) {
	attempts := []ffplayLaunchAttempt{
		{
			mode: ffplayLaunchMode{label: "hardware(auto)", hwaccel: "auto"},
			args: []string{"-hwaccel", "auto"},
		},
		{
			mode: ffplayLaunchMode{label: "software"},
			args: []string{"-an"},
		},
	}

	callCount := 0
	result, err := launchFFplayWithFallback(attempts, func(args []string, env []string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, error) {
		callCount++
		if containsSequence(args, "-hwaccel", "auto") {
			return nil, nil, nil, errors.New("hwaccel init failed")
		}
		return &exec.Cmd{}, noopWriteCloser{}, io.NopCloser(nil), nil
	})
	if err != nil {
		t.Fatalf("expected software fallback to succeed, got err=%v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected two launch attempts, got=%d", callCount)
	}
	if result.mode.label != "software" {
		t.Fatalf("expected software fallback mode, got=%s", result.mode.label)
	}
}

func containsSequence(values []string, parts ...string) bool {
	if len(parts) == 0 {
		return true
	}

	for index := 0; index+len(parts) <= len(values); index++ {
		matched := true
		for partIndex, part := range parts {
			if values[index+partIndex] != part {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}

	return false
}
