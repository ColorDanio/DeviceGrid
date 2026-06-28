<template>
  <div class="docker-page">
    <!-- Left: Node sidebar -->
    <aside class="d-sidebar">
      <div class="d-sidebar-header">
        <h3>Docker 管理</h3>
      </div>
      <div class="d-sidebar-search">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none"><circle cx="11" cy="11" r="8" stroke="currentColor" stroke-width="2"/><path d="M21 21l-4.35-4.35" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
        <input v-model="search" placeholder="搜索节点..." />
      </div>
      <div class="d-node-list">
        <button v-for="n in filteredNodes" :key="n.id" class="d-node" :class="{ online: n.status === 'online', active: selectedNode === n.id }" :disabled="n.status !== 'online'" @click="selectNode(n)">
          <span class="dn-dot" :class="n.status"></span>
          <div class="dn-info"><span class="dn-name">{{ n.name }}</span><span class="dn-host">{{ n.host }}</span></div>
          <span v-if="n.docker_version" class="dn-badge">Docker</span>
        </button>
        <div v-if="filteredNodes.length === 0" class="dn-empty">无匹配节点</div>
      </div>
    </aside>

    <!-- Right: Docker content -->
    <div class="d-main">
      <!-- No node selected -->
      <div v-if="!selectedNode" class="d-welcome">
        <svg viewBox="0 0 24 24" width="48" height="48" fill="none" style="opacity:0.15;margin-bottom:12px"><path d="M22 12c0-5.5-4.5-10-10-10S2 6.5 2 12s4.5 10 10 10 10-4.5 10-10z" stroke="currentColor" stroke-width="1.5"/><path d="M8 14s1.5 2 4 2 4-2 4-2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
        <p class="dw-title">选择左侧节点管理 Docker</p>
        <p class="dw-sub">容器 · 镜像 · Compose · 网络 · 卷</p>
      </div>

      <!-- Node selected -->
      <template v-else>
        <!-- Node info bar -->
        <div class="d-node-bar">
          <div class="dn-bar-left">
            <span class="dn-bar-name">{{ currentNodeName }}</span>
            <span class="dn-bar-host">{{ currentNodeHost }}</span>
            <span class="dn-bar-docker" v-if="dockerInstalled">Docker {{ dockerVersion }}</span>
          </div>
          <button class="dn-bar-install" v-if="!dockerInstalled && selectedNode" @click="installDocker">安装 Docker</button>
        </div>

        <!-- Tabs -->
        <div class="d-tabs">
          <div v-for="t in tabs" :key="t.id" class="d-tab" :class="{ active: activeTab === t.id }" @click="activeTab = t.id">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" v-html="t.svg"></svg>{{ t.name }}
            <span v-if="t.count !== undefined" class="tab-count">{{ t.count }}</span>
          </div>
        </div>

        <!-- Tab content -->
        <div class="d-content" v-loading="loading">
          <!-- Containers -->
          <template v-if="activeTab === 'containers'">
            <div class="d-toolbar">
              <label class="d-toggle"><input type="checkbox" v-model="showAll" @change="loadContainers" /><span>显示已停止</span></label>
              <button class="d-btn" @click="loadContainers"><svg viewBox="0 0 24 24" width="13" height="13" fill="none"><path d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15" stroke="currentColor" stroke-width="2"/></svg>刷新</button>
            </div>
            <div v-if="containers.length === 0" class="d-empty">暂无容器</div>
            <div v-else class="c-list">
              <div v-for="c in containers" :key="c.id" class="c-row" :class="{ stopped: c.state !== 'running' }">
                <span class="c-dot" :class="c.state"></span>
                <div class="c-info">
                  <div class="c-name">{{ c.name }}</div>
                  <div class="c-meta"><span class="mono">{{ c.image }}</span><span class="c-id">{{ c.id.substring(0, 12) }}</span><span>{{ c.status }}</span></div>
                </div>
                <div class="c-ports" v-if="c.ports?.length">
                  <span v-for="(p,i) in c.ports.slice(0,3)" :key="i" class="port-chip">{{ p.host_port }}</span>
                </div>
                <div class="c-btns">
                  <button v-if="c.state === 'running'" class="c-btn" @click.stop="openContainerLogs(c)" title="查看日志"><svg viewBox="0 0 24 24" width="13" height="13" fill="none"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" stroke="currentColor" stroke-width="1.8"/><path d="M14 2v6h6M8 13h8M8 17h8M8 9h2" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg></button>
                  <button v-if="c.state === 'running'" class="c-btn" @click.stop="showStats(c)" title="资源监控"><svg viewBox="0 0 24 24" width="13" height="13" fill="none"><path d="M3 3v18h18" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/><path d="M7 14l4-4 4 3 4-6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg></button>
                  <button v-if="c.state === 'running'" class="c-btn" @click.stop="execContainer(c)" title="进入终端"><svg viewBox="0 0 24 24" width="13" height="13" fill="none"><rect x="2" y="4" width="20" height="16" rx="2" stroke="currentColor" stroke-width="1.8"/><path d="M6 9l3 3-3 3M12 15h4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg></button>
                  <button v-if="c.state === 'running'" class="c-btn" @click.stop="doAction(c.id,'stop')" title="停止"><svg viewBox="0 0 24 24" width="13" height="13" fill="none"><rect x="6" y="6" width="12" height="12" rx="1" fill="currentColor"/></svg></button>
                  <button v-if="c.state === 'running'" class="c-btn" @click.stop="doAction(c.id,'restart')" title="重启"><svg viewBox="0 0 24 24" width="13" height="13" fill="none"><path d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15" stroke="currentColor" stroke-width="1.8"/></svg></button>
                  <button v-if="c.state !== 'running'" class="c-btn c-start" @click.stop="doAction(c.id,'start')" title="启动"><svg viewBox="0 0 24 24" width="13" height="13" fill="none"><path d="M5 3l14 9-14 9V3z" fill="currentColor"/></svg></button>
                  <button class="c-btn c-del" @click.stop="doAction(c.id,'remove')" title="删除"><svg viewBox="0 0 24 24" width="13" height="13" fill="none"><path d="M3 6h18M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2m3 0v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6" stroke="currentColor" stroke-width="1.8"/></svg></button>
                </div>
              </div>
            </div>
          </template>

          <!-- Images -->
          <template v-if="activeTab === 'images'">
            <div class="d-toolbar">
              <button class="d-btn" @click="loadImages">刷新</button>
              <button class="d-btn d-btn-primary" @click="showPull = true">拉取镜像</button>
            </div>
            <div v-if="images.length === 0" class="d-empty">暂无镜像</div>
            <div v-else class="c-list">
              <div v-for="img in images" :key="img.id" class="c-row">
                <div class="c-info" style="flex:1"><div class="c-name">{{ img.tags }}</div><div class="c-meta"><span class="c-id">{{ img.id.substring(0,19) }}</span><span>{{ img.size }}</span><span>{{ img.created }}</span></div></div>
                <div class="c-btns"><button class="c-btn c-del" @click="removeImg(img.id)"><svg viewBox="0 0 24 24" width="13" height="13" fill="none"><path d="M3 6h18M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2m3 0v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6" stroke="currentColor" stroke-width="1.8"/></svg></button></div>
              </div>
            </div>
          </template>

          <!-- Compose -->
          <template v-if="activeTab === 'compose'">
            <div class="d-toolbar">
              <input v-model="composeName" placeholder="项目名称" class="compose-name" />
              <button class="d-btn d-btn-primary" @click="composeUp">部署</button>
              <button class="d-btn" @click="composeDown">停止</button>
            </div>
            <textarea v-model="composeContent" class="compose-editor" spellcheck="false"></textarea>
          </template>

          <!-- Networks -->
          <template v-if="activeTab === 'networks'">
            <button class="d-btn" @click="loadNetworks" style="margin-bottom:12px">刷新</button>
            <div v-if="networks.length === 0" class="d-empty">暂无网络</div>
            <div v-else class="c-list">
              <div v-for="net in networks" :key="net.id" class="c-row"><div class="c-info" style="flex:1"><div class="c-name">{{ net.name }}</div><div class="c-meta"><span class="mono">{{ net.driver }}</span><span>{{ net.scope }}</span></div></div></div>
            </div>
          </template>

          <!-- Volumes -->
          <template v-if="activeTab === 'volumes'">
            <button class="d-btn" @click="loadVolumes" style="margin-bottom:12px">刷新</button>
            <div v-if="volumes.length === 0" class="d-empty">暂无存储卷</div>
            <div v-else class="c-list">
              <div v-for="vol in volumes" :key="vol.name" class="c-row"><div class="c-info" style="flex:1"><div class="c-name">{{ vol.name }}</div><div class="c-meta"><span class="mono">{{ vol.driver }}</span><span>{{ vol.mountpoint }}</span></div></div></div>
            </div>
          </template>
        </div>
      </template>
    </div>

    <!-- Container Terminal -->
    <el-dialog v-model="execVisible" :title="`容器终端: ${execName}`" width="90%" top="5vh" destroy-on-close @opened="initExecTerm" @closed="closeExecTerm">
      <div ref="execTermEl" class="exec-term"></div>
    </el-dialog>

    <!-- Container Logs -->
    <el-dialog v-model="logsVisible" :title="`容器日志: ${logsName}`" width="85%" top="5vh" destroy-on-close @opened="initLogsWs" @closed="closeLogsWs">
      <div ref="logsEl" class="logs-viewer"></div>
    </el-dialog>

    <!-- Container Stats Dialog -->
    <el-dialog v-model="statsVisible" :title="`容器监控: ${statsName}`" width="480px">
      <div v-if="statsData" class="stats-grid">
        <div class="stat-item"><span class="si-label">CPU</span><span class="si-value">{{ statsData.cpu || '—' }}</span></div>
        <div class="stat-item"><span class="si-label">内存</span><span class="si-value">{{ statsData.mem || '—' }}</span></div>
        <div class="stat-item"><span class="si-label">内存%</span><span class="si-value">{{ statsData.mem_pct || '—' }}</span></div>
        <div class="stat-item"><span class="si-label">网络 I/O</span><span class="si-value">{{ statsData.net_io || '—' }}</span></div>
        <div class="stat-item"><span class="si-label">磁盘 I/O</span><span class="si-value">{{ statsData.block_io || '—' }}</span></div>
        <div class="stat-item"><span class="si-label">PIDs</span><span class="si-value">{{ statsData.pids || '—' }}</span></div>
      </div>
      <div v-else class="loading-text">采集中...</div>
      <template #footer>
        <button class="d-btn" @click="refreshStats" style="margin-right:auto">刷新</button>
        <el-button @click="statsVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- Pull Image -->
    <el-dialog v-model="showPull" title="拉取镜像" width="400px">
      <el-input v-model="pullName" placeholder="nginx:latest" />
      <template #footer><el-button @click="showPull = false">取消</el-button><el-button type="primary" :loading="pulling" @click="doPull">拉取</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Terminal } from '@xterm/xterm'
