package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

type SSEEvent struct {
	File     string  `json:"file"`
	Elapsed  float64 `json:"elapsed"`
	Total    float64 `json:"total"`
	Done     int     `json:"done"`
	TotalFiles int   `json:"total_files"`
}

type LogBroadcaster struct {
	clients map[chan string]bool
	mu      sync.RWMutex
}

func NewLogBroadcaster() *LogBroadcaster {
	return &LogBroadcaster{
		clients: make(map[chan string]bool),
	}
}

func (lb *LogBroadcaster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan string, 50)
	lb.mu.Lock()
	lb.clients[ch] = true
	lb.mu.Unlock()

	defer func() {
		lb.mu.Lock()
		delete(lb.clients, ch)
		lb.mu.Unlock()
		close(ch)
	}()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func (lb *LogBroadcaster) BroadcastLog(line string) {
	msg, _ := json.Marshal(map[string]string{
		"type": "log",
		"line": line,
	})
	lb.broadcast(string(msg))
}

func (lb *LogBroadcaster) BroadcastProgress(file string, elapsed, total float64, done, totalFiles int) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type":        "progress",
		"file":        file,
		"elapsed":     elapsed,
		"total":       total,
		"done":        done,
		"total_files": totalFiles,
	})
	lb.broadcast(string(msg))
}

func (lb *LogBroadcaster) BroadcastStatus(state string, filesTotal, filesDone int) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type":        "status",
		"state":       state,
		"files_total": filesTotal,
		"files_done":  filesDone,
	})
	lb.broadcast(string(msg))
}

func (lb *LogBroadcaster) broadcast(msg string) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	for ch := range lb.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func tailLogFile(path string, n string) []string {
	var lines []string
	f, err := os.Open(path)
	if err != nil {
		return lines
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	count := len(lines)
	nInt := 100
	if n != "" {
		fmt.Sscanf(n, "%d", &nInt)
	}

	if count > nInt {
		lines = lines[count-nInt:]
	}
	return lines
}

type LogWriter struct {
	lb      *LogBroadcaster
	builder strings.Builder
}

func (w *LogWriter) Write(p []byte) (int, error) {
	w.builder.Write(p)
	if strings.Contains(w.builder.String(), "\n") {
		parts := strings.Split(w.builder.String(), "\n")
		for _, part := range parts {
			if part != "" {
				w.lb.BroadcastLog(part)
			}
		}
		w.builder.Reset()
	}
	return len(p), nil
}
