<!-- 页面：AI 对话工作台 -->
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ChatDotRound,
  Delete,
  Edit,
  MoreFilled,
  Plus,
  Promotion,
  Search,
  VideoPause,
} from '@element-plus/icons-vue'
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

interface ConversationGroup {
  label: string
  items: ChatConversation[]
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
const searchKeyword = ref('')
const sessions = reactive(new Map<number, SessionState>())
const messagesEl = ref<HTMLElement | null>(null)
const followBottom = ref(true)
const resyncing = new Set<number>()
let pollTimer: number | undefined
let searchTimer: number | undefined

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
const showWelcome = computed(
  () =>
    showEmpty.value ||
    (session.value !== null && session.value.loaded && session.value.messages.length === 0),
)
const searchActive = computed(() => searchKeyword.value.trim() !== '')
const balanceText = computed(() => {
  const raw = balance.value?.balance
  if (raw === undefined || raw === '') return '--'
  const value = Number(raw)
  return Number.isFinite(value) ? value.toFixed(2) : raw
})
const welcomeTitle = computed(() =>
  activeConversation.value === null
    ? t('ai.workbench.startTitle')
    : t('ai.workbench.firstMessageTitle', { name: activeConversation.value.agentName }),
)
const welcomeTip = computed(() =>
  activeConversation.value === null
    ? t('ai.workbench.startHint')
    : t('ai.workbench.firstMessageHint'),
)
const starterPrompts = computed(() => [
  { label: t('ai.workbench.starterSummarize'), prompt: t('ai.workbench.starterSummarizePrompt') },
  { label: t('ai.workbench.starterCode'), prompt: t('ai.workbench.starterCodePrompt') },
  { label: t('ai.workbench.starterExplain'), prompt: t('ai.workbench.starterExplainPrompt') },
])
const groupedConversations = computed<ConversationGroup[]>(() => {
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const yesterday = today - 24 * 60 * 60 * 1000
  const week = today - 6 * 24 * 60 * 60 * 1000
  const groups: ConversationGroup[] = [
    { label: t('ai.workbench.groupToday'), items: [] },
    { label: t('ai.workbench.groupYesterday'), items: [] },
    { label: t('ai.workbench.groupWeek'), items: [] },
    { label: t('ai.workbench.groupEarlier'), items: [] },
  ]
  for (const item of conversations.value) {
    const timestamp = Date.parse(item.lastMessageAt || item.updatedAt)
    const index =
      Number.isNaN(timestamp) || timestamp >= today
        ? 0
        : timestamp >= yesterday
          ? 1
          : timestamp >= week
            ? 2
            : 3
    groups[index]?.items.push(item)
  }
  return groups.filter((group) => group.items.length > 0)
})

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
    const keyword = searchKeyword.value.trim()
    const result = await api.getConversations({
      page: 1,
      pageSize: 50,
      ...(keyword === '' ? {} : { keyword }),
    })
    conversations.value = result.list
    if (activeId.value !== null && !result.list.some((item) => item.id === activeId.value)) {
      activeId.value = null
    }
    if (selectFirst && keyword === '' && activeId.value === null && result.list.length > 0)
      await selectConversation(result.list[0].id)
  } catch {
    listError.value = t('ai.workbench.conversationsFailed')
  } finally {
    loading.value = false
  }
}

function onSearchInput(value: string): void {
  searchKeyword.value = value
  if (searchTimer !== undefined) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    void loadConversations(false)
  }, 260)
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
    searchKeyword.value = ''
    const result = await api.createConversation({ agentId: newAgentId.value })
    await loadConversations(false)
    await selectConversation(result.id)
  } catch {
    // request.ts shows the API error
  }
}

function selectAgent(id: number): void {
  newAgentId.value = id
}

function useStarter(prompt: string): void {
  input.value = prompt
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
  if (searchTimer !== undefined) window.clearTimeout(searchTimer)
  unsubscribeEvent()
  unsubscribeReconnect()
})
</script>

