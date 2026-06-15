<template>
  <div class="page-container" v-if="node">
    <div class="page-header">
      <div class="page-title-group">
        <button class="back-btn" @click="$router.push('/kanban')">
          <svg viewBox="0 0 24 24" fill="none" width="16" height="16"><path d="M19 12H5M12 19l-7-7 7-7" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
          返回
        </button>
        <h2 style="margin-top:12px; display:flex; align-items:center; gap:8px">
          {{ node.name }}
          <span class="status-dot" :class="node.status"></span>
        </h2>
        <p class="page-subtitle mono">{{ node.host }}:{{ node.port }} · {{ node.username }}<span v-if="node.country_code"> · {{ countryCodeToFlag(node.country_code) }} {{ node.country }}</span></p>
      </div>
      <div class="header-actions">
        <button class="action-card" @click="$router.push(`/terminal?node=${node.id}`)">
          <div class="ac-icon"><svg viewBox="0 0 24 24" fill="none" width="18" height="18"><rect x="2" y="4" width="20" height="16" rx="2" stroke="currentColor" stroke-width="1.8"/><path d="M6 9l3 3-3 3M12 15h4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg></div>
          <span>终端</span>
        </button>
        <button class="action-card" @click="$router.push('/docker')">
          <div class="ac-icon"><svg viewBox="0 0 24 24" fill="none" width="18" height="18"><path d="M22 12c0-5.5-4.5-10-10-10S2 6.5 2 12s4.5 10 10 10 10-4.5 10-10z" stroke="currentColor" stroke-width="1.8"/></svg></div>
          <span>Docker</span>
        </button>
        <button class="action-card" @click="$router.push('/deploy')">
          <div class="ac-icon"><svg viewBox="0 0 24 24" fill="none" width="18" height="18"><path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg></div>
          <span>部署</span>
        </button>
      </div>
    </div>

    <div class="detail-grid">
      <!-- Left: Hardware + Metrics -->
      <div class="detail-main">
        <!-- Live Metrics -->
        <div class="dg-card s-card" v-if="metrics">
          <h3 class="card-title">实时监控</h3>
          <div class="live-metrics">
            <div class="lm-bar">
              <div class="lm-header"><span class="lm-label">CPU 使用率</span><span class="lm-value">{{ metrics.cpu_usage.toFixed(2) }}%</span></div>
              <div class="lm-track"><div class="lm-fill" :class="loadLevel(metrics.cpu_usage)" :style="{ width: Math.min(metrics.cpu_usage, 100) + '%' }"></div></div>
            </div>
            <div class="lm-bar">
              <div class="lm-header"><span class="lm-label">内存使用率</span><span class="lm-value">{{ memPct.toFixed(2) }}%</span></div>
              <div class="lm-track"><div class="lm-fill" :class="loadLevel(memPct)" :style="{ width: memPct + '%' }"></div></div>
              <div class="lm-detail">{{ fmtBytes(metrics.mem_used) }} / {{ fmtBytes(metrics.mem_total) }}</div>
            </div>
            <div class="lm-bar">
              <div class="lm-header"><span class="lm-label">磁盘使用率</span><span class="lm-value">{{ diskPct.toFixed(2) }}%</span></div>
              <div class="lm-track"><div class="lm-fill" :class="loadLevel(diskPct)" :style="{ width: diskPct + '%' }"></div></div>
              <div class="lm-detail">{{ fmtBytes(metrics.disk_used) }} / {{ fmtBytes(metrics.disk_total) }}</div>
            </div>
            <div class="lm-bar" v-if="metrics.swap_total > 0">
              <div class="lm-header"><span class="lm-label">Swap</span><span class="lm-value">{{ (metrics.swap_used / metrics.swap_total * 100).toFixed(2) }}%</span></div>
              <div class="lm-track"><div class="lm-fill" :class="loadLevel(metrics.swap_used / metrics.swap_total * 100)" :style="{ width: metrics.swap_used / metrics.swap_total * 100 + '%' }"></div></div>
            </div>
            <div class="lm-stats">
              <div class="lm-stat"><span class="ls-label">负载 (1/5/15)</span><span class="ls-val mono">{{ metrics.load_avg_1.toFixed(2) }} / {{ metrics.load_avg_5.toFixed(2) }} / {{ metrics.load_avg_15.toFixed(2) }}</span></div>
              <div class="lm-stat" v-if="metrics.net_iface"><span class="ls-label">网络 {{ metrics.net_iface }}</span><span class="ls-val mono">↓{{ fmtRate(metrics.net_rx) }} · ↑{{ fmtRate(metrics.net_tx) }}</span></div>
              <div class="lm-stat"><span class="ls-label">运行时间</span><span class="ls-val mono">{{ formatUptime(metrics.uptime) }}</span></div>
            </div>
          </div>
          <div v-if="metrics.gpus?.length" class="gpu-list">
            <div v-for="g in metrics.gpus" :key="g.index" class="gpu-card">
              <span class="gpu-name">GPU {{ g.index }}</span>
              <span class="gpu-detail">{{ g.name }}</span>
              <span class="gpu-stat">{{ g.utilization.toFixed(0) }}% · {{ (g.memory_used/1073741824).toFixed(1) }}/{{ (g.memory_total/1073741824).toFixed(1) }}GB · {{ g.temperature }}°C</span>
            </div>
          </div>
        </div>

        <!-- Hardware Info -->
        <div class="dg-card s-card">
          <h3 class="card-title">硬件信息</h3>
          <div class="hw-list" v-if="metrics">
            <div class="hw-item"><span class="hw-label">CPU</span><span class="hw-value">{{ metrics.cpu_model || '未知' }}</span></div>
            <div class="hw-item"><span class="hw-label">架构</span><span class="hw-value">{{ metrics.cpu_sockets }} 路 × {{ metrics.cpu_cores }} 核 / {{ metrics.cpu_threads }} 线程</span></div>
            <div class="hw-item"><span class="hw-label">内存</span><span class="hw-value">{{ fmtBytes(metrics.mem_total) }}</span></div>
            <div class="hw-item"><span class="hw-label">磁盘</span><span class="hw-value">{{ fmtBytes(metrics.disk_total) }}</span></div>
            <div class="hw-item"><span class="hw-label">虚拟化</span><span class="hw-value">{{ virtLabel(metrics.virt_type) }}</span></div>
          </div>
          <div v-else class="hw-list">
            <div class="hw-item"><span class="hw-label">操作系统</span><span class="hw-value">{{ node.os || '未检测' }}</span></div>
            <div class="hw-item"><span class="hw-label">架构</span><span class="hw-value">{{ node.arch || '—' }}</span></div>
            <div class="hw-item"><span class="hw-label">Docker</span><span class="hw-value">{{ node.docker_version || '未安装' }}</span></div>
          </div>
        </div>

        <!-- Top Processes -->
        <div class="dg-card s-card">
          <h3 class="card-title">资源占用 TOP 10</h3>
          <pre class="console-output" v-if="processes">{{ processes }}</pre>
          <div v-else class="loading-text">加载中...</div>
        </div>

        <!-- Streaming Unlock Check -->
        <div class="dg-card s-card" v-if="netFeatures.enable_streaming">
          <div class="card-header-row">
            <h3 class="card-title">流媒体解锁</h3>
            <div class="header-right-group">
              <span class="last-tested" v-if="streamingTime">最后测试: {{ streamingTime }}</span>
              <button class="check-btn" @click="runStreamingCheck" :disabled="streamingLoading">{{ streamingLoading ? '检测中...' : '开始检测' }}</button>
            </div>
          </div>
          <div v-if="streamingLoading" class="loading-text">正在检测流媒体解锁状态（约30秒）...</div>
          <div v-else-if="streamingResults.length > 0" class="streaming-grid">
            <div v-for="r in streamingResults" :key="r.name" class="stream-item" :class="r.status">
              <span class="si-icon">
                <svg v-if="['UNLOCK','AVAILABLE'].includes(r.status)" viewBox="0 0 24 24" width="14" height="14" fill="none"><path d="M5 13l4 4L19 7" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/></svg>
                <svg v-else-if="r.status === 'PARTIAL'" viewBox="0 0 24 24" width="14" height="14" fill="none"><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2"/><path d="M12 8v4M12 16h.01" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
                <svg v-else viewBox="0 0 24 24" width="14" height="14" fill="none"><path d="M6 6l12 12M6 18L18 6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
              </span>
              <span class="si-name">{{ r.name }}</span>
              <span class="si-region" v-if="r.region">{{ r.region }}</span>
            </div>
          </div>
          <div v-else class="check-empty">点击"开始检测"检查流媒体解锁状态</div>
        </div>

        <!-- AI Service Check -->
        <div class="dg-card s-card" v-if="netFeatures.enable_ai">
          <div class="card-header-row">
            <h3 class="card-title">AI 服务可用性</h3>
            <div class="header-right-group">
              <span class="last-tested" v-if="aiTime">最后测试: {{ aiTime }}</span>
              <button class="check-btn" @click="runAICheck" :disabled="aiLoading">{{ aiLoading ? '检测中...' : '开始检测' }}</button>
            </div>
          </div>
          <div v-if="aiLoading" class="loading-text">正在检测 AI 服务可用性...</div>
          <div v-else-if="aiResults.length > 0" class="streaming-grid">
            <div v-for="r in aiResults" :key="r.name" class="stream-item" :class="r.status">
              <span class="si-icon">
                <svg v-if="r.status === 'OK'" viewBox="0 0 24 24" width="14" height="14" fill="none"><path d="M5 13l4 4L19 7" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/></svg>
                <svg v-else-if="r.status === 'LIMITED'" viewBox="0 0 24 24" width="14" height="14" fill="none"><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2"/><path d="M12 8v4M12 16h.01" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
                <svg v-else viewBox="0 0 24 24" width="14" height="14" fill="none"><path d="M6 6l12 12M6 18L18 6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
              </span>
              <span class="si-name">{{ r.name }}</span>
              <span class="si-region" v-if="r.region">{{ r.region }}</span>
            </div>
          </div>
          <div v-else class="check-empty">点击"开始检测"检查 AI 服务可用性</div>
        </div>

        <!-- Connectivity Test -->
        <div class="dg-card s-card" v-if="netFeatures.enable_connectivity">
          <div class="card-header-row">
            <h3 class="card-title">网络联通性</h3>
            <div class="header-right-group">
              <span class="last-tested" v-if="connectivityTime">最后测试: {{ connectivityTime }}</span>
              <button class="check-btn" @click="runConnectivityTest" :disabled="connectivityLoading">{{ connectivityLoading ? '测试中...' : '开始测试' }}</button>
            </div>
          </div>
          <div v-if="connectivityLoading" class="loading-text">正在测试各区域网络延迟...</div>
          <div v-else-if="connectivityResults.length > 0" class="connectivity-list">
            <div v-for="r in connectivityResults" :key="r.region" class="conn-item">
              <span class="ci-region">{{ r.region }}</span>
              <span class="ci-latency" :class="latencyClass(r.latency_ms, r.ok)">{{ r.ok ? r.latency_ms.toFixed(0) + 'ms' : '不通' }}</span>
              <span class="ci-loss" v-if="r.ok && r.loss_pct > 0" :class="{ bad: r.loss_pct >= 30 }">{{ r.loss_pct }}%丢包</span>
              <span class="ci-bar" v-if="r.ok"><span class="ci-bar-fill" :style="{ width: Math.min(r.latency_ms / 5, 100) + '%', background: latencyColor(r.latency_ms) }"></span></span>
            </div>
          </div>
          <div v-else class="check-empty">点击"开始测试"检测各区域网络延迟</div>
        </div>

        <!-- Return Route (三网回程) -->
        <div class="dg-card s-card" v-if="netFeatures.enable_route">
          <div class="card-header-row">
            <h3 class="card-title">三网回程路由</h3>
            <div class="header-right-group">
              <span class="last-tested" v-if="routeTime">最后测试: {{ routeTime }}</span>
              <button class="check-btn" @click="runRouteTest" :disabled="routeLoading">{{ routeLoading ? '测试中...' : '开始测试' }}</button>
            </div>
          </div>
          <div v-if="routeLoading" class="loading-text">正在测试三网回程路由（电信/联通/移动，约1-2分钟）...</div>
          <div v-else-if="routeResults.length > 0" class="route-results">
            <div v-for="r in routeResults" :key="r.isp + r.city + r.ip" class="route-card">
              <div class="rc-header">
                <span class="rc-isp" :class="ispClass(r.isp)">{{ r.isp }}</span>
                <span class="rc-city">{{ r.city }}</span>
                <span class="rc-linetype" v-if="r.line_type && r.line_type !== 'Unknown'">{{ r.line_type }}</span>
                <span class="rc-ip">{{ r.ip }}</span>
                <span class="rc-latency" :class="latencyClass(r.latency_ms, r.latency_ms < 999)">{{ r.latency_ms < 999 ? r.latency_ms.toFixed(0) + 'ms' : '不通' }}</span>
              </div>
              <div class="rc-hops" v-if="r.hops && r.hops.length > 0 && !r.hops[0].includes('no traceroute')">
                <div v-for="(hop, i) in r.hops.slice(0, 12)" :key="i" class="hop-line">{{ hop }}</div>
              </div>
            </div>
          </div>
          <div v-else class="check-empty">点击"开始测试"检测三网回程路由</div>
        </div>
      </div>

      <!-- Right: Info + Timeline + Logins -->
      <div class="detail-side">
        <!-- Basic Info -->
        <div class="dg-card s-card">
          <h3 class="card-title">基本信息</h3>
          <div class="info-list">
            <div class="info-row"><span class="ir-label">状态</span><span class="ir-value"><span class="status-dot" :class="node.status"></span> {{ statusLabel(node.status) }}</span></div>
            <div class="info-row"><span class="ir-label">地址</span><span class="ir-value mono">{{ node.host }}:{{ node.port }}</span></div>
            <div class="info-row"><span class="ir-label">用户</span><span class="ir-value">{{ node.username }}</span></div>
            <div class="info-row"><span class="ir-label">通信</span><span class="ir-value">{{ node.transport_mode === 'agent' ? 'Agent' : 'SSH' }}</span></div>
            <div class="info-row"><span class="ir-label">认证</span><span class="ir-value">{{ node.auth_mode === 'key' ? '密钥' : '密码' }}</span></div>
            <div class="info-row"><span class="ir-label">地区</span><span class="ir-value" v-if="node.country_code">{{ countryCodeToFlag(node.country_code) }} {{ node.country }} / {{ node.region }}</span></div>
            <div class="info-row"><span class="ir-label">ISP</span><span class="ir-value">{{ node.isp || '—' }}</span></div>
            <div class="info-row"><span class="ir-label">Docker</span><span class="ir-value">{{ node.docker_version || '未安装' }}</span></div>
            <div class="info-row"><span class="ir-label">RKE2</span><span class="ir-value">{{ node.rke2_role || '—' }}</span></div>
          </div>
          <div class="tag-list" v-if="node.tags?.length">
            <span class="chip" v-for="tag in node.tags" :key="tag">{{ tag }}</span>
          </div>
        </div>

        <!-- Login History -->
        <div class="dg-card s-card">
          <h3 class="card-title">最近登录</h3>
          <pre class="console-output small" v-if="logins">{{ logins }}</pre>
          <div v-else class="loading-text">加载中...</div>
        </div>

        <!-- Timeline -->
        <div class="dg-card s-card">
          <h3 class="card-title">时间线</h3>
          <el-timeline>
            <el-timeline-item :timestamp="formatTime(node.created_at)" placement="top" type="primary">
              节点创建
            </el-timeline-item>
            <el-timeline-item v-if="node.last_seen_at && !node.last_seen_at.startsWith('0001')" :timestamp="formatTime(node.last_seen_at)" placement="top" type="success">
              最后在线
            </el-timeline-item>
            <el-timeline-item v-if="node.auth_mode === 'key'" :timestamp="formatTime(node.updated_at)" placement="top" type="info">
              密钥授信
            </el-timeline-item>
          </el-timeline>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { getNode, getMetrics, getTopProcesses, getLoginHistory, checkStreaming, checkAI, checkConnectivity, checkReturnRoute, getNetworkConfig, type Node, type NodeMetrics, type StreamingResult, type ConnectivityResult, type RouteHop, type NetworkFeatures } from '@/api/nodes'

