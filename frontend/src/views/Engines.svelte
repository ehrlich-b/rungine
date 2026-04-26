<script lang="ts">
  import { onMount } from 'svelte';
  import { App, on } from '../lib/wails';
  import EngineOptionsEditor from '../components/EngineOptionsEditor.svelte';
  import type { registry } from '../../wailsjs/go/models';

  type Available = registry.EngineInfo;
  type Installed = registry.InstalledEngine;

  let available = $state<Available[]>([]);
  let installed = $state<Installed[]>([]);
  let busyId = $state<string | null>(null);
  let error = $state<string | null>(null);
  let progress = $state<Record<string, string>>({});
  let configuring = $state<string | null>(null);
  let addingCustom = $state(false);
  let customName = $state('');

  async function refresh() {
    error = null;
    try {
      const [a, i] = await Promise.all([
        App.ListAvailableEngines(),
        App.ListInstalledEngines(),
      ]);
      available = a ?? [];
      installed = i ?? [];
    } catch (e) {
      error = `Failed to load engines: ${e}`;
    }
  }

  function isInstalled(id: string): boolean {
    return installed.some((e) => e.RegistryID === id || e.ID === id);
  }

  async function install(id: string) {
    busyId = id;
    progress[id] = 'Installing…';
    try {
      await App.InstallEngine(id);
      await refresh();
      delete progress[id];
    } catch (e) {
      error = `Install failed: ${e}`;
      progress[id] = 'Failed';
    } finally {
      busyId = null;
    }
  }

  async function uninstall(id: string) {
    busyId = id;
    try {
      await App.UninstallEngine(id);
      await refresh();
    } catch (e) {
      error = `Uninstall failed: ${e}`;
    } finally {
      busyId = null;
    }
  }

  async function addCustom() {
    error = null;
    addingCustom = true;
    try {
      const path = await App.PickEngineBinary();
      if (!path) {
        addingCustom = false;
        return;
      }
      await App.AddCustomEngine(path, customName.trim());
      customName = '';
      await refresh();
    } catch (e) {
      error = `Add custom engine failed: ${e}`;
    } finally {
      addingCustom = false;
    }
  }

  onMount(() => {
    refresh();

    const offDownload = on<{ engineID: string; downloaded: number; total: number }>(
      'download:progress',
      (p) => {
        if (!p) return;
        const pct = p.total > 0 ? Math.round((p.downloaded / p.total) * 100) : 0;
        progress[p.engineID] = `Downloading ${pct}%`;
      },
    );
    const offInstall = on<{ engineID: string; stage: string }>('install:progress', (p) => {
      if (!p) return;
      progress[p.engineID] = p.stage;
    });

    return () => {
      offDownload();
      offInstall();
    };
  });
</script>

<section class="page">
  <header>
    <h1>Engines</h1>
    <button onclick={refresh} disabled={busyId !== null}>Refresh</button>
  </header>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  <h2>Installed</h2>
  <div class="custom-row">
    <input
      type="text"
      class="custom-name"
      placeholder="Display name (optional)"
      bind:value={customName}
      spellcheck="false" />
    <button
      onclick={addCustom}
      disabled={busyId !== null || addingCustom}>
      {addingCustom ? 'Adding…' : 'Add custom engine'}
    </button>
  </div>

  {#if installed.length === 0}
    <p class="subtle">No engines installed yet — install one below to get started.</p>
  {:else}
    <div class="grid">
      {#each installed as eng (eng.ID)}
        <article class="card" class:expanded={configuring === eng.ID}>
          <div class="card-head">
            <strong>{eng.Name}</strong>
            <span class="muted">{eng.Version}</span>
          </div>
          <div class="path muted">{eng.BinaryPath}</div>
          {#if eng.OptionValues && Object.keys(eng.OptionValues).length > 0}
            <div class="overrides muted small">
              {Object.keys(eng.OptionValues).length} option override(s)
            </div>
          {/if}
          <div class="actions">
            <button
              disabled={busyId !== null}
              onclick={() => (configuring = configuring === eng.ID ? null : eng.ID)}>
              {configuring === eng.ID ? 'Close' : 'Configure'}
            </button>
            <button
              class="danger"
              disabled={busyId !== null}
              onclick={() => uninstall(eng.ID)}>
              Uninstall
            </button>
          </div>
          {#if configuring === eng.ID}
            <EngineOptionsEditor engineId={eng.ID} onSaved={refresh} />
          {/if}
        </article>
      {/each}
    </div>
  {/if}

  <h2>Available</h2>
  {#if available.length === 0}
    <p class="subtle">Registry is empty — check the embedded TOML.</p>
  {:else}
    <div class="grid">
      {#each available as eng (eng.id)}
        {@const installedAlready = isInstalled(eng.id)}
        {@const status = progress[eng.id]}
        <article class="card" class:dim={!eng.hasBuild}>
          <div class="card-head">
            <strong>{eng.name}</strong>
            <span class="muted">{eng.version}</span>
          </div>
          <p class="desc">{eng.description}</p>
          <div class="meta">
            <span class="badge">~{eng.eloEstimate} Elo</span>
            {#if eng.requiresNetwork}
              <span class="badge">network file</span>
            {/if}
            {#if !eng.hasBuild}
              <span class="badge danger">no build for this CPU</span>
            {/if}
          </div>
          <div class="actions">
            {#if installedAlready}
              <span class="muted">Installed</span>
            {:else if status}
              <span class="muted">{status}</span>
            {:else}
              <button
                class="primary"
                disabled={busyId !== null || !eng.hasBuild}
                onclick={() => install(eng.id)}>
                Install
              </button>
            {/if}
          </div>
        </article>
      {/each}
    </div>
  {/if}
</section>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
    height: 100%;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  h2 {
    margin-top: var(--space-md);
    color: var(--text-secondary);
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    font-weight: 500;
  }

  .error {
    background: rgba(248, 113, 113, 0.1);
    border: 1px solid var(--danger);
    color: var(--danger);
    padding: var(--space-sm) var(--space-md);
    border-radius: var(--radius-sm);
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: var(--space-md);
  }

  .card {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-md);
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }

  .card.dim {
    opacity: 0.7;
  }

  .card.expanded {
    grid-column: 1 / -1;
  }

  .overrides {
    font-style: italic;
  }

  .small {
    font-size: 0.7rem;
  }

  .card-head {
    display: flex;
    align-items: baseline;
    gap: var(--space-sm);
  }

  .desc {
    margin: 0;
    color: var(--text-secondary);
    font-size: 0.85rem;
    line-height: 1.4;
  }

  .meta {
    display: flex;
    gap: var(--space-xs);
    flex-wrap: wrap;
  }

  .badge {
    background: var(--surface-2);
    color: var(--text-secondary);
    padding: 2px 8px;
    border-radius: 999px;
    font-size: 0.75rem;
    border: 1px solid var(--border);
  }

  .badge.danger {
    background: rgba(248, 113, 113, 0.1);
    border-color: var(--danger);
    color: var(--danger);
  }

  .path {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.7rem;
    word-break: break-all;
  }

  .actions {
    margin-top: auto;
    display: flex;
    align-items: center;
    justify-content: flex-end;
  }

  .custom-row {
    display: flex;
    gap: var(--space-sm);
    margin-bottom: var(--space-sm);
  }

  .custom-name {
    flex: 1;
    min-width: 0;
  }
</style>
