export type GimbalMode =
  | "idle"
  | "follow"
  | "lock"
  | "gyro"
  | "calibrating"
  | "error";

export type LinkQuality = "excellent" | "good" | "fair" | "poor" | "offline";
export type TeamRole = "hero" | "engineer" | "infantry" | "sentry" | "unknown";
// What: 收口当前客户端真正支持的双视频源枚举。
// Why: 前后端都会围绕 official/custom 二选一工作，必须先在类型层固定下来，避免 UI 再出现第三种伪入口。
export type VideoSource = "official" | "custom";
// What: 统一定义遥测字段的数据来源标记。
// Why: 状态卡必须区分“真实后端数据”和“前端仿真占位”，否则 UI 文案再准确也会继续误导用户。
export type TelemetryOrigin = "backend" | "simulated" | "unknown";
// What: 收口原生视频层显示状态枚举。
// Why: 连接状态卡、设置面板和遮幕提示都依赖同一组状态语义，不能在多个文件里各自维护一份字符串集合。
export type VideoDisplayState =
  | "live"
  | "waiting_source"
  | "waiting_frame"
  | "resyncing"
  | "stalled"
  | "runtime_error";

export interface ChassisData {
  powerW: number;
  currentA: number;
  positionX: number;
  positionY: number;
  positionZ: number;
  headingDeg: number;
  updatedAt: number;
  origin: TelemetryOrigin;
}

export interface GimbalData {
  pitchDeg: number;
  yawDeg: number;
  mode: GimbalMode;
  isOnline: boolean;
  updatedAt: number;
  origin: TelemetryOrigin;
}

export interface ConnectionState {
  backendConnected: boolean;
  videoConnected: boolean;
  controlLinkConnected: boolean;
  activeSource: VideoSource;
  pipSource: VideoSource;
  officialAvailable: boolean;
  customAvailable: boolean;
  officialVideoConnected: boolean;
  customVideoConnected: boolean;
  videoDisplayState: VideoDisplayState;
  officialDisplayState: VideoDisplayState;
  customDisplayState: VideoDisplayState;
  latencyMs: number;
  officialLatencyMs: number;
  customLatencyMs: number;
  officialPacketRateHz: number;
  officialFrameRateHz: number;
  officialDropRate: number;
  customBlockRateHz: number;
  customAUFPS: number;
  customDropRate: number;
  customCorruptRate: number;
  linkQuality: LinkQuality;
  stale: boolean;
  lastHeartbeatAt: number;
  udpDropFrames: number;
  decodeDropFrames: number;
  corruptFrames: number;
  decoderResets: number;
  headerOrder: string;
  message: string;
  officialMessage: string;
  customMessage: string;
}

// What: 统一定义战斗态数据契约。
// Why: 血量、热量、弹量、电容和补给可用态都要被多个 HUD 组件复用，必须先收口到同一状态源。
export interface CombatData {
  hp: number;
  maxHp: number;
  ammo: number;
  maxAmmo: number;
  heat: number;
  maxHeat: number;
  lastProjectileFireRate: number;
  chassisEnergy: number;
  maxChassisEnergy: number;
  bufferEnergy: number;
  maxBufferEnergy: number;
  currentExperience: number;
  experienceForUpgrade: number;
  totalProjectilesFired: number;
  isOutOfCombat: boolean;
  outOfCombatCountdownSec: number;
  canRemoteHeal: boolean;
  bulletSpeed: number;
  capacitorPct: number;
  canRemoteAmmo: boolean;
  updatedAt: number;
  origin: TelemetryOrigin;
}

export interface TeamRobotState {
  robotId: number;
  role: TeamRole;
  roleLabel: string;
  hp: number;
  maxHp: number;
  avatarKey: string;
  online: boolean;
}

export interface TeamSideState {
  baseHp: number;
  baseMaxHp: number;
  baseStatus: number;
  baseShield: number;
  outpostHp: number;
  outpostStatus: number;
  totalDamage: number;
  units: TeamRobotState[];
}

// What: 顶部比赛态统一收口真实 GameStatus 与红蓝血量编组。
// Why: 顶部中条和左右血量编组来自不同后端主题，必须先在类型层明确字段边界，避免前端再混入 round/economy 假字段。
export interface MatchState {
  red: TeamSideState;
  blue: TeamSideState;
  globalStatusReady: boolean;
  gameStatusReady: boolean;
  currentRound: number;
  totalRounds: number;
  redScore: number;
  blueScore: number;
  currentStage: number;
  stageCountdownSec: number;
  stageElapsedSec: number;
  isPaused: boolean;
  updatedAt: number;
  origin: TelemetryOrigin;
}

