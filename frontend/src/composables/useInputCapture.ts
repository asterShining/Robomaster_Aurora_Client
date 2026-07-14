import { onBeforeUnmount, onMounted, type Ref } from 'vue'
import { useInputCaptureStore } from '../store/inputCapture'

export function useInputCapture(targetRef: Ref<HTMLElement | null>): void {
  const inputStore = useInputCaptureStore()

  onMounted(() => {
    // What: 在根容器挂载后注入输入监听。
    // Why: 保证 pointer lock 目标元素稳定存在，避免初始化阶段空引用。
    inputStore.install(() => targetRef.value)
  })

  onBeforeUnmount(() => {
    // What: 组件销毁时解除监听并清理状态。
    // Why: 防止热更新或页面切换时残留全局监听导致串扰。
    inputStore.uninstall()
  })
}
