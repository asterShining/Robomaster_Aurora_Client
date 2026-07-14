<template>
  <aside
    v-if="uiPanelStore.isOpen"
    ref="panelRef"
    class="center-panel hud-realtime"
    role="dialog"
    aria-label="视频与HUD设置"
    @click.stop
  >
    <header class="panel-head">
      <div>
        <h2>视频与HUD</h2>
        <p>双视频常驻，当前面板只切换主画面</p>
      </div>

      <button class="close-btn" type="button" @click="uiPanelStore.forceClose">
        ×
      </button>
    </header>

    <section class="panel-section">
      <h3>主画面来源</h3>

      <div class="source-grid">
        <button
          class="source-btn"
          :class="{ active: applied.videoSource === 'official' }"
          type="button"
          @click="applyVideoSource('official')"
        >
          <strong>官方视频</strong>
          <small>{{ officialButtonText }}</small>
        </button>

        <button
          class="source-btn"
          :class="{ active: applied.videoSource === 'custom' }"
          :disabled="!allowsCustomVideo"
          type="button"
          @click="applyVideoSource('custom')"
        >
          <strong>自定义视频</strong>
          <small>{{ customButtonText }}</small>
        </button>
      </div>
    </section>

    <section class="panel-section">
      <h3>机器人身份</h3>

      <div class="identity-groups">
        <article class="identity-group identity-group--red">
          <span>红方</span>
          <div class="identity-grid">
            <button
              v-for="option in redRobotIdentityOptions"
              :key="option.id"
              class="identity-btn"
              :class="{ active: applied.mqttRobotIdentity === option.id }"
              type="button"
              @click="applyMQTTRobotIdentity(option.id)"
            >
              <strong>{{ option.shortLabel }}</strong>
              <small>ID {{ option.clientId }}</small>
            </button>
          </div>
        </article>

        <article class="identity-group identity-group--blue">
          <span>蓝方</span>
          <div class="identity-grid">
            <button
              v-for="option in blueRobotIdentityOptions"
              :key="option.id"
              class="identity-btn"
              :class="{ active: applied.mqttRobotIdentity === option.id }"
              type="button"
              @click="applyMQTTRobotIdentity(option.id)"
            >
              <strong>{{ option.shortLabel }}</strong>
              <small>ID {{ option.clientId }}</small>
            </button>
          </div>
        </article>
      </div>
    </section>

    <section class="panel-section">
      <h3>分辨率</h3>
      <select
        class="resolution-select"
        :value="applied.resolution"
        @change="applyResolution(($event.target as HTMLSelectElement).value)"
      >
        <option
          v-for="res in RESOLUTION_PRESETS"
          :key="res"
          :value="res"
        >
          {{ res }}
        </option>
      </select>
    </section>

    <section class="panel-section">
      <h3>状态卡</h3>

      <label class="toggle-row">
        <input
          type="checkbox"
          :checked="applied.showDiagnosticsCard"
          @change="applyBooleanPatch('showDiagnosticsCard', $event)"
        />
        <span>链路诊断卡</span>
      </label>

      <label class="toggle-row">
        <input
          type="checkbox"
          :checked="applied.showRobotStatusCard"
          @change="applyBooleanPatch('showRobotStatusCard', $event)"
        />
        <span>本车状态卡</span>
      </label>

      <label class="toggle-row">
        <input
          type="checkbox"
          :checked="applied.showFireControlCard"
          @change="applyBooleanPatch('showFireControlCard', $event)"
        />
        <span>火控状态卡</span>
      </label>
    </section>

    <section class="panel-section panel-section-diagnostics">
      <h3>当前状态</h3>

      <div class="diag-grid">
        <article class="diag-card">
          <span>主画面</span>
          <b>{{ activeSourceLabel }}</b>
        </article>

        <article class="diag-card">
          <span>PiP</span>
          <b>{{ pipSourceLabel }}</b>
        </article>

        <article class="diag-card">
          <span>官方链路</span>
          <b>{{ officialSourceText }}</b>
        </article>

        <article class="diag-card">
          <span>自定义链路</span>
          <b>{{ customSourceText }}</b>
        </article>

        <article class="diag-card">
          <span>机器人身份</span>
          <b>{{ mqttRobotIdentityLabel }}</b>
        </article>

        <article class="diag-card">
          <span>主画面状态</span>
          <b>{{ displayStateLabel }}</b>
        </article>
      </div>

      <p class="status-note">{{ currentMainMessage }}</p>
      <p v-if="uiPanelStore.saveMessage" class="save-note">
        {{ uiPanelStore.saveMessage }}
      </p>
    </section>

    <section class="panel-section panel-section-credits">
      <button class="credits-btn" type="button" @click="showCredits = true">
        <span>鸣谢名单</span>
        <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor">
          <path d="M5 2l5 5-5 5" stroke="currentColor" stroke-width="1.5" fill="none"/>
        </svg>
      </button>
    </section>
  </aside>

  <section
    v-if="showCredits"
    class="credits-backdrop"
    role="dialog"
    aria-modal="true"
    aria-label="鸣谢名单"
    @click="showCredits = false"
  >
    <article class="credits-card" @click.stop>
      <header class="credits-head">
        <h3>鸣谢名单</h3>
        <button class="close-btn" type="button" @click="showCredits = false">&times;</button>
      </header>

      <ul class="credits-list">
        <li v-for="person in creditsList" :key="person.name" class="credits-person">
          <strong>{{ person.name }}</strong>
          <span v-if="person.role">{{ person.role }}</span>
          <small v-if="person.note">{{ person.note }}</small>
        </li>
      </ul>

      <footer class="credits-footer">
        <p>感谢所有为项目做出贡献的人</p>
      </footer>
    </article>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { storeToRefs } from "pinia";
