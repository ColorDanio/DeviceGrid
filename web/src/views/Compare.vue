<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title-group"><h2>节点配置对比</h2><p class="page-subtitle">横向对比两个节点的硬件和配置</p></div>
    </div>
    <div class="dg-card" style="padding:20px">
      <div class="compare-selector">
        <NodeSelector v-model="nodeA" placeholder="节点 A" />
        <span class="vs">VS</span>
        <NodeSelector v-model="nodeB" placeholder="节点 B" />
        <button class="btn btn-primary" @click="loadCompare" :disabled="!nodeA || !nodeB || nodeA === nodeB">对比</button>
      </div>
      <div v-loading="loading" v-if="compareData" class="compare-grid">
        <table class="compare-table">
          <thead><tr><th>属性</th><th>{{ compareData.node_a.name }}</th><th>{{ compareData.node_b.name }}</th></tr></thead>
          <tbody>
            <tr><td>状态</td><td>{{ compareData.node_a.status }}</td><td>{{ compareData.node_b.status }}</td></tr>
            <tr><td>地址</td><td class="mono">{{ compareData.node_a.host }}:{{ compareData.node_a.port }}</td><td class="mono">{{ compareData.node_b.host }}:{{ compareData.node_b.port }}</td></tr>
            <tr><td>用户</td><td>{{ compareData.node_a.username }}</td><td>{{ compareData.node_b.username }}</td></tr>
            <tr><td>通信</td><td>{{ compareData.node_a.transport_mode }}</td><td>{{ compareData.node_b.transport_mode }}</td></tr>
            <tr><td>系统</td><td>{{ compareData.node_a.os || '—' }}</td><td>{{ compareData.node_b.os || '—' }}</td></tr>
            <tr><td>架构</td><td>{{ compareData.node_a.arch || '—' }}</td><td>{{ compareData.node_b.arch || '—' }}</td></tr>
            <tr><td>Docker</td><td>{{ compareData.node_a.docker_version || '—' }}</td><td>{{ compareData.node_b.docker_version || '—' }}</td></tr>
            <tr><td>地区</td><td>{{ compareData.node_a.country || '—' }}</td><td>{{ compareData.node_b.country || '—' }}</td></tr>
            <tr><td>CPU 型号</td><td>{{ compareData.metrics_a?.cpu_model || '—' }}</td><td>{{ compareData.metrics_b?.cpu_model || '—' }}</td></tr>
            <tr><td>CPU 核数</td><td>{{ compareData.metrics_a?.cpu_sockets }}P×{{ compareData.metrics_a?.cpu_cores }}C/{{ compareData.metrics_a?.cpu_threads }}T</td><td>{{ compareData.metrics_b?.cpu_sockets }}P×{{ compareData.metrics_b?.cpu_cores }}C/{{ compareData.metrics_b?.cpu_threads }}T</td></tr>
            <tr><td>内存</td><td>{{ compareData.metrics_a ? fmt(compareData.metrics_a.mem_total) : '—' }}</td><td>{{ compareData.metrics_b ? fmt(compareData.metrics_b.mem_total) : '—' }}</td></tr>
            <tr><td>磁盘</td><td>{{ compareData.metrics_a ? fmt(compareData.metrics_a.disk_total) : '—' }}</td><td>{{ compareData.metrics_b ? fmt(compareData.metrics_b.disk_total) : '—' }}</td></tr>
          </tbody>
        </table>
      </div>
      <div v-else-if="!loading" class="empty-text">选择两个节点后点击对比</div>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import NodeSelector from '@/components/NodeSelector.vue'
import { compareNodes } from '@/api/features'
const nodeA = ref(''); const nodeB = ref(''); const loading = ref(false); const compareData = ref<any>(null)
function fmt(b: number) { if (!b) return '—'; if (b < 1099511627776) return (b / 1073741824).toFixed(0) + ' GB'; return (b / 1099511627776).toFixed(1) + ' TB' }
async function loadCompare() { loading.value = true; try { compareData.value = await compareNodes(nodeA.value, nodeB.value) } finally { loading.value = false } }
</script>
<style scoped lang="scss">
.compare-selector { display: flex; align-items: center; gap: 12px; margin-bottom: 20px; .vs { font-weight: 700; color: var(--dg-text-faint); } }
.btn { padding: 7px 16px; border-radius: 7px; border: 1px solid var(--dg-border); background: var(--dg-surface-solid); color: var(--dg-text-dim); font-size: 12px; cursor: pointer; font-family: inherit;
  &.btn-primary { background: var(--accent); border-color: var(--accent); color: #fff; } }
.compare-table { width: 100%; border-collapse: collapse; th, td { padding: 10px 14px; text-align: left; font-size: 13px; border-bottom: 1px solid var(--dg-border); }
  th { font-weight: 600; color: var(--dg-text-dim); background: var(--dg-table-header-bg); } td:first-child { color: var(--dg-text-faint); font-weight: 500; }
  .mono { font-family: 'JetBrains Mono', monospace; } }
.empty-text { padding: 40px; text-align: center; color: var(--dg-text-faint); }
</style>
