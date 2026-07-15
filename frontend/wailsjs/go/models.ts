export namespace main {
	
	export class StatusInfo {
	    mode: string;
	    proxyAddr: string;
	    dataDir: string;
	    caCertPath: string;
	    verbose: boolean;
	    toolCount: number;
	
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

