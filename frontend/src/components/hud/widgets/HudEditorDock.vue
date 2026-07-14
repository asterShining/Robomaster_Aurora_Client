<template>
  <aside
    v-if="shapeStore.isDrawMode"
    ref="dockRef"
    class="editor-dock hud-realtime"
    :style="dockStyle"
    :class="{ collapsed, dragging }"
    @pointerdown.stop
  >
    <header class="dock-head" @pointerdown="onDockHeaderPointerDown">
      <div class="dock-title">
        <strong>UI编辑</strong>
        <small>U 退出</small>
      </div>
      <div class="dock-head-actions">
        <button type="button" class="head-btn" @pointerdown.stop @click="onToggleCollapsed">
          {{ collapsed ? '展开' : '收起' }}
        </button>
        <button type="button" class="head-btn danger" @pointerdown.stop @click="onCloseEditor">
          关闭
        </button>
      </div>
    </header>

    <div v-show="!collapsed" class="dock-body" @pointerdown.stop>
      <section class="dock-mode">
        <span class="mode-pill" :class="{ add: isAddMode }">
          {{ isAddMode ? `添加模式：${currentAddLabel}` : '编辑模式' }}
        </span>
        <button v-if="isAddMode" type="button" class="mini-btn danger" @click="onCancelAddMode">取消添加</button>
      </section>

      <section class="dock-inline-actions">
        <button type="button" class="mini-btn" :class="{ 'active-add': tool === 'line' }" @click="onEditorAddShape('line')">
          添加直线
        </button>
        <button type="button" class="mini-btn" :class="{ 'active-add': tool === 'rect' }" @click="onEditorAddShape('rect')">
          添加方形
        </button>
        <button type="button" class="mini-btn" :class="{ 'active-add': tool === 'circle' }" @click="onEditorAddShape('circle')">
          添加圆形
        </button>
        <button type="button" class="mini-btn" @click="onEditorAddImageButtonClick">添加图像</button>
        <button type="button" class="mini-btn danger" @click="onEditorResetShapes">清空</button>
        <input
          ref="imageFileRef"
          class="scheme-file-input"
          type="file"
          accept="image/png,image/jpeg,image/webp,image/svg+xml"
          @change="onEditorImageFileChange"
        />
      </section>

      <section class="dock-shape-list">
        <article
          v-for="item in editorShapeRows"
          :key="item.id"
          class="shape-item"
          :class="{ active: selectedShape?.id === item.id }"
          @click="onEditorSelectShape(item.id)"
        >
          <header>
            <strong>{{ item.label }}</strong>
            <small>{{ resolveShapeKindLabel(item.shape.kind) }}</small>
          </header>
          <div class="shape-item-actions">
            <button type="button" class="tiny-btn" @click.stop="onEditorShapeDuplicate(item.id)">复制</button>
            <button type="button" class="tiny-btn danger" @click.stop="onEditorShapeDelete(item.id)">删</button>
          </div>
        </article>
      </section>

      <section v-if="selectedShape" class="dock-detail">
        <div class="detail-caption">已选中：{{ selectedShape.name || resolveShapeKindLabel(selectedShape.kind) }}</div>
        <label class="field">
          <span>图形名</span>
          <input type="text" :value="selectedShape.name" @input="onEditorShapeNameInput" />
        </label>
        <label v-if="selectedShape.kind !== 'image'" class="field">
          <span>{{ selectedShape.kind === 'line' ? '线宽' : '描边宽度' }} {{ selectedShape.style.strokeWidth.toFixed(1) }}</span>
          <input
            type="range"
            :min="HUD_STROKE_WIDTH_MIN"
            :max="HUD_STROKE_WIDTH_MAX"
            step="0.05"
            :value="selectedShape.style.strokeWidth"
            @input="onEditorShapeStrokeWidthInput"
          />
        </label>
        <label v-if="selectedShape.kind === 'line'" class="field">
          <span>长度 {{ selectedShape.w.toFixed(1) }}</span>
          <input
            type="range"
            min="0.6"
            max="100"
            step="0.1"
            :value="selectedShape.w"
            @input="onEditorShapeLengthInput"
          />
          <small class="field-hint">拖动两端手柄可直接改长度与角度，滚轮可直接微调长度</small>
        </label>
        <label v-if="selectedShape.kind !== 'image'" class="field">
          <span>{{ selectedShape.kind === 'line' ? '线条颜色' : '描边颜色' }}</span>
          <input type="color" :value="selectedShape.style.strokeColor" @input="onEditorShapeStrokeColorInput" />
        </label>
        <label v-if="selectedShape.kind !== 'image'" class="field">
          <span>{{ selectedShape.kind === 'line' ? '线条透明' : '描边透明' }} {{ selectedShape.style.strokeOpacity.toFixed(2) }}</span>
          <input
            type="range"
            min="0"
            max="1"
            step="0.01"
            :value="selectedShape.style.strokeOpacity"
            @input="onEditorShapeStrokeOpacityInput"
          />
        </label>
        <section v-if="selectedShape.kind === 'image'" class="image-asset">
          <div class="detail-caption image-caption">
            <span>{{ selectedShape.imageAsset?.name || '未设置图片' }}</span>
            <small>{{ selectedShape.w.toFixed(1) }} × {{ selectedShape.h.toFixed(1) }}</small>
          </div>
          <div class="dock-inline-actions">
            <button type="button" class="mini-btn" @click="onEditorReplaceImageButtonClick">替换图片</button>
            <button type="button" class="mini-btn danger" @click="onEditorShapeDelete(selectedShape.id)">删除图片</button>
          </div>
        </section>

        <section v-if="selectedShape.kind === 'line' && selectedLineBinding" class="line-binding">
          <h4>受击绑定</h4>
          <label class="check-field">
            <input type="checkbox" :checked="selectedLineBinding.enabled" @change="onEditorLineBindingEnabledChange" />
            <span>启用高亮</span>
          </label>
          <div class="field-grid">
            <label class="field">
              <span>事件ID</span>
              <input type="number" min="0" :value="selectedLineBinding.eventId" @input="onEditorLineBindingEventIdInput" />
            </label>
            <label class="field">
              <span>方位</span>
              <select :value="selectedLineBinding.zone" @change="onEditorLineBindingZoneInput">
                <option value="any">任意</option>
                <option value="front">前</option>
                <option value="back">后</option>
                <option value="left">左</option>
                <option value="right">右</option>
              </select>
            </label>
          </div>
          <label class="field">
            <span>高亮时长 {{ selectedLineBinding.flashMs }}ms</span>
            <input
              type="range"
              min="80"
              max="1200"
              step="20"
              :value="selectedLineBinding.flashMs"
              @input="onEditorLineBindingFlashMsInput"
            />
          </label>
          <label class="field">
            <span>冷却 {{ selectedLineBinding.cooldownMs }}ms</span>
            <input
              type="range"
              min="0"
              max="2000"
              step="20"
              :value="selectedLineBinding.cooldownMs"
              @input="onEditorLineBindingCooldownInput"
            />
          </label>
          <button type="button" class="mini-btn" @click="onEditorLineBindingPreview">预览高亮</button>
        </section>
      </section>
      <section v-else class="dock-empty">未选中图形</section>

      <section class="dock-scheme">
        <button type="button" class="scheme-toggle" @click="onToggleSchemePanel">
          {{ schemeExpanded ? '隐藏方案管理' : '显示方案管理' }}
        </button>
        <div v-show="schemeExpanded" class="scheme-body">
          <label class="field">
            <span>导出名</span>
            <input type="text" v-model="exportSchemeName" />
          </label>
          <div class="dock-inline-actions">
            <button type="button" class="mini-btn" @click="onEditorExportScheme">导出</button>
            <button type="button" class="mini-btn" @click="onEditorImportButtonClick">导入</button>
            <input
              ref="schemeFileRef"
              class="scheme-file-input"
              type="file"
              accept=".json,application/json"
              @change="onEditorSchemeFileChange"
            />
          </div>
          <div class="dock-inline-actions">
            <select v-model="selectedPresetId">
              <option v-for="preset in HUD_PRESETS" :key="preset.id" :value="preset.id">
                {{ preset.label }}
              </option>
            </select>
            <button type="button" class="mini-btn" @click="onEditorApplyPreset">应用预设</button>
          </div>
        </div>
      </section>
      <small class="dock-message">{{ editorMessage || ' ' }}</small>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useHudLayoutStore } from '../../../store/hudLayout'
