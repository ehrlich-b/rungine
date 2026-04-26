<script lang="ts">
  import { onMount } from 'svelte';
  import Nav from './components/Nav.svelte';
  import StatusBar from './components/StatusBar.svelte';
  import Tournaments from './views/Tournaments.svelte';
  import Analyze from './views/Analyze.svelte';
  import Engines from './views/Engines.svelte';
  import Settings from './views/Settings.svelte';
  import { route } from './lib/router';
  import { initTheme } from './lib/theme';

  onMount(() => {
    initTheme();
  });
</script>

<div class="shell">
  <Nav />
  <main>
    {#if $route === 'engines'}
      <Engines />
    {:else if $route === 'analyze'}
      <Analyze />
    {:else if $route === 'settings'}
      <Settings />
    {:else}
      <Tournaments />
    {/if}
  </main>
  <StatusBar />
</div>

<style>
  .shell {
    display: grid;
    grid-template-rows: auto 1fr auto;
    height: 100%;
  }

  main {
    overflow: auto;
    padding: var(--space-lg);
  }
</style>
