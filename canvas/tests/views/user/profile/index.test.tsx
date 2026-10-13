import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ProfilePage } from '@/views/user/profile'
import { authStore, permissionStore, request } from '@/store'

const profile = { userId: 7, username: 'Canvas user', email: 'user@example.com', phone: null, avatar: '', birthday: null, gender: 0 }
const codes = ['user:profile:view','user:profile:detail','user:profile:update','user:password:update','user:email:update','user:phone:update']
beforeEach(async()=>{
 authStore.setAuthenticated({accessToken:'token',expiresIn:3600,isNewUser:false,passwordSetRequired:false},{...profile,passwordSetRequired:false})
 permissionStore.reset()
 vi.spyOn(request,'get').mockImplementation(async(path)=>{
  if(path==='/api/v1/access')return {roleCodes:[],menuTree:[{code:'user:profile:view',menuType:'page',path:'/user/profile',componentPath:'user/profile',i18nKey:'layout.user.profile',icon:null,isHidden:1,children:[]}],permissionCodes:codes}
  if(path.endsWith('/options'))return {genders:[{value:0,label:'未设置'},{value:1,label:'男'}]}
  if(path==='/api/v1/auth/me')return authStore.state.user
  return profile
 })
 await permissionStore.load()
})
const mount = () => render(<MemoryRouter><ProfilePage /></MemoryRouter>)
describe('Canvas personal center',()=>{
 it('loads and saves profile through shared API without changing identity fields',async()=>{
  const save=vi.spyOn(request,'put').mockResolvedValue({...profile,username:'New name',updatedAt:'now'})
  mount();fireEvent.change(await screen.findByLabelText('用户名'),{target:{value:'New name'}})
  fireEvent.click(screen.getByRole('button',{name:'保存资料'}))
  await waitFor(()=>expect(save).toHaveBeenCalledWith('/api/v1/user/profile',expect.objectContaining({username:'New name',avatar:'',birthday:null,gender:0}),expect.anything()))
  expect(await screen.findByRole('status')).toHaveTextContent('资料已保存')
 })
 it('does not fetch protected profile details when detail action is absent',async()=>{
  permissionStore.state={...permissionStore.state,permissionCodes:['user:profile:view']}
  mount();expect(await screen.findByText('未获得读取个人资料权限')).toBeInTheDocument()
  expect(request.get).not.toHaveBeenCalledWith('/api/v1/user/profile',expect.anything())
  expect(screen.queryByRole('button',{name:'保存资料'})).not.toBeInTheDocument()
 })
 it('keeps edits on save failure and does not show success',async()=>{
  vi.spyOn(request,'put').mockRejectedValue(new Error('用户名已存在'))
  mount();fireEvent.change(await screen.findByLabelText('用户名'),{target:{value:'Existing'}});fireEvent.click(screen.getByRole('button',{name:'保存资料'}))
  expect(await screen.findByRole('alert')).toHaveTextContent('用户名已存在')
  expect(screen.getByLabelText('用户名')).toHaveValue('Existing')
 })
 it('sets a first password without requiring a nonexistent current password',async()=>{
  if(authStore.state.user)authStore.state.user={...authStore.state.user,passwordSetRequired:true}
  const post=vi.spyOn(request,'post').mockResolvedValue({})
  mount();await screen.findByLabelText('用户名')
  fireEvent.change(screen.getByLabelText('新密码'),{target:{value:'Password123!'}})
  fireEvent.change(screen.getByLabelText('确认新密码'),{target:{value:'Password123!'}})
  fireEvent.click(screen.getByRole('button',{name:'设置密码'}))
  await waitFor(()=>expect(post).toHaveBeenCalledWith('/api/v1/user/password/set',{newPassword:'Password123!',confirmPassword:'Password123!'}))
 })
 it('first phone binding sends only the new identity proof',async()=>{
  vi.spyOn(request,'post').mockResolvedValue({challengeId:'next-proof',expiresAt:'2030-01-01',resendAfterSeconds:60})
  const put=vi.spyOn(request,'put').mockResolvedValue({phone:'13800138000'})
  mount();await screen.findByLabelText('用户名');fireEvent.click(screen.getByRole('button',{name:'绑定手机'}))
  fireEvent.change(screen.getByLabelText('新手机号'),{target:{value:'13800138000'}})
  fireEvent.click(screen.getByRole('button',{name:'发送新手机验证码'}))
  await waitFor(()=>expect(request.post).toHaveBeenCalledWith('/api/v1/user/phone/send-code',{target:'next',phone:'13800138000',challengeId:undefined}))
  fireEvent.change(screen.getByLabelText('新手机验证码'),{target:{value:'123456'}});fireEvent.click(screen.getByRole('button',{name:'确认绑定'}))
  await waitFor(()=>expect(put).toHaveBeenCalledWith('/api/v1/user/phone',{nextPhone:'13800138000',nextChallengeId:'next-proof',nextCode:'123456'}))
 })
})
