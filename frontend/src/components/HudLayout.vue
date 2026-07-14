<template>
  <section class="hud-layout">
    <div ref="canvasRef" class="hud-canvas" data-testid="hud-canvas">
      <DraggableHudItem
        v-for="item in renderWidgets"
        :key="item.id"
        :id="item.id"
        :label="item.label"
        :layout="widgets[item.id]"
        :edit-mode="isEditMode"
        :canvas-el="canvasRef"
        @update="handleWidgetUpdate"
        @scale="handleWidgetScale"
      >
        <component :is="item.component" v-bind="item.props" />
      </DraggableHudItem>

      <div class="hud-toolbar hud-realtime">
        <button
          class="toolbar-btn"
          :class="{ 'is-active': isEditMode }"
          type="button"
          @click.stop="toggleLayoutMode"
        >
          {{ isEditMode ? "锁定布局" : "调整布局" }}
        </button>

        <button
          class="toolbar-btn"
          type="button"
          @click.stop="toggleSettingsPanel"
        >
          设置
        </button>
      </div>

      <CenterSettingsPanel />

      <HudCaptureToggle />

      <!-- What: Esc 触发后弹出统一退出确认层。
           Why: HUD 是全屏透明叠加窗，误按 Esc 直接退出风险太高，必须先给操作手一次明确确认。 -->
      <section
        v-if="showExitConfirm"
        class="exit-confirm-backdrop hud-realtime"
        role="dialog"
        aria-modal="true"
        aria-label="退出应用确认"
        @click="cancelExitConfirm"
      >
        <article class="exit-confirm-card" @click.stop>
          <header class="exit-confirm-head">
            <h3>退出应用？</h3>
            <p>当前将关闭 HUD 窗口，并停止视频与控制链路。</p>
          </header>

          <p v-if="exitConfirmMessage" class="exit-confirm-message">
            {{ exitConfirmMessage }}
          </p>

          <footer class="exit-confirm-actions">
            <button
              class="exit-confirm-btn exit-confirm-btn--ghost"
              type="button"
              :disabled="exitConfirmSubmitting"
              @click="cancelExitConfirm"
            >
              取消
            </button>

            <button
              class="exit-confirm-btn exit-confirm-btn--danger"
              type="button"
              :disabled="exitConfirmSubmitting"
              @click="confirmExitApp"
            >
              {{ exitConfirmSubmitting ? "正在退出..." : "确认退出" }}
            </button>
          </footer>
        </article>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import type { Component } from "vue";
import { storeToRefs } from "pinia";
import type { HudWidgetId, HudWidgetRect } from "../types/hudLayout";
import { HUD_WIDGET_LABELS } from "../types/hudLayout";
import { requestBackendQuit } from "../services/clientConfig";
import { useInputCapture } from "../composables/useInputCapture";
import DraggableHudItem from "./hud/core/DraggableHudItem.vue";
import HudTopRedBaseHpWidget from "./hud/widgets/HudTopRedBaseHpWidget.vue";
import HudTopBlueBaseHpWidget from "./hud/widgets/HudTopBlueBaseHpWidget.vue";
import HudTopRedTeamWidget from "./hud/widgets/HudTopRedTeamWidget.vue";
import HudTopBlueTeamWidget from "./hud/widgets/HudTopBlueTeamWidget.vue";
import HudTopScoreWidget from "./hud/widgets/HudTopScoreWidget.vue";
import HudModuleStatusWidget from "./hud/widgets/HudModuleStatusWidget.vue";
import HudRobotStatusWidget from "./hud/widgets/HudRobotStatusWidget.vue";
import HudFireControlWidget from "./hud/widgets/HudFireControlWidget.vue";
import HudCrosshairWidget from "./hud/widgets/HudCrosshairWidget.vue";
import HudGlobalUnitStatusWidget from "./hud/widgets/HudGlobalUnitStatusWidget.vue";
import HudGlobalLogisticsWidget from "./hud/widgets/HudGlobalLogisticsWidget.vue";
import HudSpecialMechanismWidget from "./hud/widgets/HudSpecialMechanismWidget.vue";
import HudRobotModuleStripWidget from "./hud/widgets/HudRobotModuleStripWidget.vue";
import HudBuffTagsWidget from "./hud/widgets/HudBuffTagsWidget.vue";
import HudDeployModeTagWidget from "./hud/widgets/HudDeployModeTagWidget.vue";
import HudRefereeToastLayer from "./hud/widgets/HudRefereeToastLayer.vue";
import HudCaptureToggle from "./hud/widgets/HudCaptureToggle.vue";
import CenterSettingsPanel from "./hud/CenterSettingsPanel.vue";
import { useRobotDataStore } from "../store/robotData";
import { useUiPanelStore } from "../store/uiPanel";
import { useHudLayoutStore } from "../store/hudLayout";

