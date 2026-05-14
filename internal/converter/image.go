package converter

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"

	"slimr/internal/config"
)

func ConvertImage(in, out string, cfg *config.Config) error {
	var args []string

	switch cfg.ImageCodec {
	case "avif":
		args = []string{
			"-noautorotate", "-i", in,
			"-c:v", "libaom-av1",
			"-still-picture", "1",
			"-cpu-used", strconv.Itoa(cfg.AvifCPU),
			"-crf", strconv.Itoa(cfg.ImageQuality),
			"-y", out,
		}
	case "jpg":
		args = []string{
			"-noautorotate", "-i", in,
			"-q:v", strconv.Itoa(cfg.ImageQuality),
			"-y", out,
		}
	default:
		return fmt.Errorf("unknown image codec: %s", cfg.ImageCodec)
	}

	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, stderr.String())
	}
	return nil
}
