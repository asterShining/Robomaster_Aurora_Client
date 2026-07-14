package network

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	packetHeaderSize = 8
	maxUDPSize       = 65535
	maxFrameSize     = 8 * 1024 * 1024
	frameTimeout     = 50 * time.Millisecond // 如果某帧 50ms 还没拼完，彻底判定为死帧

	// What: 首个官方视频包的等待告警周期。
	// Why: “UDP receiver bound on :3334” 只代表端口监听成功，不代表外部真的开始发流；这里必须在持续未见首包时主动报警，避免用户被“已绑定端口”误导。
	firstPacketWaitWarnInterval = 5 * time.Second
)

// headerOrder 描述图传 UDP 头部字段当前采用的字节序。
// 之所以需要显式状态，是因为官方协议只定义了前 8 个字节的字段含义，没有写死大小端。
// 如果这里假定错误，就会把 frameSize 解析成荒谬的大数，表现为“只能收包，永远组不出完整帧”。
type headerOrder uint8

const (
	headerOrderUnknown headerOrder = iota
	headerOrderLittle
	headerOrderBig
)

// String 将内部字节序枚举映射为日志可读字符串。
// 这样做的目的不是美观，而是为了赛场排障时能一眼看出当前接收器锁定了哪种头格式。
func (o headerOrder) String() string {
	switch o {
	case headerOrderLittle:
		return "little"
	case headerOrderBig:
		return "big"
	default:
		return "unknown"
	}
}

// opposite 返回另一套候选字节序。
// 这里专门保留切换逻辑，是为了在接收器已经锁定某种字节序后，仍然能在发现当前包不合法时快速回退。
func (o headerOrder) opposite() headerOrder {
	switch o {
	case headerOrderLittle:
		return headerOrderBig
	case headerOrderBig:
		return headerOrderLittle
	default:
		return headerOrderUnknown
	}
}

// uint16 按当前候选字节序解析 16 位字段。
// 将二进制解析能力收口到这个方法里，可以避免上层业务逻辑反复散落大小端分支。
func (o headerOrder) uint16(data []byte) uint16 {
	if o == headerOrderLittle {
		return binary.LittleEndian.Uint16(data)
	}
	return binary.BigEndian.Uint16(data)
}

// uint32 按当前候选字节序解析 32 位字段。
// frameSize 是决定组帧能否完成的关键字段，因此必须与 uint16 使用完全一致的端序来源。
func (o headerOrder) uint32(data []byte) uint32 {
	if o == headerOrderLittle {
		return binary.LittleEndian.Uint32(data)
	}
	return binary.BigEndian.Uint32(data)
}

// packetHeader 表示单个 UDP 视频包前 8 字节头部的解析结果。
// 单独建模的目的，是让“头解析”和“帧拼装”两个阶段各自对自己的输入负责，减少交叉假设。
type packetHeader struct {
	frameID   uint16
	sliceID   uint16
	frameSize uint32
}

// frameBuilder 用于临时存放乱序或碎片的 H.265 分片
type frameBuilder struct {
	frameSize    uint32
	receivedSize uint32
	slices       map[uint16][]byte // sliceID -> data
	startTime    time.Time
}

// UDPReceiverStats 导出 UDP 视频接收器的只读统计快照。
// What: 统一收口组帧成功率、坏帧和活跃时间戳。
// Why: App 层需要周期性把这些指标桥接给前端，但不应该直接读取接收器内部状态。
type UDPReceiverStats struct {
	PacketCount          uint64
	FrameCount           uint64
	InvalidHeaderCount   uint64
	IncompleteFrameDrops uint64
	TimeoutFrameDrops    uint64
	InvalidFrameDrops    uint64
	DuplicateSliceCount  uint64
	LastPacketAt         int64
	LastFrameAt          int64
	HeaderOrder          string
}

