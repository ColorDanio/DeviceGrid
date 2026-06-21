<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title-group">
        <h2>RKE2 集群</h2>
        <p class="page-subtitle">管理 RKE2 Kubernetes 集群的全生命周期</p>
      </div>
      <button class="btn-primary" @click="showWizard = true">
        <svg viewBox="0 0 24 24" fill="none" width="14" height="14"><path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
        创建集群
      </button>
    </div>

    <!-- Cluster List -->
    <div v-loading="loading" class="cluster-grid">
      <div v-for="c in clusters" :key="c.id" class="cluster-card dg-card" @click="viewCluster(c)">
        <div class="cc-header">
          <div class="cc-icon"><svg viewBox="0 0 24 24" width="22" height="22" fill="none"><circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="1.8"/><circle cx="5" cy="5" r="2" stroke="currentColor" stroke-width="1.8"/><circle cx="19" cy="5" r="2" stroke="currentColor" stroke-width="1.8"/><circle cx="5" cy="19" r="2" stroke="currentColor" stroke-width="1.8"/><circle cx="19" cy="19" r="2" stroke="currentColor" stroke-width="1.8"/></svg></div>
          <div class="cc-info">
            <span class="cc-name">{{ c.name }}</span>
            <span class="cc-version">{{ c.version || 'latest' }}</span>
          </div>
          <span class="cc-status" :class="c.status">{{ statusLabel(c.status) }}</span>
        </div>
        <div class="cc-body">
          <div class="cc-row"><span>节点数</span><strong>{{ c.nodes?.length || 0 }}</strong></div>
          <div class="cc-row"><span>Server</span><strong class="mono">{{ getNodeName(c.server_node) }}</strong></div>
        </div>
      </div>

      <div v-if="!loading && clusters.length === 0" class="empty-card" @click="showWizard = true">
        <svg viewBox="0 0 24 24" width="32" height="32" fill="none" style="opacity:0.3"><circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="1.5"/><circle cx="5" cy="5" r="2" stroke="currentColor" stroke-width="1.5"/><circle cx="19" cy="5" r="2" stroke="currentColor" stroke-width="1.5"/><circle cx="5" cy="19" r="2" stroke="currentColor" stroke-width="1.5"/><circle cx="19" cy="19" r="2" stroke="currentColor" stroke-width="1.5"/></svg>
        <span>创建第一个集群</span>
      </div>
    </div>

    <!-- Create Cluster Wizard -->
    <el-dialog v-model="showWizard" title="创建 RKE2 集群" width="640px" :close-on-click-modal="false" align-center>
      <!-- Step 1: Select Server Node -->
      <div class="wiz-step">
        <div class="step-title"><span class="step-num">1</span>选择 Server 节点（控制平面）</div>
        <div class="wiz-nodes">
          <label v-for="n in onlineNodes" :key="n.id" class="wiz-node" :class="{ checked: form.serverNode === n.id }">
            <input type="radio" :value="n.id" v-model="form.serverNode" />
            <span class="wn-dot online"></span>
            <span class="wn-name">{{ n.name }}</span>
            <span class="wn-host">{{ n.host }}</span>
            <span v-if="n.country_code" class="wn-flag">{{ flag(n.country_code) }}</span>
          </label>
          <div v-if="onlineNodes.length === 0" class="wiz-empty">暂无在线节点</div>
        </div>
        <button v-if="form.serverNode" class="preflight-btn" @click="runPreflight(form.serverNode)" :disabled="preflightLoading">
          {{ preflightLoading ? '检查中...' : '运行前置检查' }}
        </button>
        <!-- Pre-flight Results -->
        <div v-if="preflightResult" class="preflight-result">
          <div v-for="c in preflightResult.checks" :key="c.name" class="pf-item" :class="{ pass: c.passed, fail: !c.passed, fixed: c.fixed }">
            <span class="pf-icon">{{ c.passed ? '✓' : c.fixed ? '🔧' : '✗' }}</span>
            <span class="pf-name">{{ c.name }}</span>
            <span class="pf-value">{{ c.value }}</span>
            <span class="pf-required">{{ c.required }}</span>
            <span v-if="c.warning" class="pf-warning">{{ c.warning }}</span>
          </div>
        </div>
      </div>

      <!-- Step 2: Select Agent Nodes -->
      <div class="wiz-step">
        <div class="step-title"><span class="step-num">2</span>选择 Agent 节点（工作节点，可选）</div>
        <div class="wiz-nodes">
          <label v-for="n in onlineNodes.filter(x => x.id !== form.serverNode)" :key="n.id" class="wiz-node check" :class="{ checked: form.agentNodes.includes(n.id) }">
            <input type="checkbox" :value="n.id" v-model="form.agentNodes" />
            <span class="wn-dot online"></span>
            <span class="wn-name">{{ n.name }}</span>
            <span class="wn-host">{{ n.host }}</span>
          </label>
          <div v-if="onlineNodes.filter(x => x.id !== form.serverNode).length === 0" class="wiz-empty">暂无其他在线节点</div>
        </div>
      </div>

      <!-- Step 3: Configuration -->
      <div class="wiz-step">
        <div class="step-title"><span class="step-num">3</span>集群配置</div>
        <div class="wiz-config">
          <div class="cfg-row">
            <label>集群名称</label>
            <input v-model="form.name" placeholder="my-cluster" />
          </div>
          <div class="cfg-row">
            <label>RKE2 版本</label>
            <input v-model="form.version" placeholder="留空使用最新版" />
          </div>
          <div class="cfg-row">
            <label>CNI 插件</label>
            <select v-model="form.cni">
              <option value="canal">canal (默认)</option>
              <option value="calico">calico</option>
              <option value="cilium">cilium</option>
            </select>
          </div>
          <div class="cfg-row">
            <label>网络代理 (可选)</label>
            <input v-model="form.proxy" placeholder="http://proxy:port" />
          </div>
          <div class="cfg-row">
            <label>ETCD 自动备份</label>
            <label class="toggle"><input type="checkbox" v-model="form.etcdSnapshots" /> <span></span></label>
          </div>
        </div>
      </div>

      <template #footer>
        <el-button @click="showWizard = false">取消</el-button>
        <el-button type="primary" :loading="creating" :disabled="!form.serverNode || !form.name" @click="handleCreate">创建集群</el-button>
      </template>
    </el-dialog>

    <!-- Cluster Detail -->
    <el-dialog v-model="detailVisible" :title="currentCluster?.name || '集群详情'" width="860px" top="4vh" align-center>
      <div v-if="currentCluster" class="cluster-detail">
        <!-- Tabs -->
        <div class="detail-tabs">
          <button v-for="t in tabs" :key="t.id" class="detail-tab" :class="{ active: activeTab === t.id }" @click="activeTab = t.id">{{ t.name }}</button>
        </div>

        <!-- Overview Tab -->
        <div v-if="activeTab === 'overview'" class="detail-content">
          <div class="detail-info">
            <div class="di-item"><span>状态</span><strong class="cc-status" :class="currentCluster.status">{{ statusLabel(currentCluster.status) }}</strong></div>
            <div class="di-item"><span>版本</span><strong>{{ currentCluster.version || 'latest' }}</strong></div>
            <div class="di-item"><span>节点数</span><strong>{{ currentCluster.nodes?.length || 0 }}</strong></div>
            <div class="di-item"><span>创建时间</span><strong>{{ formatTime(currentCluster.created_at) }}</strong></div>
          </div>
          <div class="detail-actions">
            <button class="action-btn" @click="loadClusterStatus(currentCluster.id)">刷新状态</button>
            <button class="action-btn action-danger" @click="handleDelete(currentCluster)">卸载集群</button>
          </div>
          <!-- Node status table -->
          <div v-if="clusterStatus.length > 0" class="status-table">
            <div v-for="n in clusterStatus" :key="n.name" class="status-row">
              <span class="sr-dot" :class="{ ready: n.ready }"></span>
              <span class="sr-name">{{ n.name }}</span>
              <span class="sr-role">{{ n.role }}</span>
              <span class="sr-version">{{ n.version }}</span>
              <span class="sr-state" :class="{ ready: n.ready }">{{ n.status }}</span>
            </div>
          </div>
        </div>

        <!-- Config Tab -->
        <div v-if="activeTab === 'config'" class="detail-content">
          <div class="config-header">
            <h4>config.yaml</h4>
            <button class="action-btn" @click="handleSaveConfig">保存</button>
          </div>
          <textarea v-model="editingConfig" class="config-editor" spellcheck="false" rows="14"></textarea>
        </div>

        <!-- Pods Tab -->
        <div v-if="activeTab === 'pods'" class="detail-content">
          <div class="config-header">
            <h4>Pod 列表</h4>
            <button class="action-btn" @click="loadPods(currentCluster.id)">刷新</button>
          </div>
          <pre class="console-output">{{ podsOutput || '点击刷新查看 Pod 列表' }}</pre>
        </div>

        <!-- Helm Tab -->
        <div v-if="activeTab === 'helm'" class="detail-content">
          <div class="config-header">
            <h4>Helm 管理</h4>
            <button class="action-btn" @click="loadHelm(currentCluster.id)">刷新列表</button>
          </div>
          <div class="helm-install-bar">
            <input v-model="helmChart" placeholder="chart 名称 (如: bitnami/nginx)" class="helm-input" />
            <input v-model="helmRepo" placeholder="repo URL (可选)" class="helm-input" />
            <input v-model="helmNs" placeholder="namespace" class="helm-input small" />
            <button class="action-btn action-primary" @click="handleHelmInstall" :disabled="!helmChart">安装</button>
          </div>
          <pre class="console-output">{{ helmOutput || '点击刷新查看已安装的 Helm releases' }}</pre>
        </div>

        <!-- Rancher Tab -->
        <div v-if="activeTab === 'rancher'" class="detail-content">
          <div class="rancher-section">
            <div class="rancher-status">
              <span class="rs-label">Rancher Manager 状态:</span>
              <span v-if="rancherInfo.installed" class="rs-installed">已安装 ({{ rancherInfo.version }})</span>
              <span v-else class="rs-not-installed">未安装</span>
            </div>
            <div v-if="!rancherInfo.installed" class="rancher-install">
              <input v-model="rancherHost" placeholder="Rancher 访问域名 (如: rancher.example.com)" class="helm-input" />
              <button class="action-btn action-primary" @click="handleInstallRancher" :disabled="!rancherHost">安装 Rancher</button>
            </div>
            <div v-if="rancherMsg" class="rancher-msg">{{ rancherMsg }}</div>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listNodes, type Node } from '@/api/nodes'
