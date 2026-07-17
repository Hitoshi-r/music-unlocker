export namespace decoder {
	
	export class QQCookieCheckResult {
	    state: string;
	    message: string;
	    loginType: string;
	    account: string;
	    localAccount: string;
	    accountMismatch: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QQCookieCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.message = source["message"];
	        this.loginType = source["loginType"];
	        this.account = source["account"];
	        this.localAccount = source["localAccount"];
	        this.accountMismatch = source["accountMismatch"];
	    }
	}

}