const route = useRoute()
const node = ref<Node | null>(null)
const metrics = ref<NodeMetrics | null>(null)
const processes = ref('')
const logins = ref('')

// Check results with timestamps
interface CachedCheck<T> { data: T[]; testedAt: string }
const streamingResults = ref<StreamingResult[]>([])
const streamingTime = ref('')
const streamingLoading = ref(false)
const aiResults = ref<StreamingResult[]>([])
const aiTime = ref('')
const aiLoading = ref(false)
const connectivityResults = ref<ConnectivityResult[]>([])
const connectivityTime = ref('')
const connectivityLoading = ref(false)
const routeResults = ref<RouteHop[]>([])
const routeTime = ref('')
const routeLoading = ref(false)
const netFeatures = ref<NetworkFeatures>({ environment: 'public', enable_geo: true, enable_streaming: true, enable_ai: true, enable_connectivity: true, enable_route: true })
let metricTimer: ReturnType<typeof setInterval> | null = null

function statusLabel(s: string) { return ({ online: '在线', offline: '离线', untrusted: '未授信', error: '异常' } as Record<string,string>)[s] || s }
function formatTime(t: string) { if (!t || t.startsWith('0001')) return '—'; return new Date(t).toLocaleString() }
function countryCodeToFlag(cc: string) { if (!cc || cc.length !== 2) return ''; const A = 0x1F1E6; return String.fromCodePoint(A + cc.charCodeAt(0) - 65) + String.fromCodePoint(A + cc.charCodeAt(1) - 65) }
function virtLabel(v: string) { if (!v || v === 'bare-metal' || v === 'none') return '物理机'; if (v === 'kvm') return 'KVM 虚拟机'; if (v === 'docker') return 'Docker 容器'; if (v === 'lxc') return 'LXC 容器'; if (v === 'xen') return 'Xen 虚拟机'; if (v === 'vmware') return 'VMware'; return v }
function fmtBytes(b: number) { if (!b) return '0B'; if (b < 1024) return b+'B'; if (b < 1048576) return (b/1024).toFixed(0)+'KB'; if (b < 1073741824) return (b/1048576).toFixed(0)+'MB'; if (b < 1099511627776) return (b/1073741824).toFixed(0)+'GB'; return (b/1099511627776).toFixed(1)+'TB' }
function fmtRate(b: number) { if (!b) return '0B'; if (b < 1048576) return (b/1024).toFixed(1)+'KB'; if (b < 1073741824) return (b/1048576).toFixed(1)+'MB'; return (b/1073741824).toFixed(2)+'GB' }
function loadLevel(pct: number): string { if (pct >= 85) return 'critical'; if (pct >= 70) return 'high'; if (pct >= 40) return 'medium'; return 'low' }
function formatUptime(sec: number) { if (!sec) return '—'; const d = Math.floor(sec/86400); const h = Math.floor((sec%86400)/3600); const m = Math.floor((sec%3600)/60); if (d > 0) return `${d}天 ${h}小时`; if (h > 0) return `${h}小时 ${m}分钟`; return `${m}分钟` }

