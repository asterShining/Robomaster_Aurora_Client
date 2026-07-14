<template>
  <section class="shape-layer" :class="{ editing: editMode }">
    <svg
      ref="svgRef"
      class="shape-svg"
      :class="{ drawing: editMode && isDrawingTool }"
      viewBox="0 0 100 100"
      preserveAspectRatio="none"
      @pointerdown="onSvgPointerDown"
      @wheel="onSvgWheel"
    >
      <g
        v-for="shape in orderedShapes"
        :key="shape.id"
        class="shape-node"
        :class="{
          selected: shape.id === selectedShapeId,
          locked: shape.locked,
          hidden: !shape.visible,
          line: shape.kind === 'line',
          rect: shape.kind === 'rect',
          circle: shape.kind === 'circle',
          image: shape.kind === 'image',
        }"
        :transform="shapeTransform(shape)"
      >
        <template v-if="resolveShapeWithDraft(shape).kind === 'line'">
          <line
            class="line-hit-area"
            x1="0"
            y1="0"
            :x2="resolveShapeWithDraft(shape).w"
            y2="0"
            :stroke-width="resolveLineHitWidth(resolveShapeWithDraft(shape))"
            @pointerdown="onShapePointerDown($event, shape.id)"
          />
          <line
            class="line-stroke-soft"
            x1="0"
            y1="0"
            :x2="resolveShapeWithDraft(shape).w"
            y2="0"
            :stroke="resolvedStrokeColor(shape)"
            :stroke-width="resolvedSoftStrokeWidth(shape)"
            :stroke-opacity="resolvedSoftStrokeOpacity(shape)"
            stroke-linecap="round"
          />
          <line
            class="line-stroke"
            x1="0"
            y1="0"
            :x2="resolveShapeWithDraft(shape).w"
            y2="0"
            :stroke="resolvedStrokeColor(shape)"
            :stroke-width="resolvedStrokeWidth(shape)"
            :stroke-opacity="resolvedStrokeOpacity(shape)"
            stroke-linecap="round"
            @pointerdown="onShapePointerDown($event, shape.id)"
          />
          <template v-if="shouldShowHandles(shape)">
            <circle
              class="control-hit"
              cx="0"
              cy="0"
              :r="resolveControlHitRadius(shape)"
              @pointerdown.stop="onLineHandlePointerDown($event, shape.id, 'start')"
            />
            <circle
              class="control-hit"
              :cx="resolveShapeWithDraft(shape).w"
              cy="0"
              :r="resolveControlHitRadius(shape)"
              @pointerdown.stop="onLineHandlePointerDown($event, shape.id, 'end')"
            />
            <circle class="line-handle" cx="0" cy="0" :r="resolveVisibleHandleRadius(shape)" />
            <circle
              class="line-handle"
              :cx="resolveShapeWithDraft(shape).w"
              cy="0"
              :r="resolveVisibleHandleRadius(shape)"
            />
          </template>
        </template>

        <template v-else>
          <template v-if="resolveShapeWithDraft(shape).kind === 'rect'">
            <rect
              class="shape-hit-area"
              x="0"
              y="0"
              :width="resolveShapeWithDraft(shape).w"
              :height="resolveShapeWithDraft(shape).h"
              @pointerdown="onShapePointerDown($event, shape.id)"
            />
            <rect
              class="shape-stroke-soft"
              x="0"
              y="0"
              :width="resolveShapeWithDraft(shape).w"
              :height="resolveShapeWithDraft(shape).h"
              :stroke="resolvedStrokeColor(shape)"
              :stroke-width="resolvedSoftStrokeWidth(shape)"
              :stroke-opacity="resolvedSoftStrokeOpacity(shape)"
              fill="none"
              rx="0.6"
              ry="0.6"
              stroke-linejoin="round"
            />
            <rect
              x="0"
              y="0"
              :width="resolveShapeWithDraft(shape).w"
              :height="resolveShapeWithDraft(shape).h"
              :stroke="resolvedStrokeColor(shape)"
              :stroke-width="resolvedStrokeWidth(shape)"
              :stroke-opacity="resolvedStrokeOpacity(shape)"
              fill="none"
              rx="0.6"
              ry="0.6"
              stroke-linejoin="round"
              @pointerdown="onShapePointerDown($event, shape.id)"
            />
          </template>
          <template v-else-if="resolveShapeWithDraft(shape).kind === 'circle'">
            <ellipse
              class="shape-hit-area"
              :cx="resolveShapeWithDraft(shape).w / 2"
              :cy="resolveShapeWithDraft(shape).h / 2"
              :rx="resolveShapeWithDraft(shape).w / 2"
              :ry="resolveShapeWithDraft(shape).h / 2"
              @pointerdown="onShapePointerDown($event, shape.id)"
            />
            <ellipse
              class="shape-stroke-soft"
              :cx="resolveShapeWithDraft(shape).w / 2"
              :cy="resolveShapeWithDraft(shape).h / 2"
              :rx="resolveShapeWithDraft(shape).w / 2"
              :ry="resolveShapeWithDraft(shape).h / 2"
              :stroke="resolvedStrokeColor(shape)"
              :stroke-width="resolvedSoftStrokeWidth(shape)"
              :stroke-opacity="resolvedSoftStrokeOpacity(shape)"
              fill="none"
            />
            <ellipse
              :cx="resolveShapeWithDraft(shape).w / 2"
              :cy="resolveShapeWithDraft(shape).h / 2"
              :rx="resolveShapeWithDraft(shape).w / 2"
              :ry="resolveShapeWithDraft(shape).h / 2"
              :stroke="resolvedStrokeColor(shape)"
              :stroke-width="resolvedStrokeWidth(shape)"
              :stroke-opacity="resolvedStrokeOpacity(shape)"
              fill="none"
              @pointerdown="onShapePointerDown($event, shape.id)"
            />
          </template>
          <template v-else>
            <rect
              class="shape-hit-area"
              x="0"
              y="0"
              :width="resolveShapeWithDraft(shape).w"
              :height="resolveShapeWithDraft(shape).h"
              rx="0.8"
              ry="0.8"
              @pointerdown="onShapePointerDown($event, shape.id)"
            />
            <image
              v-if="resolveShapeWithDraft(shape).imageAsset?.src"
              x="0"
              y="0"
              :width="resolveShapeWithDraft(shape).w"
              :height="resolveShapeWithDraft(shape).h"
              :href="resolveShapeWithDraft(shape).imageAsset?.src"
              preserveAspectRatio="none"
              @pointerdown="onShapePointerDown($event, shape.id)"
            />
            <rect
              class="shape-stroke-soft"
              x="0"
              y="0"
              :width="resolveShapeWithDraft(shape).w"
              :height="resolveShapeWithDraft(shape).h"
              :stroke="resolvedStrokeColor(shape)"
              :stroke-width="resolvedSoftStrokeWidth(shape)"
              :stroke-opacity="resolvedSoftStrokeOpacity(shape)"
              fill="none"
              rx="0.8"
              ry="0.8"
              stroke-linejoin="round"
            />
            <rect
              x="0"
              y="0"
              :width="resolveShapeWithDraft(shape).w"
              :height="resolveShapeWithDraft(shape).h"
              :stroke="resolvedStrokeColor(shape)"
              :stroke-width="resolvedStrokeWidth(shape)"
              :stroke-opacity="resolvedStrokeOpacity(shape)"
              fill="none"
              rx="0.8"
              ry="0.8"
              stroke-linejoin="round"
              @pointerdown="onShapePointerDown($event, shape.id)"
            />
            <text
              v-if="!resolveShapeWithDraft(shape).imageAsset?.src"
              class="image-placeholder-text"
              :x="resolveShapeWithDraft(shape).w / 2"
              :y="resolveShapeWithDraft(shape).h / 2"
            >
              图片
            </text>
          </template>

          <template v-if="shouldShowHandles(shape)">
            <rect
              class="selection-box"
              x="0"
              y="0"
              :width="resolveShapeWithDraft(shape).w"
              :height="resolveShapeWithDraft(shape).h"
              rx="0.8"
              ry="0.8"
              fill="none"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
            <template v-for="handle in boxHandleList" :key="`${shape.id}-${handle.id}`">
              <circle
                class="control-hit"
                :cx="handle.x * resolveShapeWithDraft(shape).w"
                :cy="handle.y * resolveShapeWithDraft(shape).h"
                :r="resolveControlHitRadius(shape)"
                @pointerdown.stop="onBoxHandlePointerDown($event, shape.id, handle.id)"
              />
              <circle
                class="box-handle"
                :cx="handle.x * resolveShapeWithDraft(shape).w"
                :cy="handle.y * resolveShapeWithDraft(shape).h"
                :r="resolveVisibleHandleRadius(shape)"
              />
            </template>
          </template>
        </template>
      </g>

      <g v-if="drawDraft" class="shape-draft" :transform="shapeTransform(drawDraft)">
        <template v-if="drawDraft.kind === 'line'">
          <line
            class="line-stroke-soft"
            x1="0"
            y1="0"
            :x2="drawDraft.w"
            y2="0"
            :stroke="drawDraft.style.strokeColor"
            :stroke-width="resolvedSoftStrokeWidth(drawDraft)"
            :stroke-opacity="resolvedSoftStrokeOpacity(drawDraft)"
            stroke-linecap="round"
          />
          <line
            x1="0"
            y1="0"
            :x2="drawDraft.w"
            y2="0"
            :stroke="drawDraft.style.strokeColor"
            :stroke-width="drawDraft.style.strokeWidth"
            :stroke-opacity="drawDraft.style.strokeOpacity"
            stroke-linecap="round"
          />
        </template>
        <template v-else-if="drawDraft.kind === 'rect'">
          <rect
            class="shape-stroke-soft"
            x="0"
            y="0"
            :width="drawDraft.w"
            :height="drawDraft.h"
            :stroke="drawDraft.style.strokeColor"
            :stroke-width="resolvedSoftStrokeWidth(drawDraft)"
            :stroke-opacity="resolvedSoftStrokeOpacity(drawDraft)"
            fill="none"
            rx="0.6"
            ry="0.6"
            stroke-linejoin="round"
          />
          <rect
            x="0"
            y="0"
            :width="drawDraft.w"
            :height="drawDraft.h"
            :stroke="drawDraft.style.strokeColor"
            :stroke-width="drawDraft.style.strokeWidth"
            :stroke-opacity="drawDraft.style.strokeOpacity"
            fill="none"
            rx="0.6"
            ry="0.6"
            stroke-linejoin="round"
          />
        </template>
        <template v-else>
          <ellipse
            class="shape-stroke-soft"
            :cx="drawDraft.w / 2"
            :cy="drawDraft.h / 2"
            :rx="drawDraft.w / 2"
            :ry="drawDraft.h / 2"
            :stroke="drawDraft.style.strokeColor"
            :stroke-width="resolvedSoftStrokeWidth(drawDraft)"
            :stroke-opacity="resolvedSoftStrokeOpacity(drawDraft)"
            fill="none"
          />
          <ellipse
            :cx="drawDraft.w / 2"
            :cy="drawDraft.h / 2"
            :rx="drawDraft.w / 2"
            :ry="drawDraft.h / 2"
            :stroke="drawDraft.style.strokeColor"
            :stroke-width="drawDraft.style.strokeWidth"
            :stroke-opacity="drawDraft.style.strokeOpacity"
            fill="none"
          />
        </template>
      </g>
    </svg>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useHudShapeStore } from '../../../store/hudShape'
