export type HudEditorTool = 'select' | 'line' | 'rect' | 'circle' | 'image'
export type HudShapeKind = 'line' | 'rect' | 'circle' | 'image'
export type ArmorZone = 'front' | 'back' | 'left' | 'right'

export interface HudShapeStyle {
  strokeColor: string
  strokeWidth: number
  strokeOpacity: number
  fillColor: string
  fillOpacity: number
}

export interface HudLineBinding {
  enabled: boolean
  eventType: 'armor-hit'
  eventId: number
  zone: ArmorZone | 'any'
  flashMs: number
  flashColor: string
  flashWidthBoost: number
  cooldownMs: number
}

export interface HudImageAsset {
  name: string
  mimeType: string
  src: string
  naturalWidth: number
  naturalHeight: number
}

export interface HudShapeEntity {
  id: string
  kind: HudShapeKind
  name: string
  x: number
  y: number
  w: number
  h: number
  rotationDeg: number
  z: number
  visible: boolean
  locked: boolean
  style: HudShapeStyle
  lineBinding?: HudLineBinding
  imageAsset?: HudImageAsset
}

export interface HudShapeSnapshot {
  version: number
  shapes: HudShapeEntity[]
}

export interface ArmorHitEventPayload {
  event_id: number
  zone: string
  param: string
  timestamp: number
}

export const HUD_SHAPE_VERSION = 1
export const HUD_SHAPE_STORAGE_KEY = `rm-client-hud-shape:v${HUD_SHAPE_VERSION}`
export const HUD_SHAPE_LIMIT = 120

// What: 收敛 UI 线条粗细范围到细线区间。
// Why: HUD 画布常用于装甲板反馈与结构标注，过粗线条会遮挡战场信息。
export const HUD_STROKE_WIDTH_MIN = 0.3
export const HUD_STROKE_WIDTH_MAX = 1.6

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

function normalizeColor(raw: unknown, fallback: string): string {
  return typeof raw === 'string' && raw.trim() ? raw : fallback
}

function normalizeZone(raw: unknown): ArmorZone | 'any' {
  if (raw === 'front' || raw === 'back' || raw === 'left' || raw === 'right' || raw === 'any') return raw
  return 'any'
}

function normalizeImageMimeType(raw: unknown, fallback: string): string {
  const value = typeof raw === 'string' ? raw.trim().toLowerCase() : ''
  if (
    value === 'image/png' ||
    value === 'image/jpeg' ||
    value === 'image/webp' ||
    value === 'image/svg+xml'
  ) {
    return value
  }
  return fallback
}

// What: 统一图片资源默认值。
// Why: 导入坏数据或用户取消替换时，图片图层仍能保持结构完整，避免渲染崩溃。
export function createDefaultHudImageAsset(): HudImageAsset {
  return {
    name: '图像素材',
    mimeType: 'image/png',
    src: '',
    naturalWidth: 1,
    naturalHeight: 1,
  }
}

// What: 统一图形基础样式默认值。
// Why: 让新建图形和异常数据回退时保持一致视觉基线，避免随机样式污染 HUD。
export function createDefaultHudShapeStyle(): HudShapeStyle {
  return {
    // What: 新建图形默认使用细红线。
    // Why: 与用户常用瞄准/标注习惯一致，首次落笔就能得到清晰但不遮挡画面的线条。
    strokeColor: '#FF6B72',
    strokeWidth: 0.4,
    strokeOpacity: 0.96,
    fillColor: '#FF6B72',
    fillOpacity: 0,
  }
}

// What: 统一线条绑定默认值。
// Why: 保证“装甲板受击高亮”功能开关明确，避免未配置时误触发。
export function createDefaultHudLineBinding(): HudLineBinding {
  return {
    enabled: false,
    eventType: 'armor-hit',
    eventId: 0,
    zone: 'any',
    flashMs: 300,
    flashColor: '#FFFFFF',
    flashWidthBoost: 2,
    cooldownMs: 120,
  }
}

