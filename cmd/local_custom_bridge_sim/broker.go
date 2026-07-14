package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
)

const (
	mqttPacketTypeConnect    = 0x01
	mqttPacketTypeConnack    = 0x02
	mqttPacketTypePublish    = 0x03
	mqttPacketTypeSubscribe  = 0x08
	mqttPacketTypeSuback     = 0x09
	mqttPacketTypePingreq    = 0x0C
	mqttPacketTypePingresp   = 0x0D
	mqttPacketTypeDisconnect = 0x0E

	mqttConnackAccepted           = 0x00
	mqttConnackIdentifierRejected = 0x02
)

type mqttPacket struct {
	packetType byte
	flags      byte
	body       []byte
}

type mqttClientConn struct {
	conn          net.Conn
	clientID      string
	subscriptions map[string]byte
	writeMu       sync.Mutex
}

type localMQTTBroker struct {
	cfg      simulatorConfig
	listener net.Listener

	mu      sync.Mutex
	clients map[*mqttClientConn]struct{}
}

func newLocalMQTTBroker(cfg simulatorConfig) (*localMQTTBroker, error) {
	return &localMQTTBroker{
		cfg:     cfg,
		clients: make(map[*mqttClientConn]struct{}),
	}, nil
}

func (b *localMQTTBroker) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", b.cfg.brokerAddress())
	if err != nil {
		return err
	}
	b.listener = listener

	log.Printf("[sim-broker] listening on %s", b.cfg.brokerAddress())

	go func() {
		<-ctx.Done()
		b.Close()
	}()

	go b.acceptLoop()
	return nil
}

func (b *localMQTTBroker) Close() {
	b.mu.Lock()
	clients := make([]*mqttClientConn, 0, len(b.clients))
	for client := range b.clients {
		clients = append(clients, client)
	}
	b.mu.Unlock()

	if b.listener != nil {
		_ = b.listener.Close()
	}

	for _, client := range clients {
		b.closeClient(client)
	}
}

func (b *localMQTTBroker) acceptLoop() {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return
		}

		client := &mqttClientConn{
			conn:          conn,
			subscriptions: make(map[string]byte),
		}

		b.mu.Lock()
		b.clients[client] = struct{}{}
		b.mu.Unlock()

		go b.handleClient(client)
	}
}

func (b *localMQTTBroker) handleClient(client *mqttClientConn) {
	defer b.closeClient(client)

	for {
		packet, err := readMQTTPacket(client.conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("[sim-broker] client=%s read failed: %v", client.clientID, err)
			}
			return
		}

		switch packet.packetType {
		case mqttPacketTypeConnect:
			clientID, err := parseMQTTConnectClientID(packet.body)
			if err != nil {
				log.Printf("[sim-broker] connect parse failed: %v", err)
				client.writeMu.Lock()
				_ = writeMQTTConnack(client.conn, mqttConnackIdentifierRejected)
				client.writeMu.Unlock()
				return
			}
			client.clientID = clientID
			if !b.isClientIDAllowed(clientID) {
				log.Printf("[sim-broker] reject client_id=%s", clientID)
				client.writeMu.Lock()
				_ = writeMQTTConnack(client.conn, mqttConnackIdentifierRejected)
				client.writeMu.Unlock()
				return
			}
			client.writeMu.Lock()
			err = writeMQTTConnack(client.conn, mqttConnackAccepted)
			client.writeMu.Unlock()
			if err != nil {
				return
			}
			log.Printf("[sim-broker] connected client_id=%s", client.clientID)

		case mqttPacketTypeSubscribe:
			packetID, topics, err := parseMQTTSubscribe(packet.body)
			if err != nil {
				log.Printf("[sim-broker] subscribe parse failed client=%s: %v", client.clientID, err)
				return
			}
			for topic, qos := range topics {
				client.subscriptions[topic] = qos
				log.Printf("[sim-broker] client=%s subscribed topic=%s qos=%d", client.clientID, topic, qos)
			}
			client.writeMu.Lock()
			err = writeMQTTSuback(client.conn, packetID, len(topics))
			client.writeMu.Unlock()
			if err != nil {
				return
			}

		case mqttPacketTypePingreq:
			client.writeMu.Lock()
			err = writeMQTTPingresp(client.conn)
			client.writeMu.Unlock()
			if err != nil {
				return
			}

		case mqttPacketTypeDisconnect:
			log.Printf("[sim-broker] disconnect client=%s", client.clientID)
			return

		case mqttPacketTypePublish:
			// 本地仿真链路当前不消费客户端发回的控制消息，只保持连接不报错。
			continue

		default:
			log.Printf("[sim-broker] ignore packet type=%d flags=0x%02x client=%s", packet.packetType, packet.flags, client.clientID)
		}
	}
}

func (b *localMQTTBroker) Publish(topic string, payload []byte) int {
	recipients := make([]*mqttClientConn, 0, 1)

	b.mu.Lock()
	for client := range b.clients {
		if _, subscribed := client.subscriptions[topic]; subscribed {
			recipients = append(recipients, client)
		}
	}
	b.mu.Unlock()

	if len(recipients) == 0 {
		return 0
	}

	packet := buildMQTTPublishPacket(topic, payload)
	delivered := 0
	for _, client := range recipients {
		client.writeMu.Lock()
		_, err := client.conn.Write(packet)
		client.writeMu.Unlock()
		if err != nil {
			log.Printf("[sim-broker] publish failed client=%s topic=%s: %v", client.clientID, topic, err)
			b.closeClient(client)
			continue
		}
		delivered++
	}

	return delivered
}

