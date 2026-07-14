<template>
  <section v-if="inDeploy" class="deploy-tag hud-realtime" data-testid="hud-deploy-mode-tag">
    <span>部署模式</span>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";

interface Props {
  status: number;
}

const props = defineProps<Props>();

const DEPLOY_STATUS_ACTIVE = 1;

const inDeploy = computed(() => props.status === DEPLOY_STATUS_ACTIVE);
</script>

<style scoped>
.deploy-tag {
  width: 100%;
  height: 100%;
  border-radius: 16px;
  border: 1px solid rgba(190, 148, 64, 0.28);
  background: rgba(29, 21, 8, 0.48);
  backdrop-filter: blur(6px);
  display: grid;
  place-items: center;
}

.deploy-tag span {
  color: #ffe9b4;
  font-size: 14px;
  letter-spacing: 0.08em;
}
</style>
