<template>
  <div class="awareness-overlay" aria-live="polite">
    <div
      v-if="props.showLowHpVignette && props.lowHpLevel !== 'normal'"
      class="hp-vignette"
      :class="`level-${props.lowHpLevel}`"
      data-testid="hud-awareness-vignette"
    ></div>

    <transition name="alert-fade">
      <section
        v-if="props.alert"
        class="center-alert"
        :class="[`tone-${props.alert.tone}`, `level-${props.alert.level}`]"
        data-testid="hud-awareness-alert"
      >
        <small>{{ alertTag }}</small>
        <strong>{{ props.alert.title }}</strong>
        <span>{{ props.alert.detail }}</span>
      </section>
    </transition>

    <section
      v-if="props.armorCue"
      class="armor-cue hud-realtime"
      :style="armorCueStyle"
      data-testid="hud-awareness-armor"
    >
      <i class="armor-ring"></i>
      <div class="armor-label">
        <strong>{{ armorTitle }}</strong>
        <small>{{ armorDetail }}</small>
      </div>
    </section>

    <section
      v-if="props.threatCue"
      class="threat-chip"
      data-testid="hud-awareness-threat"
    >
      <span class="source">{{
        props.threatCue.source === "vision" ? "VIS" : "RAD"
      }}</span>
      <strong>{{ threatTitle }}</strong>
      <small>{{ threatDetail }}</small>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { AwarenessTargetCue } from "../../types";
import type {
  AwarenessAlert,
  AwarenessLevel,
} from "../../composables/useSituationalAwareness";

interface Props {
  alert: AwarenessAlert | null;
  lowHpLevel: AwarenessLevel;
  showLowHpVignette?: boolean;
  threatCue: AwarenessTargetCue | null;
  armorCue: AwarenessTargetCue | null;
}

const props = withDefaults(defineProps<Props>(), {
  showLowHpVignette: false,
});

