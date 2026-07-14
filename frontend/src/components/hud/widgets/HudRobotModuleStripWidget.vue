<template>
  <section class="module-strip hud-realtime" data-testid="hud-robot-module-strip">
    <article v-for="item in modules" :key="item.key" class="module-chip" :class="`state-${item.ok ? 'ok' : 'bad'}`">
      <i></i>
      <span>{{ item.label }}</span>
    </article>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { RobotModuleStatusState } from "../../../types";

const props = defineProps<RobotModuleStatusState>();

function isOk(value: number): boolean {
  return value > 0;
}

const modules = computed(() => [
  { key: "main", label: "主控", ok: isOk(props.mainController) },
  { key: "power", label: "电源", ok: isOk(props.powerManager) },
  { key: "armor", label: "装甲", ok: isOk(props.armor) },
  { key: "small", label: "小弹丸", ok: isOk(props.smallShooter) },
  { key: "big", label: "大弹丸", ok: isOk(props.bigShooter) },
  { key: "video", label: "图传", ok: isOk(props.videoTransmission) },
  { key: "rfid", label: "RFID", ok: isOk(props.rfid) },
  { key: "uwb", label: "UWB", ok: isOk(props.uwb) },
  { key: "cap", label: "电容", ok: isOk(props.capacitor) },
  { key: "laser", label: "激光", ok: isOk(props.laserDetectionModule) },
]);
</script>

<style scoped>
.module-strip {
  width: 100%;
  height: 100%;
  padding: 6px 8px;
  border-radius: 999px;
  border: 1px solid rgba(109, 147, 206, 0.24);
  background: rgba(5, 10, 20, 0.42);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  gap: 7px;
  overflow: hidden;
}

.module-chip {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--hud-text-secondary);
  font-size: 10px;
  white-space: nowrap;
}

.module-chip i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #ff6b7a;
  box-shadow: 0 0 8px rgba(255, 107, 122, 0.58);
}

.module-chip.state-ok i {
  background: #75f3b0;
  box-shadow: 0 0 8px rgba(117, 243, 176, 0.58);
}
</style>
