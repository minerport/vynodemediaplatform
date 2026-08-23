export type GlobalAccount={id:string;username:string;displayName:string};
export type GlobalSession={accessToken:string;refreshToken?:string;expiresIn:number;account:GlobalAccount};
export type ConnectEndpoint={url:string;kind:string;secure:boolean};
export type ConnectedServer={id:string;name:string;relationship:"OWNER"|"MEMBER";status:string;endpoints:ConnectEndpoint[]};

/** Connect is a separate control-plane origin; media credentials always remain server-scoped. */
export class ConnectClient{
  private accessToken="";
  constructor(readonly origin:string){}
  async login(username:string,password:string){const v=await this.request<GlobalSession>("/api/v1/account/login",{method:"POST",body:JSON.stringify({username,password,deviceName:"Web browser",platform:"WEB",clientName:"VyNode Web"})});this.accessToken=v.accessToken;return v}
  async register(username:string,displayName:string,password:string){const v=await this.request<GlobalSession>("/api/v1/account/register",{method:"POST",body:JSON.stringify({username,displayName,password,deviceName:"Web browser",platform:"WEB",clientName:"VyNode Web"})});this.accessToken=v.accessToken;return v}
  servers(){return this.request<ConnectedServer[]>("/api/v1/servers")}
  assertion(serverId:string){return this.request<{assertion:string}>(`/api/v1/servers/${encodeURIComponent(serverId)}/assertion`,{method:"POST",body:"{}"})}
  clear(){this.accessToken=""}
  private async request<T>(path:string,init:RequestInit={}){const headers=new Headers(init.headers);headers.set("Content-Type","application/json");if(this.accessToken)headers.set("Authorization",`Bearer ${this.accessToken}`);const response=await fetch(this.origin.replace(/\/$/,"")+path,{...init,headers,credentials:"include"});if(!response.ok)throw new Error(`Connect request failed (${response.status})`);return response.json() as Promise<T>}
}
