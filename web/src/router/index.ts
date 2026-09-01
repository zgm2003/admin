import {
    createRouter,
    createWebHistory,
    type RouterHistory,
    type RouteRecordRaw,
} from "vue-router";

import Dashboard from "../views/dashboard/index.vue";

declare module "vue-router" {
    interface RouteMeta {
        requiresAuth: boolean;
        i18nKey?: string;
        requiredPermission?: string;
        affix?: boolean;
    }
}

const routes: RouteRecordRaw[] = [
    {
        path: "/login",
        name: "login",
        component: () => import("../views/auth/login/index.vue"),
        meta: {requiresAuth: false},
    },
    {
        path: "/",
        name: "admin-layout",
        component: () => import("../layout/index.vue"),
        redirect: "/dashboard",
        meta: {requiresAuth: true},
        children: [
            {
                path: "dashboard",
                name: "dashboard",
                component: Dashboard,
                meta: {
                    requiresAuth: true,
                    i18nKey: "navigation.dashboard",
                    affix: true,
                },
            },
            {
                path: "access/menus",
                name: "access-menus",
                component: () => import("@/views/permission/menus/index.vue"),
                meta: {
                    requiresAuth: true,
                    i18nKey: "navigation.accessMenus",
                    requiredPermission: "permission:menu:view",
                },
            },
        ],
    },
];

export function createAppRouter(history: RouterHistory = createWebHistory()) {
    return createRouter({history, routes});
}

export const router = createAppRouter();
