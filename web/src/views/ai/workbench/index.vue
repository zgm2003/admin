<!-- 页面：AI 对话工作台 -->
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChatDotRound, Delete, Edit, Plus, Promotion, VideoPause } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'

import * as api from '@/api/ai/chat'
import type { ChatActiveRun, ChatAgent, ChatConversation, ChatMessage } from '@/api/ai/chat'
import { subscribeChatRealtime, subscribeChatReconnect } from '@/realtime'
import { usePermissionStore } from '@/store/permission'
import MarkdownContent from './components/MarkdownContent/index.vue'

defineOptions({ name: 'AIWorkbenchPage' })

interface SessionState {
  messages: ChatMessage[]
  activeRun: ChatActiveRun | null
  loaded: boolean
  loading: boolean
  hasMore: boolean
  firstSeq: number
}

const { t } = useI18n()
const access = usePermissionStore()
const agents = ref<ChatAgent[]>([])
const conversations = ref<ChatConversation[]>([])
const activeId = ref<number | null>(null)
const newAgentId = ref<number | null>(null)
const input = ref('')
const sending = ref(false)
const canceling = ref(false)
const loading = ref(false)
const balance = ref<api.ChatBalance | null>(null)
const listError = ref('')
const chatError = ref('')
const sessions = reactive(new Map<number, SessionState>())
const messagesEl = ref<HTMLElement | null>(null)
const followBottom = ref(true)
const resyncing = new Set<number>()
let pollTimer: number | undefined

const canSendPermission = computed(() => access.hasPermission('ai:workbench:send'))
const canCreate = computed(() => access.hasPermission('ai:workbench:create'))
const canUpdate = computed(() => access.hasPermission('ai:workbench:update'))
const canDelete = computed(() => access.hasPermission('ai:workbench:delete'))
const activeConversation = computed(
  () => conversations.value.find((item) => item.id === activeId.value) ?? null,
)
const session = computed(() =>
  activeId.value === null ? null : (sessions.get(activeId.value) ?? null),
)
const activeRun = computed(() => session.value?.activeRun ?? null)
const streaming = computed(() => activeRun.value !== null)
const canSend = computed(
  () =>
    canSendPermission.value &&
    activeId.value !== null &&
    !sending.value &&
    !streaming.value &&
    input.value.trim() !== '' &&
    balance.value?.canSend === true,
)
const showEmpty = computed(() => activeId.value === null)

function ensureSession(id: number): SessionState {
  let state = sessions.get(id)
  if (state === undefined) {
    state = reactive({
      messages: [],
      activeRun: null,
      loaded: false,
      loading: false,
      hasMore: false,
      firstSeq: 0,
    })
    sessions.set(id, state)
  }
  return state
}

async function loadAgents(): Promise<void> {
  try {
    const result = await api.getChatAgents()
    agents.value = result.list
    if (newAgentId.value === null && result.list.length > 0) newAgentId.value = result.list[0].id
  } catch {
    chatError.value = t('ai.workbench.agentsFailed')
  }
}

async function loadBalance(): Promise<void> {
  try {
    balance.value = await api.getChatBalance()
  } catch {
    balance.value = null
  }
}

async function loadConversations(selectFirst = true): Promise<void> {
  loading.value = true
  listError.value = ''
  try {
    const result = await api.getConversations({ page: 1, pageSize: 50 })
    conversations.value = result.list
    if (activeId.value !== null && !result.list.some((item) => item.id === activeId.value)) {
      activeId.value = null
    }
    if (selectFirst && activeId.value === null && result.list.length > 0)
      await selectConversation(result.list[0].id)
  } catch {
    listError.value = t('ai.workbench.conversationsFailed')
  } finally {
    loading.value = false
  }
}

async function loadSession(id: number, older = false): Promise<void> {
  const state = ensureSession(id)
  if (state.loading) return
  state.loading = true
  try {
    const result = await api.getMessages(
      id,
      older ? { beforeSeq: state.firstSeq, limit: 50 } : { limit: 50 },
    )
    if (older) state.messages = [...result.list, ...state.messages]
    else state.messages = result.list
    state.hasMore = result.hasMore
    state.firstSeq = state.messages.length > 0 ? state.messages[0].seq : 0
    state.activeRun = result.activeRun ?? null
    state.loaded = true
    if (activeId.value === id) await nextTick(() => scrollToBottom())
  } catch {
    chatError.value = t('ai.workbench.messagesFailed')
  } finally {
    state.loading = false
  }
}

