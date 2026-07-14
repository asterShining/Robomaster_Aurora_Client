<template>
  <HudPanel title="本车状态" :editMode="props.editMode">
    <div class="vs-body">

      <!-- 血量 -->
      <div class="vs-row">
        <div class="vs-row__icon vs-icon--hp">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor">
            <path d="M7 12.5S1 8.5 1 4.5C1 2.57 2.57 1 4.5 1c.96 0 1.84.4 2.5 1.04A3.48 3.48 0 0 1 9.5 1C11.43 1 13 2.57 13 4.5c0 4-6 8-6 8z"/>
          </svg>
        </div>
        <span class="vs-row__label">血量</span>
        <span class="vs-row__val">
          <b>{{ props.hp }}</b>
          <em> / {{ props.maxHp }}</em>
        </span>
      </div>

      <!-- 等级 -->
      <div class="vs-row">
        <div class="vs-row__icon vs-icon--level">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor">
            <polygon points="7,1 9,5.5 14,6.1 10.5,9.4 11.5,14 7,11.5 2.5,14 3.5,9.4 0,6.1 5,5.5"/>
          </svg>
        </div>
        <span class="vs-row__label">等级</span>
        <div class="vs-level-btns">
          <button
            v-for="l in [1, 2, 3]"
            :key="l"
            class="vs-level-btn"
            :class="{ 'vs-level-btn--active': props.level === l }"
            @click="$emit('setLevel', l)"
          >{{ l }}</button>
        </div>
      </div>

      <!-- 功率 -->
      <div class="vs-row">
        <div class="vs-row__icon vs-icon--power">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor">
            <path d="M8 1L3 8h4l-1 5 6-7H8L9 1z"/>
          </svg>
        </div>
        <span class="vs-row__label">功率</span>
        <span class="vs-row__val">
          <b :class="props.power > props.maxPower ? 'vs-over' : ''">{{ props.power.toFixed(1) }}W</b>
          <em> / {{ props.maxPower }}W</em>
        </span>
      </div>

      <!-- 功率子条：缓冲 + 超电 -->
      <div class="vs-sub-bars">
        <div class="vs-sub-row">
          <span class="vs-sub-label">缓冲</span>
          <div class="vs-sub-bar">
            <div class="vs-sub-bar__fill vs-sub-bar__fill--buf"
              :style="{ width: toPercent(props.buffer, props.maxBuffer) }"></div>
          </div>
          <span class="vs-sub-val">{{ props.buffer }}</span>
        </div>
        <div class="vs-sub-row">
          <span class="vs-sub-label">超电</span>
          <div class="vs-sub-bar">
            <div class="vs-sub-bar__fill vs-sub-bar__fill--cap"
              :style="{ width: toPercent(props.superCap, props.maxSuperCap) }"></div>
          </div>
          <span class="vs-sub-val">{{ props.superCap }}</span>
        </div>
      </div>

      <div class="vs-divider"></div>

      <!-- 剩余发弹量 -->
      <div class="vs-row">
        <div class="vs-row__icon vs-icon--ammo">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor">
            <rect x="4" y="1" width="6" height="10" rx="2"/>
            <rect x="5" y="11" width="4" height="2" rx="1"/>
          </svg>
        </div>
        <span class="vs-row__label">剩余发弹量</span>
        <span class="vs-row__val">
          <b>{{ props.ammo }}</b>
          <em> 发</em>
        </span>
      </div>

    </div>
  </HudPanel>
</template>

<script setup lang="ts">
import HudPanel from './HudPanel.vue'

const props = withDefaults(defineProps<{
  hp?: number
  maxHp?: number
  level?: 1 | 2 | 3
  power?: number
  maxPower?: number
  buffer?: number
  maxBuffer?: number
  superCap?: number
  maxSuperCap?: number
  ammo?: number
  editMode?: boolean
}>(), {
  hp: 0,
  maxHp: 500,
  level: 1,
  power: 56.0,
  maxPower: 55,
  buffer: 60,
  maxBuffer: 60,
  superCap: 199,
  maxSuperCap: 500,
  ammo: 500,
  editMode: false,
})

defineEmits<{ setLevel: [level: number] }>()

const toPercent = (v: number, max: number) =>
  `${Math.max(0, Math.min(100, (v / Math.max(max, 1)) * 100))}%`
</script>

<style scoped>
.vs-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 220px;
  font-family: 'Baloo 2', 'Patrick Hand', 'Comic Sans MS', sans-serif;
  color: #e8dff5;
}

.vs-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.vs-row__icon {
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.vs-icon--hp    { color: #ff4455; }
.vs-icon--level { color: #f5c542; }
.vs-icon--power { color: #c084fc; }
.vs-icon--ammo  { color: #529aff; }

.vs-row__label {
  font-size: 12px;
  font-weight: 700;
  color: rgba(232, 223, 245, 0.45);
  flex: 1;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.vs-row__val {
  font-size: 13px;
  text-align: right;
  font-weight: 700;
}

.vs-row__val b { font-weight: 800; color: #e8dff5; }
.vs-row__val em { font-style: normal; font-size: 11px; color: rgba(232, 223, 245, 0.35); }
.vs-over { color: #ff4455 !important; }

/* 等级选择器 */
.vs-level-btns {
  display: flex;
  gap: 4px;
}

.vs-level-btn {
  width: 24px;
  height: 24px;
  background: rgba(232, 223, 245, 0.07);
  border: 2px solid #2e2a48;
  border-radius: 6px;
  color: rgba(232, 223, 245, 0.3);
  font-size: 12px;
  font-weight: 800;
  font-family: 'Baloo 2', 'Patrick Hand', sans-serif;
  cursor: pointer;
  transition: all 0.12s;
  pointer-events: all;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 0 rgba(245, 197, 66, 0.3);
}
.vs-level-btn:active { transform: translateY(2px); box-shadow: none; }

.vs-level-btn--active {
  background: #f5c542;
  color: #0d0b18;
  border-color: #f5c542;
}

/* 子条：缓冲 / 超电 */
.vs-sub-bars {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding-left: 24px;
}

.vs-sub-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.vs-sub-label {
  font-size: 11px;
  font-weight: 700;
  color: rgba(232, 223, 245, 0.35);
  min-width: 28px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.vs-sub-bar {
  flex: 1;
  height: 7px;
  background: rgba(232, 223, 245, 0.07);
  border-radius: 999px;
  overflow: hidden;
  border: 2px solid #2e2a48;
}

.vs-sub-bar__fill {
  height: 100%;
  border-radius: inherit;
  transition: width 0.3s ease;
}

.vs-sub-bar__fill--buf { background: linear-gradient(90deg, #1a64d4, #529aff); }
.vs-sub-bar__fill--cap { background: linear-gradient(90deg, #7c3aed, #c084fc); }

.vs-sub-val {
  font-size: 11px;
  font-family: 'Courier New', monospace;
  font-weight: 700;
  color: rgba(232, 223, 245, 0.5);
  min-width: 28px;
  text-align: right;
}

.vs-divider {
  height: 1px;
  background: rgba(232, 223, 245, 0.1);
  border-radius: 1px;
  margin: 2px 0;
}
</style>
