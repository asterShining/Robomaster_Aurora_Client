package video

import (
	"bytes"
	"testing"
)

func TestH264ReassemblerWaitsForRecoverableIDR(t *testing.T) {
	// What: 构造“先收到一个普通 P slice，再收到 SPS/PPS/IDR，最后再收到下一张 P slice”的连续码流。
	// Why: 这正是用户在码流中途切到 custom 的真实场景；重组器必须先丢掉无法解码的预同步 P 帧，等到下一次带参数集的 IDR 后再恢复输出。
	reassembler := NewH264Reassembler()

	pSliceBeforeSync := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x11}, 24)...)
	sps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x1e, 0xf4}, bytes.Repeat([]byte{0x22}, 8)...)
	pps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xce, 0x38, 0x80}, bytes.Repeat([]byte{0x33}, 4)...)
	idr := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x80}, bytes.Repeat([]byte{0x44}, 18)...)
	nextPSlice := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x55}, 12)...)
	nextPSlice2 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x66}, 10)...)
	nextPSlice3 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x77}, 6)...)

	stream := append(append(append(append([]byte{}, pSliceBeforeSync...), sps...), pps...), idr...)
	stream = append(stream, nextPSlice...)
	stream = append(stream, nextPSlice2...)
	stream = append(stream, nextPSlice3...)

	if got := reassembler.Feed(stream[:80]); got != nil {
		t.Fatalf("expected no AU before recoverable IDR, got %d bytes", len(got))
	}

	firstAU := reassembler.Feed(stream[80:])
	if firstAU == nil {
		t.Fatalf("expected first recoverable AU after IDR boundary")
	}

	expectedIDRAU := append(append(append([]byte{}, sps...), pps...), idr...)
	if !bytes.Equal(firstAU, expectedIDRAU) {
		t.Fatalf("unexpected recoverable IDR AU length got=%d want=%d", len(firstAU), len(expectedIDRAU))
	}

	secondAU := reassembler.Feed(nil)
	if secondAU == nil {
		t.Fatalf("expected queued next P AU to be drainable")
	}
	if !bytes.Equal(secondAU, nextPSlice) {
		t.Fatalf("unexpected queued P AU length got=%d want=%d", len(secondAU), len(nextPSlice))
	}

	if extraAU := reassembler.Feed(nil); extraAU != nil {
		t.Fatalf("expected output queue to be empty, got %d bytes", len(extraAU))
	}
}

func TestH264ReassemblerPrependsCachedParameterSetsToIDR(t *testing.T) {
	// What: 先单独收到 SPS/PPS，再收到一个带边界的 IDR AU。
	// Why: 参数集和 IDR 不一定总是落在同一个 chunk 里；重组器必须缓存它们，并在 IDR 真正到来时自动补齐到同一 AU 前面。
	reassembler := NewH264Reassembler()

	sps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x64, 0x00, 0x1f}, bytes.Repeat([]byte{0x12}, 6)...)
	pps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xee, 0x06, 0xf2}, bytes.Repeat([]byte{0x23}, 3)...)
	idr := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x80}, bytes.Repeat([]byte{0x34}, 16)...)
	nextPSlice := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x45}, 10)...)
	nextPSlice2 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x46}, 6)...)

	stream := append(append(append([]byte{}, sps...), pps...), idr...)
	stream = append(stream, nextPSlice...)
	stream = append(stream, nextPSlice2...)

	got := reassembler.Feed(stream)
	if got == nil {
		t.Fatalf("expected cached parameter sets to unlock first IDR AU")
	}

	expected := append(append(append([]byte{}, sps...), pps...), idr...)
	if !bytes.Equal(got, expected) {
		t.Fatalf("unexpected IDR AU with cached parameter sets length got=%d want=%d", len(got), len(expected))
	}
}

func TestH264ReassemblerResyncPreservesParameterSets(t *testing.T) {
	// What: 模拟 custom ffplay 因切源/窗口尺寸变化重启，但上游 H.264 字节流没有断。
	// Why: 重启显示进程后只应等待下一个 IDR，不能丢掉已经缓存的 SPS/PPS，否则会额外黑屏并放大坏块恢复时间。
	reassembler := NewH264Reassembler()

	sps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x1e}, bytes.Repeat([]byte{0x11}, 4)...)
	pps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xce, 0x38, 0x80}, bytes.Repeat([]byte{0x22}, 2)...)
	idr := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x84}, bytes.Repeat([]byte{0x33}, 14)...)
	pSlice := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x44}, 9)...)
	pSlice2 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x45}, 6)...)
	nextIDR := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x84}, bytes.Repeat([]byte{0x55}, 12)...)
	nextPSlice := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x66}, 6)...)
	nextPSlice2 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x67}, 4)...)

	initialStream := append(append(append(append([]byte{}, sps...), pps...), idr...), pSlice...)
	initialStream = append(initialStream, pSlice2...)
	if firstAU := reassembler.Feed(initialStream); firstAU == nil {
		t.Fatalf("expected initial recoverable AU")
	}
	if !reassembler.Diagnostics().Synced {
		t.Fatalf("expected synced reassembler after initial IDR")
	}

	reassembler.ResyncPreservingParameterSets()
	diag := reassembler.Diagnostics()
	if diag.Synced || !diag.HasCachedSPS || !diag.HasCachedPPS {
		t.Fatalf("expected resync to keep SPS/PPS while marking stream unsynced: %+v", diag)
	}

	streamAfterRestart := append(append([]byte{}, nextIDR...), nextPSlice...)
	streamAfterRestart = append(streamAfterRestart, nextPSlice2...)
	got := reassembler.Feed(streamAfterRestart)
	if got == nil {
		t.Fatalf("expected next IDR to recover using cached SPS/PPS")
	}

	expected := append(append(append([]byte{}, sps...), pps...), nextIDR...)
	if !bytes.Equal(got, expected) {
		t.Fatalf("unexpected recovered IDR AU length got=%d want=%d", len(got), len(expected))
	}
}

