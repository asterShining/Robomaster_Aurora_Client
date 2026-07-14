<template>
  <HudWidgetCard title="态势雷达" :subtitle="sourceLabel" accent="cyan">
    <div class="radar-shell hud-realtime">
      <svg
        v-if="hasBackendRadar"
        class="radar-svg"
        viewBox="0 0 120 120"
        aria-hidden="true"
      >
        <defs>
          <radialGradient id="radarCoreGlow" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stop-color="rgba(123, 245, 255, 0.16)" />
            <stop offset="100%" stop-color="rgba(123, 245, 255, 0)" />
          </radialGradient>
        </defs>

        <circle class="ring ring-outer" cx="60" cy="60" r="52" />
        <circle class="ring ring-mid" cx="60" cy="60" r="34" />
        <circle class="ring ring-inner" cx="60" cy="60" r="16" />
        <circle class="axis-dot" cx="60" cy="60" r="3.4" />
        <circle
          class="core-glow"
          cx="60"
          cy="60"
          r="28"
          fill="url(#radarCoreGlow)"
        />

        <g class="sweep-layer" :style="sweepStyle">
          <path d="M60 60 L60 10 A50 50 0 0 1 96 26 Z" />
          <line x1="60" y1="60" x2="60" y2="10" />
        </g>

        <g :transform="headingTransform">
          <line class="heading-line" x1="60" y1="60" x2="60" y2="18" />
          <circle class="self-dot" cx="60" cy="60" r="4.2" />
        </g>

        <g
          v-for="point in plottedContacts"
          :key="point.id"
          class="contact"
          :class="[`side-${point.side}`, { primary: point.isPrimary }]"
          :transform="`translate(${point.cx} ${point.cy})`"
        >
          <circle class="contact-core" r="2.8" />
          <circle v-if="point.isPrimary" class="contact-lock" r="6.2" />
        </g>
      </svg>

      <!-- What: 无真实 radar-state 时显示未接入占位。Why: 用户已明确拒绝 simulated 目标点，空白也必须有清晰语义。 -->
      <section v-else class="radar-empty" data-testid="hud-radar-offline">
        <strong>未接入</strong>
        <small>等待真实 radar-state</small>
      </section>

      <footer class="radar-foot">
        <span>范围 {{ Math.round(props.rangeM) }}m</span>
        <span
          v-if="hasBackendRadar && props.showThreatFusion && props.primaryCue"
          class="focus-pill"
          data-testid="hud-radar-focus"
        >
          {{ focusLabel }}
        </span>
        <span>目标 {{ plottedContacts.length }}</span>
      </footer>
    </div>
  </HudWidgetCard>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type {
  AwarenessTargetCue,
  RadarContact,
  RadarDataSource,
} from "../../../types";
import HudWidgetCard from "../primitives/HudWidgetCard.vue";

interface Props {
  contacts: RadarContact[];
  sweepDeg: number;
  rangeM: number;
  headingDeg: number;
  source: RadarDataSource;
  primaryCue?: AwarenessTargetCue | null;
  showThreatFusion?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  contacts: () => [],
  sweepDeg: 0,
  rangeM: 15,
  headingDeg: 0,
  source: "unknown",
  primaryCue: null,
  showThreatFusion: false,
});

// What: 只在明确拿到 backend 雷达时把组件视为已接入。
// Why: 没有真实来源时，组件必须稳定落到“未接入”降级态，不能再显示 simulated 字样误导用户。
const hasBackendRadar = computed(() => props.source === "backend");

const sourceLabel = computed(() =>
  hasBackendRadar.value ? "已接入" : "未接入",
);

