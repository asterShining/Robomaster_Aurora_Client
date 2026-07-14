<template>
  <div
    class="crosshair-root"
    :class="{ 'imminent-heat': props.imminentOverheat }"
    aria-hidden="true"
    :style="crosshairStyle"
    data-testid="hud-crosshair"
  >
    <svg class="crosshair-svg" viewBox="0 0 100 100">
      <circle class="ring-base" cx="50" cy="50" r="26" />
      <circle class="ring-heat" cx="50" cy="50" r="26" :stroke-dasharray="dashArray" />

      <line class="tick" x1="50" y1="31" x2="50" y2="40" />
      <line class="tick" x1="50" y1="60" x2="50" y2="69" />
      <line class="tick" x1="31" y1="50" x2="40" y2="50" />
      <line class="tick" x1="60" y1="50" x2="69" y2="50" />

      <circle class="center-dot" cx="50" cy="50" r="1.4" />
    </svg>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  heat: number
  maxHeat: number
  color?: string
  imminentOverheat?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  color: '#EAF3FF',
  imminentOverheat: false,
})

// What: 用圆环周长映射热量占比。
// Why: 在不增加额外 HUD 噪声的前提下，仍然保留连续可读的热量反馈。
const dashArray = computed(() => {
  const safeMax = Math.max(1, props.maxHeat)
  const ratio = Math.max(0, Math.min(1, props.heat / safeMax))
  const perimeter = Math.PI * 2 * 26
  const active = perimeter * ratio
  return `${active} ${perimeter}`
})

const crosshairStyle = computed<Record<string, string>>(() => ({
  '--crosshair-color': props.color,
}))
</script>

<style scoped>
.crosshair-root {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  position: relative;
  pointer-events: none;
}

.crosshair-svg {
  width: min(100%, 128px);
  height: min(100%, 128px);
  overflow: visible;
  filter: drop-shadow(0 0 6px rgba(192, 213, 255, 0.12));
}

.ring-base,
.ring-heat,
.tick,
.center-dot {
  vector-effect: non-scaling-stroke;
}

.ring-base {
  fill: none;
  stroke: color-mix(in srgb, var(--crosshair-color) 42%, transparent);
  stroke-width: 1;
}

.ring-heat {
  fill: none;
  stroke: var(--hud-crosshair-heat);
  stroke-width: 1.25;
  stroke-linecap: round;
  transform-origin: 50px 50px;
  transform: rotate(-90deg);
  transition: stroke-dasharray 120ms linear, stroke 120ms ease, stroke-width 120ms ease;
}

.tick {
  stroke: var(--crosshair-color);
  stroke-width: 1.1;
  stroke-linecap: round;
}

.center-dot {
  fill: var(--crosshair-color);
}

/* What: 即将过热时只增强热量圆环和中心点。 */
/* Why: 提示必须聚焦在瞄准核心区域，但不能突然长出大块文字或厚重特效干扰射击。 */
.crosshair-root.imminent-heat .ring-heat {
  stroke: var(--hud-crosshair-heat-imminent);
  stroke-width: 1.6;
  animation: warning-pulse 720ms ease-in-out infinite;
}

.crosshair-root.imminent-heat .center-dot {
  filter: drop-shadow(0 0 4px var(--hud-crosshair-heat-imminent));
}
</style>
