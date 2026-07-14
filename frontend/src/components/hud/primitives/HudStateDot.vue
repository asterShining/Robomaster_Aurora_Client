<template>
  <span class="state-dot" :class="stateClass"></span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  online: boolean
}

const props = defineProps<Props>()

// What: 只暴露在线/离线两态。Why: 避免状态组件承担业务判断，保持原子性与复用性。
const stateClass = computed(() => (props.online ? 'is-online' : 'is-offline'))
</script>

<style scoped>
.state-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 999px;
  box-shadow: 0 0 0 1px rgba(12, 20, 36, 0.9), 0 0 0 2px rgba(90, 124, 186, 0.25);
}

.state-dot.is-online {
  background: #61f4ff;
  box-shadow: 0 0 0 1px rgba(9, 18, 35, 0.88), 0 0 8px rgba(97, 244, 255, 0.75);
}

.state-dot.is-offline {
  background: #ff8e8e;
  box-shadow: 0 0 0 1px rgba(22, 10, 14, 0.88), 0 0 8px rgba(255, 97, 97, 0.54);
}
</style>
