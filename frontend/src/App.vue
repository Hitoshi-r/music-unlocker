<template>
  <div class="app">

    <header class="header">
      🎵 Music Unlocker
    </header>

    <div class="container">

      <div class="file-list">
        <div 
          v-for="file in files" 
          :key="file.path" 
          class="file-item"
        >
          <div class="name">{{ getFileName(file.path) }}</div>
          <div class="status">{{ file.status }}</div>

          <div class="progress">
            <div 
              class="bar" 
              :style="{ width: file.progress + '%' }"
            ></div>
          </div>
        </div>
      </div>

      <div class="panel">

        <button @click="selectFiles">选择文件</button>
        <button @click="selectFolder">选择文件夹</button>
        <button @click="chooseOutputDir">
        选择输出目录
        </button>

        <div class="output-path">
          {{ outputDir || '未选择目录' }}
        </div>

        <button @click="openOutputDir">
          打开输出目录
        </button>

        <select v-model="outputFormat">
          <option value="origin">原格式</option>
          <option value="mp3">MP3</option>
          <option value="flac">FLAC</option>
          <option value="wav">WAV</option>
        </select>

        <select v-model="bitrate">
          <option value="128k">128k</option>
          <option value="192k">192k</option>
          <option value="320k">320k</option>
        </select>

        <button class="start" @click="startConvert">
          开始转换
        </button>

      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
//import { SelectFiles, ConvertFile, SelectOutputDir } from '../wailsjs/go/main/App'
import { 
  SelectFiles, 
  ConvertFile, 
  SelectOutputDir,
  OpenFolder,
  SelectFolderFiles
} from '../wailsjs/go/main/App'
const files = ref([])
const outputDir = ref('')

const outputFormat = ref('origin')

const isPaused = ref(false)
const isCancelled = ref(false)
const bitrate = ref('128k')
//ConvertFile(file.path, outputDir.value, outputFormat.value, bitrate.value)
function pauseConvert() {
  isPaused.value = true
}

function resumeConvert() {
  isPaused.value = false
}

function cancelConvert() {
  isCancelled.value = true
  isPaused.value = false

  files.value.forEach(file => {
    if (file.status === '待处理' || file.status === '处理中...') {
      file.status = '已取消'
    }
  })
}

async function openOutputDir() {
  if (!outputDir.value) {
    alert("请先选择输出目录")
    return
  }

  try {
    await OpenFolder(outputDir.value)
  } catch (err) {
    console.error(err)
  }
}


async function selectFiles() {
  try {
    const result = await SelectFiles()

    if (result && result.length > 0) {
      addFiles(result)
    }
  } catch (err) {
    console.error(err)
  }
}

async function selectFolder() {
  try {
    const result = await SelectFolderFiles()
    if (result && result.length > 0) {
      addFiles(result)
    }
  } catch (err) {
    console.error(err)
  }
}

function addFiles(paths) {
  const newFiles = paths
    .filter(path => !files.value.some(file => file.path === path))
    .map(path => ({
      path,
      status: getPlatform(path) === '未知格式' ? '❌ 不支持' : '待处理',
      progress: 0
    }))

  files.value.push(...newFiles)
}

function handleDrop(event) {
  const droppedFiles = Array.from(event.dataTransfer.files)

  const paths = droppedFiles
    .map(file => file.path)
    .filter(Boolean)

  if (paths.length > 0) {
    addFiles(paths)
  }
}

async function chooseOutputDir() {
  try {
    const dir = await SelectOutputDir()

    if (dir) {
      outputDir.value = dir
    }
  } catch (err) {
    console.error(err)
  }
}

function getFileName(path) {
  return path.split('\\').pop().split('/').pop()
}

function removeFile(index) {
  files.value.splice(index, 1)
}

function clearFiles() {
  files.value = []
}

function getFileExt(path) {
  const name = getFileName(path)
  const index = name.lastIndexOf('.')

  if (index === -1) return ''
  return name.slice(index).toLowerCase()
}

function getPlatform(path) {
  const ext = getFileExt(path)

  const map = {
    '.ncm': '网易云音乐',

    '.qmc0': 'QQ音乐',
    '.qmc3': 'QQ音乐',
    '.qmcflac': 'QQ音乐',
    '.qmcogg': 'QQ音乐',
    '.mflac': 'QQ音乐',
    '.mgg': 'QQ音乐',

    '.kgm': '酷狗音乐',
    '.vpr': '酷狗音乐',

    '.kwm': '酷我音乐',

    '.xm': '虾米音乐'
  }

  return map[ext] || '未知格式'
}

// 🔥 状态样式升级
function getStatusClass(status) {
  if (status.startsWith('✔')) return 'success'
  if (status.startsWith('❌')) return 'failed'

  const map = {
    '待处理': 'pending',
    '处理中...': 'processing',
    '已取消': 'cancelled'
  }

  return map[status] || 'pending'
}

// 🔥 核心改动：接收后端返回信息
async function startConvert() {
  const maxConcurrent = 3

  isPaused.value = false
  isCancelled.value = false

  let index = 0

  async function waitIfPaused() {
    while (isPaused.value && !isCancelled.value) {
      await new Promise(resolve => setTimeout(resolve, 300))
    }
  }

  async function worker() {
    while (index < files.value.length) {
      if (isCancelled.value) break

      await waitIfPaused()

      if (isCancelled.value) break

      const current = index++
      const file = files.value[current]

      if (!file || file.status.startsWith('❌') || file.status.startsWith('✔')) {
        continue
      }

      try {
        file.status = '处理中...'
        file.progress = 10

        const res = await ConvertFile(
          file.path,
          outputDir.value,
          outputFormat.value,
          bitrate.value
        )

        if (!isCancelled.value) {
          file.status = "✔ " + res
          file.progress = 100
        }

      } catch (err) {
        console.error(err)

        file.status = "❌ " + (err.message || '转换失败')
        file.progress = 0
      }
    }
  }

  const workers = []

  for (let i = 0; i < maxConcurrent; i++) {
    workers.push(worker())
  }

  await Promise.all(workers)

  if (isCancelled.value) {
    files.value.forEach(file => {
      if (file.status === '待处理' || file.status === '处理中...') {
        file.status = '已取消'
      }
    })
  }
}
</script>



<style src="./styles/app.css"></style>