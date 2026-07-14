<template>
  <aside class="edit-toolbar" :class="{ active: hudLayoutStore.isEditMode }">
    <button
      class="edit-fab"
      :class="{ active: hudLayoutStore.isEditMode }"
      type="button"
      aria-label="编辑HUD组件"
      title="编辑HUD组件"
      @click="onToggleEditMode"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M4 17.2V20h2.8L18.9 7.9l-2.8-2.8z" fill="currentColor" />
        <path d="M15.9 3.2 18.7 6l1.2-1.2a1.9 1.9 0 0 0 0-2.8l-.3-.3a1.9 1.9 0 0 0-2.8 0z" fill="currentColor" />
      </svg>
    </button>
    <button
      v-if="hudLayoutStore.isEditMode"
      class="reset-btn"
      type="button"
      @click="onResetLayout"
    >
      重置布局
    </button>
    <small v-if="hudLayoutStore.isEditMode">拖拽移动，滚轮缩放</small>
  </aside>
</template>

<script setup lang="ts">
import { useHudLayoutStore } from '../../../store/hudLayout'

const hudLayoutStore = useHudLayoutStore()

function onToggleEditMode(): void {
  // What: 统一通过工具条切换布局编辑态。
  // Why: 让按钮点击与键盘入口共享同一状态语义，避免局部交互分叉。
  hudLayoutStore.toggleEditMode()
}

function onResetLayout(): void {
  // What: 编辑态支持一键恢复默认布局。Why: 防止误拖误缩放后难以手动回正。
  hudLayoutStore.resetLayout()
}
</script>

<style scoped>
.edit-toolbar {
  position: absolute;
  left: 12px;
  top: 12px;
  z-index: var(--z-menu);
  display: grid;
  justify-items: start;
  gap: 8px;
  pointer-events: auto;
}

.edit-fab {
  width: 42px;
  height: 42px;
  border-radius: 999px;
  border: 1px solid var(--hud-border-strong);
  background: linear-gradient(160deg, var(--hud-surface-1), var(--hud-surface-0));
  color: var(--hud-text-primary);
  box-shadow: 0 10px 18px rgba(8, 18, 42, 0.28), 0 0 0 1px rgba(111, 149, 214, 0.12) inset;
  display: grid;
  place-items: center;
  transition: border-color 120ms ease, box-shadow 120ms ease, transform 100ms ease;
}

.edit-fab svg {
  width: 18px;
  height: 18px;
}

.edit-fab.active {
  border-color: var(--hud-border-focus);
  box-shadow: 0 0 0 2px var(--hud-focus-ring), 0 10px 20px rgba(6, 25, 60, 0.34);
}

.edit-fab:hover,
.edit-fab:focus-visible {
  border-color: var(--hud-border-focus);
  box-shadow: 0 0 0 2px var(--hud-focus-ring), 0 10px 20px rgba(6, 25, 60, 0.34);
  outline: none;
}

.edit-fab:active {
  transform: scale(0.96);
}

.reset-btn {
  height: 30px;
  padding: 0 12px;
  border-radius: 999px;
  border: 1px solid var(--hud-border-soft);
  background: var(--hud-surface-1);
  color: var(--hud-text-primary);
  font-size: 12px;
}

.reset-btn:hover,
.reset-btn:focus-visible {
  border-color: var(--hud-border-focus);
  box-shadow: 0 0 0 2px var(--hud-focus-ring);
  outline: none;
}

.edit-toolbar small {
  font-size: 10px;
  color: var(--hud-text-secondary);
}
</style>
