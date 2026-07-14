<template>
  <aside v-if="showToggleArea" class="capture-toggle">
    <button
      v-if="inputStore.showCaptureButton && inputStore.pointerSupported"
      class="capture-btn"
      :class="{ active: inputStore.pointerLocked }"
      type="button"
      @click="onToggleCapture"
    >
      <span class="dot" :class="{ active: inputStore.pointerLocked }"></span>
      <span class="label">{{ inputStore.pointerLocked ? '退出' : '捕抓' }}</span>
      <span class="hint">Q</span>
    </button>
    <small v-if="displayMessage" :class="`tone-${displayTone}`">{{ displayMessage }}</small>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useInputCaptureStore } from '../../../store/inputCapture'

const inputStore = useInputCaptureStore()

// What: 统一挑选右侧轻提示要显示的文案。
// Why: 买弹反馈与键鼠链路错误共用同一块轻提示区，避免 HUD 右侧再长出第二个提示组件。
const displayMessage = computed(() => inputStore.quickActionFeedback || inputStore.backendError)

// What: 根据当前提示来源切换文本色调。
// Why: 成功、拦截和失败需要可扫视区分，但提示区域仍应保持轻量不抢视线。
const displayTone = computed(() => {
  if (inputStore.quickActionFeedback) return inputStore.quickActionFeedbackTone
  return 'danger'
})

// What: 在隐藏捕抓按钮时仍允许临时反馈冒泡显示。
// Why: 用户可能关闭常驻入口，但一键买弹触发后依旧需要看到最小确认反馈。
const showToggleArea = computed(() => {
  return Boolean(displayMessage.value) || (inputStore.showCaptureButton && inputStore.pointerSupported)
})

async function onToggleCapture(): Promise<void> {
  // What: 通过单按钮切换 pointer lock。
  // Why: 保留无遮罩 HUD 的同时提供清晰可控的输入接管入口。
  await inputStore.togglePointerLock()
}
</script>

<style scoped>
.capture-toggle {
  position: absolute;
  right: 0;
  top: 52%;
  transform: translateY(-50%);
  z-index: var(--z-menu);
  display: grid;
  justify-items: end;
  gap: 6px;
  pointer-events: auto;
}

.capture-btn {
  width: 42px;
  min-height: 118px;
  border-radius: 18px 0 0 18px;
  border: 1px solid var(--hud-border-strong);
  border-right: none;
  background: linear-gradient(160deg, var(--hud-surface-1), var(--hud-surface-0));
  color: var(--hud-text-primary);
  padding: 8px 6px;
  display: inline-grid;
  align-items: center;
  justify-items: center;
  gap: 6px;
  font-size: 12px;
  transition: border-color 120ms ease, box-shadow 120ms ease, transform 100ms ease;
}

.capture-btn .dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: var(--hud-text-secondary);
}

.capture-btn.active .dot {
  background: var(--hud-success);
  box-shadow: 0 0 8px rgba(92, 247, 177, 0.42);
}

.capture-btn.active {
  border-color: var(--hud-success);
  box-shadow: 0 0 0 2px rgba(57, 197, 141, 0.16);
}

.label {
  writing-mode: vertical-rl;
  text-orientation: mixed;
  letter-spacing: 0.08em;
  font-size: 11px;
}

.hint {
  font-size: 10px;
  color: var(--hud-text-secondary);
  border: 1px solid var(--hud-border-soft);
  border-radius: 999px;
  min-width: 20px;
  height: 20px;
  display: grid;
  place-items: center;
}

.capture-btn:hover,
.capture-btn:focus-visible {
  border-color: var(--hud-border-focus);
  box-shadow: 0 0 0 2px var(--hud-focus-ring);
  outline: none;
}

.capture-btn:active {
  transform: scale(0.96);
}

.capture-toggle small {
  margin-right: 4px;
  max-width: 160px;
  text-align: right;
  font-size: 10px;
  color: var(--hud-text-secondary);
  text-shadow: 0 0 4px rgba(122, 171, 255, 0.2);
}

.capture-toggle small.tone-success {
  color: var(--hud-success);
  text-shadow: 0 0 4px rgba(92, 247, 177, 0.28);
}

.capture-toggle small.tone-info {
  color: var(--hud-warning);
  text-shadow: 0 0 4px rgba(255, 191, 120, 0.26);
}

.capture-toggle small.tone-danger {
  color: var(--hud-danger-text);
  text-shadow: 0 0 4px rgba(255, 76, 76, 0.36);
}

@media (max-width: 960px) {
  .capture-btn {
    min-height: 102px;
  }
}
</style>
