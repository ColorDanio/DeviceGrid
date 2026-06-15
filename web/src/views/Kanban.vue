<template>
  <div class="page-container">
    <div class="kpi-row">
      <div class="kpi-card" v-for="k in kpis" :key="k.label">
        <div class="kpi-label">{{ k.label }}</div>
        <div class="kpi-value" :style="{ color: k.color }">{{ k.value }}<span class="kpi-unit">{{ k.unit }}</span></div>
        <div class="kpi-bar"><div class="kpi-fill" :style="{ width: k.pct + '%', background: k.color }"></div></div>
      </div>
    </div>

    <div class="section-header">
      <h3>节点状态</h3>
      <div class="section-tools">
        <div class="filter-chips">
          <button class="fchip" :class="{ active: filterStatus === '' }" @click="filterStatus = ''">全部 {{ nodes.length }}</button>
          <button class="fchip" :class="{ active: filterStatus === 'online' }" @click="filterStatus = 'online'">在线 {{ onlineCount }}</button>
          <button class="fchip" :class="{ active: filterStatus === 'offline' }" @click="filterStatus = 'offline'">离线 {{ offlineCount }}</button>
          <button class="fchip" :class="{ active: filterStatus === 'untrusted' }" @click="filterStatus = 'untrusted'">未授信 {{ untrustedCount }}</button>
        </div>
        <button class="icon-btn" :class="{ spinning: loading }" @click="loadNodes"><svg viewBox="0 0 24 24" width="14" height="14" fill="none"><path d="M23 4v6h-6M1 20v-6h6" stroke="currentColor" stroke-width="2"/><path d="M3.5 9a9 9 0 0114.8-3.4L23 10M1 14l4.6 4.4A9 9 0 0020.5 15" stroke="currentColor" stroke-width="2"/></svg></button>
      </div>
    </div>

    <div v-loading="loading" class="node-grid">
      <div v-for="n in filteredNodes" :key="n.id" class="node-card" @click="$router.push(`/nodes/${n.id}`)">
        <div class="nc-head">
          <div class="nc-title">
            <span class="nc-status-dot" :class="n.status"></span>
            <div>
              <div class="nc-name">{{ n.name }}</div>
              <div class="nc-ip">{{ n.host }}<span class="nc-port">:{{ n.port }}</span></div>
            </div>
          </div>
          <div class="nc-tags">
            <span class="nc-tag" v-if="n.country_code"><span class="nc-flag">{{ flag(n.country_code) }}</span>{{ n.country }}</span>
            <span class="nc-tag virt" v-if="metrics[n.id]?.virt_type">{{ virtLabel(metrics[n.id].virt_type) }}</span>
            <span class="nc-tag" v-if="n.docker_version">Docker</span>
          </div>
        </div>

        <!-- Hardware info -->
        <div class="hw-info" v-if="metrics[n.id] && n.status === 'online'">
          <div class="hw-row" v-if="metrics[n.id].cpu_model">
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none"><rect x="4" y="4" width="16" height="16" rx="2" stroke="currentColor" stroke-width="1.5"/><path d="M9 9h6v6H9z" stroke="currentColor" stroke-width="1.5"/><path d="M9 2v2M15 2v2M9 20v2M15 20v2M2 9h2M2 15h2M20 9h2M20 15h2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
            <span class="hw-text">{{ shortCPU(metrics[n.id].cpu_model) }}</span>
            <span class="hw-detail">{{ metrics[n.id].cpu_sockets }}P × {{ metrics[n.id].cpu_cores }}C / {{ metrics[n.id].cpu_threads }}T</span>
          </div>
          <div class="hw-row">
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none"><rect x="2" y="6" width="20" height="12" rx="1" stroke="currentColor" stroke-width="1.5"/><path d="M6 10v4M10 10v4M14 10v4M18 10v4" stroke="currentColor" stroke-width="1.5"/></svg>
            <span class="hw-text">{{ fmtBytes(metrics[n.id].mem_total) }} RAM</span>
            <span class="hw-detail">{{ fmtBytes(metrics[n.id].disk_total) }} Disk</span>
          </div>
        </div>

        <!-- Metrics -->
        <div class="nc-metrics" v-if="metrics[n.id] && n.status === 'online'">
          <div class="metric-bar">
            <div class="mb-header"><span class="mb-label">CPU</span><span class="mb-value">{{ metrics[n.id]!.cpu_usage.toFixed(2) }}<small>%</small></span></div>
            <div class="mb-track"><div class="mb-fill" :class="loadLevel(metrics[n.id]!.cpu_usage)" :style="{ width: Math.min(metrics[n.id]!.cpu_usage, 100) + '%' }"></div></div>
          </div>
          <div class="metric-bar">
            <div class="mb-header"><span class="mb-label">内存</span><span class="mb-value">{{ memPct(metrics[n.id]!).toFixed(2) }}<small>%</small></span></div>
            <div class="mb-track"><div class="mb-fill" :class="loadLevel(memPct(metrics[n.id]!))" :style="{ width: memPct(metrics[n.id]!) + '%' }"></div></div>
            <div class="mb-sub">{{ fmtBytes(metrics[n.id]!.mem_used) }} / {{ fmtBytes(metrics[n.id]!.mem_total) }}</div>
          </div>
          <div class="metric-bar">
            <div class="mb-header"><span class="mb-label">磁盘</span><span class="mb-value">{{ diskPct(metrics[n.id]!).toFixed(2) }}<small>%</small></span></div>
            <div class="mb-track"><div class="mb-fill" :class="loadLevel(diskPct(metrics[n.id]!))" :style="{ width: diskPct(metrics[n.id]!) + '%' }"></div></div>
            <div class="mb-sub">{{ fmtBytes(metrics[n.id]!.disk_used) }} / {{ fmtBytes(metrics[n.id]!.disk_total) }}</div>
          </div>
          <div class="metric-line">
            <span class="ml-item">负载 <strong>{{ metrics[n.id]!.load_avg_1.toFixed(2) }}</strong></span>
            <span class="ml-divider"></span>
            <span class="ml-item" v-if="metrics[n.id]!.net_iface">↓{{ fmtRate(metrics[n.id]!.net_rx) }} ↑{{ fmtRate(metrics[n.id]!.net_tx) }}</span>
          </div>
          <div v-if="metrics[n.id]!.gpus?.length" class="dc-gpus">
            <span v-for="g in metrics[n.id]!.gpus" :key="g.index" class="gpu-chip">GPU{{ g.index }} {{ g.utilization.toFixed(0) }}% {{ g.temperature }}°C</span>
          </div>
        </div>
        <div class="nc-offline-metrics" v-else-if="n.status === 'online'">
          <span class="collecting">采集中...</span>
        </div>
        <div class="nc-offline-metrics" v-else>
          <span :class="n.status">{{ statusLabel(n.status) }}</span>
        </div>
      </div>
    </div>

    <div v-if="!loading && filteredNodes.length === 0" class="empty"><p>暂无节点</p></div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { listNodes, getMetrics, type Node, type NodeMetrics } from '@/api/nodes'

