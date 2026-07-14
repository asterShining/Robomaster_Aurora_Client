package video

import (
	"log"
	"sync/atomic"
)

const (
	// What: 重组缓冲区最大字节数。
	// Why: 自定义图传是连续 H.264 Annex-B 字节流，若长时间没有识别到新的 AU 边界，继续无限累积只会把延迟和内存一起拖垮。
	h264ReassemblerMaxBufSize = 2 * 1024 * 1024

	// What: 单帧 Annex-B 起始码扫描的最大步进距离。
	// Why: 一旦上游码流错位或参数集长期缺失，客户端不能一直在线性扫描超大缓冲区，必须尽快放弃旧数据并等待下一次可恢复边界。
	h264ReassemblerMaxFrameSearchLen = 512 * 1024
)

const (
	// What: H.264 NAL type 取值常量。
	// Why: 当前自定义图传恢复逻辑的核心就是显式识别 SPS/PPS/IDR/非 IDR slice，并以此判断 AU 边界和同步时机。
	h264NALTypeNonIDR = 1
	h264NALTypeIDR    = 5
	h264NALTypeSEI    = 6
	h264NALTypeSPS    = 7
	h264NALTypePPS    = 8
	h264NALTypeAUD    = 9
)

// H264Reassembler 将任意分块到达的 H.264 Annex-B 字节流重组为可直接送进解码器的访问单元（AU）。
//
// 发送端 `video_encoder_node` 已经输出 `alignment=au` 的 byte-stream，但机器人链路会再把原始字节流切成 0x0310 小块，
// 客户端收到时不再保留 AU 边界，因此这里必须自行恢复：
// 1. 先从任意 chunk 里重新切出一个个 NAL；
// 2. 再按 H.264 访问单元规则把多种 NAL 拼回一个 AU；
// 3. 若客户端是在码流中途切入，则先缓存 SPS/PPS，等到下一次 IDR 后再解锁输出。
//
// 这样做的直接目的，是避免当前现场日志里连续出现的：
// - "non-existing PPS 0 referenced"
// - "no frame!"
// 这些报错说明解码器先吃到了 slice，却还没拿到完整参数集；客户端必须显式等到一个可恢复的关键帧边界。
type H264Reassembler struct {
	buf []byte

	// What: 当前正在拼装、但尚未确认结束的访问单元。
	// Why: 一个 AU 往往包含 SPS/PPS/SEI/一个或多个 slice；只有攒够完整边界，才能作为“可解码单元”交给 ffplay。
	pendingAU []byte

	// What: 当前访问单元内部是否已经出现 VCL / IDR / SPS / PPS。
	// Why: 这些标记决定了何时要结束上一帧、何时可以解锁同步，以及是否需要把缓存参数集补到 IDR 前面。
	pendingHasVCL bool
	pendingHasIDR bool
	pendingHasSPS bool
	pendingHasPPS bool

	// What: 最近一次看到的 SPS/PPS 副本。
	// Why: 客户端可能在码流中途切到 custom，首批收到的可能全是 P 帧；只有缓存住后续重新出现的参数集，才能在下一个 IDR 到来时补成完整可解码 AU。
	cachedSPS []byte
	cachedPPS []byte

	// What: 是否已经成功对齐到“带参数集的关键帧”并进入稳定输出状态。
	// Why: 在拿到第一组可恢复的 SPS/PPS + IDR 之前，继续把普通 slice 灌给 ffplay 只会持续报错，并且无法真正出画。
	synced bool

	// What: 已经完成重组、等待调用方取走的 AU 队列。
	// Why: 单个 chunk 里可能一次性跨过多个 AU 边界；如果仍然只允许即时返回一个结果，后面的已完成 AU 就会被静默丢掉。
	outputQueue [][]byte

	frameOut uint64 // 原子计数，已输出 AU 数
	dropOut  uint64 // 原子计数，因错位/溢出/预同步丢弃的 AU 数

	nalTotal       uint64
	nalNonIDR      uint64
	nalIDR         uint64
	nalSPS         uint64
	nalPPS         uint64
	nalAUD         uint64
	nalSEI         uint64
	nalOther       uint64
	noStartCodeOut uint64
	lastNALType    uint32
	hasCachedSPS   uint32
	hasCachedPPS   uint32
	isSynced       uint32
}

