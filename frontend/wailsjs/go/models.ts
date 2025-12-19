export namespace main {
	
	export class AppPackage {
	    name: string;
	    type: string;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new AppPackage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.state = source["state"];
	    }
	}
	export class Device {
	    id: string;
	    state: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.state = source["state"];
	        this.model = source["model"];
	    }
	}
	export class ScrcpyConfig {
	    maxSize: number;
	    bitRate: number;
	    maxFps: number;
	    stayAwake: boolean;
	    turnScreenOff: boolean;
	    noAudio: boolean;
	    alwaysOnTop: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScrcpyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxSize = source["maxSize"];
	        this.bitRate = source["bitRate"];
	        this.maxFps = source["maxFps"];
	        this.stayAwake = source["stayAwake"];
	        this.turnScreenOff = source["turnScreenOff"];
	        this.noAudio = source["noAudio"];
	        this.alwaysOnTop = source["alwaysOnTop"];
	    }
	}

}

