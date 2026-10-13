import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { createProfileApi } from '@/api/user/profile'
import { authStore, request } from '@/store'

const api = createProfileApi(request)
interface Props { email: string; phone: string | null }
export function PasswordSecurityForm({ email, phone }: Props) {
  const navigate = useNavigate()
  const [first, setFirst] = useState(authStore.state.user?.passwordSetRequired ?? false)
  const [mode, setMode] = useState<'password' | 'code'>('password')
  const [loginType, setLoginType] = useState<'email' | 'phone'>(email ? 'email' : 'phone')
  const [form, setForm] = useState({ currentPassword: '', newPassword: '', confirmPassword: '' })
  const [code, setCode] = useState('')
  const [challengeId, setChallengeId] = useState('')
  const [deadline, setDeadline] = useState(0)
  const [now, setNow] = useState(Date.now())
  const [saving, setSaving] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [warning, setWarning] = useState('')
  const alive = useRef(false)
  const busy = useRef(false)
  const seconds = Math.max(0, Math.ceil((deadline - now) / 1000))
  useEffect(() => { alive.current = true; const timer = window.setInterval(() => setNow(Date.now()), 1000); return () => { alive.current = false; window.clearInterval(timer) } }, [])
  useEffect(() => { setCode(''); setChallengeId(''); setDeadline(0); setError('') }, [email, phone, loginType, mode])

  async function sendCode() {
    if (busy.current || deadline > Date.now() || !(loginType === 'email' ? email : phone)) return
    busy.current = true; setSending(true); setError(''); setNotice(''); setChallengeId(''); setCode('')
    try { const proof = await api.sendPasswordCode(loginType); if (alive.current) { setChallengeId(proof.challengeId); setDeadline(Date.now() + proof.resendAfterSeconds * 1000); setNow(Date.now()); setNotice('验证码已发送') }
    } catch (cause) { if (alive.current) setError(cause instanceof Error ? cause.message : '验证码发送失败')
    } finally { busy.current = false; if (alive.current) setSending(false) }
  }

  async function save() {
    if (busy.current) return
    if (!form.newPassword || form.newPassword !== form.confirmPassword) { setError('请填写新密码，两次输入必须一致'); return }
    if (!first && mode === 'password' && !form.currentPassword) { setError('请输入当前密码'); return }
    if (!first && mode === 'code' && (!challengeId || !/^\d{6}$/.test(code))) { setError('请先发送并输入六位密码验证码'); return }
    busy.current = true; setSaving(true); setError(''); setNotice(''); setWarning('')
    try {
      if (first) await api.setPassword({ newPassword: form.newPassword, confirmPassword: form.confirmPassword })
      else if (mode === 'code') await api.changePasswordByCode({ loginType, challengeId, code, newPassword: form.newPassword, confirmPassword: form.confirmPassword })
      else {
        const startedToken = request.getAccessToken()
        const startedUserID = authStore.state.user?.userId
        await api.changePassword(form)
        if (alive.current && authStore.state.status === 'authenticated' && authStore.state.user?.userId === startedUserID && request.getAccessToken() === startedToken) {
          authStore.markAnonymous()
          navigate('/login', { replace: true })
        }
        return
      }
      if (!alive.current) return
      setNotice(first ? '密码已设置' : '密码已修改'); setFirst(false); setForm({ currentPassword: '', newPassword: '', confirmPassword: '' }); setCode(''); setChallengeId('')
      try { await authStore.reloadUser() } catch (cause) { if (alive.current) setWarning(`密码操作已成功，账户信息刷新失败：${cause instanceof Error ? cause.message : '请刷新页面重试'}`) }
    } catch (cause) { if (alive.current) setError(cause instanceof Error ? cause.message : '密码操作失败')
    } finally { busy.current = false; if (alive.current) setSaving(false) }
  }

  return <div className="security-block"><h3>{first ? '设置密码' : '修改密码'}</h3>
    {!first && <div className="password-mode"><button type="button" aria-pressed={mode === 'password'} disabled={saving || sending} onClick={() => setMode('password')}>原密码修改</button><button type="button" aria-pressed={mode === 'code'} disabled={saving || sending || (!email && !phone)} onClick={() => setMode('code')}>验证码修改</button></div>}
    {!first && mode === 'password' && <label className="profile-field">当前密码<input type="password" autoComplete="current-password" disabled={saving} value={form.currentPassword} onChange={event => setForm({ ...form, currentPassword: event.target.value })} /></label>}
    {!first && mode === 'code' && <><label className="profile-field">验证方式<select disabled={sending || saving} value={loginType} onChange={event => setLoginType(event.target.value === 'phone' ? 'phone' : 'email')}>{email && <option value="email">邮箱 · {email}</option>}{phone && <option value="phone">手机 · {phone}</option>}</select></label><div className="code-row"><label className="profile-field">密码验证码<input inputMode="numeric" autoComplete="one-time-code" maxLength={6} disabled={saving} value={code} onChange={event => setCode(event.target.value)} /></label><button className="secondary-button" type="button" disabled={saving || sending || seconds > 0} onClick={() => void sendCode()}>{seconds > 0 ? `${seconds}s` : '发送密码验证码'}</button></div></>}
    <label className="profile-field">新密码<input type="password" autoComplete="new-password" disabled={saving} value={form.newPassword} onChange={event => setForm({ ...form, newPassword: event.target.value })} /></label>
    <label className="profile-field">确认新密码<input type="password" autoComplete="new-password" disabled={saving} value={form.confirmPassword} onChange={event => setForm({ ...form, confirmPassword: event.target.value })} /></label>
    {!first && mode === 'password' && <p className="profile-hint">使用原密码修改后，需要重新登录。</p>}
    {error && <p role="alert" className="profile-error">{error}</p>}{notice && <p role="status" className="profile-success">{notice}</p>}{warning && <p className="profile-hint">{warning}</p>}
    <button type="button" className="secondary-button" disabled={saving || sending} onClick={() => void save()}>{saving ? '保存中…' : first ? '设置密码' : '修改密码'}</button>
  </div>
}
