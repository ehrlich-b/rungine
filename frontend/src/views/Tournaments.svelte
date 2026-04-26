<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { App, on } from '../lib/wails';
  import { navigate } from '../lib/router';
  import GameView from '../components/GameView.svelte';
  import LiveGames from '../components/LiveGames.svelte';
  import type { registry } from '../../wailsjs/go/models';
  import { main } from '../../wailsjs/go/models';

  type Format = 'match' | 'round-robin' | 'gauntlet';

  type Slot = {
    slotId: string;
    engineId: string;
    name: string;
    optionsText: string; // raw "Key=Value" lines, parsed at submit
    showOptions: boolean;
  };

  type TcMode = 'movetime' | 'tplus' | 'depth' | 'nodes';

  let installed = $state<registry.InstalledEngine[]>([]);
  let slots = $state<Slot[]>([]);
  let format = $state<Format>('match');
  let games = $state(4);
  let tcMode = $state<TcMode>('movetime');
  let movetimeMs = $state(200);
  let tcInitialSec = $state(60);
  let tcIncrementSec = $state(0.6);
  let depthLimit = $state(10);
  let nodesLimit = $state(100000);
  let concurrency = $state(1);
  let pairMode = $state(true);

  let sprtEnabled = $state(false);
  let sprtElo0 = $state(0);
  let sprtElo1 = $state(20);
  let sprtAlpha = $state(0.05);
  let sprtBeta = $state(0.05);

  let startFen = $state('');

  type Preset = {
    name: string;
    slots: Slot[];
    format: Format;
    games: number;
    tcMode: TcMode;
    movetimeMs: number;
    tcInitialSec: number;
    tcIncrementSec: number;
    depthLimit: number;
    nodesLimit: number;
    concurrency: number;
    pairMode: boolean;
    sprtEnabled: boolean;
    sprtElo0: number;
    sprtElo1: number;
    sprtAlpha: number;
    sprtBeta: number;
  };

  const PRESETS_KEY = 'rungine.tournamentPresets';
  let presets = $state<Preset[]>(loadPresets());
  let presetName = $state('');

  function loadPresets(): Preset[] {
    try {
      const raw = localStorage.getItem(PRESETS_KEY);
      if (!raw) return [];
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? (parsed as Preset[]) : [];
    } catch {
      return [];
    }
  }

  function savePresets() {
    try {
      localStorage.setItem(PRESETS_KEY, JSON.stringify(presets));
    } catch {
      // localStorage full or disabled — silently drop.
    }
  }

  function savePreset() {
    const name = presetName.trim();
    if (!name) return;
    const preset: Preset = {
      name,
      slots: slots.map((s) => ({ ...s, showOptions: false })),
      format,
      games,
      tcMode,
      movetimeMs,
      tcInitialSec,
      tcIncrementSec,
      depthLimit,
      nodesLimit,
      concurrency,
      pairMode,
      sprtEnabled,
      sprtElo0,
      sprtElo1,
      sprtAlpha,
      sprtBeta,
    };
    const idx = presets.findIndex((p) => p.name === name);
    if (idx >= 0) presets[idx] = preset;
    else presets = [...presets, preset];
    savePresets();
    presetName = '';
  }

  function applyPreset(p: Preset) {
    slots = p.slots.map((s) => ({ ...s }));
    format = p.format;
    games = p.games;
    tcMode = p.tcMode ?? 'movetime';
    movetimeMs = p.movetimeMs;
    tcInitialSec = p.tcInitialSec ?? 60;
    tcIncrementSec = p.tcIncrementSec ?? 0;
    depthLimit = p.depthLimit ?? 10;
    nodesLimit = p.nodesLimit ?? 100000;
    concurrency = p.concurrency;
    pairMode = p.pairMode;
    sprtEnabled = p.sprtEnabled;
    sprtElo0 = p.sprtElo0;
    sprtElo1 = p.sprtElo1;
    sprtAlpha = p.sprtAlpha;
    sprtBeta = p.sprtBeta;
    presetName = p.name;
  }

  function deletePreset(name: string) {
    presets = presets.filter((p) => p.name !== name);
    savePresets();
  }

  let starting = $state(false);
  let error = $state<string | null>(null);
  let activeTournamentId = $state<string | null>(null);
  let summary = $state<main.TournamentSummary | null>(null);
  let tournaments = $state<main.TournamentSummary[]>([]);

  let viewingGame = $state<{ id: string; gameNumber: number } | null>(null);
  let gameDetail = $state<main.GameDetail | null>(null);

  let unsubs: Array<() => void> = [];

  async function openGame(id: string, gameNumber: number) {
    viewingGame = { id, gameNumber };
    try {
      gameDetail = await App.GetGameDetail(id, gameNumber);
    } catch (e) {
      error = `Failed to load game: ${e}`;
      viewingGame = null;
    }
  }

  function closeGame() {
    viewingGame = null;
    gameDetail = null;
  }

  async function refresh() {
    try {
      installed = (await App.ListInstalledEngines()) ?? [];
      tournaments = (await App.ListTournaments()) ?? [];
      if (!activeTournamentId && tournaments.length > 0) {
        // Auto-focus the most recent tournament so the dashboard is visible
        // immediately, even when there's only one in the list.
        activeTournamentId = tournaments[tournaments.length - 1].id;
      }
      if (activeTournamentId) {
        summary = await App.GetTournament(activeTournamentId);
      }
    } catch (e) {
      error = `Refresh failed: ${e}`;
    }
  }

  function defaultSlotName(engineId: string): string {
    const eng = installed.find((e) => e.ID === engineId);
    const base = eng?.Name ?? engineId;
    const sameEngine = slots.filter((s) => s.engineId === engineId).length;
    return sameEngine === 0 ? base : `${base} #${sameEngine + 1}`;
  }

  function addSlot(engineId: string) {
    const slot: Slot = {
      slotId: `s${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`,
      engineId,
      name: defaultSlotName(engineId),
      optionsText: '',
      showOptions: false,
    };
    slots = [...slots, slot];
  }

  function removeSlot(slotId: string) {
    slots = slots.filter((s) => s.slotId !== slotId);
  }

  function patchSlot(slotId: string, patch: Partial<Slot>) {
    slots = slots.map((s) => (s.slotId === slotId ? { ...s, ...patch } : s));
  }

  function parseOptions(text: string): Record<string, string> {
    const out: Record<string, string> = {};
    for (const raw of text.split('\n')) {
      const line = raw.trim();
      if (!line || line.startsWith('#')) continue;
      const eq = line.indexOf('=');
      if (eq <= 0) continue;
      const k = line.slice(0, eq).trim();
      const v = line.slice(eq + 1).trim();
      if (k) out[k] = v;
    }
    return out;
  }

  function canStart(): boolean {
    if (starting) return false;
    if (format === 'match' && slots.length !== 2) return false;
    if (slots.length < 2 || games < 1) return false;
    switch (tcMode) {
      case 'movetime':
        return movetimeMs >= 50;
      case 'tplus':
        return tcInitialSec >= 1;
      case 'depth':
        return depthLimit >= 1;
      case 'nodes':
        return nodesLimit >= 100;
    }
    return false;
  }

  async function start() {
    error = null;
    starting = true;
    try {
      const engines = slots.map((s) => {
        const opts = parseOptions(s.optionsText);
        return {
          id: s.engineId,
          name: s.name,
          options: Object.keys(opts).length > 0 ? opts : undefined,
        };
      });
      const id = await App.StartTournament({
        format,
        engines,
        games,
        concurrency,
        timeControlMs: tcMode === 'movetime' ? movetimeMs : 0,
        depthLimit: tcMode === 'depth' ? depthLimit : 0,
        nodesLimit: tcMode === 'nodes' ? nodesLimit : 0,
        tcInitialMs: tcMode === 'tplus' ? Math.round(tcInitialSec * 1000) : 0,
        tcIncrementMs: tcMode === 'tplus' ? Math.round(tcIncrementSec * 1000) : 0,
        event: 'Rungine GUI',
        pairMode,
        maxPlies: 400,
        resignScore: 0,
        resignMoves: 4,
        drawScore: -1,
        drawMoves: 8,
        drawMinPly: 60,
        sprtElo0: format === 'match' && sprtEnabled ? sprtElo0 : 0,
        sprtElo1: format === 'match' && sprtEnabled ? sprtElo1 : 0,
        sprtAlpha: format === 'match' && sprtEnabled ? sprtAlpha : 0,
        sprtBeta: format === 'match' && sprtEnabled ? sprtBeta : 0,
        startFen: startFen.trim(),
      } as any);
      activeTournamentId = id;
      summary = await App.GetTournament(id);
      tournaments = (await App.ListTournaments()) ?? [];
    } catch (e) {
      error = `Start failed: ${e}`;
    } finally {
      starting = false;
    }
  }

  async function stop() {
    if (!activeTournamentId) return;
    try {
      await App.StopTournament(activeTournamentId);
    } catch (e) {
      error = `Stop failed: ${e}`;
    }
  }

  async function exportPGN() {
    if (!summary) return;
    try {
      const pgn = await App.GetTournamentPGN(summary.id);
      const blob = new Blob([pgn], { type: 'application/x-chess-pgn' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${summary.id}.pgn`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (e) {
      error = `Export failed: ${e}`;
    }
  }

  function formatOutcome(o: string): string {
    if (o === '1-0') return 'White wins';
    if (o === '0-1') return 'Black wins';
    if (o === '1/2-1/2') return 'Draw';
    return o || '—';
  }

  function outcomeClass(o: string): string {
    if (o === '1-0') return 'win';
    if (o === '0-1') return 'loss';
    if (o === '1/2-1/2') return 'draw';
    return '';
  }

  onMount(() => {
    refresh();

    unsubs.push(
      on<{ tournamentId: string }>('tournament:gameComplete', async (p) => {
        if (p && (p.tournamentId === activeTournamentId || activeTournamentId === null)) {
          if (activeTournamentId) {
            summary = await App.GetTournament(activeTournamentId);
          }
        }
      }),
      on<{ tournamentId: string; status: string }>('tournament:done', async (p) => {
        if (p && p.tournamentId === activeTournamentId) {
          summary = await App.GetTournament(activeTournamentId);
        }
        tournaments = (await App.ListTournaments()) ?? [];
      }),
      on<{ tournamentId: string; sprt: main.SprtState }>('tournament:sprt', (p) => {
        if (p && p.tournamentId === activeTournamentId && summary) {
          summary = main.TournamentSummary.createFrom({ ...summary, sprt: p.sprt });
        }
      }),
    );
  });

  onDestroy(() => {
    unsubs.forEach((u) => u());
  });
</script>

<section class="page">
  <header>
    <h1>Tournaments</h1>
    {#if summary && summary.status === 'running'}
      <button class="danger" onclick={stop}>Stop tournament</button>
    {/if}
  </header>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  {#if installed.length < 1}
    <div class="empty">
      <h2>Install an engine to begin</h2>
      <p class="subtle">
        You can play one engine against itself with different option configs, or
        install a second engine for engine-vs-engine matches.
      </p>
      <button class="primary" onclick={() => navigate('engines')}>Go to Engines</button>
    </div>
  {:else}
    <div class="layout">
      <form
        class="setup"
        onsubmit={(e) => {
          e.preventDefault();
          if (canStart()) start();
        }}>
        <h2>New tournament</h2>

        <div class="field">
          <span class="label">Available engines</span>
          <div class="engines">
            {#each installed as eng (eng.ID)}
              <button
                type="button"
                class="add-engine"
                onclick={() => addSlot(eng.ID)}>
                <span class="plus">+</span>
                <span>{eng.Name}</span>
                <span class="muted">{eng.Version}</span>
              </button>
            {/each}
          </div>
          <span class="hint subtle">
            Click + to add an engine slot. Add the same engine twice with different
            options to play it against itself.
          </span>
        </div>

        <div class="field">
          <span class="label">Tournament slots ({slots.length})</span>
          {#if slots.length === 0}
            <p class="subtle small">No engines selected yet.</p>
          {:else}
            <div class="slots">
              {#each slots as slot, i (slot.slotId)}
                <div class="slot">
                  <div class="slot-row">
                    <span class="slot-num muted">{i + 1}</span>
                    <input
                      class="slot-name"
                      value={slot.name}
                      oninput={(e) =>
                        patchSlot(slot.slotId, {
                          name: (e.currentTarget as HTMLInputElement).value,
                        })}
                      placeholder="Display name"
                      spellcheck="false" />
                    <button
                      type="button"
                      class="slot-toggle"
                      class:active={slot.showOptions}
                      onclick={() =>
                        patchSlot(slot.slotId, { showOptions: !slot.showOptions })}>
                      {slot.showOptions ? 'Hide options' : 'Options'}
                    </button>
                    <button
                      type="button"
                      class="slot-remove"
                      onclick={() => removeSlot(slot.slotId)}
                      title="Remove">×</button>
                  </div>
                  {#if slot.showOptions}
                    <textarea
                      class="slot-options"
                      placeholder="One UCI option per line, e.g.&#10;Hash=256&#10;Threads=4&#10;Skill Level=10"
                      value={slot.optionsText}
                      oninput={(e) =>
                        patchSlot(slot.slotId, {
                          optionsText: (e.currentTarget as HTMLTextAreaElement).value,
                        })}
                      rows="4"
                      spellcheck="false"></textarea>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
          {#if format === 'match' && slots.length !== 2}
            <span class="hint">Match needs exactly two slots</span>
          {/if}
        </div>

        <div class="field">
          <span class="label">Format</span>
          <div class="seg">
            {#each ['match', 'round-robin', 'gauntlet'] as f (f)}
              <button
                type="button"
                class:active={format === f}
                onclick={() => (format = f as Format)}>
                {f}
              </button>
            {/each}
          </div>
        </div>

        <div class="grid">
          <label>
            <span class="label">Games</span>
            <input type="number" min="1" max="1000" bind:value={games} />
          </label>
          <label>
            <span class="label">Concurrency</span>
            <input type="number" min="1" max="32" bind:value={concurrency} />
          </label>
          <label class="check inline">
            <input type="checkbox" bind:checked={pairMode} />
            <span>Pair mode (mirror colors)</span>
          </label>
        </div>

        <div class="field">
          <span class="label">Time control</span>
          <div class="seg">
            {#each ['movetime', 'tplus', 'depth', 'nodes'] as m (m)}
              <button
                type="button"
                class:active={tcMode === m}
                onclick={() => (tcMode = m as TcMode)}>
                {m === 'tplus' ? 'time + inc' : m}
              </button>
            {/each}
          </div>
          <div class="grid tc-grid">
            {#if tcMode === 'movetime'}
              <label>
                <span class="label">Movetime (ms)</span>
                <input type="number" min="50" max="60000" step="50" bind:value={movetimeMs} />
              </label>
            {:else if tcMode === 'tplus'}
              <label>
                <span class="label">Initial (s)</span>
                <input type="number" min="1" max="10800" step="1" bind:value={tcInitialSec} />
              </label>
              <label>
                <span class="label">Increment (s)</span>
                <input type="number" min="0" max="60" step="0.1" bind:value={tcIncrementSec} />
              </label>
            {:else if tcMode === 'depth'}
              <label>
                <span class="label">Depth</span>
                <input type="number" min="1" max="60" step="1" bind:value={depthLimit} />
              </label>
            {:else if tcMode === 'nodes'}
              <label>
                <span class="label">Nodes</span>
                <input type="number" min="100" max="1000000000" step="1000" bind:value={nodesLimit} />
              </label>
            {/if}
          </div>
        </div>

        <div class="field">
          <span class="label">Start position (FEN)</span>
          <input
            type="text"
            class="fen-input"
            placeholder="empty = standard startpos"
            bind:value={startFen}
            spellcheck="false" />
        </div>

        {#if format === 'match'}
          <div class="field">
            <label class="check inline">
              <input type="checkbox" bind:checked={sprtEnabled} />
              <span>SPRT (early-stop on ELO bound)</span>
            </label>
            {#if sprtEnabled}
              <div class="grid sprt-grid">
                <label>
                  <span class="label">Elo0 (H0)</span>
                  <input type="number" step="1" bind:value={sprtElo0} />
                </label>
                <label>
                  <span class="label">Elo1 (H1)</span>
                  <input type="number" step="1" bind:value={sprtElo1} />
                </label>
                <label>
                  <span class="label">Alpha</span>
                  <input type="number" step="0.01" min="0.001" max="0.5" bind:value={sprtAlpha} />
                </label>
                <label>
                  <span class="label">Beta</span>
                  <input type="number" step="0.01" min="0.001" max="0.5" bind:value={sprtBeta} />
                </label>
              </div>
              <span class="hint subtle">
                First slot is the candidate. Match stops as soon as LLR crosses
                a bound.
              </span>
            {/if}
          </div>
        {/if}

        <div class="field">
          <span class="label">Presets</span>
          <div class="presets-row">
            <input
              type="text"
              class="preset-name"
              placeholder="Preset name"
              bind:value={presetName}
              spellcheck="false" />
            <button
              type="button"
              class="preset-save"
              onclick={savePreset}
              disabled={presetName.trim() === ''}>
              Save
            </button>
          </div>
          {#if presets.length > 0}
            <div class="presets-list">
              {#each presets as p (p.name)}
                <div class="preset-item">
                  <button
                    type="button"
                    class="preset-apply"
                    onclick={() => applyPreset(p)}
                    title="Load this preset">
                    {p.name}
                  </button>
                  <span class="preset-meta muted small">
                    {p.format} · {p.slots.length} slots
                  </span>
                  <button
                    type="button"
                    class="preset-delete"
                    onclick={() => deletePreset(p.name)}
                    title="Delete preset">×</button>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <button type="submit" class="primary" disabled={!canStart()}>
          {starting ? 'Starting…' : 'Start tournament'}
        </button>
      </form>

      <div class="dashboard">
        {#if viewingGame && gameDetail}
          <GameView
            detail={gameDetail}
            onClose={closeGame}
            tournamentId={viewingGame.id} />

        {:else if summary}
          <div class="card">
            <div class="card-head">
              <strong>Tournament {summary.id}</strong>
              <span class="status status-{summary.status}">{summary.status}</span>
              {#if summary.outcomes.length > 0}
                <button class="export" onclick={exportPGN}>Export PGN</button>
              {/if}
            </div>
            <div class="progress">
              <div
                class="bar"
                style:width="{summary.gamesTotal > 0
                  ? (summary.gamesPlayed / summary.gamesTotal) * 100
                  : 0}%">
              </div>
              <span class="progress-text">
                {summary.gamesPlayed} / {summary.gamesTotal} games
              </span>
            </div>

            {#if summary.sprt}
              {@const s = summary.sprt}
              {@const span = s.upperBound - s.lowerBound}
              {@const pos = span > 0
                ? Math.max(0, Math.min(1, (s.llr - s.lowerBound) / span)) * 100
                : 50}
              <div class="sprt-panel" class:decided={s.decision !== 'continue'}>
                <div class="sprt-head">
                  <strong>SPRT</strong>
                  <span class="sprt-decision sprt-{s.decision.replace(' ', '-')}">
                    {s.decision}
                  </span>
                  <span class="sprt-wdl muted">
                    {s.wins}W / {s.draws}D / {s.losses}L
                  </span>
                </div>
                <div class="sprt-track">
                  <div class="sprt-bound sprt-bound-low" title="Reject H1">
                    {s.lowerBound.toFixed(2)}
                  </div>
                  <div class="sprt-bar">
                    <div class="sprt-marker" style:left="{pos}%"></div>
                  </div>
                  <div class="sprt-bound sprt-bound-high" title="Accept H1">
                    {s.upperBound.toFixed(2)}
                  </div>
                </div>
                <div class="sprt-llr">
                  LLR <strong>{s.llr.toFixed(3)}</strong>
                </div>
              </div>
            {/if}

            {#if summary.status === 'running'}
              <LiveGames
                tournamentId={summary.id}
                onSelectGame={(n) => openGame(summary!.id, n)} />
            {/if}

            {#if summary.standings.length > 0}
              <h3>Standings</h3>
              <table class="standings">
                <thead>
                  <tr>
                    <th>Engine</th>
                    <th>G</th>
                    <th>W</th>
                    <th>D</th>
                    <th>L</th>
                    <th>Pts</th>
                    <th>Elo</th>
                    <th>± 95%</th>
                  </tr>
                </thead>
                <tbody>
                  {#each summary.standings as p (p.name)}
                    <tr>
                      <td title={p.name}>{p.name}</td>
                      <td>{p.games}</td>
                      <td>{p.wins}</td>
                      <td>{p.draws}</td>
                      <td>{p.losses}</td>
                      <td>{p.points.toFixed(1)}</td>
                      <td class="cell-elo">{p.elo > 0 ? '+' : ''}{p.elo.toFixed(0)}</td>
                      <td class="muted cell-ci">
                        [{p.eloLo > 0 ? '+' : ''}{p.eloLo.toFixed(0)},
                        {p.eloHi > 0 ? '+' : ''}{p.eloHi.toFixed(0)}]
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}

            {#if summary.crosstable.players.length >= 3}
              <h3>Crosstable</h3>
              <div class="crosstable-wrap">
                <table class="crosstable">
                  <thead>
                    <tr>
                      <th></th>
                      {#each summary.crosstable.players as p (p)}
                        <th class="rot" title={p}>{p.slice(0, 8)}</th>
                      {/each}
                      <th>Pts</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each summary.crosstable.players as p, i (p)}
                      <tr>
                        <th class="row-name">{p}</th>
                        {#each summary.crosstable.players as _, j}
                          {@const cell = summary.crosstable.cells[i]?.[j]}
                          {#if i === j}
                            <td class="diag">—</td>
                          {:else if !cell || cell.games === 0}
                            <td class="muted">·</td>
                          {:else}
                            <td>{cell.points.toFixed(1)}/{cell.games}</td>
                          {/if}
                        {/each}
                        <td class="row-total">
                          {summary.standings.find((s) => s.name === p)?.points.toFixed(1) ?? '—'}
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}

            {#if summary.outcomes.length > 0}
              <h3>Games ({summary.outcomes.length})</h3>
              <div class="games">
                {#each summary.outcomes.slice().reverse() as g (g.gameNumber)}
                  <button
                    class="game"
                    class:err={g.error}
                    onclick={() => openGame(summary!.id, g.gameNumber)}>
                    <span class="g-num muted">#{g.gameNumber}</span>
                    <span class="g-pair">
                      {g.white} <span class="muted">vs</span> {g.black}
                    </span>
                    <span class="g-result {outcomeClass(g.outcome)}">
                      {g.error ? `error: ${g.error}` : formatOutcome(g.outcome)}
                    </span>
                    {#if g.reason}
                      <span class="muted small">({g.reason})</span>
                    {/if}
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        {:else}
          <div class="hint-card">
            Configure a tournament on the left and click Start. Live standings and
            game results appear here.
          </div>
        {/if}

        {#if tournaments.length > 1}
          <h3 class="section">All tournaments</h3>
          <div class="tlist">
            {#each tournaments.slice().reverse() as t (t.id)}
              <button
                class="tlist-item"
                class:active={t.id === activeTournamentId}
                onclick={async () => {
                  activeTournamentId = t.id;
                  summary = await App.GetTournament(t.id);
                }}>
                <span class="tlist-id">{t.id}</span>
                <span class="muted">{t.spec.format}</span>
                <span class="status status-{t.status}">{t.status}</span>
                <span class="muted small">
                  {t.gamesPlayed}/{t.gamesTotal}
                </span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {/if}
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

  .error {
    background: rgba(248, 113, 113, 0.1);
    border: 1px solid var(--danger);
    color: var(--danger);
    padding: var(--space-sm) var(--space-md);
    border-radius: var(--radius-sm);
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
    margin: 0;
  }

  .layout {
    display: grid;
    grid-template-columns: minmax(280px, 340px) minmax(0, 1fr);
    gap: var(--space-md);
    align-items: flex-start;
  }

  @media (max-width: 900px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }

  .dashboard {
    min-width: 0;
    overflow: hidden;
  }

  .standings,
  .crosstable-wrap,
  .games {
    max-width: 100%;
    overflow-x: auto;
  }

  .setup,
  .card,
  .hint-card {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-md) var(--space-lg);
  }

  .setup {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
  }

  h2 {
    margin: 0;
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text-secondary);
    font-weight: 500;
  }

  h3 {
    font-size: 0.78rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text-secondary);
    font-weight: 500;
    margin-top: var(--space-md);
    margin-bottom: var(--space-sm);
  }

  .field {
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

  .engines {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 200px;
    overflow-y: auto;
  }

  .check {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    padding: 4px 8px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    border: 1px solid transparent;
  }

  .check.inline {
    align-self: flex-end;
  }

  .check:hover {
    background: var(--surface-2);
  }

  .check.on {
    background: rgba(74, 222, 128, 0.08);
    border-color: rgba(74, 222, 128, 0.3);
  }

  .add-engine {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    padding: 4px 8px;
    border-radius: var(--radius-sm);
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-primary);
    text-align: left;
    font-size: 0.85rem;
    cursor: pointer;
  }

  .add-engine:hover {
    background: var(--surface-2);
    border-color: var(--accent);
  }

  .plus {
    color: var(--accent);
    font-weight: 700;
    width: 14px;
    text-align: center;
  }

  .slots {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .slot {
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: var(--space-xs);
  }

  .slot-row {
    display: flex;
    gap: 4px;
    align-items: center;
  }

  .slot-num {
    width: 16px;
    text-align: center;
    font-variant-numeric: tabular-nums;
    font-size: 0.8rem;
  }

  .slot-name {
    flex: 1;
    background: var(--surface-1);
    font-size: 0.85rem;
    padding: 4px 8px;
  }

  .slot-toggle {
    background: transparent;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 3px 8px;
    font-size: 0.75rem;
    color: var(--text-secondary);
  }

  .slot-toggle.active {
    border-color: var(--accent);
    color: var(--accent);
  }

  .slot-remove {
    background: transparent;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    width: 24px;
    padding: 0;
    color: var(--text-muted);
    font-size: 1rem;
    line-height: 1;
  }

  .slot-remove:hover {
    color: var(--danger);
    border-color: var(--danger);
  }

  .slot-options {
    margin-top: 4px;
    width: 100%;
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 6px 8px;
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.8rem;
    resize: vertical;
  }

  .small {
    font-size: 0.75rem;
  }

  .seg {
    display: flex;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }

  .seg button {
    flex: 1;
    border: none;
    border-radius: 0;
    background: transparent;
    color: var(--text-secondary);
    padding: 6px 12px;
    font-size: 0.85rem;
    text-transform: capitalize;
  }

  .seg button.active {
    background: var(--surface-2);
    color: var(--text-primary);
  }

  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-sm);
  }

  .grid label {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .hint {
    font-size: 0.75rem;
    color: var(--warning);
  }

  .hint-card {
    color: var(--text-secondary);
    text-align: center;
    padding: var(--space-xl);
  }

  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }

  .card-head {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }

  .card-head .export {
    margin-left: auto;
    font-size: 0.8rem;
    padding: 4px 10px;
  }

  .status {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    padding: 2px 8px;
    border-radius: 999px;
    background: var(--surface-2);
    color: var(--text-secondary);
  }

  .status-running {
    background: rgba(74, 222, 128, 0.15);
    color: var(--accent);
  }

  .status-done {
    background: var(--surface-2);
    color: var(--text-secondary);
  }

  .status-stopped,
  .status-error {
    background: rgba(248, 113, 113, 0.15);
    color: var(--danger);
  }

  .progress {
    position: relative;
    height: 8px;
    background: var(--surface-2);
    border-radius: 999px;
    overflow: hidden;
  }

  .progress .bar {
    background: var(--accent);
    height: 100%;
    transition: width 200ms ease;
  }

  .progress-text {
    position: absolute;
    top: 12px;
    right: 0;
    font-size: 0.75rem;
    color: var(--text-secondary);
  }

  .standings {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8rem;
    table-layout: auto;
  }

  .standings th,
  .standings td {
    padding: 4px 6px;
    text-align: left;
    white-space: nowrap;
  }

  .standings td:first-child {
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .standings th {
    color: var(--text-secondary);
    font-weight: 500;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    border-bottom: 1px solid var(--border);
  }

  .standings td:nth-child(n + 2) {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  .standings th:nth-child(n + 2) {
    text-align: right;
  }

  .standings .cell-elo {
    font-variant-numeric: tabular-nums;
  }

  .standings .cell-ci {
    font-size: 0.72rem;
  }

  .crosstable-wrap {
    overflow-x: auto;
  }

  .crosstable {
    border-collapse: collapse;
    font-size: 0.8rem;
    font-variant-numeric: tabular-nums;
  }

  .crosstable th,
  .crosstable td {
    padding: 4px 8px;
    text-align: center;
    border: 1px solid var(--border);
  }

  .crosstable th.rot {
    writing-mode: vertical-rl;
    transform: rotate(180deg);
    font-weight: 500;
    color: var(--text-secondary);
    font-size: 0.7rem;
    height: 60px;
  }

  .crosstable .row-name {
    text-align: left;
    font-weight: 500;
    white-space: nowrap;
  }

  .crosstable .row-total {
    font-weight: 600;
  }

  .crosstable .diag {
    background: var(--surface-2);
    color: var(--text-muted);
  }

  .games {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-height: 320px;
    overflow-y: auto;
  }

  .game {
    display: grid;
    grid-template-columns: auto 1fr auto auto;
    gap: var(--space-sm);
    align-items: center;
    padding: 4px 8px;
    border-radius: var(--radius-sm);
    font-size: 0.8rem;
  }

  .game:hover {
    background: var(--surface-2);
  }

  .g-num {
    font-variant-numeric: tabular-nums;
  }

  .g-result.win {
    color: var(--result-win);
  }
  .g-result.loss {
    color: var(--result-loss);
  }
  .g-result.draw {
    color: var(--result-draw);
  }

  .g-result {
    font-weight: 500;
  }

  .game.err {
    background: rgba(248, 113, 113, 0.06);
  }

  .small {
    font-size: 0.7rem;
  }

  .section {
    margin-top: var(--space-md);
  }

  .tlist {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: var(--space-sm);
  }

  .tlist-item {
    display: grid;
    grid-template-columns: auto 1fr auto auto;
    gap: var(--space-sm);
    align-items: center;
    text-align: left;
    padding: 6px 10px;
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 0.85rem;
  }

  .tlist-item:hover {
    background: var(--surface-2);
  }

  .tlist-item.active {
    border-color: var(--accent);
  }

  .tlist-id {
    font-family: ui-monospace, SFMono-Regular, monospace;
    color: var(--text-primary);
  }

  .dashboard {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
    min-width: 0;
  }

  .sprt-grid {
    margin-top: var(--space-sm);
  }

  .fen-input {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.85rem;
  }

  .presets-row {
    display: flex;
    gap: var(--space-sm);
    align-items: center;
  }

  .preset-name {
    flex: 1;
    min-width: 0;
  }

  .preset-save {
    padding: 4px 12px;
  }

  .presets-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-xs);
    margin-top: var(--space-sm);
  }

  .preset-item {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    padding: 4px 8px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface-1);
  }

  .preset-apply {
    flex: 1;
    text-align: left;
    background: transparent;
    border: 0;
    padding: 4px 0;
    color: var(--text-primary);
    font-size: 0.9rem;
    cursor: pointer;
  }

  .preset-apply:hover {
    color: var(--accent);
  }

  .preset-meta {
    margin-left: auto;
  }

  .preset-delete {
    background: transparent;
    border: 0;
    color: var(--text-muted);
    font-size: 1.1rem;
    cursor: pointer;
    padding: 0 4px;
  }

  .preset-delete:hover {
    color: var(--danger);
  }

  .sprt-panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-xs);
    padding: var(--space-sm) var(--space-md);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-2);
    margin: var(--space-sm) 0;
  }

  .sprt-panel.decided {
    border-color: var(--accent);
  }

  .sprt-head {
    display: flex;
    align-items: baseline;
    gap: var(--space-md);
  }

  .sprt-decision {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.8rem;
    padding: 2px 8px;
    border-radius: var(--radius-sm);
    background: var(--surface-3);
    color: var(--text-secondary);
  }

  .sprt-decision.sprt-accept-H1 {
    background: rgba(74, 222, 128, 0.15);
    color: var(--result-win);
  }

  .sprt-decision.sprt-accept-H0 {
    background: rgba(248, 113, 113, 0.15);
    color: var(--result-loss);
  }

  .sprt-wdl {
    font-size: 0.85rem;
    margin-left: auto;
  }

  .sprt-track {
    display: grid;
    grid-template-columns: auto 1fr auto;
    align-items: center;
    gap: var(--space-sm);
  }

  .sprt-bound {
    font-family: ui-monospace, SFMono-Regular, monospace;
    font-size: 0.75rem;
    color: var(--text-muted);
  }

  .sprt-bar {
    position: relative;
    height: 6px;
    background: var(--surface-3);
    border-radius: 3px;
  }

  .sprt-marker {
    position: absolute;
    top: -2px;
    width: 3px;
    height: 10px;
    background: var(--accent);
    border-radius: 2px;
    transform: translateX(-50%);
  }

  .sprt-llr {
    font-size: 0.85rem;
    color: var(--text-secondary);
  }
</style>