import { useHudShapeStore } from '../../../store/hudShape'
import type { HudLayoutSnapshot } from '../../../types/hudLayout'
import type { HudPresetId } from '../../../types/hudPresets'
import { HUD_PRESETS } from '../../../types/hudPresets'
import type { HudShapeKind, HudShapeSnapshot } from '../../../types/hudShape'
import { HUD_STROKE_WIDTH_MAX, HUD_STROKE_WIDTH_MIN } from '../../../types/hudShape'
import { buildHudDesignScheme, downloadHudDesignScheme, parseHudDesignSchemeFile, resolveHudPresetScheme } from '../../../services/hudScheme'
import { buildHudImageAssetFromFile } from '../../../services/hudImageAsset'
import { clampHudPercent, resolveHudCanvasMetrics, resolveHudImagePercentSize } from '../core/hudCanvasGeometry'

interface Props {
  canvasEl: HTMLElement | null
}

const props = defineProps<Props>()

interface DockStorageSnapshot {
  x: number
  y: number
  collapsed: boolean
  schemeExpanded: boolean
}

const HUD_EDITOR_DOCK_STORAGE_KEY = 'rm-client-hud-editor-dock:v1'

const dockRef = ref<HTMLElement | null>(null)
const imageFileRef = ref<HTMLInputElement | null>(null)
const schemeFileRef = ref<HTMLInputElement | null>(null)
const selectedPresetId = ref<HudPresetId>('match_standard')
const exportSchemeName = ref('比赛方案')
const editorMessage = ref('')
const collapsed = ref(false)
const schemeExpanded = ref(false)
const dragging = ref(false)
const dockX = ref(0)
const dockY = ref(0)
const hasAppliedStoredState = ref(false)
const imageUploadIntent = ref<'add' | 'replace'>('add')

