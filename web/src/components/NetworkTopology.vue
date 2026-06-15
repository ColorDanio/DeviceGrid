<template>
  <div class="topology">
    <!-- Search -->
    <div class="topo-search">
      <svg viewBox="0 0 24 24" width="15" height="15" fill="none"><circle cx="11" cy="11" r="8" stroke="currentColor" stroke-width="2"/><path d="M21 21l-4.35-4.35" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
      <input v-model="search" placeholder="搜索节点名称或 IP · 回车快速连接" @keydown.enter="quickConnect" />
    </div>

    <!-- Network topology -->
    <div class="topo-canvas">
      <svg :viewBox="`0 0 ${viewBoxW} ${viewBoxH}`" class="topo-svg">
        <!-- Connection lines -->
        <g class="links">
          <line v-for="(n, i) in positionedNodes" :key="`l-${n.id}`"
            :x1="centerX" :y1="centerY"
            :x2="n.x" :y2="n.y"
            :class="['link', n.status]"
            :style="{ animationDelay: `${i * 0.05}s` }" />
        </g>

        <!-- Central hub -->
        <g class="hub" :transform="`translate(${centerX}, ${centerY})`">
          <circle r="28" class="hub-glow" />
          <circle r="22" class="hub-circle" />
          <text y="2" text-anchor="middle" class="hub-label">HUB</text>
        </g>

        <!-- Pulsing rings on hub -->
        <g class="hub-pulse" :transform="`translate(${centerX}, ${centerY})`">
          <circle r="22" class="pulse-ring r1" />
          <circle r="22" class="pulse-ring r2" />
        </g>

        <!-- Node groups -->
        <g v-for="n in positionedNodes" :key="n.id" :transform="`translate(${n.x}, ${n.y})`"
           class="node-group" :class="{ hover: hoveredId === n.id }"
           @mouseenter="hoveredId = n.id" @mouseleave="hoveredId = null">
          <!-- Node circle -->
          <circle r="20" class="node-bg" :class="n.status" />
          <circle r="20" class="node-ring" :class="n.status" />

          <!-- Node label -->
          <text y="38" text-anchor="middle" class="node-label">{{ n.name }}</text>

          <!-- Status indicator -->
          <circle cx="14" cy="-14" r="5" class="node-status-dot" :class="n.status" />

          <!-- Click target -->
          <circle r="24" class="node-click" @click="$emit('select', n)" :class="{ disabled: n.status !== 'online' }" />

          <!-- Hover tooltip -->
          <g v-if="hoveredId === n.id" class="tooltip" transform="translate(0, -42)">
            <rect x="-60" y="-20" width="120" height="22" rx="6" class="tooltip-bg" />
            <text y="-6" text-anchor="middle" class="tooltip-text">{{ n.host }}:{{ n.port }}</text>
          </g>
        </g>
      </svg>
    </div>

    <!-- Recent connections -->
    <div v-if="recentList.length > 0 && !search" class="topo-recent">
      <div class="recent-title">
        <svg viewBox="0 0 24 24" width="12" height="12" fill="none"><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2"/><path d="M12 6v6l4 2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
        最近连接
      </div>
      <div class="recent-chips">
        <button v-for="n in recentList" :key="n.id" class="recent-chip" @click="$emit('select', n)" :disabled="n.status !== 'online'">
          <span class="rc-dot" :class="n.status"></span>
          {{ n.name }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Node } from '@/api/nodes'

const props = defineProps<{ nodes: Node[]; recentList: Node[] }>()
const emits = defineEmits<{ select: [node: Node] }>()

const search = ref('')
const hoveredId = ref<string | null>(null)

const viewBoxW = 600
const viewBoxH = 380
const centerX = viewBoxW / 2
const centerY = viewBoxH / 2

const filteredNodes = computed(() => {
  if (!search.value) return props.nodes
  const q = search.value.toLowerCase()
  return props.nodes.filter(n => n.name.toLowerCase().includes(q) || n.host.includes(q))
})

const positionedNodes = computed(() => {
  const nodes = filteredNodes.value
  const n = nodes.length
  if (n === 0) return []
  const radius = Math.min(centerX, centerY) - 35
  return nodes.map((node, i) => {
    const angle = (i / n) * 2 * Math.PI - Math.PI / 2
    return {
      ...node,
      x: centerX + radius * Math.cos(angle),
      y: centerY + radius * Math.sin(angle),
    }
  })
})

function quickConnect() {
  const first = filteredNodes.value.find(n => n.status === 'online')
  if (first) emits('select', first)
}
</script>