import { createTerminal } from '@/utils/terminal'
import { decodeBase64 } from '@/utils/codec'
import { listNodes, type Node } from '@/api/nodes'
import { listContainers, containerAction, listImages, pullImage, removeImage, listNetworks, listVolumes, getDockerInfo, getContainerStats, type ContainerInfo, type ImageInfo, type NetworkInfo, type VolumeInfo } from '@/api/docker'
import client from '@/api/client'

const nodes = ref<Node[]>([])
const search = ref('')
const selectedNode = ref('')
const activeTab = ref('containers')
const loading = ref(false)
const showAll = ref(false)
const dockerInstalled = ref(false)
const dockerVersion = ref('')

const containers = ref<ContainerInfo[]>([])
const images = ref<ImageInfo[]>([])
const networks = ref<NetworkInfo[]>([])
const volumes = ref<VolumeInfo[]>([])

const composeContent = ref(`services:\n  web:\n    image: nginx:latest\n    ports:\n      - "80:80"`)
const composeName = ref('')
const showPull = ref(false)
const pullName = ref('')
const pulling = ref(false)

const filteredNodes = computed(() => {
  if (!search.value) return nodes.value
  const q = search.value.toLowerCase()
  return nodes.value.filter(n => n.name.toLowerCase().includes(q) || n.host.includes(q))
})

