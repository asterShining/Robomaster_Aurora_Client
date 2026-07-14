package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

const (
	packetHeaderSize = 8
)

func main() {
	var (
		addr       string
		duration   time.Duration
		interval   time.Duration
		sliceBytes int
		startSlice uint
		endian     string
	)

	flag.StringVar(&addr, "addr", "127.0.0.1:3334", "UDP target address")
	flag.DurationVar(&duration, "duration", 10*time.Second, "how long to inject frames")
	flag.DurationVar(&interval, "interval", 33*time.Millisecond, "interval between synthetic HEVC access units")
	flag.IntVar(&sliceBytes, "slice-bytes", 64, "maximum payload bytes per UDP fragment")
	flag.UintVar(&startSlice, "start-slice", 1, "first slice id to use in each frame")
	flag.StringVar(&endian, "endian", "little", "packet header byte order: little or big")
	flag.Parse()

	if duration <= 0 {
		log.Fatalf("duration must be positive, got %s", duration)
	}
	if interval <= 0 {
		log.Fatalf("interval must be positive, got %s", interval)
	}
	if sliceBytes <= 0 {
		log.Fatalf("slice-bytes must be positive, got %d", sliceBytes)
	}
	if startSlice > 0xffff {
		log.Fatalf("start-slice out of uint16 range: %d", startSlice)
	}

	order, err := parseByteOrder(endian)
	if err != nil {
		log.Fatal(err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatalf("resolve UDP addr %q failed: %v", addr, err)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		log.Fatalf("dial UDP %q failed: %v", addr, err)
	}
	defer conn.Close()

	frame := buildSyntheticHEVCBootstrapFrame()
	if len(frame) > 0xffffffff {
		log.Fatalf("synthetic frame too large: %d", len(frame))
	}

	deadline := time.Now().Add(duration)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var (
		frameID uint16 = 1
		frames  uint64
		packets uint64
	)

	log.Printf("[official-injector] target=%s duration=%s interval=%s frame_bytes=%d slice_bytes=%d start_slice=%d endian=%s",
		addr, duration, interval, len(frame), sliceBytes, startSlice, endian)

	for {
		if time.Now().After(deadline) {
			break
		}

		sentPackets, err := sendFrame(conn, order, frameID, uint16(startSlice), uint32(len(frame)), frame, sliceBytes)
		if err != nil {
			log.Fatalf("send frame %d failed: %v", frameID, err)
		}
		frames++
		packets += uint64(sentPackets)
		frameID++

		<-ticker.C
	}

	log.Printf("[official-injector] done frames=%d packets=%d", frames, packets)
}

func parseByteOrder(raw string) (binary.ByteOrder, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "little", "le":
		return binary.LittleEndian, nil
	case "big", "be":
		return binary.BigEndian, nil
	default:
		return nil, fmt.Errorf("unsupported endian %q", raw)
	}
}

func sendFrame(conn *net.UDPConn, order binary.ByteOrder, frameID uint16, startSliceID uint16, frameSize uint32, frame []byte, sliceBytes int) (int, error) {
	sliceID := startSliceID
	packetCount := 0

	for offset := 0; offset < len(frame); offset += sliceBytes {
		end := offset + sliceBytes
		if end > len(frame) {
			end = len(frame)
		}

		packet := make([]byte, packetHeaderSize+end-offset)
		order.PutUint16(packet[0:2], frameID)
		order.PutUint16(packet[2:4], sliceID)
		order.PutUint32(packet[4:8], frameSize)
		copy(packet[packetHeaderSize:], frame[offset:end])

		if _, err := conn.Write(packet); err != nil {
			return packetCount, err
		}

		packetCount++
		sliceID++
	}

	return packetCount, nil
}

func buildSyntheticHEVCBootstrapFrame() []byte {
	frame := make([]byte, 0, 256)
	frame = append(frame, buildHEVCNAL(32, repeatByte(0xaa, 24)...)...)
	frame = append(frame, buildHEVCNAL(33, repeatByte(0xbb, 32)...)...)
	frame = append(frame, buildHEVCNAL(34, repeatByte(0xcc, 16)...)...)
	frame = append(frame, buildHEVCNAL(19, repeatByte(0xdd, 128)...)...)
	return frame
}

func buildHEVCNAL(nalType byte, payload ...byte) []byte {
	nal := []byte{0x00, 0x00, 0x00, 0x01, nalType << 1, 0x01}
	return append(nal, payload...)
}

func repeatByte(value byte, count int) []byte {
	if count <= 0 {
		return nil
	}
	data := make([]byte, count)
	for i := range data {
		data[i] = value
	}
	return data
}