import { useUiPanelStore } from "../../store/uiPanel";
import { useRobotDataStore } from "../../store/robotData";
import type { ConnectionState } from "../../types";
import type { MQTTRobotIdentity, VideoSource } from "../../types/ui";
import {
  MQTT_ROBOT_IDENTITY_OPTIONS,
  RESOLUTION_PRESETS,
  isHeroRobotIdentity,
  resolveMQTTRobotIdentityLabel,
} from "../../types/ui";

const panelRef = ref<HTMLElement | null>(null);
const showCredits = ref(false);

interface CreditPerson {
  name: string;
  role?: string;
  note?: string;
}

const creditsList: CreditPerson[] = [
  {
    name: "张天乐",
    role: "牢牢牢英雄电控",
    note: "经常带火腿肠来实验室，基本都进了我的肚子",
  },
  {
    name: "聂政华 (Neomelt)",
    role: "牢哨兵导航",
    note: "项目初期技术选型和全程思路点拨，帮助巨大",
  },
];

const uiPanelStore = useUiPanelStore();
const robotStore = useRobotDataStore();
const { applied } = storeToRefs(uiPanelStore);
const { connection } = storeToRefs(robotStore);

const redRobotIdentityOptions = MQTT_ROBOT_IDENTITY_OPTIONS.filter(
  (option) => option.side === "red",
);
const blueRobotIdentityOptions = MQTT_ROBOT_IDENTITY_OPTIONS.filter(
  (option) => option.side === "blue",
);

const activeSourceLabel = computed(() =>
  connection.value.activeSource === "custom" ? "自定义视频源" : "官方视频源",
);

const pipSourceLabel = computed(() =>
  allowsCustomVideo.value && connection.value.pipSource === "custom"
    ? "自定义视频源"
    : "官方视频源",
);

const allowsCustomVideo = computed(() =>
  isHeroRobotIdentity(applied.value.mqttRobotIdentity),
);

function resolveSourceRole(source: VideoSource): string {
  if (connection.value.activeSource === source) return "主画面";
  if (connection.value.pipSource === source) return "PiP";
  return "待机";
}

function resolveDisplayStateLabel(state: ConnectionState["videoDisplayState"]): string {
  if (state === "live") return "正常";
  if (state === "waiting_source") return "等待源";
  if (state === "waiting_frame") return "等待协议/首帧";
  if (state === "resyncing") return "重同步";
  if (state === "stalled") return "已断开";
  return "异常";
}

const officialSourceText = computed(() =>
  `${resolveSourceRole("official")} · ${resolveDisplayStateLabel(connection.value.officialDisplayState)}`,
);

const customSourceText = computed(() =>
  allowsCustomVideo.value
    ? `${resolveSourceRole("custom")} · ${resolveDisplayStateLabel(connection.value.customDisplayState)}`
    : "当前身份不使用",
);

const officialButtonText = computed(() =>
  applied.value.videoSource === "official"
    ? "当前主画面 · custom 自动进入 PiP"
    : "设为主画面 · 当前作为 PiP/待机",
);

const customButtonText = computed(() =>
  !allowsCustomVideo.value
    ? "仅英雄机器人可用"
    : applied.value.videoSource === "custom"
      ? "当前主画面 · official 自动进入 PiP"
      : "设为主画面 · 当前作为 PiP/待机",
);

const mqttRobotIdentityLabel = computed(() =>
  resolveMQTTRobotIdentityLabel(applied.value.mqttRobotIdentity),
);

