package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFormatErrorPreservesFFmpegInstallInstructions(t *testing.T) {
	want := "找不到 FFmpeg；请先运行 brew install ffmpeg"
	if got := formatError(errors.New(want)); got != want {
		t.Fatalf("formatError() = %q, want %q", got, want)
	}
}

func TestGetRuntimePlatform(t *testing.T) {
	if got := NewApp().GetRuntimePlatform(); got != runtime.GOOS {
		t.Fatalf("GetRuntimePlatform() = %q, want %q", got, runtime.GOOS)
	}
}

func TestDefaultOutputDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	got, err := defaultOutputDir()
	if err != nil {
		t.Fatalf("defaultOutputDir() error = %v", err)
	}

	want := filepath.Join(homeDir, "Music", "Music Unlocker")
	if got != want {
		t.Fatalf("defaultOutputDir() = %q, want %q", got, want)
	}
}

func TestEnsureOutputDirUsesAndCreatesDefault(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	got, err := ensureOutputDir("")
	if err != nil {
		t.Fatalf("ensureOutputDir() error = %v", err)
	}

	info, err := os.Stat(got)
	if err != nil {
		t.Fatalf("default output directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("default output path %q is not a directory", got)
	}
}

func TestEnsureOutputDirUsesSelectedDirectory(t *testing.T) {
	selectedDir := filepath.Join(t.TempDir(), "selected")

	got, err := ensureOutputDir("  " + selectedDir + "  ")
	if err != nil {
		t.Fatalf("ensureOutputDir() error = %v", err)
	}
	if got != selectedDir {
		t.Fatalf("ensureOutputDir() = %q, want %q", got, selectedDir)
	}
}
