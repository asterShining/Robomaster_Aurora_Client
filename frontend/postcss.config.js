// PostCSS 配置文件
// 功能：注册 Tailwind CSS 和 Autoprefixer 为 PostCSS 插件
// 原因 (Why)：Vite 使用 PostCSS 处理 CSS 文件，此文件告知 Vite 在构建时
//   先运行 Tailwind 生成 utility class，再运行 Autoprefixer 添加浏览器兼容前缀
//   （如 -webkit-backdrop-filter 等，对 Wails/WebKit2GTK 至关重要）
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
