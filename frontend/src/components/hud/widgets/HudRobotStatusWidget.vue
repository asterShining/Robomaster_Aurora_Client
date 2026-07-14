<template>
  <section
    class="robot-replica hud-realtime"
    :class="[`side-${teamSide}`, `alert-${props.alertLevel}`]"
  >
    <aside class="avatar-col">
      <HudRobotAvatar :team-side="teamSide" :code="avatarCode" />
    </aside>

    <section class="content-col">
      <header class="title-strip">
        <span>{{ hpTitleText }}</span>
      </header>

      <section
        class="hp-panel"
        :class="{ unavailable: !hasRealCombat }"
        data-testid="hud-robot-hp-panel"
      >
        <div class="hp-panel__head">
          <span>生命值</span>
          <b>{{ hpValueText }}</b>
        </div>

        <div class="hp-track" data-testid="hud-robot-hp-track">
          <i
            class="hp-track__fill"
            data-testid="hud-robot-hp-fill"
            :style="{ width: `${hpPercent}%` }"
          ></i>
        </div>

        <small>{{ hpFootText }}</small>
      </section>

      <ul class="status-list">
        <li v-for="item in statusRows" :key="item.name">
          <span>{{ item.name }}</span>
          <b class="pill" :class="`state-${item.state}`">{{ item.text }}</b>
        </li>
      </ul>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { storeToRefs } from "pinia";
import { useRobotDataStore } from "../../../store/robotData";
import type { ConnectionState } from "../../../types";
import HudRobotAvatar from "../primitives/HudRobotAvatar.vue";

interface Props {
  hp: number;
  maxHp: number;
  teamSide?: "red" | "blue";
  alertLevel?: "normal" | "warn" | "critical";
}

const props = withDefaults(defineProps<Props>(), {
  teamSide: "red",
  alertLevel: "normal",
});

type StatusTone = "online" | "warn" | "offline";

const robotStore = useRobotDataStore();
const { combat, connection } = storeToRefs(robotStore);
const teamSide = computed(() => props.teamSide);
const hp = computed(() => props.hp);
const maxHp = computed(() => props.maxHp);
const avatarCode = computed(() => (teamSide.value === "red" ? "R1" : "B1"));

// What: 本车卡仍然只在拿到真实 combat-state 后才把血量视为可信。
// Why: 功能精简后这张卡承担的是“我车当前状态”核心职责，不能再拿默认值伪装成实车在线。
const hasRealCombat = computed(() => combat.value.origin === "backend");

const hpPercent = computed(() => {
  if (!hasRealCombat.value) return 0;
  return Math.max(
    0,
    Math.min(100, Math.round((hp.value / Math.max(1, maxHp.value)) * 100)),
  );
});

const hpTitleText = computed(() =>
  hasRealCombat.value ? `本车状态 ${hp.value} / ${maxHp.value}` : "本车状态 未接入",
);
const hpValueText = computed(() =>
  hasRealCombat.value ? `${hp.value} / ${maxHp.value}` : "未接入",
);
const hpFootText = computed(() =>
  hasRealCombat.value ? `当前生命值 ${hpPercent.value}%` : "等待真实 combat-state",
);

function resolveDisplayStateLabel(state: ConnectionState["videoDisplayState"]): string {
  if (state === "live") return "正常";
  if (state === "waiting_source") return "等待源";
  if (state === "waiting_frame") return "待协议/首帧";
  if (state === "resyncing") return "重同步";
  if (state === "stalled") return "已断开";
  return "异常";
}

function resolveDisplayStateTone(state: ConnectionState["videoDisplayState"]): StatusTone {
  if (state === "live") return "online";
  if (state === "runtime_error") return "offline";
  return "warn";
}

// What: 本车卡现在只保留和当前画面、当前战斗态直接相关的状态行。
// Why: 外设控制、买弹和旧模块标签都已下线，继续显示这些字段只会挤占血量卡的核心阅读空间。
const statusRows = computed(() => [
  {
    name: "当前视频源",
    state: connection.value.activeSource === "custom" ? "warn" : "online",
    text: connection.value.activeSource === "custom" ? "自定义" : "官方",
  },
  {
    name: "战斗态",
    state: hasRealCombat.value ? "online" : "offline",
    text: hasRealCombat.value ? "已接入" : "未接入",
  },
  {
    name: "显示状态",
    state: resolveDisplayStateTone(connection.value.videoDisplayState),
    text: resolveDisplayStateLabel(connection.value.videoDisplayState),
  },
  {
    name: "当前延迟",
    state: connection.value.backendConnected ? "online" : "offline",
    text: connection.value.latencyMs > 0 ? `${connection.value.latencyMs} ms` : "--",
  },
] as const);
</script>

