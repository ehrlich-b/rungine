<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { App, on } from '../lib/wails';
  import { navigate } from '../lib/router';
  import GameView from '../components/GameView.svelte';
  import LiveGames from '../components/LiveGames.svelte';
  import type { registry, main } from '../../wailsjs/go/models';

  type Format = 'match' | 'round-robin' | 'gauntlet';

  let installed = $state<registry.InstalledEngine[]>([]);
  let selected = $state<string[]>([]);
  let format = $state<Format>('match');
  let games = $state(4);
  let movetimeMs = $state(200);
  let concurrency = $state(1);
  let pairMode = $state(true);

  let starting = $state(false);
  let error = $state<string | null>(null);
  let activeTournamentId = $state<string | null>(null);
  let summary = $state<main.TournamentSummary | null>(null);
  let tournaments = $state<main.TournamentSummary[]>([]);

  let viewingGame = $state<{ id: string; gameNumber: number } | null>(null);
  let gameDetail = $state<main.GameDetail | null>(null);

  let unsubs: Array<() => void> = [];

  async function openGame(id: string, gameNumber: number) {
    viewingGame = { id, gameNumber };
    try {
      gameDetail = await App.GetGameDetail(id, gameNumber);
    } catch (e) {
      error = `Failed to load game: ${e}`;
      viewingGame = null;
    }
  }

  function closeGame() {
    viewingGame = null;
    gameDetail = null;
  }

  async function refresh() {
    try {
      installed = (await App.ListInstalledEngines()) ?? [];
      tournaments = (await App.ListTournaments()) ?? [];
      if (activeTournamentId) {
        summary = await App.GetTournament(activeTournamentId);
      }
    } catch (e) {
      error = `Refresh failed: ${e}`;
    }
  }

  function toggleEngine(id: string) {
    if (selected.includes(id)) {
      selected = selected.filter((x) => x !== id);
    } else {
      selected = [...selected, id];
    }
  }

  function canStart(): boolean {
    if (starting) return false;
    if (format === 'match' && selected.length !== 2) return false;
    return selected.length >= 2 && games >= 1 && movetimeMs >= 50;
  }

  async function start() {
    error = null;
    starting = true;
    try {
      const id = await App.StartTournament({
        format,
        engines: selected.map((eid) => ({ id: eid })),
        games,
        concurrency,
        timeControlMs: movetimeMs,
        depthLimit: 0,
        event: 'Rungine GUI',
        pairMode,
        maxPlies: 400,
        resignScore: 0,
        resignMoves: 4,
        drawScore: -1,
        drawMoves: 8,
        drawMinPly: 60,
      } as any);
      activeTournamentId = id;
      summary = await App.GetTournament(id);
      tournaments = (await App.ListTournaments()) ?? [];
    } catch (e) {
      error = `Start failed: ${e}`;
    } finally {
      starting = false;
    }
  }

  async function stop() {
    if (!activeTournamentId) return;
    try {
      await App.StopTournament(activeTournamentId);
    } catch (e) {
      error = `Stop failed: ${e}`;
    }
  }

  async function exportPGN() {
    if (!summary) return;
    try {
      const pgn = await App.GetTournamentPGN(summary.id);
      const blob = new Blob([pgn], { type: 'application/x-chess-pgn' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${summary.id}.pgn`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (e) {
      error = `Export failed: ${e}`;
    }
  }

  function formatOutcome(o: string): string {
    if (o === '1-0') return 'White wins';
    if (o === '0-1') return 'Black wins';
    if (o === '1/2-1/2') return 'Draw';
    return o || '—';
  }

  function outcomeClass(o: string): string {
    if (o === '1-0') return 'win';
    if (o === '0-1') return 'loss';
    if (o === '1/2-1/2') return 'draw';
    return '';
  }

  onMount(() => {
    refresh();

    unsubs.push(
      on<{ tournamentId: string }>('tournament:gameComplete', async (p) => {
        if (p && (p.tournamentId === activeTournamentId || activeTournamentId === null)) {
          if (activeTournamentId) {
            summary = await App.GetTournament(activeTournamentId);
          }
        }
      }),
      on<{ tournamentId: string; status: string }>('tournament:done', async (p) => {
        if (p && p.tournamentId === activeTournamentId) {
          summary = await App.GetTournament(activeTournamentId);
        }
        tournaments = (await App.ListTournaments()) ?? [];
      }),
    );
  });

  onDestroy(() => {
    unsubs.forEach((u) => u());
  });
</script>

<section class="page">
  <header>
    <h1>Tournaments</h1>
    {#if summary && summary.status === 'running'}
      <button class="danger" onclick={stop}>Stop tournament</button>
    {/if}
  </header>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  {#if installed.length < 2}
    <div class="empty">
      <h2>Need at least two engines</h2>
      <p class="subtle">Install engines first, then come back to start a tournament.</p>
      <button class="primary" onclick={() => navigate('engines')}>Go to Engines</button>
    </div>
  {:else}
    <div class="layout">
      <form
        class="setup"
        onsubmit={(e) => {
          e.preventDefault();
          if (canStart()) start();
        }}>
        <h2>New tournament</h2>

        <div class="field">
          <span class="label">Engines ({selected.length})</span>
          <div class="engines">
            {#each installed as eng (eng.ID)}
              {@const checked = selected.includes(eng.ID)}
              <label class="check" class:on={checked}>
                <input
                  type="checkbox"
                  {checked}
                  onchange={() => toggleEngine(eng.ID)} />
                <span>{eng.Name}</span>
                <span class="muted">{eng.Version}</span>
              </label>
            {/each}
          </div>
          {#if format === 'match' && selected.length !== 2}
            <span class="hint">Match needs exactly two engines</span>
          {/if}
        </div>

        <div class="field">
          <span class="label">Format</span>
          <div class="seg">
            {#each ['match', 'round-robin', 'gauntlet'] as f (f)}
              <button
                type="button"
                class:active={format === f}
                onclick={() => (format = f as Format)}>
                {f}
              </button>
            {/each}
          </div>
        </div>

        <div class="grid">
          <label>
            <span class="label">Games</span>
            <input type="number" min="1" max="1000" bind:value={games} />
          </label>
          <label>
            <span class="label">Movetime (ms)</span>
            <input type="number" min="50" max="60000" step="50" bind:value={movetimeMs} />
          </label>
          <label>
            <span class="label">Concurrency</span>
            <input type="number" min="1" max="32" bind:value={concurrency} />
          </label>
          <label class="check inline">
            <input type="checkbox" bind:checked={pairMode} />
            <span>Pair mode (mirror colors)</span>
          </label>
        </div>

        <button type="submit" class="primary" disabled={!canStart()}>
          {starting ? 'Starting…' : 'Start tournament'}
        </button>
      </form>

      <div class="dashboard">
        {#if viewingGame && gameDetail}
          <GameView detail={gameDetail} onClose={closeGame} />
        {:else if summary}
          <div class="card">
            <div class="card-head">
              <strong>Tournament {summary.id}</strong>
              <span class="status status-{summary.status}">{summary.status}</span>
              {#if summary.outcomes.length > 0}
                <button class="export" onclick={exportPGN}>Export PGN</button>
              {/if}
            </div>
            <div class="progress">
              <div
                class="bar"
                style:width="{summary.gamesTotal > 0
                  ? (summary.gamesPlayed / summary.gamesTotal) * 100
                  : 0}%">
              </div>
              <span class="progress-text">
                {summary.gamesPlayed} / {summary.gamesTotal} games
              </span>
            </div>

            {#if summary.status === 'running'}
              <LiveGames
                tournamentId={summary.id}
                onSelectGame={(n) => openGame(summary!.id, n)} />
            {/if}

            {#if summary.standings.length > 0}
              <h3>Standings</h3>
              <table class="standings">
                <thead>
                  <tr>
                    <th>Engine</th>
                    <th>G</th>
                    <th>W</th>
                    <th>D</th>
                    <th>L</th>
                    <th>Pts</th>
                    <th>Elo</th>
                    <th>± 95%</th>
                  </tr>
                </thead>
                <tbody>
                  {#each summary.standings as p (p.name)}
                    <tr>
                      <td>{p.name}</td>
                      <td>{p.games}</td>
                      <td>{p.wins}</td>
                      <td>{p.draws}</td>
                      <td>{p.losses}</td>
                      <td>{p.points.toFixed(1)}</td>
                      <td>{p.elo > 0 ? '+' : ''}{p.elo.toFixed(0)}</td>
                      <td class="muted">
                        [{p.eloLo > 0 ? '+' : ''}{p.eloLo.toFixed(0)},
                        {p.eloHi > 0 ? '+' : ''}{p.eloHi.toFixed(0)}]
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}

            {#if summary.crosstable.players.length >= 3}
              <h3>Crosstable</h3>
              <div class="crosstable-wrap">
                <table class="crosstable">
                  <thead>
                    <tr>
                      <th></th>
                      {#each summary.crosstable.players as p (p)}
                        <th class="rot" title={p}>{p.slice(0, 8)}</th>
                      {/each}
                      <th>Pts</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each summary.crosstable.players as p, i (p)}
                      <tr>
                        <th class="row-name">{p}</th>
                        {#each summary.crosstable.players as _, j}
                          {@const cell = summary.crosstable.cells[i]?.[j]}
                          {#if i === j}
                            <td class="diag">—</td>
                          {:else if !cell || cell.games === 0}
                            <td class="muted">·</td>
                          {:else}
                            <td>{cell.points.toFixed(1)}/{cell.games}</td>
                          {/if}
                        {/each}
                        <td class="row-total">
                          {summary.standings.find((s) => s.name === p)?.points.toFixed(1) ?? '—'}
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}

            {#if summary.outcomes.length > 0}
              <h3>Games ({summary.outcomes.length})</h3>
              <div class="games">
                {#each summary.outcomes.slice().reverse() as g (g.gameNumber)}
                  <button
                    class="game"
                    class:err={g.error}
                    onclick={() => openGame(summary!.id, g.gameNumber)}>
                    <span class="g-num muted">#{g.gameNumber}</span>
                    <span class="g-pair">
                      {g.white} <span class="muted">vs</span> {g.black}
                    </span>
                    <span class="g-result {outcomeClass(g.outcome)}">
                      {g.error ? `error: ${g.error}` : formatOutcome(g.outcome)}
                    </span>
                    {#if g.reason}
                      <span class="muted small">({g.reason})</span>
                    {/if}
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        {:else}
          <div class="hint-card">
            Configure a tournament on the left and click Start. Live standings and
            game results appear here.
          </div>
        {/if}

        {#if tournaments.length > 1}
          <h3 class="section">All tournaments</h3>
          <div class="tlist">
            {#each tournaments.slice().reverse() as t (t.id)}
              <button
                class="tlist-item"
                class:active={t.id === activeTournamentId}
                onclick={async () => {
                  activeTournamentId = t.id;
                  summary = await App.GetTournament(t.id);
                }}>
                <span class="tlist-id">{t.id}</span>
                <span class="muted">{t.spec.format}</span>
                <span class="status status-{t.status}">{t.status}</span>
                <span class="muted small">
                  {t.gamesPlayed}/{t.gamesTotal}
                </span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {/if}
</section>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-lg);
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .error {
    background: rgba(248, 113, 113, 0.1);
    border: 1px solid var(--danger);
    color: var(--danger);
    padding: var(--space-sm) var(--space-md);
    border-radius: var(--radius-sm);
  }

  .empty {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-md);
    padding: var(--space-xl);
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    max-width: 640px;
  }

  .empty h2 {
    text-transform: none;
    letter-spacing: 0;
    font-size: 1.25rem;
    color: var(--text-primary);
    font-weight: 600;
    margin: 0;
  }

  .layout {
    display: grid;
    grid-template-columns: minmax(300px, 380px) 1fr;
    gap: var(--space-lg);
    align-items: flex-start;
  }

  @media (max-width: 900px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }

  .setup,
  .card,
  .hint-card {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-md) var(--space-lg);
  }

  .setup {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
  }

  h2 {
    margin: 0;
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text-secondary);
    font-weight: 500;
  }

  h3 {
    font-size: 0.78rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text-secondary);
    font-weight: 500;
    margin-top: var(--space-md);
    margin-bottom: var(--space-sm);
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-xs);
  }

  .label {
    font-size: 0.75rem;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .engines {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 200px;
    overflow-y: auto;
  }

  .check {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    padding: 4px 8px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    border: 1px solid transparent;
  }

  .check.inline {
    align-self: flex-end;
  }

  .check:hover {
    background: var(--surface-2);
  }

  .check.on {
    background: rgba(74, 222, 128, 0.08);
    border-color: rgba(74, 222, 128, 0.3);
  }

  .seg {
    display: flex;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }

  .seg button {
    flex: 1;
    border: none;
    border-radius: 0;
    background: transparent;
    color: var(--text-secondary);
    padding: 6px 12px;
    font-size: 0.85rem;
    text-transform: capitalize;
  }

  .seg button.active {
    background: var(--surface-2);
    color: var(--text-primary);
  }

  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-sm);
  }

  .grid label {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .hint {
    font-size: 0.75rem;
    color: var(--warning);
  }

  .hint-card {
    color: var(--text-secondary);
    text-align: center;
    padding: var(--space-xl);
  }

  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }

  .card-head {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }

  .card-head .export {
    margin-left: auto;
    font-size: 0.8rem;
    padding: 4px 10px;
  }

  .status {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    padding: 2px 8px;
    border-radius: 999px;
    background: var(--surface-2);
    color: var(--text-secondary);
  }

  .status-running {
    background: rgba(74, 222, 128, 0.15);
    color: var(--accent);
  }

  .status-done {
    background: var(--surface-2);
    color: var(--text-secondary);
  }

  .status-stopped,
  .status-error {
    background: rgba(248, 113, 113, 0.15);
    color: var(--danger);
  }

  .progress {
    position: relative;
    height: 8px;
    background: var(--surface-2);
    border-radius: 999px;
    overflow: hidden;
  }

  .progress .bar {
    background: var(--accent);
    height: 100%;
    transition: width 200ms ease;
  }

  .progress-text {
    position: absolute;
    top: 12px;
    right: 0;
    font-size: 0.75rem;
    color: var(--text-secondary);
  }

  .standings {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }

  .standings th,
  .standings td {
    padding: 4px 8px;
    text-align: left;
  }

  .standings th {
    color: var(--text-secondary);
    font-weight: 500;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    border-bottom: 1px solid var(--border);
  }

  .standings td:nth-child(n + 2) {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  .standings th:nth-child(n + 2) {
    text-align: right;
  }

  .crosstable-wrap {
    overflow-x: auto;
  }

  .crosstable {
    border-collapse: collapse;
    font-size: 0.8rem;
    font-variant-numeric: tabular-nums;
  }

  .crosstable th,
  .crosstable td {
    padding: 4px 8px;
    text-align: center;
    border: 1px solid var(--border);
  }

  .crosstable th.rot {
    writing-mode: vertical-rl;
    transform: rotate(180deg);
    font-weight: 500;
    color: var(--text-secondary);
    font-size: 0.7rem;
    height: 60px;
  }

  .crosstable .row-name {
    text-align: left;
    font-weight: 500;
    white-space: nowrap;
  }

  .crosstable .row-total {
    font-weight: 600;
  }

  .crosstable .diag {
    background: var(--surface-2);
    color: var(--text-muted);
  }

  .games {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-height: 320px;
    overflow-y: auto;
  }

  .game {
    display: grid;
    grid-template-columns: auto 1fr auto auto;
    gap: var(--space-sm);
    align-items: center;
    padding: 4px 8px;
    border-radius: var(--radius-sm);
    font-size: 0.8rem;
  }

  .game:hover {
    background: var(--surface-2);
  }

  .g-num {
    font-variant-numeric: tabular-nums;
  }

  .g-result.win {
    color: var(--result-win);
  }
  .g-result.loss {
    color: var(--result-loss);
  }
  .g-result.draw {
    color: var(--result-draw);
  }

  .g-result {
    font-weight: 500;
  }

  .game.err {
    background: rgba(248, 113, 113, 0.06);
  }

  .small {
    font-size: 0.7rem;
  }

  .section {
    margin-top: var(--space-md);
  }

  .tlist {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: var(--space-sm);
  }

  .tlist-item {
    display: grid;
    grid-template-columns: auto 1fr auto auto;
    gap: var(--space-sm);
    align-items: center;
    text-align: left;
    padding: 6px 10px;
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 0.85rem;
  }

  .tlist-item:hover {
    background: var(--surface-2);
  }

  .tlist-item.active {
    border-color: var(--accent);
  }

  .tlist-id {
    font-family: ui-monospace, SFMono-Regular, monospace;
    color: var(--text-primary);
  }

  .dashboard {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
    min-width: 0;
  }
</style>
