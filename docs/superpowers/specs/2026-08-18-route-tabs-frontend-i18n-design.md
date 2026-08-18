# Route Tabs and Frontend i18n Design

## 1. Goal

Complete the protected Admin shell before RBAC work begins by adding the
missing route-tab navigation from `D:/github-project/admin_front_ts`, giving
it a conventional name, making `main` the only vertical scroll owner, and
establishing frontend internationalization for Chinese and English.

The reference project defines the target interaction and visual result. Its
Menu Store, persistence kernel, component wrappers, dynamic-route registry,
and other application architecture are not copied. The implementation in this
repository remains direct and linear.

## 2. Scope

This slice includes:

- a new `RouteTabs.vue` layout component;
- the complete reference interaction set: previous/next navigation, refresh,
  content fullscreen, close current, close others, close all, right-click
  actions, and active-tab scrolling;
- a fixed application viewport where `main` scrolls internally and visible
  scrollbars are hidden without disabling wheel, keyboard, or touch scrolling;
- a small `vue-i18n` foundation with `zh-CN` and `en-US`;
- a Header language command and persisted display-language preference;
- translation of all currently implemented user-visible Login, layout,
  Dashboard, validation, and client-generated error text, so changing language
  does not produce a mixed-language first-phase application;
- typed route metadata for translated route titles and fixed tabs.

The backend i18n contract and RBAC are separate follow-up slices. They are
sequenced after this slice so RBAC menu and error contracts do not need to be
redesigned later.

## 3. Non-goals and constraints

- Do not add a Menu Store, route-tab Pinia Store, persistence framework, global
  component registry, generic layout framework, adapter, manager, or factory.
- Do not persist visited tabs across a browser reload in this slice. RBAC must
  first define how restored tabs are revalidated against current permission
  routes.
- Do not change authentication, health, or task API contracts.
- Do not add fake routes or business menus to demonstrate multiple tabs.
- Do not add explicit TypeScript `any`, `as any`, or TypeScript suppression.
- Do not silently fall back from a missing translation to another locale.
  Both locale files must contain the same statically verified message shape.
- Do not start or stop the user's services and do not commit or push.

## 4. Naming and file boundaries

The new component is named `RouteTabs`, not `TabTag` or `TagsView`. A single
item is a `RouteTab`.

Expected new files:

```text
web/src/i18n/index.ts
web/src/i18n/messages/zh-CN.ts
web/src/i18n/messages/en-US.ts
web/src/i18n/index.test.ts
web/src/layout/components/RouteTabs.vue
web/src/layout/components/RouteTabs.test.ts
```

Expected modified files are limited to the application bootstrap, current
router metadata, current Login/layout/Dashboard surfaces and tests, shared
styles, and package metadata required for `vue-i18n`.

Existing `AppAside`, `AppHeader`, and `AppFooter` names remain unchanged. A
broader component naming cleanup would be unrelated churn immediately before
RBAC.

## 5. Layout and scrolling

The protected layout becomes:

```text
AdminLayout
|-- AppAside
`-- Workspace
    |-- AppHeader
    |-- RouteTabs
    |-- Main
    `-- AppFooter
```

`html`, `body`, and `#app` occupy the viewport and use `overflow: hidden`.
`admin-layout` and its workspace use a fixed `100dvh` height with flex columns
and `min-height: 0`. Header, RouteTabs, and Footer do not shrink. Main uses
`flex: 1`, `min-height: 0`, and `overflow: auto`.

All application scroll containers hide their visible scrollbar with shared
Firefox and WebKit rules, but scrolling remains enabled for mouse wheels,
touch, keyboard, and programmatic focus. RouteTabs retains its own horizontal
scrolling under the same rule. No page component may create a body-level
scroll owner.

Content fullscreen hides Aside, Header, and Footer while preserving RouteTabs
and Main, matching the reference behavior. Mobile continues to use the current
Element Plus Drawer.

## 6. Route metadata and tab model

Protected leaf routes declare:

```ts
interface RouteMeta {
  requiresAuth: boolean
  titleKey?: AppMessageKey
  affix?: boolean
}
```

The authenticated layout route may omit `titleKey`; each visible leaf route
must provide it. Dashboard uses `navigation.dashboard` and `affix: true`.
Router tests enforce these rules.

The local tab model contains only data needed to render and navigate:

```ts
interface RouteTab {
  path: string
  titleKey: AppMessageKey
  affix: boolean
}
```

