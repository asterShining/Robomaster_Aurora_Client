// What: 保留 editor 页签类型兼容历史状态值。
// Why: 即使 UI 编辑迁移为独立浮窗，也避免旧状态数据触发类型不兼容。
export type PanelTab =
  | "mqtt"
  | "keymap"
  | "hud"
  | "layout"
  | "editor"
  | "diagnostic";

// What: 定义设置面板里真正可切换的双视频源枚举。
// Why: 当前客户端已经收敛为 official/custom 两条视频链，配置层必须和后端切源接口保持 1:1 对齐。
export type VideoSource = "official" | "custom";

// What: 定义客户端当前应以哪台常用机器人身份接入 MQTT。
// Why: 裁判系统 broker 以机器人 ID 校验 clientID，身份枚举必须覆盖常用机器人并禁止自由字符串漂进运行时。
export type MQTTRobotIdentity =
  | "red_hero"
  | "red_engineer"
  | "red_infantry_3"
  | "red_infantry_4"
  | "red_infantry_5"
  | "red_drone"
  | "red_sentry"
  | "blue_hero"
  | "blue_engineer"
  | "blue_infantry_3"
  | "blue_infantry_4"
  | "blue_infantry_5"
  | "blue_drone"
  | "blue_sentry";

// 兼容旧代码和旧配置命名；新逻辑统一使用 MQTTRobotIdentity。
export type MQTTHeroIdentity = MQTTRobotIdentity;

export interface MQTTRobotIdentityOption {
  id: MQTTRobotIdentity;
  side: "red" | "blue";
  label: string;
  shortLabel: string;
  clientId: number;
}

export const MQTT_ROBOT_IDENTITY_OPTIONS: MQTTRobotIdentityOption[] = [
  { id: "red_hero", side: "red", label: "红方英雄", shortLabel: "英雄", clientId: 1 },
  { id: "red_engineer", side: "red", label: "红方工程", shortLabel: "工程", clientId: 2 },
  { id: "red_infantry_3", side: "red", label: "红方步兵 3", shortLabel: "步兵 3", clientId: 3 },
  { id: "red_infantry_4", side: "red", label: "红方步兵 4", shortLabel: "步兵 4", clientId: 4 },
  { id: "red_infantry_5", side: "red", label: "红方步兵 5", shortLabel: "步兵 5", clientId: 5 },
  { id: "red_drone", side: "red", label: "红方空中", shortLabel: "空中", clientId: 6 },
  { id: "red_sentry", side: "red", label: "红方哨兵", shortLabel: "哨兵", clientId: 7 },
  { id: "blue_hero", side: "blue", label: "蓝方英雄", shortLabel: "英雄", clientId: 101 },
  { id: "blue_engineer", side: "blue", label: "蓝方工程", shortLabel: "工程", clientId: 102 },
  { id: "blue_infantry_3", side: "blue", label: "蓝方步兵 3", shortLabel: "步兵 3", clientId: 103 },
  { id: "blue_infantry_4", side: "blue", label: "蓝方步兵 4", shortLabel: "步兵 4", clientId: 104 },
  { id: "blue_infantry_5", side: "blue", label: "蓝方步兵 5", shortLabel: "步兵 5", clientId: 105 },
  { id: "blue_drone", side: "blue", label: "蓝方空中", shortLabel: "空中", clientId: 106 },
  { id: "blue_sentry", side: "blue", label: "蓝方哨兵", shortLabel: "哨兵", clientId: 107 },
];

const MQTT_ROBOT_IDENTITY_BY_ID = new Map(
  MQTT_ROBOT_IDENTITY_OPTIONS.map((option) => [option.id, option]),
);

const MQTT_ROBOT_IDENTITY_BY_CLIENT_ID = new Map(
  MQTT_ROBOT_IDENTITY_OPTIONS.map((option) => [String(option.clientId), option.id]),
);

const MQTT_ROBOT_IDENTITY_ALIASES: Record<string, MQTTRobotIdentity> = {
  red_infantry: "red_infantry_3",
  blue_infantry: "blue_infantry_3",
};

export interface MqttMetricItem {
  id: string;
  label: string;
  path: string;
  desc: string;
  enabled: boolean;
}

export interface KeyBindingItem {
  action: string;
  key: string;
  category: "move" | "combat" | "system";
}

export interface HudDisplayConfig {
  showTopBar: boolean;
  showPoseCard: boolean;
  showStatusCard: boolean;
  scale: number;
  opacity: number;
  glow: number;
  themeSide: "red" | "blue";
  crosshairColor: string;
  awareness: HudAwarenessConfig;
}

