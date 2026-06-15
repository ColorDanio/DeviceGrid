import client from './client'

export interface NodeFilter {
  search?: string
  status?: string
  tag?: string
}

export interface Node {
  id: string
  name: string
  host: string
  port: number
  username: string
  auth_mode: string
  transport_mode: string
  agent_port: number
  status: string
  tags: string[]
  os: string
  arch: string
  docker_version: string
  rke2_role: string
  cluster_id?: string
  country?: string
  country_code?: string
  region?: string
  isp?: string
  last_seen_at: string
  created_at: string
  updated_at: string
}

export interface CreateNodeRequest {
  name: string
  host: string
  port?: number
  username?: string
  password: string
  private_key?: string
  tags?: string[]
}

export async function listNodes(filter?: NodeFilter): Promise<Node[]> {
  const { data } = await client.get('/nodes', { params: filter })
  return data.data || []
}

export async function getNode(id: string): Promise<Node> {
  const { data } = await client.get(`/nodes/${id}`)
  return data.data
}

export async function createNode(req: CreateNodeRequest): Promise<Node> {
  const { data } = await client.post('/nodes', req)
  return data.data
}

export async function updateNode(id: string, req: Partial<CreateNodeRequest>): Promise<Node> {
  const { data } = await client.put(`/nodes/${id}`, req)
  return data.data
}

export async function deleteNode(id: string): Promise<void> {
  await client.delete(`/nodes/${id}`)
}

export async function checkHealth(id: string): Promise<any> {
  const { data } = await client.post(`/nodes/${id}/health`)
  return data.data
}

export async function establishTrust(id: string): Promise<any> {
  const { data } = await client.post(`/nodes/${id}/trust`)
  return data.data
}

export async function deployAgent(id: string): Promise<any> {
  const { data } = await client.post(`/nodes/${id}/deploy-agent`)
  return data.data
}

export interface NodeMetrics {
  cpu_usage: number
  cpu_model: string
  cpu_cores: number
  cpu_sockets: number
  cpu_threads: number
  load_avg_1: number
  load_avg_5: number
  load_avg_15: number
  mem_total: number
  mem_used: number
  swap_total: number
  swap_used: number
  disk_total: number
  disk_used: number
  virt_type: string
  net_iface: string
  net_rx: number
  net_tx: number
  uptime: number
  gpus: GPUInfo[]
}

export interface GPUInfo {
  index: number
  name: string
  memory_total: number
  memory_used: number
  utilization: number
  temperature: number
}

export async function getMetrics(id: string): Promise<NodeMetrics> {
  const { data } = await client.get(`/nodes/${id}/metrics`)
  return data.data
}

export async function getTopProcesses(id: string): Promise<string> {
  const { data } = await client.get(`/nodes/${id}/processes`)
  return data.data.output
}

export async function getLoginHistory(id: string): Promise<string> {
  const { data } = await client.get(`/nodes/${id}/logins`)
  return data.data.output
}

export async function getNetworkConfig(): Promise<NetworkFeatures> {
  const { data } = await client.get('/network/config')
  return data.data
}

export interface NetworkFeatures {
  environment: string
  enable_geo: boolean
  enable_streaming: boolean
  enable_ai: boolean
  enable_connectivity: boolean
  enable_route: boolean
}

export interface StreamingResult {
  name: string
  status: string
  region?: string
}

export interface CheckResponse<T> {
  results: T[]
  tested_at: string
}

export async function checkStreaming(id: string): Promise<CheckResponse<StreamingResult>> {
  const { data } = await client.get(`/nodes/${id}/streaming`, { timeout: 120000 })
  return data.data
}

export async function checkAI(id: string): Promise<CheckResponse<StreamingResult>> {
  const { data } = await client.get(`/nodes/${id}/ai-check`, { timeout: 90000 })
  return data.data
}

export interface ConnectivityResult {
  region: string
  latency_ms: number
  loss_pct: number
  ok: boolean
}

export async function checkConnectivity(id: string): Promise<CheckResponse<ConnectivityResult>> {
  const { data } = await client.get(`/nodes/${id}/connectivity`, { timeout: 60000 })
  return data.data
}

export interface RouteHop {
  isp: string
  city: string
  ip: string
  latency_ms: number
  loss_pct: number
  line_type: string
  hops: string[]
}

export async function checkReturnRoute(id: string): Promise<CheckResponse<RouteHop>> {
  const { data } = await client.get(`/nodes/${id}/return-route`, { timeout: 150000 })
  return data.data
}
