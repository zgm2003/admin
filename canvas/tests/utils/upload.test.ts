import { describe, expect, it, vi } from 'vitest'
import { uploadAvatar } from '@/utils/upload'
import { createRequestClient } from '@/utils/request'
describe('avatar upload',()=>{
 it('uploads using only signed headers then completes the owned file',async()=>{
  const apiFetch=vi.fn<typeof fetch>().mockResolvedValueOnce(new Response(JSON.stringify({code:0,data:{items:[{fileId:9,method:'PUT',uploadUrl:'https://storage.invalid/signed',objectKey:'file/test.png',headers:{'Content-Type':'image/png'},expiresAt:'2030-01-01T00:00:00Z'}]},message:'ok'}))).mockResolvedValueOnce(new Response(JSON.stringify({code:0,data:{fileId:9,objectKey:'file/test.png'},message:'ok'})))
  const client=createRequestClient({baseUrl:'http://localhost',platform:'canvas',deviceId:'device',initialAccessToken:'secret',fetchImpl:apiFetch})
  const storageFetch=vi.fn<typeof fetch>().mockResolvedValue(new Response('',{status:200}))
  await expect(uploadAvatar(client,new File(['png'],'image.png',{type:'image/png'}),undefined,storageFetch)).resolves.toMatchObject({fileId:9})
  const [,init]=storageFetch.mock.calls[0];expect(init?.credentials).toBe('omit');expect(new Headers(init?.headers).get('Authorization')).toBeNull()
  expect(JSON.parse(String(apiFetch.mock.calls[0][1]?.body))).toMatchObject({ruleCode:'avatar'})
  expect(String(apiFetch.mock.calls[1][0])).toContain('/storage/file/9/complete')
 })
 it('never completes or adopts a failed storage upload',async()=>{
  const apiFetch=vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify({code:0,data:{items:[{fileId:9,method:'PUT',uploadUrl:'https://storage.invalid/signed',headers:{},objectKey:'file/test.png',expiresAt:'2030-01-01T00:00:00Z'}]},message:'ok'})))
  const client=createRequestClient({baseUrl:'http://localhost',platform:'canvas',deviceId:'device',fetchImpl:apiFetch})
  await expect(uploadAvatar(client,new File(['png'],'image.png',{type:'image/png'}),undefined,vi.fn<typeof fetch>().mockResolvedValue(new Response('',{status:403})))).rejects.toThrow()
  expect(apiFetch).toHaveBeenCalledTimes(1)
 })
 it('rejects empty and non-image files without asking for credentials',async()=>{
  const fetchMock=vi.fn<typeof fetch>();const client=createRequestClient({baseUrl:'http://localhost',platform:'canvas',deviceId:'device',fetchImpl:fetchMock})
  for(const file of [new File([],'empty.png',{type:'image/png'}),new File(['x'],'x.txt',{type:'text/plain'})]) await expect(uploadAvatar(client,file)).rejects.toThrow()
  expect(fetchMock).not.toHaveBeenCalled()
 })
})
