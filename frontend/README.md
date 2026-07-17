# Music Unlocker 前端

该目录是 Music Unlocker 的 Vue 3 + Vite 界面，由 Wails 调用。

- `src/App.vue`：主界面和任务队列交互；
- `src/styles/app.css`：应用布局、桌面与移动端响应式样式；
- `src/style.css`：全局字体和页面基础样式；
- `wailsjs/`：Wails 生成的 Go/JavaScript 绑定，请勿手工编辑；
- `dist/`：`npm run build` 生成的静态资源，不提交仓库。

```bash
npm ci
npm run build
```

完整开发、测试和目录约定见 [`../docs/development.md`](../docs/development.md)。
