<template>
  <!-- 可拖拽面板容器 -->
  <div
    class="hud-panel"
    :style="panelStyle"
    :class="{ 'hud-panel--dragging': dragging, 'hud-panel--edit': editMode }"
  >
    <!-- 面板标题栏 -->
    <div
      class="hud-panel__header"
      @mousedown="editMode ? startDrag($event) : null"
      :style="editMode ? 'cursor: grab' : ''"
    >
      <span class="hud-panel__title">{{ title }}</span>
      <div class="hud-panel__controls">
        <span class="hud-panel__dot"></span>
        <span class="hud-panel__dot"></span>
        <button class="hud-panel__toggle" @click="collapsed = !collapsed" @mousedown.stop>
          <svg :style="collapsed ? 'transform:rotate(-90deg)' : ''" width="12" height="12" viewBox="0 0 12 12" fill="currentColor">
            <path d="M6 8L1 3h10z"/>
          </svg>
        </button>
      </div>
    </div>
    <!-- 面板内容 -->
    <div v-show="!collapsed" class="hud-panel__body">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const props = withDefaults(defineProps<{
  title: string
  defaultX?: number
  defaultY?: number
  editMode?: boolean
}>(), {
  defaultX: 0,
  defaultY: 0,
  editMode: false,
})

const collapsed = ref(false)
const x = ref(props.defaultX)
const y = ref(props.defaultY)
const dragging = ref(false)
let dragStartX = 0, dragStartY = 0, panelStartX = 0, panelStartY = 0

const panelStyle = computed(() => ({
  transform: `translate(${x.value}px, ${y.value}px)`,
}))

function startDrag(e: MouseEvent) {
  dragging.value = true
  dragStartX = e.clientX
  dragStartY = e.clientY
  panelStartX = x.value
  panelStartY = y.value

  const onMove = (ev: MouseEvent) => {
    x.value = panelStartX + ev.clientX - dragStartX
    y.value = panelStartY + ev.clientY - dragStartY
  }
  const onUp = () => {
    dragging.value = false
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}
</script>

<style scoped>
/* ── 暗色卡通机甲面板容器 ── */
.hud-panel {
  background: rgba(16, 13, 28, 0.97);
  border: 2.5px solid #2e2a48;
  border-radius: 14px;
  overflow: hidden;
  box-shadow: 0 5px 0 rgba(245, 197, 66, 0.5), 0 8px 24px rgba(0, 0, 0, 0.6);
  min-width: 160px;
  font-family: 'Baloo 2', 'Patrick Hand', 'Comic Sans MS', sans-serif;
  font-size: 13px;
  color: #e8dff5;
  user-select: none;
}

/* 编辑模式：金色描边提示 */
.hud-panel--edit {
  border-color: #f5c542;
  box-shadow: 0 5px 0 rgba(245, 197, 66, 0.7), 0 8px 24px rgba(245, 197, 66, 0.15);
}

.hud-panel--dragging {
  opacity: 0.9;
  box-shadow: 0 10px 0 rgba(245, 197, 66, 0.6), 0 16px 40px rgba(0, 0, 0, 0.7);
  transform: scale(1.02) !important;
}

/* 标题栏 */
.hud-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 7px 12px 6px;
  border-bottom: 2px solid #2e2a48;
  background: rgba(255, 255, 255, 0.04);
}

.hud-panel__title {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 1.5px;
  color: rgba(232, 223, 245, 0.45);
  text-transform: uppercase;
}

.hud-panel__controls {
  display: flex;
  align-items: center;
  gap: 5px;
}

.hud-panel__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: rgba(232, 223, 245, 0.1);
  border: 1.5px solid rgba(232, 223, 245, 0.2);
}

.hud-panel__toggle {
  background: none;
  border: none;
  padding: 2px;
  color: rgba(232, 223, 245, 0.3);
  cursor: pointer;
  display: flex;
  align-items: center;
  transition: color 0.15s;
  pointer-events: all;
}
.hud-panel__toggle:hover { color: #f5c542; }
.hud-panel__toggle svg { transition: transform 0.2s ease; }

.hud-panel__body {
  padding: 10px 12px 12px;
}
</style>
