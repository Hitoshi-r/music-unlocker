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

// QMC1 uses a fixed 64-byte mask table. The mask is mirrored after index 0x3f
// and repeats at the 0x7fff boundary. Algorithm reference: Presburger's
// qmc-decoder (MIT); see THIRD_PARTY_NOTICES.md.
var qmc1KeyTable = [64]byte{
	0xc3, 0x4a, 0xd6, 0xca, 0x90, 0x67, 0xf7, 0x52,
	0xd8, 0xa1, 0x66, 0x62, 0x9f, 0x5b, 0x09, 0x00,
	0xc3, 0x5e, 0x95, 0x23, 0x9f, 0x13, 0x11, 0x7e,
	0xd8, 0x92, 0x3f, 0xbc, 0x90, 0xbb, 0x74, 0x0e,
	0xc3, 0x47, 0x74, 0x3d, 0x90, 0xaa, 0x3f, 0x51,
	0xd8, 0xf4, 0x11, 0x84, 0x9f, 0xde, 0x95, 0x1d,
	0xc3, 0xc6, 0x09, 0xd5, 0x9f, 0xfa, 0x66, 0xf9,
	0xd8, 0xf0, 0xf7, 0xa0, 0x90, 0xa1, 0xd6, 0xf3,
}

func NewQMCDecoder() *QMCDecoder {
	return &QMCDecoder{}
}

func (d *QMCDecoder) Name() string {
	return "QQ音乐"
}

func (d *QMCDecoder) Extensions() []string {
	return []string{
		".qmc0", ".qmc2", ".qmc3", ".qmc4", ".qmc6", ".qmc8", ".qmcflac", ".qmcogg",
		".mgg", ".mgg0", ".mgg1", ".mggl",
		".mflac", ".mflac0", ".mflach", ".mmp4",
		".bkcmp3", ".bkcflac", ".bkcwav", ".bkcogg", ".bkcwma", ".bkcape", ".bkcm4a", ".tkm",
		".666c6163", ".6d7033", ".6f6767", ".6d3461", ".776176",
		".tm0", ".tm2", ".tm3", ".tm6",
	}
}

func (d *QMCDecoder) Decode(
	inputPath string,
	outputDir string,
	outputFormat string,
	bitrate string,
	options DecodeOptions,
	ctx context.Context,
	onProgress ProgressCallback,
) (*DecodeResult, error) {
	ext := strings.ToLower(filepath.Ext(inputPath))
	if isQQTMExtension(ext) {
		return d.decodeQQTM(inputPath, outputDir, outputFormat, bitrate, ctx, onProgress)
	}
	if isQMC1Extension(ext) {
		if strings.TrimSpace(options.QQEKey) != "" || fileHasQMC2Footer(inputPath) {
			return d.decodeQMC2(inputPath, outputDir, outputFormat, bitrate, options, ctx, onProgress)
		}
		return d.decodeQMC1(inputPath, outputDir, outputFormat, bitrate, ctx, onProgress)
	}
	if isQMC2Extension(ext) {
		if strings.TrimSpace(options.QQEKey) == "" && !fileHasQMC2Footer(inputPath) && qmc1ProbeLooksLikeAudio(inputPath) {
			return d.decodeQMC1(inputPath, outputDir, outputFormat, bitrate, ctx, onProgress)
		}
		return d.decodeQMC2(inputPath, outputDir, outputFormat, bitrate, options, ctx, onProgress)
	}
	return nil, errors.New("未知 QQ 音乐格式")
}

func isQMC1Extension(ext string) bool {
	switch ext {
	case ".qmc0", ".qmc2", ".qmc3", ".qmc4", ".qmc6", ".qmc8", ".qmcflac", ".qmcogg",
		".666c6163", ".6d7033", ".6f6767", ".6d3461", ".776176":
		return true
	default:
		return false
	}
}

func isQMC2Extension(ext string) bool {
	switch ext {
	case ".mgg", ".mgg0", ".mgg1", ".mggl", ".mflac", ".mflac0", ".mflach", ".mmp4",
		".bkcmp3", ".bkcflac", ".bkcwav", ".bkcogg", ".bkcwma", ".bkcape", ".bkcm4a", ".tkm":
		return true
	default:
		return false
	}
}

func isQQTMExtension(ext string) bool {
	switch ext {
	case ".tm0", ".tm2", ".tm3", ".tm6":
		return true
	default:
		return false
	}
}

func qmc1Mask(offset int64) byte {
	current := offset
	if current > 0x7fff {
		current %= 0x7fff
	}
	index := int(current & 0x7f)
	if index > 0x3f {
		index = (0x80 - index) & 0x3f
	}
	return qmc1KeyTable[index]
}

