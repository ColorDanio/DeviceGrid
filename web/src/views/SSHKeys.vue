<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title-group">
        <h2>{{ t('feature.sshKeysTitle') }}</h2>
        <p class="page-subtitle">{{ t('feature.sshKeysSubtitle') }}</p>
      </div>
      <NodeSelector v-model="selectedNode" />
    </div>
    <div v-if="!selectedNode" class="dg-card empty-card">
      <p>{{ t('feature.selectNodeForKeys') }}</p>
    </div>
    <div v-else class="dg-card" style="padding:20px">
      <div class="section-header"><h3>{{ t('feature.keyInfo') }}</h3>
        <div style="display:flex;gap:8px">
          <button class="btn" @click="loadKeyInfo" :disabled="!selectedNode">{{ t('common.refresh') }}</button>
          <button class="btn btn-primary" @click="handleRotate" :disabled="rotating">{{ rotating ? t('feature.rotatingKey') : t('feature.rotateKey') }}</button>
        </div>
      </div>
      <pre class="output">{{ keyInfo || t('feature.clickRefreshForKey') }}</pre>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import NodeSelector from '@/components/NodeSelector.vue'
import { getSSHKeyInfo, rotateSSHKey } from '@/api/features'

const selectedNode = ref(''); const keyInfo = ref(''); const rotating = ref(false)
const { t } = useI18n()
async function loadKeyInfo() { if (!selectedNode.value) return; try { keyInfo.value = await getSSHKeyInfo(selectedNode.value) } catch { keyInfo.value = t('feature.keyLoadFailed') } }
async function handleRotate() {
  rotating.value = true
  try { await rotateSSHKey(selectedNode.value); ElMessage.success(t('feature.keyRotateSucceeded')); loadKeyInfo() }
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
