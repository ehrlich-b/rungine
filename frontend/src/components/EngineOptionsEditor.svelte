<script lang="ts">
  import { onMount } from 'svelte';
  import { App } from '../lib/wails';
  import type { main } from '../../wailsjs/go/models';

  type Props = {
    engineId: string;
    onSaved?: () => void;
  };

  let { engineId, onSaved }: Props = $props();

  let config = $state<main.EngineOptionConfig | null>(null);
  let working = $state<Record<string, string>>({});
  let saving = $state(false);
  let error = $state<string | null>(null);
  let profiles = $state<main.EngineProfile[]>([]);
  let applyingProfile = $state<string | null>(null);

  async function load() {
    error = null;
    try {
      const [c, ps] = await Promise.all([
        App.GetEngineOptionConfig(engineId),
        App.ListEngineProfiles(engineId),
      ]);
      config = c;
      working = { ...c.values };
      profiles = ps ?? [];
    } catch (e) {
      error = `Load failed: ${e}`;
    }
  }

  async function applyProfile(name: string) {
    applyingProfile = name;
    error = null;
    try {
      const values = await App.ApplyEngineProfile(engineId, name);
      working = { ...(values ?? {}) };
      await load();
      onSaved?.();
    } catch (e) {
      error = `Apply profile failed: ${e}`;
    } finally {
      applyingProfile = null;
    }
  }

  function effectiveValue(def: main.EngineOptionDef): string {
    if (def.name in working) return working[def.name];
    return def.default ?? '';
  }

  function isOverridden(def: main.EngineOptionDef): boolean {
    return def.name in working && working[def.name] !== def.default;
  }

  function setValue(name: string, value: string) {
    working = { ...working, [name]: value };
  }

  function clearOverride(name: string) {
    const next = { ...working };
    delete next[name];
    working = next;
  }

  function applyRecommended(def: main.EngineOptionDef) {
    if (def.recommended) {
      setValue(def.name, def.recommended);
    }
  }

  async function save() {
    saving = true;
    error = null;
    try {
      // Send only overrides; keys missing from `working` are removed by backend.
      const payload: Record<string, string> = {};
      const definedNames = new Set((config?.definitions ?? []).map((d) => d.name));
      for (const k of definedNames) {
        if (k in working) {
          payload[k] = working[k];
        } else {
          payload[k] = ''; // empty deletes
        }
      }
      await App.SetEngineOptionConfig(engineId, payload);
      await load();
      onSaved?.();
    } catch (e) {
      error = `Save failed: ${e}`;
    } finally {
      saving = false;
    }
  }

  function reset() {
    if (config) working = { ...config.values };
  }

  function isDirty(): boolean {
    if (!config) return false;
    const cur = config.values ?? {};
    const keys = new Set([...Object.keys(cur), ...Object.keys(working)]);
    for (const k of keys) {
      if ((cur[k] ?? '') !== (working[k] ?? '')) return true;
    }
    return false;
  }

  let dirty = $derived(isDirty());

  onMount(load);
</script>

