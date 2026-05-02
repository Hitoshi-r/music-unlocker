package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	stdRuntime "runtime" // 👈 标准库重命名
	"strings"

	"music-unlocker/decoder"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx       context.Context
	cancelMap map[string]context.CancelFunc
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		cancelMap: make(map[string]context.CancelFunc),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SelectFiles opens file selection dialog
func (a *App) SelectFiles() ([]string, error) {
	files, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择需要解密的音频文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "加密音频文件",
				Pattern:     "*.ncm;*.qmc*;*.mflac;*.mgg;*.kgm;*.vpr;*.kwm;*.xm",
			},
			{
				DisplayName: "所有文件",
				Pattern:     "*.*",
			},
		},
	})

	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return []string{}, nil
	}

	return files, nil
}

//SelectFolderFiles opens folder selection dialog and scans for supported files

func (a *App) SelectFolderFiles() ([]string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择文件夹",
	})
	if err != nil || dir == "" {
		return nil, err
	}

	var files []string

	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		switch ext {
		case ".ncm", ".qmc0", ".qmc3", ".qmcflac", ".mflac", ".mgg", ".kgm", ".vpr", ".kwm", ".xm":
			files = append(files, path)
		}

		return nil
	})

	fmt.Println("扫描到文件数量：", len(files)) // 👈 调试用

	return files, nil
}

// SelectOutputDir opens output directory selection dialog
func (a *App) SelectOutputDir() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择输出目录",
	})

	if err != nil || dir == "" {
		return "", err
	}

	fmt.Println("选择输出目录:", dir)

	return dir, nil
}

func (a *App) OpenFolder(path string) error {
	if path == "" {
		return nil
	}

	var cmd *exec.Cmd

	switch stdRuntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	}

	return cmd.Start()
}
func progressEmit(a *App, path string) decoder.ProgressCallback {
	return func(percent int) {
		runtime.EventsEmit(a.ctx, "convert-progress", map[string]interface{}{
			"path":    path,
			"percent": percent,
		})
	}
}

// ConvertFile converts a selected audio file

func (a *App) ConvertFile(path string, outputDir string, outputFormat string, bitrate string) (string, error) {
	fmt.Println("ConvertFile 被调用：", path, outputDir, outputFormat, bitrate)

	// ✅ 放这里
	if strings.TrimSpace(outputDir) == "" {
		outputDir = filepath.Dir(path)
		fmt.Println("未选择输出目录，使用原文件目录:", outputDir)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %v", err)
	}

	// 🔥【核心修复】默认输出目录
	if strings.TrimSpace(outputDir) == "" {
		outputDir = filepath.Dir(path)
		fmt.Println("未选择输出目录，使用默认目录:", outputDir)
	}

	// 🔥【核心修复】确保目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("创建输出目录失败")
	}

	ext := strings.TrimSpace(strings.ToLower(filepath.Ext(path)))
	fmt.Println("扩展名：", ext)

	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelMap[path] = cancel
	defer delete(a.cancelMap, path)

	var result *decoder.DecodeResult
	var err error

	switch ext {

	case ".ncm":
		d := &decoder.NCMDecoder{}
		result, err = d.Decode(path, outputDir, outputFormat, bitrate, ctx, progressEmit(a, path))

	case ".qmc0", ".qmc3", ".qmcflac", ".qmcogg", ".mflac", ".mgg":
		d := &decoder.QMCDecoder{}
		result, err = d.Decode(path, outputDir, outputFormat, bitrate, ctx, progressEmit(a, path))

	case ".kgm", ".vpr":
		d := &decoder.KGMDecoder{}
		result, err = d.Decode(path, outputDir, outputFormat, bitrate, ctx, progressEmit(a, path))

	case ".kwm":
		d := &decoder.KWMDecoder{}
		result, err = d.Decode(path, outputDir, outputFormat, bitrate, ctx, progressEmit(a, path))

	case ".xm":
		d := &decoder.XMDecoder{}
		result, err = d.Decode(path, outputDir, outputFormat, bitrate, ctx, progressEmit(a, path))

	default:
		return "", fmt.Errorf("❌ 当前格式暂不支持")
	}

	if err != nil {
		fmt.Println("Decode 错误：", err)
		return "", fmt.Errorf(formatError(err))
	}

	return "✔ " + result.Message + "，输出路径：" + result.OutputPath, nil
}

// 取消接口
func (a *App) CancelFile(path string) {
	if cancel, ok := a.cancelMap[path]; ok {
		cancel()
	}
}
func detectPlatform(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".ncm":
		return "网易云音乐"

	case ".qmc0", ".qmc3", ".qmcflac", ".qmcogg", ".mflac", ".mgg":
		return "QQ音乐"

	case ".kgm", ".vpr":
		return "酷狗音乐"

	case ".kwm":
		return "酷我音乐"

	case ".xm":
		return "虾米音乐"

	default:
		return "未知格式"
	}
}
func formatError(err error) string {
	msg := err.Error()

	if strings.Contains(msg, "ffmpeg") {
		return "转换失败（格式不支持或文件损坏）"
	}

	if strings.Contains(msg, "暂不支持") {
		return msg
	}

	if strings.Contains(msg, "no such file") {
		return "文件不存在"
	}

	if strings.Contains(msg, "permission") {
		return "权限不足"
	}

	return "转换失败"
}
