import client from './client'

export interface AlertRule {
  id: string
  name: string
  enabled: boolean
  metric: string // "cpu" | "mem" | "disk" | "node_offline"
  operator: string // ">" | "<"
  threshold: number
  webhook_url: string
  cooldown_min: number
}

export async function listAlertRules(): Promise<AlertRule[]> {
  const { data } = await client.get('/alerts/rules')
  return data.data || []
}

export async function createAlertRule(rule: Partial<AlertRule>): Promise<AlertRule> {
  const { data } = await client.post('/alerts/rules', rule)
  return data.data
}

export async function deleteAlertRule(id: string): Promise<void> {
  await client.delete(`/alerts/rules/${id}`)
}

export async function testWebhook(url: string): Promise<void> {
  await client.post('/alerts/test', { url })
}

export interface CronTask {
  id: string
  name: string
  node_ids: string[]
  script: string
  interval: string
  enabled: boolean
  created_at: string
  last_run?: string
  next_run: string
}

export async function listCronTasks(): Promise<CronTask[]> {
  const { data } = await client.get('/cron')
  return data.data || []
}

export async function createCronTask(req: {name: string; node_ids: string[]; script: string; interval: string}): Promise<CronTask> {
  const { data } = await client.post('/cron', req)
  return data.data
}

export async function deleteCronTask(id: string): Promise<void> {
  await client.delete(`/cron/${id}`)
}

export async function toggleCronTask(id: string): Promise<CronTask> {
  const { data } = await client.post(`/cron/${id}/toggle`)
  return data.data
}
