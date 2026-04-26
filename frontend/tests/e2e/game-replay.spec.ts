import { test, expect, setupMock, makeInstalled } from './fixtures';

const sampleDetail = {
  gameNumber: 1,
  round: '1',
  white: 'SF-A',
  black: 'SF-B',
  result: '1-0',
  reason: 'checkmate',
  startFen: 'rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1',
  pgn: '[Event "Test"]\n\n1. e4 e5 2. Nf3 Nc6 1-0',
  moves: [
    {
      ply: 1,
      side: 'w',
      uci: 'e2e4',
      san: 'e4',
      fen: 'rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1',
      depth: 12,
      evalCp: 35,
      elapsedMs: 200,
      clockAfterMs: 0,
    },
    {
      ply: 2,
      side: 'b',
      uci: 'e7e5',
      san: 'e5',
      fen: 'rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 2',
      depth: 12,
      evalCp: -10,
      elapsedMs: 200,
      clockAfterMs: 0,
    },
    {
      ply: 3,
      side: 'w',
      uci: 'g1f3',
      san: 'Nf3',
      fen: 'rnbqkbnr/pppp1ppp/8/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R b KQkq - 1 2',
      depth: 14,
      evalCp: 28,
      elapsedMs: 250,
      clockAfterMs: 0,
    },
  ],
};

const twoEngines = [
  makeInstalled({ ID: 'sf-a', Name: 'SF-A' }),
  makeInstalled({ ID: 'sf-b', Name: 'SF-B' }),
];

function makeTournamentSummary(id: string) {
  return {
    id,
    spec: { format: 'match', engines: [{ id: 'sf-a' }, { id: 'sf-b' }] },
    status: 'done',
    startedAt: new Date().toISOString(),
    finishedAt: new Date().toISOString(),
    gamesTotal: 1,
    gamesPlayed: 1,
    outcomes: [
      {
        gameNumber: 1,
        round: '1',
        white: 'SF-A',
        black: 'SF-B',
        outcome: '1-0',
        reason: 'checkmate',
        plies: 3,
      },
    ],
    standings: [
      { name: 'SF-A', wins: 1, draws: 0, losses: 0, games: 1, points: 1, elo: 100, eloLo: -50, eloHi: 250 },
      { name: 'SF-B', wins: 0, draws: 0, losses: 1, games: 1, points: 0, elo: -100, eloLo: -250, eloHi: 50 },
    ],
    crosstable: { players: [], cells: [] },
  };
}

async function setupTournamentWithGame(page: any) {
  // Two tournaments so the tournament-list UI renders (it only shows when >1).
  await setupMock(page, {
    installed: twoEngines,
    tournaments: [makeTournamentSummary('t1'), makeTournamentSummary('t2')],
    gameDetails: { 't1/1': sampleDetail, 't2/1': sampleDetail },
  });
}

test('clicking a finished game opens the replay view', async ({ page }) => {
  await setupTournamentWithGame(page);
  await page.goto('/');

  // The first tournament is loaded once we click in the tournament list.
  // List item selector — there's only one, so it's auto-selected at goto time? Let me trigger.
  await page.waitForSelector('.tlist');
  await page.locator('.tlist-item').first().click();
  await expect(page.getByRole('heading', { name: 'Standings' })).toBeVisible();

  await page.locator('.game').first().click();

  const view = page.locator('.game-view');
  await expect(view).toBeVisible();
  await expect(view.getByText('SF-A')).toBeVisible();
  await expect(view.getByText('1-0')).toBeVisible();
  await expect(view.getByText('checkmate')).toBeVisible();
});

test('arrow keys advance and rewind plies', async ({ page }) => {
  await setupTournamentWithGame(page);
  await page.goto('/');

  await page.waitForSelector('.tlist');
  await page.locator('.tlist-item').first().click();
  await page.locator('.game').first().click();

  // Initially at ply 0/3.
  await expect(page.locator('.ply')).toContainText('0 / 3');

  await page.keyboard.press('ArrowRight');
  await expect(page.locator('.ply')).toContainText('1 / 3');
  await expect(page.locator('.ply')).toContainText('e4');

  await page.keyboard.press('ArrowRight');
  await expect(page.locator('.ply')).toContainText('2 / 3');
  await expect(page.locator('.ply')).toContainText('e5');

  await page.keyboard.press('End');
  await expect(page.locator('.ply')).toContainText('3 / 3');
  await expect(page.locator('.ply')).toContainText('Nf3');

  await page.keyboard.press('Home');
  await expect(page.locator('.ply')).toContainText('0 / 3');
});

test('clicking move list jumps to that ply', async ({ page }) => {
  await setupTournamentWithGame(page);
  await page.goto('/');
  await page.waitForSelector('.tlist');
  await page.locator('.tlist-item').first().click();
  await page.locator('.game').first().click();

  // Scope to button.move (excludes the "Starting position" div).
  await page.locator('.move-list button.move').nth(2).click(); // Nf3
  await expect(page.locator('.ply')).toContainText('3 / 3');
});

test('eval graph appears for games with moves', async ({ page }) => {
  await setupTournamentWithGame(page);
  await page.goto('/');
  await page.waitForSelector('.tlist');
  await page.locator('.tlist-item').first().click();
  await page.locator('.game').first().click();

  await expect(page.locator('svg.graph')).toBeVisible();
});
