<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus'
import AppPage from '@/components/AppPage/index.vue'
import AppDialog from '@/components/AppDialog/index.vue'
import RunRecordTable from '@/views/ai/components/RunRecordTable/index.vue'
import FileSelectDialog from '@/components/FileSelectDialog/index.vue'
import type { StorageFile } from '@/api/storage/file'
import type { BackendOption } from '@/types/option'
import { usePermissionStore } from '@/store/permission'
import {
  createConversation,
  renameConversation,
  deleteConversation,
  listAgentOptions,
  listConversations,
  listMessages,
  type Conversation,
  type Message,
  type AgentOption,
} from '@/api/ai/chat'
import {
  cancelRun,
  createRun,
  getRun,
  listRuns,
  listRunEvents,
  listRunMedia,
  listRunRecords,
  getRunOptions,
  retryRunTransfer,
  previewRunMedia,
  type RunItem,
  type RunDetail,
  type RunEvent,
  type RunMedia,
  type RunOptions,
  type RunCharge,
  type RunChargeItem,
  type CreateRunInput,
} from '@/api/ai/run'
const { t, locale } = useI18n(),
  access = usePermissionStore()
const canList = computed(() => access.hasPermission('ai:chat:list')),
  canDetail = computed(() => access.hasPermission('ai:chat:detail')),
  canGenerate = computed(() => access.hasPermission('ai:chat:generate'))
const canCreate = computed(() => access.hasPermission('ai:chat:create')),
  canUpdate = computed(() => access.hasPermission('ai:chat:update')),
  canDelete = computed(() => access.hasPermission('ai:chat:delete'))
const canFiles = computed(
  () => access.hasPermission('storage:file:list') && access.hasPermission('storage:file:detail'),
)
const rows = ref<Conversation[]>([]),
  agents = ref<AgentOption[]>([]),
  messages = ref<Message[]>([]),
  selected = ref<Conversation | null>(null)
const agentId = ref(''),
  title = ref(''),
  text = ref(''),
  operation = ref(''),
  parameters = ref<Record<string, string | number>>({}),
  files = ref<StorageFile[]>([])
const fileVisible = ref(false),
  creating = ref(false),
  sending = ref(false),
  failed = ref(false),
  loading = ref(false),
  options = ref<RunOptions | null>(null)
const activeRun = ref<RunDetail | null>(null),
  history = ref<RunItem[]>([]),
  events = ref<RunEvent[]>([]),
  streamedText = ref(''),
  eventResetReason = ref(''),
  media = ref<RunMedia[]>([])
const conversationNext = ref(''),
  conversationMore = ref(false),
  historyNext = ref(''),
  historyMore = ref(false),
  agentNext = ref(''),
  agentMore = ref(false),
  messageMore = ref(false),
  messageSequence = ref(0),
  mediaNext = ref(''),
  mediaMore = ref(false)
const charges = ref<RunCharge[]>([]),
  chargeItems = ref<RunChargeItem[]>([]),
  chargeNext = ref(''),
  chargeMore = ref(false),
  chargeItemNext = ref(''),
  chargeItemMore = ref(false)
const previewVisible = ref(false),
  previewURL = ref(''),
  previewMime = ref('')
const currentAgent = computed(() => agents.value.find((a) => a.id === agentId.value))
const agentOptions = computed(() => agents.value.map((a) => ({ value: a.id, label: a.name })))
const operations = computed(() => {
  const allowed = currentAgent.value?.operations ?? []
  return (options.value?.operations ?? [])
    .filter((o) => allowed.includes(o.value))
    .map((o) => ({ ...o, disabled: o.value === 'video.generate' }))
})
const showVideoDisabledNotice = computed(() =>
  operations.value.some((item) => item.value === 'video.generate' && item.disabled),
)
function statusLabel(items: BackendOption<number>[] | undefined, value: number): string {
  return items?.find((item) => item.value === value)?.label ?? String(value)
}
const parameterOptions = computed(() =>
  (currentAgent.value?.parameterOptions ?? []).filter((item) => item.operation === operation.value),
)
const needsFile = computed(
  () => operation.value === 'image.edit' || operation.value === 'audio.transcribe',
)
const canSend = computed(
  () =>
    canGenerate.value &&
    !!currentAgent.value &&
    operation.value !== '' &&
    operations.value.some((item) => item.value === operation.value && !item.disabled) &&
    (!needsFile.value || files.value.length > 0) &&
    (operation.value === 'audio.transcribe' || text.value.trim() !== '') &&
    (operation.value !== 'text.generate' || selected.value?.agentId === agentId.value),
)
let timer: number | undefined,
  pollGeneration = 0,
  selectionGeneration = 0,
  eventSequence = 0,
  disposed = false,
  lastLoadedVersion = -1