import * as clustersApi from '@/api/clusters'

const loading = ref(false)
const clusters = ref<clustersApi.Cluster[]>([])
const nodes = ref<Node[]>([])
const showWizard = ref(false)
const creating = ref(false)
const detailVisible = ref(false)
const currentCluster = ref<clustersApi.Cluster | null>(null)
const activeTab = ref('overview')

// Wizard form
const form = ref({
  name: '',
  version: '',
  serverNode: '',
  agentNodes: [] as string[],
  cni: 'canal',
  proxy: '',
  etcdSnapshots: false,
})

// Pre-flight
const preflightLoading = ref(false)
const preflightResult = ref<clustersApi.PreFlightResult | null>(null)

// Detail data
const clusterStatus = ref<any[]>([])
const editingConfig = ref('')
const podsOutput = ref('')
const helmOutput = ref('')
const helmChart = ref('')
const helmRepo = ref('')
const helmNs = ref('')
const rancherInfo = ref({ installed: false, version: '' })
const rancherHost = ref('')
const rancherMsg = ref('')

const onlineNodes = computed(() => nodes.value.filter(n => n.status === 'online'))
const tabs = [
  { id: 'overview', name: '概览' },
  { id: 'config', name: '配置' },
  { id: 'pods', name: 'Pods' },
  { id: 'helm', name: 'Helm' },
  { id: 'rancher', name: 'Rancher' },
]

