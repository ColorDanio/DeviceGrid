<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title-group"><h2>审计日志</h2><p class="page-subtitle">所有 POST/PUT/DELETE 操作记录</p></div>
      <div style="display:flex;gap:8px">
        <button class="btn" @click="exportCSV">导出指标 CSV</button>
        <button class="btn" @click="loadAudit">刷新</button>
      </div>
    </div>
    <div class="dg-card" style="padding:18px">
      <el-table :data="entries" stripe v-loading="loading">
        <el-table-column label="时间" width="180">
          <template #default="{ row }">{{ row.timestamp }}</template>
        </el-table-column>
        <el-table-column label="方法" width="70">
          <template #default="{ row }"><span class="method-badge" :class="row.method">{{ row.method }}</span></template>
        </el-table-column>
        <el-table-column prop="path" label="路径" min-width="200" />
        <el-table-column prop="user" label="用户" width="100" />
        <el-table-column prop="ip" label="IP" width="120" />
        <el-table-column label="状态" width="70">
          <template #default="{ row }"><span :class="row.status < 400 ? 'ok' : 'err'">{{ row.status }}</span></template>
        </el-table-column>
        <el-table-column prop="duration" label="耗时" width="80" />
      </el-table>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getAuditLog, downloadMetricsCSV, type AuditEntry } from '@/api/features'

const entries = ref<AuditEntry[]>([]); const loading = ref(false)
async function loadAudit() { loading.value = true; try { entries.value = await getAuditLog() } finally { loading.value = false } }
async function exportCSV() { await downloadMetricsCSV() }
onMounted(() => loadAudit())
</script>
<style scoped lang="scss">
.btn { padding: 6px 14px; border-radius: 7px; border: 1px solid var(--dg-border); background: var(--dg-surface-solid); color: var(--dg-text-dim); font-size: 12px; cursor: pointer; font-family: inherit; &:hover { border-color: var(--accent); color: var(--accent); } }
.method-badge { font-size: 10px; font-weight: 700; padding: 2px 6px; border-radius: 4px;
  &.POST { background: rgba(34,197,94,0.15); color: var(--dg-success); }
  &.PUT { background: rgba(245,158,11,0.15); color: var(--dg-warning); }
  &.DELETE { background: rgba(239,68,68,0.15); color: var(--dg-danger); } }
.ok { color: var(--dg-success); font-weight: 600; } .err { color: var(--dg-danger); font-weight: 600; }
</style>