No menu API object or RBAC permission object is stored in RouteTabs. The later
RBAC route builder only needs to create normal Vue Router records with the same
metadata.

## 7. Linear data flow

The route-tab flow is:

```text
route changes
  -> RouteTabs reads current leaf route metadata
  -> add or activate one RouteTab
  -> translate titleKey for display
  -> user action calls Vue Router or emits one layout command
```

RouteTabs owns its in-memory visited list and context-menu state. It calls
`router.push` for navigation. It emits only `refresh` and `toggleFullscreen`,
because those operations affect the RouterView and outer shell.

AdminLayout owns:

- `refreshKey`, incremented to remount the current RouterView;
- `contentFullscreen`, used directly to show or hide shell regions.

The RouterView key is `route.fullPath + '::' + refreshKey`. No event bus or
global navigation Store is introduced.

## 8. Route-tab behavior

- Dashboard is fixed and cannot be closed.
- Visiting a protected leaf route adds it once and activates it.
- Clicking a tab navigates to its path.
- Closing the active tab selects the nearest remaining tab; Dashboard is the
  final destination when no closable tab remains.
- Close others retains Dashboard and the selected/current tab.
- Close all retains Dashboard and navigates to it.
- Previous and next buttons navigate relative to the active tab and expose
  disabled states at the ends.
- Refresh remounts only the active RouterView.
- Content fullscreen hides outer layout chrome but keeps RouteTabs available.
- Right-click opens the same refresh/close/close-others/close-all commands.
- Clicking outside, pressing Escape, or navigating closes the context menu.
- The active tab scrolls into view. Reduced-motion preference disables smooth
  scrolling and transition motion.

All icon controls use Element Plus icons, tooltips or accessible labels, and
stable dimensions.

## 9. Frontend i18n

`vue-i18n` runs in Composition API mode. Supported locales are exactly
`zh-CN | en-US`; the default is `zh-CN`. The storage key is `admin:locale`.
An absent or invalid stored value resolves to the documented Chinese default.
Changing locale updates the i18n locale, persists it, and updates
`document.documentElement.lang`.

`zh-CN.ts` defines one flat, readonly message object whose keys use namespaced
dot notation, for example `navigation.dashboard` and
`layout.routeTabs.closeOthers`. `AppMessageKey` is `keyof typeof zhCN`.
`en-US.ts` must satisfy `Record<AppMessageKey, string>`, so a missing or extra
message fails TypeScript rather than silently falling back at runtime.
Vue I18n enables flat JSON key handling and disables `fallbackLocale`.

The Header adds a Translate icon dropdown for Chinese and English beside the
theme control. RouteTabs and existing pages use `t()` for user-visible text.
Server-provided messages remain unchanged in this frontend slice; the future
backend i18n layer will localize them from stable error codes and the request
language.

## 10. Backend i18n follow-up boundary

The next i18n spec must define, before RBAC implementation:

- supported locale parsing and default language;
- `Accept-Language` handling and request-context propagation;
- stable machine-readable error codes independent of translated text;
- backend-owned message catalogs and parameter interpolation;
- validation-error translation;
- missing-key behavior with no silent locale fallback;
- whether RBAC menu records carry translation keys or localized database
  values, including uniqueness and update rules.

No backend i18n code is changed in the RouteTabs/frontend-i18n slice.

## 11. Error handling

- An invalid persisted locale is treated as absent and replaced by `zh-CN`.
- A protected leaf route without `titleKey` is a programming error covered by
  router and RouteTabs tests; RouteTabs does not invent a label.
- Router navigation failures use the existing router behavior and do not leave
  the visited list pointing at a route that was not activated.
- Refresh and fullscreen are local synchronous UI operations and require no
  fallback state.

## 12. Verification

Tests cover:

- locale initialization, persistence, root `lang`, and switching;
- equal Chinese and English message shapes;
- translated Header, current route, RouteTabs commands, Login, and Dashboard;
- tab addition, deduplication, activation, navigation, close selection,
  fixed-dashboard rules, close others/all, refresh, fullscreen, context menu,
  Escape/outside dismissal, and active-tab scrolling;
- root overflow ownership and Main's hidden but active scrolling contract;
- preservation of collapse, mobile Drawer, logout, theme, auth, health, and
  task behavior.

Final verification runs the full Vitest suite, strict `vue-tsc`, production
build, AnyScript scan, and `git diff --check`. Build warnings are reported
separately from failures.
