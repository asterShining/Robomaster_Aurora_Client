<template>
  <!-- What: 准星图标原子组件。Why: 中心准心可单独迭代，不影响热量环逻辑。 -->
  <svg class="hud-icon" viewBox="0 0 24 24" aria-hidden="true">
    <circle cx="12" cy="12" r="6" fill="none" stroke="currentColor" stroke-width="1.4" />
    <path d="M12 2v4M12 18v4M2 12h4M18 12h4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
  </svg>
</template>

<style scoped>
.hud-icon {
  width: 16px;
  height: 16px;
  display: block;
}
</style>
