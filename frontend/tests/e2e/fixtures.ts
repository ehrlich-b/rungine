import { test as base, expect, type Page } from '@playwright/test';

export type InstalledEngineMock = {
  ID: string;
  RegistryID: string;
  Name: string;
  Version: string;
  BinaryPath: string;
  InstalledAt: string;
  BuildKey: string;
  OptionValues: Record<string, string>;
};

export type AvailableEngineMock = {
  id: string;
  name: string;
  version: string;
  author: string;
  description: string;
  eloEstimate: number;
  requiresNetwork: boolean;
  hasBuild: boolean;
};

export type GameRowMock = {
  gameNumber: number;
  round: string;
  white: string;
  black: string;
  outcome: string;
  reason: string;
  plies: number;
  error?: string;
};

export type StandingsRowMock = {
  name: string;
  wins: number;
  draws: number;
  losses: number;
  games: number;
  points: number;
  elo: number;
  eloLo: number;
  eloHi: number;
};

export type TournamentSummaryMock = {
  id: string;
  spec: any;
  status: string;
  error?: string;
  startedAt: string;
  finishedAt?: string;
  gamesTotal: number;
  gamesPlayed: number;
  outcomes: GameRowMock[];
  standings: StandingsRowMock[];
  crosstable: { players: string[]; cells: any[][] };
};

export type EngineOptionConfigMock = {
  definitions: Array<{
    name: string;
    type: string;
    default: string;
    min?: number;
    max?: number;
    vars?: string[];
    description?: string;
    recommended?: string;
  }>;
  values: Record<string, string>;
};

export type GameDetailMock = {
  gameNumber: number;
  round: string;
  white: string;
  black: string;
  result: string;
  reason?: string;
  startFen: string;
  pgn?: string;
  moves: Array<{
    ply: number;
    side: string;
    uci: string;
    san: string;
    fen: string;
    depth?: number;
    evalCp?: number;
    evalMate?: number;
    elapsedMs: number;
    clockAfterMs: number;
  }>;
};

export type RungineMockState = {
  installed: InstalledEngineMock[];
  available: AvailableEngineMock[];
  tournaments: TournamentSummaryMock[];
  gameDetails: Record<string, GameDetailMock>;
  liveGames: any[];
  cpuFeatures: string;
  optionConfig: EngineOptionConfigMock;
  startTournamentCalls: any[];
  setEngineOptionConfigCalls: any[];
  startAnalysisCalls: any[];
  stopAnalysisCalls: any[];
  installCalls: string[];
  uninstallCalls: string[];
};

export const defaultMockState: RungineMockState = {
  installed: [],
  available: [],
  tournaments: [],
  gameDetails: {},
  liveGames: [],
  cpuFeatures: 'AVX2,BMI2',
  optionConfig: { definitions: [], values: {} },
  startTournamentCalls: [],
  setEngineOptionConfigCalls: [],
  startAnalysisCalls: [],
  stopAnalysisCalls: [],
  installCalls: [],
  uninstallCalls: [],
};

declare global {
  interface Window {
    __rungineMock: {
      state: RungineMockState;
      fire: (event: string, payload: any) => void;
      listeners: Record<string, Array<(...args: any[]) => void>>;
    };
  }
}

/**
 * Inject a Wails binding mock into the page's `window.go` and
 * `window.runtime` before any application script runs. Call before
 * `page.goto()`. Tests interact with the running mock through
 * `window.__rungineMock` (state for setup, fire() for events).
 */
