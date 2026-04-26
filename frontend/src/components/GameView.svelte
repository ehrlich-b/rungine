<script lang="ts">
  import { onMount } from 'svelte';
  import Board from './Board.svelte';
  import EvalGraph from './EvalGraph.svelte';
  import { STARTING_FEN } from '../lib/chess';
  import type { main } from '../../wailsjs/go/models';

  type Props = {
    detail: main.GameDetail;
    onClose?: () => void;
  };

  let { detail, onClose }: Props = $props();

  // currentPly: 0 = starting position, 1..N = after Nth ply.
  let currentPly = $state(0);
  let flipped = $state(false);

  let totalPlies = $derived(detail.moves.length);
  let position = $derived(currentPly === 0 ? detail.startFen : detail.moves[currentPly - 1]?.fen ?? STARTING_FEN);
  let lastMove = $derived(currentPly === 0 ? null : detail.moves[currentPly - 1]?.uci ?? null);
  let currentMove = $derived(currentPly === 0 ? null : detail.moves[currentPly - 1] ?? null);

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
    return () => window.removeEventListener('keydown', key);
  });

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
    while (i < detail.moves.length) {
      const w = detail.moves[i];
      let b: main.MoveDetail | null = null;
      const pair = { num: Math.floor(i / 2) + 1, white: w, black: null as main.MoveDetail | null };
      if (i + 1 < detail.moves.length && detail.moves[i + 1].side === 'b') {
        b = detail.moves[i + 1];
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
</script>

<div class="game-view">
  <header class="head">
    <div class="title">
      <button class="back" onclick={() => onClose?.()} title="Back (Esc)">←</button>
      <span class="round muted">Round {detail.round}</span>
      <span class="players">
        <strong>{detail.white}</strong>
        <span class="muted">vs</span>
        <strong>{detail.black}</strong>
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
      {#if detail.pgn}
        <button onclick={copyPGN}>Copy PGN</button>
      {/if}
    </div>
  </header>

  <div class="layout">
    <div class="board-area">
      <Board fen={position} {flipped} {lastMove} size={56} />
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
      {#if detail.moves.length > 0}
        <EvalGraph
          moves={detail.moves}
          {currentPly}
          onJump={(p) => go(p)}
          height={70} />
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
      {#if detail.moves.length === 0}
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
</style>
