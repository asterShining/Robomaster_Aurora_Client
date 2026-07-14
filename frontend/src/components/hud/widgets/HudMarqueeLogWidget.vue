<template>
  <HudWidgetCard title="跑马灯提示区" subtitle="System Log" accent="blue">
    <div class="marquee-wrap">
      <div class="marquee-track">
        <span
          v-for="(line, idx) in displayLines"
          :key="`line-${idx}`"
          class="line"
          >{{ line }}</span
        >
      </div>
    </div>
  </HudWidgetCard>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { storeToRefs } from "pinia";
import HudWidgetCard from "../primitives/HudWidgetCard.vue";
import { useRobotDataStore } from "../../../store/robotData";

const robotStore = useRobotDataStore();
const { combat, connection, matchState, radar } = storeToRefs(robotStore);

function resolveStageLabel(stage: number): string {
  // What: 将官方阶段码转成简短文本。
  // Why: 跑马灯空间有限，只保留用于快速扫读的最小标签。
  if (stage === 0) return "未开始";
  if (stage === 1) return "准备";
  if (stage === 2) return "自检";
  if (stage === 3) return "5秒倒计时";
  if (stage === 4) return "比赛中";
  if (stage === 5) return "结算中";
  return "未知";
}

// What: 跑马灯只展示真实链路与真实比赛状态。
// Why: 用户已经明确指出旧的 chassis/gimbal 假值会误导判断，因此这里彻底改成“接入状态 + 真实摘要”。
const displayLines = computed(() => {
  const matchLine = matchState.value.gameStatusReady
    ? `[MATCH] ${resolveStageLabel(matchState.value.currentStage)} R${matchState.value.currentRound}/${matchState.value.totalRounds || "-"} ${matchState.value.redScore}:${matchState.value.blueScore} T-${matchState.value.stageCountdownSec}s`
    : "[MATCH] 未接入真实 GameStatus";
  const objectiveLine = matchState.value.globalStatusReady
    ? `[OBJECT] BASE ${matchState.value.red.baseHp}:${matchState.value.blue.baseHp} OUTPOST ${matchState.value.red.outpostHp}:${matchState.value.blue.outpostHp}`
    : "[OBJECT] 未接入真实 GlobalUnitStatus";
  const damageLine = matchState.value.globalStatusReady
    ? `[DAMAGE] TOTAL ${matchState.value.red.totalDamage}:${matchState.value.blue.totalDamage} OUTPOST_ST ${matchState.value.red.outpostStatus}:${matchState.value.blue.outpostStatus}`
    : "[DAMAGE] 等待真实总伤害/前哨站状态";
  const combatLine =
    combat.value.origin === "backend"
      ? `[COMBAT] HP=${combat.value.hp}/${combat.value.maxHp} REMOTE_AMMO=${combat.value.canRemoteAmmo ? "ALLOW" : "DENY"}`
      : "[COMBAT] 未接入真实 combat-state";
  const radarLine =
    radar.value.source === "backend"
      ? `[RADAR] 已接入 contacts=${radar.value.contacts.length}`
      : "[RADAR] 未接入真实 radar-state";

  return [
    `[LINK] VIDEO=${connection.value.backendConnected ? "ON" : "WAIT"} MQTT=${connection.value.controlLinkConnected ? "ON" : "OFF"} DISPLAY=${connection.value.videoConnected ? "LIVE" : "HOLD"}`,
    matchLine,
    objectiveLine,
    damageLine,
    combatLine,
    radarLine,
    `[NET] latency=${connection.value.latencyMs}ms quality=${connection.value.linkQuality}`,
  ];
});
</script>

<style scoped>
.marquee-wrap {
  height: 100%;
  overflow: hidden;
  border: 1px solid rgba(91, 132, 216, 0.34);
  background: rgba(9, 14, 25, 0.74);
}

.marquee-track {
  display: grid;
  gap: 6px;
  padding: 7px 8px;
  animation: marquee-scroll 8s linear infinite;
}

.line {
  font-size: 11px;
  color: rgba(193, 213, 245, 0.92);
  font-family: "JetBrains Mono", "Consolas", monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* What: 轻量纵向滚动。Why: 在有限区域展示多条诊断信息，同时保持视觉密度接近官方界面。 */
@keyframes marquee-scroll {
  0% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-22%);
  }
  100% {
    transform: translateY(0);
  }
}
</style>
