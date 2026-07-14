import type { ClientUiConfigSnapshot } from '../types/ui'
import {
  UI_PANEL_LEGACY_STORAGE_KEYS,
  UI_PANEL_STORAGE_KEY,
  createDefaultClientUiSnapshot,
  mergeUiPanelSnapshot,
} from '../types/ui'
import type { MQTTRobotIdentity, VideoSource } from '../types/ui'

type BackendAppApi = {
  SaveClientConfig?: (configJSON: string) => Promise<void>
  GetClientConfig?: () => Promise<string>
  SetActiveVideoSource?: (source: string) => Promise<void>
  SetMQTTRobotIdentity?: (identity: string) => Promise<void>
  SetMQTTHeroIdentity?: (identity: string) => Promise<void>
  QuitApp?: () => Promise<void>
}

function getBackendApi(): BackendAppApi | null {
  const api = (window as any)?.go?.main?.App as BackendAppApi | undefined
  return api ?? null
}

// readLocalUiConfig 从浏览器本地缓存读取配置。
// What: 提供前端离线可恢复配置能力。
// Why: 后端暂不可达时仍需保证设置页与 HUD 参数稳定恢复。
export function readLocalUiConfig(): ClientUiConfigSnapshot {
  try {
    const raw = window.localStorage.getItem(UI_PANEL_STORAGE_KEY)
      ?? UI_PANEL_LEGACY_STORAGE_KEYS
        .map((key) => window.localStorage.getItem(key))
        .find((value): value is string => Boolean(value))
    if (!raw) return createDefaultClientUiSnapshot()
    return mergeUiPanelSnapshot(JSON.parse(raw) as Partial<ClientUiConfigSnapshot>)
  } catch {
    return createDefaultClientUiSnapshot()
  }
}

// writeLocalUiConfig 将配置写入浏览器本地缓存。
// What: 保存每次“应用”后的最新配置快照。
// Why: 减少冷启动与后端不可用时的配置丢失风险。
export function writeLocalUiConfig(snapshot: ClientUiConfigSnapshot): void {
  window.localStorage.setItem(UI_PANEL_STORAGE_KEY, JSON.stringify(snapshot))
}

// loadBackendUiConfig 从 Wails 后端拉取配置。
// What: 统一读取后端持久化配置，并回落到默认快照。
// Why: 保持多次启动时配置一致，避免仅本地缓存造成状态漂移。
export async function loadBackendUiConfig(): Promise<ClientUiConfigSnapshot | null> {
  const backendApi = getBackendApi()
  if (!backendApi?.GetClientConfig) return null

  try {
    const raw = await backendApi.GetClientConfig()
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<ClientUiConfigSnapshot>
    if (!parsed || typeof parsed !== 'object' || !('uiPanel' in parsed)) return null
    return mergeUiPanelSnapshot(parsed)
  } catch {
    return null
  }
}

// saveBackendUiConfig 将配置同步到 Wails 后端。
// What: 为设置保存动作提供后端确认链路。
// Why: 修复“保存无反应”问题，确保配置不是仅存在前端内存。
export async function saveBackendUiConfig(snapshot: ClientUiConfigSnapshot): Promise<void> {
  const backendApi = getBackendApi()
  if (!backendApi?.SaveClientConfig) return
  await backendApi.SaveClientConfig(JSON.stringify(snapshot))
}

// setBackendActiveVideoSource 将当前视频源立即同步到 Wails 后端。
// What: 配置层之外再补一条显式切源调用。
// Why: 视频源切换属于运行时行为，不能等到下次启动后端重新读配置才生效。
export async function setBackendActiveVideoSource(source: VideoSource): Promise<void> {
  const backendApi = getBackendApi()
  if (!backendApi?.SetActiveVideoSource) return
  await backendApi.SetActiveVideoSource(source)
}

// setBackendMQTTRobotIdentity 将当前机器人身份立即同步到 Wails 后端。
// What: 配置层之外补一条显式 MQTT 身份切换调用。
// Why: 这个身份直接决定 broker 是否接受当前 clientID，不能等到下次启动后端重新读配置才生效。
export async function setBackendMQTTRobotIdentity(identity: MQTTRobotIdentity): Promise<void> {
  const backendApi = getBackendApi()
  if (backendApi?.SetMQTTRobotIdentity) {
    await backendApi.SetMQTTRobotIdentity(identity)
    return
  }
  if (!backendApi?.SetMQTTHeroIdentity) return
  await backendApi.SetMQTTHeroIdentity(identity)
}

export const setBackendMQTTHeroIdentity = setBackendMQTTRobotIdentity

// requestBackendQuit 请求 Wails 后端退出当前应用。
// What: 为前端退出确认弹层提供统一的运行时退出入口。
// Why: 退出动作属于原生窗口生命周期控制，必须走后端 runtime.Quit，而不是在前端自行假装关闭页面。
export async function requestBackendQuit(): Promise<void> {
  const backendApi = getBackendApi()
  if (!backendApi?.QuitApp) {
    throw new Error('未检测到可用的应用退出接口')
  }
  await backendApi.QuitApp()
}