func (b *localMQTTBroker) subscriberCount(topic string) int {
	count := 0

	b.mu.Lock()
	defer b.mu.Unlock()

	for client := range b.clients {
		if _, subscribed := client.subscriptions[topic]; subscribed {
			count++
		}
	}

	return count
}

func (b *localMQTTBroker) isClientIDAllowed(clientID string) bool {
	if len(b.cfg.allowedClientIDs) == 0 {
		return true
	}
	_, allowed := b.cfg.allowedClientIDs[clientID]
	return allowed
}

func (b *localMQTTBroker) closeClient(client *mqttClientConn) {
	b.mu.Lock()
	if _, exists := b.clients[client]; !exists {
		b.mu.Unlock()
		return
	}
	delete(b.clients, client)
	b.mu.Unlock()

	_ = client.conn.Close()
}

func readMQTTPacket(r io.Reader) (mqttPacket, error) {
	fixedHeader := make([]byte, 1)
	if _, err := io.ReadFull(r, fixedHeader); err != nil {
		return mqttPacket{}, err
	}

	remainingLength, err := readMQTTRemainingLength(r)
	if err != nil {
		return mqttPacket{}, err
	}

	body := make([]byte, remainingLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return mqttPacket{}, err
	}

	return mqttPacket{
		packetType: fixedHeader[0] >> 4,
		flags:      fixedHeader[0] & 0x0F,
		body:       body,
	}, nil
}

func readMQTTRemainingLength(r io.Reader) (int, error) {
	multiplier := 1
	value := 0

	for {
		encoded := make([]byte, 1)
		if _, err := io.ReadFull(r, encoded); err != nil {
			return 0, err
		}

		value += int(encoded[0]&127) * multiplier
		if (encoded[0] & 128) == 0 {
			return value, nil
		}

		multiplier *= 128
		if multiplier > 128*128*128 {
			return 0, fmt.Errorf("mqtt remaining length overflow")
		}
	}
}

func parseMQTTConnectClientID(body []byte) (string, error) {
	const clientIDLengthOffset = 10

	if len(body) < clientIDLengthOffset+2 {
		return "", fmt.Errorf("connect body too short: %d", len(body))
	}

	clientIDLength := int(binary.BigEndian.Uint16(body[clientIDLengthOffset : clientIDLengthOffset+2]))
	clientIDStart := clientIDLengthOffset + 2
	clientIDEnd := clientIDStart + clientIDLength
	if clientIDEnd > len(body) {
		return "", fmt.Errorf("connect client_id overflow: len=%d body=%d", clientIDLength, len(body))
	}

	clientID := strings.TrimSpace(string(body[clientIDStart:clientIDEnd]))
	if clientID == "" {
		return "", fmt.Errorf("empty client_id")
	}

	return clientID, nil
}

func parseMQTTSubscribe(body []byte) (uint16, map[string]byte, error) {
	if len(body) < 5 {
		return 0, nil, fmt.Errorf("subscribe body too short: %d", len(body))
	}

	packetID := binary.BigEndian.Uint16(body[:2])
	topics := make(map[string]byte)
	offset := 2
	for offset < len(body) {
		if offset+2 > len(body) {
			return 0, nil, fmt.Errorf("subscribe topic length truncated")
		}

		topicLength := int(binary.BigEndian.Uint16(body[offset : offset+2]))
		offset += 2
		if offset+topicLength+1 > len(body) {
			return 0, nil, fmt.Errorf("subscribe topic payload truncated")
		}

		topic := string(body[offset : offset+topicLength])
		offset += topicLength
		requestedQoS := body[offset]
		offset++

		topics[topic] = requestedQoS
	}

	return packetID, topics, nil
}

func writeMQTTConnack(w io.Writer, returnCode byte) error {
	_, err := w.Write([]byte{
		byte(mqttPacketTypeConnack << 4),
		0x02,
		0x00,
		returnCode,
	})
	return err
}

func writeMQTTSuback(w io.Writer, packetID uint16, topicCount int) error {
	payload := make([]byte, 2, 2+topicCount)
	binary.BigEndian.PutUint16(payload[:2], packetID)
	for range topicCount {
		payload = append(payload, 0x02)
	}

	packet := append([]byte{byte(mqttPacketTypeSuback << 4)}, encodeMQTTRemainingLength(len(payload))...)
	packet = append(packet, payload...)
	_, err := w.Write(packet)
	return err
}

func writeMQTTPingresp(w io.Writer) error {
	_, err := w.Write([]byte{
		byte(mqttPacketTypePingresp << 4),
		0x00,
	})
	return err
}

func buildMQTTPublishPacket(topic string, payload []byte) []byte {
	topicBytes := []byte(topic)
	body := make([]byte, 0, 2+len(topicBytes)+len(payload))
	body = append(body, byte(len(topicBytes)>>8), byte(len(topicBytes)))
	body = append(body, topicBytes...)
	body = append(body, payload...)

	packet := append([]byte{byte(mqttPacketTypePublish << 4)}, encodeMQTTRemainingLength(len(body))...)
	packet = append(packet, body...)
	return packet
}

func encodeMQTTRemainingLength(length int) []byte {
	encoded := make([]byte, 0, 4)
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		encoded = append(encoded, digit)
		if length == 0 {
			return encoded
		}
	}
}
