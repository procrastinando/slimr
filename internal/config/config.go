package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	InputPath       string   `json:"input_path"`
	OutputPath      string   `json:"output_path"`
	BindAddress     string   `json:"bind_address"`
	Port            string   `json:"port"`
	Running         bool     `json:"running"`
	ImageCodec      string   `json:"image_codec"`
	ImageQuality    int      `json:"image_quality"`
	AvifCPU         int      `json:"avif_cpu"`
	VideoCodec      string   `json:"video_codec"`
	VideoBitrate    string   `json:"video_bitrate"`
	AudioCodec      string   `json:"audio_codec"`
	AudioBitrate    string   `json:"audio_bitrate"`
	VideoQuality    string   `json:"video_quality"`
	DeleteOriginal  bool     `json:"delete_original"`
	WindowStart     string   `json:"window_start"`
	WindowEnd       string   `json:"window_end"`
	ImageExtensions []string `json:"image_extensions"`
	VideoExtensions []string `json:"video_extensions"`
}

func Default() *Config {
	return &Config{
		InputPath:       "~/storage/dcim/Camera",
		OutputPath:      "~/storage/dcim/slimr",
		BindAddress:     "127.0.0.1",
		Port:            "8880",
		ImageCodec:      "avif",
		ImageQuality:    30,
		AvifCPU:         4,
		VideoCodec:      "hevc_mediacodec",
		VideoBitrate:    "3500k",
		AudioCodec:      "libopus",
		AudioBitrate:    "96k",
		VideoQuality:    "",
		DeleteOriginal:  false,
		WindowStart:     "23:00",
		WindowEnd:       "07:00",
		ImageExtensions: []string{".jpg", ".jpeg", ".png", ".heic", ".webp"},
		VideoExtensions: []string{".mp4", ".mkv", ".mov", ".avi", ".webm", ".3gp"},
	}
}

func (c *Config) ExpandPaths() {
	if strings.HasPrefix(c.InputPath, "~/") {
		home, _ := os.UserHomeDir()
		c.InputPath = filepath.Join(home, c.InputPath[2:])
	}
	if strings.HasPrefix(c.OutputPath, "~/") {
		home, _ := os.UserHomeDir()
		c.OutputPath = filepath.Join(home, c.OutputPath[2:])
	}
}

func (c *Config) UnexpandPaths() {
	home, _ := os.UserHomeDir()
	homePrefix := home + "/storage/"
	if strings.HasPrefix(c.InputPath, homePrefix) {
		c.InputPath = "~/storage/" + c.InputPath[len(homePrefix):]
	}
	if strings.HasPrefix(c.OutputPath, homePrefix) {
		c.OutputPath = "~/storage/" + c.OutputPath[len(homePrefix):]
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.ExpandPaths()
	return cfg, nil
}

func Save(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