// H264Diagnostics 是自定义 0x0310 链路的轻量只读诊断快照。
//
// 客户端现场排障最需要回答两个问题：0x0310 里到底是不是 Annex-B H.264，以及是否已经见到
// SPS/PPS/IDR 这组可恢复关键帧。这里全部用原子计数保存，状态广播线程可以随时读取，不会和
// MQTT 收包路径互相阻塞。
type H264Diagnostics struct {
	NALTotal     uint64
	NonIDR       uint64
	IDR          uint64
	SPS          uint64
	PPS          uint64
	AUD          uint64
	SEI          uint64
	Other        uint64
	NoStartCode  uint64
	LastNALType  byte
	HasCachedSPS bool
	HasCachedPPS bool
	Synced       bool
	FramesOut    uint64
	DropsOut     uint64
}

// NewH264Reassembler 创建一个新的 H.264 AU 重组器。
func NewH264Reassembler() *H264Reassembler {
	return &H264Reassembler{
		buf:         make([]byte, 0, 256*1024),
		pendingAU:   make([]byte, 0, 128*1024),
		outputQueue: make([][]byte, 0, 4),
	}
}

// Feed 向重组器提交一个原始 H.264 数据块，或在 `chunk=nil/len=0` 时只尝试排空内部输出队列。
// What: 允许调用方在一次 chunk 到来后，循环调用 `Feed(nil)` 继续取走同轮已经完成的多个 AU。
// Why: 当前 0x0310 只是把连续字节流切块传输，一个 chunk 可能恰好跨过多个 AU 边界；若没有“排空模式”，后续 AU 会被卡在内部直到下一个网络包到来。
func (r *H264Reassembler) Feed(chunk []byte) []byte {
	if len(chunk) == 0 {
		return r.popOutput()
	}

	// What: 在真正追加新数据前先尝试把历史已完成输出弹给调用方。
	// Why: 若上一次处理已经生成多个 AU，而上层还没取完，本轮不应继续无节制积压输出队列。
	if frame := r.popOutput(); frame != nil {
		// What: 先把当前 chunk 继续并入内部缓冲，避免因为“先吐旧帧”而丢掉新数据。
		// Why: 上层可能在 `for frame := Feed(chunk); frame != nil; frame = Feed(nil)` 这种模式下取数据，这里必须保证首个返回不会吞掉本次输入。
		r.appendChunk(chunk)
		r.processBuffer()
		return frame
	}

	r.appendChunk(chunk)
	r.processBuffer()
	return r.popOutput()
}

// Reset 清空内部缓冲区与同步状态。
// What: 同时重置当前 AU、缓存参数集、输出队列和同步标记。
// Why: custom 源切换或播放器重建后，旧码流语义已经失效；若继续沿用历史 SPS/PPS 或残留 AU，只会把新一轮首帧继续污染成坏流。
func (r *H264Reassembler) Reset() {
	r.buf = r.buf[:0]
	r.pendingAU = r.pendingAU[:0]
	r.pendingHasVCL = false
	r.pendingHasIDR = false
	r.pendingHasSPS = false
	r.pendingHasPPS = false
	r.cachedSPS = nil
	r.cachedPPS = nil
	r.synced = false
	r.outputQueue = r.outputQueue[:0]
	atomic.StoreUint32(&r.hasCachedSPS, 0)
	atomic.StoreUint32(&r.hasCachedPPS, 0)
	atomic.StoreUint32(&r.isSynced, 0)
}

// ResyncPreservingParameterSets 让输出重新等待下一个可恢复 IDR，但保留最近一次 SPS/PPS。
// What: 清空输入缓冲、未完成 AU 和输出队列，并把 synced 打回 false。
// Why: ffplay 因切源/窗口尺寸变化被重启时，新进程确实不能继续吃 P 帧，但上游 H.264 参数集没有失效；
//      保留 SPS/PPS 后，下一次 IDR 可以立刻补齐参数集恢复出画，避免全量 Reset 后等待多轮头部造成黑屏和坏块堆积。
func (r *H264Reassembler) ResyncPreservingParameterSets() {
	r.buf = r.buf[:0]
	r.pendingAU = r.pendingAU[:0]
	r.pendingHasVCL = false
	r.pendingHasIDR = false
	r.pendingHasSPS = false
	r.pendingHasPPS = false
	r.synced = false
	r.outputQueue = r.outputQueue[:0]
	atomic.StoreUint32(&r.hasCachedSPS, boolToUint32(len(r.cachedSPS) > 0))
	atomic.StoreUint32(&r.hasCachedPPS, boolToUint32(len(r.cachedPPS) > 0))
	atomic.StoreUint32(&r.isSynced, 0)
}