const dragState = {
  pointerId: -1,
  startClientX: 0,
  startClientY: 0,
  startX: 0,
  startY: 0,
}

const layoutStore = useHudLayoutStore()
const shapeStore = useHudShapeStore()
const { tool, selectedShape } = storeToRefs(shapeStore)

const dockStyle = computed<Record<string, string>>(() => ({
  left: `${dockX.value}px`,
  top: `${dockY.value}px`,
}))

const editorShapeRows = computed(() => {
  // What: 图形列表按 Z 轴倒序展示。
  // Why: 先展示上层图形，用户更容易定位遮挡关系。
  return [...shapeStore.orderedShapes]
    .reverse()
    .map((shape) => ({
      id: shape.id,
      shape,
      label: shape.name || resolveShapeKindLabel(shape.kind),
    }))
})

const selectedLineBinding = computed(() => {
  if (!selectedShape.value || selectedShape.value.kind !== 'line') return null
  return selectedShape.value.lineBinding ?? null
})

const isAddMode = computed(() => tool.value === 'line' || tool.value === 'rect' || tool.value === 'circle')

const currentAddLabel = computed(() => {
  if (!isAddMode.value) return ''
  return resolveShapeKindLabel(tool.value as HudShapeKind)
})

function resolveShapeKindLabel(kind: HudShapeKind): string {
  if (kind === 'line') return '直线'
  if (kind === 'rect') return '方形'
  if (kind === 'circle') return '圆形'
  return '图像'
}

