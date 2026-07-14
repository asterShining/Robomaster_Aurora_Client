<template>
  <section class="hud-widget-card" :class="[`accent-${accent}`]">
    <header v-if="title || subtitle" class="card-head">
      <p v-if="title" class="card-title">{{ title }}</p>
      <p v-if="subtitle" class="card-subtitle">{{ subtitle }}</p>
    </header>
    <div class="card-body">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
interface Props {
  title?: string
  subtitle?: string
  accent?: 'blue' | 'red' | 'cyan'
}

const props = withDefaults(defineProps<Props>(), {
  title: '',
  subtitle: '',
  accent: 'blue',
})

// What: 显式引用 props。Why: 确保模板中可追踪当前组件公开的展示配置。
void props
</script>

<style scoped>
.hud-widget-card {
  width: 100%;
  height: 100%;
  padding: 8px 10px;
  display: grid;
  grid-template-rows: auto 1fr;
  gap: 6px;
  border: 1px solid var(--hud-border-soft);
  background: linear-gradient(152deg, var(--hud-surface-1), var(--hud-surface-0));
  box-shadow: inset 0 0 0 1px rgba(33, 52, 92, 0.22), var(--panel-shadow);
  overflow: hidden;
  position: relative;
  border-radius: 20px;
}

.hud-widget-card.accent-red {
  border-color: var(--hud-panel-red-border);
}

.hud-widget-card.accent-cyan {
  border-color: var(--hud-border-strong);
}

/* What: 红色主题用于机器人状态等高风险信息。Why: 复刻官方红方语义，强化危险态识别。 */
.hud-widget-card.accent-red {
  background: var(--hud-panel-red-bg);
}

.card-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
}

.card-title {
  margin: 0;
  font-size: 12px;
  line-height: 1;
  color: var(--hud-text-primary);
  letter-spacing: 0.03em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-subtitle {
  margin: 0;
  font-size: 10px;
  line-height: 1;
  color: var(--hud-text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-body {
  min-height: 0;
  min-width: 0;
}
</style>