// What: 定义态势增强器的分项开关。
// Why: 将“显示基础数据”和“强化提醒策略”拆开，方便用户按实战习惯裁剪干扰信息。
export interface HudAwarenessConfig {
  enabled: boolean;
  showLowHpAlert: boolean;
  showHeatAlert: boolean;
  showAmmoGraph: boolean;
  showThreatFusion: boolean;
  showCapacitorAlert: boolean;
  showArmorCue: boolean;
  showMatchCue: boolean;
}

export interface InputCaptureConfig {
  captureEnabled: boolean;
  showCaptureButton: boolean;
}

// What: 定义一键动作预设参数。
// Why: 将“触发热键”和“动作目标值”拆开保存，便于比赛现场快速改量而不用重新改键。
export interface QuickActionConfig {
  buyAmmoPreset: number;
}

export interface UiPanelDraft {
  videoSource: VideoSource;
  mqttRobotIdentity: MQTTRobotIdentity;
  // 兼容 v3 及更早配置字段；写出时保持镜像，读取时以 mqttRobotIdentity 优先。
  mqttHeroIdentity: MQTTRobotIdentity;
  showDiagnosticsCard: boolean;
  showRobotStatusCard: boolean;
  showFireControlCard: boolean;
  mqttRoute: string;
  mqttFilter: string;
  selectedMetricId: string;
  metrics: MqttMetricItem[];
  keymap: KeyBindingItem[];
  hud: HudDisplayConfig;
  input: InputCaptureConfig;
  quickActions: QuickActionConfig;
  resolution: string;
}

export interface ClientUiConfigSnapshot {
  version: number;
  uiPanel: UiPanelDraft;
}

export const UI_PANEL_CONFIG_VERSION = 4;
export const UI_PANEL_STORAGE_KEY = `rm-client-ui-panel:v${UI_PANEL_CONFIG_VERSION}`;
export const UI_PANEL_LEGACY_STORAGE_KEYS = ["rm-client-ui-panel:v3"] as const;
export const HUD_SCALE_PRESETS = [0.85, 0.95, 1, 1.1, 1.2] as const;
export const RESOLUTION_PRESETS = [
  "2560x1600",
  "2560x1440",
  "1920x1200",
  "1920x1080",
  "1680x1050",
  "1600x900",
  "1440x900",
  "1366x768",
  "1280x720",
] as const;
const QUICK_ACTION_PRESET_MIN = 1;
const QUICK_ACTION_PRESET_MAX = 65535;

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

// What: 将快捷动作预设值钳制到协议安全范围。
// Why: 后端编码为 uint16，前端必须先去掉负数、小数和过大值，避免发出无效命令。
function normalizeQuickActionPreset(value: unknown, fallback: number): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return fallback;
  return clamp(
    Math.round(parsed),
    QUICK_ACTION_PRESET_MIN,
    QUICK_ACTION_PRESET_MAX,
  );
}

function normalizeUiColor(value: unknown, fallback: string): string {
  return typeof value === "string" && value.trim() ? value : fallback;
}

function normalizeVideoSource(
  value: unknown,
  fallback: VideoSource,
): VideoSource {
  if (value === "official" || value === "custom") return value;
  return fallback;
}

export function normalizeMQTTRobotIdentity(
  value: unknown,
  fallback: MQTTRobotIdentity,
): MQTTRobotIdentity {
  if (typeof value !== "string") return fallback;

  const normalized = value.trim().toLowerCase();
  if (MQTT_ROBOT_IDENTITY_BY_ID.has(normalized as MQTTRobotIdentity)) {
    return normalized as MQTTRobotIdentity;
  }
  if (MQTT_ROBOT_IDENTITY_ALIASES[normalized]) {
    return MQTT_ROBOT_IDENTITY_ALIASES[normalized];
  }
  const byClientId = MQTT_ROBOT_IDENTITY_BY_CLIENT_ID.get(normalized);
  if (byClientId) return byClientId;
  return fallback;
}

export const normalizeMQTTHeroIdentity = normalizeMQTTRobotIdentity;

export function resolveMQTTRobotIdentityOption(
  identity: MQTTRobotIdentity,
): MQTTRobotIdentityOption {
  return MQTT_ROBOT_IDENTITY_BY_ID.get(identity) ?? MQTT_ROBOT_IDENTITY_BY_ID.get("blue_hero")!;
}

export function resolveMQTTRobotIdentityLabel(identity: MQTTRobotIdentity): string {
  return resolveMQTTRobotIdentityOption(identity).label;
}