func fileHasQMC2Footer(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() < 8 {
		return false
	}
	const maxFooterProbe = int64(2048)
	start := info.Size() - maxFooterProbe
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return false
	}
	tail, err := io.ReadAll(io.LimitReader(file, maxFooterProbe))
	if err != nil {
		return false
	}
	return parseQMCFooter(tail).Kind != qmcFooterUnknown
}

func qmc1ProbeLooksLikeAudio(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	header := make([]byte, 16)
	n, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return false
	}
	header = header[:n]
	for i := range header {
		header[i] ^= qmc1Mask(int64(i))
	}
	return hasAudioMagic(header)
}

func (d *QMCDecoder) decodeQMC1(
	inputPath string,
	outputDir string,
	outputFormat string,
	bitrate string,
	ctx context.Context,
	onProgress ProgressCallback,
) (*DecodeResult, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}

	input, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer input.Close()

	info, err := input.Stat()
	if err != nil {
		return nil, err
	}

	name := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	rawPath := converter.UniqueOutputPath(outputDir, name+"_qmc_raw", ".bin")
	output, err := os.Create(rawPath)
	if err != nil {
		return nil, err
	}

	completed := false
	defer func() {
		_ = output.Close()
		if !completed {
			_ = os.Remove(rawPath)
		}
	}()

	buffer := make([]byte, 256*1024)
	var offset int64
	lastPercent := -1
	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("任务已取消")
		default:
		}

		n, readErr := input.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]
			for i := range chunk {
				chunk[i] ^= qmc1Mask(offset + int64(i))
			}
			if _, err := output.Write(chunk); err != nil {
				return nil, err
			}
			offset += int64(n)
			if info.Size() > 0 && onProgress != nil {
				percent := int(offset * 90 / info.Size())
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
			return nil, readErr
		}
	}

	if err := output.Close(); err != nil {
		return nil, err
	}

	finalPath, err := finishQMCOutput(ctx, rawPath, outputDir, name, outputFormat, bitrate)
	if err != nil {
		return nil, err
	}
	completed = true
	if onProgress != nil {
		onProgress(100)
	}

	return &DecodeResult{
		InputPath:  inputPath,
		OutputPath: finalPath,
		Platform:   d.Name(),
		Success:    true,
		Message:    "QQ 音乐文件处理完成",
	}, nil
}

func (d *QMCDecoder) decodeQMC2(
	inputPath string,
	outputDir string,
	outputFormat string,
	bitrate string,
	options DecodeOptions,
	ctx context.Context,
	onProgress ProgressCallback,
) (*DecodeResult, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}

	name := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	rawPath := converter.UniqueOutputPath(outputDir, name+"_qmc2_raw", ".bin")
	if err := decryptQMC2File(ctx, inputPath, rawPath, options, onProgress); err != nil {
		return nil, err
	}

	finalPath, err := finishQMCOutput(ctx, rawPath, outputDir, name, outputFormat, bitrate)
	if err != nil {
		_ = os.Remove(rawPath)
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
		Message:    "QQ 音乐新版文件处理完成",
	}, nil
}

func (d *QMCDecoder) decodeQQTM(
	inputPath string,
	outputDir string,
	outputFormat string,
	bitrate string,
	ctx context.Context,
	onProgress ProgressCallback,
) (*DecodeResult, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, errors.New("任务已取消")
	default:
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, err
	}
	if len(data) < 8 {
		return nil, errors.New("QQ TM 文件长度不足")
	}
	copy(data[:8], []byte{0x00, 0x00, 0x00, 0x20, 'f', 't', 'y', 'p'})
	if !hasAudioMagic(data) {
		return nil, errors.New("QQ TM 文件不是可识别的 M4A 音频")
	}
	name := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	rawPath := converter.UniqueOutputPath(outputDir, name+"_tm_raw", ".bin")
	if err := os.WriteFile(rawPath, data, 0644); err != nil {
		return nil, err
	}
	finalPath, err := finishQMCOutput(ctx, rawPath, outputDir, name, outputFormat, bitrate)
	if err != nil {
		_ = os.Remove(rawPath)
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
		Message:    "QQ 音乐 TM 文件处理完成",
	}, nil
}

func finishQMCOutput(
	ctx context.Context,
	rawPath string,
	outputDir string,
	name string,
	outputFormat string,
	bitrate string,
) (string, error) {
	realExt, err := converter.DetectAudioExt(rawPath)
	if err != nil {
		return "", err
	}
	if realExt == ".bin" {
		return "", errors.New("QQ 音乐解密结果不是可识别音频，文件可能损坏或格式尚未支持")
	}
	return converter.FinishAudio(ctx, rawPath, outputDir, name, outputFormat, bitrate)
}
