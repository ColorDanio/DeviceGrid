<template>
  <div class="page-container">
    <div class="page-header">
      <div><h2>系统设置</h2><p class="page-subtitle">外观偏好、账户与用户管理</p></div>
    </div>

    <div class="settings-grid">
      <!-- Appearance -->
      <div class="dg-card s-card">
        <h3 class="card-title">外观</h3>
        <div class="setting-row">
          <div class="setting-label"><span class="ls-name">主题模式</span><span class="ls-desc">深色 / 浅色 / 跟随系统</span></div>
          <div class="mode-tabs">
            <button v-for="m in modes" :key="m.id" class="mode-tab" :class="{ active: theme.mode === m.id }" @click="theme.setMode(m.id)">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" v-html="m.icon"></svg>{{ m.label }}
            </button>
          </div>
        </div>
        <div class="setting-row">
          <div class="setting-label"><span class="ls-name">主题色</span><span class="ls-desc">界面强调色和整体配色</span></div>
          <div class="accent-grid">
            <button v-for="(a, key) in accents" :key="key" class="accent-swatch" :class="{ active: theme.accent === key }" :style="{ '--swatch': a.primary }" :title="a.name" @click="theme.setAccent(key as any)">
              <span class="swatch-dot"></span>
            </button>
          </div>
        </div>
      </div>

      <!-- Profile -->
      <div class="dg-card s-card">
        <h3 class="card-title">当前账户</h3>
        <div class="profile-row">
          <div class="profile-avatar">{{ auth.username.charAt(0).toUpperCase() }}</div>
          <div><div class="profile-name">{{ auth.username }}</div><div class="profile-role">{{ roleLabel }}</div></div>
        </div>
        <div class="info-list">
          <div class="info-row"><span class="ir-label">角色</span><span class="ir-value">{{ roleLabel }}</span></div>
          <div class="info-row"><span class="ir-label">令牌</span><span class="ir-value mono">••••••••</span></div>
        </div>
      </div>

      <!-- System Info -->
      <div class="dg-card s-card">
        <h3 class="card-title">系统信息</h3>
        <div class="info-list">
          <div class="info-row"><span class="ir-label">后端</span><span class="ir-value">Go + Gin</span></div>
          <div class="info-row"><span class="ir-label">数据库</span><span class="ir-value">SQLite</span></div>
          <div class="info-row"><span class="ir-label">通信</span><span class="ir-value">SSH + Agent gRPC</span></div>
          <div class="info-row"><span class="ir-label">版本</span><span class="ir-value">v1.0.0</span></div>
        </div>
      </div>
    </div>

    <!-- User Management -->
    <div class="dg-card user-panel">
      <div class="panel-header">
        <h3 class="card-title">用户管理</h3>
        <button class="btn-primary" @click="showCreateDialog">
          <svg viewBox="0 0 24 24" width="13" height="13" fill="none"><path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
          添加用户
        </button>
      </div>
      <div v-loading="usersLoading" class="user-table-wrap">
        <div v-if="users.length === 0 && !usersLoading" class="empty-text">暂无用户</div>
        <div v-else class="user-list">
          <div v-for="u in users" :key="u.id" class="user-row">
            <div class="u-avatar" :style="{ background: avatarColor(u.username) }">{{ u.username.charAt(0).toUpperCase() }}</div>
            <div class="u-info">
              <div class="u-name">{{ u.username }}<span v-if="u.id === currentUserId" class="u-self">（你）</span></div>
              <div class="u-meta">创建于 {{ formatTime(u.created_at) }}</div>
            </div>
            <span class="u-role-badge" :class="u.role">{{ roleMap[u.role] || u.role }}</span>
            <button v-if="u.id !== currentUserId" class="u-delete" @click="handleDelete(u)">
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none"><path d="M3 6h18M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2m3 0v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6" stroke="currentColor" stroke-width="1.8"/></svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Create User Dialog -->
    <el-dialog v-model="createVisible" title="添加用户" width="420px">
      <el-form label-position="top" class="user-form">
        <el-form-item label="用户名">
          <el-input v-model="newUser.username" placeholder="输入用户名" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="newUser.password" type="password" show-password placeholder="输入密码" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="newUser.role" style="width: 100%">
            <el-option label="管理员 (完全权限)" value="admin" />
            <el-option label="操作员 (操作权限)" value="operator" />
            <el-option label="观察者 (只读)" value="viewer" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore, ACCENTS } from '@/stores/theme'
import { listUsers, createUser, deleteUser, type User } from '@/api/users'

const auth = useAuthStore()
const theme = useThemeStore()
const accents = ACCENTS

const modes = [
  { id: 'dark' as const, label: '深色', icon: '<path d="M21 12.8A9 9 0 1111.2 3 7 7 0 0021 12.8z" stroke="currentColor" stroke-width="1.8"/>' },
  { id: 'light' as const, label: '浅色', icon: '<circle cx="12" cy="12" r="4" stroke="currentColor" stroke-width="1.8"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>' },
  { id: 'system' as const, label: '跟随', icon: '<rect x="3" y="3" width="18" height="18" rx="3" stroke="currentColor" stroke-width="1.8"/><path d="M15 3v18" stroke="currentColor" stroke-width="1.8"/>' },
]

const roleMap: Record<string, string> = { admin: '管理员', operator: '操作员', viewer: '观察者' }
const roleLabel = computed(() => roleMap[auth.role] || auth.role)

const users = ref<User[]>([])
const usersLoading = ref(false)
const currentUserId = computed(() => auth.token ? 'current' : '')
const createVisible = ref(false)
const creating = ref(false)
const newUser = ref({ username: '', password: '', role: 'operator' })