const memPct = computed(() => metrics.value && metrics.value.mem_total > 0 ? metrics.value.mem_used / metrics.value.mem_total * 100 : 0)
const diskPct = computed(() => metrics.value && metrics.value.disk_total > 0 ? metrics.value.disk_used / metrics.value.disk_total * 100 : 0)

async function loadData() {
  node.value = await getNode(route.params.id as string)
  loadMetrics()
  loadProcesses()
  loadLogins()
}

async function loadMetrics() {
  if (node.value?.status !== 'online') return
  try { metrics.value = await getMetrics(route.params.id as string) } catch {}
}

async function loadProcesses() {
  if (node.value?.status !== 'online') return
  try { processes.value = await getTopProcesses(route.params.id as string) } catch {}
}

async function loadLogins() {
  if (node.value?.status !== 'online') return
  try { logins.value = await getLoginHistory(route.params.id as string) } catch {}
}

async function runStreamingCheck() {
  streamingLoading.value = true
  try {
    const resp = await checkStreaming(route.params.id as string)
    streamingResults.value = resp.results || []
    streamingTime.value = new Date(resp.tested_at).toLocaleString()
    saveCache('streaming', { data: resp.results, testedAt: streamingTime.value })
  } catch {} finally { streamingLoading.value = false }
}

