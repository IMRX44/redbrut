package input

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LoadLines reads a file and returns non-empty, non-comment lines.
// Supports UTF-8 (Russian, Chinese, etc.).
func LoadLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line buffer for long passwords
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines, sc.Err()
}

// ParseTarget parses "IP:PORT" or "IP" (defaults to port 3389).
func ParseTarget(s string) (Target, error) {
	s = strings.TrimSpace(s)
	if idx := strings.LastIndex(s, ":"); idx != -1 {
		host := s[:idx]
		portStr := s[idx+1:]
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return Target{}, fmt.Errorf("invalid port in %q", s)
		}
		return Target{Host: host, Port: port}, nil
	}
	return Target{Host: s, Port: 3389}, nil
}

// LoadTargets reads a file of IP:PORT lines.
func LoadTargets(path string) ([]Target, error) {
	lines, err := LoadLines(path)
	if err != nil {
		return nil, err
	}
	targets := make([]Target, 0, len(lines))
	for _, line := range lines {
		t, err := ParseTarget(line)
		if err != nil {
			return nil, fmt.Errorf("parse target %q: %w", line, err)
		}
		targets = append(targets, t)
	}
	return targets, nil
}
