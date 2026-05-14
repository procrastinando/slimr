package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sync"
	"sync/atomic"
	"time"

	"slimr/internal/config"
	"slimr/internal/converter"
	"slimr/internal/scanner"
)

type Status struct {
	Running         bool    `json:"running"`
	State           string  `json:"state"`
	CurrentFile     string  `json:"current_file"`
	CurrentProgress float64 `json:"current_progress"`
	CurrentDuration float64 `json:"current_duration"`
	FilesTotal      int     `json:"files_total"`
	FilesDone       int     `json:"files_done"`
	LastRun         string  `json:"last_run"`
	InWindow        bool    `json:"in_window"`
	Version         string  `json:"version"`
}

type Broadcaster interface {
	BroadcastLog(line string)
	BroadcastProgress(file string, elapsed, total float64, done, totalFiles int)
	BroadcastStatus(state string, total, done int)
}

type Scheduler struct {
	cfg         *config.Config
	configPath  string
	scanner     *scanner.Scanner
	broadcaster Broadcaster
	logFile     string
	running     atomic.Bool
	stopCh      chan struct{}
	mu          sync.RWMutex
	status      Status
	version     string
}

func New(cfg *config.Config, configPath string, b Broadcaster, version string) *Scheduler {
	return &Scheduler{
		cfg:         cfg,
		configPath:  configPath,
		broadcaster: b,
		logFile:     filepath.Join(cfg.OutputPath, "conversion.log"),
		stopCh:      make(chan struct{}),
		version:     version,
	}
}

func (s *Scheduler) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.status
	st.Version = s.version
	return st
}

func (s *Scheduler) updateStatus(fn func(*Status)) {
	s.mu.Lock()
	fn(&s.status)
	s.mu.Unlock()
}

func (s *Scheduler) Start() {
	if s.running.Load() {
		return
	}
	s.running.Store(true)
	s.stopCh = make(chan struct{})
	s.cfg.ExpandPaths()
	s.scanner = scanner.New(s.cfg.InputPath, s.cfg.OutputPath,
		s.cfg.ImageExtensions, s.cfg.VideoExtensions,
		s.cfg.ImageCodec, s.cfg.VideoCodec)

	s.updateStatus(func(st *Status) {
		st.Running = true
		st.State = "idle"
	})

	go s.loop()
}

func (s *Scheduler) Stop() {
	if !s.running.Load() {
		return
	}
	s.running.Store(false)
	select {
	case s.stopCh <- struct{}{}:
	default:
	}
	s.updateStatus(func(st *Status) {
		st.Running = false
		st.State = "idle"
	})
}

func (s *Scheduler) IsRunning() bool {
	return s.running.Load()
}

func (s *Scheduler) ReloadConfig() error {
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return err
	}
	cfg.ExpandPaths()
	// Copy into existing pointer so server and scheduler share the same config
	*s.cfg = *cfg
	s.scanner = scanner.New(s.cfg.InputPath, s.cfg.OutputPath,
		s.cfg.ImageExtensions, s.cfg.VideoExtensions,
		s.cfg.ImageCodec, s.cfg.VideoCodec)
	return nil
}

func (s *Scheduler) inWindow() bool {
	now := time.Now()
	start, err := time.Parse("15:04", s.cfg.WindowStart)
	if err != nil {
		return true
	}
	end, err := time.Parse("15:04", s.cfg.WindowEnd)
	if err != nil {
		return true
	}
	start = time.Date(now.Year(), now.Month(), now.Day(), start.Hour(), start.Minute(), 0, 0, now.Location())
	end = time.Date(now.Year(), now.Month(), now.Day(), end.Hour(), end.Minute(), 0, 0, now.Location())

	if start.Equal(end) {
		return true // 24h window
	}
	if end.Before(start) {
		// Window crosses midnight (e.g. 23:00–07:00)
		return !now.Before(start) || now.Before(end)
	}
	// Same-day window (e.g. 00:00–07:00)
	return !now.Before(start) && now.Before(end)
}

func (s *Scheduler) loop() {
	defer s.running.Store(false)

	for s.running.Load() {
		if !s.inWindow() {
			s.updateStatus(func(st *Status) {
				st.State = "idle"
				st.InWindow = false
			})
			select {
			case <-s.stopCh:
				return
			case <-time.After(60 * time.Second):
				continue
			}
		}

		s.updateStatus(func(st *Status) {
			st.InWindow = true
			st.State = "scanning"
		})

		files, err := s.scanner.Scan()
		if err != nil {
			s.log(fmt.Sprintf("scan error: %v", err))
			select {
			case <-s.stopCh:
				return
			case <-time.After(60 * time.Second):
				continue
			}
		}

		if len(files) == 0 {
			s.updateStatus(func(st *Status) {
				st.State = "idle"
				st.LastRun = time.Now().Format("2006-01-02 15:04:05")
			})
			s.log("scan: no new files found")
			select {
			case <-s.stopCh:
				return
			case <-time.After(60 * time.Second):
				continue
			}
		}

		s.log(fmt.Sprintf("<<<<< %d files found, starting conversion >>>>>", len(files)))
		totalFiles := len(files)
		s.updateStatus(func(st *Status) {
			st.State = "working"
			st.FilesTotal = totalFiles
			st.FilesDone = 0
		})

		for i, f := range files {
			select {
			case <-s.stopCh:
				s.log(fmt.Sprintf("<<<<< conversion interrupted (%d/%d) >>>>>", i, totalFiles))
				return
			default:
			}

			if err := os.MkdirAll(filepath.Dir(f.OutPath), 0755); err != nil {
				s.log(fmt.Sprintf("mkdir error: %v", err))
				continue
			}

			relDisp := f.RelPath
			if len(relDisp) > 40 {
				relDisp = "..." + relDisp[len(relDisp)-37:]
			}

			s.updateStatus(func(st *Status) {
				st.CurrentFile = filepath.Base(f.RelPath)
			})

			startTime := time.Now()

			if f.IsVideo {
				s.processVideo(f, relDisp, startTime, totalFiles, i)
			} else {
				s.processImage(f, relDisp, startTime, totalFiles, i)
			}

			s.updateStatus(func(st *Status) {
				st.FilesDone = i + 1
			})

			// Check window after each file — stop if we left the window
			if !s.inWindow() {
				s.updateStatus(func(st *Status) {
					st.State = "idle"
					st.InWindow = false
					st.LastRun = time.Now().Format("2006-01-02 15:04:05")
				})
				s.log(fmt.Sprintf("<<<<< out of window, paused (%d/%d) >>>>>", i+1, totalFiles))
				return
			}
		}

		s.updateStatus(func(st *Status) {
			st.LastRun = time.Now().Format("2006-01-02 15:04:05")
			st.State = "idle"
			st.CurrentFile = ""
		})
		s.log("<<<<< conversion complete >>>>>")
	}
}