// FramesOut 返回已成功输出的完整访问单元数。
func (r *H264Reassembler) FramesOut() uint64 {
	return atomic.LoadUint64(&r.frameOut)
}

// DropsOut 返回因错位、溢出或预同步失败而被丢弃的访问单元数。
func (r *H264Reassembler) DropsOut() uint64 {
	return atomic.LoadUint64(&r.dropOut)
}

// Diagnostics 返回当前 H.264 重组器的累计 NAL 统计。
func (r *H264Reassembler) Diagnostics() H264Diagnostics {
	return H264Diagnostics{
		NALTotal:     atomic.LoadUint64(&r.nalTotal),
		NonIDR:       atomic.LoadUint64(&r.nalNonIDR),
		IDR:          atomic.LoadUint64(&r.nalIDR),
		SPS:          atomic.LoadUint64(&r.nalSPS),
		PPS:          atomic.LoadUint64(&r.nalPPS),
		AUD:          atomic.LoadUint64(&r.nalAUD),
		SEI:          atomic.LoadUint64(&r.nalSEI),
		Other:        atomic.LoadUint64(&r.nalOther),
		NoStartCode:  atomic.LoadUint64(&r.noStartCodeOut),
		LastNALType:  byte(atomic.LoadUint32(&r.lastNALType)),
		HasCachedSPS: atomic.LoadUint32(&r.hasCachedSPS) != 0,
		HasCachedPPS: atomic.LoadUint32(&r.hasCachedPPS) != 0,
		Synced:       atomic.LoadUint32(&r.isSynced) != 0,
		FramesOut:    r.FramesOut(),
		DropsOut:     r.DropsOut(),
	}
}

// appendChunk 将新的原始字节块并入内部缓冲。
// What: 统一处理容量保护与错位恢复。
// Why: H.264 是连续字节流而不是离散包，客户端必须允许从任意偏移切入，但又不能在坏流状态下一直无限累积旧数据。
func (r *H264Reassembler) appendChunk(chunk []byte) {
	if len(r.buf)+len(chunk) > h264ReassemblerMaxBufSize {
		atomic.AddUint64(&r.dropOut, 1)
		log.Printf("[h264] Reassembler buffer overflow, reset (buf=%d chunk=%d)", len(r.buf), len(chunk))
		r.Reset()
	}

	r.buf = append(r.buf, chunk...)
}

// processBuffer 尝试从缓冲区中持续切出完整 NAL，并按 AU 规则推进状态机。
// What: 每次只在确认“至少有两个起始码”时切出一个 NAL，最后一个尚未闭合的 NAL 留待下轮继续补齐。
// Why: 只有这样才能在 0x0310 任意分块场景里既避免误切半个 NAL，又能持续向前推进 AU 重组。
func (r *H264Reassembler) processBuffer() {
	for {
		nal := r.extractNextNAL()
		if nal == nil {
			return
		}

		r.processNAL(nal)
	}
}

// extractNextNAL 从当前缓冲区首部提取一个已经闭合的 Annex-B NAL。
// What: 只有找到“当前起始码 + 下一起始码”这对边界时才返回一个 NAL。
// Why: 最后一个 NAL 往往在本轮 chunk 末尾被截断，贸然输出只会把未写完整的 slice 提前送进解码器。
func (r *H264Reassembler) extractNextNAL() []byte {
	buf := r.buf
	if len(buf) < 5 {
		return nil
	}

	first := findAnnexBStartCode(buf, 0)
	if first < 0 {
		// What: 缓冲区里完全找不到合法起始码时，直接清空并等待下轮重新同步。
		// Why: 这说明当前字节流已经错位到无法恢复，继续保留这些脏字节不会对后续同步有任何帮助。
		atomic.AddUint64(&r.dropOut, 1)
		atomic.AddUint64(&r.noStartCodeOut, 1)
		r.buf = r.buf[:0]
		return nil
	}

	if first > 0 {
		// What: 丢弃首个起始码前的错位垃圾字节。
		// Why: 从任意中途切入连续码流时，前缀脏数据完全不属于任何合法 NAL，必须在这里一次性剪掉。
		r.buf = append(r.buf[:0], buf[first:]...)
		buf = r.buf
		first = 0
	}

	second := findAnnexBStartCode(buf, startCodeLen(buf, first))
	if second < 0 {
		if len(buf) > h264ReassemblerMaxFrameSearchLen {
			log.Printf("[h264] Reassembler: NAL too large without next start code (%d bytes), dropping", len(buf))
			atomic.AddUint64(&r.dropOut, 1)
			r.buf = r.buf[:0]
		}
		return nil
	}

	nal := append([]byte(nil), buf[first:second]...)
	r.buf = append(r.buf[:0], buf[second:]...)
	return nal
}