const currentNodeName = computed(() => nodes.value.find(n => n.id === selectedNode.value)?.name || '')
const currentNodeHost = computed(() => nodes.value.find(n => n.id === selectedNode.value)?.host || '')

const tabs = computed(() => [
  { id: 'containers', name: '容器', svg: '<rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" stroke-width="1.8"/>', count: containers.value.length },
  { id: 'images', name: '镜像', svg: '<rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" stroke-width="1.8"/><path d="M3 9h18M3 15h18" stroke="currentColor" stroke-width="1.8"/>', count: images.value.length },
  { id: 'compose', name: 'Compose', svg: '<path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" stroke="currentColor" stroke-width="1.8"/><path d="M14 2v6h6" stroke="currentColor" stroke-width="1.8"/>' },
  { id: 'networks', name: '网络', svg: '<circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="1.8"/><path d="M2 12h20" stroke="currentColor" stroke-width="1.8"/>', count: networks.value.length },
  { id: 'volumes', name: '卷', svg: '<ellipse cx="12" cy="5" rx="9" ry="3" stroke="currentColor" stroke-width="1.8"/><path d="M21 5v14a9 3 0 01-18 0V5" stroke="currentColor" stroke-width="1.8"/>', count: volumes.value.length },
])

function selectNode(n: Node) {
  if (n.status !== 'online') return
  selectedNode.value = n.id
  checkDocker()
  loadData()
}

