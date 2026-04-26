import { test, expect, setupMock, makeInstalled } from './fixtures';

test('analyze view starts with starting position rendered (32 pieces)', async ({ page }) => {
  await setupMock(page);
  await page.goto('/#/analyze');
  await expect(page.locator('.piece')).toHaveCount(32);
});

test('flipping board and editing FEN re-renders correctly', async ({ page }) => {
  await setupMock(page);
  await page.goto('/#/analyze');

  // Empty board FEN.
  await page.getByLabel('FEN').fill('8/8/8/8/8/8/8/8 w - - 0 1');
  await expect(page.locator('.piece')).toHaveCount(0);

  // Reset to startpos.
  await page.getByRole('button', { name: 'Reset to startpos' }).click();
  await expect(page.locator('.piece')).toHaveCount(32);
});

test('start analysis sends the engine list and FEN to the backend', async ({ page }) => {
  await setupMock(page, {
    installed: [
      makeInstalled({ ID: 'sf-a', Name: 'SF-A' }),
      makeInstalled({ ID: 'sf-b', Name: 'SF-B' }),
    ],
  });
  await page.goto('/#/analyze');

  await page.getByRole('checkbox', { name: /SF-A/ }).check();
  await page.getByRole('checkbox', { name: /SF-B/ }).check();
  await page.getByRole('button', { name: 'Start analysis' }).click();

  const calls = await page.evaluate(() => window.__rungineMock.state.startAnalysisCalls);
  expect(calls).toHaveLength(1);
  expect(calls[0].engineIds).toEqual(['sf-a', 'sf-b']);
  expect(calls[0].infinite).toBe(true);
  expect(calls[0].fen).toContain('rnbqkbnr');
});

test('analysis info events update the engine panels', async ({ page }) => {
  await setupMock(page, {
    installed: [makeInstalled({ ID: 'sf-a', Name: 'SF-A' })],
  });
  await page.goto('/#/analyze');

  await page.getByRole('checkbox', { name: /SF-A/ }).check();
  await page.getByRole('button', { name: 'Start analysis' }).click();

  // Stream an info update.
  await page.evaluate(() => {
    window.__rungineMock.fire('analysis:info', {
      EngineID: 'sf-a',
      Depth: 18,
      SelDepth: 22,
      Score: { Centipawns: 42, Mate: null },
      Nodes: 1500000,
      NPS: 2_500_000,
      Time: 600_000_000, // ns -> 600ms reads as 600
      PV: ['e2e4', 'e7e5', 'g1f3'],
      MultiPV: 1,
    });
  });

  await expect(page.getByText('+0.42')).toBeVisible();
  await expect(page.getByText('depth 18/22')).toBeVisible();
  await expect(page.getByText('1.5M nodes')).toBeVisible();
  await expect(page.getByText('e2e4 e7e5 g1f3')).toBeVisible();
});

test('stop button calls StopAnalysis with selected engines', async ({ page }) => {
  await setupMock(page, {
    installed: [makeInstalled({ ID: 'sf-a', Name: 'SF-A' })],
  });
  await page.goto('/#/analyze');

  await page.getByRole('checkbox', { name: /SF-A/ }).check();
  await page.getByRole('button', { name: 'Start analysis' }).click();

  await page.getByRole('button', { name: 'Stop' }).click();

  const calls = await page.evaluate(() => window.__rungineMock.state.stopAnalysisCalls);
  expect(calls).toEqual([['sf-a']]);
});