const loading = ref(false)
const nodes = ref<Node[]>([])
const metrics = ref<Record<string, NodeMetrics>>({})
const filterStatus = ref('')
let nodeTimer: ReturnType<typeof setInterval> | null = null
let metricTimer: ReturnType<typeof setInterval> | null = null

const onlineCount = computed(() => nodes.value.filter(n => n.status === 'online').length)
const offlineCount = computed(() => nodes.value.filter(n => n.status === 'offline' || n.status === 'error').length)
const untrustedCount = computed(() => nodes.value.filter(n => n.status === 'untrusted').length)

const kpis = computed(() => {
  const ms = Object.values(metrics.value)
  const totalMem = ms.reduce((s, m) => s + m.mem_total, 0)
  const usedMem = ms.reduce((s, m) => s + m.mem_used, 0)
  const totalDisk = ms.reduce((s, m) => s + m.disk_total, 0)
  const usedDisk = ms.reduce((s, m) => s + m.disk_used, 0)
  const avgCpu = ms.length > 0 ? ms.reduce((s, m) => s + m.cpu_usage, 0) / ms.length : 0
  const totalCores = ms.reduce((s, m) => s + (m.cpu_cores || 0), 0)
  const totalThreads = ms.reduce((s, m) => s + (m.cpu_threads || 0), 0)
  const totalNetRx = ms.reduce((s, m) => s + (m.net_rx || 0), 0)
  const totalNetTx = ms.reduce((s, m) => s + (m.net_tx || 0), 0)
  return [
    { label: '节点', value: nodes.value.length, unit: '', color: 'var(--accent)', pct: nodes.value.length ? onlineCount.value / nodes.value.length * 100 : 0 },
    { label: '在线率', value: nodes.value.length ? (onlineCount.value / nodes.value.length * 100).toFixed(2) : '0.00', unit: '%', color: 'var(--dg-success)', pct: nodes.value.length ? onlineCount.value / nodes.value.length * 100 : 0 },
    { label: 'CPU核数', value: totalCores, unit: 'C/'+totalThreads+'T', color: 'var(--accent)', pct: avgCpu },
    { label: '平均CPU', value: avgCpu.toFixed(2), unit: '%', color: avgCpu > 80 ? 'var(--dg-danger)' : 'var(--accent)', pct: avgCpu },
    { label: '内存', value: totalMem ? (usedMem / totalMem * 100).toFixed(2) : '0.00', unit: '%', color: 'var(--dg-success)', pct: totalMem ? usedMem / totalMem * 100 : 0 },
    { label: '磁盘', value: totalDisk ? (usedDisk / totalDisk * 100).toFixed(2) : '0.00', unit: '%', color: 'var(--dg-warning)', pct: totalDisk ? usedDisk / totalDisk * 100 : 0 },
    { label: '网络流量', value: fmtRate(totalNetRx + totalNetTx), unit: '', color: 'var(--accent)', pct: 0 },
  ]
})