// processNAL 将单个 NAL 推进到 AU 状态机。
// What: 显式识别参数集、AUD、SEI 和 slice，并在“新 AU 开始”时把上一 AU 完整收口。
// Why: 当前自定义源的根因是客户端在码流中途切入时先拿到 P 帧，导致解码器一直缺 PPS；只有按 AU 级别建模，才能安全等待到下一次可恢复关键帧。
func (r *H264Reassembler) processNAL(nal []byte) {
	nalType, ok := h264NALType(nal)
	if !ok {
		return
	}

	r.noteNAL(nalType)

	if nalType == h264NALTypeSPS {
		r.cachedSPS = append([]byte(nil), nal...)
		atomic.StoreUint32(&r.hasCachedSPS, 1)
	}
	if nalType == h264NALTypePPS {
		r.cachedPPS = append([]byte(nil), nal...)
		atomic.StoreUint32(&r.hasCachedPPS, 1)
	}

	isVCL := nalType == h264NALTypeNonIDR || nalType == h264NALTypeIDR
	if isVCL && r.pendingHasVCL {
		firstMBInSlice, parsed := parseH264FirstMBInSlice(nal)
		if !parsed || firstMBInSlice == 0 {
			// What: 当前 AU 已经有过 slice，而新的 VCL 又是下一张图的首个 slice 时，先收口上一 AU。
			// Why: 这正是连续 Annex-B 流里最稳定的 AU 边界信号；一旦跨过这个点，上一帧已经完整，可以安全输出或丢弃。
			r.finishPendingAU()
		}
	}

	if !isVCL && r.pendingHasVCL && (nalType == h264NALTypeAUD || nalType == h264NALTypeSEI || nalType == h264NALTypeSPS || nalType == h264NALTypePPS) {
		// What: 已经收过 slice 后，又遇到属于下一 AU 前导区域的 AUD/SEI/SPS/PPS 时，也先收口上一 AU。
		// Why: 这些非 VCL NAL 通常意味着编码器已经在为下一张图准备头部；若不在这里切断，前后两帧就会被错误粘在一起。
		r.finishPendingAU()
	}

	if isVCL && len(r.pendingAU) == 0 && nalType == h264NALTypeIDR {
		// What: 新 AU 从 IDR 开始时，优先把最近缓存的 SPS/PPS 自动补到它前面。
		// Why: 用户切到 custom 往往发生在码流中途；即便参数集是稍早看到的，只要在当前 IDR 前补齐，ffplay 就能从下一关键帧重新恢复解码。
		if len(r.cachedSPS) > 0 {
			r.appendNALToPending(r.cachedSPS, h264NALTypeSPS)
		}
		if len(r.cachedPPS) > 0 {
			r.appendNALToPending(r.cachedPPS, h264NALTypePPS)
		}
	}

	r.appendNALToPending(nal, nalType)
}

// noteNAL 记录输入流里出现过的 NAL 类型。
func (r *H264Reassembler) noteNAL(nalType byte) {
	atomic.AddUint64(&r.nalTotal, 1)
	atomic.StoreUint32(&r.lastNALType, uint32(nalType))

	switch nalType {
	case h264NALTypeNonIDR:
		atomic.AddUint64(&r.nalNonIDR, 1)
	case h264NALTypeIDR:
		atomic.AddUint64(&r.nalIDR, 1)
	case h264NALTypeSPS:
		atomic.AddUint64(&r.nalSPS, 1)
	case h264NALTypePPS:
		atomic.AddUint64(&r.nalPPS, 1)
	case h264NALTypeAUD:
		atomic.AddUint64(&r.nalAUD, 1)
	case h264NALTypeSEI:
		atomic.AddUint64(&r.nalSEI, 1)
	default:
		atomic.AddUint64(&r.nalOther, 1)
	}
}

