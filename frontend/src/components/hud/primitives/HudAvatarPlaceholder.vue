<template>
  <div class="avatar-placeholder" :class="`side-${props.side}`" :title="props.avatarKey">
    <HudAvatarImageLayer :alt="avatarAlt" :title="props.avatarKey" padding="2px" radius="10px">
      <template #fallback>
        <svg class="avatar-icon" viewBox="0 0 48 48" aria-hidden="true">
          <circle cx="24" cy="16" r="7" />
          <path d="M11 37c2.8-6.4 7.4-9.6 13-9.6S34.2 30.6 37 37" />
        </svg>
      </template>
    </HudAvatarImageLayer>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import HudAvatarImageLayer from './HudAvatarImageLayer.vue'

interface Props {
  side: 'red' | 'blue'
  avatarKey: string
}

const props = defineProps<Props>()

// What: 组合阵营与编号生成头像语义文案。
// Why: 小头像去掉可见 key 文本后，仍要保留基础识别信息给 title 和无障碍语义层。
const avatarAlt = computed(() => `${props.side === 'red' ? '红方' : '蓝方'} ${props.avatarKey} 机器人头像`)
</script>

<style scoped>
.avatar-placeholder {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  border: 1px solid rgba(144, 180, 238, 0.54);
  background: linear-gradient(145deg, rgba(9, 17, 34, 0.9), rgba(6, 10, 20, 0.85));
  display: block;
  overflow: hidden;
}

.avatar-placeholder.side-red {
  border-color: rgba(211, 119, 119, 0.62);
  background: linear-gradient(145deg, rgba(36, 14, 18, 0.9), rgba(18, 8, 11, 0.84));
}

/* What: 占位图回退时仍保持简洁线稿。 */
/* Why: 静态图片异常时继续提供稳定识别，不让小卡片出现突兀空洞。 */
.avatar-icon {
  width: 100%;
  height: 100%;
  display: block;
  fill: none;
  stroke: rgba(188, 219, 255, 0.9);
  stroke-width: 2.2;
  stroke-linecap: round;
}

.side-red .avatar-icon {
  stroke: rgba(255, 205, 205, 0.9);
}
</style>
