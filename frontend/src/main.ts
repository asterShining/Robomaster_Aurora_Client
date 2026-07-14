import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css' // Assume tailwind directives are here

const app = createApp(App)

// What: 注入全局状态容器。Why: 让 HUD 与设置面板共享统一数据源，避免状态分叉。
app.use(createPinia())
app.mount('#app')
