import { defineStore } from "pinia";
import { computed, ref, shallowRef } from "vue";
import type {
  AirSupportState,
  BuffState,
  ChassisData,
  CombatData,
  ConnectionState,
  DeployModeStatusState,
  GimbalData,
  GlobalLogisticsState,
  GlobalSpecialMechanismState,
  MatchState,
  RadarContact,
  RadarData,
  RefereeEventEntry,
  RobotModuleStatusState,
  RobotStaticStatusState,
  SpecialMechanismItem,
  TeamRobotState,
  TeamSideState,
  TelemetryOrigin,
  VideoDisplayState,
  VideoSource,
  VisionData,
  VisionTarget,
} from "../types";
import {
  createDefaultAirSupportState,
  createDefaultChassisData,
  createDefaultCombatData,
  createDefaultConnectionState,
  createDefaultDeployModeStatusState,
  createDefaultGimbalData,
  createDefaultGlobalLogisticsState,
  createDefaultGlobalSpecialMechanismState,
  createDefaultMatchState,
  createDefaultRadarData,
  createDefaultRobotModuleStatusState,
  createDefaultRobotStaticStatusState,
  createDefaultVisionData,
} from "../types";

type TelemetryPatch = {
  airSupport?: Partial<AirSupportState>;
  buffs?: BuffState[];
  chassis?: Partial<ChassisData>;
  combat?: Partial<CombatData>;
  connection?: Partial<ConnectionState>;
  deployMode?: Partial<DeployModeStatusState>;
  globalLogistics?: Partial<GlobalLogisticsState>;
  globalSpecialMechanism?: Partial<GlobalSpecialMechanismState>;
  gimbal?: Partial<GimbalData>;
  match?: Partial<MatchState>;
  radar?: Partial<RadarData>;
  refereeEvents?: RefereeEventEntry[];
  robotModuleStatus?: Partial<RobotModuleStatusState>;
  robotStaticStatus?: Partial<RobotStaticStatusState>;
  vision?: Partial<VisionData>;
};

