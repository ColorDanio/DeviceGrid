<template>
  <div class="app-layout">
    <aside class="sidebar" :class="{ collapsed: auth.sidebarCollapsed }">
      <div class="sidebar-brand">
        <div class="brand-icon"><svg viewBox="0 0 24 24" width="20" height="20" fill="none"><path d="M3 3h7v7H3V3zm0 11h7v7H3v-7zm11-11h7v7h-7V3zm0 11h7v7h-7v-7z" fill="currentColor"/></svg></div>
        <transition name="fade"><span v-show="!auth.sidebarCollapsed" class="brand-name">DeviceGrid</span></transition>
      </div>
      <div class="brand-line"></div>
      <nav class="sidebar-nav">
        <router-link v-for="item in menuItems" :key="item.path" :to="item.path" class="nav-link" active-class="active">
          <div class="nav-icon" v-html="item.svg"></div>
          <transition name="fade"><span v-show="!auth.sidebarCollapsed" class="nav-text">{{ item.title }}</span></transition>
          <div class="nav-indicator"></div>
        </router-link>
      </nav>
      <div class="sidebar-footer">
        <button class="collapse-btn" @click="auth.toggleSidebar()">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none"><path v-if="!auth.sidebarCollapsed" d="M11 17l-5-5 5-5M18 17l-5-5 5-5" stroke="currentColor" stroke-width="2" stroke-linecap="round"/><path v-else d="M13 17l5-5-5-5M6 17l5-5-5-5" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
        </button>
      </div>
    </aside>

    <div class="main-area">
      <header class="app-header">
        <h1 class="header-title">{{ currentTitle }}</h1>
        <div class="header-right">
          <div class="stat-pills">
            <div class="pill" v-for="s in quickStats" :key="s.label">
              <span class="pill-dot" :style="{ background: s.color, boxShadow: `0 0 6px ${s.color}` }"></span>
              <span class="pill-val">{{ s.value }}</span>
              <span class="pill-lbl">{{ s.label }}</span>
            </div>
          </div>
          <div class="header-div"></div>
          <ThemeSwitcher />
          <div class="header-div"></div>
          <el-dropdown @command="handleCommand" trigger="click">
            <div class="user-chip">
              <div class="user-avatar">{{ auth.username.charAt(0).toUpperCase() }}</div>
              <div class="user-meta"><span class="user-name">{{ auth.username }}</span><span class="user-role">{{ roleLabel }}</span></div>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="settings">系统设置</el-dropdown-item>
                <el-dropdown-item divided command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>
      <main class="app-content">
        <router-view v-slot="{ Component }"><transition name="fade-slide" mode="out-in"><component :is="Component" /></transition></router-view>
      </main>
      <footer class="app-footer">
        <span class="footer-copy">© 2024-2026 DeviceGrid</span>
        <span class="footer-sep">·</span>
        <span class="footer-ver" v-if="appVersion">v{{ appVersion }}</span>
        <a href="https://github.com/ColorDanio/DeviceGrid" target="_blank" class="footer-link">GitHub</a>
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { listNodes, type Node } from '@/api/nodes'
import ThemeSwitcher from '@/components/ThemeSwitcher.vue'
import client from '@/api/client'

const appVersion = ref('')
async function loadVersion() {
  try {
    const { data } = await client.get('/version')
    appVersion.value = data.version || 'dev'
  } catch {}
}

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const nodes = ref<Node[]>([])