async function checkDocker() {
  if (!selectedNode.value) return
  try {
    const info = await getDockerInfo(selectedNode.value)
    dockerInstalled.value = info.installed
    dockerVersion.value = info.version
  } catch { dockerInstalled.value = false }
}

async function installDocker() {
  try {
    await client.post(`/nodes/${selectedNode.value}/docker/install`, {})
    ElMessage.success('Docker 安装已启动')
  } catch {}
}

async function loadContainers() { try { containers.value = await listContainers(selectedNode.value, showAll.value) } catch { containers.value = [] } }
async function loadImages() { try { images.value = await listImages(selectedNode.value) } catch { images.value = [] } }
async function loadNetworks() { try { networks.value = await listNetworks(selectedNode.value) } catch { networks.value = [] } }
async function loadVolumes() { try { volumes.value = await listVolumes(selectedNode.value) } catch { volumes.value = [] } }

async function loadData() {
  if (!selectedNode.value) return
  loading.value = true
  try {
    if (activeTab.value === 'containers') await loadContainers()
    else if (activeTab.value === 'images') await loadImages()
    else if (activeTab.value === 'networks') await loadNetworks()
    else if (activeTab.value === 'volumes') await loadVolumes()
  } finally { loading.value = false }
}

async function doAction(id: string, action: string) {
  if (action === 'remove') await ElMessageBox.confirm('确定删除？', '', { type: 'warning' })
  try { await containerAction(selectedNode.value, id, action); ElMessage.success('操作成功'); setTimeout(loadContainers, 500) } catch {}
}
async function removeImg(id: string) {
  await ElMessageBox.confirm('确定删除？', '', { type: 'warning' })
  await removeImage(selectedNode.value, id); ElMessage.success('已删除'); loadImages()
}
async function doPull() {
  if (!pullName.value) return
  pulling.value = true
  try { await pullImage(selectedNode.value, pullName.value); ElMessage.success('已开始拉取'); showPull.value = false; pullName.value = '' } finally { pulling.value = false }
}
async function composeUp() {
  if (!composeName.value) { ElMessage.warning('请输入项目名称'); return }
  try { await client.post(`/nodes/${selectedNode.value}/docker/compose`, { name: composeName.value, config: composeContent.value }); ElMessage.success('部署已启动') } catch {}
}
async function composeDown() {
  if (!composeName.value) { ElMessage.warning('请输入项目名称'); return }
  try { await client.delete(`/nodes/${selectedNode.value}/docker/compose`, { data: { name: composeName.value } }); ElMessage.success('停止已执行') } catch {}
}

