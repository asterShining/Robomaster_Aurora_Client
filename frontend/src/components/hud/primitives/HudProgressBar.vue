<template>
  <div class="hud-progress">
    <div class="track">
      <div class="fill" :style="fillStyle"></div>
    </div>
    <span class="value">{{ valueText }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  value: number
  max: number
  color?: 'cyan' | 'red' | 'green' | 'blue' | 'warning' | 'danger'
  decimals?: number
}

const props = withDefaults(defineProps<Props>(), {
  color: 'cyan',
  decimals: 0,
})

const percent = computed(() => {
  const safeMax = Math.max(1, props.max)
  return Math.max(0, Math.min(100, (props.value / safeMax) * 100))
})

const valueText = computed(() => `${props.value.toFixed(props.decimals)} / ${props.max.toFixed(props.decimals)}`)

// What: 通过 CSS 变量驱动进度条颜色和宽度。Why: 降低模板分支复杂度，便于多业务复用。
const fillStyle = computed<Record<string, string>>(() => ({
  '--fill-percent': `${percent.value}%`,
  // What: 对 warning/danger 这类语义色做 token 映射。
  // Why: 进度条需要复用统一色板，而不是要求每个调用方自行拼接 CSS 变量名。
  '--fill-color':
    props.color === 'warning'
      ? 'var(--hud-warning)'
      : props.color === 'danger'
        ? 'var(--hud-red)'
        : `var(--hud-${props.color})`,
}))
</script>

<style scoped>
.hud-progress {
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.track {
  position: relative;
  height: 10px;
  border: 1px solid rgba(122, 163, 255, 0.4);
  background: rgba(7, 12, 24, 0.8);
  overflow: hidden;
}

.fill {
  width: var(--fill-percent);
  height: 100%;
  background: linear-gradient(90deg, color-mix(in srgb, var(--fill-color), #ffffff 6%), color-mix(in srgb, var(--fill-color), #000000 20%));
  box-shadow: 0 0 12px color-mix(in srgb, var(--fill-color), transparent 40%);
  transition: width 140ms linear;
}

.value {
  font-size: 12px;
  line-height: 1;
  color: rgba(222, 238, 255, 0.94);
  font-family: 'JetBrains Mono', 'Consolas', monospace;
  white-space: nowrap;
}
</style>