const displayStateLabel = computed(() =>
  resolveDisplayStateLabel(connection.value.videoDisplayState),
);

const currentMainMessage = computed(() =>
  connection.value.activeSource === "custom"
    ? connection.value.customMessage
    : connection.value.officialMessage,
);

// What: 视频源切换直接落到 store 的即时保存方法。
// Why: 当前小面板不再保留“改完再点保存”的复杂流程，用户切源后必须立刻让后端跟着切。
async function applyVideoSource(source: VideoSource): Promise<void> {
  await uiPanelStore.applyMinimalPanelPatch({ videoSource: source });
}

// What: MQTT 机器人身份切换直接落到 store 的即时保存方法。
// Why: 这项选择会直接改变后端 clientID，不能停留在前端草稿态。
async function applyMQTTRobotIdentity(identity: MQTTRobotIdentity): Promise<void> {
  await uiPanelStore.applyMinimalPanelPatch({ mqttRobotIdentity: identity });
}

async function applyResolution(resolution: string): Promise<void> {
  await uiPanelStore.applyMinimalPanelPatch({ resolution });
}

async function applyBooleanPatch(
  key: "showDiagnosticsCard" | "showRobotStatusCard" | "showFireControlCard",
  event: Event,
): Promise<void> {
  const target = event.target as HTMLInputElement | null;
  if (!target) return;

  if (key === "showDiagnosticsCard") {
    await uiPanelStore.applyMinimalPanelPatch({
      showDiagnosticsCard: target.checked,
    });
    return;
  }

  if (key === "showRobotStatusCard") {
    await uiPanelStore.applyMinimalPanelPatch({
      showRobotStatusCard: target.checked,
    });
    return;
  }

  await uiPanelStore.applyMinimalPanelPatch({
    showFireControlCard: target.checked,
  });
}
</script>

<style scoped>
.center-panel {
  position: absolute;
  right: calc(var(--pip-reserve-right) + 18px);
  top: 64px;
  z-index: var(--z-menu);
  width: min(360px, calc(100vw - 24px));
  border-radius: 24px;
  border: 1px solid rgba(81, 116, 190, 0.34);
  background: linear-gradient(165deg, rgba(8, 12, 22, 0.96), rgba(10, 17, 32, 0.94));
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.34);
  padding: 16px;
  display: grid;
  gap: 14px;
  pointer-events: auto;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.panel-head h2 {
  margin: 0;
  color: var(--hud-text-primary);
  font-size: 18px;
}