function readDockStorage(): DockStorageSnapshot | null {
  if (typeof window === 'undefined') return null
  try {
    const raw = window.localStorage.getItem(HUD_EDITOR_DOCK_STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<DockStorageSnapshot>
    if (
      typeof parsed.x !== 'number' ||
      typeof parsed.y !== 'number' ||
      typeof parsed.collapsed !== 'boolean' ||
      typeof parsed.schemeExpanded !== 'boolean'
    ) {
      return null
    }
    return {
      x: parsed.x,
      y: parsed.y,
      collapsed: parsed.collapsed,
      schemeExpanded: parsed.schemeExpanded,
    }
  } catch {
    return null
  }
}

function saveDockStorage(): void {
  if (typeof window === 'undefined') return
  // What: 持久化编辑窗位置与折叠态。
  // Why: 保证用户每次进入编辑模式都回到熟悉布局，减少重复调整。
  const payload: DockStorageSnapshot = {
    x: dockX.value,
    y: dockY.value,
    collapsed: collapsed.value,
    schemeExpanded: schemeExpanded.value,
  }
  window.localStorage.setItem(HUD_EDITOR_DOCK_STORAGE_KEY, JSON.stringify(payload))
}

function resolveDockBounds(): { maxX: number; maxY: number } {
  const dockEl = dockRef.value
  const hostEl = dockEl?.parentElement as HTMLElement | null
  if (!dockEl || !hostEl) {
    return { maxX: 0, maxY: 0 }
  }
  const maxX = Math.max(8, hostEl.clientWidth - dockEl.offsetWidth - 8)
  const maxY = Math.max(8, hostEl.clientHeight - dockEl.offsetHeight - 8)
  return { maxX, maxY }
}

function clampDockPosition(nextX: number, nextY: number): { x: number; y: number } {
  const bounds = resolveDockBounds()
  return {
    x: Math.min(Math.max(8, nextX), bounds.maxX),
    y: Math.min(Math.max(8, nextY), bounds.maxY),
  }
}

function applyDefaultDockPosition(): void {
  const bounds = resolveDockBounds()
  // What: 默认将编辑窗停靠右下。
  // Why: 避开准星与中心区域，降低对绘制视野的遮挡概率。
  dockX.value = bounds.maxX
  dockY.value = bounds.maxY
}

function applyStoredDockPosition(snapshot: DockStorageSnapshot): void {
  const clamped = clampDockPosition(snapshot.x, snapshot.y)
  dockX.value = clamped.x
  dockY.value = clamped.y
  collapsed.value = snapshot.collapsed
  schemeExpanded.value = snapshot.schemeExpanded
}

async function ensureDockPositionReady(): Promise<void> {
  await nextTick()
  if (!hasAppliedStoredState.value) {
    const stored = readDockStorage()
    if (stored) {
      applyStoredDockPosition(stored)
    } else {
      applyDefaultDockPosition()
    }
    hasAppliedStoredState.value = true
    saveDockStorage()
    return
  }
  const clamped = clampDockPosition(dockX.value, dockY.value)
  dockX.value = clamped.x
  dockY.value = clamped.y
  saveDockStorage()
}

function onEditorAddShape(kind: HudShapeKind): void {
  if (!shapeStore.canCreateNewShape) {
    editorMessage.value = '图形数量已到上限'
    return
  }
  // What: 点击“添加”仅切换到单次添加模式。
  // Why: 将添加和编辑分离，避免编辑期间误触连续创建图形。
  shapeStore.enterAddMode(kind)
  editorMessage.value = `请在画布拖拽创建${resolveShapeKindLabel(kind)}，完成后自动回到编辑`
}

function resolveCanvasMetricsForImage() {
  const hostEl = props.canvasEl ?? ((dockRef.value?.parentElement as HTMLElement | null) ?? null)
  return resolveHudCanvasMetrics(hostEl)
}

function resolveCenteredImageRect(width: number, height: number): { x: number; y: number; w: number; h: number } {
  return {
    w: width,
    h: height,
    x: clampHudPercent((100 - width) / 2, 0, Math.max(0, 100 - width)),
    y: clampHudPercent((100 - height) / 2, 0, Math.max(0, 100 - height)),
  }
}

function openImageFileDialog(intent: 'add' | 'replace'): void {
  imageUploadIntent.value = intent
  imageFileRef.value?.click()
}

function onEditorAddImageButtonClick(): void {
  if (!shapeStore.canCreateNewShape) {
    editorMessage.value = '图形数量已到上限'
    return
  }
  // What: 图像元素直接走文件选择。
  // Why: 图片不需要先在画布拖框，点击即插入更符合常见编辑器习惯。
  openImageFileDialog('add')
}

function onEditorReplaceImageButtonClick(): void {
  if (!selectedShape.value || selectedShape.value.kind !== 'image') return
  openImageFileDialog('replace')
}

function onCancelAddMode(): void {
  // What: 手动取消当前添加工具并回到编辑态。
  // Why: 用户切换思路时应能快速止损，避免继续保持十字光标。
  shapeStore.enterEditMode()
  editorMessage.value = '已取消添加，返回编辑模式'
}

function onEditorSelectShape(shapeId: string): void {
  // What: 选择图形时强制回编辑态。
  // Why: 避免列表选中后仍停留添加工具，导致下一次点击继续画新图形。
  shapeStore.enterEditMode()
  shapeStore.selectShape(shapeId)
}

function onEditorShapeDuplicate(shapeId: string): void {
  shapeStore.duplicateShape(shapeId)
}

function onEditorShapeDelete(shapeId: string): void {
  shapeStore.removeShape(shapeId)
}

function onEditorShapeNameInput(event: Event): void {
  if (!selectedShape.value) return
  shapeStore.updateShape(selectedShape.value.id, {
    name: (event.target as HTMLInputElement).value,
  })
}

function onEditorShapeLengthInput(event: Event): void {
  if (!selectedShape.value || selectedShape.value.kind !== 'line') return
  // What: 直接调节线条长度。
  // Why: 线条是高频微调元素，长度单独暴露比逼用户拖滚轮更直觉。
  shapeStore.updateShapeTransform(selectedShape.value.id, {
    w: Number((event.target as HTMLInputElement).value),
  })
}

function onEditorShapeStrokeWidthInput(event: Event): void {
  if (!selectedShape.value) return
  // What: 将线宽输入交由 store 统一归一化。
  // Why: 导入、拖拽和面板输入共享同一 clamp 规则，避免边界不一致。
  shapeStore.updateShapeStyle(selectedShape.value.id, {
    strokeWidth: Number((event.target as HTMLInputElement).value),
  })
}

function onEditorShapeStrokeOpacityInput(event: Event): void {
  if (!selectedShape.value) return
  // What: 统一更新图形描边透明度。
  // Why: 线段与轮廓图形共用同一入口后，样式控制需要保持一致语义。
  shapeStore.updateShapeStyle(selectedShape.value.id, {
    strokeOpacity: Number((event.target as HTMLInputElement).value),
  })
}

function onEditorShapeStrokeColorInput(event: Event): void {
  if (!selectedShape.value) return
  // What: 统一更新图形描边颜色。
  // Why: 只保留描边色这一条主控制路径，可显著降低绘制面板的认知负担。
  shapeStore.updateShapeStyle(selectedShape.value.id, {
    strokeColor: (event.target as HTMLInputElement).value,
  })
}

function onEditorLineBindingEnabledChange(event: Event): void {
  if (!selectedShape.value || selectedShape.value.kind !== 'line') return
  shapeStore.updateLineBinding(selectedShape.value.id, {
    enabled: (event.target as HTMLInputElement).checked,
  })
}

function onEditorLineBindingEventIdInput(event: Event): void {
  if (!selectedShape.value || selectedShape.value.kind !== 'line') return
  shapeStore.updateLineBinding(selectedShape.value.id, {
    eventId: Number((event.target as HTMLInputElement).value),
  })
}

function onEditorLineBindingZoneInput(event: Event): void {
  if (!selectedShape.value || selectedShape.value.kind !== 'line') return
  const zone = (event.target as HTMLSelectElement).value as 'front' | 'back' | 'left' | 'right' | 'any'
  shapeStore.updateLineBinding(selectedShape.value.id, {
    zone,
  })
}

function onEditorLineBindingFlashMsInput(event: Event): void {
  if (!selectedShape.value || selectedShape.value.kind !== 'line') return
  shapeStore.updateLineBinding(selectedShape.value.id, {
    flashMs: Number((event.target as HTMLInputElement).value),
  })
}

function onEditorLineBindingCooldownInput(event: Event): void {
  if (!selectedShape.value || selectedShape.value.kind !== 'line') return
  shapeStore.updateLineBinding(selectedShape.value.id, {
    cooldownMs: Number((event.target as HTMLInputElement).value),
  })
}

function onEditorLineBindingPreview(): void {
  if (!selectedShape.value || selectedShape.value.kind !== 'line') return
  // What: 在编辑态手动触发一次绑定闪烁。
  // Why: 无需等待真实受击事件即可校验反馈视觉是否清晰。
  shapeStore.previewShapeBindingFlash(selectedShape.value.id)
}

function onEditorResetShapes(): void {
  shapeStore.resetShapes()
  editorMessage.value = '已清空图形图层'
}

async function onEditorImageFileChange(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  try {
    const imageAsset = await buildHudImageAssetFromFile(file)
    const metrics = resolveCanvasMetricsForImage()

    if (imageUploadIntent.value === 'replace' && selectedShape.value?.kind === 'image') {
      const current = selectedShape.value
      const nextSize = resolveHudImagePercentSize(
        imageAsset.naturalWidth,
        imageAsset.naturalHeight,
        metrics,
        current.w,
        current.h
      )
      const centerX = current.x + current.w / 2
      const centerY = current.y + current.h / 2
      shapeStore.updateShapeImageAsset(current.id, imageAsset)
      shapeStore.updateShapeTransform(current.id, {
        w: nextSize.w,
        h: nextSize.h,
        x: clampHudPercent(centerX - nextSize.w / 2, 0, Math.max(0, 100 - nextSize.w)),
        y: clampHudPercent(centerY - nextSize.h / 2, 0, Math.max(0, 100 - nextSize.h)),
      })
      editorMessage.value = `已替换图片：${imageAsset.name}`
      return
    }

    const nextSize = resolveHudImagePercentSize(imageAsset.naturalWidth, imageAsset.naturalHeight, metrics, 16, 16)
    const centeredRect = resolveCenteredImageRect(nextSize.w, nextSize.h)
    const createdId = shapeStore.addShape('image', {
      name: imageAsset.name || '图像素材',
      imageAsset,
      ...centeredRect,
    })
    if (!createdId) {
      editorMessage.value = '图形数量已到上限'
      return
    }
    shapeStore.enterEditMode()
    shapeStore.selectShape(createdId)
    editorMessage.value = `已插入图像：${imageAsset.name}`
  } catch (error) {
    const text = error instanceof Error ? error.message : '图片导入失败'
    editorMessage.value = `图片导入失败：${text}`
  } finally {
    input.value = ''
  }
}

function applySchemeSnapshot(payload: { widgets: HudLayoutSnapshot; shapes: HudShapeSnapshot; name: string }): void {
  // What: 统一应用方案快照到组件布局和图形图层。
  // Why: 让预设与文件导入走同一代码路径，避免状态分叉。
  layoutStore.importSnapshot(payload.widgets)
  shapeStore.importSnapshot(payload.shapes)
  editorMessage.value = `已应用方案：${payload.name}`
}

function onEditorExportScheme(): void {
  const layoutSnapshot = layoutStore.exportSnapshot()
  const shapeSnapshot = shapeStore.exportSnapshot()
  const scheme = buildHudDesignScheme(layoutSnapshot, shapeSnapshot, exportSchemeName.value)
  downloadHudDesignScheme(scheme)
  editorMessage.value = '方案已导出'
}

function onEditorImportButtonClick(): void {
  schemeFileRef.value?.click()
}

async function onEditorSchemeFileChange(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    const parsed = await parseHudDesignSchemeFile(file)
    applySchemeSnapshot(parsed)
  } catch (error) {
    const text = error instanceof Error ? error.message : '导入失败'
    editorMessage.value = `导入失败：${text}`
  } finally {
    input.value = ''
  }
}