// UDPReceiver 负责接收 3334 端口图传，并将完整帧送出
type UDPReceiver struct {
	conn       *net.UDPConn
	listenPort int
	onFrameOut func(h265Frame []byte) // 拼好一帧后通过回调传出

	mu           sync.Mutex
	headerOrder  headerOrder
	currentFrame *frameBuilder
	currentID    uint16
	stopCh       chan struct{}
	stopOnce     sync.Once

	pktCount                 uint64 // 收到的 UDP 包总数（仅用于日志）
	frameCount               uint64 // 成功组装并送出的帧总数
	invalidHeaderCount       uint64 // 头部无法通过合法性校验的包数（仅用于节流日志）
	incompleteFrameDropCount uint64 // 因切帧导致的未完成旧帧丢弃次数
	timeoutFrameDropCount    uint64 // 因组帧超时而丢弃的帧数
	invalidFrameDropCount    uint64 // 组帧完成后校验失败的坏帧数
	duplicateSliceCount      uint64 // 重复分片次数
	lastPacketAt             int64  // 最近一次收到 UDP 包的时间戳（UnixMilli）
	lastFrameAt              int64  // 最近一次组出完整帧的时间戳（UnixMilli）
}

// NewUDPReceiver 新建接收器实例
func NewUDPReceiver(port int, onFrame func([]byte)) (*UDPReceiver, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	log.Printf("[network] UDP receiver bound on :%d", port)

	return &UDPReceiver{
		conn:       conn,
		listenPort: port,
		onFrameOut: onFrame,
		stopCh:     make(chan struct{}),
	}, nil
}

// Start 开始阻塞接收循环
func (r *UDPReceiver) Start() {
	// What: 收包循环和“首包迟到”告警循环同时启动。
	// Why: 这样既不阻塞正常收包，又能在外部图传没真正发到本机时尽早给出硬证据。
	go r.receiveLoop()
	go r.warnIfNoFirstPacketLoop()
}

func (r *UDPReceiver) Stop() {
	r.stopOnce.Do(func() {
		close(r.stopCh)
		r.conn.Close()
	})
}

// StatsSnapshot 导出接收器状态快照。
// What: 将内部原子计数和当前端序锁定状态打包。
// Why: 上层只需要读统计，不应该为了做状态面板而直接持有接收器内部互斥锁。
func (r *UDPReceiver) StatsSnapshot() UDPReceiverStats {
	r.mu.Lock()
	headerOrder := r.headerOrder.String()
	r.mu.Unlock()

	return UDPReceiverStats{
		PacketCount:          atomic.LoadUint64(&r.pktCount),
		FrameCount:           atomic.LoadUint64(&r.frameCount),
		InvalidHeaderCount:   atomic.LoadUint64(&r.invalidHeaderCount),
		IncompleteFrameDrops: atomic.LoadUint64(&r.incompleteFrameDropCount),
		TimeoutFrameDrops:    atomic.LoadUint64(&r.timeoutFrameDropCount),
		InvalidFrameDrops:    atomic.LoadUint64(&r.invalidFrameDropCount),
		DuplicateSliceCount:  atomic.LoadUint64(&r.duplicateSliceCount),
		LastPacketAt:         atomic.LoadInt64(&r.lastPacketAt),
		LastFrameAt:          atomic.LoadInt64(&r.lastFrameAt),
		HeaderOrder:          headerOrder,
	}
}

// warnIfNoFirstPacketLoop 在接收器启动后周期性检查是否仍未见到任何视频首包。
// What: 每隔固定周期观察 pktCount 是否仍为 0。
// Why: 赛场排障最常见误判就是“端口已经绑定所以肯定在收流”；这条日志专门把“监听成功”和“首包到达”拆开。
func (r *UDPReceiver) warnIfNoFirstPacketLoop() {
	// What: 使用 ticker 做低频巡检。
	// Why: 首包缺失通常是外部发流链路问题，低频观测已经足够，不需要给高频收包路径增加额外负担。
	ticker := time.NewTicker(firstPacketWaitWarnInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			// What: 一旦已经见过首包，告警循环立即退出。
			// Why: 后续链路健康度由 LastPacketAt / LastFrameAt 接管，这里只负责“从未收到过首包”的冷启动诊断。
			if atomic.LoadUint64(&r.pktCount) > 0 {
				return
			}

			// What: 将当前主机可用 IPv4 候选一起打印。
			// Why: 现场最常见问题之一是发送端仍把视频发往旧 IP；把本机候选地址直接带出来，能显著减少来回确认成本。
			localIPv4 := strings.Join(listLocalIPv4Candidates(), ",")
			if localIPv4 == "" {
				localIPv4 = "unknown"
			}

			log.Printf(
				"[Warning] [network] UDP receiver on :%d still waiting for first video packet after %s local_ipv4=%s",
				r.listenPort,
				firstPacketWaitWarnInterval,
				localIPv4,
			)
		}
	}
}

