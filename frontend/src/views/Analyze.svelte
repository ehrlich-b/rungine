<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { App, on } from '../lib/wails';
  import Board from '../components/Board.svelte';
  import EvalBar from '../components/EvalBar.svelte';
  import { STARTING_FEN, parseFEN, uciToArrow, whitePov, type Arrow } from '../lib/chess';
  import type { registry } from '../../wailsjs/go/models';

  const MULTIPV = 3;

  type Line = {
    multipv: number;
    depth: number;
    seldepth: number;
    evalCp: number | null;
    evalMate: number | null;
    nodes: number;
    nps: number;
    timeMs: number;
    pv: string[];
  };

  type EngineState = {
    id: string;
    name: string;
    lines: Record<number, Line>;
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
      states = {};
      for (const id of selected) {
        const eng = installed.find((e) => e.ID === id);
        if (!eng) continue;
        states[id] = { id, name: eng.Name, lines: {} };
      }
      states = { ...states };

      for (const id of selected) {
        try {
          await App.StartEngine(id);
        } catch {
          // Already started — proceed.
        }
        // Ask for ranked lines; engines that lack MultiPV ignore this.
        try {
          await App.SetEngineOption(id, 'MultiPV', String(MULTIPV));
        } catch {
          // Unsupported — single line is fine.
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

  function resetPosition() {
    fen = STARTING_FEN;
  }

  // ---- score helpers (analysis scores are side-to-move POV; show white POV) ----

  let turn = $derived.by<'w' | 'b'>(() => {
    try {
      return parseFEN(fen).turn;
    } catch {
      return 'w';
    }
  });

  function whiteCp(line: Line): number | null {
    if (line.evalCp === null) return null;
    return whitePov(line.evalCp, turn);
  }
  function whiteMate(line: Line): number | null {
    if (line.evalMate === null) return null;
    return whitePov(line.evalMate, turn);
  }

  function fmtEval(line: Line): string {
    const mate = whiteMate(line);
    if (mate !== null) return mate > 0 ? `M${mate}` : `-M${-mate}`;
    const cp = whiteCp(line);
    if (cp !== null) {
      const p = cp / 100;
      return p >= 0 ? `+${p.toFixed(2)}` : p.toFixed(2);
    }
    return '—';
  }

  function evalClass(line: Line | null): string {
    if (!line) return '';
    const mate = whiteMate(line);
    if (mate !== null) return mate > 0 ? 'good' : 'bad';
    const cp = whiteCp(line);
    if (cp !== null) {
      if (cp > 50) return 'good';
      if (cp < -50) return 'bad';
    }
    return '';
  }

  function fmtNumber(n: number): string {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(0)}k`;
    return n.toString();
  }

  function rankedLines(s: EngineState): Line[] {
    return Object.values(s.lines).sort((a, b) => a.multipv - b.multipv);
  }

  let engineStates = $derived(Object.values(states));

  // Primary engine = first selected with at least one line; drives the eval
  // bar and the board arrows.
  let primary = $derived.by<EngineState | null>(() => {
    for (const id of selected) {
      const s = states[id];
      if (s && Object.keys(s.lines).length > 0) return s;
    }
    return null;
  });
  let primaryBest = $derived(primary ? primary.lines[1] ?? rankedLines(primary)[0] ?? null : null);

  let barCp = $derived(primaryBest ? whiteCp(primaryBest) : null);
  let barMate = $derived(primaryBest ? whiteMate(primaryBest) : null);

  let boardArrows = $derived.by<Arrow[]>(() => {
    if (!primary) return [];
    const arrows: Arrow[] = [];
    for (const line of rankedLines(primary)) {
      const top = line.multipv <= 1;
      const a = uciToArrow(
        line.pv[0],
        top ? 'var(--accent)' : 'rgba(148, 163, 184, 0.55)',
        top ? 1 : 0.7,
      );
      if (a) arrows.push(a);
    }
    return arrows;
  });

  onMount(() => {
    refresh();
    unsubs.push(
      on<any>('analysis:info', (info) => {
        if (!info || !info.EngineID) return;
        const id = info.EngineID;
        const cur = states[id];
        if (!cur) return;
        const mpv = info.MultiPV && info.MultiPV > 0 ? info.MultiPV : 1;
        const score = info.Score ?? {};
        const line: Line = {
          multipv: mpv,
          depth: info.Depth ?? 0,
          seldepth: info.SelDepth ?? 0,
          evalCp: score.Centipawns ?? null,
          evalMate: score.Mate ?? null,
          nodes: info.Nodes ?? 0,
          nps: info.NPS ?? 0,
          timeMs: info.Time ? Math.round(info.Time / 1_000_000) : 0,
          pv: info.PV ?? [],
        };
        states = { ...states, [id]: { ...cur, lines: { ...cur.lines, [mpv]: line } } };
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
      <div class="board-and-eval">
        <EvalBar cp={barCp} mate={barMate} />
        <Board {fen} size={48} arrows={boardArrows} />
      </div>
      {#if primaryBest}
        <div class="big-eval {evalClass(primaryBest)}">{fmtEval(primaryBest)}</div>
      {/if}
      <label class="block">
        <span class="label">FEN</span>
        <input bind:value={fen} spellcheck="false" disabled={analyzing} />
      </label>
      <button onclick={resetPosition} disabled={analyzing}>Reset to startpos</button>

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
      {#if engineStates.length === 0}
        <div class="hint-card muted">
          Pick one or more engines and click Start to see ranked engine lines
          stream live here, with the top moves drawn on the board.
        </div>
      {:else}
        {#each engineStates as s (s.id)}
          {@const ranked = rankedLines(s)}
          {@const best = ranked[0] ?? null}
          <div class="panel">
            <div class="panel-head">
              <strong>{s.name}</strong>
              {#if best}
                <span class="eval {evalClass(best)}">{fmtEval(best)}</span>
                <span class="muted">depth {best.depth}/{best.seldepth || '—'}</span>
                <span class="meta muted">{fmtNumber(best.nodes)} nodes · {fmtNumber(best.nps)} nps · {(best.timeMs / 1000).toFixed(1)}s</span>
              {:else}
                <span class="muted">thinking…</span>
              {/if}
            </div>
            {#each ranked as line (line.multipv)}
              <div class="line">
                <span class="line-eval {evalClass(line)}">{fmtEval(line)}</span>
                <span class="line-depth muted">d{line.depth}</span>
                <span class="line-pv">{line.pv.slice(0, 14).join(' ') || '…'}</span>
              </div>
            {/each}
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

  .board-and-eval {
    display: flex;
    flex-direction: row;
    align-items: stretch;
    gap: var(--space-sm);
  }

  .big-eval {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 1.4rem;
    font-weight: 700;
    text-align: center;
    color: var(--text-primary);
  }
  .big-eval.good {
    color: var(--result-win);
  }
  .big-eval.bad {
    color: var(--result-loss);
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
    min-width: 0;
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
    min-width: 0;
  }

  .panel-head {
    display: flex;
    align-items: baseline;
    gap: var(--space-sm);
    flex-wrap: wrap;
  }

  .panel-head .meta {
    margin-left: auto;
    font-size: 0.75rem;
  }

  .eval {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-weight: 600;
    color: var(--accent);
  }
  .eval.good {
    color: var(--result-win);
  }
  .eval.bad {
    color: var(--result-loss);
  }

  .line {
    display: flex;
    align-items: baseline;
    gap: var(--space-sm);
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.82rem;
    padding: 2px var(--space-sm);
    background: var(--surface-2);
    border-radius: var(--radius-sm);
    min-width: 0;
  }

  .line-eval {
    font-weight: 600;
    min-width: 3.5em;
  }
  .line-eval.good {
    color: var(--result-win);
  }
  .line-eval.bad {
    color: var(--result-loss);
  }

  .line-depth {
    min-width: 2.5em;
    font-size: 0.75rem;
  }

  .line-pv {
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
  }

  .error {
    background: rgba(248, 113, 113, 0.1);
    border: 1px solid var(--danger);
    color: var(--danger);
    padding: var(--space-sm) var(--space-md);
    border-radius: var(--radius-sm);
  }
</style>
