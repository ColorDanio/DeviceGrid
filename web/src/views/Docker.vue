<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title-group"><h2>Docker 容器管理</h2><p class="page-subtitle">容器生命周期 · 镜像 · Compose · 网络 · 卷</p></div>
      <NodeSelector v-model="selectedNode" />
    </div>

    <div class="tab-bar">
      <div v-for="t in tabs" :key="t.id" class="tab" :class="{ active: activeTab === t.id }" @click="activeTab = t.id"><svg viewBox="0 0 24 24" width="15" height="15" fill="none" v-html="t.svg"></svg>{{ t.name }}</div>
    </div>

    <div class="dg-card content-panel" v-loading="loading">
      <el-empty v-if="!selectedNode" description="选择一个节点开始" />

      <!-- Containers -->
      <template v-else-if="activeTab === 'containers'">
        <div class="panel-toolbar">
          <button class="btn-sm" @click="loadContainers"><svg viewBox="0 0 24 24" width="13" height="13" fill="none"><path d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15" stroke="currentColor" stroke-width="2"/></svg>刷新</button>
          <label class="toggle-line"><input type="checkbox" v-model="showAll" @change="loadContainers" /><span>显示已停止</span></label>
        </div>
        <div v-if="containers.length === 0" class="no-data">暂无容器</div>
        <div v-else class="c-list">
          <div v-for="c in containers" :key="c.id" class="c-row" :class="{ stopped: c.state !== 'running' }">
            <span class="c-dot" :class="c.state"></span>
            <div class="c-info"><div class="c-name">{{ c.name }}</div><div class="c-meta"><span class="mono">{{ c.image }}</span><span class="c-id">{{ c.id.substring(0, 12) }}</span><span>{{ c.status }}</span></div></div>
            <div class="c-ports" v-if="c.ports?.length"><span v-for="(p,i) in c.ports.slice(0,3)" :key="i" class="port-chip">{{ p.host_port }}</span></div>
            <div class="c-btns">
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
      <template v-else-if="activeTab === 'images'">
        <div class="panel-toolbar"><button class="btn-sm" @click="loadImages">刷新</button><button class="btn-sm btn-primary" @click="showPull = true">拉取镜像</button></div>
        <div v-if="images.length === 0" class="no-data">暂无镜像</div>
        <div v-else class="c-list"><div v-for="img in images" :key="img.id" class="c-row"><div class="c-info" style="flex:1"><div class="c-name">{{ img.tags }}</div><div class="c-meta"><span class="c-id">{{ img.id.substring(0,19) }}</span><span>{{ img.size }}</span></div></div><div class="c-btns"><button class="c-btn c-del" @click="removeImg(img.id)"><svg viewBox="0 0 24 24" width="13" height="13" fill="none"><path d="M3 6h18M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2m3 0v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6" stroke="currentColor" stroke-width="1.8"/></svg></button></div></div></div>
      </template>

      <!-- Compose -->
      <template v-else-if="activeTab === 'compose'"><div class="compose"><div class="c-toolbar"><button class="btn-sm btn-primary">部署</button></div><textarea class="c-editor" v-model="composeContent" spellcheck="false"></textarea></div></template>

      <!-- Networks / Volumes -->
      <template v-else-if="activeTab === 'networks'">
        <div v-if="networks.length === 0" class="no-data">暂无网络</div>
        <div v-else class="c-list"><div v-for="net in networks" :key="net.id" class="c-row"><div class="c-info" style="flex:1"><div class="c-name">{{ net.name }}</div><div class="c-meta"><span class="mono">{{ net.driver }}</span><span>{{ net.scope }}</span></div></div></div></div>
      </template>
      <template v-else-if="activeTab === 'volumes'">
        <div v-if="volumes.length === 0" class="no-data">暂无存储卷</div>
        <div v-else class="c-list"><div v-for="vol in volumes" :key="vol.name" class="c-row"><div class="c-info" style="flex:1"><div class="c-name">{{ vol.name }}</div><div class="c-meta"><span class="mono">{{ vol.driver }}</span><span>{{ vol.mountpoint }}</span></div></div></div></div>
      </template>
    </div>

    <!-- Container Terminal -->
    <el-dialog v-model="execVisible" :title="`容器终端: ${execName}`" width="90%" top="5vh" destroy-on-close @opened="initExecTerm" @closed="closeExecTerm">
      <div ref="execTermEl" class="exec-term"></div>
    </el-dialog>

    <!-- Pull Image -->
    <el-dialog v-model="showPull" title="拉取镜像" width="400px"><el-input v-model="pullName" placeholder="nginx:latest" /><template #footer><el-button @click="showPull = false">取消</el-button><el-button type="primary" :loading="pulling" @click="doPull">拉取</el-button></template></el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Terminal } from '@xterm/xterm'