// listLocalIPv4Candidates 枚举当前主机所有可用的非回环 IPv4 地址。
// What: 扫描 UP 状态网卡上的 IPv4 地址并去重返回。
// Why: 图传发送端常常是按固定目标 IP 推流，直接把客户端候选地址带到日志里，排查“发错地址”比只打印端口更高效。
func listLocalIPv4Candidates() []string {
	// What: 优先读取网卡清单。
	// Why: 这样可以同时覆盖有线、无线和 USB 网卡，而不是只看默认路由对应的一张卡。
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	// What: 用 map 做去重。
	// Why: 某些驱动或别名网卡可能重复暴露同一 IPv4，直接去重可以避免日志里出现无意义重复项。
	seen := make(map[string]struct{}, len(interfaces))
	candidates := make([]string, 0, len(interfaces))

	for _, iface := range interfaces {
		// What: 跳过未启用和回环接口。
		// Why: 图传发送端不可能把比赛视频发到 down 或 loopback 设备上，把它们带进日志只会干扰判断。
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// What: 读取当前网卡挂载的地址列表。
		// Why: 只有真正挂了 IPv4 的接口，才可能成为图传发送端的目标地址。
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP

			// What: 兼容 *net.IPNet 与 *net.IPAddr 两种地址承载类型。
			// Why: 标准库在不同调用路径下返回的具体实现并不完全一致，这里必须都接住，避免漏掉真实地址。
			switch value := addr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			default:
				continue
			}

			// What: 只保留 IPv4。
			// Why: 当前 RoboMaster 官方视频链路按 IPv4 地址固定发流，IPv6 地址打印出来没有排障价值。
			ipv4 := ip.To4()
			if ipv4 == nil || ipv4.IsLoopback() {
				continue
			}

			ipText := ipv4.String()
			if _, exists := seen[ipText]; exists {
				continue
			}

			seen[ipText] = struct{}{}
			candidates = append(candidates, ipText)
		}
	}

	// What: 对结果做稳定排序。
	// Why: 这样重复启动时日志顺序一致，用户更容易一眼对比有没有地址变化。
	sort.Strings(candidates)
	return candidates
}

func (r *UDPReceiver) receiveLoop() {
	buf := make([]byte, maxUDPSize)

	for {
		select {
		case <-r.stopCh:
			return
		default:
		}

		r.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, remoteAddr, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // 正常超时，跳过本次循环
			}
			select {
			case <-r.stopCh:
				return
			default:
				log.Printf("[network] UDP Read Error: %v", err)
				continue
			}
		}

		atomic.StoreInt64(&r.lastPacketAt, time.Now().UnixMilli())
		cnt := atomic.AddUint64(&r.pktCount, 1)
		if cnt == 1 {
			log.Printf("[network] First UDP packet received from %s, size=%d", remoteAddr, n)
		} else if cnt%200 == 0 {
			log.Printf("[network] UDP packets received: %d", cnt)
		}

		if n <= packetHeaderSize {
			continue
		}

		r.processPacket(buf[:n])
	}
}

