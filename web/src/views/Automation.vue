<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title-group">
        <h2>自动化运维</h2>
        <p class="page-subtitle">告警规则与定时任务管理</p>
      </div>
    </div>

    <!-- Tabs -->
    <div class="auto-tabs">
      <div class="auto-tab" :class="{active: tab==='alerts'}" @click="tab='alerts'">
        <svg viewBox="0 0 24 24" width="15" height="15" fill="none"><path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 01-3.4 0" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
        告警规则
      </div>
      <div class="auto-tab" :class="{active: tab==='cron'}" @click="tab='cron'">
        <svg viewBox="0 0 24 24" width="15" height="15" fill="none"><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2"/><path d="M12 6v6l4 2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
        定时任务
      </div>
    </div>

    <!-- Alerts Tab -->
    <div v-if="tab==='alerts'" class="tab-content">
      <div class="section-header">
        <h3>告警规则</h3>
        <button class="btn-primary" @click="showAlertDialog = true">添加规则</button>
      </div>
      <div v-loading="alertsLoading" class="rule-list">
        <div v-for="r in alertRules" :key="r.id" class="rule-card dg-card">
          <div class="rc-left">
            <span class="rc-enabled" :class="{on: r.enabled}">{{ r.enabled ? '启用' : '禁用' }}</span>
            <div>
              <div class="rc-name">{{ r.name }}</div>
              <div class="rc-desc">{{ metricLabel(r.metric) }} {{ r.operator }} {{ r.threshold }}{{ r.metric === 'node_offline' ? '' : '%' }}</div>
            </div>
          </div>
          <div class="rc-right">
            <span v-if="r.webhook_url" class="rc-webhook">Webhook ✓</span>
            <span class="rc-cooldown">冷却 {{ r.cooldown_min }}分钟</span>
            <button class="rc-delete" @click="deleteRule(r.id)">删除</button>
          </div>
        </div>
        <div v-if="!alertsLoading && alertRules.length === 0" class="empty-text">暂无告警规则</div>
      </div>

      <!-- Alert Dialog -->
      <el-dialog v-model="showAlertDialog" title="添加告警规则" width="480px">
        <div class="alert-form">
          <div class="af-row"><label>规则名称</label><input v-model="alertForm.name" placeholder="如：CPU 高负载告警" /></div>
          <div class="af-row"><label>监控指标</label>
            <select v-model="alertForm.metric">
              <option value="cpu">CPU 使用率</option>
              <option value="mem">内存使用率</option>
              <option value="disk">磁盘使用率</option>
              <option value="node_offline">节点离线</option>
            </select>
          </div>
          <div class="af-row" v-if="alertForm.metric !== 'node_offline'">
            <label>阈值</label>
            <div class="af-inline"><select v-model="alertForm.operator"><option value=">">大于</option><option value="<">小于</option></select><input type="number" v-model.number="alertForm.threshold" placeholder="90" />%</div>
          </div>
          <div class="af-row"><label>冷却时间(分钟)</label><input type="number" v-model.number="alertForm.cooldown_min" placeholder="30" /></div>
          <div class="af-row"><label>Webhook URL</label><input v-model="alertForm.webhook_url" placeholder="https://hooks.slack.com/..." /></div>
          <div class="af-hint">支持 Slack/钉钉/企业微信/自定义 Webhook</div>
        </div>
        <template #footer>
          <el-button @click="showAlertDialog = false">取消</el-button>
          <el-button type="primary" @click="createRule">创建</el-button>
        </template>
      </el-dialog>
    </div>

    <!-- Cron Tab -->
    <div v-if="tab==='cron'" class="tab-content">
      <div class="section-header">
        <h3>定时任务</h3>
        <button class="btn-primary" @click="showCronDialog = true">添加任务</button>
      </div>
      <div v-loading="cronLoading" class="rule-list">
        <div v-for="t in cronTasks" :key="t.id" class="rule-card dg-card">
          <div class="rc-left">
            <span class="rc-enabled" :class="{on: t.enabled}" @click="toggleCron(t.id)">{{ t.enabled ? '启用' : '禁用' }}</span>
            <div>
              <div class="rc-name">{{ t.name }}</div>
              <div class="rc-desc">每 {{ t.interval }} · {{ t.node_ids?.length || 0 }} 个节点</div>
              <div class="rc-time">下次: {{ formatTime(t.next_run) }} · 上次: {{ t.last_run ? formatTime(t.last_run) : '从未' }}</div>
            </div>
          </div>
          <div class="rc-right">
            <button class="rc-delete" @click="deleteCron(t.id)">删除</button>
          </div>
        </div>
        <div v-if="!cronLoading && cronTasks.length === 0" class="empty-text">暂无定时任务</div>
      </div>

      <!-- Cron Dialog -->
      <el-dialog v-model="showCronDialog" title="添加定时任务" width="540px">
        <div class="alert-form">
          <div class="af-row"><label>任务名称</label><input v-model="cronForm.name" placeholder="如：每日清理日志" /></div>
          <div class="af-row"><label>执行间隔</label>
            <select v-model="cronForm.interval">
              <option value="30s">每 30 秒</option>
              <option value="5m">每 5 分钟</option>
              <option value="10m">每 10 分钟</option>
              <option value="30m">每 30 分钟</option>
              <option value="1h">每 1 小时</option>
              <option value="6h">每 6 小时</option>
              <option value="12h">每 12 小时</option>
              <option value="24h">每 24 小时</option>
            </select>
          </div>
          <div class="af-row"><label>目标节点</label>
            <div class="af-nodes">
              <label v-for="n in onlineNodes" :key="n.id" class="af-node">
                <input type="checkbox" :value="n.id" v-model="cronForm.node_ids" /> {{ n.name }}
              </label>
            </div>
          </div>
          <div class="af-row"><label>脚本内容</label></div>
          <textarea v-model="cronForm.script" class="af-script" placeholder="#!/bin/bash&#10;find /var/log -mtime +7 -delete" rows="4"></textarea>
        </div>
        <template #footer>
          <el-button @click="showCronDialog = false">取消</el-button>
          <el-button type="primary" @click="createCron">创建</el-button>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { listNodes, type Node } from '@/api/nodes'
