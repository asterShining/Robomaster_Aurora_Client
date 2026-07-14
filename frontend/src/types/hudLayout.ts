export type HudWidgetId =
  | 'top_red_base_hp'
  | 'top_red_team'
  | 'top_score'
  | 'top_blue_team'
  | 'top_blue_base_hp'
  | 'left_modules'
  | 'left_robot'
  | 'left_log'
  | 'right_fire'
  | 'right_input'
  | 'center_crosshair'
  | 'center_radar'
  | 'global_unit_status'
  | 'global_logistics_status'
  | 'global_special_mechanism'
  | 'robot_module_strip'
  | 'buff_tags'
  | 'deploy_mode_tag'
  | 'referee_toast'
  | 'radar_info'

// What: 统一 HUD 组件顺序与标识常量。
// Why: 主布局渲染和设置页管理共用一份列表，避免多处硬编码导致顺序漂移。
export const HUD_WIDGET_IDS: HudWidgetId[] = [
  'top_red_base_hp',
  'top_red_team',
  'top_score',
  'top_blue_team',
  'top_blue_base_hp',
  'left_modules',
  'left_robot',
  'left_log',
  'right_fire',
  'right_input',
  'center_crosshair',
  'center_radar',
  'global_unit_status',
  'global_logistics_status',
  'global_special_mechanism',
  'robot_module_strip',
  'buff_tags',
  'deploy_mode_tag',
  'referee_toast',
  'radar_info',
]

// What: 统一组件中文标签映射。
// Why: 设置面板和 HUD registry 复用同一文案，降低维护分叉风险。
export const HUD_WIDGET_LABELS: Record<HudWidgetId, string> = {
  top_red_base_hp: '红方基地血量',
  top_red_team: '红方机器人队伍',
  top_score: '比分与局时',
  top_blue_team: '蓝方机器人队伍',
  top_blue_base_hp: '蓝方基地血量',
  left_modules: '链路诊断',
  left_robot: '本车状态',
  left_log: '跑马灯提示',
  right_fire: '火控数据',
  right_input: '键鼠信息',
  center_crosshair: '中心准心',
  center_radar: '雷达态势',
  global_unit_status: '全局单位',
  global_logistics_status: '经济科技',
  global_special_mechanism: '特殊机制',
  robot_module_strip: '模块状态条',
  buff_tags: '增益标签',
  deploy_mode_tag: '部署模式',
  referee_toast: '裁判事件',
  radar_info: '雷达目标',
}

export interface HudWidgetRect {
  x: number
  y: number
  w: number
  h: number
  scale: number
  z: number
  visible: boolean
  locked: boolean
}

export type HudLayoutMap = Record<HudWidgetId, HudWidgetRect>

export interface HudLayoutSnapshot {
  version: number
  widgets: HudLayoutMap
}

export const HUD_WIDGET_SCALE_MIN = 0.7
export const HUD_WIDGET_SCALE_MAX = 1.5
export const HUD_WIDGET_SCALE_STEP = 0.02
// What: 组件管理页缩放预设档位。
// Why: 离散档位可减少现场误触微调，保持多设备布局一致性。
export const HUD_WIDGET_SCALE_PRESETS = [0.8, 0.9, 1, 1.1, 1.2, 1.3, 1.4] as const

export const HUD_LAYOUT_VERSION = 8
export const HUD_LAYOUT_STORAGE_KEY = `rm-client-hud-layout:v${HUD_LAYOUT_VERSION}`

// What: 提供官方布局导向的默认组件坐标与尺寸。Why: 首次启动即可获得可用排版，并作为布局重置基线。
export function createDefaultHudLayoutMap(): HudLayoutMap {
  return {
    // What: 顶部五块组件按当前极简 HUD 的视觉比例重新落位。
    // Why: 功能精简后恢复拖拽时，默认排布必须先对齐现有界面，而不是跳回旧版大面积底部布局。
    top_red_base_hp: { x: 2.5, y: 1.6, w: 14.3, h: 6.9, scale: 1, z: 16, visible: true, locked: false },
    top_red_team: { x: 17.3, y: 2.8, w: 22.6, h: 5.4, scale: 1, z: 16, visible: true, locked: false },
    top_score: { x: 40.4, y: 1.4, w: 19.2, h: 8.6, scale: 1, z: 16, visible: true, locked: false },
    top_blue_team: { x: 60.1, y: 2.8, w: 22.6, h: 5.4, scale: 1, z: 16, visible: true, locked: false },
    top_blue_base_hp: { x: 83.2, y: 1.6, w: 14.3, h: 6.9, scale: 1, z: 16, visible: true, locked: false },
    // What: 左右状态卡改为贴近画面边缘的上半区初始布局。
    // Why: 这是当前精简版界面的默认观看位置，能尽量少挡住中下方主视野。
    // What: 诊断卡默认尺寸上调并留足 6 项状态位。
    // Why: 用户当前最直接的问题就是后两项被裁掉，因此默认布局必须先保证完整显示。
    left_modules: { x: 1, y: 11.8, w: 18.6, h: 21.2, scale: 1, z: 20, visible: true, locked: false },
    // What: 本车状态卡同步扩容并下移，避免与诊断卡重叠。
    // Why: 这张卡有血条和 4 行状态，旧默认尺寸太紧，长文本会直接挤出边框。
    left_robot: { x: 1, y: 34.3, w: 18.6, h: 24.6, scale: 1, z: 20, visible: true, locked: false },
    left_log: { x: 1.5, y: 73, w: 18, h: 12.8, scale: 1, z: 20, visible: false, locked: false },
    right_fire: { x: 84, y: 11.8, w: 15.1, h: 20, scale: 1, z: 20, visible: true, locked: false },
    right_input: { x: 73.8, y: 82, w: 25, h: 12.4, scale: 1, z: 20, visible: false, locked: false },
    // What: 准星外框按当前组件真实像素尺寸缩小到中心区域。
    // Why: 旧版 16% 宽高会把准星放得过大，恢复拖拽后必须先给出接近实战可用的默认尺寸。
    center_crosshair: { x: 45.8, y: 45.9, w: 8.4, h: 8.4, scale: 1, z: 14, visible: true, locked: false },
    center_radar: { x: 84.2, y: 57.2, w: 14.4, h: 22.4, scale: 1, z: 20, visible: false, locked: false },
    global_unit_status: { x: 84, y: 33, w: 14.2, h: 14.8, scale: 1, z: 18, visible: false, locked: false },
    global_logistics_status: { x: 84, y: 48, w: 12.8, h: 10.8, scale: 1, z: 18, visible: false, locked: false },
    global_special_mechanism: { x: 60.8, y: 84, w: 12.4, h: 10.6, scale: 1, z: 18, visible: false, locked: false },
    robot_module_strip: { x: 1, y: 60, w: 35.6, h: 3.2, scale: 1, z: 22, visible: true, locked: false },
    buff_tags: { x: 1, y: 31, w: 18.6, h: 4.4, scale: 1, z: 22, visible: true, locked: false },
    deploy_mode_tag: { x: 42, y: 48, w: 16, h: 5.6, scale: 1, z: 28, visible: true, locked: false },
    referee_toast: { x: 72, y: 72, w: 26, h: 14.2, scale: 1, z: 24, visible: true, locked: false },
    radar_info: { x: 70, y: 42, w: 14, h: 12.8, scale: 1, z: 18, visible: false, locked: false },
  }
}
