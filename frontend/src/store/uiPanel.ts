import { defineStore } from 'pinia'
import { computed, ref, toRaw } from 'vue'
import type { ClientUiConfigSnapshot, KeyBindingItem, MQTTRobotIdentity, PanelTab, UiPanelDraft, VideoSource } from '../types/ui'
import { createDefaultClientUiSnapshot, createDefaultUiPanelDraft, mergeUiPanelDraft, normalizeVideoSourceForRobotIdentity } from '../types/ui'
import { loadBackendUiConfig, readLocalUiConfig, saveBackendUiConfig, setBackendActiveVideoSource, setBackendMQTTRobotIdentity, writeLocalUiConfig } from '../services/clientConfig'
import { normalizeKeyToken, resolveBindingConflict } from '../composables/useKeymap'

type SaveStatus = 'idle' | 'saving' | 'success' | 'error'

function cloneDraft<T>(value: T): T {
  const raw = toRaw(value)
  if (typeof structuredClone === 'function') return structuredClone(raw)
  return JSON.parse(JSON.stringify(raw)) as T
}

export const useUiPanelStore = defineStore('ui-panel', () => {
  const isOpen = ref(false)
  const activeTab = ref<PanelTab>('mqtt')
  const pendingCloseConfirm = ref(false)
  const hasHydrated = ref(false)

  const saveStatus = ref<SaveStatus>('idle')
  const saveMessage = ref('')

  const applied = ref<UiPanelDraft>(createDefaultUiPanelDraft())
  const draft = ref<UiPanelDraft>(cloneDraft(applied.value))

  const isDirty = computed(() => JSON.stringify(draft.value) !== JSON.stringify(applied.value))

  function setSaveState(status: SaveStatus, message = ''): void {
    saveStatus.value = status
    saveMessage.value = message
  }

  function buildSnapshot(source: UiPanelDraft): ClientUiConfigSnapshot {
    return {
      ...createDefaultClientUiSnapshot(),
      uiPanel: cloneDraft(source),
    }
  }

  // persistAppliedState 将当前已应用配置落到本地、后端和运行时。
  // What: 把“配置保存”和“运行时切源”统一收口到一条路径。
  // Why: 视频源已经变成真实后端能力，不能再出现配置已保存但当前运行实例仍停在旧源上的错位。
  async function persistAppliedState(nextApplied: UiPanelDraft): Promise<void> {
    writeLocalUiConfig(buildSnapshot(nextApplied))
    await saveBackendUiConfig(buildSnapshot(nextApplied))
    await setBackendMQTTRobotIdentity(nextApplied.mqttRobotIdentity)
    await setBackendActiveVideoSource(nextApplied.videoSource)
  }

  // hydrate 在启动时恢复本地与后端配置。
  // What: 按“本地优先快速恢复 + 后端覆盖最终一致”顺序加载配置。
  // Why: 先保证页面快速可用，再确保配置与后端持久化对齐。
  async function hydrate(): Promise<void> {
    if (hasHydrated.value) return
    hasHydrated.value = true

    const localSnapshot = readLocalUiConfig()
    applied.value = mergeUiPanelDraft(localSnapshot.uiPanel)
    draft.value = cloneDraft(applied.value)

    const backendSnapshot = await loadBackendUiConfig()
    if (backendSnapshot?.uiPanel) {
      applied.value = mergeUiPanelDraft(backendSnapshot.uiPanel)
      draft.value = cloneDraft(applied.value)
      writeLocalUiConfig(buildSnapshot(applied.value))
    }

    // What: 启动期在 hydrate 结束后立即同步当前视频源。
    // Why: MQTT 身份与视频源都属于运行时行为；若只等下次重启，用户刚保存的机器人选择不会立刻作用到后端。
    await setBackendMQTTRobotIdentity(applied.value.mqttRobotIdentity)
    await setBackendActiveVideoSource(applied.value.videoSource)
  }

  // What: 打开面板时重建草稿副本。
  // Why: 确保编辑隔离，避免未保存内容污染已应用配置。
  function openPanel(tab?: PanelTab): void {
    draft.value = cloneDraft(applied.value)
    if (tab) activeTab.value = tab
    pendingCloseConfirm.value = false
    isOpen.value = true
    setSaveState('idle')
  }

  // What: 快捷键切换入口。
  // Why: 统一鼠标与键盘触发逻辑，避免状态机分叉。
  function togglePanel(tab?: PanelTab): void {
    if (isOpen.value) {
      closeWithGuard()
      return
    }
    openPanel(tab)
  }

  // What: 安全关闭。
  // Why: 防止用户误触关闭导致参数丢失。
  function closeWithGuard(): boolean {
    if (!isDirty.value) {
      isOpen.value = false
      pendingCloseConfirm.value = false
      return true
    }
    pendingCloseConfirm.value = true
    return false
  }

  function forceClose(): void {
    pendingCloseConfirm.value = false
    isOpen.value = false
  }

  function confirmDiscardAndClose(): void {
    draft.value = cloneDraft(applied.value)
    forceClose()
    setSaveState('idle')
  }

  function discardDraft(): void {
    draft.value = cloneDraft(applied.value)
    pendingCloseConfirm.value = false
    isOpen.value = false
    setSaveState('idle')
  }

  // applyDraft 将草稿应用并同步后端。
  // What: 保存流程包含前端应用、本地落盘、后端持久化三步。
  // Why: 修复“保存无反应”并确保配置在重启后仍保持一致。
  async function applyDraft(): Promise<void> {
    if (saveStatus.value === 'saving') return

    const nextApplied = mergeUiPanelDraft(draft.value)
    applied.value = cloneDraft(nextApplied)
    draft.value = cloneDraft(nextApplied)
    writeLocalUiConfig(buildSnapshot(nextApplied))

    setSaveState('saving', '正在同步后端...')

    try {
      await persistAppliedState(nextApplied)
      pendingCloseConfirm.value = false
      isOpen.value = false
      setSaveState('success', '配置已保存并同步后端')
      window.setTimeout(() => {
        if (saveStatus.value === 'success') setSaveState('idle')
      }, 1200)
    } catch (error) {
      const text = error instanceof Error ? error.message : '保存失败'
      setSaveState('error', `保存失败：${text}`)
      isOpen.value = true
    }
  }

  function setActiveTab(tab: PanelTab): void {
    activeTab.value = tab
  }

  function setMqttRoute(route: string): void {
    draft.value.mqttRoute = route
  }

  function setMqttFilter(filter: string): void {
    draft.value.mqttFilter = filter
  }

  function selectMetric(metricId: string): void {
    draft.value.selectedMetricId = metricId
  }

  function toggleMetric(metricId: string): void {
    draft.value.metrics = draft.value.metrics.map((item) =>
      item.id === metricId ? { ...item, enabled: !item.enabled } : item
    )
  }

  function updateKeyBinding(action: string, key: string): void {
    const normalized = normalizeKeyToken(key)
    draft.value.keymap = resolveBindingConflict(draft.value.keymap, action, normalized)
  }

  function resetKeymap(): void {
    const defaults = createDefaultUiPanelDraft()
    draft.value.keymap = defaults.keymap
  }

  function importKeymap(payload: KeyBindingItem[]): void {
    draft.value.keymap = payload
  }

  function updateHudDisplay(partial: Partial<UiPanelDraft['hud']>): void {
    draft.value.hud = { ...draft.value.hud, ...partial }
  }

  function updateInputConfig(partial: Partial<UiPanelDraft['input']>): void {
    draft.value.input = { ...draft.value.input, ...partial }
  }

  function updateQuickActionConfig(partial: Partial<UiPanelDraft['quickActions']>): void {
    // What: 合并更新快捷动作草稿配置。
    // Why: 让买弹预设等动作参数与其余设置共享同一保存链路，避免出现第二套临时状态源。
    draft.value.quickActions = { ...draft.value.quickActions, ...partial }
  }

  // applyMinimalPanelPatch 用于极简设置面板的即时保存。
  // What: 对视频源和三张状态卡开关做“改即生效”保存。
  // Why: 当前设置已经收敛成小面板，再保留旧版多 tab 草稿提交流程只会增加操作成本。
  async function applyMinimalPanelPatch(partial: {
    videoSource?: VideoSource
    mqttRobotIdentity?: MQTTRobotIdentity
    mqttHeroIdentity?: MQTTRobotIdentity
    showDiagnosticsCard?: boolean
    showRobotStatusCard?: boolean
    showFireControlCard?: boolean
    resolution?: string
  }): Promise<void> {
    const requestedIdentity = partial.mqttRobotIdentity ?? partial.mqttHeroIdentity
    const requestedVideoSource = partial.videoSource ?? applied.value.videoSource
    const effectiveIdentity = requestedIdentity ?? applied.value.mqttRobotIdentity
    const effectiveVideoSource = normalizeVideoSourceForRobotIdentity(
      requestedVideoSource,
      effectiveIdentity,
    )
    const nextApplied = mergeUiPanelDraft({
      ...applied.value,
      ...partial,
      videoSource: effectiveVideoSource,
      ...(requestedIdentity ? {
        mqttRobotIdentity: requestedIdentity,
        mqttHeroIdentity: requestedIdentity,
      } : {}),
    })

    applied.value = cloneDraft(nextApplied)
    draft.value = cloneDraft(nextApplied)
    setSaveState('saving', '正在同步后端...')

    try {
      await persistAppliedState(nextApplied)
      setSaveState('success', '已应用')
      window.setTimeout(() => {
        if (saveStatus.value === 'success') setSaveState('idle')
      }, 1000)
    } catch (error) {
      const text = error instanceof Error ? error.message : '保存失败'
      setSaveState('error', `保存失败：${text}`)
    }
  }

  return {
    isOpen,
    activeTab,
    pendingCloseConfirm,
    isDirty,
    draft,
    applied,
    saveStatus,
    saveMessage,
    hydrate,
    openPanel,
    togglePanel,
    closeWithGuard,
    forceClose,
    confirmDiscardAndClose,
    discardDraft,
    applyDraft,
    setActiveTab,
    setMqttRoute,
    setMqttFilter,
    selectMetric,
    toggleMetric,
    updateKeyBinding,
    resetKeymap,
    importKeymap,
    updateHudDisplay,
    updateInputConfig,
    updateQuickActionConfig,
    applyMinimalPanelPatch,
  }
})
