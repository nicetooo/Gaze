export namespace main {
	
	export class AppPackage {
	    name: string;
	    label: string;
	    icon: string;
	    type: string;
	    state: string;
	    versionName: string;
	    versionCode: string;
	    minSdkVersion: string;
	    targetSdkVersion: string;
	    permissions: string[];
	    activities: string[];
	    launchableActivities: string[];
	
	    static createFrom(source: any = {}) {
	        return new AppPackage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.label = source["label"];
	        this.icon = source["icon"];
	        this.type = source["type"];
	        this.state = source["state"];
	        this.versionName = source["versionName"];
	        this.versionCode = source["versionCode"];
	        this.minSdkVersion = source["minSdkVersion"];
	        this.targetSdkVersion = source["targetSdkVersion"];
	        this.permissions = source["permissions"];
	        this.activities = source["activities"];
	        this.launchableActivities = source["launchableActivities"];
	    }
	}
	export class BatchOperation {
	    type: string;
	    deviceIds: string[];
	    packageName: string;
	    apkPath: string;
	    command: string;
	    localPath: string;
	    remotePath: string;
	
	    static createFrom(source: any = {}) {
	        return new BatchOperation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.deviceIds = source["deviceIds"];
	        this.packageName = source["packageName"];
	        this.apkPath = source["apkPath"];
	        this.command = source["command"];
	        this.localPath = source["localPath"];
	        this.remotePath = source["remotePath"];
	    }
	}
	export class BatchResult {
	    deviceId: string;
	    success: boolean;
	    output: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new BatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceId = source["deviceId"];
	        this.success = source["success"];
	        this.output = source["output"];
	        this.error = source["error"];
	    }
	}
	export class BatchOperationResult {
	    totalDevices: number;
	    successCount: number;
	    failureCount: number;
	    results: BatchResult[];
	
	    static createFrom(source: any = {}) {
	        return new BatchOperationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalDevices = source["totalDevices"];
	        this.successCount = source["successCount"];
	        this.failureCount = source["failureCount"];
	        this.results = this.convertValues(source["results"], BatchResult);
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
	
	export class Device {
	    id: string;
	    serial: string;
	    state: string;
	    model: string;
	    brand: string;
	    type: string;
	    ids: string[];
	    wifiAddr: string;
	    lastActive: number;
	    isPinned: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.serial = source["serial"];
	        this.state = source["state"];
	        this.model = source["model"];
	        this.brand = source["brand"];
	        this.type = source["type"];
	        this.ids = source["ids"];
	        this.wifiAddr = source["wifiAddr"];
	        this.lastActive = source["lastActive"];
	        this.isPinned = source["isPinned"];
	    }
	}
	export class DeviceInfo {
	    model: string;
	    brand: string;
	    manufacturer: string;
	    androidVer: string;
	    sdk: string;
	    abi: string;
	    serial: string;
	    resolution: string;
	    density: string;
	    cpu: string;
	    memory: string;
	    props: {[key: string]: string};
	
	    static createFrom(source: any = {}) {
	        return new DeviceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.brand = source["brand"];
	        this.manufacturer = source["manufacturer"];
	        this.androidVer = source["androidVer"];
	        this.sdk = source["sdk"];
	        this.abi = source["abi"];
	        this.serial = source["serial"];
	        this.resolution = source["resolution"];
	        this.density = source["density"];
	        this.cpu = source["cpu"];
	        this.memory = source["memory"];
	        this.props = source["props"];
	    }
	}
	export class FileInfo {
	    name: string;
	    size: number;
	    mode: string;
	    modTime: string;
	    isDir: boolean;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.size = source["size"];
	        this.mode = source["mode"];
	        this.modTime = source["modTime"];
	        this.isDir = source["isDir"];
	        this.path = source["path"];
	    }
	}
	export class HistoryDevice {
	    id: string;
	    serial: string;
	    model: string;
	    brand: string;
	    type: string;
	    wifiAddr: string;
	    // Go type: time
	    lastSeen: any;
	
	    static createFrom(source: any = {}) {
	        return new HistoryDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.serial = source["serial"];
	        this.model = source["model"];
	        this.brand = source["brand"];
	        this.type = source["type"];
	        this.wifiAddr = source["wifiAddr"];
	        this.lastSeen = this.convertValues(source["lastSeen"], null);
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
	export class ScrcpyConfig {
	    maxSize: number;
	    bitRate: number;
	    maxFps: number;
	    stayAwake: boolean;
	    turnScreenOff: boolean;
	    noAudio: boolean;
	    alwaysOnTop: boolean;
	    showTouches: boolean;
	    fullscreen: boolean;
	    readOnly: boolean;
	    powerOffOnClose: boolean;
	    windowBorderless: boolean;
	    videoCodec: string;
	    audioCodec: string;
	    recordPath: string;
	    displayId: number;
	    videoSource: string;
	    cameraId: string;
	    cameraSize: string;
	    displayOrientation: string;
	    captureOrientation: string;
	    keyboardMode: string;
	    mouseMode: string;
	    noClipboardSync: boolean;
	    showFps: boolean;
	    noPowerOn: boolean;
	
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
	        this.showTouches = source["showTouches"];
	        this.fullscreen = source["fullscreen"];
	        this.readOnly = source["readOnly"];
	        this.powerOffOnClose = source["powerOffOnClose"];
	        this.windowBorderless = source["windowBorderless"];
	        this.videoCodec = source["videoCodec"];
	        this.audioCodec = source["audioCodec"];
	        this.recordPath = source["recordPath"];
	        this.displayId = source["displayId"];
	        this.videoSource = source["videoSource"];
	        this.cameraId = source["cameraId"];
	        this.cameraSize = source["cameraSize"];
	        this.displayOrientation = source["displayOrientation"];
	        this.captureOrientation = source["captureOrientation"];
	        this.keyboardMode = source["keyboardMode"];
	        this.mouseMode = source["mouseMode"];
	        this.noClipboardSync = source["noClipboardSync"];
	        this.showFps = source["showFps"];
	        this.noPowerOn = source["noPowerOn"];
	    }
	}
	export class TouchEvent {
	    timestamp: number;
	    type: string;
	    x: number;
	    y: number;
	    x2?: number;
	    y2?: number;
	    duration?: number;
	
	    static createFrom(source: any = {}) {
	        return new TouchEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.type = source["type"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.x2 = source["x2"];
	        this.y2 = source["y2"];
	        this.duration = source["duration"];
	    }
	}
	export class TouchScript {
	    name: string;
	    deviceId: string;
	    resolution: string;
	    createdAt: string;
	    events: TouchEvent[];
	
	    static createFrom(source: any = {}) {
	        return new TouchScript(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.deviceId = source["deviceId"];
	        this.resolution = source["resolution"];
	        this.createdAt = source["createdAt"];
	        this.events = this.convertValues(source["events"], TouchEvent);
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

