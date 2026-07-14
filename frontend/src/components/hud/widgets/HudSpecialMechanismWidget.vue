<template>
  <HudWidgetCard title="特殊机制" :subtitle="subtitle" accent="cyan">
    <div class="mechanism-list hud-realtime">
      <article v-if="mechanisms.length === 0" class="empty">等待 GlobalSpecialMechanism</article>
      <article v-for="item in mechanisms" :key="item.mechanismId" class="mechanism">
        <span>#{{ item.mechanismId }}</span>
        <b>{{ item.timeSec }}s</b>
      </article>
    </div>
  </HudWidgetCard>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { SpecialMechanismItem } from "../../../types";
import HudWidgetCard from "../primitives/HudWidgetCard.vue";

interface Props {
  mechanisms: SpecialMechanismItem[];
  updatedAt: number;
}

const props = withDefaults(defineProps<Props>(), {
  mechanisms: () => [],
  updatedAt: 0,
});

const subtitle = computed(() => (props.updatedAt > 0 ? "GlobalSpecialMechanism" : "未接入"));
</script>

<style scoped>
.mechanism-list {
  height: 100%;
  display: grid;
  grid-auto-rows: minmax(24px, auto);
  align-content: start;
  gap: 6px;
  overflow: hidden;
}

.mechanism,
.empty {
  min-width: 0;
  border-radius: 999px;
  border: 1px solid rgba(114, 197, 233, 0.28);
  background: rgba(8, 18, 32, 0.44);
  padding: 5px 9px;
  color: var(--hud-text-secondary);
  font-size: 10px;
}

.mechanism {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.mechanism b {
  color: var(--hud-text-primary);
  font-family: var(--font-data);
}
</style>
