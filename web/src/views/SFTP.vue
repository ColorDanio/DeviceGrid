<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title-group">
        <h2>文件管理</h2>
        <p class="page-subtitle">通过 SFTP 浏览和管理节点文件系统</p>
      </div>
      <NodeSelector v-model="selectedNode" placeholder="选择节点..." />
    </div>

    <div v-if="!selectedNode" class="dg-card empty-state-card">
      <svg viewBox="0 0 24 24" width="48" height="48" fill="none" style="opacity:0.2;margin-bottom:12px"><path d="M3 7l2-3h6l2 3h8v12H3V7z" stroke="currentColor" stroke-width="1.5"/></svg>
      <p style="color:var(--dg-text-faint)">请选择一个节点以浏览文件系统</p>
    </div>

    <div v-else class="dg-card sftp-panel">
      <div class="file-toolbar">
        <div class="breadcrumb">
          <div class="crumb" @click="navigateTo('/')">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none"><path d="M3 12l9-9 9 9M5 10v10h14V10" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </div>
          <template v-for="(seg, i) in pathSegments" :key="i">
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" style="color:var(--dg-text-faint)"><path d="M9 18l6-6-6-6" stroke="currentColor" stroke-width="2"/></svg>
            <div class="crumb" @click="navigateTo(seg.path)">{{ seg.name }}</div>
          </template>
        </div>
        <div class="toolbar-actions">
          <button class="tool-btn" @click="showMkdir = true" title="新建文件夹">
            <svg viewBox="0 0 24 24" width="15" height="15" fill="none"><path d="M3 7l2-3h6l2 3h8v12H3V7z" stroke="currentColor" stroke-width="1.8"/><path d="M12 11v6M9 14h6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
          </button>
          <label class="tool-btn tool-upload" title="上传文件">
            <svg viewBox="0 0 24 24" width="15" height="15" fill="none"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M7 10l5-5 5 5M12 5v12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
            <input type="file" multiple @change="handleUpload" style="display:none" />
          </label>
          <button class="tool-btn" @click="loadFiles" title="刷新">
            <svg viewBox="0 0 24 24" width="15" height="15" fill="none"><path d="M23 4v6h-6M1 20v-6h6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/><path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </button>
        </div>
      </div>

      <div class="file-list" v-loading="loading">
        <div v-if="files.length === 0 && !loading" class="file-empty">空目录</div>
        <div v-for="f in files" :key="f.path" class="file-row" :class="{ dir: f.is_dir }" @dblclick="f.is_dir ? navigateTo(f.path) : null">
          <div class="f-icon">
            <svg v-if="f.is_dir" viewBox="0 0 24 24" width="18" height="18" fill="none"><path d="M3 7l2-3h6l2 3h8v12H3V7z" stroke="var(--dg-cyan)" stroke-width="1.8"/></svg>
            <svg v-else viewBox="0 0 24 24" width="18" height="18" fill="none"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" stroke="var(--dg-text-faint)" stroke-width="1.8"/><path d="M14 2v6h6" stroke="var(--dg-text-faint)" stroke-width="1.8"/></svg>
          </div>
          <div class="f-name">{{ f.name }}</div>
          <div class="f-size">{{ f.is_dir ? '—' : formatSize(f.size) }}</div>
          <div class="f-time">{{ formatTime(f.mod_time) }}</div>
          <div class="f-actions" v-if="!f.is_dir">
            <button class="f-btn" @click.stop="handleDownload(f)" title="下载">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M7 10l5 5 5-5M12 5v12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
            </button>
          </div>
          <div class="f-actions" v-else></div>
          <button class="f-btn f-delete" @click.stop="handleDelete(f)" title="删除">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none"><path d="M3 6h18M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2m3 0v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6h14z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </button>
        </div>
      </div>
    </div>

    <el-dialog v-model="showMkdir" title="新建文件夹" width="360px">
      <el-input v-model="mkdirName" placeholder="文件夹名称" @keyup.enter="doMkdir" />
      <template #footer>
        <el-button @click="showMkdir = false">取消</el-button>
        <el-button type="primary" @click="doMkdir">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import NodeSelector from '@/components/NodeSelector.vue'
import { listFiles, uploadFile, deleteFile, mkdir, downloadFile, type FileEntry } from '@/api/sftp'

const selectedNode = ref('')
const currentPath = ref('/')
const files = ref<FileEntry[]>([])
const loading = ref(false)
const showMkdir = ref(false)
const mkdirName = ref('')

