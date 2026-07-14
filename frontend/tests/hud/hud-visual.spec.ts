import { expect, test } from '@playwright/test'

test('HUD 主界面视觉回归', async ({ page }) => {
  await page.goto('/')

  // What: 切到编辑态后截图。Why: 该状态同时覆盖顶部工具栏与组件边界提示，便于发现样式回归。
  await page.getByRole('button', { name: '调整布局' }).click()
  await page.setViewportSize({ width: 1365, height: 768 })

  await expect(page).toHaveScreenshot('hud-main.png', {
    maxDiffPixelRatio: 0.03,
    fullPage: true,
  })
})

test('HUD 设置面板视觉回归', async ({ page }) => {
  await page.goto('/')
  await page.setViewportSize({ width: 1365, height: 768 })

  // What: 打开设置面板截图。
  // Why: 极简面板包含视频源、MQTT 身份和状态卡开关，需要锁定视觉基线。
  await page.keyboard.press('p')
  await expect(page.getByRole('dialog', { name: '视频与HUD设置' })).toBeVisible()

  await expect(page).toHaveScreenshot('hud-settings-panel.png', {
    maxDiffPixelRatio: 0.03,
    fullPage: true,
  })
})
