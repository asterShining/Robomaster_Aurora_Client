<template>
  <section class="team-shell side-red hud-realtime">
    <HudTeamRobotCard
      v-for="unit in units"
      :key="`red-${unit.robotId}`"
      side="red"
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
  border: 1px solid rgba(170, 95, 103, 0.52);
  background: linear-gradient(
    160deg,
    rgba(31, 12, 16, 0.82),
    rgba(14, 7, 10, 0.72)
  );
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 4px;
  padding: 4px;
}
</style>
