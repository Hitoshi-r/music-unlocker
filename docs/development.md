# 开发与目录约定

## 本地环境

- Go 1.23 或更高版本
- Node.js 22
- Wails CLI 2.12.0

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
wails dev
```

## 常用检查

```bash
go test ./...
go vet ./...
cd frontend && npm run build
```

完整桌面构建使用根目录的 `wails.json`：

```bash
wails build -platform windows/amd64
```

macOS Universal 构建和最低系统版本设置见根目录 README；自动化构建位于 `.github/workflows/build-desktop.yml`。

## 目录职责

```text
music-unlocker/
├─ .github/workflows/     # Windows/macOS 自动构建
├─ build/                 # Wails 图标、清单和平台打包输入
│  └─ bin/                # 生成的应用，不提交
├─ converter/             # 音频格式检测、收尾和 FFmpeg 转码
├─ decoder/               # 各音乐平台解码及 QQ MusicEx 授权
├─ docs/                  # 平台和开发文档
├─ frontend/
│  ├─ src/                # Vue 界面
│  ├─ wailsjs/            # Wails 生成并由前端导入的绑定
│  ├─ dist/               # 前端构建结果，不提交
│  └─ node_modules/       # 前端依赖，不提交
├─ app.go                 # 前后端绑定与任务调度
├─ main.go                # Wails 桌面入口
└─ wails.json             # Wails 项目配置
```

以下位置有固定含义，不要随意移动：

- `main.go`、`app.go` 和 `app_test.go` 必须同属根包 `main`；
- `decoder/qq_local_windows.go` 与 `decoder/qq_local_other.go` 是同一功能的平台实现；
- `build/windows/`、`build/darwin/` 和 `build/appicon.png` 是打包输入，不是缓存；
- `frontend/wailsjs/` 虽由 Wails 生成，但当前项目会跟踪它，后端绑定变化后应重新生成；
- `THIRD_PARTY_NOTICES.md` 是第三方算法与字体等许可说明，应留在仓库根目录。

## 私有样本

真实加密音频和登录凭据不要提交到仓库。测试可通过环境变量指向外部样本目录：

```powershell
$env:QMC_SAMPLE_DIR = 'E:\music-unlocker-review\样本文件'
go test ./decoder -count=1
```

本地的 `样本文件/`、`.env*`、构建产物和缓存均已加入 `.gitignore`。
