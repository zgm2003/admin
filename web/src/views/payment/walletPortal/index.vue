<!-- 页面：钱包 -->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { usePermissionStore } from '@/store/permission'
import MyWalletPage from '@/views/payment/myWallet/index.vue'
import RechargePage from '@/views/payment/recharge/index.vue'

const { t } = useI18n()
const access = usePermissionStore()
const canWallet = computed(() => access.hasPermission('payment:myWallet:view'))
const canRecharge = computed(() => access.hasPermission('payment:recharge:view'))
const activeTab = ref<'wallet' | 'recharge'>(canWallet.value ? 'wallet' : 'recharge')

const tabs = computed(() => [
  ...(canWallet.value ? [{ name: 'wallet' as const, label: t('payment.wallet.myTitle') }] : []),
  ...(canRecharge.value
    ? [{ name: 'recharge' as const, label: t('navigation.paymentRecharge') }]
    : []),
])

function selectTab(value: string | number): void {
  if (value === 'wallet' || value === 'recharge') activeTab.value = value
}
</script>

<template>
  <AppPage class="wallet-page">
    <header class="wallet-page__hero">
      <div>
        <p class="wallet-page__eyebrow">{{ t('payment.wallet.eyebrow') }}</p>
        <h1>{{ t('payment.wallet.pageTitle') }}</h1>
        <p class="wallet-page__intro">{{ t('payment.wallet.managementIntro') }}</p>
      </div>
      <div class="wallet-page__mark" aria-hidden="true">¥</div>
    </header>

    <el-empty
      v-if="tabs.length === 0"
      :description="t('payment.wallet.noAccess')"
      data-testid="wallet-empty"
    />
    <el-tabs
      v-else
      v-model="activeTab"
      class="wallet-page__tabs"
      type="border-card"
      data-testid="wallet-tabs"
      @tab-change="selectTab"
    >
      <el-tab-pane
        v-if="canWallet"
        name="wallet"
        :label="t('payment.wallet.myTitle')"
        data-testid="wallet-tab-my-wallet"
      >
        <div data-testid="wallet-tab-panel-my-wallet"><MyWalletPage /></div>
      </el-tab-pane>
      <el-tab-pane
        v-if="canRecharge"
        name="recharge"
        :label="t('navigation.paymentRecharge')"
        data-testid="wallet-tab-recharge"
      >
        <RechargePage />
      </el-tab-pane>
    </el-tabs>
  </AppPage>
</template>

<style scoped lang="scss">
.wallet-page {
  min-width: 0;
  gap: 18px;
  background:
    radial-gradient(circle at 92% 0%, rgb(42 157 143 / 12%), transparent 28%), var(--admin-surface);
}

.wallet-page__hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 132px;
  padding: 26px 30px;
  color: #f7fbf8;
  background: linear-gradient(115deg, #153b3a 0%, #1b5b53 58%, #2a9d8f 100%);
  border-radius: 14px;
  box-shadow: 0 16px 34px rgb(21 59 58 / 18%);
}

.wallet-page__eyebrow {
  margin: 0 0 8px;
  color: #a9e5d8;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

h1 {
  margin: 0;
  font-size: clamp(24px, 3vw, 34px);
  letter-spacing: -0.04em;
}

.wallet-page__intro {
  max-width: 520px;
  margin: 8px 0 0;
  color: rgb(247 251 248 / 72%);
  font-size: 13px;
}

.wallet-page__mark {
  display: grid;
  width: 72px;
  height: 72px;
  place-items: center;
  color: #153b3a;
  background: #f3c969;
  border-radius: 22px 22px 22px 6px;
  font-size: 34px;
  font-weight: 800;
  transform: rotate(8deg);
}

.wallet-page__tabs {
  min-width: 0;
  border: 0;
  box-shadow: none;
  background: transparent;
}

.wallet-page__tabs :deep(.el-tabs__content) {
  padding: 18px 0 0;
}

.wallet-page__tabs :deep(.management-page) {
  min-height: 0;
  padding: 0;
  border: 0;
  background: transparent;
}

@media (max-width: 560px) {
  .wallet-page__hero {
    min-height: 116px;
    padding: 22px;
  }

  .wallet-page__mark {
    width: 54px;
    height: 54px;
    font-size: 26px;
    border-radius: 16px 16px 16px 5px;
  }
}
</style>