// appendNALToPending 将一个 NAL 追加到当前待完成 AU，并同步更新标记。
// What: 统一处理 pendingAU 的字节拼接和元数据标记。
// Why: 这样所有“参数集补齐”和“真实接收到的 NAL”都走同一条收口路径，避免状态与字节流不一致。
func (r *H264Reassembler) appendNALToPending(nal []byte, nalType byte) {
	r.pendingAU = append(r.pendingAU, nal...)

	switch nalType {
	case h264NALTypeSPS:
		r.pendingHasSPS = true
	case h264NALTypePPS:
		r.pendingHasPPS = true
	case h264NALTypeIDR:
		r.pendingHasVCL = true
		r.pendingHasIDR = true
	case h264NALTypeNonIDR:
		r.pendingHasVCL = true
	}
}

// finishPendingAU 尝试将当前待完成 AU 收口为可输出单元。
// What: 只有满足“已同步”或“当前就是带参数集的可恢复 IDR”两类条件时才真正出队。
// Why: 这一步就是为了解决当前现场的 `non-existing PPS`；在没拿到可恢复关键帧前，继续输出普通 slice 只会让解码器持续报错。
func (r *H264Reassembler) finishPendingAU() {
	if len(r.pendingAU) == 0 {
		return
	}

	emit := false
	if r.pendingHasIDR && (r.pendingHasSPS || len(r.cachedSPS) > 0) && (r.pendingHasPPS || len(r.cachedPPS) > 0) {
		// What: 带参数集的 IDR 是进入稳定同步态的唯一入口。
		// Why: 只有从这种 AU 开始，后续 P/B 帧才有可靠的参考参数，ffplay 才能真正恢复正常出画。
		emit = true
		r.synced = true
		atomic.StoreUint32(&r.isSynced, 1)
	} else if r.synced && r.pendingHasVCL {
		// What: 已同步后允许继续输出后续普通 AU。
		// Why: 一旦首个可恢复关键帧已经过了，后续普通帧就可以直接沿用当前参数集继续解码。
		emit = true
	}

	if emit {
		completed := append([]byte(nil), r.pendingAU...)
		r.outputQueue = append(r.outputQueue, completed)
		atomic.AddUint64(&r.frameOut, 1)
	} else if r.pendingHasVCL {
		// What: 只对“本来像一帧，但仍不满足同步条件”的 AU 记一次丢弃。
		// Why: 这样既能统计预同步阶段丢掉了多少无效 slice，又不会把单独的 SPS/PPS/AUD 噪声也记成一整帧丢失。
		dropCount := atomic.AddUint64(&r.dropOut, 1)
		if dropCount <= 5 || dropCount%60 == 0 {
			log.Printf(
				"[h264] Drop pre-sync AU has_vcl=%t has_idr=%t has_sps=%t has_pps=%t cached_sps=%t cached_pps=%t drop_count=%d",
				r.pendingHasVCL,
				r.pendingHasIDR,
				r.pendingHasSPS,
				r.pendingHasPPS,
				len(r.cachedSPS) > 0,
				len(r.cachedPPS) > 0,
				dropCount,
			)
		}
	}

	r.pendingAU = r.pendingAU[:0]
	r.pendingHasVCL = false
	r.pendingHasIDR = false
	r.pendingHasSPS = false
	r.pendingHasPPS = false
}

// popOutput 从内部输出队列弹出一个已完成 AU。
// What: 始终按 FIFO 顺序交给调用方。
// Why: AU 的时序必须严格保序，一旦把后完成的访问单元提前交出去，解码器参考链会立刻错乱。
func (r *H264Reassembler) popOutput() []byte {
	if len(r.outputQueue) == 0 {
		return nil
	}

	frame := r.outputQueue[0]
	r.outputQueue = append(r.outputQueue[:0], r.outputQueue[1:]...)
	return frame
}

// h264NALType 返回一个 Annex-B NAL 的类型。
// What: 从起始码后的第一个字节提取 H.264 nal_unit_type。
// Why: AU 重组、参数集缓存和同步解锁都完全依赖这个 5bit 类型字段。
func h264NALType(nal []byte) (byte, bool) {
	startCodeIndex := findAnnexBStartCode(nal, 0)
	if startCodeIndex < 0 {
		return 0, false
	}

	headerIndex := startCodeIndex + startCodeLen(nal, startCodeIndex)
	if headerIndex >= len(nal) {
		return 0, false
	}

	return nal[headerIndex] & 0x1F, true
}

func boolToUint32(value bool) uint32 {
	if value {
		return 1
	}
	return 0
}

