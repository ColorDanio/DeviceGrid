import client from './client'

export interface FileEntry {
  name: string
  path: string
  is_dir: boolean
  size: number
  mod_time: number
  mode: string
}

export async function listFiles(nodeId: string, dirPath: string) {
  const { data } = await client.get(`/nodes/${nodeId}/sftp/list`, { params: { path: dirPath } })
  return data.data as { path: string; entries: FileEntry[] }
}

export async function uploadFile(nodeId: string, dirPath: string, file: File) {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('path', dirPath)
  const { data } = await client.post(`/nodes/${nodeId}/sftp/upload`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return data.data
}

export async function deleteFile(nodeId: string, filePath: string) {
  const { data } = await client.delete(`/nodes/${nodeId}/sftp/delete`, { params: { path: filePath } })
  return data.data
}

export async function mkdir(nodeId: string, dirPath: string) {
  const { data } = await client.post(`/nodes/${nodeId}/sftp/mkdir`, { path: dirPath })
  return data.data
}

export function getDownloadUrl(nodeId: string, filePath: string) {
  return `/api/nodes/${nodeId}/sftp/download?path=${encodeURIComponent(filePath)}`
}

export async function downloadFile(nodeId: string, filePath: string, fileName: string) {
  const resp = await client.get(getDownloadUrl(nodeId, filePath), { responseType: 'blob' })
  const url = window.URL.createObjectURL(resp.data)
  const a = document.createElement('a')
  a.href = url
  a.download = fileName
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(url)
}
