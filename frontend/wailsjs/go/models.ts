export namespace main {

	export class AnalysisParams {
	    fen: string;
	    moves: string[];
	    engineIds: string[];
	    infinite: boolean;
	    depth: number;
	    moveTime: number;

	    static createFrom(source: any = {}) {
	        return new AnalysisParams(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fen = source["fen"];
	        this.moves = source["moves"];
	        this.engineIds = source["engineIds"];
	        this.infinite = source["infinite"];
	        this.depth = source["depth"];
	        this.moveTime = source["moveTime"];
	    }
	}
	export class EngineOptionDef {
	    name: string;
	    type: string;
	    default: string;
	    min?: number;
	    max?: number;
	    vars?: string[];
	    description?: string;
	    recommended?: string;

	    static createFrom(source: any = {}) {
	        return new EngineOptionDef(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.default = source["default"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.vars = source["vars"];
	        this.description = source["description"];
	        this.recommended = source["recommended"];
	    }
	}
	export class EngineOptionConfig {
	    definitions: EngineOptionDef[];
	    values: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new EngineOptionConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.definitions = (source["definitions"] ?? []).map((d: any) => new EngineOptionDef(d));
	        this.values = source["values"] ?? {};
	    }
	}
	export class TournamentEngineRef {
	    id: string;
	    name?: string;
	    options?: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new TournamentEngineRef(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.options = source["options"];
	    }
	}
	export class TournamentSpec {
	    format: string;
	    engines: TournamentEngineRef[];
	    games: number;
	    concurrency: number;
	    timeControlMs: number;
	    depthLimit: number;
	    event: string;
	    pairMode: boolean;
	    maxPlies: number;
	    resignScore: number;
	    resignMoves: number;
	    drawScore: number;
	    drawMoves: number;
	    drawMinPly: number;

	    static createFrom(source: any = {}) {
	        return new TournamentSpec(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.engines = (source["engines"] ?? []).map((e: any) => new TournamentEngineRef(e));
	        this.games = source["games"];
	        this.concurrency = source["concurrency"];
	        this.timeControlMs = source["timeControlMs"];
	        this.depthLimit = source["depthLimit"];
	        this.event = source["event"];
	        this.pairMode = source["pairMode"];
	        this.maxPlies = source["maxPlies"];
	        this.resignScore = source["resignScore"];
	        this.resignMoves = source["resignMoves"];
	        this.drawScore = source["drawScore"];
	        this.drawMoves = source["drawMoves"];
	        this.drawMinPly = source["drawMinPly"];
	    }
	}
	export class MoveDetail {
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

	    static createFrom(source: any = {}) {
	        return new MoveDetail(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ply = source["ply"];
	        this.side = source["side"];
	        this.uci = source["uci"];
	        this.san = source["san"];
	        this.fen = source["fen"];
	        this.depth = source["depth"];
	        this.evalCp = source["evalCp"];
	        this.evalMate = source["evalMate"];
	        this.elapsedMs = source["elapsedMs"];
	        this.clockAfterMs = source["clockAfterMs"];
	    }
	}
	export class GameDetail {
	    gameNumber: number;
	    round: string;
	    white: string;
	    black: string;
	    result: string;
	    reason?: string;
	    error?: string;
	    startFen: string;
	    pgn?: string;
	    moves: MoveDetail[];

	    static createFrom(source: any = {}) {
	        return new GameDetail(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameNumber = source["gameNumber"];
	        this.round = source["round"];
	        this.white = source["white"];
	        this.black = source["black"];
	        this.result = source["result"];
	        this.reason = source["reason"];
	        this.error = source["error"];
	        this.startFen = source["startFen"];
	        this.pgn = source["pgn"];
	        this.moves = (source["moves"] ?? []).map((m: any) => new MoveDetail(m));
	    }
	}
	export class GameRow {
	    gameNumber: number;
	    round: string;
	    white: string;
	    black: string;
	    outcome: string;
	    reason: string;
	    plies: number;
	    error?: string;

	    static createFrom(source: any = {}) {
	        return new GameRow(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameNumber = source["gameNumber"];
	        this.round = source["round"];
	        this.white = source["white"];
	        this.black = source["black"];
	        this.outcome = source["outcome"];
	        this.reason = source["reason"];
	        this.plies = source["plies"];
	        this.error = source["error"];
	    }
	}
	export class PlayerScoreRow {
	    name: string;
	    wins: number;
	    draws: number;
	    losses: number;
	    games: number;
	    points: number;
	    elo: number;
	    eloLo: number;
	    eloHi: number;

	    static createFrom(source: any = {}) {
	        return new PlayerScoreRow(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.wins = source["wins"];
	        this.draws = source["draws"];
	        this.losses = source["losses"];
	        this.games = source["games"];
	        this.points = source["points"];
	        this.elo = source["elo"];
	        this.eloLo = source["eloLo"];
	        this.eloHi = source["eloHi"];
	    }
	}
	export class CrosstableCell {
	    wins: number;
	    draws: number;
	    losses: number;
	    games: number;
	    points: number;

	    static createFrom(source: any = {}) {
	        return new CrosstableCell(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.wins = source["wins"];
	        this.draws = source["draws"];
	        this.losses = source["losses"];
	        this.games = source["games"];
	        this.points = source["points"];
	    }
	}
	export class CrosstableData {
	    players: string[];
	    cells: CrosstableCell[][];

	    static createFrom(source: any = {}) {
	        return new CrosstableData(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.players = source["players"] ?? [];
	        this.cells = (source["cells"] ?? []).map((row: any[]) =>
	            (row ?? []).map((c) => new CrosstableCell(c)),
	        );
	    }
	}
	export class TournamentSummary {
	    id: string;
	    spec: TournamentSpec;
	    status: string;
	    error?: string;
	    startedAt: string;
	    finishedAt?: string;
	    gamesTotal: number;
	    gamesPlayed: number;
	    outcomes: GameRow[];
	    standings: PlayerScoreRow[];
	    crosstable: CrosstableData;

	    static createFrom(source: any = {}) {
	        return new TournamentSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.spec = source["spec"] ? new TournamentSpec(source["spec"]) : (undefined as any);
	        this.status = source["status"];
	        this.error = source["error"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.gamesTotal = source["gamesTotal"];
	        this.gamesPlayed = source["gamesPlayed"];
	        this.outcomes = (source["outcomes"] ?? []).map((g: any) => new GameRow(g));
	        this.standings = (source["standings"] ?? []).map((s: any) => new PlayerScoreRow(s));
	        this.crosstable = new CrosstableData(source["crosstable"] ?? {});
	    }
	}

}

export namespace registry {

	export class EngineInfo {
	    id: string;
	    name: string;
	    version: string;
	    author: string;
	    description: string;
	    eloEstimate: number;
	    requiresNetwork: boolean;
	    hasBuild: boolean;

	    static createFrom(source: any = {}) {
	        return new EngineInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.description = source["description"];
	        this.eloEstimate = source["eloEstimate"];
	        this.requiresNetwork = source["requiresNetwork"];
	        this.hasBuild = source["hasBuild"];
	    }
	}
	export class InstalledEngine {
	    ID: string;
	    RegistryID: string;
	    Name: string;
	    Version: string;
	    BinaryPath: string;
	    InstalledAt: string;
	    BuildKey: string;
	    OptionValues: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new InstalledEngine(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.RegistryID = source["RegistryID"];
	        this.Name = source["Name"];
	        this.Version = source["Version"];
	        this.BinaryPath = source["BinaryPath"];
	        this.InstalledAt = source["InstalledAt"];
	        this.BuildKey = source["BuildKey"];
	        this.OptionValues = source["OptionValues"];
	    }
	}

}

export namespace uci {

	export class EngineInfo {
	    id: string;
	    name: string;
	    author: string;
	    binaryPath: string;
	    state: string;

	    static createFrom(source: any = {}) {
	        return new EngineInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.author = source["author"];
	        this.binaryPath = source["binaryPath"];
	        this.state = source["state"];
	    }
	}

}
