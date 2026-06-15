import client from './client'

export interface LoginRequest {
  username: string
  password: string
}

export interface UserInfo {
  token: string
  username: string
  role: string
}

export async function login(req: LoginRequest): Promise<UserInfo> {
  const { data } = await client.post('/auth/login', req)
  return data.data
}

export async function getMe(): Promise<{ user_id: string; username: string; role: string }> {
  const { data } = await client.get('/auth/me')
  return data.data
}

export async function refreshToken(): Promise<string> {
  const { data } = await client.post('/auth/refresh')
  return data.data.token
}