// Container exec terminal
const execVisible = ref(false); const execName = ref(''); const execTermEl = ref<HTMLElement>()
let execTerm: Terminal | null = null; let execWs: WebSocket | null = null; let execCid = ''; let execFit: any = null
function execContainer(c: ContainerInfo) { execCid = c.id; execName.value = c.name; execVisible.value = true }
const statsVisible = ref(false); const statsName = ref(''); const statsData = ref<Record<string,string>|null>(null); let statsCid = ''
async function showStats(c: ContainerInfo) { statsCid = c.id; statsName.value = c.name; statsData.value = null; statsVisible.value = true; refreshStats() }
async function refreshStats() { if (!selectedNode.value || !statsCid) return; try { statsData.value = await getContainerStats(selectedNode.value, statsCid) } catch { statsData.value = null } }
function initExecTerm() {
  if (!execTermEl.value) return
  const { term, fit } = createTerminal(execTermEl.value)
  execTerm = term
  execFit = fit
  // Fit after dialog is fully rendered
  setTimeout(() => { try { fit.fit() } catch {} }, 100)

  const token = sessionStorage.getItem('dg_token') || ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  execWs = new WebSocket(`${proto}://${location.host}/ws/container/${selectedNode.value}/${execCid}`)

  execWs.onopen = () => {
    // Send auth token as first message
    execWs!.send(JSON.stringify({ token: token }))
    if (execWs?.readyState === WebSocket.OPEN) {
      execWs.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }
  }
  execWs.onmessage = (ev) => {
    try {
      const m = JSON.parse(ev.data)
      if (m.type === 'output') execTerm!.write(decodeBase64(m.data))
      else if (m.type === 'error') execTerm!.writeln(`\x1b[31m${m.data}\x1b[0m`)
    } catch {}
  }
  execWs.onerror = () => { execTerm?.writeln('\x1b[31m● 连接错误\x1b[0m') }
  execWs.onclose = () => { execTerm?.writeln('\x1b[33m\r\n● 连接已关闭\x1b[0m') }

  execTerm.onData((d) => {
    if (execWs?.readyState === WebSocket.OPEN) {
      execWs.send(JSON.stringify({ type: 'input', data: d }))
    }
  })
  execTerm.onResize(({ cols, rows }) => {
    if (execWs?.readyState === WebSocket.OPEN) {
      execWs.send(JSON.stringify({ type: 'resize', cols, rows }))
    }
  })
}
function closeExecTerm() { if (execWs) { execWs.close(); execWs = null } if (execTerm) { execTerm.dispose(); execTerm = null } }

// Container logs viewer
const logsVisible = ref(false); const logsName = ref(''); const logsEl = ref<HTMLElement>()
let logsTerm: Terminal | null = null; let logsWs: WebSocket | null = null; let logsCid = ''
function openContainerLogs(c: ContainerInfo) { logsCid = c.id; logsName.value = c.name; logsVisible.value = true }
function initLogsWs() {
  if (!logsEl.value) return
  const { term } = createTerminal(logsEl.value)
  logsTerm = term
  term.writeln('\x1b[90m● 连接日志流...\x1b[0m')
  const token = sessionStorage.getItem('dg_token') || ''; const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  logsWs = new WebSocket(`${proto}://${location.host}/ws/logs/${selectedNode.value}/${logsCid}`)
  logsWs.onopen = () => { logsWs!.send(JSON.stringify({ token: token })) }
  logsWs.onmessage = (ev) => { try { const m = JSON.parse(ev.data); if (m.type === 'output') term.write(decodeBase64(m.data)); else if (m.type === 'done') term.writeln('\x1b[33m\r\n● 日志流结束\x1b[0m') } catch {} }
  logsWs.onclose = () => term.writeln('\x1b[33m\r\n● 连接断开\x1b[0m')
}
function closeLogsWs() { if (logsWs) { logsWs.close(); logsWs = null } if (logsTerm) { logsTerm.dispose(); logsTerm = null } }