<style scoped>
.robot-replica {
  width: 100%;
  height: 100%;
  border-radius: 24px;
  border: 1px solid var(--hud-panel-red-border);
  background: var(--hud-panel-red-bg);
  box-shadow: var(--hud-panel-red-shadow);
  display: grid;
  grid-template-columns: 64px 1fr;
  align-items: stretch;
  gap: 10px;
  padding: 8px 12px;
  overflow: hidden;
}

.robot-replica.side-blue {
  border-color: var(--hud-panel-blue-border);
  background: var(--hud-panel-blue-bg);
  box-shadow: var(--hud-panel-blue-shadow);
}

.robot-replica.alert-warn {
  box-shadow: var(--hud-panel-red-shadow),
    0 0 0 1px rgba(255, 201, 126, 0.22);
}

.robot-replica.alert-critical {
  box-shadow: var(--hud-panel-red-shadow),
    0 0 0 1px rgba(255, 128, 142, 0.26);
  animation: warning-pulse 760ms ease-in-out infinite;
}

.avatar-col {
  display: grid;
  place-items: center;
}

.content-col {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr);
  gap: 8px;
  min-width: 0;
  min-height: 0;
}

.title-strip {
  min-height: 26px;
  border-radius: 14px;
  border: 1px solid var(--hud-strip-red-border);
  background: var(--hud-strip-red-bg);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 10px;
}

.side-blue .title-strip {
  border-color: var(--hud-strip-blue-border);
  background: var(--hud-strip-blue-bg);
}

.title-strip span {
  font-size: 14px;
  line-height: 1;
  color: var(--hud-text-primary);
  font-family: var(--font-data);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.hp-panel {
  padding: 10px 12px;
  border-radius: 16px;
  border: 1px solid rgba(179, 98, 105, 0.42);
  background: linear-gradient(
    165deg,
    rgba(42, 14, 20, 0.92),
    rgba(18, 9, 12, 0.82)
  );
  display: grid;
  gap: 7px;
}

.side-blue .hp-panel {
  border-color: rgba(92, 130, 196, 0.42);
  background: linear-gradient(
    165deg,
    rgba(10, 24, 46, 0.92),
    rgba(8, 13, 25, 0.82)
  );
}

.hp-panel.unavailable {
  border-color: rgba(115, 127, 153, 0.3);
  background: linear-gradient(
    165deg,
    rgba(18, 22, 31, 0.9),
    rgba(10, 13, 18, 0.84)
  );
}

.hp-panel__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
}

.hp-panel__head span {
  min-width: 0;
  font-size: 12px;
  color: rgba(255, 214, 214, 0.92);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.side-blue .hp-panel__head span {
  color: rgba(214, 234, 255, 0.92);
}

.hp-panel__head b {
  min-width: 0;
  font-size: 17px;
  line-height: 1;
  color: #fff0de;
  font-family: var(--font-data);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.side-blue .hp-panel__head b {
  color: #e3f6ff;
}

.hp-panel small {
  color: var(--hud-text-secondary);
  font-size: 10px;
}

.hp-track {
  position: relative;
  width: 100%;
  height: 10px;
  border-radius: 999px;
  overflow: hidden;
  border: 1px solid var(--hud-track-red-border);
  background: var(--hud-track-red-bg);
}

.side-blue .hp-track {
  border-color: var(--hud-track-blue-border);
  background: var(--hud-track-blue-bg);
}

.hp-track__fill {
  position: absolute;
  inset: 0 auto 0 0;
  background: var(--hud-track-red-fill);
}

.side-blue .hp-track__fill {
  background: var(--hud-track-blue-fill);
}

.status-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-auto-rows: minmax(0, 1fr);
  gap: 6px;
  min-width: 0;
  min-height: 0;
}

.status-list li {
  min-width: 0;
  min-height: 0;
  border-radius: 12px;
  border: 1px solid rgba(90, 112, 162, 0.26);
  background: rgba(10, 16, 30, 0.44);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 0 10px;
}

.status-list span {
  min-width: 0;
  flex: 1 1 auto;
  color: var(--hud-text-secondary);
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pill {
  min-height: 18px;
  flex: 0 0 auto;
  max-width: 56%;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 10px;
  line-height: 18px;
  font-weight: 600;
  font-family: var(--font-data);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pill.state-online {
  color: #9af7c8;
  background: rgba(29, 83, 62, 0.5);
}

.pill.state-warn {
  color: #ffd793;
  background: rgba(88, 64, 20, 0.48);
}

.pill.state-offline {
  color: #ffaaaa;
  background: rgba(94, 37, 44, 0.5);
}
</style>
