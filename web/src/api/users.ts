import client from './client'

export interface User {
  id: string
  username: string
  role: string
  created_at: string
}

export async function listUsers(): Promise<User[]> {
  const { data } = await client.get('/users')
  return data.data || []
}

export async function createUser(username: string, password: string, role: string): Promise<User> {
  const { data } = await client.post('/users', { username, password, role })
  return data.data
}

export async function deleteUser(id: string): Promise<void> {
  await client.delete(`/users/${id}`)
}
