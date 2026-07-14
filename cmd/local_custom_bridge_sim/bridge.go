package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"rm-aurora/pkg/rmcp"

	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"
)

type serialBridgeStats struct {
	mu sync.Mutex

	rawBytes       uint64
	publishedBlock uint64
	queueDrops     uint64
	pendingDrops   uint64
	lastReadAt     time.Time
	lastPublishAt  time.Time
}

type serialCustomBridge struct {
	cfg    simulatorConfig
	broker *localMQTTBroker
	stats  serialBridgeStats
}

type serialInput struct {
	readFile         *os.File
	displayPath      string
	physicalReadPath string
	cleanup          func()
}

func newSerialCustomBridge(cfg simulatorConfig, broker *localMQTTBroker) (*serialCustomBridge, error) {
	return &serialCustomBridge{
		cfg:    cfg,
		broker: broker,
	}, nil
}

func (b *serialCustomBridge) Run(ctx context.Context) error {
	input, err := openSerialInput(b.cfg)
	if err != nil {
		return err
	}
	defer input.cleanup()

	log.Printf(
		"[sim-pty] ready exposed_path=%s read_path=%s",
		input.displayPath,
		input.physicalReadPath,
	)

	rawChunkCh := make(chan []byte, 64)
	readErrCh := make(chan error, 1)
	go b.readLoop(ctx, input.readFile, rawChunkCh, readErrCh)

	publishTicker := time.NewTicker(b.cfg.publishInterval)
	defer publishTicker.Stop()

	logTicker := time.NewTicker(b.cfg.logInterval)
	defer logTicker.Stop()

	pending := make([]byte, 0, b.cfg.maxPendingBytes)
	queue := make([][]byte, 0, b.cfg.maxQueueBlocks)

	for {
		select {
		case <-ctx.Done():
			return context.Canceled

		case err := <-readErrCh:
			return err

		case chunk := <-rawChunkCh:
			pending = b.appendPending(pending, chunk)
			queue, pending = b.packBlocks(queue, pending)

		case <-publishTicker.C:
			if len(queue) == 0 {
				continue
			}

			block := queue[0]
			queue = queue[1:]

			blockPayload, err := proto.Marshal(&rmcp.CustomByteBlock{Data: block})
			if err != nil {
				return err
			}

			delivered := b.broker.Publish(b.cfg.topic, blockPayload)
			b.notePublish()
			if delivered > 0 {
				log.Printf(
					"[sim-bridge] published topic=%s block_bytes=%d delivered=%d queued=%d pending=%d",
					b.cfg.topic,
					len(block),
					delivered,
					len(queue),
					len(pending),
				)
			}

		case <-logTicker.C:
			b.logSnapshot(len(queue), len(pending), b.broker.subscriberCount(b.cfg.topic))
		}
	}
}

func (b *serialCustomBridge) readLoop(ctx context.Context, serialFile *os.File, rawChunkCh chan<- []byte, readErrCh chan<- error) {
	buffer := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		readBytes, err := serialFile.Read(buffer)
		if readBytes > 0 {
			chunk := append([]byte(nil), buffer[:readBytes]...)
			b.noteRead(readBytes)
			rawChunkCh <- chunk
		}

		if err == nil {
			continue
		}
		if isRecoverableSerialReadError(err) {
			time.Sleep(20 * time.Millisecond)
			continue
		}

		readErrCh <- err
		return
	}
}

func (b *serialCustomBridge) appendPending(pending []byte, chunk []byte) []byte {
	if len(chunk) == 0 {
		return pending
	}

	if len(chunk) >= b.cfg.maxPendingBytes {
		b.notePendingDrop(len(chunk) - b.cfg.maxPendingBytes)
		return append(pending[:0], chunk[len(chunk)-b.cfg.maxPendingBytes:]...)
	}

	if len(pending)+len(chunk) > b.cfg.maxPendingBytes {
		dropBytes := len(pending) + len(chunk) - b.cfg.maxPendingBytes
		b.notePendingDrop(dropBytes)
		pending = append(pending[:0], pending[dropBytes:]...)
	}

	return append(pending, chunk...)
}

func (b *serialCustomBridge) packBlocks(queue [][]byte, pending []byte) ([][]byte, []byte) {
	for len(pending) >= b.cfg.blockBytes {
		block := append([]byte(nil), pending[:b.cfg.blockBytes]...)
		pending = append(pending[:0], pending[b.cfg.blockBytes:]...)

		if len(queue) >= b.cfg.maxQueueBlocks {
			queue = append(queue[:0], queue[1:]...)
			b.noteQueueDrop()
		}
		queue = append(queue, block)
	}

	return queue, pending
}

func (b *serialCustomBridge) noteRead(readBytes int) {
	b.stats.mu.Lock()
	defer b.stats.mu.Unlock()

	b.stats.rawBytes += uint64(readBytes)
	b.stats.lastReadAt = time.Now()
}

func (b *serialCustomBridge) notePublish() {
	b.stats.mu.Lock()
	defer b.stats.mu.Unlock()

	b.stats.publishedBlock++
	b.stats.lastPublishAt = time.Now()
}

func (b *serialCustomBridge) noteQueueDrop() {
	b.stats.mu.Lock()
	defer b.stats.mu.Unlock()

	b.stats.queueDrops++
}

func (b *serialCustomBridge) notePendingDrop(dropBytes int) {
	if dropBytes <= 0 {
		return
	}

	b.stats.mu.Lock()
	defer b.stats.mu.Unlock()

	b.stats.pendingDrops += uint64(dropBytes)
}

func (b *serialCustomBridge) logSnapshot(queueLen int, pendingLen int, subscriberCount int) {
	b.stats.mu.Lock()
	defer b.stats.mu.Unlock()

	log.Printf(
		"[sim-bridge] raw_bytes=%d published_blocks=%d queue=%d pending=%d queue_drops=%d pending_drops=%d subscribers=%d last_read=%s last_publish=%s",
		b.stats.rawBytes,
		b.stats.publishedBlock,
		queueLen,
		pendingLen,
		b.stats.queueDrops,
		b.stats.pendingDrops,
		subscriberCount,
		formatLogTime(b.stats.lastReadAt),
		formatLogTime(b.stats.lastPublishAt),
	)
}

func formatLogTime(timestamp time.Time) string {
	if timestamp.IsZero() {
		return "never"
	}
	return timestamp.Format(time.RFC3339Nano)
}

func openRawSerialPort(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}

	if err := configureRawTermios(fd); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}

	return os.NewFile(uintptr(fd), path), nil
}

func configureRawTermios(fd int) error {
	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}

	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	termios.Oflag = 0
	termios.Lflag = 0
	termios.Cflag &^= unix.PARENB | unix.PARODD | unix.CSTOPB | unix.CSIZE | unix.CRTSCTS
	termios.Cflag |= unix.CS8 | unix.CLOCAL | unix.CREAD
	termios.Cc[unix.VMIN] = 0
	termios.Cc[unix.VTIME] = 1

	return unix.IoctlSetTermios(fd, unix.TCSETS, termios)
}

func isRecoverableSerialReadError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, io.EOF) ||
		errors.Is(err, unix.EAGAIN) ||
		errors.Is(err, unix.EINTR) ||
		errors.Is(err, unix.EIO)
}
