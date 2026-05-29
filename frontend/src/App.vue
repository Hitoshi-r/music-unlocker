<template>
  <div class="app">
    <header class="topbar">
      <div>
        <p class="eyebrow">Music Unlocker</p>
        <h1>音频格式识别与批量转换</h1>
      </div>
      <div class="summary">
        <div>
          <strong>{{ stats.total }}</strong>
          <span>文件</span>
        </div>
        <div>
          <strong>{{ stats.done }}</strong>
          <span>完成</span>
        </div>
        <div>
          <strong>{{ stats.failed }}</strong>
          <span>失败</span>
        </div>
      </div>
    </header>

    <main class="layout">
      <section class="workspace">
        <div
          class="drop-zone"
          :class="{ active: isDragging }"
          @dragenter.prevent="isDragging = true"
          @dragover.prevent
          @dragleave.prevent="isDragging = false"
          @drop.prevent="handleDrop"
        >
          <div>
            <strong>拖入音频文件</strong>
            <span>支持标准音频、NCM、旧版 QMC、KGM、KWM、XM；新版 QQ 受保护格式会被识别并给出提示。</span>
          </div>
          <button @click="selectFiles">选择文件</button>
        </div>

        <div class="toolbar">
          <input v-model="keyword" placeholder="搜索文件名、路径或平台" />
          <select v-model="statusFilter">
            <option value="all">全部状态</option>
            <option value="pending">待处理</option>
            <option value="processing">处理中</option>
            <option value="done">已完成</option>
            <option value="failed">失败</option>
            <option value="unsupported">不支持</option>
          </select>
          <button class="ghost" @click="selectFolder">选择文件夹</button>
          <button class="ghost" :disabled="isRunning" @click="clearFinished">清理完成项</button>
          <button class="ghost danger" :disabled="isRunning || files.length === 0" @click="clearFiles">清空</button>
        </div>

        <div class="file-list">
          <div v-if="filteredFiles.length === 0" class="empty">
            {{ files.length === 0 ? '还没有文件，先添加一些音频。' : '没有匹配当前筛选条件的文件。' }}
          </div>

          <article
            v-for="file in filteredFiles"
            :key="file.id"
            class="file-item"
            :class="getStatusClass(file.status)"
          >
            <div class="file-head">
              <div class="file-title">
                <strong>{{ getFileName(file.path) }}</strong>
                <span>{{ file.platform }} · {{ getFileExt(file.path) || '未知扩展名' }}</span>
              </div>
              <div class="file-actions">
                <span class="badge">{{ statusLabel(file.status) }}</span>
                <button class="icon-btn" :disabled="isRunning" title="移除" @click="removeFile(file.id)">×</button>
              </div>
            </div>

            <div class="file-path">{{ file.path }}</div>
            <div v-if="file.message" class="message">{{ file.message }}</div>

            <div class="progress">
              <div class="bar" :style="{ width: file.progress + '%' }"></div>
            </div>
          </article>
        </div>
      </section>

      <aside class="side-panel">
        <section class="panel-section">
          <h2>输出设置</h2>

          <label>
            输出格式
            <select v-model="outputFormat">
              <option value="origin">保持原格式</option>
              <option value="mp3">MP3</option>
              <option value="flac">FLAC</option>
              <option value="wav">WAV</option>
              <option value="ogg">OGG</option>
            </select>
          </label>

          <label>
            MP3 码率
            <select v-model="bitrate" :disabled="outputFormat !== 'mp3'">
              <option value="128k">128k</option>
              <option value="192k">192k</option>
              <option value="320k">320k</option>
            </select>
          </label>

          <label>
            并发任务
            <input v-model.number="concurrency" type="number" min="1" max="8" />
          </label>

          <button @click="chooseOutputDir">选择输出目录</button>
          <div class="output-path">{{ outputDir || '未选择时输出到原文件目录' }}</div>
          <button class="ghost" @click="openOutputDir">打开输出目录</button>
        </section>

        <section class="panel-section">
          <h2>任务控制</h2>
          <button class="start" :disabled="isRunning || runnableCount === 0" @click="startConvert">
            开始转换 {{ runnableCount ? `(${runnableCount})` : '' }}
          </button>
          <button class="ghost" :disabled="!isRunning" @click="togglePause">
            {{ isPaused ? '继续任务' : '暂停任务' }}
          </button>
          <button class="ghost danger" :disabled="!isRunning" @click="cancelAll">
            取消任务
          </button>
        </section>

        <section class="panel-section">
          <h2>处理日志</h2>
          <div class="log-list">
            <div v-for="entry in logs" :key="entry.id" class="log-item">
              <span>{{ entry.time }}</span>
              <p>{{ entry.text }}</p>
            </div>
          </div>
        </section>
      </aside>
    </main>
  </div>
