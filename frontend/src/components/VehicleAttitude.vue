<template>
  <HudPanel title="车辆姿态" :defaultX="0" :defaultY="0" :editMode="props.editMode">
    <div class="va-body">
      <!-- 同心圆姿态仪 -->
      <div class="va-dial">
        <svg viewBox="0 0 120 120" width="120" height="120">
          <!-- 外环 -->
          <circle cx="60" cy="60" r="56" fill="none" stroke="rgba(255,255,255,0.06)" stroke-width="1.5"/>
          <!-- 刻度环 -->
          <circle cx="60" cy="60" r="48" fill="none" stroke="rgba(0,180,216,0.2)" stroke-width="1"/>
          <!-- 内环背景 -->
          <circle cx="60" cy="60" r="38" fill="rgba(0,180,216,0.04)" stroke="rgba(0,180,216,0.3)" stroke-width="1.5"/>
          <!-- 细十字辅助线 -->
          <line x1="60" y1="12" x2="60" y2="22" stroke="rgba(255,255,255,0.15)" stroke-width="1"/>
          <line x1="60" y1="98" x2="60" y2="108" stroke="rgba(255,255,255,0.15)" stroke-width="1"/>
          <line x1="12" y1="60" x2="22" y2="60" stroke="rgba(255,255,255,0.15)" stroke-width="1"/>
          <line x1="98" y1="60" x2="108" y2="60" stroke="rgba(255,255,255,0.15)" stroke-width="1"/>
          <!-- 方位字母 -->
          <text x="60" y="10" text-anchor="middle" fill="rgba(200,212,224,0.5)" font-size="8" font-family="monospace">N</text>
          <text x="60" y="116" text-anchor="middle" fill="rgba(200,212,224,0.3)" font-size="7" font-family="monospace">S</text>
          <text x="8"  y="63"  text-anchor="middle" fill="rgba(200,212,224,0.3)" font-size="7" font-family="monospace">W</text>
          <text x="114" y="63" text-anchor="middle" fill="rgba(200,212,224,0.3)" font-size="7" font-family="monospace">E</text>

          <!-- 偏航指示针 (绕中心旋转) -->
          <g :transform="`rotate(${props.yaw}, 60, 60)`">
            <line x1="60" y1="22" x2="60" y2="42" stroke="#00b4d8" stroke-width="2" stroke-linecap="round"/>
            <polygon points="60,18 57,26 63,26" fill="#00b4d8"/>
          </g>

          <!-- 中心点 -->
          <circle cx="60" cy="60" r="4" fill="#00b4d8" opacity="0.9"/>
          <circle cx="60" cy="60" r="8" fill="none" stroke="rgba(0,180,216,0.4)" stroke-width="1"/>

          <!-- 俯仰环 (横线，随pitch偏移) -->
          <line
            :x1="30" :y1="60 + props.pitch * 0.4"
            :x2="90" :y2="60 + props.pitch * 0.4"
            stroke="rgba(0,229,160,0.5)" stroke-width="1" stroke-dasharray="3 3"
          />
        </svg>

        <!-- 角度数字叠加 -->
        <div class="va-overlay">
          <span class="va-angle-val">{{ props.yaw.toFixed(1) }}°</span>
        </div>
      </div>

      <!-- 底部数值 -->
      <div class="va-values">
        <div class="va-val-item">
          <span class="va-val-label">偏航</span>
          <span class="va-val-num">{{ props.yaw.toFixed(1) }}°</span>
        </div>
        <div class="va-val-item">
          <span class="va-val-label">俯仰</span>
          <span class="va-val-num">{{ props.pitch.toFixed(1) }}°</span>
        </div>
      </div>
    </div>
  </HudPanel>
</template>

<script setup lang="ts">
import HudPanel from './HudPanel.vue'

const props = withDefaults(defineProps<{
  yaw?: number
  pitch?: number
  editMode?: boolean
}>(), {
  yaw: 0,
  pitch: 0,
  editMode: false,
})
</script>

<style scoped>
.va-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  font-family: 'Baloo 2', 'Patrick Hand', sans-serif;
  color: #e8dff5;
}

.va-dial {
  position: relative;
  width: 120px;
  height: 120px;
}

.va-dial :deep(circle),
.va-dial :deep(line) {
  /* SVG 内部 stroke/fill 属性不覆盖 */
}

.va-overlay {
  position: absolute;
  bottom: 6px;
  left: 50%;
  transform: translateX(-50%);
  pointer-events: none;
}

.va-angle-val {
  font-size: 10px;
  font-family: 'Courier New', monospace;
  font-weight: 700;
  color: rgba(232, 223, 245, 0.5);
  letter-spacing: 0.5px;
}

.va-values {
  display: flex;
  gap: 16px;
  width: 100%;
  padding: 0 4px;
}

.va-val-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 1;
  gap: 2px;
}

.va-val-label {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 1px;
  text-transform: uppercase;
  color: rgba(232, 223, 245, 0.35);
}

.va-val-num {
  font-size: 13px;
  font-family: 'Courier New', monospace;
  font-weight: 700;
  color: #529aff;
}
</style>