async function selectConversation(id: number): Promise<void> {
  activeId.value = id
  chatError.value = ''
  const state = ensureSession(id)
  if (!state.loaded) await loadSession(id)
  else await nextTick(() => scrollToBottom())
}

async function createConversation(): Promise<void> {
  if (newAgentId.value === null) {
    ElMessage.warning(t('ai.workbench.needAgent'))
    return
  }
  try {
    const result = await api.createConversation({ agentId: newAgentId.value })
    await loadConversations(false)
    await selectConversation(result.id)
  } catch {
    // request.ts shows the API error
  }
}

async function renameConversation(item: ChatConversation): Promise<void> {
  try {
    const result = await ElMessageBox.prompt(
      t('ai.workbench.renamePrompt'),
      t('ai.workbench.rename'),
      {
        inputValue: item.title,
        inputValidator: (value) => value.trim() !== '' && value.trim().length <= 128,
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
      },
    )
    await api.renameConversation(item.id, result.value.trim())
    await loadConversations(false)
  } catch {
    // cancelled or request failed
  }
}

async function removeConversation(item: ChatConversation): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('ai.workbench.deleteConfirm', { title: item.title }),
      t('confirm.title'),
      {
        type: 'warning',
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
      },
    )
    await api.deleteConversation(item.id)
    sessions.delete(item.id)
    if (activeId.value === item.id) activeId.value = null
    await loadConversations()
  } catch {
    // cancelled or request failed
  }
}

async function send(): Promise<void> {
  if (!canSend.value || activeId.value === null) return
  const content = input.value.trim()
  const conversationID = activeId.value
  sending.value = true
  chatError.value = ''
  try {
    const result = await api.sendMessage(conversationID, {
      content,
      requestId: crypto.randomUUID(),
    })
    input.value = ''
    const state = ensureSession(conversationID)
    if (!state.messages.some((message) => message.id === result.userMessage.id))
      state.messages.push(result.userMessage)
    if (!state.messages.some((message) => message.id === result.assistantMessage.id))
      state.messages.push(result.assistantMessage)
    state.activeRun = {
      id: result.runId,
      requestId: '',
      status: 1,
      billingStatus: 1,
      assistantMessageId: result.assistantMessage.id,
      deliverySeq: result.assistantMessage.deliverySeq,
    }
    followBottom.value = true
    void loadConversations(false)
    await nextTick(() => scrollToBottom())
    await loadSession(conversationID)
  } catch {
    // request.ts shows the API error
  } finally {
    sending.value = false
  }
}

