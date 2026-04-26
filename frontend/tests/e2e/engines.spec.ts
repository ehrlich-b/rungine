import { test, expect, setupMock, makeAvailable, makeInstalled } from './fixtures';

test('lists available engines from the registry', async ({ page }) => {
  await setupMock(page, {
    available: [
      makeAvailable({ id: 'stockfish-17', name: 'Stockfish 17', eloEstimate: 3600 }),
      makeAvailable({ id: 'lc0', name: 'Leela Chess Zero', requiresNetwork: true }),
    ],
  });
  await page.goto('/#/engines');

  await expect(page.getByText('Stockfish 17')).toBeVisible();
  await expect(page.getByText('Leela Chess Zero')).toBeVisible();
  await expect(page.getByText('~3600 Elo')).toBeVisible();
  await expect(page.getByText('network file')).toBeVisible();
});

test('install button installs the engine and shows it as installed', async ({ page }) => {
  await setupMock(page, {
    available: [makeAvailable({ id: 'stockfish-17', name: 'Stockfish 17' })],
  });
  await page.goto('/#/engines');

  await page.getByRole('button', { name: 'Install', exact: true }).click();

  await expect(page.getByText('Installed').first()).toBeVisible();

  const installCalls = await page.evaluate(() => window.__rungineMock.state.installCalls);
  expect(installCalls).toEqual(['stockfish-17']);
});

test('uninstall removes the engine card', async ({ page }) => {
  await setupMock(page, {
    installed: [
      makeInstalled({ ID: 'sf17', RegistryID: 'stockfish-17', Name: 'Stockfish 17' }),
    ],
  });
  await page.goto('/#/engines');

  await expect(page.getByText('Stockfish 17')).toBeVisible();
  await page.getByRole('button', { name: 'Uninstall' }).click();

  await expect(page.getByText('No engines installed yet')).toBeVisible();
  const uninstallCalls = await page.evaluate(() => window.__rungineMock.state.uninstallCalls);
  expect(uninstallCalls).toEqual(['sf17']);
});

test('disables install when no compatible build', async ({ page }) => {
  await setupMock(page, {
    available: [
      makeAvailable({ id: 'arm-only', name: 'ARM Engine', hasBuild: false }),
    ],
  });
  await page.goto('/#/engines');

  await expect(page.getByText('no build for this CPU')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Install' })).toBeDisabled();
});

test('options editor saves overrides', async ({ page }) => {
  await setupMock(page, {
    installed: [
      makeInstalled({ ID: 'sf17', RegistryID: 'stockfish-17', Name: 'Stockfish 17' }),
    ],
    optionConfig: {
      values: {},
      definitions: [
        {
          name: 'Hash',
          type: 'spin',
          default: '16',
          min: 1,
          max: 33554432,
          description: 'Hash table size in MB',
          recommended: '512',
        },
        {
          name: 'Threads',
          type: 'spin',
          default: '1',
          min: 1,
          max: 1024,
          description: 'CPU threads',
        },
      ],
    },
  });
  await page.goto('/#/engines');

  await page.getByRole('button', { name: 'Configure' }).click();
  await expect(page.getByText('Hash table size in MB')).toBeVisible();

  // Apply the recommended value for Hash.
  await page.getByRole('button', { name: /use recommended \(512\)/ }).click();
  await page.getByRole('button', { name: 'Save', exact: true }).click();

  const calls = await page.evaluate(() => window.__rungineMock.state.setEngineOptionConfigCalls);
  expect(calls).toHaveLength(1);
  expect(calls[0].id).toBe('sf17');
  expect(calls[0].options.Hash).toBe('512');
});
