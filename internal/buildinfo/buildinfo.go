package buildinfo

import "strings"

var (
	version = "dev"
	commit  = ""
	date    = ""
	builtBy = ""
)

func Set(versionValue string, commitValue string, dateValue string, builtByValue string) {
	if trimmed := strings.TrimSpace(versionValue); trimmed != "" {
		version = trimmed
	}
	if trimmed := strings.TrimSpace(commitValue); trimmed != "" {
		commit = trimmed
	}
	if trimmed := strings.TrimSpace(dateValue); trimmed != "" {
		date = trimmed
	}
	if trimmed := strings.TrimSpace(builtByValue); trimmed != "" {
		builtBy = trimmed
	}
}

func Version() string {
	if strings.TrimSpace(version) == "" {
		return "dev"
	}
	return strings.TrimSpace(version)
}

func Commit() string {
	return strings.TrimSpace(commit)
}

func Date() string {
	return strings.TrimSpace(date)
}

func BuiltBy() string {
	return strings.TrimSpace(builtBy)
}
