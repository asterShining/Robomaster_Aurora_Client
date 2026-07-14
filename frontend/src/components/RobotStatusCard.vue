<template>
  <div class="rsc-root">

    <!-- ─── 左侧：机器人头像 ─── -->
    <div class="rsc-avatar-wrap">
      <!-- 橙色状态箭头 -->
      <div class="rsc-chevron">
        <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
          <circle cx="9" cy="9" r="8.5" stroke="#f59e0b" stroke-width="1.5" fill="rgba(245,158,11,0.15)"/>
          <path d="M5 7l4 4 4-4" stroke="#f59e0b" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </div>

      <!-- 机器人图像圆圈 -->
      <div class="rsc-avatar">
        <!-- 占位 SVG，可替换为 <img :src="props.avatarSrc" /> -->
        <svg viewBox="0 0 80 80" width="80" height="80" fill="none" xmlns="http://www.w3.org/2000/svg">
          <!-- 机甲底盘 -->
          <rect x="20" y="50" width="40" height="14" rx="3" fill="#444"/>
          <rect x="14" y="52" width="8"  height="10" rx="2" fill="#333"/>
          <rect x="58" y="52" width="8"  height="10" rx="2" fill="#333"/>
          <!-- 履带 -->
          <ellipse cx="18" cy="62" rx="5" ry="4" fill="#222"/>
          <ellipse cx="62" cy="62" rx="5" ry="4" fill="#222"/>
          <!-- 主体 -->
          <rect x="24" y="28" width="32" height="24" rx="4" fill="#555"/>
          <rect x="30" y="22" width="20" height="10" rx="3" fill="#666"/>
          <!-- 炮管 -->
          <rect x="38" y="10" width="4" height="18" rx="2" fill="#777"/>
          <!-- 装甲板高光 -->
          <rect x="26" y="30" width="14" height="8" rx="2" fill="#606060"/>
          <rect x="42" y="30" width="12" height="8" rx="2" fill="#585858"/>
          <!-- 红色装饰条 -->
          <rect x="24" y="46" width="32" height="3" fill="#cc2222"/>
          <!-- 传感器 -->
          <circle cx="40" cy="20" r="3" fill="#e0e0e0"/>
          <circle cx="40" cy="20" r="1.5" fill="#00c8ff"/>
        </svg>
      </div>

      <!-- 等级徽章 -->
      <div class="rsc-level-badge">{{ props.level }}</div>
    </div>

    <!-- ─── 右侧：数据面板 ─── -->
    <div class="rsc-data">

      <!-- HP 大字横幅 -->
      <div class="rsc-hp-banner">
        <span class="rsc-hp-val">
          <b>{{ props.hp }}</b>
          <em> / {{ props.maxHp }}</em>
        </span>
      </div>

      <!-- 功率条 -->
      <div class="rsc-bar-row">
        <span class="rsc-bar-label">功率</span>
        <span class="rsc-slash">/</span>
        <div class="rsc-bar-track">
          <div
            class="rsc-bar-fill rsc-bar-fill--power"
            :style="{ width: toPercent(props.power, props.maxPower) }"
          ></div>
        </div>
        <span class="rsc-bar-val">{{ props.power.toFixed(0) }} W</span>
      </div>

      <!-- 缓冲 / 底盘 双条 -->
      <div class="rsc-dual-row">
        <span class="rsc-bar-label">缓冲</span>
        <span class="rsc-slash">/</span>
        <div class="rsc-bar-track rsc-bar-track--half">
          <div
            class="rsc-bar-fill rsc-bar-fill--buffer"
            :style="{ width: toPercent(props.buffer, props.maxBuffer) }"
          ></div>
        </div>
        <span class="rsc-bar-val">{{ props.buffer }}J</span>

        <span class="rsc-dual-sep"></span>

        <span class="rsc-bar-label">底盘</span>
        <span class="rsc-slash">/</span>
        <div class="rsc-bar-track rsc-bar-track--half">
          <div
            class="rsc-bar-fill rsc-bar-fill--chassis"
            :style="{ width: toPercent(props.chassis, props.maxChassis) }"
          ></div>
        </div>
        <span class="rsc-bar-val">{{ props.chassis }}J</span>
      </div>

      <!-- 模块状态列表 -->
      <div class="rsc-modules">
        <span
          v-for="mod in moduleList"
          :key="mod.key"
          class="rsc-mod-item"
          :class="{ 'rsc-mod-item--off': !mod.online }"
        >
          <span class="rsc-mod-slash">/</span>
          {{ mod.label }}
        </span>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface ModuleState {
  主控:    boolean
  电源管理: boolean
  图传:    boolean
  RFID:   boolean
  UWB:    boolean
  装甲板:  boolean
  小弹丸:  boolean
  大弹丸:  boolean
  超级电容: boolean
  灯条:    boolean
}

const props = withDefaults(defineProps<{
  hp?:         number
  maxHp?:      number
  level?:      number
  power?:      number
  maxPower?:   number
  buffer?:     number
  maxBuffer?:  number
  chassis?:    number
  maxChassis?: number
  modules?:    Partial<ModuleState>
  avatarSrc?:  string
}>(), {
  hp:         0,
  maxHp:      600,
  level:      1,
  power:      0,
  maxPower:   80,
  buffer:     0,
  maxBuffer:  60,
  chassis:    0,
  maxChassis: 60,
  modules:    () => ({}),
})