import type { HudShapeEntity, HudShapeKind } from '../../../types/hudShape'
import { createDefaultHudShapeEntity } from '../../../types/hudShape'
import { clampHudPercent, resolveHudCanvasMetrics, toHudCanvasPercent } from './hudCanvasGeometry'

interface Props {
  canvasEl: HTMLElement | null
  editMode: boolean
}

type BoxHandleId = 'nw' | 'ne' | 'sw' | 'se'
type LineHandleId = 'start' | 'end'

interface ShapeTransformDraft {
  id: string
  patch: Partial<Pick<HudShapeEntity, 'x' | 'y' | 'w' | 'h' | 'rotationDeg'>>
}

const props = defineProps<Props>()
const shapeStore = useHudShapeStore()
const { orderedShapes, selectedShapeId, tool } = storeToRefs(shapeStore)
const svgRef = ref<SVGSVGElement | null>(null)
const drawDraft = ref<HudShapeEntity | null>(null)
const transformDraft = ref<ShapeTransformDraft | null>(null)

const boxHandleList = [
  { id: 'nw', x: 0, y: 0 },
  { id: 'ne', x: 1, y: 0 },
  { id: 'sw', x: 0, y: 1 },
  { id: 'se', x: 1, y: 1 },
] as const satisfies Array<{ id: BoxHandleId; x: number; y: number }>

