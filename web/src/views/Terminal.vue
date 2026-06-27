<template>
  <div class="term-app">
    <!-- Sidebar -->
    <aside class="term-sidebar">
      <div class="sidebar-search">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none"><circle cx="11" cy="11" r="8" stroke="currentColor" stroke-width="2"/><path d="M21 21l-4.35-4.35" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
        <input v-model="search" placeholder="搜索节点..." @keydown.enter="connectFirst" />
      </div>
      <div class="sidebar-list">
        <button v-for="n in filteredNodes" :key="n.id" class="node-row" :class="{ online: n.status === 'online', connected: isConnected(n.id) }" :disabled="n.status !== 'online'" @click="openSession(n)">
          <span class="nr-dot" :class="n.status"></span>
          <div class="nr-info">
            <span class="nr-name">{{ n.name }}</span>
            <span class="nr-host">{{ n.host }}</span>
          </div>
          <span v-if="n.country_code" class="nr-flag">{{ flag(n.country_code) }}</span>
        </button>
        <div v-if="filteredNodes.length === 0" class="sidebar-empty">无匹配节点</div>
      </div>
    </aside>

    <!-- Main -->
    <div class="term-main">
      <!-- Tab Bar -->
      <div v-if="sessions.size > 0" class="tabbar">
        <div class="tabbar-scroll">
          <div v-for="[sid, s] in sessionList" :key="sid"
            class="tab" :class="{ active: isPrimaryActive(sid) || isSecondaryActive(sid), reconnectable: ['disconnected', 'error', 'reconnecting'].includes(s.status) }"
            @click="switchTo(sid)" @mousedown.middle.prevent="closeSession(sid)">
            <span class="tab-dot" :class="s.status"></span>
            <span class="tab-name">{{ s.node.name }}</span>
            <button v-if="['disconnected', 'error'].includes(s.status)" class="tab-reconnect" @click.stop="manualReconnect(sid)" title="重连">
              <svg viewBox="0 0 24 24" width="10" height="10" fill="none"><path d="M23 4v6h-6M1 20v-6h6" stroke="currentColor" stroke-width="2"/><path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15" stroke="currentColor" stroke-width="2"/></svg>
            </button>
            <button class="tab-x" @click.stop="closeSession(sid)">
              <svg viewBox="0 0 24 24" width="9" height="9" fill="none"><path d="M6 6l12 12M6 18L18 6" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/></svg>
            </button>
          </div>
        </div>
        <div class="tabbar-tools">
          <button v-if="sessions.size >= 2" class="tool-btn" :class="{ on: splitMode !== 'none' }" @click="toggleSplit" :title="splitMode === 'none' ? '分屏' : '取消分屏'">
            <svg viewBox="0 0 24 24" width="13" height="13" fill="none"><rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" stroke-width="1.8"/><path d="M12 3v18" stroke="currentColor" stroke-width="1.8"/></svg>
          </button>
        </div>
      </div>

      <!-- Terminal Panes -->
      <div v-if="sessions.size > 0" class="term-body" :class="splitMode">
        <!-- Primary -->
        <div class="pane primary-pane" :class="{ focused: focusedPane === 'primary' }" @click="focusedPane = 'primary'">
          <div class="pane-term-box">
            <div v-for="[sid, s] in sessionList" :key="sid"
              class="term-host" :ref="el => setPrimaryRef(el as HTMLElement, sid)"
              v-show="sid === primaryId"></div>
          </div>
        </div>
        <!-- Secondary -->
        <div v-if="splitMode !== 'none'" class="pane secondary-pane" :class="{ focused: focusedPane === 'secondary' }" @click="focusedPane = 'secondary'">
          <div class="pane-term-box">
            <div v-for="[sid, s] in sessionList" :key="'s'+sid"
              class="term-host" :ref="el => setSecondaryRef(el as HTMLElement, sid)"
              v-show="sid === secondaryId"></div>
          </div>
        </div>
      </div>

      <!-- Empty -->
      <div v-else class="term-welcome">
        <svg viewBox="0 0 24 24" width="40" height="40" fill="none" style="opacity:0.15;margin-bottom:12px"><rect x="2" y="4" width="20" height="16" rx="2" stroke="currentColor" stroke-width="1.5"/><path d="M6 9l3 3-3 3M12 15h4" stroke="currentColor" stroke-width="1.5"/></svg>
        <p class="wc-title">选择左侧节点开始终端会话</p>
        <p class="wc-sub">支持多 Tab · 分屏 · 快捷键</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { useRoute } from 'vue-router'
