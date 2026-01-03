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
	export class ElementActionConfig {
	    Timeout: number;
	    RetryInterval: number;
	    PreWait: number;
	    PostDelay: number;
	    OnError: string;
	
	    static createFrom(source: any = {}) {
	        return new ElementActionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Timeout = source["Timeout"];
	        this.RetryInterval = source["RetryInterval"];
	        this.PreWait = source["PreWait"];
	        this.PostDelay = source["PostDelay"];
	        this.OnError = source["OnError"];
	    }
	}
	export class ElementSelector {
	    type: string;
	    value: string;
	    index?: number;
	
	    static createFrom(source: any = {}) {
	        return new ElementSelector(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.value = source["value"];
	        this.index = source["index"];
	    }
	}
	export class ElementInfo {
	    x: number;
	    y: number;
	    class: string;
	    bounds: string;
	    selector?: ElementSelector;
	    timestamp: number;
	
	    static createFrom(source: any = {}) {
	        return new ElementInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.class = source["class"];
	        this.bounds = source["bounds"];
	        this.selector = this.convertValues(source["selector"], ElementSelector);
	        this.timestamp = source["timestamp"];
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
	    lastSeen: number;
	
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
	        this.lastSeen = source["lastSeen"];
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
	export class TaskStep {
	    type: string;
	    value: string;
	    loop: number;
	    postDelay: number;
	    checkType: string;
	    checkValue: string;
	    waitTimeout: number;
	    onFailure: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.value = source["value"];
	        this.loop = source["loop"];
	        this.postDelay = source["postDelay"];
	        this.checkType = source["checkType"];
	        this.checkValue = source["checkValue"];
	        this.waitTimeout = source["waitTimeout"];
	        this.onFailure = source["onFailure"];
	    }
	}
	export class ScriptTask {
	    name: string;
	    steps: TaskStep[];
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.steps = this.convertValues(source["steps"], TaskStep);
	        this.createdAt = source["createdAt"];
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
	export class UINode {
	    text: string;
	    resourceId: string;
	    class: string;
	    package: string;
	    contentDesc: string;
	    checkable: string;
	    checked: string;
	    clickable: string;
	    enabled: string;
	    focusable: string;
	    focused: string;
	    scrollable: string;
	    longClickable: string;
	    password: string;
	    selected: string;
	    bounds: string;
	    nodes: UINode[];
	
	    static createFrom(source: any = {}) {
	        return new UINode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.resourceId = source["resourceId"];
	        this.class = source["class"];
	        this.package = source["package"];
	        this.contentDesc = source["contentDesc"];
	        this.checkable = source["checkable"];
	        this.checked = source["checked"];
	        this.clickable = source["clickable"];
	        this.enabled = source["enabled"];
	        this.focusable = source["focusable"];
	        this.focused = source["focused"];
	        this.scrollable = source["scrollable"];
	        this.longClickable = source["longClickable"];
	        this.password = source["password"];
	        this.selected = source["selected"];
	        this.bounds = source["bounds"];
	        this.nodes = this.convertValues(source["nodes"], UINode);
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
	export class SearchResult {
	    node?: UINode;
	    path: string;
	    depth: number;
	    index: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node = this.convertValues(source["node"], UINode);
	        this.path = source["path"];
	        this.depth = source["depth"];
	        this.index = source["index"];
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
	export class SelectorSuggestion {
	    type: string;
	    value: string;
	    priority: number;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new SelectorSuggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.value = source["value"];
	        this.priority = source["priority"];
	        this.description = source["description"];
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
	    selector?: ElementSelector;
	
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
	        this.selector = this.convertValues(source["selector"], ElementSelector);
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
	export class TouchScript {
	    name: string;
	    deviceId: string;
	    deviceModel?: string;
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
	        this.deviceModel = source["deviceModel"];
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
	export class UIHierarchyResult {
	    root?: UINode;
	    rawXml: string;
	
	    static createFrom(source: any = {}) {
	        return new UIHierarchyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = this.convertValues(source["root"], UINode);
	        this.rawXml = source["rawXml"];
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
	
	export class WorkflowStep {
	    id: string;
	    type: string;
	    name?: string;
	    selector?: ElementSelector;
	    value?: string;
	    timeout?: number;
	    onError?: string;
	    loop?: number;
	    postDelay?: number;
	    preWait?: number;
	    swipeDistance?: number;
	    swipeDuration?: number;
	    conditionType?: string;
	    nextStepId?: string;
	    nextSource?: string;
	    nextTarget?: string;
	    trueStepId?: string;
	    trueSource?: string;
	    trueTarget?: string;
	    falseStepId?: string;
	    falseSource?: string;
	    falseTarget?: string;
	    posX?: number;
	    posY?: number;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.selector = this.convertValues(source["selector"], ElementSelector);
	        this.value = source["value"];
	        this.timeout = source["timeout"];
	        this.onError = source["onError"];
	        this.loop = source["loop"];
	        this.postDelay = source["postDelay"];
	        this.preWait = source["preWait"];
	        this.swipeDistance = source["swipeDistance"];
	        this.swipeDuration = source["swipeDuration"];
	        this.conditionType = source["conditionType"];
	        this.nextStepId = source["nextStepId"];
	        this.nextSource = source["nextSource"];
	        this.nextTarget = source["nextTarget"];
	        this.trueStepId = source["trueStepId"];
	        this.trueSource = source["trueSource"];
	        this.trueTarget = source["trueTarget"];
	        this.falseStepId = source["falseStepId"];
	        this.falseSource = source["falseSource"];
	        this.falseTarget = source["falseTarget"];
	        this.posX = source["posX"];
	        this.posY = source["posY"];
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
	export class Workflow {
	    id: string;
	    name: string;
	    description?: string;
	    steps: WorkflowStep[];
	    variables?: {[key: string]: string};
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Workflow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.steps = this.convertValues(source["steps"], WorkflowStep);
	        this.variables = source["variables"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
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

export namespace time {
	
	export class Time {
	
	
	    static createFrom(source: any = {}) {
	        return new Time(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace xml {
	
	export class Name {
	    Space: string;
	    Local: string;
	
	    static createFrom(source: any = {}) {
	        return new Name(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Space = source["Space"];
	        this.Local = source["Local"];
	    }
	}

}

