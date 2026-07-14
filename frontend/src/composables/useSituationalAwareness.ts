import { computed, onScopeDispose, ref, watch } from "vue";
import { storeToRefs } from "pinia";
import type { AwarenessTargetCue } from "../types";
import { useRobotDataStore } from "../store/robotData";

export type AwarenessLevel = "normal" | "warn" | "critical";
export type AmmoBand = "normal" | "warn" | "critical";

export interface AwarenessAlert {
  id: string;
  kind: "low-hp" | "overheat" | "capacitor" | "countdown";
  title: string;
  detail: string;
  tone: "danger" | "warning";
  level: AwarenessLevel;
}

const LOW_HP_WARN_RATIO = 0.3;
const LOW_HP_CRITICAL_RATIO = 0.15;
const LOW_HP_WARN_EXIT_RATIO = 0.35;
const LOW_HP_CRITICAL_EXIT_RATIO = 0.2;
const AMMO_WARN_RATIO = 0.25;
const AMMO_CRITICAL_RATIO = 0.1;
// What: 定义超级电容与比赛资源的增强器阈值。
// Why: 这些参数会同时驱动 HUD 高亮和中心提醒，集中声明便于后续按实战体验调教。
const CAPACITOR_WARN_RATIO = 0.25;
const CAPACITOR_CRITICAL_RATIO = 0.12;
const CAPACITOR_WARN_EXIT_RATIO = 0.3;
const CAPACITOR_CRITICAL_EXIT_RATIO = 0.16;
const MATCH_TIME_WARN_SEC = 60;
const MATCH_TIME_CRITICAL_SEC = 20;
const HEAT_SAMPLE_WINDOW_MS = 400;
const HEAT_LOOKAHEAD_MS = 500;
const HEAT_FALLBACK_RATIO = 0.9;
const HEAT_CLEAR_RATIO = 0.85;
const ALERT_DURATION_MS = 1300;
const OVERHEAT_ALERT_COOLDOWN_MS = 1200;

