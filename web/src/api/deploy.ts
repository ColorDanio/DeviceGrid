import client from './client'

export interface DeployTask {
  id: string
  name: string
  type: string
  node_ids: string[]
  payload: string
  timeout: number
  concurrency: number
  status: string
  created_by: string
  created_at: string
  started_at?: string
  finished_at?: string
}

export interface DeployResult {
  id: string
  task_id: string
  node_id: string
  node_name: string
  status: string
  exit_code: number
  output: string
  error: string
  duration_ms: number
  started_at: string
  finished_at?: string
}

export interface CreateDeployRequest {
  name: string
  type: string
  node_ids: string[]
  payload: string
  timeout?: number
  concurrency?: number
}

export async function listDeploys(limit = 50, offset = 0): Promise<DeployTask[]> {
  const { data } = await client.get('/deploys', { params: { limit, offset } })
  return data.data || []
}

export async function createDeploy(req: CreateDeployRequest): Promise<DeployTask> {
  const { data } = await client.post('/deploys', req)
  return data.data
}

export async function getDeploy(taskId: string): Promise<{ task: DeployTask; results: DeployResult[] }> {
  const { data } = await client.get(`/deploys/${taskId}`)
  return data.data
}

export async function cancelDeploy(taskId: string): Promise<void> {
  await client.delete(`/deploys/${taskId}`)
}