// What: 定义空中支援同步态数据契约。
// Why: 经济趋势与起飞建议都要复用这组字段，必须先在类型层收口，避免组件各自猜协议含义。
export interface AirSupportState {
  airSupportStatus: number;
  leftTimeSec: number;
  costCoins: number;
  isBeingTargeted: number;
  shooterStatus: number;
  updatedAt: number;
}

export type RadarContactSide = "ally" | "enemy" | "neutral";
// What: 统一定义雷达链路来源状态。
// Why: 用户已明确要求“没真实来源就显示未接入”，因此这里不能再默认回退到 simulated。
export type RadarDataSource = "unknown" | "backend";

export interface RadarContact {
  id: string;
  x: number;
  y: number;
  velocity: number;
  side: RadarContactSide;
  confidence: number;
  headingDeg: number;
  highlighted: boolean;
  updatedAt: number;
}

export interface RadarData {
  rangeM: number;
  sweepDeg: number;
  source: RadarDataSource;
  contacts: RadarContact[];
  updatedAt: number;
}

export interface VisionTarget {
  id: string;
  xNorm: number;
  yNorm: number;
  confidence: number;
  side: RadarContactSide;
  armorLabel: string;
  updatedAt: number;
}

export interface VisionData {
  targets: VisionTarget[];
  updatedAt: number;
}

export interface AwarenessTargetCue {
  source: "vision" | "radar";
  targetId: string;
  radarId: string | null;
  label: string;
  side: RadarContactSide;
  confidence: number;
  angleDeg: number;
  distanceM: number | null;
  xNorm: number | null;
  yNorm: number | null;
  updatedAt: number;
}

export interface GlobalLogisticsState {
  remainingEconomy: number;
  totalEconomyObtained: number;
  techLevel: number;
  encryptionLevel: number;
  updatedAt: number;
}

export interface SpecialMechanismItem {
  mechanismId: number;
  timeSec: number;
}

export interface GlobalSpecialMechanismState {
  mechanisms: SpecialMechanismItem[];
  updatedAt: number;
}

export interface RefereeEventEntry {
  id: string;
  eventId: number;
  param: string;
  updatedAt: number;
}

export interface RobotStaticStatusState {
  connectionState: number;
  fieldState: number;
  aliveState: number;
  robotId: number;
  robotType: number;
  performanceSystemShooter: number;
  performanceSystemChassis: number;
  level: number;
  maxHealth: number;
  maxHeat: number;
  heatCooldownRate: number;
  maxPower: number;
  maxBufferEnergy: number;
  maxChassisEnergy: number;
  updatedAt: number;
}

export interface RobotModuleStatusState {
  powerManager: number;
  rfid: number;
  lightStrip: number;
  smallShooter: number;
  bigShooter: number;
  uwb: number;
  armor: number;
  videoTransmission: number;
  capacitor: number;
  mainController: number;
  laserDetectionModule: number;
  updatedAt: number;
}

export interface BuffState {
  robotId: number;
  buffType: number;
  buffLevel: number;
  buffMaxTime: number;
  buffLeftTime: number;
  updatedAt: number;
}

export interface DeployModeStatusState {
  status: number;
  updatedAt: number;
}

// What: 底盘默认值模板。Why: 后端断联或字段缺失时，UI 仍有稳定可渲染数据。
export const DEFAULT_CHASSIS_DATA: ChassisData = {
  powerW: 0,
  currentA: 0,
  positionX: 0,
  positionY: 0,
  positionZ: 0,
  headingDeg: 0,
  updatedAt: 0,
  origin: "unknown",
};

// What: 云台默认值模板。Why: 避免初始阶段或异常阶段出现 undefined 导致的渲染报错。
export const DEFAULT_GIMBAL_DATA: GimbalData = {
  pitchDeg: 0,
  yawDeg: 0,
  mode: "idle",
  isOnline: false,
  updatedAt: 0,
  origin: "unknown",
};

// What: 连接状态默认值模板。Why: 统一“未连接”语义，避免不同模块各自定义导致状态割裂。
export const DEFAULT_CONNECTION_STATE: ConnectionState = {
  backendConnected: false,
  videoConnected: false,
  controlLinkConnected: false,
  activeSource: "official",
  pipSource: "custom",
  officialAvailable: false,
  customAvailable: false,
  officialVideoConnected: false,
  customVideoConnected: false,
  videoDisplayState: "waiting_source",
  officialDisplayState: "waiting_source",
  customDisplayState: "waiting_source",
  latencyMs: 0,
  officialLatencyMs: 0,
  customLatencyMs: 0,
  officialPacketRateHz: 0,
  officialFrameRateHz: 0,
  officialDropRate: 0,
  customBlockRateHz: 0,
  customAUFPS: 0,
  customDropRate: 0,
  customCorruptRate: 0,
  linkQuality: "offline",
  stale: true,
  lastHeartbeatAt: 0,
  udpDropFrames: 0,
  decodeDropFrames: 0,
  corruptFrames: 0,
  decoderResets: 0,
  headerOrder: "unknown",
  message: "等待官方视频状态",
  officialMessage: "等待官方视频状态",
  customMessage: "等待自定义视频状态",
};

