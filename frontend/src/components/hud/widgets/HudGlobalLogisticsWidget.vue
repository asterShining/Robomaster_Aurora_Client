<template>
  <HudWidgetCard title="经济科技" :subtitle="subtitle" accent="cyan">
    <div class="logistics-grid hud-realtime">
      <article v-for="item in items" :key="item.name" class="metric">
        <span>{{ item.name }}</span>
        <b>{{ item.value }}</b>
      </article>
    </div>
  </HudWidgetCard>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { GlobalLogisticsState } from "../../../types";
import HudWidgetCard from "../primitives/HudWidgetCard.vue";

const props = defineProps<GlobalLogisticsState>();

const subtitle = computed(() => (props.updatedAt > 0 ? "GlobalLogisticsStatus" : "未接入"));
const items = computed(() => [
  { name: "剩余经济", value: props.remainingEconomy },
  { name: "累计经济", value: props.totalEconomyObtained },
  { name: "科技等级", value: props.techLevel },
  { name: "加密等级", value: props.encryptionLevel },
]);
</script>

<style scoped>
.logistics-grid {
  height: 100%;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 7px;
}

.metric {
  min-width: 0;
  min-height: 0;
  padding: 8px;
  border-radius: 14px;
  border: 1px solid rgba(105, 169, 218, 0.28);
  background: rgba(8, 18, 32, 0.42);
  display: grid;
  align-content: center;
  gap: 4px;
}

.metric span {
  color: var(--hud-text-secondary);
  font-size: 10px;
}

.metric b {
  color: var(--hud-text-primary);
  font-family: var(--font-data);
  font-size: 17px;
}
</style>
