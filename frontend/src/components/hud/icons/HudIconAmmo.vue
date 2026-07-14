<template>
  <!-- What: 发弹量图标原子组件。Why: 独立后可在不同火控组件间保持一致。 -->
  <svg class="hud-icon" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M8 3h8v4l2.4 2.8v8.6A2.6 2.6 0 0 1 15.8 21H8.2A2.6 2.6 0 0 1 5.6 18.4V9.8L8 7z" fill="currentColor" opacity="0.86" />
    <rect x="10" y="4.2" width="4" height="2" fill="#0b1223" opacity="0.86" />
  </svg>
</template>

<style scoped>
.hud-icon {
  width: 16px;
  height: 16px;
  display: block;
}
</style>
