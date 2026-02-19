package parser

import (
	"path/filepath"
	"strings"
)

var projectMarkers = []string{
	"code", "projects", "repos", "src", "work", "dev",
}

var ignoredSystemDirs = map[string]bool{
	"users": true, "home": true, "var": true,
	"tmp": true, "private": true,
}

func normalizeName(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

// GetProjectName converts an encoded Claude project directory name
// to a clean project name. Claude encodes paths like
// /Users/wesm/code/my-app as -Users-wesm-code-my-app.
func GetProjectName(dirName string) string {
	if dirName == "" {
		return ""
	}

	if !strings.HasPrefix(dirName, "-") {
		return normalizeName(dirName)
	}

	parts := strings.Split(dirName, "-")

	// Strategy 1: find a known project parent directory marker
	for _, marker := range projectMarkers {
		for i, part := range parts {
			if strings.EqualFold(part, marker) && i+1 < len(parts) {
				result := strings.Join(parts[i+1:], "-")
				if result != "" {
					return normalizeName(result)
				}
			}
		}
	}

	// Strategy 2: use last non-system-directory component
	for i := len(parts) - 1; i >= 0; i-- {
		if p := parts[i]; p != "" && !ignoredSystemDirs[strings.ToLower(p)] {
			return normalizeName(p)
		}
	}

	return normalizeName(dirName)
}

// ExtractProjectFromCwd extracts a project name from a working
// directory path. Returns the last path component, normalized.
func ExtractProjectFromCwd(cwd string) string {
	if cwd == "" {
		return ""
	}
	name := filepath.Base(filepath.Clean(cwd))
	if name == "." || name == ".." || name == "/" || name == string(filepath.Separator) {
		return ""
	}
	if strings.ContainsAny(name, "/\\") {
		return ""
	}
	return normalizeName(name)
}

// NeedsProjectReparse checks if a stored project name looks like
// an un-decoded encoded path that should be re-extracted.
func NeedsProjectReparse(project string) bool {
	bad := []string{
		"_Users", "_home", "_private", "_tmp", "_var",
	}
	for _, prefix := range bad {
		if strings.HasPrefix(project, prefix) {
			return true
		}
	}
	return strings.Contains(project, "_var_folders_") ||
		strings.Contains(project, "_var_tmp_")
}
