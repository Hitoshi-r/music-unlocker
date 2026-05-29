<template>
  <div class="app">
    <header class="header">
      <div>
        <strong>Music Unlocker</strong>
        <span>多格式音频识别与转换工具</span>
      </div>
    </header>

    <main class="container">
      <section
        class="file-list"
        @dragover.prevent
        @drop.prevent="handleDrop"
      >
        <div v-if="files.length === 0" class="empty">
          拖入文件，或从右侧选择文件/文件夹
        </div>

        <article
          v-for="(file, index) in files"
          :key="file.path"
          class="file-item"
        >
          <div class="file-main">
            <div>
              <div class="name">{{ getFileName(file.path) }}</div>
              <div class="meta">{{ getPlatform(file.path) }} · {{ file.path }}</div>
            </div>
            <button class="ghost" @click="removeFile(index)">移除</button>
          </div>

          <div class="status" :class="getStatusClass(file.status)">
            {{ file.status }}
          </div>

          <div class="progress">
            <div class="bar" :style="{ width: file.progress + '%' }"></div>
          </div>
        </article>
      </section>

      <aside class="panel">
        <button @click="selectFiles">选择文件</button>
        <button @click="selectFolder">选择文件夹</button>
        <button @click="chooseOutputDir">选择输出目录</button>

        <div class="output-path">
          {{ outputDir || '未选择输出目录，将输出到原文件目录' }}
        </div>

        <button @click="openOutputDir">打开输出目录</button>

        <label>
          输出格式
          <select v-model="outputFormat">
            <option value="origin">原格式</option>
            <option value="mp3">MP3</option>
            <option value="flac">FLAC</option>
            <option value="wav">WAV</option>
            <option value="ogg">OGG</option>
          </select>
        </label>

        <label>
          MP3 码率
          <select v-model="bitrate">
            <option value="128k">128k</option>
            <option value="192k">192k</option>
            <option value="320k">320k</option>
          </select>
        </label>

        <button class="start" :disabled="isRunning || files.length === 0" @click="startConvert">
          {{ isRunning ? '处理中...' : '开始转换' }}
        </button>
        <button class="ghost" :disabled="isRunning" @click="clearFiles">清空列表</button>
      </aside>
    </main>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import {
  ConvertFile,
  OpenFolder,
  SelectFiles,
  SelectFolderFiles,
  SelectOutputDir
} from '../wailsjs/go/main/App'

const files = ref([])
const outputDir = ref('')
const outputFormat = ref('origin')
const bitrate = ref('128k')
const isRunning = ref(false)

const platforms = {
  '.mp3': '标准音频',
  '.flac': '标准音频',
  '.ogg': '标准音频',
  '.wav': '标准音频',
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

async function selectFiles() {
  try {
    addFiles(await SelectFiles())
  } catch (err) {
    console.error(err)
  }
}

async function selectFolder() {
  try {
    addFiles(await SelectFolderFiles())
  } catch (err) {
    console.error(err)
  }
}

function addFiles(paths = []) {
  const pending = paths
    .filter(Boolean)
    .filter(path => !files.value.some(file => file.path === path))
    .map(path => {
      const supported = getPlatform(path) !== '未知格式'
      return {
        path,
        status: supported ? '待处理' : '不支持',
        progress: 0
      }
    })

  files.value.push(...pending)
}

function handleDrop(event) {
  const paths = Array.from(event.dataTransfer.files)
    .map(file => file.path)
    .filter(Boolean)
  addFiles(paths)
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

function removeFile(index) {
  files.value.splice(index, 1)
}

function clearFiles() {
  files.value = []
}

function getFileExt(path) {
  const name = getFileName(path)
  const index = name.lastIndexOf('.')
  return index === -1 ? '' : name.slice(index).toLowerCase()
}

function getPlatform(path) {
  return platforms[getFileExt(path)] || '未知格式'
}

function getStatusClass(status) {
  if (status.startsWith('完成')) return 'success'
  if (status.startsWith('失败') || status === '不支持') return 'failed'
  if (status === '处理中') return 'processing'
  return 'pending'
}

async function startConvert() {
  isRunning.value = true

  try {
    for (const file of files.value) {
      if (file.status === '不支持' || file.status.startsWith('完成')) {
        continue
      }

      file.status = '处理中'
      file.progress = 15

      try {
        const message = await ConvertFile(
          file.path,
          outputDir.value,
          outputFormat.value,
          bitrate.value
        )
        file.status = '完成：' + message
        file.progress = 100
      } catch (err) {
        file.status = '失败：' + (err?.message || err || '转换失败')
        file.progress = 0
      }
    }
  } finally {
    isRunning.value = false
  }
}
</script>

<style src="./styles/app.css"></style>
