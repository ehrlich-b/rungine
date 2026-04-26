import { test, expect, setupMock, makeInstalled, type TournamentSummaryMock } from './fixtures';

// Wails default window size (set in main.go).
const WAILS_VIEWPORT = { width: 1024, height: 768 };

const threeEngines = [
  makeInstalled({ ID: 'sf17', RegistryID: 'stockfish-17', Name: 'Stockfish 17', Version: '17' }),
  makeInstalled({ ID: 'sf161', RegistryID: 'stockfish-161', Name: 'Stockfish 16.1', Version: '16.1' }),
  makeInstalled({ ID: 'sf17b', RegistryID: 'stockfish-17', Name: 'Stockfish 17 (Hash 256)', Version: '17' }),
];

function runningTournament(): TournamentSummaryMock {
  return {
    id: 't1',
    spec: {
      format: 'round-robin',
      engines: [{ id: 'sf17' }, { id: 'sf161' }, { id: 'sf17b' }],
      games: 1,
    },
    status: 'running',
    startedAt: new Date().toISOString(),
    gamesTotal: 12,
    gamesPlayed: 7,
    outcomes: [
      { gameNumber: 1, round: '1', white: 'Stockfish 17', black: 'Stockfish 16.1', outcome: '1-0', reason: 'checkmate', plies: 64 },
      { gameNumber: 2, round: '2', white: 'Stockfish 16.1', black: 'Stockfish 17', outcome: '0-1', reason: 'resignation', plies: 58 },
      { gameNumber: 3, round: '3', white: 'Stockfish 17', black: 'Stockfish 17 (Hash 256)', outcome: '1/2-1/2', reason: 'threefold-repetition', plies: 90 },
      { gameNumber: 4, round: '4', white: 'Stockfish 17 (Hash 256)', black: 'Stockfish 17', outcome: '1/2-1/2', reason: '50-move-rule', plies: 100 },
      { gameNumber: 5, round: '5', white: 'Stockfish 16.1', black: 'Stockfish 17 (Hash 256)', outcome: '0-1', reason: 'checkmate', plies: 73 },
      { gameNumber: 6, round: '6', white: 'Stockfish 17 (Hash 256)', black: 'Stockfish 16.1', outcome: '1-0', reason: 'checkmate', plies: 80 },
      { gameNumber: 7, round: '7', white: 'Stockfish 17', black: 'Stockfish 16.1', outcome: '1-0', reason: 'checkmate', plies: 47 },
    ],
    standings: [
      { name: 'Stockfish 17', wins: 3, draws: 1, losses: 0, games: 4, points: 3.5, elo: 156, eloLo: 18, eloHi: 312 },
      { name: 'Stockfish 17 (Hash 256)', wins: 2, draws: 1, losses: 0, games: 3, points: 2.5, elo: 88, eloLo: -45, eloHi: 248 },
      { name: 'Stockfish 16.1', wins: 0, draws: 0, losses: 4, games: 4, points: 0, elo: -210, eloLo: -380, eloHi: -65 },
    ],
    crosstable: {
      players: ['Stockfish 17', 'Stockfish 17 (Hash 256)', 'Stockfish 16.1'],
      cells: [
        [
          { wins: 0, draws: 0, losses: 0, games: 0, points: 0 },
          { wins: 0, draws: 1, losses: 0, games: 1, points: 0.5 },
          { wins: 3, draws: 0, losses: 0, games: 3, points: 3 },
        ],
        [
          { wins: 0, draws: 1, losses: 0, games: 1, points: 0.5 },
          { wins: 0, draws: 0, losses: 0, games: 0, points: 0 },
          { wins: 1, draws: 0, losses: 0, games: 1, points: 1 },
        ],
        [
          { wins: 0, draws: 0, losses: 3, games: 3, points: 0 },
          { wins: 0, draws: 0, losses: 1, games: 1, points: 0 },
          { wins: 0, draws: 0, losses: 0, games: 0, points: 0 },
        ],
      ],
    },
  };
}

