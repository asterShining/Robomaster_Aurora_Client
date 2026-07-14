<template>
  <span class="module-tag" :class="[`state-${state}`]">
    <em class="dot"></em>
    <i>/</i>
    <span>{{ label }}</span>
  </span>
</template>

<script setup lang="ts">
interface Props {
  label: string
  state?: 'online' | 'warn' | 'offline'
}

const props = withDefaults(defineProps<Props>(), {
  state: 'online',
})

// What: 模块标签统一抽象。Why: 机器人状态卡和模块区可复用同一视觉语义，降低样式分叉。
void props
</script>

<style scoped>
.module-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  white-space: nowrap;
}

.module-tag i {
  font-style: normal;
  font-weight: 800;
  color: var(--hud-module-online-slash);
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--hud-module-online-dot);
  box-shadow: 0 0 8px rgba(255, 94, 94, 0.5);
}

.module-tag span {
  color: var(--hud-module-online);
}

.module-tag.state-warn span {
  color: var(--hud-module-warn);
}

.module-tag.state-warn .dot {
  background: var(--hud-module-warn-dot);
  box-shadow: 0 0 8px rgba(255, 206, 89, 0.44);
}

.module-tag.state-offline span {
  color: var(--hud-module-offline);
}

.module-tag.state-offline .dot {
  background: var(--hud-module-offline-dot);
  box-shadow: none;
}
</style>
