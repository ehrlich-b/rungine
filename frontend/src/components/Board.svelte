<script lang="ts">
  import {
    STARTING_FEN,
    parseFEN,
    pieceGlyph,
    coordsToSquare,
    type Position,
  } from '../lib/chess';

  type Props = {
    fen?: string;
    flipped?: boolean;
    showCoords?: boolean;
    /** Last move in UCI notation, e.g. "e2e4". Highlights from + to squares. */
    lastMove?: string | null;
    /** Square size in pixels. */
    size?: number;
    onSquareClick?: (square: string) => void;
  };

  let {
    fen = STARTING_FEN,
    flipped = false,
    showCoords = true,
    lastMove = null,
    size = 56,
    onSquareClick,
  }: Props = $props();

  let position = $derived<Position | null>(safeParse(fen));
  let parseError = $derived<string | null>(parseErr(fen));

  function safeParse(f: string): Position | null {
    try {
      return parseFEN(f);
    } catch {
      return null;
    }
  }

  function parseErr(f: string): string | null {
    try {
      parseFEN(f);
      return null;
    } catch (e) {
      return String(e);
    }
  }

  function highlightSet(): Set<string> {
    if (!lastMove || lastMove.length < 4) return new Set();
    return new Set([lastMove.slice(0, 2), lastMove.slice(2, 4)]);
  }

  let highlights = $derived(highlightSet());

  /** Iteration order of board ranks (top-to-bottom in render). */
  let rankOrder = $derived(flipped ? [7, 6, 5, 4, 3, 2, 1, 0] : [0, 1, 2, 3, 4, 5, 6, 7]);
  let fileOrder = $derived(flipped ? [7, 6, 5, 4, 3, 2, 1, 0] : [0, 1, 2, 3, 4, 5, 6, 7]);

  function isLight(file: number, rank: number): boolean {
    return (file + rank) % 2 === 0;
  }
</script>

{#if parseError}
  <div class="board-error">Invalid FEN: {parseError}</div>
{:else if position}
  <div
    class="board"
    style:--size="{size}px"
    style:width="{size * 8 + (showCoords ? 18 : 0)}px"
    class:with-coords={showCoords}>
    <div class="grid">
      {#each rankOrder as r (r)}
        {#each fileOrder as f (f)}
          {@const sq = coordsToSquare(f, r)}
          {@const piece = position.board[r][f]}
          {@const light = isLight(f, r)}
          {@const isHighlighted = highlights.has(sq)}
          <button
            type="button"
            class="square"
            class:light
            class:dark={!light}
            class:highlight={isHighlighted}
            data-square={sq}
            tabindex={onSquareClick ? 0 : -1}
            disabled={!onSquareClick}
            onclick={() => onSquareClick?.(sq)}>
            {#if piece}
              <span class="piece" class:white={piece === piece.toUpperCase()}>
                {pieceGlyph(piece)}
              </span>
            {/if}
            {#if showCoords && f === fileOrder[0]}
              <span class="rank-label">{8 - r}</span>
            {/if}
            {#if showCoords && r === rankOrder[7]}
              <span class="file-label">{String.fromCharCode(97 + f)}</span>
            {/if}
          </button>
        {/each}
      {/each}
    </div>
  </div>
{/if}

<style>
  .board {
    --light: #ebecd0;
    --dark: #779556;
    --highlight: rgba(255, 230, 100, 0.5);
    --piece-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
    display: inline-block;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(8, var(--size));
    grid-template-rows: repeat(8, var(--size));
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }

  .square {
    position: relative;
    border: 0;
    border-radius: 0;
    padding: 0;
    background: var(--sq-bg);
    cursor: default;
    width: var(--size);
    height: var(--size);
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .square:disabled {
    opacity: 1;
    cursor: default;
  }

  .square.light {
    --sq-bg: var(--light);
  }
  .square.dark {
    --sq-bg: var(--dark);
  }

  .square.highlight::before {
    content: '';
    position: absolute;
    inset: 0;
    background: var(--highlight);
    pointer-events: none;
  }

  .piece {
    font-size: calc(var(--size) * 0.78);
    line-height: 1;
    user-select: none;
    -webkit-user-select: none;
    text-shadow: var(--piece-shadow);
    color: #1a1a1a;
    pointer-events: none;
  }

  .piece.white {
    color: #f9f9f9;
    text-shadow:
      -1px -1px 0 #1a1a1a,
      1px -1px 0 #1a1a1a,
      -1px 1px 0 #1a1a1a,
      1px 1px 0 #1a1a1a,
      0 1px 2px rgba(0, 0, 0, 0.5);
  }

  .rank-label,
  .file-label {
    position: absolute;
    font-size: 9px;
    font-weight: 600;
    pointer-events: none;
    color: var(--text-muted);
    opacity: 0.85;
  }

  .square.light .rank-label,
  .square.light .file-label {
    color: var(--dark);
  }

  .square.dark .rank-label,
  .square.dark .file-label {
    color: var(--light);
  }

  .rank-label {
    top: 2px;
    left: 3px;
  }

  .file-label {
    bottom: 1px;
    right: 4px;
  }

  .board-error {
    padding: var(--space-sm) var(--space-md);
    background: rgba(248, 113, 113, 0.1);
    border: 1px solid var(--danger);
    border-radius: var(--radius-sm);
    color: var(--danger);
    font-size: 0.85rem;
  }
</style>