function onEditorApplyPreset(): void {
  const preset = resolveHudPresetScheme(selectedPresetId.value)
  applySchemeSnapshot(preset)
}

function onToggleCollapsed(): void {
  collapsed.value = !collapsed.value
}

function onToggleSchemePanel(): void {
  schemeExpanded.value = !schemeExpanded.value
}

function onCloseEditor(): void {
  // What: 关闭编辑模式并清理当前图形选择。
  // Why: 退出后保持 HUD 纯展示态，避免残留选中描边干扰视图。
  shapeStore.enterEditMode()
  shapeStore.selectShape(null)
  shapeStore.setDrawMode(false)
}

function onDockPointerMove(event: PointerEvent): void {
  if (!dragging.value) return
  if (event.pointerId !== dragState.pointerId) return
  const nextX = dragState.startX + (event.clientX - dragState.startClientX)
  const nextY = dragState.startY + (event.clientY - dragState.startClientY)
  const clamped = clampDockPosition(nextX, nextY)
  dockX.value = clamped.x
  dockY.value = clamped.y
}

function stopDockDragging(): void {
  if (!dragging.value) return
  dragging.value = false
  dragState.pointerId = -1
  window.removeEventListener('pointermove', onDockPointerMove)
  window.removeEventListener('pointerup', onDockPointerUp)
  window.removeEventListener('pointercancel', onDockPointerUp)
  saveDockStorage()
}

