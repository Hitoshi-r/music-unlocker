package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	stdRuntime "runtime"
	"strings"

	"music-unlocker/decoder"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx       context.Context
	cancelMap map[string]context.CancelFunc
}

func NewApp() *App {
	return &App{
		cancelMap: make(map[string]context.CancelFunc),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) SelectFiles() ([]string, error) {
	files, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择需要处理的音频文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "支持的音频文件",
				Pattern:     decoder.FileDialogPattern(),
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

func (a *App) SelectFolderFiles() ([]string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择文件夹",
	})
	if err != nil || dir == "" {
		return nil, err
	}

	var files []string
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if decoder.SupportsPath(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	fmt.Println("扫描到文件数量:", len(files))
	return files, nil
}

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
	default:
		return nil
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

func (a *App) ConvertFile(path string, outputDir string, outputFormat string, bitrate string) (string, error) {
	fmt.Println("ConvertFile:", path, outputDir, outputFormat, bitrate)

	if strings.TrimSpace(outputDir) == "" {
		outputDir = filepath.Dir(path)
		fmt.Println("未选择输出目录，使用原文件目录:", outputDir)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %v", err)
	}

	item := decoder.FindByPath(path)
	if item == nil {
		return "", fmt.Errorf("当前格式暂不支持")
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelMap[path] = cancel
	defer delete(a.cancelMap, path)

	result, err := item.Decode(path, outputDir, outputFormat, bitrate, ctx, progressEmit(a, path))
	if err != nil {
		fmt.Println("Decode error:", err)
		return "", fmt.Errorf(formatError(err))
	}
	if result == nil {
		return "", fmt.Errorf("转换失败")
	}
	if !result.Success {
		return "", fmt.Errorf(result.Message)
	}

	return result.Message + "，输出路径：" + result.OutputPath, nil
}

func (a *App) CancelFile(path string) {
	if cancel, ok := a.cancelMap[path]; ok {
		cancel()
	}
}

func detectPlatform(path string) string {
	return decoder.DetectPlatform(path)
}

func formatError(err error) string {
	msg := err.Error()

	if strings.Contains(msg, "ffmpeg") {
		return "转换失败：格式不支持或文件损坏"
	}
	if strings.Contains(msg, "暂不支持") || strings.Contains(msg, "尚未实现") {
		return msg
	}
	if strings.Contains(msg, "no such file") {
		return "文件不存在"
	}
	if strings.Contains(msg, "permission") {
		return "权限不足"
	}

	return msg
}
