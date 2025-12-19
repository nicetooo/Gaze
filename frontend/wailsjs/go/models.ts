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

}

