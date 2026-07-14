package video

import "testing"

func buildHEVCNAL(nalType byte, payload ...byte) []byte {
	nal := []byte{0x00, 0x00, 0x00, 0x01, nalType << 1, 0x01}
	return append(nal, payload...)
}

func TestHEVCSyncGateDropsUntilRecoverableFrame(t *testing.T) {
	gate := NewHEVCSyncGate()

	pFrame := buildHEVCNAL(1, 0x11, 0x22, 0x33)
	decision := gate.Observe(pFrame)
	if decision.Pass {
		t.Fatalf("expected pre-sync P frame to be dropped")
	}
	if decision.DropCount != 1 {
		t.Fatalf("unexpected drop count: %d", decision.DropCount)
	}
	if gate.IsSynced() {
		t.Fatalf("gate should stay unsynced after plain P frame")
	}

	recoverable := append([]byte{}, buildHEVCNAL(hevcNALTypeVPS, 0xaa)...)
	recoverable = append(recoverable, buildHEVCNAL(hevcNALTypeSPS, 0xbb)...)
	recoverable = append(recoverable, buildHEVCNAL(hevcNALTypePPS, 0xcc)...)
	recoverable = append(recoverable, buildHEVCNAL(19, 0xdd)...)

	decision = gate.Observe(recoverable)
	if !decision.Pass || !decision.Recoverable {
		t.Fatalf("expected recoverable frame to unlock gate")
	}
	if !gate.IsSynced() {
		t.Fatalf("gate should become synced after recoverable frame")
	}

	decision = gate.Observe(pFrame)
	if !decision.Pass {
		t.Fatalf("expected post-sync P frame to pass")
	}
}

func TestHEVCSyncGateResetReturnsToUnsynced(t *testing.T) {
	gate := NewHEVCSyncGate()

	recoverable := append([]byte{}, buildHEVCNAL(hevcNALTypeVPS, 0xaa)...)
	recoverable = append(recoverable, buildHEVCNAL(hevcNALTypeSPS, 0xbb)...)
	recoverable = append(recoverable, buildHEVCNAL(hevcNALTypePPS, 0xcc)...)
	recoverable = append(recoverable, buildHEVCNAL(21, 0xdd)...)

	if decision := gate.Observe(recoverable); !decision.Pass {
		t.Fatalf("recoverable frame should pass")
	}
	gate.Reset()
	if gate.IsSynced() {
		t.Fatalf("gate should reset to unsynced")
	}

	pFrame := buildHEVCNAL(1, 0x44)
	decision := gate.Observe(pFrame)
	if decision.Pass {
		t.Fatalf("plain P frame should be dropped after reset")
	}
	if decision.DropCount != 1 {
		t.Fatalf("drop count should restart after reset, got=%d", decision.DropCount)
	}
}

func TestHEVCSyncGateRejectsParameterSetsAppearingAfterVCL(t *testing.T) {
	gate := NewHEVCSyncGate()

	frame := append([]byte{}, buildHEVCNAL(1, 0x11)...)
	frame = append(frame, buildHEVCNAL(hevcNALTypeVPS, 0xaa)...)
	frame = append(frame, buildHEVCNAL(hevcNALTypeSPS, 0xbb)...)
	frame = append(frame, buildHEVCNAL(hevcNALTypePPS, 0xcc)...)
	frame = append(frame, buildHEVCNAL(19, 0xdd)...)

	decision := gate.Observe(frame)
	if decision.Pass {
		t.Fatalf("expected frame with parameter sets after VCL to be rejected")
	}
	if gate.IsSynced() {
		t.Fatalf("gate should stay unsynced after invalid bootstrap ordering")
	}
}
