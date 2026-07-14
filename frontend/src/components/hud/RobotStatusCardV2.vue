<template>
  <section class="robot-card hud-realtime" :style="cardStyle">
    <header class="card-head">
      <h3>本车状态</h3>
      <span class="state-text" :class="stateClass">{{ stateLabel }}</span>
    </header>

    <div class="row">
      <span class="label">血量</span>
      <span class="value is-red">{{ hp }} / {{ maxHp }}</span>
    </div>

    <div class="row">
      <span class="label">功率</span>
      <span class="value is-cyan">{{ powerDisplay }}</span>
    </div>

    <div class="bar">
      <span class="bar-label">热量</span>
      <div class="track"><i class="fill is-heat" :style="heatStyle"></i></div>
      <span class="num">{{ heat }} / {{ maxHeat }}</span>
    </div>

    <div class="bar">
      <span class="bar-label">弹速</span>
      <div class="track"><i class="fill is-speed" :style="speedStyle"></i></div>
      <span class="num">{{ bulletSpeed.toFixed(1) }}</span>
    </div>

    <div class="row">
      <span class="label">剩余发弹量</span>
      <span class="value">{{ ammoAllowance }} 发</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useRobotDataStore } from '../../store/robotData'

interface Props {
  hp?: number
  maxHp?: number
  heat?: number
  maxHeat?: number
  bulletSpeed?: number
  ammoAllowance?: number
  scale?: number
  opacity?: number
}

const props = withDefaults(defineProps<Props>(), {
  hp: 350,
  maxHp: 350,
  heat: 0,
  maxHeat: 100,
  bulletSpeed: 0,
  ammoAllowance: 0,
  scale: 1,
  opacity: 0.95,
})

const robotStore = useRobotDataStore()
const { chassis, connection } = storeToRefs(robotStore)

const heatPercent = computed(() => Math.max(0, Math.min(100, (props.heat / Math.max(props.maxHeat, 1)) * 100)))
const speedPercent = computed(() => Math.max(0, Math.min(100, (props.bulletSpeed / 40) * 100)))
const powerDisplay = computed(() => `${chassis.value.powerW.toFixed(1)}W / 55W`)

const heatStyle = computed<Record<string, string>>(() => ({ width: `${heatPercent.value}%` }))
const speedStyle = computed<Record<string, string>>(() => ({ width: `${speedPercent.value}%` }))

const cardStyle = computed<Record<string, string>>(() => ({
  transform: `scale(${props.scale}) translateZ(0)`,
  opacity: String(props.opacity),
}))

const stateLabel = computed(() => (connection.value.backendConnected ? '在线' : '断联'))
const stateClass = computed(() => (connection.value.backendConnected ? 'is-online' : 'is-offline'))
</script>

<style scoped>
.robot-card {
  width: 270px;
  padding: 12px;
  border-radius: 12px;
  pointer-events: auto;
  background: linear-gradient(165deg, rgba(13, 18, 34, 0.96), rgba(8, 13, 26, 0.9));
  border: 1px solid rgba(82, 124, 255, 0.24);
  box-shadow: 0 10px 26px rgba(0, 8, 26, 0.6), inset 0 0 0 1px rgba(21, 43, 86, 0.52);
}

.card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.card-head h3 {
  margin: 0;
  font-size: 14px;
  color: #ff7970;
}

.state-text {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 999px;
}

.state-text.is-online {
  color: #83e8ff;
  background: rgba(36, 112, 174, 0.35);
}

.state-text.is-offline {
  color: #ff8f8f;
  background: rgba(135, 29, 44, 0.35);
}

.row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  margin-bottom: 8px;
}

.label {
  color: #90a9d6;
}

.value {
  color: #d9e7ff;
  font-family: 'JetBrains Mono', 'SF Mono', 'Consolas', monospace;
}

.value.is-red {
  color: #ff8179;
}

.value.is-cyan {
  color: #53ecff;
}

.bar {
  display: grid;
  grid-template-columns: 32px 1fr auto;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.bar-label,
.num {
  font-size: 11px;
  color: #6f92ca;
}

.track {
  height: 7px;
  border-radius: 999px;
  background: rgba(125, 154, 210, 0.18);
  overflow: hidden;
  box-shadow: inset 0 0 8px rgba(7, 16, 31, 0.84);
}

.fill {
  display: block;
  height: 100%;
  transition: width 120ms linear;
}

.fill.is-heat {
  background: linear-gradient(90deg, #2fd2ff, #ffcf3f);
}

.fill.is-speed {
  background: linear-gradient(90deg, #35f0ff, #4c7cff);
}
</style>
