<template>
  <div class="page-container">
    <div class="page-header">
      <div><h2>批量部署</h2><p class="page-subtitle">向多个节点批量推送脚本并实时查看执行结果</p></div>
      <button class="btn-primary" @click="showWizard = true">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none"><path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
        新建任务
      </button>
    </div>

    <!-- Task List -->
    <div class="dg-card task-list-card" v-loading="loading">
      <div v-if="tasks.length === 0" class="empty-state">
        <svg viewBox="0 0 24 24" width="40" height="40" fill="none" style="opacity:0.15;margin-bottom:8px"><path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" stroke="currentColor" stroke-width="1.5"/></svg>
        <p>暂无部署任务</p>
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
              <span>{{ t.node_ids?.length || 0 }} 节点</span>
              <span>{{ formatTime(t.created_at) }}</span>
            </div>
          </div>
          <span class="task-status-badge" :class="t.status">{{ statusLabel(t.status) }}</span>
        </div>
      </div>
    </div>

    <!-- Wizard Dialog -->
    <el-dialog v-model="showWizard" title="新建部署任务" width="680px" top="6vh" :close-on-click-modal="false">
      <!-- Step 1: Select Nodes -->
      <div class="wizard-step">
        <div class="step-title"><span class="step-num">1</span>选择目标节点</div>
        <div class="node-pick-list">
          <label v-for="n in nodes" :key="n.id" class="node-pick" :class="{ checked: selectedNodes.includes(n.id), disabled: n.status !== 'online' }">
            <input type="checkbox" :value="n.id" v-model="selectedNodes" :disabled="n.status !== 'online'" />
            <span class="np-dot" :class="n.status"></span>
            <span class="np-name">{{ n.name }}</span>
            <span class="np-ip">{{ n.host }}</span>
          </label>
          <div v-if="nodes.length === 0" class="pick-empty">暂无节点</div>
        </div>
        <div class="pick-toolbar">
          <button class="link-btn" @click="selectAllOnline">全选在线</button>
          <button class="link-btn" @click="selectedNodes = []">清空</button>
          <span class="pick-count">已选 {{ selectedNodes.length }} 个</span>
        </div>
      </div>

      <!-- Step 2: Task Config -->
      <div class="wizard-step">
        <div class="step-title"><span class="step-num">2</span>任务配置</div>
        <el-form label-position="top" class="wiz-form">
          <el-form-item label="任务名称">
            <el-input v-model="deployForm.name" placeholder="如：批量更新系统" />
          </el-form-item>
          <div class="form-row">
            <el-form-item label="类型" style="flex:1">
              <el-select v-model="deployForm.type" style="width:100%">
                <el-option label="Shell 脚本" value="script" />
                <el-option label="安装软件包" value="package" />
              </el-select>
            </el-form-item>
            <el-form-item label="超时(秒)" style="width:120px">
              <el-input-number v-model="deployForm.timeout" :min="0" :step="60" style="width:100%" />
            </el-form-item>
            <el-form-item label="并发数" style="width:120px">
              <el-input-number v-model="deployForm.concurrency" :min="1" :max="50" style="width:100%" />
            </el-form-item>
          </div>
          <el-form-item :label="deployForm.type === 'script' ? '脚本内容' : '包名（空格分隔）'">
            <textarea v-model="deployForm.payload" class="script-editor" :placeholder="deployForm.type === 'script' ? '#!/bin/bash\napt update && apt upgrade -y' : 'nginx redis-server'" spellcheck="false" rows="6"></textarea>
          </el-form-item>
        </el-form>
      </div>

      <template #footer>
        <el-button @click="showWizard = false">取消</el-button>
        <el-button type="primary" :loading="executing" :disabled="selectedNodes.length === 0 || !deployForm.payload" @click="executeDeploy">执行部署</el-button>
      </template>
    </el-dialog>

    <!-- Task Detail Dialog -->
    <el-dialog v-model="detailVisible" :title="currentTask?.name || '任务详情'" width="860px" top="5vh" :close-on-click-modal="false">
      <div v-if="taskDetail" class="detail-content">
        <div class="detail-summary">
          <div class="ds-item"><span class="ds-label">状态</span><span class="ds-status" :class="taskDetail.task.status">{{ statusLabel(taskDetail.task.status) }}</span></div>
          <div class="ds-item"><span class="ds-label">节点数</span><span>{{ taskDetail.results.length }}</span></div>
          <div class="ds-item"><span class="ds-label">类型</span><span>{{ typeLabel(taskDetail.task.type) }}</span></div>
          <div class="ds-item"><span class="ds-label">创建</span><span>{{ formatTime(taskDetail.task.created_at) }}</span></div>
        </div>

        <div class="results-grid">
          <div v-for="r in taskDetail.results" :key="r.id" class="result-card" :class="r.status">
            <div class="rc-head">
              <span class="rc-dot" :class="r.status"></span>
              <span class="rc-name">{{ r.node_name || r.node_id.substring(0, 8) }}</span>
              <span class="rc-exit" v-if="r.status !== 'running'">exit={{ r.exit_code }}</span>
            </div>
            <pre class="rc-output">{{ r.output || r.error || (r.status === 'running' ? '执行中...' : '无输出') }}</pre>
          </div>
        </div>

        <div class="detail-payload" v-if="taskDetail.task.payload">
          <div class="dp-label">执行内容</div>
          <pre class="dp-code">{{ taskDetail.task.payload }}</pre>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { listNodes, type Node } from '@/api/nodes'
import { listDeploys, createDeploy, getDeploy, type DeployTask } from '@/api/deploy'

const loading = ref(false)
const executing = ref(false)
const tasks = ref<DeployTask[]>([])
const nodes = ref<Node[]>([])
const showWizard = ref(false)
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

function statusLabel(s: string) { return ({ pending: '等待', running: '执行中', completed: '成功', failed: '失败', cancelled: '已取消' } as Record<string, string>)[s] || s }
function typeLabel(t: string) { return ({ script: '脚本', file: '文件', package: '软件包' } as Record<string, string>)[t] || t }
function formatTime(t: string) { if (!t || t.startsWith('0001')) return '-'; return new Date(t).toLocaleString() }

async function loadTasks() { loading.value = true; try { tasks.value = await listDeploys() } finally { loading.value = false } }

function selectAllOnline() { selectedNodes.value = nodes.value.filter(n => n.status === 'online').map(n => n.id) }

async function executeDeploy() {
  if (selectedNodes.value.length === 0) { ElMessage.warning('请选择至少一个节点'); return }
  if (!deployForm.payload) { ElMessage.warning('请输入执行内容'); return }
  executing.value = true
  try {
    const task = await createDeploy({
      name: deployForm.name || `部署-${Date.now()}`,
      type: deployForm.type,
      node_ids: selectedNodes.value,
      payload: deployForm.payload,
      timeout: deployForm.timeout,
      concurrency: deployForm.concurrency,
    })
    ElMessage.success('任务已创建，正在执行')
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
</script>

<style scoped lang="scss">
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
</style>