import { listNodes, type Node } from '@/api/nodes'
import { createTerminal } from '@/utils/terminal'
import { decodeBase64 } from '@/utils/codec'
import type { Terminal } from '@xterm/xterm'
import type { FitAddon } from '@xterm/addon-fit'

const route = useRoute()
const nodes = ref<Node[]>([])
const search = ref('')

interface Session {
  id: string
  node: Node
  term: Terminal | null
  fit: FitAddon | null
  ws: WebSocket | null
  status: 'connecting' | 'connected' | 'disconnected' | 'error' | 'reconnecting'
  reconnectAttempts: number
  maxReconnect: number
  intentionalClose: boolean
  container: HTMLElement | null
}
const sessions = reactive(new Map<string, Session>())
const sessionList = computed(() => Array.from(sessions.entries()))

const primaryId = ref<string | null>(null)
const secondaryId = ref<string | null>(null)
const focusedPane = ref<'primary' | 'secondary'>('primary')

type SplitMode = 'none' | 'horizontal' | 'vertical'
const splitMode = ref<SplitMode>('none')

const RECENT_KEY = 'dg_recent_terminals'
const recentIds = ref<string[]>(JSON.parse(localStorage.getItem(RECENT_KEY) || '[]'))

const filteredNodes = computed(() => {
  if (!search.value) return nodes.value
  const q = search.value.toLowerCase()
  return nodes.value.filter(n => n.name.toLowerCase().includes(q) || n.host.includes(q))
})

function isPrimaryActive(sid: string) { return sid === primaryId.value }
function isSecondaryActive(sid: string) { return sid === secondaryId.value }
function isConnected(nodeId: string) {
  for (const [, s] of sessions) { if (s.node.id === nodeId && s.status === 'connected') return true }
  return false
}
function flag(cc: string) { if (!cc || cc.length !== 2) return ''; return String.fromCodePoint(0x1F1E6 + cc.charCodeAt(0) - 65) + String.fromCodePoint(0x1F1E6 + cc.charCodeAt(1) - 65) }

// Element refs storage
const primaryRefs = new Map<string, HTMLElement>()
const secondaryRefs = new Map<string, HTMLElement>()

function setPrimaryRef(el: HTMLElement | null, sid: string) {
  if (el) primaryRefs.set(sid, el)
  else primaryRefs.delete(sid)
}
function setSecondaryRef(el: HTMLElement | null, sid: string) {
  if (el) secondaryRefs.set(sid, el)
  else secondaryRefs.delete(sid)
}

function connectFirst() {
  const n = filteredNodes.value.find(n => n.status === 'online')
  if (n) openSession(n)
}

function openSession(node: Node) {
  if (node.status !== 'online') return

  // Already open → switch
  for (const [sid, s] of sessions) {
    if (s.node.id === node.id) { switchTo(sid); return }
  }

  const sid = `s-${Date.now()}`
  sessions.set(sid, { id: sid, node, term: null, fit: null, ws: null, status: 'connecting', reconnectAttempts: 0, maxReconnect: 5, intentionalClose: false, container: null })
  recentIds.value = [node.id, ...recentIds.value.filter(x => x !== node.id)].slice(0, 10)
  localStorage.setItem(RECENT_KEY, JSON.stringify(recentIds.value))

  if (focusedPane.value === 'secondary' && splitMode.value !== 'none') {
    secondaryId.value = sid
  } else {
    primaryId.value = sid
  }

  nextTick(() => setTimeout(() => attachTerminal(sid), 50))
}

