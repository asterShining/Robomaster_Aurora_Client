<template>
  <section
    class="draggable-item"
    :class="{ 'is-editing': editMode, 'is-dragging': dragging, 'is-locked': editMode && layout.locked }"
    :style="itemStyle"
    :data-widget-id="id"
    @pointerdown="onPointerDown"
    @wheel.prevent="onWheel"
  >
    <span v-if="editMode" class="drag-tag">
      {{ label }} {{ scaleLabel }}{{ layout.locked ? ' · 已锁定' : '' }}
    </span>
    <slot />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import type { HudWidgetId, HudWidgetRect } from '../../../types/hudLayout'
import { HUD_WIDGET_SCALE_STEP } from '../../../types/hudLayout'
import { clampHudPercent, hudPxToPercent, resolveHudCanvasMetrics, snapHudPixel } from './hudCanvasGeometry'

interface Props {
  id: HudWidgetId
  label: string
  layout: HudWidgetRect
  editMode: boolean
  canvasEl: HTMLElement | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  update: [payload: { id: HudWidgetId; patch: Partial<HudWidgetRect> }]
  scale: [payload: { id: HudWidgetId; delta: number }]
}>()

const dragging = ref(false)
const previewPos = ref<{ x: number; y: number } | null>(null)

const dragState = {
  startClientX: 0,
  startClientY: 0,
  startLeftPx: 0,
  startTopPx: 0,
  widthPx: 0,
  heightPx: 0,
  canvasWidthPx: 0,
  canvasHeightPx: 0,
}

const scaleLabel = computed(() => `${Math.round(props.layout.scale * 100)}%`)

const itemStyle = computed<Record<string, string>>(() => {
  const x = previewPos.value?.x ?? props.layout.x
  const y = previewPos.value?.y ?? props.layout.y

  return {
    left: `${x}%`,
    top: `${y}%`,
    width: `${props.layout.w}%`,
    height: `${props.layout.h}%`,
    transform: `scale(${props.layout.scale}) translateZ(0)`,
    transformOrigin: 'top left',
    zIndex: String(dragging.value ? 999 : props.layout.z),
    pointerEvents: props.editMode ? 'auto' : 'none',
  }
})

function onPointerDown(event: PointerEvent): void {
  if (!props.editMode) return
  // What: 锁定组件直接禁用拖拽入口。
  // Why: 避免比赛中误触拖动关键组件导致排版错位。
  if (props.layout.locked) return
  const canvas = props.canvasEl
  if (!canvas) return

  // What: 阻断默认选区与冒泡。Why: 编辑态拖拽时不能影响底层图传控制区域。
  event.preventDefault()
  event.stopPropagation()

  const metrics = resolveHudCanvasMetrics(canvas)
  dragState.canvasWidthPx = metrics.widthPx
  dragState.canvasHeightPx = metrics.heightPx
  dragState.widthPx = ((props.layout.w * props.layout.scale) / 100) * dragState.canvasWidthPx
  dragState.heightPx = ((props.layout.h * props.layout.scale) / 100) * dragState.canvasHeightPx
  dragState.startLeftPx = (props.layout.x / 100) * dragState.canvasWidthPx
  dragState.startTopPx = (props.layout.y / 100) * dragState.canvasHeightPx
  dragState.startClientX = event.clientX
  dragState.startClientY = event.clientY

  dragging.value = true
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerUp)
}

function onPointerMove(event: PointerEvent): void {
  if (!dragging.value) return

  const dx = event.clientX - dragState.startClientX
  const dy = event.clientY - dragState.startClientY
  const maxX = dragState.canvasWidthPx - dragState.widthPx
  const maxY = dragState.canvasHeightPx - dragState.heightPx

  const nextLeftPx = snapHudPixel(Math.max(0, Math.min(dragState.startLeftPx + dx, Math.max(0, maxX))))
  const nextTopPx = snapHudPixel(Math.max(0, Math.min(dragState.startTopPx + dy, Math.max(0, maxY))))

  previewPos.value = {
    x: clampHudPercent(hudPxToPercent(nextLeftPx, dragState.canvasWidthPx), 0, 100 - props.layout.w * props.layout.scale),
    y: clampHudPercent(hudPxToPercent(nextTopPx, dragState.canvasHeightPx), 0, 100 - props.layout.h * props.layout.scale),
  }
}

function onPointerUp(): void {
  if (!dragging.value) return

  const finalPos = previewPos.value
  if (finalPos) {
    emit('update', {
      id: props.id,
      patch: { x: finalPos.x, y: finalPos.y },
    })
  }

  dragging.value = false
  previewPos.value = null
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerUp)
}

function onWheel(event: WheelEvent): void {
  if (!props.editMode) return
  // What: 锁定组件时禁用滚轮缩放。
  // Why: 与拖拽锁定语义保持一致，防止“锁了还能缩”的状态割裂。
  if (props.layout.locked) return

  // What: 编辑态滚轮缩放。Why: 满足高频微调组件大小的需求，避免额外弹窗设置带来的操作阻塞。
  event.preventDefault()
  const delta = event.deltaY < 0 ? HUD_WIDGET_SCALE_STEP : -HUD_WIDGET_SCALE_STEP
  emit('scale', { id: props.id, delta })
}

onBeforeUnmount(() => {
  // What: 销毁时兜底移除监听。Why: 防止热更新后残留监听导致拖拽串扰。
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerUp)
})
</script>

<style scoped>
.draggable-item {
  position: absolute;
  min-width: 0;
  min-height: 0;
  transition: transform 120ms ease;
}

.draggable-item.is-editing {
  /* What: 编辑与拖拽反馈统一走语义 token。
     Why: 布局编辑高频出现，颜色必须和主 HUD 编辑态保持同一套规则。 */
  outline: 1px dashed var(--hud-drag-outline);
  outline-offset: 2px;
  border-radius: 14px;
  cursor: move;
  transition: box-shadow 120ms ease;
}

.draggable-item.is-editing:hover {
  box-shadow: 0 0 0 2px var(--hud-drag-hover);
}

.draggable-item.is-editing.is-locked {
  outline-color: var(--hud-drag-locked-outline);
  cursor: not-allowed;
}

.draggable-item.is-editing.is-locked:hover {
  box-shadow: 0 0 0 2px var(--hud-drag-locked-hover);
}

.draggable-item.is-dragging {
  transition: none;
  box-shadow: 0 0 0 2px var(--hud-drag-active-ring), 0 10px 20px var(--hud-drag-active-shadow);
}

.drag-tag {
  position: absolute;
  left: 6px;
  top: -24px;
  height: 20px;
  padding: 0 8px;
  display: inline-flex;
  align-items: center;
  font-size: 10px;
  color: var(--hud-drag-tag-text);
  border: 1px solid var(--hud-drag-tag-border);
  border-radius: 999px;
  background: var(--hud-drag-tag-bg);
  white-space: nowrap;
  pointer-events: none;
}
</style>
