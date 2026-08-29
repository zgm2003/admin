import { describe, expect, it, vi, beforeEach } from 'vitest'
import { request, ProtocolError } from '@src/utils/request'
import { parseCosConfigResponse, listCosConfigs, type CosConfig } from '@src/api/storage/cosconfig'

vi.mock('@src/utils/request', async () => { const actual = await vi.importActual<typeof import('@src/utils/request')>('@src/utils/request'); return { ...actual, request: vi.fn() } })
const requestMock = vi.mocked(request)
const config: CosConfig = { id:1,name:'Main',appId:'app',bucket:'assets',region:'ap-guangzhou',endpoint:null,bucketDomain:null,isEnabled:1,hasCredentials:true,remark:'',createdAt:'2026-08-30T00:00:00Z',updatedAt:'2026-08-30T00:00:00Z' }
describe('COS config API',()=>{beforeEach(()=>vi.clearAllMocks());it('parses safe config metadata only',()=>{expect(parseCosConfigResponse(config)).toEqual(config);expect(()=>parseCosConfigResponse({...config,secretId:'leak'})).toThrow(ProtocolError)});it('uses the exact list endpoint and query',async()=>{requestMock.mockResolvedValue({list:[config],total:1,page:1,pageSize:20});await expect(listCosConfigs({page:1,pageSize:20,keyword:'main'})).resolves.toEqual({list:[config],total:1,page:1,pageSize:20});expect(requestMock).toHaveBeenCalledWith({method:'GET',url:'/api/admin/v1/storage/cos-configs',params:{page:1,pageSize:20,keyword:'main'}})})})
