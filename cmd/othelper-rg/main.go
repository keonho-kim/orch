package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	matched, err := run(os.Args[1:], os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rg: %v\n", err)
		os.Exit(2)
	}
	if !matched {
		os.Exit(1)
	}
}

type rgConfig struct {
	Pattern string
	Paths   []string
	Hidden  bool
}

func run(args []string, stdout io.Writer) (bool, error) {
	config, err := parseArgs(args)
	if err != nil {
		return false, err
	}

	pattern, err := regexp.Compile(config.Pattern)
	if err != nil {
		return false, fmt.Errorf("compile pattern: %w", err)
	}

	matched := false
	for _, root := range config.Paths {
		found, err := searchPath(root, pattern, config.Hidden, stdout)
		if err != nil {
			return false, err
		}
		if found {
			matched = true
		}
	}
	return matched, nil
}

func parseArgs(args []string) (rgConfig, error) {
	config := rgConfig{}
	separator := -1
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--line-number", "--no-heading":
		case "--hidden":
			config.Hidden = true
		case "--color":
			if index+1 >= len(args) {
				return rgConfig{}, fmt.Errorf("--color requires a value")
			}
			index++
		case "--":
			separator = index
			index = len(args)
		default:
			return rgConfig{}, fmt.Errorf("unsupported arg %q", args[index])
		}
	}

	if separator == -1 || separator+2 > len(args) {
		return rgConfig{}, fmt.Errorf("pattern and path are required")
	}

	config.Pattern = args[separator+1]
	config.Paths = args[separator+2:]
	if len(config.Paths) == 0 {
		return rgConfig{}, fmt.Errorf("path is required")
	}
	return config, nil
}

func searchPath(root string, pattern *regexp.Regexp, includeHidden bool, stdout io.Writer) (bool, error) {
	info, err := os.Stat(root)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", root, err)
	}

	if !includeHidden && pathHasHiddenSegment(root) {
		return false, nil
	}

	if !info.IsDir() {
		return searchFile(root, pattern, stdout)
	}

	matched := false
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !includeHidden && pathHasHiddenSegment(path) && path != root {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}

		found, err := searchFile(path, pattern, stdout)
		if err != nil {
			return err
		}
		if found {
			matched = true
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("search %s: %w", root, err)
	}
	return matched, nil
}

func searchFile(path string, pattern *regexp.Regexp, stdout io.Writer) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	lineNo := 0
	matched := false
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if !pattern.MatchString(line) {
			continue
		}
		matched = true
		if _, err := fmt.Fprintf(stdout, "%s:%d:%s\n", path, lineNo, line); err != nil {
			return false, fmt.Errorf("write match: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("scan %s: %w", path, err)
	}
	return matched, nil
}

func pathHasHiddenSegment(path string) bool {
	cleaned := filepath.Clean(path)
	for _, segment := range strings.Split(cleaned, string(filepath.Separator)) {
		if segment == "" || segment == "." || segment == ".." {
			continue
		}
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}
