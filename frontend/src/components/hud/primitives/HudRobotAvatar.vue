<template>
  <div class="avatar-shell" :class="`side-${props.teamSide}`">
    <HudAvatarImageLayer :alt="avatarAlt" :title="props.code" padding="6px" radius="999px">
      <template #fallback>
        <svg viewBox="0 0 64 64" class="avatar-svg" aria-hidden="true">
          <circle cx="32" cy="32" r="30" class="ring" />
          <circle cx="32" cy="32" r="21" class="inner" />
          <path
            d="M20 43c3-7 7-10 12-10s9 3 12 10"
            class="accent"
          />
          <circle cx="25" cy="27" r="3" class="accent" />
          <circle cx="39" cy="27" r="3" class="accent" />
        </svg>
      </template>
    </HudAvatarImageLayer>
    <span class="code">{{ props.code }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import HudAvatarImageLayer from './HudAvatarImageLayer.vue'

interface Props {
  teamSide: 'red' | 'blue'
  code: string
}

const props = defineProps<Props>()

// What: 根据阵营生成头像语义文案。
// Why: 让真实图片替换占位后仍保留可访问文本，并保持两侧状态语义明确。
const avatarAlt = computed(() => `${props.teamSide === 'red' ? '红方' : '蓝方'}机器人头像`)
</script>

<style scoped>
.avatar-shell {
  width: 64px;
  height: 64px;
  border-radius: 999px;
  position: relative;
  border: 2px solid var(--hud-avatar-red-border);
  background: var(--hud-avatar-red-bg);
}

.avatar-shell.side-blue {
  border-color: var(--hud-avatar-blue-border);
  background: var(--hud-avatar-blue-bg);
}

.avatar-svg {
  width: 100%;
  height: 100%;
  display: block;
}

.ring {
  fill: transparent;
  stroke: var(--hud-avatar-ring);
  stroke-width: 1.5;
}

.inner {
  fill: var(--hud-avatar-inner);
}

.accent {
  fill: none;
  stroke: var(--hud-avatar-red-accent);
  stroke-width: 2;
  stroke-linecap: round;
}

.side-blue .accent {
  stroke: var(--hud-avatar-blue-accent);
}

.code {
  position: absolute;
  left: -6px;
  top: -4px;
  width: 22px;
  height: 22px;
  border-radius: 999px;
  border: 1px solid var(--hud-avatar-red-code-border);
  display: grid;
  place-items: center;
  font-size: 9px;
  color: var(--hud-avatar-red-code-text);
  background: var(--hud-avatar-red-code-bg);
}

.side-blue .code {
  border-color: var(--hud-avatar-blue-code-border);
  color: var(--hud-avatar-blue-code-text);
  background: var(--hud-avatar-blue-code-bg);
}
</style>
