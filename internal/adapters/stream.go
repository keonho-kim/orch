package adapters

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type streamScanOptions struct {
	AllowComments   bool
	RequireDataLine bool
	StripDataPrefix bool
}

func scanStreamLines(body io.Reader, options streamScanOptions, handle func(string) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if options.AllowComments && strings.HasPrefix(line, ":") {
			continue
		}
		if options.RequireDataLine {
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		} else if options.StripDataPrefix && strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "[DONE]" {
			break
		}
		if err := handle(line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan stream: %w", err)
	}
	return nil
}

func emitDelta(onDelta DeltaHandler, delta Delta) error {
	if onDelta == nil {
		return nil
	}
	if delta.Content == "" && delta.Reasoning == "" {
		return nil
	}
	return onDelta(delta)
}
