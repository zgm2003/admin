import { describe, expect, it, vi } from 'vitest'
import { request } from '@src/utils/request'
import { listUploadRules, getUploadRulePageInit } from '@src/api/storage/uploadrule'
vi.mock('@src/utils/request',()=>({request:vi.fn()}));const requestMock=vi.mocked(request)
describe('upload rule API',()=>{it('uses exact admin routes',async()=>{requestMock.mockResolvedValueOnce({list:[],total:0,page:1,pageSize:20});await listUploadRules({page:1,pageSize:20,platformId:2});expect(requestMock).toHaveBeenCalledWith({method:'GET',url:'/api/admin/v1/storage/upload-rules',params:{page:1,pageSize:20,platformId:2}});requestMock.mockResolvedValueOnce({platforms:[],configs:[]});await getUploadRulePageInit();expect(requestMock).toHaveBeenCalledWith({method:'GET',url:'/api/admin/v1/storage/upload-rules/page-init'})})})
