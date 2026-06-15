import client from './client'

export interface ContainerInfo {
  id: string
  name: string
  image: string
  status: string
  state: string
  ports: { host_port: string; container_port: string }[]
}

export interface ImageInfo {
  id: string
  tags: string
  size: string
  created: string
}

export interface NetworkInfo {
  id: string
  name: string
  driver: string
  scope: string
}

export interface VolumeInfo {
  name: string
  driver: string
  mountpoint: string
}

export async function getDockerInfo(nodeId: string) {
  const { data } = await client.get(`/nodes/${nodeId}/docker/info`)
  return data.data
}

export async function listContainers(nodeId: string, all = false): Promise<ContainerInfo[]> {
  const { data } = await client.get(`/nodes/${nodeId}/docker/containers`, { params: { all } })
  return data.data || []
}

export async function containerAction(nodeId: string, containerId: string, action: string) {
  const { data } = await client.post(`/nodes/${nodeId}/docker/containers/${containerId}/action`, { action })
  return data.data
}

export async function listImages(nodeId: string): Promise<ImageInfo[]> {
  const { data } = await client.get(`/nodes/${nodeId}/docker/images`)
  return data.data || []
}

export async function pullImage(nodeId: string, image: string) {
  const { data } = await client.post(`/nodes/${nodeId}/docker/images/pull`, { image })
  return data.data
}

export async function removeImage(nodeId: string, imageId: string, force = false) {
  const { data } = await client.delete(`/nodes/${nodeId}/docker/images/${imageId}`, { params: { force } })
  return data.data
}

export async function listNetworks(nodeId: string): Promise<NetworkInfo[]> {
  const { data } = await client.get(`/nodes/${nodeId}/docker/networks`)
  return data.data || []
}

export async function listVolumes(nodeId: string): Promise<VolumeInfo[]> {
  const { data } = await client.get(`/nodes/${nodeId}/docker/volumes`)
  return data.data || []
}