// What: 当前机器人战斗态默认值模板。
// Why: 火控、血量和态势增强器都依赖这组字段，集中默认值可避免多组件各自兜底。
export const DEFAULT_COMBAT_DATA: CombatData = {
  hp: 350,
  maxHp: 350,
  ammo: 0,
  maxAmmo: 120,
  heat: 0,
  maxHeat: 100,
  lastProjectileFireRate: 0,
  chassisEnergy: 0,
  maxChassisEnergy: 0,
  bufferEnergy: 0,
  maxBufferEnergy: 0,
  currentExperience: 0,
  experienceForUpgrade: 0,
  totalProjectilesFired: 0,
  isOutOfCombat: false,
  outOfCombatCountdownSec: 0,
  canRemoteHeal: false,
  bulletSpeed: 0,
  capacitorPct: 0,
  canRemoteAmmo: false,
  updatedAt: 0,
  origin: "unknown",
};

const DEFAULT_TEAM_ORDER = [1, 2, 3, 4, 7] as const;

function resolveRoleFromRobotId(robotId: number): TeamRole {
  if (robotId === 1) return "hero";
  if (robotId === 2) return "engineer";
  if (robotId === 3 || robotId === 4) return "infantry";
  if (robotId === 7) return "sentry";
  return "unknown";
}

function resolveRoleLabel(role: TeamRole): string {
  if (role === "hero") return "英雄";
  if (role === "engineer") return "工程";
  if (role === "infantry") return "步兵";
  if (role === "sentry") return "哨兵";
  return "未知";
}

// What: 生成固定顺序的队伍占位数据。
// Why: 在后端字段延迟或断联时仍保持顶部编组稳定，避免布局抖动。
function createDefaultTeamUnits(side: "red" | "blue"): TeamRobotState[] {
  const prefix = side === "red" ? "R" : "B";
  return DEFAULT_TEAM_ORDER.map((robotId) => {
    const role = resolveRoleFromRobotId(robotId);
    return {
      robotId,
      role,
      roleLabel: resolveRoleLabel(role),
      hp: 0,
      maxHp: 600,
      avatarKey: `${prefix}${robotId}`,
      online: false,
    };
  });
}

export const DEFAULT_MATCH_STATE: MatchState = {
  red: {
    baseHp: 0,
    baseMaxHp: 5000,
    baseStatus: 0,
    baseShield: 0,
    outpostHp: 0,
    outpostStatus: 0,
    totalDamage: 0,
    units: createDefaultTeamUnits("red"),
  },
  blue: {
    baseHp: 0,
    baseMaxHp: 5000,
    baseStatus: 0,
    baseShield: 0,
    outpostHp: 0,
    outpostStatus: 0,
    totalDamage: 0,
    units: createDefaultTeamUnits("blue"),
  },
  // What: 比赛元信息默认全部置空。
  // Why: 只要 GameStatus 还没接进来，顶部中条就必须明确显示“未接入”，不能再伪造局次或倒计时。
  globalStatusReady: false,
  gameStatusReady: false,
  currentRound: 0,
  totalRounds: 0,
  redScore: 0,
  blueScore: 0,
  currentStage: 0,
  stageCountdownSec: 0,
  stageElapsedSec: 0,
  isPaused: false,
  updatedAt: 0,
  origin: "unknown",
};

// What: 空中支援默认值模板。
// Why: 后端尚未接通时也要保持顶栏建议区稳定降级，避免出现 undefined 分支。
export const DEFAULT_AIR_SUPPORT_STATE: AirSupportState = {
  airSupportStatus: 0,
  leftTimeSec: 0,
  costCoins: 0,
  isBeingTargeted: 0,
  shooterStatus: 0,
  updatedAt: 0,
};

// What: 雷达数据默认值模板。
// Why: 后端雷达链路未接入时仍能维持组件稳定渲染，避免出现空引用崩溃。
export const DEFAULT_RADAR_DATA: RadarData = {
  rangeM: 15,
  sweepDeg: 0,
  source: "unknown",
  contacts: [],
  updatedAt: 0,
};

// What: 视觉目标默认值模板。
// Why: 视觉链路尚未接入时返回空集合，可保证融合层只处理稳定数组。
export const DEFAULT_VISION_DATA: VisionData = {
  targets: [],
  updatedAt: 0,
};

export const DEFAULT_GLOBAL_LOGISTICS_STATE: GlobalLogisticsState = {
  remainingEconomy: 0,
  totalEconomyObtained: 0,
  techLevel: 0,
  encryptionLevel: 0,
  updatedAt: 0,
};

