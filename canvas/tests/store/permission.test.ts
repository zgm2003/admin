import { describe, expect, it, vi } from 'vitest'
import { PermissionStore } from '@/store/permission'
import { createRequestClient } from '@/utils/request'

const page = { code: 'user:profile:view', menuType: 'page', path: '/user/profile', componentPath: 'user/profile', i18nKey: 'layout.user.profile', icon: null, isHidden: 1, children: [] }
const snapshot = { roleCodes: ['registered_user'], permissionCodes: ['user:profile:view','user:profile:detail'], menuTree: [page] }
function setup() {
 const fetchMock = vi.fn<typeof fetch>()
 const client = createRequestClient({baseUrl:'http://localhost',platform:'canvas',deviceId:'device',fetchImpl:fetchMock})
 return {fetchMock, store:new PermissionStore(client)}
}
const response = (data:unknown) => new Response(JSON.stringify({code:0,data,message:'ok'}))
describe('Canvas permissions',()=>{
 it('registers a hidden page only from directly granted menuTree, not an action',async()=>{
  const {fetchMock,store}=setup();fetchMock.mockResolvedValueOnce(response(snapshot))
  await store.load(); expect(store.hasPage('user/profile','user:profile:view')).toBe(true)
  store.reset();fetchMock.mockResolvedValueOnce(response({roleCodes:[],permissionCodes:['user:profile:update'],menuTree:[]}))
  await store.load();expect(store.hasPage('user/profile','user:profile:view')).toBe(false)
 })
 it('clears routes and actions when reload fails or protocol is malformed',async()=>{
  const {fetchMock,store}=setup();fetchMock.mockResolvedValueOnce(response(snapshot));await store.load()
  fetchMock.mockResolvedValueOnce(response({...snapshot,permissionCodes:'invalid'}));await expect(store.load(true)).rejects.toThrow()
  expect(store.state.status).toBe('error');expect(store.hasPermission('user:profile:detail')).toBe(false);expect(store.state.menuTree).toEqual([])
 })
 it('ignores a response arriving after logout/reset',async()=>{
  const {fetchMock,store}=setup();let resolve!: (r:Response)=>void
  fetchMock.mockImplementationOnce(()=>new Promise<Response>(r=>{resolve=r}))
  const pending=store.load();store.reset();resolve(response(snapshot));await pending
  expect(store.state.status).toBe('idle');expect(store.state.permissionCodes).toEqual([])
 })
})