func TestH264ReassemblerSupportsDrainAfterSingleChunkMultipleAUs(t *testing.T) {
	// What: 构造一个单次 Feed 就同时跨过多个 AU 边界的字节流。
	// Why: 0x0310 chunk 大小和编码 AU 大小并不对齐；调用方必须能在一次 chunk 后把所有已完成 AU 持续排空。
	reassembler := NewH264Reassembler()

	sps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x1e}, bytes.Repeat([]byte{0x61}, 4)...)
	pps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xce, 0x38, 0x80}, bytes.Repeat([]byte{0x62}, 2)...)
	idr := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x84}, bytes.Repeat([]byte{0x63}, 14)...)
	pSlice1 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x64}, 9)...)
	pSlice2 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x65}, 7)...)
	pSlice3 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x66}, 5)...)
	pSlice4 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x67}, 4)...)

	stream := append(append(append(append([]byte{}, sps...), pps...), idr...), pSlice1...)
	stream = append(stream, pSlice2...)
	stream = append(stream, pSlice3...)
	stream = append(stream, pSlice4...)

	firstAU := reassembler.Feed(stream)
	if firstAU == nil {
		t.Fatalf("expected first AU from single chunk")
	}

	expectedIDR := append(append(append([]byte{}, sps...), pps...), idr...)
	if !bytes.Equal(firstAU, expectedIDR) {
		t.Fatalf("unexpected first AU length got=%d want=%d", len(firstAU), len(expectedIDR))
	}

	secondAU := reassembler.Feed(nil)
	if secondAU == nil {
		t.Fatalf("expected second AU to be drainable from queue")
	}
	if !bytes.Equal(secondAU, pSlice1) {
		t.Fatalf("unexpected second AU length got=%d want=%d", len(secondAU), len(pSlice1))
	}
}

func TestH264ReassemblerKeepsContinuationSlicesInSameAU(t *testing.T) {
	// What: 构造一个 IDR 后跟两个 `first_mb_in_slice > 0` 的续 slice，再接下一帧 P slice 的码流。
	// Why: 上位机启用 x264 sliced threads 时，同一帧会被拆成多个 VCL NAL；客户端必须把这些续 slice 留在同一个 AU 里，不能错误拆成多帧导致 ffplay 间歇坏帧。
	reassembler := NewH264Reassembler()

	sps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x1e}, bytes.Repeat([]byte{0x61}, 4)...)
	pps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xce, 0x38, 0x80}, bytes.Repeat([]byte{0x62}, 2)...)
	idrFirstSlice := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x88}, bytes.Repeat([]byte{0x63}, 12)...)
	idrContinuationSlice1 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x48}, bytes.Repeat([]byte{0x64}, 9)...)
	idrContinuationSlice2 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x28}, bytes.Repeat([]byte{0x65}, 7)...)
	nextPSlice := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x66}, 8)...)
	nextPSlice2 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x67}, 5)...)
	nextPSlice3 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x68}, 4)...)

	stream := append(append(append(append(append([]byte{}, sps...), pps...), idrFirstSlice...), idrContinuationSlice1...), idrContinuationSlice2...)
	stream = append(stream, nextPSlice...)
	stream = append(stream, nextPSlice2...)
	stream = append(stream, nextPSlice3...)

	firstAU := reassembler.Feed(stream)
	if firstAU == nil {
		t.Fatalf("expected first AU")
	}

	expectedIDR := append(append(append(append(append([]byte{}, sps...), pps...), idrFirstSlice...), idrContinuationSlice1...), idrContinuationSlice2...)
	if !bytes.Equal(firstAU, expectedIDR) {
		t.Fatalf("continuation slices should stay in IDR AU, got=%d want=%d", len(firstAU), len(expectedIDR))
	}

	secondAU := reassembler.Feed(nil)
	if !bytes.Equal(secondAU, nextPSlice) {
		t.Fatalf("unexpected next AU after sliced IDR")
	}
}

func TestH264ReassemblerDiagnosticsTrackSyncInputs(t *testing.T) {
	reassembler := NewH264Reassembler()

	sps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x1e}, bytes.Repeat([]byte{0x61}, 4)...)
	pps := append([]byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xce, 0x38, 0x80}, bytes.Repeat([]byte{0x62}, 2)...)
	idr := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x84}, bytes.Repeat([]byte{0x63}, 14)...)
	pSlice := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x64}, 9)...)
	pSlice2 := append([]byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x80}, bytes.Repeat([]byte{0x65}, 7)...)
	stream := append(append(append(append([]byte{}, sps...), pps...), idr...), pSlice...)
	stream = append(stream, pSlice2...)

	if firstAU := reassembler.Feed(stream); firstAU == nil {
		t.Fatalf("expected recoverable AU")
	}

	diag := reassembler.Diagnostics()
	if diag.NALTotal == 0 || diag.SPS != 1 || diag.PPS != 1 || diag.IDR != 1 || !diag.HasCachedSPS || !diag.HasCachedPPS || !diag.Synced {
		t.Fatalf("unexpected h264 diagnostics: %+v", diag)
	}
}
