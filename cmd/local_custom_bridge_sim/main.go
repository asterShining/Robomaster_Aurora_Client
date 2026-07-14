package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type simulatorConfig struct {
	serialPort       string
	serialLink       string
	brokerHost       string
	brokerPort       int
	topic            string
	blockBytes       int
	publishInterval  time.Duration
	logInterval      time.Duration
	maxPendingBytes  int
	maxQueueBlocks   int
	allowedClientIDs map[string]struct{}
}

func defaultAllowedClientIDs() map[string]struct{} {
	return map[string]struct{}{
		"1":   {},
		"101": {},
	}
}

func parseAllowedClientIDs(raw string) map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, token := range strings.Split(raw, ",") {
		normalized := strings.TrimSpace(token)
		if normalized == "" {
			continue
		}
		allowed[normalized] = struct{}{}
	}
	if len(allowed) == 0 {
		return defaultAllowedClientIDs()
	}
	return allowed
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg := simulatorConfig{}
	var allowedClientIDs string

	flag.StringVar(&cfg.serialPort, "serial-port", "", "existing serial/PTy path to read raw H.264 bytes from")
	flag.StringVar(&cfg.serialLink, "serial-link", "/tmp/rm_virtual_lower_tx", "symlink path exposing a generated PTY slave to the upper program; set empty to use serial-port directly")
	flag.StringVar(&cfg.brokerHost, "broker-host", "127.0.0.1", "host for the local MQTT broker")
	flag.IntVar(&cfg.brokerPort, "broker-port", 3333, "port for the local MQTT broker")
	flag.StringVar(&cfg.topic, "topic", "CustomByteBlock", "MQTT topic used to publish custom byte blocks")
	flag.IntVar(&cfg.blockBytes, "block-bytes", 300, "size of each custom byte block")
	flag.DurationVar(&cfg.publishInterval, "publish-interval", 20*time.Millisecond, "interval between published blocks")
	flag.DurationVar(&cfg.logInterval, "log-interval", time.Second, "bridge statistics log interval")
	flag.IntVar(&cfg.maxPendingBytes, "max-pending-bytes", 8192, "maximum pending raw-byte buffer")
	flag.IntVar(&cfg.maxQueueBlocks, "max-queue-blocks", 24, "maximum queued blocks waiting to publish")
	flag.StringVar(&allowedClientIDs, "allow-client-ids", "1,101", "comma-separated MQTT client IDs accepted by the local broker")
	flag.Parse()

	cfg.allowedClientIDs = parseAllowedClientIDs(allowedClientIDs)
	if cfg.blockBytes <= 0 {
		log.Fatalf("invalid block-bytes=%d", cfg.blockBytes)
	}
	if cfg.publishInterval <= 0 {
		log.Fatalf("invalid publish-interval=%s", cfg.publishInterval)
	}
	if cfg.maxPendingBytes < cfg.blockBytes {
		log.Fatalf("max-pending-bytes=%d must be >= block-bytes=%d", cfg.maxPendingBytes, cfg.blockBytes)
	}
	if cfg.maxQueueBlocks <= 0 {
		log.Fatalf("invalid max-queue-blocks=%d", cfg.maxQueueBlocks)
	}
	if cfg.serialPort == "" && cfg.serialLink == "" {
		log.Fatalf("either serial-port or serial-link must be configured")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	broker, err := newLocalMQTTBroker(cfg)
	if err != nil {
		log.Fatalf("create local broker failed: %v", err)
	}
	defer broker.Close()

	if err := broker.Start(ctx); err != nil {
		log.Fatalf("start local broker failed: %v", err)
	}

	bridge, err := newSerialCustomBridge(cfg, broker)
	if err != nil {
		log.Fatalf("create serial bridge failed: %v", err)
	}

	log.Printf(
		"[sim] serial_port=%s serial_link=%s broker=%s:%d topic=%s block_bytes=%d publish_interval=%s",
		cfg.serialPort,
		cfg.serialLink,
		cfg.brokerHost,
		cfg.brokerPort,
		cfg.topic,
		cfg.blockBytes,
		cfg.publishInterval,
	)

	if err := bridge.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("serial bridge exited with error: %v", err)
	}

	log.Printf("[sim] shutdown complete")
}

func (cfg simulatorConfig) brokerAddress() string {
	return fmt.Sprintf("%s:%d", cfg.brokerHost, cfg.brokerPort)
}