const icons: Record<string, string> = {
  Kanban: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><rect x="3" y="3" width="7" height="9" rx="1.5" stroke="currentColor" stroke-width="1.8"/><rect x="14" y="3" width="7" height="5" rx="1.5" stroke="currentColor" stroke-width="1.8"/><rect x="14" y="12" width="7" height="9" rx="1.5" stroke="currentColor" stroke-width="1.8"/><rect x="3" y="16" width="7" height="5" rx="1.5" stroke="currentColor" stroke-width="1.8"/></svg>',
  Nodes: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><rect x="2" y="3" width="20" height="6" rx="2" stroke="currentColor" stroke-width="1.8"/><rect x="2" y="15" width="20" height="6" rx="2" stroke="currentColor" stroke-width="1.8"/><circle cx="6" cy="6" r="1" fill="currentColor"/><circle cx="6" cy="18" r="1" fill="currentColor"/></svg>',
  Deploy: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  Docker: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><path d="M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z" stroke="currentColor" stroke-width="1.8"/><path d="M3.27 6.96L12 12.01l8.73-5.05M12 22.08V12" stroke="currentColor" stroke-width="1.8"/></svg>',
  Clusters: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="1.8"/><circle cx="5" cy="5" r="2" stroke="currentColor" stroke-width="1.8"/><circle cx="19" cy="5" r="2" stroke="currentColor" stroke-width="1.8"/><circle cx="5" cy="19" r="2" stroke="currentColor" stroke-width="1.8"/><circle cx="19" cy="19" r="2" stroke="currentColor" stroke-width="1.8"/><path d="M6.5 6.5l3 3M17.5 6.5l-3 3M6.5 17.5l3-3M17.5 17.5l-3-3" stroke="currentColor" stroke-width="1.5"/></svg>',
  Terminal: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><rect x="2" y="4" width="20" height="16" rx="2" stroke="currentColor" stroke-width="1.8"/><path d="M6 9l3 3-3 3M12 15h4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>',
  SFTP: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><path d="M3 7l2-3h6l2 3h8v12H3V7z" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/></svg>',
  Settings: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="1.8"/><path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>',
  Automation: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 01-3.4 0" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  SSHKeys: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><circle cx="8" cy="15" r="4" stroke="currentColor" stroke-width="1.8"/><path d="M10.85 12.15L19 4M18 5l2 2M15 8l2 2" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>',
  Switch: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><path d="M16 3l4 4-4 4M20 7H4M8 21l-4-4 4-4M4 17h16" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  Document: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" stroke="currentColor" stroke-width="1.8"/><path d="M14 2v6h6M8 13h8M8 17h8M8 9h2" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>',
}

const menuItems = computed(() =>
  router.options.routes.flatMap((r) => r.children || []).filter((r) => !r.meta?.hidden).map((r) => ({
    path: r.path.startsWith('/') ? r.path : `/${r.path}`, title: r.meta?.title || '',
    svg: icons[r.name as string] || '',
  })),
)
const currentTitle = computed(() => (route.meta?.title as string) || 'DeviceGrid')
const roleLabel = computed(() => ({ admin: '管理员', operator: '操作员', viewer: '观察者' }[auth.role] || auth.role))
const quickStats = computed(() => [
  { label: '在线', value: nodes.value.filter((n) => n.status === 'online').length, color: '#22c55e' },
  { label: '离线', value: nodes.value.filter((n) => n.status === 'offline' || n.status === 'error').length, color: '#ef4444' },
  { label: '总数', value: nodes.value.length, color: '#22d3ee' },
])

function handleCommand(cmd: string) {
  if (cmd === 'logout') {
    ElMessageBox.confirm('确定要退出登录吗？', '', { confirmButtonText: '退出', cancelButtonText: '取消', type: 'warning' })
      .then(() => { auth.logout(); router.push('/login') }).catch(() => {})
  } else if (cmd === 'settings') router.push('/settings')
}

let pt: ReturnType<typeof setInterval> | null = null
onMounted(() => { loadVersion(); (async () => { try { nodes.value = await listNodes() } catch {} })(); pt = setInterval(async () => { try { nodes.value = await listNodes() } catch {} }, 15000) })
onBeforeUnmount(() => { if (pt) clearInterval(pt) })
</script>

<style scoped lang="scss">
.app-layout { display: flex; height: 100vh; overflow: hidden; }

.sidebar {
  width: var(--dg-sidebar-w); background: var(--dg-bg-2); backdrop-filter: blur(20px);
  display: flex; flex-direction: column; transition: width 0.3s cubic-bezier(0.4,0,0.2,1); z-index: 100; flex-shrink: 0;
  border-right: 1px solid var(--dg-border); &.collapsed { width: var(--dg-sidebar-collapsed); }
}
.sidebar-brand {
  display: flex; align-items: center; gap: 10px; padding: 18px 20px; height: var(--dg-header-h); overflow: hidden; flex-shrink: 0;
  .brand-icon {
    width: 34px; height: 34px; border-radius: 9px; background: var(--accent);
    display: flex; align-items: center; justify-content: center; color: #fff; flex-shrink: 0;
    box-shadow: 0 0 16px rgba(59, 130, 246, 0.3);
  }
  .brand-name { font-size: 16px; font-weight: 700; color: var(--dg-text); letter-spacing: -0.01em; white-space: nowrap; }
}
.brand-line { height: 1px; background: linear-gradient(90deg, transparent, var(--dg-border), transparent); margin: 0 16px; }

