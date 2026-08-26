<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

import { logout } from "../api/auth";
import { readLocale, setLocale, type AppLocale } from "../i18n";
import { useAccessStore } from "../store/access";
import { useAuthStore } from "../store/auth";
import { useUIPreferencesStore } from "../store/ui-preferences";
import { ProtocolError } from "../types/http";
import { resolveBreadcrumbs } from "./breadcrumbs";
import AppAside from "./components/AppAside.vue";
import AppFooter from "./components/AppFooter.vue";
import AppHeader from "./components/AppHeader.vue";
import RouteTabs from "./components/RouteTabs.vue";

const mobileBreakpoint = 840;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const access = useAccessStore();
const auth = useAuthStore();
const uiPreferences = useUIPreferencesStore();
const collapsed = ref(false);
const mobileMenuOpen = ref(false);
const isMobile = ref(window.innerWidth <= mobileBreakpoint);
const logoutPending = ref(false);
const locale = ref<AppLocale>(readLocale());
const refreshKey = ref(0);
const contentFullscreen = ref(false);

const asideWidth = computed(() => (collapsed.value ? "80px" : "248px"));
const username = computed(() => (auth.user === null ? "" : auth.user.username));
const breadcrumbs = computed(
  () => resolveBreadcrumbs(route.path, access.menuTree) ?? [],
);
const breadcrumbMissing = computed(
  () =>
    route.meta.requiresAuth === true &&
    route.path !== "/dashboard" &&
    access.status === "ready" &&
    breadcrumbs.value.length === 0,
);
const preferenceErrorMessage = computed(() => {
  if (uiPreferences.persistenceError === "invalid")
    return t("layout.settings.invalidStorage");
  if (uiPreferences.persistenceError === "write")
    return t("layout.settings.writeFailed");
  return "";
});

function updateViewport(): void {
  isMobile.value = window.innerWidth <= mobileBreakpoint;
  if (!isMobile.value) mobileMenuOpen.value = false;
}

function toggleMenu(): void {
  if (isMobile.value) {
    mobileMenuOpen.value = true;
    return;
  }
  collapsed.value = !collapsed.value;
}

function handleLocaleChange(nextLocale: AppLocale): void {
  locale.value = nextLocale;
  setLocale(nextLocale);
}

function handleRefresh(): void {
  refreshKey.value += 1;
}

function handleToggleFullscreen(): void {
  contentFullscreen.value = !contentFullscreen.value;
}

async function handleLogout(): Promise<void> {
  if (logoutPending.value) return;
  logoutPending.value = true;
  try {
    await logout();
    access.reset();
    auth.setAnonymous();
    await router.replace({ name: "login" });
  } catch (error: unknown) {
    const message =
      error instanceof ProtocolError
        ? t("request.protocolError")
        : error instanceof Error && error.message !== ""
          ? error.message
          : t("auth.logoutFailed");
    ElMessage.error(message);
  } finally {
    logoutPending.value = false;
  }
}

watch(
  () => uiPreferences.preferences.showMenuToggle,
  (visible) => {
    if (!visible) collapsed.value = false;
  },
  { immediate: true },
);

onMounted(() => window.addEventListener("resize", updateViewport));
onBeforeUnmount(() => window.removeEventListener("resize", updateViewport));
</script>

<template>
  <el-container class="admin-layout">
    <el-aside
      v-if="!contentFullscreen"
      class="admin-layout__aside"
      :width="asideWidth"
    >
      <AppAside
        :collapsed="collapsed"
        :unique-opened="uiPreferences.preferences.uniqueOpened"
      />
    </el-aside>

    <el-container class="admin-layout__workspace">
      <el-header
        v-if="!contentFullscreen"
        class="admin-layout__header"
        height="56px"
      >
        <AppHeader
          :locale="locale"
          :breadcrumbs="breadcrumbs"
          :show-breadcrumb="uiPreferences.preferences.showBreadcrumb"
          :show-menu-toggle="uiPreferences.preferences.showMenuToggle"
          :content-fullscreen="contentFullscreen"
          :username="username"
          :logout-pending="logoutPending"
          @toggle-menu="toggleMenu"
          @change-locale="handleLocaleChange"
          @logout="handleLogout"
        />
      </el-header>

      <div
        v-if="uiPreferences.preferences.showRouteTabs || contentFullscreen"
        class="admin-layout__tabs admin-layout__horizontal-scroll"
      >
        <RouteTabs
          :fullscreen="contentFullscreen"
          :menu-tree="access.menuTree"
          @refresh="handleRefresh"
          @toggle-fullscreen="handleToggleFullscreen"
        />
      </div>

      <el-main class="admin-layout__main admin-layout__scroll-owner">
        <el-alert
          v-if="access.status === 'error'"
          data-testid="access-error"
          type="error"
          :title="access.errorMessage"
          :closable="false"
          show-icon
        />
        <el-alert
          v-if="uiPreferences.persistenceError !== null"
          data-testid="preference-error"
          type="error"
          :title="preferenceErrorMessage"
          :closable="false"
          show-icon
        />
        <el-alert
          v-if="breadcrumbMissing"
          data-testid="breadcrumb-missing"
          type="error"
          :title="t('layout.breadcrumb.missing')"
          :closable="false"
          show-icon
        />
        <RouterView v-slot="{ Component }">
          <Transition
            v-if="uiPreferences.preferences.pageTransition"
            :name="uiPreferences.preferences.transitionName"
            mode="out-in"
          >
            <component
              :is="Component"
              :key="`${route.fullPath}::${refreshKey}`"
            />
          </Transition>
          <component
            v-else
            :is="Component"
            :key="`${route.fullPath}::${refreshKey}`"
          />
        </RouterView>
      </el-main>

      <el-footer
        v-if="uiPreferences.preferences.showFooter && !contentFullscreen"
        class="admin-layout__footer"
        height="40px"
      >
        <AppFooter />
      </el-footer>
    </el-container>

    <el-drawer
      v-if="!contentFullscreen"
      v-model="mobileMenuOpen"
      class="admin-layout__drawer"
      direction="ltr"
      :with-header="false"
      size="264px"
    >
      <AppAside
        :collapsed="false"
        :unique-opened="uiPreferences.preferences.uniqueOpened"
      />
    </el-drawer>
  </el-container>
</template>
