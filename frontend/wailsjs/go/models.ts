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

