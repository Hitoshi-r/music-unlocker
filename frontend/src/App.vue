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
            <span>支持标准音频、NCM、QMC1/QMC2、KGM、KWM、XM；QQ MusicEx 可使用本机 QQ 音乐登录状态。</span>
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

            <div v-if="showQQAdvanced && isQQEncryptedPath(file.path)" class="file-credential">
              <input
                v-model="file.qqEKey"
                :type="showQQSecrets ? 'text' : 'password'"
                autocomplete="off"
                spellcheck="false"
                :disabled="isRunning"
                placeholder="此文件专用 EKey（可选；优先于自动登录/Cookie）"
              />
            </div>

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
            <select v-model="outputFormat" :disabled="isMobileRuntime">
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

          <p v-if="isMobileRuntime" class="credential-hint">移动端首版保持原格式，结果写入 App 沙箱后通过系统分享或“保存到文件”。</p>
          <button v-if="!isMobileRuntime" @click="chooseOutputDir">选择输出目录</button>
          <div class="output-path">
            {{ isMobileRuntime ? 'App 沙箱（转换完成后分享/保存）' : (outputDir || '正在加载默认输出目录…') }}
          </div>
          <button v-if="!isMobileRuntime" class="ghost" @click="openOutputDir">打开输出目录</button>
        </section>

        <section class="panel-section">
          <h2>QQ 音乐登录授权</h2>
          <label v-if="runtimePlatform === 'windows'" class="qq-auto-login">
            <input v-model="qqAutoLogin" type="checkbox" :disabled="isRunning" />
            <span>
              <strong>自动使用本机 QQ 音乐登录状态</strong>
              <small>适合普通用户；请先打开 Windows QQ 音乐客户端并登录下载歌曲的账号。</small>
            </span>
          </label>
          <div v-else-if="runtimePlatform" class="platform-notice">
            <strong>{{ runtimePlatform === 'darwin' ? 'macOS 转换已支持' : (isMobileRuntime ? '移动端预览模式' : '当前平台使用手动授权') }}</strong>
            <span v-if="isMobileRuntime">系统不允许读取 QQ 音乐 App 的登录数据；MusicEx 仅接受当前文件的 EKey。</span>
            <span v-else>不扫描 QQ 音乐进程；MusicEx 请使用下方临时 Cookie 或逐文件 EKey。</span>
          </div>
          <p v-if="runtimePlatform === 'windows'" class="credential-hint">
            默认关闭。勾选后仅在本次任务中只读 QQMusic.exe 及 QQ 音乐自身的本地登录数据；不读取其他进程，不显示、不写入磁盘，任务结束即清除。
          </p>
          <p v-else-if="!runtimePlatform" class="credential-hint">正在检测当前运行平台…</p>
          <button class="ghost" :disabled="isRunning" @click="showQQAdvanced = !showQQAdvanced">
            {{ showQQAdvanced ? '收起高级选项' : (isMobileRuntime ? '高级：逐文件 EKey' : '高级：手动 Cookie / EKey') }}
          </button>
          <div v-if="showQQAdvanced" class="credential-advanced">
            <p class="credential-hint">
              {{ isMobileRuntime ? '逐文件 EKey 只保存在当前页面内存。' : '仅在自动登录不可用时使用。Cookie 和逐文件 EKey 都只保存在当前页面内存。' }}
            </p>
            <label v-if="!isMobileRuntime">
              临时 QQ Music Cookie
              <input
                v-model="qqCookie"
                :type="showQQSecrets ? 'text' : 'password'"
                autocomplete="off"
                spellcheck="false"
                :disabled="isRunning"
                placeholder="qqmusic_key=... 或完整 Cookie"
              />
            </label>
            <div class="credential-actions">
              <button class="ghost" @click="showQQSecrets = !showQQSecrets">
                {{ showQQSecrets ? '隐藏凭据' : '显示凭据' }}
              </button>
              <button class="ghost danger" :disabled="isRunning" @click="clearQQCredentials">清除手动凭据</button>
            </div>
          </div>
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
  ClearQQLoginCache,
  ConvertFile,
  GetDefaultOutputDir,
  GetRuntimePlatform,
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
const qqCookie = ref('')
const qqAutoLogin = ref(false)
const runtimePlatform = ref('')
const showQQAdvanced = ref(false)
const showQQSecrets = ref(false)

