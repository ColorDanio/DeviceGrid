<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title-group"><h2>{{ t('feature.auditTitle') }}</h2><p class="page-subtitle">{{ t('feature.auditSubtitle') }}</p></div>
      <div style="display:flex;gap:8px">
        <button class="btn" @click="exportCSV">{{ t('feature.exportMetrics') }}</button>
        <button class="btn" @click="loadAudit">{{ t('common.refresh') }}</button>
      </div>
    </div>
    <div class="dg-card" style="padding:18px">
      <el-table :data="entries" stripe v-loading="loading">
        <el-table-column :label="t('feature.time')" width="180">
          <template #default="{ row }">{{ row.timestamp }}</template>
        </el-table-column>
        <el-table-column :label="t('feature.method')" width="70">
          <template #default="{ row }"><span class="method-badge" :class="row.method">{{ row.method }}</span></template>
        </el-table-column>
        <el-table-column prop="path" :label="t('feature.path')" min-width="200" />
        <el-table-column prop="user" :label="t('feature.user')" width="100" />
        <el-table-column prop="ip" label="IP" width="120" />
        <el-table-column :label="t('feature.status')" width="70">
          <template #default="{ row }"><span :class="row.status < 400 ? 'ok' : 'err'">{{ row.status }}</span></template>
        </el-table-column>
        <el-table-column prop="duration" :label="t('feature.duration')" width="80" />
      </el-table>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getAuditLog, downloadMetricsCSV, type AuditEntry } from '@/api/features'

const entries = ref<AuditEntry[]>([]); const loading = ref(false)
const { t } = useI18n()
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