function levelRank(level: AwarenessLevel): number {
  if (level === "critical") return 2;
  if (level === "warn") return 1;
  return 0;
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

// What: 用带滞回的资源阈值状态机计算等级。
// Why: 血量和电容会在阈值附近抖动，没有退出阈值时会频繁来回跳级，造成提示闪烁。
function resolveDescendingLevel(
  current: AwarenessLevel,
  value: number,
  options: {
    warnEnter: number;
    criticalEnter: number;
    warnExit: number;
    criticalExit: number;
  },
): AwarenessLevel {
  if (current === "normal") {
    if (value <= options.criticalEnter) return "critical";
    if (value <= options.warnEnter) return "warn";
    return "normal";
  }

  if (current === "warn") {
    if (value <= options.criticalEnter) return "critical";
    if (value >= options.warnExit) return "normal";
    return "warn";
  }

  if (value >= options.criticalExit) {
    return value <= options.warnEnter ? "warn" : "normal";
  }
  return "critical";
}

function formatAngleLabel(angleDeg: number): string {
  if (angleDeg <= -25) return "左侧威胁";
  if (angleDeg < -8) return "左前威胁";
  if (angleDeg < 8) return "正前威胁";
  if (angleDeg < 25) return "右前威胁";
  return "右侧威胁";
}

function distanceOf(x: number, y: number): number {
  return Math.sqrt(x * x + y * y);
}

export function useSituationalAwareness() {
  const robotStore = useRobotDataStore();
  const { combat, matchState, radar, vision } = storeToRefs(robotStore);

  const lowHpLevel = ref<AwarenessLevel>("normal");
  const capacitorLevel = ref<AwarenessLevel>("normal");
  const overheatActive = ref(false);
  const heatSamples = ref<Array<{ at: number; heat: number }>>([]);
  const alertQueue = ref<AwarenessAlert[]>([]);

  let alertTimer: number | null = null;
  let lastOverheatAlertAt = 0;

  const hpRatio = computed(() => {
    const safeMax = Math.max(1, combat.value.maxHp);
    return clamp(combat.value.hp / safeMax, 0, 1);
  });

  const ammoRatio = computed(() => {
    const safeMax = Math.max(1, combat.value.maxAmmo);
    return clamp(combat.value.ammo / safeMax, 0, 1);
  });

  const capacitorRatio = computed(() =>
    clamp(combat.value.capacitorPct / 100, 0, 1),
  );

  const ammoVisualState = computed(() => {
    const level: AmmoBand =
      ammoRatio.value <= AMMO_CRITICAL_RATIO
        ? "critical"
        : ammoRatio.value <= AMMO_WARN_RATIO
          ? "warn"
          : "normal";
    const segmentCount = 12;
    const activeSegments = Math.ceil(ammoRatio.value * segmentCount);

    return {
      ratio: ammoRatio.value,
      level,
      segmentCount,
      activeSegments,
    };
  });

  const heatTrendState = computed(() => {
    const samples = heatSamples.value;
    const latest = samples[samples.length - 1];
    if (!latest) {
      return {
        predictedHeat: combat.value.heat,
        predictedRatio: clamp(
          combat.value.heat / Math.max(1, combat.value.maxHeat),
          0,
          1,
        ),
        reliable: false,
        slopePerMs: 0,
      };
    }

    const earliest = samples[0];
    const safeWindow = Math.max(1, latest.at - earliest.at);
    const slopePerMs = (latest.heat - earliest.heat) / safeWindow;
    const predictedHeat = combat.value.heat + slopePerMs * HEAT_LOOKAHEAD_MS;

    return {
      predictedHeat,
      predictedRatio: clamp(
        predictedHeat / Math.max(1, combat.value.maxHeat),
        0,
        1,
      ),
      reliable: safeWindow >= 120 && samples.length >= 2,
      slopePerMs,
    };
  });

  const imminentOverheatState = computed(() => ({
    active: overheatActive.value,
    predictedHeat: heatTrendState.value.predictedHeat,
    predictedRatio: heatTrendState.value.predictedRatio,
    currentRatio: clamp(
      combat.value.heat / Math.max(1, combat.value.maxHeat),
      0,
      1,
    ),
  }));

  // What: 将倒计时转换为比赛压力等级。
  // Why: 只在真实 GameStatus 且处于“比赛中”阶段时才允许触发时间提醒，避免默认 0 或准备阶段被误判成终局告警。
  const timePressureState = computed(() => {
    const countdownSec = Math.max(
      0,
      Math.round(matchState.value.stageCountdownSec),
    );
    const ready =
      matchState.value.gameStatusReady && matchState.value.currentStage === 4;
    const level: AwarenessLevel = !ready
      ? "normal"
      : countdownSec <= MATCH_TIME_CRITICAL_SEC
        ? "critical"
        : countdownSec <= MATCH_TIME_WARN_SEC
          ? "warn"
          : "normal";

    return {
      ready,
      value: countdownSec,
      level,
    };
  });

  const activeAlert = computed(() => alertQueue.value[0] ?? null);

  const primaryThreatCue = computed<AwarenessTargetCue | null>(() => {
    const enemyRadar = [...radar.value.contacts]
      .filter((item) => item.side === "enemy")
      .sort((left, right) => {
        const confidenceGap = right.confidence - left.confidence;
        if (Math.abs(confidenceGap) > 0.001) return confidenceGap;
        return distanceOf(left.x, left.y) - distanceOf(right.x, right.y);
      });

    const enemyVision = [...vision.value.targets]
      .filter((item) => item.side === "enemy")
      .sort((left, right) => {
        const confidenceGap = right.confidence - left.confidence;
        if (Math.abs(confidenceGap) > 0.001) return confidenceGap;
        return (
          Math.abs((left.xNorm ?? 0.5) - 0.5) -
          Math.abs((right.xNorm ?? 0.5) - 0.5)
        );
      });

    const bestRadar = enemyRadar[0] ?? null;
    const bestVision = enemyVision[0] ?? null;

    // What: 视觉确认优先于纯雷达点迹。
    // Why: 视觉锁定更贴近操作手当前瞄准语义，可减少“看见和提示不一致”的割裂感。
    if (bestVision) {
      return {
        source: "vision",
        targetId: bestVision.id,
        radarId: bestRadar?.id ?? null,
        label: bestVision.armorLabel
          ? `${bestVision.armorLabel} 装甲`
          : "视觉锁定",
        side: bestVision.side,
        confidence: bestVision.confidence,
        angleDeg: (bestVision.xNorm - 0.5) * 60,
        distanceM: bestRadar ? distanceOf(bestRadar.x, bestRadar.y) : null,
        xNorm: bestVision.xNorm,
        yNorm: bestVision.yNorm,
        updatedAt: bestVision.updatedAt,
      };
    }

    if (!bestRadar) return null;

    return {
      source: "radar",
      targetId: bestRadar.id,
      radarId: bestRadar.id,
      label: formatAngleLabel(
        (Math.atan2(bestRadar.y, bestRadar.x) * 180) / Math.PI,
      ),
      side: bestRadar.side,
      confidence: bestRadar.confidence,
      angleDeg: (Math.atan2(bestRadar.y, bestRadar.x) * 180) / Math.PI,
      distanceM: distanceOf(bestRadar.x, bestRadar.y),
      xNorm: null,
      yNorm: null,
      updatedAt: bestRadar.updatedAt,
    };
  });

  const armorRecognitionCue = computed<AwarenessTargetCue | null>(() => {
    const cue = primaryThreatCue.value;
    if (
      !cue ||
      cue.source !== "vision" ||
      cue.xNorm === null ||
      cue.yNorm === null
    )
      return null;

    // What: 只为视觉主目标生成装甲板标记。
    // Why: 视觉识别点位和实际瞄准语义最一致，直接复用主目标结果可避免第二套选目标逻辑打架。
    return cue;
  });

  // What: 对超级电容输出一个稳定视图模型。
  // Why: 右侧火控卡和增强器配置都要消费同一份数据，避免组件自行再算百分比和等级。
  const capacitorState = computed(() => ({
    ratio: capacitorRatio.value,
    level: capacitorLevel.value,
    value: clamp(combat.value.capacitorPct, 0, 100),
  }));

  function pumpAlertQueue(): void {
    if (alertTimer !== null || alertQueue.value.length === 0) return;
    alertTimer = window.setTimeout(() => {
      alertQueue.value.shift();
      alertTimer = null;
      pumpAlertQueue();
    }, ALERT_DURATION_MS);
  }

  function enqueueAlert(alert: AwarenessAlert): void {
    const exists = alertQueue.value.some(
      (item) => item.kind === alert.kind && item.level === alert.level,
    );
    if (exists) return;
    alertQueue.value.push(alert);
    pumpAlertQueue();
  }

  watch(
    hpRatio,
    (ratio) => {
      lowHpLevel.value = resolveDescendingLevel(lowHpLevel.value, ratio, {
        warnEnter: LOW_HP_WARN_RATIO,
        criticalEnter: LOW_HP_CRITICAL_RATIO,
        warnExit: LOW_HP_WARN_EXIT_RATIO,
        criticalExit: LOW_HP_CRITICAL_EXIT_RATIO,
      });
    },
    { immediate: true },
  );

  watch(lowHpLevel, (next, prev) => {
    if (levelRank(next) <= levelRank(prev)) return;
    if (next === "normal") return;

    // What: 低血量告警只在跨阈值瞬间入队。
    // Why: 持续低血量阶段若反复弹窗，会严重干扰操作手瞄准和读图。
    enqueueAlert({
      id: `low-hp-${next}-${Date.now()}`,
      kind: "low-hp",
      title: next === "critical" ? "血量危险" : "血量偏低",
      detail: `当前血量 ${(hpRatio.value * 100).toFixed(0)}%`,
      tone: "danger",
      level: next,
    });
  });

  watch(
    capacitorRatio,
    (ratio) => {
      capacitorLevel.value = resolveDescendingLevel(
        capacitorLevel.value,
        ratio,
        {
          warnEnter: CAPACITOR_WARN_RATIO,
          criticalEnter: CAPACITOR_CRITICAL_RATIO,
          warnExit: CAPACITOR_WARN_EXIT_RATIO,
          criticalExit: CAPACITOR_CRITICAL_EXIT_RATIO,
        },
      );
    },
    { immediate: true },
  );

  watch(capacitorLevel, (next, prev) => {
    if (levelRank(next) <= levelRank(prev)) return;
    if (next !== "critical") return;

    // What: 仅在超级电容跌入危险区时推送中心提醒。
    // Why: 电容是持续资源，常规偏低可交给条形图表达，只有临界值才值得抢占中心视线。
    enqueueAlert({
      id: `capacitor-${next}-${Date.now()}`,
      kind: "capacitor",
      title: "超级电容告急",
      detail: `当前余量 ${Math.round(capacitorRatio.value * 100)}%`,
      tone: "warning",
      level: next,
    });
  });

  watch(
    () => [combat.value.heat, combat.value.updatedAt] as const,
    ([heat]) => {
      const now = Date.now();
      heatSamples.value = [...heatSamples.value, { at: now, heat }].filter(
        (item) => now - item.at <= HEAT_SAMPLE_WINDOW_MS,
      );
    },
    { immediate: true },
  );

  watch(
    heatTrendState,
    (next) => {
      const fallbackHot = next.predictedRatio >= HEAT_FALLBACK_RATIO;
      const reliableHot =
        next.reliable &&
        next.predictedHeat >= combat.value.maxHeat &&
        next.slopePerMs > 0;

      if (!overheatActive.value && (reliableHot || fallbackHot)) {
        overheatActive.value = true;
        const now = Date.now();
        if (now - lastOverheatAlertAt >= OVERHEAT_ALERT_COOLDOWN_MS) {
          lastOverheatAlertAt = now;
          // What: 提前 0.5 秒的过热风险只推送一次简短预警。
          // Why: 让操作手及时停火，同时避免每一帧都刷同一条提示。
          enqueueAlert({
            id: `overheat-${now}`,
            kind: "overheat",
            title: "即将过热",
            detail: `预测热量 ${Math.round(next.predictedHeat)} / ${combat.value.maxHeat}`,
            tone: "warning",
            level: "warn",
          });
        }
        return;
      }

      if (!overheatActive.value) return;
      const safeCurrentRatio = clamp(
        combat.value.heat / Math.max(1, combat.value.maxHeat),
        0,
        1,
      );
      if (
        next.predictedRatio < HEAT_FALLBACK_RATIO &&
        safeCurrentRatio < HEAT_CLEAR_RATIO
      ) {
        overheatActive.value = false;
      }
    },
    { immediate: true },
  );

  watch(
    () => timePressureState.value.level,
    (next, prev) => {
      if (!timePressureState.value.ready) return;
      if (levelRank(next) <= levelRank(prev ?? "normal")) return;
      if (next !== "critical") return;

      // What: 只在读秒阶段推送比赛时间压力提醒。
      // Why: 终局二十秒会直接改变操作节奏，中心短提示比一直闪烁更符合实战习惯。
      enqueueAlert({
        id: `countdown-${Date.now()}`,
        kind: "countdown",
        title: "终局时间压力",
        detail: `剩余 ${timePressureState.value.value}s`,
        tone: "warning",
        level: next,
      });
    },
    { immediate: true },
  );

  onScopeDispose(() => {
    if (alertTimer !== null) {
      window.clearTimeout(alertTimer);
      alertTimer = null;
    }
  });

  return {
    lowHpState: computed(() => ({
      ratio: hpRatio.value,
      level: lowHpLevel.value,
    })),
    ammoVisualState,
    capacitorState,
    imminentOverheatState,
    timePressureState,
    primaryThreatCue,
    armorRecognitionCue,
    activeAlert,
    alertQueue,
  };
}
