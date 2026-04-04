package apiserver

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Server) discoveryFilePath() string {
	return filepath.Join(s.paths.APIDir, s.discovery.SessionID+".json")
}

func (s *Server) currentDiscoveryPath() string {
	return filepath.Join(s.paths.APIDir, "current.json")
}

func (s *Server) writeDiscoveryFiles() error {
	if err := os.MkdirAll(s.paths.APIDir, 0o755); err != nil {
		return fmt.Errorf("create api discovery dir: %w", err)
	}
	if err := writeJSONFileAtomic(s.discoveryFilePath(), s.discovery); err != nil {
		return err
	}
	if err := writeJSONFileAtomic(s.currentDiscoveryPath(), s.discovery); err != nil {
		return err
	}
	return nil
}

func (s *Server) removeDiscoveryFiles() error {
	if err := os.Remove(s.discoveryFilePath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove api discovery file: %w", err)
	}

	data, err := os.ReadFile(s.currentDiscoveryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read current api discovery file: %w", err)
	}

	var current Discovery
	if err := json.Unmarshal(data, &current); err != nil {
		return fmt.Errorf("decode current api discovery file: %w", err)
	}
	if current.BaseURL == s.discovery.BaseURL && current.SessionID == s.discovery.SessionID {
		if err := os.Remove(s.currentDiscoveryPath()); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove current api discovery file: %w", err)
		}
	}
	return nil
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func writeJSONFileAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp %s: %w", filepath.Base(path), err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temp %s: %w", filepath.Base(path), err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename %s: %w", filepath.Base(path), err)
	}
	return nil
}
