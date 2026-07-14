package network

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// buildVideoPacket 按指定候选字节序构造一包最小可测的官方图传 UDP 数据。
// 之所以在测试里手工造包，而不是复用真实网络输入，是为了把“头解析正确性”与“网络环境抖动”完全隔离开。
func buildVideoPacket(order headerOrder, frameID, sliceID uint16, frameSize uint32, payload []byte) []byte {
	packet := make([]byte, packetHeaderSize+len(payload))

	// 这里直接按候选端序写入头部字段。
	// Why: 测试的核心就是验证接收器能否从这 8 个字节里恢复出正确 frameID / sliceID / frameSize。
	if order == headerOrderLittle {
		binary.LittleEndian.PutUint16(packet[0:2], frameID)
		binary.LittleEndian.PutUint16(packet[2:4], sliceID)
		binary.LittleEndian.PutUint32(packet[4:8], frameSize)
	} else {
		binary.BigEndian.PutUint16(packet[0:2], frameID)
		binary.BigEndian.PutUint16(packet[2:4], sliceID)
		binary.BigEndian.PutUint32(packet[4:8], frameSize)
	}

	copy(packet[packetHeaderSize:], payload)
	return packet
}

func TestDetectHeaderLittleEndianOfficialPacket(t *testing.T) {
	// 这里故意选择小端下合法、而大端下会超出 maxFrameSize 的 frameSize。
	// Why: 这样可以让测试准确验证“只有小端候选合法时，接收器能自动锁定小端”。
	receiver := &UDPReceiver{}
	payload := bytes.Repeat([]byte{0x11}, 20)
	packet := buildVideoPacket(headerOrderLittle, 7, 1, 300, payload)

	header, ok := receiver.detectHeader(packet)
	if !ok {
		t.Fatalf("detectHeader should accept little-endian official packet")
	}

	// 这里同时校验字段值与锁定状态。
	// Why: 只有两者都正确，后续 frameBuilder 才不会在 frameID 或 frameSize 上继续带错。
	if receiver.headerOrder != headerOrderLittle {
		t.Fatalf("header order mismatch: got=%s want=%s", receiver.headerOrder.String(), headerOrderLittle.String())
	}
	if header.frameID != 7 || header.sliceID != 1 || header.frameSize != 300 {
		t.Fatalf("unexpected header parsed: %+v", header)
	}
}

func TestDetectHeaderFallsBackWhenLockedOrderInvalid(t *testing.T) {
	// 先把接收器人为设成错误的大端锁定状态。
	// Why: 该场景对应“历史代码固定大端，但现在切到官方小端实流”的现场故障。
	receiver := &UDPReceiver{headerOrder: headerOrderBig}
	payload := bytes.Repeat([]byte{0x22}, 32)
	packet := buildVideoPacket(headerOrderLittle, 9, 2, 400, payload)

	header, ok := receiver.detectHeader(packet)
	if !ok {
		t.Fatalf("detectHeader should switch to valid little-endian order")
	}

	// 切换后既要返回正确字段，也要把内部锁定状态改掉。
	// Why: 如果只当前包临时成功、却不更新 receiver 状态，后续所有包仍然会继续失败。
	if receiver.headerOrder != headerOrderLittle {
		t.Fatalf("header order should switch to little-endian, got=%s", receiver.headerOrder.String())
	}
	if header.frameID != 9 || header.sliceID != 2 || header.frameSize != 400 {
		t.Fatalf("unexpected switched header parsed: %+v", header)
	}
}

func TestAssembleAcceptsSlicesStartingAtZero(t *testing.T) {
	// 起始分片为 0 是旧实现默认支持的路径。
	// Why: 这条用例确保这次改动在兼容官方新流的同时，不会把已有测试源打坏。
	builder := &frameBuilder{
		frameSize: 4,
		slices: map[uint16][]byte{
			0: {0x01, 0x02},
			1: {0x03, 0x04},
		},
	}

	frame, minSliceID, maxSliceID, err := assemble(builder)
	if err != nil {
		t.Fatalf("assemble should accept zero-based slices: %v", err)
	}
	if minSliceID != 0 || maxSliceID != 1 {
		t.Fatalf("unexpected slice range: %d-%d", minSliceID, maxSliceID)
	}
	if !bytes.Equal(frame, []byte{0x01, 0x02, 0x03, 0x04}) {
		t.Fatalf("unexpected assembled frame: %v", frame)
	}
}