function statusLabel(s: string) { return ({ healthy: '健康', degraded: '降级', provisioning: '部署中', error: '异常' } as Record<string, string>)[s] || s }
function formatTime(t: string) { if (!t || t.startsWith('0001')) return '-'; return new Date(t).toLocaleString() }
function flag(cc: string) { if (!cc || cc.length !== 2) return ''; return String.fromCodePoint(0x1F1E6 + cc.charCodeAt(0) - 65) + String.fromCodePoint(0x1F1E6 + cc.charCodeAt(1) - 65) }
function getNodeName(id: string) { return nodes.value.find(n => n.id === id)?.name || id.substring(0, 8) }

async function loadData() {
  loading.value = true
  try {
    const [c, n] = await Promise.all([clustersApi.listClusters(), listNodes()])
    clusters.value = c
    nodes.value = n
  } finally { loading.value = false }
}

async function runPreflight(nodeId: string) {
  preflightLoading.value = true
  preflightResult.value = null
  try {
    preflightResult.value = await clustersApi.preflightCheck(nodeId, false)
    if (!preflightResult.value.all_passed) {
      const fixable = preflightResult.value.checks.some(c => !c.passed && (c.name.includes('Swap') || c.name.includes('内核') || c.name.includes('网络')))
      if (fixable) {
        ElMessage.warning('部分检查未通过，正在自动修复...')
        preflightResult.value = await clustersApi.preflightCheck(nodeId, true)
        ElMessage.success('自动修复完成')
      }
    } else {
      ElMessage.success('前置检查全部通过')
    }
  } catch {} finally { preflightLoading.value = false }
}

