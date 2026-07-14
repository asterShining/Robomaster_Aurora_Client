/** @type {import('tailwindcss').Config} */
// Tailwind CSS 配置文件
// 功能：告诉 Tailwind 扫描哪些文件中的 class 名，并据此生成对应的 CSS 样式
// 原因 (Why)：缺少此文件时 Tailwind 不会生成任何 utility class，导致 UI 元素无样式、不可见
export default {
  // content 数组指定 Tailwind 需要扫描的模板文件路径
  // 这样 backdrop-blur-md、bg-white/10、text-cyan-400 等类才会被编译到最终 CSS 中
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}
