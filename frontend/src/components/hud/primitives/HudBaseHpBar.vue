<template>
  <section class="base-hp-bar" :class="`side-${side}`">
    <div class="head">
      <span>{{ label }}</span>
      <small>{{ valueText }}</small>
    </div>
    <div class="track">
      <i :style="{ width: `${percent}%` }"></i>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";

interface Props {
  side: "red" | "blue";
  label: string;
  value: number;
  max: number;
  ready?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  ready: false,
});

const percent = computed(() => {
  if (!props.ready) return 0;
  const safeMax = Math.max(1, props.max);
  return Math.max(0, Math.min(100, Math.round((props.value / safeMax) * 100)));
});

// What: 基地条在未接入全局血量时明确显示未接入。
// Why: 默认 0 / 5000 会被误解成真实残血，必须和“真实打空”状态区分开。
const valueText = computed(() =>
  props.ready ? `${props.value} / ${props.max}` : "未接入",
);
</script>

<style scoped>
.base-hp-bar {
  width: 100%;
  height: 100%;
  border-radius: 14px;
  border: 1px solid rgba(102, 141, 214, 0.56);
  background: linear-gradient(
    165deg,
    rgba(10, 17, 33, 0.9),
    rgba(7, 12, 22, 0.78)
  );
  display: grid;
  align-content: center;
  gap: 5px;
  padding: 7px 10px;
}

.base-hp-bar.side-red {
  border-color: rgba(167, 93, 100, 0.64);
  background: linear-gradient(
    165deg,
    rgba(30, 12, 16, 0.9),
    rgba(12, 7, 9, 0.78)
  );
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.head span {
  font-size: 13px;
  color: rgba(191, 216, 252, 0.92);
}

.side-red .head span {
  color: rgba(247, 209, 209, 0.92);
}

.head small {
  font-size: 11px;
  color: rgba(174, 202, 243, 0.92);
  font-family: "JetBrains Mono", "Consolas", monospace;
}

.side-red .head small {
  color: rgba(245, 194, 194, 0.9);
}

.track {
  /* What: 基地血条轨道改成纯背景层。Why: 满血时应直接铺满到边缘，不能再因为边框内缩看起来像没满。 */
  position: relative;
  height: 9px;
  border-radius: 999px;
  background: rgba(7, 12, 22, 0.8);
  overflow: hidden;
}

.side-red .track {
  background: rgba(16, 8, 10, 0.82);
}

.track::after {
  /* What: 通过伪元素承担描边。Why: 这样填充层可完整占满轨道，扣血时才会从右侧自然露出缺口。 */
  content: "";
  position: absolute;
  inset: 0;
  border: 1px solid rgba(110, 151, 226, 0.52);
  border-radius: inherit;
  pointer-events: none;
}

.side-red .track::after {
  /* What: 红方基地条沿用独立描边色。Why: 保持队伍语义色，不让红方基地条退回默认蓝系外观。 */
  border-color: rgba(169, 92, 101, 0.56);
}

.track i {
  /* What: 填充层贴着轨道左边缘展开。Why: 满血时完全填满，非满血时才留下真实的空白区。 */
  position: absolute;
  inset: 0 auto 0 0;
  background: linear-gradient(90deg, #83c7ff, #ccf5ff);
  border-radius: inherit;
  transition: width 140ms linear;
}

.side-red .track i {
  background: linear-gradient(90deg, #ff8d83, #ffd586);
}
</style>