async function setupRunning(page: any) {
  await setupMock(page, {
    installed: threeEngines,
    tournaments: [runningTournament()],
  });
  await page.setViewportSize(WAILS_VIEWPORT);
  await page.goto('/');
  // Latest tournament auto-selects on mount; standings should appear directly.
  await expect(page.getByRole('heading', { name: 'Standings' })).toBeVisible();
}

async function fireLiveGames(page: any) {
  await page.evaluate(() => {
    const fire = window.__rungineMock.fire;
    fire('tournament:gameStart', {
      tournamentId: 't1',
      gameNumber: 8,
      round: '8',
      white: 'Stockfish 17',
      black: 'Stockfish 17 (Hash 256)',
    });
    fire('tournament:move', {
      tournamentId: 't1',
      gameNumber: 8,
      ply: 5,
      side: 'w',
      uci: 'g1f3',
      san: 'Nf3',
      fen: 'r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3',
      depth: 18,
      evalCp: 22,
      elapsedMs: 220,
    });
    fire('tournament:gameStart', {
      tournamentId: 't1',
      gameNumber: 9,
      round: '9',
      white: 'Stockfish 16.1',
      black: 'Stockfish 17',
    });
    fire('tournament:move', {
      tournamentId: 't1',
      gameNumber: 9,
      ply: 3,
      side: 'b',
      uci: 'b8c6',
      san: 'Nc6',
      fen: 'r1bqkbnr/pppp1ppp/2n5/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 2 2',
      depth: 14,
      evalCp: -8,
      elapsedMs: 200,
    });
  });
}

function rectsOverlap(a: { x: number; y: number; width: number; height: number }, b: typeof a): boolean {
  if (!a || !b) return false;
  return !(
    a.x + a.width <= b.x ||
    b.x + b.width <= a.x ||
    a.y + a.height <= b.y ||
    b.y + b.height <= a.y
  );
}

test('default viewport renders the running dashboard without horizontal overflow', async ({ page }) => {
  await setupRunning(page);
  await fireLiveGames(page);

  const overflow = await page.evaluate(() => {
    const el = document.scrollingElement || document.documentElement;
    return { scrollW: el.scrollWidth, clientW: el.clientWidth };
  });
  expect(overflow.scrollW).toBeLessThanOrEqual(overflow.clientW);
});

test('all dashboard sections render without occluding each other', async ({ page }) => {
  await setupRunning(page);
  await fireLiveGames(page);

  await expect(page.locator('form.setup')).toBeVisible();
  await expect(page.locator('.progress')).toBeVisible();
  await expect(page.locator('table.standings')).toBeVisible();
  await expect(page.locator('table.crosstable')).toBeVisible();
  await expect(page.locator('.games')).toBeVisible();

  // Live games grid populated from the fired events.
  await expect(page.getByText('Live games (2)')).toBeVisible();
  const liveTiles = await page.locator('.tile').count();
  expect(liveTiles).toBe(2);

  // Pairwise overlap check for the major non-overlapping regions.
  const sections = ['form.setup', '.dashboard'];
  const boxes = await Promise.all(
    sections.map((s) => page.locator(s).first().boundingBox()),
  );
  expect(rectsOverlap(boxes[0]!, boxes[1]!)).toBe(false);

  // Header / nav / status bar stack vertically.
  const navBox = (await page.locator('nav').first().boundingBox())!;
  const mainBox = (await page.locator('main').first().boundingBox())!;
  const footerBox = (await page.locator('footer').first().boundingBox())!;
  expect(navBox.y + navBox.height).toBeLessThanOrEqual(mainBox.y + 1);
  expect(mainBox.y + mainBox.height).toBeLessThanOrEqual(footerBox.y + 1);
});