const dragState = {
  type: '' as '' | 'draw' | 'move' | 'line-resize' | 'box-resize',
  pointerId: -1,
  shapeId: '',
  startClientX: 0,
  startClientY: 0,
  startX: 0,
  startY: 0,
  startW: 0,
  startH: 0,
  canvasWidth: 1,
  canvasHeight: 1,
  lineHeight: 0.4,
  anchorPoint: { x: 0, y: 0 },
  lineHandle: 'start' as LineHandleId,
  boxHandle: 'se' as BoxHandleId,
  shapeSnapshot: null as HudShapeEntity | null,
}

const pointerFrameState = {
  rafId: null as number | null,
  clientX: 0,
  clientY: 0,
}

const isDrawingTool = computed(() => tool.value === 'line' || tool.value === 'rect' || tool.value === 'circle')

function toRadians(value: number): number {
  return (value * Math.PI) / 180
}

function toDegrees(value: number): number {
  return (value * 180) / Math.PI
}

function rotatePoint(point: { x: number; y: number }, deg: number): { x: number; y: number } {
  const rad = toRadians(deg)
  return {
    x: point.x * Math.cos(rad) - point.y * Math.sin(rad),
    y: point.x * Math.sin(rad) + point.y * Math.cos(rad),
  }
}

// What: 将当前拖拽草稿叠加到图形实体上。
// Why: 拖拽和缩放过程中只更新局部预览，避免每一帧都写入 store 造成卡顿。
function resolveShapeWithDraft(shape: HudShapeEntity): HudShapeEntity {
  if (!transformDraft.value || transformDraft.value.id !== shape.id) return shape
  return {
    ...shape,
    ...transformDraft.value.patch,
  }
}