// ==========================================
// 自由度 2 (Go 组帧逻辑) : 防丢包防血崩的组帧拼片缓冲池
// ==========================================
func (r *UDPReceiver) processPacket(data []byte) {
	var completeFrame []byte

	// 先在锁内完成所有帧状态更新。
	// 这里刻意不使用 defer 解锁，是因为组帧成功后要在锁外调用 onFrameOut，避免解码阻塞收包状态机。
	r.mu.Lock()

	// 1. 自动探测当前包头的有效字节序。
	// 只有头部先被正确解析，后面的 frameID / sliceID / frameSize 才有任何业务意义。
	header, ok := r.detectHeader(data)
	if !ok {
		invalidCount := atomic.AddUint64(&r.invalidHeaderCount, 1)
		if invalidCount <= 5 || invalidCount%200 == 0 {
			log.Printf("[network] Drop invalid UDP video packet size=%d header_order=%s invalid_count=%d",
				len(data), r.headerOrder.String(), invalidCount)
		}
		r.mu.Unlock()
		return
	}

	// 2. 头部合法后再切出真正的 HEVC 负载。
	// 这样可以保证 payload 的解释一定建立在已验证过的 header 上，而不是继续把坏数据往后传。
	payload := data[packetHeaderSize:]

	// 3. 帧生命周期判断（旧帧丢弃、新帧开启）。
	// 只要 frameID 变化，就立即切到新帧，避免旧帧残片持续占据缓冲导致后续所有帧都被拖死。
	if header.frameID != r.currentID || r.currentFrame == nil {
		if r.currentFrame != nil && r.currentFrame.receivedSize < r.currentFrame.frameSize {
			atomic.AddUint64(&r.incompleteFrameDropCount, 1)

			// 这里额外打印分片区间。
			// 原因是赛场上“丢片”与“起始分片编号假设错误”在表面上都表现为组帧失败，必须把上下界带出来才有区分度。
			minSliceID, maxSliceID := sliceRange(r.currentFrame.slices)
			log.Printf("[network] Drop incomplete frame id=%d recv=%d/%d slice_range=%d-%d",
				r.currentID, r.currentFrame.receivedSize, r.currentFrame.frameSize, minSliceID, maxSliceID)
		}

		// 新帧一旦建立，frameID 与 frameSize 都完全以本包头部为准。
		// 这样做可以避免旧帧状态污染新帧，特别是上一帧 frameSize 错误时不会把错误继续带进来。
		r.currentID = header.frameID
		r.currentFrame = &frameBuilder{
			frameSize:    header.frameSize,
			receivedSize: 0,
			slices:       make(map[uint16][]byte),
			startTime:    time.Now(),
		}
	}

	// 4. 超时保护机制。
	// 官方码流本身没有重传；如果一帧残片在本地缓存里挂太久，不及时清掉只会让后续正常帧继续被拖累。
	if time.Since(r.currentFrame.startTime) > frameTimeout {
		atomic.AddUint64(&r.timeoutFrameDropCount, 1)

		// 超时前先打印已有分片区间，方便确认到底是网络丢片，还是 slice 起始编号假设与官方流不一致。
		minSliceID, maxSliceID := sliceRange(r.currentFrame.slices)
		log.Printf("[network] Drop timed out frame id=%d recv=%d/%d slice_range=%d-%d",
			r.currentID, r.currentFrame.receivedSize, r.currentFrame.frameSize, minSliceID, maxSliceID)

		// 超时后立刻用当前包重新开启该帧的缓冲容器。
		// 这样可以确保“最新包”不会因为前面一堆坏残片而被白白丢掉。
		r.currentID = header.frameID
		r.currentFrame = &frameBuilder{
			frameSize:    header.frameSize,
			receivedSize: 0,
			slices:       make(map[uint16][]byte),
			startTime:    time.Now(),
		}
	}

	// 5. 装载分片（去重保护）。
	// 官方流允许分片乱序到达，因此这里只按 sliceID 去重，不依赖到达顺序。
	if _, exists := r.currentFrame.slices[header.sliceID]; !exists {
		// 这里必须深拷贝 payload。
		// 原因是 receiveLoop 复用了同一个 buf，如果直接引用切片，下一包到来时内容会被覆盖。
		payloadCopy := make([]byte, len(payload))
		copy(payloadCopy, payload)

		r.currentFrame.slices[header.sliceID] = payloadCopy
		r.currentFrame.receivedSize += uint32(len(payload))
	} else {
		atomic.AddUint64(&r.duplicateSliceCount, 1)
	}

	// 6. 达到目标总字节数后尝试组装完整帧。
	// 注意这里只是“尝试”，真正是否完整还要经过 slice 连续性与长度一致性的双重校验。
	if r.currentFrame.receivedSize >= r.currentFrame.frameSize {
		assembledFrame, minSliceID, maxSliceID, err := assemble(r.currentFrame)
		if err != nil {
			atomic.AddUint64(&r.invalidFrameDropCount, 1)
			log.Printf("[network] Drop invalid frame id=%d reason=%v recv=%d/%d slice_range=%d-%d",
				header.frameID, err, r.currentFrame.receivedSize, r.currentFrame.frameSize, minSliceID, maxSliceID)
		} else {
			frameCount := atomic.AddUint64(&r.frameCount, 1)
			atomic.StoreInt64(&r.lastFrameAt, time.Now().UnixMilli())
			if frameCount == 1 {
				log.Printf("[network] First complete H.265 frame assembled: id=%d size=%d bytes", header.frameID, len(assembledFrame))
			} else if frameCount%30 == 0 {
				log.Printf("[network] Frames assembled: %d (latest id=%d size=%d)", frameCount, header.frameID, len(assembledFrame))
			}
			completeFrame = assembledFrame
		}

		// 无论当前帧是组装成功还是校验失败，都必须清掉当前 builder。
		// 否则坏帧残留会让后续 frameID 相同或递增帧继续复用错误状态，问题会持续扩散。
		r.currentFrame = nil
	}

	r.mu.Unlock()

	// 7. 在锁外触发上游回调。
	// 这样即便上游解码很慢，也不会把接收器内部互斥锁长时间占住。
	if completeFrame != nil && r.onFrameOut != nil {
		r.onFrameOut(completeFrame)
	}
}

