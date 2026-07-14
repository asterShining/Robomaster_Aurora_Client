import type { HudLayoutMap } from './hudLayout'
import { createDefaultHudLayoutMap } from './hudLayout'
import type { HudShapeEntity } from './hudShape'
import { createDefaultHudLineBinding, createDefaultHudShapeEntity, normalizeHudShapeEntity } from './hudShape'

export type HudPresetId = 'match_standard' | 'tactical_minimal' | 'training_debug'

export interface HudPresetItem {
  id: HudPresetId
  label: string
  description: string
  build: () => { widgets: HudLayoutMap; shapes: HudShapeEntity[] }
}

function cloneLayoutMap(map: HudLayoutMap): HudLayoutMap {
  return JSON.parse(JSON.stringify(map)) as HudLayoutMap
}

function createLineShape(partial: Partial<HudShapeEntity>): HudShapeEntity {
  const fallback = createDefaultHudShapeEntity('line')
  return normalizeHudShapeEntity(
    {
      ...partial,
      kind: 'line',
    },
    fallback
  )
}

function createRectShape(partial: Partial<HudShapeEntity>): HudShapeEntity {
  const fallback = createDefaultHudShapeEntity('rect')
  return normalizeHudShapeEntity(
    {
      ...partial,
      kind: 'rect',
    },
    fallback
  )
}

function createLineBinding(partial: Partial<NonNullable<HudShapeEntity['lineBinding']>>): NonNullable<HudShapeEntity['lineBinding']> {
  return {
    ...createDefaultHudLineBinding(),
    ...partial,
  }
}

function buildMatchStandardPreset(): { widgets: HudLayoutMap; shapes: HudShapeEntity[] } {
  const widgets = cloneLayoutMap(createDefaultHudLayoutMap())
  // What: 标准赛预设强调中心装甲示意线。
  // Why: 让首次启用时即具备“受击方向可视反馈”的直观效果。
  const shapes: HudShapeEntity[] = [
    createLineShape({
      name: '装甲-前',
      x: 46.5,
      y: 42.2,
      w: 7,
      h: 1.4,
      z: 13,
      lineBinding: createLineBinding({ enabled: true, eventId: 1, zone: 'front' }),
    }),
    createLineShape({
      name: '装甲-后',
      x: 46.5,
      y: 56.4,
      w: 7,
      h: 1.4,
      z: 13,
      lineBinding: createLineBinding({ enabled: true, eventId: 1, zone: 'back' }),
    }),
    createLineShape({
      name: '装甲-左',
      x: 43.3,
      y: 46.6,
      w: 7,
      h: 1.4,
      rotationDeg: -90,
      z: 13,
      lineBinding: createLineBinding({ enabled: true, eventId: 1, zone: 'left' }),
    }),
    createLineShape({
      name: '装甲-右',
      x: 55.3,
      y: 46.6,
      w: 7,
      h: 1.4,
      rotationDeg: 90,
      z: 13,
      lineBinding: createLineBinding({ enabled: true, eventId: 1, zone: 'right' }),
    }),
  ]
  return { widgets, shapes }
}

function buildTacticalMinimalPreset(): { widgets: HudLayoutMap; shapes: HudShapeEntity[] } {
  const widgets = cloneLayoutMap(createDefaultHudLayoutMap())
  widgets.left_log.visible = false
  widgets.center_radar.visible = false
  // What: 极简预设仅保留轻量指示线与中心标记。
  // Why: 在高压比赛视角降低视觉噪音，避免遮挡核心图传区域。
  const shapes: HudShapeEntity[] = [
    createLineShape({
      name: '水平参考线',
      x: 42,
      y: 49.6,
      w: 16,
      h: 1.2,
      z: 12,
      lineBinding: createLineBinding({ enabled: false }),
    }),
    createLineShape({
      name: '垂直参考线',
      x: 49.4,
      y: 42,
      w: 16,
      h: 1.2,
      rotationDeg: 90,
      z: 12,
      lineBinding: createLineBinding({ enabled: false }),
    }),
  ]
  return { widgets, shapes }
}

function buildTrainingDebugPreset(): { widgets: HudLayoutMap; shapes: HudShapeEntity[] } {
  const widgets = cloneLayoutMap(createDefaultHudLayoutMap())
  widgets.center_radar.visible = true
  widgets.right_input.visible = true
  // What: 训练调试预设加入边框和标定线。
  // Why: 便于赛后复盘和模块联调时快速观察布局与命中反馈区域。
  const shapes: HudShapeEntity[] = [
    createRectShape({
      name: '中心观察框',
      x: 36,
      y: 36,
      w: 28,
      h: 28,
      z: 11,
      style: {
        strokeColor: '#88C6FF',
        strokeWidth: 2,
        strokeOpacity: 0.78,
        fillColor: '#7CA6FF',
        fillOpacity: 0.04,
      },
    }),
    createLineShape({
      name: '命中参考-前',
      x: 46.5,
      y: 40.8,
      w: 7,
      h: 1.4,
      z: 13,
      lineBinding: createLineBinding({ enabled: true, eventId: 1, zone: 'front' }),
    }),
    createLineShape({
      name: '命中参考-后',
      x: 46.5,
      y: 57.8,
      w: 7,
      h: 1.4,
      z: 13,
      lineBinding: createLineBinding({ enabled: true, eventId: 1, zone: 'back' }),
    }),
    createLineShape({
      name: '命中参考-左',
      x: 41.8,
      y: 46.6,
      w: 8,
      h: 1.4,
      rotationDeg: -90,
      z: 13,
      lineBinding: createLineBinding({ enabled: true, eventId: 1, zone: 'left' }),
    }),
    createLineShape({
      name: '命中参考-右',
      x: 56.2,
      y: 46.6,
      w: 8,
      h: 1.4,
      rotationDeg: 90,
      z: 13,
      lineBinding: createLineBinding({ enabled: true, eventId: 1, zone: 'right' }),
    }),
  ]
  return { widgets, shapes }
}

// What: 内置预设清单。
// Why: 提供可快速切换的官方模板，减少用户从零配置的时间成本。
export const HUD_PRESETS: HudPresetItem[] = [
  {
    id: 'match_standard',
    label: '比赛标准',
    description: '标准布局 + 四向装甲受击线',
    build: buildMatchStandardPreset,
  },
  {
    id: 'tactical_minimal',
    label: '极简作战',
    description: '最小化图层干扰，聚焦核心信息',
    build: buildTacticalMinimalPreset,
  },
  {
    id: 'training_debug',
    label: '训练调试',
    description: '强化观察框与调试辅助标记',
    build: buildTrainingDebugPreset,
  },
]

export const HUD_PRESET_MAP: Record<HudPresetId, HudPresetItem> = {
  match_standard: HUD_PRESETS[0],
  tactical_minimal: HUD_PRESETS[1],
  training_debug: HUD_PRESETS[2],
}