// What: 规范化图片资源字段。
// Why: 导入方案和本地快照都可能缺字段，统一兜底可避免空地址或非法尺寸污染画布。
export function normalizeHudImageAsset(
  raw: Partial<HudImageAsset> | null | undefined,
  fallback: HudImageAsset = createDefaultHudImageAsset()
): HudImageAsset {
  const resolvedFallback = fallback ?? createDefaultHudImageAsset()
  return {
    name: typeof raw?.name === 'string' && raw.name.trim() ? raw.name : resolvedFallback.name,
    mimeType: normalizeImageMimeType(raw?.mimeType, resolvedFallback.mimeType),
    src: typeof raw?.src === 'string' && raw.src.startsWith('data:image/') ? raw.src : resolvedFallback.src,
    naturalWidth: Math.max(1, Math.round(Number(raw?.naturalWidth ?? resolvedFallback.naturalWidth) || resolvedFallback.naturalWidth)),
    naturalHeight: Math.max(1, Math.round(Number(raw?.naturalHeight ?? resolvedFallback.naturalHeight) || resolvedFallback.naturalHeight)),
  }
}

// What: 规范化图形样式字段。
// Why: 导入方案或历史快照缺字段时，避免渲染层出现 NaN 和非法透明度。
export function normalizeHudShapeStyle(raw: Partial<HudShapeStyle> | null | undefined): HudShapeStyle {
  const defaults = createDefaultHudShapeStyle()
  return {
    strokeColor: normalizeColor(raw?.strokeColor, defaults.strokeColor),
    strokeWidth: clamp(Number(raw?.strokeWidth ?? defaults.strokeWidth) || defaults.strokeWidth, HUD_STROKE_WIDTH_MIN, HUD_STROKE_WIDTH_MAX),
    strokeOpacity: clamp(Number(raw?.strokeOpacity ?? defaults.strokeOpacity) || defaults.strokeOpacity, 0, 1),
    fillColor: normalizeColor(raw?.fillColor, defaults.fillColor),
    fillOpacity: clamp(Number(raw?.fillOpacity ?? defaults.fillOpacity) || defaults.fillOpacity, 0, 1),
  }
}

// What: 规范化线条绑定字段。
// Why: 绑定配置来自文件导入和面板输入，统一校验可避免运行时触发异常。
export function normalizeHudLineBinding(raw: Partial<HudLineBinding> | null | undefined): HudLineBinding {
  const defaults = createDefaultHudLineBinding()
  return {
    enabled: typeof raw?.enabled === 'boolean' ? raw.enabled : defaults.enabled,
    eventType: 'armor-hit',
    eventId: Math.max(0, Math.floor(Number(raw?.eventId ?? defaults.eventId) || defaults.eventId)),
    zone: normalizeZone(raw?.zone),
    flashMs: clamp(Math.floor(Number(raw?.flashMs ?? defaults.flashMs) || defaults.flashMs), 80, 1200),
    flashColor: normalizeColor(raw?.flashColor, defaults.flashColor),
    flashWidthBoost: clamp(Number(raw?.flashWidthBoost ?? defaults.flashWidthBoost) || defaults.flashWidthBoost, 0.2, 6),
    cooldownMs: clamp(Math.floor(Number(raw?.cooldownMs ?? defaults.cooldownMs) || defaults.cooldownMs), 0, 2000),
  }
}

function normalizeKind(raw: unknown, fallback: HudShapeKind): HudShapeKind {
  if (raw === 'line' || raw === 'rect' || raw === 'circle' || raw === 'image') return raw
  return fallback
}

// What: 创建新图形 id。
// Why: 需要在本地编辑和导入覆盖中保持稳定唯一 key，避免渲染复用错位。
export function createHudShapeId(): string {
  const rand = Math.random().toString(36).slice(2, 10)
  return `shape_${Date.now()}_${rand}`
}

