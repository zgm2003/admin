import { LogOut, Orbit, Settings2 } from 'lucide-react'

import { authStore } from '@/store'

export function WorkbenchPage() {
  const user = authStore.state.user
  return <main className="workbench"><header className="workbench-header"><div className="brand-lockup"><span className="brand-mark" aria-hidden="true"><span /></span><span>infinite canvas</span></div><div className="workbench-actions"><span className="user-chip">{user?.username ?? user?.email ?? 'Canvas user'}</span><button type="button" aria-label="设置" title="设置"><Settings2 size={17} /></button><button type="button" aria-label="退出登录" title="退出登录" onClick={() => void authStore.logout()}><LogOut size={17} /></button></div></header><section className="workbench-empty"><div className="empty-orbit"><Orbit size={34} /></div><p className="eyebrow">AUTHENTICATED WORKSPACE</p><h1>你的画布已准备好</h1><p>认证联调完成。下一阶段将把参考项目的项目库、无限画布和生成工作流搬入这里。</p></section></main>
}