const filteredNodes = computed(() => filterStatus.value ? nodes.value.filter(n => n.status === filterStatus.value) : nodes.value)

function statusLabel(s: string) { return ({ online: '在线', offline: '离线', untrusted: '未授信', error: '异常' } as Record<string,string>)[s] || s }
function virtLabel(v: string) { if (!v || v === 'bare-metal' || v === 'none') return '物理机'; if (v === 'kvm') return 'KVM'; if (v === 'docker') return 'Docker'; if (v === 'lxc') return 'LXC'; return v.toUpperCase() }
function flag(cc: string) { if (!cc || cc.length !== 2) return ''; return String.fromCodePoint(0x1F1E6 + cc.charCodeAt(0) - 65) + String.fromCodePoint(0x1F1E6 + cc.charCodeAt(1) - 65) }
function shortCPU(model: string) { if (!model) return ''; const parts = model.split(/\s+/); return parts.slice(0, 4).join(' ').replace(/\(R\)|\(TM\)|CPU/g, '').replace(/\s+/g, ' ').trim() }
function memPct(m: NodeMetrics) { return m.mem_total > 0 ? m.mem_used / m.mem_total * 100 : 0 }
function diskPct(m: NodeMetrics) { return m.disk_total > 0 ? m.disk_used / m.disk_total * 100 : 0 }
function loadLevel(pct: number): string { if (pct >= 85) return 'critical'; if (pct >= 70) return 'high'; if (pct >= 40) return 'medium'; return 'low' }
function fmtBytes(b: number) { if (!b) return '0B'; if (b < 1024) return b+'B'; if (b < 1048576) return (b/1024).toFixed(0)+'KB'; if (b < 1073741824) return (b/1048576).toFixed(0)+'MB'; if (b < 1099511627776) return (b/1073741824).toFixed(0)+'GB'; return (b/1099511627776).toFixed(1)+'TB' }
function fmtRate(bytes: number) { if (!bytes) return '0B'; if (bytes < 1024) return bytes+'B'; if (bytes < 1048576) return (bytes/1024).toFixed(1)+'KB'; if (bytes < 1073741824) return (bytes/1048576).toFixed(1)+'MB'; if (bytes < 1099511627776) return (bytes/1073741824).toFixed(2)+'GB'; return (bytes/1099511627776).toFixed(1)+'TB' }

