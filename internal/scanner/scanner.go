package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Scanner struct {
	InputPath       string
	OutputPath      string
	ImageExtensions []string
	VideoExtensions []string
	ImageCodec      string
	VideoCodec      string
	processedDB     string
	processed       map[string]bool
	mu              sync.RWMutex
}

type File struct {
	AbsPath string
	RelPath string
	OutPath string
	IsVideo bool
	Ext     string
}

func New(inputPath, outputPath string, imageExts, videoExts []string, imageCodec, videoCodec string) *Scanner {
	dbPath := filepath.Join(outputPath, ".processed.db")
	s := &Scanner{
		InputPath:       inputPath,
		OutputPath:      outputPath,
		ImageExtensions: imageExts,
		VideoExtensions: videoExts,
		ImageCodec:      imageCodec,
		VideoCodec:      videoCodec,
		processedDB:     dbPath,
		processed:       make(map[string]bool),
	}
	s.loadProcessed()
	return s
}

func (s *Scanner) loadProcessed() {
	data, err := os.ReadFile(s.processedDB)
	if err != nil {
		return
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return
	}
	s.mu.Lock()
	for _, p := range list {
		s.processed[p] = true
	}
	s.mu.Unlock()
}

func (s *Scanner) saveProcessed() error {
	s.mu.RLock()
	list := make([]string, 0, len(s.processed))
	for p := range s.processed {
		list = append(list, p)
	}
	s.mu.RUnlock()
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.processedDB)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(s.processedDB, data, 0644)
}

func (s *Scanner) MarkProcessed(relPath string) error {
	s.mu.Lock()
	s.processed[relPath] = true
	s.mu.Unlock()
	return s.saveProcessed()
}

func (s *Scanner) IsProcessed(relPath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.processed[relPath]
}

func (s *Scanner) Scan() ([]File, error) {
	var files []File
	entries, err := os.ReadDir(s.InputPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}

		// Skip hidden/temp/empty files
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Size() == 0 {
			continue
		}

		path := filepath.Join(s.InputPath, name)
		ext := strings.ToLower(filepath.Ext(name))
		isImage := contains(s.ImageExtensions, ext)
		isVideo := contains(s.VideoExtensions, ext)
		if !isImage && !isVideo {
			continue
		}

		relPath := name
		outExt := ext
		if isImage {
			outExt = s.outputExtForImage()
		} else if isVideo {
			outExt = ".mp4"
		}

		outRel := relPath[:len(relPath)-len(ext)] + outExt
		outAbs := filepath.Join(s.OutputPath, outRel)

		if _, err := os.Stat(outAbs); err == nil {
			if s.IsProcessed(relPath) {
				continue
			}
		}

		files = append(files, File{
			AbsPath: path,
			RelPath: relPath,
			OutPath: outAbs,
			IsVideo: isVideo,
			Ext:     ext,
		})
	}
	return files, nil
}

func (s *Scanner) outputExtForImage() string {
	switch s.ImageCodec {
	case "avif":
		return ".avif"
	default:
		return ".jpg"
	}
}

func (s *Scanner) TotalProcessed() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.processed)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
