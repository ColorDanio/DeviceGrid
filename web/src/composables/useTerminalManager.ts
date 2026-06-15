import { ref, reactive } from 'vue'
import { listNodes, type Node } from '@/api/nodes'

export interface TerminalTab {
  id: string
  nodeId: string
  nodeName: string
  nodeHost: string
  status: 'connecting' | 'connected' | 'disconnected' | 'error'
  title: string
}

export type SplitDirection = 'horizontal' | 'vertical' | 'none'

const tabs = ref<TerminalTab[]>([])
const activeTabId = ref<string | null>(null)
const splitDirection = ref<SplitDirection>('none')
const leftTabId = ref<string | null>(null)
const rightTabId = ref<string | null>(null)
const allNodes = ref<Node[]>([])

const RECENT_KEY = 'dg_recent_terminals'
const recentIds = ref<string[]>(JSON.parse(localStorage.getItem(RECENT_KEY) || '[]'))

export function useTerminalManager() {
  const recentNodes = () => {
    return recentIds.value
      .map(id => allNodes.value.find(n => n.id === id))
      .filter((n): n is Node => !!n)
      .slice(0, 8)
  }

  function addToRecent(id: string) {
    recentIds.value = [id, ...recentIds.value.filter(x => x !== id)].slice(0, 12)
    localStorage.setItem(RECENT_KEY, JSON.stringify(recentIds.value))
  }

  async function loadNodes() {
    allNodes.value = await listNodes()
  }

  function openTab(nodeId: string): string {
    const node = allNodes.value.find(n => n.id === nodeId)
    if (!node) return ''

    // If tab already exists, activate it
    const existing = tabs.value.find(t => t.nodeId === nodeId)
    if (existing) {
      activateTab(existing.id)
      return existing.id
    }

    const tabId = `tab-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`
    const tab: TerminalTab = {
      id: tabId,
      nodeId,
      nodeName: node.name,
      nodeHost: node.host,
      status: 'connecting',
      title: node.name,
    }
    tabs.value.push(tab)
    activateTab(tabId)
    addToRecent(nodeId)
    return tabId
  }

  function activateTab(tabId: string) {
    activeTabId.value = tabId
    if (splitDirection.value === 'none') {
      leftTabId.value = tabId
    } else {
      // In split mode, activate in whichever pane is "active"
      if (rightTabId.value === null) {
        rightTabId.value = tabId
      } else {
        leftTabId.value = tabId
      }
    }
  }

  function closeTab(tabId: string) {
    const idx = tabs.value.findIndex(t => t.id === tabId)
    if (idx === -1) return

    tabs.value.splice(idx, 1)

    if (leftTabId.value === tabId) leftTabId.value = null
    if (rightTabId.value === tabId) rightTabId.value = null

    if (activeTabId.value === tabId) {
      const remaining = tabs.value[tabs.value.length - 1]
      activeTabId.value = remaining?.id || null
      if (remaining && !leftTabId.value && !rightTabId.value) {
        leftTabId.value = remaining.id
      }
    }

    // If only one tab left in split, collapse split
    if (tabs.value.length <= 1) {
      splitDirection.value = 'none'
      rightTabId.value = null
      if (tabs.value.length === 1) {
        leftTabId.value = tabs.value[0].id
      }
    }
  }

  function closeAllTabs() {
    tabs.value = []
    activeTabId.value = null
    leftTabId.value = null
    rightTabId.value = null
    splitDirection.value = 'none'
  }

  function toggleSplit(dir: SplitDirection) {
    if (splitDirection.value === dir) {
      // Collapse split
      splitDirection.value = 'none'
      rightTabId.value = null
    } else {
      splitDirection.value = dir
      if (!rightTabId.value && tabs.value.length > 1) {
        const otherTab = tabs.value.find(t => t.id !== leftTabId.value)
        if (otherTab) rightTabId.value = otherTab.id
      }
    }
  }

  function setPaneTab(pane: 'left' | 'right', tabId: string) {
    if (pane === 'left') leftTabId.value = tabId
    else rightTabId.value = tabId
  }

  function updateTabStatus(tabId: string, status: TerminalTab['status']) {
    const tab = tabs.value.find(t => t.id === tabId)
    if (tab) tab.status = status
  }

  function switchTab(direction: 1 | -1) {
    if (tabs.value.length === 0) return
    const idx = tabs.value.findIndex(t => t.id === activeTabId.value)
    const nextIdx = (idx + direction + tabs.value.length) % tabs.value.length
    activateTab(tabs.value[nextIdx].id)
  }

  return {
    tabs,
    activeTabId,
    splitDirection,
    leftTabId,
    rightTabId,
    allNodes,
    recentNodes,
    loadNodes,
    openTab,
    activateTab,
    closeTab,
    closeAllTabs,
    toggleSplit,
    setPaneTab,
    updateTabStatus,
    switchTab,
  }
}
