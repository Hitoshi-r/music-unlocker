# 🎵 Music Unlocker

一个基于 **Go + Wails** 构建的音乐解锁工具，支持将常见加密音频文件转换为可播放格式。

---

## ✨ 功能特性

* 🔓 支持 NCM、QMC1/QMC2/MusicEx、KGM、KWM、XM 等格式
* ⚡ 快速转换，性能优秀
* 🖥️ 简洁直观的桌面 GUI（基于 Wails）
* 📂 支持批量处理与默认输出目录
* 🎵 Windows 下可在用户主动授权后使用本机 QQ 音乐登录状态，无需手动获取 Cookie
* 🧩 模块化结构，易于扩展新格式

---

## 平台支持

| 平台 | 状态 | 说明 |
| --- | --- | --- |
| Windows 10/11 x64 | 已支持 | 可在用户主动授权后临时使用本机 QQ 音乐登录状态 |
| macOS 11+ Intel / Apple Silicon | 已支持构建 | 一份 Universal App 同时支持两种架构；MusicEx 使用手动 EKey/Cookie |
| Android | 规划中的移动预览版 | 系统沙箱不允许读取 QQ 音乐 App 的登录数据 |
| iPhone / iPad | 规划中的移动预览版 | 需要在 Mac + Xcode 上完成构建、签名和真机测试 |

界面已经支持窄屏响应式布局。移动端将使用 Wails v3 复用现有 Vue 界面和 Go 解码核心；当前 Wails v2 桌面项目不能直接生成 Android/iOS 安装包。具体边界和实施顺序见 [平台支持与移动端路线](docs/platform-support.md)。

---

## QQ 音乐 MusicEx

处理新版 MusicEx 文件时，先打开 Windows QQ 音乐客户端并登录下载该歌曲的账号，然后在应用中勾选“自动使用本机 QQ 音乐登录状态”。该选项默认关闭，只读取 `QQMusic.exe` 和 QQ 音乐自身的本地登录数据，不读取浏览器或其他进程；登录令牌不会显示、写入配置或日志，并在批量任务结束后从程序缓存中清除。

如果自动登录不可用，可以展开高级选项，为单个文件填写 EKey，或临时填写 QQ Music Cookie。`result=104003` 表示当前登录账号没有获得该文件的解密授权，常见于下载账号与当前账号不同，或会员/购买权限已失效。

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

* Go >= 1.23
* Node.js 22
* Wails CLI 2.12.0

安装 Wails：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

#### 3️⃣ 运行项目

```bash
wails dev
```

---

## 📦 构建发布版本

Windows：

```bash
wails build -platform windows/amd64
```

macOS Universal（必须在 macOS 上运行）：

```bash
MACOSX_DEPLOYMENT_TARGET=11.0 \
CGO_CFLAGS=-mmacosx-version-min=11.0 \
CGO_LDFLAGS=-mmacosx-version-min=11.0 \
wails build -platform darwin/universal
```

macOS 从 Finder 启动时会自动查找应用内置 FFmpeg、`PATH`、Apple Silicon Homebrew 的 `/opt/homebrew/bin/ffmpeg` 和 Intel Homebrew 的 `/usr/local/bin/ffmpeg`。如果只做“保持原格式”解密，不需要 FFmpeg；二次转码可先执行：

```bash
brew install ffmpeg
```

构建完成后，产物位于：

```bash
build/bin/
```

仓库中的 `Build desktop apps` GitHub Actions 会同时生成 Windows x64 和 macOS Universal 测试产物。macOS 测试产物只是临时签名，面向普通用户发布前仍需 Developer ID 签名与 Apple 公证。

---

## 🛠 项目结构

```bash
music-unlocker/
├── .github/workflows/  # Windows/macOS 自动构建
├── build/              # Wails 图标、清单与平台打包输入
├── converter/          # 格式检测、音频收尾与 FFmpeg 转码
├── decoder/            # 解码核心、QQ MusicEx 授权
├── docs/               # 平台和开发文档
├── frontend/           # Vue 3 界面与 Wails JS 绑定
├── app.go              # 前后端绑定与任务调度
├── main.go             # Wails 程序入口
├── wails.json          # Wails 项目配置
└── THIRD_PARTY_NOTICES.md
```

详细说明见 [开发与目录约定](docs/development.md)；平台能力和移动端边界见 [平台支持与移动端路线](docs/platform-support.md)。生成目录 `build/bin/`、`frontend/dist/`、`frontend/node_modules/` 和 `.gocache/` 不提交仓库。

---

## 🔧 技术栈

* **Go** — 后端逻辑
* **Wails** — 桌面应用框架
* **Vue 3** — 前端界面

---

## 📌 TODO

* [x] 拖拽上传、批量转换与进度显示
* [x] Windows/macOS 桌面构建
* [ ] Android Wails v3 预览版
* [ ] iPhone/iPad Wails v3 预览版
* [ ] macOS Developer ID 签名与公证
* [ ] 多语言支持
* [ ] 经过授权的跨平台 QQ 登录方案

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
