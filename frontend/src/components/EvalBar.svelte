<script lang="ts">
  // Vertical evaluation bar (Lichess-style). cp/mate are WHITE-POV scores;
  // the white share fills from the bottom (or top when flipped).
  type Props = {
    cp?: number | null;
    mate?: number | null;
    flipped?: boolean;
  };

  let { cp = null, mate = null, flipped = false }: Props = $props();

  // White's share of the bar, 0..1. Uses a logistic map of centipawns to a
  // win-probability-like fraction, clamped so neither side fully vanishes.
  let whiteFraction = $derived.by<number>(() => {
    if (mate !== null && mate !== undefined) return mate > 0 ? 1 : 0;
    if (cp === null || cp === undefined) return 0.5;
    const f = 1 / (1 + Math.pow(10, -cp / 400));
    return Math.max(0.02, Math.min(0.98, f));
  });

  let label = $derived.by<string>(() => {
    if (mate !== null && mate !== undefined) return mate > 0 ? `M${mate}` : `-M${-mate}`;
    if (cp === null || cp === undefined) return '';
    const p = cp / 100;
    return p >= 0 ? `+${p.toFixed(1)}` : p.toFixed(1);
  });

  // Whether white is at least even — controls which end shows the label.
  let whiteAhead = $derived(whiteFraction >= 0.5);
</script>

<div class="evalbar" class:flipped title={label}>
  <div class="white" style:flex-basis={`${whiteFraction * 100}%`}></div>
  <div class="black" style:flex-basis={`${(1 - whiteFraction) * 100}%`}></div>
  {#if label}
    <span class="label" class:on-white={whiteAhead} class:on-black={!whiteAhead}>{label}</span>
  {/if}
</div>

<style>
  .evalbar {
    position: relative;
    width: 20px;
    height: 100%;
    display: flex;
    flex-direction: column-reverse; /* white at the bottom */
    border-radius: var(--radius-sm);
    overflow: hidden;
    border: 1px solid var(--border);
    background: #3a3a3a;
  }

  .evalbar.flipped {
    flex-direction: column; /* white at the top when the board is flipped */
  }

  .white {
    background: #ededed;
    transition: flex-basis 200ms ease;
  }

  .black {
    background: #2b2b2b;
    transition: flex-basis 200ms ease;
  }

  .label {
    position: absolute;
    left: 0;
    right: 0;
    text-align: center;
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.6rem;
    font-weight: 600;
    line-height: 1;
    padding: 2px 0;
    pointer-events: none;
  }

  /* Label sits at white's end, in contrasting ink. */
  .label.on-white {
    bottom: 2px;
    color: #1a1a1a;
  }
  .label.on-black {
    top: 2px;
    color: #ededed;
  }

  .evalbar.flipped .label.on-white {
    bottom: auto;
    top: 2px;
  }
  .evalbar.flipped .label.on-black {
    top: auto;
    bottom: 2px;
  }
</style>
