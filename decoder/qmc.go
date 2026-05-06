package decoder

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"music-unlocker/converter"
)

type QMCDecoder struct{}

var qmcKey = []byte{
	0x77, 0x48, 0x32, 0x73, 0x3A, 0x64, 0x6B, 0x6A,
	0x6F, 0x31, 0x33, 0x34, 0x6C, 0x6B, 0x73, 0x64,
}

func NewQMCDecoder() *QMCDecoder {
	return &QMCDecoder{}
}

func (d *QMCDecoder) Name() string {
	return "QQ音乐"
}

func (d *QMCDecoder) Extensions() []string {
	return []string{
		".qmc0",
		".qmc3",
		".qmcflac",
		".qmcogg",
	}
}

func (d *QMCDecoder) Decode(
	inputPath string,
	outputDir string,
	outputFormat string,
	bitrate string,
	ctx context.Context,
	onProgress ProgressCallback,
) (*DecodeResult, error) {
	ext := strings.ToLower(filepath.Ext(inputPath))

	switch ext {
	case ".qmc0", ".qmc3", ".qmcflac", ".qmcogg":
		return d.decodeOldQMC(inputPath, outputDir, outputFormat, bitrate, ctx, onProgress)

	default:
		return nil, errors.New("未知 QMC 格式")
	}
}

func (d *QMCDecoder) decodeOldQMC(
	inputPath string,
	outputDir string,
	outputFormat string,
	bitrate string,
	ctx context.Context,
	onProgress ProgressCallback,
) (*DecodeResult, error) {

	if outputDir == "" {
		outputDir = filepath.Dir(inputPath)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}

	file, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	name := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	rawPath := filepath.Join(outputDir, name+"_qmc_raw.bin")

	outFile, err := os.Create(rawPath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		outFile.Close()
		return nil, err
	}

	totalSize := fileInfo.Size()
	var processed int64
	lastPercent := -1
	buffer := make([]byte, 64*1024)

	for {
		select {
		case <-ctx.Done():
			outFile.Close()
			os.Remove(rawPath)
			return nil, errors.New("任务已取消")
		default:
		}

		n, readErr := file.Read(buffer)

		if n > 0 {
			data := buffer[:n]

			for i := 0; i < n; i++ {
				index := (int(processed) + i) & 0x7fff
				key := qmcKey[index%len(qmcKey)]
				data[i] ^= key
			}

			if _, err := outFile.Write(data); err != nil {
				outFile.Close()
				os.Remove(rawPath)
				return nil, err
			}

			processed += int64(n)

			if totalSize > 0 && onProgress != nil {
				percent := int(processed * 90 / totalSize)
				if percent > 90 {
					percent = 90
				}
				if percent != lastPercent {
					lastPercent = percent
					onProgress(percent)
				}
			}
		}

		if readErr == io.EOF {
			break
		}

		if readErr != nil {
			outFile.Close()
			os.Remove(rawPath)
			return nil, readErr
		}
	}

	if err := outFile.Close(); err != nil {
		os.Remove(rawPath)
		return nil, err
	}

	finalPath, err := finishQMCOutput(
		rawPath,
		outputDir,
		name,
		outputFormat,
		bitrate,
	)

	os.Remove(rawPath)

	if err != nil {
		return nil, err
	}

	if onProgress != nil {
		onProgress(100)
	}

	return &DecodeResult{
		InputPath:  inputPath,
		OutputPath: finalPath,
		Platform:   d.Name(),
		Success:    true,
		Message:    "QMC 解密完成",
	}, nil
}

func (d *QMCDecoder) decodeUnsupportedQQNew(
	inputPath string,
	outputDir string,
	ctx context.Context,
	onProgress ProgressCallback,
) (*DecodeResult, error) {

	if onProgress != nil {
		onProgress(100)
	}

	return &DecodeResult{
		InputPath:  inputPath,
		OutputPath: "",
		Platform:   d.Name(),
		Success:    false,
		Message:    "该文件是 QQ 音乐新版 mflac/mgg 加密格式，当前版本暂不支持。",
	}, nil
}

func finishQMCOutput(
	rawPath string,
	outputDir string,
	name string,
	outputFormat string,
	bitrate string,
) (string, error) {

	data, err := os.ReadFile(rawPath)
	if err != nil {
		return "", err
	}

	realExt := detectQMCRealExt(data)

	if outputFormat == "" || outputFormat == "auto" || outputFormat == "origin" {
		finalPath := filepath.Join(outputDir, name+realExt)
		return finalPath, os.WriteFile(finalPath, data, 0644)
	}

	return converter.FinishAudio(
		rawPath,
		outputDir,
		name,
		outputFormat,
		bitrate,
	)
}

func detectQMCRealExt(data []byte) string {
	if len(data) >= 4 && string(data[:4]) == "fLaC" {
		return ".flac"
	}

	if len(data) >= 4 && string(data[:4]) == "OggS" {
		return ".ogg"
	}

	if len(data) >= 4 && string(data[:4]) == "RIFF" {
		return ".wav"
	}

	if len(data) >= 3 && string(data[:3]) == "ID3" {
		return ".mp3"
	}

	if len(data) >= 2 && data[0] == 0xFF && data[1]&0xE0 == 0xE0 {
		return ".mp3"
	}

	return ".mp3"
}
