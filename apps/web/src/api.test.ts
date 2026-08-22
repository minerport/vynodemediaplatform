import{afterEach,describe,expect,it,vi}from'vitest'
import{getSystemInfo}from'./api'
afterEach(()=>vi.unstubAllGlobals())
describe('getSystemInfo',()=>{it('returns server information',async()=>{vi.stubGlobal('fetch',vi.fn().mockResolvedValue(new Response(JSON.stringify({version:'0.1.0',serverName:'Test',instanceId:'id',databaseType:'sqlite',operatingSystem:'linux',architecture:'amd64',uptimeSeconds:2}),{status:200})));await expect(getSystemInfo()).resolves.toMatchObject({serverName:'Test',databaseType:'sqlite'})});it('rejects unsuccessful responses',async()=>{vi.stubGlobal('fetch',vi.fn().mockResolvedValue(new Response('',{status:503})));await expect(getSystemInfo()).rejects.toThrow('503')})})