type CoreHudWidgetId =
  | "top_red_base_hp"
  | "top_red_team"
  | "top_score"
  | "top_blue_team"
  | "top_blue_base_hp"
  | "left_modules"
  | "left_robot"
  | "right_fire"
  | "center_crosshair"
  | "global_unit_status"
  | "global_logistics_status"
  | "global_special_mechanism"
  | "robot_module_strip"
  | "buff_tags"
  | "deploy_mode_tag"
  | "referee_toast";

interface RenderWidgetItem {
  id: CoreHudWidgetId;
  label: string;
  component: Component;
  props: Record<string, unknown>;
}

const CORE_HUD_WIDGET_IDS: readonly CoreHudWidgetId[] = [
  "top_red_base_hp",
  "top_red_team",
  "top_score",
  "top_blue_team",
  "top_blue_base_hp",
  "left_modules",
  "left_robot",
  "right_fire",
  "center_crosshair",
  "global_unit_status",
  "global_logistics_status",
  "global_special_mechanism",
  "robot_module_strip",
  "buff_tags",
  "deploy_mode_tag",
  "referee_toast",
];

const HUD_WIDGET_COMPONENTS: Record<CoreHudWidgetId, Component> = {
  top_red_base_hp: HudTopRedBaseHpWidget,
  top_red_team: HudTopRedTeamWidget,
  top_score: HudTopScoreWidget,
  top_blue_team: HudTopBlueTeamWidget,
  top_blue_base_hp: HudTopBlueBaseHpWidget,
  left_modules: HudModuleStatusWidget,
  left_robot: HudRobotStatusWidget,
  right_fire: HudFireControlWidget,
  center_crosshair: HudCrosshairWidget,
  global_unit_status: HudGlobalUnitStatusWidget,
  global_logistics_status: HudGlobalLogisticsWidget,
  global_special_mechanism: HudSpecialMechanismWidget,
  robot_module_strip: HudRobotModuleStripWidget,
  buff_tags: HudBuffTagsWidget,
  deploy_mode_tag: HudDeployModeTagWidget,
  referee_toast: HudRefereeToastLayer,
};

const CORE_HUD_WIDGET_SET = new Set<CoreHudWidgetId>(CORE_HUD_WIDGET_IDS);
const canvasRef = ref<HTMLElement | null>(null);
const robotStore = useRobotDataStore();
const uiPanelStore = useUiPanelStore();
const layoutStore = useHudLayoutStore();
const {
  combat,
  matchState,
  globalLogistics,
  globalSpecialMechanism,
  refereeEvents,
  robotModuleStatus,
  buffs,
  deployModeStatus,
} = storeToRefs(robotStore);
const { applied } = storeToRefs(uiPanelStore);
const { isEditMode, widgets, orderedWidgetIds } = storeToRefs(layoutStore);

const teamSide = computed(() => applied.value.hud.themeSide);
const crosshairColor = computed(() => applied.value.hud.crosshairColor);
const showDiagnosticsCard = computed(() => applied.value.showDiagnosticsCard);
const showRobotStatusCard = computed(() => applied.value.showRobotStatusCard);
const showFireControlCard = computed(() => applied.value.showFireControlCard);
const showExitConfirm = ref(false);
const exitConfirmSubmitting = ref(false);
const exitConfirmMessage = ref("");

// What: 只保留当前极简版 HUD 真正需要渲染的组件集合。
// Why: 继续沿用旧 layout store 做拖拽缩放，但不把已下线的雷达、输入、跑马灯重新带回主界面。
const renderWidgets = computed<RenderWidgetItem[]>(() => {
  return orderedWidgetIds.value.flatMap((widgetId) => {
    if (!CORE_HUD_WIDGET_SET.has(widgetId as CoreHudWidgetId)) return [];
    const coreWidgetId = widgetId as CoreHudWidgetId;
    const layout = widgets.value[coreWidgetId];
    if (!layout?.visible || !isWidgetEnabled(coreWidgetId)) return [];
    return [
      {
        id: coreWidgetId,
        label: HUD_WIDGET_LABELS[coreWidgetId],
        component: HUD_WIDGET_COMPONENTS[coreWidgetId],
        props: buildWidgetProps(coreWidgetId),
      },
    ];
  });
});

function shouldIgnoreHotkeyTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tagName = target.tagName;
  return (
    tagName === "INPUT" ||
    tagName === "TEXTAREA" ||
    tagName === "SELECT" ||
    target.isContentEditable
  );
}

// What: 仅让三张可开关状态卡受设置面板控制。
// Why: 顶栏和准星属于核心观测信息，精简版客户端不应允许被误关导致主界面缺关键信息。
function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

// What: 仅让三张可开关状态卡受设置面板控制。
// Why: 顶栏和准星属于核心观测信息，精简版客户端不应允许被误关导致主界面缺关键信息。
function isWidgetEnabled(widgetId: CoreHudWidgetId): boolean {
  if (widgetId === "left_modules") return showDiagnosticsCard.value;
  if (widgetId === "left_robot") return showRobotStatusCard.value;
  if (widgetId === "right_fire") return showFireControlCard.value;
  return true;
}

// What: 按组件 ID 组装实时渲染所需 props。
// Why: 把数据映射集中在布局层，避免模板里堆大量条件判断和重复绑定。
function buildWidgetProps(widgetId: CoreHudWidgetId): Record<string, unknown> {
  if (widgetId === "top_red_base_hp") {
    return {
      value: matchState.value.red.baseHp,
      max: matchState.value.red.baseMaxHp,
      ready: matchState.value.globalStatusReady,
    };
  }

  if (widgetId === "top_red_team") {
    return {
      units: matchState.value.red.units,
      ready: matchState.value.globalStatusReady,
    };
  }

  if (widgetId === "top_score") {
    return {
      gameStatusReady: matchState.value.gameStatusReady,
      currentRound: matchState.value.currentRound,
      totalRounds: matchState.value.totalRounds,
      redScore: matchState.value.redScore,
      blueScore: matchState.value.blueScore,
      currentStage: matchState.value.currentStage,
      stageCountdownSec: matchState.value.stageCountdownSec,
      stageElapsedSec: matchState.value.stageElapsedSec,
      isPaused: matchState.value.isPaused,
      countdownLevel: "normal",
    };
  }

  if (widgetId === "top_blue_team") {
    return {
      units: matchState.value.blue.units,
      ready: matchState.value.globalStatusReady,
    };
  }

  if (widgetId === "top_blue_base_hp") {
    return {
      value: matchState.value.blue.baseHp,
      max: matchState.value.blue.baseMaxHp,
      ready: matchState.value.globalStatusReady,
    };
  }

  if (widgetId === "left_modules") {
    return {
      teamSide: teamSide.value,
    };
  }

  if (widgetId === "left_robot") {
    return {
      hp: combat.value.hp,
      maxHp: combat.value.maxHp,
      teamSide: teamSide.value,
    };
  }

  if (widgetId === "right_fire") {
    return {
      bulletSpeed: combat.value.lastProjectileFireRate,
      ammoAllowance: combat.value.ammo,
      maxAmmo: combat.value.maxAmmo,
      heat: combat.value.heat,
      maxHeat: combat.value.maxHeat,
      capacitorPct: combat.value.bufferEnergy && combat.value.maxBufferEnergy
        ? clamp(combat.value.bufferEnergy / combat.value.maxBufferEnergy * 100, 0, 100)
        : combat.value.capacitorPct,
      chassisEnergy: combat.value.chassisEnergy,
      maxChassisEnergy: combat.value.maxChassisEnergy,
      experience: combat.value.currentExperience,
      experienceForUpgrade: combat.value.experienceForUpgrade,
      totalProjectilesFired: combat.value.totalProjectilesFired,
      isOutOfCombat: combat.value.isOutOfCombat,
      outOfCombatCountdownSec: combat.value.outOfCombatCountdownSec,
      hasRealData: combat.value.origin === "backend",
      ammoBand: "normal",
      capacitorLevel: "normal",
      showAmmoGraph: true,
      imminentOverheat: false,
    };
  }

  if (widgetId === "global_unit_status") {
    return {
      red: matchState.value.red,
      blue: matchState.value.blue,
      ready: matchState.value.globalStatusReady,
    };
  }

  if (widgetId === "global_logistics_status") {
    return { ...globalLogistics.value };
  }

  if (widgetId === "global_special_mechanism") {
    return {
      mechanisms: globalSpecialMechanism.value.mechanisms,
      updatedAt: globalSpecialMechanism.value.updatedAt,
    };
  }

  if (widgetId === "robot_module_strip") {
    return { ...robotModuleStatus.value };
  }

  if (widgetId === "buff_tags") {
    return {
      buffs: buffs.value,
    };
  }

  if (widgetId === "deploy_mode_tag") {
    return {
      status: deployModeStatus.value.status,
    };
  }

  if (widgetId === "referee_toast") {
    return {
      events: refereeEvents.value,
    };
  }

  return {
    heat: combat.value.heat,
    maxHeat: combat.value.maxHeat,
    color: crosshairColor.value,
    imminentOverheat: false,
  };
}

