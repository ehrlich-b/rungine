<script lang="ts">
  import { onMount } from 'svelte';
  import { theme, setTheme } from '../lib/theme';
  import { App } from '../lib/wails';

  let cpuFeatures = $state('');

  onMount(async () => {
    try {
      cpuFeatures = await App.GetCPUFeatures();
    } catch {
      cpuFeatures = 'unavailable';
    }
  });
</script>

<section class="page">
  <h1>Settings</h1>

  <div class="group">
    <h2>Appearance</h2>
    <div class="row">
      <span class="label">Theme</span>
      <div class="seg" role="group" aria-label="Theme">
        <button class:active={$theme === 'dark'} onclick={() => setTheme('dark')}>
          Dark
        </button>
        <button class:active={$theme === 'light'} onclick={() => setTheme('light')}>
          Light
        </button>
      </div>
    </div>
  </div>

  <div class="group">
    <h2>System</h2>
    <div class="row">
      <span class="label">CPU features</span>
      <code>{cpuFeatures || '—'}</code>
    </div>
  </div>
</section>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-lg);
    max-width: 640px;
  }

  .group {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-md) var(--space-lg);
  }

  h2 {
    margin-bottom: var(--space-md);
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text-secondary);
    font-weight: 500;
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-sm) 0;
  }

  .label {
    color: var(--text-secondary);
  }

  .seg {
    display: flex;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }

  .seg button {
    border: none;
    border-radius: 0;
    background: transparent;
    color: var(--text-secondary);
    padding: 4px 14px;
    font-size: 0.85rem;
  }

  .seg button.active {
    background: var(--surface-2);
    color: var(--text-primary);
  }

  code {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.85rem;
    color: var(--text-secondary);
  }
</style>
