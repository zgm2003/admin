import { LogOut, Orbit, Settings2 } from "lucide-react";
import { useSyncExternalStore, useState } from "react";
import { useNavigate } from "react-router-dom";

import { authStore, permissionStore } from "@/store";

export function WorkbenchPage() {
  const auth = useSyncExternalStore(authStore.subscribe.bind(authStore), authStore.getSnapshot, authStore.getSnapshot);
  const user = auth.user;
  const [logoutError, setLogoutError] = useState("");
  const navigate = useNavigate();
  const access = useSyncExternalStore(permissionStore.subscribe.bind(permissionStore), permissionStore.getSnapshot, permissionStore.getSnapshot);
  const canOpenProfile = permissionStore.hasPage("user/profile", "user:profile:view");
  return (
    <main className="workbench">
      <header className="workbench-header">
        <div className="brand-lockup">
          <span className="brand-mark" aria-hidden="true">
            <span />
          </span>
          <span>infinite canvas</span>
        </div>
        <div className="workbench-actions">
          <span className="user-chip">
            {user?.username ?? user?.email ?? "Canvas user"}
          </span>
          {access.status === "ready" && canOpenProfile && <button type="button" aria-label="个人中心" title="个人中心" onClick={() => navigate("/user/profile")}>
            <Settings2 size={17} />
          </button>}
          <button
            type="button"
            aria-label="退出登录"
            title="退出登录"
            onClick={() => void authStore.logout().catch((error: unknown) => setLogoutError(error instanceof Error ? error.message : "退出请求失败，已清除本地登录状态"))}
          >
            <LogOut size={17} />
          </button>
        </div>
      </header>
      {access.status === "error" && <div className="inline-error" role="alert"><span>{access.error}</span><button type="button" onClick={() => void permissionStore.load(true).catch(() => undefined)}>重新加载权限</button></div>}
      {logoutError && <p role="alert">{logoutError}</p>}
      <section className="workbench-empty">
        <div className="empty-orbit">
          <Orbit size={34} />
        </div>
        <p className="eyebrow">AUTHENTICATED WORKSPACE</p>
        <h1>你的画布已准备好</h1>
        <p>
          认证联调完成。下一阶段将把参考项目的项目库、无限画布和生成工作流搬入这里。
        </p>
      </section>
    </main>
  );
}
