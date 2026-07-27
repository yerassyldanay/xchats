package main

import (
	"bufio"
	"os"
	"strings"
)

// loadDotEnv best-effort loads KEY=VALUE lines from path into the process environment,
// without overwriting a variable already set (so `OPENROUTER_API_KEY=x harness extract`
// on the command line always wins over a stale .env). Missing file is not an error — .env
// is optional; cases.yaml runs can also be fed via a real environment (e.g. CI secrets).
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}
