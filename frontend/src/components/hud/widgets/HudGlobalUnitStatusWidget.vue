<template>
  <HudWidgetCard title="全局单位" :subtitle="readyLabel" accent="blue">
    <div class="unit-grid hud-realtime">
      <article class="base-card side-red">
        <span>红方基地</span>
        <b>{{ red.baseHp }} / {{ red.baseMaxHp }}</b>
        <small>护盾 {{ red.baseShield }} · 前哨 {{ red.outpostHp }} · 伤害 {{ red.totalDamage }}</small>
      </article>
      <article class="base-card side-blue">
        <span>蓝方基地</span>
        <b>{{ blue.baseHp }} / {{ blue.baseMaxHp }}</b>
        <small>护盾 {{ blue.baseShield }} · 前哨 {{ blue.outpostHp }} · 伤害 {{ blue.totalDamage }}</small>
      </article>
    </div>
  </HudWidgetCard>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { TeamSideState } from "../../../types";
import HudWidgetCard from "../primitives/HudWidgetCard.vue";

interface Props {
  red: TeamSideState;
  blue: TeamSideState;
  ready?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  ready: false,
});

const readyLabel = computed(() => (props.ready ? "GlobalUnitStatus" : "未接入"));
</script>

<style scoped>
.unit-grid {
  height: 100%;
  display: grid;
  grid-template-rows: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.base-card {
  min-width: 0;
  min-height: 0;
  padding: 8px 10px;
  border-radius: 16px;
  border: 1px solid rgba(115, 141, 207, 0.3);
  background: rgba(8, 14, 27, 0.46);
  display: grid;
  align-content: center;
  gap: 4px;
}

.base-card span,
.base-card small {
  color: var(--hud-text-secondary);
  font-size: 10px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.base-card b {
  color: var(--hud-text-primary);
  font-family: var(--font-data);
  font-size: 18px;
  line-height: 1;
}

.side-red { border-color: var(--hud-panel-red-border); }
.side-blue { border-color: var(--hud-panel-blue-border); }
</style>
