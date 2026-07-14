<template>
  <article
    class="team-robot-card"
    :class="[
      `side-${side}`,
      { offline: ready && !unit.online, unavailable: !ready },
    ]"
  >
    <header class="title">
      <span>{{ unit.roleLabel }}</span>
      <small>{{ idLabel }}</small>
    </header>

    <HudAvatarPlaceholder :side="side" :avatar-key="unit.avatarKey" />

    <footer class="hp">
      <div class="track">
        <i :style="{ width: `${hpPercent}%` }"></i>
      </div>
      <small>{{ hpText }}</small>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { TeamRobotState } from "../../../types";
import HudAvatarPlaceholder from "./HudAvatarPlaceholder.vue";

interface Props {
  side: "red" | "blue";
  unit: TeamRobotState;
  ready?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  ready: false,
});

const ready = computed(() => props.ready);

const hpPercent = computed(() => {
  if (!ready.value) return 0;
  const max = Math.max(1, props.unit.maxHp);
  return Math.max(0, Math.min(100, (props.unit.hp / max) * 100));
});

const idLabel = computed(
  () => `${props.side === "red" ? "R" : "B"}${props.unit.robotId}`,
);
const hpText = computed(() =>
  ready.value ? `${props.unit.hp} / ${props.unit.maxHp}` : "未接入",
);
</script>

<style scoped>
.team-robot-card {
  min-width: 60px;
  border-radius: 10px;
  border: 1px solid rgba(108, 145, 210, 0.58);
  background: linear-gradient(
    160deg,
    rgba(10, 18, 35, 0.9),
    rgba(7, 11, 23, 0.82)
  );
  padding: 4px;
  display: grid;
  justify-items: center;
  gap: 3px;
}

.team-robot-card.side-red {
  border-color: rgba(174, 97, 105, 0.62);
  background: linear-gradient(
    160deg,
    rgba(31, 12, 16, 0.9),
    rgba(14, 7, 10, 0.82)
  );
}

.team-robot-card.offline {
  opacity: 0.52;
  filter: grayscale(0.55);
}

.team-robot-card.unavailable {
  opacity: 0.78;
  filter: saturate(0.6);
}

.title {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}

.title span {
  font-size: 9px;
  color: rgba(190, 217, 251, 0.94);
}

.side-red .title span {
  color: rgba(250, 205, 205, 0.94);
}

.title small {
  font-size: 8px;
  color: rgba(161, 192, 236, 0.9);
  font-family: "JetBrains Mono", "Consolas", monospace;
}

.side-red .title small {
  color: rgba(233, 181, 181, 0.9);
}

.hp {
  width: 100%;
  display: grid;
  gap: 2px;
}

.track {
  /* What: 血量轨道改成纯容器层。Why: 让满血填充可以真正铺满整条轨道，而不是被描边吃掉 1px 视觉宽度。 */
  position: relative;
  height: 4px;
  border-radius: 999px;
  background: rgba(7, 12, 21, 0.8);
  overflow: hidden;
}

.side-red .track {
  background: rgba(16, 8, 10, 0.82);
}

.track::after {
  /* What: 将描边挪到覆盖层绘制。Why: 保留轨道边框质感，同时保证 100% 血量时不会因为内缩边框看起来还少一截。 */
  content: "";
  position: absolute;
  inset: 0;
  border: 1px solid rgba(100, 140, 214, 0.5);
  border-radius: inherit;
  pointer-events: none;
}

.side-red .track::after {
  /* What: 红方描边颜色单独覆盖。Why: 继续保持红蓝双方语义色一致，避免共享默认蓝色描边。 */
  border-color: rgba(170, 95, 104, 0.55);
}

.track i {
  /* What: 填充条改为绝对定位贴边铺开。Why: 满血时必须贴满两端，只有掉血后才露出右侧空槽。 */
  position: absolute;
  inset: 0 auto 0 0;
  background: linear-gradient(90deg, #7bc4ff, #caf3ff);
  border-radius: inherit;
  transition: width 140ms linear;
}

.side-red .track i {
  background: linear-gradient(90deg, #ff8a81, #ffd27a);
}

.hp small {
  font-size: 7px;
  line-height: 1;
  color: rgba(173, 200, 240, 0.92);
  text-align: center;
  font-family: "JetBrains Mono", "Consolas", monospace;
}

.side-red .hp small {
  color: rgba(240, 192, 192, 0.92);
}
</style>
