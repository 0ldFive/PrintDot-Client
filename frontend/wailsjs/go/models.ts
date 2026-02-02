export namespace main {
	
	export class AppSettings {
	    language: string;
	    autoStart: boolean;
	    remoteServer: string;
	    remoteUser: string;
	    remotePassword: string;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.autoStart = source["autoStart"];
	        this.remoteServer = source["remoteServer"];
	        this.remoteUser = source["remoteUser"];
	        this.remotePassword = source["remotePassword"];
	    }
	}

}