import { createTerminal } from '@/utils/terminal'
import { decodeBase64 } from '@/utils/codec'
import NodeSelector from '@/components/NodeSelector.vue'
import { listContainers, containerAction, listImages, pullImage, removeImage, listNetworks, listVolumes, type ContainerInfo, type ImageInfo, type NetworkInfo, type VolumeInfo } from '@/api/docker'

const selectedNode = ref(''); const activeTab = ref('containers'); const loading = ref(false); const showAll = ref(false)
const containers = ref<ContainerInfo[]>([]); const images = ref<ImageInfo[]>([]); const networks = ref<NetworkInfo[]>([]); const volumes = ref<VolumeInfo[]>([])
const composeContent = ref(`services:\n  web:\n    image: nginx:latest\n    ports:\n      - "80:80"`)
const showPull = ref(false); const pullName = ref(''); const pulling = ref(false)
const tabs = [
  { id: 'containers', name: '容器', svg: '<rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" stroke-width="1.8"/>' },
  { id: 'images', name: '镜像', svg: '<rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" stroke-width="1.8"/><path d="M3 9h18M3 15h18" stroke="currentColor" stroke-width="1.8"/>' },
  { id: 'compose', name: 'Compose', svg: '<path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" stroke="currentColor" stroke-width="1.8"/><path d="M14 2v6h6" stroke="currentColor" stroke-width="1.8"/>' },
  { id: 'networks', name: '网络', svg: '<circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="1.8"/><path d="M2 12h20" stroke="currentColor" stroke-width="1.8"/>' },
  { id: 'volumes', name: '卷', svg: '<ellipse cx="12" cy="5" rx="9" ry="3" stroke="currentColor" stroke-width="1.8"/><path d="M21 5v14a9 3 0 01-18 0V5" stroke="currentColor" stroke-width="1.8"/>' },
]

async function loadContainers() { containers.value = await listContainers(selectedNode.value, showAll.value) }
async function loadImages() { images.value = await listImages(selectedNode.value) }
async function loadNetworks() { networks.value = await listNetworks(selectedNode.value) }
async function loadVolumes() { volumes.value = await listVolumes(selectedNode.value) }
async function loadData() { if (!selectedNode.value) return; loading.value = true; try { if (activeTab.value === 'containers') await loadContainers(); else if (activeTab.value === 'images') await loadImages(); else if (activeTab.value === 'networks') await loadNetworks(); else if (activeTab.value === 'volumes') await loadVolumes() } finally { loading.value = false } }
async function doAction(id: string, action: string) { if (action === 'remove') await ElMessageBox.confirm('确定删除？', '', { type: 'warning' }); try { await containerAction(selectedNode.value, id, action); ElMessage.success(`操作成功`); setTimeout(loadContainers, 500) } catch {} }
async function removeImg(id: string) { await ElMessageBox.confirm('确定删除？', '', { type: 'warning' }); await removeImage(selectedNode.value, id); ElMessage.success('已删除'); loadImages() }
async function doPull() { if (!pullName.value) return; pulling.value = true; try { await pullImage(selectedNode.value, pullName.value); ElMessage.success('已开始拉取'); showPull.value = false; pullName.value = '' } catch {} finally { pulling.value = false } }

const execVisible = ref(false); const execName = ref(''); const execTermEl = ref<HTMLElement>()
let execTerm: Terminal | null = null; let execWs: WebSocket | null = null; let execCid = ''
let execFit: any = null

function execContainer(c: ContainerInfo) { execCid = c.id; execName.value = c.name; execVisible.value = true }

function initExecTerm() { if (!execTermEl.value) return
  const result = createTerminal(execTermEl.value)
  execTerm = result.term
  execFit = result.fit
  const token = localStorage.getItem('dg_token') || ''; const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  execWs = new WebSocket(`${proto}://${location.host}/ws/container/${selectedNode.value}/${execCid}?token=${token}`)
  execWs.onopen = () => execWs!.send(JSON.stringify({ type: 'resize', cols: execTerm!.cols, rows: execTerm!.rows }))
  execWs.onmessage = (ev) => { try { const m = JSON.parse(ev.data); if (m.type === 'output') execTerm!.write(decodeBase64(m.data)); else if (m.type === 'error') execTerm!.writeln(`\x1b[31m${m.data}\x1b[0m`) } catch {} }
  execWs.onclose = () => execTerm?.writeln('\x1b[33m\r\n● 连接已关闭\x1b[0m')
  execTerm.onData((d) => { if (execWs?.readyState === WebSocket.OPEN) execWs.send(JSON.stringify({ type: 'input', data: d })) })
  execTerm.onResize(({ cols, rows }) => { if (execWs?.readyState === WebSocket.OPEN) execWs.send(JSON.stringify({ type: 'resize', cols, rows })) })
}
function closeExecTerm() { if (execWs) { execWs.close(); execWs = null } if (execTerm) { execTerm.dispose(); execTerm = null } }

