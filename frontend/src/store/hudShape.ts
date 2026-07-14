import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type {
  ArmorHitEventPayload,
  HudEditorTool,
  HudImageAsset,
  HudShapeEntity,
  HudShapeKind,
  HudShapeSnapshot,
  HudShapeStyle,
} from '../types/hudShape'
import {
  createDefaultHudLineBinding,
  createDefaultHudShapeEntity,
  createDefaultHudShapeSnapshot,
  createHudShapeId,
  HUD_SHAPE_LIMIT,
  HUD_SHAPE_STORAGE_KEY,
  HUD_SHAPE_VERSION,
  normalizeHudShapeEntity,
  normalizeHudShapeList,
} from '../types/hudShape'

interface ShapeFlashState {
  activeUntil: number
  color: string
  widthBoost: number
}

function safeNow(): number {
  return Date.now()
}

function normalizeZone(raw: unknown): 'front' | 'back' | 'left' | 'right' | null {
  if (raw === 'front' || raw === 'back' || raw === 'left' || raw === 'right') return raw
  return null
}

function loadShapeSnapshot(): HudShapeSnapshot {
  if (typeof window === 'undefined') return createDefaultHudShapeSnapshot()
  const raw = window.localStorage.getItem(HUD_SHAPE_STORAGE_KEY)
  if (!raw) return createDefaultHudShapeSnapshot()
  try {
    const parsed = JSON.parse(raw) as Partial<HudShapeSnapshot>
    if (parsed.version !== HUD_SHAPE_VERSION) return createDefaultHudShapeSnapshot()
    return {
      version: HUD_SHAPE_VERSION,
      shapes: normalizeHudShapeList(Array.isArray(parsed.shapes) ? parsed.shapes : []),
    }
  } catch {
    return createDefaultHudShapeSnapshot()
  }
}

function saveShapeSnapshot(shapes: HudShapeEntity[]): void {
  if (typeof window === 'undefined') return
  const payload: HudShapeSnapshot = {
    version: HUD_SHAPE_VERSION,
    shapes,
  }
  window.localStorage.setItem(HUD_SHAPE_STORAGE_KEY, JSON.stringify(payload))
}

function normalizeShapeZOrder(source: HudShapeEntity[]): HudShapeEntity[] {
  // What: 统一将图形层级压缩为连续且唯一的 z 序列。
  // Why: 既保证导入与复制后的排序稳定，也避免删除“上移/下移”后残留脏层级造成列表抖动。
  return [...source]
    .sort((a, b) => {
      if (a.z !== b.z) return a.z - b.z
      return a.id.localeCompare(b.id)
    })
    .map((item, index) => ({
      ...item,
      z: index + 1,
    }))
}

