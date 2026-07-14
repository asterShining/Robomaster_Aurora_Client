<template>
  <section class="pose-card hud-realtime" :style="cardStyle">
    <header class="card-head">
      <h3>车辆姿态</h3>
      <span class="status-dot" :class="{ online: gimbal.isOnline }"></span>
    </header>

    <div class="pose-ring" :style="ringStyle">
      <i class="ring-core"></i>
    </div>

    <div class="pose-meta">
      <div>
        <span class="k">Yaw</span>
        <span class="v">{{ gimbal.yawDeg.toFixed(1) }}°</span>
      </div>
      <div>
        <span class="k">Pitch</span>
        <span class="v">{{ gimbal.pitchDeg.toFixed(1) }}°</span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useRobotDataStore } from '../../store/robotData'

interface Props {
  scale?: number
  opacity?: number
}

const props = withDefaults(defineProps<Props>(), {
  scale: 1,
  opacity: 0.95,
})

const robotStore = useRobotDataStore()
const { gimbal, chassis } = storeToRefs(robotStore)

const cardStyle = computed<Record<string, string>>(() => ({
  transform: `scale(${props.scale}) translateZ(0)`,
  opacity: String(props.opacity),
}))

const ringStyle = computed<Record<string, string>>(() => {
  const heading = ((chassis.value.headingDeg % 360) + 360) % 360
  return {
    '--heading': `${heading}deg`,
  } as Record<string, string>
})
</script>

<style scoped>
.pose-card {
  width: 172px;
  padding: 10px;
  border-radius: 12px;
  pointer-events: auto;
  background: linear-gradient(160deg, rgba(12, 18, 34, 0.95), rgba(9, 15, 30, 0.88));
  border: 1px solid rgba(78, 118, 214, 0.26);
  box-shadow: 0 10px 24px rgba(0, 7, 24, 0.58), inset 0 0 0 1px rgba(14, 33, 74, 0.5);
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.card-head h3 {
  margin: 0;
  font-size: 13px;
  color: #ff8075;
  letter-spacing: 0.04em;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: rgba(116, 131, 162, 0.8);
  box-shadow: 0 0 0 2px rgba(90, 104, 130, 0.2);
}

.status-dot.online {
  background: #21e7ff;
  box-shadow: 0 0 8px rgba(43, 225, 255, 0.65);
}

.pose-ring {
  width: 122px;
  height: 122px;
  margin: 0 auto 10px;
  border-radius: 999px;
  position: relative;
  display: grid;
  place-items: center;
  background:
    radial-gradient(circle at center, rgba(8, 15, 30, 0.92) 34%, transparent 34%),
    conic-gradient(from var(--heading), #28f2ff 0 36%, rgba(19, 32, 58, 0.26) 36% 58%, #5f45ff 58% 70%, rgba(14, 24, 42, 0.2) 70% 100%);
  box-shadow: inset 0 0 0 10px rgba(7, 13, 25, 0.82), 0 0 18px rgba(45, 192, 255, 0.28);
}

.ring-core {
  width: 32px;
  height: 32px;
  border-radius: 999px;
  border: 1px solid rgba(80, 178, 255, 0.6);
  box-shadow: inset 0 0 10px rgba(49, 131, 255, 0.45);
}

.pose-meta {
  display: grid;
  gap: 6px;
}

.pose-meta > div {
  display: flex;
  justify-content: space-between;
}

.k {
  font-size: 11px;
  color: #6a88bb;
}

.v {
  font-size: 12px;
  color: #d6e7ff;
  font-family: 'JetBrains Mono', 'SF Mono', 'Consolas', monospace;
}
</style>
