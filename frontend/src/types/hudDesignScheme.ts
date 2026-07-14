import type { HudLayoutMap } from './hudLayout'
import type { HudShapeEntity } from './hudShape'

// What: UI 编辑方案统一导入导出的快照结构。
// Why: 将组件布局与图形图层打包到同一文件，避免方案迁移时数据分叉。
export interface HudDesignSchemeSnapshot {
  version: 1
  name: string
  exportedAt: string
  widgets: HudLayoutMap
  shapes: HudShapeEntity[]
}