// detectHeader 负责在官方图传头部的候选端序之间做自动判定。
// 逻辑重点有两点：第一，已锁定端序时优先按当前端序解析；第二，当前端序不合法时允许切换到另一套候选。
func (r *UDPReceiver) detectHeader(data []byte) (packetHeader, bool) {
	if r.headerOrder != headerOrderUnknown {
		// 已锁定端序时先走快路径，避免每个包都做双候选解析。
		currentHeader, currentOK := parseHeaderCandidate(data, r.headerOrder)
		if currentOK {
			return currentHeader, true
		}

		// 当前锁定端序失效时，立即尝试另一套端序。
		// 这一步是为了覆盖比赛现场设备切换、历史假设错误或接入测试源时的容错场景。
		otherOrder := r.headerOrder.opposite()
		otherHeader, otherOK := parseHeaderCandidate(data, otherOrder)
		if otherOK {
			r.headerOrder = otherOrder
			log.Printf("[network] Video UDP header order switched to %s-endian frame=%d slice=%d frame_size=%d",
				otherOrder.String(), otherHeader.frameID, otherHeader.sliceID, otherHeader.frameSize)
			return otherHeader, true
		}

		return packetHeader{}, false
	}

	// 未锁定端序时并行尝试小端与大端。
	// 这样做的核心目的，是让接收器在第一次接官方视频源时就能自举出正确配置，而不是要求用户手动改代码。
	littleHeader, littleOK := parseHeaderCandidate(data, headerOrderLittle)
	bigHeader, bigOK := parseHeaderCandidate(data, headerOrderBig)

	switch {
	case littleOK && !bigOK:
		r.headerOrder = headerOrderLittle
		log.Printf("[network] Video UDP header order detected as little-endian frame=%d slice=%d frame_size=%d",
			littleHeader.frameID, littleHeader.sliceID, littleHeader.frameSize)
		return littleHeader, true
	case bigOK && !littleOK:
		r.headerOrder = headerOrderBig
		log.Printf("[network] Video UDP header order detected as big-endian frame=%d slice=%d frame_size=%d",
			bigHeader.frameID, bigHeader.sliceID, bigHeader.frameSize)
		return bigHeader, true
	case littleOK && bigOK:
		// 双候选都落在“合法区间”时，优先采用小端。
		// 这是一个明确的保守默认值：当前故障正是固定大端假设导致的，优先小端能最大概率恢复官方源。
		r.headerOrder = headerOrderLittle
		log.Printf("[network] Video UDP header order ambiguous, prefer little-endian frame=%d slice=%d frame_size=%d",
			littleHeader.frameID, littleHeader.sliceID, littleHeader.frameSize)
		return littleHeader, true
	default:
		return packetHeader{}, false
	}
}