test('standings table fits within its column without clipping the engine column', async ({ page }) => {
  await setupRunning(page);
  const standings = page.locator('table.standings');
  await standings.scrollIntoViewIfNeeded();
  const tableBox = (await standings.boundingBox())!;
  // Each row's first cell (Engine name) should fit within table width.
  const firstCell = page.locator('table.standings tbody tr').first().locator('td').first();
  const cellBox = (await firstCell.boundingBox())!;
  expect(cellBox.x).toBeGreaterThanOrEqual(tableBox.x - 1);
  expect(cellBox.x + cellBox.width).toBeLessThanOrEqual(tableBox.x + tableBox.width + 1);
});

test('long engine names in standings do not break the table layout', async ({ page }) => {
  await setupMock(page, {
    installed: threeEngines,
    tournaments: [
      {
        ...runningTournament(),
        standings: [
          {
            name: 'Stockfish 17 (Hash 1024 / Threads 8 / Tournament profile)',
            wins: 3, draws: 1, losses: 0, games: 4, points: 3.5,
            elo: 156, eloLo: 18, eloHi: 312,
          },
          ...runningTournament().standings.slice(1),
        ],
      },
    ],
  });
  await page.setViewportSize(WAILS_VIEWPORT);
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Standings' })).toBeVisible();

  const overflow = await page.evaluate(() => {
    const el = document.scrollingElement || document.documentElement;
    return el.scrollWidth - el.clientWidth;
  });
  expect(overflow).toBeLessThanOrEqual(0);
});

test('engine library page fits at 1024x768 with multiple available engines', async ({ page }) => {
  await setupMock(page, {
    installed: threeEngines,
    available: [
      { id: 'stockfish-17', name: 'Stockfish 17', version: '17', author: 'Stockfish Team', description: 'The strongest open-source chess engine.', eloEstimate: 3600, requiresNetwork: false, hasBuild: true },
      { id: 'stockfish-161', name: 'Stockfish 16.1', version: '16.1', author: 'Stockfish Team', description: 'Previous major Stockfish release, useful as a sparring partner.', eloEstimate: 3500, requiresNetwork: false, hasBuild: true },
      { id: 'lc0-032', name: 'Leela Chess Zero', version: '0.32.1', author: 'LCZero Team', description: 'Neural net engine.', eloEstimate: 3550, requiresNetwork: true, hasBuild: false },
    ],
  });
  await page.setViewportSize(WAILS_VIEWPORT);
  await page.goto('/#/engines');

  const overflow = await page.evaluate(() => {
    const el = document.scrollingElement || document.documentElement;
    return el.scrollWidth - el.clientWidth;
  });
  expect(overflow).toBeLessThanOrEqual(0);
});

test('analyze view fits at 1024x768 with multiple engine panels', async ({ page }) => {
  await setupMock(page, { installed: threeEngines });
  await page.setViewportSize(WAILS_VIEWPORT);
  await page.goto('/#/analyze');

  // Add three engine panels (use exact name matching to avoid substring collisions).
  for (const eng of threeEngines) {
    await page.getByRole('checkbox', { name: new RegExp(`^${eng.Name.replace(/[()]/g, '\\$&')}\\s+${eng.Version}$`) }).check();
  }
  await page.getByRole('button', { name: 'Start analysis' }).click();
  await page.evaluate(() => {
    for (const id of ['sf17', 'sf161', 'sf17b']) {
      window.__rungineMock.fire('analysis:info', {
        EngineID: id,
        Depth: 18, SelDepth: 22,
        Score: { Centipawns: 35, Mate: null },
        Nodes: 1500000, NPS: 2_500_000, Time: 600_000_000,
        PV: ['e2e4', 'e7e5', 'g1f3', 'b8c6'],
        MultiPV: 1,
      });
    }
  });

  const overflow = await page.evaluate(() => {
    const el = document.scrollingElement || document.documentElement;
    return el.scrollWidth - el.clientWidth;
  });
  expect(overflow).toBeLessThanOrEqual(0);
});