// What: 将米制坐标投影到雷达极坐标画布。
// Why: 统一转换逻辑可保证目标点越界时被裁剪，避免 SVG 外溢导致穿模。
const plottedContacts = computed(() => {
  if (!hasBackendRadar.value) return [];

  const safeRange = Math.max(1, props.rangeM);
  return props.contacts.slice(0, 32).map((contact) => {
    const dist = Math.sqrt(contact.x * contact.x + contact.y * contact.y);
    const ratio = Math.min(1, dist / safeRange);
    const angle = Math.atan2(contact.y, contact.x);
    const radiusPx = ratio * 50;

    return {
      id: contact.id,
      side: contact.side,
      cx: 60 + Math.cos(angle) * radiusPx,
      cy: 60 - Math.sin(angle) * radiusPx,
      isPrimary:
        props.showThreatFusion && props.primaryCue?.radarId === contact.id,
    };
  });
});

// What: 扫描线只更新 transform 角度。
// Why: 高频刷新时避免重新布局，保持 100Hz 下的稳定帧率。
const sweepStyle = computed(() => ({
  transform: `rotate(${props.sweepDeg}deg)`,
}));

const headingTransform = computed(() => `rotate(${props.headingDeg} 60 60)`);

const focusLabel = computed(() => {
  if (!props.showThreatFusion || !props.primaryCue) return "";
  const sourceLabel = props.primaryCue.source === "vision" ? "VIS" : "RAD";
  const confidence = `${Math.round(props.primaryCue.confidence * 100)}%`;
  return `${sourceLabel} ${confidence}`;
});
</script>

<style scoped>
.radar-shell {
  width: 100%;
  height: 100%;
  display: grid;
  grid-template-rows: 1fr auto;
  gap: 6px;
  pointer-events: none;
}

.radar-svg {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.radar-empty {
  height: 100%;
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 6px;
  text-align: center;
}

.radar-empty strong {
  color: var(--hud-text-primary);
  font-size: 15px;
  letter-spacing: 0.08em;
}

.radar-empty small {
  color: var(--hud-text-secondary);
  font-size: 10px;
  letter-spacing: 0.08em;
}

.ring {
  fill: none;
  stroke: var(--hud-radar-ring);
  stroke-width: 1;
}

.ring-outer {
  stroke-width: 1.3;
}

.ring-mid,
.ring-inner {
  stroke-dasharray: 2 4;
}

.axis-dot {
  fill: var(--hud-radar-core);
}

.sweep-layer {
  transform-origin: 60px 60px;
  transition: transform 120ms linear;
  will-change: transform;
}

.sweep-layer path {
  fill: var(--hud-radar-sweep-fill);
}

.sweep-layer line {
  stroke: var(--hud-radar-sweep-line);
  stroke-width: 1.2;
}

.heading-line {
  stroke: var(--hud-radar-heading);
  stroke-width: 1;
}

.self-dot {
  fill: var(--hud-radar-self);
  filter: drop-shadow(0 0 6px rgba(208, 236, 255, 0.44));
}

.contact-core {
  fill: var(--hud-radar-friendly);
  filter: drop-shadow(0 0 6px rgba(112, 255, 178, 0.5));
}

.contact.side-enemy .contact-core {
  fill: var(--hud-radar-enemy);
  filter: drop-shadow(0 0 6px rgba(255, 129, 129, 0.48));
}

.contact.side-neutral .contact-core {
  fill: var(--hud-radar-neutral);
  filter: drop-shadow(0 0 6px rgba(255, 223, 140, 0.45));
}

.contact-lock {
  fill: none;
  stroke: var(--hud-radar-primary-ring);
  stroke-width: 1.15;
  animation: warning-pulse 860ms ease-in-out infinite;
}

.contact.primary .contact-core {
  fill: var(--hud-radar-primary-fill);
  filter: drop-shadow(0 0 8px var(--hud-radar-primary-shadow));
}

.radar-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 10px;
  color: var(--hud-text-secondary);
  font-family: var(--font-data);
}

.focus-pill {
  padding: 2px 8px;
  border-radius: 999px;
  color: var(--hud-text-primary);
  border: 1px solid var(--hud-awareness-threat-border);
  background: rgba(10, 18, 34, 0.74);
}
</style>
