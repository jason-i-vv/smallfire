import { test, expect } from '@playwright/test'

test.describe('Auth Flow', () => {
  test('login page renders correctly', async ({ page }) => {
    await page.goto('/login')

    // Check logo
    await expect(page.locator('.logo-icon')).toBeVisible()
    await expect(page.locator('.logo-text')).toHaveText('Starfire')

    // Check form elements
    await expect(page.locator('.login-title')).toBeVisible()
    await expect(page.locator('input[placeholder*="邮箱"]')).toBeVisible()

    // Check language switcher
    await expect(page.locator('.language-switcher')).toBeVisible()
  })

  test('login page responsive layout on mobile', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 })
    await page.goto('/login')

    // Left panel should be visible but compact
    await expect(page.locator('.auth-left')).toBeVisible()

    // Right panel should be below the left panel
    const leftPanel = page.locator('.auth-left')
    const rightPanel = page.locator('.auth-right')
    const leftBox = await leftPanel.boundingBox()
    const rightBox = await rightPanel.boundingBox()

    // On mobile, right panel should be below left panel
    expect(rightBox.y).toBeGreaterThan(leftBox.y)
  })

  test('language switcher changes UI language', async ({ page }) => {
    await page.goto('/login')

    // Default is Chinese
    await expect(page.locator('.login-title')).toHaveText('登录')

    // Click English button
    await page.locator('.lang-btn', { hasText: 'EN' }).click()

    // Should change to English
    await expect(page.locator('.login-title')).toHaveText('Sign in')

    // Switch back to Chinese
    await page.locator('.lang-btn', { hasText: '中文' }).click()
    await expect(page.locator('.login-title')).toHaveText('登录')
  })

  test('google button shows alert message', async ({ page }) => {
    await page.goto('/login')

    // Set up dialog handler
    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('即将支持')
      await dialog.dismiss()
    })

    await page.locator('.google-button').click()
  })

  test('register page renders correctly', async ({ page }) => {
    await page.goto('/register')

    // Check form elements
    await expect(page.locator('.register-title')).toBeVisible()
    await expect(page.locator('.register-logo')).toBeVisible()

    // Check all input fields
    await expect(page.locator('input[placeholder*="用户名"]')).toBeVisible()
    await expect(page.locator('input[placeholder*="昵称"]')).toBeVisible()
  })

  test('register page has working language switcher', async ({ page }) => {
    await page.goto('/register')

    // Default is Chinese
    await expect(page.locator('.register-title')).toHaveText('注册')

    // Click English button
    await page.locator('.lang-btn', { hasText: 'EN' }).click()

    // Should change to English
    await expect(page.locator('.register-title')).toHaveText('Create account')
  })

  test('navigation between login and register', async ({ page }) => {
    await page.goto('/login')

    // Click register link
    await page.locator('a', { hasText: '立即注册' }).click()
    await expect(page).toHaveURL(/\/register/)
    await expect(page.locator('.register-title')).toBeVisible()

    // Go back to login
    await page.locator('a', { hasText: '立即登录' }).click()
    await expect(page).toHaveURL(/\/login/)
    await expect(page.locator('.login-title')).toBeVisible()
  })

  test('form validation shows error on empty submit', async ({ page }) => {
    await page.goto('/login')

    // Click login button without entering credentials
    await page.locator('.auth-button').click()

    // Should show validation error (Element Plus form validation)
    await expect(page.locator('.el-form-item__error')).toBeVisible()
  })

  test('K-line visualization renders on login page', async ({ page }) => {
    await page.goto('/login')

    // Canvas should exist
    await expect(page.locator('canvas.kline-canvas')).toBeVisible()

    // Brand overlay should be visible
    await expect(page.locator('.brand')).toBeVisible()
    await expect(page.locator('.tagline')).toHaveText('智能量化，稳健收益')
  })
})
