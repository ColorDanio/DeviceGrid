<template>
  <div class="page-container">
    <div class="page-header">
      <div><h2>{{ t('feature.deployTitle') }}</h2><p class="page-subtitle">{{ t('feature.deploySubtitle') }}</p></div>
      <div class="header-actions">
        <button class="btn-primary" @click="showFileDialog = true">{{ t('feature.fileDistribution') }}</button>
        <button class="btn-primary" @click="showWizard = true">{{ t('feature.newDeployTask') }}</button>
      </div>
    </div>

    <!-- Task List -->
    <div class="dg-card task-list-card" v-loading="loading">
      <div v-if="tasks.length === 0" class="empty-state">
        <svg viewBox="0 0 24 24" width="40" height="40" fill="none" style="opacity:0.15;margin-bottom:8px"><path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" stroke="currentColor" stroke-width="1.5"/></svg>
        <p>{{ nodes.length === 0 ? t('feature.deployNeedsNode') : t('feature.noDeployTasks') }}</p>
        <button v-if="nodes.length === 0" class="btn-primary" @click="$router.push('/nodes')">{{ t('feature.addNode') }}</button>
      </div>
      <div v-else class="task-list">
        <div v-for="t in tasks" :key="t.id" class="task-row" @click="viewTask(t)">
          <div class="task-status-icon" :class="t.status">
            <svg v-if="t.status === 'running'" viewBox="0 0 24 24" width="14" height="14" fill="none"><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" opacity="0.3"/><path d="M12 6v6l4 2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
            <svg v-else-if="t.status === 'completed'" viewBox="0 0 24 24" width="14" height="14" fill="none"><path d="M5 13l4 4L19 7" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/></svg>
            <svg v-else-if="t.status === 'failed'" viewBox="0 0 24 24" width="14" height="14" fill="none"><path d="M6 6l12 12M6 18L18 6" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/></svg>
            <svg v-else viewBox="0 0 24 24" width="14" height="14" fill="none"><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2"/></svg>
          </div>
          <div class="task-info">
            <div class="task-name">{{ t.name }}</div>
            <div class="task-meta">
              <span class="tm-type">{{ typeLabel(t.type) }}</span>
              <span>{{ $t('feature.nodeCount', { count: t.node_ids?.length || 0 }) }}</span>
              <span>{{ formatTime(t.created_at) }}</span>
            </div>
          </div>
          <span class="task-status-badge" :class="t.status">{{ statusLabel(t.status) }}</span>
        </div>
      </div>
    </div>

    <!-- Wizard Dialog -->
    <el-dialog v-model="showWizard" :title="t('feature.newDeployTask')" width="680px" top="6vh" :close-on-click-modal="false">
      <!-- Step 1: Select Nodes -->
      <div class="wizard-step">
        <div class="step-title"><span class="step-num">1</span>{{ t('feature.selectTargetNodes') }}</div>
        <div class="node-pick-list">
          <label v-for="n in nodes" :key="n.id" class="node-pick" :class="{ checked: selectedNodes.includes(n.id), disabled: n.status !== 'online' }">
            <input type="checkbox" :value="n.id" v-model="selectedNodes" :disabled="n.status !== 'online'" />
            <span class="np-dot" :class="n.status"></span>
            <span class="np-name">{{ n.name }}</span>
            <span class="np-ip">{{ n.host }}</span>
          </label>
          <div v-if="nodes.length === 0" class="pick-empty">{{ t('feature.noNodes') }}</div>
        </div>
        <div class="pick-toolbar">
          <button class="link-btn" @click="selectAllOnline">{{ t('feature.selectAllOnline') }}</button>
          <button class="link-btn" @click="selectedNodes = []">{{ t('feature.clear') }}</button>
          <span class="pick-count">{{ t('feature.selectedCount', { count: selectedNodes.length }) }}</span>
        </div>
      </div>

      <!-- Step 2: Task Config -->
      <div class="wizard-step">
        <div class="step-title"><span class="step-num">2</span>{{ t('feature.taskConfiguration') }}</div>
        <el-form label-position="top" class="wiz-form">
          <el-form-item :label="t('feature.taskName')">
            <el-input v-model="deployForm.name" :placeholder="t('feature.taskExampleDeploy')" />
          </el-form-item>
          <div class="form-row">
            <el-form-item :label="t('feature.type')" style="flex:1">
              <el-select v-model="deployForm.type" style="width:100%">
                <el-option :label="t('feature.shellScript')" value="script" />
                <el-option :label="t('feature.installPackage')" value="package" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('feature.timeoutSeconds')" style="width:120px">
              <el-input-number v-model="deployForm.timeout" :min="0" :step="60" style="width:100%" />
            </el-form-item>
            <el-form-item :label="t('feature.concurrency')" style="width:120px">
              <el-input-number v-model="deployForm.concurrency" :min="1" :max="50" style="width:100%" />
            </el-form-item>
          </div>
          <el-form-item :label="t('feature.scriptContent')">
            <textarea v-model="deployForm.payload" class="script-editor" :placeholder="deployForm.type === 'script' ? '#!/bin/bash\napt update && apt upgrade -y' : 'nginx redis-server'" spellcheck="false" rows="6"></textarea>
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
          <el-button @click="showWizard = false">{{ t('common.cancel') }}</el-button>
          <el-button type="primary" :loading="executing" :disabled="selectedNodes.length === 0 || !deployForm.payload" @click="executeDeploy">{{ t('feature.executeDeploy') }}</el-button>
        </template>
      </el-dialog>

      <!-- File Distribution Dialog -->
      <el-dialog v-model="showFileDialog" :title="t('feature.batchFileDistribution')" width="520px">
        <div class="file-dist-form">
          <div class="fd-row">
            <label>{{ t('feature.selectTargetNodes') }}</label>
            <div class="fd-nodes">
              <label v-for="n in onlineNodes" :key="n.id" class="fd-node">
                <input type="checkbox" :value="n.id" v-model="fileForm.nodeIds" /> {{ n.name }}
              </label>
            </div>
          </div>
          <div class="fd-row">
            <label>{{ t('feature.remotePath') }}</label>
            <input v-model="fileForm.remotePath" :placeholder="t('feature.remotePathPlaceholder')" />
          </div>
          <div class="fd-row">
            <label>{{ t('feature.selectFile') }}</label>
            <input type="file" @change="onFileSelect" class="fd-file" />
          </div>
        </div>
        <template #footer>
          <el-button @click="showFileDialog = false">{{ t('common.cancel') }}</el-button>
          <el-button type="primary" :loading="distributing" :disabled="fileForm.nodeIds.length === 0 || !selectedFile" @click="handleDistribute">{{ t('feature.distribute') }}</el-button>
        </template>
      </el-dialog>

    <!-- Task Detail Dialog -->
    <el-dialog v-model="detailVisible" :title="currentTask?.name || t('feature.taskDetail')" width="860px" top="5vh" :close-on-click-modal="false">
      <div v-if="taskDetail" class="detail-content">
        <div class="detail-summary">
          <div class="ds-item"><span class="ds-label">{{ t('feature.status') }}</span><span class="ds-status" :class="taskDetail.task.status">{{ statusLabel(taskDetail.task.status) }}</span></div>
          <div class="ds-item"><span class="ds-label">{{ t('common.nodeCount') }}</span><span>{{ taskDetail.results.length }}</span></div>
          <div class="ds-item"><span class="ds-label">{{ t('feature.type') }}</span><span>{{ typeLabel(taskDetail.task.type) }}</span></div>
          <div class="ds-item"><span class="ds-label">{{ t('feature.createdLabel') }}</span><span>{{ formatTime(taskDetail.task.created_at) }}</span></div>
        </div>

        <div class="results-grid">
          <div v-for="r in taskDetail.results" :key="r.id" class="result-card" :class="r.status">
            <div class="rc-head">
              <span class="rc-dot" :class="r.status"></span>
              <span class="rc-name">{{ r.node_name || r.node_id.substring(0, 8) }}</span>
              <span class="rc-exit" v-if="r.status !== 'running'">exit={{ r.exit_code }}</span>
            </div>
            <pre class="rc-output">{{ r.output || r.error || (r.status === 'running' ? t('feature.running') : t('feature.noOutput')) }}</pre>
          </div>
        </div>

        <div class="detail-payload" v-if="taskDetail.task.payload">
          <div class="dp-label">{{ t('feature.executionContent') }}</div>
          <pre class="dp-code">{{ taskDetail.task.payload }}</pre>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { listNodes, type Node } from '@/api/nodes'
