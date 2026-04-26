<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Board from './Board.svelte';
  import EvalGraph from './EvalGraph.svelte';
  import { STARTING_FEN, parseFEN, coordsToSquare } from '../lib/chess';
  import { on } from '../lib/wails';
  import { main } from '../../wailsjs/go/models';

  type Props = {
    detail: main.GameDetail;
    onClose?: () => void;
    // When set, GameView subscribes to live tournament events for this
    // game and renders a per-engine analysis panel.
    tournamentId?: string;
  };

  let { detail, onClose, tournamentId }: Props = $props();

  type EngineInfo = {
    engine: string;
    depth: number;
    selDepth: number;
    nodes: number;
    nps: number;
    timeMs: number;
    pv: string[];
    evalCp: number | null;
    evalMate: number | null;
  };

  let liveWhite = $state<EngineInfo | null>(null);
  let liveBlack = $state<EngineInfo | null>(null);
  let unsubs: Array<() => void> = [];

  // currentPly: 0 = starting position, 1..N = after Nth ply.
  let currentPly = $state(0);
  let flipped = $state(false);
  let showArrows = $state(true);

  // Mirror of detail.moves that grows as live tournament:move events arrive.
  // Resets when the prop changes (different game opened); live events
  // subscribed in onMount append to it.
  let liveMoves = $state<main.MoveDetail[]>([]);
  $effect(() => {
    liveMoves = [...detail.moves];
  });

  let totalPlies = $derived(liveMoves.length);
  let position = $derived(currentPly === 0 ? detail.startFen : liveMoves[currentPly - 1]?.fen ?? STARTING_FEN);
  let lastMove = $derived(currentPly === 0 ? null : liveMoves[currentPly - 1]?.uci ?? null);
  let currentMove = $derived(currentPly === 0 ? null : liveMoves[currentPly - 1] ?? null);

  function go(target: number) {
    currentPly = Math.max(0, Math.min(totalPlies, target));
  }

  function key(e: KeyboardEvent) {
    if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
    if (e.key === 'ArrowLeft') {
      e.preventDefault();
      go(currentPly - 1);
    } else if (e.key === 'ArrowRight') {
      e.preventDefault();
      go(currentPly + 1);
    } else if (e.key === 'Home') {
      e.preventDefault();
      go(0);
    } else if (e.key === 'End') {
      e.preventDefault();
      go(totalPlies);
    } else if (e.key === 'f' || e.key === 'F') {
      flipped = !flipped;
    } else if (e.key === 'Escape' && onClose) {
      onClose();
    }
  }

  onMount(() => {
    window.addEventListener('keydown', key);
    if (tournamentId) {
      unsubs.push(
        on<any>('tournament:engineInfo', (p) => {
          if (!p) return;
          if (p.tournamentId !== tournamentId) return;
          if (p.gameNumber !== detail.gameNumber) return;
          const info: EngineInfo = {
            engine: p.engine ?? '',
            depth: p.depth ?? 0,
            selDepth: p.selDepth ?? 0,
            nodes: p.nodes ?? 0,
            nps: p.nps ?? 0,
            timeMs: p.timeMs ?? 0,
            pv: Array.isArray(p.pv) ? p.pv : [],
            evalCp: p.evalCp ?? null,
            evalMate: p.evalMate ?? null,
          };
          if (p.side === 'b') liveBlack = info;
          else liveWhite = info;
        }),
        on<any>('tournament:move', (p) => {
          if (!p) return;
          if (p.tournamentId !== tournamentId) return;
          if (p.gameNumber !== detail.gameNumber) return;
          if (typeof p.ply !== 'number' || p.ply <= liveMoves.length) return;
          const md = main.MoveDetail.createFrom({
            ply: p.ply,
            side: p.side ?? 'w',
            uci: p.uci ?? '',
            san: p.san ?? '',
            fen: p.fen ?? '',
            depth: p.depth,
            evalCp: p.evalCp ?? undefined,
            evalMate: p.evalMate ?? undefined,
            elapsedMs: p.elapsedMs ?? 0,
            clockAfterMs: p.clockAfterMs ?? 0,
            check: p.check,
          });
          const wasAtEnd = currentPly === liveMoves.length;
          liveMoves = [...liveMoves, md];
          if (wasAtEnd) currentPly = liveMoves.length;
        }),
      );
    }
    return () => window.removeEventListener('keydown', key);
  });

  onDestroy(() => unsubs.forEach((u) => u()));

  function fmtNumber(n: number): string {
    if (n >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(1)}B`;
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
    return n.toString();
  }

  function fmtScore(info: EngineInfo): string {
    if (info.evalMate !== null) return info.evalMate > 0 ? `M${info.evalMate}` : `-M${-info.evalMate}`;
    if (info.evalCp !== null) {
      const cp = info.evalCp / 100;
      return cp >= 0 ? `+${cp.toFixed(2)}` : cp.toFixed(2);
    }
    return '—';
  }

  function formatEval(m: main.MoveDetail): string {
    if (m.evalMate !== undefined && m.evalMate !== null) {
      return m.evalMate > 0 ? `M${m.evalMate}` : `-M${-m.evalMate}`;
    }
    if (m.evalCp !== undefined && m.evalCp !== null) {
      const cp = m.evalCp / 100;
      return cp >= 0 ? `+${cp.toFixed(2)}` : cp.toFixed(2);
    }
    return '';
  }

  function evalColor(m: main.MoveDetail | null): string {
    if (!m) return '';
    if (m.evalMate !== undefined && m.evalMate !== null) {
      return m.evalMate > 0 ? 'good' : 'bad';
    }
    if (m.evalCp !== undefined && m.evalCp !== null) {
      if (m.evalCp > 50) return 'good';
      if (m.evalCp < -50) return 'bad';
    }
    return '';
  }

  function moveClick(target: number) {
    go(target);
  }

  function copyPGN() {
    if (detail.pgn) {
      navigator.clipboard.writeText(detail.pgn);
    }
  }

  function pairs(): { num: number; white: main.MoveDetail | null; black: main.MoveDetail | null }[] {
    const out: { num: number; white: main.MoveDetail | null; black: main.MoveDetail | null }[] = [];
    let i = 0;
    while (i < liveMoves.length) {
      const w = liveMoves[i];
      let b: main.MoveDetail | null = null;
      const pair = { num: Math.floor(i / 2) + 1, white: w, black: null as main.MoveDetail | null };
      if (i + 1 < liveMoves.length && liveMoves[i + 1].side === 'b') {
        b = liveMoves[i + 1];
        pair.black = b;
        i += 2;
      } else {
        i += 1;
      }
      out.push(pair);
    }
    return out;
  }

  let movePairs = $derived(pairs());

  type Arrow = { from: string; to: string; color?: string; weight?: number };

  function pvToArrow(info: EngineInfo | null, color: string): Arrow | null {
    if (!info || !info.pv || info.pv.length === 0) return null;
    const move = info.pv[0];
    if (!move || move.length < 4) return null;
    return { from: move.slice(0, 2), to: move.slice(2, 4), color };
  }

  let boardArrows = $derived.by<Arrow[]>(() => {
    if (!showArrows) return [];
    // Only meaningful while the game is live: at the latest ply with no result yet.
    if (currentPly !== totalPlies) return [];
    if (detail.result && detail.result !== '*') return [];
    const sideToMove = totalPlies % 2 === 0 ? 'w' : 'b';
    const info = sideToMove === 'w' ? liveWhite : liveBlack;
    const arrow = pvToArrow(info, 'var(--accent)');
    return arrow ? [arrow] : [];
  });

  function findKingSquare(fen: string, color: 'w' | 'b'): string | null {
    try {
      const pos = parseFEN(fen);
      const target = color === 'w' ? 'K' : 'k';
      for (let r = 0; r < 8; r++) {
        for (let f = 0; f < 8; f++) {
          if (pos.board[r][f] === target) {
            return coordsToSquare(f, r);
          }
        }
      }
    } catch {
      return null;
    }
    return null;
  }

  let checkSquare = $derived.by<string | null>(() => {
    if (currentPly === 0) return null;
    const m = liveMoves[currentPly - 1];
    if (!m || !m.check) return null;
    // Side to move after this ply is the side that just received check.
    const kingOf: 'w' | 'b' = m.side === 'w' ? 'b' : 'w';
    return findKingSquare(m.fen, kingOf);
  });
</script>

<div class="game-view">
  <header class="head">
    <div class="title">
      <button class="back" onclick={() => onClose?.()} title="Back (Esc)">←</button>
      <span class="round muted">Round {detail.round}</span>
      <span class="players">
        <strong>{detail.white}</strong>
        {#if detail.whiteSha}
          <span class="sha" title={detail.whiteSha}>#{detail.whiteSha.slice(0, 7)}</span>
        {/if}
        <span class="muted">vs</span>
        <strong>{detail.black}</strong>
        {#if detail.blackSha}
          <span class="sha" title={detail.blackSha}>#{detail.blackSha.slice(0, 7)}</span>
        {/if}
      </span>
      <span class="result">{detail.result || '*'}</span>
      {#if detail.reason}
        <span class="muted small">({detail.reason})</span>
      {/if}
    </div>
    <div class="actions">
      <button onclick={() => (flipped = !flipped)} title="Flip board (F)">
        {flipped ? 'Unflip' : 'Flip'}
      </button>
      {#if tournamentId}
        <button
          onclick={() => (showArrows = !showArrows)}
          class:active={showArrows}
          title="Toggle PV arrow on board">
          {showArrows ? 'Arrows on' : 'Arrows off'}
        </button>
      {/if}
      {#if detail.pgn}
        <button onclick={copyPGN}>Copy PGN</button>
      {/if}
    </div>
  </header>

  <div class="layout">
    <div class="board-area">
      <Board fen={position} {flipped} {lastMove} arrows={boardArrows} {checkSquare} size={56} />
      <div class="nav">
        <button onclick={() => go(0)} title="Start (Home)">⏮</button>
        <button onclick={() => go(currentPly - 1)} title="Previous (←)">◀</button>
        <span class="ply">
          {currentPly} / {totalPlies}
          {#if currentMove}
            <span class="muted small">· {currentMove.san}</span>
          {/if}
        </span>
        <button onclick={() => go(currentPly + 1)} title="Next (→)">▶</button>
        <button onclick={() => go(totalPlies)} title="End (End)">⏭</button>
      </div>
      {#if totalPlies > 0}
        <input
          class="scrubber"
          type="range"
          min="0"
          max={totalPlies}
          step="1"
          value={currentPly}
          aria-label="Game timeline"
          oninput={(e) => go(parseInt((e.currentTarget as HTMLInputElement).value, 10))} />
      {/if}
      {#if currentMove}
        <div class="move-info">
          {#if formatEval(currentMove)}
            <span class="eval {evalColor(currentMove)}">{formatEval(currentMove)}</span>
          {/if}
          {#if currentMove.depth && currentMove.depth > 0}
            <span class="muted">d{currentMove.depth}</span>
          {/if}
          {#if currentMove.elapsedMs > 0}
            <span class="muted">{(currentMove.elapsedMs / 1000).toFixed(1)}s</span>
          {/if}
        </div>
      {/if}
      {#if liveMoves.length > 0}
        <EvalGraph
          moves={liveMoves}
          {currentPly}
          onJump={(p) => go(p)}
          height={70} />
      {/if}
      {#if liveWhite || liveBlack}
        <div class="engines-panel">
          {#each [{ side: 'white', info: liveWhite }, { side: 'black', info: liveBlack }] as engine (engine.side)}
            {@const info = engine.info}
            {#if info}
              <div class="engine-card engine-{engine.side}">
                <div class="engine-head">
                  <strong>{info.engine || engine.side}</strong>
                  <span class="engine-eval">{fmtScore(info)}</span>
                </div>
                <div class="engine-stats muted small">
                  <span>d{info.depth}{info.selDepth ? `/${info.selDepth}` : ''}</span>
                  <span>{fmtNumber(info.nodes)}n</span>
                  {#if info.nps > 0}
                    <span>{fmtNumber(info.nps)}/s</span>
                  {/if}
                  {#if info.timeMs > 0}
                    <span>{(info.timeMs / 1000).toFixed(1)}s</span>
                  {/if}
                </div>
                {#if info.pv.length > 0}
                  <div class="engine-pv">
                    {info.pv.slice(0, 12).join(' ')}
                  </div>
                {/if}
              </div>
            {/if}
          {/each}
        </div>
      {/if}
    </div>

    <div class="move-list">
      <div
        class="move ply-zero"
        class:active={currentPly === 0}
        onclick={() => go(0)}
        onkeydown={(e) => e.key === 'Enter' && go(0)}
        role="button"
        tabindex="0">
        Starting position
      </div>
      {#each movePairs as p (p.num)}
        <div class="move-row">
          <span class="num muted">{p.num}.</span>
          {#if p.white}
            {@const idx = (p.num - 1) * 2 + 1}
            <button
              class="move"
              class:active={currentPly === idx}
              onclick={() => moveClick(idx)}>
              <span class="san">{p.white.san}</span>
              {#if formatEval(p.white)}
                <span class="me {evalColor(p.white)}">{formatEval(p.white)}</span>
              {/if}
            </button>
          {/if}
          {#if p.black}
            {@const idx = (p.num - 1) * 2 + 2}
            <button
              class="move"
              class:active={currentPly === idx}
              onclick={() => moveClick(idx)}>
              <span class="san">{p.black.san}</span>
              {#if formatEval(p.black)}
                <span class="me {evalColor(p.black)}">{formatEval(p.black)}</span>
              {/if}
            </button>
          {/if}
        </div>
      {/each}
      {#if liveMoves.length === 0}
        <p class="muted small">No moves recorded.</p>
      {/if}
    </div>
  </div>
</div>

<style>
  .game-view {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-md) var(--space-lg);
  }

  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-md);
  }

  .title {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    flex-wrap: wrap;
  }

  .back {
    background: transparent;
    border: 1px solid var(--border);
    width: 28px;
    padding: 2px 0;
    border-radius: var(--radius-sm);
    font-size: 1.1rem;
    line-height: 1;
  }

  .round {
    font-size: 0.8rem;
  }

  .players strong {
    color: var(--text-primary);
  }

  .result {
    background: var(--surface-2);
    padding: 2px 8px;
    border-radius: var(--radius-sm);
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.85rem;
  }

  .sha {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.7rem;
    color: var(--text-muted);
    background: var(--surface-2);
    padding: 1px 5px;
    border-radius: var(--radius-sm);
    cursor: help;
  }

  .actions {
    display: flex;
    gap: var(--space-xs);
  }

  .layout {
    display: grid;
    grid-template-columns: auto minmax(220px, 1fr);
    gap: var(--space-lg);
    align-items: flex-start;
  }

  @media (max-width: 900px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }

  .board-area {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
    align-items: flex-start;
  }

  .nav {
    display: flex;
    align-items: center;
    gap: 4px;
    width: 100%;
  }

  .nav button {
    width: 32px;
    padding: 4px 0;
    font-size: 0.85rem;
  }

  .ply {
    flex: 1;
    text-align: center;
    font-size: 0.85rem;
    font-variant-numeric: tabular-nums;
  }

  .scrubber {
    width: 100%;
    -webkit-appearance: none;
    appearance: none;
    background: transparent;
    margin: 0;
    height: 18px;
    cursor: pointer;
  }

  .scrubber::-webkit-slider-runnable-track {
    height: 4px;
    background: var(--surface-3);
    border-radius: 2px;
  }

  .scrubber::-moz-range-track {
    height: 4px;
    background: var(--surface-3);
    border-radius: 2px;
  }

  .scrubber::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 14px;
    height: 14px;
    background: var(--accent);
    border-radius: 50%;
    border: none;
    margin-top: -5px;
    cursor: pointer;
  }

  .scrubber::-moz-range-thumb {
    width: 14px;
    height: 14px;
    background: var(--accent);
    border-radius: 50%;
    border: none;
    cursor: pointer;
  }

  .move-info {
    display: flex;
    gap: var(--space-md);
    font-size: 0.85rem;
    align-items: center;
  }

  .eval {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: var(--radius-sm);
    background: var(--surface-2);
  }

  .eval.good {
    color: var(--result-win);
  }

  .eval.bad {
    color: var(--result-loss);
  }

  .move-list {
    max-height: 540px;
    overflow-y: auto;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: var(--space-sm);
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 0.85rem;
  }

  .ply-zero {
    text-align: left;
    padding: 4px 8px;
    border-radius: var(--radius-sm);
    color: var(--text-secondary);
    cursor: pointer;
    border: 1px solid transparent;
    background: transparent;
  }

  .ply-zero.active,
  .move.active {
    background: var(--accent);
    color: var(--accent-text);
  }

  .ply-zero:hover {
    background: var(--surface-3);
  }

  .move-row {
    display: grid;
    grid-template-columns: 28px 1fr 1fr;
    align-items: center;
    gap: 2px;
  }

  .num {
    text-align: right;
    padding-right: 4px;
    font-variant-numeric: tabular-nums;
  }

  .move {
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    padding: 2px 6px;
    text-align: left;
    font-size: 0.85rem;
    color: var(--text-primary);
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-xs);
  }

  .move:hover {
    background: var(--surface-3);
  }

  .me {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.7rem;
    opacity: 0.8;
  }

  .move.active .me {
    color: var(--accent-text);
  }

  .me.good {
    color: var(--result-win);
  }
  .me.bad {
    color: var(--result-loss);
  }

  .small {
    font-size: 0.75rem;
  }

  .engines-panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-xs);
    width: 100%;
  }

  .engine-card {
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 6px 10px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .engine-card.engine-white {
    border-left: 3px solid var(--text-primary);
  }

  .engine-card.engine-black {
    border-left: 3px solid var(--text-secondary);
  }

  .engine-head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: var(--space-sm);
    font-size: 0.85rem;
  }

  .engine-eval {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-weight: 600;
  }

  .engine-stats {
    display: flex;
    gap: var(--space-sm);
    flex-wrap: wrap;
    font-variant-numeric: tabular-nums;
  }

  .engine-pv {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.75rem;
    color: var(--text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