export function isHeroRobotIdentity(identity: MQTTRobotIdentity): boolean {
  return identity === "red_hero" || identity === "blue_hero";
}

// What: 按机器人身份收敛视频源。
// Why: 非英雄机器人没有自定义视频源，不能让旧配置或误触把主画面切到等待 0x0310 的黑幕。
export function normalizeVideoSourceForRobotIdentity(
  source: VideoSource,
  identity: MQTTRobotIdentity,
): VideoSource {
  return isHeroRobotIdentity(identity) ? source : "official";
}

function snapHudScaleToPreset(value: number): number {
  let nearest: number = HUD_SCALE_PRESETS[0];
  let distance = Math.abs(value - nearest);
  for (const item of HUD_SCALE_PRESETS) {
    const nextDistance = Math.abs(value - item);
    if (nextDistance < distance) {
      distance = nextDistance;
      nearest = item;
    }
  }
  return nearest;
}

export function createDefaultMetrics(): MqttMetricItem[] {
  return [
    {
      id: "game-status-stage",
      label: "比赛阶段",
      path: "gameStatus.currentStage",
      desc: "用于顶部中条阶段展示",
      enabled: true,
    },
    {
      id: "game-status-countdown",
      label: "阶段剩余时间",
      path: "gameStatus.stageCountdownSec",
      desc: "用于顶部中条剩余时间展示",
      enabled: true,
    },
    {
      id: "game-status-red-score",
      label: "红方得分",
      path: "gameStatus.redScore",
      desc: "用于顶部中条比分展示",
      enabled: true,
    },
    {
      id: "game-status-blue-score",
      label: "蓝方得分",
      path: "gameStatus.blueScore",
      desc: "用于顶部中条比分展示",
      enabled: true,
    },
    {
      id: "red-base-health",
      label: "红方基地血量",
      path: "matchState.red.baseHp",
      desc: "用于顶部基地血条显示",
      enabled: true,
    },
    {
      id: "blue-base-health",
      label: "蓝方基地血量",
      path: "matchState.blue.baseHp",
      desc: "用于顶部基地血条显示",
      enabled: true,
    },
    {
      id: "red-outpost-health",
      label: "红方前哨站血量",
      path: "matchState.red.outpostHp",
      desc: "用于前哨站状态展示",
      enabled: true,
    },
    {
      id: "blue-outpost-health",
      label: "蓝方前哨站血量",
      path: "matchState.blue.outpostHp",
      desc: "用于前哨站状态展示",
      enabled: true,
    },
    {
      id: "red-total-damage",
      label: "红方总伤害",
      path: "matchState.red.totalDamage",
      desc: "用于总伤害展示",
      enabled: false,
    },
    {
      id: "blue-total-damage",
      label: "蓝方总伤害",
      path: "matchState.blue.totalDamage",
      desc: "用于总伤害展示",
      enabled: false,
    },
  ];
}

export function createDefaultKeymap(): KeyBindingItem[] {
  return [
    { action: "前进", key: "KeyW", category: "move" },
    { action: "后退", key: "KeyS", category: "move" },
    { action: "左移", key: "KeyA", category: "move" },
    { action: "右移", key: "KeyD", category: "move" },
    { action: "主武器", key: "Mouse0", category: "combat" },
    { action: "副武器", key: "Mouse1", category: "combat" },
    // What: 默认给一键买弹预留独立热键。
    // Why: 让操作手开箱即用，不必先自己录入 O 键才能验证快捷补给链路。
    { action: "购买弹丸", key: "KeyO", category: "combat" },
    { action: "打开设置面板", key: "KeyP", category: "system" },
  ];
}

export function createDefaultHudDisplayConfig(): HudDisplayConfig {
  return {
    showTopBar: true,
    showPoseCard: true,
    showStatusCard: true,
    scale: 1,
    opacity: 0.95,
    glow: 0.8,
    // What: 默认使用红方主题。
    // Why: 与当前实车主视角默认渲染保持一致，降低首次认知成本。
    themeSide: "red",
    // What: 为中心准星提供独立颜色配置。
    // Why: 瞄准组件不能与其余 HUD 色板强绑定，用户需要按视野习惯单独微调。
    crosshairColor: "#EAF3FF",
    // What: 第一阶段默认启用态势增强器完整闭环。
    // Why: 用户诉求就是压缩噪声并高亮关键风险，因此默认应直接进入增强态而不是关闭。
    awareness: createDefaultHudAwarenessConfig(),
  };
}

