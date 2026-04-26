<script lang="ts">
  import { navigate } from '../lib/router';
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
</section>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-lg);
    height: 100%;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
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

  code {
    background: var(--surface-2);
    padding: 1px 6px;
    border-radius: var(--radius-sm);
    font-size: 0.85em;
  }
</style>
