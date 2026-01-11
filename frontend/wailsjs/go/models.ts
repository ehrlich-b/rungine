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

