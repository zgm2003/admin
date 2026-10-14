import { useEffect, useSyncExternalStore } from "react";
import { createBrowserRouter, Navigate, Outlet } from "react-router-dom";

import { AnalyticsTracker } from "@/components/layout/analytics-tracker";
import UserLayout from "@/layouts/user-layout";
import AssetsPage from "@/pages/assets";
import CanvasPage from "@/pages/canvas";
import CanvasProjectPage from "@/pages/canvas/project";
import ConfigPage from "@/pages/config";
import HomePage from "@/pages/home";
import ImagePage from "@/pages/image";
import NotFound from "@/pages/not-found";
import PromptsPage from "@/pages/prompts";
import VideoPage from "@/pages/video";
import { LoginPage } from "@/views/auth/login";
import { ProfilePage } from "@/views/user/profile";
import { authStore, permissionStore } from "@/store";

function AuthBootstrap() {
    const auth = useSyncExternalStore(authStore.subscribe.bind(authStore), authStore.getSnapshot, authStore.getSnapshot);

    useEffect(() => {
        if (auth.status === "unknown") void authStore.bootstrap().catch(() => undefined);
    }, [auth.status]);

    return <Outlet />;
}

function ProtectedRoute() {
    const auth = useSyncExternalStore(authStore.subscribe.bind(authStore), authStore.getSnapshot, authStore.getSnapshot);
    const access = useSyncExternalStore(permissionStore.subscribe, permissionStore.getSnapshot, permissionStore.getSnapshot);

    useEffect(() => {
        if (auth.status === "authenticated" && access.status === "idle") void permissionStore.load().catch(() => undefined);
    }, [access.status, auth.status]);

    if (auth.status === "unknown" || auth.status === "loading") return <div className="min-h-dvh bg-background" aria-label="Loading" />;
    if (auth.status !== "authenticated") return <Navigate to="/login" replace />;
    return <Outlet />;
}

function ProfileRoute() {
    const access = useSyncExternalStore(permissionStore.subscribe, permissionStore.getSnapshot, permissionStore.getSnapshot);
    if (access.status === "idle" || access.status === "loading") return <div className="min-h-dvh bg-background" aria-label="Loading" />;
    if (access.status === "error") return <section className="profile-page"><div className="profile-card"><p role="alert">{access.error}</p><button type="button" onClick={() => void permissionStore.load(true).catch(() => undefined)}>重新加载权限</button></div></section>;
    if (!permissionStore.hasPage("user/profile", "user:profile:view")) return <Navigate to="/" replace />;
    return <ProfilePage />;
}

function PublicRoute() {
    const auth = useSyncExternalStore(authStore.subscribe.bind(authStore), authStore.getSnapshot, authStore.getSnapshot);
    if (auth.status === "authenticated") return <Navigate to="/" replace />;
    return <Outlet />;
}

export const router = createBrowserRouter([
    {
        element: <AuthBootstrap />,
        children: [
            { element: <PublicRoute />, children: [{ path: "/login", element: <LoginPage /> }] },
            {
                element: <ProtectedRoute />,
                children: [
                    {
                        element: (
                            <UserLayout>
                                <AnalyticsTracker />
                                <Outlet />
                            </UserLayout>
                        ),
                        children: [
                            { path: "/", element: <HomePage /> },
                            { path: "/image", element: <ImagePage /> },
                            { path: "/video", element: <VideoPage /> },
                            { path: "/assets", element: <AssetsPage /> },
                            { path: "/prompts", element: <PromptsPage /> },
                            { path: "/canvas", element: <CanvasPage /> },
                            { path: "/canvas/:id", element: <CanvasProjectPage /> },
                            { path: "/config", element: <ConfigPage /> },
                            { path: "/user/profile", element: <ProfileRoute /> },
                            { path: "*", element: <NotFound /> },
                        ],
                    },
                ],
            },
            { path: "*", element: <Navigate to="/" replace /> },
        ],
    },
]);
