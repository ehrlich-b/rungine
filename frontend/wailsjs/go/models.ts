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
	        this.players = source["players"];
	        this.cells = this.convertValues(source["cells"], CrosstableCell);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
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
	        this.definitions = this.convertValues(source["definitions"], EngineOptionDef);
	        this.values = source["values"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
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
	        this.moves = this.convertValues(source["moves"], MoveDetail);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
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
	    sprtElo0: number;
	    sprtElo1: number;
	    sprtAlpha: number;
	    sprtBeta: number;

	    static createFrom(source: any = {}) {
	        return new TournamentSpec(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.engines = this.convertValues(source["engines"], TournamentEngineRef);
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
	        this.sprtElo0 = source["sprtElo0"];
	        this.sprtElo1 = source["sprtElo1"];
	        this.sprtAlpha = source["sprtAlpha"];
	        this.sprtBeta = source["sprtBeta"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SprtState {
	    llr: number;
	    lowerBound: number;
	    upperBound: number;
	    decision: string;
	    wins: number;
	    draws: number;
	    losses: number;

	    static createFrom(source: any = {}) {
	        return new SprtState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.llr = source["llr"];
	        this.lowerBound = source["lowerBound"];
	        this.upperBound = source["upperBound"];
	        this.decision = source["decision"];
	        this.wins = source["wins"];
	        this.draws = source["draws"];
	        this.losses = source["losses"];
	    }
	}
	export class TournamentSummary {
	    id: string;
	    spec: TournamentSpec;
	    status: string;
	    error?: string;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt?: any;
	    gamesTotal: number;
	    gamesPlayed: number;
	    outcomes: GameRow[];
	    standings: PlayerScoreRow[];
	    crosstable: CrosstableData;
	    sprt?: SprtState;

	    static createFrom(source: any = {}) {
	        return new TournamentSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.spec = this.convertValues(source["spec"], TournamentSpec);
	        this.status = source["status"];
	        this.error = source["error"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	        this.gamesTotal = source["gamesTotal"];
	        this.gamesPlayed = source["gamesPlayed"];
	        this.outcomes = this.convertValues(source["outcomes"], GameRow);
	        this.standings = this.convertValues(source["standings"], PlayerScoreRow);
	        this.crosstable = this.convertValues(source["crosstable"], CrosstableData);
	        this.sprt = this.convertValues(source["sprt"], SprtState);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
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
	    NetworkPath: string;
	    InstalledAt: string;
	    BuildKey: string;
	    NetworkKey: string;
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
	        this.NetworkPath = source["NetworkPath"];
	        this.InstalledAt = source["InstalledAt"];
	        this.BuildKey = source["BuildKey"];
	        this.NetworkKey = source["NetworkKey"];
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