func (s *Scheduler) processImage(f scanner.File, relDisp string, startTime time.Time, totalFiles, doneSoFar int) {
	if err := converter.ConvertImage(f.AbsPath, f.OutPath, s.cfg); err != nil {
		s.log(fmt.Sprintf("ERROR > %s > %v", relDisp, err))
		return
	}

	if err := converter.TransferMetadata(f.AbsPath, f.OutPath); err != nil {
		s.log(fmt.Sprintf("META WARN > %s > %v", relDisp, err))
	}

	if s.cfg.DeleteOriginal {
		os.Remove(f.AbsPath)
	}

	s.scanner.MarkProcessed(f.RelPath)
	elapsed := time.Since(startTime).Seconds()
	line := fmt.Sprintf("%s > %s > %.2fs", timeNow(), relDisp, elapsed)
	s.log(line)
	s.broadcaster.BroadcastProgress(f.RelPath, elapsed, elapsed, doneSoFar+1, totalFiles)
}

func (s *Scheduler) processVideo(f scanner.File, relDisp string, startTime time.Time, totalFiles, doneSoFar int) {
	duration, _ := converter.Duration(f.AbsPath)

	// Check if original bitrate is already below target — skip conversion, just copy
	targetBitrate, err := converter.ParseBitrate(s.cfg.VideoBitrate)
	if err == nil && targetBitrate > 0 {
		origBitrate, err := converter.VideoBitrate(f.AbsPath)
		if err == nil && origBitrate > 0 {
			targetBps := targetBitrate * 1000
			if origBitrate < targetBps {
				s.log(fmt.Sprintf("%s > %s > SKIP (bitrate %dk < target %dk, copying)",
					timeNow(), relDisp, origBitrate/1000, targetBitrate))
				if s.cfg.DeleteOriginal {
					if err := converter.MoveFile(f.AbsPath, f.OutPath); err != nil {
						s.log(fmt.Sprintf("MOVE ERROR > %s > %v", relDisp, err))
						return
					}
				} else {
					if err := converter.CopyFile(f.AbsPath, f.OutPath); err != nil {
						s.log(fmt.Sprintf("COPY ERROR > %s > %v", relDisp, err))
						return
					}
				}
				if err := converter.TransferMetadata(f.AbsPath, f.OutPath); err != nil {
					s.log(fmt.Sprintf("META WARN > %s > %v", relDisp, err))
				}
				s.scanner.MarkProcessed(f.RelPath)
				return
			}
		}
	}

	cmd, stderr, err := converter.ConvertVideo(f.AbsPath, f.OutPath, s.cfg)
	if err != nil {
		s.log(fmt.Sprintf("ERROR > %s > %v", relDisp, err))
		return
	}

	progressCh := converter.ParseProgress(stderr, duration)
	go func() {
		for p := range progressCh {
			s.updateStatus(func(st *Status) {
				st.CurrentProgress = p.Elapsed
				st.CurrentDuration = duration
			})
			s.broadcaster.BroadcastProgress(f.RelPath, p.Elapsed, duration, doneSoFar+1, totalFiles)
		}
	}()

	if err := cmd.Wait(); err != nil {
		s.log(fmt.Sprintf("ERROR > %s > %v", relDisp, err))
		stderr.Close()
		return
	}
	stderr.Close()

	if err := converter.TransferMetadata(f.AbsPath, f.OutPath); err != nil {
		s.log(fmt.Sprintf("META WARN > %s > %v", relDisp, err))
	}

	if s.cfg.DeleteOriginal {
		os.Remove(f.AbsPath)
	}

	s.scanner.MarkProcessed(f.RelPath)
	elapsed := time.Since(startTime).Seconds()
	line := fmt.Sprintf("%s > %s > %.2fs / %.2fs", timeNow(), relDisp, elapsed, duration)
	s.log(line)
	s.broadcaster.BroadcastProgress(f.RelPath, elapsed, duration, doneSoFar+1, totalFiles)
}

func (s *Scheduler) log(line string) {
	logPath := s.logFile
	if strings.HasPrefix(logPath, "~/") {
		home, _ := os.UserHomeDir()
		logPath = filepath.Join(home, logPath[2:])
	}
	dir := filepath.Dir(logPath)
	os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("LOG ERROR: %v\n", err)
		return
	}
	defer f.Close()
	f.WriteString(line + "\n")
	if s.broadcaster != nil {
		s.broadcaster.BroadcastLog(line)
	}
}

func timeNow() string {
	return time.Now().Format("15:04:05")
}
