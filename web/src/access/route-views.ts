import type { Component } from 'vue'

export type RouteViewLoader = () => Promise<Component>
export type RouteViewMap = Readonly<Record<string, RouteViewLoader>>

export const routeViews: RouteViewMap = {}
