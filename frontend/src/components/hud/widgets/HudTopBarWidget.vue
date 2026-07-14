<template>
  <div class="top-bar-widget">
    <section class="team-zone side-red">
      <header class="team-head">
        <span class="team-name">红方机器人</span>
        <b class="team-score">{{ redVictory }}</b>
      </header>
      <div class="victory-track">
        <i class="victory-fill" :style="{ width: `${redProgress}%` }"></i>
      </div>
      <div class="robot-row">
        <HudRobotUnitIcon
          v-for="unit in redUnits"
          :key="unit.label"
          :label="unit.label"
          :hp="unit.hp"
          :alive="unit.alive"
          side="red"
        />
      </div>
    </section>

    <section class="center-zone">
      <div class="score-pill">
        <span class="left">{{ redRoundWins }}</span>
        <span class="label">比 分</span>
        <span class="right">{{ blueRoundWins }}</span>
      </div>
      <div class="meta-pill">
        <span class="meta-k">局次</span>
        <span class="meta-v">{{ roundLabel }}</span>
        <span class="meta-k">倒计时</span>
        <span class="meta-v">{{ countdown }}</span>
        <span class="meta-k">经济</span>
        <span class="meta-v">{{ economy }}</span>
      </div>
    </section>

    <section class="team-zone side-blue">
      <header class="team-head">
        <span class="team-name">蓝方机器人</span>
        <b class="team-score">{{ blueVictory }}</b>
      </header>
      <div class="victory-track">
        <i class="victory-fill" :style="{ width: `${blueProgress}%` }"></i>
      </div>
      <div class="robot-row">
        <HudRobotUnitIcon
          v-for="unit in blueUnits"
          :key="unit.label"
          :label="unit.label"
          :hp="unit.hp"
          :alive="unit.alive"
          side="blue"
        />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import HudRobotUnitIcon from '../primitives/HudRobotUnitIcon.vue'

interface UnitItem {
  label: string
  hp: number
  alive: boolean
}

interface Props {
  roundLabel: string
  countdown: string
  economy: number
  redVictory?: number
  blueVictory?: number
  redRoundWins?: number
  blueRoundWins?: number
}

const props = withDefaults(defineProps<Props>(), {
  redVictory: 200,
  blueVictory: 200,
  redRoundWins: 0,
  blueRoundWins: 0,
})

// What: 先用本地占位阵列渲染红蓝机器人图标。Why: 在后端阵容字段未接入前保持顶部结构完整。
const redUnits = computed<UnitItem[]>(() => [
  { label: 'R1', hp: 350, alive: true },
  { label: 'R3', hp: 410, alive: true },
  { label: 'R7', hp: 520, alive: true },
])

const blueUnits = computed<UnitItem[]>(() => [
  { label: 'B1', hp: 350, alive: true },
  { label: 'B3', hp: 420, alive: true },
  { label: 'B7', hp: 750, alive: true },
])

const redProgress = computed(() => Math.max(0, Math.min(100, (props.redVictory / 250) * 100)))
const blueProgress = computed(() => Math.max(0, Math.min(100, (props.blueVictory / 250) * 100)))
</script>

<style scoped>
.top-bar-widget {
  width: 100%;
  height: 100%;
  padding: 6px 8px;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  gap: 10px;
  border: 1px solid rgba(92, 129, 206, 0.52);
  background: linear-gradient(180deg, rgba(12, 20, 35, 0.9), rgba(9, 14, 26, 0.68));
  box-shadow: inset 0 0 0 1px rgba(16, 34, 70, 0.68);
  pointer-events: none;
}

.team-zone {
  display: grid;
  grid-template-rows: auto auto 1fr;
  gap: 4px;
  min-width: 0;
}

.team-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-width: 0;
}

.team-name {
  font-size: 11px;
  color: rgba(196, 216, 246, 0.92);
  letter-spacing: 0.04em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.team-score {
  font-size: 28px;
  line-height: 0.92;
  color: rgba(226, 239, 255, 0.95);
  font-family: 'JetBrains Mono', 'Consolas', monospace;
}

.side-red .team-score {
  color: #ff978f;
}

.side-blue .team-score {
  color: #8fc4ff;
}

.victory-track {
  height: 12px;
  border: 1px solid rgba(114, 156, 234, 0.45);
  background: rgba(7, 11, 21, 0.85);
  overflow: hidden;
}

.victory-fill {
  display: block;
  height: 100%;
}

.side-red .victory-fill {
  background: linear-gradient(90deg, rgba(255, 107, 107, 0.88), rgba(255, 212, 109, 0.84));
}

.side-blue .victory-fill {
  background: linear-gradient(90deg, rgba(143, 196, 255, 0.9), rgba(226, 250, 146, 0.82));
}

.robot-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  overflow: hidden;
}

.center-zone {
  min-width: 248px;
  display: grid;
  align-content: center;
  gap: 6px;
}

.score-pill {
  height: 24px;
  padding: 0 10px;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  border: 1px solid rgba(110, 156, 238, 0.54);
  background: rgba(13, 23, 41, 0.88);
  clip-path: polygon(8px 0, calc(100% - 8px) 0, 100% 50%, calc(100% - 8px) 100%, 8px 100%, 0 50%);
}

.score-pill .left,
.score-pill .right {
  font-size: 16px;
  line-height: 1;
  text-align: center;
  color: #dcecff;
  font-family: 'JetBrains Mono', 'Consolas', monospace;
}

.score-pill .label {
  font-size: 10px;
  color: rgba(166, 195, 245, 0.88);
  letter-spacing: 0.1em;
}

.meta-pill {
  min-height: 24px;
  padding: 0 10px;
  display: flex;
  align-items: center;
  gap: 6px;
  border: 1px solid rgba(102, 147, 227, 0.4);
  background: rgba(8, 14, 27, 0.82);
  overflow: hidden;
}

.meta-k {
  font-size: 10px;
  color: rgba(142, 173, 228, 0.92);
  letter-spacing: 0.05em;
}

.meta-v {
  font-size: 12px;
  color: rgba(220, 235, 255, 0.95);
  font-family: 'JetBrains Mono', 'Consolas', monospace;
}

@media (max-width: 980px) {
  .top-bar-widget {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .center-zone {
    min-width: 0;
    order: -1;
  }
}
</style>