export const useHudShapeStore = defineStore('hud-shape', () => {
  const tool = ref<HudEditorTool>('select')
  const isDrawMode = ref(false)
  const shapes = ref<HudShapeEntity[]>([])
  const selectedShapeId = ref<string | null>(null)
  const hydrated = ref(false)
  const flashStateMap = ref<Record<string, ShapeFlashState>>({})

  const flashTimeoutMap = new Map<string, number>()
  const flashCooldownMap = new Map<string, number>()

  const orderedShapes = computed(() => {
    return [...shapes.value].sort((a, b) => a.z - b.z)
  })

  const selectedShape = computed(() => {
    if (!selectedShapeId.value) return null
    return shapes.value.find((item) => item.id === selectedShapeId.value) ?? null
  })

  const canCreateNewShape = computed(() => shapes.value.length < HUD_SHAPE_LIMIT)

  function clearShapeFlashTimer(shapeId: string): void {
    const timer = flashTimeoutMap.get(shapeId)
    if (typeof timer === 'number') {
      window.clearTimeout(timer)
      flashTimeoutMap.delete(shapeId)
    }
  }

  function clearAllFlashTimers(): void {
    for (const id of flashTimeoutMap.keys()) {
      clearShapeFlashTimer(id)
    }
  }

  function sanitizeShapes(nextShapes: Array<Partial<HudShapeEntity> | null | undefined>): HudShapeEntity[] {
    // What: 统一图形列表归一化入口。
    // Why: 所有写入路径共享同一套校验与排序，避免导入/编辑行为产生不一致数据。
    return normalizeShapeZOrder(normalizeHudShapeList(nextShapes))
  }

  function commitShapes(nextShapes: Array<Partial<HudShapeEntity> | null | undefined>): void {
    const normalized = sanitizeShapes(nextShapes)
    shapes.value = normalized
    if (selectedShapeId.value && !normalized.some((item) => item.id === selectedShapeId.value)) {
      selectedShapeId.value = null
    }
    saveShapeSnapshot(normalized)
  }

  function hydrate(): void {
    if (hydrated.value) return
    const snapshot = loadShapeSnapshot()
    shapes.value = normalizeShapeZOrder(snapshot.shapes)
    hydrated.value = true
  }

  function setTool(nextTool: HudEditorTool): void {
    tool.value = nextTool
  }

  function enterAddMode(kind: HudShapeKind): void {
    // What: 将编辑器切换到“单次添加”工具。
    // Why: 将添加与编辑解耦，避免在编辑态误触连续创建图形。
    setTool(kind)
  }

  function enterEditMode(): void {
    // What: 强制回到选择工具。
    // Why: 退出绘制或完成一次添加后，需要立即恢复可选中/可移动的稳定编辑态。
    setTool('select')
  }

  function setDrawMode(value: boolean): void {
    // What: 独立控制 UI 绘制模式开关。
    // Why: 将“图形绘制”与“组件布局编辑”拆分，避免状态混用导致交互干扰。
    isDrawMode.value = value
    if (!value) {
      // What: 退出 UI 绘制时重置为编辑工具。
      // Why: 防止下次进入仍停留在线条工具，出现十字光标残留和误绘制。
      enterEditMode()
    }
  }

  function toggleDrawMode(): void {
    setDrawMode(!isDrawMode.value)
  }

  function selectShape(shapeId: string | null): void {
    if (!shapeId) {
      selectedShapeId.value = null
      return
    }
    if (!shapes.value.some((item) => item.id === shapeId)) {
      selectedShapeId.value = null
      return
    }
    selectedShapeId.value = shapeId
  }

  function addShape(kind: HudShapeKind, partial?: Partial<HudShapeEntity>): string {
    if (!canCreateNewShape.value) return ''
    const base = createDefaultHudShapeEntity(kind)
    const maxZ = shapes.value.reduce((max, item) => Math.max(max, item.z), 0)
    const next = normalizeHudShapeEntity(
      {
        ...base,
        ...partial,
        id: createHudShapeId(),
        z: maxZ + 1,
      },
      base
    )
    commitShapes([...shapes.value, next])
    selectedShapeId.value = next.id
    return next.id
  }

  function updateShape(shapeId: string, patch: Partial<HudShapeEntity>): void {
    const next = shapes.value.map((item) => {
      if (item.id !== shapeId) return item
      return normalizeHudShapeEntity({ ...item, ...patch }, item)
    })
    commitShapes(next)
  }

  function updateShapeTransform(
    shapeId: string,
    patch: Partial<Pick<HudShapeEntity, 'x' | 'y' | 'w' | 'h' | 'rotationDeg'>>
  ): void {
    // What: 单独更新图形几何变换。
    // Why: 减少样式和绑定字段被误覆盖，保证拖拽与属性编辑路径稳定。
    updateShape(shapeId, patch)
  }

  function updateShapeStyle(shapeId: string, patch: Partial<HudShapeStyle>): void {
    const target = shapes.value.find((item) => item.id === shapeId)
    if (!target) return
    updateShape(shapeId, {
      style: {
        ...target.style,
        ...patch,
      },
    })
  }

  function updateShapeImageAsset(shapeId: string, imageAsset: HudImageAsset): void {
    const target = shapes.value.find((item) => item.id === shapeId)
    if (!target || target.kind !== 'image') return
    // What: 单独更新图片资源。
    // Why: 替换图片时只改素材与必要字段，避免误伤其他几何配置。
    updateShape(shapeId, {
      imageAsset,
      name: imageAsset.name || target.name,
    })
  }

  function updateLineBinding(shapeId: string, patch: Partial<NonNullable<HudShapeEntity['lineBinding']>>): void {
    const target = shapes.value.find((item) => item.id === shapeId)
    if (!target || target.kind !== 'line') return
    const prev = target.lineBinding ?? createDefaultHudLineBinding()
    updateShape(shapeId, {
      lineBinding: {
        ...prev,
        ...patch,
      },
    })
  }

  function setShapeVisible(shapeId: string, visible: boolean): void {
    updateShape(shapeId, { visible })
  }

  function setShapeLocked(shapeId: string, locked: boolean): void {
    updateShape(shapeId, { locked })
  }

  function duplicateShape(shapeId: string): void {
    if (!canCreateNewShape.value) return
    const target = shapes.value.find((item) => item.id === shapeId)
    if (!target) return
    const maxZ = shapes.value.reduce((max, item) => Math.max(max, item.z), 0)
    const duplicate = normalizeHudShapeEntity(
      {
        ...target,
        id: createHudShapeId(),
        name: `${target.name}-副本`,
        x: target.x + 1.2,
        y: target.y + 1.2,
        z: maxZ + 1,
      },
      target
    )
    commitShapes([...shapes.value, duplicate])
    selectedShapeId.value = duplicate.id
  }

  function removeShape(shapeId: string): void {
    clearShapeFlashTimer(shapeId)
    flashCooldownMap.delete(shapeId)
    const next = shapes.value.filter((item) => item.id !== shapeId)
    commitShapes(next)
  }

  function resetShapes(): void {
    // What: 一键清空图形图层。
    // Why: 复杂编辑后需要快速回到纯 HUD 组件布局，减少现场恢复成本。
    clearAllFlashTimers()
    flashCooldownMap.clear()
    flashStateMap.value = {}
    selectedShapeId.value = null
    commitShapes([])
  }

  function replaceShapes(nextShapes: Array<Partial<HudShapeEntity> | null | undefined>): void {
    clearAllFlashTimers()
    flashCooldownMap.clear()
    flashStateMap.value = {}
    commitShapes(nextShapes)
  }

  function exportSnapshot(): HudShapeSnapshot {
    return {
      version: HUD_SHAPE_VERSION,
      shapes: sanitizeShapes(shapes.value),
    }
  }

  function importSnapshot(snapshot: Partial<HudShapeSnapshot>): void {
    // What: 从外部快照整体导入图形图层。
    // Why: 方案切换要求一次性替换，保证布局与绑定状态一致。
    if (snapshot.version !== HUD_SHAPE_VERSION) {
      replaceShapes([])
      return
    }
    const rawShapes = Array.isArray(snapshot.shapes) ? snapshot.shapes : []
    replaceShapes(rawShapes)
  }

  function clearShapeFlash(shapeId: string): void {
    const next = { ...flashStateMap.value }
    delete next[shapeId]
    flashStateMap.value = next
  }

  function triggerShapeFlash(shapeId: string, color: string, widthBoost: number, durationMs: number): void {
    const now = safeNow()
    const activeUntil = now + Math.max(1, durationMs)
    clearShapeFlashTimer(shapeId)
    flashStateMap.value = {
      ...flashStateMap.value,
      [shapeId]: {
        activeUntil,
        color,
        widthBoost,
      },
    }

    const timeoutId = window.setTimeout(() => {
      clearShapeFlash(shapeId)
      flashTimeoutMap.delete(shapeId)
    }, Math.max(1, durationMs))
    flashTimeoutMap.set(shapeId, timeoutId)
  }

  function previewShapeBindingFlash(shapeId: string): void {
    const target = shapes.value.find((item) => item.id === shapeId)
    if (!target || target.kind !== 'line') return
    const binding = target.lineBinding
    if (!binding) return
    triggerShapeFlash(shapeId, binding.flashColor, binding.flashWidthBoost, binding.flashMs)
  }

  function triggerArmorHit(payload: ArmorHitEventPayload): void {
    const zone = normalizeZone(String(payload.zone || payload.param || '').trim().toLowerCase())
    if (!zone) return
    const eventId = Math.max(0, Math.floor(Number(payload.event_id) || 0))
    const now = safeNow()

    const nextTargetList = shapes.value.filter((item) => {
      if (item.kind !== 'line') return false
      const binding = item.lineBinding
      if (!binding || !binding.enabled) return false
      if (binding.eventType !== 'armor-hit') return false
      if (binding.eventId !== eventId) return false
      if (binding.zone !== 'any' && binding.zone !== zone) return false
      const lastAt = flashCooldownMap.get(item.id) ?? 0
      return now - lastAt >= binding.cooldownMs
    })

    // What: 命中后触发线条高亮脉冲。
    // Why: 让用户在局部绑定元素上获得即时反馈，而不影响其他图层稳定性。
    nextTargetList.forEach((shape) => {
      const binding = shape.lineBinding
      if (!binding) return
      flashCooldownMap.set(shape.id, now)
      triggerShapeFlash(shape.id, binding.flashColor, binding.flashWidthBoost, binding.flashMs)
    })
  }

  function getShapeFlashState(shapeId: string): ShapeFlashState | null {
    const target = flashStateMap.value[shapeId]
    if (!target) return null
    if (target.activeUntil <= safeNow()) {
      clearShapeFlash(shapeId)
      return null
    }
    return target
  }

  return {
    tool,
    isDrawMode,
    shapes,
    orderedShapes,
    selectedShapeId,
    selectedShape,
    hydrated,
    canCreateNewShape,
    hydrate,
    setTool,
    enterAddMode,
    enterEditMode,
    setDrawMode,
    toggleDrawMode,
    selectShape,
    addShape,
    updateShape,
    updateShapeTransform,
    updateShapeStyle,
    updateShapeImageAsset,
    updateLineBinding,
    setShapeVisible,
    setShapeLocked,
    duplicateShape,
    removeShape,
    resetShapes,
    replaceShapes,
    exportSnapshot,
    importSnapshot,
    triggerArmorHit,
    previewShapeBindingFlash,
    getShapeFlashState,
  }
})