// parseH264FirstMBInSlice 解析 slice header 里的 `first_mb_in_slice`。
// What: 仅解析 H.264 slice header 的第一个 Exp-Golomb 字段。
// Why: 识别“这是不是下一张图的第一个 slice”已经足够判断 AU 边界，不需要在客户端完整实现整个 H.264 语法树。
func parseH264FirstMBInSlice(nal []byte) (uint32, bool) {
	startCodeIndex := findAnnexBStartCode(nal, 0)
	if startCodeIndex < 0 {
		return 0, false
	}

	headerIndex := startCodeIndex + startCodeLen(nal, startCodeIndex)
	if headerIndex >= len(nal) {
		return 0, false
	}

	rbsp := h264NALToRBSP(nal[headerIndex+1:])
	reader := newH264BitReader(rbsp)
	value, ok := reader.readUE()
	if !ok {
		return 0, false
	}

	return value, true
}

// h264NALToRBSP 去掉 NAL 里的 emulation-prevention 字节。
// What: 将 `00 00 03` 还原成标准 RBSP 字节流。
// Why: Exp-Golomb 解析必须基于 RBSP，否则遇到防竞争插入字节后会把 slice header 位流读歪。
func h264NALToRBSP(payload []byte) []byte {
	if len(payload) == 0 {
		return nil
	}

	rbsp := make([]byte, 0, len(payload))
	zeroRun := 0
	for _, value := range payload {
		if zeroRun >= 2 && value == 0x03 {
			zeroRun = 0
			continue
		}

		rbsp = append(rbsp, value)
		if value == 0x00 {
			zeroRun++
		} else {
			zeroRun = 0
		}
	}

	return rbsp
}

// h264BitReader 提供最小化的按位读取能力。
// What: 只实现当前 slice header 解析真正需要的位访问和 UE 读取。
// Why: 重组器的目标是稳、轻、可维护，不应为了一个 `first_mb_in_slice` 去引入完整编解码依赖。
type h264BitReader struct {
	data    []byte
	bitRead int
}

// newH264BitReader 创建一个新的 RBSP 位流读取器。
func newH264BitReader(data []byte) *h264BitReader {
	return &h264BitReader{data: data}
}

// readBit 读取单个比特。
func (r *h264BitReader) readBit() (uint8, bool) {
	if r.bitRead >= len(r.data)*8 {
		return 0, false
	}

	byteIndex := r.bitRead / 8
	bitIndex := 7 - (r.bitRead % 8)
	r.bitRead++
	return (r.data[byteIndex] >> bitIndex) & 0x01, true
}

// readBits 读取固定宽度的无符号位字段。
func (r *h264BitReader) readBits(count int) (uint32, bool) {
	if count <= 0 {
		return 0, true
	}

	var value uint32
	for bitIndex := 0; bitIndex < count; bitIndex++ {
		bitValue, ok := r.readBit()
		if !ok {
			return 0, false
		}
		value = (value << 1) | uint32(bitValue)
	}

	return value, true
}

// readUE 读取无符号 Exp-Golomb 值。
func (r *h264BitReader) readUE() (uint32, bool) {
	leadingZeroBits := 0
	for {
		bitValue, ok := r.readBit()
		if !ok {
			return 0, false
		}
		if bitValue == 1 {
			break
		}
		leadingZeroBits++
		if leadingZeroBits > 31 {
			return 0, false
		}
	}

	if leadingZeroBits == 0 {
		return 0, true
	}

	suffixValue, ok := r.readBits(leadingZeroBits)
	if !ok {
		return 0, false
	}

	return (1 << leadingZeroBits) - 1 + suffixValue, true
}

// findAnnexBStartCode 从 buf[offset:] 中扫描第一个 Annex-B 起始码（0x000001 或 0x00000001）。
// 返回起始码在 buf 中的绝对偏移量，未找到则返回 -1。
func findAnnexBStartCode(buf []byte, offset int) int {
	for index := offset; index+2 < len(buf); index++ {
		if buf[index] != 0x00 || buf[index+1] != 0x00 {
			continue
		}
		if buf[index+2] == 0x01 {
			return index
		}
		if index+3 < len(buf) && buf[index+2] == 0x00 && buf[index+3] == 0x01 {
			return index
		}
	}
	return -1
}

// startCodeLen 返回 buf[pos] 处起始码的字节长度（3 或 4）。
func startCodeLen(buf []byte, pos int) int {
	if pos+3 < len(buf) && buf[pos+2] == 0x00 && buf[pos+3] == 0x01 {
		return 4
	}
	return 3
}
