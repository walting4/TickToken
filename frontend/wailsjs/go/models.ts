export namespace main {
	
	export class APIKeyMonitorStats {
	    totalRequests: number;
	    completedRequests: number;
	    failedRequests: number;
	    anomalyRequests: number;
	    inflightCount: number;
	
	    static createFrom(source: any = {}) {
	        return new APIKeyMonitorStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalRequests = source["totalRequests"];
	        this.completedRequests = source["completedRequests"];
	        this.failedRequests = source["failedRequests"];
	        this.anomalyRequests = source["anomalyRequests"];
	        this.inflightCount = source["inflightCount"];
	    }
	}
	export class LanguageInfo {
	    current: string;
	    available: string[];
	
	    static createFrom(source: any = {}) {
	        return new LanguageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.available = source["available"];
	    }
	}
	export class StatusInfo {
	    mode: string;
	    proxyAddr: string;
	    dataDir: string;
	    caCertPath: string;
	    verbose: boolean;
	    toolCount: number;
	    inflightRequests: number;
	    totalAPIKeyReqs: number;
	    verificationRate: number;
	    avgDeviationPct: number;
	    watchedFilesCount: number;
	
	    static createFrom(source: any = {}) {
	        return new StatusInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.proxyAddr = source["proxyAddr"];
	        this.dataDir = source["dataDir"];
	        this.caCertPath = source["caCertPath"];
	        this.verbose = source["verbose"];
	        this.toolCount = source["toolCount"];
	        this.inflightRequests = source["inflightRequests"];
	        this.totalAPIKeyReqs = source["totalAPIKeyReqs"];
	        this.verificationRate = source["verificationRate"];
	        this.avgDeviationPct = source["avgDeviationPct"];
	        this.watchedFilesCount = source["watchedFilesCount"];
	    }
	}
	export class SummaryStats {
	    totalPrompt: number;
	    totalCompletion: number;
	    totalCacheHit: number;
	    totalEvents: number;
	
	    static createFrom(source: any = {}) {
	        return new SummaryStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalPrompt = source["totalPrompt"];
	        this.totalCompletion = source["totalCompletion"];
	        this.totalCacheHit = source["totalCacheHit"];
	        this.totalEvents = source["totalEvents"];
	    }
	}
	export class VerificationStats {
	    totalVerified: number;
	    passed: number;
	    anomalies: number;
	    accuracyRate: number;
	    avgDeviationPct: number;
	    withResponseUsage: number;
	    withLocalOnly: number;
	    avgLatencyMs: number;
	    maxLatencyMs: number;
	
	    static createFrom(source: any = {}) {
	        return new VerificationStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalVerified = source["totalVerified"];
	        this.passed = source["passed"];
	        this.anomalies = source["anomalies"];
	        this.accuracyRate = source["accuracyRate"];
	        this.avgDeviationPct = source["avgDeviationPct"];
	        this.withResponseUsage = source["withResponseUsage"];
	        this.withLocalOnly = source["withLocalOnly"];
	        this.avgLatencyMs = source["avgLatencyMs"];
	        this.maxLatencyMs = source["maxLatencyMs"];
	    }
	}
	export class ProxyStatus {
	    proxyAddr: string;
	    isListening: boolean;
	    systemProxySet: boolean;
	    cacertInstalled: boolean;
	    traeProxySet: boolean;
	    cacertPath: string;
	    traeConfigPath: string;
	    platform: string;
	    proxyStats: { totalRequests: number; totalCaptured: number; totalErrors: number; totalSSEStream: number; };

	    static createFrom(source: any = {}) {
	        return new ProxyStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyAddr = source["proxyAddr"];
	        this.isListening = source["isListening"];
	        this.systemProxySet = source["systemProxySet"];
	        this.cacertInstalled = source["cacertInstalled"];
	        this.traeProxySet = source["traeProxySet"];
	        this.cacertPath = source["cacertPath"];
	        this.traeConfigPath = source["traeConfigPath"];
	        this.platform = source["platform"];
	        this.proxyStats = source["proxyStats"];
	    }
	}
	export class SetupResult {
	    success: boolean;
	    message: string;
	    steps: string[];
	    warnings: string[];

	    static createFrom(source: any = {}) {
	        return new SetupResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.steps = source["steps"];
	        this.warnings = source["warnings"];
	    }
	}

}