let pollTimer: ReturnType<typeof setInterval> | null = null
watch(activeTab, () => loadData())
onMounted(async () => {
  nodes.value = await listNodes()
  // Auto-select first online node with Docker
  const dockerNode = nodes.value.find(n => n.status === 'online' && n.docker_version)
  if (dockerNode) { selectedNode.value = dockerNode.id; checkDocker(); loadData() }
  pollTimer = setInterval(() => { if (activeTab.value === 'containers' && selectedNode.value) loadContainers() }, 5000)
})
onBeforeUnmount(() => { if (pollTimer) clearInterval(pollTimer); closeExecTerm(); closeLogsWs() })
</script>

<style scoped lang="scss">
.docker-page { display: flex; height: calc(100vh - var(--dg-header-h)); overflow: hidden; }

/* Sidebar */
.d-sidebar { width: 240px; flex-shrink: 0; background: var(--dg-bg-2); border-right: 1px solid var(--dg-border); display: flex; flex-direction: column; }
.d-sidebar-header { padding: 14px 16px 10px; h3 { font-size: 14px; font-weight: 700; color: var(--dg-text); } }
.d-sidebar-search { display: flex; align-items: center; gap: 8px; padding: 8px 12px; border-bottom: 1px solid var(--dg-border);
  svg { color: var(--dg-text-faint); }
  input { flex: 1; border: none; outline: none; background: transparent; color: var(--dg-text); font-size: 13px; font-family: inherit; &::placeholder { color: var(--dg-text-faint); } } }
.d-node-list { flex: 1; overflow-y: auto; padding: 6px; }
.dn-empty { padding: 20px; text-align: center; color: var(--dg-text-faint); font-size: 12px; }
.d-node { display: flex; align-items: center; gap: 8px; width: 100%; padding: 8px 10px; border-radius: 8px; border: none; background: transparent; color: var(--dg-text-dim); font-size: 13px; cursor: pointer; transition: background 0.15s; font-family: inherit; text-align: left; margin-bottom: 2px;
  &:hover { background: var(--dg-bg-3); color: var(--dg-text); }
  &:not(.online) { opacity: 0.35; cursor: not-allowed; }
  &.active { background: var(--accent-tint); .dn-name { color: var(--accent); } }
  .dn-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0;
    &.online { background: var(--dg-success); box-shadow: 0 0 4px var(--dg-success); }
    &.offline { background: var(--dg-danger); } &.untrusted { background: var(--dg-warning); } }
  .dn-info { flex: 1; min-width: 0; .dn-name { display: block; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; } .dn-host { display: block; font-size: 11px; color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace; } }
  .dn-badge { font-size: 9px; padding: 1px 5px; border-radius: 3px; background: var(--dg-warning-bg); color: var(--dg-warning); } }

/* Main */
.d-main { flex: 1; display: flex; flex-direction: column; overflow: hidden; background: var(--dg-bg); min-width: 0; }
.d-welcome { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; .dw-title { font-size: 15px; font-weight: 600; color: var(--dg-text-dim); margin-bottom: 4px; } .dw-sub { font-size: 12px; color: var(--dg-text-faint); } }

