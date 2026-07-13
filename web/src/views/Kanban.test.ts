import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/api/nodes', () => ({
  listNodes: vi.fn().mockResolvedValue([]),
  getMetrics: vi.fn(),
}))

import Kanban from './Kanban.vue'

describe('Kanban', () => {
  it('shows an actionable empty state when the fleet is empty', async () => {
    const i18n = createI18n({
      legacy: false,
      locale: 'en-US',
      messages: { 'en-US': { common: { firstNodeTitle: 'Add your first node', firstNodeDescription: 'Connect a server', addNode: 'Add node', noNodes: 'No nodes', nodeStatus: 'Node status', all: 'All', online: 'Online', offline: 'Offline', untrusted: 'Untrusted', nodeCount: 'Nodes', onlineRate: 'Online rate', cpuCores: 'CPU cores', averageCpu: 'Average CPU', memory: 'Memory', disk: 'Disk', networkTraffic: 'Network traffic', collecting: 'Collecting', physicalMachine: 'Physical machine', refresh: 'Refresh' } } },
    })
    const push = vi.fn()
    const wrapper = mount(Kanban, { global: { plugins: [createPinia(), i18n], config: { globalProperties: { $router: { push } } as any } } })

    await flushPromises()

    expect(wrapper.find('.first-node-empty').exists()).toBe(true)
    await wrapper.get('button.btn-primary').trigger('click')
    expect(push).toHaveBeenCalledWith('/nodes')
    wrapper.unmount()
  })
})