.panel-head p {
  margin: 6px 0 0;
  color: var(--hud-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.close-btn {
  width: 34px;
  height: 34px;
  border-radius: 999px;
  border: 1px solid rgba(96, 134, 208, 0.42);
  background: rgba(9, 15, 29, 0.92);
  color: var(--hud-text-primary);
  font-size: 18px;
}

.panel-section {
  display: grid;
  gap: 10px;
}

.panel-section h3 {
  margin: 0;
  color: var(--hud-text-primary);
  font-size: 13px;
  letter-spacing: 0.08em;
}

.source-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.source-btn {
  min-height: 76px;
  border-radius: 18px;
  border: 1px solid rgba(78, 108, 174, 0.38);
  background: rgba(8, 14, 27, 0.88);
  color: var(--hud-text-primary);
  display: grid;
  align-content: center;
  justify-items: start;
  gap: 6px;
  padding: 12px;
  text-align: left;
}

.source-btn.active {
  border-color: rgba(119, 183, 255, 0.8);
  box-shadow: 0 0 0 1px rgba(119, 183, 255, 0.22);
}

.source-btn:disabled {
  cursor: not-allowed;
  opacity: 0.52;
  border-color: rgba(78, 108, 174, 0.2);
}

.source-btn small {
  color: var(--hud-text-secondary);
  font-size: 11px;
}

.identity-groups {
  display: grid;
  gap: 10px;
}

.identity-group {
  border-radius: 16px;
  border: 1px solid rgba(74, 106, 172, 0.32);
  background: rgba(7, 13, 26, 0.72);
  padding: 10px;
  display: grid;
  gap: 8px;
}

.identity-group > span {
  color: var(--hud-text-secondary);
  font-size: 11px;
}

.identity-group--red {
  border-color: rgba(196, 86, 96, 0.34);
}

.identity-group--blue {
  border-color: rgba(86, 145, 220, 0.36);
}

.identity-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.identity-btn {
  min-height: 48px;
  border-radius: 12px;
  border: 1px solid rgba(78, 108, 174, 0.34);
  background: rgba(8, 14, 27, 0.88);
  color: var(--hud-text-primary);
  display: grid;
  align-content: center;
  justify-items: start;
  gap: 4px;
  padding: 8px 9px;
  text-align: left;
}

.identity-btn.active {
  border-color: rgba(119, 183, 255, 0.86);
  box-shadow: 0 0 0 1px rgba(119, 183, 255, 0.22);
}

.identity-btn strong {
  font-size: 12px;
  line-height: 1.2;
}

.identity-btn small {
  color: var(--hud-text-secondary);
  font-size: 10px;
  line-height: 1.2;
}

.toggle-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 42px;
  padding: 0 12px;
  border-radius: 14px;
  border: 1px solid rgba(73, 105, 170, 0.3);
  background: rgba(8, 13, 26, 0.78);
  color: var(--hud-text-primary);
}

.toggle-row input {
  width: 16px;
  height: 16px;
}

.diag-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.diag-card {
  min-height: 62px;
  border-radius: 16px;
  border: 1px solid rgba(68, 101, 167, 0.32);
  background: rgba(8, 13, 25, 0.82);
  padding: 10px 12px;
  display: grid;
  align-content: space-between;
  gap: 8px;
}

.diag-card span {
  color: var(--hud-text-secondary);
  font-size: 11px;
}

.diag-card b {
  color: var(--hud-text-primary);
  font-size: 13px;
}

.status-note,
.save-note {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
}

.status-note {
  color: var(--hud-text-secondary);
}

.save-note {
  color: rgba(188, 216, 255, 0.92);
}

.panel-section-credits {
  border-top: 1px solid rgba(78, 108, 174, 0.18);
  padding-top: 8px;
}

.credits-btn {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 42px;
  padding: 0 14px;
  border-radius: 14px;
  border: 1px solid rgba(73, 105, 170, 0.3);
  background: rgba(8, 13, 26, 0.78);
  color: var(--hud-text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s;
}

.credits-btn:hover {
  border-color: rgba(119, 183, 255, 0.6);
  color: var(--hud-text-primary);
}

.credits-btn svg {
  opacity: 0.5;
}

/* 弹窗遮罩 */
.credits-backdrop {
  position: fixed;
  inset: 0;
  z-index: var(--z-modal);
  display: grid;
  place-items: center;
  padding: 20px;
  pointer-events: auto;
  background: rgba(2, 6, 14, 0.48);
  backdrop-filter: blur(8px);
}

/* 弹窗卡片 */
.credits-card {
  width: min(380px, calc(100vw - 32px));
  max-height: min(520px, calc(100vh - 64px));
  border-radius: 24px;
  border: 1px solid rgba(102, 136, 209, 0.3);
  background: linear-gradient(165deg, rgba(8, 13, 25, 0.97), rgba(10, 18, 35, 0.95));
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.4);
  padding: 20px;
  display: grid;
  gap: 14px;
  overflow-y: auto;
}

.credits-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.credits-head h3 {
  margin: 0;
  color: var(--hud-text-primary);
  font-size: 18px;
}

.credits-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: 8px;
}

.credits-person {
  display: grid;
  gap: 2px;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(73, 105, 170, 0.22);
  background: rgba(8, 13, 25, 0.72);
}

.credits-person strong {
  color: var(--hud-text-primary);
  font-size: 14px;
}

.credits-person span {
  color: var(--hud-text-secondary);
  font-size: 12px;
}

.credits-person small {
  color: rgba(188, 216, 255, 0.7);
  font-size: 11px;
}

.credits-footer {
  border-top: 1px solid rgba(78, 108, 174, 0.15);
  padding-top: 10px;
}

.credits-footer p {
  margin: 0;
  color: var(--hud-text-secondary);
  font-size: 11px;
  text-align: center;
}

.resolution-select {
  width: 100%;
  min-height: 42px;
  padding: 0 14px;
  border-radius: 14px;
  border: 1px solid rgba(73, 105, 170, 0.3);
  background: rgba(8, 13, 26, 0.78);
  color: var(--hud-text-primary);
  font-size: 13px;
  cursor: pointer;
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath d='M6 8L1 3h10z' fill='%23a0b8df'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 14px center;
  padding-right: 36px;
}

.resolution-select:focus {
  outline: none;
  border-color: rgba(119, 183, 255, 0.8);
  box-shadow: 0 0 0 1px rgba(119, 183, 255, 0.22);
}

@media (max-width: 980px) {
  .center-panel {
    top: 56px;
    right: 12px;
    left: 12px;
    width: auto;
  }

  .source-grid,
  .identity-grid,
  .diag-grid {
    grid-template-columns: 1fr;
  }

  .credits-card {
    width: auto;
    left: 12px;
    right: 12px;
  }
}
</style>