function toSafeNumber(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

function toSafeBoolean(value: unknown, fallback: boolean): boolean {
  if (typeof value === "boolean") return value;
  return fallback;
}

function normalizeTelemetryOrigin(value: unknown): TelemetryOrigin | null {
  if (value === "backend" || value === "simulated" || value === "unknown")
    return value;
  return null;
}

function mergeTelemetryOrigin(
  previous: TelemetryOrigin,
  patchValue: unknown,
): TelemetryOrigin {
  const next = normalizeTelemetryOrigin(patchValue);
  if (next === null || next === "unknown") return previous;
  if (previous === "backend" && next === "simulated") return previous;
  return next;
}

function shouldIgnoreSimulatedTelemetry(
  previous: TelemetryOrigin,
  patchValue: unknown,
): boolean {
  return (
    previous === "backend" &&
    normalizeTelemetryOrigin(patchValue) === "simulated"
  );
}

function normalizeVideoDisplayState(
  value: unknown,
  fallback: VideoDisplayState,
): VideoDisplayState {
  if (
    value === "live" ||
    value === "waiting_source" ||
    value === "waiting_frame" ||
    value === "resyncing" ||
    value === "stalled" ||
    value === "runtime_error"
  ) {
    return value;
  }
  return fallback;
}

function normalizeVideoSource(
  value: unknown,
  fallback: VideoSource,
): VideoSource {
  if (value === "official" || value === "custom") return value;
  return fallback;
}

const runOnNextFrame = (callback: () => void): number => {
  if (
    typeof window !== "undefined" &&
    typeof window.requestAnimationFrame === "function"
  ) {
    return window.requestAnimationFrame(callback);
  }
  return window.setTimeout(callback, 16);
};

const cancelNextFrame = (id: number): void => {
  if (
    typeof window !== "undefined" &&
    typeof window.cancelAnimationFrame === "function"
  ) {
    window.cancelAnimationFrame(id);
    return;
  }
  globalThis.clearTimeout(id);
};

export const useRobotDataStore = defineStore("robot-data", () => {
  const airSupport = shallowRef<AirSupportState>(
    createDefaultAirSupportState(),
  );
  const chassis = shallowRef<ChassisData>(createDefaultChassisData());
  const combat = shallowRef<CombatData>(createDefaultCombatData());
  const gimbal = shallowRef<GimbalData>(createDefaultGimbalData());
  const connection = ref<ConnectionState>(createDefaultConnectionState());
  const matchState = shallowRef<MatchState>(createDefaultMatchState());
  const radar = shallowRef<RadarData>(createDefaultRadarData());
  const vision = shallowRef<VisionData>(createDefaultVisionData());
  const globalLogistics = shallowRef<GlobalLogisticsState>(
    createDefaultGlobalLogisticsState(),
  );
  const globalSpecialMechanism = shallowRef<GlobalSpecialMechanismState>(
    createDefaultGlobalSpecialMechanismState(),
  );
  const refereeEvents = shallowRef<RefereeEventEntry[]>([]);
  const robotStaticStatus = shallowRef<RobotStaticStatusState>(
    createDefaultRobotStaticStatusState(),
  );
  const robotModuleStatus = shallowRef<RobotModuleStatusState>(
    createDefaultRobotModuleStatusState(),
  );
  const buffs = shallowRef<BuffState[]>([]);
  const deployModeStatus = shallowRef<DeployModeStatusState>(
    createDefaultDeployModeStatusState(),
  );

  const lastDataAt = ref(0);

  let pendingChassis: Partial<ChassisData> | null = null;
  let pendingGimbal: Partial<GimbalData> | null = null;
  let frameTask: number | null = null;

  // What: 底盘分层更新逻辑。Why: 保证字段校验与归一化只在一处执行，避免多组件重复逻辑。
  function applyChassisPatch(patch: Partial<ChassisData>): void {
    const prev = chassis.value;
    const now = Date.now();
    const nextOrigin = mergeTelemetryOrigin(prev.origin, patch.origin);

    // What: 一旦拿到真实底盘遥测，就拒绝再用前端仿真值覆盖。
    // Why: 严格真实模式下，真实数据优先级必须永久高于占位仿真，否则卡片会在两种来源之间来回跳。
    if (shouldIgnoreSimulatedTelemetry(prev.origin, patch.origin)) {
      return;
    }

    chassis.value = {
      powerW: toSafeNumber(patch.powerW, prev.powerW),
      currentA: toSafeNumber(patch.currentA, prev.currentA),
      positionX: toSafeNumber(patch.positionX, prev.positionX),
      positionY: toSafeNumber(patch.positionY, prev.positionY),
      positionZ: toSafeNumber(patch.positionZ, prev.positionZ),
      headingDeg: clamp(
        toSafeNumber(patch.headingDeg, prev.headingDeg),
        -360,
        360,
      ),
      updatedAt: now,
      origin: nextOrigin,
    };

    lastDataAt.value = now;
  }

  // What: 当前机器人战斗态统一更新。
  // Why: 低血量、过热、弹量、电容和补给可用态都依赖同一状态源，集中归一化可避免多处阈值分叉。
  function applyCombatPatch(patch: Partial<CombatData>): void {
    const prev = combat.value;
    const now = Date.now();
    const nextOrigin = mergeTelemetryOrigin(prev.origin, patch.origin);

    // What: 真实 combat-state 到达后，不允许再被本地占位 combat 回写。
    // Why: 血量与远程买弹是用户最敏感的字段，一旦被仿真覆盖，状态卡就会再次出现“看起来有数据但其实不准”的问题。
    if (shouldIgnoreSimulatedTelemetry(prev.origin, patch.origin)) {
      return;
    }

    combat.value = {
      hp: Math.max(0, toSafeNumber(patch.hp, prev.hp)),
      maxHp: Math.max(1, toSafeNumber(patch.maxHp, prev.maxHp)),
      ammo: Math.max(0, toSafeNumber(patch.ammo, prev.ammo)),
      maxAmmo: Math.max(1, toSafeNumber(patch.maxAmmo, prev.maxAmmo)),
      heat: Math.max(0, toSafeNumber(patch.heat, prev.heat)),
      maxHeat: Math.max(1, toSafeNumber(patch.maxHeat, prev.maxHeat)),
      lastProjectileFireRate: Math.max(
        0,
        toSafeNumber(patch.lastProjectileFireRate, prev.lastProjectileFireRate),
      ),
      chassisEnergy: Math.max(0, toSafeNumber(patch.chassisEnergy, prev.chassisEnergy)),
      maxChassisEnergy: Math.max(
        0,
        toSafeNumber(patch.maxChassisEnergy, prev.maxChassisEnergy),
      ),
      bufferEnergy: Math.max(0, toSafeNumber(patch.bufferEnergy, prev.bufferEnergy)),
      maxBufferEnergy: Math.max(
        0,
        toSafeNumber(patch.maxBufferEnergy, prev.maxBufferEnergy),
      ),
      currentExperience: Math.max(
        0,
        toSafeNumber(patch.currentExperience, prev.currentExperience),
      ),
      experienceForUpgrade: Math.max(
        0,
        toSafeNumber(patch.experienceForUpgrade, prev.experienceForUpgrade),
      ),
      totalProjectilesFired: Math.max(
        0,
        Math.round(toSafeNumber(patch.totalProjectilesFired, prev.totalProjectilesFired)),
      ),
      isOutOfCombat: toSafeBoolean(patch.isOutOfCombat, prev.isOutOfCombat),
      outOfCombatCountdownSec: Math.max(
        0,
        Math.round(toSafeNumber(patch.outOfCombatCountdownSec, prev.outOfCombatCountdownSec)),
      ),
      canRemoteHeal: toSafeBoolean(patch.canRemoteHeal, prev.canRemoteHeal),
      bulletSpeed: Math.max(
        0,
        toSafeNumber(patch.bulletSpeed, prev.bulletSpeed),
      ),
      capacitorPct: clamp(
        toSafeNumber(
          patch.capacitorPct,
          patch.maxBufferEnergy && patch.bufferEnergy !== undefined
            ? (patch.bufferEnergy / Math.max(1, patch.maxBufferEnergy)) * 100
            : prev.capacitorPct,
        ),
        0,
        100,
      ),
      canRemoteAmmo: toSafeBoolean(patch.canRemoteAmmo, prev.canRemoteAmmo),
      updatedAt: patch.updatedAt ?? now,
      origin: nextOrigin,
    };

    lastDataAt.value = now;
  }

  // What: 空中支援状态统一归一化更新。
  // Why: 起飞建议与中心提醒都依赖这组低频状态，集中收口可避免不同组件各自理解 left_time/cost_coins。
  function applyAirSupportPatch(patch: Partial<AirSupportState>): void {
    const prev = airSupport.value;
    const now = Date.now();

    airSupport.value = {
      airSupportStatus: Math.max(
        0,
        Math.round(toSafeNumber(patch.airSupportStatus, prev.airSupportStatus)),
      ),
      leftTimeSec: Math.max(
        0,
        Math.round(toSafeNumber(patch.leftTimeSec, prev.leftTimeSec)),
      ),
      costCoins: Math.max(
        0,
        Math.round(toSafeNumber(patch.costCoins, prev.costCoins)),
      ),
      isBeingTargeted: Math.max(
        0,
        Math.round(toSafeNumber(patch.isBeingTargeted, prev.isBeingTargeted)),
      ),
      shooterStatus: Math.max(
        0,
        Math.round(toSafeNumber(patch.shooterStatus, prev.shooterStatus)),
      ),
      updatedAt: patch.updatedAt ?? now,
    };

    lastDataAt.value = now;
  }

  // What: 云台分层更新逻辑。Why: 对高频姿态数据保持浅响应对象替换，降低深层代理开销。
  function applyGimbalPatch(patch: Partial<GimbalData>): void {
    const prev = gimbal.value;
    const now = Date.now();
    const nextOrigin = mergeTelemetryOrigin(prev.origin, patch.origin);

    // What: 真实云台态优先于本地演示姿态。
    // Why: 未来若接入真实姿态回包，这里必须先挡住仿真持续刷新的覆盖行为。
    if (shouldIgnoreSimulatedTelemetry(prev.origin, patch.origin)) {
      return;
    }

    gimbal.value = {
      pitchDeg: clamp(toSafeNumber(patch.pitchDeg, prev.pitchDeg), -45, 45),
      yawDeg: clamp(toSafeNumber(patch.yawDeg, prev.yawDeg), -180, 180),
      mode: patch.mode ?? prev.mode,
      isOnline: patch.isOnline ?? prev.isOnline,
      updatedAt: now,
      origin: nextOrigin,
    };

    lastDataAt.value = now;
  }

  // What: 连接状态更新。Why: 连接态是低频但高影响数据，单独管理可让 UI 统一降级策略。
  function setConnectionState(patch: Partial<ConnectionState>): void {
    const prev = connection.value;
    connection.value = {
      ...prev,
      ...patch,
      activeSource: normalizeVideoSource(patch.activeSource, prev.activeSource),
      pipSource: normalizeVideoSource(patch.pipSource, prev.pipSource),
      officialAvailable: toSafeBoolean(
        patch.officialAvailable,
        prev.officialAvailable,
      ),
      customAvailable: toSafeBoolean(
        patch.customAvailable,
        prev.customAvailable,
      ),
      officialVideoConnected: toSafeBoolean(
        patch.officialVideoConnected,
        prev.officialVideoConnected,
      ),
      customVideoConnected: toSafeBoolean(
        patch.customVideoConnected,
        prev.customVideoConnected,
      ),
      videoDisplayState: normalizeVideoDisplayState(
        patch.videoDisplayState,
        prev.videoDisplayState,
      ),
      officialDisplayState: normalizeVideoDisplayState(
        patch.officialDisplayState,
        prev.officialDisplayState,
      ),
      customDisplayState: normalizeVideoDisplayState(
        patch.customDisplayState,
        prev.customDisplayState,
      ),
      latencyMs: toSafeNumber(patch.latencyMs, prev.latencyMs),
      officialLatencyMs: toSafeNumber(
        patch.officialLatencyMs,
        prev.officialLatencyMs,
      ),
      customLatencyMs: toSafeNumber(
        patch.customLatencyMs,
        prev.customLatencyMs,
      ),
      officialPacketRateHz: Math.max(
        0,
        toSafeNumber(patch.officialPacketRateHz, prev.officialPacketRateHz),
      ),
      officialFrameRateHz: Math.max(
        0,
        toSafeNumber(patch.officialFrameRateHz, prev.officialFrameRateHz),
      ),
      officialDropRate: clamp(
        toSafeNumber(patch.officialDropRate, prev.officialDropRate),
        0,
        1,
      ),
      customBlockRateHz: Math.max(
        0,
        toSafeNumber(patch.customBlockRateHz, prev.customBlockRateHz),
      ),
      customAUFPS: Math.max(
        0,
        toSafeNumber(patch.customAUFPS, prev.customAUFPS),
      ),
      customDropRate: clamp(
        toSafeNumber(patch.customDropRate, prev.customDropRate),
        0,
        1,
      ),
      customCorruptRate: clamp(
        toSafeNumber(patch.customCorruptRate, prev.customCorruptRate),
        0,
        1,
      ),
      lastHeartbeatAt: patch.lastHeartbeatAt ?? Date.now(),
      udpDropFrames: Math.max(
        0,
        Math.round(toSafeNumber(patch.udpDropFrames, prev.udpDropFrames)),
      ),
      decodeDropFrames: Math.max(
        0,
        Math.round(toSafeNumber(patch.decodeDropFrames, prev.decodeDropFrames)),
      ),
      corruptFrames: Math.max(
        0,
        Math.round(toSafeNumber(patch.corruptFrames, prev.corruptFrames)),
      ),
      decoderResets: Math.max(
        0,
        Math.round(toSafeNumber(patch.decoderResets, prev.decoderResets)),
      ),
      headerOrder:
        typeof patch.headerOrder === "string"
          ? patch.headerOrder
          : prev.headerOrder,
      officialMessage:
        typeof patch.officialMessage === "string"
          ? patch.officialMessage
          : prev.officialMessage,
      customMessage:
        typeof patch.customMessage === "string"
          ? patch.customMessage
          : prev.customMessage,
    };
  }

  function mergeRobotUnit(
    prev: TeamRobotState,
    patch: Partial<TeamRobotState>,
  ): TeamRobotState {
    return {
      robotId: Number.isFinite(Number(patch.robotId))
        ? Number(patch.robotId)
        : prev.robotId,
      role: patch.role ?? prev.role,
      roleLabel: patch.roleLabel ?? prev.roleLabel,
      hp: Math.max(0, toSafeNumber(patch.hp, prev.hp)),
      maxHp: Math.max(1, toSafeNumber(patch.maxHp, prev.maxHp)),
      avatarKey: patch.avatarKey ?? prev.avatarKey,
      online: toSafeBoolean(patch.online, prev.online),
    };
  }

  function mergeTeamSide(
    prev: TeamSideState,
    patch: Partial<TeamSideState>,
  ): TeamSideState {
    const baseUnits = prev.units;
    const patchMap = new Map<number, Partial<TeamRobotState>>();
    (patch.units ?? []).forEach((item) => {
      if (!item) return;
      const key = Number(item.robotId);
      if (!Number.isFinite(key)) return;
      patchMap.set(key, item);
    });

    const units = baseUnits.map((prevUnit) => {
      const unitPatch = patchMap.get(prevUnit.robotId);
      return mergeRobotUnit(prevUnit, unitPatch ?? {});
    });

    return {
      baseHp: Math.max(0, toSafeNumber(patch.baseHp, prev.baseHp)),
      baseMaxHp: Math.max(1, toSafeNumber(patch.baseMaxHp, prev.baseMaxHp)),
      baseStatus: Math.max(
        0,
        Math.round(toSafeNumber(patch.baseStatus, prev.baseStatus)),
      ),
      baseShield: Math.max(
        0,
        Math.round(toSafeNumber(patch.baseShield, prev.baseShield)),
      ),
      outpostHp: Math.max(0, toSafeNumber(patch.outpostHp, prev.outpostHp)),
      outpostStatus: Math.max(
        0,
        Math.round(toSafeNumber(patch.outpostStatus, prev.outpostStatus)),
      ),
      totalDamage: Math.max(
        0,
        Math.round(toSafeNumber(patch.totalDamage, prev.totalDamage)),
      ),
      units,
    };
  }

  // What: 顶部比赛态分层更新逻辑。
  // Why: 红蓝基地与机器人编组属于高频显示数据，需要统一在 store 做字段归一化兜底。
  function applyMatchStatePatch(patch: Partial<MatchState>): void {
    const prev = matchState.value;
    const nextOrigin = mergeTelemetryOrigin(prev.origin, patch.origin);
    matchState.value = {
      red: mergeTeamSide(prev.red, patch.red ?? {}),
      blue: mergeTeamSide(prev.blue, patch.blue ?? {}),
      // What: 全局血量编组单独维护显式就绪位。
      // Why: 基地、前哨站和总伤害都可能合法为 0，不能继续靠数值本身推断“有没有接入真实 GlobalUnitStatus”。
      globalStatusReady: toSafeBoolean(
        patch.globalStatusReady,
        prev.globalStatusReady,
      ),
      // What: 比赛元信息严格对齐官方 GameStatus 字段。
      // Why: 顶部中条现在只允许消费真实局次、比分、阶段和时间，不能再混入 roundLabel/economy 之类旧假字段。
      gameStatusReady: toSafeBoolean(
        patch.gameStatusReady,
        prev.gameStatusReady,
      ),
      currentRound: Math.max(
        0,
        Math.round(toSafeNumber(patch.currentRound, prev.currentRound)),
      ),
      totalRounds: Math.max(
        0,
        Math.round(toSafeNumber(patch.totalRounds, prev.totalRounds)),
      ),
      redScore: Math.max(
        0,
        Math.round(toSafeNumber(patch.redScore, prev.redScore)),
      ),
      blueScore: Math.max(
        0,
        Math.round(toSafeNumber(patch.blueScore, prev.blueScore)),
      ),
      currentStage: Math.max(
        0,
        Math.round(toSafeNumber(patch.currentStage, prev.currentStage)),
      ),
      stageCountdownSec: Math.max(
        0,
        Math.round(
          toSafeNumber(patch.stageCountdownSec, prev.stageCountdownSec),
        ),
      ),
      stageElapsedSec: Math.max(
        0,
        Math.round(toSafeNumber(patch.stageElapsedSec, prev.stageElapsedSec)),
      ),
      isPaused: toSafeBoolean(patch.isPaused, prev.isPaused),
      updatedAt: patch.updatedAt ?? Date.now(),
      // What: 比赛态来源继续沿用统一优先级合并。
      // Why: 后续若局部调试桥只补某一块真实字段，也不能把已建立的 backend 来源退回 unknown。
      origin: nextOrigin,
    };
    lastDataAt.value = Date.now();
  }

  // What: 归一化雷达目标阵营枚举。
  // Why: 后端异常值回包时回退到安全值，防止 UI 渲染分支失效。
  function normalizeRadarSide(
    value: unknown,
    fallback: RadarContact["side"],
  ): RadarContact["side"] {
    if (value === "ally" || value === "enemy" || value === "neutral")
      return value;
    return fallback;
  }

  // What: 将单个雷达目标做字段级兜底。
  // Why: 保证目标点渲染总是使用稳定结构，避免 NaN 坐标污染图层。
  function normalizeRadarContact(
    raw: Partial<RadarContact>,
    fallbackUpdatedAt: number,
  ): RadarContact {
    return {
      id: String(raw.id ?? ""),
      x: toSafeNumber(raw.x, 0),
      y: toSafeNumber(raw.y, 0),
      velocity: Math.max(0, toSafeNumber(raw.velocity, 0)),
      side: normalizeRadarSide(raw.side, "enemy"),
      confidence: clamp(toSafeNumber(raw.confidence, 0), 0, 1),
      headingDeg: toSafeNumber(raw.headingDeg, 0),
      highlighted: toSafeBoolean(raw.highlighted, false),
      updatedAt: toSafeNumber(raw.updatedAt, fallbackUpdatedAt),
    };
  }

  // What: 雷达数据归一化更新。
  // Why: 雷达回包频率高且可能缺字段，统一在 store 兜底可避免组件层反复判空。
  function applyRadarPatch(patch: Partial<RadarData>): void {
    const prev = radar.value;
    const now = Date.now();
    const source =
      patch.source === "backend" || patch.source === "unknown"
        ? patch.source
        : prev.source;

    const contacts = Array.isArray(patch.contacts)
      ? patch.contacts
          .slice(0, 32)
          .map((item) => normalizeRadarContact(item ?? {}, now))
      : prev.contacts;

    radar.value = {
      rangeM: clamp(toSafeNumber(patch.rangeM, prev.rangeM), 1, 120),
      sweepDeg:
        ((toSafeNumber(patch.sweepDeg, prev.sweepDeg) % 360) + 360) % 360,
      source,
      contacts,
      updatedAt: toSafeNumber(patch.updatedAt, now),
    };

    lastDataAt.value = now;
  }

  function normalizeVisionTarget(
    raw: Partial<VisionTarget>,
    fallbackUpdatedAt: number,
  ): VisionTarget {
    return {
      id: String(raw.id ?? ""),
      xNorm: clamp(toSafeNumber(raw.xNorm, 0.5), 0, 1),
      yNorm: clamp(toSafeNumber(raw.yNorm, 0.5), 0, 1),
      confidence: clamp(toSafeNumber(raw.confidence, 0), 0, 1),
      side: normalizeRadarSide(raw.side, "enemy"),
      armorLabel: String(raw.armorLabel ?? ""),
      updatedAt: toSafeNumber(raw.updatedAt, fallbackUpdatedAt),
    };
  }

  // What: 视觉目标统一归一化更新。
  // Why: 后续融合雷达和视觉时需要稳定数组结构，避免空字段打断态势增强器。
  function applyVisionPatch(patch: Partial<VisionData>): void {
    const prev = vision.value;
    const now = Date.now();
    const targets = Array.isArray(patch.targets)
      ? patch.targets
          .slice(0, 12)
          .map((item) => normalizeVisionTarget(item ?? {}, now))
      : prev.targets;

    vision.value = {
      targets,
      updatedAt: toSafeNumber(patch.updatedAt, now),
    };

    lastDataAt.value = now;
  }

  function applyGlobalLogisticsPatch(patch: Partial<GlobalLogisticsState>): void {
    const prev = globalLogistics.value;
    const now = Date.now();
    globalLogistics.value = {
      remainingEconomy: Math.max(0, Math.round(toSafeNumber(patch.remainingEconomy, prev.remainingEconomy))),
      totalEconomyObtained: Math.max(0, Math.round(toSafeNumber(patch.totalEconomyObtained, prev.totalEconomyObtained))),
      techLevel: Math.max(0, Math.round(toSafeNumber(patch.techLevel, prev.techLevel))),
      encryptionLevel: Math.max(0, Math.round(toSafeNumber(patch.encryptionLevel, prev.encryptionLevel))),
      updatedAt: toSafeNumber(patch.updatedAt, now),
    };
    lastDataAt.value = now;
  }

  function normalizeMechanismItem(raw: Partial<SpecialMechanismItem>): SpecialMechanismItem {
    return {
      mechanismId: Math.max(0, Math.round(toSafeNumber(raw.mechanismId, 0))),
      timeSec: Math.max(0, Math.round(toSafeNumber(raw.timeSec, 0))),
    };
  }

  function applyGlobalSpecialMechanismPatch(patch: Partial<GlobalSpecialMechanismState>): void {
    const now = Date.now();
    globalSpecialMechanism.value = {
      mechanisms: Array.isArray(patch.mechanisms)
        ? patch.mechanisms.slice(0, 16).map((item) => normalizeMechanismItem(item ?? {}))
        : globalSpecialMechanism.value.mechanisms,
      updatedAt: toSafeNumber(patch.updatedAt, now),
    };
    lastDataAt.value = now;
  }

  function applyRobotStaticStatusPatch(patch: Partial<RobotStaticStatusState>): void {
    const prev = robotStaticStatus.value;
    const now = Date.now();
    robotStaticStatus.value = {
      connectionState: Math.max(0, Math.round(toSafeNumber(patch.connectionState, prev.connectionState))),
      fieldState: Math.max(0, Math.round(toSafeNumber(patch.fieldState, prev.fieldState))),
      aliveState: Math.max(0, Math.round(toSafeNumber(patch.aliveState, prev.aliveState))),
      robotId: Math.max(0, Math.round(toSafeNumber(patch.robotId, prev.robotId))),
      robotType: Math.max(0, Math.round(toSafeNumber(patch.robotType, prev.robotType))),
      performanceSystemShooter: Math.max(0, Math.round(toSafeNumber(patch.performanceSystemShooter, prev.performanceSystemShooter))),
      performanceSystemChassis: Math.max(0, Math.round(toSafeNumber(patch.performanceSystemChassis, prev.performanceSystemChassis))),
      level: Math.max(0, Math.round(toSafeNumber(patch.level, prev.level))),
      maxHealth: Math.max(0, Math.round(toSafeNumber(patch.maxHealth, prev.maxHealth))),
      maxHeat: Math.max(0, toSafeNumber(patch.maxHeat, prev.maxHeat)),
      heatCooldownRate: Math.max(0, toSafeNumber(patch.heatCooldownRate, prev.heatCooldownRate)),
      maxPower: Math.max(0, Math.round(toSafeNumber(patch.maxPower, prev.maxPower))),
      maxBufferEnergy: Math.max(0, Math.round(toSafeNumber(patch.maxBufferEnergy, prev.maxBufferEnergy))),
      maxChassisEnergy: Math.max(0, Math.round(toSafeNumber(patch.maxChassisEnergy, prev.maxChassisEnergy))),
      updatedAt: toSafeNumber(patch.updatedAt, now),
    };
    applyCombatPatch({
      maxHp: robotStaticStatus.value.maxHealth || combat.value.maxHp,
      maxHeat: robotStaticStatus.value.maxHeat || combat.value.maxHeat,
      maxBufferEnergy: robotStaticStatus.value.maxBufferEnergy,
      maxChassisEnergy: robotStaticStatus.value.maxChassisEnergy,
      origin: "backend",
    });
    lastDataAt.value = now;
  }

  function applyRobotModuleStatusPatch(patch: Partial<RobotModuleStatusState>): void {
    const prev = robotModuleStatus.value;
    const now = Date.now();
    robotModuleStatus.value = {
      powerManager: Math.max(0, Math.round(toSafeNumber(patch.powerManager, prev.powerManager))),
      rfid: Math.max(0, Math.round(toSafeNumber(patch.rfid, prev.rfid))),
      lightStrip: Math.max(0, Math.round(toSafeNumber(patch.lightStrip, prev.lightStrip))),
      smallShooter: Math.max(0, Math.round(toSafeNumber(patch.smallShooter, prev.smallShooter))),
      bigShooter: Math.max(0, Math.round(toSafeNumber(patch.bigShooter, prev.bigShooter))),
      uwb: Math.max(0, Math.round(toSafeNumber(patch.uwb, prev.uwb))),
      armor: Math.max(0, Math.round(toSafeNumber(patch.armor, prev.armor))),
      videoTransmission: Math.max(0, Math.round(toSafeNumber(patch.videoTransmission, prev.videoTransmission))),
      capacitor: Math.max(0, Math.round(toSafeNumber(patch.capacitor, prev.capacitor))),
      mainController: Math.max(0, Math.round(toSafeNumber(patch.mainController, prev.mainController))),
      laserDetectionModule: Math.max(0, Math.round(toSafeNumber(patch.laserDetectionModule, prev.laserDetectionModule))),
      updatedAt: toSafeNumber(patch.updatedAt, now),
    };
    lastDataAt.value = now;
  }

  function pushRefereeEvent(entry: RefereeEventEntry): void {
    const next = [entry, ...refereeEvents.value.filter((item) => item.id !== entry.id)].slice(0, 8);
    refereeEvents.value = next;
    lastDataAt.value = Date.now();
  }

  function upsertBuff(buff: BuffState): void {
    const key = `${buff.robotId}:${buff.buffType}`;
    const filtered = buffs.value.filter((item) => `${item.robotId}:${item.buffType}` !== key);
    buffs.value = buff.buffLeftTime > 0 ? [buff, ...filtered].slice(0, 16) : filtered;
    lastDataAt.value = Date.now();
  }

  function applyDeployModeStatusPatch(patch: Partial<DeployModeStatusState>): void {
    const prev = deployModeStatus.value;
    const now = Date.now();
    deployModeStatus.value = {
      status: Math.max(0, Math.round(toSafeNumber(patch.status, prev.status))),
      updatedAt: toSafeNumber(patch.updatedAt, now),
    };
    lastDataAt.value = now;
  }

  function flushPendingTelemetry(): void {
    frameTask = null;

    if (pendingChassis) {
      applyChassisPatch(pendingChassis);
      pendingChassis = null;
    }

    if (pendingGimbal) {
      applyGimbalPatch(pendingGimbal);
      pendingGimbal = null;
    }
  }

  function scheduleFlush(): void {
    if (frameTask !== null) return;
    // What: 将 100Hz 以上输入合并到每帧。Why: 减少组件级频繁重绘，保持界面流畅度。
    frameTask = runOnNextFrame(flushPendingTelemetry);
  }

  function queueChassisUpdate(patch: Partial<ChassisData>): void {
    pendingChassis = { ...(pendingChassis ?? {}), ...patch };
    scheduleFlush();
  }

  function queueGimbalUpdate(patch: Partial<GimbalData>): void {
    pendingGimbal = { ...(pendingGimbal ?? {}), ...patch };
    scheduleFlush();
  }

  function queueTelemetry(patch: TelemetryPatch): void {
    if (patch.airSupport) applyAirSupportPatch(patch.airSupport);
    if (patch.buffs) patch.buffs.forEach((item) => upsertBuff(item));
    if (patch.chassis) queueChassisUpdate(patch.chassis);
    if (patch.combat) applyCombatPatch(patch.combat);
    if (patch.connection) setConnectionState(patch.connection);
    if (patch.deployMode) applyDeployModeStatusPatch(patch.deployMode);
    if (patch.globalLogistics) applyGlobalLogisticsPatch(patch.globalLogistics);
    if (patch.globalSpecialMechanism) applyGlobalSpecialMechanismPatch(patch.globalSpecialMechanism);
    if (patch.gimbal) queueGimbalUpdate(patch.gimbal);
    if (patch.match) applyMatchStatePatch(patch.match);
    if (patch.radar) applyRadarPatch(patch.radar);
    if (patch.refereeEvents) patch.refereeEvents.forEach((item) => pushRefereeEvent(item));
    if (patch.robotModuleStatus) applyRobotModuleStatusPatch(patch.robotModuleStatus);
    if (patch.robotStaticStatus) applyRobotStaticStatusPatch(patch.robotStaticStatus);
    if (patch.vision) applyVisionPatch(patch.vision);
  }

  function markDisconnected(message = "后端断联"): void {
    setConnectionState({
      backendConnected: false,
      videoConnected: false,
      controlLinkConnected: false,
      officialAvailable: false,
      customAvailable: false,
      officialVideoConnected: false,
      customVideoConnected: false,
      videoDisplayState: "stalled",
      officialDisplayState: "stalled",
      customDisplayState: "stalled",
      stale: true,
      linkQuality: "offline",
      udpDropFrames: 0,
      decodeDropFrames: 0,
      corruptFrames: 0,
      decoderResets: 0,
      headerOrder: "unknown",
      message,
      officialMessage: message,
      customMessage: message,
    });
  }

  function resetToDefault(): void {
    if (frameTask !== null) {
      cancelNextFrame(frameTask);
      frameTask = null;
    }

    pendingChassis = null;
    pendingGimbal = null;
    airSupport.value = createDefaultAirSupportState();
    chassis.value = createDefaultChassisData();
    combat.value = createDefaultCombatData();
    gimbal.value = createDefaultGimbalData();
    connection.value = createDefaultConnectionState();
    matchState.value = createDefaultMatchState();
    radar.value = createDefaultRadarData();
    vision.value = createDefaultVisionData();
    globalLogistics.value = createDefaultGlobalLogisticsState();
    globalSpecialMechanism.value = createDefaultGlobalSpecialMechanismState();
    refereeEvents.value = [];
    robotStaticStatus.value = createDefaultRobotStaticStatusState();
    robotModuleStatus.value = createDefaultRobotModuleStatusState();
    buffs.value = [];
    deployModeStatus.value = createDefaultDeployModeStatusState();
    lastDataAt.value = 0;
  }

  const isConnected = computed(
    () => connection.value.backendConnected && !connection.value.stale,
  );

  return {
    airSupport,
    chassis,
    combat,
    gimbal,
    connection,
    matchState,
    radar,
    vision,
    globalLogistics,
    globalSpecialMechanism,
    refereeEvents,
    robotStaticStatus,
    robotModuleStatus,
    buffs,
    deployModeStatus,
    isConnected,
    lastDataAt,
    applyAirSupportPatch,
    applyChassisPatch,
    applyCombatPatch,
    applyGimbalPatch,
    setConnectionState,
    applyMatchStatePatch,
    applyRadarPatch,
    applyVisionPatch,
    applyGlobalLogisticsPatch,
    applyGlobalSpecialMechanismPatch,
    applyRobotStaticStatusPatch,
    applyRobotModuleStatusPatch,
    pushRefereeEvent,
    upsertBuff,
    applyDeployModeStatusPatch,
    queueChassisUpdate,
    queueGimbalUpdate,
    queueTelemetry,
    markDisconnected,
    resetToDefault,
  };
});