async function cancel(): Promise<void> {
  if (activeRun.value === null || canceling.value) return
  canceling.value = true
  try {
    await api.cancelRun(activeRun.value.id)
  } catch {
    // request.ts shows the API error
  } finally {
    canceling.value = false
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return
  event.preventDefault()
  void send()
}

function scrollToBottom(): void {
  const element = messagesEl.value
  if (element === null) return
  element.scrollTop = element.scrollHeight
  followBottom.value = true
}

function onScroll(): void {
  const element = messagesEl.value
  if (element === null) return
  followBottom.value = element.scrollHeight - element.scrollTop - element.clientHeight < 80
}

function applyDelta(event: {
  conversationId: number
  assistantMessageId: number
  deliverySeq: number
  delta: string
}): void {
  const state = sessions.get(event.conversationId)
  if (state === undefined || !state.loaded) return
  const message = state.messages.find((item) => item.id === event.assistantMessageId)
  if (message === undefined) return
  if (event.deliverySeq <= message.deliverySeq) return
  if (event.deliverySeq !== message.deliverySeq + 1) {
    void resync(event.conversationId)
    return
  }
  message.content += event.delta
  message.deliverySeq = event.deliverySeq
  if (activeId.value === event.conversationId && followBottom.value)
    void nextTick(() => scrollToBottom())
}

async function resync(conversationID: number): Promise<void> {
  if (resyncing.has(conversationID)) return
  resyncing.add(conversationID)
  try {
    await loadSession(conversationID)
  } finally {
    resyncing.delete(conversationID)
  }
}

const unsubscribeEvent = subscribeChatRealtime((event) => {
  if (event.type === 'ai.chat.started.v1') {
    const state = sessions.get(event.data.conversationId)
    if (state === undefined || !state.loaded) return
    state.activeRun = {
      id: event.data.runId,
      requestId: event.data.requestId,
      status: 2,
      billingStatus: 1,
      assistantMessageId: event.data.assistantMessageId,
      deliverySeq: event.data.deliverySeq,
    }
    return
  }
  if (event.type === 'ai.chat.delta.v1') {
    applyDelta(event.data)
    return
  }
  const state = sessions.get(event.data.conversationId)
  if (state !== undefined) {
    state.activeRun = null
    const message = state.messages.find((item) => item.id === event.data.assistantMessageId)
    if (message !== undefined)
      message.status =
        event.type === 'ai.chat.completed.v1' ? 2 : event.type === 'ai.chat.canceled.v1' ? 3 : 4
  }
  if (activeId.value === event.data.conversationId) {
    void resync(event.data.conversationId)
    void loadBalance()
  } else if (state !== undefined) {
    state.loaded = false
  }
})

const unsubscribeReconnect = subscribeChatReconnect(() => {
  if (activeId.value !== null) void resync(activeId.value)
  void loadBalance()
})

watch(
  () => {
    const current = session.value
    if (current === null || current.messages.length === 0) return ''
    const last = current.messages[current.messages.length - 1]
    return `${current.messages.length}:${last.content.length}`
  },
  () => {
    if (followBottom.value) void nextTick(() => scrollToBottom())
  },
)

onMounted(async () => {
  await Promise.all([loadAgents(), loadBalance()])
  await loadConversations()
  pollTimer = window.setInterval(() => {
    if (activeId.value !== null && session.value?.activeRun) void resync(activeId.value)
  }, 5000)
})

onBeforeUnmount(() => {
  if (pollTimer !== undefined) window.clearInterval(pollTimer)
  unsubscribeEvent()
  unsubscribeReconnect()
})
</script>

<template>
  <div class="ai-workbench">
    <aside class="ai-workbench__aside">
      <div class="ai-workbench__aside-head">
        <div>
          <strong>{{ t('ai.workbench.title') }}</strong>
          <span>{{ t('ai.workbench.subtitle') }}</span>
        </div>
        <el-button
          v-if="canCreate"
          circle
          :icon="Plus"
          :aria-label="t('ai.workbench.newConversation')"
          @click="createConversation"
        />
      </div>

      <div class="ai-workbench__agent">
        <span>{{ t('ai.workbench.agent') }}</span>
        <el-select-v2
          v-model="newAgentId"
          size="small"
          :options="agents.map((item) => ({ label: item.name, value: item.id }))"
          :placeholder="t('ai.workbench.agentPlaceholder')"
        />
      </div>

      <el-alert v-if="listError" :title="listError" type="error" :closable="false" show-icon />
      <div v-loading="loading" class="ai-workbench__conversations">
        <button
          v-for="item in conversations"
          :key="item.id"
          type="button"
          class="ai-workbench__conversation"
          :class="{ 'is-active': item.id === activeId }"
          @click="selectConversation(item.id)"
        >
          <ChatDotRound class="ai-workbench__conversation-icon" />
          <span class="ai-workbench__conversation-body">
            <strong>{{ item.title }}</strong>
            <small>{{ item.agentName }} · {{ item.modelName }}</small>
          </span>
          <span class="ai-workbench__conversation-actions">
            <el-button v-if="canUpdate" link :icon="Edit" @click.stop="renameConversation(item)" />
            <el-button
              v-if="canDelete"
              link
              type="danger"
              :icon="Delete"
              @click.stop="removeConversation(item)"
            />
          </span>
        </button>
        <el-empty
          v-if="!loading && conversations.length === 0"
          :description="t('ai.workbench.empty')"
        />
      </div>
    </aside>

    <section class="ai-workbench__main">
      <header class="ai-workbench__head">
        <div>
          <strong>{{ activeConversation?.title || t('ai.workbench.welcome') }}</strong>
          <span v-if="activeConversation">
            {{ activeConversation.agentName }} · {{ activeConversation.modelName }}
          </span>
        </div>
        <div
          class="ai-workbench__balance"
          :class="{ 'is-negative': (balance?.balanceUnits ?? 0) <= 0 }"
        >
          <small>{{ t('ai.workbench.balance') }}</small>
          <strong>{{ balance?.balance ?? '--' }}</strong>
        </div>
      </header>

      <el-alert
        v-if="balance && !balance.canSend"
        :title="t('ai.workbench.insufficient')"
        type="warning"
        :closable="false"
        show-icon
      />
      <el-alert v-if="chatError" :title="chatError" type="error" :closable="false" show-icon />

      <div v-if="showEmpty" class="ai-workbench__empty">
        <el-empty :description="t('ai.workbench.selectConversation')">
          <el-button
            v-if="canCreate"
            type="primary"
            :disabled="newAgentId === null"
            @click="createConversation"
          >
            {{ t('ai.workbench.newConversation') }}
          </el-button>
        </el-empty>
      </div>

      <template v-else>
        <div ref="messagesEl" class="ai-workbench__messages" @scroll.passive="onScroll">
          <div v-if="session?.hasMore" class="ai-workbench__more">
            <el-button link @click="loadSession(activeId!, true)">{{
              t('ai.workbench.loadMore')
            }}</el-button>
          </div>
          <div
            v-for="message in session?.messages ?? []"
            :key="message.id"
            class="ai-workbench__message"
            :class="message.role === 1 ? 'is-user' : 'is-assistant'"
          >
            <div class="ai-workbench__bubble">
              <MarkdownContent
                v-if="message.role === 2"
                :content="message.content"
                :streaming="message.status === 1"
              />
              <span v-else class="ai-workbench__plain">{{ message.content }}</span>
              <div v-if="message.status === 4" class="ai-workbench__message-error">
                {{ t('ai.workbench.messageFailed') }}
              </div>
            </div>
          </div>
          <el-empty
            v-if="session && session.messages.length === 0"
            :description="t('ai.workbench.startChat')"
          />
        </div>

        <button
          v-if="!followBottom"
          type="button"
          class="ai-workbench__to-bottom"
          @click="scrollToBottom"
        >
          {{ t('ai.workbench.toBottom') }}
        </button>

        <footer class="ai-workbench__composer">
          <el-input
            v-model="input"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 6 }"
            resize="none"
            :disabled="showEmpty || !canSendPermission || streaming || balance?.canSend === false"
            :placeholder="t('ai.workbench.inputPlaceholder')"
            @keydown="onKeydown"
          />
          <div class="ai-workbench__composer-actions">
            <span>{{ t('ai.workbench.sendHint') }}</span>
            <div>
              <el-button v-if="streaming" :loading="canceling" :icon="VideoPause" @click="cancel">
                {{ t('ai.workbench.stop') }}
              </el-button>
              <el-button
                v-else
                type="primary"
                :icon="Promotion"
                :loading="sending"
                :disabled="!canSend"
                @click="send"
              >
                {{ t('ai.workbench.send') }}
              </el-button>
            </div>
          </div>
        </footer>
      </template>
    </section>
  </div>
