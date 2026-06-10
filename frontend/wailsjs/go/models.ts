export namespace http {
	
	export class Response {
	    Status: string;
	    StatusCode: number;
	    Proto: string;
	    ProtoMajor: number;
	    ProtoMinor: number;
	    Header: Record<string, Array<string>>;
	    Body: any;
	    ContentLength: number;
	    TransferEncoding: string[];
	    Close: boolean;
	    Uncompressed: boolean;
	    Trailer: Record<string, Array<string>>;
	    Request?: Request;
	    TLS?: tls.ConnectionState;
	
	    static createFrom(source: any = {}) {
	        return new Response(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Status = source["Status"];
	        this.StatusCode = source["StatusCode"];
	        this.Proto = source["Proto"];
	        this.ProtoMajor = source["ProtoMajor"];
	        this.ProtoMinor = source["ProtoMinor"];
	        this.Header = source["Header"];
	        this.Body = source["Body"];
	        this.ContentLength = source["ContentLength"];
	        this.TransferEncoding = source["TransferEncoding"];
	        this.Close = source["Close"];
	        this.Uncompressed = source["Uncompressed"];
	        this.Trailer = source["Trailer"];
	        this.Request = this.convertValues(source["Request"], Request);
	        this.TLS = this.convertValues(source["TLS"], tls.ConnectionState);
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
	export class Request {
	    Method: string;
	    URL?: url.URL;
	    Proto: string;
	    ProtoMajor: number;
	    ProtoMinor: number;
	    Header: Record<string, Array<string>>;
	    Body: any;
	    ContentLength: number;
	    TransferEncoding: string[];
	    Close: boolean;
	    Host: string;
	    Form: Record<string, Array<string>>;
	    PostForm: Record<string, Array<string>>;
	    MultipartForm?: multipart.Form;
	    Trailer: Record<string, Array<string>>;
	    RemoteAddr: string;
	    RequestURI: string;
	    TLS?: tls.ConnectionState;
	    Response?: Response;
	    Pattern: string;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Method = source["Method"];
	        this.URL = this.convertValues(source["URL"], url.URL);
	        this.Proto = source["Proto"];
	        this.ProtoMajor = source["ProtoMajor"];
	        this.ProtoMinor = source["ProtoMinor"];
	        this.Header = source["Header"];
	        this.Body = source["Body"];
	        this.ContentLength = source["ContentLength"];
	        this.TransferEncoding = source["TransferEncoding"];
	        this.Close = source["Close"];
	        this.Host = source["Host"];
	        this.Form = source["Form"];
	        this.PostForm = source["PostForm"];
	        this.MultipartForm = this.convertValues(source["MultipartForm"], multipart.Form);
	        this.Trailer = source["Trailer"];
	        this.RemoteAddr = source["RemoteAddr"];
	        this.RequestURI = source["RequestURI"];
	        this.TLS = this.convertValues(source["TLS"], tls.ConnectionState);
	        this.Response = this.convertValues(source["Response"], Response);
	        this.Pattern = source["Pattern"];
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
	export class AssertionExpected {
	    exists?: boolean;
	    count?: number;
	    minCount?: number;
	    maxCount?: number;
	    sequence?: EventCriteria[];
	    ordered?: boolean;
	    minInterval?: number;
	    maxInterval?: number;
	    expression?: string;
	
	    static createFrom(source: any = {}) {
	        return new AssertionExpected(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exists = source["exists"];
	        this.count = source["count"];
	        this.minCount = source["minCount"];
	        this.maxCount = source["maxCount"];
	        this.sequence = this.convertValues(source["sequence"], EventCriteria);
	        this.ordered = source["ordered"];
	        this.minInterval = source["minInterval"];
	        this.maxInterval = source["maxInterval"];
	        this.expression = source["expression"];
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
	export class DataMatcher {
	    path: string;
	    operator: string;
	    value: any;
	
	    static createFrom(source: any = {}) {
	        return new DataMatcher(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.operator = source["operator"];
	        this.value = source["value"];
	    }
	}
	export class EventCriteria {
	    sources?: string[];
	    categories?: string[];
	    types?: string[];
	    levels?: string[];
	    titleMatch?: string;
	    dataMatch?: DataMatcher[];
	
	    static createFrom(source: any = {}) {
	        return new EventCriteria(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sources = source["sources"];
	        this.categories = source["categories"];
	        this.types = source["types"];
	        this.levels = source["levels"];
	        this.titleMatch = source["titleMatch"];
	        this.dataMatch = this.convertValues(source["dataMatch"], DataMatcher);
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
	export class TimeRange {
	    start: number;
	    end: number;
	
	    static createFrom(source: any = {}) {
	        return new TimeRange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = source["start"];
	        this.end = source["end"];
	    }
	}
	export class Assertion {
	    id: string;
	    name: string;
	    description?: string;
	    type: string;
	    sessionId?: string;
	    deviceId?: string;
	    timeRange?: TimeRange;
	    criteria: EventCriteria;
	    expected: AssertionExpected;
	    timeout?: number;
	    createdAt: number;
	    tags?: string[];
	    metadata?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Assertion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.type = source["type"];
	        this.sessionId = source["sessionId"];
	        this.deviceId = source["deviceId"];
	        this.timeRange = this.convertValues(source["timeRange"], TimeRange);
	        this.criteria = this.convertValues(source["criteria"], EventCriteria);
	        this.expected = this.convertValues(source["expected"], AssertionExpected);
	        this.timeout = source["timeout"];
	        this.createdAt = source["createdAt"];
	        this.tags = source["tags"];
	        this.metadata = source["metadata"];
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
	
	export class AssertionResult {
	    id: string;
	    assertionId: string;
	    assertionName: string;
	    sessionId: string;
	    passed: boolean;
	    message: string;
	    matchedEvents?: string[];
	    actualValue?: any;
	    expectedValue?: any;
	    executedAt: number;
	    duration: number;
	    details?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new AssertionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.assertionId = source["assertionId"];
	        this.assertionName = source["assertionName"];
	        this.sessionId = source["sessionId"];
	        this.passed = source["passed"];
	        this.message = source["message"];
	        this.matchedEvents = source["matchedEvents"];
	        this.actualValue = source["actualValue"];
	        this.expectedValue = source["expectedValue"];
	        this.executedAt = source["executedAt"];
	        this.duration = source["duration"];
	        this.details = source["details"];
	    }
	}
	export class AssertionSet {
	    id: string;
	    name: string;
	    description?: string;
	    assertions: string[];
	    createdAt: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new AssertionSet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.assertions = source["assertions"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class AssertionSetSummary {
	    total: number;
	    passed: number;
	    failed: number;
	    error: number;
	    passRate: number;
	
	    static createFrom(source: any = {}) {
	        return new AssertionSetSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.passed = source["passed"];
	        this.failed = source["failed"];
	        this.error = source["error"];
	        this.passRate = source["passRate"];
	    }
	}
	export class AssertionSetResult {
	    id: string;
	    setId: string;
	    setName: string;
	    sessionId: string;
	    deviceId: string;
	    executionId: string;
	    startTime: number;
	    endTime: number;
	    duration: number;
	    status: string;
	    summary: AssertionSetSummary;
	    results: AssertionResult[];
	    executedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new AssertionSetResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.setId = source["setId"];
	        this.setName = source["setName"];
	        this.sessionId = source["sessionId"];
	        this.deviceId = source["deviceId"];
	        this.executionId = source["executionId"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.duration = source["duration"];
	        this.status = source["status"];
	        this.summary = this.convertValues(source["summary"], AssertionSetSummary);
	        this.results = this.convertValues(source["results"], AssertionResult);
	        this.executedAt = source["executedAt"];
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
	
	export class Bookmark {
	    id: string;
	    sessionId: string;
	    relativeTime: number;
	    label: string;
	    color?: string;
	    type: string;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Bookmark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.relativeTime = source["relativeTime"];
	        this.label = source["label"];
	        this.color = source["color"];
	        this.type = source["type"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class BreakpointRule {
	    id: string;
	    urlPattern: string;
	    method: string;
	    phase: string;
	    enabled: boolean;
	    description: string;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new BreakpointRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.urlPattern = source["urlPattern"];
	        this.method = source["method"];
	        this.phase = source["phase"];
	        this.enabled = source["enabled"];
	        this.description = source["description"];
	        this.createdAt = source["createdAt"];
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
	    props: Record<string, string>;
	
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
	export class MonitorConfig {
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MonitorConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	    }
	}
	export class ProxyConfig {
	    enabled: boolean;
	    port?: number;
	    mitmEnabled?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.port = source["port"];
	        this.mitmEnabled = source["mitmEnabled"];
	    }
	}
	export class RecordingConfig {
	    enabled: boolean;
	    quality?: string;
	
	    static createFrom(source: any = {}) {
	        return new RecordingConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.quality = source["quality"];
	    }
	}
	export class LogcatConfig {
	    enabled: boolean;
	    packageName?: string;
	    preFilter?: string;
	    excludeFilter?: string;
	
	    static createFrom(source: any = {}) {
	        return new LogcatConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.packageName = source["packageName"];
	        this.preFilter = source["preFilter"];
	        this.excludeFilter = source["excludeFilter"];
	    }
	}
	export class SessionConfig {
	    logcat: LogcatConfig;
	    recording: RecordingConfig;
	    proxy: ProxyConfig;
	    monitor: MonitorConfig;
	
	    static createFrom(source: any = {}) {
	        return new SessionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.logcat = this.convertValues(source["logcat"], LogcatConfig);
	        this.recording = this.convertValues(source["recording"], RecordingConfig);
	        this.proxy = this.convertValues(source["proxy"], ProxyConfig);
	        this.monitor = this.convertValues(source["monitor"], MonitorConfig);
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
	export class DeviceSession {
	    id: string;
	    deviceId: string;
	    type: string;
	    name: string;
	    startTime: number;
	    endTime: number;
	    status: string;
	    eventCount: number;
	    config: SessionConfig;
	    videoPath?: string;
	    videoDuration?: number;
	    videoOffset?: number;
	    metadata?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new DeviceSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.deviceId = source["deviceId"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.status = source["status"];
	        this.eventCount = source["eventCount"];
	        this.config = this.convertValues(source["config"], SessionConfig);
	        this.videoPath = source["videoPath"];
	        this.videoDuration = source["videoDuration"];
	        this.videoOffset = source["videoOffset"];
	        this.metadata = source["metadata"];
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
	export class ElementInfo {
	    x: number;
	    y: number;
	    class: string;
	    bounds: string;
	    selector?: types.ElementSelector;
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
	        this.selector = this.convertValues(source["selector"], types.ElementSelector);
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
	
	export class EventQuery {
	    sessionId?: string;
	    deviceId?: string;
	    sources?: string[];
	    categories?: string[];
	    types?: string[];
	    levels?: string[];
	    startTime?: number;
	    endTime?: number;
	    searchText?: string;
	    parentId?: string;
	    stepId?: string;
	    traceId?: string;
	    limit?: number;
	    offset?: number;
	    orderDesc?: boolean;
	    includeData?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EventQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.deviceId = source["deviceId"];
	        this.sources = source["sources"];
	        this.categories = source["categories"];
	        this.types = source["types"];
	        this.levels = source["levels"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.searchText = source["searchText"];
	        this.parentId = source["parentId"];
	        this.stepId = source["stepId"];
	        this.traceId = source["traceId"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	        this.orderDesc = source["orderDesc"];
	        this.includeData = source["includeData"];
	    }
	}
	export class UnifiedEvent {
	    id: string;
	    sessionId: string;
	    deviceId: string;
	    timestamp: number;
	    relativeTime: number;
	    duration?: number;
	    source: string;
	    category: string;
	    type: string;
	    level: string;
	    title: string;
	    summary?: string;
	    data?: number[];
	    detail?: number[];
	    parentId?: string;
	    stepId?: string;
	    traceId?: string;
	    aggregateCount?: number;
	    aggregateFirst?: number;
	    aggregateLast?: number;
	    tags?: string[];
	    metadata?: Record<string, any>;
	    parentEventId?: string;
	    generatedByPlugin?: string;
	    derivedDepth?: number;
	
	    static createFrom(source: any = {}) {
	        return new UnifiedEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.deviceId = source["deviceId"];
	        this.timestamp = source["timestamp"];
	        this.relativeTime = source["relativeTime"];
	        this.duration = source["duration"];
	        this.source = source["source"];
	        this.category = source["category"];
	        this.type = source["type"];
	        this.level = source["level"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.data = source["data"];
	        this.detail = source["detail"];
	        this.parentId = source["parentId"];
	        this.stepId = source["stepId"];
	        this.traceId = source["traceId"];
	        this.aggregateCount = source["aggregateCount"];
	        this.aggregateFirst = source["aggregateFirst"];
	        this.aggregateLast = source["aggregateLast"];
	        this.tags = source["tags"];
	        this.metadata = source["metadata"];
	        this.parentEventId = source["parentEventId"];
	        this.generatedByPlugin = source["generatedByPlugin"];
	        this.derivedDepth = source["derivedDepth"];
	    }
	}
	export class EventQueryResult {
	    events: UnifiedEvent[];
	    total: number;
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EventQueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.events = this.convertValues(source["events"], UnifiedEvent);
	        this.total = source["total"];
	        this.hasMore = source["hasMore"];
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
	
	export class MapRemoteRule {
	    id: string;
	    sourcePattern: string;
	    targetUrl: string;
	    method: string;
	    enabled: boolean;
	    description: string;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new MapRemoteRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sourcePattern = source["sourcePattern"];
	        this.targetUrl = source["targetUrl"];
	        this.method = source["method"];
	        this.enabled = source["enabled"];
	        this.description = source["description"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class MockCondition {
	    type: string;
	    key: string;
	    operator: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new MockCondition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.key = source["key"];
	        this.operator = source["operator"];
	        this.value = source["value"];
	    }
	}
	export class MockRule {
	    id: string;
	    urlPattern: string;
	    method: string;
	    statusCode: number;
	    headers: Record<string, string>;
	    body: string;
	    bodyFile: string;
	    delay: number;
	    enabled: boolean;
	    description: string;
	    createdAt: number;
	    conditions: MockCondition[];
	
	    static createFrom(source: any = {}) {
	        return new MockRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.urlPattern = source["urlPattern"];
	        this.method = source["method"];
	        this.statusCode = source["statusCode"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.bodyFile = source["bodyFile"];
	        this.delay = source["delay"];
	        this.enabled = source["enabled"];
	        this.description = source["description"];
	        this.createdAt = source["createdAt"];
	        this.conditions = this.convertValues(source["conditions"], MockCondition);
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
	
	export class PerfMonitorConfig {
	    packageName?: string;
	    intervalMs: number;
	    enableCPU: boolean;
	    enableMemory: boolean;
	    enableFPS: boolean;
	    enableNetwork: boolean;
	    enableBattery: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PerfMonitorConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packageName = source["packageName"];
	        this.intervalMs = source["intervalMs"];
	        this.enableCPU = source["enableCPU"];
	        this.enableMemory = source["enableMemory"];
	        this.enableFPS = source["enableFPS"];
	        this.enableNetwork = source["enableNetwork"];
	        this.enableBattery = source["enableBattery"];
	    }
	}
	export class ProcessPerfData {
	    pid: number;
	    name: string;
	    cpu: number;
	    memoryKB: number;
	    user: number;
	    kernel: number;
	    linuxUser: string;
	    ppid: number;
	    vszKB: number;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new ProcessPerfData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.name = source["name"];
	        this.cpu = source["cpu"];
	        this.memoryKB = source["memoryKB"];
	        this.user = source["user"];
	        this.kernel = source["kernel"];
	        this.linuxUser = source["linuxUser"];
	        this.ppid = source["ppid"];
	        this.vszKB = source["vszKB"];
	        this.state = source["state"];
	    }
	}
	export class PerfSampleData {
	    cpuUsage: number;
	    cpuApp: number;
	    cpuCores: number;
	    cpuFreqMHz: number;
	    cpuTempC: number;
	    memTotalMB: number;
	    memUsedMB: number;
	    memFreeMB: number;
	    memUsage: number;
	    memAppMB: number;
	    fps: number;
	    jankCount: number;
	    netRxKBps: number;
	    netTxKBps: number;
	    netRxTotalMB: number;
	    netTxTotalMB: number;
	    batteryLevel: number;
	    batteryTemp: number;
	    packageName?: string;
	    processes?: ProcessPerfData[];
	
	    static createFrom(source: any = {}) {
	        return new PerfSampleData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuUsage = source["cpuUsage"];
	        this.cpuApp = source["cpuApp"];
	        this.cpuCores = source["cpuCores"];
	        this.cpuFreqMHz = source["cpuFreqMHz"];
	        this.cpuTempC = source["cpuTempC"];
	        this.memTotalMB = source["memTotalMB"];
	        this.memUsedMB = source["memUsedMB"];
	        this.memFreeMB = source["memFreeMB"];
	        this.memUsage = source["memUsage"];
	        this.memAppMB = source["memAppMB"];
	        this.fps = source["fps"];
	        this.jankCount = source["jankCount"];
	        this.netRxKBps = source["netRxKBps"];
	        this.netTxKBps = source["netTxKBps"];
	        this.netRxTotalMB = source["netRxTotalMB"];
	        this.netTxTotalMB = source["netTxTotalMB"];
	        this.batteryLevel = source["batteryLevel"];
	        this.batteryTemp = source["batteryTemp"];
	        this.packageName = source["packageName"];
	        this.processes = this.convertValues(source["processes"], ProcessPerfData);
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
	export class PluginFilters {
	    sources: string[];
	    types: string[];
	    levels: string[];
	    urlPattern: string;
	    titleMatch: string;
	
	    static createFrom(source: any = {}) {
	        return new PluginFilters(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sources = source["sources"];
	        this.types = source["types"];
	        this.levels = source["levels"];
	        this.urlPattern = source["urlPattern"];
	        this.titleMatch = source["titleMatch"];
	    }
	}
	export class PluginMetadata {
	    id: string;
	    name: string;
	    version: string;
	    author: string;
	    description: string;
	    enabled: boolean;
	    filters: PluginFilters;
	    config: Record<string, any>;
	    createdAt: time.Time;
	    updatedAt: time.Time;
	
	    static createFrom(source: any = {}) {
	        return new PluginMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.description = source["description"];
	        this.enabled = source["enabled"];
	        this.filters = this.convertValues(source["filters"], PluginFilters);
	        this.config = source["config"];
	        this.createdAt = this.convertValues(source["createdAt"], time.Time);
	        this.updatedAt = this.convertValues(source["updatedAt"], time.Time);
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
	export class Plugin {
	    metadata: PluginMetadata;
	    sourceCode: string;
	    language: string;
	    compiledCode: string;
	
	    static createFrom(source: any = {}) {
	        return new Plugin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.metadata = this.convertValues(source["metadata"], PluginMetadata);
	        this.sourceCode = source["sourceCode"];
	        this.language = source["language"];
	        this.compiledCode = source["compiledCode"];
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
	
	
	export class PluginSaveRequest {
	    id: string;
	    name: string;
	    version: string;
	    author: string;
	    description: string;
	    sourceCode: string;
	    language: string;
	    compiledCode: string;
	    filters: PluginFilters;
	    config: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new PluginSaveRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.description = source["description"];
	        this.sourceCode = source["sourceCode"];
	        this.language = source["language"];
	        this.compiledCode = source["compiledCode"];
	        this.filters = this.convertValues(source["filters"], PluginFilters);
	        this.config = source["config"];
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
	export class PluginTestResult {
	    success: boolean;
	    derivedEvents: UnifiedEvent[];
	    executionTimeMs: number;
	    error?: string;
	    logs?: string[];
	    matchedFilters: boolean;
	    eventSnapshot?: UnifiedEvent;
	
	    static createFrom(source: any = {}) {
	        return new PluginTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.derivedEvents = this.convertValues(source["derivedEvents"], UnifiedEvent);
	        this.executionTimeMs = source["executionTimeMs"];
	        this.error = source["error"];
	        this.logs = source["logs"];
	        this.matchedFilters = source["matchedFilters"];
	        this.eventSnapshot = this.convertValues(source["eventSnapshot"], UnifiedEvent);
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
	export class ProcessObjects {
	    views: number;
	    viewRootImpl: number;
	    appContexts: number;
	    activities: number;
	    assets: number;
	    assetManagers: number;
	    localBinders: number;
	    proxyBinders: number;
	    deathRecipients: number;
	    webViews: number;
	
	    static createFrom(source: any = {}) {
	        return new ProcessObjects(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.views = source["views"];
	        this.viewRootImpl = source["viewRootImpl"];
	        this.appContexts = source["appContexts"];
	        this.activities = source["activities"];
	        this.assets = source["assets"];
	        this.assetManagers = source["assetManagers"];
	        this.localBinders = source["localBinders"];
	        this.proxyBinders = source["proxyBinders"];
	        this.deathRecipients = source["deathRecipients"];
	        this.webViews = source["webViews"];
	    }
	}
	export class ProcessMemoryCategory {
	    name: string;
	    pssKB: number;
	    rssKB: number;
	
	    static createFrom(source: any = {}) {
	        return new ProcessMemoryCategory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.pssKB = source["pssKB"];
	        this.rssKB = source["rssKB"];
	    }
	}
	export class ProcessDetail {
	    pid: number;
	    packageName: string;
	    totalPssKB: number;
	    totalRssKB: number;
	    swapPssKB: number;
	    memory: ProcessMemoryCategory[];
	    javaHeapSizeKB: number;
	    javaHeapAllocKB: number;
	    javaHeapFreeKB: number;
	    nativeHeapSizeKB: number;
	    nativeHeapAllocKB: number;
	    nativeHeapFreeKB: number;
	    objects: ProcessObjects;
	    threads: number;
	    fdSize: number;
	    vmSwapKB: number;
	    oomScoreAdj: number;
	    uid: number;
	
	    static createFrom(source: any = {}) {
	        return new ProcessDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.packageName = source["packageName"];
	        this.totalPssKB = source["totalPssKB"];
	        this.totalRssKB = source["totalRssKB"];
	        this.swapPssKB = source["swapPssKB"];
	        this.memory = this.convertValues(source["memory"], ProcessMemoryCategory);
	        this.javaHeapSizeKB = source["javaHeapSizeKB"];
	        this.javaHeapAllocKB = source["javaHeapAllocKB"];
	        this.javaHeapFreeKB = source["javaHeapFreeKB"];
	        this.nativeHeapSizeKB = source["nativeHeapSizeKB"];
	        this.nativeHeapAllocKB = source["nativeHeapAllocKB"];
	        this.nativeHeapFreeKB = source["nativeHeapFreeKB"];
	        this.objects = this.convertValues(source["objects"], ProcessObjects);
	        this.threads = source["threads"];
	        this.fdSize = source["fdSize"];
	        this.vmSwapKB = source["vmSwapKB"];
	        this.oomScoreAdj = source["oomScoreAdj"];
	        this.uid = source["uid"];
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
	
	
	
	export class ProtoFileEntry {
	    id: string;
	    name: string;
	    content: string;
	    loadedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new ProtoFileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.loadedAt = source["loadedAt"];
	    }
	}
	export class ProtoMapping {
	    id: string;
	    urlPattern: string;
	    messageType: string;
	    direction: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new ProtoMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.urlPattern = source["urlPattern"];
	        this.messageType = source["messageType"];
	        this.direction = source["direction"];
	        this.description = source["description"];
	    }
	}
	
	
	export class RewriteRule {
	    id: string;
	    urlPattern: string;
	    method: string;
	    phase: string;
	    target: string;
	    headerName: string;
	    match: string;
	    replace: string;
	    enabled: boolean;
	    description: string;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new RewriteRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.urlPattern = source["urlPattern"];
	        this.method = source["method"];
	        this.phase = source["phase"];
	        this.target = source["target"];
	        this.headerName = source["headerName"];
	        this.match = source["match"];
	        this.replace = source["replace"];
	        this.enabled = source["enabled"];
	        this.description = source["description"];
	        this.createdAt = source["createdAt"];
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
	    checkable: boolean;
	    checked: boolean;
	    clickable: boolean;
	    enabled: boolean;
	    focusable: boolean;
	    focused: boolean;
	    scrollable: boolean;
	    longClickable: boolean;
	    password: boolean;
	    selected: boolean;
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
	
	export class StoredAssertion {
	    id: string;
	    name: string;
	    description?: string;
	    type: string;
	    sessionId?: string;
	    deviceId?: string;
	    // Go type: struct { Start int64 "json:\"start\""; End int64 "json:\"end\"" }
	    timeRange?: any;
	    criteria: number[];
	    expected: number[];
	    timeout?: number;
	    tags?: string[];
	    metadata?: number[];
	    isTemplate: boolean;
	    createdAt: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new StoredAssertion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.type = source["type"];
	        this.sessionId = source["sessionId"];
	        this.deviceId = source["deviceId"];
	        this.timeRange = this.convertValues(source["timeRange"], Object);
	        this.criteria = source["criteria"];
	        this.expected = source["expected"];
	        this.timeout = source["timeout"];
	        this.tags = source["tags"];
	        this.metadata = source["metadata"];
	        this.isTemplate = source["isTemplate"];
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
	
	export class TimeIndexEntry {
	    second: number;
	    eventCount: number;
	    firstEventId: string;
	    hasError: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TimeIndexEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.second = source["second"];
	        this.eventCount = source["eventCount"];
	        this.firstEventId = source["firstEventId"];
	        this.hasError = source["hasError"];
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
	    selector?: types.ElementSelector;
	
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
	        this.selector = this.convertValues(source["selector"], types.ElementSelector);
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
	    smartTapTimeoutMs?: number;
	    playbackSpeed?: number;
	
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
	        this.smartTapTimeoutMs = source["smartTapTimeoutMs"];
	        this.playbackSpeed = source["playbackSpeed"];
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
	
	
	export class VideoMetadata {
	    path: string;
	    duration: number;
	    durationMs: number;
	    width: number;
	    height: number;
	    frameRate: number;
	    codec: string;
	    bitRate: number;
	    totalFrames: number;
	    thumbnailPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new VideoMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.duration = source["duration"];
	        this.durationMs = source["durationMs"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.frameRate = source["frameRate"];
	        this.codec = source["codec"];
	        this.bitRate = source["bitRate"];
	        this.totalFrames = source["totalFrames"];
	        this.thumbnailPath = source["thumbnailPath"];
	    }
	}
	export class VideoServiceInfo {
	    available: boolean;
	    ffmpegPath?: string;
	    ffprobePath?: string;
	
	    static createFrom(source: any = {}) {
	        return new VideoServiceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ffprobePath = source["ffprobePath"];
	    }
	}
	export class VideoThumbnail {
	    timeMs: number;
	    base64: string;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new VideoThumbnail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timeMs = source["timeMs"];
	        this.base64 = source["base64"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}

}

export namespace multipart {
	
	export class FileHeader {
	    Filename: string;
	    Header: Record<string, Array<string>>;
	    Size: number;
	
	    static createFrom(source: any = {}) {
	        return new FileHeader(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Filename = source["Filename"];
	        this.Header = source["Header"];
	        this.Size = source["Size"];
	    }
	}
	export class Form {
	    Value: Record<string, Array<string>>;
	    File: Record<string, Array<FileHeader>>;
	
	    static createFrom(source: any = {}) {
	        return new Form(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Value = source["Value"];
	        this.File = this.convertValues(source["File"], Array<FileHeader>, true);
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

export namespace net {
	
	export class IPNet {
	    IP: number[];
	    Mask: number[];
	
	    static createFrom(source: any = {}) {
	        return new IPNet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.IP = source["IP"];
	        this.Mask = source["Mask"];
	    }
	}

}

export namespace pkix {
	
	export class AttributeTypeAndValue {
	    Type: number[];
	    Value: any;
	
	    static createFrom(source: any = {}) {
	        return new AttributeTypeAndValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Type = source["Type"];
	        this.Value = source["Value"];
	    }
	}
	export class Extension {
	    Id: number[];
	    Critical: boolean;
	    Value: number[];
	
	    static createFrom(source: any = {}) {
	        return new Extension(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Id = source["Id"];
	        this.Critical = source["Critical"];
	        this.Value = source["Value"];
	    }
	}
	export class Name {
	    Country: string[];
	    Organization: string[];
	    OrganizationalUnit: string[];
	    Locality: string[];
	    Province: string[];
	    StreetAddress: string[];
	    PostalCode: string[];
	    SerialNumber: string;
	    CommonName: string;
	    Names: AttributeTypeAndValue[];
	    ExtraNames: AttributeTypeAndValue[];
	
	    static createFrom(source: any = {}) {
	        return new Name(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Country = source["Country"];
	        this.Organization = source["Organization"];
	        this.OrganizationalUnit = source["OrganizationalUnit"];
	        this.Locality = source["Locality"];
	        this.Province = source["Province"];
	        this.StreetAddress = source["StreetAddress"];
	        this.PostalCode = source["PostalCode"];
	        this.SerialNumber = source["SerialNumber"];
	        this.CommonName = source["CommonName"];
	        this.Names = this.convertValues(source["Names"], AttributeTypeAndValue);
	        this.ExtraNames = this.convertValues(source["ExtraNames"], AttributeTypeAndValue);
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

export namespace proxy {
	
	export class PendingBreakpointInfo {
	    id: string;
	    ruleId: string;
	    phase: string;
	    method: string;
	    url: string;
	    headers?: Record<string, Array<string>>;
	    body?: string;
	    statusCode?: number;
	    respHeaders?: Record<string, Array<string>>;
	    respBody?: string;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new PendingBreakpointInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ruleId = source["ruleId"];
	        this.phase = source["phase"];
	        this.method = source["method"];
	        this.url = source["url"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.statusCode = source["statusCode"];
	        this.respHeaders = source["respHeaders"];
	        this.respBody = source["respBody"];
	        this.createdAt = source["createdAt"];
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

export namespace tls {
	
	export class ConnectionState {
	    Version: number;
	    HandshakeComplete: boolean;
	    DidResume: boolean;
	    CipherSuite: number;
	    CurveID: number;
	    NegotiatedProtocol: string;
	    NegotiatedProtocolIsMutual: boolean;
	    ServerName: string;
	    PeerCertificates: x509.Certificate[];
	    VerifiedChains: x509.Certificate[][];
	    SignedCertificateTimestamps: number[][];
	    OCSPResponse: number[];
	    TLSUnique: number[];
	    ECHAccepted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Version = source["Version"];
	        this.HandshakeComplete = source["HandshakeComplete"];
	        this.DidResume = source["DidResume"];
	        this.CipherSuite = source["CipherSuite"];
	        this.CurveID = source["CurveID"];
	        this.NegotiatedProtocol = source["NegotiatedProtocol"];
	        this.NegotiatedProtocolIsMutual = source["NegotiatedProtocolIsMutual"];
	        this.ServerName = source["ServerName"];
	        this.PeerCertificates = this.convertValues(source["PeerCertificates"], x509.Certificate);
	        this.VerifiedChains = this.convertValues(source["VerifiedChains"], x509.Certificate);
	        this.SignedCertificateTimestamps = source["SignedCertificateTimestamps"];
	        this.OCSPResponse = source["OCSPResponse"];
	        this.TLSUnique = source["TLSUnique"];
	        this.ECHAccepted = source["ECHAccepted"];
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

export namespace types {
	
	export class ADBParams {
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new ADBParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	    }
	}
	export class AppParams {
	    packageName: string;
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new AppParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packageName = source["packageName"];
	        this.action = source["action"];
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
	export class BranchParams {
	    condition: string;
	    selector?: ElementSelector;
	    expectedValue?: string;
	    variableName?: string;
	
	    static createFrom(source: any = {}) {
	        return new BranchParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.condition = source["condition"];
	        this.selector = this.convertValues(source["selector"], ElementSelector);
	        this.expectedValue = source["expectedValue"];
	        this.variableName = source["variableName"];
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
	export class ElementParams {
	    selector: ElementSelector;
	    action: string;
	    inputText?: string;
	    swipeDir?: string;
	    swipeDistance?: number;
	    swipeDuration?: number;
	
	    static createFrom(source: any = {}) {
	        return new ElementParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selector = this.convertValues(source["selector"], ElementSelector);
	        this.action = source["action"];
	        this.inputText = source["inputText"];
	        this.swipeDir = source["swipeDir"];
	        this.swipeDistance = source["swipeDistance"];
	        this.swipeDuration = source["swipeDuration"];
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
	
	export class HandleInfo {
	    sourceHandle?: string;
	    targetHandle?: string;
	
	    static createFrom(source: any = {}) {
	        return new HandleInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceHandle = source["sourceHandle"];
	        this.targetHandle = source["targetHandle"];
	    }
	}
	export class ReadToVariableParams {
	    selector: ElementSelector;
	    variableName: string;
	    attribute?: string;
	    regex?: string;
	    defaultValue?: string;
	
	    static createFrom(source: any = {}) {
	        return new ReadToVariableParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selector = this.convertValues(source["selector"], ElementSelector);
	        this.variableName = source["variableName"];
	        this.attribute = source["attribute"];
	        this.regex = source["regex"];
	        this.defaultValue = source["defaultValue"];
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
	export class ScriptParams {
	    scriptName: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scriptName = source["scriptName"];
	    }
	}
	export class SessionParams {
	    sessionName?: string;
	    logcatEnabled?: boolean;
	    logcatPackageName?: string;
	    logcatPreFilter?: string;
	    logcatExcludeFilter?: string;
	    recordingEnabled?: boolean;
	    recordingQuality?: string;
	    proxyEnabled?: boolean;
	    proxyPort?: number;
	    proxyMitmEnabled?: boolean;
	    monitorEnabled?: boolean;
	    status?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionName = source["sessionName"];
	        this.logcatEnabled = source["logcatEnabled"];
	        this.logcatPackageName = source["logcatPackageName"];
	        this.logcatPreFilter = source["logcatPreFilter"];
	        this.logcatExcludeFilter = source["logcatExcludeFilter"];
	        this.recordingEnabled = source["recordingEnabled"];
	        this.recordingQuality = source["recordingQuality"];
	        this.proxyEnabled = source["proxyEnabled"];
	        this.proxyPort = source["proxyPort"];
	        this.proxyMitmEnabled = source["proxyMitmEnabled"];
	        this.monitorEnabled = source["monitorEnabled"];
	        this.status = source["status"];
	    }
	}
	export class StepCommon {
	    timeout?: number;
	    onError?: string;
	    loop?: number;
	    postDelay?: number;
	    preWait?: number;
	
	    static createFrom(source: any = {}) {
	        return new StepCommon(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timeout = source["timeout"];
	        this.onError = source["onError"];
	        this.loop = source["loop"];
	        this.postDelay = source["postDelay"];
	        this.preWait = source["preWait"];
	    }
	}
	export class StepConnections {
	    successStepId?: string;
	    errorStepId?: string;
	    trueStepId?: string;
	    falseStepId?: string;
	
	    static createFrom(source: any = {}) {
	        return new StepConnections(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.successStepId = source["successStepId"];
	        this.errorStepId = source["errorStepId"];
	        this.trueStepId = source["trueStepId"];
	        this.falseStepId = source["falseStepId"];
	    }
	}
	export class StepLayout {
	    posX?: number;
	    posY?: number;
	    handles?: Record<string, HandleInfo>;
	
	    static createFrom(source: any = {}) {
	        return new StepLayout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.posX = source["posX"];
	        this.posY = source["posY"];
	        this.handles = this.convertValues(source["handles"], HandleInfo, true);
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
	export class SubWorkflowParams {
	    workflowId: string;
	
	    static createFrom(source: any = {}) {
	        return new SubWorkflowParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workflowId = source["workflowId"];
	    }
	}
	export class SwipeParams {
	    x?: number;
	    y?: number;
	    x2?: number;
	    y2?: number;
	    direction?: string;
	    distance?: number;
	    duration?: number;
	
	    static createFrom(source: any = {}) {
	        return new SwipeParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.x2 = source["x2"];
	        this.y2 = source["y2"];
	        this.direction = source["direction"];
	        this.distance = source["distance"];
	        this.duration = source["duration"];
	    }
	}
	export class TapParams {
	    x: number;
	    y: number;
	
	    static createFrom(source: any = {}) {
	        return new TapParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class VariableParams {
	    name: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new VariableParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	    }
	}
	export class WaitParams {
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new WaitParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.durationMs = source["durationMs"];
	    }
	}
	export class WorkflowStep {
	    id: string;
	    type: string;
	    name?: string;
	    common?: StepCommon;
	    connections?: StepConnections;
	    tap?: TapParams;
	    swipe?: SwipeParams;
	    element?: ElementParams;
	    app?: AppParams;
	    branch?: BranchParams;
	    wait?: WaitParams;
	    script?: ScriptParams;
	    variable?: VariableParams;
	    adb?: ADBParams;
	    workflow?: SubWorkflowParams;
	    readToVariable?: ReadToVariableParams;
	    session?: SessionParams;
	    layout?: StepLayout;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.common = this.convertValues(source["common"], StepCommon);
	        this.connections = this.convertValues(source["connections"], StepConnections);
	        this.tap = this.convertValues(source["tap"], TapParams);
	        this.swipe = this.convertValues(source["swipe"], SwipeParams);
	        this.element = this.convertValues(source["element"], ElementParams);
	        this.app = this.convertValues(source["app"], AppParams);
	        this.branch = this.convertValues(source["branch"], BranchParams);
	        this.wait = this.convertValues(source["wait"], WaitParams);
	        this.script = this.convertValues(source["script"], ScriptParams);
	        this.variable = this.convertValues(source["variable"], VariableParams);
	        this.adb = this.convertValues(source["adb"], ADBParams);
	        this.workflow = this.convertValues(source["workflow"], SubWorkflowParams);
	        this.readToVariable = this.convertValues(source["readToVariable"], ReadToVariableParams);
	        this.session = this.convertValues(source["session"], SessionParams);
	        this.layout = this.convertValues(source["layout"], StepLayout);
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
	    version?: number;
	    steps: WorkflowStep[];
	    variables?: Record<string, string>;
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
	        this.version = source["version"];
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
	export class WorkflowExecutionResult {
	    workflowId: string;
	    workflowName: string;
	    status: string;
	    error?: string;
	    startTime: number;
	    endTime: number;
	    duration: number;
	    stepsTotal: number;
	    currentStepId?: string;
	    currentStepName?: string;
	    currentStepType?: string;
	    variables?: Record<string, string>;
	    stepsExecuted: number;
	    isPaused: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowExecutionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workflowId = source["workflowId"];
	        this.workflowName = source["workflowName"];
	        this.status = source["status"];
	        this.error = source["error"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.duration = source["duration"];
	        this.stepsTotal = source["stepsTotal"];
	        this.currentStepId = source["currentStepId"];
	        this.currentStepName = source["currentStepName"];
	        this.currentStepType = source["currentStepType"];
	        this.variables = source["variables"];
	        this.stepsExecuted = source["stepsExecuted"];
	        this.isPaused = source["isPaused"];
	    }
	}

}

export namespace url {
	
	export class Userinfo {
	
	
	    static createFrom(source: any = {}) {
	        return new Userinfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class URL {
	    Scheme: string;
	    Opaque: string;
	    // Go type: Userinfo
	    User?: any;
	    Host: string;
	    Path: string;
	    RawPath: string;
	    OmitHost: boolean;
	    ForceQuery: boolean;
	    RawQuery: string;
	    Fragment: string;
	    RawFragment: string;
	
	    static createFrom(source: any = {}) {
	        return new URL(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Scheme = source["Scheme"];
	        this.Opaque = source["Opaque"];
	        this.User = this.convertValues(source["User"], null);
	        this.Host = source["Host"];
	        this.Path = source["Path"];
	        this.RawPath = source["RawPath"];
	        this.OmitHost = source["OmitHost"];
	        this.ForceQuery = source["ForceQuery"];
	        this.RawQuery = source["RawQuery"];
	        this.Fragment = source["Fragment"];
	        this.RawFragment = source["RawFragment"];
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

export namespace x509 {
	
	export class PolicyMapping {
	    // Go type: OID
	    IssuerDomainPolicy: any;
	    // Go type: OID
	    SubjectDomainPolicy: any;
	
	    static createFrom(source: any = {}) {
	        return new PolicyMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.IssuerDomainPolicy = this.convertValues(source["IssuerDomainPolicy"], null);
	        this.SubjectDomainPolicy = this.convertValues(source["SubjectDomainPolicy"], null);
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
	export class OID {
	
	
	    static createFrom(source: any = {}) {
	        return new OID(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class Certificate {
	    Raw: number[];
	    RawTBSCertificate: number[];
	    RawSubjectPublicKeyInfo: number[];
	    RawSubject: number[];
	    RawIssuer: number[];
	    Signature: number[];
	    SignatureAlgorithm: number;
	    PublicKeyAlgorithm: number;
	    PublicKey: any;
	    Version: number;
	    // Go type: big
	    SerialNumber?: any;
	    Issuer: pkix.Name;
	    Subject: pkix.Name;
	    NotBefore: time.Time;
	    NotAfter: time.Time;
	    KeyUsage: number;
	    Extensions: pkix.Extension[];
	    ExtraExtensions: pkix.Extension[];
	    UnhandledCriticalExtensions: number[][];
	    ExtKeyUsage: number[];
	    UnknownExtKeyUsage: number[][];
	    BasicConstraintsValid: boolean;
	    IsCA: boolean;
	    MaxPathLen: number;
	    MaxPathLenZero: boolean;
	    SubjectKeyId: number[];
	    AuthorityKeyId: number[];
	    OCSPServer: string[];
	    IssuingCertificateURL: string[];
	    DNSNames: string[];
	    EmailAddresses: string[];
	    IPAddresses: number[][];
	    URIs: url.URL[];
	    PermittedDNSDomainsCritical: boolean;
	    PermittedDNSDomains: string[];
	    ExcludedDNSDomains: string[];
	    PermittedIPRanges: net.IPNet[];
	    ExcludedIPRanges: net.IPNet[];
	    PermittedEmailAddresses: string[];
	    ExcludedEmailAddresses: string[];
	    PermittedURIDomains: string[];
	    ExcludedURIDomains: string[];
	    CRLDistributionPoints: string[];
	    PolicyIdentifiers: number[][];
	    Policies: OID[];
	    InhibitAnyPolicy: number;
	    InhibitAnyPolicyZero: boolean;
	    InhibitPolicyMapping: number;
	    InhibitPolicyMappingZero: boolean;
	    RequireExplicitPolicy: number;
	    RequireExplicitPolicyZero: boolean;
	    PolicyMappings: PolicyMapping[];
	
	    static createFrom(source: any = {}) {
	        return new Certificate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Raw = source["Raw"];
	        this.RawTBSCertificate = source["RawTBSCertificate"];
	        this.RawSubjectPublicKeyInfo = source["RawSubjectPublicKeyInfo"];
	        this.RawSubject = source["RawSubject"];
	        this.RawIssuer = source["RawIssuer"];
	        this.Signature = source["Signature"];
	        this.SignatureAlgorithm = source["SignatureAlgorithm"];
	        this.PublicKeyAlgorithm = source["PublicKeyAlgorithm"];
	        this.PublicKey = source["PublicKey"];
	        this.Version = source["Version"];
	        this.SerialNumber = this.convertValues(source["SerialNumber"], null);
	        this.Issuer = this.convertValues(source["Issuer"], pkix.Name);
	        this.Subject = this.convertValues(source["Subject"], pkix.Name);
	        this.NotBefore = this.convertValues(source["NotBefore"], time.Time);
	        this.NotAfter = this.convertValues(source["NotAfter"], time.Time);
	        this.KeyUsage = source["KeyUsage"];
	        this.Extensions = this.convertValues(source["Extensions"], pkix.Extension);
	        this.ExtraExtensions = this.convertValues(source["ExtraExtensions"], pkix.Extension);
	        this.UnhandledCriticalExtensions = source["UnhandledCriticalExtensions"];
	        this.ExtKeyUsage = source["ExtKeyUsage"];
	        this.UnknownExtKeyUsage = source["UnknownExtKeyUsage"];
	        this.BasicConstraintsValid = source["BasicConstraintsValid"];
	        this.IsCA = source["IsCA"];
	        this.MaxPathLen = source["MaxPathLen"];
	        this.MaxPathLenZero = source["MaxPathLenZero"];
	        this.SubjectKeyId = source["SubjectKeyId"];
	        this.AuthorityKeyId = source["AuthorityKeyId"];
	        this.OCSPServer = source["OCSPServer"];
	        this.IssuingCertificateURL = source["IssuingCertificateURL"];
	        this.DNSNames = source["DNSNames"];
	        this.EmailAddresses = source["EmailAddresses"];
	        this.IPAddresses = source["IPAddresses"];
	        this.URIs = this.convertValues(source["URIs"], url.URL);
	        this.PermittedDNSDomainsCritical = source["PermittedDNSDomainsCritical"];
	        this.PermittedDNSDomains = source["PermittedDNSDomains"];
	        this.ExcludedDNSDomains = source["ExcludedDNSDomains"];
	        this.PermittedIPRanges = this.convertValues(source["PermittedIPRanges"], net.IPNet);
	        this.ExcludedIPRanges = this.convertValues(source["ExcludedIPRanges"], net.IPNet);
	        this.PermittedEmailAddresses = source["PermittedEmailAddresses"];
	        this.ExcludedEmailAddresses = source["ExcludedEmailAddresses"];
	        this.PermittedURIDomains = source["PermittedURIDomains"];
	        this.ExcludedURIDomains = source["ExcludedURIDomains"];
	        this.CRLDistributionPoints = source["CRLDistributionPoints"];
	        this.PolicyIdentifiers = source["PolicyIdentifiers"];
	        this.Policies = this.convertValues(source["Policies"], OID);
	        this.InhibitAnyPolicy = source["InhibitAnyPolicy"];
	        this.InhibitAnyPolicyZero = source["InhibitAnyPolicyZero"];
	        this.InhibitPolicyMapping = source["InhibitPolicyMapping"];
	        this.InhibitPolicyMappingZero = source["InhibitPolicyMappingZero"];
	        this.RequireExplicitPolicy = source["RequireExplicitPolicy"];
	        this.RequireExplicitPolicyZero = source["RequireExplicitPolicyZero"];
	        this.PolicyMappings = this.convertValues(source["PolicyMappings"], PolicyMapping);
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

