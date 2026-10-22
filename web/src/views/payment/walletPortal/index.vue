<!-- 页面：钱包 -->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
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

watch(tabs, (value) => {
  if (!value.some((tab) => tab.name === activeTab.value)) {
    activeTab.value = value[0]?.name ?? 'wallet'
  }
})
function openRecharge(): void {
  if (canRecharge.value) activeTab.value = 'recharge'
}
</script>

<template>
  <AppPage class="wallet-page">
    <el-empty
      v-if="tabs.length === 0"
      :description="t('payment.wallet.noAccess')"
      data-testid="wallet-empty"
    />
    <el-tabs v-else v-model="activeTab" class="wallet-page__tabs" data-testid="wallet-tabs">
      <el-tab-pane
        v-if="canWallet"
        name="wallet"
        :label="t('payment.wallet.myTitle')"
        data-testid="wallet-tab-my-wallet"
      >
        <div data-testid="wallet-tab-panel-my-wallet">
          <MyWalletPage @recharge="openRecharge" />
        </div>
      </el-tab-pane>
      <el-tab-pane
        v-if="canRecharge"
        lazy
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
}
.wallet-page__tabs :deep(.el-tabs__header) {
  margin-bottom: 20px;
}
.wallet-page__tabs :deep(.management-page) {
  min-height: 0;
  padding: 0;
  border: 0;
  background: transparent;
}
</style>