.sidebar-nav { flex: 1; padding: 8px 10px; overflow-y: auto; overflow-x: hidden;
  .nav-link {
    display: flex; align-items: center; gap: 12px; padding: 10px 12px; border-radius: 10px;
    color: var(--dg-text-dim); text-decoration: none; transition: all 0.2s; margin-bottom: 2px; white-space: nowrap; position: relative;
    &:hover { color: var(--dg-cyan); background: var(--dg-table-row-hover); }
    &.active { color: var(--dg-cyan); background: var(--dg-table-row-hover);
      .nav-indicator { opacity: 1; }
      .nav-icon { filter: drop-shadow(0 0 4px var(--dg-cyan)); }
    }
    .nav-icon { display: flex; align-items: center; width: 20px; height: 20px; flex-shrink: 0; transition: filter 0.2s; }
    .nav-text { font-size: 13px; font-weight: 500; }
    .nav-indicator { position: absolute; left: 0; top: 50%; transform: translateY(-50%); width: 3px; height: 20px; background: var(--dg-cyan); border-radius: 0 3px 3px 0; opacity: 0; transition: opacity 0.2s; box-shadow: 0 0 8px var(--dg-cyan); }
  }
}
.sidebar-footer { padding: 8px 10px 12px; flex-shrink: 0;
  .collapse-btn {
    width: 100%; height: 34px; display: flex; align-items: center; justify-content: center; border-radius: 8px;
    background: transparent; border: 1px solid var(--dg-border); color: var(--dg-text-faint); cursor: pointer; transition: all 0.2s;
    &:hover { border-color: var(--dg-border-bright); color: var(--dg-cyan); }
  }
}

.main-area { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.app-header {
  height: var(--dg-header-h); background: var(--dg-bg-2); backdrop-filter: blur(16px);
  display: flex; align-items: center; justify-content: space-between; padding: 0 28px;
  border-bottom: 1px solid var(--dg-border); z-index: 50; flex-shrink: 0;
  .header-title { font-size: 17px; font-weight: 600; color: var(--dg-text); letter-spacing: -0.01em; }
  .header-right { display: flex; align-items: center; gap: 14px; }
  .stat-pills { display: flex; gap: 6px; }
  .pill {
    display: flex; align-items: center; gap: 5px; padding: 4px 10px; background: var(--dg-table-header-bg);
    border: 1px solid var(--dg-border); border-radius: 20px; font-size: 12px;
    .pill-dot { width: 6px; height: 6px; border-radius: 50%; }
    .pill-val { font-weight: 700; color: var(--dg-text); }
    .pill-lbl { color: var(--dg-text-faint); font-size: 11px; }
  }
  .header-div { width: 1px; height: 24px; background: var(--dg-border); }
  .user-chip {
    display: flex; align-items: center; gap: 10px; cursor: pointer; padding: 4px 12px 4px 4px; border-radius: 30px; transition: all 0.2s;
    &:hover { background: var(--dg-table-row-hover); }
    .user-avatar {
      width: 32px; height: 32px; border-radius: 50%; background: var(--accent);
      color: #fff; display: flex; align-items: center; justify-content: center; font-weight: 600; font-size: 13px;
    }
    .user-meta { display: flex; flex-direction: column;
      .user-name { font-size: 13px; font-weight: 600; color: var(--dg-text); line-height: 1.2; }
      .user-role { font-size: 11px; color: var(--dg-text-faint); }
    }
  }
}
.app-content { flex: 1; overflow-y: auto; }

.app-footer {
  height: 28px;
  background: var(--dg-bg-2);
  border-top: 1px solid var(--dg-border);
  display: flex; align-items: center; justify-content: center; gap: 8px;
  font-size: 11px; color: var(--dg-text-faint); flex-shrink: 0;
  .footer-sep { opacity: 0.4; }
  .footer-ver { font-family: 'JetBrains Mono', monospace; }
  .footer-link { color: var(--dg-text-faint); text-decoration: none; &:hover { color: var(--accent); } }
}
.fade-enter-active, .fade-leave-active { transition: opacity 0.15s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