async function handleCreate() {
  creating.value = true
  try {
    await clustersApi.createCluster({
      name: form.value.name,
      version: form.value.version,
      server_node: form.value.serverNode,
      agent_nodes: form.value.agentNodes,
      config: {
        cni: form.value.cni,
        proxy: form.value.proxy || undefined,
        etcd_snapshots: form.value.etcdSnapshots,
      },
    })
    ElMessage.success('集群创建任务已启动，安装过程请查看集群详情')
    showWizard.value = false
    form.value = { name: '', version: '', serverNode: '', agentNodes: [], cni: 'canal', proxy: '', etcdSnapshots: false }
    loadData()
  } catch {} finally { creating.value = false }
}

async function viewCluster(c: clustersApi.Cluster) {
  currentCluster.value = c
  activeTab.value = 'overview'
  editingConfig.value = c.config || ''
  clusterStatus.value = []
  detailVisible.value = true
  loadClusterStatus(c.id)
  loadRancherStatus(c.id)
}

async function loadClusterStatus(id: string) {
  try { clusterStatus.value = await clustersApi.getClusterStatus(id) } catch {}
}

async function loadPods(id: string) {
  try { podsOutput.value = (await clustersApi.getClusterPods(id)).join('\n') } catch {}
}

async function loadHelm(id: string) {
  try { helmOutput.value = await clustersApi.helmList(id) } catch {}
}

async function handleSaveConfig() {
  if (!currentCluster.value) return
  try {
    await clustersApi.updateClusterConfig(currentCluster.value.id, editingConfig.value)
    ElMessage.success('配置已保存')
  } catch {}
}

async function handleHelmInstall() {
  if (!currentCluster.value || !helmChart.value) return
  try {
    await clustersApi.helmInstall(currentCluster.value.id, {
      chart_name: helmChart.value,
      repo_url: helmRepo.value || undefined,
      namespace: helmNs.value || undefined,
    })
    ElMessage.success('Helm 安装已启动')
    helmChart.value = ''
    helmRepo.value = ''
    setTimeout(() => loadHelm(currentCluster.value!.id), 5000)
  } catch {}
}

async function loadRancherStatus(id: string) {
  try { rancherInfo.value = await clustersApi.rancherStatus(id) } catch {}
}

async function handleInstallRancher() {
  if (!currentCluster.value || !rancherHost.value) return
  rancherMsg.value = '正在安装 Rancher，预计需要 3-5 分钟...'
  try {
    await clustersApi.installRancher(currentCluster.value.id, rancherHost.value)
    ElMessage.success('Rancher 安装任务已启动')
  } catch { rancherMsg.value = '安装失败' }
}

async function handleDelete(c: clustersApi.Cluster) {
  await ElMessageBox.confirm(`确定卸载集群 ${c.name}？所有节点上的 RKE2 将被清除。`, '', { type: 'warning', confirmButtonText: '卸载', cancelButtonText: '取消' })
  try {
    await clustersApi.deleteCluster(c.id)
    ElMessage.success('集群卸载中')
    detailVisible.value = false
    loadData()
  } catch {}
}

