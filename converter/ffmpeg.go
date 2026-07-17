package converter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrTaskCanceled = errors.New("任务已取消")

var ffmpegCommandContext = exec.CommandContext
var ffmpegBinaryResolver = resolveFFmpegBinary

const ffmpegPathEnv = "MUSIC_UNLOCKER_FFMPEG"

func resolveFFmpegBinary() (string, error) {
	if override := strings.TrimSpace(os.Getenv(ffmpegPathEnv)); override != "" {
		if isUsableFFmpegFile(override) {
			return override, nil
		}
		return "", fmt.Errorf("%s 指向的 FFmpeg 可执行文件不存在", ffmpegPathEnv)
	}

	candidates := make([]string, 0, 6)
	if executable, err := os.Executable(); err == nil {
		executableDir := filepath.Dir(executable)
		switch runtime.GOOS {
		case "darwin":
			// Finder launches GUI applications with a minimal PATH. A bundled
			// FFmpeg belongs in Music Unlocker.app/Contents/Resources/ffmpeg.
			candidates = append(candidates, filepath.Clean(filepath.Join(executableDir, "..", "Resources", "ffmpeg")))
		case "windows":
			candidates = append(candidates, filepath.Join(executableDir, "ffmpeg.exe"))
		default:
			candidates = append(candidates, filepath.Join(executableDir, "ffmpeg"))
		}
	}

	if resolved, err := exec.LookPath("ffmpeg"); err == nil {
		candidates = append(candidates, resolved)
	}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates,
			"/opt/homebrew/bin/ffmpeg",
			"/usr/local/bin/ffmpeg",
			"/opt/local/bin/ffmpeg",
		)
	}

	for _, candidate := range candidates {
		if isUsableFFmpegFile(candidate) {
			return candidate, nil
		}
	}

	switch runtime.GOOS {
	case "darwin":
		return "", errors.New("找不到 FFmpeg；请先运行 brew install ffmpeg，或通过 MUSIC_UNLOCKER_FFMPEG 指定可执行文件")
	case "windows":
		return "", errors.New("找不到 FFmpeg；请安装 FFmpeg 并加入 PATH，或将 ffmpeg.exe 放到程序目录")
	default:
		return "", errors.New("找不到 FFmpeg；请安装 FFmpeg 并加入 PATH，或通过 MUSIC_UNLOCKER_FFMPEG 指定可执行文件")
	}
}

func isUsableFFmpegFile(path string) bool {
	info, err := os.Stat(filepath.Clean(path))
	return err == nil && !info.IsDir()
}

func ConvertAudio(ctx context.Context, inputPath string, outputPath string, outputFormat string, bitrate string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return ErrTaskCanceled
	}

	args := []string{
		"-y",
		"-i", inputPath,
	}

	switch outputFormat {
	case "mp3":
		if bitrate == "" {
			bitrate = "320k"
		}
		args = append(args, "-b:a", bitrate)
	case "flac":
		args = append(args, "-c:a", "flac")
	case "wav":
		args = append(args, "-c:a", "pcm_s16le")
	case "ogg":
		args = append(args, "-c:a", "libvorbis")
	}

	args = append(args, outputPath)

	ffmpegBinary, err := ffmpegBinaryResolver()
	if err != nil {
		return err
	}

	cmd := ffmpegCommandContext(ctx, ffmpegBinary, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return ErrTaskCanceled
	}

	fmt.Println("FFmpeg 输出:")
	fmt.Println(string(output))

	if err != nil {
		return fmt.Errorf("ffmpeg error: %v", err)
	}

	return nil
}
