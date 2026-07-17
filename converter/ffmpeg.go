package converter

import (
	"fmt"
	"os/exec"
)

func ConvertAudio(inputPath string, outputPath string, outputFormat string, bitrate string) error {
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

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()

	fmt.Println("FFmpeg 输出:")
	fmt.Println(string(output))

	if err != nil {
		return fmt.Errorf("ffmpeg error: %v", err)
	}

	return nil
}