async function loadNodes() { loading.value = true; try { nodes.value = await listNodes() } finally { loading.value = false }; fetchAllMetrics() }

// Fetch metrics for all online nodes in parallel
async function fetchAllMetrics() {
  const onlineNodes = nodes.value.filter(n => n.status === 'online')
  await Promise.allSettled(onlineNodes.map(async n => {
    try { metrics.value[n.id] = await getMetrics(n.id) } catch {}
  }))
}

// Refresh only existing metrics
async function refreshMetrics() {
  const toRefresh = nodes.value.filter(n => n.status === 'online' && metrics.value[n.id])
  // Don't re-fetch nodes that were updated < 10s ago
  await Promise.allSettled(toRefresh.map(async n => {
    try { metrics.value[n.id] = await getMetrics(n.id) } catch {}
  }))
}

onMounted(() => {
  loadNodes()
  nodeTimer = setInterval(loadNodes, 30000)
  metricTimer = setInterval(refreshMetrics, 20000)
})
onBeforeUnmount(() => { if (nodeTimer) clearInterval(nodeTimer); if (metricTimer) clearInterval(metricTimer) })
</script>

<style scoped lang="scss">
.kpi-row { display: grid; grid-template-columns: repeat(7, 1fr); gap: 10px; margin-bottom: 20px; }
.kpi-card { padding: 16px; background: var(--dg-surface); border: 1px solid var(--dg-border); border-radius: 12px;
  .kpi-label { font-size: 11px; color: var(--dg-text-dim); text-transform: uppercase; letter-spacing: 0.05em; font-weight: 600; margin-bottom: 6px; }
  .kpi-value { font-size: 28px; font-weight: 700; line-height: 1; .kpi-unit { font-size: 14px; font-weight: 500; margin-left: 2px; } }
  .kpi-bar { height: 3px; background: var(--dg-border); border-radius: 2px; margin-top: 10px; overflow: hidden; .kpi-fill { height: 100%; border-radius: 2px; transition: width 0.5s; } }
}

.section-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px;
  h3 { font-size: 15px; font-weight: 600; }
  .section-tools { display: flex; align-items: center; gap: 10px; }
  .filter-chips { display: flex; gap: 4px; }
  .fchip { padding: 5px 12px; border-radius: 6px; border: 1px solid var(--dg-border); background: transparent; color: var(--dg-text-dim); font-size: 12px; font-weight: 500; cursor: pointer; transition: all 0.15s; font-family: inherit;
    &:hover { color: var(--accent); } &.active { background: var(--accent); border-color: var(--accent); color: #fff; } }
  .icon-btn { width: 30px; height: 30px; border-radius: 8px; border: 1px solid var(--dg-border); background: transparent; color: var(--dg-text-dim); cursor: pointer; display: flex; align-items: center; justify-content: center;
    &:hover { color: var(--accent); border-color: var(--accent); } &.spinning svg { animation: spin 0.8s linear infinite; } }
}
@keyframes spin { to { transform: rotate(360deg); } }

.node-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 12px; }
.node-card { padding: 16px; background: var(--dg-surface); border: 1px solid var(--dg-border); border-radius: 12px; cursor: pointer; transition: all 0.2s;
  &:hover { border-color: var(--accent); transform: translateY(-1px); }
  .nc-head { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 10px; }
  .nc-title { display: flex; gap: 8px; align-items: flex-start; }
  .nc-status-dot { width: 8px; height: 8px; border-radius: 50%; margin-top: 6px; flex-shrink: 0;
    &.online { background: var(--dg-success); box-shadow: 0 0 6px var(--dg-success); } &.offline { background: var(--dg-danger); } &.untrusted { background: var(--dg-warning); } &.error { background: var(--dg-danger); } }
  .nc-name { font-size: 14px; font-weight: 600; } .nc-ip { font-size: 11px; color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace; .nc-port { opacity: 0.6; } }
  .nc-tags { display: flex; flex-direction: column; gap: 3px; align-items: flex-end; }
  .nc-tag { font-size: 10px; padding: 2px 7px; border-radius: 4px; font-weight: 500; background: var(--dg-info-bg); color: var(--accent);
    &.docker { background: var(--dg-warning-bg); color: var(--dg-warning); } &.virt { background: var(--dg-bg-3); color: var(--dg-text-dim); }
    .nc-flag { margin-right: 2px; } }
}