// What: 生成指定图形的默认实体。
// Why: 工具栏创建动作必须始终得到可渲染对象，避免空对象进入 store。
export function createDefaultHudShapeEntity(kind: HudShapeKind): HudShapeEntity {
  const base: HudShapeEntity = {
    id: createHudShapeId(),
    kind,
    name: kind === 'line' ? '新线条' : kind === 'rect' ? '新矩形' : kind === 'circle' ? '新圆形' : '新图像',
    x: 45,
    y: 45,
    w: kind === 'line' ? 5.6 : kind === 'image' ? 12 : 8,
    h: kind === 'line' ? 0.4 : kind === 'image' ? 12 : 8,
    rotationDeg: 0,
    z: 30,
    visible: true,
    locked: false,
    style: createDefaultHudShapeStyle(),
  }
  if (kind === 'line') {
    base.lineBinding = createDefaultHudLineBinding()
  }
  if (kind === 'image') {
    base.imageAsset = createDefaultHudImageAsset()
  }
  return base
}

// What: 规范化单个图形实体。
// Why: 统一限制坐标、尺寸和层级范围，避免图形越界或层级异常导致不可编辑。
export function normalizeHudShapeEntity(
  raw: Partial<HudShapeEntity> | null | undefined,
  fallback: HudShapeEntity
): HudShapeEntity {
  const kind = normalizeKind(raw?.kind, fallback.kind)
  const style = normalizeHudShapeStyle(raw?.style)
  // What: 线条单独允许更小尺寸下限。
  // Why: 细线 HUD 标注需要低于 1% 的尺寸精度，避免被通用矩形最小值强行放大。
  const minSize = kind === 'line' ? 0.4 : 1
  const width = clamp(Number(raw?.w ?? fallback.w) || fallback.w, minSize, 100)
  const height = clamp(Number(raw?.h ?? fallback.h) || fallback.h, minSize, 100)
  const x = clamp(Number(raw?.x ?? fallback.x) || fallback.x, 0, kind === 'line' ? 100 : 100 - width)
  const y = clamp(Number(raw?.y ?? fallback.y) || fallback.y, 0, 100 - height)
  const normalized: HudShapeEntity = {
    id: typeof raw?.id === 'string' && raw.id.trim() ? raw.id : fallback.id,
    kind,
    name: typeof raw?.name === 'string' && raw.name.trim() ? raw.name : fallback.name,
    x,
    y,
    w: width,
    h: height,
    rotationDeg: clamp(Number(raw?.rotationDeg ?? fallback.rotationDeg) || fallback.rotationDeg, -180, 180),
    z: Math.max(0, Math.floor(Number(raw?.z ?? fallback.z) || fallback.z)),
    visible: typeof raw?.visible === 'boolean' ? raw.visible : fallback.visible,
    locked: typeof raw?.locked === 'boolean' ? raw.locked : fallback.locked,
    style,
  }

  if (kind === 'line') {
    normalized.lineBinding = normalizeHudLineBinding(raw?.lineBinding)
  }
  if (kind === 'image') {
    normalized.imageAsset = normalizeHudImageAsset(raw?.imageAsset, fallback.imageAsset ?? createDefaultHudImageAsset())
  }
  return normalized
}

// What: 规范化图形数组并限制上限。
// Why: 防止导入超量图形导致性能抖动，同时确保列表中的每个元素都可安全渲染。
export function normalizeHudShapeList(raw: Array<Partial<HudShapeEntity> | null | undefined>): HudShapeEntity[] {
  const list: HudShapeEntity[] = []
  for (let index = 0; index < raw.length; index += 1) {
    if (list.length >= HUD_SHAPE_LIMIT) break
    const source = raw[index] ?? {}
    const kind = normalizeKind(source.kind, 'line')
    const fallback = createDefaultHudShapeEntity(kind)
    list.push(normalizeHudShapeEntity(source, fallback))
  }
  return list
}

// What: 创建图形快照默认值。
// Why: 保证首次启动和坏数据回退时都能得到一致空图层状态。
export function createDefaultHudShapeSnapshot(): HudShapeSnapshot {
  return {
    version: HUD_SHAPE_VERSION,
    shapes: [],
  }
}