// What: 极简模式保留 P 打开设置、L 切布局、Esc 收口当前浮层。
// Why: 轻量客户端不再有复杂工具栏，但拖拽缩放仍然需要一组稳定快捷键入口。
function handleGlobalKeydown(event: KeyboardEvent): void {
  if (shouldIgnoreHotkeyTarget(event.target)) return;

  if (event.code === "KeyP") {
    event.preventDefault();
    toggleSettingsPanel();
    return;
  }

  if (event.code === "KeyL") {
    event.preventDefault();
    toggleLayoutMode();
    return;
  }

  if (event.key !== "Escape") return;
  event.preventDefault();

  // What: 退出确认层打开时，Esc 优先执行取消。
  // Why: 这样能让"再按一次 Esc 收回确认框"形成稳定心智，而不是把同一个按键继续向下穿透到别的状态机。
  if (showExitConfirm.value) {
    cancelExitConfirm();
    return;
  }

  if (uiPanelStore.isOpen) {
    uiPanelStore.forceClose();
    return;
  }

  if (layoutStore.isEditMode) {
    layoutStore.setEditMode(false);
    return;
  }

  // What: 只有当前没有别的高优先级浮层时，Esc 才转为"请求退出应用"。
  // Why: 这样可以保持现有"Esc 先收口当前编辑态/设置面板"的操作顺序，不会因为新增退出功能打断原有使用习惯。
  openExitConfirm();
}

function toggleSettingsPanel(): void {
  uiPanelStore.togglePanel();
}

function toggleLayoutMode(): void {
  layoutStore.toggleEditMode();
}

// What: 打开退出确认层并清空上一次错误文案。
// Why: 退出失败通常只发生在开发态或 runtime 未就绪阶段，重新打开确认层时必须恢复到干净状态，避免残留旧错误误导用户。
function openExitConfirm(): void {
  exitConfirmSubmitting.value = false;
  exitConfirmMessage.value = "";
  showExitConfirm.value = true;
}

// What: 关闭退出确认层并复位提交态。
// Why: 无论用户是点取消、点背景还是按 Esc，都应该回到统一的空闲状态，避免按钮残留 disabled 态。
function cancelExitConfirm(): void {
  exitConfirmSubmitting.value = false;
  exitConfirmMessage.value = "";
  showExitConfirm.value = false;
}

// What: 调用后端退出接口并在失败时回写错误提示。
// Why: 退出动作必须走 Wails runtime；若当前是纯前端调试环境或 runtime 尚未就绪，必须直接把失败原因告诉用户，而不是按钮点了没反应。
async function confirmExitApp(): Promise<void> {
  if (exitConfirmSubmitting.value) return;

  exitConfirmSubmitting.value = true;
  exitConfirmMessage.value = "";

  try {
    await requestBackendQuit();
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "退出应用失败";

    exitConfirmSubmitting.value = false;
    exitConfirmMessage.value = `退出失败：${message}`;
  }
}

// What: 将拖拽后的百分比矩形回写到 layout store。
// Why: 让鼠标拖动结果立刻持久化，避免热更新或重启后布局丢失。
function handleWidgetUpdate(payload: {
  id: HudWidgetId;
  patch: Partial<HudWidgetRect>;
}): void {
  layoutStore.updateWidgetRect(payload.id, payload.patch);
}

