# 平台支持与移动端路线

## 当前支持范围

| 平台 | 状态 | QQ MusicEx 授权 | 输出方式 |
| --- | --- | --- | --- |
| Windows 10/11 x64 | 已支持 | 用户主动勾选后，可临时读取本机 QQ 音乐登录状态；也可手动 EKey/Cookie | 默认目录或自选目录 |
| macOS 11+ Intel / Apple Silicon | 已加入构建与界面适配 | 手动 EKey/Cookie；不扫描 QQ 音乐进程 | `~/Music/Music Unlocker` 或自选目录 |
| Android | 移动预览版规划中 | 不能读取其他 App 的 Cookie、数据库或进程 | App 沙箱后通过系统分享/保存 |
| iPhone / iPad | 移动预览版规划中 | 不能读取其他 App 的 Cookie、数据库或进程 | App 沙箱后通过系统分享/保存 |

macOS 使用 Wails v2 的 `darwin/universal` 构建，一份 `.app` 同时包含 `x86_64` 与 `arm64`。GitHub Actions 产物目前是临时签名测试包；面向普通用户分发前，仍需 Apple Developer ID 签名和公证。

## 为什么手机不能照搬 Windows 自动登录

Android 和 iOS 都禁止普通应用读取其他 App 的专属数据和进程内存。因此移动端不会尝试提取 QQ 音乐 Cookie、密钥数据库或进程凭据，也不会从桌面端同步完整 Cookie。

手机端若要免手动 Cookie，安全路线只有两种：

1. 使用 QQ 官方允许的登录/授权能力，在应用内完成登录；
2. 桌面伴侣仅为当前歌曲传递短期单曲 EKey，不传递账号 Cookie。

## 推荐实施路线

桌面稳定版继续使用 Wails v2。Android/iOS 单独建立 Wails v3 移动预览版，共享现有 Go 解码核心和 Vue 界面，不立即替换稳定桌面入口。

移动端第一阶段范围：

1. 使用系统文件选择器导入单个或多个文件；
2. 解密后保留原音频格式；
3. 写入应用沙箱，再调用系统分享或“保存到文件”；
4. MusicEx 接受逐文件 EKey；
5. 不调用外部 FFmpeg。

桌面版通过外部 FFmpeg 做 MP3/FLAC/WAV/OGG 二次转码，但 Android/iOS 不能直接执行桌面 FFmpeg。移动端转码需要以后接入移动版 libav/FFmpeg，并分别处理许可证和各 CPU 架构。

Wails v3 当前仍处于 Alpha，Android/iOS 支持也标记为实验性，因此先放在移动预览入口，不替换稳定桌面版。它需要 Go 1.25+；Android 可以在 Windows 上继续开发和构建，iOS 最终构建、签名与真机测试必须使用安装了 Xcode 的 Mac。

参考：[Wails v3 移动端指南](https://v3.wails.io/guides/mobile/)、[Wails v2 到 v3 迁移指南](https://v3.wails.io/migration/v2-to-v3/)、[Android 应用专属存储](https://developer.android.com/training/data-storage/app-specific)、[Apple App Sandbox](https://developer.apple.com/documentation/security/protecting-user-data-with-app-sandbox)。
