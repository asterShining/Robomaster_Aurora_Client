<template>
  <!-- TopHUD：顶部战况总览条，绝对定位于屏幕顶部正中 -->
  <div class="hud-root">
    <div class="hud-body">

      <!-- ══════════ 红方区块（左） ══════════ -->
      <div class="side-wrap side-wrap--red">
        <!-- 红方基地血条 -->
        <div class="base-section base-section--red">
          <span class="base-label">RED BASE</span>
          <div class="bar-shell bar-shell--red">
            <div
              class="bar-fill bar-fill--red"
              :style="{ width: toPercent(redBase.hp, redBase.maxHp) }"
            ></div>
            <span class="bar-hp-text">{{ redBase.hp }}<em>/{{ redBase.maxHp }}</em></span>
          </div>
        </div>
        <!-- 红方机器人存活图标 -->
        <div class="robot-row">
          <div
            v-for="(r, i) in redRobots"
            :key="i"
            class="r-chip r-chip--red"
            :class="{ 'r-chip--dead': !r.alive }"
            :title="r.name"
          >
            <span class="r-chip__icon">{{ r.alive ? r.label : '✕' }}</span>
          </div>
        </div>
      </div>

      <!-- ══════════ 倒计时（中央） ══════════ -->
      <div class="timer-wrap">
        <div class="timer-shell">
          <div class="timer-phase">{{ matchPhase }}</div>
          <div class="timer-digits">{{ formattedTime }}</div>
        </div>
      </div>

      <!-- ══════════ 蓝方区块（右） ══════════ -->
      <div class="side-wrap side-wrap--blue">
        <!-- 蓝方基地血条 -->
        <div class="base-section base-section--blue">
          <span class="base-label">BLUE BASE</span>
          <div class="bar-shell bar-shell--blue">
            <div
              class="bar-fill bar-fill--blue"
              :style="{ width: toPercent(blueBase.hp, blueBase.maxHp) }"
            ></div>
            <span class="bar-hp-text">{{ blueBase.hp }}<em>/{{ blueBase.maxHp }}</em></span>
          </div>
        </div>
        <!-- 蓝方机器人存活图标 -->
        <div class="robot-row">
          <div
            v-for="(r, i) in blueRobots"
            :key="i"
            class="r-chip r-chip--blue"
            :class="{ 'r-chip--dead': !r.alive }"
            :title="r.name"
          >
            <span class="r-chip__icon">{{ r.alive ? r.label : '✕' }}</span>
          </div>
        </div>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'

// ── Mock 数据 ──────────────────────────────────────────────
const redBase  = ref({ hp: 7200, maxHp: 10000 })
const blueBase = ref({ hp: 5400, maxHp: 10000 })

interface Robot { label: string; name: string; alive: boolean }

const redRobots = ref<Robot[]>([
  { label: 'H', name: 'Hero',      alive: true  },
  { label: 'E', name: 'Engineer',  alive: true  },
  { label: '3', name: 'Infantry3', alive: false },
  { label: '4', name: 'Infantry4', alive: true  },
  { label: '5', name: 'Infantry5', alive: true  },
])

const blueRobots = ref<Robot[]>([
  { label: 'H', name: 'Hero',      alive: true  },
  { label: 'E', name: 'Engineer',  alive: false },
  { label: '3', name: 'Infantry3', alive: true  },
  { label: '4', name: 'Infantry4', alive: false },
  { label: '5', name: 'Infantry5', alive: true  },
])

// ── 倒计时 ────────────────────────────────────────────────
const totalSeconds = ref(7 * 60 - 1) // 06:59

const matchPhase = computed(() =>
  totalSeconds.value > 3 * 60 ? 'BATTLE' : totalSeconds.value > 0 ? '⚡ OVERTIME' : '🏁 END'
)

const formattedTime = computed(() => {
  const s = Math.max(0, totalSeconds.value)
  const m = Math.floor(s / 60)
  const sec = s % 60
  return `${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`
})

let timer: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  timer = setInterval(() => {
    if (totalSeconds.value > 0) totalSeconds.value--
  }, 1000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })

// ── 工具 ──────────────────────────────────────────────────
const toPercent = (v: number, max: number) =>
  `${Math.max(0, Math.min(100, (v / Math.max(max, 1)) * 100))}%`
</script>

<style scoped>
/* ═══ 根容器：顶部居中，pointer-events 关闭 ═══ */
.hud-root {
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  z-index: 50;
  pointer-events: none;
  font-family: 'Baloo 2', 'Patrick Hand', 'Comic Sans MS', sans-serif;
  user-select: none;
}

/* ═══ 主体水平 flex 条 ═══ */
.hud-body {
  display: flex;
  align-items: stretch;
  gap: 0;
  /* 整体外轮廓：梯形切角 */
  clip-path: polygon(0 0, 100% 0, calc(100% - 14px) 100%, 14px 100%);
  background: rgba(10, 8, 6, 0.72);
  border-bottom: 3px solid #1f1a14;
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.55);
}

/* ═══ 两侧区块容器 ═══ */
.side-wrap {
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 6px 10px 5px;
  gap: 4px;
  min-width: 300px;
}

.side-wrap--red {
  /* 红方整体向右倾斜，增加动感 */
  background: linear-gradient(105deg, rgba(200, 30, 30, 0.18) 0%, rgba(0,0,0,0) 100%);
  border-right: 2px solid rgba(255, 60, 60, 0.3);
}

