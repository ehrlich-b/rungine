<script lang="ts">
  import { navigate } from '../lib/router';
  import Board from '../components/Board.svelte';
  import { STARTING_FEN } from '../lib/chess';

  let fen = $state(STARTING_FEN);
  let flipped = $state(false);
  let lastMove = $state<string | null>(null);

  function reset() {
    fen = STARTING_FEN;
    lastMove = null;
  }
</script>

<section class="page">
  <header>
    <h1>Tournaments</h1>
    <button class="primary" disabled title="Tournament setup wizard coming next">
      New tournament
    </button>
  </header>

  <div class="empty">
    <h2>No tournaments yet</h2>
    <p class="subtle">
      The tournament backend (match, round-robin, gauntlet, Swiss, SPRT) is wired up
      and runs from the CLI today: <code>rungine-tournament</code>. The in-app setup
      wizard is the next slice.
    </p>
    <p class="subtle">
      Install some engines first so they show up in the tournament picker.
    </p>
    <button onclick={() => navigate('engines')}>Go to Engines</button>
  </div>

  <div class="preview">
    <h2>Board preview</h2>
    <p class="subtle">
      Chessboard component preview — paste a FEN to test rendering. Used by the
      tournament viewer when games start streaming in.
    </p>
    <div class="preview-body">
      <Board {fen} {flipped} {lastMove} size={48} />
      <div class="preview-controls">
        <label>
          <span class="label">FEN</span>
          <input bind:value={fen} spellcheck="false" />
        </label>
        <label>
          <span class="label">Last move (UCI)</span>
          <input
            placeholder="e2e4"
            value={lastMove ?? ''}
            oninput={(e) => (lastMove = (e.currentTarget as HTMLInputElement).value || null)}
            spellcheck="false" />
        </label>
        <div class="row">
          <button onclick={() => (flipped = !flipped)}>
            {flipped ? 'Unflip' : 'Flip'}
          </button>
          <button onclick={reset}>Reset</button>
        </div>
      </div>
    </div>
  </div>
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

  h2 {
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text-secondary);
    font-weight: 500;
    margin-bottom: var(--space-sm);
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
  }

  code {
    background: var(--surface-2);
    padding: 1px 6px;
    border-radius: var(--radius-sm);
    font-size: 0.85em;
  }

  .preview {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-md) var(--space-lg);
    max-width: 800px;
  }

  .preview-body {
    display: flex;
    gap: var(--space-lg);
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .preview-controls {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
    min-width: 280px;
    flex: 1;
  }

  .preview-controls label {
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

  .preview-controls input {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.8rem;
  }

  .row {
    display: flex;
    gap: var(--space-sm);
    margin-top: var(--space-xs);
  }
</style>
