import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from 'react'
import { useNavigate } from 'react-router-dom'

import { createProfileApi, type Profile, type ProfileOptions } from '@/api/user/profile'
import { createUploadApi } from '@/api/storage/upload'
import { authStore, permissionStore, request } from '@/store'
import { uploadAvatar } from '@/utils/upload'
import { IdentityBindingDialog, type IdentityKind } from './IdentityBindingDialog'
import { PasswordSecurityForm } from './PasswordSecurityForm'
import './profile.scss'

const profileApi = createProfileApi(request)
const uploadApi = createUploadApi(request)
interface FormState { username: string; birthday: string | null; gender: number; avatar: string; avatarFileId?: number; avatarRequestKey?: string }
const profileForm = (value: Profile): FormState => ({ username: value.username, birthday: value.birthday, gender: value.gender, avatar: value.avatar })

export function ProfilePage() {
  const navigate = useNavigate()
  const access = useSyncExternalStore(permissionStore.subscribe, permissionStore.getSnapshot, permissionStore.getSnapshot)
  const canDetail = permissionStore.hasPermission('user:profile:detail')
  const canUpdate = permissionStore.hasPermission('user:profile:update')
  const canPassword = permissionStore.hasPermission('user:password:update')
  const canEmail = permissionStore.hasPermission('user:email:update')
  const canPhone = permissionStore.hasPermission('user:phone:update')
  const canAvatar = canUpdate && permissionStore.hasPermission('storage:object:upload') && permissionStore.hasPermission('storage:file:detail')
  const [profile, setProfile] = useState<Profile | null>(null)
  const [options, setOptions] = useState<ProfileOptions>({ genders: [] })
  const [form, setForm] = useState<FormState>({ username: '', birthday: null, gender: 0, avatar: '' })
  const [loading, setLoading] = useState(canDetail)
  const [loadError, setLoadError] = useState('')
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [status, setStatus] = useState('')
  const [error, setError] = useState('')
  const [warning, setWarning] = useState('')
  const [bindingNotice, setBindingNotice] = useState('')
  const [bindingTarget, setBindingTarget] = useState<IdentityKind | null>(null)
  const [avatarPreview, setAvatarPreview] = useState('')
  const [avatarError, setAvatarError] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)
  const loadController = useRef<AbortController | null>(null)
  const uploadController = useRef<AbortController | null>(null)
  const saveController = useRef<AbortController | null>(null)
  const previewController = useRef<AbortController | null>(null)
  const blobPreview = useRef('')
  const alive = useRef(false)
  const busy = useRef(false)

  const showAvatar = useCallback(async (objectKey: string) => {
    previewController.current?.abort()
    const controller = new AbortController(); previewController.current = controller
    setAvatarError('')
    if (blobPreview.current) { URL.revokeObjectURL(blobPreview.current); blobPreview.current = '' }
    setAvatarPreview('')
    if (!objectKey) return
    try {
      const object = await uploadApi.objectURL(objectKey, controller.signal)
      if (!controller.signal.aborted && alive.current) setAvatarPreview(object.url)
    } catch (cause) { if (!controller.signal.aborted && alive.current) setAvatarError(messageOf(cause, '头像预览加载失败')) }
  }, [])

  const load = useCallback(async () => {
    if (!canDetail) { setLoading(false); return }
    loadController.current?.abort()
    const controller = new AbortController(); loadController.current = controller
    setLoading(true); setLoadError('')
    try {
      const [nextProfile, nextOptions] = await Promise.all([profileApi.get(controller.signal), profileApi.options(controller.signal)])
      if (controller.signal.aborted || !alive.current) return
      setProfile(nextProfile); setOptions(nextOptions); setForm(profileForm(nextProfile))
      void showAvatar(nextProfile.avatar)
    } catch (cause) { if (!controller.signal.aborted && alive.current) setLoadError(messageOf(cause, '个人资料加载失败'))
    } finally { if (!controller.signal.aborted && alive.current) setLoading(false) }
  }, [canDetail, showAvatar])

  useEffect(() => {
    alive.current = true; void load()
    return () => {
      alive.current = false; loadController.current?.abort(); uploadController.current?.abort(); saveController.current?.abort(); previewController.current?.abort()
      if (blobPreview.current) { URL.revokeObjectURL(blobPreview.current); blobPreview.current = '' }
    }
  }, [load])
  const updateForm = (patch: Partial<FormState>) => { setStatus(''); setForm(current => ({ ...current, ...patch })) }

  async function refreshUser(success: string) {
    try { await authStore.reloadUser() } catch (cause) { if (alive.current) setWarning(`${success}，账户信息刷新失败：${messageOf(cause, '请刷新页面重试')}`) }
  }

  async function saveProfile() {
    if (!profile || !canUpdate || busy.current) return
    if (!form.username.trim()) { setError('请输入用户名'); return }
    busy.current = true; const controller = new AbortController(); saveController.current = controller
    setSaving(true); setError(''); setStatus(''); setWarning('')
    try {
      const saved = await profileApi.update(form, controller.signal)
      if (controller.signal.aborted || !alive.current) return
      setProfile(saved); setForm(profileForm(saved)); setStatus('资料已保存')
      if (form.avatarFileId !== undefined) void showAvatar(saved.avatar)
      await refreshUser('资料已保存')
    } catch (cause) { if (!controller.signal.aborted && alive.current) setError(messageOf(cause, '资料保存失败'))
    } finally { busy.current = false; if (alive.current && !controller.signal.aborted) setSaving(false) }
  }

  async function chooseAvatar(file: File | undefined) {
    if (!file || !canAvatar || busy.current) return
    busy.current = true; const controller = new AbortController(); uploadController.current = controller
    setUploading(true); setError(''); setStatus(''); setAvatarError('')
    try {
      const completed = await uploadAvatar(request, file, controller.signal)
      if (controller.signal.aborted || !alive.current) return
      previewController.current?.abort()
      if (blobPreview.current) URL.revokeObjectURL(blobPreview.current)
      blobPreview.current = URL.createObjectURL(file); setAvatarPreview(blobPreview.current)
      // Preserve this key on failure so retrying adoption cannot duplicate the copy.
      updateForm({ avatar: '', avatarFileId: completed.fileId, avatarRequestKey: crypto.randomUUID() })
    } catch (cause) { if (!controller.signal.aborted && alive.current) setError(messageOf(cause, '头像上传失败'))
    } finally { busy.current = false; if (fileRef.current) fileRef.current.value = ''; if (alive.current && !controller.signal.aborted) setUploading(false) }
  }

  const back = <button type="button" className="back-button" onClick={() => navigate('/')}>返回画布</button>
  if (access.status === 'error') return <section className="profile-page"><div className="profile-card">{back}<p role="alert">{access.error}</p><button type="button" onClick={() => void permissionStore.load(true).catch(() => undefined)}>重试</button></div></section>
  if (!canDetail) return <section className="profile-page"><div className="profile-card">{back}<p>未获得读取个人资料权限</p></div></section>
  if (loading) return <section className="profile-page"><div className="profile-card"><p aria-live="polite">正在加载个人资料…</p></div></section>
  if (loadError || !profile) return <section className="profile-page"><div className="profile-card">{back}<p role="alert">{loadError || '个人资料不可用'}</p><button type="button" onClick={() => void load()}>重试</button></div></section>

  return <main className="profile-page">
    <header className="profile-header"><div><p className="eyebrow">PERSONAL CENTER</p><h1>个人中心</h1><p>管理你的身份资料和账户安全。</p></div>{back}</header>
    <div className="profile-layout">
      <section className="profile-card profile-details"><div className="card-heading"><div><h2>基本资料</h2><p>这些信息会用于你的工作空间。</p></div><div className="avatar-wrap"><button type="button" aria-label="更换头像" className="avatar-button" disabled={!canAvatar || saving || uploading} onClick={() => fileRef.current?.click()}>{avatarPreview ? <img src={avatarPreview} alt="头像" onError={() => setAvatarError('头像预览不可用，请重新加载')} /> : <span>{form.username.slice(0, 1).toUpperCase()}</span>}</button><input ref={fileRef} aria-label="头像文件" type="file" accept="image/*" hidden disabled={!canAvatar || saving || uploading} onChange={event => void chooseAvatar(event.target.files?.[0])} /><small>{uploading ? '上传中…' : canAvatar ? '更换头像' : '暂无头像上传权限'}</small></div></div>
        {avatarError && <p className="profile-error">{avatarError} <button type="button" disabled={saving || uploading} onClick={() => void showAvatar(profile.avatar)}>重新加载头像</button></p>}
        {form.avatarFileId !== undefined && <p className="profile-hint">头像已上传，保存资料后生效</p>}
        <label className="profile-field">用户名<input autoComplete="nickname" value={form.username} disabled={!canUpdate || saving || uploading} onChange={event => updateForm({ username: event.target.value })} /></label>
        <label className="profile-field">生日<input type="date" value={form.birthday ?? ''} disabled={!canUpdate || saving || uploading} onChange={event => updateForm({ birthday: event.target.value || null })} /></label>
        <label className="profile-field">性别<select value={form.gender} disabled={!canUpdate || saving || uploading} onChange={event => updateForm({ gender: Number(event.target.value) })}>{options.genders.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}{options.genders.every(option => option.value !== form.gender) && <option value={form.gender}>未知（{form.gender}）</option>}</select></label>
        {error && <p role="alert" className="profile-error">{error}</p>}{status && <p role="status" className="profile-success">{status}</p>}{warning && <p className="profile-hint">{warning}</p>}{canUpdate && <button type="button" className="primary-button" disabled={saving || uploading} onClick={() => void saveProfile()}>{saving ? '保存中…' : '保存资料'}</button>}
      </section>
      <section className="profile-card security-card"><div className="card-heading"><div><h2>账户安全</h2><p>更新登录凭据与验证方式。</p></div></div>{canPassword && <PasswordSecurityForm email={profile.email} phone={profile.phone} />}
        <div className="identity-row"><span>邮箱</span><strong>{profile.email || '未绑定'}</strong>{canEmail && <button type="button" onClick={() => setBindingTarget('email')}>{profile.email ? '更换邮箱' : '绑定邮箱'}</button>}</div>
        <div className="identity-row"><span>手机号</span><strong>{profile.phone || '未绑定'}</strong>{canPhone && <button type="button" onClick={() => setBindingTarget('phone')}>{profile.phone ? '更换手机' : '绑定手机'}</button>}</div>
        {bindingNotice && <p role="status" className="profile-success">{bindingNotice}</p>}
      </section>
    </div>
    {bindingTarget && (bindingTarget === 'email' ? canEmail : canPhone) && <IdentityBindingDialog key={bindingTarget} kind={bindingTarget} currentValue={bindingTarget === 'email' ? profile.email : profile.phone ?? ''} onClose={() => setBindingTarget(null)} onSaved={value => {
      setProfile(current => current ? { ...current, ...(bindingTarget === 'email' ? { email: value } : { phone: value }) } : current)
      setBindingNotice(bindingTarget === 'email' ? '邮箱已更新' : '手机号已更新'); setBindingTarget(null); setWarning(''); void refreshUser('身份已更新')
    }} />}
  </main>
}
function messageOf(cause: unknown, fallback: string): string { return cause instanceof Error && cause.message !== '' ? cause.message : fallback }