func TestAssembleAcceptsSlicesStartingAtOne(t *testing.T) {
	// 起始分片为 1 是本次新增兼容路径。
	// Why: 这条用例直接锁住“官方流首分片不一定从 0 开始”的修复目标。
	builder := &frameBuilder{
		frameSize: 4,
		slices: map[uint16][]byte{
			1: {0x0A, 0x0B},
			2: {0x0C, 0x0D},
		},
	}

	frame, minSliceID, maxSliceID, err := assemble(builder)
	if err != nil {
		t.Fatalf("assemble should accept one-based slices: %v", err)
	}
	if minSliceID != 1 || maxSliceID != 2 {
		t.Fatalf("unexpected slice range: %d-%d", minSliceID, maxSliceID)
	}
	if !bytes.Equal(frame, []byte{0x0A, 0x0B, 0x0C, 0x0D}) {
		t.Fatalf("unexpected assembled frame: %v", frame)
	}
}

func TestAssembleRejectsGapBetweenSlices(t *testing.T) {
	// 中间分片缺口必须被拒绝。
	// Why: 如果允许靠总字节数“蒙混过关”，下游 FFmpeg 很容易直接收到破碎 HEVC 帧并报错。
	builder := &frameBuilder{
		frameSize: 2,
		slices: map[uint16][]byte{
			0: {0x01},
			2: {0x02},
		},
	}

	_, _, _, err := assemble(builder)
	if err == nil {
		t.Fatalf("assemble should reject missing middle slice")
	}
}

func TestAssembleRejectsFrameSizeMismatch(t *testing.T) {
	// 这里故意让拼接结果比头部 frameSize 少 1 字节。
	// Why: 这条用例锁住“必须精确等长”这道最终保险，避免错误 frameSize 继续漏进解码器。
	builder := &frameBuilder{
		frameSize: 3,
		slices: map[uint16][]byte{
			0: {0x01},
			1: {0x02},
		},
	}

	_, _, _, err := assemble(builder)
	if err == nil {
		t.Fatalf("assemble should reject frame-size mismatch")
	}
}

func TestUDPReceiverAssemblesOfficialFrameFromSocketPackets(t *testing.T) {
	frame := buildNetworkTestHEVCFrame()
	receiverFrameCh := make(chan []byte, 1)

	receiver, err := NewUDPReceiver(0, func(frame []byte) {
		receiverFrameCh <- append([]byte(nil), frame...)
	})
	if err != nil {
		t.Fatalf("create UDP receiver failed: %v", err)
	}
	defer receiver.Stop()

	udpAddr, ok := receiver.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("unexpected receiver addr type: %T", receiver.conn.LocalAddr())
	}

	receiver.Start()

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.Fatalf("dial test UDP receiver failed: %v", err)
	}
	defer conn.Close()

	chunks := [][]byte{
		frame[:7],
		frame[7:31],
		frame[31:],
	}
	for index, chunk := range chunks {
		packet := buildVideoPacket(headerOrderLittle, 23, uint16(index+1), uint32(len(frame)), chunk)
		if _, err := conn.Write(packet); err != nil {
			t.Fatalf("write UDP packet %d failed: %v", index, err)
		}
	}

	select {
	case got := <-receiverFrameCh:
		if !bytes.Equal(got, frame) {
			t.Fatalf("unexpected assembled frame: got=%x want=%x", got, frame)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for assembled official frame")
	}

	stats := receiver.StatsSnapshot()
	if stats.PacketCount != 3 {
		t.Fatalf("unexpected packet count: %d", stats.PacketCount)
	}
	if stats.FrameCount != 1 {
		t.Fatalf("unexpected frame count: %d", stats.FrameCount)
	}
	if stats.LastPacketAt == 0 || stats.LastFrameAt == 0 {
		t.Fatalf("expected packet/frame timestamps, got packet=%d frame=%d", stats.LastPacketAt, stats.LastFrameAt)
	}
	if stats.HeaderOrder != headerOrderLittle.String() {
		t.Fatalf("unexpected header order: %s", stats.HeaderOrder)
	}
}

func buildNetworkTestHEVCFrame() []byte {
	frame := append([]byte{}, buildNetworkTestHEVCNAL(32, 0xaa, 0xbb)...)
	frame = append(frame, buildNetworkTestHEVCNAL(33, 0xcc, 0xdd)...)
	frame = append(frame, buildNetworkTestHEVCNAL(34, 0xee, 0xff)...)
	frame = append(frame, buildNetworkTestHEVCNAL(19, 0x11, 0x22, 0x33)...)
	return frame
}

func buildNetworkTestHEVCNAL(nalType byte, payload ...byte) []byte {
	nal := []byte{0x00, 0x00, 0x00, 0x01, nalType << 1, 0x01}
	return append(nal, payload...)
}
