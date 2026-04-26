import { test, expect, setupMock } from './fixtures';

test('theme defaults to dark', async ({ page }) => {
  await setupMock(page);
  await page.goto('/');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
});

test('switching to light theme updates html and persists', async ({ page }) => {
  await setupMock(page);
  await page.goto('/#/settings');

  const lightBtn = page.getByRole('button', { name: 'Light', exact: true });
  const darkBtn = page.getByRole('button', { name: 'Dark', exact: true });

  await lightBtn.click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');

  // Should survive reload via localStorage.
  await page.reload();
  await page.waitForSelector('html[data-theme="light"]');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');

  await darkBtn.click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
});

test('settings page shows CPU features', async ({ page }) => {
  await setupMock(page, { cpuFeatures: 'AVX512,AVX2' });
  await page.goto('/#/settings');
  await expect(page.getByText('AVX512,AVX2', { exact: true })).toBeVisible();
});