import { listDeploys, createDeploy, getDeploy, type DeployTask } from '@/api/deploy'
import { distributeFile } from '@/api/features'

const { t } = useI18n()

const loading = ref(false)
const executing = ref(false)
const tasks = ref<DeployTask[]>([])
const nodes = ref<Node[]>([])
const showWizard = ref(false)
const showFileDialog = ref(false)
const distributing = ref(false)
const selectedFile = ref<File | null>(null)
const fileForm = reactive({ nodeIds: [] as string[], remotePath: '' })
const onlineNodes = computed(() => nodes.value.filter(n => n.status === 'online'))
const selectedNodes = ref<string[]>([])
const detailVisible = ref(false)
const currentTask = ref<DeployTask | null>(null)
const taskDetail = ref<{ task: DeployTask; results: any[] } | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null

const deployForm = reactive({
  name: '',
  type: 'script',
  payload: '',
  timeout: 300,
  concurrency: 10,
})

function statusLabel(s: string) { return ({ pending: t('feature.pending'), running: t('feature.running'), completed: t('feature.completed'), failed: t('feature.failed'), cancelled: t('feature.cancelled') } as Record<string, string>)[s] || s }
function typeLabel(type: string) { return ({ script: t('feature.script'), file: t('feature.file'), package: t('feature.package') } as Record<string, string>)[type] || type }
function formatTime(t: string) { if (!t || t.startsWith('0001')) return '-'; return new Date(t).toLocaleString() }

