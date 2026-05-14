package converter

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"slimr/internal/config"
)

func ConvertVideo(in, out string, cfg *config.Config) (*exec.Cmd, io.ReadCloser, error) {
	var args []string

	switch cfg.VideoCodec {
	case "hevc_mediacodec":
		args = []string{
			"-hwaccel", "mediacodec",
			"-i", in,
			"-c:v", "hevc_mediacodec",
			"-bitrate_mode", "vbr",
			"-b:v", cfg.VideoBitrate,
			"-c:a", cfg.AudioCodec,
			"-b:a", cfg.AudioBitrate,
			"-y", out,
		}
	case "libsvtav1":
		args = []string{
			"-i", in,
			"-c:v", "libsvtav1",
			"-preset", cfg.VideoQuality,
			"-b:v", cfg.VideoBitrate,
			"-c:a", cfg.AudioCodec,
			"-b:a", cfg.AudioBitrate,
			"-y", out,
		}
	case "libx265":
		args = []string{
			"-i", in,
			"-c:v", "libx265",
			"-preset", cfg.VideoQuality,
			"-crf", cfg.VideoQuality,
			"-c:a", cfg.AudioCodec,
			"-b:a", cfg.AudioBitrate,
			"-y", out,
		}
	default:
		return nil, nil, fmt.Errorf("unknown video codec: %s", cfg.VideoCodec)
	}

	cmd := exec.Command("ffmpeg", args...)
	stdoutPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	return cmd, stdoutPipe, nil
}

func ConvertVideoSimple(in, out string, cfg *config.Config) error {
	cmd, stderr, err := ConvertVideo(in, out, cfg)
	if err != nil {
		return err
	}
	defer stderr.Close()

	var buf bytes.Buffer
	io.Copy(&buf, stderr)
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, buf.String())
	}
	return nil
}

func AudioCodecForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp4", ".mkv", ".mov", ".avi", ".webm", ".3gp":
		return "libopus"
	default:
		return "aac"
	}
}
