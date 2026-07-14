<template>
  <div
    class="sc-wrap"
    :class="`sc-wrap--${props.status}`"
  >
    <span class="sc-dot"></span>
    <span class="sc-label">{{ props.label }}</span>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{
  status: 'good' | 'warn' | 'error'
  label: string
}>()
</script>

<style scoped>
.sc-wrap {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 9px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  font-family: 'Baloo 2', 'Patrick Hand', sans-serif;
}

.sc-wrap--good  {
  background: rgba(42, 224, 132, 0.12);
  color: #2ae084;
  border: 2px solid rgba(42, 224, 132, 0.5);
  box-shadow: 0 2px 0 rgba(42, 224, 132, 0.3);
}
.sc-wrap--warn  {
  background: rgba(245, 197, 66, 0.12);
  color: #f5c542;
  border: 2px solid rgba(245, 197, 66, 0.5);
  box-shadow: 0 2px 0 rgba(245, 197, 66, 0.3);
}
.sc-wrap--error {
  background: rgba(255, 68, 85, 0.12);
  color: #ff4455;
  border: 2px solid rgba(255, 68, 85, 0.5);
  box-shadow: 0 2px 0 rgba(255, 68, 85, 0.3);
}

.sc-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  border: 1.5px solid rgba(255,255,255,0.2);
}

.sc-wrap--good  .sc-dot { background: #2ae084; }
.sc-wrap--warn  .sc-dot { background: #f5c542; }
.sc-wrap--error .sc-dot { background: #ff4455; }
</style>
