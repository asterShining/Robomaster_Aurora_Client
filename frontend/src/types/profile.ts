export type RobotType = 'hero' | 'infantry' | 'engineer' | 'sentry' | 'drone' | 'unknown'

export interface RobotProfile {
  id: string
  teamSide: 'red' | 'blue'
  robotType: RobotType
  displayName: string
  avatarAsset: string
}

// What: 提供头像占位映射。
// Why: 在真实素材未接入前先确保 UI 结构和数据契约稳定，后续只需替换资源路径。
export const DEFAULT_ROBOT_PROFILES: Record<'red' | 'blue', RobotProfile> = {
  red: {
    id: 'R1',
    teamSide: 'red',
    robotType: 'hero',
    displayName: '红方英雄',
    avatarAsset: 'inline://avatar-red-default',
  },
  blue: {
    id: 'B1',
    teamSide: 'blue',
    robotType: 'hero',
    displayName: '蓝方英雄',
    avatarAsset: 'inline://avatar-blue-default',
  },
}
