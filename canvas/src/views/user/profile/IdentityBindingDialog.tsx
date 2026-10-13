import { useEffect, useRef, useState } from 'react'
import { createEmailApi } from '@/api/user/email'
import { createPhoneApi } from '@/api/user/phone'
import { request } from '@/store'

export type IdentityKind = 'email' | 'phone'
interface Props { kind: IdentityKind; currentValue: string; onClose: () => void; onSaved: (value: string) => void }
interface Proof { challengeId: string; code: string; deadline: number; sending: boolean }
const blankProof = (): Proof => ({ challengeId: '', code: '', deadline: 0, sending: false })
const emailApi = createEmailApi(request)
const phoneApi = createPhoneApi(request)

export function IdentityBindingDialog({ kind, currentValue, onClose, onSaved }: Props) {
  const [value, setValue] = useState('')
  const [current, setCurrent] = useState(blankProof)
  const [next, setNext] = useState(blankProof)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [now, setNow] = useState(Date.now())
  const alive = useRef(false)
  const busy = useRef(new Set<string>())
  const label = kind === 'email' ? '邮箱' : '手机'
  const changing = currentValue !== ''
  const seconds = (proof: Proof) => Math.max(0, Math.ceil((proof.deadline - now) / 1000))
  useEffect(() => {
    alive.current = true
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => { alive.current = false; window.clearInterval(timer) }
  }, [])

  async function send(target: 'current' | 'next') {
    const proof = target === 'current' ? current : next
    const update = target === 'current' ? setCurrent : setNext
    if (busy.current.has(target) || proof.deadline > Date.now() || busy.current.has('save')) return
    if (target === 'next' && value.trim() === '') { setError(`请输入新${label}`); return }
    busy.current.add(target); update({ ...proof, sending: true, challengeId: '', code: '' }); setError(''); setNotice('')
    try {
      const result = kind === 'email'
        ? await emailApi.sendCode(target === 'current' ? { target } : { target, email: value.trim(), challengeId: undefined })
        : await phoneApi.sendCode(target === 'current' ? { target } : { target, phone: value.trim(), challengeId: undefined })
      if (!alive.current) return
      update({ challengeId: result.challengeId, code: '', sending: false, deadline: Date.now() + result.resendAfterSeconds * 1000 })
      setNow(Date.now()); setNotice('验证码已发送')
    } catch (cause) {
      if (alive.current) setError(cause instanceof Error ? cause.message : '验证码发送失败')
    } finally { busy.current.delete(target); if (alive.current) update(proof => ({ ...proof, sending: false })) }
  }

  async function save() {
    if (busy.current.size > 0) return
    if (!value.trim() || !next.challengeId || !/^\d{6}$/.test(next.code) || (changing && (!current.challengeId || !/^\d{6}$/.test(current.code)))) {
      setError(changing ? '请填写新身份并完成当前与新身份的六位验证码验证' : '请填写新身份并输入六位验证码'); return
    }
    busy.current.add('save'); setSaving(true); setError('')
    try {
      const currentProof = changing ? { currentChallengeId: current.challengeId, currentCode: current.code } : {}
      const saved = kind === 'email'
        ? (await emailApi.bind({ nextEmail: value.trim(), nextChallengeId: next.challengeId, nextCode: next.code, ...currentProof })).email
        : (await phoneApi.bind({ nextPhone: value.trim(), nextChallengeId: next.challengeId, nextCode: next.code, ...currentProof })).phone
      if (alive.current) onSaved(saved)
    } catch (cause) { if (alive.current) setError(cause instanceof Error ? cause.message : '绑定失败')
    } finally { busy.current.delete('save'); if (alive.current) setSaving(false) }
  }

  return <div className="binding-panel"><section className="binding-card" role="dialog" aria-modal="true" aria-labelledby="binding-heading" onKeyDown={event => { if (event.key === 'Escape' && !saving && !current.sending && !next.sending) onClose() }}>
    <div className="card-heading"><h2 id="binding-heading">{changing ? '更换' : '绑定'}{label}</h2><button type="button" aria-label="关闭" disabled={saving || current.sending || next.sending} onClick={onClose}>×</button></div>
    {changing && <><p className="identity-current">当前{label}：{currentValue}</p><div className="code-row"><label className="profile-field">当前{label}验证码<input aria-label={`当前${label}验证码`} inputMode="numeric" maxLength={6} autoComplete="one-time-code" disabled={saving} value={current.code} onChange={event => setCurrent({ ...current, code: event.target.value })} /></label><button className="secondary-button" type="button" disabled={saving || current.sending || seconds(current) > 0} onClick={() => void send('current')}>{seconds(current) > 0 ? `${seconds(current)}s` : `发送当前${label}验证码`}</button></div></>}
    <label className="profile-field">{kind === 'email' ? '新邮箱' : '新手机号'}<input aria-label={kind === 'email' ? '新邮箱' : '新手机号'} type={kind === 'email' ? 'email' : 'tel'} autoFocus disabled={saving || next.sending} value={value} onChange={event => { setValue(event.target.value); setNext(proof => ({ ...proof, challengeId: '', code: '' })); setError(''); setNotice('') }} /></label>
    <div className="code-row"><label className="profile-field">新{label}验证码<input aria-label={`新${label}验证码`} inputMode="numeric" maxLength={6} autoComplete="one-time-code" disabled={saving} value={next.code} onChange={event => setNext({ ...next, code: event.target.value })} /></label><button type="button" className="secondary-button" disabled={saving || next.sending || seconds(next) > 0 || !value.trim()} onClick={() => void send('next')}>{seconds(next) > 0 ? `${seconds(next)}s` : `发送新${label}验证码`}</button></div>
    {error && <p role="alert" className="profile-error">{error}</p>}{notice && <p role="status" className="profile-success">{notice}</p>}
    <button type="button" className="primary-button" disabled={saving || next.sending || current.sending} onClick={() => void save()}>{saving ? '保存中…' : changing ? '确认更换' : '确认绑定'}</button>
  </section></div>
}
