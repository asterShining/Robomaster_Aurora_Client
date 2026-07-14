<template>
  <HudPanel title="模块状态" :editMode="props.editMode">
    <div class="ms-body">

      <!-- 相机 -->
      <div class="ms-row">
        <span class="ms-row__name">相机</span>
        <div class="ms-row__right">
          <StatusChip
            :status="props.camera === 'connected' ? 'good' : 'error'"
            :label="props.camera === 'connected' ? '已连接' : '未连接'"
          />
        </div>
      </div>

      <!-- 检测 -->
      <div class="ms-row">
        <span class="ms-row__name">检测</span>
        <div class="ms-row__right">
          <StatusChip
            :status="props.detection === 'running' ? 'good' : 'warn'"
            :label="props.detection === 'running' ? '运行中' : '服务未启动'"
          />
          <button
            v-if="props.detection !== 'running'"
            class="ms-action-btn"
            @click="$emit('startDetection')"
          >启动</button>
        </div>
      </div>

      <!-- 通信 -->
      <div class="ms-row">
        <span class="ms-row__name">通信</span>
        <div class="ms-row__right">
          <StatusChip
            :status="commStatusClass"
            :label="commLabel"
          />
        </div>
      </div>

      <!-- 电源 -->
      <div class="ms-row">
        <span class="ms-row__name">电源</span>
        <div class="ms-row__right">
          <div class="ms-power-bar">
            <div
              class="ms-power-bar__fill"
              :class="powerClass"
              :style="{ width: props.powerPercent + '%' }"
            ></div>
          </div>
          <span class="ms-power-val">{{ props.powerPercent }}%</span>
        </div>
      </div>

    </div>
  </HudPanel>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import HudPanel from './HudPanel.vue'
import StatusChip from './StatusChip.vue'

const props = withDefaults(defineProps<{
  camera?: 'connected' | 'disconnected'
  detection?: 'running' | 'stopped'
  comm?: 'good' | 'poor' | 'disconnected'
  powerPercent?: number
  editMode?: boolean
}>(), {
  camera: 'disconnected',
  detection: 'stopped',
  comm: 'good',
  powerPercent: 84,
  editMode: false,
})

defineEmits<{ startDetection: [] }>()

const commLabel = computed(() => ({
  good: '良好', poor: '较差', disconnected: '断开',
}[props.comm]))

const commStatusClass = computed(() => ({
  good: 'good', poor: 'warn', disconnected: 'error',
}[props.comm] as 'good' | 'warn' | 'error'))

const powerClass = computed(() => {
  if (props.powerPercent > 50) return 'ms-power-bar__fill--high'
  if (props.powerPercent > 20) return 'ms-power-bar__fill--mid'
  return 'ms-power-bar__fill--low'
})
</script>

<style scoped>
.ms-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 200px;
  font-family: 'Baloo 2', 'Patrick Hand', sans-serif;
  color: #e8dff5;
}

.ms-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.ms-row__name {
  font-size: 12px;
  font-weight: 700;
  color: rgba(232, 223, 245, 0.5);
  min-width: 36px;
}

.ms-row__right {
  display: flex;
  align-items: center;
  gap: 6px;
}

.ms-action-btn {
  background: #f5c542;
  border: 2px solid rgba(245, 197, 66, 0.6);
  border-radius: 6px;
  color: #0d0b18;
  font-size: 11px;
  font-weight: 700;
  font-family: 'Baloo 2', 'Patrick Hand', sans-serif;
  padding: 2px 10px;
  cursor: pointer;
  transition: background 0.15s;
  pointer-events: all;
  box-shadow: 0 2px 0 rgba(245, 197, 66, 0.5);
}
.ms-action-btn:hover { background: #ffd966; }
.ms-action-btn:active { transform: translateY(2px); box-shadow: none; }

.ms-power-bar {
  width: 64px;
  height: 8px;
  background: rgba(232, 223, 245, 0.07);
  border-radius: 999px;
  overflow: hidden;
  border: 2px solid #2e2a48;
}

.ms-power-bar__fill {
  height: 100%;
  border-radius: inherit;
  transition: width 0.4s ease;
}

.ms-power-bar__fill--high { background: linear-gradient(90deg, #0d9e6a, #2ae084); }
.ms-power-bar__fill--mid  { background: linear-gradient(90deg, #c47800, #f5c542); }
.ms-power-bar__fill--low  { background: linear-gradient(90deg, #cc2233, #ff4455); }

.ms-power-val {
  font-size: 12px;
  font-weight: 700;
  color: #e8dff5;
  min-width: 32px;
  text-align: right;
}
</style>