function formatDirection(angleDeg: number): string {
  if (angleDeg <= -25) return "左侧";
  if (angleDeg < -8) return "左前";
  if (angleDeg < 8) return "正前";
  if (angleDeg < 25) return "右前";
  return "右侧";
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

// What: 为不同提醒类型生成统一短标签。
// Why: 中心弹窗头部要能一眼区分来源，但不能塞入冗长句子抢占中央视野。
const alertTag = computed(() => {
  if (!props.alert) return "";
  if (props.alert.kind === "low-hp") return "HP ALERT";
  if (props.alert.kind === "overheat") return "HEAT ALERT";
  if (props.alert.kind === "capacitor") return "CAP ALERT";
  return "TIME ALERT";
});

// What: 将融合后的主目标 cue 压缩成短文本。
// Why: 中央区域必须惜字如金，过长文案会直接遮挡射界与敌方轮廓。
const threatTitle = computed(() => {
  if (!props.threatCue) return "";
  if (props.threatCue.source === "vision") {
    return props.threatCue.label || "视觉锁定";
  }
  return `${formatDirection(props.threatCue.angleDeg)}敌情`;
});

const threatDetail = computed(() => {
  if (!props.threatCue) return "";
  const confidence = `${Math.round(props.threatCue.confidence * 100)}%`;
  if (props.threatCue.distanceM !== null) {
    return `${confidence} · ${props.threatCue.distanceM.toFixed(1)}m`;
  }
  return confidence;
});

// What: 把装甲识别标记限制在安全显示区域内。
// Why: 视觉目标靠边时若直接贴边渲染，标签会被裁掉，反而破坏识别体验。
const armorCueStyle = computed<Record<string, string>>(() => {
  if (
    !props.armorCue ||
    props.armorCue.xNorm === null ||
    props.armorCue.yNorm === null
  ) {
    return {} as Record<string, string>;
  }
  return {
    left: `${clamp(props.armorCue.xNorm, 0.08, 0.92) * 100}%`,
    top: `${clamp(props.armorCue.yNorm, 0.12, 0.88) * 100}%`,
  };
});

// What: 将装甲 cue 再压缩成一行标题和一行详情。
// Why: 标记本身已经占据画面主体附近，文字必须短而稳，不能重新变成大面积悬浮卡片。
const armorTitle = computed(() => props.armorCue?.label || "装甲锁定");

const armorDetail = computed(() => {
  if (!props.armorCue) return "";
  const confidence = `${Math.round(props.armorCue.confidence * 100)}%`;
  if (props.armorCue.distanceM !== null) {
    return `${confidence} · ${props.armorCue.distanceM.toFixed(1)}m`;
  }
  return confidence;
});
</script>

<style scoped>
.awareness-overlay {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: calc(var(--z-widget) + 2);
}

/* What: 低血量时只使用轻量边缘红晕。 */
/* Why: 目标是提醒风险而不是大面积挡视野，因此不能做厚重遮罩。 */
.hp-vignette {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  opacity: 0.72;
  box-shadow: inset 0 0 64px var(--hud-awareness-vignette-warn);
}

.hp-vignette.level-critical {
  opacity: 0.92;
  box-shadow: inset 0 0 92px var(--hud-awareness-vignette-critical);
  animation: warning-pulse 780ms ease-in-out infinite;
}

.center-alert {
  position: absolute;
  left: 50%;
  top: 15%;
  min-width: 210px;
  max-width: min(42vw, 440px);
  transform: translateX(-50%);
  padding: 10px 14px;
  display: grid;
  gap: 2px;
  border-radius: 18px;
  border: 1px solid var(--hud-awareness-alert-warning-border);
  background: var(--hud-awareness-alert-warning-bg);
  box-shadow: var(--hud-awareness-alert-warning-shadow);
  backdrop-filter: blur(10px);
}

.center-alert.tone-danger {
  border-color: var(--hud-awareness-alert-danger-border);
  background: var(--hud-awareness-alert-danger-bg);
  box-shadow: var(--hud-awareness-alert-danger-shadow);
}

.center-alert small,
.center-alert span {
  color: var(--hud-text-secondary);
}

.center-alert strong {
  color: var(--hud-text-primary);
  font-size: 15px;
  line-height: 1.1;
}

.center-alert small {
  font-size: 10px;
  line-height: 1;
  letter-spacing: 0.16em;
  font-family: var(--font-data);
}

.center-alert span {
  font-size: 11px;
  line-height: 1.2;
}

.armor-cue {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: visible;
}

/* What: 装甲识别标记只使用轻量圆环与短标签。
   Why: 既要告诉操作手“这里是已识别装甲板”，又不能做成大方框遮挡敌人轮廓。 */
.armor-ring {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 26px;
  height: 26px;
  transform: translate(-50%, -50%);
  border-radius: 999px;
  border: 1px solid var(--hud-awareness-armor-ring);
  background: var(--hud-awareness-armor-fill);
  box-shadow: var(--hud-awareness-armor-shadow);
}

.armor-ring::after {
  content: "";
  position: absolute;
  left: 50%;
  top: 50%;
  width: 4px;
  height: 4px;
  transform: translate(-50%, -50%);
  border-radius: 999px;
  background: var(--hud-awareness-armor-dot);
}

.armor-label {
  position: absolute;
  left: 50%;
  top: calc(50% - 18px);
  transform: translate(-50%, -100%);
  min-width: 84px;
  padding: 4px 8px;
  display: grid;
  justify-items: center;
  gap: 1px;
  border-radius: 999px;
  border: 1px solid var(--hud-awareness-armor-label-border);
  background: var(--hud-awareness-armor-label-bg);
  box-shadow: var(--hud-awareness-armor-label-shadow);
  backdrop-filter: blur(8px);
}

.armor-label strong {
  color: var(--hud-text-primary);
  font-size: 11px;
  line-height: 1.1;
}

.armor-label small {
  color: var(--hud-text-secondary);
  font-size: 10px;
  line-height: 1;
  font-family: var(--font-data);
}

.threat-chip {
  position: absolute;
  left: 50%;
  top: 59%;
  transform: translateX(-50%);
  min-width: 180px;
  max-width: min(30vw, 280px);
  padding: 8px 12px;
  display: grid;
  justify-items: center;
  gap: 1px;
  border-radius: 999px;
  border: 1px solid var(--hud-awareness-threat-border);
  background: var(--hud-awareness-threat-bg);
  box-shadow: var(--hud-awareness-threat-shadow);
  backdrop-filter: blur(8px);
}

.source {
  font-size: 10px;
  line-height: 1;
  color: var(--hud-awareness-threat-source);
  font-family: var(--font-data);
  letter-spacing: 0.18em;
}

.threat-chip strong {
  color: var(--hud-text-primary);
  font-size: 12px;
  line-height: 1.1;
}

.threat-chip small {
  color: var(--hud-text-secondary);
  font-size: 10px;
  line-height: 1;
  font-family: var(--font-data);
}

.alert-fade-enter-active,
.alert-fade-leave-active {
  transition:
    opacity 140ms ease,
    transform 140ms ease;
}

.alert-fade-enter-from,
.alert-fade-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(-6px);
}
</style>
