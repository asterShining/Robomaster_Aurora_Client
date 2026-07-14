import { defineStore } from 'pinia'
import { computed, ref, shallowRef } from 'vue'
import {
  ACTION_PURCHASE_AMMO,
  buildKeyboardBitmask,
  eventToKeyToken,
  findActionKeyToken,
  shouldIgnoreKeyboardTarget,
} from '../composables/useKeymap'
import { useUiPanelStore } from './uiPanel'
import { useRobotDataStore } from './robotData'
import { useHudLayoutStore } from './hudLayout'
import { useHudShapeStore } from './hudShape'

interface MouseButtonsState {
  left: boolean
  right: boolean
  middle: boolean
}

interface InputActivityState {
  leftAt: number
  rightAt: number
  middleAt: number
  moveAt: number
  wheelAt: number
  keyActiveAt: Record<string, number>
}

interface ControlPacket {
  x: number
  y: number
  z: number
  left: boolean
  right: boolean
  middle: boolean
  kb: number
}

type PointerTargetResolver = () => HTMLElement | null
type QuickActionFeedbackTone = 'info' | 'success' | 'danger'
type QuickActionApi = (action: string, value: number) => Promise<void>

// What: 集中声明一键动作协议常量。
// Why: 前端热键触发与后端下发必须共享稳定字面量，避免后续扩展时出现魔法字符串分叉。
const QUICK_ACTION_KIND_BUY_AMMO = 'buy_ammo'
const QUICK_ACTION_FEEDBACK_MS = 1400

function clampI32(value: number): number {
  const rounded = Math.round(value)
  return Math.max(-2147483648, Math.min(2147483647, rounded))
}

function getPushKeyboardMouseApi():
  | ((x: number, y: number, z: number, left: boolean, right: boolean, middle: boolean, kb: number) => Promise<void>)
  | null {
  const fn = (window as any)?.go?.main?.App?.PushKeyboardMouse
  return typeof fn === 'function' ? fn : null
}

function getTriggerQuickActionApi(): QuickActionApi | null {
  const fn = (window as any)?.go?.main?.App?.TriggerQuickAction
  return typeof fn === 'function' ? fn : null
}

