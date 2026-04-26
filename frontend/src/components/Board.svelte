<script lang="ts">
  import {
    STARTING_FEN,
    parseFEN,
    pieceGlyph,
    coordsToSquare,
    type Position,
  } from '../lib/chess';

  type Arrow = {
    /** Source square in UCI notation, e.g. "e2". */
    from: string;
    /** Destination square in UCI notation, e.g. "e4". */
    to: string;
    /** CSS color string. Defaults to the accent color. */
    color?: string;
    /** Multiplier on the default stroke width. */
    weight?: number;
  };

  type Props = {
    fen?: string;
    flipped?: boolean;
    showCoords?: boolean;
    /** Last move in UCI notation, e.g. "e2e4". Highlights from + to squares. */
    lastMove?: string | null;
    /** Square size in pixels. */
    size?: number;
    /** Arrow overlays (engine PV, user annotations). */
    arrows?: Arrow[];
    /** Square holding the king of the side currently in check, e.g. "e1". */
    checkSquare?: string | null;
    onSquareClick?: (square: string) => void;
  };

  let {
    fen = STARTING_FEN,
    flipped = false,
    showCoords = true,
    lastMove = null,
    size = 56,
    arrows = [],
    checkSquare = null,
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

  /** Returns the pixel center of square `sq` inside the 8*size grid, accounting for flip. */
  function squareCenter(sq: string): { x: number; y: number } | null {
    if (!sq || sq.length < 2) return null;
    const f = sq.charCodeAt(0) - 97;
    const rankNum = parseInt(sq[1], 10);
    if (f < 0 || f > 7 || isNaN(rankNum) || rankNum < 1 || rankNum > 8) return null;
    // board[r][f] where r=0 means rank 8 (FEN order).
    const boardR = 8 - rankNum;
    const renderRow = flipped ? 7 - boardR : boardR;
    const renderCol = flipped ? 7 - f : f;
    return {
      x: renderCol * size + size / 2,
      y: renderRow * size + size / 2,
    };
  }

  type Geom = { x1: number; y1: number; x2: number; y2: number; color: string; weight: number };

  function arrowGeometry(): Geom[] {
    const out: Geom[] = [];
    for (const a of arrows ?? []) {
      const from = squareCenter(a.from);
      const to = squareCenter(a.to);
      if (!from || !to) continue;
      // Shorten destination so the arrowhead lands inside the square, not on its edge.
      const dx = to.x - from.x;
      const dy = to.y - from.y;
      const dist = Math.hypot(dx, dy) || 1;
      const shrink = size * 0.25;
      const tx = to.x - (dx / dist) * shrink;
      const ty = to.y - (dy / dist) * shrink;
      out.push({
        x1: from.x, y1: from.y, x2: tx, y2: ty,
        color: a.color ?? 'var(--accent)',
        weight: a.weight ?? 1,
      });
    }
    return out;
  }

  let arrowGeom = $derived(arrowGeometry());
  let overlaySize = $derived(size * 8);
  let arrowStroke = $derived(Math.max(3, size * 0.12));
  let arrowHead = $derived(arrowStroke * 2.6);
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
          {@const isCheck = checkSquare === sq}
          <button
            type="button"
            class="square"
            class:light
            class:dark={!light}
            class:highlight={isHighlighted}
            class:check={isCheck}
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
      {#if arrowGeom.length > 0}
        <svg
          class="arrows"
          viewBox="0 0 {overlaySize} {overlaySize}"
          width={overlaySize}
          height={overlaySize}
          aria-hidden="true">
          {#each arrowGeom as g, i (i)}
            {@const headLen = arrowHead * g.weight}
            {@const dx = g.x2 - g.x1}
            {@const dy = g.y2 - g.y1}
            {@const dist = Math.hypot(dx, dy) || 1}
            {@const ux = dx / dist}
            {@const uy = dy / dist}
            {@const lineEndX = g.x2 - ux * headLen * 0.6}
            {@const lineEndY = g.y2 - uy * headLen * 0.6}
            {@const headBaseX = g.x2 - ux * headLen}
            {@const headBaseY = g.y2 - uy * headLen}
            {@const perpX = -uy * headLen * 0.45}
            {@const perpY = ux * headLen * 0.45}
            <g style="color: {g.color}">
              <line
                x1={g.x1}
                y1={g.y1}
                x2={lineEndX}
                y2={lineEndY}
                stroke="currentColor"
                stroke-width={arrowStroke * g.weight}
                stroke-linecap="round" />
              <polygon
                points="{g.x2},{g.y2} {headBaseX + perpX},{headBaseY + perpY} {headBaseX - perpX},{headBaseY - perpY}"
                fill="currentColor" />
            </g>
          {/each}
        </svg>
      {/if}
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
    position: relative;
    display: grid;
    grid-template-columns: repeat(8, var(--size));
    grid-template-rows: repeat(8, var(--size));
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }

  .arrows {
    position: absolute;
    top: 0;
    left: 0;
    pointer-events: none;
    opacity: 0.85;
    filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.45));
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

  .square.check::after {
    content: '';
    position: absolute;
    inset: 0;
    background: radial-gradient(
      circle at center,
      rgba(220, 38, 38, 0.85) 0%,
      rgba(220, 38, 38, 0.5) 35%,
      rgba(220, 38, 38, 0) 70%
    );
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
