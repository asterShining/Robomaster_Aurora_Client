<template>
  <!-- What: 初速度图标原子组件。Why: 火控面板与其它模块可复用同一视觉语义。 -->
  <svg class="hud-icon" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M4 16a8 8 0 1 1 16 0" fill="none" stroke="currentColor" stroke-width="1.8" />
    <path d="M12 12l5-3" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
    <circle cx="12" cy="12" r="1.8" fill="currentColor" />
  </svg>
</template>

<style scoped>
.hud-icon {
  width: 16px;
  height: 16px;
  display: block;
}
</style>
