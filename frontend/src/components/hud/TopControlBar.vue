<template>
  <header class="top-control-bar hud-realtime" :style="barStyle">
    <div class="mode-group">
      <button class="mode-chip is-red" type="button">1</button>
      <button class="mode-chip is-blue" type="button">2</button>
    </div>

    <div class="status-group">
      <span class="status-item">自瞄</span>
      <span class="status-item">小陀螺</span>
      <span class="status-item">部署</span>
      <span class="status-item">无弹</span>
    </div>

    <div class="energy-track" aria-hidden="true">
      <i class="energy-fill"></i>
    </div>

    <div class="meta-group">
      <span class="round">{{ roundLabel }}</span>
      <span class="timer">{{ countdown }}</span>
      <button class="tool-btn" type="button" @click="onOpenPanel">
        <span class="gear">⚙</span>
      </button>
      <button class="start-btn" type="button" @click="onOpenPanel">等待开始</button>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useUiPanelStore } from '../../store/uiPanel'

interface Props {
  roundLabel?: string
  countdown?: string
  scale?: number
  opacity?: number
}

const props = withDefaults(defineProps<Props>(), {
  roundLabel: '第1局',
  countdown: '06:59',
  scale: 1,
  opacity: 0.95,
})

const uiPanelStore = useUiPanelStore()

const barStyle = computed<Record<string, string>>(() => ({
  transform: `translateX(-50%) scale(${props.scale}) translateZ(0)`,
  opacity: String(props.opacity),
}))

// What: 顶部条齿轮与启动按钮都打开设置面板。Why: 统一操作入口，降低用户找入口成本。
function onOpenPanel(): void {
  uiPanelStore.openPanel('mqtt')
}
</script>

<style scoped>
.top-control-bar {
  position: absolute;
  left: 50%;
  top: 14px;
  width: min(1040px, calc(100% - 210px));
  min-height: 46px;
  padding: 8px 12px;
  border-radius: 12px;
  display: grid;
  grid-template-columns: auto auto 1fr auto;
  align-items: center;
  gap: 10px;
  pointer-events: auto;
  z-index: var(--z-data);
  /* What: 顶部条统一复用 HUD 语义色板。
     Why: 保持主界面控制条与其余 HUD 面板在同一视觉体系中，不再局部漂色。 */
  background: var(--hud-topbar-bg);
  border: 1px solid var(--panel-border);
  box-shadow: var(--hud-topbar-shadow);
  backdrop-filter: blur(6px);
}

.mode-group {
  display: flex;
  gap: 8px;
}

.mode-chip {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  border: 1px solid transparent;
  font-size: 12px;
  color: var(--text-strong);
  background: var(--panel-surface);
  transition: border-color 120ms ease, box-shadow 120ms ease, transform 100ms ease;
}

.mode-chip:hover,
.mode-chip:focus-visible {
  border-color: var(--panel-border-strong);
  box-shadow: 0 0 0 2px var(--accent-soft);
  outline: none;
}

.mode-chip:active {
  transform: scale(0.94);
}

.mode-chip.is-red {
  color: var(--hud-chip-red-text);
  border-color: var(--hud-chip-red-border);
}

.mode-chip.is-blue {
  color: var(--hud-chip-blue-text);
  border-color: var(--hud-chip-blue-border);
}

.status-group {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--text-muted);
  font-size: 12px;
}

.status-item {
  opacity: 0.85;
  white-space: nowrap;
}

.energy-track {
  height: 10px;
  border-radius: 999px;
  background: var(--hud-topbar-energy-track);
  overflow: hidden;
  box-shadow: var(--hud-topbar-energy-shadow);
}

.energy-fill {
  display: block;
  height: 100%;
  width: 56%;
  background: var(--hud-topbar-energy-fill);
}

.meta-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.round,
.timer {
  font-size: 12px;
  color: var(--hud-topbar-meta-text);
  font-family: var(--font-data);
}

.tool-btn,
.start-btn {
  border-radius: 9px;
  border: 1px solid var(--panel-border-strong);
  background: var(--hud-topbar-tool-bg);
  color: var(--text-strong);
  transition: border-color 120ms ease, box-shadow 120ms ease, transform 100ms ease;
}

.tool-btn {
  width: 30px;
  height: 30px;
}

.start-btn {
  height: 30px;
  padding: 0 14px;
  font-size: 12px;
  font-weight: 600;
}

.tool-btn:hover,
.tool-btn:focus-visible,
.start-btn:hover,
.start-btn:focus-visible {
  border-color: var(--panel-border-strong);
  box-shadow: 0 0 0 2px var(--accent-soft);
  outline: none;
}

.tool-btn:active,
.start-btn:active {
  transform: scale(0.95);
}

@media (max-width: 1200px) {
  .top-control-bar {
    width: calc(100% - 28px);
    grid-template-columns: auto 1fr auto;
  }

  .status-group {
    display: none;
  }
}
</style>