export const useInputCaptureStore = defineStore('input-capture', () => {
  const uiPanelStore = useUiPanelStore()
  const robotStore = useRobotDataStore()
  const layoutStore = useHudLayoutStore()
  const shapeStore = useHudShapeStore()

  const installed = ref(false)
  const pointerLocked = ref(false)
  const pointerSupported = ref(typeof document !== 'undefined' && 'pointerLockElement' in document)

  const mouseButtons = ref<MouseButtonsState>({
    left: false,
    right: false,
    middle: false,
  })

  const activity = ref<InputActivityState>({
    leftAt: 0,
    rightAt: 0,
    middleAt: 0,
    moveAt: 0,
    wheelAt: 0,
    keyActiveAt: {},
  })

  const pressedTokens = shallowRef<Set<string>>(new Set())
  const backendError = ref('')
  const quickActionFeedback = ref('')
  const quickActionFeedbackTone = ref<QuickActionFeedbackTone>('info')

  const captureEnabled = computed(() => uiPanelStore.applied.input.captureEnabled)
  const showCaptureButton = computed(() => uiPanelStore.applied.input.showCaptureButton)
  const pressedKeyTokens = computed(() => Array.from(pressedTokens.value))
  const isHudEditing = computed(() => layoutStore.isEditMode || shapeStore.isDrawMode)
  const buyAmmoToken = computed(() => findActionKeyToken(uiPanelStore.applied.keymap, ACTION_PURCHASE_AMMO))
  const buyAmmoPreset = computed(() => uiPanelStore.applied.quickActions.buyAmmoPreset)
  const canQuickBuyAmmo = computed(() => robotStore.combat.canRemoteAmmo)

  let pointerTargetResolver: PointerTargetResolver | null = null
  let rafId: number | null = null
  let sending = false
  let quickActionSending = false
  let accumMouseX = 0
  let accumMouseY = 0
  let accumMouseZ = 0
  let quickActionFeedbackTimer: number | null = null

  let lastPacket: ControlPacket = {
    x: 0,
    y: 0,
    z: 0,
    left: false,
    right: false,
    middle: false,
    kb: 0,
  }

  function setMouseButton(button: number, pressed: boolean): void {
    const now = Date.now()
    if (button === 0) {
      mouseButtons.value.left = pressed
      activity.value.leftAt = now
    } else if (button === 1) {
      mouseButtons.value.middle = pressed
      activity.value.middleAt = now
    } else if (button === 2) {
      mouseButtons.value.right = pressed
      activity.value.rightAt = now
    }
  }

  function markTokenActivity(token: string): void {
    activity.value.keyActiveAt[token] = Date.now()
  }

  function addPressedToken(token: string): void {
    const next = new Set(pressedTokens.value)
    next.add(token)
    pressedTokens.value = next
    markTokenActivity(token)
  }

  function removePressedToken(token: string): void {
    const next = new Set(pressedTokens.value)
    next.delete(token)
    pressedTokens.value = next
    markTokenActivity(token)
  }

  function isPacketChanged(nextPacket: ControlPacket): boolean {
    return (
      nextPacket.x !== lastPacket.x ||
      nextPacket.y !== lastPacket.y ||
      nextPacket.z !== lastPacket.z ||
      nextPacket.left !== lastPacket.left ||
      nextPacket.right !== lastPacket.right ||
      nextPacket.middle !== lastPacket.middle ||
      nextPacket.kb !== lastPacket.kb
    )
  }

  function clearMotionAccumulators(): void {
    accumMouseX = 0
    accumMouseY = 0
    accumMouseZ = 0
  }

  function clearInputState(): void {
    // What: 失焦时清空按键与按钮状态。
    // Why: 防止窗口切换后出现“卡键/卡鼠标按下”的幽灵状态。
    pressedTokens.value = new Set()
    mouseButtons.value = { left: false, right: false, middle: false }
    clearMotionAccumulators()
  }

  function clearQuickActionFeedbackTimer(): void {
    if (quickActionFeedbackTimer === null) return
    window.clearTimeout(quickActionFeedbackTimer)
    quickActionFeedbackTimer = null
  }

  function setQuickActionFeedback(message: string, tone: QuickActionFeedbackTone): void {
    // What: 在右侧轻提示区域显示一键动作结果。
    // Why: 买弹成功或被拦截都需要即时确认，但又不能弹出大面积浮层遮挡战场画面。
    quickActionFeedback.value = message
    quickActionFeedbackTone.value = tone
    clearQuickActionFeedbackTimer()
    quickActionFeedbackTimer = window.setTimeout(() => {
      quickActionFeedback.value = ''
      quickActionFeedbackTimer = null
    }, QUICK_ACTION_FEEDBACK_MS)
  }

  async function triggerQuickBuyAmmo(): Promise<void> {
    const api = getTriggerQuickActionApi()
    if (!api) {
      setQuickActionFeedback('买弹接口不可用', 'danger')
      return
    }
    if (!canQuickBuyAmmo.value) {
      setQuickActionFeedback('当前不可远程买弹', 'info')
      return
    }
    if (quickActionSending) return

    quickActionSending = true
    try {
      // What: 一键买弹只下发标准化动作名与预设数量。
      // Why: 让前端只负责交互与参数，具体字节协议统一收口在后端桥接层。
      await api(QUICK_ACTION_KIND_BUY_AMMO, buyAmmoPreset.value)
      setQuickActionFeedback(`已发送买弹 ${buyAmmoPreset.value} 发`, 'success')
    } catch (error) {
      setQuickActionFeedback(error instanceof Error ? error.message : '买弹发送失败', 'danger')
    } finally {
      quickActionSending = false
    }
  }

  async function pushPacketToBackend(packet: ControlPacket): Promise<void> {
    const api = getPushKeyboardMouseApi()
    if (!api) return

    if (sending) return
    sending = true
    try {
      await api(packet.x, packet.y, packet.z, packet.left, packet.right, packet.middle, packet.kb)
      backendError.value = ''
    } catch (error) {
      backendError.value = error instanceof Error ? error.message : '键鼠下发失败'
    } finally {
      sending = false
    }
  }

  function buildCurrentPacket(): ControlPacket {
    const keyBitmask = buildKeyboardBitmask(pressedTokens.value, uiPanelStore.applied.keymap)
    const allowPointerMotion = captureEnabled.value && pointerLocked.value

    const packet: ControlPacket = {
      x: allowPointerMotion ? clampI32(accumMouseX) : 0,
      y: allowPointerMotion ? clampI32(accumMouseY) : 0,
      z: allowPointerMotion ? clampI32(accumMouseZ) : 0,
      left: mouseButtons.value.left,
      right: mouseButtons.value.right,
      middle: mouseButtons.value.middle,
      kb: keyBitmask,
    }

    clearMotionAccumulators()
    return packet
  }

  function flushPacket(): void {
    if (!captureEnabled.value) return
    const packet = buildCurrentPacket()
    if (!isPacketChanged(packet)) return
    lastPacket = packet
    void pushPacketToBackend(packet)
  }

  function sendLoop(): void {
    rafId = window.requestAnimationFrame(sendLoop)
    flushPacket()
  }

  function ensureSendLoop(): void {
    if (rafId !== null) return
    rafId = window.requestAnimationFrame(sendLoop)
  }

  function stopSendLoop(): void {
    if (rafId === null) return
    window.cancelAnimationFrame(rafId)
    rafId = null
  }

  function onPointerLockChange(): void {
    pointerLocked.value = document.pointerLockElement !== null
  }

  function onPointerLockError(): void {
    pointerLocked.value = false
  }

  function onMouseMove(event: MouseEvent): void {
    if (!pointerLocked.value) return
    accumMouseX += event.movementX
    accumMouseY += event.movementY
    activity.value.moveAt = Date.now()
  }

  function onMouseDown(event: MouseEvent): void {
    setMouseButton(event.button, true)
  }

  function onMouseUp(event: MouseEvent): void {
    setMouseButton(event.button, false)
  }

  function onWheel(event: WheelEvent): void {
    if (!pointerLocked.value) return
    // What: 捕抓态拦截滚轮并累积到 Z 轴。
    // Why: 防止页面滚动并保证滚轮控制可进入统一协议通道。
    event.preventDefault()
    accumMouseZ += event.deltaY
    activity.value.wheelAt = Date.now()
  }

  function shouldBlockQuickAction(event: KeyboardEvent, wasPrevented: boolean): boolean {
    if (uiPanelStore.isOpen) return true
    if (isHudEditing.value) return true
    if (shouldIgnoreKeyboardTarget(event.target)) return true
    return wasPrevented
  }

  function onKeyDown(event: KeyboardEvent): void {
    if (!pointerLocked.value && shouldIgnoreKeyboardTarget(event.target)) return
    // What: 在非捕抓态跳过已被系统热键拦截的按键。
    // Why: 防止 U/Q/P 等 UI 系统键污染控制位图并误下发后端。
    const wasPrevented = !pointerLocked.value && event.defaultPrevented
    if (wasPrevented) return
    if (pointerLocked.value) event.preventDefault()
    const token = eventToKeyToken(event)
    if (!token) return

    if (!pressedTokens.value.has(token) && token === buyAmmoToken.value && !shouldBlockQuickAction(event, wasPrevented)) {
      // What: 将一键买弹绑定为 keydown 边沿触发。
      // Why: 按住按键不应重复发送买弹命令，否则实战里容易瞬间误买多次。
      void triggerQuickBuyAmmo()
    }

    addPressedToken(token)
  }

  function onKeyUp(event: KeyboardEvent): void {
    if (!pointerLocked.value && shouldIgnoreKeyboardTarget(event.target)) return
    // What: 与 keydown 保持同一拦截规则。
    // Why: 避免出现 keydown 未入集合但 keyup 误删导致的状态抖动。
    if (!pointerLocked.value && event.defaultPrevented) return
    if (pointerLocked.value) event.preventDefault()
    const token = eventToKeyToken(event)
    if (!token) return
    removePressedToken(token)
  }

  function onWindowBlur(): void {
    clearInputState()
  }

  function onContextMenu(event: MouseEvent): void {
    if (!pointerLocked.value) return
    event.preventDefault()
  }

  function bindListeners(): void {
    document.addEventListener('pointerlockchange', onPointerLockChange)
    document.addEventListener('pointerlockerror', onPointerLockError)
    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mousedown', onMouseDown)
    window.addEventListener('mouseup', onMouseUp)
    window.addEventListener('wheel', onWheel, { passive: false })
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)
    window.addEventListener('blur', onWindowBlur)
    window.addEventListener('contextmenu', onContextMenu)
  }

  function unbindListeners(): void {
    document.removeEventListener('pointerlockchange', onPointerLockChange)
    document.removeEventListener('pointerlockerror', onPointerLockError)
    window.removeEventListener('mousemove', onMouseMove)
    window.removeEventListener('mousedown', onMouseDown)
    window.removeEventListener('mouseup', onMouseUp)
    window.removeEventListener('wheel', onWheel)
    window.removeEventListener('keydown', onKeyDown)
    window.removeEventListener('keyup', onKeyUp)
    window.removeEventListener('blur', onWindowBlur)
    window.removeEventListener('contextmenu', onContextMenu)
  }

  function install(resolver?: PointerTargetResolver): void {
    if (installed.value) return
    pointerTargetResolver = resolver ?? (() => document.documentElement)
    bindListeners()
    ensureSendLoop()
    installed.value = true
  }

  function uninstall(): void {
    if (!installed.value) return
    stopSendLoop()
    unbindListeners()
    clearInputState()
    clearQuickActionFeedbackTimer()
    quickActionFeedback.value = ''
    installed.value = false
    pointerLocked.value = false
  }

  async function requestPointerLock(): Promise<void> {
    if (!captureEnabled.value) return
    const target = pointerTargetResolver?.()
    if (!target || typeof target.requestPointerLock !== 'function') return
    await target.requestPointerLock()
  }

  async function exitPointerLock(): Promise<void> {
    if (document.pointerLockElement && typeof document.exitPointerLock === 'function') {
      await document.exitPointerLock()
    }
  }

  async function togglePointerLock(): Promise<void> {
    if (pointerLocked.value) {
      await exitPointerLock()
      return
    }
    await requestPointerLock()
  }

  function isTokenPressed(token: string): boolean {
    return pressedTokens.value.has(token)
  }

  function getTokenActiveAt(token: string): number {
    return activity.value.keyActiveAt[token] ?? 0
  }

  return {
    installed,
    pointerLocked,
    pointerSupported,
    captureEnabled,
    showCaptureButton,
    mouseButtons,
    activity,
    pressedKeyTokens,
    backendError,
    quickActionFeedback,
    quickActionFeedbackTone,
    install,
    uninstall,
    requestPointerLock,
    exitPointerLock,
    togglePointerLock,
    isTokenPressed,
    getTokenActiveAt,
  }
})