export function createDefaultHudAwarenessConfig(): HudAwarenessConfig {
  return {
    enabled: true,
    showLowHpAlert: true,
    showHeatAlert: true,
    showAmmoGraph: true,
    showThreatFusion: true,
    showCapacitorAlert: true,
    showArmorCue: true,
    showMatchCue: true,
  };
}

export function createDefaultInputCaptureConfig(): InputCaptureConfig {
  return {
    captureEnabled: true,
    showCaptureButton: true,
  };
}

export function createDefaultQuickActionConfig(): QuickActionConfig {
  return {
    buyAmmoPreset: 20,
  };
}

export function createDefaultUiPanelDraft(): UiPanelDraft {
  return {
    videoSource: "official",
    mqttRobotIdentity: "blue_hero",
    mqttHeroIdentity: "blue_hero",
    showDiagnosticsCard: true,
    showRobotStatusCard: true,
    showFireControlCard: true,
    mqttRoute: "rm/global/topic",
    mqttFilter: "",
    selectedMetricId: "game-status-stage",
    metrics: createDefaultMetrics(),
    keymap: createDefaultKeymap(),
    hud: createDefaultHudDisplayConfig(),
    input: createDefaultInputCaptureConfig(),
    quickActions: createDefaultQuickActionConfig(),
    resolution: "2560x1600",
  };
}

export function createDefaultClientUiSnapshot(): ClientUiConfigSnapshot {
  return {
    version: UI_PANEL_CONFIG_VERSION,
    uiPanel: createDefaultUiPanelDraft(),
  };
}

function normalizeMetric(
  raw: Partial<MqttMetricItem>,
  fallback: MqttMetricItem,
): MqttMetricItem {
  return {
    id: String(raw.id ?? fallback.id),
    label: String(raw.label ?? fallback.label),
    path: String(raw.path ?? fallback.path),
    desc: String(raw.desc ?? fallback.desc),
    enabled: typeof raw.enabled === "boolean" ? raw.enabled : fallback.enabled,
  };
}

function normalizeKeyBinding(
  raw: Partial<KeyBindingItem>,
  fallback: KeyBindingItem,
): KeyBindingItem {
  const category = raw.category;
  return {
    action: String(raw.action ?? fallback.action),
    key: String(raw.key ?? fallback.key),
    category:
      category === "move" || category === "combat" || category === "system"
        ? category
        : fallback.category,
  };
}

