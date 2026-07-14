<template>
  <!-- What: 火控信息组件。Why: 将弹速/发弹量/热量集中管理，减少右侧区域重复样式。 -->
  <HudWidgetCard title="射击数据" accent="cyan">
    <div v-if="props.hasRealData" class="fire-grid">
      <div class="fire-row">
        <span class="row-label">
          <HudIconSpeed class="icon" />
          射速
        </span>
        <b class="value">{{ props.bulletSpeed.toFixed(1) }}</b>
      </div>

      <div class="fire-row">
        <span class="row-label">
          <HudIconAmmo class="icon" />
          剩余弹量
        </span>
        <b class="value ammo" :class="`ammo-${props.ammoBand}`">{{
          props.ammoAllowance
        }}</b>
      </div>

      <div
        v-if="props.showAmmoGraph"
        class="ammo-graph"
        data-testid="hud-ammo-graph"
      >
        <i
          v-for="segment in ammoSegments"
          :key="segment.id"
          class="ammo-segment"
          :class="[`state-${props.ammoBand}`, { 'is-active': segment.active }]"
          data-testid="ammo-segment"
        ></i>
      </div>

      <div class="fire-row">
        <span class="row-label">累计发弹</span>
        <b class="value">{{ props.totalProjectilesFired }}</b>
      </div>

      <div class="fire-row">
        <span class="row-label">底盘能量</span>
        <b class="value">{{ props.chassisEnergy }}<small> / {{ props.maxChassisEnergy }}</small></b>
      </div>

      <div class="fire-row">
        <span class="row-label">经验</span>
        <b class="value">{{ props.experience }}<small> / {{ props.experienceForUpgrade }}</small></b>
      </div>

      <div v-if="props.isOutOfCombat" class="fire-row fire-row--alert">
        <span class="row-label">脱战</span>
        <b class="value">{{ props.outOfCombatCountdownSec }}s</b>
      </div>

      <div
        class="capacitor-row"
        :class="`level-${props.capacitorLevel}`"
        data-testid="hud-capacitor-row"
      >
        <div class="resource-head">
          <span>缓冲能量</span>
          <small class="resource-state">{{ capacitorStateLabel }}</small>
        </div>
        <HudProgressBar
          :value="props.capacitorPct"
          :max="100"
          :color="capacitorColor"
          :decimals="0"
        />
      </div>

      <div class="heat-row">
        <div class="resource-head">
          <span>热量</span>
          <small v-if="props.imminentOverheat" class="heat-alert"
            >即将过热</small
          >
        </div>
        <HudProgressBar
          :value="props.heat"
          :max="props.maxHeat"
          :color="heatColor"
          :decimals="0"
        />
      </div>
    </div>

    <!-- What: 无真实 combat-state 时直接进入未接入态。Why: 用户明确要求火控区不能再拿默认值伪装成真实数据。 -->
    <section v-else class="fire-offline" data-testid="hud-fire-offline">
      <strong>未接入</strong>
      <small>等待真实 combat-state</small>
    </section>
  </HudWidgetCard>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { AwarenessLevel } from "../../../composables/useSituationalAwareness";
import HudWidgetCard from "../primitives/HudWidgetCard.vue";
import HudProgressBar from "../primitives/HudProgressBar.vue";
import HudIconSpeed from "../icons/HudIconSpeed.vue";
import HudIconAmmo from "../icons/HudIconAmmo.vue";

interface Props {
  bulletSpeed: number;
  ammoAllowance: number;
  maxAmmo: number;
  heat: number;
  maxHeat: number;
  capacitorPct: number;
  chassisEnergy: number;
  maxChassisEnergy: number;
  experience: number;
  experienceForUpgrade: number;
  totalProjectilesFired: number;
  isOutOfCombat?: boolean;
  outOfCombatCountdownSec?: number;
  hasRealData?: boolean;
  ammoBand?: "normal" | "warn" | "critical";
  capacitorLevel?: AwarenessLevel;
  showAmmoGraph?: boolean;
  imminentOverheat?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  chassisEnergy: 0,
  maxChassisEnergy: 0,
  experience: 0,
  experienceForUpgrade: 0,
  totalProjectilesFired: 0,
  isOutOfCombat: false,
  outOfCombatCountdownSec: 0,
  hasRealData: false,
  ammoBand: "normal",
  capacitorLevel: "normal",
  showAmmoGraph: true,
  imminentOverheat: false,
});