let pendingRequest: { fingerprint: string; input: CreateRunInput } | null = null
function showText(content: unknown): string {
  if (typeof content === 'string') return content
  if (Array.isArray(content))
    return content
      .map((part) =>
        typeof part === 'object' && part !== null && 'text' in part
          ? String(part.text)
          : JSON.stringify(part),
      )
      .join('\n')
  return JSON.stringify(content, null, 2) ?? ''
}
async function loadConversations(append = false) {
  if (!canList.value) return
  loading.value = true
  try {
    const result = await listConversations({
      limit: 20,
      beforeId: append ? conversationNext.value : undefined,
    })
    rows.value = append ? [...rows.value, ...result.list] : result.list
    conversationNext.value = result.nextBeforeId
    conversationMore.value = result.hasMore
  } catch {
    failed.value = true
  } finally {
    loading.value = false
  }
}
async function loadAgents(append = false) {
  if (!canGenerate.value) return
  try {
    const result = await listAgentOptions({
      limit: 20,
      beforeId: append ? agentNext.value : undefined,
    })
    agents.value = append ? [...agents.value, ...result.list] : result.list
    agentNext.value = result.nextBeforeId
    agentMore.value = result.hasMore
  } catch {
    failed.value = true
  }
}
async function loadHistory(append = false) {
  if (!canList.value) return
  try {
    const result = await listRuns({ limit: 20, beforeId: append ? historyNext.value : undefined })
    history.value = append ? [...history.value, ...result.list] : result.list
    historyNext.value = result.nextBeforeId
    historyMore.value = result.hasMore
  } catch {
    failed.value = true
  }
}
async function loadMessages(append = true) {
  if (!selected.value || !canDetail.value) return
  const current = selectionGeneration,
    id = selected.value.id
  try {
    const result = await listMessages(id, {
      afterSequence: append ? messageSequence.value : 0,
      limit: 20,
    })
    if (current !== selectionGeneration) return
    messages.value = append
      ? [
          ...messages.value,
          ...result.list.filter((item) => !messages.value.some((old) => old.id === item.id)),
        ]
      : result.list
    messageSequence.value = result.nextSequence
    messageMore.value = result.hasMore
  } catch {
    failed.value = true
  }
}
async function selectConversation(row: Conversation) {
  if (!canDetail.value) return
  ++selectionGeneration
  stopPolling()
  selected.value = row
  agentId.value = row.agentId
  messages.value = []
  messageSequence.value = 0
  activeRun.value = null
  events.value = []
  streamedText.value = ''
  eventResetReason.value = ''
  media.value = []
  await loadMessages(false)
}
async function createNewConversation() {
  if (!canCreate.value || creating.value || !agentId.value || !title.value.trim()) return
  creating.value = true
  try {
    const row = await createConversation({ agentId: agentId.value, title: title.value.trim() })
    title.value = ''
    await loadConversations()
    if (canDetail.value) await selectConversation(row)
  } catch {
    failed.value = true
  } finally {
    creating.value = false
  }
}
async function rename(row: Conversation) {
  if (!canUpdate.value) return
  try {
    const result = await ElMessageBox.prompt(t('aiChat.titleField'), t('aiChat.rename'), {
      inputValue: row.title,
      inputValidator: (v) => !!v.trim() && v.length <= 200,
    })
    await renameConversation(row.id, result.value.trim())
    await loadConversations()
    if (selected.value?.id === row.id) selected.value = { ...row, title: result.value.trim() }
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') failed.value = true
  }
}
async function remove(row: Conversation) {
  if (!canDelete.value) return
  try {
    await ElMessageBox.confirm(t('aiChat.deleteConfirm'), t('aiChat.delete'))
    await deleteConversation(row.id)
    if (selected.value?.id === row.id) {
      stopPolling()
      selected.value = null
      messages.value = []
    }
    await loadConversations()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') failed.value = true
  }
}
function addFile(file: StorageFile) {
  if (!options.value || files.value.length >= options.value.constraints.maxFiles) return
  if (!files.value.some((item) => item.id === file.id)) files.value.push(file)
}
async function send() {
  if (!canSend.value || sending.value) return
  sending.value = true
  failed.value = false
  const input: CreateRunInput = {
    agentId: agentId.value,
    requestKey: '',
    operation: operation.value,
    input: {
      text: text.value.trim(),
      ...parameters.value,
      ...(files.value.length ? { fileIds: files.value.map((f) => String(f.id)) } : {}),
    },
    ...(operation.value === 'text.generate' && selected.value
      ? { conversationId: selected.value.id }
      : {}),
  }
  const fingerprint = JSON.stringify(input)
  if (pendingRequest?.fingerprint !== fingerprint)
    pendingRequest = { fingerprint, input: { ...input, requestKey: crypto.randomUUID() } }
  try {
    const run = await createRun(pendingRequest.input)
    pendingRequest = null
    text.value = ''
    files.value = []
    if (canDetail.value) await restore(run)
    await loadHistory()
  } catch {
    failed.value = true
  } finally {
    sending.value = false
  }
}
function stopPolling() {
  ++pollGeneration
  if (timer !== undefined) window.clearTimeout(timer)
  timer = undefined
}
async function loadMedia(append = false) {
  if (!activeRun.value || !canDetail.value) return
  const id = activeRun.value.id
  try {
    const result = await listRunMedia(id, {
      limit: 20,
      beforeId: append ? mediaNext.value : undefined,
    })
    if (id !== activeRun.value?.id) return
    media.value = append ? [...media.value, ...result.list] : result.list
    mediaNext.value = result.nextBeforeId
    mediaMore.value = result.hasMore
  } catch {
    failed.value = true
  }
}
async function loadCharges(items = false, append = false) {
  if (!activeRun.value || !canDetail.value) return
  const id = activeRun.value.id
  try {
    const query = {
      limit: 20,
      beforeId: append ? (items ? chargeItemNext.value : chargeNext.value) : undefined,
    }
    if (items) {
      const result = await listRunRecords(id, 'usage-charge-item', query, false)
      if (activeRun.value?.id !== id) return
      chargeItems.value = append ? [...chargeItems.value, ...result.list] : result.list
      chargeItemNext.value = result.nextBeforeId
      chargeItemMore.value = result.hasMore
    } else {
      const result = await listRunRecords(id, 'usage-charge', query, false)
      if (activeRun.value?.id !== id) return
      charges.value = append ? [...charges.value, ...result.list] : result.list
      chargeNext.value = result.nextBeforeId
      chargeMore.value = result.hasMore
    }
  } catch {
    failed.value = true
  }
}
async function restore(run: RunItem) {
  if (!canDetail.value) return
  stopPolling()
  failed.value = false
  events.value = []
  streamedText.value = ''
  eventResetReason.value = ''
  media.value = []
  charges.value = []
  chargeItems.value = []
  eventSequence = 0
  lastLoadedVersion = -1
  const generation = pollGeneration
  try {
    const value = await getRun(run.id)
    if (disposed || generation !== pollGeneration) return
    activeRun.value = value
    const url = new URL(window.location.href)
    url.searchParams.set('runId', run.id)
    window.history.replaceState(null, '', url)
    await poll(generation, run.id)
  } catch {
    if (generation === pollGeneration) failed.value = true
  }
}
async function poll(generation: number, id: string) {
  if (disposed || generation !== pollGeneration) return
  try {
    const [run, eventPage] = await Promise.all([
      getRun(id),
      listRunEvents(id, { afterSequence: eventSequence, limit: 100 }),
    ])
    if (disposed || generation !== pollGeneration) return
    const changed = lastLoadedVersion !== run.stateVersion
    activeRun.value = run
    const eventsExpired = eventPage.resetReason === 'EVENTS_EXPIRED'
    if (eventPage.resetReason) eventResetReason.value = eventPage.resetReason
    if (eventsExpired) {
      events.value = []
      streamedText.value = ''
    }
    const additions = eventPage.list.filter((event) => event.sequence > eventSequence)
    events.value.push(...additions)
    for (const event of additions) {
      if (
        event.eventType === 'textDelta' &&
        typeof event.payload === 'object' &&
        event.payload !== null &&
        'text' in event.payload &&
        typeof event.payload.text === 'string'
      )
        streamedText.value += event.payload.text
    }
    eventSequence = Math.max(eventSequence, eventPage.nextSequence)
    if (changed) {
      await Promise.all([loadMedia(), loadCharges(), loadCharges(true)])
      if (generation !== pollGeneration || disposed) return
      lastLoadedVersion = run.stateVersion
    }
    if (
      selected.value &&
      run.conversationId === selected.value.id &&
      (eventsExpired || changed || eventPage.list.length > 0)
    )
      await loadMessages(!eventsExpired)
    if (disposed || generation !== pollGeneration) return
    if (eventPage.hasMore) {
      timer = window.setTimeout(() => void poll(generation, id), 0)
    } else if (!run.finishedAt || run.transferStatus === 1 || run.transferStatus === 2) {
      timer = window.setTimeout(() => void poll(generation, id), 1500)
    }
  } catch {
    if (generation === pollGeneration) failed.value = true
  }
}
async function cancel() {
  if (!activeRun.value?.canCancel || !access.hasPermission('ai:chat:cancel')) return
  try {
    await cancelRun(activeRun.value.id)
  } catch {
    failed.value = true
  }
}
async function retryTransfer() {
  if (!activeRun.value || !access.hasPermission('ai:chat:retryTransfer')) return
  try {
    await retryRunTransfer(activeRun.value.id)
    await restore(activeRun.value)
  } catch {
    failed.value = true
  }
}
async function showMedia(item: RunMedia, download = false) {
  if (!activeRun.value || !canDetail.value) return
  try {
    const value = await previewRunMedia(
      activeRun.value.id,
      item.id,
      download ? 'attachment' : 'inline',
    )
    if (download) {
      window.open(value.url, '_blank', 'noopener,noreferrer')
      return
    }
    previewURL.value = value.url
    previewMime.value = item.mimeType ?? ''
    previewVisible.value = true
  } catch {
    failed.value = true
  }
}
async function refresh() {
  failed.value = false
  await Promise.all([loadConversations(), loadHistory(), loadAgents(), loadOptions()])
}
async function loadOptions() {
  try {
    options.value = await getRunOptions()
  } catch {
    failed.value = true
  }
}
watch(agentId, () => {
  parameters.value = {}
  if (!operations.value.some((o) => o.value === operation.value)) operation.value = ''
})
watch(operation, () => {
  parameters.value = {}
  files.value = []
  pendingRequest = null
})
watch(locale, () => {
  void loadOptions()
  void loadAgents()
})
onMounted(() => {
  void loadOptions()
  void loadConversations()
  void loadAgents()
  void loadHistory()
  const runId = new URL(window.location.href).searchParams.get('runId')
  if (runId && canDetail.value)
    void getRun(runId)
      .then((run) => restore(run))
      .catch(() => {
        failed.value = true
      })
})
onBeforeUnmount(() => {
  disposed = true
  ++selectionGeneration
  stopPolling()
})
</script>
<template>
  <AppPage
    ><div class="ai-chat">
      <aside class="ai-chat__sidebar">
        <h3>{{ t('aiChat.conversations') }}</h3>
        <el-button v-if="canList || canGenerate" @click="refresh">{{
          t('aiRun.refresh')
        }}</el-button>
        <el-select-v2
          v-model="agentId"
          :disabled="!canGenerate"
          :options="agentOptions"
          :placeholder="t('aiChat.agentId')"
          filterable
          clearable
        /><el-button v-if="agentMore && canGenerate" @click="loadAgents(true)">{{
          t('aiChat.loadMore')
        }}</el-button
        ><el-input v-model="title" :placeholder="t('aiChat.titleField')" /><el-button
          v-if="canCreate"
          data-testid="ai-chat-create"
          :loading="creating"
          @click="createNewConversation"
          >{{ t('aiChat.new') }}</el-button
        >
        <article v-for="row in rows" :key="row.id">
          <el-button :disabled="!canDetail" link @click="selectConversation(row)">{{
            row.title
          }}</el-button
          ><el-button v-if="canUpdate" link @click="rename(row)">{{ t('aiChat.rename') }}</el-button
          ><el-button v-if="canDelete" link type="danger" @click="remove(row)">{{
            t('aiChat.delete')
          }}</el-button>
        </article>
        <el-button v-if="conversationMore" :loading="loading" @click="loadConversations(true)">{{
          t('aiChat.loadMore')
        }}</el-button>
        <h3>{{ t('aiChat.history') }}</h3>
        <article v-for="run in history" :key="run.id">
          <el-button :disabled="!canDetail" link @click="restore(run)"
            >{{ run.id }} · {{ run.operation }} ·
            {{ statusLabel(options?.executionStatuses, run.executionStatus) }}</el-button
          >
        </article>
        <el-button v-if="historyMore" @click="loadHistory(true)">{{
          t('aiChat.loadMore')
        }}</el-button>
      </aside>
      <section class="ai-chat__main">
        <el-alert
          v-if="failed"
          type="error"
          :closable="false"
          :title="t('aiChat.loadFailed')"
        /><el-alert v-if="!canGenerate" :closable="false" :title="t('aiChat.noPermission')" />
        <h3>{{ selected?.title ?? t('aiChat.title') }}</h3>
        <div class="ai-chat__messages">
          <article v-for="message in messages" :key="message.id">
            <strong>{{
              message.role === 1
                ? t('aiChat.user')
                : message.role === 2
                  ? t('aiChat.assistant')
                  : message.role
            }}</strong>
            <pre>{{ showText(message.content) }}</pre>
          </article>
          <el-button v-if="messageMore" @click="loadMessages()">{{
            t('aiChat.loadMore')
          }}</el-button>
        </div>
        <el-select-v2
          v-model="operation"
          :options="operations"
          :placeholder="t('aiRun.operation')"
        />
        <p v-if="showVideoDisabledNotice">{{ t('aiChat.videoDisabled') }}</p>
        <div class="ai-chat__parameters">
          <label v-for="param in parameterOptions" :key="param.key"
            >{{ param.label
            }}<el-select-v2
              v-model="parameters[param.key]"
              :options="param.options"
              clearable
              :placeholder="param.label"
          /></label>
        </div>
        <el-input v-model="text" type="textarea" :rows="4" :placeholder="t('aiChat.message')" />
        <div class="ai-chat__actions">
          <el-button
            :disabled="!canFiles || !options || files.length >= options.constraints.maxFiles"
            @click="fileVisible = true"
            >{{ t('aiChat.attach') }}</el-button
          ><span v-if="!canFiles">{{ t('aiChat.filePermission') }}</span
          ><el-tag
            v-for="file in files"
            :key="file.id"
            closable
            @close="files = files.filter((f) => f.id !== file.id)"
            >{{ file.fileName }}</el-tag
          ><el-button
            v-if="canGenerate"
            data-testid="ai-chat-send"
            type="primary"
            :disabled="!canSend"
            :loading="sending"
            @click="send"
            >{{ t('aiChat.send') }}</el-button
          >
        </div>
        <p v-if="operation === 'text.generate' && !selected">
          {{ t('aiChat.selectConversation') }}
        </p>
        <section v-if="activeRun">
          <h4>
            {{ activeRun.id }} ·
            {{ statusLabel(options?.executionStatuses, activeRun.executionStatus) }}
          </h4>
          <p>
            {{ t('aiRun.estimatedUnits') }}: {{ activeRun.estimatedUnits }} ·
            {{ t('aiRun.usageStatus') }}:
            {{ statusLabel(options?.usageStatuses, activeRun.usageStatus) }} ·
            {{ t('aiRun.billingStatus') }}:
            {{ statusLabel(options?.billingStatuses, activeRun.billingStatus) }} ·
            {{ t('aiRun.transferStatus') }}:
            {{ statusLabel(options?.transferStatuses, activeRun.transferStatus) }}
          </p>
          <p>{{ activeRun.errorCode }} {{ activeRun.errorMessage }}</p>
          <el-alert
            v-if="eventResetReason"
            :title="
              eventResetReason === 'EVENTS_EXPIRED' ? t('aiChat.eventsExpired') : eventResetReason
            "
            type="info"
            :closable="false"
          />
          <pre data-testid="ai-chat-result">{{
            activeRun.finishedAt && activeRun.result !== null
              ? showText(activeRun.result)
              : streamedText || showText(activeRun.result)
          }}</pre>
          <el-button
            v-if="access.hasPermission('ai:chat:cancel') && activeRun.canCancel"
            @click="cancel"
            >{{ t('aiChat.cancelRun') }}</el-button
          ><el-button
            v-if="
              access.hasPermission('ai:chat:retryTransfer') &&
              media.some((item) => item.canRetryTransfer)
            "
            @click="retryTransfer"
            >{{ t('aiChat.retryTransfer') }}</el-button
          ><el-button @click="restore(activeRun)">{{ t('aiRun.refresh') }}</el-button>
          <h4>{{ t('aiRun.charges') }}</h4>
          <RunRecordTable
            kind="usage-charge"
            :charges="charges"
            :options="options"
            @refresh="loadCharges()"
          />
          <el-button v-if="chargeMore" @click="loadCharges(false, true)">{{
            t('aiChat.loadMore')
          }}</el-button>
          <h4>{{ t('aiRun.chargeItems') }}</h4>
          <RunRecordTable
            kind="usage-charge-item"
            :charge-items="chargeItems"
            :options="options"
            @refresh="loadCharges(true)"
          />
          <el-button v-if="chargeItemMore" @click="loadCharges(true, true)">{{
            t('aiChat.loadMore')
          }}</el-button>
          <ol>
            <li v-for="event in events" :key="event.id">
              {{ event.sequence }} · {{ event.eventType }}
              <pre>{{ showText(event.payload) }}</pre>
            </li>
          </ol>
          <div class="ai-chat__media">
            <article v-for="item in media" :key="item.id">
              <p>
                {{ item.mimeType }} ·
                {{
                  options?.mediaTransferStatuses.find(
                    (option) => option.value === item.transferStatus,
                  )?.label ?? item.transferStatus
                }}
                {{ item.errorMessage }}
              </p>
              <el-button :disabled="!item.canPreview" @click="showMedia(item)">{{
                t('aiChat.preview')
              }}</el-button
              ><el-button :disabled="!item.canPreview" @click="showMedia(item, true)">{{
                t('aiChat.download')
              }}</el-button>
            </article>
          </div>
          <el-button v-if="mediaMore" @click="loadMedia(true)">{{
            t('aiChat.loadMore')
          }}</el-button>
        </section>
      </section>
      <FileSelectDialog
        v-if="canFiles"
        v-model="fileVisible"
        purpose="ai/input"
        @select="addFile"
      /><AppDialog v-model="previewVisible" :title="t('aiChat.preview')"
        ><img
          v-if="previewMime.startsWith('image/')"
          :src="previewURL"
          class="ai-chat__preview"
        /><audio v-else-if="previewMime.startsWith('audio/')" controls :src="previewURL" /><video
          v-else-if="previewMime.startsWith('video/')"
          controls
          :src="previewURL"
          class="ai-chat__preview"
        /><a v-else :href="previewURL" target="_blank" rel="noopener noreferrer">{{
          t('aiChat.download')
        }}</a></AppDialog
      >
    </div></AppPage
  >
</template>
<style scoped lang="scss">
.ai-chat {
  display: grid;
  grid-template-columns: minmax(240px, 28%) minmax(0, 1fr);
  gap: 16px;
}
.ai-chat__sidebar,
.ai-chat__main {
  padding: 16px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
}
.ai-chat__sidebar {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.ai-chat__messages {
  max-height: 400px;
  overflow: auto;
}
.ai-chat__actions,
.ai-chat__parameters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 12px 0;
}
.ai-chat__media {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
}
.ai-chat__preview {
  max-width: 100%;
}
pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
article {
  padding: 8px;
  background: var(--el-fill-color-light);
  margin-bottom: 8px;
}
@media (max-width: 900px) {
  .ai-chat {
    grid-template-columns: 1fr;
  }
}
</style>
