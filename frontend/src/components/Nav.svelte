<script lang="ts">
  import { route, navigate, type Route } from '../lib/router';

  const items: { id: Route; label: string }[] = [
    { id: 'tournaments', label: 'Tournaments' },
    { id: 'analyze', label: 'Analyze' },
    { id: 'engines', label: 'Engines' },
    { id: 'settings', label: 'Settings' },
  ];
</script>

<nav>
  <div class="brand">Rungine</div>
  <div class="links">
    {#each items as item (item.id)}
      <button
        class="link"
        class:active={$route === item.id}
        onclick={() => navigate(item.id)}>
        {item.label}
      </button>
    {/each}
  </div>
</nav>

<style>
  nav {
    display: flex;
    align-items: center;
    gap: var(--space-lg);
    padding: var(--space-sm) var(--space-lg);
    background: var(--surface-1);
    border-bottom: 1px solid var(--border);
    height: 44px;
  }

  .brand {
    font-weight: 600;
    font-size: 1.05rem;
    letter-spacing: 0.5px;
    color: var(--accent);
  }

  .links {
    display: flex;
    gap: var(--space-xs);
  }

  .link {
    background: transparent;
    border: 1px solid transparent;
    color: var(--text-secondary);
    padding: 4px 12px;
    border-radius: var(--radius-sm);
    font-size: 0.9rem;
  }

  .link:hover:not(:disabled) {
    background: var(--surface-2);
    color: var(--text-primary);
    border-color: transparent;
  }

  .link.active {
    background: var(--surface-2);
    color: var(--text-primary);
  }
</style>