function resolveLineEndpoints(shape: HudShapeEntity): { start: { x: number; y: number }; end: { x: number; y: number } } {
  const target = resolveShapeWithDraft(shape)
  const start = {
    x: target.x,
    y: target.y + target.h / 2,
  }
  const rad = toRadians(target.rotationDeg)
  return {
    start,
    end: {
      x: start.x + Math.cos(rad) * target.w,
      y: start.y + Math.sin(rad) * target.w,
    },
  }
}

function resolveShapeCenter(shape: HudShapeEntity): { x: number; y: number } {
  const target = resolveShapeWithDraft(shape)
  return {
    x: target.x + target.w / 2,
    y: target.y + target.h / 2,
  }
}

function worldToShapeLocal(shape: HudShapeEntity, point: { x: number; y: number }): { x: number; y: number } {
  const target = resolveShapeWithDraft(shape)
  const center = resolveShapeCenter(shape)
  const relative = {
    x: point.x - center.x,
    y: point.y - center.y,
  }
  const local = rotatePoint(relative, -target.rotationDeg)
  return {
    x: local.x + target.w / 2,
    y: local.y + target.h / 2,
  }
}

function shapeLocalToWorld(shape: HudShapeEntity, point: { x: number; y: number }): { x: number; y: number } {
  const target = resolveShapeWithDraft(shape)
  const center = resolveShapeCenter(shape)
  const rotated = rotatePoint(
    {
      x: point.x - target.w / 2,
      y: point.y - target.h / 2,
    },
    target.rotationDeg
  )
  return {
    x: center.x + rotated.x,
    y: center.y + rotated.y,
  }
}

function shapeTransform(shape: HudShapeEntity): string {
  const target = resolveShapeWithDraft(shape)
  if (target.kind === 'line') {
    return `translate(${target.x} ${target.y + target.h / 2}) rotate(${target.rotationDeg} 0 0)`
  }
  return `translate(${target.x} ${target.y}) rotate(${target.rotationDeg} ${target.w / 2} ${target.h / 2})`
}

function resolvedStrokeWidth(shape: HudShapeEntity): number {
  const flash = shapeStore.getShapeFlashState(shape.id)
  if (!flash) return shape.style.strokeWidth
  return shape.style.strokeWidth + flash.widthBoost
}

function resolvedStrokeColor(shape: HudShapeEntity): string {
  const flash = shapeStore.getShapeFlashState(shape.id)
  return flash ? flash.color : shape.style.strokeColor
}

function resolvedStrokeOpacity(shape: HudShapeEntity): number {
  const flash = shapeStore.getShapeFlashState(shape.id)
  return flash ? 1 : shape.style.strokeOpacity
}

function resolvedSoftStrokeWidth(shape: HudShapeEntity): number {
  // What: 为主描边补一层更宽的柔化描边。
  // Why: 细线直接贴在视频上容易显得发硬，稍宽的低透明底线能让边缘更顺眼。
  const widthBoost = shape.kind === 'line' ? 0.28 : 0.24
  return resolvedStrokeWidth(shape) + widthBoost
}

function resolvedSoftStrokeOpacity(shape: HudShapeEntity): number {
  // What: 统一计算柔化描边透明度。
  // Why: 保持主线清晰的同时，只给底层一点柔光，不把细线做成发糊的粗条。
  return Math.min(0.34, resolvedStrokeOpacity(shape) * 0.32)
}

