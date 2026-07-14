<template>
  <div class="robot-icon" :class="[`side-${side}`, { offline: !alive }]">
    <span class="name">{{ label }}</span>
    <span class="hp">{{ hp }}</span>
  </div>
</template>

<script setup lang="ts">
interface Props {
  label: string
  hp: number
  alive?: boolean
  side: 'red' | 'blue'
}

const props = withDefaults(defineProps<Props>(), {
  alive: true,
})

// What: 让 props 在脚本域显式使用。Why: 保持 SFC 类型推断稳定且便于后续扩展验证逻辑。
void props
</script>

<style scoped>
.robot-icon {
  /* What: 队伍单元改为胶囊圆角。Why: 统一全局圆润视觉语言，避免顶部出现硬切角。 */
  min-width: 48px;
  height: 20px;
  padding: 0 6px;
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  border: 1px solid rgba(120, 160, 255, 0.38);
  background: rgba(9, 15, 28, 0.85);
  border-radius: 999px;
}

.robot-icon.side-red {
  border-color: rgba(255, 126, 126, 0.46);
}

.robot-icon.side-blue {
  border-color: rgba(114, 178, 255, 0.46);
}

.robot-icon.offline {
  opacity: 0.38;
  filter: grayscale(0.6);
}

.name {
  font-size: 10px;
  color: rgba(216, 232, 255, 0.9);
  white-space: nowrap;
}

.hp {
  font-size: 10px;
  color: rgba(191, 213, 252, 0.86);
  font-family: 'JetBrains Mono', 'Consolas', monospace;
}
</style>