function onDockPointerUp(event: PointerEvent): void {
  if (event.pointerId !== dragState.pointerId) return
  stopDockDragging()
}

function onDockHeaderPointerDown(event: PointerEvent): void {
  if (event.button !== 0) return
  event.preventDefault()
  // What: 使用标题栏作为拖拽手柄。
  // Why: 将可拖区域与内容编辑区域分离，避免误操作干扰画布编辑。
  dragging.value = true
  dragState.pointerId = event.pointerId
  dragState.startClientX = event.clientX
  dragState.startClientY = event.clientY
  dragState.startX = dockX.value
  dragState.startY = dockY.value
  window.addEventListener('pointermove', onDockPointerMove)
  window.addEventListener('pointerup', onDockPointerUp)
  window.addEventListener('pointercancel', onDockPointerUp)
}

function onWindowResize(): void {
  if (!shapeStore.isDrawMode) return
  void ensureDockPositionReady()
}

watch(
  () => shapeStore.isDrawMode,
  (enabled) => {
    if (!enabled) {
      stopDockDragging()
      return
    }
    // What: 进入编辑态时再计算小窗位置。
    // Why: v-if 渲染后才能拿到真实尺寸，避免初始定位抖动。
    void ensureDockPositionReady()
  }
)

watch([collapsed, schemeExpanded], () => {
  // What: 面板折叠态变化后重新落盘。
  // Why: 下次进入编辑态保持用户偏好的信息密度。
  void ensureDockPositionReady()
})

