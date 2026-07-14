package network

import (
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func TestOrderMQTTClientIDCandidatesPrefersConfiguredID(t *testing.T) {
	// What: 构造一份同时包含 preferredID 和重复值的候选表。
	// Why: 自动探测必须保证首选 ID 排在最前，且不会重复探测同一个候选。
	ordered := orderMQTTClientIDCandidates(101, []uint16{1, 101, 2, 1, 102})

	expected := []uint16{101, 1, 2, 102}
	if len(ordered) != len(expected) {
		t.Fatalf("unexpected ordered length: got=%d want=%d values=%v", len(ordered), len(expected), ordered)
	}

	for index, want := range expected {
		if ordered[index] != want {
			t.Fatalf("unexpected candidate at index=%d got=%d want=%d ordered=%v", index, ordered[index], want, ordered)
		}
	}
}

func TestDetectAcceptedMQTTClientIDFindsBrokerAcceptedRobotID(t *testing.T) {
	// What: 起一个最小本地 mock broker，仅对 clientID=1 返回成功。
	// Why: 这条用例锁住“首选 ID 错误时，会继续探测并切到真正被 broker 接受的机器人 ID”。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock broker failed: %v", err)
	}
	defer listener.Close()

	doneCh := make(chan struct{})
	defer close(doneCh)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-doneCh:
					return
				default:
					return
				}
			}

			go func(connection net.Conn) {
				defer connection.Close()

				// What: 读取固定头前两字节。
				// Why: 这样就能知道 CONNECT 剩余载荷长度，从而完整收下本次探测报文。
				header := make([]byte, 2)
				if _, err := io.ReadFull(connection, header); err != nil {
					return
				}

				body := make([]byte, int(header[1]))
				if _, err := io.ReadFull(connection, body); err != nil {
					return
				}

				// What: MQTT 3.1.1 CONNECT 的可变头固定长度是 10 字节。
				// Why: clientID 长度字段总是紧跟在这 10 字节之后，不能用“body 末尾倒推”的方式猜偏移。
				clientIDLengthOffset := 10
				if len(body) < clientIDLengthOffset+2 {
					return
				}

				clientIDLength := int(body[clientIDLengthOffset])<<8 | int(body[clientIDLengthOffset+1])
				if clientIDLengthOffset+2+clientIDLength > len(body) {
					return
				}

				clientID := string(body[clientIDLengthOffset+2 : clientIDLengthOffset+2+clientIDLength])
				connack := []byte{mqttPacketTypeConnack, 0x02, 0x00, mqttConnackIdentifierRejected}
				if clientID == "1" {
					connack[3] = mqttConnackAccepted
				}

				_, _ = connection.Write(connack)
			}(conn)
		}
	}()

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split listener addr failed: %v", err)
	}

	var port int
	if _, err := fmt.Sscanf(portText, "%d", &port); err != nil {
		t.Fatalf("parse mock broker port failed: %v", err)
	}

	resolvedClientID, err := DetectAcceptedMQTTClientID(host, port, 101, []uint16{1, 101}, time.Second)
	if err != nil {
		t.Fatalf("detect accepted client id failed: %v", err)
	}
	if resolvedClientID != 1 {
		t.Fatalf("unexpected resolved client id: got=%d want=%d", resolvedClientID, 1)
	}
}
