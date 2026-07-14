<template>
  <div class="avatar-image-layer" :style="layerStyle">
    <img
      v-if="!imageLoadFailed"
      class="avatar-image"
      :src="avatarSource"
      :alt="props.alt"
      :title="props.title || props.alt"
      decoding="async"
      @error="handleImageError"
    />
    <slot v-else name="fallback" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import defaultHudAvatarUrl from '../../../assets/avatars/default-robot-avatar.webp'

interface Props {
  alt: string
  title?: string
  padding?: string
  radius?: string
}

const props = withDefaults(defineProps<Props>(), {
  title: '',
  padding: '0px',
  radius: 'inherit',
})

const imageLoadFailed = ref(false)

// What: 在共享头像图层里统一持有默认头像资源。
// Why: 避免多个 HUD 组件各自导入同一张图片，减少后续替换素材时的维护分叉。
const avatarSource = defaultHudAvatarUrl

// What: 头像图片加载失败后切回调用方提供的占位图层。
// Why: 避免资源路径异常时直接出现浏览器破图图标，影响 HUD 可读性。
function handleImageError() {
  imageLoadFailed.value = true
}

// What: 把圆角和内边距透传给同一套头像样式。
// Why: 让大头像和小头像共用完整显示逻辑，同时保留各自外壳尺寸差异。
const layerStyle = computed(() => ({
  '--hud-avatar-layer-padding': props.padding,
  '--hud-avatar-layer-radius': props.radius,
}))
</script>

<style scoped>
.avatar-image-layer {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  padding: var(--hud-avatar-layer-padding);
  box-sizing: border-box;
  border-radius: var(--hud-avatar-layer-radius);
  overflow: hidden;
}

/* What: 头像图片始终完整显示在图层内部。 */
/* Why: 用户要求整张战车图替换预留头像，因此不能用 cover 裁掉主体。 */
.avatar-image {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: contain;
  object-position: center;
  transform: translateZ(0);
  will-change: transform;
}
</style>