function attachTerminal(sid: string) {
  const s = sessions.get(sid)
  if (!s || s.term) return

  const pane = sid === secondaryId.value && splitMode.value !== 'none' ? 'secondary' : 'primary'
  const refMap = pane === 'primary' ? primaryRefs : secondaryRefs
  const container = refMap.get(sid)
  if (!container) { setTimeout(() => attachTerminal(sid), 50); return }

  const { term, fit } = createTerminal(container)
  s.term = term
  s.fit = fit
  s.container = container

  connectWS(sid)
  setTimeout(() => { try { fit.fit() } catch {} }, 80)
}

function connectWS(sid: string) {
  const s = sessions.get(sid)
  if (!s || !s.term) return

  s.status = s.reconnectAttempts > 0 ? 'reconnecting' : 'connecting'
  const token = localStorage.getItem('dg_token') || ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const ws = new WebSocket(`${proto}://${location.host}/ws/terminal/${s.node.id}`)
  s.ws = ws

  ws.onopen = () => {
    // Send auth token as first message (not in URL to prevent proxy log leakage)
    ws.send(JSON.stringify({ token: token }))
    s.status = 'connected'
    s.reconnectAttempts = 0
    s.term!.write('')
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols: s.term!.cols, rows: s.term!.rows }))
    }
  }
  ws.onmessage = (ev) => {
    try {
      const m = JSON.parse(ev.data)
      if (m.type === 'output') s.term!.write(decodeBase64(m.data))
      else if (m.type === 'error') s.term!.writeln(`\x1b[31m${m.data}\x1b[0m`)
    } catch {}
  }
  ws.onerror = () => { s.status = 'error' }
  ws.onclose = () => {
    if (s.intentionalClose) return
    s.status = 'disconnected'
    tryAutoReconnect(sid)
  }

  s.term!.onData((d) => { if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'input', data: d })) })
  s.term!.onResize(({ cols, rows }) => { if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'resize', cols, rows })) })
}

function tryAutoReconnect(sid: string) {
  const s = sessions.get(sid)
  if (!s || s.intentionalClose) return
  if (s.reconnectAttempts >= s.maxReconnect) {
    s.status = 'error'
    if (s.term) s.term.writeln(`\x1b[33m\r\n● 连接已断开，自动重连失败 (${s.reconnectAttempts} 次)\r\n● 请检查网络后手动重连\x1b[0m`)
    return
  }

  s.reconnectAttempts++
  s.status = 'reconnecting'
  const delay = Math.min(1000 * Math.pow(1.5, s.reconnectAttempts), 10000)
  if (s.term) s.term.writeln(`\x1b[33m\r\n● 连接断开，${(delay/1000).toFixed(0)}秒后重连 (第${s.reconnectAttempts}次)...\x1b[0m`)

  setTimeout(() => {
    const check = sessions.get(sid)
    if (!check || check.intentionalClose) return
    if (check.status !== 'connected') {
      connectWS(sid)
    }
  }, delay)
}

function manualReconnect(sid: string) {
  const s = sessions.get(sid)
  if (!s) return
  s.reconnectAttempts = 0
  s.intentionalClose = false
  if (s.ws) { s.ws.close(); s.ws = null }
  if (s.term) s.term.writeln('\x1b[36m\r\n● 正在重连...\x1b[0m')
  connectWS(sid)
}

function switchTo(sid: string) {
  if (focusedPane.value === 'secondary' && splitMode.value !== 'none') {
    secondaryId.value = sid
  } else {
    primaryId.value = sid
  }
  nextTick(() => setTimeout(() => fitSession(sid), 30))
}

