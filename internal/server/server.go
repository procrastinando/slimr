package server

import (
	"slimr/internal/config"
	"slimr/internal/scheduler"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

type Server struct {
	cfg        *config.Config
	configPath string
	scheduler  *scheduler.Scheduler
	logger     *LogBroadcaster
	mux        *http.ServeMux
	webFS      fs.FS
}

func New(cfg *config.Config, configPath string, sched *scheduler.Scheduler, webFS fs.FS) *Server {
	srv := &Server{
		cfg:        cfg,
		configPath: configPath,
		scheduler:  sched,
		logger:     NewLogBroadcaster(),
		mux:        http.NewServeMux(),
		webFS:      webFS,
	}
	srv.setupRoutes()
	return srv
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) LogBroadcaster() *LogBroadcaster {
	return s.logger
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/api/config", s.handleConfigAPI)
	s.mux.HandleFunc("/api/start", s.handleStartAPI)
	s.mux.HandleFunc("/api/stop", s.handleStopAPI)
	s.mux.HandleFunc("/api/status", s.handleStatusAPI)
	s.mux.HandleFunc("/api/logs", s.handleLogsAPI)
	s.mux.HandleFunc("/api/logs/stream", s.handleLogStreamAPI)

	webDir, _ := fs.Sub(s.webFS, "web")
	fileServer := http.FileServer(http.FS(webDir))
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, map[string]string{"error": "not found"})
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			data, err := fs.ReadFile(webDir, "index.html")
			if err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(data)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) handleConfigAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.cfg)
	case http.MethodPut:
		var incoming config.Config
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		mergeConfig(s.cfg, &incoming)
		s.cfg.ExpandPaths()
		if err := config.Save(s.configPath, s.cfg); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		if s.scheduler.IsRunning() {
			s.scheduler.ReloadConfig()
		}
		writeJSON(w, map[string]string{"ok": "saved"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleStartAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.scheduler.Start()
	writeJSON(w, map[string]string{"ok": "started"})
}

func (s *Server) handleStopAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.scheduler.Stop()
	writeJSON(w, map[string]string{"ok": "stopped"})
}

func (s *Server) handleStatusAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.scheduler.Status())
}

func (s *Server) handleLogsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}
	lines := tailLogFile(s.cfg.OutputPath+"/conversion.log", tail)
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, strings.Join(lines, "\n"))
}

func (s *Server) handleLogStreamAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.logger.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func mergeConfig(current *config.Config, partial *config.Config) {
	if partial.InputPath != "" {
		current.InputPath = partial.InputPath
	}
	if partial.OutputPath != "" {
		current.OutputPath = partial.OutputPath
	}
	if partial.BindAddress != "" {
		current.BindAddress = partial.BindAddress
	}
	if partial.Port != "" {
		current.Port = partial.Port
	}
	if partial.ImageCodec != "" {
		current.ImageCodec = partial.ImageCodec
	}
	if partial.ImageQuality != 0 {
		current.ImageQuality = partial.ImageQuality
	}
	if partial.AvifCPU != 0 {
		current.AvifCPU = partial.AvifCPU
	}
	if partial.VideoCodec != "" {
		current.VideoCodec = partial.VideoCodec
	}
	if partial.VideoBitrate != "" {
		current.VideoBitrate = partial.VideoBitrate
	}
	if partial.AudioCodec != "" {
		current.AudioCodec = partial.AudioCodec
	}
	if partial.AudioBitrate != "" {
		current.AudioBitrate = partial.AudioBitrate
	}
	if partial.VideoQuality != "" {
		current.VideoQuality = partial.VideoQuality
	}
	// DeleteOriginal is always set from partial (bool, false means don't delete)
	current.DeleteOriginal = partial.DeleteOriginal
	if partial.WindowStart != "" {
		current.WindowStart = partial.WindowStart
	}
	if partial.WindowEnd != "" {
		current.WindowEnd = partial.WindowEnd
	}
	if len(partial.ImageExtensions) > 0 {
		current.ImageExtensions = partial.ImageExtensions
	}
	if len(partial.VideoExtensions) > 0 {
		current.VideoExtensions = partial.VideoExtensions
	}
}