.side-wrap--blue {
  background: linear-gradient(75deg, rgba(0,0,0,0) 0%, rgba(30, 80, 200, 0.18) 100%);
  border-left: 2px solid rgba(60, 120, 255, 0.3);
}

/* ═══ 基地血条区 ═══ */
.base-section {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.base-label {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 2px;
  text-transform: uppercase;
  opacity: 0.8;
  color: #e8e0cf;
}

.base-section--red .base-label { color: #ff9999; }
.base-section--blue .base-label { color: #99ccff; text-align: right; }

/* 血条外壳：使用 clip-path 实现装甲斜切感 */
.bar-shell {
  position: relative;
  height: 16px;
  width: 100%;
  background: rgba(255, 255, 255, 0.08);
  border: 2px solid #1f1a14;
  overflow: hidden;
}

/* 红方血条：右侧斜切 */
.bar-shell--red {
  clip-path: polygon(0 0, 100% 0, calc(100% - 10px) 100%, 0 100%);
  border-right: none;
}

/* 蓝方血条：左侧斜切 */
.bar-shell--blue {
  clip-path: polygon(10px 0, 100% 0, 100% 100%, 0 100%);
  border-left: none;
}

/* 血条填充 */
.bar-fill {
  position: absolute;
  top: 0; left: 0; bottom: 0;
  transition: width 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.bar-fill--red {
  background: linear-gradient(90deg, #cc0000, #ff5555, #ffaaaa);
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.3);
}

.bar-fill--blue {
  background: linear-gradient(90deg, #aaccff, #5599ff, #0044cc);
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.3);
}

/* 血量数字 */
.bar-hp-text {
  position: absolute;
  right: 14px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 9.5px;
  font-weight: 700;
  color: #fff;
  text-shadow: 0 0 4px #000, 1px 1px 0 #000;
  mix-blend-mode: normal;
  z-index: 1;
  letter-spacing: 0.5px;
}
.bar-hp-text em {
  font-style: normal;
  opacity: 0.6;
  font-size: 8px;
}
.bar-shell--blue .bar-hp-text {
  right: auto;
  left: 16px;
}

/* ═══ 机器人存活图标行 ═══ */
.robot-row {
  display: flex;
  gap: 3px;
}

.side-wrap--blue .robot-row {
  justify-content: flex-end;
}

/* 单个机器人图标：菱形 / 切角矩形 */
.r-chip {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 22px;
  border: 2px solid;
  font-size: 10px;
  font-weight: 700;
  position: relative;
  /* 使用 skewX 制造机甲切割感 */
  transform: skewX(-8deg);
  clip-path: polygon(3px 0%, calc(100% - 3px) 0%, 100% 3px, 100% calc(100% - 3px), calc(100% - 3px) 100%, 3px 100%, 0% calc(100% - 3px), 0% 3px);
  transition: opacity 0.3s, filter 0.3s;
}

.r-chip__icon {
  transform: skewX(8deg); /* 反向 skew 保持文字正常 */
  display: block;
  line-height: 1;
}

/* 红方图标 */
.r-chip--red {
  background: rgba(220, 30, 30, 0.85);
  border-color: #ff3333;
  color: #ffe0e0;
  box-shadow: 0 2px 0 #660000, 0 0 6px rgba(255, 60, 60, 0.4);
  text-shadow: 0 1px 2px rgba(0,0,0,0.6);
}

/* 蓝方图标 */
.r-chip--blue {
  background: rgba(30, 80, 220, 0.85);
  border-color: #3366ff;
  color: #d0e8ff;
  box-shadow: 0 2px 0 #001a66, 0 0 6px rgba(60, 120, 255, 0.4);
  text-shadow: 0 1px 2px rgba(0,0,0,0.6);
}

/* 阵亡状态：灰化 + 变暗 */
.r-chip--dead {
  background: rgba(40, 35, 30, 0.7) !important;
  border-color: #555 !important;
  color: #666 !important;
  box-shadow: none !important;
  filter: grayscale(1) brightness(0.55);
  text-shadow: none !important;
}

/* ═══ 中央倒计时 ═══ */
.timer-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4px 6px;
  flex-shrink: 0;
}

.timer-shell {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: rgba(248, 241, 222, 0.07);
  border: 3px solid #1f1a14;
  border-top: none;
  padding: 2px 20px 6px;
  position: relative;
  /* 倒梯形：下宽上窄 */
  clip-path: polygon(8px 0%, calc(100% - 8px) 0%, 100% 100%, 0% 100%);
  min-width: 110px;
}

/* 赛段标签 */
.timer-phase {
  font-size: 8.5px;
  font-weight: 700;
  letter-spacing: 2px;
  text-transform: uppercase;
  color: #f5c542;
  text-shadow: 0 0 6px rgba(245, 197, 66, 0.7);
  margin-bottom: 0px;
}

/* 大数字倒计时 */
.timer-digits {
  font-size: 34px;
  font-weight: 700;
  line-height: 1;
  color: #fff8ec;
  letter-spacing: 3px;
  text-shadow:
    0 0 10px rgba(255, 200, 80, 0.5),
    2px 3px 0 #3a2800,
    -1px -1px 0 #1f1a14;
  /* 粗描边效果 */
  -webkit-text-stroke: 1.5px #1f1a14;
}

/* 当进入overtime，数字变红 */
.timer-wrap:has(.timer-phase:not(:empty)) .timer-digits {
  /* 保持白色，overtime 状态由 phase 文字区分 */
}
</style>
