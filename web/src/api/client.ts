import axios, { type AxiosInstance } from 'axios'
import { ElMessage } from 'element-plus'

const client: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

client.interceptors.request.use(
  (config) => {
    const token = sessionStorage.getItem('dg_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error),
)

client.interceptors.response.use(
  (response) => {
    const data = response.data
    if (data && data.code !== undefined && data.code !== 0) {
      ElMessage.error(data.message || 'Request failed')
      return Promise.reject(new Error(data.message))
    }
    return response
  },
  (error) => {
    if (error.response) {
      const { status, data } = error.response
      if (status === 401) {
        sessionStorage.removeItem('dg_token')
        localStorage.removeItem('dg_user')
        if (window.location.pathname !== '/login') {
          window.location.href = '/login'
        }
        ElMessage.error('登录已过期，请重新登录')
      } else {
        ElMessage.error(data?.message || error.message || '网络错误')
      }
    } else {
      ElMessage.error(error.message || '网络连接失败')
    }
    return Promise.reject(error)
  },
)

export default client
