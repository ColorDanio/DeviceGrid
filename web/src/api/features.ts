import client from './client'

// K8s Workload Management
export async function getK8sResources(clusterId: string, type?: string, namespace?: string) {
  const { data } = await client.get(`/clusters/${clusterId}/resources`, { params: { type, namespace } })
  return data.data?.output || ''
}

export async function applyYAML(clusterId: string, yaml: string) {
  const { data } = await client.post(`/clusters/${clusterId}/apply`, { yaml })
  return data.data
}

export async function deleteK8sResource(clusterId: string, kind: string, name: string, namespace?: string) {
  const { data } = await client.post(`/clusters/${clusterId}/delete-resource`, { kind, name, namespace })
  return data.data
}

// SSH Key Management
export async function getSSHKeyInfo(nodeId: string) {
  const { data } = await client.get(`/nodes/${nodeId}/ssh-key`)
  return data.data?.output || ''
}

export async function rotateSSHKey(nodeId: string) {
  const { data } = await client.post(`/nodes/${nodeId}/rotate-key`)
  return data.data
}

// Node Comparison
export async function compareNodes(nodeA: string, nodeB: string) {
  const { data } = await client.get('/nodes/compare', { params: { a: nodeA, b: nodeB } })
  return data.data
}

// Audit Log
export interface AuditEntry {
  timestamp: string
  method: string
  path: string
  user: string
  ip: string
  status: number
  duration: string
}

export async function getAuditLog(): Promise<AuditEntry[]> {
  const { data } = await client.get('/audit')
  return data.data || []
}

// Metrics Export
export async function downloadMetricsCSV() {
  const resp = await client.get('/metrics/export', { responseType: 'blob' })
  const blob = new Blob([resp.data], { type: 'text/csv' })
  const link = document.createElement('a')
  link.href = window.URL.createObjectURL(blob)
  link.download = 'devicegrid_metrics.csv'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(link.href)
}

// Batch File Distribution
export async function distributeFile(nodeIds: string[], file: File, remotePath?: string) {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('node_ids', nodeIds.join(','))
  if (remotePath) formData.append('remote_path', remotePath)
  const { data } = await client.post('/nodes/distribute-file', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return data.data
}
