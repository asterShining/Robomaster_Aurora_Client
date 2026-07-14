<template>
  <section
    class="score-card hud-realtime"
    :class="{ unavailable: !props.gameStatusReady }"
  >
    <template v-if="props.gameStatusReady">
      <div class="score-row">
        <div class="score-block score-block--red">
          <small>红方</small>
          <b>{{ props.redScore }}</b>
        </div>

        <div class="score-center">
          <span>比 分</span>
          <small v-if="props.isPaused" class="pause-pill">已暂停</small>
        </div>

        <div class="score-block score-block--blue">
          <small>蓝方</small>
          <b>{{ props.blueScore }}</b>
        </div>
      </div>

      <div class="meta-row">
        <small class="meta-pill" data-testid="hud-match-round">{{
          roundLabel
        }}</small>
        <small class="meta-pill" data-testid="hud-match-stage">{{
          stageLabel
        }}</small>
      </div>

      <div class="meta-row">
        <small
          class="meta-pill cue-pill"
          :class="`level-${props.countdownLevel}`"
          data-testid="hud-match-countdown"
        >
          剩余 {{ countdownText }}
        </small>
        <small class="meta-pill" data-testid="hud-match-elapsed"
          >已进行 {{ elapsedText }}</small
        >
      </div>
    </template>

    <section v-else class="offline-shell" data-testid="hud-match-unavailable">
      <strong>比赛信息未接入</strong>
      <small>等待真实 GameStatus</small>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { AwarenessLevel } from "../../../composables/useSituationalAwareness";

interface Props {
  gameStatusReady: boolean;
  currentRound: number;
  totalRounds: number;
  redScore: number;
  blueScore: number;
  currentStage: number;
  stageCountdownSec: number;
  stageElapsedSec: number;
  isPaused: boolean;
  countdownLevel?: AwarenessLevel;
}

const props = withDefaults(defineProps<Props>(), {
  countdownLevel: "normal",
});

function formatClock(totalSeconds: number): string {
  // What: 将官方秒数字段格式化为 mm:ss。
  // Why: 顶部中条直接消费稳定文本，避免模板层重复拼接时间字符串。
  const safeSeconds = Math.max(0, Math.round(totalSeconds));
  const minutes = Math.floor(safeSeconds / 60);
  const seconds = safeSeconds % 60;
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

function resolveStageLabel(stage: number): string {
  // What: 将协议 current_stage 数值翻译为固定中文标签。
  // Why: 用户要求顶部中条严格对齐官方协议语义，阶段必须可直接读懂，不能只显示裸数字。
  if (stage === 0) return "未开始比赛";
  if (stage === 1) return "准备阶段";
  if (stage === 2) return "裁判自检";
  if (stage === 3) return "五秒倒计时";
  if (stage === 4) return "比赛中";
  if (stage === 5) return "比赛结算中";
  return "未知阶段";
}

// What: 局次文本在 totalRounds 缺失时做保守降级。
// Why: 现场有些版本可能先给当前局次、后给总局数，顶部中条不能因此抖成空白。
const roundLabel = computed(() => {
  if (props.totalRounds > 0)
    return `第 ${props.currentRound} / ${props.totalRounds} 局`;
  if (props.currentRound > 0) return `第 ${props.currentRound} 局`;
  return "局次 未知";
});

// What: 阶段标签统一通过映射生成。
// Why: 这样组件不需要感知协议文档，只依赖单一映射出口。
const stageLabel = computed(
  () => `阶段 ${resolveStageLabel(props.currentStage)}`,
);

// What: 剩余时间直接显示当前阶段倒计时。
// Why: 这是官方 GameStatus 里最关键的比赛时间字段，用户要求顶栏以真实时间为准。
const countdownText = computed(() => formatClock(props.stageCountdownSec));

// What: 已进行时间单独保留。
// Why: 它来自同一条官方 GameStatus，可帮助快速核对阶段切换是否正常，不需要额外假字段。
const elapsedText = computed(() => formatClock(props.stageElapsedSec));
</script>

<style scoped>
.score-card {
  width: 100%;
  height: 100%;
  border-radius: 26px;
  border: 1px solid var(--hud-border-strong);
  background: linear-gradient(
    165deg,
    var(--hud-surface-1),
    var(--hud-surface-0)
  );
  display: grid;
  align-content: center;
  gap: 6px;
  padding: 8px 12px;
}

.score-card.unavailable {
  border-color: rgba(112, 136, 188, 0.32);
}

.score-row {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 8px;
}

.score-block {
  display: grid;
  justify-items: center;
  gap: 2px;
}

.score-block small,
.score-center span {
  font-size: 10px;
  color: var(--hud-text-secondary);
  letter-spacing: 0.12em;
}

.score-block b {
  font-size: 22px;
  line-height: 1;
  font-family: var(--font-data);
}

.score-block--red b {
  color: var(--hud-score-red);
}

.score-block--blue b {
  color: var(--hud-score-blue);
}

.score-center {
  display: grid;
  justify-items: center;
  gap: 4px;
}

.pause-pill {
  padding: 2px 8px;
  border-radius: 999px;
  border: 1px solid rgba(255, 201, 128, 0.42);
  background: rgba(68, 42, 9, 0.42);
  color: var(--hud-warning);
  font-size: 10px;
  line-height: 1;
  letter-spacing: 0.08em;
}

.meta-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex-wrap: wrap;
}

.meta-pill {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 10px;
  color: var(--hud-text-secondary);
  white-space: nowrap;
  background: var(--hud-top-score-pill-bg);
  border: 1px solid var(--hud-top-score-pill-border);
}

.cue-pill {
  transition:
    border-color 120ms ease,
    color 120ms ease,
    box-shadow 120ms ease,
    background 120ms ease;
}

.cue-pill.level-warn {
  color: var(--hud-warning);
  border-color: var(--hud-top-score-warn-border);
  background: var(--hud-top-score-warn-bg);
}

.cue-pill.level-critical {
  color: var(--hud-danger-text);
  border-color: var(--hud-top-score-critical-border);
  background: var(--hud-top-score-critical-bg);
  box-shadow: var(--hud-top-score-critical-shadow);
  animation: warning-pulse 760ms ease-in-out infinite;
}

.offline-shell {
  height: 100%;
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 5px;
  text-align: center;
}

.offline-shell strong {
  color: var(--hud-text-primary);
  font-size: 14px;
  letter-spacing: 0.08em;
}

.offline-shell small {
  color: var(--hud-text-secondary);
  font-size: 10px;
  letter-spacing: 0.1em;
}
</style>
