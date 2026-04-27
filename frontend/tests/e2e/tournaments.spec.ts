import { test, expect, setupMock, makeInstalled } from './fixtures';

const twoEngines = [
  makeInstalled({ ID: 'sf-a', Name: 'SF-A' }),
  makeInstalled({ ID: 'sf-b', Name: 'SF-B' }),
];

test('shows empty state with link to engines page when no engines installed', async ({ page }) => {
  await setupMock(page, { installed: [] });
  await page.goto('/');

  await expect(page.getByText('Install an engine to begin')).toBeVisible();
  await page.getByRole('button', { name: 'Go to Engines' }).click();
  await expect(page).toHaveURL(/#\/engines$/);
});

test('one installed engine can play itself by adding two slots', async ({ page }) => {
  await setupMock(page, {
    installed: [makeInstalled({ ID: 'sf-a', Name: 'SF-A' })],
  });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();

  await expect(page.getByText('Tournament slots (2)')).toBeVisible();

  await page.getByRole('button', { name: 'Start tournament' }).click();
  const calls = await page.evaluate(() => window.__rungineMock.state.startTournamentCalls);
  expect(calls).toHaveLength(1);
  expect(calls[0].engines).toHaveLength(2);
  expect(calls[0].engines[0].id).toBe('sf-a');
  expect(calls[0].engines[1].id).toBe('sf-a');
  expect(calls[0].engines[0].name).toBe('SF-A');
  expect(calls[0].engines[1].name).toBe('SF-A #2');
});

test('setup form submits a tournament spec to the backend', async ({ page }) => {
  await setupMock(page, { installed: twoEngines });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-B/ }).click();

  await page.getByLabel('Games').fill('6');
  await page.getByLabel('Movetime (ms)').fill('150');
  await page.getByLabel('Concurrency').fill('2');

  await page.getByRole('button', { name: 'Start tournament' }).click();

  const calls = await page.evaluate(() => window.__rungineMock.state.startTournamentCalls);
  expect(calls).toHaveLength(1);
  expect(calls[0].format).toBe('match');
  expect(calls[0].engines).toEqual([
    { id: 'sf-a', name: 'SF-A', options: undefined },
    { id: 'sf-b', name: 'SF-B', options: undefined },
  ]);
  expect(calls[0].games).toBe(6);
  expect(calls[0].timeControlMs).toBe(150);
  expect(calls[0].concurrency).toBe(2);
});

test('changing format to gauntlet allows three engines', async ({ page }) => {
  await setupMock(page, {
    installed: [...twoEngines, makeInstalled({ ID: 'sf-c', Name: 'SF-C' })],
  });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-B/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-C/ }).click();

  // Match would reject 3 slots; switch to gauntlet.
  await page.getByRole('button', { name: 'gauntlet', exact: true }).click();

  await page.getByRole('button', { name: 'Start tournament' }).click();
  const calls = await page.evaluate(() => window.__rungineMock.state.startTournamentCalls);
  expect(calls[0].format).toBe('gauntlet');
});

test('match format requires exactly two engines', async ({ page }) => {
  await setupMock(page, {
    installed: [...twoEngines, makeInstalled({ ID: 'sf-c', Name: 'SF-C' })],
  });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-B/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-C/ }).click();

  await expect(page.getByText('Match needs exactly two slots')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Start tournament' })).toBeDisabled();
});

test('per-slot UCI option overrides are sent to the backend', async ({ page }) => {
  await setupMock(page, {
    installed: [makeInstalled({ ID: 'sf-a', Name: 'SF-A' })],
  });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();

  // Open options on slot 2 and set Hash override.
  const optionsButtons = page.getByRole('button', { name: 'Options', exact: true });
  await optionsButtons.nth(1).click();
  const optionsAreas = page.locator('.slot-options');
  await optionsAreas.last().fill('Hash=512\nThreads=4');

  await page.getByRole('button', { name: 'Start tournament' }).click();
  const calls = await page.evaluate(() => window.__rungineMock.state.startTournamentCalls);
  expect(calls[0].engines[0].options).toBeUndefined();
  expect(calls[0].engines[1].options).toEqual({ Hash: '512', Threads: '4' });
});

test('adjudication defaults are sent in the tournament spec', async ({ page }) => {
  await setupMock(page, { installed: twoEngines });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-B/ }).click();
  await page.getByRole('button', { name: 'Start tournament' }).click();

  const calls = await page.evaluate(() => window.__rungineMock.state.startTournamentCalls);
  expect(calls).toHaveLength(1);
  expect(calls[0].resignScore).toBe(900);
  expect(calls[0].resignMoves).toBe(4);
  expect(calls[0].drawScore).toBe(10);
  expect(calls[0].drawMoves).toBe(8);
  expect(calls[0].drawMinPly).toBe(60);
  expect(calls[0].maxPlies).toBe(400);
});

test('unchecking adjudication boxes sends backend "off" sentinels', async ({ page }) => {
  await setupMock(page, { installed: twoEngines });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-B/ }).click();

  await page.getByLabel('Resign when losing').uncheck();
  await page.getByLabel('Adjudicate quiet draws').uncheck();

  await page.getByRole('button', { name: 'Start tournament' }).click();

  const calls = await page.evaluate(() => window.__rungineMock.state.startTournamentCalls);
  expect(calls[0].resignScore).toBe(0);
  expect(calls[0].resignMoves).toBe(0);
  expect(calls[0].drawScore).toBe(-1);
  expect(calls[0].drawMoves).toBe(0);
  expect(calls[0].drawMinPly).toBe(0);
});