onMounted(() => {
  window.addEventListener('resize', onWindowResize)
  if (shapeStore.isDrawMode) {
    void ensureDockPositionReady()
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onWindowResize)
  stopDockDragging()
})
</script>

<style scoped>
.editor-dock {
  position: absolute;
  width: min(300px, calc(100% - 20px));
  max-height: min(72vh, 520px);
  z-index: var(--z-menu);
  pointer-events: auto;
  border-radius: 24px;
  border: 1px solid var(--hud-border-soft);
  background: linear-gradient(165deg, var(--hud-surface-1), var(--hud-surface-0));
  box-shadow: inset 0 0 0 1px rgba(34, 65, 118, 0.16), var(--panel-shadow);
  overflow: hidden;
}

.editor-dock.dragging {
  box-shadow: inset 0 0 0 1px rgba(60, 117, 235, 0.18), 0 20px 34px rgba(4, 10, 24, 0.4);
}

.dock-head {
  height: 44px;
  padding: 0 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--hud-border-soft);
  cursor: grab;
}

.dock-head:active {
  cursor: grabbing;
}

.dock-title {
  display: grid;
  gap: 2px;
}

.dock-title strong {
  font-size: 13px;
  color: var(--hud-text-primary);
  letter-spacing: 0.02em;
}

.dock-title small {
  font-size: 10px;
  color: var(--hud-text-secondary);
}

.dock-head-actions {
  display: inline-flex;
  gap: 6px;
}

.head-btn {
  height: 26px;
  border-radius: 999px;
  border: 1px solid var(--hud-border-soft);
  background: var(--hud-surface-2);
  color: var(--hud-text-primary);
  padding: 0 9px;
  font-size: 11px;
}

.head-btn.danger {
  border-color: var(--hud-danger-border);
  color: var(--hud-danger-text);
}