// What: 将滚轮缩放增量统一交给 layout store 处理。
// Why: store 已经封装了中心点缩放与边界钳制，布局层不应重复维护这套几何逻辑。
function handleWidgetScale(payload: { id: HudWidgetId; delta: number }): void {
  layoutStore.updateWidgetScale(payload.id, payload.delta);
}

useInputCapture(canvasRef);

onMounted(() => {
  // What: HUD 挂载时同时恢复设置和布局快照。
  // Why: 当前界面已经收敛成"视频 + 少量 HUD"，冷启动就必须立刻恢复用户上次的排布与配置。
  layoutStore.hydrate();
  void uiPanelStore.hydrate();
  window.addEventListener("keydown", handleGlobalKeydown);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleGlobalKeydown);
});
</script>

<style scoped>
.hud-layout {
  position: absolute;
  inset: 0;
  z-index: var(--z-hud);
  pointer-events: none;
}

.hud-canvas {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.hud-toolbar {
  position: absolute;
  right: calc(var(--pip-reserve-right) + 18px);
  top: 18px;
  z-index: var(--z-menu);
  display: flex;
  align-items: center;
  gap: 10px;
  pointer-events: auto;
}

.toolbar-btn {
  min-width: 84px;
  height: 36px;
  padding: 0 14px;
  border-radius: 999px;
  border: 1px solid rgba(110, 156, 243, 0.42);
  background: linear-gradient(
    140deg,
    rgba(10, 18, 35, 0.92),
    rgba(16, 26, 48, 0.9)
  );
  color: var(--hud-text-primary);
  font-size: 12px;
  letter-spacing: 0.08em;
  box-shadow: 0 10px 20px rgba(3, 7, 17, 0.34);
}

.toolbar-btn:hover {
  border-color: rgba(133, 185, 255, 0.68);
}

.toolbar-btn.is-active {
  border-color: rgba(126, 216, 255, 0.74);
  box-shadow:
    0 0 0 1px rgba(126, 216, 255, 0.18),
    0 10px 20px rgba(3, 7, 17, 0.34);
}

.exit-confirm-backdrop {
  position: absolute;
  inset: 0;
  z-index: var(--z-modal);
  display: grid;
  place-items: center;
  padding: 20px;
  pointer-events: auto;
  background: rgba(2, 6, 14, 0.42);
  backdrop-filter: blur(8px);
}

.exit-confirm-card {
  width: min(420px, calc(100vw - 32px));
  border-radius: 24px;
  border: 1px solid rgba(102, 136, 209, 0.3);
  background: linear-gradient(
    165deg,
    rgba(8, 13, 25, 0.96),
    rgba(10, 18, 35, 0.94)
  );
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.38);
  padding: 20px;
  display: grid;
  gap: 16px;
}

.exit-confirm-head {
  display: grid;
  gap: 8px;
}

.exit-confirm-head h3 {
  margin: 0;
  color: var(--hud-text-primary);
  font-size: 20px;
  letter-spacing: 0.06em;
}

.exit-confirm-head p {
  margin: 0;
  color: var(--hud-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.exit-confirm-message {
  margin: 0;
  min-height: 22px;
  color: rgba(255, 202, 202, 0.94);
  font-size: 12px;
  line-height: 1.6;
}

.exit-confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.exit-confirm-btn {
  min-width: 102px;
  height: 38px;
  padding: 0 16px;
  border-radius: 999px;
  font-size: 12px;
  letter-spacing: 0.08em;
}

.exit-confirm-btn:disabled {
  opacity: 0.6;
}

.exit-confirm-btn--ghost {
  border: 1px solid rgba(104, 141, 221, 0.34);
  background: rgba(11, 18, 34, 0.84);
  color: var(--hud-text-primary);
}

.exit-confirm-btn--danger {
  border: 1px solid rgba(223, 125, 137, 0.4);
  background: linear-gradient(
    145deg,
    rgba(61, 16, 27, 0.92),
    rgba(39, 11, 19, 0.92)
  );
  color: rgba(255, 232, 235, 0.98);
}

@media (max-width: 980px) {
  .hud-toolbar {
    right: 12px;
    top: 12px;
    gap: 8px;
  }

  .toolbar-btn {
    min-width: 76px;
    padding: 0 12px;
  }

  .exit-confirm-card {
    padding: 18px 16px;
  }

  .exit-confirm-actions {
    flex-direction: column-reverse;
  }

  .exit-confirm-btn {
    width: 100%;
  }
}
</style>
