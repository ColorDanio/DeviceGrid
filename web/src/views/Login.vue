<template>
  <div class="login-page">
    <div class="grid-bg"></div>
    <div class="login-card">
      <div class="form-section">
        <div class="logo-area">
          <div class="logo-icon"><svg viewBox="0 0 24 24" width="24" height="24" fill="none"><path d="M3 3h7v7H3V3zm0 11h7v7H3v-7zm11-11h7v7h-7V3zm0 11h7v7h-7v-7z" fill="currentColor"/></svg></div>
          <span class="logo-text">DeviceGrid</span>
        </div>
        <h1 class="form-title">服务器集群<br/>智能管控平台</h1>
        <p class="form-desc">SSH + Agent 双通道 · 实时监控 · Docker 管理 · RKE2 编排</p>
        <form @submit.prevent="handleLogin" class="login-form">
          <div class="input-group">
            <input v-model="form.username" type="text" placeholder="用户名" autocomplete="username" :class="{ focused: focused === 'user' }" @focus="focused = 'user'" @blur="focused = ''" />
          </div>
          <div class="input-group">
            <input v-model="form.password" :type="showPwd ? 'text' : 'password'" placeholder="密码" autocomplete="current-password" :class="{ focused: focused === 'pwd' }" @focus="focused = 'pwd'" @blur="focused = ''" />
            <button type="button" class="pwd-btn" @click="showPwd = !showPwd">{{ showPwd ? '隐藏' : '显示' }}</button>
          </div>
          <button type="submit" class="submit-btn" :disabled="loading">
            <span v-if="!loading">登 录</span>
            <span v-else class="loading"></span>
          </button>
        </form>
        <div class="hint">默认账户 <strong>admin</strong> / <strong>admin123</strong></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const loading = ref(false)
const showPwd = ref(false)
const focused = ref('')
const form = reactive({ username: 'admin', password: 'admin123' })

async function handleLogin() {
  if (!form.username || !form.password) { ElMessage.warning('请输入用户名和密码'); return }
  loading.value = true
  try { await auth.login(form); router.push('/kanban') } catch {} finally { loading.value = false }
}
</script>

<style scoped lang="scss">
.login-page {
  height: 100vh; display: flex; align-items: center; justify-content: center;
  background: var(--dg-bg); position: relative; overflow: hidden;
  background-image: radial-gradient(ellipse 60% 50% at 50% 0%, var(--dg-glow-overlay-1), transparent),
                    radial-gradient(ellipse 50% 40% at 100% 100%, var(--dg-glow-overlay-2), transparent);
}
.grid-bg {
  position: absolute; inset: 0;
  background-image: linear-gradient(var(--dg-border) 1px, transparent 1px), linear-gradient(90deg, var(--dg-border) 1px, transparent 1px);
  background-size: 40px 40px;
  mask-image: radial-gradient(ellipse 70% 60% at center, black 30%, transparent 80%);
  -webkit-mask-image: radial-gradient(ellipse 70% 60% at center, black 30%, transparent 80%);
}

.login-card {
  width: 420px; max-width: 92%; background: var(--dg-table-header-bg); backdrop-filter: blur(24px);
  border: 1px solid var(--dg-border); border-radius: 20px; padding: 44px; z-index: 1;
  box-shadow: 0 20px 60px rgba(0,0,0,0.4), 0 0 80px rgba(59,130,246,0.05);
}

.logo-area { display: flex; align-items: center; gap: 10px; margin-bottom: 36px;
  .logo-icon { width: 36px; height: 36px; border-radius: 10px; background: linear-gradient(135deg, var(--dg-blue), var(--dg-indigo)); display: flex; align-items: center; justify-content: center; color: #fff; box-shadow: 0 0 20px rgba(59,130,246,0.3); }
  .logo-text { font-size: 17px; font-weight: 700; color: var(--dg-text); }
}
.form-title { font-size: 24px; font-weight: 700; color: var(--dg-text); line-height: 1.3; margin-bottom: 8px; letter-spacing: -0.02em; }
.form-desc { font-size: 13px; color: var(--dg-text-dim); margin-bottom: 32px; }

.login-form { display: flex; flex-direction: column; gap: 14px; }
.input-group { position: relative;
  input {
    width: 100%; height: 46px; padding: 0 16px; border: 1px solid var(--dg-border); border-radius: 12px;
    background: var(--dg-bg-2); color: var(--dg-text); font-size: 14px; font-family: inherit; outline: none; transition: all 0.2s;
    &::placeholder { color: var(--dg-text-faint); }
    &.focused { border-color: var(--dg-cyan); box-shadow: 0 0 0 3px var(--dg-table-row-hover); }
  }
  .pwd-btn { position: absolute; right: 12px; top: 50%; transform: translateY(-50%); background: none; border: none; color: var(--dg-text-faint); font-size: 12px; cursor: pointer; font-family: inherit; &:hover { color: var(--dg-cyan); } }
}
.submit-btn {
  width: 100%; height: 48px; border: none; border-radius: 12px;
  background: linear-gradient(135deg, var(--dg-blue), var(--dg-indigo)); color: #fff; font-size: 15px; font-weight: 600;
  cursor: pointer; transition: all 0.2s; font-family: inherit; margin-top: 8px;
  &:hover:not(:disabled) { box-shadow: 0 0 24px rgba(59,130,246,0.3); transform: translateY(-1px); }
  &:disabled { opacity: 0.6; }
}
.loading { display: inline-block; width: 18px; height: 18px; border: 2px solid rgba(255,255,255,0.3); border-top-color: #fff; border-radius: 50%; animation: spin 0.6s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.hint { margin-top: 20px; padding: 10px 16px; background: var(--dg-bg-2); border: 1px solid var(--dg-border); border-radius: 10px; font-size: 12px; color: var(--dg-text-dim); text-align: center;
  strong { color: var(--dg-cyan); font-weight: 600; }
}
</style>