const pathSegments = computed(() => {
  const parts = currentPath.value.split('/').filter(Boolean)
  const segs: { name: string; path: string }[] = []
  let p = ''
  for (const part of parts) {
    p += '/' + part
    segs.push({ name: part, path: p })
  }
  return segs
})

async function loadFiles() {
  if (!selectedNode.value) return
  loading.value = true
  try {
    const result = await listFiles(selectedNode.value, currentPath.value)
    files.value = result.entries
  } catch {} finally { loading.value = false }
}

function navigateTo(p: string) {
  currentPath.value = p
  loadFiles()
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MB'
  return (bytes / 1073741824).toFixed(1) + ' GB'
}

function formatTime(ts: number): string {
  if (!ts) return '—'
  return new Date(ts * 1000).toLocaleString()
}

function downloadUrl(filePath: string): string { return '' }
async function handleDownload(f: FileEntry) {
  await downloadFile(selectedNode.value, f.path, f.name)
}

async function handleUpload(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files) return
  for (const file of Array.from(input.files)) {
    try {
      await uploadFile(selectedNode.value, currentPath.value, file)
      ElMessage.success(`已上传: ${file.name}`)
    } catch {}
  }
  input.value = ''
  loadFiles()
}

async function handleDelete(f: FileEntry) {
  await ElMessageBox.confirm(`确定删除 ${f.name}？`, '', { type: 'warning' })
  await deleteFile(selectedNode.value, f.path)
  ElMessage.success('已删除')
  loadFiles()
}

async function doMkdir() {
  if (!mkdirName.value) return
  const fullPath = currentPath.value === '/' ? '/' + mkdirName.value : currentPath.value + '/' + mkdirName.value
  await mkdir(selectedNode.value, fullPath)
  ElMessage.success('已创建')
  showMkdir.value = false
  mkdirName.value = ''
  loadFiles()
}

watch(selectedNode, () => { currentPath.value = '/'; loadFiles() })
</script>

<style scoped lang="scss">
.empty-state-card { min-height: 400px; display: flex; flex-direction: column; align-items: center; justify-content: center; }

.sftp-panel { padding: 16px; }

.file-toolbar {
  display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; padding-bottom: 12px; border-bottom: 1px solid var(--dg-border);
  .breadcrumb { display: flex; align-items: center; gap: 4px; flex-wrap: wrap; }
  .crumb {
    padding: 4px 10px; border-radius: 6px; font-size: 13px; color: var(--dg-text-dim); cursor: pointer; transition: all 0.15s;
    &:hover { background: var(--dg-table-row-hover); color: var(--dg-cyan); }
  }
  .toolbar-actions { display: flex; gap: 6px; }
  .tool-btn {
    width: 32px; height: 32px; border-radius: 8px; border: 1px solid var(--dg-border); background: var(--dg-bg-2);
    color: var(--dg-text-dim); cursor: pointer; display: flex; align-items: center; justify-content: center; transition: all 0.2s;
    &:hover { border-color: var(--dg-cyan); color: var(--dg-cyan); }
    &.tool-upload { position: relative; overflow: hidden; }
  }
}

.file-list { min-height: 300px; }
.file-empty { padding: 40px; text-align: center; color: var(--dg-text-faint); font-size: 14px; }

.file-row {
  display: flex; align-items: center; gap: 12px; padding: 8px 12px; border-radius: 8px; cursor: default; transition: all 0.15s;
  &:hover { background: var(--dg-table-row-hover); }
  &.dir { cursor: pointer; .f-name { color: var(--dg-cyan); font-weight: 500; } }
  .f-icon { width: 20px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
  .f-name { flex: 1; font-size: 13px; color: var(--dg-text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .f-size { width: 80px; text-align: right; font-size: 12px; color: var(--dg-text-faint); font-family: 'JetBrains Mono', monospace; }
  .f-time { width: 150px; font-size: 12px; color: var(--dg-text-faint); }
  .f-actions { width: 28px; display: flex; gap: 4px; }
  .f-btn {
    width: 26px; height: 26px; border-radius: 6px; border: none; background: transparent; color: var(--dg-text-faint); cursor: pointer;
    display: flex; align-items: center; justify-content: center; transition: all 0.15s; text-decoration: none;
    &:hover { background: var(--dg-success-bg); color: var(--dg-cyan); }
    &.f-delete:hover { background: var(--dg-danger-bg); color: var(--dg-red); }
  }
}
</style>
