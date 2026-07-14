import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { HudLayoutMap, HudLayoutSnapshot, HudWidgetId, HudWidgetRect } from '../types/hudLayout'
import {
  createDefaultHudLayoutMap,
  HUD_LAYOUT_STORAGE_KEY,
  HUD_LAYOUT_VERSION,
  HUD_WIDGET_IDS,
  HUD_WIDGET_SCALE_MAX,
  HUD_WIDGET_SCALE_MIN,
} from '../types/hudLayout'

const LEGACY_V6_LAYOUT_STORAGE_KEY = 'rm-client-hud-layout:v6'
const LEGACY_V6_LAYOUT_VERSION = 6
const LEGACY_V6_LEFT_MODULES_RECT: HudWidgetRect = {
  x: 0.9,
  y: 11.8,
  w: 15.1,
  h: 16.7,
  scale: 1,
  z: 20,
  visible: true,
  locked: false,
}
const LEGACY_V6_LEFT_ROBOT_RECT: HudWidgetRect = {
  x: 0.9,
  y: 29.6,
  w: 15.1,
  h: 21.5,
  scale: 1,
  z: 20,
  visible: true,
  locked: false,
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

function normalizeRect(rect: Partial<HudWidgetRect> | null | undefined, fallback: HudWidgetRect): HudWidgetRect {
  const baseW = clamp(Number(rect?.w ?? fallback.w) || 10, 4, 100)
  const baseH = clamp(Number(rect?.h ?? fallback.h) || 10, 4, 100)
  const scaleUpperBound = Math.min(HUD_WIDGET_SCALE_MAX, 100 / baseW, 100 / baseH)
  const scale = clamp(
    Number(rect?.scale ?? fallback.scale) || 1,
    HUD_WIDGET_SCALE_MIN,
    Math.max(HUD_WIDGET_SCALE_MIN, scaleUpperBound)
  )

  const finalW = baseW * scale
  const finalH = baseH * scale
  const x = clamp(Number(rect?.x ?? fallback.x) || 0, 0, 100 - finalW)
  const y = clamp(Number(rect?.y ?? fallback.y) || 0, 0, 100 - finalH)

  return {
    x,
    y,
    w: baseW,
    h: baseH,
    scale,
    z: Math.max(0, Math.floor(Number(rect?.z ?? fallback.z) || 10)),
    visible: typeof rect?.visible === 'boolean' ? rect.visible : fallback.visible,
    locked: typeof rect?.locked === 'boolean' ? rect.locked : fallback.locked,
  }
}

function normalizeMap(input: Partial<HudLayoutMap> | null | undefined): HudLayoutMap {
  const defaults = createDefaultHudLayoutMap()
  const normalized = {} as HudLayoutMap
  for (const widgetId of HUD_WIDGET_IDS) {
    // What: 逐个组件做字段归一化与兜底。
    // Why: 兼容旧版本快照缺字段（如 locked）时也能稳定恢复布局。
    normalized[widgetId] = normalizeRect(input?.[widgetId], defaults[widgetId])
  }
  return normalized
}

function isNear(value: number, target: number, epsilon = 0.01): boolean {
  return Math.abs(value - target) <= epsilon
}

function migrateLegacyCrosshairRect(map: HudLayoutMap): HudLayoutMap {
  const current = map.center_crosshair
  if (
    !isNear(current.x, 37.6) ||
    !isNear(current.y, 42.2) ||
    !isNear(current.w, 24.8) ||
    !isNear(current.h, 24.8) ||
    !isNear(current.scale, 1)
  ) {
    return map
  }
  const defaults = createDefaultHudLayoutMap()
  return {
    ...map,
    center_crosshair: {
      ...current,
      x: defaults.center_crosshair.x,
      y: defaults.center_crosshair.y,
      w: defaults.center_crosshair.w,
      h: defaults.center_crosshair.h,
      scale: defaults.center_crosshair.scale,
    },
  }
}

function isSameRect(current: HudWidgetRect, target: HudWidgetRect): boolean {
  return (
    isNear(current.x, target.x) &&
    isNear(current.y, target.y) &&
    isNear(current.w, target.w) &&
    isNear(current.h, target.h) &&
    isNear(current.scale, target.scale) &&
    current.visible === target.visible &&
    current.locked === target.locked
  )
}

function migrateLegacyCardRects(map: HudLayoutMap): HudLayoutMap {
  const defaults = createDefaultHudLayoutMap()
  const nextMap = { ...map }

  // What: 仅迁移仍停留在 v6 默认尺寸的左侧两张卡。
  // Why: 这些默认值会直接导致内容裁切，而用户手调过的位置和缩放绝不能被这次修复覆盖。
  if (isSameRect(map.left_modules, LEGACY_V6_LEFT_MODULES_RECT)) {
    nextMap.left_modules = {
      ...map.left_modules,
      x: defaults.left_modules.x,
      y: defaults.left_modules.y,
      w: defaults.left_modules.w,
      h: defaults.left_modules.h,
      scale: defaults.left_modules.scale,
    }
  }

  if (isSameRect(map.left_robot, LEGACY_V6_LEFT_ROBOT_RECT)) {
    nextMap.left_robot = {
      ...map.left_robot,
      x: defaults.left_robot.x,
      y: defaults.left_robot.y,
      w: defaults.left_robot.w,
      h: defaults.left_robot.h,
      scale: defaults.left_robot.scale,
    }
  }

  return nextMap
}

function saveSnapshot(map: HudLayoutMap): void {
  if (typeof window === 'undefined') return

  const payload: HudLayoutSnapshot = {
    version: HUD_LAYOUT_VERSION,
    widgets: map,
  }

  window.localStorage.setItem(HUD_LAYOUT_STORAGE_KEY, JSON.stringify(payload))
}

function loadSnapshot(): HudLayoutMap {
  if (typeof window === 'undefined') return createDefaultHudLayoutMap()

  const raw = window.localStorage.getItem(HUD_LAYOUT_STORAGE_KEY)
  const legacyRaw = window.localStorage.getItem(LEGACY_V6_LAYOUT_STORAGE_KEY)
  const source = raw ?? legacyRaw
  if (!source) return createDefaultHudLayoutMap()

  try {
    const parsed = JSON.parse(source) as Partial<HudLayoutSnapshot>
    const isCurrentSnapshot = parsed.version === HUD_LAYOUT_VERSION
    const isLegacySnapshot = parsed.version === LEGACY_V6_LAYOUT_VERSION
    if (!isCurrentSnapshot && !isLegacySnapshot) return createDefaultHudLayoutMap()
    // What: 先兼容旧快照，再做定向布局修复。
    // Why: 既要保住用户已有的手调结果，也要把当前默认过小的卡片自动拉回可读尺寸。
    return migrateLegacyCardRects(migrateLegacyCrosshairRect(normalizeMap(parsed.widgets)))
  } catch {
    return createDefaultHudLayoutMap()
  }
}

export const useHudLayoutStore = defineStore('hud-layout', () => {
  const isEditMode = ref(false)
  const widgets = ref<HudLayoutMap>(createDefaultHudLayoutMap())
  const hydrated = ref(false)

  const orderedWidgetIds = computed(() => {
    return (Object.keys(widgets.value) as HudWidgetId[]).sort((a, b) => widgets.value[a].z - widgets.value[b].z)
  })

  // What: 首次加载时恢复本地布局。Why: 用户拖拽/缩放配置跨重启可复用，减少重复调位成本。
  function hydrate(): void {
    if (hydrated.value) return
    widgets.value = loadSnapshot()
    // What: 冷启动后立即把归一化后的布局写回当前版本快照。
    // Why: 这样 v6 -> v7 的智能迁移只发生一次，后续启动都直接吃修复后的布局。
    saveSnapshot(widgets.value)
    hydrated.value = true
  }

  function setEditMode(value: boolean): void {
    isEditMode.value = value
  }

  function toggleEditMode(): void {
    isEditMode.value = !isEditMode.value
  }

  function commitWidgets(nextMap: Partial<HudLayoutMap>): void {
    // What: 统一提交布局并落盘。
    // Why: 避免不同入口各自保存导致快照时序不一致。
    widgets.value = normalizeMap(nextMap)
    saveSnapshot(widgets.value)
  }

  // What: 更新组件位置信息并持久化。Why: 编辑态每次变更都要可回放，避免崩溃后状态丢失。
  function updateWidgetRect(widgetId: HudWidgetId, patch: Partial<HudWidgetRect>): void {
    const prev = widgets.value[widgetId]
    const next = normalizeRect({ ...prev, ...patch }, prev)
    commitWidgets({
      ...widgets.value,
      [widgetId]: next,
    })
  }

  // What: 以组件中心为锚点应用目标缩放值。Why: 缩放时保持视觉中心稳定，避免组件“漂移”影响精调。
  function applyWidgetScale(widgetId: HudWidgetId, nextScaleRaw: number): void {
    const prev = widgets.value[widgetId]
    const oldFinalW = prev.w * prev.scale
    const oldFinalH = prev.h * prev.scale
    const centerX = prev.x + oldFinalW / 2
    const centerY = prev.y + oldFinalH / 2

    const scaleUpperBound = Math.min(HUD_WIDGET_SCALE_MAX, 100 / prev.w, 100 / prev.h)
    const nextScale = clamp(nextScaleRaw, HUD_WIDGET_SCALE_MIN, Math.max(HUD_WIDGET_SCALE_MIN, scaleUpperBound))
    const nextFinalW = prev.w * nextScale
    const nextFinalH = prev.h * nextScale
    const nextX = centerX - nextFinalW / 2
    const nextY = centerY - nextFinalH / 2

    updateWidgetRect(widgetId, {
      scale: nextScale,
      x: nextX,
      y: nextY,
    })
  }

  // What: 以组件中心为锚点执行缩放。Why: 缩放过程更直观，不会出现单侧“漂移”导致难以微调。
  function updateWidgetScale(widgetId: HudWidgetId, delta: number): void {
    const prev = widgets.value[widgetId]
    applyWidgetScale(widgetId, prev.scale + delta)
  }

  function setWidgetScale(widgetId: HudWidgetId, nextScale: number): void {
    // What: 按目标值设置组件缩放。
    // Why: 设置页使用预设档位时需要直接命中期望比例而非增量叠加。
    applyWidgetScale(widgetId, nextScale)
  }

  function setWidgetVisible(widgetId: HudWidgetId, visible: boolean): void {
    updateWidgetRect(widgetId, { visible })
  }

  function setWidgetLocked(widgetId: HudWidgetId, locked: boolean): void {
    // What: 切换组件锁定状态。
    // Why: 提供编辑保护，避免关键组件被误拖拽或误缩放。
    updateWidgetRect(widgetId, { locked })
  }

  function resetWidget(widgetId: HudWidgetId): void {
    const defaults = createDefaultHudLayoutMap()
    // What: 单组件恢复默认布局参数。
    // Why: 避免用户只改坏一个组件时必须全局重置。
    updateWidgetRect(widgetId, defaults[widgetId])
  }

  function resetLayout(): void {
    // What: 全量重置布局到官方基线。Why: 复杂调整后提供一次性回退路径。
    commitWidgets(createDefaultHudLayoutMap())
  }

  function exportSnapshot(): HudLayoutSnapshot {
    // What: 导出当前组件布局快照。
    // Why: 方案导出需要与图形图层一起打包，保证跨设备还原一致。
    return {
      version: HUD_LAYOUT_VERSION,
      widgets: normalizeMap(widgets.value),
    }
  }

  function importSnapshot(snapshot: Partial<HudLayoutSnapshot>): void {
    // What: 从外部快照导入组件布局。
    // Why: 预设方案与文件导入都要求一次性替换当前布局。
    if (snapshot.version !== HUD_LAYOUT_VERSION) {
      commitWidgets(createDefaultHudLayoutMap())
      return
    }
    const incoming = snapshot.widgets
    commitWidgets(incoming ?? createDefaultHudLayoutMap())
  }

  return {
    isEditMode,
    widgets,
    hydrated,
    orderedWidgetIds,
    hydrate,
    setEditMode,
    toggleEditMode,
    updateWidgetRect,
    updateWidgetScale,
    setWidgetScale,
    setWidgetVisible,
    setWidgetLocked,
    resetWidget,
    resetLayout,
    exportSnapshot,
    importSnapshot,
  }
})