onMounted(() => loadData())
</script>

<style scoped lang="scss">
.btn-primary { display: flex; align-items: center; gap: 6px; padding: 8px 16px; border: none; border-radius: 8px; background: var(--accent); color: #fff; font-size: 13px; font-weight: 600; cursor: pointer; font-family: inherit; &:hover { background: var(--accent-dark); } }

.cluster-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 12px; }
.cluster-card { padding: 18px; cursor: pointer; transition: all 0.2s; &:hover { border-color: var(--accent); transform: translateY(-1px); }
  .cc-header { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
  .cc-icon { color: var(--accent); }
  .cc-info { flex: 1; .cc-name { display: block; font-size: 15px; font-weight: 600; } .cc-version { font-size: 11px; color: var(--dg-text-faint); } }
  .cc-status { font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 4px;
    &.healthy { background: var(--dg-success-bg); color: var(--dg-success); }
    &.provisioning { background: var(--dg-warning-bg); color: var(--dg-warning); }
    &.error, &.degraded { background: var(--dg-danger-bg); color: var(--dg-danger); } }
  .cc-body { .cc-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; span { color: var(--dg-text-faint); } strong { color: var(--dg-text); } .mono { font-family: 'JetBrains Mono', monospace; } } }
}
.empty-card { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; min-height: 200px; border: 2px dashed var(--dg-border); border-radius: 12px; cursor: pointer; color: var(--dg-text-faint); transition: all 0.2s;
  &:hover { border-color: var(--accent); color: var(--accent); } span { font-size: 13px; } }