// mergeUiPanelDraft 将外部配置安全并入默认配置。
// What: 对本地存档或后端回包做字段级归一化。
// Why: 防止旧版本配置缺字段或类型漂移时把 UI 渲染链路带崩。
export function mergeUiPanelDraft(
  raw: Partial<UiPanelDraft> | null | undefined,
): UiPanelDraft {
  const defaults = createDefaultUiPanelDraft();
  if (!raw) return defaults;

  const rawIdentity = raw.mqttRobotIdentity ?? raw.mqttHeroIdentity;
  const mqttRobotIdentity = normalizeMQTTRobotIdentity(
    rawIdentity,
    defaults.mqttRobotIdentity,
  );

  const safeMetrics = Array.isArray(raw.metrics)
    ? raw.metrics.map((item, idx) =>
        normalizeMetric(
          item ?? {},
          defaults.metrics[idx] ?? defaults.metrics[0],
        ),
      )
    : defaults.metrics;

  const safeKeymap = Array.isArray(raw.keymap)
    ? raw.keymap.map((item, idx) =>
        normalizeKeyBinding(
          item ?? {},
          defaults.keymap[idx] ?? defaults.keymap[0],
        ),
      )
    : defaults.keymap;

  const videoSource = normalizeVideoSourceForRobotIdentity(
    normalizeVideoSource(raw.videoSource, defaults.videoSource),
    mqttRobotIdentity,
  );

  return {
    videoSource,
    mqttRobotIdentity,
    mqttHeroIdentity: mqttRobotIdentity,
    resolution:
      typeof raw.resolution === "string" && (RESOLUTION_PRESETS as readonly string[]).includes(raw.resolution)
        ? raw.resolution
        : defaults.resolution,
    showDiagnosticsCard:
      typeof raw.showDiagnosticsCard === "boolean"
        ? raw.showDiagnosticsCard
        : defaults.showDiagnosticsCard,
    showRobotStatusCard:
      typeof raw.showRobotStatusCard === "boolean"
        ? raw.showRobotStatusCard
        : defaults.showRobotStatusCard,
    showFireControlCard:
      typeof raw.showFireControlCard === "boolean"
        ? raw.showFireControlCard
        : defaults.showFireControlCard,
    mqttRoute:
      typeof raw.mqttRoute === "string" ? raw.mqttRoute : defaults.mqttRoute,
    mqttFilter:
      typeof raw.mqttFilter === "string" ? raw.mqttFilter : defaults.mqttFilter,
    selectedMetricId:
      typeof raw.selectedMetricId === "string"
        ? raw.selectedMetricId
        : defaults.selectedMetricId,
    metrics: safeMetrics,
    keymap: safeKeymap,
    hud: {
      showTopBar:
        typeof raw.hud?.showTopBar === "boolean"
          ? raw.hud.showTopBar
          : defaults.hud.showTopBar,
      showPoseCard:
        typeof raw.hud?.showPoseCard === "boolean"
          ? raw.hud.showPoseCard
          : defaults.hud.showPoseCard,
      showStatusCard:
        typeof raw.hud?.showStatusCard === "boolean"
          ? raw.hud.showStatusCard
          : defaults.hud.showStatusCard,
      // What: HUD 缩放吸附到预设档位。
      // Why: 预设档位更便于比赛现场快速切换，避免滑杆微调导致布局不一致。
      scale: snapHudScaleToPreset(
        clamp(Number(raw.hud?.scale ?? defaults.hud.scale), 0.8, 1.2),
      ),
      opacity: clamp(Number(raw.hud?.opacity ?? defaults.hud.opacity), 0.5, 1),
      glow: clamp(Number(raw.hud?.glow ?? defaults.hud.glow), 0, 1),
      themeSide: raw.hud?.themeSide === "blue" ? "blue" : "red",
      crosshairColor: normalizeUiColor(
        raw.hud?.crosshairColor,
        defaults.hud.crosshairColor,
      ),
      // What: 对态势增强器开关做字段级归一化。
      // Why: 旧版本本地配置没有 awareness 节点时，也要自动补齐为可用默认值。
      awareness: {
        enabled:
          typeof raw.hud?.awareness?.enabled === "boolean"
            ? raw.hud.awareness.enabled
            : defaults.hud.awareness.enabled,
        showLowHpAlert:
          typeof raw.hud?.awareness?.showLowHpAlert === "boolean"
            ? raw.hud.awareness.showLowHpAlert
            : defaults.hud.awareness.showLowHpAlert,
        showHeatAlert:
          typeof raw.hud?.awareness?.showHeatAlert === "boolean"
            ? raw.hud.awareness.showHeatAlert
            : defaults.hud.awareness.showHeatAlert,
        showAmmoGraph:
          typeof raw.hud?.awareness?.showAmmoGraph === "boolean"
            ? raw.hud.awareness.showAmmoGraph
            : defaults.hud.awareness.showAmmoGraph,
        showThreatFusion:
          typeof raw.hud?.awareness?.showThreatFusion === "boolean"
            ? raw.hud.awareness.showThreatFusion
            : defaults.hud.awareness.showThreatFusion,
        showCapacitorAlert:
          typeof raw.hud?.awareness?.showCapacitorAlert === "boolean"
            ? raw.hud.awareness.showCapacitorAlert
            : defaults.hud.awareness.showCapacitorAlert,
        showArmorCue:
          typeof raw.hud?.awareness?.showArmorCue === "boolean"
            ? raw.hud.awareness.showArmorCue
            : defaults.hud.awareness.showArmorCue,
        showMatchCue:
          typeof raw.hud?.awareness?.showMatchCue === "boolean"
            ? raw.hud.awareness.showMatchCue
            : defaults.hud.awareness.showMatchCue,
      },
    },
    input: {
      captureEnabled:
        typeof raw.input?.captureEnabled === "boolean"
          ? raw.input.captureEnabled
          : defaults.input.captureEnabled,
      showCaptureButton:
        typeof raw.input?.showCaptureButton === "boolean"
          ? raw.input.showCaptureButton
          : defaults.input.showCaptureButton,
    },
    quickActions: {
      // What: 对快捷动作预设做字段级归一化。
      // Why: 旧版本配置没有该节点时也要自动补齐，且必须保证值能安全编码到后端协议。
      buyAmmoPreset: normalizeQuickActionPreset(
        raw.quickActions?.buyAmmoPreset,
        defaults.quickActions.buyAmmoPreset,
      ),
    },
  };
}

export function mergeUiPanelSnapshot(
  raw: Partial<ClientUiConfigSnapshot> | null | undefined,
): ClientUiConfigSnapshot {
  const defaults = createDefaultClientUiSnapshot();
  if (!raw || typeof raw !== "object") return defaults;
  return {
    version: typeof raw.version === "number" ? raw.version : defaults.version,
    uiPanel: mergeUiPanelDraft(raw.uiPanel),
  };
}
