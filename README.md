# README

## About

This is the official Wails Vue template.

You can configure the project by editing `wails.json`. More information about the project settings can be found
here: https://wails.io/docs/reference/project-config

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.


# 🎵 Music Unlocker

一个基于 **Go + Wails** 构建的音乐解锁工具，支持将常见加密音频文件转换为可播放格式。

---

## ✨ 功能特性

* 🔓 支持多种加密音乐格式解锁
* ⚡ 快速转换，性能优秀
* 🖥️ 简洁直观的桌面 GUI（基于 Wails）
* 📂 支持批量处理（可扩展）
* 🧩 模块化结构，易于扩展新格式

---

## 🖼️ 预览

> （你可以后面自己加截图）

---

## 🚀 快速开始

### 方式一：下载可执行文件（推荐）

1. 前往 [Releases](../../releases)
2. 下载最新版本
3. 双击运行即可

---

### 方式二：本地运行

#### 1️⃣ 克隆项目

```bash
git clone https://github.com/Hitoshi-r/music-unlocker.git
cd music-unlocker
```

#### 2️⃣ 安装依赖

确保你已安装：

* Go >= 1.20
* Node.js
* Wails CLI

安装 Wails：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

#### 3️⃣ 运行项目

```bash
wails dev
```

---

## 📦 构建发布版本

```bash
wails build
```

构建完成后：

```bash
build/bin/
```

目录下会生成可执行文件（.exe）

---

## 🛠 项目结构

```bash
music-unlocker/
├── app.go              # 程序入口
├── decoder/            # 解码核心逻辑
├── frontend/           # 前端界面
├── build/              # 构建输出
└── README.md
```

---

## 🔧 技术栈

* **Go** — 后端逻辑
* **Wails** — 桌面应用框架
* **Vue / React** — 前端界面（根据你项目实际情况）

---

## 📌 TODO（可以自己扩展）

* [ ] 支持更多音乐格式
* [ ] 拖拽上传文件
* [ ] 批量转换 UI
* [ ] 转换进度显示
* [ ] 错误提示优化
* [ ] 多语言支持

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

如果你有好的想法或功能建议，也欢迎讨论 👍

---

## 📄 License

MIT License

---

## ⭐ Star History

如果这个项目对你有帮助，欢迎点个 ⭐ 支持一下！