async function loadTasks() { loading.value = true; try { tasks.value = await listDeploys() } finally { loading.value = false } }

function selectAllOnline() { selectedNodes.value = nodes.value.filter(n => n.status === 'online').map(n => n.id) }

async function executeDeploy() {
  if (selectedNodes.value.length === 0) { ElMessage.warning(t('feature.selectAtLeastOneNode')); return }
  if (!deployForm.payload) { ElMessage.warning(t('feature.enterExecutionContent')); return }
  executing.value = true
  try {
    const task = await createDeploy({
      name: deployForm.name || t('feature.deployDefaultName', { timestamp: Date.now() }),
      type: deployForm.type,
      node_ids: selectedNodes.value,
      payload: deployForm.payload,
      timeout: deployForm.timeout,
      concurrency: deployForm.concurrency,
    })
    ElMessage.success(t('feature.taskCreatedExecuting'))
    showWizard.value = false
    deployForm.name = ''; deployForm.payload = ''; selectedNodes.value = []
    loadTasks()
    viewTask(task)
  } catch {} finally { executing.value = false }
}

async function viewTask(t: DeployTask) {
  currentTask.value = t
  detailVisible.value = true
  await loadTaskDetail(t.id)
}

async function loadTaskDetail(taskId: string) {
  try {
    taskDetail.value = await getDeploy(taskId)
    if (taskDetail.value && taskDetail.value.task.status === 'running') {
      setTimeout(() => loadTaskDetail(taskId), 3000)
    }
  } catch {}
}