const platforms = {
  '.mp3': '标准音频',
  '.flac': '标准音频',
  '.ogg': '标准音频',
  '.wav': '标准音频',
  '.aac': '标准音频',
  '.m4a': '标准音频',
  '.ncm': '网易云音乐',
  '.qmc0': 'QQ音乐',
  '.qmc2': 'QQ音乐',
  '.qmc3': 'QQ音乐',
  '.qmc4': 'QQ音乐',
  '.qmc6': 'QQ音乐',
  '.qmc8': 'QQ音乐',
  '.qmcflac': 'QQ音乐',
  '.qmcogg': 'QQ音乐',
  '.mflac': 'QQ音乐新版',
  '.mflac0': 'QQ音乐新版',
  '.mflach': 'QQ音乐新版',
  '.mmp4': 'QQ音乐新版',
  '.mgg': 'QQ音乐新版',
  '.mgg0': 'QQ音乐新版',
  '.mgg1': 'QQ音乐新版',
  '.mggl': 'QQ音乐新版',
  '.bkcmp3': 'QQ音乐',
  '.bkcflac': 'QQ音乐',
  '.bkcwav': 'QQ音乐',
  '.bkcogg': 'QQ音乐',
  '.bkcwma': 'QQ音乐',
  '.bkcape': 'QQ音乐',
  '.bkcm4a': 'QQ音乐',
  '.tkm': 'QQ音乐',
  '.666c6163': 'QQ音乐缓存',
  '.6d7033': 'QQ音乐缓存',
  '.6f6767': 'QQ音乐缓存',
  '.6d3461': 'QQ音乐缓存',
  '.776176': 'QQ音乐缓存',
  '.tm0': 'QQ音乐旧版',
  '.tm2': 'QQ音乐旧版',
  '.tm3': 'QQ音乐旧版',
  '.tm6': 'QQ音乐旧版',
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
const isMobileRuntime = computed(() => ['android', 'ios'].includes(runtimePlatform.value))

const filteredFiles = computed(() => {
  const text = keyword.value.trim().toLowerCase()

  return files.value.filter(file => {
    const matchesStatus = statusFilter.value === 'all' || file.status === statusFilter.value
    const haystack = `${file.path} ${file.platform} ${file.message}`.toLowerCase()
    return matchesStatus && (!text || haystack.includes(text))
  })
})

onMounted(() => {
  GetRuntimePlatform()
    .then(platform => {
      runtimePlatform.value = platform
      if (platform !== 'windows') {
        qqAutoLogin.value = false
        showQQAdvanced.value = true
      }
      if (['android', 'ios'].includes(platform)) {
        outputFormat.value = 'origin'
        concurrency.value = 1
        qqCookie.value = ''
      }
    })
    .catch(() => {
      runtimePlatform.value = 'unknown'
      showQQAdvanced.value = true
    })

  GetDefaultOutputDir()
    .then(dir => {
      if (!outputDir.value) {
        outputDir.value = dir
      }
    })
    .catch(err => addLog(`默认输出目录不可用：${cleanError(err)}`))

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
  clearQQCredentials()
  ClearQQLoginCache().catch(() => {})
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
      const protectedFormat = isQMC2Path(path)
      const supported = platform !== '未知格式'

      return {
        id: crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random()}`,
        path,
        platform,
        status: supported ? 'pending' : 'unsupported',
        progress: 0,
        message: protectedFormat ? 'QMC2/MusicEx：内嵌密钥可直接处理；新格式可自动使用本机 QQ 音乐登录状态。' : '',
        qqEKey: ''
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

function isQMC2Path(path) {
  return [
    '.mgg', '.mgg0', '.mgg1', '.mggl', '.mflac', '.mflac0', '.mflach', '.mmp4',
    '.bkcmp3', '.bkcflac', '.bkcwav', '.bkcogg', '.bkcwma', '.bkcape', '.bkcm4a', '.tkm'
  ].includes(getFileExt(path))
}

function isQQEncryptedPath(path) {
  return [
    '.qmc0', '.qmc2', '.qmc3', '.qmc4', '.qmc6', '.qmc8', '.qmcflac', '.qmcogg',
    '.mgg', '.mgg0', '.mgg1', '.mggl', '.mflac', '.mflac0', '.mflach', '.mmp4',
    '.bkcmp3', '.bkcflac', '.bkcwav', '.bkcogg', '.bkcwma', '.bkcape', '.bkcm4a', '.tkm',
    '.666c6163', '.6d7033', '.6f6767', '.6d3461', '.776176'
  ].includes(getFileExt(path))
}

function clearQQCredentials() {
  qqCookie.value = ''
  showQQSecrets.value = false
  files.value.forEach(file => {
    file.qqEKey = ''
  })
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

  const queue = files.value
    .filter(file => ['pending', 'failed'].includes(file.status))
    .map(file => ({ file, qqEKey: file.qqEKey || '' }))
  const workerCount = Math.max(1, Math.min(Number(concurrency.value) || 1, 8, queue.length || 1))
  const qqCookieSnapshot = qqCookie.value
  const qqAutoLoginSnapshot = runtimePlatform.value === 'windows' && qqAutoLogin.value
  let cursor = 0

  if (qqAutoLoginSnapshot) {
    await ClearQQLoginCache().catch(() => {})
  }
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

      const queued = queue[cursor++]
      const file = queued?.file
      if (!file || file.status === 'unsupported') continue

      file.status = 'processing'
      file.progress = Math.max(file.progress, 5)
      file.message = '处理中'

      try {
        const message = await ConvertFile(
          file.path,
          outputDir.value,
          outputFormat.value,
          bitrate.value,
          queued.qqEKey,
          qqCookieSnapshot,
          qqAutoLoginSnapshot
        )
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
    if (qqAutoLoginSnapshot) {
      await ClearQQLoginCache().catch(() => {})
    }
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