import * as auto from '@/api/automation'

const tab = ref('alerts')
const nodes = ref<Node[]>([])
const onlineNodes = computed(() => nodes.value.filter(n => n.status === 'online'))

// Alerts
const alertsLoading = ref(false)
const alertRules = ref<auto.AlertRule[]>([])
const alertDialog = ref(false)
const showAlertDialog = ref(false)
const alertForm = ref({ name: '', metric: 'cpu', operator: '>', threshold: 90, cooldown_min: 30, webhook_url: '' })

// Cron
const cronLoading = ref(false)
const cronTasks = ref<auto.CronTask[]>([])
const showCronDialog = ref(false)
const cronForm = ref({ name: '', interval: '5m', node_ids: [] as string[], script: '' })

function metricLabel(m: string) { return ({ cpu: 'CPU', mem: '内存', disk: '磁盘', node_offline: '节点离线' } as Record<string,string>)[m] || m }
function formatTime(t: string) { if (!t || t.startsWith('0001')) return '—'; return new Date(t).toLocaleString() }

async function loadAlerts() {
  alertsLoading.value = true
  try { alertRules.value = await auto.listAlertRules() } finally { alertsLoading.value = false }
}
async function createRule() {
  if (!alertForm.value.name) return
  try { await auto.createAlertRule(alertForm.value); ElMessage.success('规则已创建'); showAlertDialog.value = false; alertForm.value = { name: '', metric: 'cpu', operator: '>', threshold: 90, cooldown_min: 30, webhook_url: '' }; loadAlerts() } catch {}
}
async function deleteRule(id: string) { await auto.deleteAlertRule(id); ElMessage.success('已删除'); loadAlerts() }