function resolveVisibleHandleRadius(shape: HudShapeEntity): number {
  // What: 可视手柄保持细小。
  // Why: 用户需要明确选中反馈，但不希望大块手柄遮挡视频内容。
  return Math.max(0.42, shape.kind === 'line' ? shape.style.strokeWidth * 0.95 : 0.58)
}

function resolveControlHitRadius(shape: HudShapeEntity): number {
  // What: 放大手柄命中范围。
  // Why: 细线和小图形也要容易拖到，避免“看得见但点不中”的反人类体验。
  return Math.max(1.4, resolveVisibleHandleRadius(shape) + 0.8)
}

function resolveLineHitWidth(shape: HudShapeEntity): number {
  // What: 为细线叠加透明热区。
  // Why: 默认线条更细后，仍需保证整条线都能直接拖动。
  return Math.max(2.8, shape.style.strokeWidth + 1.9)
}

function shouldShowHandles(shape: HudShapeEntity): boolean {
  return props.editMode && tool.value === 'select' && selectedShapeId.value === shape.id && !shape.locked
}

function applyPointerFrame(clientX: number, clientY: number): void {
  if (dragState.type === 'draw') {
    updateDrawDraft(clientX, clientY)
    return
  }
  if (dragState.type === 'move') {
    updateMoveDraft(clientX, clientY)
    return
  }
  if (dragState.type === 'line-resize') {
    const patch = resolveLineDraftFromPointer(clientX, clientY)
    if (!patch || !transformDraft.value) return
    transformDraft.value = {
      id: transformDraft.value.id,
      patch,
    }
    return
  }
  if (dragState.type === 'box-resize') {
    const patch = resolveBoxResizeDraft(clientX, clientY)
    if (!patch || !transformDraft.value) return
    transformDraft.value = {
      id: transformDraft.value.id,
      patch,
    }
  }
}

function flushPointerFrame(clientX?: number, clientY?: number): void {
  const nextClientX = clientX ?? pointerFrameState.clientX
  const nextClientY = clientY ?? pointerFrameState.clientY
  applyPointerFrame(nextClientX, nextClientY)
}

function queuePointerFrame(clientX: number, clientY: number): void {
  pointerFrameState.clientX = clientX
  pointerFrameState.clientY = clientY
  if (pointerFrameState.rafId !== null) return
  // What: 预览层统一合帧处理 pointer move。
  // Why: 高频拖拽时只消费最后一帧指针位置，可显著减少响应式抖动和掉帧。
  pointerFrameState.rafId = window.requestAnimationFrame(() => {
    pointerFrameState.rafId = null
    flushPointerFrame()
  })
}

function cancelPointerFrame(): void {
  if (pointerFrameState.rafId === null) return
  window.cancelAnimationFrame(pointerFrameState.rafId)
  pointerFrameState.rafId = null
}

function beginPointerTracking(event: PointerEvent): void {
  dragState.pointerId = event.pointerId
  pointerFrameState.clientX = event.clientX
  pointerFrameState.clientY = event.clientY
  window.addEventListener('pointermove', onWindowPointerMove)
  window.addEventListener('pointerup', onWindowPointerUp)
  window.addEventListener('pointercancel', onWindowPointerUp)
}

function stopPointerTracking(): void {
  window.removeEventListener('pointermove', onWindowPointerMove)
  window.removeEventListener('pointerup', onWindowPointerUp)
  window.removeEventListener('pointercancel', onWindowPointerUp)
  dragState.pointerId = -1
  cancelPointerFrame()
}

function startDraw(event: PointerEvent, kind: Exclude<HudShapeKind, 'image'>): void {
  const point = toHudCanvasPercent(props.canvasEl, event.clientX, event.clientY)
  const draft = createDefaultHudShapeEntity(kind)
  const lineHeight = 0.4
  drawDraft.value =
    kind === 'line'
      ? {
          ...draft,
          x: point.x,
          y: clampHudPercent(point.y - lineHeight / 2, 0, 100 - lineHeight),
          w: 0.6,
          h: lineHeight,
        }
      : {
          ...draft,
          x: point.x,
          y: point.y,
          w: 1,
          h: 1,
        }
  dragState.type = 'draw'
  dragState.startX = point.x
  dragState.startY = point.y
  dragState.lineHeight = lineHeight
  beginPointerTracking(event)
}