function avatarColor(name: string) {
  const colors = ['var(--accent)', 'var(--dg-success)', 'var(--dg-warning)', 'var(--dg-danger)']
  return colors[name.charCodeAt(0) % colors.length]
}
function formatTime(t: string) { if (!t || t.startsWith('0001')) return '-'; return new Date(t).toLocaleString() }

async function loadUsers() {
  usersLoading.value = true
  try { users.value = await listUsers() } catch {} finally { usersLoading.value = false }
}

function showCreateDialog() { newUser.value = { username: '', password: '', role: 'operator' }; createVisible.value = true }

async function handleCreate() {
  if (!newUser.value.username || !newUser.value.password) { ElMessage.warning('请填写用户名和密码'); return }
  creating.value = true
  try {
    await createUser(newUser.value.username, newUser.value.password, newUser.value.role)
    ElMessage.success('用户创建成功')
    createVisible.value = false
    loadUsers()
  } catch {} finally { creating.value = false }
}

async function handleDelete(u: User) {
  await ElMessageBox.confirm(`确定删除用户 ${u.username}？`, '', { type: 'warning' })
  await deleteUser(u.id)
  ElMessage.success('已删除')
  loadUsers()
}

onMounted(() => loadUsers())
</script>

<style scoped lang="scss">
.settings-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 16px; margin-bottom: 16px; }
.s-card { padding: 22px; }
.card-title { font-size: 14px; font-weight: 600; margin-bottom: 16px; }

.setting-row { display: flex; justify-content: space-between; align-items: center; padding: 12px 0; border-top: 1px solid var(--dg-table-border); &:first-of-type { border-top: none; padding-top: 0; } }
.setting-label { display: flex; flex-direction: column; .ls-name { font-size: 13px; font-weight: 500; } .ls-desc { font-size: 11px; color: var(--dg-text-faint); margin-top: 2px; } }

.mode-tabs { display: flex; gap: 3px; background: var(--dg-bg-3); padding: 3px; border-radius: 8px;
  .mode-tab { display: flex; align-items: center; gap: 5px; padding: 5px 10px; border-radius: 6px; border: none; background: transparent; color: var(--dg-text-dim); font-size: 12px; font-weight: 500; cursor: pointer; transition: all 0.15s; font-family: inherit;
    &.active { background: var(--accent); color: #fff; } } }

.accent-grid { display: flex; gap: 8px; }
.accent-swatch { width: 26px; height: 26px; border-radius: 50%; border: 2px solid var(--dg-border); background: var(--dg-bg-3); cursor: pointer; display: flex; align-items: center; justify-content: center; transition: all 0.2s; padding: 0;
  .swatch-dot { width: 12px; height: 12px; border-radius: 50%; background: var(--swatch); }
  &.active { border-color: var(--swatch); transform: scale(1.15); } }

.profile-row { display: flex; align-items: center; gap: 12px; padding-bottom: 16px; margin-bottom: 16px; border-bottom: 1px solid var(--dg-table-border);
  .profile-avatar { width: 42px; height: 42px; border-radius: 11px; background: var(--accent); color: #fff; display: flex; align-items: center; justify-content: center; font-size: 17px; font-weight: 700; }
  .profile-name { font-size: 14px; font-weight: 600; } .profile-role { font-size: 12px; color: var(--dg-text-faint); margin-top: 2px; } }

.info-list { display: flex; flex-direction: column; gap: 10px; }
.info-row { display: flex; justify-content: space-between; .ir-label { font-size: 12px; color: var(--dg-text-faint); } .ir-value { font-size: 13px; font-weight: 500; } .mono { font-family: 'JetBrains Mono', monospace; } }

/* User Panel */
.user-panel { padding: 20px; }
.panel-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; .card-title { margin-bottom: 0; } }
.btn-primary { display: flex; align-items: center; gap: 5px; padding: 7px 14px; border: none; border-radius: 8px; background: var(--accent); color: #fff; font-size: 12px; font-weight: 600; cursor: pointer; font-family: inherit; &:hover { background: var(--accent-dark); } }

.user-table-wrap { min-height: 100px; }
.empty-text { padding: 30px; text-align: center; color: var(--dg-text-faint); font-size: 13px; }
.user-list { display: flex; flex-direction: column; gap: 6px; }
.user-row { display: flex; align-items: center; gap: 12px; padding: 10px 14px; border-radius: 10px; background: var(--dg-bg-3); transition: all 0.15s; &:hover { background: var(--dg-surface-hover); }
  .u-avatar { width: 32px; height: 32px; border-radius: 8px; display: flex; align-items: center; justify-content: center; color: #fff; font-weight: 700; font-size: 13px; flex-shrink: 0; }
  .u-info { flex: 1; .u-name { font-size: 13px; font-weight: 600; .u-self { font-size: 11px; color: var(--dg-text-faint); font-weight: 400; } } .u-meta { font-size: 11px; color: var(--dg-text-faint); margin-top: 2px; } }
  .u-role-badge { font-size: 11px; font-weight: 600; padding: 3px 10px; border-radius: 6px;
    &.admin { background: var(--dg-danger-bg); color: var(--dg-danger); } &.operator { background: var(--dg-warning-bg); color: var(--dg-warning); } &.viewer { background: var(--accent-tint); color: var(--accent); } }
  .u-delete { width: 28px; height: 28px; border-radius: 7px; border: 1px solid var(--dg-border); background: transparent; color: var(--dg-text-faint); cursor: pointer; display: flex; align-items: center; justify-content: center; &:hover { border-color: var(--dg-danger); color: var(--dg-danger); } } }

.user-form { .el-form-item { margin-bottom: 16px; } }
</style>
