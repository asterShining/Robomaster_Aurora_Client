<template>
  <HudBaseHpBar
    side="blue"
    label="蓝方基地"
    :value="value"
    :max="max"
    :ready="ready"
    class="hud-realtime"
  />
</template>

<script setup lang="ts">
import HudBaseHpBar from "../primitives/HudBaseHpBar.vue";

interface Props {
  value: number;
  max: number;
  ready?: boolean;
}

const props = defineProps<Props>();

// What: 显式消费 props。
// Why: 便于后续接入后端真实字段时保持接口稳定。
void props;
</script>