function fitSession(sid: string) {
  const s = sessions.get(sid)
  if (s?.fit) { try { s.fit.fit() } catch {} }
}
function fitVisible() {
  if (primaryId.value) fitSession(primaryId.value)
  if (secondaryId.value && splitMode.value !== 'none') fitSession(secondaryId.value)
}

function closeSession(sid: string) {
  const s = sessions.get(sid)
  if (!s) return
  s.intentionalClose = true
  s.ws?.close()
  s.term?.dispose()
  sessions.delete(sid)
  primaryRefs.delete(sid)
  secondaryRefs.delete(sid)

  if (primaryId.value === sid) {
    const remaining = Array.from(sessions.keys())
    primaryId.value = remaining[0] || null
  }
  if (secondaryId.value === sid) secondaryId.value = null
  if (sessions.size === 0) splitMode.value = 'none'
  nextTick(() => setTimeout(fitVisible, 30))
}

function toggleSplit() {
  if (splitMode.value !== 'none') {
    splitMode.value = 'none'
    secondaryId.value = null
  } else {
    splitMode.value = 'horizontal'
    focusedPane.value = 'secondary'
    const others = Array.from(sessions.keys()).filter(id => id !== primaryId.value)
    if (others.length > 0) secondaryId.value = others[0]
  }
  nextTick(() => setTimeout(fitVisible, 50))
}

function onKeyDown(e: KeyboardEvent) {
  const mod = e.metaKey || e.ctrlKey
  if (!mod) return
  if (e.key === 'w') { e.preventDefault(); if (primaryId.value) closeSession(primaryId.value) }
  else if (e.key === 'd') { e.preventDefault(); toggleSplit() }
}
function onWinResize() { setTimeout(fitVisible, 100) }

const presetNode = (route.query.node as string) || ''
onMounted(async () => {
  nodes.value = await listNodes()
  window.addEventListener('keydown', onKeyDown)
  window.addEventListener('resize', onWinResize)
  if (presetNode) {
    const n = nodes.value.find(n => n.id === presetNode && n.status === 'online')
    if (n) openSession(n)
  }
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('resize', onWinResize)
  for (const [, s] of sessions) { s.ws?.close(); s.term?.dispose() }
})
watch(splitMode, () => nextTick(() => setTimeout(fitVisible, 50)))
</script>

<style scoped lang="scss">
.term-app { display: flex; height: 100%; overflow: hidden; }

/* Sidebar */
.term-sidebar { width: 240px; flex-shrink: 0; background: var(--dg-bg-2); border-right: 1px solid var(--dg-border); display: flex; flex-direction: column; overflow: hidden; }
.sidebar-search { display: flex; align-items: center; gap: 8px; padding: 10px 12px; border-bottom: 1px solid var(--dg-border); flex-shrink: 0;
  svg { color: var(--dg-text-faint); }
  input { flex: 1; border: none; outline: none; background: transparent; color: var(--dg-text); font-size: 13px; font-family: inherit; &::placeholder { color: var(--dg-text-faint); } }
}
.sidebar-list { flex: 1; overflow-y: auto; padding: 6px; }
.sidebar-empty { padding: 20px; text-align: center; color: var(--dg-text-faint); font-size: 13px; }
.node-row { display: flex; align-items: center; gap: 8px; width: 100%; padding: 8px 10px; border-radius: 8px; border: none; background: transparent; color: var(--dg-text-dim); font-size: 13px; cursor: pointer; transition: background 0.15s; font-family: inherit; text-align: left; margin-bottom: 2px;
  &:hover { background: var(--dg-bg-3); color: var(--dg-text); }
  &:not(.online) { opacity: 0.35; cursor: not-allowed; }
  &.connected { background: var(--accent-tint); .nr-name { color: var(--accent); } }
  .nr-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0;
    &.online { background: var(--dg-success); box-shadow: 0 0 4px var(--dg-success); }
    &.offline { background: var(--dg-danger); } &.untrusted { background: var(--dg-warning); } &.error { background: var(--dg-danger); } }
  .nr-info { flex: 1; min-width: 0; .nr-name { display: block; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .nr-host { display: block; font-size: 11px; color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace; } }
  .nr-flag { font-size: 13px; flex-shrink: 0; }
}

