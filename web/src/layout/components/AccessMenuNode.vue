<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { menuIcons } from '../../access/menu-icons'
import type { AccessMenuNode as AccessMenuNodeDTO } from '../../api/access.contract'

const props = defineProps<{
  node: AccessMenuNodeDTO
}>()

const { t } = useI18n()
const icon = computed(() => props.node.icon === null ? null : menuIcons[props.node.icon])
</script>

<template>
  <el-sub-menu v-if="node.menuType === 'directory'" :index="node.code">
    <template #title>
      <el-icon v-if="icon !== null"><component :is="icon" /></el-icon>
      <span>{{ t(node.titleKey) }}</span>
    </template>
    <AccessMenuNode v-for="child in node.children" :key="child.code" :node="child" />
  </el-sub-menu>

  <el-menu-item v-else-if="node.path !== null" :index="node.path">
    <el-icon v-if="icon !== null"><component :is="icon" /></el-icon>
    <template #title>{{ t(node.titleKey) }}</template>
  </el-menu-item>
</template>