</template>

<script setup>
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import {
  CancelFile,
  ConvertFile,
  OpenFolder,
  SelectFiles,
  SelectFolderFiles,
  SelectOutputDir
} from '../wailsjs/go/main/App'
import { EventsOff, EventsOn, OnFileDrop, OnFileDropOff } from '../wailsjs/runtime/runtime'

const files = ref([])
const outputDir = ref('')
const outputFormat = ref('origin')
const bitrate = ref('320k')
const concurrency = ref(3)
const isRunning = ref(false)
const isPaused = ref(false)
const isCancelled = ref(false)
const isDragging = ref(false)
const keyword = ref('')
const statusFilter = ref('all')
const logs = ref([])

const platforms = {
  '.mp3': '标准音频',
  '.flac': '标准音频',
  '.ogg': '标准音频',
  '.wav': '标准音频',
  '.aac': '标准音频',
  '.m4a': '标准音频',
  '.ncm': '网易云音乐',
  '.qmc0': 'QQ音乐',
  '.qmc3': 'QQ音乐',
  '.qmcflac': 'QQ音乐',
  '.qmcogg': 'QQ音乐',
  '.mflac': 'QQ音乐新版',
  '.mgg': 'QQ音乐新版',
  '.kgm': '酷狗音乐',
  '.vpr': '酷狗音乐',
  '.kwm': '酷我音乐',
  '.xm': '虾米音乐'
}

const stats = computed(() => ({
  total: files.value.length,
  done: files.value.filter(file => file.status === 'done').length,
  failed: files.value.filter(file => file.status === 'failed').length
}))

const runnableCount = computed(() =>
  files.value.filter(file => ['pending', 'failed'].includes(file.status)).length
)

const filteredFiles = computed(() => {
  const text = keyword.value.trim().toLowerCase()

  return files.value.filter(file => {
    const matchesStatus = statusFilter.value === 'all' || file.status === statusFilter.value
    const haystack = `${file.path} ${file.platform} ${file.message}`.toLowerCase()
    return matchesStatus && (!text || haystack.includes(text))
  })
})

onMounted(() => {
  EventsOn('convert-progress', payload => {
    const target = files.value.find(file => file.path === payload.path)
    if (target) {
      target.progress = payload.percent
    }
  })

  OnFileDrop((_, __, paths) => addFiles(paths), false)
})

onBeforeUnmount(() => {
  EventsOff('convert-progress')
  OnFileDropOff()
})

async function selectFiles() {
  addFiles(await SelectFiles())
}

async function selectFolder() {
  addFiles(await SelectFolderFiles())
}

