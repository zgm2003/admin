<!-- 组件：个人中心头像与资料概览 -->
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

defineOptions({ name: 'ProfileHero' })

const props = defineProps<{
  name: string
  email: string
  avatarUrl: string
}>()

const emit = defineEmits<{ avatarError: [] }>()
const { t } = useI18n()
const initial = computed(() => props.name.slice(0, 1).toUpperCase() || 'A')
</script>

<template>
  <header class="profile-hero">
    <el-avatar
      class="profile-hero__avatar"
      :size="72"
      :src="avatarUrl"
      @error="emit('avatarError')"
      >{{ initial }}</el-avatar
    >
    <div class="profile-hero__copy">
      <p>{{ t('layout.user.profile') }}</p>
      <h1>{{ name }}</h1>
      <span>{{ email }}</span>
    </div>
  </header>
</template>

<style scoped lang="scss">
.profile-hero {
  display: flex;
  align-items: center;
  min-width: 0;
  padding: 30px 32px;
  gap: 20px;
  color: var(--admin-text);
  background: linear-gradient(115deg, var(--el-color-primary-light-9), var(--admin-surface) 75%);
  border: 1px solid var(--admin-border);
  border-radius: var(--admin-radius-md);
  box-shadow: var(--admin-shadow-sm);

  &__avatar {
    flex: 0 0 auto;
    color: var(--el-color-primary);
    background: var(--admin-surface);
    border: 3px solid var(--el-color-primary-light-8);
    box-shadow: var(--admin-shadow-sm);
    font-size: 26px;
    font-weight: 800;
  }

  &__copy {
    display: grid;
    min-width: 0;
    gap: 3px;
  }

  &__copy p,
  &__copy h1,
  &__copy span {
    margin: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__copy p,
  &__copy span {
    color: var(--admin-text-soft);
    font-size: 13px;
  }

  &__copy p {
    font-size: 11px;
    font-weight: 750;
    text-transform: uppercase;
  }

  &__copy h1 {
    font-size: 24px;
    font-weight: 760;
  }
}

.profile-hero {
  @media (max-width: 560px) {
    align-items: flex-start;
    flex-direction: column;
    padding: 24px 20px;
    gap: 14px;
  }
}
</style>