/* Main */
.term-main { flex: 1; display: flex; flex-direction: column; overflow: hidden; background: #0e0e1c; min-width: 0; }

/* Tabs */
.tabbar { display: flex; align-items: center; height: 36px; background: var(--dg-bg-2); border-bottom: 1px solid var(--dg-border); flex-shrink: 0; }
.tabbar-scroll { display: flex; align-items: center; gap: 1px; flex: 1; overflow-x: auto; padding: 0 4px; &::-webkit-scrollbar { height: 0; } }
.tab { display: flex; align-items: center; gap: 6px; padding: 4px 8px 4px 10px; border-radius: 6px; cursor: pointer; white-space: nowrap; flex-shrink: 0; max-width: 180px;
  &:hover { background: var(--dg-bg-3); }
  &.active { background: var(--accent-tint); .tab-name { color: var(--accent); font-weight: 600; } }
  .tab-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0;
    &.connected { background: var(--dg-success); } &.connecting { background: var(--dg-warning); animation: blink 1.2s infinite; }
    &.disconnected { background: var(--dg-text-faint); } &.error { background: var(--dg-danger); }
    &.reconnecting { background: var(--dg-warning); animation: blink 1s infinite; } }
  .tab-name { font-size: 12px; color: var(--dg-text-dim); overflow: hidden; text-overflow: ellipsis; }
  .tab-x { width: 15px; height: 15px; border-radius: 4px; border: none; background: transparent; color: var(--dg-text-faint); cursor: pointer; display: flex; align-items: center; justify-content: center;
    &:hover { background: var(--dg-danger); color: #fff; } }
  .tab-reconnect { width: 15px; height: 15px; border-radius: 4px; border: none; background: transparent; color: var(--dg-warning); cursor: pointer; display: flex; align-items: center; justify-content: center;
    &:hover { background: var(--dg-warning); color: #fff; animation: spin 0.6s linear infinite; } }
}
@keyframes blink { 0%,50%{opacity:1} 51%,100%{opacity:0.3} }
.tabbar-tools { display: flex; align-items: center; gap: 2px; padding: 0 6px; flex-shrink: 0; border-left: 1px solid var(--dg-border); height: 100%; }
.tool-btn { width: 26px; height: 26px; border-radius: 6px; border: none; background: transparent; color: var(--dg-text-faint); cursor: pointer; display: flex; align-items: center; justify-content: center;
  &:hover { background: var(--dg-bg-3); color: var(--dg-text); }
  &.on { color: var(--accent); background: var(--accent-tint); } }

/* Terminal Body — key fix: flex-basis 0 + min-height/width 0 */
.term-body { flex: 1 1 0; display: flex; min-height: 0; min-width: 0; overflow: hidden;
  &.horizontal { flex-direction: row; }
  &.vertical { flex-direction: column; }
}
.pane { flex: 1 1 0; min-width: 0; min-height: 0; overflow: hidden; position: relative;
  &.focused { box-shadow: inset 0 0 0 1.5px var(--accent); }
}
.term-body.horizontal > .primary-pane { border-right: 2px solid var(--dg-border); }
.term-body.vertical > .primary-pane { border-bottom: 2px solid var(--dg-border); }

/* Terminal host: fills pane, hidden ones use display:none */
.pane-term-box { width: 100%; height: 100%; }
.term-host { width: 100%; height: 100%; }

:deep(.xterm) { width: 100%; height: 100%; }
:deep(.xterm .xterm-viewport) { background-color: transparent !important; }

/* Welcome */
.term-welcome { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  .wc-title { font-size: 15px; font-weight: 600; color: var(--dg-text-dim); margin-bottom: 4px; }
  .wc-sub { font-size: 12px; color: var(--dg-text-faint); }
}
</style>
