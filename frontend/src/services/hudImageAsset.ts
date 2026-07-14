import type { HudImageAsset } from '../types/hudShape'

const HUD_IMAGE_MAX_EDGE_PX = 1600
const HUD_IMAGE_MAX_DATA_URL_BYTES = 1_200_000
const HUD_IMAGE_ACCEPTED_MIME_TYPES = ['image/png', 'image/jpeg', 'image/webp', 'image/svg+xml'] as const

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(new Error('读取图片失败'))
    reader.readAsDataURL(file)
  })
}

function loadImageElement(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => reject(new Error('解析图片尺寸失败'))
    image.src = src
  })
}

function resolveScaledSize(width: number, height: number): { width: number; height: number } {
  const scale = Math.min(HUD_IMAGE_MAX_EDGE_PX / Math.max(1, width), HUD_IMAGE_MAX_EDGE_PX / Math.max(1, height), 1)
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  }
}

function estimateDataUrlBytes(dataUrl: string): number {
  const base64 = dataUrl.split(',')[1] ?? ''
  return Math.ceil((base64.length * 3) / 4)
}

function assertSupportedImageFile(file: File): void {
  // What: 限定绘制层可接受的图片格式。
  // Why: 导出方案要长期存储在 localStorage 中，提前限制格式可避免不兼容资源污染快照。
  if (!HUD_IMAGE_ACCEPTED_MIME_TYPES.includes(file.type as (typeof HUD_IMAGE_ACCEPTED_MIME_TYPES)[number])) {
    throw new Error('仅支持 PNG、JPEG、WebP 或 SVG 图片')
  }
}

async function buildSvgAsset(file: File): Promise<HudImageAsset> {
  const src = await readFileAsDataUrl(file)
  const image = await loadImageElement(src)
  if (estimateDataUrlBytes(src) > HUD_IMAGE_MAX_DATA_URL_BYTES) {
    throw new Error('SVG 资源过大，请换一张更小的图片')
  }
  return {
    name: file.name,
    mimeType: 'image/svg+xml',
    src,
    naturalWidth: Math.max(1, image.naturalWidth || 1),
    naturalHeight: Math.max(1, image.naturalHeight || 1),
  }
}

async function buildRasterAsset(file: File): Promise<HudImageAsset> {
  const src = await readFileAsDataUrl(file)
  const image = await loadImageElement(src)
  const canvas = document.createElement('canvas')
  const size = resolveScaledSize(image.naturalWidth || image.width, image.naturalHeight || image.height)
  canvas.width = size.width
  canvas.height = size.height
  const context = canvas.getContext('2d')
  if (!context) {
    throw new Error('浏览器不支持图片压缩')
  }
  // What: 将栅格图片统一压成 WebP。
  // Why: 限制快照体积，避免本地图层配置很快把 localStorage 打满。
  context.clearRect(0, 0, size.width, size.height)
  context.drawImage(image, 0, 0, size.width, size.height)
  const nextSrc = canvas.toDataURL('image/webp', 0.9)
  if (estimateDataUrlBytes(nextSrc) > HUD_IMAGE_MAX_DATA_URL_BYTES) {
    throw new Error('图片压缩后仍过大，请换一张更小的图片')
  }
  return {
    name: file.name.replace(/\.[^.]+$/, '') || '图像素材',
    mimeType: 'image/webp',
    src: nextSrc,
    naturalWidth: size.width,
    naturalHeight: size.height,
  }
}

// What: 将用户上传文件转换为可持久化的图片资源。
// Why: 悬浮编辑器新增与替换图片都需要同一套压缩、校验和尺寸抽取逻辑。
export async function buildHudImageAssetFromFile(file: File): Promise<HudImageAsset> {
  assertSupportedImageFile(file)
  if (file.type === 'image/svg+xml') {
    return buildSvgAsset(file)
  }
  return buildRasterAsset(file)
}
