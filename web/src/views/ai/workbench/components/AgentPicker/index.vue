<script setup lang="ts">
import { computed } from 'vue'
import { Check, ChevronDown } from 'lucide-vue-next'

import type { ChatAgent } from '@/api/ai/chat'

const props = withDefaults(
  defineProps<{
    modelValue: number | null
    agents: ChatAgent[]
    avatarUrls: Record<string, string>
    placeholder?: string
  }>(),
  { placeholder: 'Select agent' },
)

const emit = defineEmits<{ select: [id: number] }>()

const selected = computed(() => props.agents.find((agent) => agent.id === props.modelValue) ?? null)

function choose(id: number): void {
  if (id !== props.modelValue) emit('select', id)
}
</script>

<template>
  <el-dropdown
    trigger="click"
    placement="top-start"
    popper-class="ai-agent-picker__popper"
    :disabled="agents.length === 0"
    @command="choose"
  >
    <button type="button" class="ai-agent-picker__trigger" :aria-label="placeholder">
      <el-avatar
        class="ai-agent-picker__avatar"
        :size="24"
        :src="selected ? avatarUrls[selected.avatar] : undefined"
      >
        {{ selected?.name.slice(0, 1) }}
      </el-avatar>
      <span class="ai-agent-picker__copy">
        <strong>{{ selected?.name || placeholder }}</strong>
        <small v-if="selected">{{ selected.modelName }}</small>
      </span>
      <ChevronDown :size="14" class="ai-agent-picker__chevron" aria-hidden="true" />
    </button>
    <template #dropdown>
      <el-dropdown-menu class="ai-agent-picker__menu">
        <el-dropdown-item
          v-for="agent in agents"
          :key="agent.id"
          :command="agent.id"
          class="ai-agent-picker__item"
        >
          <div class="ai-agent-picker__option">
            <el-avatar class="ai-agent-picker__avatar" :size="28" :src="avatarUrls[agent.avatar]">
              {{ agent.name.slice(0, 1) }}
            </el-avatar>
            <span class="ai-agent-picker__copy">
              <strong>{{ agent.name }}</strong>
              <small>{{ agent.modelName }}</small>
            </span>
            <Check
              v-if="agent.id === modelValue"
              :size="15"
              class="ai-agent-picker__check"
              aria-hidden="true"
            />
          </div>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<style scoped lang="scss">
.ai-agent-picker__trigger {
  display: inline-flex;
  max-width: 230px;
  min-width: 0;
  align-items: center;
  gap: 7px;
  padding: 3px 7px 3px 3px;
  border: 1px solid #e4eaf3;
  border-radius: 9px;
  color: #38445a;
  background: #fff;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  transition:
    border-color 150ms ease,
    box-shadow 150ms ease,
    background 150ms ease;
}

.ai-agent-picker__trigger:hover,
.ai-agent-picker__trigger:focus-visible {
  border-color: #b9cff0;
  outline: none;
  background: #f7faff;
  box-shadow: 0 0 0 3px rgb(37 99 235 / 7%);
}

.ai-agent-picker__avatar {
  flex: 0 0 auto;
  color: #315a9d;
  background: linear-gradient(135deg, #e8f0ff, #f2edff);
  font-size: 10px;
  font-weight: 750;
}

.ai-agent-picker__copy {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 1px;
}

.ai-agent-picker__copy strong,
.ai-agent-picker__copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-agent-picker__copy strong {
  color: #354157;
  font-size: 11px;
  font-weight: 650;
  line-height: 1.3;
}

.ai-agent-picker__copy small {
  color: #98a2b3;
  font-size: 9px;
  line-height: 1.2;
}

.ai-agent-picker__chevron {
  flex: 0 0 auto;
  color: #9aa5b7;
}
</style>

<style lang="scss">
.ai-agent-picker__popper {
  min-width: 260px;
}

.ai-agent-picker__menu {
  padding: 5px;
}

.ai-agent-picker__item {
  margin: 1px 0;
  padding: 7px 8px !important;
  border-radius: 8px;
}

.ai-agent-picker__option {
  display: flex;
  width: 100%;
  min-width: 0;
  align-items: center;
  gap: 9px;
}

.ai-agent-picker__option .ai-agent-picker__avatar {
  flex: 0 0 auto;
  color: #315a9d;
  background: linear-gradient(135deg, #e8f0ff, #f2edff);
  font-size: 11px;
  font-weight: 750;
}

.ai-agent-picker__option .ai-agent-picker__copy {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 1px;
}

.ai-agent-picker__option .ai-agent-picker__copy strong,
.ai-agent-picker__option .ai-agent-picker__copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-agent-picker__option .ai-agent-picker__copy strong {
  color: #354157;
  font-size: 12px;
  font-weight: 650;
}

.ai-agent-picker__option .ai-agent-picker__copy small {
  color: #98a2b3;
  font-size: 10px;
}

.ai-agent-picker__check {
  flex: 0 0 auto;
  color: #2563eb;
}
</style>