async function runAICheck() {
  aiLoading.value = true
  try {
    const resp = await checkAI(route.params.id as string)
    aiResults.value = resp.results || []
    aiTime.value = new Date(resp.tested_at).toLocaleString()
    saveCache('ai', { data: resp.results, testedAt: aiTime.value })
  } catch {} finally { aiLoading.value = false }
}

async function runConnectivityTest() {
  connectivityLoading.value = true
  try {
    const resp = await checkConnectivity(route.params.id as string)
    connectivityResults.value = resp.results || []
    connectivityTime.value = new Date(resp.tested_at).toLocaleString()
    saveCache('connectivity', { data: resp.results, testedAt: connectivityTime.value })
  } catch {} finally { connectivityLoading.value = false }
}

async function runRouteTest() {
  routeLoading.value = true
  try {
    const resp = await checkReturnRoute(route.params.id as string)
    routeResults.value = resp.results || []
    routeTime.value = new Date(resp.tested_at).toLocaleString()
    saveCache('route', { data: resp.results, testedAt: routeTime.value })
  } catch {} finally { routeLoading.value = false }
}

// localStorage cache
const nodeId = computed(() => route.params.id as string)
function cacheKey(type: string) { return `dg_check_${nodeId.value}_${type}` }
function saveCache(type: string, data: CachedCheck<any>) {
  try { localStorage.setItem(cacheKey(type), JSON.stringify(data)) } catch {}
}
function loadCache(type: string): CachedCheck<any> | null {
  try {
    const raw = localStorage.getItem(cacheKey(type))
    if (!raw) return null
    return JSON.parse(raw)
  } catch { return null }
}

