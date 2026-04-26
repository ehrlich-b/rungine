<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { App, on } from '../lib/wails';
  import Board from '../components/Board.svelte';
  import { STARTING_FEN } from '../lib/chess';
  import type { registry } from '../../wailsjs/go/models';

  type EngineState = {
    id: string;
    name: string;
    depth: number;
    seldepth: number;
    evalCp: number | null;
    evalMate: number | null;
    nodes: number;
    nps: number;
    timeMs: number;
    pv: string[];
    pvSan: string;
  };

  let installed = $state<registry.InstalledEngine[]>([]);
  let selected = $state<string[]>([]);
  let fen = $state(STARTING_FEN);
  let analyzing = $state(false);
  let error = $state<string | null>(null);
  let states = $state<Record<string, EngineState>>({});
  let unsubs: Array<() => void> = [];

  async function refresh() {
    try {
      installed = (await App.ListInstalledEngines()) ?? [];
    } catch (e) {
      error = `Refresh failed: ${e}`;
    }
  }

  function toggle(id: string) {
    if (selected.includes(id)) selected = selected.filter((x) => x !== id);
    else selected = [...selected, id];
  }

  async function start() {
    error = null;
    if (selected.length === 0) {
      error = 'Select at least one engine.';
      return;
    }
    try {
      // Reset panels
      states = {};
      for (const id of selected) {
        const eng = installed.find((e) => e.ID === id);
        if (!eng) continue;
        states[id] = {
          id,
          name: eng.Name,
          depth: 0,
          seldepth: 0,
          evalCp: null,
          evalMate: null,
          nodes: 0,
          nps: 0,
          timeMs: 0,
          pv: [],
          pvSan: '',
        };
      }
      states = { ...states };

      // Start each engine process; ignore "already running" errors.
      for (const id of selected) {
        try {
          await App.StartEngine(id);
        } catch {
          // Already started — proceed.
        }
      }
      await App.StartAnalysis({
        fen,
        moves: [],
        engineIds: selected,
        infinite: true,
        depth: 0,
        moveTime: 0,
      } as any);
      analyzing = true;
    } catch (e) {
      error = `Start failed: ${e}`;
    }
  }

  async function stop() {
    if (!analyzing) return;
    try {
      await App.StopAnalysis(selected);
    } catch (e) {
      error = `Stop failed: ${e}`;
    } finally {
      analyzing = false;
    }
  }

  function fmtEval(s: EngineState): string {
    if (s.evalMate !== null) {
      return s.evalMate > 0 ? `M${s.evalMate}` : `-M${-s.evalMate}`;
    }
    if (s.evalCp !== null) {
      const cp = s.evalCp / 100;
      return cp >= 0 ? `+${cp.toFixed(2)}` : cp.toFixed(2);
    }
    return '—';
  }

  function fmtNumber(n: number): string {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(0)}k`;
    return n.toString();
  }

  onMount(() => {
    refresh();
    unsubs.push(
      on<any>('analysis:info', (info) => {
        if (!info || !info.EngineID) return;
        const id = info.EngineID;
        const cur = states[id];
        if (!cur) return;
        const score = info.Score ?? {};
        states = {
          ...states,
          [id]: {
            ...cur,
            depth: info.Depth ?? cur.depth,
            seldepth: info.SelDepth ?? cur.seldepth,
            evalCp: score.Centipawns ?? null,
            evalMate: score.Mate ?? null,
            nodes: info.Nodes ?? cur.nodes,
            nps: info.NPS ?? cur.nps,
            timeMs: info.Time ? Math.round(info.Time / 1_000_000) : cur.timeMs,
            pv: info.PV ?? cur.pv,
            pvSan: (info.PV ?? cur.pv).slice(0, 12).join(' '),
          },
        };
      }),
    );
  });

  onDestroy(() => {
    unsubs.forEach((u) => u());
    if (analyzing && selected.length > 0) {
      App.StopAnalysis(selected).catch(() => {});
    }
  });
</script>

<section class="page">
  <header>
    <h1>Analyze</h1>
    {#if analyzing}
      <button class="danger" onclick={stop}>Stop</button>
    {:else}
      <button class="primary" onclick={start} disabled={selected.length === 0}>
        Start analysis
      </button>
    {/if}
  </header>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  <div class="layout">
    <div class="left">
      <Board {fen} size={48} />
      <label class="block">
        <span class="label">FEN</span>
        <input bind:value={fen} spellcheck="false" disabled={analyzing} />
      </label>
      <button onclick={() => (fen = STARTING_FEN)} disabled={analyzing}>
        Reset to startpos
      </button>

      <h3>Engines</h3>
      {#if installed.length === 0}
        <p class="muted">Install engines first from the Engines tab.</p>
      {:else}
        <div class="engines">
          {#each installed as eng (eng.ID)}
            {@const checked = selected.includes(eng.ID)}
            <label class="check" class:on={checked}>
              <input
                type="checkbox"
                {checked}
                disabled={analyzing}
                onchange={() => toggle(eng.ID)} />
              <span>{eng.Name}</span>
              <span class="muted">{eng.Version}</span>
            </label>
          {/each}
        </div>
      {/if}
    </div>

    <div class="panels">
      {#if Object.keys(states).length === 0}
        <div class="hint-card muted">
          Pick one or more engines and click Start to see their analysis stream
          live here.
        </div>
      {:else}
        {#each Object.values(states) as s (s.id)}
          <div class="panel">
            <div class="panel-head">
              <strong>{s.name}</strong>
              <span class="eval">{fmtEval(s)}</span>
              <span class="muted">depth {s.depth}/{s.seldepth || '—'}</span>
            </div>
            <div class="panel-meta">
              <span class="muted">{fmtNumber(s.nodes)} nodes</span>
              <span class="muted">{fmtNumber(s.nps)} nps</span>
              <span class="muted">{(s.timeMs / 1000).toFixed(1)}s</span>
            </div>
            {#if s.pvSan}
              <div class="pv">{s.pvSan}</div>
            {/if}
          </div>
        {/each}
      {/if}
    </div>
  </div>
</section>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
  }

  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .layout {
    display: grid;
    grid-template-columns: minmax(280px, 360px) 1fr;
    gap: var(--space-lg);
    align-items: flex-start;
  }

  @media (max-width: 900px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }

  .left {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-md);
  }

  .block {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .label {
    font-size: 0.7rem;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  h3 {
    font-size: 0.78rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text-secondary);
    font-weight: 500;
    margin: var(--space-sm) 0 4px 0;
  }

  .engines {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-height: 240px;
    overflow-y: auto;
  }

  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 8px;
    border-radius: var(--radius-sm);
    border: 1px solid transparent;
    cursor: pointer;
    font-size: 0.85rem;
  }

  .check:hover {
    background: var(--surface-2);
  }

  .check.on {
    background: rgba(74, 222, 128, 0.08);
    border-color: rgba(74, 222, 128, 0.3);
  }

  .panels {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }

  .hint-card {
    text-align: center;
    padding: var(--space-xl);
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
  }

  .panel {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-md);
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .panel-head {
    display: flex;
    align-items: baseline;
    gap: var(--space-sm);
  }

  .eval {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-weight: 600;
    color: var(--accent);
  }

  .panel-meta {
    display: flex;
    gap: var(--space-md);
    font-size: 0.78rem;
  }

  .pv {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.85rem;
    color: var(--text-primary);
    background: var(--surface-2);
    padding: var(--space-xs) var(--space-sm);
    border-radius: var(--radius-sm);
    line-height: 1.4;
    word-break: break-word;
  }

  .error {
    background: rgba(248, 113, 113, 0.1);
    border: 1px solid var(--danger);
    color: var(--danger);
    padding: var(--space-sm) var(--space-md);
    border-radius: var(--radius-sm);
  }
</style>