export async function setupMock(page: Page, overrides: Partial<RungineMockState> = {}) {
  await page.addInitScript((seed) => {
    const listeners: Record<string, Array<(...args: any[]) => void>> = {};
    const state = seed as RungineMockState;

    function gameKey(tid: string, num: number) {
      return `${tid}/${num}`;
    }

    (window as any).runtime = {
      EventsOnMultiple: (name: string, cb: (...args: any[]) => void) => {
        if (!listeners[name]) listeners[name] = [];
        listeners[name].push(cb);
        return () => {
          listeners[name] = (listeners[name] ?? []).filter((c) => c !== cb);
        };
      },
      EventsOn: (name: string, cb: (...args: any[]) => void) => {
        if (!listeners[name]) listeners[name] = [];
        listeners[name].push(cb);
        return () => {
          listeners[name] = (listeners[name] ?? []).filter((c) => c !== cb);
        };
      },
      EventsOff: (name: string) => {
        listeners[name] = [];
      },
      EventsOffAll: () => {
        for (const k of Object.keys(listeners)) listeners[k] = [];
      },
      EventsEmit: () => {},
      LogPrint: () => {},
      LogTrace: () => {},
      LogDebug: () => {},
      LogInfo: () => {},
      LogWarning: () => {},
      LogError: () => {},
      LogFatal: () => {},
    };

    (window as any).go = {
      main: {
        App: {
          GetCPUFeatures: () => Promise.resolve(state.cpuFeatures),
          GetEngineOptions: () => Promise.resolve({}),
          GetEngineOptionConfig: () => Promise.resolve(state.optionConfig),
          ListEngineProfiles: () => Promise.resolve([]),
          ApplyEngineProfile: () => Promise.resolve({}),
          GetTournament: (id: string) =>
            Promise.resolve(state.tournaments.find((t) => t.id === id) ?? null),
          GetTournamentPGN: () => Promise.resolve(''),
          LiveGames: () => Promise.resolve(state.liveGames),
          GetGameDetail: (tid: string, num: number) =>
            Promise.resolve(state.gameDetails[gameKey(tid, num)] ?? null),
          ListAvailableEngines: () => Promise.resolve(state.available),
          ListEngines: () => Promise.resolve([]),
          ListInstalledEngines: () => Promise.resolve(state.installed),
          ListTournaments: () => Promise.resolve(state.tournaments),
          RegisterEngine: () => Promise.resolve(),
          UnregisterEngine: () => Promise.resolve(),
          SetAnalysisThrottle: () => Promise.resolve(),
          SetEngineOption: () => Promise.resolve(),
          SetEngineOptionConfig: (id: string, options: Record<string, string>) => {
            state.setEngineOptionConfigCalls.push({ id, options });
            const e = state.installed.find((x) => x.ID === id);
            if (e) e.OptionValues = { ...options };
            state.optionConfig = {
              ...state.optionConfig,
              values: { ...options },
            };
            return Promise.resolve();
          },
          StartAnalysis: (params: any) => {
            state.startAnalysisCalls.push(params);
            return Promise.resolve();
          },
          StartEngine: () => Promise.resolve(),
          StartTournament: (spec: any) => {
            state.startTournamentCalls.push(spec);
            const id = `t${state.tournaments.length + 1}`;
            const summary: TournamentSummaryMock = {
              id,
              spec,
              status: 'running',
              startedAt: new Date().toISOString(),
              gamesTotal: spec.games ?? 1,
              gamesPlayed: 0,
              outcomes: [],
              standings: [],
              crosstable: { players: [], cells: [] },
            };
            state.tournaments.push(summary);
            return Promise.resolve(id);
          },
          StopAnalysis: (ids: string[]) => {
            state.stopAnalysisCalls.push(ids);
            return Promise.resolve();
          },
          StopEngine: () => Promise.resolve(),
          StopTournament: (id: string) => {
            const t = state.tournaments.find((x) => x.id === id);
            if (t) {
              t.status = 'stopped';
              t.finishedAt = new Date().toISOString();
            }
            return Promise.resolve();
          },
          InstallEngine: (id: string) => {
            state.installCalls.push(id);
            const avail = state.available.find((a) => a.id === id);
            if (avail) {
              state.installed.push({
                ID: avail.id,
                RegistryID: avail.id,
                Name: avail.name,
                Version: avail.version,
                BinaryPath: `/tmp/${avail.id}`,
                InstalledAt: new Date().toISOString(),
                BuildKey: 'mock',
                OptionValues: {},
              });
            }
            return Promise.resolve();
          },
          UninstallEngine: (id: string) => {
            state.uninstallCalls.push(id);
            state.installed = state.installed.filter((e) => e.ID !== id);
            return Promise.resolve();
          },
        },
      },
    };

    window.__rungineMock = {
      state,
      listeners,
      fire: (event: string, payload: any) => {
        (listeners[event] ?? []).forEach((cb) => cb(payload));
      },
    };
  }, mergeState(defaultMockState, overrides));
}

function mergeState(
  base: RungineMockState,
  overrides: Partial<RungineMockState>,
): RungineMockState {
  return {
    ...base,
    ...overrides,
    optionConfig: overrides.optionConfig ?? base.optionConfig,
    gameDetails: overrides.gameDetails ?? base.gameDetails,
  };
}

export const test = base;
export { expect };

/** Convenience builders. */

export function makeAvailable(
  partial: Partial<AvailableEngineMock> & { id: string; name: string },
): AvailableEngineMock {
  return {
    id: partial.id,
    name: partial.name,
    version: partial.version ?? '17',
    author: partial.author ?? 'Author',
    description: partial.description ?? 'A chess engine.',
    eloEstimate: partial.eloEstimate ?? 3000,
    requiresNetwork: partial.requiresNetwork ?? false,
    hasBuild: partial.hasBuild ?? true,
  };
}

export function makeInstalled(
  partial: Partial<InstalledEngineMock> & { ID: string; Name: string },
): InstalledEngineMock {
  return {
    ID: partial.ID,
    RegistryID: partial.RegistryID ?? partial.ID,
    Name: partial.Name,
    Version: partial.Version ?? '17',
    BinaryPath: partial.BinaryPath ?? `/tmp/${partial.ID}`,
    InstalledAt: partial.InstalledAt ?? new Date().toISOString(),
    BuildKey: partial.BuildKey ?? 'mock',
    OptionValues: partial.OptionValues ?? {},
  };
}