function startMove(event: PointerEvent, shape: HudShapeEntity): void {
  const metrics = resolveHudCanvasMetrics(props.canvasEl)
  dragState.type = 'move'
  dragState.shapeId = shape.id
  dragState.startClientX = event.clientX
  dragState.startClientY = event.clientY
  dragState.startX = shape.x
  dragState.startY = shape.y
  dragState.startW = shape.w
  dragState.startH = shape.h
  dragState.canvasWidth = metrics.widthPx
  dragState.canvasHeight = metrics.heightPx
  transformDraft.value = {
    id: shape.id,
    patch: {
      x: shape.x,
      y: shape.y,
    },
  }
  beginPointerTracking(event)
}

function startLineResize(event: PointerEvent, shape: HudShapeEntity, handle: LineHandleId): void {
  const endpoints = resolveLineEndpoints(shape)
  dragState.type = 'line-resize'
  dragState.shapeId = shape.id
  dragState.lineHeight = shape.h
  dragState.lineHandle = handle
  dragState.shapeSnapshot = shape
  dragState.anchorPoint = handle === 'start' ? endpoints.end : endpoints.start
  transformDraft.value = {
    id: shape.id,
    patch: {},
  }
  beginPointerTracking(event)
}

function startBoxResize(event: PointerEvent, shape: HudShapeEntity, handle: BoxHandleId): void {
  dragState.type = 'box-resize'
  dragState.shapeId = shape.id
  dragState.boxHandle = handle
  dragState.shapeSnapshot = shape
  transformDraft.value = {
    id: shape.id,
    patch: {},
  }
  beginPointerTracking(event)
}

function onShapePointerDown(event: PointerEvent, shapeId: string): void {
  if (!props.editMode) return
  if (event.button !== 0) return
  const shape = shapeStore.shapes.find((item) => item.id === shapeId)
  if (!shape) return
  shapeStore.selectShape(shapeId)
  if (tool.value !== 'select') return
  if (shape.locked) return

  // What: 选择工具下仅对当前图形开启拖拽预览。
  // Why: 减少频繁写 localStorage，避免拖拽过程卡顿和状态竞争。
  event.preventDefault()
  event.stopPropagation()
  startMove(event, shape)
}

function onLineHandlePointerDown(event: PointerEvent, shapeId: string, handle: LineHandleId): void {
  if (!props.editMode || event.button !== 0) return
  const shape = shapeStore.shapes.find((item) => item.id === shapeId)
  if (!shape || shape.kind !== 'line' || shape.locked || tool.value !== 'select') return
  event.preventDefault()
  startLineResize(event, shape, handle)
}

function onBoxHandlePointerDown(event: PointerEvent, shapeId: string, handle: BoxHandleId): void {
  if (!props.editMode || event.button !== 0) return
  const shape = shapeStore.shapes.find((item) => item.id === shapeId)
  if (!shape || shape.kind === 'line' || shape.locked || tool.value !== 'select') return
  event.preventDefault()
  startBoxResize(event, shape, handle)
}

function onSvgPointerDown(event: PointerEvent): void {
  if (!props.editMode) return
  if (event.button !== 0) return
  if (!isDrawingTool.value) {
    // What: 选择工具点击空白处取消选中。
    // Why: 让属性面板与画布选中态保持同步，避免“幽灵选中”。
    shapeStore.selectShape(null)
    return
  }

  // What: 绘制工具在空白画布开始创建草图。
  // Why: 支持线条/矩形/圆形通过同一交互路径生成，降低状态机复杂度。
  event.preventDefault()
  event.stopPropagation()
  startDraw(event, tool.value as Exclude<HudShapeKind, 'image'>)
}

function updateDrawDraft(clientX: number, clientY: number): void {
  if (!drawDraft.value) return
  const point = toHudCanvasPercent(props.canvasEl, clientX, clientY)
  if (drawDraft.value.kind === 'line') {
    const dx = point.x - dragState.startX
    const dy = point.y - dragState.startY
    const nextLength = Math.max(0.6, Math.sqrt(dx * dx + dy * dy))
    drawDraft.value = {
      ...drawDraft.value,
      x: clampHudPercent(dragState.startX, 0, 100),
      y: clampHudPercent(dragState.startY - dragState.lineHeight / 2, 0, 100 - dragState.lineHeight),
      w: nextLength,
      h: dragState.lineHeight,
      rotationDeg: toDegrees(Math.atan2(dy, dx)),
    }
    return
  }

  drawDraft.value = {
    ...drawDraft.value,
    x: Math.min(dragState.startX, point.x),
    y: Math.min(dragState.startY, point.y),
    w: Math.max(1, Math.abs(point.x - dragState.startX)),
    h: Math.max(1, Math.abs(point.y - dragState.startY)),
  }
}

