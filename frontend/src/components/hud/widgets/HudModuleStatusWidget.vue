<template>
  <HudWidgetCard title="链路诊断" subtitle="Video Sources" accent="red">
    <div class="module-shell" :class="`side-${teamSide}`">
      <div class="diag-grid">
        <article v-for="item in stateRows" :key="item.name" class="diag-item">
          <span class="diag-name">{{ item.name }}</span>
          <b class="pill" :class="`state-${item.state}`">{{ item.text }}</b>
        </article>
      </div>

      <p class="status-note">{{ connection.message }}</p>
    </div>
  </HudWidgetCard>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { storeToRefs } from "pinia";
import HudWidgetCard from "../primitives/HudWidgetCard.vue";
import { useRobotDataStore } from "../../../store/robotData";
import type { ConnectionState } from "../../../types";

interface Props {
  teamSide?: "red" | "blue";
}

const props = withDefaults(defineProps<Props>(), {
  teamSide: "red",
});

type StatusTone = "online" | "warn" | "offline";

const robotStore = useRobotDataStore();
const { connection } = storeToRefs(robotStore);
const teamSide = computed(() => props.teamSide);

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

function resolveSourceTone(available: boolean): StatusTone {
  return available ? "online" : "offline";
}

function formatRate(value: number, unit: string): string {
  if (!Number.isFinite(value) || value <= 0) return "--";
  return `${value.toFixed(1)} ${unit}`;
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0%";
  return `${Math.round(value * 100)}%`;
}

const activeRateText = computed(() =>
  connection.value.activeSource === "custom"
    ? formatRate(connection.value.customAUFPS, "fps")
    : formatRate(connection.value.officialFrameRateHz, "fps"),
);

const activeIssueText = computed(() =>
  connection.value.activeSource === "custom"
    ? `${formatPercent(connection.value.customDropRate)} / ${formatPercent(connection.value.customCorruptRate)}`
    : formatPercent(connection.value.officialDropRate),
);

// What: 诊断卡现在只回答双视频源和当前显示链路的真实状态。
// Why: 精简版客户端已经去掉输入、模块编辑和外设控制，再显示旧模块标签只会继续制造误导。
const stateRows = computed(() => [
  {
    name: "当前源",
    state: connection.value.activeSource === "custom" ? "warn" : "online",
    text: connection.value.activeSource === "custom" ? "自定义" : "官方",
  },
  {
    name: "官方源",
    state: resolveSourceTone(connection.value.officialAvailable),
    text: connection.value.officialAvailable ? "在线" : "离线",
  },
  {
    name: "自定义源",
    state: resolveSourceTone(connection.value.customAvailable),
    text: connection.value.customAvailable ? "在线" : "离线",
  },
  {
    name: "MQTT链路",
    state: connection.value.controlLinkConnected ? "online" : "offline",
    text: connection.value.controlLinkConnected ? "在线" : "离线",
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
  {
    name: "源速率",
    state: connection.value.backendConnected ? "online" : "offline",
    text: activeRateText.value,
  },
  {
    name: connection.value.activeSource === "custom" ? "丢包/坏帧" : "丢帧率",
    state:
      connection.value.activeSource === "custom"
        ? connection.value.customDropRate > 0.02 || connection.value.customCorruptRate > 0
          ? "warn"
          : "online"
        : connection.value.officialDropRate > 0.02
          ? "warn"
          : "online",
    text: activeIssueText.value,
  },
] as const);
</script>

<style scoped>
.module-shell.side-blue {
  filter: hue-rotate(170deg) saturate(0.95);
}

.module-shell {
  height: 100%;
  min-height: 0;
  display: grid;
  grid-template-rows: minmax(0, 1fr) auto;
  gap: 8px;
}

.diag-grid {
  min-height: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  grid-auto-rows: minmax(0, 1fr);
  gap: 7px;
}

.diag-item {
  min-width: 0;
  min-height: 0;
  padding: 7px 8px;
  border-radius: 14px;
  border: 1px solid rgba(83, 106, 154, 0.26);
  background: rgba(9, 14, 27, 0.38);
  display: grid;
  align-content: space-between;
  gap: 4px;
}

.diag-name {
  min-width: 0;
  font-size: 10px;
  line-height: 1.2;
  color: rgba(213, 222, 239, 0.95);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pill {
  justify-self: start;
  min-height: 18px;
  max-width: 100%;
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

.status-note {
  margin: 0;
  min-height: 28px;
  font-size: 10px;
  line-height: 1.4;
  color: rgba(201, 211, 231, 0.84);
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
</style>