// parseHeaderCandidate 按给定候选端序解析包头，并做最基本的结构合法性校验。
// 这里故意只做与协议强相关的硬约束，不把业务层猜测混进来，避免把本该接受的官方包误判掉。
func parseHeaderCandidate(data []byte, order headerOrder) (packetHeader, bool) {
	if len(data) <= packetHeaderSize {
		return packetHeader{}, false
	}

	// payloadSize 是当前这个 UDP 包真实承载的 HEVC 负载字节数。
	// frameSize 如果小于它，就说明当前端序把头部解析错了，因为一帧总长度不可能小于单包长度。
	payloadSize := len(data) - packetHeaderSize
	header := packetHeader{
		frameID:   order.uint16(data[0:2]),
		sliceID:   order.uint16(data[2:4]),
		frameSize: order.uint32(data[4:8]),
	}

	if header.frameSize == 0 {
		return packetHeader{}, false
	}
	if header.frameSize < uint32(payloadSize) {
		return packetHeader{}, false
	}
	if header.frameSize > maxFrameSize {
		return packetHeader{}, false
	}

	return header, true
}

// assemble 将当前 frameBuilder 中的所有分片按 sliceID 升序拼接为完整 H.265 帧。允许官方流从 0 或 1 起步。
func assemble(b *frameBuilder) ([]byte, uint16, uint16, error) {
	if len(b.slices) == 0 {
		return nil, 0, 0, fmt.Errorf("no slices received")
	}

	// 先把所有收到的 sliceID 排序。
	// 原因是官方 UDP 包允许乱序到达，只有显式排序后才能准确判断“是否连续”。
	sliceIDs := make([]int, 0, len(b.slices))
	for sid := range b.slices {
		sliceIDs = append(sliceIDs, int(sid))
	}
	sort.Ints(sliceIDs)

	minSliceID := uint16(sliceIDs[0])
	maxSliceID := uint16(sliceIDs[len(sliceIDs)-1])

	// 官方协议没有写死首分片一定从 0 开始。
	// 这里允许 0 或 1 两种起始方式，既兼容旧测试源，也兼容常见的从 1 开始编号的官方流实现。
	if minSliceID != 0 && minSliceID != 1 {
		return nil, minSliceID, maxSliceID, fmt.Errorf("unexpected first slice id=%d", minSliceID)
	}

	result := make([]byte, 0, int(b.frameSize))
	prevSliceID := sliceIDs[0]

	for idx, rawSliceID := range sliceIDs {
		// 相邻分片必须严格连续。
		// 只要中间有缺口，即便总字节数恰好凑够，也只能说明 frameSize 被误判或发生了错拼，绝不能继续往下游解码。
		if idx > 0 && rawSliceID != prevSliceID+1 {
			return nil, minSliceID, maxSliceID, fmt.Errorf("missing slice between %d and %d", prevSliceID, rawSliceID)
		}

		sid := uint16(rawSliceID)
		result = append(result, b.slices[sid]...)
		prevSliceID = rawSliceID
	}

	// 最终长度必须与头部声明的 frameSize 完全一致。
	// 这一步是最后一道保险，用于挡住“分片连续但头长错了”这种最隐蔽的坏帧。
	if uint32(len(result)) != b.frameSize {
		return nil, minSliceID, maxSliceID, fmt.Errorf("assembled size mismatch got=%d want=%d", len(result), b.frameSize)
	}

	return result, minSliceID, maxSliceID, nil
}

// sliceRange 返回当前 frameBuilder 中已收到分片的最小和最大编号。
// 这个信息专门用于诊断日志，帮助区分“纯丢片”和“首分片编号假设错误”。
func sliceRange(slices map[uint16][]byte) (uint16, uint16) {
	if len(slices) == 0 {
		return 0, 0
	}

	first := true
	var minSliceID uint16
	var maxSliceID uint16

	for sid := range slices {
		if first {
			minSliceID = sid
			maxSliceID = sid
			first = false
			continue
		}

		if sid < minSliceID {
			minSliceID = sid
		}
		if sid > maxSliceID {
			maxSliceID = sid
		}
	}

	return minSliceID, maxSliceID
}
