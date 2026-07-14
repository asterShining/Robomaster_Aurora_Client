<template>
  <section class="buff-strip hud-realtime" data-testid="hud-buff-strip">
    <article v-if="buffs.length === 0" class="buff-empty">无增益</article>
    <article v-for="buff in buffs" :key="`${buff.robotId}:${buff.buffType}`" class="buff-tag">
      <span>{{ buffLabel(buff) }}</span>
      <b>{{ buffLeftLabel(buff) }}</b>
    </article>
  </section>
</template>

<script setup lang="ts">
import type { BuffState } from "../../../types";

interface Props {
  buffs: BuffState[];
}

withDefaults(defineProps<Props>(), {
  buffs: () => [],
});

const BUFF_LABELS: Record<number, string> = {
  1: "攻击↑",
  2: "防御↑",
  3: "移速↑",
  4: "能量↑",
  5: "急救",
  6: "加热",
  7: "冷却↓",
  8: "护盾",
};

function buffLabel(buff: BuffState): string {
  return BUFF_LABELS[buff.buffType] ?? `Buff#${buff.buffType}`;
}

function buffLeftLabel(buff: BuffState): string {
  if (buff.buffLeftTime <= 0) return "";
  if (buff.buffLeftTime >= 60) return `${Math.round(buff.buffLeftTime / 60)}m`;
  return `${buff.buffLeftTime}s`;
}
</script>

<style scoped>
.buff-strip {
  width: 100%;
  height: 100%;
  padding: 4px 8px;
  display: flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
}

.buff-empty {
  font-size: 10px;
  color: var(--hud-text-secondary);
}

.buff-tag {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border-radius: 999px;
  border: 1px solid rgba(186, 163, 106, 0.34);
  background: rgba(39, 29, 11, 0.48);
  padding: 3px 8px;
  color: #ffd793;
  font-size: 10px;
  white-space: nowrap;
}

.buff-tag b {
  color: #ffeed0;
  font-family: var(--font-data);
}
</style>