<template>
  <div class="ai-workbench">
    <aside class="ai-workbench__aside">
      <section class="ai-workbench__agents" :aria-label="t('ai.workbench.agent')">
        <div class="ai-workbench__section-label">{{ t('ai.workbench.agent') }}</div>
        <button
          v-for="agent in agents"
          :key="agent.id"
          type="button"
          class="ai-workbench__agent-item"
          :class="{ 'is-active': agent.id === newAgentId }"
          @click="selectAgent(agent.id)"
        >
          <span class="ai-workbench__agent-avatar">{{ agent.name.slice(0, 1) }}</span>
          <span class="ai-workbench__agent-copy">
            <strong>{{ agent.name }}</strong>
            <small>{{ agent.modelName }}</small>
          </span>
          <span v-if="agent.id === newAgentId" class="ai-workbench__agent-indicator" />
        </button>
        <p v-if="agents.length === 0" class="ai-workbench__sidebar-tip">
          {{ t('ai.workbench.noAgent') }}
        </p>
      </section>

      <div class="ai-workbench__actions">
        <el-button
          v-if="canCreate"
          class="ai-workbench__new-chat"
          :disabled="newAgentId === null"
          @click="createConversation"
        >
          <Plus :size="16" />
          <span>{{ t('ai.workbench.newConversation') }}</span>
        </el-button>
        <el-input
          v-model="searchKeyword"
          class="ai-workbench__search"
          :prefix-icon="Search"
          :placeholder="t('ai.workbench.searchPlaceholder')"
          clearable
          @input="onSearchInput"
          @clear="onSearchInput('')"
        />
      </div>

      <el-alert
        v-if="listError"
        class="ai-workbench__aside-alert"
        :title="listError"
        type="error"
        :closable="false"
        show-icon
      />
      <div v-loading="loading" class="ai-workbench__conversations">
        <section
          v-for="group in groupedConversations"
          :key="group.label"
          class="ai-workbench__group"
        >
          <div class="ai-workbench__group-label">{{ group.label }}</div>
          <div
            v-for="item in group.items"
            :key="item.id"
            class="ai-workbench__conversation"
            :class="{ 'is-active': item.id === activeId }"
          >
            <button
              type="button"
              class="ai-workbench__conversation-main"
              @click="selectConversation(item.id)"
            >
              <span class="ai-workbench__conversation-title">{{ item.title }}</span>
              <span class="ai-workbench__conversation-meta">
                {{ item.agentName }} · {{ item.modelName }}
              </span>
            </button>
            <el-dropdown trigger="click" class="ai-workbench__conversation-menu">
              <el-button
                text
                :icon="MoreFilled"
                :aria-label="t('ai.workbench.moreActions')"
                @click.stop
              />
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-if="canUpdate" :icon="Edit" @click="renameConversation(item)">
                    {{ t('ai.workbench.rename') }}
                  </el-dropdown-item>
                  <el-dropdown-item
                    v-if="canDelete"
                    :icon="Delete"
                    divided
                    @click="removeConversation(item)"
                  >
                    {{ t('common.delete') }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </section>
        <div v-if="!loading && conversations.length === 0" class="ai-workbench__sidebar-empty">
          <ChatDotRound :size="34" />
          <strong>
            {{ searchActive ? t('ai.workbench.searchEmpty') : t('ai.workbench.empty') }}
          </strong>
          <span>
            {{ searchActive ? t('ai.workbench.searchEmptyHint') : t('ai.workbench.emptyHint') }}
          </span>
        </div>
      </div>

      <footer class="ai-workbench__aside-footer">
        <span class="ai-workbench__status-dot" />
        <span>{{ t('ai.workbench.privateHint') }}</span>
      </footer>
    </aside>

    <section class="ai-workbench__main">
      <header class="ai-workbench__head">
        <div class="ai-workbench__head-copy">
          <strong>{{ activeConversation?.title || t('ai.workbench.welcome') }}</strong>
          <span v-if="activeConversation">
            {{ activeConversation.agentName }} · {{ activeConversation.modelName }}
          </span>
          <span v-else>{{ t('ai.workbench.subtitle') }}</span>
        </div>
        <div
          class="ai-workbench__balance"
          :class="{ 'is-negative': (balance?.balanceUnits ?? 0) <= 0 }"
        >
          <small>{{ t('ai.workbench.balance') }}</small>
          <strong>¥{{ balanceText }}</strong>
        </div>
      </header>

      <div v-if="balance && !balance.canSend" class="ai-workbench__alert">
        <el-alert
          :title="t('ai.workbench.insufficient')"
          type="warning"
          :closable="false"
          show-icon
        />
      </div>
      <div v-if="chatError" class="ai-workbench__alert">
        <el-alert :title="chatError" type="error" :closable="false" show-icon />
      </div>

      <div ref="messagesEl" class="ai-workbench__messages" @scroll.passive="onScroll">
        <div v-if="session?.loading && !session?.loaded" class="ai-workbench__loading">
          <span class="ai-workbench__typing-dot" />
          <span class="ai-workbench__typing-dot" />
          <span class="ai-workbench__typing-dot" />
        </div>

        <div v-else-if="showWelcome" class="ai-workbench__welcome">
          <span class="ai-workbench__welcome-mark">
            <ChatDotRound :size="30" />
          </span>
          <h1>{{ welcomeTitle }}</h1>
          <p>{{ welcomeTip }}</p>
          <el-button
            v-if="showEmpty && canCreate"
            type="primary"
            class="ai-workbench__welcome-action"
            :disabled="newAgentId === null"
            @click="createConversation"
          >
            <Plus :size="16" />
            <span>{{ t('ai.workbench.newConversation') }}</span>
          </el-button>
          <div v-else-if="!showEmpty" class="ai-workbench__starters">
            <button
              v-for="starter in starterPrompts"
              :key="starter.label"
              type="button"
              class="ai-workbench__starter"
              @click="useStarter(starter.prompt)"
            >
              <span>{{ starter.label }}</span>
              <small>{{ t('ai.workbench.starterAction') }}</small>
            </button>
          </div>
        </div>

        <template v-else>
          <div v-if="session?.hasMore" class="ai-workbench__more">
            <el-button link @click="loadSession(activeId!, true)">
              {{ t('ai.workbench.loadMore') }}
            </el-button>
          </div>
          <article
            v-for="message in session?.messages ?? []"
            :key="message.id"
            class="ai-workbench__message"
            :class="message.role === 1 ? 'is-user' : 'is-assistant'"
          >
            <span v-if="message.role === 2" class="ai-workbench__message-mark">
              <ChatDotRound :size="15" />
            </span>
            <div class="ai-workbench__message-body">
              <MarkdownContent
                v-if="message.role === 2 && (message.content || message.status !== 1)"
                :content="message.content"
                :streaming="message.status === 1"
              />
              <span v-else-if="message.role === 1" class="ai-workbench__plain">
                {{ message.content }}
              </span>
              <span v-else class="ai-workbench__typing">
                <span class="ai-workbench__typing-dot" />
                <span class="ai-workbench__typing-dot" />
                <span class="ai-workbench__typing-dot" />
              </span>
              <div v-if="message.status === 4" class="ai-workbench__message-error">
                {{ t('ai.workbench.messageFailed') }}
              </div>
            </div>
          </article>
        </template>
      </div>

      <button
        v-if="!followBottom && !showWelcome"
        type="button"
        class="ai-workbench__to-bottom"
        @click="scrollToBottom"
      >
        {{ t('ai.workbench.toBottom') }}
      </button>

      <footer v-if="!showEmpty" class="ai-workbench__composer">
        <div class="ai-workbench__composer-shell">
          <el-input
            v-model="input"
            class="ai-workbench__composer-input"
            type="textarea"
            :autosize="{ minRows: 1, maxRows: 7 }"
            resize="none"
            :disabled="!canSendPermission || streaming || balance?.canSend === false"
            :placeholder="t('ai.workbench.inputPlaceholder')"
            @keydown="onKeydown"
          />
          <div class="ai-workbench__composer-bar">
            <span class="ai-workbench__composer-hint">
              {{ t('ai.workbench.inputHint') }}
            </span>
            <div class="ai-workbench__composer-actions">
              <el-button
                v-if="streaming"
                circle
                class="ai-workbench__stop"
                :loading="canceling"
                :icon="VideoPause"
                :title="t('ai.workbench.stop')"
                :aria-label="t('ai.workbench.stop')"
                @click="cancel"
              />
              <el-button
                v-else
                type="primary"
                circle
                class="ai-workbench__send"
                :icon="Promotion"
                :loading="sending"
                :disabled="!canSend"
                :title="t('ai.workbench.send')"
                :aria-label="t('ai.workbench.send')"
                @click="send"
              />
            </div>
          </div>
        </div>
      </footer>
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

/* Refined workspace shell */
:global(.admin-layout__main:has(.ai-workbench)) {
  padding: 0;
  overflow: hidden;
}

.ai-workbench {
  height: 100%;
  max-height: 100%;
  min-height: 0;
  overflow: hidden;
  border: 0;
  border-radius: 0;
  background: #fff;
  box-shadow: none;
}

.ai-workbench__aside {
  width: 286px;
  flex: 0 0 286px;
  background: linear-gradient(180deg, #f8fafc 0%, #f3f6fa 100%);
  border-right: 1px solid #e8edf3;
}

.ai-workbench__brand {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 18px 16px 14px;
}

.ai-workbench__brand-mark {
  display: grid;
  width: 36px;
  height: 36px;
  flex: 0 0 36px;
  place-items: center;
  border-radius: 11px;
  color: #fff;
  background: linear-gradient(135deg, #2563eb 0%, #6d5dfc 100%);
  box-shadow: 0 6px 16px rgb(37 99 235 / 24%);
}

.ai-workbench__brand-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
}

.ai-workbench__brand-copy strong {
  color: #172033;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.ai-workbench__brand-copy small {
  color: #8a94a6;
  font-size: 11px;
}

.ai-workbench__agents {
  max-height: 224px;
  padding: 14px 10px 8px;
  overflow-y: auto;
}

.ai-workbench__section-label {
  padding: 4px 7px 6px;
  color: #98a2b3;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.ai-workbench__agent-item {
  position: relative;
  display: flex;
  width: 100%;
  align-items: center;
  gap: 9px;
  margin-bottom: 3px;
  padding: 7px 9px;
  border: 1px solid transparent;
  border-radius: 10px;
  color: #263044;
  background: transparent;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  transition:
    background 150ms ease,
    border-color 150ms ease,
    box-shadow 150ms ease;
}

.ai-workbench__agent-item:hover {
  background: rgb(255 255 255 / 70%);
}

.ai-workbench__agent-item.is-active {
  border-color: #dce6f5;
  background: #fff;
  box-shadow: 0 2px 8px rgb(15 23 42 / 5%);
}

.ai-workbench__agent-avatar {
  display: grid;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  place-items: center;
  border-radius: 9px;
  color: #315a9d;
  background: linear-gradient(135deg, #e8f0ff, #f2edff);
  font-size: 12px;
  font-weight: 750;
}

.ai-workbench__agent-item.is-active .ai-workbench__agent-avatar {
  color: #fff;
  background: linear-gradient(135deg, #2563eb, #6d5dfc);
}

.ai-workbench__agent-copy {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 1px;
}

.ai-workbench__agent-copy strong,
.ai-workbench__agent-copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-workbench__agent-copy strong {
  color: #263044;
  font-size: 13px;
  font-weight: 600;
}

.ai-workbench__agent-copy small {
  color: #919bad;
  font-size: 10px;
}

.ai-workbench__agent-indicator {
  width: 6px;
  height: 6px;
  flex: 0 0 6px;
  border-radius: 50%;
  background: #2563eb;
  box-shadow: 0 0 0 3px rgb(37 99 235 / 10%);
}

.ai-workbench__sidebar-tip {
  margin: 6px 8px;
  color: #98a2b3;
  font-size: 12px;
}

.ai-workbench__actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 2px 12px 10px;
}

.ai-workbench__new-chat {
  width: 100%;
  height: 38px;
  justify-content: flex-start;
  margin: 0;
  padding: 0 12px;
  border: 1px solid #dbe6f6;
  border-radius: 10px;
  color: #2f5f9e;
  background: #fff;
  font-weight: 600;
  box-shadow: 0 1px 2px rgb(15 23 42 / 4%);
}

.ai-workbench__new-chat:hover,
.ai-workbench__new-chat:focus-visible {
  border-color: #9dbce7;
  color: #1d4ed8;
  background: #f5f9ff;
}

.ai-workbench__new-chat :deep(span) {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.ai-workbench__search :deep(.el-input__wrapper) {
  min-height: 36px;
  border-radius: 10px;
  background: #fff;
  box-shadow: 0 0 0 1px #e4e9f1 inset;
}

.ai-workbench__search :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px #8bb2e8 inset;
}

.ai-workbench__search :deep(.el-input__inner) {
  font-size: 12px;
}

.ai-workbench__aside-alert {
  margin: 0 12px 8px;
}

.ai-workbench__conversations {
  flex: 1;
  min-height: 0;
  padding: 0 8px 10px;
  overflow-y: auto;
}

.ai-workbench__group {
  padding-top: 4px;
}

.ai-workbench__group-label {
  padding: 8px 8px 4px;
  color: #98a2b3;
  font-size: 10px;
  font-weight: 650;
  letter-spacing: 0.04em;
}

.ai-workbench__conversation {
  position: relative;
  display: flex;
  width: 100%;
  align-items: center;
  gap: 2px;
  margin-bottom: 2px;
  padding: 3px;
  border: 1px solid transparent;
  border-radius: 10px;
  transition:
    background 140ms ease,
    border-color 140ms ease,
    box-shadow 140ms ease;
}

.ai-workbench__conversation:hover {
  background: rgb(255 255 255 / 68%);
}

.ai-workbench__conversation.is-active {
  border-color: #e1e8f2;
  background: #fff;
  box-shadow: 0 2px 8px rgb(15 23 42 / 5%);
}

.ai-workbench__conversation.is-active::before {
  position: absolute;
  top: 12px;
  bottom: 12px;
  left: 0;
  width: 2px;
  border-radius: 2px;
  background: #2563eb;
  content: '';
}

.ai-workbench__conversation-main {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 3px;
  padding: 7px 8px;
  border: 0;
  border-radius: 7px;
  color: inherit;
  background: transparent;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
}

.ai-workbench__conversation-main:focus-visible {
  outline: 3px solid rgb(37 99 235 / 18%);
  outline-offset: 0;
}

.ai-workbench__conversation-title,
.ai-workbench__conversation-meta {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-workbench__conversation-title {
  color: #303b4f;
  font-size: 12.5px;
  font-weight: 550;
}

.ai-workbench__conversation-meta {
  color: #9aa4b5;
  font-size: 10px;
}

.ai-workbench__conversation.is-active .ai-workbench__conversation-title {
  color: #1d4ed8;
  font-weight: 650;
}

.ai-workbench__conversation-menu {
  flex: 0 0 auto;
  opacity: 0;
  transition: opacity 120ms ease;
}

.ai-workbench__conversation:hover .ai-workbench__conversation-menu,
.ai-workbench__conversation.is-active .ai-workbench__conversation-menu,
.ai-workbench__conversation:focus-within .ai-workbench__conversation-menu {
  opacity: 1;
}

.ai-workbench__conversation-menu :deep(.el-button) {
  width: 28px;
  height: 28px;
  min-height: 28px;
  margin: 0;
  padding: 0;
  border-radius: 7px;
  color: #8d98aa;
}

.ai-workbench__sidebar-empty {
  display: flex;
  min-height: 170px;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 6px;
  padding: 20px;
  color: #a3adbd;
  text-align: center;
}

.ai-workbench__sidebar-empty svg {
  margin-bottom: 4px;
  color: #c5cedb;
}

.ai-workbench__sidebar-empty strong {
  color: #5d6b80;
  font-size: 12px;
  font-weight: 600;
}

.ai-workbench__sidebar-empty span {
  font-size: 11px;
}

.ai-workbench__aside-footer {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 10px 16px 12px;
  border-top: 1px solid #e8edf3;
  color: #9aa4b5;
  font-size: 10px;
}

.ai-workbench__status-dot {
  width: 6px;
  height: 6px;
  flex: 0 0 6px;
  border-radius: 50%;
  background: #34a853;
  box-shadow: 0 0 0 3px rgb(52 168 83 / 10%);
}

/* Main conversation canvas */
.ai-workbench__main {
  background: #fff;
}

.ai-workbench__head {
  min-height: 64px;
  padding: 0 24px;
  border-bottom: 1px solid #edf0f4;
  background: rgb(255 255 255 / 94%);
}

.ai-workbench__head-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.ai-workbench__head-copy strong {
  color: #1b2537;
  font-size: 14px;
  font-weight: 650;
}

.ai-workbench__head-copy span {
  color: #98a2b3;
  font-size: 11px;
}

.ai-workbench__balance {
  display: flex;
  align-items: center;
  flex-direction: row;
  gap: 8px;
  padding: 6px 11px;
  border: 1px solid #e8edf4;
  border-radius: 999px;
  background: #f8fafc;
}

.ai-workbench__balance small {
  color: #8f9aac;
  font-size: 10px;
}

.ai-workbench__balance strong {
  color: #1d4ed8;
  font-family: var(--admin-font-data);
  font-size: 13px;
  font-weight: 700;
}

.ai-workbench__balance.is-negative strong {
  color: #dc2626;
}

.ai-workbench__alert {
  padding: 10px 20px 0;
}

.ai-workbench__main :deep(.el-alert) {
  margin: 0;
}

.ai-workbench__messages {
  min-height: 0;
  gap: 0;
  padding: 28px 28px 22px;
  overflow-y: auto;
}

.ai-workbench__loading {
  display: flex;
  min-height: 200px;
  align-items: center;
  justify-content: center;
  gap: 5px;
}

.ai-workbench__welcome {
  display: flex;
  width: min(560px, 100%);
  min-height: 100%;
  margin: auto;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  padding: 28px 12px 36px;
  text-align: center;
}

.ai-workbench__welcome-mark {
  display: grid;
  width: 72px;
  height: 72px;
  place-items: center;
  border: 1px solid #e2e9f5;
  border-radius: 22px;
  color: #2563eb;
  background:
    radial-gradient(circle at 30% 20%, #fff 0%, transparent 55%),
    linear-gradient(135deg, #eef4ff, #f5f0ff);
  box-shadow: 0 12px 30px rgb(37 99 235 / 10%);
}

.ai-workbench__welcome h1 {
  margin: 20px 0 8px;
  color: #172033;
  font-size: 24px;
  font-weight: 700;
  letter-spacing: -0.025em;
}

.ai-workbench__welcome p {
  max-width: 420px;
  margin: 0;
  color: #8a94a6;
  font-size: 13px;
  line-height: 1.7;
}

.ai-workbench__welcome-action {
  margin-top: 22px;
  padding: 0 18px;
  height: 38px;
  border-radius: 10px;
}

.ai-workbench__welcome-action :deep(span) {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.ai-workbench__starters {
  display: grid;
  width: 100%;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 9px;
  margin-top: 24px;
}

.ai-workbench__starter {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: flex-start;
  gap: 5px;
  padding: 12px 13px;
  border: 1px solid #e6ebf3;
  border-radius: 11px;
  color: #33415a;
  background: #fff;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  transition:
    border-color 150ms ease,
    box-shadow 150ms ease,
    transform 150ms ease;
}

.ai-workbench__starter:hover,
.ai-workbench__starter:focus-visible {
  border-color: #b9cdec;
  box-shadow: 0 6px 16px rgb(37 99 235 / 8%);
  transform: translateY(-1px);
}

.ai-workbench__starter span {
  overflow: hidden;
  font-size: 12px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-workbench__starter small {
  color: #9aa4b5;
  font-size: 10px;
}

.ai-workbench__more {
  padding: 0 0 18px;
}

.ai-workbench__message {
  display: flex;
  width: 100%;
  max-width: 100%;
  align-items: flex-start;
  gap: 10px;
  margin: 0 auto 24px;
  animation: message-in 180ms ease both;
}

.ai-workbench__message.is-user {
  justify-content: flex-end;
}

.ai-workbench__message-mark {
  display: grid;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  place-items: center;
  margin-top: 2px;
  border-radius: 9px;
  color: #315a9d;
  background: linear-gradient(135deg, #e8f0ff, #f2edff);
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.03em;
}

.ai-workbench__message-body {
  min-width: 0;
  max-width: min(760px, calc(100% - 40px));
  color: #344054;
  font-size: 14px;
  line-height: 1.75;
}

.ai-workbench__message.is-user .ai-workbench__message-body {
  max-width: min(72%, 620px);
  padding: 10px 14px;
  border: 1px solid #e9eef5;
  border-radius: 14px 14px 4px 14px;
  background: #f3f6fa;
  color: #2f3b4f;
}

.ai-workbench__plain {
  white-space: pre-wrap;
}

.ai-workbench__message-error {
  margin-top: 6px;
  color: #dc2626;
  font-size: 11px;
}

.ai-workbench__typing {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-height: 24px;
}

.ai-workbench__typing-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #9aa9c0;
  animation: typing 1.15s infinite ease-in-out;
}

.ai-workbench__typing-dot:nth-child(2) {
  animation-delay: 0.15s;
}

.ai-workbench__typing-dot:nth-child(3) {
  animation-delay: 0.3s;
}

.ai-workbench__to-bottom {
  right: 38px;
  bottom: 150px;
  padding: 6px 13px;
  border: 1px solid #e2e8f1;
  border-radius: 999px;
  color: #5c6b82;
  background: #fff;
  box-shadow: 0 5px 16px rgb(15 23 42 / 8%);
  font-size: 11px;
}

.ai-workbench__composer {
  padding: 10px 28px 18px;
  border-top: 1px solid #f0f3f7;
  background: linear-gradient(180deg, rgb(255 255 255 / 70%) 0%, #fff 30%);
}

.ai-workbench__composer-shell {
  width: min(820px, 100%);
  margin: 0 auto;
  padding: 9px 11px 8px;
  border: 1px solid #dfe6ef;
  border-radius: 16px;
  background: #fff;
  box-shadow: 0 8px 24px rgb(15 23 42 / 6%);
  transition:
    border-color 150ms ease,
    box-shadow 150ms ease;
}

.ai-workbench__composer-shell:focus-within {
  border-color: #9cbceb;
  box-shadow:
    0 0 0 3px rgb(37 99 235 / 8%),
    0 10px 26px rgb(15 23 42 / 7%);
}

.ai-workbench__composer-input :deep(.el-textarea__inner) {
  min-height: 40px !important;
  padding: 6px 4px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  box-shadow: none;
  color: #2f3b4f;
  font-family: inherit;
  font-size: 13.5px;
  line-height: 1.65;
}

.ai-workbench__composer-input :deep(.el-textarea__inner:focus) {
  box-shadow: none;
}

.ai-workbench__composer-input :deep(.el-textarea__inner::placeholder) {
  color: #a5aebe;
}

.ai-workbench__composer-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 2px 2px 0 6px;
}

.ai-workbench__composer-hint {
  color: #a0aaba;
  font-size: 10px;
}

.ai-workbench__composer-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  margin: 0;
}

.ai-workbench__send,
.ai-workbench__stop {
  width: 36px;
  height: 36px;
  min-height: 36px;
  margin: 0;
  padding: 0;
  border-radius: 11px;
}

.ai-workbench__send {
  box-shadow: 0 5px 12px rgb(37 99 235 / 22%);
}

.ai-workbench__stop {
  color: #475467;
  border-color: #e1e7f0;
  background: #f8fafc;
}

@keyframes message-in {
  from {
    opacity: 0;
    transform: translateY(4px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes typing {
  0%,
  80%,
  100% {
    opacity: 0.3;
    transform: translateY(0);
  }

  40% {
    opacity: 1;
    transform: translateY(-3px);
  }
}

@media (max-width: 1000px) {
  .ai-workbench__aside {
    width: 264px;
    flex-basis: 264px;
  }

  .ai-workbench__messages {
    padding: 24px 20px 18px;
  }

  .ai-workbench__composer {
    padding: 10px 20px 16px;
  }
}

@media (max-width: 760px) {
  .ai-workbench {
    height: 100%;
    max-height: 100%;
    min-height: 0;
    flex-direction: column;
  }

  .ai-workbench__aside {
    width: 100%;
    max-height: 42%;
    flex: 0 0 42%;
    border-right: 0;
    border-bottom: 1px solid #e8edf3;
  }

  .ai-workbench__agents {
    max-height: 104px;
  }

  .ai-workbench__conversations {
    min-height: 0;
  }

  .ai-workbench__aside-footer {
    display: none;
  }

  .ai-workbench__main {
    min-height: 0;
    flex: 1;
  }

  .ai-workbench__head {
    padding: 0 16px;
  }

  .ai-workbench__messages {
    padding: 20px 16px 16px;
  }

  .ai-workbench__composer {
    padding: 8px 16px 14px;
  }

  .ai-workbench__starters {
    grid-template-columns: 1fr;
  }

  .ai-workbench__welcome h1 {
    font-size: 21px;
  }

  .ai-workbench__message.is-user .ai-workbench__message-body {
    max-width: 88%;
  }

  .ai-workbench__to-bottom {
    right: 20px;
    bottom: 138px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .ai-workbench__conversation,
  .ai-workbench__agent-item,
  .ai-workbench__starter,
  .ai-workbench__composer-shell,
  .ai-workbench__message {
    animation: none;
    transition: none;
  }

  .ai-workbench__typing-dot {
    animation: none;
  }
}
</style>
