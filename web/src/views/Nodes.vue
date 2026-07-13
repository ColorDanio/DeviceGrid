<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title-group">
        <h2>{{ t('nav.nodes') }}</h2>
        <p class="page-subtitle">{{ t('feature.nodesSubtitle') }}</p>
      </div>
      <div class="header-actions">
        <button class="btn-outline" @click="downloadTemplate">
          <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6M8 13h8M8 17h8" stroke="currentColor" stroke-width="2"/></svg>
          {{ t('feature.nodesCsvTemplate') }}
        </button>
        <label class="btn-outline">
          <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M7 10l5-5 5 5M12 5v12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
          {{ t('feature.nodesImportCsv') }}
          <input type="file" accept=".csv" @change="handleImport" style="display:none" />
        </label>
        <button class="btn-outline" @click="exportCSV">
          <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M7 10l5 5 5-5M12 15V3" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
          {{ t('feature.nodesExport') }}
        </button>
        <button class="btn-primary" @click="showDialog()">
          <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
          {{ t('common.addNode') }}
        </button>
      </div>
    </div>

    <div class="dg-card" style="padding: 20px">
      <div class="table-toolbar">
        <div class="search-box">
          <svg viewBox="0 0 24 24" fill="none" width="16" height="16"><circle cx="11" cy="11" r="8" stroke="currentColor" stroke-width="2"/><path d="M21 21l-4.35-4.35" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
          <input v-model="filter.search" :placeholder="t('feature.nodesSearch')" @input="loadNodes" />
        </div>
        <select v-model="filter.status" class="filter-select" @change="loadNodes">
          <option value="">{{ t('feature.nodesAllStatuses') }}</option>
          <option value="online">{{ t('common.online') }}</option>
          <option value="offline">{{ t('common.offline') }}</option>
          <option value="untrusted">{{ t('common.untrusted') }}</option>
          <option value="error">{{ t('common.error') }}</option>
        </select>
      </div>

      <!-- Batch Toolbar -->
      <transition name="fade-slide">
        <div v-if="selectedNodes.length > 0" class="batch-bar">
          <span class="batch-count">{{ t('feature.nodesSelected', { count: selectedNodes.length }) }}</span>
          <div class="batch-actions">
            <button class="batch-btn" @click="batchTrust">
              <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71" stroke="currentColor" stroke-width="2" stroke-linecap="round"/><path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
              {{ t('feature.nodesBatchTrust') }}
            </button>
            <button class="batch-btn" @click="batchHealth">
              <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M22 12h-4l-3 9L9 3l-3 9H2" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
              {{ t('feature.nodesBatchHealth') }}
            </button>
            <button class="batch-btn batch-danger" @click="batchDelete">
              <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M3 6h18M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2m3 0v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
              {{ t('feature.nodesBatchDelete') }}
            </button>
          </div>
          <button class="batch-clear" @click="clearSelection">{{ t('feature.nodesClearSelection') }}</button>
        </div>
      </transition>

      <el-table ref="tableRef" v-loading="loading" :data="nodes" stripe @row-click="onRowClick" @selection-change="onSelectionChange">
        <el-table-column type="selection" width="42" />
        <el-table-column :label="t('feature.nodesStatus')" width="100">
          <template #default="{ row }">
            <div class="status-badge" :class="(row as any).status">
              <span class="dot"></span>
              {{ statusLabel((row as any).status) }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="name" :label="t('feature.nodesName')" min-width="130">
          <template #default="{ row }">
            <span style="font-weight:600">{{ (row as any).name }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('feature.nodesAddress')" min-width="160">
          <template #default="{ row }">
            <span class="mono">{{ (row as any).host }}:{{ (row as any).port }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('feature.nodesRegion')" width="120">
          <template #default="{ row }">
            <span v-if="(row as any).country_code" class="geo-cell">
              <span class="flag">{{ countryCodeToFlag((row as any).country_code) }}</span>
              {{ (row as any).country || (row as any).country_code }}
            </span>
            <span v-else class="text-muted">—</span>
          </template>
        </el-table-column>
        <el-table-column prop="username" :label="t('feature.nodesUser')" width="80" />
        <el-table-column :label="t('feature.nodesHardware')" min-width="200">
          <template #default="{ row }">
            <div class="hw-cell" v-if="hardwareInfo[(row as any).id]">
              <span class="hw-item" title="CPU">{{ hardwareInfo[(row as any).id] }}</span>
            </div>
            <span v-else class="text-muted">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('feature.nodesTransport')" width="80">
          <template #default="{ row }">
            <span class="chip" :class="(row as any).transport_mode">{{ (row as any).transport_mode === 'agent' ? 'Agent' : 'SSH' }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('feature.nodesTags')" min-width="140">
          <template #default="{ row }">
            <span class="chip chip-tag" v-for="tag in (row as any).tags" :key="tag">{{ tag }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Docker" width="90">
          <template #default="{ row }">
            <span class="chip chip-docker" v-if="(row as any).docker_version">{{ (row as any).docker_version }}</span>
            <span v-else class="text-muted">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('feature.nodesLastSeen')" width="130">
          <template #default="{ row }">
            <span class="text-muted">{{ formatTime((row as any).last_seen_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('feature.nodesActions')" width="180" fixed="right">
          <template #default="{ row }">
            <div class="action-cell">
            <button class="action-btn" @click.stop="handleTrust((row as any))">
              <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71" stroke="currentColor" stroke-width="2" stroke-linecap="round"/><path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
              {{ t('feature.nodesTrust') }}
            </button>
            <button class="action-btn action-agent" @click.stop="handleDeployAgent((row as any))" v-if="(row as any).auth_mode === 'key' || (row as any).status === 'online'">
              <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M12 2a10 10 0 100 20 10 10 0 000-20zM12 6v6l4 2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
              Agent
            </button>
            <button class="action-btn action-edit" @click.stop="showEditDialog((row as any))">
              <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
              {{ t('feature.nodesEdit') }}
            </button>
            <button class="action-btn action-danger" @click.stop="handleDelete((row as any))">
              <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M3 6h18M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2m3 0v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6h14z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
              {{ t('feature.nodesDelete') }}
            </button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- Dialog -->
    <el-dialog v-model="dialogVisible" :title="editing ? t('feature.nodesEditNode') : t('common.addNode')" width="560px" align-center :before-close="closeNodeDialog">
      <el-steps v-if="!editing" :active="onboardingStep" finish-status="success" simple class="onboarding-steps">
        <el-step :title="t('feature.nodesConnect')" />
        <el-step :title="t('feature.nodesVerify')" />
        <el-step :title="t('feature.nodesTrust')" />
        <el-step :title="t('feature.nodesSave')" />
      </el-steps>

      <el-form v-if="editing || onboardingStep === 0" ref="formRef" :model="form" :rules="rules" label-width="80px">
        <el-form-item :label="t('feature.nodesName')" prop="name">
          <el-input v-model="form.name" :placeholder="t('feature.nodesNameExample')" />
        </el-form-item>
        <el-form-item :label="t('feature.nodesHost')" prop="host">
          <el-input v-model="form.host" :placeholder="t('feature.nodesHostPlaceholder')" />
        </el-form-item>
        <div style="display:flex;gap:12px">
          <el-form-item :label="t('feature.nodesPort')" prop="port" style="flex:1">
            <el-input-number v-model="form.port" :min="1" :max="65535" style="width:100%" />
          </el-form-item>
          <el-form-item :label="t('feature.nodesUsername')" prop="username" style="flex:1">
            <el-input v-model="form.username" />
          </el-form-item>
        </div>
        <el-form-item v-if="editing" :label="t('feature.nodesNewPassword')">
          <el-input v-model="form.password" type="password" show-password :placeholder="t('feature.nodesPasswordOptional')" />
        </el-form-item>
        <el-form-item v-if="!editing" :label="t('feature.nodesPassword')" prop="password">
          <el-input v-model="form.password" type="password" show-password :placeholder="t('feature.nodesSshPasswordOptional')" />
        </el-form-item>
        <el-form-item :label="t('feature.nodesPrivateKey')">
          <el-input v-model="form.privateKey" type="textarea" :rows="3" :placeholder="t('feature.nodesPrivateKeyPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('feature.nodesTags')">
          <el-select v-model="form.tags" multiple filterable allow-create default-first-option :placeholder="t('feature.nodesTagsPlaceholder')" style="width:100%">
            <el-option v-for="t in ['web','db','cache','mq','app']" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>
      </el-form>
      <div v-else class="onboarding-panel">
        <template v-if="onboardingStep === 1">
          <div class="onboarding-icon pending">2</div>
          <h3>{{ t('feature.nodesVerifyConnection') }}</h3>
          <p>{{ t('feature.nodesVerifyDescription', { host: draftNode?.host, port: draftNode?.port }) }}</p>
        </template>
        <template v-else-if="onboardingStep === 2">
          <div class="onboarding-icon pending">3</div>
          <h3>{{ t('feature.nodesEstablishTrust') }}</h3>
          <p>{{ t('feature.nodesTrustDescription') }}</p>
          <p v-if="!form.password" class="onboarding-warning">{{ t('feature.nodesTrustPasswordRequired') }}</p>
        </template>
        <template v-else>
          <div class="onboarding-icon success">✓</div>
          <h3>{{ t('feature.nodesReady') }}</h3>
          <p>{{ t('feature.nodesReadyDescription', { name: draftNode?.name }) }}</p>
        </template>
        <p v-if="onboardingError" class="onboarding-error">{{ onboardingError }}</p>
      </div>
      <template #footer>
        <el-button @click="cancelNodeDialog">{{ t('common.cancel') }}</el-button>
        <template v-if="editing">
          <el-button type="primary" :loading="saving" @click="handleSave">{{ t('common.confirm') }}</el-button>
        </template>
        <template v-else-if="onboardingStep === 0">
          <el-button type="primary" :loading="onboardingBusy" @click="connectNode">{{ t('feature.nodesConnectContinue') }}</el-button>
        </template>
        <template v-else-if="onboardingStep === 1">
          <el-button :disabled="onboardingBusy" @click="onboardingStep = 0">{{ t('feature.nodesBack') }}</el-button>
          <el-button type="primary" :loading="onboardingBusy" @click="verifyNode">{{ t('feature.nodesVerifyConnection') }}</el-button>
        </template>
        <template v-else-if="onboardingStep === 2">
          <el-button :disabled="onboardingBusy" @click="onboardingStep = 1">{{ t('feature.nodesBack') }}</el-button>
          <el-button type="primary" :loading="onboardingBusy" :disabled="!form.password" @click="trustNode">{{ t('feature.nodesEstablishTrust') }}</el-button>
        </template>
        <el-button v-else type="primary" @click="finishOnboarding">{{ t('feature.nodesSaveNode') }}</el-button>
      </template>
    </el-dialog>

    <!-- Batch Progress -->
    <div v-if="batchProgress.active" class="batch-progress">
      <div class="bp-label">{{ t('feature.nodesInProgress', { label: batchProgress.label }) }}</div>
      <div class="bp-count">{{ batchProgress.done }} / {{ batchProgress.total }}</div>
      <div class="bp-track"><div class="bp-fill" :style="{ width: (batchProgress.total > 0 ? batchProgress.done / batchProgress.total * 100 : 0) + '%' }"></div></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { listNodes, createNode, updateNode, deleteNode, establishTrust, deployAgent, checkHealth, type Node, type NodeFilter } from '@/api/nodes'
import { useFleetStore } from '@/stores/fleet'
import client from '@/api/client'

const router = useRouter()
const { t } = useI18n()

const loading = ref(false)
const fleet = useFleetStore()
const { nodes: fleetNodes, metrics: fleetMetrics } = storeToRefs(fleet)
const saving = ref(false)
const nodes = ref<Node[]>([])
const dialogVisible = ref(false)
const editing = ref(false)
const editingId = ref('')
const onboardingStep = ref(0)
const onboardingBusy = ref(false)
const onboardingError = ref('')
const draftNode = ref<Node | null>(null)
const formRef = ref<FormInstance>()
const tableRef = ref()
const selectedRows = ref<Node[]>([])
const selectedNodes = ref<string[]>([])
const batchProgress = ref({ active: false, total: 0, done: 0, label: '' })
const filter = reactive<NodeFilter>({ search: '', status: '' })
const hardwareInfo = ref<Record<string, string>>({})

const form = reactive({ name: '', host: '', port: 22, username: 'root', password: '', privateKey: '', tags: [] as string[] })
const rules: FormRules = {
  name: [{ required: true, message: t('feature.nodesEnterName'), trigger: 'blur' }],
  host: [{ required: true, message: t('feature.nodesEnterHost'), trigger: 'blur' }],
}

function statusLabel(s: string) {
  return { online: t('common.online'), offline: t('common.offline'), untrusted: t('common.untrusted'), error: t('common.error') }[s] || s
}
function countryCodeToFlag(cc: string): string {
  if (!cc || cc.length !== 2) return ''
  const A = 0x1F1E6
  return String.fromCodePoint(A + cc.charCodeAt(0) - 65) + String.fromCodePoint(A + cc.charCodeAt(1) - 65)
}
function formatTime(t: string) {
  if (!t || t.startsWith('0001-')) return '—'
  return new Date(t).toLocaleString()
}

async function loadNodes() {
  loading.value = true
  try { nodes.value = await listNodes(filter) } finally { loading.value = false }
  loadHardware()
}

async function loadHardware() {
  const online = nodes.value.filter(n => n.status === 'online')
  for (const n of online) {
    const m = fleetMetrics.value[n.id]
    if (!m) continue
    const cpu = m.cpu_sockets > 0 ? `${m.cpu_sockets}P×${m.cpu_cores}C/${m.cpu_threads}T` : `${m.cpu_cores || '?'}C`
    const mem = m.mem_total > 0 ? fmtBytes(m.mem_total) : '?'
    const disk = m.disk_total > 0 ? fmtBytes(m.disk_total) : '?'
    hardwareInfo.value[n.id] = `CPU: ${cpu} | Mem: ${mem} | Disk: ${disk}`
  }
}

function fmtBytes(b: number): string {
  if (!b) return '?'
  if (b < 1073741824) return (b / 1048576).toFixed(0) + 'MB'
  if (b < 1099511627776) return (b / 1073741824).toFixed(0) + 'G'
  return (b / 1099511627776).toFixed(1) + 'T'
}

function showDialog() {
  editing.value = false
  editingId.value = ''
  onboardingStep.value = 0
  onboardingBusy.value = false
  onboardingError.value = ''
  draftNode.value = null
  Object.assign(form, { name: '', host: '', port: 22, username: 'root', password: '', privateKey: '', tags: [] })
  dialogVisible.value = true
}

function showEditDialog(node: Node) {
  editing.value = true
  editingId.value = node.id
  Object.assign(form, {
    name: node.name,
    host: node.host,
    port: node.port,
    username: node.username,
    password: '',
    privateKey: '',
    tags: [...(node.tags || [])],
  })
  dialogVisible.value = true
}

function requestError(error: unknown, fallback: string): string {
  const err = error as { response?: { data?: { message?: string } }, message?: string }
  return err?.response?.data?.message || err?.message || fallback
}

async function connectNode() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  onboardingBusy.value = true
  onboardingError.value = ''
  try {
    draftNode.value = await createNode({ ...form, private_key: form.privateKey })
    onboardingStep.value = 1
  } catch (error) {
    onboardingError.value = requestError(error, t('feature.nodesSaveConnectionFailed'))
  } finally {
    onboardingBusy.value = false
  }
}

async function verifyNode() {
  if (!draftNode.value) return
  onboardingBusy.value = true
  onboardingError.value = ''
  try {
    const result = await checkHealth(draftNode.value.id)
    if (result?.status !== 'online') throw new Error(result?.error || t('feature.nodesServerUnresponsive'))
    onboardingStep.value = 2
  } catch (error) {
    onboardingError.value = requestError(error, t('feature.nodesConnectFailed'))
  } finally {
    onboardingBusy.value = false
  }
}

async function trustNode() {
  if (!draftNode.value) return
  onboardingBusy.value = true
  onboardingError.value = ''
  try {
    await establishTrust(draftNode.value.id)
    onboardingStep.value = 3
  } catch (error) {
    onboardingError.value = requestError(error, t('feature.nodesTrustFailed'))
  } finally {
    onboardingBusy.value = false
  }
}

async function closeNodeDialog(done?: () => void) {
  if (!editing.value && draftNode.value) {
    try {
      await deleteNode(draftNode.value.id)
    } catch {
      ElMessage.warning(t('feature.nodesCleanupFailed'))
    }
  }
  draftNode.value = null
  onboardingError.value = ''
  dialogVisible.value = false
  done?.()
  loadNodes()
}

function cancelNodeDialog() {
  void closeNodeDialog()
}

function finishOnboarding() {
  draftNode.value = null
  onboardingError.value = ''
  dialogVisible.value = false
  ElMessage.success(t('feature.nodesAddedTrusted'))
  loadNodes()
}

async function handleSave() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    saving.value = true
    try {
      if (editing.value && editingId.value) {
        const updateData: Record<string, any> = {
          name: form.name,
          host: form.host,
          port: form.port,
          username: form.username,
          tags: form.tags,
        }
        if (form.password) {
          updateData.password = form.password
        }
        if (form.privateKey) {
          updateData.private_key = form.privateKey
        }
        await updateNode(editingId.value, updateData)
        ElMessage.success(t('feature.nodesUpdated'))
      } else {
        await createNode({ ...form, private_key: form.privateKey })
        ElMessage.success(t('feature.nodesAdded'))
      }
      dialogVisible.value = false
      loadNodes()
    } finally { saving.value = false }
  })
}

async function handleDelete(node: Node) {
  await ElMessageBox.confirm(t('feature.nodesDeleteConfirm', { name: node.name }), '', { type: 'warning' })
  await deleteNode(node.id)
  ElMessage.success(t('feature.nodesDeleted'))
  loadNodes()
}

async function handleTrust(node: Node) {
  ElMessage.info(t('feature.nodesTrusting', { name: node.name }))
  try {
    await establishTrust(node.id)
    ElMessage.success(t('feature.nodesTrustSucceeded', { name: node.name }))
    loadNodes()
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || t('feature.nodesUnknownError')
    ElMessageBox.alert(msg, t('feature.nodesTrustFailedTitle', { name: node.name }), { confirmButtonText: t('feature.nodesAcknowledged'), type: 'error' })
  }
}

async function handleDeployAgent(node: Node) {
  ElMessage.info(t('feature.nodesDeployingAgent', { name: node.name }))
  try {
    const result = await deployAgent(node.id)
    if (result.status === 'deployed') {
      ElMessage.success(t('feature.nodesAgentDeployed', { arch: result.arch, active: result.active }))
    } else {
      ElMessageBox.alert(
        t('feature.nodesAgentDegraded', { active: result.active, output: result.output || t('feature.nodesNone') }),
        t('feature.nodesAgentDeployment', { name: node.name }),
        { type: 'warning' }
      )
    }
    loadNodes()
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || t('feature.nodesUnknownError')
    ElMessageBox.alert(msg, t('feature.nodesAgentDeploymentFailed', { name: node.name }), { confirmButtonText: t('feature.nodesAcknowledged'), type: 'error' })
  }
}

function onSelectionChange(rows: Node[]) {
  selectedRows.value = rows
  selectedNodes.value = rows.map(r => r.id)
}

function onRowClick(row: Node, column: any) {
  if (column && column.type === 'selection') return
  router.push(`/nodes/${row.id}`)
}

function clearSelection() {
  tableRef.value?.clearSelection()
  selectedRows.value = []
  selectedNodes.value = []
}

async function batchTrust() {
  const targets = [...selectedRows.value]
  if (targets.length === 0) return
  batchProgress.value = { active: true, total: targets.length, done: 0, label: t('feature.nodesBatchTrust') }
  let ok = 0, fail = 0
  const errors: string[] = []
  for (const node of targets) {
    try {
      await establishTrust(node.id)
      ok++
    } catch (err: any) {
      fail++
      errors.push(`${node.name}: ${err?.response?.data?.message || t('feature.nodesFailed')}`)
    }
    batchProgress.value.done++
  }
  batchProgress.value.active = false
  if (fail === 0) {
    ElMessage.success(t('feature.nodesBatchTrustSucceeded', { count: ok }))
  } else {
    ElMessageBox.alert(t('feature.nodesBatchResult', { success: ok, failed: fail, errors: errors.join('\n') }), t('feature.nodesBatchTrustResult'), { type: 'warning' })
  }
  clearSelection()
  loadNodes()
}

async function batchHealth() {
  const targets = [...selectedRows.value]
  if (targets.length === 0) return
  batchProgress.value = { active: true, total: targets.length, done: 0, label: t('feature.nodesHealthCheck') }
  let ok = 0, fail = 0
  for (const node of targets) {
    try {
      await checkHealth(node.id)
      ok++
    } catch {
      fail++
    }
    batchProgress.value.done++
  }
  batchProgress.value.active = false
  ElMessage.success(t('feature.nodesHealthComplete', { online: ok, offline: fail }))
  clearSelection()
  loadNodes()
}

async function batchDelete() {
  const targets = [...selectedRows.value]
  if (targets.length === 0) return
  await ElMessageBox.confirm(t('feature.nodesBatchDeleteConfirm', { count: targets.length }), t('feature.nodesBatchDelete'), { type: 'warning', confirmButtonText: t('feature.nodesDelete'), cancelButtonText: t('common.cancel') })
  batchProgress.value = { active: true, total: targets.length, done: 0, label: t('feature.nodesDelete') }
  let ok = 0, fail = 0
  for (const node of targets) {
    try { await deleteNode(node.id); ok++ } catch { fail++ }
    batchProgress.value.done++
  }
  batchProgress.value.active = false
  ElMessage.success(t('feature.nodesDeleteComplete', { success: ok, failed: fail }))
  clearSelection()
  loadNodes()
}

function downloadTemplate() {
  downloadBlob('/nodes/import/template', 'devicegrid_nodes_template.csv')
}

async function exportCSV() {
  await downloadBlob('/nodes/export', 'devicegrid_nodes.csv')
}

async function downloadBlob(url: string, filename: string) {
  try {
    const resp = await client.get(url, { responseType: 'blob' })
    const blob = new Blob([resp.data], { type: 'text/csv;charset=utf-8' })
    const link = document.createElement('a')
    link.href = window.URL.createObjectURL(blob)
    link.download = filename
    link.style.display = 'none'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    setTimeout(() => window.URL.revokeObjectURL(link.href), 1000)
    ElMessage.success(t('feature.nodesDownloadSucceeded'))
  } catch { ElMessage.error(t('feature.nodesDownloadFailed')) }
}

async function handleImport(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files?.[0]) return
  const formData = new FormData()
  formData.append('file', input.files[0])
  try {
    const { data } = await client.post('/nodes/import', formData)
    const result = data.data
    ElMessage.success(t('feature.nodesImportComplete', { success: result.success, total: result.total, failed: result.failed }))
    if (result.errors?.length > 0) {
      ElMessageBox.alert(result.errors.join('\n'), t('feature.nodesImportErrors'), { type: 'warning' })
    }
    loadNodes()
  } catch { ElMessage.error(t('feature.nodesImportFailed')) }
  input.value = ''
}

onMounted(() => {
  fleet.start()
  nodes.value = fleetNodes.value
  loadHardware()
})

watch([fleetNodes, fleetMetrics], () => {
  if (!filter.search && !filter.status) {
    nodes.value = fleetNodes.value
    loadHardware()
  }
})
</script>

<style scoped lang="scss">
.btn-primary {
  display: flex; align-items: center; gap: 6px; padding: 8px 16px; border: none; border-radius: 8px;
  background: var(--accent); color: #fff; font-size: 13px; font-weight: 600;
  cursor: pointer; transition: all 0.2s; font-family: inherit; &:hover { background: var(--accent-dark); }
}
.btn-outline {
  display: flex; align-items: center; gap: 5px; padding: 8px 14px; border: 1px solid var(--dg-border); border-radius: 8px;
  background: var(--dg-bg-2); color: var(--dg-text-dim); font-size: 13px; font-weight: 500;
  cursor: pointer; transition: all 0.2s; font-family: inherit; &:hover { border-color: var(--accent); color: var(--accent); }
}
.header-actions { display: flex; align-items: center; gap: 8px; }
.dg-card { padding: 18px; }
.table-toolbar { display: flex; gap: 10px; margin-bottom: 14px;
  .search-box { display: flex; align-items: center; gap: 8px; padding: 0 12px; border-radius: 8px; border: 1px solid var(--dg-border); background: var(--dg-input-bg); svg { color: var(--dg-text-faint); }
    input { width: 200px; height: 36px; border: none; outline: none; background: transparent; color: var(--dg-text); font-size: 13px; font-family: inherit; &::placeholder { color: var(--dg-text-faint); } } }
  .filter-select { height: 36px; padding: 0 12px; border-radius: 8px; border: 1px solid var(--dg-border); background: var(--dg-input-bg); color: var(--dg-text); font-size: 13px; font-family: inherit; cursor: pointer; outline: none; option { background: var(--dg-surface-solid); } }
}
.status-badge { display: inline-flex; align-items: center; gap: 5px; font-size: 11px; font-weight: 500; padding: 3px 8px; border-radius: 20px;
  .dot { width: 6px; height: 6px; border-radius: 50%; }
  &.online { background: var(--dg-success-bg); color: var(--dg-success); .dot { background: var(--dg-success); box-shadow: 0 0 4px var(--dg-success); } }
  &.offline { background: var(--dg-danger-bg); color: var(--dg-danger); .dot { background: var(--dg-danger); } }
  &.untrusted { background: var(--dg-warning-bg); color: var(--dg-warning); .dot { background: var(--dg-warning); } }
  &.error { background: var(--dg-danger-bg); color: var(--dg-danger); .dot { background: var(--dg-danger); } }
}
.chip { display: inline-block; font-size: 10px; font-weight: 500; padding: 2px 7px; border-radius: 5px; margin-right: 3px;
  &.ssh { background: var(--dg-info-bg); color: var(--accent); }
  &.agent { background: var(--dg-success-bg); color: var(--dg-success); }
  &.chip-docker { background: var(--dg-warning-bg); color: var(--dg-warning); }
  &.chip-tag { background: var(--dg-bg-3); color: var(--dg-text-dim); }
}
.mono { font-family: 'JetBrains Mono', monospace; font-size: 12px; color: var(--dg-text-dim); }
.text-muted { color: var(--dg-text-faint); font-size: 12px; }
.geo-cell { display: flex; align-items: center; gap: 5px; font-size: 12px; .flag { font-size: 14px; } }
.hw-cell { .hw-item { font-size: 11px; font-family: 'JetBrains Mono', monospace; color: var(--dg-text-dim); white-space: nowrap; } }
.action-btn { display: inline-flex; align-items: center; gap: 4px; padding: 5px 9px; border: 1px solid var(--dg-border); border-radius: 7px; background: var(--dg-bg-2); color: var(--dg-text-dim); font-size: 11px; cursor: pointer; transition: all 0.15s; font-family: inherit; margin-right: 3px;
  &:hover { border-color: var(--accent); color: var(--accent); }
  &.action-danger:hover { border-color: var(--dg-danger); color: var(--dg-danger); }
  &.action-edit:hover { border-color: var(--accent); color: var(--accent); }
  &.action-agent:hover { border-color: var(--dg-success); color: var(--dg-success); }
}
.action-cell { display: flex; gap: 3px; flex-wrap: wrap; }
.onboarding-steps { margin: 0 0 24px; }
.onboarding-panel { min-height: 210px; display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center;
  h3 { margin: 12px 0 6px; font-size: 17px; }
  p { max-width: 380px; margin: 0; color: var(--dg-text-dim); line-height: 1.6; }
  .onboarding-icon { width: 36px; height: 36px; display: grid; place-items: center; border-radius: 50%; font-weight: 700;
    &.pending { background: var(--dg-info-bg); color: var(--accent); }
    &.success { background: var(--dg-success-bg); color: var(--dg-success); font-size: 20px; }
  }
  .onboarding-warning { margin-top: 14px; color: var(--dg-warning); }
  .onboarding-error { margin-top: 14px; padding: 8px 12px; border-radius: 6px; background: var(--dg-danger-bg); color: var(--dg-danger); font-size: 12px; white-space: pre-wrap; }
}

@media (max-width: 767px) {
  .dg-card { padding: 12px !important; }
  .table-toolbar { flex-direction: column; gap: 8px;
    .search-box { width: 100%; input { width: 100%; min-width: 0; } }
    .filter-select { width: 100%; }
  }
  :deep(.el-table) { min-width: 760px; }
}

/* Batch bar */
.batch-bar {
  display: flex; align-items: center; gap: 12px; padding: 10px 16px; margin-bottom: 12px;
  background: var(--accent-tint, var(--dg-info-bg)); border: 1px solid var(--accent, var(--dg-border-bright));
  border-radius: 10px;
  .batch-count { font-size: 13px; color: var(--dg-text); strong { color: var(--accent); font-size: 15px; } }
  .batch-actions { display: flex; gap: 6px; flex: 1; }
  .batch-btn {
    display: flex; align-items: center; gap: 5px; padding: 6px 14px; border-radius: 8px;
    border: 1px solid var(--dg-border); background: var(--dg-surface-solid); color: var(--dg-text-dim);
    font-size: 12px; font-weight: 500; cursor: pointer; transition: all 0.15s; font-family: inherit;
    &:hover { border-color: var(--accent); color: var(--accent); background: var(--dg-surface-hover); }
    &.batch-danger:hover { border-color: var(--dg-danger); color: var(--dg-danger); }
  }
  .batch-clear {
    background: none; border: none; color: var(--dg-text-faint); font-size: 12px; cursor: pointer; font-family: inherit;
    &:hover { color: var(--dg-text); }
  }
}

/* Batch progress overlay */
.batch-progress {
  position: fixed; bottom: 24px; right: 24px; z-index: 9999;
  padding: 16px 20px; background: var(--dg-surface-solid); border: 1px solid var(--accent);
  border-radius: 12px; box-shadow: var(--dg-shadow-lg); display: flex; flex-direction: column; gap: 8px;
  min-width: 260px;
  .bp-label { font-size: 13px; font-weight: 600; color: var(--dg-text); }
  .bp-count { font-size: 12px; color: var(--dg-text-dim); }
  .bp-track { height: 4px; background: var(--dg-border); border-radius: 2px; overflow: hidden;
    .bp-fill { height: 100%; background: var(--accent); border-radius: 2px; transition: width 0.3s; } }
}
</style>