const ammoSegments = computed(() => {
  const segmentCount = 12;
  const safeMax = Math.max(1, props.maxAmmo);
  const activeSegments = Math.ceil(
    (Math.max(0, props.ammoAllowance) / safeMax) * segmentCount,
  );

  // What: 把剩余弹量离散成固定段数显示。
  // Why: 赛场上扫一眼分段图比读小数字更快，更符合“剩余资源”类信息的感知习惯。
  return Array.from({ length: segmentCount }, (_, index) => ({
    id: `segment-${index}`,
    active: index < activeSegments,
  }));
});

// What: 将电容等级映射为紧凑文本标签。
// Why: 右侧火控卡空间有限，短标签比长句子更适合赛时扫视。
const capacitorStateLabel = computed(() => {
  if (props.capacitorLevel === "critical") return "危险";
  if (props.capacitorLevel === "warn") return "偏低";
  return "稳定";
});

// What: 统一电容条的语义色。
// Why: 让“稳定 / 偏低 / 危险”与增强器其它组件保持同一套视觉语言。
const capacitorColor = computed<"green" | "warning" | "danger">(() => {
  if (props.capacitorLevel === "critical") return "danger";
  if (props.capacitorLevel === "warn") return "warning";
  return "green";
});

// What: 即将过热时将热量条切换到预警色。
// Why: 中心提示之外还要让火控卡本体同步进入危险语义，避免操作手视线切回右侧后丢上下文。
const heatColor = computed<"blue" | "warning">(() =>
  props.imminentOverheat ? "warning" : "blue",
);
</script>

<style scoped>
.fire-grid {
  height: 100%;
  overflow-y: auto;
  display: grid;
  grid-auto-rows: auto;
  gap: 4px;
  align-content: start;
}

.fire-offline {
  height: 100%;
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 6px;
  text-align: center;
}

.fire-offline strong {
  color: var(--hud-text-primary);
  font-size: 15px;
  letter-spacing: 0.08em;
}

.fire-offline small {
  color: var(--hud-text-secondary);
  font-size: 10px;
  letter-spacing: 0.08em;
}

.fire-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 6px;
  min-width: 0;
}

.row-label {
  font-size: 9px;
  color: rgba(178, 206, 247, 0.9);
  display: inline-flex;
  align-items: center;
  gap: 3px;
  white-space: nowrap;
}

.row-label :deep(.icon) {
  color: rgba(105, 223, 255, 0.88);
  width: 11px;
  height: 11px;
}

.value {
  font-size: 13px;
  line-height: 1;
  color: rgba(227, 243, 255, 0.96);
  font-family: var(--font-data);
}

.value small {
  font-size: 10px;
  color: var(--hud-text-secondary);
  font-weight: 400;
}

.value.ammo {
  color: var(--hud-text-primary);
}

.value.ammo.ammo-warn {
  color: var(--hud-warning);
}

.value.ammo.ammo-critical {
  color: var(--hud-score-red);
}

.fire-row--alert {
  border-radius: 999px;
  border: 1px solid rgba(255, 158, 114, 0.28);
  background: rgba(44, 17, 9, 0.38);
  padding: 2px 8px;
}

.fire-row--alert .row-label {
  color: #ffb294;
}

.ammo-graph {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 3px;
}

.ammo-segment {
  height: 5px;
  border-radius: 999px;
  background: var(--hud-ammo-segment-bg);
  box-shadow: inset 0 0 0 1px rgba(20, 31, 55, 0.32);
}

.ammo-segment.is-active {
  background: var(--hud-ammo-segment-fill);
  box-shadow: var(--hud-ammo-segment-shadow);
}

.ammo-segment.state-warn.is-active {
  background: var(--hud-ammo-segment-warn);
}

.ammo-segment.state-critical.is-active {
  background: var(--hud-ammo-segment-critical);
}

.heat-row,
.capacitor-row {
  display: grid;
  gap: 3px;
}

.resource-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

.heat-row span,
.capacitor-row span {
  font-size: 9px;
  color: var(--hud-text-secondary);
}

.resource-state {
  font-size: 9px;
  line-height: 1;
  color: var(--hud-text-secondary);
  font-family: var(--font-data);
  letter-spacing: 0.04em;
}

.capacitor-row.level-warn .resource-state {
  color: var(--hud-warning);
}

.capacitor-row.level-critical .resource-state {
  color: var(--hud-score-red);
  animation: warning-pulse 760ms ease-in-out infinite;
}

.heat-alert {
  font-size: 9px;
  line-height: 1;
  color: var(--hud-warning);
  font-family: var(--font-data);
  letter-spacing: 0.04em;
}
</style>
