export interface HudCanvasMetrics {
  widthPx: number
  heightPx: number
}

export interface HudCanvasPoint {
  x: number
  y: number
}

export interface HudCanvasSize {
  w: number
  h: number
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

// What: 统一读取 HUD 画布像素尺寸。
// Why: 组件拖拽和图形编辑都依赖同一套尺寸基线，避免不同模块各自换算产生漂移。
export function resolveHudCanvasMetrics(canvasEl: HTMLElement | null): HudCanvasMetrics {
  const rect = canvasEl?.getBoundingClientRect()
  if (!rect) {
    return {
      widthPx: 1,
      heightPx: 1,
    }
  }
  return {
    widthPx: Math.max(1, rect.width),
    heightPx: Math.max(1, rect.height),
  }
}

// What: 将指针位置转换为画布百分比坐标。
// Why: 图形数据以百分比持久化，统一入口可避免拖拽与导入后的坐标系统不一致。
export function toHudCanvasPercent(canvasEl: HTMLElement | null, clientX: number, clientY: number): HudCanvasPoint {
  const rect = canvasEl?.getBoundingClientRect()
  if (!rect) return { x: 0, y: 0 }
  return {
    x: clamp(((clientX - rect.left) / Math.max(1, rect.width)) * 100, 0, 100),
    y: clamp(((clientY - rect.top) / Math.max(1, rect.height)) * 100, 0, 100),
  }
}

// What: 将像素位移转换为画布百分比。
// Why: 拖拽与缩放预览都基于 pointer delta，统一换算可减少边界误差。
export function hudPxToPercent(deltaPx: number, canvasSizePx: number): number {
  return (deltaPx / Math.max(1, canvasSizePx)) * 100
}

// What: 对 HUD 百分比坐标做边界裁剪。
// Why: 防止拖拽或缩放后元素掉出画布，保证后续仍可再次选中。
export function clampHudPercent(value: number, min: number, max: number): number {
  return clamp(value, min, max)
}

// What: 将拖拽结果吸附到 8px 网格。
// Why: 组件布局是低频精调场景，适当吸附能减少视觉抖动和半像素错位。
export function snapHudPixel(value: number, stepPx = 8): number {
  return Math.round(value / stepPx) * stepPx
}

// What: 依据图片原始比例计算画布百分比尺寸。
// Why: 让图像插入与替换时默认保持等比，不出现第一眼就变形的糟糕体验。
export function resolveHudImagePercentSize(
  naturalWidth: number,
  naturalHeight: number,
  metrics: HudCanvasMetrics,
  maxWidthPercent = 16,
  maxHeightPercent = 16
): HudCanvasSize {
  const safeNaturalWidth = Math.max(1, naturalWidth)
  const safeNaturalHeight = Math.max(1, naturalHeight)
  const maxWidthPx = (Math.max(1, maxWidthPercent) / 100) * metrics.widthPx
  const maxHeightPx = (Math.max(1, maxHeightPercent) / 100) * metrics.heightPx
  const scale = Math.min(maxWidthPx / safeNaturalWidth, maxHeightPx / safeNaturalHeight, 1)
  return {
    w: ((safeNaturalWidth * scale) / metrics.widthPx) * 100,
    h: ((safeNaturalHeight * scale) / metrics.heightPx) * 100,
  }
}