/* Node bar */
.d-node-bar { display: flex; align-items: center; justify-content: space-between; padding: 10px 20px; border-bottom: 1px solid var(--dg-border); background: var(--dg-surface); flex-shrink: 0;
  .dn-bar-left { display: flex; align-items: center; gap: 12px; .dn-bar-name { font-size: 14px; font-weight: 600; } .dn-bar-host { font-size: 12px; color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace; } .dn-bar-docker { font-size: 10px; padding: 2px 7px; border-radius: 4px; background: var(--dg-warning-bg); color: var(--dg-warning); font-weight: 500; } }
  .dn-bar-install { padding: 5px 14px; border-radius: 7px; border: none; background: var(--accent); color: #fff; font-size: 12px; font-weight: 600; cursor: pointer; font-family: inherit; } }

/* Tabs */
.d-tabs { display: flex; gap: 1px; padding: 0 20px; border-bottom: 1px solid var(--dg-border); flex-shrink: 0; }
.d-tab { display: flex; align-items: center; gap: 5px; padding: 9px 14px; font-size: 12px; font-weight: 500; color: var(--dg-text-faint); cursor: pointer; border-bottom: 2px solid transparent; margin-bottom: -1px; transition: all 0.15s;
  &:hover { color: var(--dg-text-dim); }
  &.active { color: var(--accent); border-bottom-color: var(--accent); }
  .tab-count { font-size: 10px; padding: 0 5px; border-radius: 8px; background: var(--dg-bg-3); color: var(--dg-text-faint); min-width: 18px; text-align: center; } }

/* Content */
.d-content { flex: 1; overflow-y: auto; padding: 16px 20px; }
.d-toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; }
.d-toggle { display: flex; align-items: center; gap: 5px; font-size: 12px; color: var(--dg-text-dim); cursor: pointer; input { accent-color: var(--accent); width: 14px; height: 14px; } }
.d-btn { display: flex; align-items: center; gap: 5px; padding: 5px 12px; border-radius: 7px; border: 1px solid var(--dg-border); background: var(--dg-surface-solid); color: var(--dg-text-dim); font-size: 12px; cursor: pointer; font-family: inherit; &:hover { border-color: var(--accent); color: var(--accent); }
  &.d-btn-primary { background: var(--accent); border-color: var(--accent); color: #fff; &:hover { background: var(--accent-dark); } } }
.d-empty { padding: 60px 0; text-align: center; color: var(--dg-text-faint); font-size: 13px; }

/* Container list */
.c-list { display: flex; flex-direction: column; gap: 6px; }
.c-row { display: flex; align-items: center; gap: 12px; padding: 10px 14px; border-radius: 10px; background: var(--dg-surface); border: 1px solid var(--dg-border); transition: all 0.15s;
  &:hover { border-color: var(--accent); }
  &.stopped { opacity: 0.5; }
  .c-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; &.running { background: var(--dg-success); box-shadow: 0 0 6px var(--dg-success); } &.exited { background: var(--dg-text-faint); } &.paused { background: var(--dg-warning); } }
  .c-info { flex: 1; min-width: 0; .c-name { font-size: 13px; font-weight: 600; color: var(--dg-text); } .c-meta { display: flex; gap: 10px; margin-top: 2px; font-size: 11px; color: var(--dg-text-faint); .mono { font-family: 'JetBrains Mono', monospace; } .c-id { font-family: 'JetBrains Mono', monospace; opacity: 0.7; } } }
  .c-ports { display: flex; gap: 4px; .port-chip { font-size: 10px; padding: 2px 7px; border-radius: 4px; background: var(--accent-tint); color: var(--accent); font-family: 'JetBrains Mono', monospace; } }
  .c-btns { display: flex; gap: 4px; }
  .c-btn { width: 28px; height: 28px; border-radius: 7px; border: 1px solid var(--dg-border); background: var(--dg-surface-solid); color: var(--dg-text-dim); cursor: pointer; display: flex; align-items: center; justify-content: center; transition: all 0.15s;
    &:hover { border-color: var(--accent); color: var(--accent); }
    &.c-start:hover { border-color: var(--dg-success); color: var(--dg-success); }
    &.c-del:hover { border-color: var(--dg-danger); color: var(--dg-danger); } } }

/* Compose */
.compose-name { height: 30px; padding: 0 10px; border: 1px solid var(--dg-border); border-radius: 7px; background: var(--dg-input-bg); color: var(--dg-text); font-size: 12px; outline: none; font-family: inherit; &:focus { border-color: var(--accent); } }
.compose-editor { width: 100%; min-height: 300px; padding: 14px; border: 1px solid var(--dg-border); border-radius: 10px; font-family: 'JetBrains Mono', monospace; font-size: 13px; line-height: 1.6; outline: none; resize: vertical; background: var(--dg-input-bg); color: var(--dg-text); &:focus { border-color: var(--accent); } }

/* Terminal / Logs */
.exec-term, .logs-viewer { height: 60vh; background: #0e0e1c; border-radius: 8px; padding: 8px; overflow: hidden; }
:deep(.xterm) { padding: 8px; }
:deep(.xterm-viewport) { background-color: transparent !important; }
</style>