export namespace storage {
	
	export class AggregationItem {
	    Key: string;
	    PromptTokens: number;
	    CompletionTokens: number;
	    CacheHit: number;
	    CacheMiss: number;
	    CacheCreation: number;
	    Total: number;
	
	    static createFrom(source: any = {}) {
	        return new AggregationItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Key = source["Key"];
	        this.PromptTokens = source["PromptTokens"];
	        this.CompletionTokens = source["CompletionTokens"];
	        this.CacheHit = source["CacheHit"];
	        this.CacheMiss = source["CacheMiss"];
	        this.CacheCreation = source["CacheCreation"];
	        this.Total = source["Total"];
	    }
	}
	export class AnomalyStats {
	    TotalAnomalies: number;
	    AnomalyRate: number;
	    ByType: Record<string, number>;
	    AvgDeviation: number;
	
	    static createFrom(source: any = {}) {
	        return new AnomalyStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.TotalAnomalies = source["TotalAnomalies"];
	        this.AnomalyRate = source["AnomalyRate"];
	        this.ByType = source["ByType"];
	        this.AvgDeviation = source["AvgDeviation"];
	    }
	}
	export class TimeSeriesPoint {
	    // Go type: time
	    Timestamp: any;
	    PromptTokens: number;
	    CompletionTokens: number;
	    Total: number;
	
	    static createFrom(source: any = {}) {
	        return new TimeSeriesPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Timestamp = this.convertValues(source["Timestamp"], null);
	        this.PromptTokens = source["PromptTokens"];
	        this.CompletionTokens = source["CompletionTokens"];
	        this.Total = source["Total"];
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
	export class TokenEvent {
	    ID: number;
	    // Go type: time
	    Timestamp: any;
	    Tool: string;
	    Model: string;
	    PromptTokens: number;
	    CompletionTokens: number;
	    CacheHit: number;
	    CacheMiss: number;
	    CacheCreation: number;
	    Source: string;
	    Tokenizer: string;
	    IsAnomaly: boolean;
	    AnomalyType: string;
	    DeviationPct: number;
	    LatencyMs: number;
	    Provider: string;
	
	    static createFrom(source: any = {}) {
	        return new TokenEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Timestamp = this.convertValues(source["Timestamp"], null);
	        this.Tool = source["Tool"];
	        this.Model = source["Model"];
	        this.PromptTokens = source["PromptTokens"];
	        this.CompletionTokens = source["CompletionTokens"];
	        this.CacheHit = source["CacheHit"];
	        this.CacheMiss = source["CacheMiss"];
	        this.CacheCreation = source["CacheCreation"];
	        this.Source = source["Source"];
	        this.Tokenizer = source["Tokenizer"];
	        this.IsAnomaly = source["IsAnomaly"];
	        this.AnomalyType = source["AnomalyType"];
	        this.DeviationPct = source["DeviationPct"];
	        this.LatencyMs = source["LatencyMs"];
	        this.Provider = source["Provider"];
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
	export class TrendPoint {
	    // Go type: time
	    Timestamp: any;
	    TotalTokens: number;
	    PromptTokens: number;
	    CompletionTokens: number;
	    EventCount: number;
	    AnomalyCount: number;
	
	    static createFrom(source: any = {}) {
	        return new TrendPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Timestamp = this.convertValues(source["Timestamp"], null);
	        this.TotalTokens = source["TotalTokens"];
	        this.PromptTokens = source["PromptTokens"];
	        this.CompletionTokens = source["CompletionTokens"];
	        this.EventCount = source["EventCount"];
	        this.AnomalyCount = source["AnomalyCount"];
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

