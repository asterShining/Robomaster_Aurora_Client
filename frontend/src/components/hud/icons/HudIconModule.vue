<template>
  <!-- What: 模块状态图标原子组件。Why: 避免各面板重复绘制形状。 -->
  <svg class="hud-icon" viewBox="0 0 24 24" aria-hidden="true">
    <rect x="3" y="3" width="6" height="6" rx="1" fill="currentColor" opacity="0.88" />
    <rect x="15" y="3" width="6" height="6" rx="1" fill="currentColor" opacity="0.7" />
    <rect x="3" y="15" width="6" height="6" rx="1" fill="currentColor" opacity="0.7" />
    <rect x="15" y="15" width="6" height="6" rx="1" fill="currentColor" opacity="0.88" />
    <path d="M9 6h6M6 9v6M18 9v6M9 18h6" stroke="currentColor" stroke-width="1.4" opacity="0.6" />
  </svg>
</template>

<style scoped>
.hud-icon {
  width: 16px;
  height: 16px;
  display: block;
}
</style>
