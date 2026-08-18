# Admin UI Foundation Design

## 1. Goal

Refresh the first-phase Admin UI so that the existing authentication slice and
Element Plus shell have one coherent visual language on desktop and mobile.
The visual direction is informed by `D:/github-project/admin_front_ts`, but
only its design-system ideas are reused. Its business modules, adapters,
platform abstractions, stores, i18n system, and routing structure are out of
scope.

This slice covers the login page, the protected layout, and the dashboard. A
registration page is not part of the target product experience. The current
registration API and implementation remain untouched until the separate
email-first-login authentication design is approved.

## 2. Non-goals and constraints

- Do not change the current API request and response contracts.
- Do not change authentication, refresh, session, router, or permission
  behavior except removing the visible registration entry from the UI.
- Do not introduce RBAC menus, fake user-management screens, or speculative
  components.
- Do not copy the reference project's business architecture or add adapters,
  managers, factories, or generic UI infrastructure.
- Keep Vue business TypeScript explicit; no `any`, `as any`, or suppression
  comments.
- Keep the existing `el-container` layout structure and Element Plus
  components.

## 3. Visual direction

The interface should feel like a quiet developer-facing operations console:
clear hierarchy, compact spacing, neutral surfaces, and restrained action
color. It should be easy to scan and easy to debug rather than promotional.

Element Plus is the source of truth for component colors and states. Project
styles may define semantic aliases, but they must resolve to Element Plus
variables rather than duplicate a second palette.

Required theme variables include:

- `--el-color-primary` and its light/dark variants;
- `--el-bg-color-page`, `--el-bg-color`, and `--el-bg-color-overlay`;
- `--el-text-color-primary`, `--el-text-color-regular`, and
  `--el-text-color-secondary`;
- `--el-border-color`, `--el-border-color-light`, and fill colors;
- success, warning, and danger variables used by messages and statuses.

The application uses the Element Plus `html.dark` convention for dark mode.
The theme toggle changes this root state and persists only the user's display
preference. It does not persist authentication credentials.

Visual rules:

- light mode is the default;
- surfaces use white and Element Plus page/fill backgrounds;
- panels use a maximum 8px radius, thin borders, and restrained shadows;
- no marketing hero, decorative gradient blobs, glassmorphism, or card nesting;
- icons come from `@element-plus/icons-vue` and unfamiliar icon buttons have
  accessible labels/tooltips;
- keyboard focus remains visible and reduced-motion preferences are respected.

## 4. Page design

### 4.1 Login

The page is a full-height two-column desktop composition that collapses into a
single column on mobile. The left side carries a compact product identity and
one sentence describing the Admin console. The right side contains the login
form in a clearly framed surface.

The form keeps the current username/password fields and submit behavior so the
existing API remains valid. The visible registration link is removed. Error
states remain available to the current tests and are rendered as concise
inline guidance; the request layer may also show its centralized notification.

### 4.2 Protected shell

The shell remains:

```text
el-container
├── el-aside
└── el-container
    ├── el-header
    ├── el-main
    └── el-footer
```

Desktop uses a fixed aside with a compact brand mark, one current dashboard
menu item, and collapse support. Mobile replaces the aside with an Element
Plus drawer. Header content is limited to the menu toggle, breadcrumb/current
location, theme toggle, current username, and logout command. Footer is quiet
and short.

The static dashboard entry is deliberately retained only as the temporary
first-phase shell. It must be easy to replace with backend-provided RBAC menu
items later without changing the layout boundary.

### 4.3 Dashboard

The dashboard is a practical work surface, not a marketing page. It contains a
page heading, a small set of operational summary values, a readiness/status
area, and a recent activity/task area using existing data where available.
Empty, loading, and error states are explicit. No fake RBAC or user-management
data is introduced.

## 5. File boundaries

Expected changes are limited to:

```text
web/src/styles/index.scss
web/src/styles/variables.scss
web/src/layout/index.vue
web/src/layout/components/AppAside.vue
web/src/layout/components/AppHeader.vue
web/src/layout/components/AppFooter.vue
web/src/views/auth/login/index.vue
web/src/views/auth/register/index.vue (remove from visible routing/UI)
web/src/views/dashboard/index.vue
web/src/main.ts (theme initialization only if required)
web/src/router/index.ts (remove visible registration route only if required)
web/src/permission.ts (remove the stale registration-route name check)
web/src/utils/theme.ts
web/src/utils/theme.test.ts
```

Existing tests and `data-testid` hooks must be updated only when the visible
registration experience is intentionally removed. API, store, and request
files are not part of this UI slice. The permission guard change is limited to
removing the route name that no longer exists.

## 6. Verification

Run the focused layout/auth/dashboard tests, then the full frontend suite and
production build:

```powershell
cd D:\admin\web
pnpm vitest run
pnpm exec vue-tsc -b --pretty false
pnpm build
```

Also run the explicit TypeScript AnyScript scan and `git diff --check`.

## 7. Follow-up authentication slice

The final product direction is email-first login: when an email has no user,
the backend creates the user as part of the login flow. That is intentionally
deferred. It requires a separate spec covering request fields, user creation
rules, password or passwordless behavior, email ownership, abuse controls,
database uniqueness, and public error semantics.
