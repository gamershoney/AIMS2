export namespace main {
	
	export class PermissionRequest {
	    tool: string;
	    description: string;
	    skill_name?: string;
	    job_id?: number;
	    arguments?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new PermissionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.description = source["description"];
	        this.skill_name = source["skill_name"];
	        this.job_id = source["job_id"];
	        this.arguments = source["arguments"];
	    }
	}

}