function loadCachedChecks() {
  const s = loadCache('streaming'); if (s) { streamingResults.value = s.data; streamingTime.value = s.testedAt }
  const a = loadCache('ai'); if (a) { aiResults.value = a.data; aiTime.value = a.testedAt }
  const c = loadCache('connectivity'); if (c) { connectivityResults.value = c.data; connectivityTime.value = c.testedAt }
  const r = loadCache('route'); if (r) { routeResults.value = r.data; routeTime.value = r.testedAt }
}

function streamingStatusLabel(s: string) { return ({ UNLOCK: '解锁', AVAILABLE: '可用', PARTIAL: '部分', LOCKED: '锁定', UNAVAILABLE: '不可用', OK: '可用', LIMITED: '受限', BLOCKED: '不可用' } as Record<string,string>)[s] || s }
function latencyClass(ms: number, ok: boolean) { if (!ok) return 'bad'; if (ms < 100) return 'good'; if (ms < 200) return 'ok'; return 'slow' }
function latencyColor(ms: number) { if (ms < 100) return 'var(--dg-success)'; if (ms < 200) return 'var(--dg-warning)'; return 'var(--dg-danger)' }
function ispClass(isp: string) { if (isp.includes('电信')) return 'ct'; if (isp.includes('联通')) return 'cu'; if (isp.includes('移动')) return 'cm'; return '' }

