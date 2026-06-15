<template>
  <div class="term-pane-wrapper" :class="{ active: isActive }">
    <!-- Tab selector for this pane -->
    <div v-if="showSelector" class="pane-selector">
      <select v-model="localTabId" @change="$emit('assign', localTabId)" class="pane-select">
        <option value="">选择终端...</option>
        <option v-for="t in tabs" :key="t.id" :value="t.id">
          {{ t.nodeName }} ({{ t.nodeHost }})
        </option>
      </select>
    </div>
    <!-- Terminal container -->
    <div ref="containerEl" class="term-container" v-show="tabId"></div>
    <!-- Empty state -->
    <div v-if="!tabId" class="pane-empty">
      <svg viewBox="0 0 24 24" width="28" height="28" fill="none" style="opacity:0.2"><rect x="2" y="4" width="20" height="16" rx="2" stroke="currentColor" stroke-width="1.5"/><path d="M6 9l3 3-3 3M12 15h4" stroke="currentColor" stroke-width="1.5"/></svg>
      <p>选择节点连接</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { TerminalTab } from '@/composables/useTerminalManager'

const props = defineProps<{
  tabId: string | null
  tabs: TerminalTab[]
  isActive: boolean
  showSelector: boolean
}>()

defineEmits<{ assign: [tabId: string] }>()

const containerEl = ref<HTMLElement>()
const localTabId = ref(props.tabId || '')

// Expose container for parent to attach terminal
defineExpose({ containerEl })
</script>

<style scoped lang="scss">
.term-pane-wrapper { position: relative; height: 100%; display: flex; flex-direction: column; background: #0e0e1c; overflow: hidden;
  &.active { } }
.pane-selector { flex-shrink: 0; padding: 4px 8px; background: rgba(0,0,0,0.2); border-bottom: 1px solid rgba(255,255,255,0.05); display: flex; align-items: center; gap: 6px; }
.pane-select { background: transparent; border: 1px solid var(--dg-border); border-radius: 6px; color: var(--dg-text); font-size: 11px; padding: 3px 8px; outline: none; font-family: inherit; max-width: 180px;
  option { background: var(--dg-bg-3); } }
.term-container { flex: 1; overflow: hidden; }
.pane-empty { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; color: var(--dg-text-faint); font-size: 12px; }
</style>