let pt: ReturnType<typeof setInterval> | null = null
watch(selectedNode, () => loadData())
watch(activeTab, () => loadData())
onMounted(() => { if (selectedNode.value) loadData(); pt = setInterval(() => { if (activeTab.value === 'containers' && selectedNode.value) loadContainers() }, 5000) })
onBeforeUnmount(() => { if (pt) clearInterval(pt); closeExecTerm() })
</script>

<style scoped lang="scss">
.tab-bar { display: flex; gap: 2px; margin-bottom: 14px; border-bottom: 1px solid var(--dg-border);
  .tab { display: flex; align-items: center; gap: 6px; padding: 9px 16px; font-size: 13px; font-weight: 500; color: var(--dg-text-faint); cursor: pointer; border-bottom: 2px solid transparent; margin-bottom: -1px; transition: all 0.2s; &:hover { color: var(--dg-text-dim); } &.active { color: var(--dg-cyan); border-bottom-color: var(--dg-cyan); } }
}
.content-panel { padding: 18px; min-height: 500px; }
.panel-toolbar { display: flex; align-items: center; gap: 14px; margin-bottom: 14px; }
.btn-sm { display: flex; align-items: center; gap: 5px; padding: 6px 12px; border-radius: 8px; font-size: 12px; font-weight: 500; cursor: pointer; transition: all 0.2s; border: 1px solid var(--dg-border); background: var(--dg-bg-2); color: var(--dg-text-dim); font-family: inherit; &:hover { border-color: var(--dg-border-bright); color: var(--dg-cyan); } &.btn-primary { background: linear-gradient(135deg, var(--dg-blue), var(--dg-indigo)); border-color: transparent; color: #fff; &:hover { box-shadow: var(--dg-glow-blue); } } }
.toggle-line { display: flex; align-items: center; gap: 6px; font-size: 12px; color: var(--dg-text-dim); cursor: pointer; input { width: 14px; height: 14px; accent-color: var(--dg-cyan); } }
.no-data { padding: 60px 0; text-align: center; color: var(--dg-text-faint); font-size: 13px; }

.c-list { display: flex; flex-direction: column; gap: 6px; }
.c-row { display: flex; align-items: center; gap: 12px; padding: 10px 14px; border-radius: 10px; background: var(--dg-bg-2); border: 1px solid var(--dg-border); transition: all 0.15s; &:hover { border-color: var(--dg-border); background: var(--dg-bg-2); } &.stopped { opacity: 0.5; }
  .c-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; &.running { background: var(--dg-green); box-shadow: 0 0 6px var(--dg-green); } &.exited { background: var(--dg-text-faint); } &.paused { background: var(--dg-amber); } }
  .c-info { flex: 1; min-width: 0; .c-name { font-size: 13px; font-weight: 600; color: var(--dg-text); } .c-meta { display: flex; gap: 10px; margin-top: 2px; font-size: 11px; color: var(--dg-text-faint); .mono { font-family: 'JetBrains Mono', monospace; } .c-id { font-family: 'JetBrains Mono', monospace; opacity: 0.7; } } }
  .c-ports { display: flex; gap: 4px; .port-chip { font-size: 10px; padding: 2px 7px; border-radius: 4px; background: var(--dg-info-bg); color: var(--dg-blue); font-family: 'JetBrains Mono', monospace; } }
  .c-btns { display: flex; gap: 4px; }
  .c-btn { width: 28px; height: 28px; border-radius: 7px; border: 1px solid var(--dg-border); background: var(--dg-bg-2); color: var(--dg-text-dim); cursor: pointer; display: flex; align-items: center; justify-content: center; transition: all 0.15s; &:hover { border-color: var(--dg-cyan); color: var(--dg-cyan); } &.c-start:hover { border-color: var(--dg-green); color: var(--dg-green); } &.c-del:hover { border-color: var(--dg-red); color: var(--dg-red); } }
}
.compose { .c-toolbar { display: flex; gap: 8px; margin-bottom: 12px; } .c-editor { width: 100%; min-height: 320px; padding: 16px; border: 1px solid var(--dg-border); border-radius: 12px; font-family: 'JetBrains Mono', monospace; font-size: 13px; line-height: 1.6; outline: none; resize: vertical; background: var(--dg-bg-2); color: var(--dg-text); &:focus { border-color: var(--dg-cyan); } } }
.exec-term { height: 60vh; background: #0e0e1c; border-radius: 8px; padding: 8px; overflow: hidden; }
:deep(.xterm) { padding: 8px; }
:deep(.xterm-viewport) { background-color: transparent !important; }
</style>
