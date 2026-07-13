import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { getMetrics, listNodes, type Node, type NodeMetrics } from '@/api/nodes'

const POLL_INTERVAL_MS = 20_000

export const useFleetStore = defineStore('fleet', () => {
  const nodes = ref<Node[]>([])
  const metrics = ref<Record<string, NodeMetrics>>({})
  const loading = ref(false)
  const started = ref(false)
  let timer: ReturnType<typeof setInterval> | null = null
  let socket: WebSocket | null = null

  const onlineNodes = computed(() => nodes.value.filter((node) => node.status === 'online'))

  async function refresh() {
    if (document.visibilityState !== 'visible' || socket?.readyState === WebSocket.OPEN) return
    loading.value = true
    try {
      nodes.value = await listNodes()
      const current = new Set(nodes.value.map((node) => node.id))
      for (const id of Object.keys(metrics.value)) {
        if (!current.has(id)) delete metrics.value[id]
      }
      await Promise.allSettled(onlineNodes.value.map(async (node) => {
        metrics.value[node.id] = await getMetrics(node.id)
      }))
    } finally {
      loading.value = false
    }
  }

  function onVisibilityChange() {
    if (document.visibilityState === 'visible') void refresh()
  }

  function start() {
    if (started.value) return
    started.value = true
    document.addEventListener('visibilitychange', onVisibilityChange)
    void refresh()
    connect()
    timer = setInterval(() => {
      connect()
      void refresh()
    }, POLL_INTERVAL_MS)
  }

  function connect() {
    const token = sessionStorage.getItem('dg_token')
    if (!token || socket) return
    const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
    socket = new WebSocket(`${protocol}://${location.host}/ws`)
    socket.onopen = () => {
      socket?.send(JSON.stringify({ token }))
      socket?.send(JSON.stringify({ type: 'subscribe', topics: ['kanban', ...nodes.value.map((node) => `metrics-${node.id}`)] }))
    }
    socket.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)
        if (message.topic?.startsWith('metrics-')) {
          metrics.value[message.topic.slice('metrics-'.length)] = message.data
        } else if (message.topic === 'kanban') {
          void refresh()
        }
      } catch {}
    }
    socket.onclose = () => { socket = null }
    socket.onerror = () => socket?.close()
  }

  function stop() {
    if (!started.value) return
    started.value = false
    document.removeEventListener('visibilitychange', onVisibilityChange)
    if (timer) clearInterval(timer)
    timer = null
    socket?.close()
    socket = null
  }

  return { nodes, metrics, loading, onlineNodes, refresh, start, stop }
})