onMounted(async () => {
  loadTasks()
  nodes.value = await listNodes()
  pollTimer = setInterval(loadTasks, 10000)
})
onBeforeUnmount(() => { if (pollTimer) clearInterval(pollTimer) })

function onFileSelect(e: Event) { const input = e.target as HTMLInputElement; selectedFile.value = input.files?.[0] || null }
async function handleDistribute() {
  if (!selectedFile.value || fileForm.nodeIds.length === 0) return
  distributing.value = true
  try {
    const result = await distributeFile(fileForm.nodeIds, selectedFile.value, fileForm.remotePath || undefined)
    ElMessage.success(t('feature.distributionComplete', { success: result.success, failed: result.failed }))
    showFileDialog.value = false; selectedFile.value = null; fileForm.nodeIds = []; fileForm.remotePath = ''
  } catch {} finally { distributing.value = false }
}
</script>

<style scoped lang="scss">
.header-actions { display: flex; gap: 8px; }
.btn-primary { display: flex; align-items: center; gap: 6px; padding: 8px 16px; border: none; border-radius: 8px; background: var(--accent); color: #fff; font-size: 13px; font-weight: 600; cursor: pointer; font-family: inherit; &:hover { background: var(--accent-dark); } }

.task-list-card { min-height: 300px; padding: 12px; }
.empty-state { display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 280px; color: var(--dg-text-faint); font-size: 14px; }
.task-list { display: flex; flex-direction: column; gap: 4px; }
.task-row { display: flex; align-items: center; gap: 12px; padding: 10px 14px; border-radius: 10px; cursor: pointer; transition: all 0.15s; &:hover { background: var(--dg-table-row-hover); }
  .task-status-icon { width: 28px; height: 28px; border-radius: 8px; display: flex; align-items: center; justify-content: center; flex-shrink: 0;
    &.running { background: var(--dg-info-bg); color: var(--accent); } &.completed { background: var(--dg-success-bg); color: var(--dg-success); } &.failed { background: var(--dg-danger-bg); color: var(--dg-danger); } &.pending { background: var(--dg-bg-3); color: var(--dg-text-faint); } }
  .task-info { flex: 1; .task-name { font-size: 14px; font-weight: 600; } .task-meta { display: flex; gap: 10px; font-size: 11px; color: var(--dg-text-faint); margin-top: 2px; .tm-type { font-weight: 500; } } }
  .task-status-badge { font-size: 11px; font-weight: 600; padding: 3px 10px; border-radius: 6px;
    &.running { background: var(--dg-info-bg); color: var(--accent); } &.completed { background: var(--dg-success-bg); color: var(--dg-success); } &.failed { background: var(--dg-danger-bg); color: var(--dg-danger); } &.pending { background: var(--dg-bg-3); color: var(--dg-text-faint); } }
}

/* Wizard */
.wizard-step { margin-bottom: 20px; }
.step-title { display: flex; align-items: center; gap: 8px; font-size: 14px; font-weight: 600; margin-bottom: 12px;
  .step-num { width: 20px; height: 20px; border-radius: 6px; background: var(--accent); color: #fff; display: flex; align-items: center; justify-content: center; font-size: 11px; } }

.node-pick-list { max-height: 200px; overflow-y: auto; display: flex; flex-direction: column; gap: 4px; padding: 8px; background: var(--dg-bg-3); border-radius: 10px; }
.node-pick { display: flex; align-items: center; gap: 8px; padding: 7px 10px; border-radius: 8px; cursor: pointer; transition: all 0.15s;
  &:hover { background: var(--dg-surface-hover); } &.checked { background: var(--dg-info-bg); } &.disabled { opacity: 0.4; cursor: not-allowed; }
  input { accent-color: var(--accent); width: 14px; height: 14px; }
  .np-dot { width: 7px; height: 7px; border-radius: 50%; &.online { background: var(--dg-success); } &.offline { background: var(--dg-danger); } &.untrusted { background: var(--dg-warning); } }
  .np-name { font-size: 13px; font-weight: 500; } .np-ip { font-size: 11px; color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace; margin-left: auto; } }
.pick-empty { padding: 20px; text-align: center; color: var(--dg-text-faint); font-size: 13px; }
.pick-toolbar { display: flex; align-items: center; gap: 12px; padding: 8px 4px; .link-btn { background: none; border: none; color: var(--accent); font-size: 12px; cursor: pointer; font-family: inherit; &:hover { text-decoration: underline; } } .pick-count { font-size: 12px; color: var(--dg-text-faint); margin-left: auto; } }

.wiz-form { .form-row { display: flex; gap: 12px; } }
.script-editor { width: 100%; padding: 12px; background: var(--dg-input-bg); border: 1px solid var(--dg-input-border); border-radius: 8px; color: var(--dg-text); font-family: 'JetBrains Mono', monospace; font-size: 13px; line-height: 1.6; outline: none; resize: vertical; &:focus { border-color: var(--accent); } }

/* Task Detail */
.detail-content { display: flex; flex-direction: column; gap: 16px; }
.detail-summary { display: flex; gap: 20px; padding: 14px; background: var(--dg-bg-3); border-radius: 10px;
  .ds-item { display: flex; flex-direction: column; .ds-label { font-size: 11px; color: var(--dg-text-faint); text-transform: uppercase; } .ds-status { font-weight: 600; &.completed { color: var(--dg-success); } &.failed { color: var(--dg-danger); } &.running { color: var(--accent); } } } }

.results-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 10px; max-height: 400px; overflow-y: auto; }
.result-card { padding: 12px; border: 1px solid var(--dg-border); border-radius: 10px;
  &.success { border-left: 3px solid var(--dg-success); } &.failed { border-left: 3px solid var(--dg-danger); } &.running { border-left: 3px solid var(--accent); }
  .rc-head { display: flex; align-items: center; gap: 6px; margin-bottom: 8px; .rc-dot { width: 6px; height: 6px; border-radius: 50%; &.success { background: var(--dg-success); } &.failed { background: var(--dg-danger); } &.running { background: var(--accent); } } .rc-name { font-size: 13px; font-weight: 600; flex: 1; } .rc-exit { font-size: 10px; color: var(--dg-text-faint); font-family: monospace; } }
  .rc-output { font-size: 11px; font-family: 'JetBrains Mono', monospace; max-height: 120px; overflow-y: auto; color: var(--dg-text-dim); white-space: pre-wrap; word-break: break-all; margin: 0; } }

.detail-payload { .dp-label { font-size: 12px; font-weight: 600; margin-bottom: 6px; } .dp-code { font-size: 11px; font-family: 'JetBrains Mono', monospace; padding: 10px; background: var(--dg-bg-3); border-radius: 8px; max-height: 120px; overflow-y: auto; white-space: pre-wrap; margin: 0; } }

.file-dist-form { display: flex; flex-direction: column; gap: 14px; }
.fd-row { display: flex; flex-direction: column; gap: 6px; label { font-size: 12px; color: var(--dg-text-dim); }
  input { height: 32px; padding: 0 10px; border: 1px solid var(--dg-border); border-radius: 6px; background: var(--dg-input-bg); color: var(--dg-text); font-size: 12px; outline: none; font-family: inherit; &:focus { border-color: var(--accent); } }
  .fd-file { padding: 4px; border: 1px solid var(--dg-border); border-radius: 6px; background: var(--dg-input-bg); color: var(--dg-text); } }
.fd-nodes { display: flex; flex-wrap: wrap; gap: 6px; .fd-node { display: flex; align-items: center; gap: 4px; font-size: 12px; padding: 3px 10px; border-radius: 6px; border: 1px solid var(--dg-border); cursor: pointer; input { accent-color: var(--accent); } } }
</style>