<style scoped lang="scss">
.topology { display: flex; flex-direction: column; align-items: center; gap: 20px; height: 100%; padding: 20px; }

.topo-search { display: flex; align-items: center; gap: 10px; padding: 0 16px; width: 100%; max-width: 420px; border: 1px solid var(--dg-border); border-radius: 12px; background: var(--dg-bg-2); transition: border-color 0.2s;
  &:focus-within { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-tint); }
  svg { color: var(--dg-text-faint); flex-shrink: 0; }
  input { flex: 1; height: 42px; border: none; outline: none; background: transparent; color: var(--dg-text); font-size: 14px; font-family: inherit; &::placeholder { color: var(--dg-text-faint); } }
}

.topo-canvas { flex: 1; display: flex; align-items: center; justify-content: center; width: 100%; }
.topo-svg { width: 100%; max-width: 600px; height: 100%; max-height: 380px; }

/* Links */
.link { stroke: var(--dg-border); stroke-width: 1; fill: none; transition: stroke 0.3s;
  &.online { stroke: var(--accent); stroke-opacity: 0.4; stroke-dasharray: 4 4; animation: dash 20s linear infinite; }
  &.offline { stroke: var(--dg-danger); stroke-opacity: 0.2; }
  &.untrusted { stroke: var(--dg-warning); stroke-opacity: 0.2; }
}
@keyframes dash { to { stroke-dashoffset: -100; } }

/* Hub */
.hub-glow { fill: var(--accent); opacity: 0.06; }
.hub-circle { fill: var(--dg-bg-3); stroke: var(--accent); stroke-width: 2; }
.hub-label { fill: var(--accent); font-size: 10px; font-weight: 700; font-family: inherit; }
.pulse-ring { fill: none; stroke: var(--accent); stroke-width: 1.5; opacity: 0; transform-origin: center;
  &.r1 { animation: pulse-out 3s ease-out infinite; }
  &.r2 { animation: pulse-out 3s ease-out infinite 1.5s; }
}
@keyframes pulse-out { 0% { transform: scale(1); opacity: 0.5; } 100% { transform: scale(2); opacity: 0; } }

/* Nodes */
.node-group { cursor: pointer; transition: transform 0.2s;
  &.hover { transform-origin: center; }
}
.node-bg { fill: var(--dg-bg-3); transition: all 0.2s; }
.node-ring { fill: none; stroke-width: 2; transition: stroke 0.3s;
  &.online { stroke: var(--dg-success); }
  &.offline { stroke: var(--dg-danger); }
  &.untrusted { stroke: var(--dg-warning); }
  &.error { stroke: var(--dg-danger); }
}
.node-label { fill: var(--dg-text-dim); font-size: 10px; font-weight: 500; font-family: inherit; pointer-events: none; }
.node-status-dot {
  &.online { fill: var(--dg-success); }
  &.offline { fill: var(--dg-danger); }
  &.untrusted { fill: var(--dg-warning); }
  &.error { fill: var(--dg-danger); }
}
.node-group:hover {
  .node-bg { fill: var(--accent-tint); }
  .node-ring { stroke-width: 3; filter: drop-shadow(0 0 6px currentColor); }
  .node-label { fill: var(--dg-text); }
}
.node-click { fill: transparent; cursor: pointer; &.disabled { cursor: not-allowed; opacity: 0; } }

.tooltip-bg { fill: var(--dg-bg-3); stroke: var(--dg-border); stroke-width: 1; }
.tooltip-text { fill: var(--dg-text-dim); font-size: 10px; font-family: 'JetBrains Mono', monospace; }

/* Recent */
.topo-recent { width: 100%; max-width: 420px; }
.recent-title { display: flex; align-items: center; gap: 5px; font-size: 11px; font-weight: 600; color: var(--dg-text-faint); text-transform: uppercase; margin-bottom: 8px; }
.recent-chips { display: flex; flex-wrap: wrap; gap: 6px; }
.recent-chip { display: flex; align-items: center; gap: 5px; padding: 5px 12px; border-radius: 20px; border: 1px solid var(--dg-border); background: var(--dg-bg-2); color: var(--dg-text-dim); font-size: 12px; cursor: pointer; font-family: inherit; transition: all 0.15s;
  &:hover { border-color: var(--accent); color: var(--accent); }
  &:disabled { opacity: 0.3; cursor: not-allowed; }
  .rc-dot { width: 6px; height: 6px; border-radius: 50%; &.online { background: var(--dg-success); } &.offline { background: var(--dg-danger); } &.untrusted { background: var(--dg-warning); } }
}
</style>