function updateMoveDraft(clientX: number, clientY: number): void {
  if (!transformDraft.value) return
  const dx = ((clientX - dragState.startClientX) / dragState.canvasWidth) * 100
  const dy = ((clientY - dragState.startClientY) / dragState.canvasHeight) * 100
  transformDraft.value = {
    id: transformDraft.value.id,
    patch: {
      x: clampHudPercent(dragState.startX + dx, 0, 100 - dragState.startW),
      y: clampHudPercent(dragState.startY + dy, 0, 100 - dragState.startH),
    },
  }
}

function resolveLineDraftFromPointer(clientX: number, clientY: number): Partial<HudShapeEntity> | null {
  const shape = dragState.shapeSnapshot
  if (!shape) return null
  const pointer = toHudCanvasPercent(props.canvasEl, clientX, clientY)
  const nextPoint = {
    x: clampHudPercent(pointer.x, 0, 100),
    y: clampHudPercent(pointer.y, 0, 100),
  }
  const start = dragState.lineHandle === 'start' ? nextPoint : dragState.anchorPoint
  const end = dragState.lineHandle === 'start' ? dragState.anchorPoint : nextPoint
  const dx = end.x - start.x
  const dy = end.y - start.y
  return {
    x: start.x,
    y: clampHudPercent(start.y - dragState.lineHeight / 2, 0, 100 - dragState.lineHeight),
    w: Math.max(0.6, Math.sqrt(dx * dx + dy * dy)),
    h: dragState.lineHeight,
    rotationDeg: toDegrees(Math.atan2(dy, dx)),
  }
}

function resolveBoxResizeDraft(clientX: number, clientY: number): Partial<HudShapeEntity> | null {
  const shape = dragState.shapeSnapshot
  if (!shape || shape.kind === 'line') return null
  const pointer = toHudCanvasPercent(props.canvasEl, clientX, clientY)
  const pointerLocal = worldToShapeLocal(shape, pointer)
  const oppositeCorner =
    dragState.boxHandle === 'nw'
      ? { x: shape.w, y: shape.h }
      : dragState.boxHandle === 'ne'
        ? { x: 0, y: shape.h }
        : dragState.boxHandle === 'sw'
          ? { x: shape.w, y: 0 }
          : { x: 0, y: 0 }

  const minSize = shape.kind === 'image' ? 2 : 1
  let nextWidth = Math.max(minSize, Math.abs(pointerLocal.x - oppositeCorner.x))
  let nextHeight = Math.max(minSize, Math.abs(pointerLocal.y - oppositeCorner.y))

  if (shape.kind === 'image') {
    const ratio = Math.max(0.1, (shape.imageAsset?.naturalWidth ?? shape.w) / Math.max(1, shape.imageAsset?.naturalHeight ?? shape.h))
    // What: 图片拖角时强制等比。
    // Why: 贴图元素主要用于图标和装甲示意，默认保持比例更符合使用习惯也更不容易做丑。
    if (nextWidth / nextHeight > ratio) {
      nextHeight = Math.max(minSize, nextWidth / ratio)
    } else {
      nextWidth = Math.max(minSize, nextHeight * ratio)
    }
  }

  const nextAnchorLocal =
    dragState.boxHandle === 'nw'
      ? { x: nextWidth, y: nextHeight }
      : dragState.boxHandle === 'ne'
        ? { x: 0, y: nextHeight }
        : dragState.boxHandle === 'sw'
          ? { x: nextWidth, y: 0 }
          : { x: 0, y: 0 }

  const worldAnchor = shapeLocalToWorld(shape, oppositeCorner)
  const nextCenterLocal = { x: nextWidth / 2, y: nextHeight / 2 }
  const rotatedAnchorOffset = rotatePoint(
    {
      x: nextAnchorLocal.x - nextCenterLocal.x,
      y: nextAnchorLocal.y - nextCenterLocal.y,
    },
    shape.rotationDeg
  )

  return {
    x: worldAnchor.x - nextCenterLocal.x - rotatedAnchorOffset.x,
    y: worldAnchor.y - nextCenterLocal.y - rotatedAnchorOffset.y,
    w: nextWidth,
    h: nextHeight,
  }
}

function onWindowPointerMove(event: PointerEvent): void {
  if (event.pointerId !== dragState.pointerId) return
  queuePointerFrame(event.clientX, event.clientY)
}