</template>

<style scoped lang="scss">
.ai-workbench {
  display: flex;
  height: calc(100vh - 148px);
  min-height: 560px;
  overflow: hidden;
  border: 1px solid var(--admin-border);
  border-radius: var(--admin-radius-md);
  background: var(--admin-surface);
}

.ai-workbench__aside {
  display: flex;
  width: 296px;
  flex: 0 0 296px;
  flex-direction: column;
  border-right: 1px solid var(--admin-border);
  background: var(--admin-surface-soft);
}

.ai-workbench__aside-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 18px 16px 12px;
}

.ai-workbench__aside-head div {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.ai-workbench__aside-head strong {
  color: var(--admin-text);
  font-size: 15px;
}

.ai-workbench__aside-head span,
.ai-workbench__agent > span {
  color: var(--admin-text-muted);
  font-size: 12px;
}

.ai-workbench__agent {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 0 16px 12px;
}

.ai-workbench__conversations {
  flex: 1;
  padding: 0 8px 12px;
  overflow-y: auto;
}

.ai-workbench__conversation {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
  padding: 10px;
  border: 0;
  border-radius: 8px;
  color: var(--admin-text);
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.ai-workbench__conversation:hover,
.ai-workbench__conversation.is-active {
  background: var(--admin-surface);
  box-shadow: var(--admin-shadow-sm);
}

.ai-workbench__conversation-icon {
  width: 18px;
  flex: 0 0 18px;
  color: var(--el-color-primary);
}

.ai-workbench__conversation-body {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 3px;
}

.ai-workbench__conversation-body strong,
.ai-workbench__conversation-body small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-workbench__conversation-body strong {
  font-size: 13px;
  font-weight: 600;
}

.ai-workbench__conversation-body small {
  color: var(--admin-text-muted);
  font-size: 11px;
}

.ai-workbench__conversation-actions {
  display: none;
}

.ai-workbench__conversation:hover .ai-workbench__conversation-actions {
  display: flex;
}

.ai-workbench__main {
  position: relative;
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  background: var(--admin-surface);
}

.ai-workbench__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 20px;
  border-bottom: 1px solid var(--admin-border);
}

.ai-workbench__head div:first-child {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.ai-workbench__head strong {
  overflow: hidden;
  color: var(--admin-text);
  font-size: 15px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-workbench__head span {
  color: var(--admin-text-muted);
  font-size: 12px;
}

.ai-workbench__balance {
  display: flex;
  align-items: flex-end;
  flex-direction: column;
  gap: 2px;
}

.ai-workbench__balance small {
  color: var(--admin-text-muted);
  font-size: 11px;
}

.ai-workbench__balance strong {
  color: var(--el-color-primary);
  font-family: var(--admin-font-data);
  font-size: 15px;
}

.ai-workbench__balance.is-negative strong {
  color: var(--el-color-danger);
}

.ai-workbench__main :deep(.el-alert) {
  margin: 8px 16px 0;
}

.ai-workbench__messages {
  position: relative;
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 16px;
  padding: 20px 24px;
  overflow-y: auto;
}

.ai-workbench__message {
  display: flex;
  max-width: 100%;
}

.ai-workbench__message.is-user {
  justify-content: flex-end;
}

.ai-workbench__message.is-assistant {
  justify-content: flex-start;
}

.ai-workbench__bubble {
  max-width: min(760px, 88%);
  padding: 10px 14px;
  border: 1px solid var(--admin-border);
  border-radius: 12px;
  background: var(--admin-surface-soft);
}

.ai-workbench__message.is-user .ai-workbench__bubble {
  border-color: transparent;
  color: #fff;
  background: var(--el-color-primary);
}

.ai-workbench__plain {
  white-space: pre-wrap;
}

.ai-workbench__message-error {
  margin-top: 6px;
  color: var(--el-color-danger);
  font-size: 12px;
}

.ai-workbench__more {
  text-align: center;
}

.ai-workbench__to-bottom {
  position: absolute;
  right: 40px;
  bottom: 136px;
  padding: 5px 12px;
  border: 1px solid var(--admin-border);
  border-radius: 999px;
  color: var(--admin-text-soft);
  background: var(--admin-surface);
  box-shadow: var(--admin-shadow-sm);
  cursor: pointer;
}

.ai-workbench__composer {
  padding: 12px 20px 16px;
  border-top: 1px solid var(--admin-border);
}

.ai-workbench__composer-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 8px;
}

.ai-workbench__composer-actions > span {
  color: var(--admin-text-muted);
  font-size: 11px;
}

.ai-workbench__empty {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: center;
}

@media (max-width: 900px) {
  .ai-workbench__aside {
    width: 230px;
    flex-basis: 230px;
  }

  .ai-workbench__messages {
    padding: 16px;
  }
}
</style>
