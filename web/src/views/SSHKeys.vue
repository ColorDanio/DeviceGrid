<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title-group">
        <h2>SSH 密钥管理</h2>
        <p class="page-subtitle">查看节点密钥指纹、轮换密钥</p>
      </div>
      <NodeSelector v-model="selectedNode" />
    </div>
    <div v-if="!selectedNode" class="dg-card empty-card">
      <p>选择左侧节点查看密钥信息</p>
    </div>
    <div v-else class="dg-card" style="padding:20px">
      <div class="section-header"><h3>密钥信息</h3>
        <div style="display:flex;gap:8px">
          <button class="btn" @click="loadKeyInfo" :disabled="!selectedNode">刷新</button>
          <button class="btn btn-primary" @click="handleRotate" :disabled="rotating">{{ rotating ? '轮换中...' : '轮换密钥' }}</button>
        </div>
      </div>
      <pre class="output">{{ keyInfo || '点击刷新查看密钥信息' }}</pre>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import NodeSelector from '@/components/NodeSelector.vue'
import { getSSHKeyInfo, rotateSSHKey } from '@/api/features'

const selectedNode = ref(''); const keyInfo = ref(''); const rotating = ref(false)
async function loadKeyInfo() { if (!selectedNode.value) return; try { keyInfo.value = await getSSHKeyInfo(selectedNode.value) } catch { keyInfo.value = '获取失败' } }
async function handleRotate() {
  rotating.value = true
  try { await rotateSSHKey(selectedNode.value); ElMessage.success('密钥轮换成功'); loadKeyInfo() }
  catch {} finally { rotating.value = false }
}
watch(selectedNode, () => { keyInfo.value = ''; loadKeyInfo() })
</script>
<style scoped lang="scss">
.empty-card { min-height: 300px; display: flex; align-items: center; justify-content: center; color: var(--dg-text-faint); }
.section-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; h3 { font-size: 14px; font-weight: 600; } }
.btn { padding: 5px 14px; border-radius: 7px; border: 1px solid var(--dg-border); background: var(--dg-surface-solid); color: var(--dg-text-dim); font-size: 12px; cursor: pointer; font-family: inherit; &:hover { border-color: var(--accent); color: var(--accent); }
  &.btn-primary { background: var(--accent); border-color: var(--accent); color: #fff; &:hover { background: var(--accent-dark); } &:disabled { opacity: 0.5; } } }
.output { font-family: 'JetBrains Mono', monospace; font-size: 12px; color: var(--dg-text-dim); white-space: pre-wrap; padding: 12px; background: var(--dg-bg-3); border-radius: 8px; margin: 0; }
</style>