test('custom adjudication thresholds round-trip through the form', async ({ page }) => {
  await setupMock(page, { installed: twoEngines });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-B/ }).click();

  await page.getByLabel('Resign threshold (cp)').fill('600');
  await page.getByLabel('Resign consecutive moves').fill('6');
  await page.getByLabel('Draw eval threshold (cp)').fill('5');
  await page.getByLabel('Draw consecutive plies').fill('12');
  await page.getByLabel('Draw min ply').fill('40');
  await page.getByLabel('Max plies').fill('300');

  await page.getByRole('button', { name: 'Start tournament' }).click();

  const calls = await page.evaluate(() => window.__rungineMock.state.startTournamentCalls);
  expect(calls[0].resignScore).toBe(600);
  expect(calls[0].resignMoves).toBe(6);
  expect(calls[0].drawScore).toBe(5);
  expect(calls[0].drawMoves).toBe(12);
  expect(calls[0].drawMinPly).toBe(40);
  expect(calls[0].maxPlies).toBe(300);
});

test('starting a tournament shows a running dashboard with progress', async ({ page }) => {
  await setupMock(page, { installed: twoEngines });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-B/ }).click();
  await page.getByLabel('Games').fill('4');
  await page.getByRole('button', { name: 'Start tournament' }).click();

  await expect(page.getByText(/Tournament t1/)).toBeVisible();
  await expect(page.locator('.status-running').first()).toContainText('running');
  await expect(page.locator('.progress-text')).toContainText('0 / 4 games');
});

test('live games grid populates from gameStart and move events', async ({ page }) => {
  await setupMock(page, { installed: twoEngines });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-B/ }).click();
  await page.getByRole('button', { name: 'Start tournament' }).click();
  await expect(page.getByText('Tournament t1')).toBeVisible();

  // Fire a gameStart event.
  await page.evaluate(() => {
    window.__rungineMock.fire('tournament:gameStart', {
      tournamentId: 't1',
      gameNumber: 1,
      round: '1',
      white: 'SF-A',
      black: 'SF-B',
    });
  });

  await expect(page.getByText('Live games (1)')).toBeVisible();
  await expect(page.getByText('#1')).toBeVisible();

  // Fire a move event with eval — tile should update.
  await page.evaluate(() => {
    window.__rungineMock.fire('tournament:move', {
      tournamentId: 't1',
      gameNumber: 1,
      ply: 1,
      side: 'w',
      uci: 'e2e4',
      san: 'e4',
      fen: 'rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1',
      depth: 12,
      evalCp: 35,
      elapsedMs: 200,
    });
  });

  await expect(page.getByText('ply 1')).toBeVisible();
  await expect(page.getByText('+0.35')).toBeVisible();
  await expect(page.getByText(/last: e4/)).toBeVisible();
});

test('stop button cancels the running tournament', async ({ page }) => {
  await setupMock(page, { installed: twoEngines });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-B/ }).click();
  await page.getByRole('button', { name: 'Start tournament' }).click();

  await page.getByRole('button', { name: 'Stop tournament' }).click();

  // Re-fetch summary after stop emits 'tournament:done'.
  await page.evaluate(() => {
    window.__rungineMock.fire('tournament:done', { tournamentId: 't1', status: 'stopped' });
  });

  await expect(page.locator('.status').first()).toContainText('stopped');
});

test('completed games appear in the standings table', async ({ page }) => {
  await setupMock(page, { installed: twoEngines });
  await page.goto('/');

  await page.getByRole('button', { name: /\+\s*SF-A/ }).click();
  await page.getByRole('button', { name: /\+\s*SF-B/ }).click();
  await page.getByRole('button', { name: 'Start tournament' }).click();
  await expect(page.getByText('Tournament t1')).toBeVisible();

  // Push a finished game into mock state and fire complete event.
  await page.evaluate(() => {
    const t = window.__rungineMock.state.tournaments[0];
    t.outcomes = [
      {
        gameNumber: 1,
        round: '1',
        white: 'SF-A',
        black: 'SF-B',
        outcome: '1-0',
        reason: 'checkmate',
        plies: 30,
      },
    ];
    t.gamesPlayed = 1;
    t.standings = [
      { name: 'SF-A', wins: 1, draws: 0, losses: 0, games: 1, points: 1, elo: 100, eloLo: -50, eloHi: 250 },
      { name: 'SF-B', wins: 0, draws: 0, losses: 1, games: 1, points: 0, elo: -100, eloLo: -250, eloHi: 50 },
    ];
    window.__rungineMock.fire('tournament:gameComplete', { tournamentId: 't1', row: t.outcomes[0] });
  });

  await expect(page.getByRole('heading', { name: 'Standings' })).toBeVisible();
  await expect(page.locator('.standings tbody tr').first()).toContainText('SF-A');
  await expect(page.locator('.standings tbody tr').first()).toContainText('+100');
});