onMounted(() => {
  loadData()
  loadCachedChecks()
  getNetworkConfig().then(c => { netFeatures.value = c }).catch(() => {})
  metricTimer = setInterval(() => { loadMetrics() }, 10000)
})
onBeforeUnmount(() => { if (metricTimer) clearInterval(metricTimer) })
</script>

<style scoped lang="scss">
.back-btn { display: flex; align-items: center; gap: 6px; padding: 6px 14px; border-radius: 8px; border: 1px solid var(--dg-border); background: var(--dg-surface); color: var(--dg-text-dim); font-size: 13px; cursor: pointer; transition: all 0.2s; font-family: inherit; &:hover { border-color: var(--accent); color: var(--accent); } }
.mono { font-family: 'JetBrains Mono', monospace; }
.header-actions { display: flex; gap: 8px; }
.action-card { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 10px 16px; border-radius: 10px; border: 1px solid var(--dg-border); background: var(--dg-surface); cursor: pointer; transition: all 0.2s; font-family: inherit; font-size: 12px; font-weight: 500; color: var(--dg-text-dim);
  &:hover { border-color: var(--accent); color: var(--accent); }
  .ac-icon { display: flex; align-items: center; justify-content: center; }
}

.detail-grid { display: grid; grid-template-columns: 2fr 1fr; gap: 16px; }
.detail-main, .detail-side { display: flex; flex-direction: column; gap: 16px; }