/* Wizard */
.wiz-step { margin-bottom: 20px; }
.step-title { display: flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 600; margin-bottom: 10px; .step-num { width: 18px; height: 18px; border-radius: 5px; background: var(--accent); color: #fff; display: flex; align-items: center; justify-content: center; font-size: 10px; } }
.wiz-nodes { display: flex; flex-direction: column; gap: 4px; max-height: 160px; overflow-y: auto; }
.wiz-node { display: flex; align-items: center; gap: 8px; padding: 7px 10px; border-radius: 8px; cursor: pointer; transition: background 0.15s; font-size: 12px;
  &:hover { background: var(--dg-bg-3); }
  &.checked { background: var(--accent-tint); }
  input { accent-color: var(--accent); width: 14px; height: 14px; }
  .wn-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; &.online { background: var(--dg-success); } }
  .wn-name { font-weight: 500; }
  .wn-host { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: var(--dg-text-faint); margin-left: auto; }
  .wn-flag { font-size: 13px; } }
.wiz-empty { padding: 16px; text-align: center; color: var(--dg-text-faint); font-size: 12px; }
.preflight-btn { margin-top: 8px; padding: 5px 14px; border-radius: 7px; border: 1px solid var(--accent); background: transparent; color: var(--accent); font-size: 12px; cursor: pointer; font-family: inherit; &:hover { background: var(--accent-tint); } &:disabled { opacity: 0.5; } }
.preflight-result { margin-top: 10px; border: 1px solid var(--dg-border); border-radius: 8px; overflow: hidden; }
.pf-item { display: flex; align-items: center; gap: 8px; padding: 6px 10px; font-size: 11px; border-bottom: 1px solid var(--dg-table-border);
  &:last-child { border-bottom: none; }
  &.pass { color: var(--dg-success); }
  &.fail { color: var(--dg-danger); }
  &.fixed { color: var(--dg-warning); }
  .pf-icon { font-weight: 700; width: 14px; text-align: center; }
  .pf-name { font-weight: 600; min-width: 100px; color: var(--dg-text); }
  .pf-value { font-family: 'JetBrains Mono', monospace; }
  .pf-required { color: var(--dg-text-faint); margin-left: auto; }
  .pf-warning { color: var(--dg-warning); font-size: 10px; } }
.wiz-config { display: flex; flex-direction: column; gap: 10px; }
.cfg-row { display: flex; align-items: center; gap: 10px;
  label { font-size: 12px; color: var(--dg-text-dim); min-width: 100px; }
  input, select { flex: 1; height: 32px; padding: 0 10px; border: 1px solid var(--dg-border); border-radius: 6px; background: var(--dg-input-bg); color: var(--dg-text); font-size: 12px; font-family: inherit; outline: none; &:focus { border-color: var(--accent); } }
  .toggle { display: flex; align-items: center; cursor: pointer; input { width: 16px; height: 16px; accent-color: var(--accent); } } }

/* Detail */
.cluster-detail { display: flex; flex-direction: column; gap: 16px; }
.detail-tabs { display: flex; gap: 2px; border-bottom: 1px solid var(--dg-border); }
.detail-tab { padding: 8px 16px; font-size: 12px; font-weight: 500; color: var(--dg-text-faint); cursor: pointer; border: none; background: transparent; border-bottom: 2px solid transparent; margin-bottom: -1px; font-family: inherit;
  &:hover { color: var(--dg-text-dim); }
  &.active { color: var(--accent); border-bottom-color: var(--accent); } }
.detail-content { min-height: 300px; }
.detail-info { display: flex; flex-wrap: wrap; gap: 20px; margin-bottom: 16px;
  .di-item { display: flex; flex-direction: column; span { font-size: 11px; color: var(--dg-text-faint); } strong { font-size: 13px; } } }
.detail-actions { display: flex; gap: 8px; margin-bottom: 16px; }
.action-btn { padding: 5px 14px; border-radius: 7px; border: 1px solid var(--dg-border); background: var(--dg-surface-solid); color: var(--dg-text-dim); font-size: 12px; cursor: pointer; font-family: inherit; &:hover { border-color: var(--accent); color: var(--accent); }
  &.action-primary { background: var(--accent); border-color: var(--accent); color: #fff; &:hover { background: var(--accent-dark); } }
  &.action-danger { &:hover { border-color: var(--dg-danger); color: var(--dg-danger); } } }
.status-table { display: flex; flex-direction: column; gap: 4px; }
.status-row { display: flex; align-items: center; gap: 10px; padding: 8px 10px; border-radius: 8px; background: var(--dg-bg-3); font-size: 12px;
  .sr-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; background: var(--dg-danger); &.ready { background: var(--dg-success); } }
  .sr-name { font-weight: 600; min-width: 120px; }
  .sr-role { color: var(--dg-text-faint); min-width: 60px; }
  .sr-version { font-family: 'JetBrains Mono', monospace; color: var(--dg-text-faint); margin-left: auto; }
  .sr-state { font-weight: 600; &.ready { color: var(--dg-success); } } }
.config-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; h4 { font-size: 13px; font-weight: 600; } }
.config-editor { width: 100%; padding: 12px; border: 1px solid var(--dg-border); border-radius: 8px; background: var(--dg-input-bg); color: var(--dg-text); font-family: 'JetBrains Mono', monospace; font-size: 12px; line-height: 1.6; outline: none; resize: vertical; &:focus { border-color: var(--accent); } }
.console-output { font-family: 'JetBrains Mono', monospace; font-size: 11px; line-height: 1.5; color: var(--dg-text-dim); white-space: pre-wrap; word-break: break-all; max-height: 350px; overflow-y: auto; margin: 0; padding: 10px; background: var(--dg-bg-3); border-radius: 8px; }
.helm-install-bar { display: flex; gap: 6px; margin-bottom: 12px; }
.helm-input { flex: 1; height: 30px; padding: 0 8px; border: 1px solid var(--dg-border); border-radius: 6px; background: var(--dg-input-bg); color: var(--dg-text); font-size: 12px; font-family: inherit; outline: none;
  &.small { max-width: 100px; }
  &:focus { border-color: var(--accent); } }
.rancher-section { .rancher-status { margin-bottom: 14px; .rs-label { font-size: 12px; color: var(--dg-text-faint); margin-right: 8px; } .rs-installed { color: var(--dg-success); font-weight: 600; } .rs-not-installed { color: var(--dg-text-faint); } }
  .rancher-install { display: flex; gap: 8px; } .rancher-msg { margin-top: 10px; font-size: 12px; color: var(--dg-text-dim); } }
</style>
