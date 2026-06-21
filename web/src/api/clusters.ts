import client from './client'

export interface Cluster {
  id: string
  name: string
  version: string
  server_node: string
  config: string
  nodes: ClusterNode[]
  status: string
  created_at: string
  updated_at: string
}

export interface ClusterNode {
  node_id: string
  role: string
  ready: boolean
}

export interface CreateClusterRequest {
  name: string
  version: string
  server_node: string
  agent_nodes: string[]
  config: {
    cni?: string
    cluster_cidr?: string
    service_cidr?: string
    proxy?: string
    no_proxy?: string
    etcd_snapshots?: boolean
    disable?: string[]
  }
}

export async function listClusters(): Promise<Cluster[]> {
  const { data } = await client.get('/clusters')
  return data.data || []
}

export async function getCluster(id: string): Promise<Cluster> {
  const { data } = await client.get(`/clusters/${id}`)
  return data.data
}

export async function createCluster(req: CreateClusterRequest): Promise<Cluster> {
  const { data } = await client.post('/clusters', req)
  return data.data
}

export async function deleteCluster(id: string): Promise<void> {
  await client.delete(`/clusters/${id}`)
}

export async function upgradeCluster(id: string, version: string): Promise<any> {
  const { data } = await client.post(`/clusters/${id}/upgrade`, { version })
  return data.data
}

export async function getClusterStatus(id: string): Promise<any[]> {
  const { data } = await client.get(`/clusters/${id}/status`)
  return data.data || []
}

export async function getClusterPods(id: string, namespace?: string): Promise<string[]> {
  const { data } = await client.get(`/clusters/${id}/pods`, { params: { namespace } })
  return data.data?.pods || []
}

export async function updateClusterConfig(id: string, config: string): Promise<any> {
  const { data } = await client.put(`/clusters/${id}/config`, { config })
  return data.data
}

// Pre-flight check
export interface PreFlightResult {
  node_id: string
  node_name: string
  all_passed: boolean
  checks: PreFlightCheck[]
}

export interface PreFlightCheck {
  name: string
  passed: boolean
  value: string
  required: string
  warning?: string
  fixed?: boolean
}

export async function preflightCheck(nodeId: string, autofix = false): Promise<PreFlightResult> {
  const method = autofix ? 'post' : 'get'
  const { data } = await client[method](`/nodes/${nodeId}/preflight`, { params: { autofix } })
  return data.data
}

// Helm
export async function helmList(clusterId: string): Promise<string> {
  const { data } = await client.get(`/clusters/${clusterId}/helm`)
  return data.data?.output || ''
}

export async function helmInstall(clusterId: string, req: {
  repo_name?: string
  repo_url?: string
  chart_name: string
  namespace?: string
  values?: Record<string, string>
}): Promise<any> {
  const { data } = await client.post(`/clusters/${clusterId}/helm/install`, req)
  return data.data
}

export async function helmUninstall(clusterId: string, release: string, namespace?: string): Promise<any> {
  const { data } = await client.delete(`/clusters/${clusterId}/helm/${release}`, { params: { namespace } })
  return data.data
}

// Rancher
export async function installRancher(clusterId: string, hostname: string, password?: string): Promise<any> {
  const { data } = await client.post(`/clusters/${clusterId}/rancher`, { hostname, password })
  return data.data
}

export async function rancherStatus(clusterId: string): Promise<{ installed: boolean; version: string }> {
  const { data } = await client.get(`/clusters/${clusterId}/rancher`)
  return data.data
}