.s-card { padding: 20px; }
.card-title { font-size: 14px; font-weight: 600; margin-bottom: 14px; }

/* Live metrics — clean progress bars */
.live-metrics { display: flex; flex-direction: column; gap: 14px; }
.lm-bar {
  .lm-header { display: flex; align-items: baseline; justify-content: space-between; margin-bottom: 5px; }
  .lm-label { font-size: 12px; font-weight: 600; color: var(--dg-text-dim); }
  .lm-value { font-family: 'JetBrains Mono', monospace; font-size: 16px; font-weight: 700; color: var(--dg-text); }
  .lm-track { height: 6px; background: var(--dg-bg-3); border-radius: 4px; overflow: hidden; }
  .lm-fill { height: 100%; border-radius: 4px; transition: width 0.6s cubic-bezier(0.4,0,0.2,1);
    &.low { background: linear-gradient(90deg, var(--accent), rgba(var(--accent-rgb), 0.5)); }
    &.medium { background: linear-gradient(90deg, var(--accent), var(--accent-light)); }
    &.high { background: linear-gradient(90deg, var(--dg-warning), #fbbf24); }
    &.critical { background: linear-gradient(90deg, var(--dg-danger), #f87171); }
  }
  .lm-detail { font-size: 11px; color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace; margin-top: 3px; }
}
.lm-stats { display: flex; flex-direction: column; gap: 6px; padding-top: 12px; border-top: 1px solid var(--dg-table-border); margin-top: 4px;
  .lm-stat { display: flex; justify-content: space-between; align-items: center; }
  .ls-label { font-size: 11px; color: var(--dg-text-faint); }
  .ls-val { font-size: 12px; color: var(--dg-text-dim); }
}
.gpu-list { margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--dg-table-border); display: flex; flex-direction: column; gap: 6px;
  .gpu-card { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font-size: 11px; .gpu-name { font-weight: 600; color: #c084fc; } .gpu-detail { color: var(--dg-text-dim); } .gpu-stat { color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace; margin-left: auto; } }
}

.hw-list { display: flex; flex-direction: column; gap: 10px;
  .hw-item { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px;
    .hw-label { font-size: 12px; color: var(--dg-text-faint); min-width: 60px; text-transform: uppercase; letter-spacing: 0.03em; flex-shrink: 0; }
    .hw-value { font-size: 13px; color: var(--dg-text); text-align: right; } }
}

.console-output { font-family: 'JetBrains Mono', monospace; font-size: 11px; line-height: 1.5; color: var(--dg-text-dim); white-space: pre-wrap; word-break: break-all; max-height: 350px; overflow-y: auto; margin: 0; padding: 8px; background: var(--dg-bg-3); border-radius: 8px; &.small { max-height: 250px; } }
.loading-text { padding: 20px; text-align: center; color: var(--dg-text-faint); font-size: 13px; }

.info-list { display: flex; flex-direction: column; gap: 8px; margin-bottom: 14px;
  .info-row { display: flex; justify-content: space-between; .ir-label { font-size: 12px; color: var(--dg-text-faint); } .ir-value { font-size: 13px; font-weight: 500; display: flex; align-items: center; gap: 5px; } }
}
.status-dot { width: 7px; height: 7px; border-radius: 50%; display: inline-block;
  &.online { background: var(--dg-success); box-shadow: 0 0 4px var(--dg-success); } &.offline { background: var(--dg-danger); } &.untrusted { background: var(--dg-warning); } &.error { background: var(--dg-danger); } }
.tag-list { display: flex; flex-wrap: wrap; gap: 5px; padding-top: 10px; border-top: 1px solid var(--dg-table-border);
  .chip { font-size: 11px; padding: 2px 8px; border-radius: 5px; background: var(--dg-bg-3); color: var(--dg-text-dim); } }

@media (max-width: 768px) { .detail-grid { grid-template-columns: 1fr; } }

/* Network check panels */
.card-header-row { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; .card-title { margin-bottom: 0; } }
.header-right-group { display: flex; align-items: center; gap: 10px; }
.last-tested { font-size: 10px; color: var(--dg-text-faint); }
.check-btn { padding: 5px 14px; border-radius: 8px; border: none; background: var(--accent); color: #fff; font-size: 12px; font-weight: 600; cursor: pointer; font-family: inherit; &:hover { background: var(--accent-dark); } &:disabled { opacity: 0.6; } }
.check-empty { padding: 16px; text-align: center; color: var(--dg-text-faint); font-size: 13px; }

/* Streaming grid */
.streaming-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 8px; }
.stream-item { display: flex; align-items: center; gap: 6px; padding: 8px 10px; border-radius: 8px; background: var(--dg-bg-3); font-size: 12px;
  &.UNLOCK, &.AVAILABLE { background: rgba(34,197,94,0.08); .si-icon { color: var(--dg-success); } }
  &.PARTIAL { background: rgba(245,158,11,0.08); .si-icon { color: var(--dg-warning); } }
  &.LOCKED, &.UNAVAILABLE { background: rgba(239,68,68,0.06); .si-icon { color: var(--dg-danger); } .si-name { opacity: 0.5; } }
  .si-icon { flex-shrink: 0; }
  .si-name { font-weight: 500; color: var(--dg-text); }
  .si-region { font-size: 10px; color: var(--dg-text-faint); margin-left: auto; }
  .si-status { font-size: 10px; color: var(--dg-text-faint); }
  &.OK { background: rgba(34,197,94,0.08); .si-icon { color: var(--dg-success); } }
  &.LIMITED { background: rgba(245,158,11,0.08); .si-icon { color: var(--dg-warning); } }
  &.BLOCKED { background: rgba(239,68,68,0.06); .si-icon { color: var(--dg-danger); } .si-name { opacity: 0.5; } }
}

/* Connectivity list */
.connectivity-list { display: flex; flex-direction: column; gap: 6px; }
.conn-item { display: flex; align-items: center; gap: 10px; padding: 8px 10px; border-radius: 8px; background: var(--dg-bg-3); font-size: 12px;
  .ci-region { font-weight: 500; min-width: 80px; color: var(--dg-text); }
  .ci-latency { font-family: 'JetBrains Mono', monospace; font-weight: 600; min-width: 50px; text-align: right;
    &.good { color: var(--dg-success); } &.ok { color: var(--dg-warning); } &.slow { color: var(--dg-danger); } &.bad { color: var(--dg-danger); } }
  .ci-loss { font-size: 10px; color: var(--dg-warning); &.bad { color: var(--dg-danger); } }
  .ci-bar { flex: 1; height: 4px; background: var(--dg-border); border-radius: 2px; overflow: hidden; .ci-bar-fill { height: 100%; border-radius: 2px; transition: width 0.3s; } }
}

/* Route results */
.route-results { display: flex; flex-direction: column; gap: 12px; }
.route-card { border: 1px solid var(--dg-border); border-radius: 10px; overflow: hidden;
  .rc-header { display: flex; align-items: center; gap: 10px; padding: 8px 12px; background: var(--dg-bg-3); font-size: 12px;
    .rc-isp { font-weight: 700; padding: 2px 8px; border-radius: 4px; font-size: 11px;
      &.ct { background: rgba(59,130,246,0.15); color: #60a5fa; }
      &.cu { background: rgba(239,68,68,0.15); color: #f87171; }
      &.cm { background: rgba(34,197,94,0.15); color: #4ade80; } }
    .rc-city { color: var(--dg-text-dim); }
    .rc-linetype { font-size: 10px; font-weight: 600; padding: 2px 6px; border-radius: 4px; background: var(--accent-tint); color: var(--accent); }
    .rc-ip { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: var(--dg-text-faint); margin-left: auto; }
    .rc-latency { font-family: 'JetBrains Mono', monospace; font-weight: 600;
      &.good { color: var(--dg-success); } &.ok { color: var(--dg-warning); } &.slow, &.bad { color: var(--dg-danger); } } }
  .rc-hops { padding: 8px 12px; max-height: 200px; overflow-y: auto;
    .hop-line { font-family: 'JetBrains Mono', monospace; font-size: 10px; color: var(--dg-text-dim); line-height: 1.6; } }
  .rc-nohops { padding: 8px 12px; font-size: 11px; color: var(--dg-text-faint); }
}
</style>
