export namespace main {
	
	export class ClientCreated {
	    Address: string;
	    Wallet: string;
	    Error: any;
	
	    static createFrom(source: any = {}) {
	        return new ClientCreated(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Address = source["Address"];
	        this.Wallet = source["Wallet"];
	        this.Error = source["Error"];
	    }
	}

}