.hw-info { display: flex; flex-direction: column; gap: 4px; margin-bottom: 10px; padding-bottom: 10px; border-bottom: 1px solid var(--dg-table-border);
  .hw-row { display: flex; align-items: center; gap: 6px; font-size: 11px; color: var(--dg-text-dim); svg { color: var(--dg-text-faint); flex-shrink: 0; }
    .hw-text { font-weight: 500; } .hw-detail { color: var(--dg-text-faint); margin-left: auto; font-family: 'JetBrains Mono', monospace; font-size: 10px; } }
}

.nc-metrics { display: flex; flex-direction: column; gap: 10px; }
.metric-bar {
  .mb-header { display: flex; align-items: baseline; justify-content: space-between; margin-bottom: 4px; }
  .mb-label { font-size: 11px; font-weight: 600; color: var(--dg-text-dim); text-transform: uppercase; letter-spacing: 0.04em; }
  .mb-value { font-family: 'JetBrains Mono', monospace; font-size: 13px; font-weight: 700; color: var(--dg-text); small { font-size: 9px; opacity: 0.6; } }
  .mb-track { height: 5px; background: var(--dg-bg-3); border-radius: 3px; overflow: hidden; }
  .mb-fill { height: 100%; border-radius: 3px; transition: width 0.6s cubic-bezier(0.4,0,0.2,1);
    &.low { background: linear-gradient(90deg, var(--accent), rgba(var(--accent-rgb), 0.6)); }
    &.medium { background: linear-gradient(90deg, var(--accent), var(--accent-light)); }
    &.high { background: linear-gradient(90deg, var(--dg-warning), #fbbf24); }
    &.critical { background: linear-gradient(90deg, var(--dg-danger), #f87171); }
  }
  .mb-sub { font-size: 10px; color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace; margin-top: 2px; }
}
.metric-line { display: flex; align-items: center; gap: 8px; font-size: 10px; color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace;
  .ml-item { strong { color: var(--dg-text-dim); font-weight: 600; } }
  .ml-divider { width: 1px; height: 10px; background: var(--dg-border); }
}
.dc-gpus { display: flex; flex-wrap: wrap; gap: 4px; .gpu-chip { font-size: 9px; padding: 2px 6px; border-radius: 4px; background: rgba(168,85,247,0.15); color: #c084fc; font-family: 'JetBrains Mono', monospace; } }

.nc-offline-metrics { text-align: center; padding: 16px 0; font-size: 12px; color: var(--dg-text-faint);
  .online { color: var(--dg-success); } .offline { color: var(--dg-danger); } .untrusted { color: var(--dg-warning); }
  .collecting { color: var(--dg-text-faint); animation: pulse-text 1.5s infinite; } }
@keyframes pulse-text { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }

.empty { padding: 60px; text-align: center; p { color: var(--dg-text-faint); } }
@media (max-width: 1400px) { .kpi-row { grid-template-columns: repeat(4, 1fr); } }
@media (max-width: 1024px) { .kpi-row { grid-template-columns: repeat(2, 1fr); } }
</style>
