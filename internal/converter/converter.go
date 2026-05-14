package converter

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Progress struct {
	Time     string
	Elapsed  float64
	Duration float64
}

var timeRe = regexp.MustCompile(`time=(\d+):(\d+):(\d+)\.(\d+)`)

func ParseDuration(s string) (float64, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, false
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	sec, _ := strconv.ParseFloat(parts[2], 64)
	return float64(h*3600) + float64(m*60) + sec, true
}

func ParseProgress(r io.Reader, duration float64) <-chan Progress {
	ch := make(chan Progress, 10)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			matches := timeRe.FindStringSubmatch(line)
			if len(matches) < 5 {
				continue
			}
			h, _ := strconv.Atoi(matches[1])
			m, _ := strconv.Atoi(matches[2])
			s, _ := strconv.Atoi(matches[3])
			ms, _ := strconv.Atoi(matches[4])
			elapsed := float64(h*3600+m*60+s) + float64(ms)/100.0
			ch <- Progress{
				Time:    fmt.Sprintf("%02d:%02d:%02d.%02d", h, m, s, ms),
				Elapsed: elapsed,
			}
		}
	}()
	return ch
}

func StatFile(path string) (exist bool, size int64, err error) {
	info, err := exec.Command("stat", "-c", "%s", path).Output()
	if err != nil {
		return false, 0, nil
	}
	sz, _ := strconv.ParseInt(strings.TrimSpace(string(info)), 10, 64)
	return true, sz, nil
}

func Duration(path string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-show_entries", "format=duration",
		"-of", "csv=p=0", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

func timeNow() string {
	return time.Now().Format("15:04:05")
}
