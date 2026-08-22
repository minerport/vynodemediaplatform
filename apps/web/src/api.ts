export type SystemInfo = { version:string; operatingSystem:string; architecture:string; instanceId:string; serverName:string; databaseType:string; uptimeSeconds:number }
export class APIError extends Error { constructor(public status:number,message:string){super(message)} }
export async function getSystemInfo(signal?:AbortSignal):Promise<SystemInfo>{const response=await fetch('/api/v1/system/info',{signal});if(!response.ok)throw new APIError(response.status,`Server returned ${response.status}`);return response.json() as Promise<SystemInfo>}

