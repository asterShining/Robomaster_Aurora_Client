<template>
  <div class="tsb-root">
    <!-- 左：机器人 ID 徽章 -->
    <div class="tsb-robot-ids">
      <div class="tsb-badge tsb-badge--red">{{ props.currentRobot }}</div>
      <span class="tsb-colon">:</span>
      <div class="tsb-badge tsb-badge--blue">{{ props.targetId }}</div>
    </div>

    <!-- 状态切换指示器 -->
    <div class="tsb-toggles">
      <div
        v-for="s in statuses"
        :key="s.key"
        class="tsb-toggle"
        :class="{ 'tsb-toggle--active': s.value }"
      >
        <span class="tsb-toggle__dot"></span>
        <span class="tsb-toggle__label">{{ s.label }}</span>
      </div>
    </div>

    <!-- 缓冲 bar + 超电 + 模式 -->
    <div class="tsb-cap-section">
      <div class="tsb-buf-bar">
        <div
          class="tsb-buf-bar__fill"
          :style="{ width: toPercent(props.buffer, props.maxBuffer) }"
        ></div>
      </div>
      <span class="tsb-cap-val">缓冲&nbsp;<b>{{ props.buffer }}</b></span>
      <span class="tsb-sep">◆</span>
      <span class="tsb-cap-val">超电&nbsp;<b>{{ props.superCap }}</b></span>
      <div class="tsb-mode-chip">{{ props.mode }}</div>
    </div>

    <!-- 比赛状态按钮 -->
    <div class="tsb-match-btn" :class="`tsb-match-btn--${props.matchState}`">
      <span class="tsb-match-dot"></span>
      <span>{{ matchLabel }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  currentRobot?: number | string
  targetId?: number | string
  autoAim?: boolean
  gyro?: boolean
  deploy?: boolean
  noBullet?: boolean
  buffer?: number
  maxBuffer?: number
  superCap?: number
  mode?: string
  matchState?: 'waiting' | 'running' | 'ended'
}>(), {
  currentRobot: 1,
  targetId: 0,
  autoAim: false,
  gyro: false,
  deploy: false,
  noBullet: false,
  buffer: 60,
  maxBuffer: 60,
  superCap: 0,
  mode: 'J',
  matchState: 'waiting',
})

const statuses = computed(() => [
  { key: 'autoAim', label: '自瞄',  value: props.autoAim  },
  { key: 'gyro',    label: '小陀螺', value: props.gyro    },
  { key: 'deploy',  label: '部署',  value: props.deploy   },
  { key: 'noBullet',label: '无弹',  value: props.noBullet },
])

const matchLabel = computed(() => ({
  waiting: '等待开始',
  running: '比赛进行',
  ended:   '比赛结束',
}[props.matchState]))

const toPercent = (v: number, max: number) =>
  `${Math.max(0, Math.min(100, (v / Math.max(max, 1)) * 100))}%`
</script>

<style scoped>
/* ── 暗色卡通机甲顶部状态栏 ── */
.tsb-root {
  display: flex;
  align-items: center;
  gap: 0;
  height: 48px;
  background: rgba(14, 11, 26, 0.97);
  border-bottom: 2.5px solid #2e2a48;
  box-shadow: 0 4px 0 rgba(245, 197, 66, 0.4), 0 6px 16px rgba(0, 0, 0, 0.5);
  padding: 0 16px;
  font-family: 'Baloo 2', 'Patrick Hand', 'Comic Sans MS', sans-serif;
  font-size: 13px;
  color: #e8dff5;
  justify-content: center;
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 60;
  pointer-events: auto;
}

/* 机器人 ID 徽章 */
.tsb-robot-ids {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-right: 18px;
}

.tsb-badge {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 15px;
  font-weight: 800;
  border-radius: 8px;
  border: 2px solid rgba(255, 255, 255, 0.15);
  box-shadow: 0 3px 0 rgba(245, 197, 66, 0.5);
  transform: skewX(-5deg);
}

.tsb-badge--red  { background: rgba(255, 68, 85, 0.18); color: #ff5566; border-color: rgba(255, 68, 85, 0.4); }
.tsb-badge--blue { background: rgba(82, 154, 255, 0.18); color: #529aff; border-color: rgba(82, 154, 255, 0.4); }

.tsb-colon {
  font-size: 18px;
  font-weight: 900;
  color: rgba(232, 223, 245, 0.25);
  margin: 0 2px;
}

/* 状态切换指示器 */
.tsb-toggles {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-right: 18px;
}

.tsb-toggle {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 2px 9px;
  border: 2px solid #2e2a48;
  border-radius: 999px;
  background: rgba(232, 223, 245, 0.05);
  font-size: 11px;
  font-weight: 700;
  opacity: 0.4;
  transition: opacity 0.2s, background 0.2s, border-color 0.2s;
  box-shadow: 0 2px 0 rgba(245, 197, 66, 0.25);
}

.tsb-toggle--active {
  opacity: 1;
  background: #f5c542;
  color: #0d0b18;
  border-color: #f5c542;
}

.tsb-toggle__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: rgba(232, 223, 245, 0.25);
  border: 1.5px solid rgba(232, 223, 245, 0.35);
  transition: background 0.2s;
}

.tsb-toggle--active .tsb-toggle__dot {
  background: #0d0b18;
  border-color: #0d0b18;
}

.tsb-toggle__label { letter-spacing: 0.3px; }

/* 缓冲 / 超电区 */
.tsb-cap-section {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-right: 18px;
}

.tsb-buf-bar {
  width: 72px;
  height: 10px;
  background: rgba(232, 223, 245, 0.07);
  border-radius: 999px;
  overflow: hidden;
  border: 2px solid #2e2a48;
}

.tsb-buf-bar__fill {
  height: 100%;
  background: linear-gradient(90deg, #1a64d4, #529aff);
  border-radius: inherit;
  transition: width 0.3s ease;
}

.tsb-cap-val {
  font-size: 12px;
  font-weight: 600;
  color: rgba(232, 223, 245, 0.5);
}

.tsb-cap-val b { color: #e8dff5; font-weight: 800; }

.tsb-sep {
  font-size: 8px;
  color: rgba(232, 223, 245, 0.15);
  margin: 0 1px;
}

.tsb-mode-chip {
  padding: 1px 8px;
  background: #f5c542;
  border: 2px solid #f5c542;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 800;
  color: #0d0b18;
  letter-spacing: 1px;
  box-shadow: 0 2px 0 rgba(0, 0, 0, 0.5);
}

/* 比赛状态按钮 */
.tsb-match-btn {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 5px 14px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 700;
  border: 2px solid #2e2a48;
  background: rgba(232, 223, 245, 0.06);
  color: #e8dff5;
  letter-spacing: 0.5px;
  box-shadow: 0 3px 0 rgba(245, 197, 66, 0.35);
}

.tsb-match-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  border: 1.5px solid rgba(255, 255, 255, 0.15);
}

.tsb-match-btn--waiting .tsb-match-dot { background: #529aff; }
.tsb-match-btn--running .tsb-match-dot { background: #2ae084; }
.tsb-match-btn--ended   .tsb-match-dot { background: #ff4455; }
</style>
