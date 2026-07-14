<template>
  <HudWidgetCard title="键鼠信息" accent="blue">
    <div class="icon-board">
      <div class="mouse-row">
        <span class="chip" :class="{ active: leftActive }">L</span>
        <HudIconMouse class="mouse-icon" />
        <span class="chip" :class="{ active: rightActive }">R</span>
      </div>

      <div class="key-row">
        <span class="key" :class="{ active: forwardActive }">{{ forwardLabel }}</span>
        <span class="key" :class="{ active: leftKeyActive }">{{ leftLabel }}</span>
        <span class="key" :class="{ active: backwardActive }">{{ backwardLabel }}</span>
        <span class="key" :class="{ active: rightKeyActive }">{{ rightLabel }}</span>
      </div>
    </div>
  </HudWidgetCard>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import HudWidgetCard from '../primitives/HudWidgetCard.vue'
import HudIconMouse from '../icons/HudIconMouse.vue'
import { useInputCaptureStore } from '../../../store/inputCapture'
import { useUiPanelStore } from '../../../store/uiPanel'
import {
  ACTION_MOVE_BACKWARD,
  ACTION_MOVE_FORWARD,
  ACTION_MOVE_LEFT,
  ACTION_MOVE_RIGHT,
  createActionTokenMap,
  keyTokenToDisplayLabel,
} from '../../../composables/useKeymap'

const ACTIVE_DECAY_MS = 140

const inputStore = useInputCaptureStore()
const uiPanelStore = useUiPanelStore()
const { activity, mouseButtons } = storeToRefs(inputStore)

const nowTs = ref(Date.now())
let ticker: number | null = null

const actionTokenMap = computed(() => createActionTokenMap(uiPanelStore.applied.keymap))

const forwardToken = computed(() => actionTokenMap.value[ACTION_MOVE_FORWARD] || 'KeyW')
const backwardToken = computed(() => actionTokenMap.value[ACTION_MOVE_BACKWARD] || 'KeyS')
const leftToken = computed(() => actionTokenMap.value[ACTION_MOVE_LEFT] || 'KeyA')
const rightToken = computed(() => actionTokenMap.value[ACTION_MOVE_RIGHT] || 'KeyD')

const forwardLabel = computed(() => keyTokenToDisplayLabel(forwardToken.value))
const backwardLabel = computed(() => keyTokenToDisplayLabel(backwardToken.value))
const leftLabel = computed(() => keyTokenToDisplayLabel(leftToken.value))
const rightLabel = computed(() => keyTokenToDisplayLabel(rightToken.value))

function isRecent(ts: number): boolean {
  return nowTs.value - ts <= ACTIVE_DECAY_MS
}

function isTokenActive(token: string): boolean {
  if (!token) return false
  if (inputStore.isTokenPressed(token)) return true
  return isRecent(inputStore.getTokenActiveAt(token))
}

// What: 鼠标高亮采用“按下即时亮 + 短暂衰减”策略。
// Why: 可避免轻微抖动造成频闪，同时让用户感知到最近一次触发。
const leftActive = computed(() => mouseButtons.value.left || isRecent(activity.value.leftAt))
const rightActive = computed(() => mouseButtons.value.right || isRecent(activity.value.rightAt))
const forwardActive = computed(() => isTokenActive(forwardToken.value))
const backwardActive = computed(() => isTokenActive(backwardToken.value))
const leftKeyActive = computed(() => isTokenActive(leftToken.value))
const rightKeyActive = computed(() => isTokenActive(rightToken.value))

onMounted(() => {
  ticker = window.setInterval(() => {
    nowTs.value = Date.now()
  }, 50)
})

onBeforeUnmount(() => {
  if (ticker !== null) window.clearInterval(ticker)
})
</script>

<style scoped>
.icon-board {
  height: 100%;
  display: grid;
  align-content: center;
  gap: 10px;
}

.mouse-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.mouse-icon {
  width: 18px;
  height: 18px;
  color: rgba(181, 210, 249, 0.88);
}

.chip,
.key {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(95, 136, 213, 0.46);
  color: rgba(180, 206, 246, 0.92);
  background: rgba(10, 16, 30, 0.82);
  font-family: 'JetBrains Mono', 'Consolas', monospace;
}

.chip {
  width: 24px;
  height: 24px;
  border-radius: 999px;
  font-size: 12px;
}

.key {
  min-width: 30px;
  height: 24px;
  border-radius: 10px;
  font-size: 11px;
  padding: 0 6px;
}

.key-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.chip.active,
.key.active {
  color: #e9f6ff;
  border-color: rgba(116, 201, 255, 0.95);
  box-shadow: 0 0 0 2px rgba(63, 145, 255, 0.22), 0 0 12px rgba(82, 191, 255, 0.4);
}
</style>