function finishDraw(): void {
  if (!drawDraft.value) return
  const next = drawDraft.value
  const id = shapeStore.addShape(next.kind, {
    x: next.x,
    y: next.y,
    w: next.w,
    h: next.h,
    rotationDeg: next.rotationDeg,
    style: next.style,
    lineBinding: next.lineBinding,
  })
  if (id) {
    shapeStore.selectShape(id)
  }
  // What: 单次添加完成后强制回编辑态。
  // Why: 将“添加”和“编辑”分离，避免连续误创建图形。
  shapeStore.enterEditMode()
  drawDraft.value = null
}

function finishTransform(): void {
  if (!transformDraft.value) return
  shapeStore.updateShapeTransform(transformDraft.value.id, transformDraft.value.patch)
  transformDraft.value = null
}

function resetDragState(): void {
  dragState.type = ''
  dragState.shapeId = ''
  dragState.shapeSnapshot = null
  stopPointerTracking()
}

function cancelActiveInteraction(): void {
  // What: 统一清理拖拽/绘制中间态。
  // Why: 退出编辑或切回选择工具时，必须立即释放 pointer 监听并移除草图，避免光标与状态残留。
  drawDraft.value = null
  transformDraft.value = null
  cancelPointerFrame()
  resetDragState()
}

function onSvgWheel(event: WheelEvent): void {
  if (!props.editMode) return
  if (tool.value !== 'select') return
  if (!selectedShapeId.value) return
  const target = shapeStore.shapes.find((item) => item.id === selectedShapeId.value)
  if (!target || target.kind !== 'line' || target.locked) return
  event.preventDefault()
  // What: 滚轮直接调节选中线条长度。
  // Why: 把缩放留在画布主路径里，用户无需再按组合键才能完成高频微调。
  const step = event.deltaY < 0 ? 0.6 : -0.6
  shapeStore.updateShapeTransform(target.id, {
    w: clampHudPercent(target.w + step, 0.6, 100),
  })
}

function onWindowPointerUp(event: PointerEvent): void {
  if (event.pointerId !== dragState.pointerId) return
  cancelPointerFrame()
  flushPointerFrame(event.clientX, event.clientY)
  if (dragState.type === 'draw') finishDraw()
  if (dragState.type === 'move' || dragState.type === 'line-resize' || dragState.type === 'box-resize') finishTransform()
  resetDragState()
}

watch(
  () => [props.editMode, tool.value] as const,
  ([editMode, currentTool]) => {
    if (editMode && currentTool !== 'select' && currentTool !== 'image') return
    cancelActiveInteraction()
  }
)

onBeforeUnmount(() => {
  cancelActiveInteraction()
})
</script>

<style scoped>
.shape-layer {
  position: absolute;
  inset: 0;
  z-index: var(--z-data);
  pointer-events: none;
}

.shape-layer.editing {
  pointer-events: auto;
}

.shape-svg {
  width: 100%;
  height: 100%;
  overflow: visible;
  pointer-events: auto;
  shape-rendering: geometricPrecision;
}

.shape-svg.drawing {
  cursor: crosshair;
}

.shape-node {
  pointer-events: auto;
  transition: filter 110ms ease, opacity 110ms ease;
}

.shape-node.hidden {
  opacity: 0;
  pointer-events: none;
}

.shape-node.selected {
  filter: drop-shadow(0 0 7px var(--hud-shape-selected-glow));
}

.shape-node.selected.locked {
  filter: drop-shadow(0 0 5px var(--hud-shape-selected-locked-glow));
}

.line-hit-area {
  stroke: var(--hud-shape-hit-fill);
  pointer-events: stroke;
}

.line-stroke-soft,
.shape-stroke-soft {
  /* What: 柔化描边永远只负责视觉过渡。
     Why: 命中与拖拽统一交给透明热区处理，避免双层描边抢事件导致交互不稳定。 */
  pointer-events: none;
}

.line-stroke {
  pointer-events: none;
}

.shape-hit-area {
  fill: var(--hud-shape-hit-fill);
  stroke: none;
}

.selection-box {
  stroke: var(--hud-selection-stroke);
  stroke-width: 0.28;
  stroke-dasharray: 2.2 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.control-hit {
  fill: var(--hud-shape-hit-fill);
  stroke: none;
}

.line-handle,
.box-handle {
  fill: var(--hud-handle-fill);
  stroke: var(--hud-handle-stroke);
  stroke-width: 0.18;
  pointer-events: none;
}

.image-placeholder-text {
  fill: var(--hud-text-primary);
  font-size: 1.8px;
  text-anchor: middle;
  dominant-baseline: middle;
}

.shape-draft {
  pointer-events: none;
  opacity: var(--hud-shape-draft-opacity);
}
</style>