async function loadCron() {
  cronLoading.value = true
  try { cronTasks.value = await auto.listCronTasks() } finally { cronLoading.value = false }
}
async function createCron() {
  if (!cronForm.value.name || !cronForm.value.script || cronForm.value.node_ids.length === 0) return
  try { await auto.createCronTask(cronForm.value); ElMessage.success('任务已创建'); showCronDialog.value = false; cronForm.value = { name: '', interval: '5m', node_ids: [], script: '' }; loadCron() } catch {}
}
async function deleteCron(id: string) { await auto.deleteCronTask(id); ElMessage.success('已删除'); loadCron() }
async function toggleCron(id: string) { await auto.toggleCronTask(id); loadCron() }

onMounted(async () => { nodes.value = await listNodes(); loadAlerts(); loadCron() })
</script>

<style scoped lang="scss">
.btn-primary { display: flex; align-items: center; gap: 6px; padding: 7px 14px; border: none; border-radius: 7px; background: var(--accent); color: #fff; font-size: 12px; font-weight: 600; cursor: pointer; font-family: inherit; }
.auto-tabs { display: flex; gap: 2px; border-bottom: 1px solid var(--dg-border); margin-bottom: 16px; }
.auto-tab { display: flex; align-items: center; gap: 6px; padding: 8px 16px; font-size: 13px; font-weight: 500; color: var(--dg-text-faint); cursor: pointer; border-bottom: 2px solid transparent; margin-bottom: -1px;
  &:hover { color: var(--dg-text-dim); }
  &.active { color: var(--accent); border-bottom-color: var(--accent); } }
.section-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; h3 { font-size: 14px; font-weight: 600; } }
.rule-list { display: flex; flex-direction: column; gap: 8px; }
.rule-card { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px;
  .rc-left { display: flex; align-items: center; gap: 12px; }
  .rc-enabled { font-size: 10px; font-weight: 600; padding: 2px 8px; border-radius: 4px; background: var(--dg-danger-bg); color: var(--dg-danger); cursor: pointer;
    &.on { background: var(--dg-success-bg); color: var(--dg-success); } }
  .rc-name { font-size: 13px; font-weight: 600; }
  .rc-desc { font-size: 11px; color: var(--dg-text-faint); margin-top: 2px; }
  .rc-time { font-size: 10px; color: var(--dg-text-faint); margin-top: 2px; font-family: 'JetBrains Mono', monospace; }
  .rc-right { display: flex; align-items: center; gap: 10px; }
  .rc-webhook { font-size: 10px; color: var(--dg-success); }
  .rc-cooldown { font-size: 10px; color: var(--dg-text-faint); }
  .rc-delete { padding: 4px 10px; border-radius: 5px; border: 1px solid var(--dg-border); background: transparent; color: var(--dg-danger); font-size: 11px; cursor: pointer; font-family: inherit; &:hover { background: var(--dg-danger); color: #fff; } }
}
.empty-text { padding: 40px; text-align: center; color: var(--dg-text-faint); }
.alert-form { display: flex; flex-direction: column; gap: 12px; }
.af-row { display: flex; align-items: center; gap: 10px; label { font-size: 12px; color: var(--dg-text-dim); min-width: 90px; flex-shrink: 0; }
  input, select { flex: 1; height: 32px; padding: 0 10px; border: 1px solid var(--dg-border); border-radius: 6px; background: var(--dg-input-bg); color: var(--dg-text); font-size: 12px; outline: none; font-family: inherit; &:focus { border-color: var(--accent); } }
  .af-inline { display: flex; align-items: center; gap: 6px; flex: 1; select { max-width: 80px; } input { width: 60px; } } }
.af-hint { font-size: 11px; color: var(--dg-text-faint); padding-left: 90px; }
.af-nodes { display: flex; flex-wrap: wrap; gap: 6px; flex: 1; .af-node { display: flex; align-items: center; gap: 4px; font-size: 12px; padding: 3px 10px; border-radius: 6px; border: 1px solid var(--dg-border); cursor: pointer; input { accent-color: var(--accent); } &:hover { border-color: var(--accent); } } }
.af-script { width: 100%; padding: 10px; border: 1px solid var(--dg-border); border-radius: 6px; background: var(--dg-input-bg); color: var(--dg-text); font-family: 'JetBrains Mono', monospace; font-size: 12px; outline: none; resize: vertical; &:focus { border-color: var(--accent); } }
</style>
