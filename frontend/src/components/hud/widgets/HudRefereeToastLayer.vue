<template>
  <section v-if="visible.length > 0" class="referee-toast-stack hud-realtime" data-testid="hud-referee-toast-stack">
    <article
      v-for="entry in visible"
      :key="entry.id"
      class="referee-toast"
      :class="{ 'referee-toast--fresh': isFresh(entry) }"
    >
      <i class="toast-dot"></i>
      <span class="toast-body">{{ formatEvent(entry) }}</span>
    </article>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { RefereeEventEntry } from "../../../types";

interface Props {
  events: RefereeEventEntry[];
}

const props = defineProps<Props>();

const EVENTS_BY_ID: Record<number, string> = {
  1: "装甲命中",
  2: "模块掉线",
  3: "机器人离线",
};

const MAX_VISIBLE = 4;
const TOAST_TTL_MS = 10000;

const visible = computed(() => props.events.slice(0, MAX_VISIBLE));

function isFresh(entry: RefereeEventEntry): boolean {
  return Date.now() - entry.updatedAt <= TOAST_TTL_MS;
}

function formatEvent(entry: RefereeEventEntry): string {
  const label = EVENTS_BY_ID[entry.eventId];
  if (label && entry.param) return `${label} · ${entry.param}`;
  if (label) return label;
  if (entry.eventId > 0) return `裁判事件 #${entry.eventId} · ${entry.param}`;
  return entry.param;
}
</script>

<style scoped>
.referee-toast-stack {
  width: 100%;
  height: 100%;
  display: grid;
  grid-auto-rows: minmax(0, auto);
  align-content: end;
  gap: 6px;
  padding: 8px 10px;
}

.referee-toast {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  border-radius: 999px;
  border: 1px solid rgba(129, 159, 217, 0.22);
  background: rgba(8, 14, 27, 0.56);
  backdrop-filter: blur(6px);
  padding: 6px 11px;
  animation: toast-enter 260ms ease-out;
  transition: opacity 400ms;
}

.referee-toast--fresh {
  border-color: rgba(255, 201, 126, 0.28);
}

.toast-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: rgba(255, 201, 126, 0.88);
}

.toast-body {
  color: var(--hud-text-secondary);
  font-size: 10px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

@keyframes toast-enter {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
