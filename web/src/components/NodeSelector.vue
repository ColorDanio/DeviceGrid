<template>
  <div class="node-selector" :class="{ active: showDropdown }">
    <div class="selector-trigger" @click="showDropdown = !showDropdown">
      <div class="trigger-left">
        <span class="trigger-dot" :class="selectedNodeStatus"></span>
        <span v-if="selectedNode" class="trigger-name">{{ selectedNode.name }}</span>
        <span v-else class="trigger-placeholder">{{ placeholder }}</span>
        <span v-if="selectedNode" class="trigger-host">{{ selectedNode.host }}</span>
      </div>
      <svg viewBox="0 0 24 24" width="14" height="14" fill="none" class="trigger-arrow"><path d="M6 9l6 6 6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
    </div>
    <transition name="dropdown">
      <div v-if="showDropdown" class="selector-dropdown">
        <div class="dd-search">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none"><circle cx="11" cy="11" r="8" stroke="currentColor" stroke-width="2"/><path d="M21 21l-4.35-4.35" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
          <input v-model="search" placeholder="搜索节点..." />
        </div>
        <div class="dd-list">
          <div v-for="n in filteredNodes" :key="n.id" class="dd-item" :class="{ selected: n.id === modelValue }" @click="select(n)">
            <span class="dd-dot" :class="n.status"></span>
            <div class="dd-info">
              <span class="dd-name">{{ n.name }}</span>
              <span class="dd-host">{{ n.host }}:{{ n.port }}</span>
            </div>
            <span v-if="n.docker_version" class="dd-badge">Docker</span>
          </div>
          <div v-if="filteredNodes.length === 0" class="dd-empty">无匹配节点</div>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { listNodes, type Node } from '@/api/nodes'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
}>(), { placeholder: '选择节点...' })

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const nodes = ref<Node[]>([])
const showDropdown = ref(false)
const search = ref('')

const selectedNode = computed(() => nodes.value.find((n) => n.id === props.modelValue))
const selectedNodeStatus = computed(() => selectedNode.value?.status || 'offline')

const filteredNodes = computed(() => {
  if (!search.value) return nodes.value
  const q = search.value.toLowerCase()
  return nodes.value.filter((n) => n.name.toLowerCase().includes(q) || n.host.includes(q))
})

function select(n: Node) {
  emit('update:modelValue', n.id)
  showDropdown.value = false
}

function handleClickOutside(e: MouseEvent) {
  const el = e.target as HTMLElement
  if (!el.closest('.node-selector')) showDropdown.value = false
}

onMounted(() => {
  listNodes().then((data) => { nodes.value = data }).catch(() => {})
  document.addEventListener('click', handleClickOutside)
})
</script>

<style scoped lang="scss">
.node-selector { position: relative; width: 260px; }

.selector-trigger {
  display: flex; align-items: center; justify-content: space-between; gap: 8px;
  padding: 8px 14px; background: var(--dg-bg-2); border: 1px solid var(--dg-border);
  border-radius: 10px; cursor: pointer; transition: all 0.2s;
  &:hover { border-color: var(--dg-border-bright); }
  .trigger-left { display: flex; align-items: center; gap: 8px; overflow: hidden; }
  .trigger-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0;
    &.online { background: var(--dg-green); box-shadow: 0 0 6px var(--dg-green); }
    &.offline { background: var(--dg-red); }
    &.untrusted { background: var(--dg-amber); }
    &.error { background: var(--dg-red); }
  }
  .trigger-name { font-size: 13px; font-weight: 600; color: var(--dg-text); white-space: nowrap; }
  .trigger-placeholder { font-size: 13px; color: var(--dg-text-faint); }
  .trigger-host { font-size: 11px; color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace; }
  .trigger-arrow { color: var(--dg-text-faint); transition: transform 0.2s; }
}
.node-selector.active .trigger-arrow { transform: rotate(180deg); }
.node-selector.active .selector-trigger { border-color: var(--dg-cyan); box-shadow: 0 0 0 2px var(--dg-table-row-hover); }

.dropdown-enter-active, .dropdown-leave-active { transition: all 0.2s; }
.dropdown-enter-from, .dropdown-leave-to { opacity: 0; transform: translateY(-4px); }

.selector-dropdown {
  position: absolute; top: calc(100% + 6px); left: 0; right: 0; z-index: 200;
  background: var(--dg-surface-solid); border: 1px solid var(--dg-border); border-radius: 12px;
  box-shadow: var(--dg-shadow-lg); overflow: hidden;

  .dd-search {
    display: flex; align-items: center; gap: 8px; padding: 10px 14px; border-bottom: 1px solid var(--dg-border);
    svg { color: var(--dg-text-faint); }
    input { flex: 1; background: transparent; border: none; outline: none; color: var(--dg-text); font-size: 13px; font-family: inherit; &::placeholder { color: var(--dg-text-faint); } }
  }
  .dd-list { max-height: 280px; overflow-y: auto; padding: 6px; }
  .dd-item {
    display: flex; align-items: center; gap: 10px; padding: 9px 10px; border-radius: 8px; cursor: pointer; transition: all 0.15s;
    &:hover { background: var(--dg-table-row-hover); }
    &.selected { background: var(--dg-success-bg); .dd-name { color: var(--dg-cyan); } }
    .dd-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0;
      &.online { background: var(--dg-green); box-shadow: 0 0 4px var(--dg-green); }
      &.offline { background: var(--dg-red); }
      &.untrusted { background: var(--dg-amber); }
    }
    .dd-info { flex: 1; display: flex; flex-direction: column; overflow: hidden;
      .dd-name { font-size: 13px; font-weight: 500; color: var(--dg-text); white-space: nowrap; }
      .dd-host { font-size: 11px; color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace; }
    }
    .dd-badge { font-size: 10px; padding: 2px 6px; border-radius: 4px; background: var(--dg-warning-bg); color: var(--dg-amber); font-weight: 500; }
  }
  .dd-empty { padding: 20px; text-align: center; font-size: 13px; color: var(--dg-text-faint); }
}
</style>
