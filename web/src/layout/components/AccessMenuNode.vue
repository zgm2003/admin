<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { AccessMenuNode as AccessMenuNodeDTO } from '../../api/rbac/access'
import { DIcon } from '../../components/DIcon'
import { YesNo } from '../../enums/yes-no'

defineProps<{
  node: AccessMenuNodeDTO
}>()

const { t } = useI18n()
</script>

<template>
	<template v-if="node.isHidden === YesNo.No">
		<el-sub-menu v-if="node.menuType === 'directory'" class="app-aside__menu-node" :index="node.code">
			<template #title>
				<DIcon v-if="node.icon !== null" :icon="node.icon" />
				<span>{{ t(node.i18nKey) }}</span>
			</template>
			<AccessMenuNode v-for="child in node.children" :key="child.code" :node="child" />
		</el-sub-menu>

		<el-menu-item v-else-if="node.path !== null" class="app-aside__menu-node" :index="node.path">
			<DIcon v-if="node.icon !== null" :icon="node.icon" />
			<template #title>{{ t(node.i18nKey) }}</template>
		</el-menu-item>
	</template>
</template>
