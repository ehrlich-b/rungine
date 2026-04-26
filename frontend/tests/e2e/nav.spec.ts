import { test, expect, setupMock } from './fixtures';

test('default route renders the Tournaments view', async ({ page }) => {
  await setupMock(page);
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Tournaments', level: 1 })).toBeVisible();
});

test('top nav switches between all four views', async ({ page }) => {
  await setupMock(page);
  await page.goto('/');

  await page.getByRole('button', { name: 'Analyze', exact: true }).click();
  await expect(page).toHaveURL(/#\/analyze$/);
  await expect(page.getByRole('heading', { name: 'Analyze', level: 1 })).toBeVisible();

  await page.getByRole('button', { name: 'Engines', exact: true }).click();
  await expect(page).toHaveURL(/#\/engines$/);
  await expect(page.getByRole('heading', { name: 'Engines', level: 1 })).toBeVisible();

  await page.getByRole('button', { name: 'Settings', exact: true }).click();
  await expect(page).toHaveURL(/#\/settings$/);
  await expect(page.getByRole('heading', { name: 'Settings', level: 1 })).toBeVisible();

  await page.getByRole('button', { name: 'Tournaments', exact: true }).click();
  await expect(page).toHaveURL(/#\/tournaments$/);
  await expect(page.getByRole('heading', { name: 'Tournaments', level: 1 })).toBeVisible();
});

test('unknown hash falls back to Tournaments', async ({ page }) => {
  await setupMock(page);
  await page.goto('/#/bogus');
  await expect(page.getByRole('heading', { name: 'Tournaments', level: 1 })).toBeVisible();
});

test('status bar shows CPU features and engine count', async ({ page }) => {
  await setupMock(page, {
    cpuFeatures: 'AVX2,BMI2,POPCNT',
    installed: [
      {
        ID: 'sf17',
        RegistryID: 'stockfish-17',
        Name: 'Stockfish 17',
        Version: '17',
        BinaryPath: '/tmp/sf',
        InstalledAt: '',
        BuildKey: 'mock',
        OptionValues: {},
      },
    ],
  });
  await page.goto('/');
  await expect(page.locator('footer')).toContainText('AVX2,BMI2,POPCNT');
  await expect(page.locator('footer')).toContainText('1 engine installed');
});
