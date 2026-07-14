package video

import "sync"

const (
	hevcNALTypeIRAPMin = 16
	hevcNALTypeIRAPMax = 23
	hevcNALTypeVPS     = 32
	hevcNALTypeSPS     = 33
	hevcNALTypePPS     = 34
	hevcNALTypeAUD     = 35
)

// HEVCSyncDecision 描述一帧 HEVC AU 对预同步门控的影响。
// What: 收口当前帧是否可放行、是否可作为 bootstrap，以及它包含的关键参数集类型。
// Why: App 层需要在不耦合显示托管器的前提下，明确区分“普通中途 P 帧”和“可恢复关键帧”。
type HEVCSyncDecision struct {
	Pass        bool
	Recoverable bool
	HasVPS      bool
	HasSPS      bool
	HasPPS      bool
	HasIRAP     bool
	DropCount   uint64
}

// HEVCSyncGate 负责在 official HEVC 流进入播放器前做一次预同步门控。
// What: 只有拿到带 VPS/SPS/PPS 的 IRAP 帧后才解锁放行。
// Why: 新 official 播放器若从流中途直接吃到普通帧，ffplay 会因为参数集缺失而秒退。
type HEVCSyncGate struct {
	mu        sync.Mutex
	synced    bool
	dropCount uint64
}

func NewHEVCSyncGate() *HEVCSyncGate {
	return &HEVCSyncGate{}
}

// Observe 根据当前 HEVC AU 更新门控状态并返回本帧决策。
// What: unsynced 阶段只接受“参数集 + IRAP”一体的可恢复帧；synced 后放行全部帧。
// Why: 这样 official ffplay 无论启动还是重启，都不会再从中途 P 帧开始误起解码器。
func (g *HEVCSyncGate) Observe(frame []byte) HEVCSyncDecision {
	hasVPS, hasSPS, hasPPS, hasIRAP := inspectHEVCFrame(frame)

	g.mu.Lock()
	defer g.mu.Unlock()

	decision := HEVCSyncDecision{
		HasVPS:      hasVPS,
		HasSPS:      hasSPS,
		HasPPS:      hasPPS,
		HasIRAP:     hasIRAP,
		Recoverable: hasVPS && hasSPS && hasPPS && hasIRAP,
	}

	if decision.Recoverable {
		g.synced = true
		decision.Pass = true
		return decision
	}

	if g.synced {
		decision.Pass = true
		return decision
	}

	g.dropCount++
	decision.DropCount = g.dropCount
	return decision
}

func (g *HEVCSyncGate) Reset() {
	g.mu.Lock()
	g.synced = false
	g.dropCount = 0
	g.mu.Unlock()
}

func (g *HEVCSyncGate) IsSynced() bool {
	g.mu.Lock()
	synced := g.synced
	g.mu.Unlock()
	return synced
}

func inspectHEVCFrame(frame []byte) (bool, bool, bool, bool) {
	if len(frame) == 0 {
		return false, false, false, false
	}

	firstNALIndex := findAnnexBStartCode(frame, 0)
	if firstNALIndex != 0 {
		return false, false, false, false
	}

	hasVPS := false
	hasSPS := false
	hasPPS := false
	hasIRAP := false
	seenVCL := false

	for index := firstNALIndex; index >= 0; {
		headerIndex := index + startCodeLen(frame, index)
		if headerIndex >= len(frame) {
			break
		}

		nalType := (frame[headerIndex] >> 1) & 0x3F
		switch nalType {
		case hevcNALTypeVPS:
			if seenVCL {
				return false, false, false, false
			}
			hasVPS = true
		case hevcNALTypeSPS:
			if seenVCL {
				return false, false, false, false
			}
			hasSPS = true
		case hevcNALTypePPS:
			if seenVCL {
				return false, false, false, false
			}
			hasPPS = true
		case hevcNALTypeAUD:
			if seenVCL {
				return false, false, false, false
			}
		default:
			if nalType >= hevcNALTypeIRAPMin && nalType <= hevcNALTypeIRAPMax {
				seenVCL = true
				hasIRAP = true
			} else if nalType < hevcNALTypeVPS {
				seenVCL = true
			}
		}

		nextSearchFrom := headerIndex + 1
		if nextSearchFrom >= len(frame) {
			break
		}
		index = findAnnexBStartCode(frame, nextSearchFrom)
	}

	return hasVPS, hasSPS, hasPPS, hasIRAP
}