export const DEFAULT_GLOBAL_SPECIAL_MECHANISM_STATE: GlobalSpecialMechanismState = {
  mechanisms: [],
  updatedAt: 0,
};

export const DEFAULT_ROBOT_STATIC_STATUS_STATE: RobotStaticStatusState = {
  connectionState: 0,
  fieldState: 0,
  aliveState: 0,
  robotId: 0,
  robotType: 0,
  performanceSystemShooter: 0,
  performanceSystemChassis: 0,
  level: 0,
  maxHealth: 0,
  maxHeat: 0,
  heatCooldownRate: 0,
  maxPower: 0,
  maxBufferEnergy: 0,
  maxChassisEnergy: 0,
  updatedAt: 0,
};

export const DEFAULT_ROBOT_MODULE_STATUS_STATE: RobotModuleStatusState = {
  powerManager: 0,
  rfid: 0,
  lightStrip: 0,
  smallShooter: 0,
  bigShooter: 0,
  uwb: 0,
  armor: 0,
  videoTransmission: 0,
  capacitor: 0,
  mainController: 0,
  laserDetectionModule: 0,
  updatedAt: 0,
};

export const DEFAULT_DEPLOY_MODE_STATUS_STATE: DeployModeStatusState = {
  status: 0,
  updatedAt: 0,
};

export function createDefaultChassisData(): ChassisData {
  return { ...DEFAULT_CHASSIS_DATA };
}

export function createDefaultGimbalData(): GimbalData {
  return { ...DEFAULT_GIMBAL_DATA };
}

export function createDefaultConnectionState(): ConnectionState {
  return { ...DEFAULT_CONNECTION_STATE };
}

export function createDefaultCombatData(): CombatData {
  return { ...DEFAULT_COMBAT_DATA };
}

export function createDefaultMatchState(): MatchState {
  return {
    red: {
      ...DEFAULT_MATCH_STATE.red,
      units: DEFAULT_MATCH_STATE.red.units.map((item) => ({ ...item })),
    },
    blue: {
      ...DEFAULT_MATCH_STATE.blue,
      units: DEFAULT_MATCH_STATE.blue.units.map((item) => ({ ...item })),
    },
    globalStatusReady: DEFAULT_MATCH_STATE.globalStatusReady,
    gameStatusReady: DEFAULT_MATCH_STATE.gameStatusReady,
    currentRound: DEFAULT_MATCH_STATE.currentRound,
    totalRounds: DEFAULT_MATCH_STATE.totalRounds,
    redScore: DEFAULT_MATCH_STATE.redScore,
    blueScore: DEFAULT_MATCH_STATE.blueScore,
    currentStage: DEFAULT_MATCH_STATE.currentStage,
    stageCountdownSec: DEFAULT_MATCH_STATE.stageCountdownSec,
    stageElapsedSec: DEFAULT_MATCH_STATE.stageElapsedSec,
    isPaused: DEFAULT_MATCH_STATE.isPaused,
    updatedAt: DEFAULT_MATCH_STATE.updatedAt,
    origin: DEFAULT_MATCH_STATE.origin,
  };
}

export function createDefaultAirSupportState(): AirSupportState {
  return { ...DEFAULT_AIR_SUPPORT_STATE };
}

export function createDefaultRadarData(): RadarData {
  return {
    ...DEFAULT_RADAR_DATA,
    contacts: DEFAULT_RADAR_DATA.contacts.map((item) => ({ ...item })),
  };
}

export function createDefaultVisionData(): VisionData {
  return {
    ...DEFAULT_VISION_DATA,
    targets: DEFAULT_VISION_DATA.targets.map((item) => ({ ...item })),
  };
}

export function createDefaultGlobalLogisticsState(): GlobalLogisticsState {
  return { ...DEFAULT_GLOBAL_LOGISTICS_STATE };
}

export function createDefaultGlobalSpecialMechanismState(): GlobalSpecialMechanismState {
  return {
    ...DEFAULT_GLOBAL_SPECIAL_MECHANISM_STATE,
    mechanisms: DEFAULT_GLOBAL_SPECIAL_MECHANISM_STATE.mechanisms.map((item) => ({ ...item })),
  };
}

export function createDefaultRobotStaticStatusState(): RobotStaticStatusState {
  return { ...DEFAULT_ROBOT_STATIC_STATUS_STATE };
}

export function createDefaultRobotModuleStatusState(): RobotModuleStatusState {
  return { ...DEFAULT_ROBOT_MODULE_STATUS_STATE };
}

export function createDefaultDeployModeStatusState(): DeployModeStatusState {
  return { ...DEFAULT_DEPLOY_MODE_STATUS_STATE };
}