<div class="editor">
  {#if error}
    <div class="error">{error}</div>
  {/if}
  {#if !config}
    <p class="muted">Loading…</p>
  {:else if config.definitions.length === 0}
    <p class="muted">No documented options for this engine.</p>
  {:else}
    {#if profiles.length > 0}
      <div class="profiles">
        <span class="muted small">Profiles:</span>
        {#each profiles as p (p.name)}
          <button
            type="button"
            class="profile-btn"
            disabled={saving || applyingProfile !== null}
            onclick={() => applyProfile(p.name)}>
            {applyingProfile === p.name ? `Applying ${p.name}…` : p.name}
          </button>
        {/each}
      </div>
    {/if}
    <div class="grid">
      {#each config.definitions as def (def.name)}
        {@const v = effectiveValue(def)}
        {@const overridden = isOverridden(def)}
        <div class="opt" class:on={overridden}>
          <div class="opt-head">
            <strong>{def.name}</strong>
            <span class="muted small">{def.type}</span>
          </div>
          {#if def.description}
            <p class="desc muted">{def.description}</p>
          {/if}
          <div class="opt-input">
            {#if def.type === 'spin'}
              <input
                type="number"
                value={v}
                min={def.min}
                max={def.max}
                oninput={(e) =>
                  setValue(def.name, (e.currentTarget as HTMLInputElement).value)} />
            {:else if def.type === 'check'}
              <label class="check">
                <input
                  type="checkbox"
                  checked={v === 'true'}
                  onchange={(e) =>
                    setValue(
                      def.name,
                      (e.currentTarget as HTMLInputElement).checked ? 'true' : 'false',
                    )} />
                <span>{v === 'true' ? 'On' : 'Off'}</span>
              </label>
            {:else if def.type === 'combo' && def.vars && def.vars.length > 0}
              <select
                value={v}
                onchange={(e) =>
                  setValue(def.name, (e.currentTarget as HTMLSelectElement).value)}>
                {#each def.vars as opt (opt)}
                  <option value={opt}>{opt}</option>
                {/each}
              </select>
            {:else if def.type === 'button'}
              <span class="muted small">button (no value)</span>
            {:else}
              <input
                type="text"
                value={v}
                oninput={(e) =>
                  setValue(def.name, (e.currentTarget as HTMLInputElement).value)} />
            {/if}
          </div>
          <div class="opt-meta">
            <span class="muted small">default: {def.default || '—'}</span>
            {#if def.recommended}
              <button
                type="button"
                class="link-btn"
                onclick={() => applyRecommended(def)}
                disabled={v === def.recommended}>
                use recommended ({def.recommended})
              </button>
            {/if}
            {#if overridden}
              <button
                type="button"
                class="link-btn"
                onclick={() => clearOverride(def.name)}>
                clear override
              </button>
            {/if}
          </div>
          {#if def.min !== undefined && def.max !== undefined}
            <span class="muted small">range: {def.min}–{def.max}</span>
          {/if}
        </div>
      {/each}
    </div>
    <div class="actions">
      <button onclick={reset} disabled={!dirty || saving}>Reset</button>
      <button class="primary" onclick={save} disabled={!dirty || saving}>
        {saving ? 'Saving…' : 'Save'}
      </button>
    </div>
  {/if}
</div>

<style>
  .editor {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
    padding: var(--space-md) 0;
    border-top: 1px solid var(--border);
    margin-top: var(--space-sm);
  }

  .error {
    background: rgba(248, 113, 113, 0.1);
    border: 1px solid var(--danger);
    color: var(--danger);
    padding: var(--space-xs) var(--space-sm);
    border-radius: var(--radius-sm);
    font-size: 0.85rem;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--space-sm);
  }

  .opt {
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: var(--space-sm);
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .opt.on {
    border-color: var(--accent);
  }

  .opt-head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
  }

  .desc {
    margin: 0;
    font-size: 0.8rem;
    line-height: 1.3;
  }

  .opt-input {
    margin-top: 4px;
  }

  .opt-input input,
  .opt-input select {
    width: 100%;
    background: var(--surface-1);
  }

  .check {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    font-size: 0.85rem;
  }

  .opt-meta {
    display: flex;
    gap: var(--space-sm);
    flex-wrap: wrap;
    align-items: baseline;
  }

  .small {
    font-size: 0.7rem;
  }

  .link-btn {
    background: transparent;
    border: none;
    color: var(--accent);
    padding: 0;
    font-size: 0.75rem;
    cursor: pointer;
    text-decoration: underline;
  }

  .link-btn:disabled {
    color: var(--text-muted);
    text-decoration: none;
    cursor: default;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-sm);
  }

  .profiles {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    flex-wrap: wrap;
  }

  .profile-btn {
    background: var(--surface-2);
    border: 1px solid var(--border);
    color: var(--text-primary);
    padding: 2px 10px;
    border-radius: 999px;
    font-size: 0.75rem;
    cursor: pointer;
    text-transform: capitalize;
  }

  .profile-btn:hover:not(:disabled) {
    border-color: var(--accent);
    color: var(--accent);
  }

  .profile-btn:disabled {
    opacity: 0.5;
    cursor: default;
  }
</style>