function addFiles(paths = []) {
  isDragging.value = false

  const items = paths
    .filter(Boolean)
    .filter(path => !files.value.some(file => file.path === path))
    .map(path => {
      const platform = getPlatform(path)
      const protectedFormat = ['.mflac', '.mgg'].includes(getFileExt(path))
      const supported = platform !== '未知格式'

      return {
        id: crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random()}`,
        path,
        platform,
        status: supported ? 'pending' : 'unsupported',
        progress: 0,
        message: protectedFormat ? '新版受保护格式：仅识别，不提供解密绕过。' : ''
      }
    })

  files.value.push(...items)
  if (items.length) {
    addLog(`添加 ${items.length} 个文件`)
  }
}

function handleDrop(event) {
  const paths = Array.from(event.dataTransfer.files)
    .map(file => file.path)
    .filter(Boolean)
  addFiles(paths)
}

async function chooseOutputDir() {
  const dir = await SelectOutputDir()
  if (dir) {
    outputDir.value = dir
    addLog(`输出目录：${dir}`)
  }
}

async function openOutputDir() {
  if (!outputDir.value) {
    alert('请先选择输出目录')
    return
  }
  await OpenFolder(outputDir.value)
}

function getFileName(path) {
  return path.split('\\').pop().split('/').pop()
}

function getFileExt(path) {
  const name = getFileName(path)
  const index = name.lastIndexOf('.')
  return index === -1 ? '' : name.slice(index).toLowerCase()
}

function getPlatform(path) {
  return platforms[getFileExt(path)] || '未知格式'
}

function removeFile(id) {
  files.value = files.value.filter(file => file.id !== id)
}

function clearFiles() {
  files.value = []
  logs.value = []
}

function clearFinished() {
  files.value = files.value.filter(file => file.status !== 'done')
}

function togglePause() {
  isPaused.value = !isPaused.value
  addLog(isPaused.value ? '任务已暂停' : '任务继续')
}

async function cancelAll() {
  isCancelled.value = true
  isPaused.value = false

  await Promise.all(
    files.value
      .filter(file => file.status === 'processing')
      .map(file => CancelFile(file.path).catch(() => {}))
  )

  files.value.forEach(file => {
    if (['pending', 'processing'].includes(file.status)) {
      file.status = 'failed'
      file.message = '任务已取消'
      file.progress = 0
    }
  })
  addLog('已取消正在运行的任务')
}

async function startConvert() {
  isRunning.value = true
  isPaused.value = false
  isCancelled.value = false

  const queue = files.value.filter(file => ['pending', 'failed'].includes(file.status))
  const workerCount = Math.max(1, Math.min(Number(concurrency.value) || 1, 8, queue.length || 1))
  let cursor = 0

  addLog(`开始处理 ${queue.length} 个文件，并发 ${workerCount}`)

  async function waitIfPaused() {
    while (isPaused.value && !isCancelled.value) {
      await new Promise(resolve => setTimeout(resolve, 250))
    }
  }

  async function worker() {
    while (cursor < queue.length && !isCancelled.value) {
      await waitIfPaused()
      if (isCancelled.value) break

      const file = queue[cursor++]
      if (!file || file.status === 'unsupported') continue

      file.status = 'processing'
      file.progress = Math.max(file.progress, 5)
      file.message = '处理中'

      try {
        const message = await ConvertFile(file.path, outputDir.value, outputFormat.value, bitrate.value)
        file.status = 'done'
        file.progress = 100
        file.message = message
        addLog(`完成：${getFileName(file.path)}`)
      } catch (err) {
        file.status = 'failed'
        file.progress = 0
        file.message = cleanError(err)
        addLog(`失败：${getFileName(file.path)} - ${file.message}`)
      }
    }
  }

  try {
    await Promise.all(Array.from({ length: workerCount }, worker))
  } finally {
    isRunning.value = false
    isPaused.value = false
    addLog('任务队列结束')
  }
}

function cleanError(err) {
  const text = String(err?.message || err || '转换失败')
  return text.replace(/^Error:\s*/i, '')
}

function statusLabel(status) {
  const map = {
    pending: '待处理',
    processing: '处理中',
    done: '完成',
    failed: '失败',
    unsupported: '不支持'
  }
  return map[status] || status
}

function getStatusClass(status) {
  return status
}

function addLog(text) {
  logs.value.unshift({
    id: `${Date.now()}-${Math.random()}`,
    time: new Date().toLocaleTimeString(),
    text
  })
  logs.value = logs.value.slice(0, 80)
}
</script>

<style src="./styles/app.css"></style>
