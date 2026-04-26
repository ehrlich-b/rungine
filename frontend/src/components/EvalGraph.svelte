<script lang="ts">
  import type { main } from '../../wailsjs/go/models';

  type Props = {
    moves: main.MoveDetail[];
    currentPly: number;
    onJump?: (ply: number) => void;
    width?: number;
    height?: number;
  };

  let { moves, currentPly, onJump, width = 360, height = 80 }: Props = $props();

  // Eval domain in pawn units. Mate scores clip to ±maxPawns.
  const maxPawns = 10;

  // Centerline at vertical mid; +pawns = up.
  function evalToPawns(m: main.MoveDetail): number {
    if (m.evalMate !== undefined && m.evalMate !== null) {
      return m.evalMate > 0 ? maxPawns : -maxPawns;
    }
    if (m.evalCp !== undefined && m.evalCp !== null) {
      let p = m.evalCp / 100;
      if (m.side === 'b') p = -p; // engine reports from side-to-move POV
      if (p > maxPawns) return maxPawns;
      if (p < -maxPawns) return -maxPawns;
      return p;
    }
    return 0;
  }

  function whitePOV(m: main.MoveDetail): number {
    return evalToPawns(m);
  }

  let pad = 4;

  function xFor(ply: number): number {
    if (moves.length <= 1) return pad;
    const t = (ply - 1) / Math.max(1, moves.length - 1);
    return pad + t * (width - 2 * pad);
  }

  function yFor(p: number): number {
    const t = (p + maxPawns) / (2 * maxPawns); // 0..1
    return height - pad - t * (height - 2 * pad);
  }

  function points(): { ply: number; x: number; y: number; pawns: number }[] {
    return moves.map((m, i) => {
      const ply = i + 1;
      const pawns = whitePOV(m);
      return { ply, x: xFor(ply), y: yFor(pawns), pawns };
    });
  }

  let pts = $derived(points());
  let zeroY = $derived(yFor(0));

  function pathD(): string {
    if (pts.length === 0) return '';
    return pts
      .map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`)
      .join(' ');
  }

  function fillD(): string {
    if (pts.length === 0) return '';
    const first = pts[0];
    const last = pts[pts.length - 1];
    return `M ${first.x} ${zeroY} L ${pathD().replace(/^M /, '')} L ${last.x} ${zeroY} Z`;
  }

  function pickAtX(x: number) {
    if (!onJump || pts.length === 0) return;
    let bestPly = pts[0].ply;
    let bestDist = Infinity;
    for (const p of pts) {
      const d = Math.abs(p.x - x);
      if (d < bestDist) {
        bestDist = d;
        bestPly = p.ply;
      }
    }
    onJump(bestPly);
  }

  function nearestPly(e: MouseEvent) {
    const rect = (e.currentTarget as SVGElement).getBoundingClientRect();
    const x = ((e.clientX - rect.left) / rect.width) * width;
    pickAtX(x);
  }

  function keynav(e: KeyboardEvent) {
    if (!onJump) return;
    if (e.key === 'ArrowLeft') {
      e.preventDefault();
      onJump(Math.max(1, currentPly - 1));
    } else if (e.key === 'ArrowRight') {
      e.preventDefault();
      onJump(Math.min(moves.length, currentPly + 1));
    }
  }

  let currentX = $derived(currentPly > 0 && currentPly <= moves.length ? pts[currentPly - 1]?.x : null);
</script>

{#if moves.length > 0}
  <svg
    class="graph"
    {width}
    {height}
    viewBox="0 0 {width} {height}"
    preserveAspectRatio="none"
    role="slider"
    tabindex="0"
    aria-label="Evaluation graph"
    aria-valuemin="1"
    aria-valuemax={moves.length}
    aria-valuenow={currentPly}
    onclick={nearestPly}
    onkeydown={keynav}>
    <!-- Centerline -->
    <line
      x1={pad}
      y1={zeroY}
      x2={width - pad}
      y2={zeroY}
      class="zero" />

    <!-- White advantage fill (above zero line) -->
    <defs>
      <clipPath id="white-clip">
        <rect x={0} y={0} width={width} height={zeroY} />
      </clipPath>
      <clipPath id="black-clip">
        <rect x={0} y={zeroY} width={width} height={height - zeroY} />
      </clipPath>
    </defs>
    <path d={fillD()} class="fill white" clip-path="url(#white-clip)" />
    <path d={fillD()} class="fill black" clip-path="url(#black-clip)" />

    <!-- Trace line -->
    <path d={pathD()} class="trace" />

    <!-- Current ply marker -->
    {#if currentX !== null && currentX !== undefined}
      <line
        x1={currentX}
        y1={pad}
        x2={currentX}
        y2={height - pad}
        class="cursor" />
    {/if}
  </svg>
{/if}

<style>
  .graph {
    display: block;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    cursor: crosshair;
    width: 100%;
  }

  .zero {
    stroke: var(--text-muted);
    stroke-width: 1;
    stroke-dasharray: 2 2;
    opacity: 0.5;
  }

  .trace {
    fill: none;
    stroke: var(--text-primary);
    stroke-width: 1.4;
    stroke-linejoin: round;
  }

  .fill.white {
    fill: rgba(235, 236, 208, 0.18);
  }

  .fill.black {
    fill: rgba(34, 41, 53, 0.6);
  }

  .cursor {
    stroke: var(--accent);
    stroke-width: 1;
    pointer-events: none;
  }
</style>
