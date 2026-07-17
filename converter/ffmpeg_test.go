package converter

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFinishAudioCanceledBeforeStartCleansRawFile(t *testing.T) {
	directory := t.TempDir()
	rawPath := filepath.Join(directory, "sample_raw.bin")
	if err := os.WriteFile(rawPath, []byte("fLaCtest audio"), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := FinishAudio(ctx, rawPath, directory, "sample", "origin", "")
	if !errors.Is(err, ErrTaskCanceled) {
		t.Fatalf("FinishAudio() error = %v, want %v", err, ErrTaskCanceled)
	}
	if err.Error() != "任务已取消" {
		t.Fatalf("cancellation error = %q", err.Error())
	}
	if _, statErr := os.Stat(rawPath); !os.IsNotExist(statErr) {
		t.Fatalf("raw file still exists after cancellation: %v", statErr)
	}
}

func TestFinishAudioCancellationStopsFFmpegAndCleansFiles(t *testing.T) {
	oldCommandContext := ffmpegCommandContext
	oldBinaryResolver := ffmpegBinaryResolver
	ffmpegBinaryResolver = func() (string, error) {
		return "ffmpeg", nil
	}
	ffmpegCommandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		helperArgs := []string{"-test.run=^TestFFmpegHelperProcess$", "--"}
		helperArgs = append(helperArgs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], helperArgs...)
		cmd.Env = append(os.Environ(), "GO_WANT_FFMPEG_HELPER_PROCESS=1")
		return cmd
	}
	t.Cleanup(func() {
		ffmpegCommandContext = oldCommandContext
		ffmpegBinaryResolver = oldBinaryResolver
	})

	directory := t.TempDir()
	rawPath := filepath.Join(directory, "sample_raw.bin")
	finalPath := filepath.Join(directory, "sample.mp3")
	if err := os.WriteFile(rawPath, []byte("fLaCtest audio"), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		_, err := FinishAudio(ctx, rawPath, directory, "sample", "mp3", "320k")
		errCh <- err
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(finalPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper ffmpeg did not create its partial output")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrTaskCanceled) {
			t.Fatalf("FinishAudio() error = %v, want %v", err, ErrTaskCanceled)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("FinishAudio did not return after cancellation")
	}

	for _, path := range []string{rawPath, finalPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("temporary file %q still exists after cancellation: %v", path, err)
		}
	}
}

func TestResolveFFmpegBinaryUsesExplicitOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-ffmpeg")
	if err := os.WriteFile(path, []byte("test"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ffmpegPathEnv, path)

	got, err := resolveFFmpegBinary()
	if err != nil {
		t.Fatalf("resolveFFmpegBinary() error = %v", err)
	}
	if got != path {
		t.Fatalf("resolveFFmpegBinary() = %q, want %q", got, path)
	}
}

func TestResolveFFmpegBinaryRejectsMissingExplicitOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-ffmpeg")
	t.Setenv(ffmpegPathEnv, path)

	_, err := resolveFFmpegBinary()
	if err == nil {
		t.Fatal("resolveFFmpegBinary() error = nil, want missing override error")
	}
	if !strings.Contains(err.Error(), ffmpegPathEnv) {
		t.Fatalf("resolveFFmpegBinary() error = %q, want environment variable name", err)
	}
}

func TestFFmpegHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_FFMPEG_HELPER_PROCESS") != "1" {
		return
	}
	if len(os.Args) == 0 {
		os.Exit(2)
	}
	outputPath := os.Args[len(os.Args)-1]
	if err := os.WriteFile(outputPath, []byte("partial output"), 0600); err != nil {
		os.Exit(3)
	}
	time.Sleep(30 * time.Second)
}
