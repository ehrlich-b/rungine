<script lang="ts">
  import { onMount } from 'svelte';
  import { App } from '../lib/wails';

  let cpuFeatures = $state('');
  let engineCount = $state(0);

  async function refresh() {
    try {
      cpuFeatures = await App.GetCPUFeatures();
      const installed = await App.ListInstalledEngines();
      engineCount = installed?.length ?? 0;
    } catch {
      cpuFeatures = 'unavailable';
    }
  }

  onMount(() => {
    refresh();
  });
</script>

<footer>
  <span class="muted">CPU: {cpuFeatures || '—'}</span>
  <span class="sep">·</span>
  <span class="muted">{engineCount} engine{engineCount === 1 ? '' : 's'} installed</span>
</footer>

<style>
  footer {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    padding: 4px var(--space-lg);
    background: var(--surface-1);
    border-top: 1px solid var(--border);
    font-size: 0.78rem;
    color: var(--text-secondary);
    height: 24px;
  }

  .sep {
    color: var(--text-muted);
  }
</style>