const MODULE_LABELS: (keyof ModuleState)[] = [
  '主控', '电源管理', '图传', 'RFID', 'UWB', '装甲板', '小弹丸', '大弹丸', '超级电容', '灯条',
]

const moduleList = computed(() =>
  MODULE_LABELS.map(key => ({
    key,
    label: key,
    online: props.modules?.[key] ?? false,
  }))
)

const toPercent = (v: number, max: number) =>
  `${Math.max(0, Math.min(100, (v / Math.max(max, 1)) * 100))}%`
</script>

<style scoped>
/* ── 暗色卡通机甲整体横向 flex ── */
.rsc-root {
  display: flex;
  align-items: stretch;
  gap: 0;
  font-family: 'Baloo 2', 'Patrick Hand', 'Comic Sans MS', sans-serif;
  color: #e8dff5;
}

/* ── 左侧头像区 ── */
.rsc-avatar-wrap {
  position: relative;
  width: 90px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.rsc-avatar {
  width: 82px;
  height: 82px;
  border-radius: 50%;
  overflow: hidden;
  border: 2.5px solid #2e2a48;
  box-shadow: 0 4px 0 rgba(245, 197, 66, 0.5);
  background: radial-gradient(circle at 40% 38%, #1e1a38, #130f28);
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 橙色状态箭头（左上角） */
.rsc-chevron {
  position: absolute;
  top: 0;
  left: 2px;
  z-index: 2;
}

/* 等级数字徽章（底部居中） */
.rsc-level-badge {
  position: absolute;
  bottom: -2px;
  left: 50%;
  transform: translateX(-50%);
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: #f5c542;
  color: #0d0b18;
  font-size: 13px;
  font-weight: 800;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px solid rgba(245, 197, 66, 0.6);
  box-shadow: 0 2px 0 rgba(245, 197, 66, 0.5);
}

/* ── 右侧数据面板 ── */
.rsc-data {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding: 6px 14px 8px;
  background: rgba(16, 13, 28, 0.97);
  border-left: 2.5px solid #2e2a48;
}

/* HP 大字横幅 */
.rsc-hp-banner {
  background: #f5c542;
  padding: 5px 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  clip-path: polygon(6px 0%, calc(100% - 6px) 0%, 100% 100%, 0% 100%);
  margin-bottom: 2px;
}

.rsc-hp-val {
  font-size: 22px;
  font-weight: 800;
  letter-spacing: 3px;
  font-family: 'Courier New', 'Consolas', monospace;
}

.rsc-hp-val b { color: #0d0b18; }
.rsc-hp-val em { font-style: normal; color: rgba(13, 11, 24, 0.5); font-size: 18px; }

/* ── 通用条行 ── */
.rsc-bar-row,
.rsc-dual-row {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 18px;
}

.rsc-bar-label {
  font-size: 11px;
  font-weight: 700;
  color: rgba(232, 223, 245, 0.45);
  min-width: 28px;
  letter-spacing: 0.3px;
  text-transform: uppercase;
}

/* 鲜红斜线分隔符 */
.rsc-slash {
  font-size: 15px;
  font-weight: 900;
  color: #ff4455;
  line-height: 1;
  flex-shrink: 0;
}

.rsc-bar-track {
  flex: 1;
  height: 7px;
  background: rgba(232, 223, 245, 0.07);
  border-radius: 999px;
  overflow: hidden;
  position: relative;
  border: 2px solid #2e2a48;
}

.rsc-bar-track--half { flex: 1; }

.rsc-bar-fill {
  height: 100%;
  border-radius: inherit;
  transition: width 0.4s ease;
}

.rsc-bar-fill--power   { background: linear-gradient(90deg, #7c3aed, #c084fc); }
.rsc-bar-fill--buffer  { background: linear-gradient(90deg, #1a64d4, #529aff); }
.rsc-bar-fill--chassis { background: linear-gradient(90deg, #0d8a5e, #2ae084); }

.rsc-bar-val {
  font-size: 11px;
  font-family: 'Courier New', monospace;
  font-weight: 700;
  color: rgba(232, 223, 245, 0.6);
  min-width: 36px;
  text-align: right;
}

/* 双条中间分隔 */
.rsc-dual-sep { width: 10px; flex-shrink: 0; }

/* ── 模块状态列表 ── */
.rsc-modules {
  display: flex;
  flex-wrap: wrap;
  gap: 2px 0;
  margin-top: 2px;
  align-items: center;
}

.rsc-mod-item {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  font-size: 12px;
  font-weight: 700;
  color: #e8dff5;
  margin-right: 10px;
  white-space: nowrap;
  letter-spacing: 0.2px;
}

/* 离线模块灰化 */
.rsc-mod-item--off {
  color: rgba(232, 223, 245, 0.25) !important;
}
.rsc-mod-item--off .rsc-mod-slash {
  color: rgba(255, 68, 85, 0.2) !important;
}

.rsc-mod-slash {
  font-weight: 900;
  font-size: 14px;
  color: #ff4455;
  line-height: 1;
}
</style>
