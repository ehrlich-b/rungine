<script lang="ts">
  import { onDestroy } from 'svelte';
  import Board from './Board.svelte';
  import { STARTING_FEN } from '../lib/chess';
  import { on } from '../lib/wails';

  type Props = {
    tournamentId: string | null;
    onSelectGame?: (gameNumber: number) => void;
  };

  type LiveGame = {
    gameNumber: number;
    white: string;
    black: string;
    fen: string;
    ply: number;
    lastMove: string | null;
    lastSan: string | null;
    evalCp: number | null;
    evalMate: number | null;
    done: boolean;
    result: string;
    // Clock state. Both clocks are stored as the value on the wall at
    // the moment of lastTickEpoch; the side-to-move's clock is
    // displayed as that value minus (now - lastTickEpoch).
    whiteMs: number | null;
    blackMs: number | null;
    sideToMove: 'w' | 'b';
    lastTickEpoch: number; // Date.now() at last move event
  };

  let { tournamentId, onSelectGame }: Props = $props();

  let games = $state<Record<number, LiveGame>>({});
  // Bumped on a 200ms interval to force clock displays to re-render.
  let now = $state(Date.now());
  let unsubs: Array<() => void> = [];

  function reset() {
    games = {};
  }

  $effect(() => {
    // Reset when tournament changes
    void tournamentId;
    reset();
  });

  unsubs.push(
    on<any>('tournament:gameStart', (p) => {
      if (!p || p.tournamentId !== tournamentId) return;
      games = {
        ...games,
        [p.gameNumber]: {
          gameNumber: p.gameNumber,
          white: p.white,
          black: p.black,
          fen: STARTING_FEN,
          ply: 0,
          lastMove: null,
          lastSan: null,
          evalCp: null,
          evalMate: null,
          done: false,
          result: '*',
          whiteMs: null,
          blackMs: null,
          sideToMove: 'w',
          lastTickEpoch: Date.now(),
        },
      };
    }),
    on<any>('tournament:move', (p) => {
      if (!p || p.tournamentId !== tournamentId) return;
      const existing = games[p.gameNumber];
      if (!existing) return;
      // p.side is the side that just moved; clockAfterMs is its remaining time.
      const clock = typeof p.clockAfterMs === 'number' && p.clockAfterMs > 0 ? p.clockAfterMs : null;
      const movedSide: 'w' | 'b' = p.side === 'black' ? 'b' : 'w';
      const next: LiveGame = {
        ...existing,
        fen: p.fen,
        ply: p.ply,
        lastMove: p.uci ?? null,
        lastSan: p.san ?? null,
        evalCp: p.evalCp ?? null,
        evalMate: p.evalMate ?? null,
        sideToMove: movedSide === 'w' ? 'b' : 'w',
        lastTickEpoch: Date.now(),
      };
      if (movedSide === 'w' && clock !== null) next.whiteMs = clock;
      if (movedSide === 'b' && clock !== null) next.blackMs = clock;
      games = { ...games, [p.gameNumber]: next };
    }),
    on<any>('tournament:gameComplete', (p) => {
      if (!p || p.tournamentId !== tournamentId) return;
      const row = p.row ?? {};
      const existing = games[row.gameNumber];
      if (!existing) return;
      games = {
        ...games,
        [row.gameNumber]: {
          ...existing,
          done: true,
          result: row.outcome || '*',
        },
      };
    }),
  );

  const tickInterval = setInterval(() => {
    now = Date.now();
  }, 200);

  onDestroy(() => {
    clearInterval(tickInterval);
    unsubs.forEach((u) => u());
  });

  function displayClock(g: LiveGame, side: 'w' | 'b'): string {
    const stored = side === 'w' ? g.whiteMs : g.blackMs;
    if (stored === null) return '';
    let ms = stored;
    if (!g.done && g.sideToMove === side) {
      ms = Math.max(0, stored - (now - g.lastTickEpoch));
    }
    return formatClock(ms);
  }

  function formatClock(ms: number): string {
    const total = Math.floor(ms / 1000);
    const m = Math.floor(total / 60);
    const s = total % 60;
    if (m >= 60) {
      const h = Math.floor(m / 60);
      const rm = m % 60;
      return `${h}:${rm.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
    }
    return `${m}:${s.toString().padStart(2, '0')}`;
  }

  function activeGames(): LiveGame[] {
    return Object.values(games)
      .filter((g) => !g.done)
      .sort((a, b) => a.gameNumber - b.gameNumber);
  }

  function formatEval(g: LiveGame): string {
    if (g.evalMate !== null) {
      return g.evalMate > 0 ? `M${g.evalMate}` : `-M${-g.evalMate}`;
    }
    if (g.evalCp !== null) {
      const cp = g.evalCp / 100;
      return cp >= 0 ? `+${cp.toFixed(2)}` : cp.toFixed(2);
    }
    return '';
  }

  function evalClass(g: LiveGame): string {
    if (g.evalMate !== null) return g.evalMate > 0 ? 'good' : 'bad';
    if (g.evalCp !== null) {
      if (g.evalCp > 50) return 'good';
      if (g.evalCp < -50) return 'bad';
    }
    return '';
  }

  let active = $derived(activeGames());
</script>

{#if active.length > 0}
  <h3>Live games ({active.length})</h3>
  <div class="grid">
    {#each active as g (g.gameNumber)}
      <button
        class="tile"
        onclick={() => onSelectGame?.(g.gameNumber)}>
        <div class="tile-head">
          <span class="num muted">#{g.gameNumber}</span>
          <span class="ply muted">ply {g.ply}</span>
          {#if formatEval(g)}
            <span class="eval {evalClass(g)}">{formatEval(g)}</span>
          {/if}
        </div>
        <Board fen={g.fen} lastMove={g.lastMove} size={28} showCoords={false} />
        <div class="tile-foot">
          <span class="player white" title={g.white}>{g.white}</span>
          <span class="muted">vs</span>
          <span class="player black" title={g.black}>{g.black}</span>
        </div>
        {#if g.whiteMs !== null || g.blackMs !== null}
          <div class="clocks">
            <span class="clock white" class:active={g.sideToMove === 'w' && !g.done}>
              {displayClock(g, 'w') || '—'}
            </span>
            <span class="clock black" class:active={g.sideToMove === 'b' && !g.done}>
              {displayClock(g, 'b') || '—'}
            </span>
          </div>
        {/if}
        {#if g.lastSan}
          <div class="last-move muted small">last: {g.lastSan}</div>
        {/if}
      </button>
    {/each}
  </div>
{/if}

<style>
  h3 {
    font-size: 0.78rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text-secondary);
    font-weight: 500;
    margin-top: var(--space-md);
    margin-bottom: var(--space-sm);
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: var(--space-sm);
  }

  .tile {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-sm);
    display: flex;
    flex-direction: column;
    gap: 6px;
    cursor: pointer;
    transition: border-color 120ms ease, background 120ms ease;
    text-align: left;
  }

  .tile:hover {
    border-color: var(--accent);
    background: var(--surface-2);
  }

  .tile-head {
    display: flex;
    gap: var(--space-sm);
    align-items: center;
    font-size: 0.75rem;
  }

  .num {
    font-variant-numeric: tabular-nums;
  }

  .ply {
    flex: 1;
    text-align: center;
  }

  .eval {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-weight: 600;
    padding: 1px 6px;
    border-radius: var(--radius-sm);
    background: var(--surface-2);
    font-size: 0.7rem;
  }

  .eval.good {
    color: var(--result-win);
  }

  .eval.bad {
    color: var(--result-loss);
  }

  .tile-foot {
    display: flex;
    gap: 4px;
    align-items: center;
    font-size: 0.75rem;
  }

  .player {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .player.white {
    color: var(--text-primary);
    font-weight: 500;
  }

  .player.black {
    color: var(--text-secondary);
    text-align: right;
  }

  .last-move {
    font-size: 0.7rem;
  }

  .small {
    font-size: 0.7rem;
  }

  .clocks {
    display: flex;
    justify-content: space-between;
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.75rem;
    color: var(--text-muted);
  }

  .clock {
    padding: 1px 6px;
    border-radius: var(--radius-sm);
    font-variant-numeric: tabular-nums;
  }

  .clock.active {
    background: var(--surface-2);
    color: var(--text-primary);
  }

  .clock.black {
    text-align: right;
  }
</style>
