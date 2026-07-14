import type { KeyBindingItem } from '../types/ui'

// What: 集中声明全局动作名常量。
// Why: 让设置页、输入捕抓和 HUD 热键判断共享同一语义键，避免字符串散落后改一处漏多处。
export const ACTION_MOVE_FORWARD = '前进'
export const ACTION_MOVE_BACKWARD = '后退'
export const ACTION_MOVE_LEFT = '左移'
export const ACTION_MOVE_RIGHT = '右移'
export const ACTION_PURCHASE_AMMO = '购买弹丸'
export const ACTION_OPEN_SETTINGS = '打开设置面板'

function normalizeAlphabetToken(raw: string): string {
  const value = raw.trim().toUpperCase()
  if (/^[A-Z]$/.test(value)) return `Key${value}`
  if (/^[0-9]$/.test(value)) return `Digit${value}`
  return raw.trim()
}

// normalizeKeyToken 统一键位字符串格式。
// What: 把输入框文本、历史存档值和事件码归一到同一 token。
// Why: 避免热键判断、位图编码和 UI 显示使用不同格式导致状态错配。
export function normalizeKeyToken(raw: string): string {
  const value = normalizeAlphabetToken(raw)
  if (!value) return ''

  const upper = value.toUpperCase()
  if (upper === 'CTRL' || upper === 'CONTROL') return 'ControlLeft'
  if (upper === 'SHIFT') return 'ShiftLeft'
  if (upper === 'ALT') return 'AltLeft'
  if (upper === 'ESC' || upper === 'ESCAPE') return 'Escape'
  if (upper === 'SPACE') return 'Space'
  if (upper === 'TAB') return 'Tab'
  if (upper === 'ENTER' || upper === 'RETURN') return 'Enter'
  if (upper === 'MOUSE0' || upper === 'MOUSE1' || upper === 'MOUSE2') return upper[0] + upper.slice(1).toLowerCase()

  if (
    value.startsWith('Key') ||
    value.startsWith('Digit') ||
    value.startsWith('Arrow') ||
    value.startsWith('Control') ||
    value.startsWith('Shift') ||
    value.startsWith('Alt')
  ) {
    return value
  }
  return normalizeAlphabetToken(value)
}

// keyTokenToDisplayLabel 将标准 token 转为 HUD 显示文案。
// What: 生成紧凑可读的按键标签。
// Why: 让右下角键鼠栏与设置页录入结果保持一致，降低理解成本。
export function keyTokenToDisplayLabel(token: string): string {
  if (!token) return '-'
  if (token.startsWith('Key')) return token.slice(3)
  if (token.startsWith('Digit')) return token.slice(5)
  if (token === 'ControlLeft' || token === 'ControlRight') return 'Ctrl'
  if (token === 'ShiftLeft' || token === 'ShiftRight') return 'Shift'
  if (token === 'AltLeft' || token === 'AltRight') return 'Alt'
  if (token === 'Escape') return 'Esc'
  if (token === 'Space') return 'Space'
  if (token === 'ArrowUp') return '↑'
  if (token === 'ArrowDown') return '↓'
  if (token === 'ArrowLeft') return '←'
  if (token === 'ArrowRight') return '→'
  if (token === 'Mouse0') return 'LMB'
  if (token === 'Mouse1') return 'RMB'
  if (token === 'Mouse2') return 'MMB'
  return token
}

export function eventToKeyToken(event: KeyboardEvent): string {
  return normalizeKeyToken(event.code || event.key)
}

export function shouldIgnoreKeyboardTarget(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null
  if (!el) return false
  const tag = el.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  return !!el.closest('[contenteditable="true"]')
}

export function resolveBindingConflict(
  keymap: KeyBindingItem[],
  action: string,
  nextToken: string
): KeyBindingItem[] {
  return keymap.map((item) => {
    if (item.action === action) return { ...item, key: nextToken }
    if (item.key === nextToken && nextToken) return { ...item, key: '' }
    return item
  })
}

export function findActionKeyToken(keymap: KeyBindingItem[], action: string): string {
  const matched = keymap.find((item) => item.action === action)?.key ?? ''
  return normalizeKeyToken(matched)
}

export function isEventMatchToken(event: KeyboardEvent, token: string): boolean {
  return eventToKeyToken(event) === normalizeKeyToken(token)
}

export function createActionTokenMap(keymap: KeyBindingItem[]): Record<string, string> {
  return keymap.reduce<Record<string, string>>((map, item) => {
    map[item.action] = normalizeKeyToken(item.key)
    return map
  }, {})
}

// buildKeyboardBitmask 生成 RMCP 键盘位图值。
// What: 根据当前按键集合与映射规则构造 uint32 keyboard_value。
// Why: 与后端协议字段严格对齐，避免前后端位图解释不一致。
export function buildKeyboardBitmask(pressedTokens: Set<string>, keymap: KeyBindingItem[]): number {
  const actionMap = createActionTokenMap(keymap)

  let bitmask = 0
  if (pressedTokens.has(actionMap[ACTION_MOVE_FORWARD])) bitmask |= 1
  if (pressedTokens.has(actionMap[ACTION_MOVE_BACKWARD])) bitmask |= 2
  if (pressedTokens.has(actionMap[ACTION_MOVE_LEFT])) bitmask |= 4
  if (pressedTokens.has(actionMap[ACTION_MOVE_RIGHT])) bitmask |= 8

  if (pressedTokens.has('ShiftLeft') || pressedTokens.has('ShiftRight')) bitmask |= 16
  if (pressedTokens.has('ControlLeft') || pressedTokens.has('ControlRight')) bitmask |= 32
  return bitmask
}
