import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const listNodes = vi.fn().mockResolvedValue([{ id: 'node-1', status: 'online' }])
const getMetrics = vi.fn().mockResolvedValue({ cpu_usage: 1 })
vi.mock('@/api/nodes', () => ({ listNodes, getMetrics }))

class FakeWebSocket {
  static OPEN = 1
  static instance: FakeWebSocket | undefined
  readyState = FakeWebSocket.OPEN
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  sent: string[] = []
  constructor() { FakeWebSocket.instance = this }
  send(value: string) { this.sent.push(value) }
  close() { this.onclose?.() }
}

describe('fleet store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.setItem('dg_token', 'token')
    vi.stubGlobal('WebSocket', FakeWebSocket)
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
  })

  it('authenticates, subscribes, and applies metric broadcasts', async () => {
    const { useFleetStore } = await import('./fleet')
    const fleet = useFleetStore()
    fleet.start()
    await vi.waitFor(() => expect(FakeWebSocket.instance).toBeDefined())
    FakeWebSocket.instance!.onopen?.()
    expect(FakeWebSocket.instance!.sent[0]).toContain('token')
    expect(FakeWebSocket.instance!.sent[1]).toContain('metrics-node-1')
    FakeWebSocket.instance!.onmessage?.({ data: JSON.stringify({ topic: 'metrics-node-1', data: { cpu_usage: 42 } }) } as MessageEvent)
    expect(fleet.metrics['node-1'].cpu_usage).toBe(42)
    fleet.stop()
  })
})
