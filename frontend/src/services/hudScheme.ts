import type { HudLayoutSnapshot } from '../types/hudLayout'
import type { HudDesignSchemeSnapshot } from '../types/hudDesignScheme'
import type { HudPresetId } from '../types/hudPresets'
import { HUD_PRESET_MAP } from '../types/hudPresets'
import type { HudShapeSnapshot } from '../types/hudShape'
import { HUD_SHAPE_VERSION, normalizeHudShapeList } from '../types/hudShape'
import { HUD_LAYOUT_VERSION, createDefaultHudLayoutMap, HUD_WIDGET_IDS } from '../types/hudLayout'

function pad2(value: number): string {
  return String(value).padStart(2, '0')
}

function buildTimestampName(): string {
  const now = new Date()
  const year = now.getFullYear()
  const month = pad2(now.getMonth() + 1)
  const day = pad2(now.getDate())
  const hour = pad2(now.getHours())
  const minute = pad2(now.getMinutes())
  const second = pad2(now.getSeconds())
  return `${year}${month}${day}-${hour}${minute}${second}`
}

// What: 组装 UI 方案导出结构。
// Why: 将组件布局和图形图层打包为单文件，避免迁移时出现版本错位。
export function buildHudDesignScheme(
  layoutSnapshot: HudLayoutSnapshot,
  shapeSnapshot: HudShapeSnapshot,
  name: string
): HudDesignSchemeSnapshot {
  return {
    version: 1,
    name: name.trim() || `HUD方案-${buildTimestampName()}`,
    exportedAt: new Date().toISOString(),
    widgets: layoutSnapshot.widgets,
    shapes: shapeSnapshot.shapes,
  }
}

// What: 触发方案 JSON 文件下载。
// Why: 让用户可在不同设备和账号之间快速迁移 UI 配置。
export function downloadHudDesignScheme(snapshot: HudDesignSchemeSnapshot): void {
  const payload = JSON.stringify(snapshot, null, 2)
  const blob = new Blob([payload], { type: 'application/json;charset=utf-8' })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `rm-hud-scheme-${buildTimestampName()}.json`
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.URL.revokeObjectURL(url)
}

function readFileAsText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(new Error('读取方案文件失败'))
    reader.readAsText(file, 'utf-8')
  })
}

function isSafeWidgetMap(raw: unknown): raw is Record<string, unknown> {
  return !!raw && typeof raw === 'object' && !Array.isArray(raw)
}

// What: 校验并归一化导入方案。
// Why: 避免坏文件覆盖当前布局，保证导入路径可恢复且可预测。
export async function parseHudDesignSchemeFile(
  file: File
): Promise<{ widgets: HudLayoutSnapshot; shapes: HudShapeSnapshot; name: string }> {
  const rawText = await readFileAsText(file)
  let parsed: Partial<HudDesignSchemeSnapshot> | null = null
  try {
    parsed = JSON.parse(rawText) as Partial<HudDesignSchemeSnapshot>
  } catch {
    throw new Error('方案 JSON 解析失败')
  }

  if (!parsed || parsed.version !== 1) {
    throw new Error('方案版本不兼容')
  }

  const defaultWidgets = createDefaultHudLayoutMap()
  const widgetRaw = isSafeWidgetMap(parsed.widgets) ? parsed.widgets : defaultWidgets
  const widgets = { ...defaultWidgets }
  for (const id of HUD_WIDGET_IDS) {
    widgets[id] = (widgetRaw[id] as any) ?? defaultWidgets[id]
  }

  return {
    name: typeof parsed.name === 'string' && parsed.name.trim() ? parsed.name : '导入方案',
    widgets: {
      version: HUD_LAYOUT_VERSION,
      widgets,
    },
    shapes: {
      version: HUD_SHAPE_VERSION,
      shapes: normalizeHudShapeList(Array.isArray(parsed.shapes) ? parsed.shapes : []),
    },
  }
}

// What: 读取内置预设并转成可导入快照。
// Why: 复用统一导入流程，减少“预设应用”与“文件导入”两套逻辑分叉。
export function resolveHudPresetScheme(presetId: HudPresetId): {
  name: string
  widgets: HudLayoutSnapshot
  shapes: HudShapeSnapshot
} {
  const preset = HUD_PRESET_MAP[presetId]
  const built = preset.build()
  return {
    name: preset.label,
    widgets: {
      version: HUD_LAYOUT_VERSION,
      widgets: built.widgets,
    },
    shapes: {
      version: HUD_SHAPE_VERSION,
      shapes: normalizeHudShapeList(built.shapes),
    },
  }
}