.dock-body {
  max-height: calc(min(72vh, 520px) - 44px);
  padding: 10px 10px 8px;
  display: grid;
  grid-template-columns: 1fr;
  gap: 8px;
  overflow: auto;
}

.dock-mode,
.dock-inline-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.mini-btn,
.tiny-btn,
.scheme-toggle,
select,
input,
button {
  font: inherit;
}

.mini-btn,
.tiny-btn,
.scheme-toggle {
  height: 27px;
  border-radius: 999px;
  border: 1px solid var(--hud-border-soft);
  background: var(--hud-surface-2);
  color: var(--hud-text-primary);
  padding: 0 10px;
  font-size: 11px;
}

.tiny-btn {
  height: 24px;
  padding: 0 8px;
}

.mini-btn.danger,
.tiny-btn.danger {
  border-color: var(--hud-danger-border);
  color: var(--hud-danger-text);
}

.mini-btn.active-add {
  border-color: var(--hud-draw-line-default);
  color: var(--hud-text-primary);
  box-shadow: 0 0 0 1px rgba(255, 107, 114, 0.18);
}

.dock-mode {
  align-items: center;
  justify-content: space-between;
}

.mode-pill {
  min-height: 27px;
  border-radius: 999px;
  border: 1px solid var(--hud-border-soft);
  background: var(--hud-surface-2);
  color: var(--hud-text-secondary);
  padding: 0 10px;
  display: inline-flex;
  align-items: center;
  font-size: 11px;
}

.mode-pill.add {
  border-color: var(--hud-draw-line-default);
  color: var(--hud-text-primary);
}

.dock-shape-list {
  max-height: 140px;
  overflow: auto;
  display: grid;
  gap: 6px;
}

.shape-item {
  border-radius: 14px;
  border: 1px solid var(--hud-border-soft);
  background: var(--hud-surface-2);
  padding: 7px;
  display: grid;
  gap: 6px;
  cursor: pointer;
}

.shape-item.active {
  border-color: var(--hud-border-focus);
  box-shadow: 0 0 0 1px var(--hud-focus-ring), inset 0 0 14px rgba(55, 120, 210, 0.08);
}

.shape-item header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.shape-item strong {
  font-size: 12px;
  color: var(--hud-text-primary);
}

.shape-item small {
  font-size: 10px;
  color: var(--hud-text-secondary);
}

.shape-item-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.dock-detail,
.dock-scheme,
.line-binding {
  border-radius: 14px;
  border: 1px solid var(--hud-border-soft);
  background: var(--hud-surface-2);
  padding: 8px;
  display: grid;
  gap: 8px;
}

.line-binding h4 {
  margin: 0;
  font-size: 12px;
  color: var(--text-strong);
}

.dock-empty {
  border-radius: 12px;
  border: 1px dashed var(--panel-border);
  color: var(--text-muted);
  padding: 8px;
  font-size: 11px;
  text-align: center;
}

.field {
  display: grid;
  gap: 4px;
}

.field > span {
  font-size: 11px;
  color: var(--text-muted);
}

.field-hint {
  font-size: 10px;
  color: var(--text-muted);
}

.detail-caption {
  font-size: 11px;
  color: var(--text-strong);
  padding-bottom: 2px;
}

.field-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.field input[type='text'],
.field input[type='number'],
.field select {
  height: 30px;
  border-radius: 10px;
  border: 1px solid var(--panel-border);
  background: rgba(5, 12, 24, 0.72);
  color: var(--text-strong);
  padding: 0 8px;
}

.field input[type='range'] {
  accent-color: rgba(126, 180, 255, 0.94);
}

.check-field {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-strong);
}

.scheme-file-input {
  display: none;
}

.dock-message {
  min-height: 14px;
  font-size: 10px;
  color: var(--text-muted);
}

.image-asset {
  display: grid;
  gap: 8px;
}

.image-caption {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.image-caption small {
  color: var(--text-muted);
  font-family: var(--font-data);
}

@media (max-width: 960px) {
  .editor-dock {
    width: min(280px, calc(100% - 16px));
    max-height: min(74vh, 500px);
    border-radius: 20px;
  }
}
</style>
