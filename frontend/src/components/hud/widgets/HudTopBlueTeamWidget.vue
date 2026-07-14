<template>
  <section class="team-shell side-blue hud-realtime">
    <HudTeamRobotCard
      v-for="unit in units"
      :key="`blue-${unit.robotId}`"
      side="blue"
      :unit="unit"
      :ready="ready"
    />
  </section>
</template>

<script setup lang="ts">
import type { TeamRobotState } from "../../../types";
import HudTeamRobotCard from "../primitives/HudTeamRobotCard.vue";

interface Props {
  units: TeamRobotState[];
  ready?: boolean;
}

const props = defineProps<Props>();

// What: 显式消费 props。
// Why: 保持编组组件输入明确，便于后端字段对齐与测试。
void props;
</script>

<style scoped>
.team-shell {
  width: 100%;
  height: 100%;
  border-radius: 10px;
  border: 1px solid rgba(95, 136, 205, 0.52);
  background: linear-gradient(
    160deg,
    rgba(10, 18, 37, 0.84),
    rgba(7, 12, 24, 0.74)
  );
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 4px;
  padding: 4px;
}
</style>
