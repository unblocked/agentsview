package sync

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/wesm/agentsview/internal/parser"
)

// uuidRe matches a standard UUID (8-4-4-4-12 hex) at the end of a rollout filename stem.
var uuidRe = regexp.MustCompile(
	`^rollout-.*-([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-` +
		`[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})$`,
)

// DiscoveredFile holds a discovered session JSONL file.
type DiscoveredFile struct {
	Path    string
	Project string           // pre-extracted project name
	Agent   parser.AgentType // AgentClaude or AgentCodex
}

// DiscoverClaudeProjects finds all project directories under the
// Claude projects dir and returns their JSONL session files.
func DiscoverClaudeProjects(projectsDir string) []DiscoveredFile {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil
	}

	var files []DiscoveredFile
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projDir := filepath.Join(projectsDir, entry.Name())
		sessionFiles, err := os.ReadDir(projDir)
		if err != nil {
			continue
		}

		for _, sf := range sessionFiles {
			if sf.IsDir() {
				continue
			}
			name := sf.Name()
			if !strings.HasSuffix(name, ".jsonl") {
				continue
			}
			stem := strings.TrimSuffix(name, ".jsonl")
			if strings.HasPrefix(stem, "agent-") {
				continue
			}
			files = append(files, DiscoveredFile{
				Path:    filepath.Join(projDir, name),
				Project: entry.Name(),
				Agent:   parser.AgentClaude,
			})
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files
}

// DiscoverCodexSessions finds all JSONL files under the Codex
// sessions dir (year/month/day structure).
func DiscoverCodexSessions(sessionsDir string) []DiscoveredFile {
	var files []DiscoveredFile

	walkCodexDayDirs(sessionsDir, func(dayPath string) bool {
		entries, err := os.ReadDir(dayPath)
		if err != nil {
			return true
		}
		for _, sf := range entries {
			if sf.IsDir() {
				continue
			}
			if !strings.HasSuffix(sf.Name(), ".jsonl") {
				continue
			}
			files = append(files, DiscoveredFile{
				Path:  filepath.Join(dayPath, sf.Name()),
				Agent: parser.AgentCodex,
			})
		}
		return true
	})

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files
}

// FindClaudeSourceFile finds the original JSONL file for a Claude
// session ID by searching all project directories.
func FindClaudeSourceFile(
	projectsDir, sessionID string,
) string {
	if !isValidSessionID(sessionID) {
		return ""
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return ""
	}

	target := sessionID + ".jsonl"
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(
			projectsDir, entry.Name(), target,
		)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// FindCodexSourceFile finds a Codex session file by UUID.
// Searches the year/month/day directory structure for files matching
// rollout-{timestamp}-{uuid}.jsonl.
func FindCodexSourceFile(sessionsDir, sessionID string) string {
	if !isValidSessionID(sessionID) {
		return ""
	}

	var result string
	walkCodexDayDirs(sessionsDir, func(dayPath string) bool {
		if result != "" {
			return false
		}
		entries, err := os.ReadDir(dayPath)
		if err != nil {
			return true
		}
		for _, f := range entries {
			if f.IsDir() {
				continue
			}
			name := f.Name()
			if !strings.HasPrefix(name, "rollout-") ||
				!strings.HasSuffix(name, ".jsonl") {
				continue
			}
			if extractUUIDFromRollout(name) == sessionID {
				result = filepath.Join(dayPath, name)
				return false
			}
		}
		return true
	})
	return result
}

// walkCodexDayDirs traverses a Codex sessions directory with
// year/month/day structure, calling fn for each valid day directory.
// fn returns false to stop traversal.
func walkCodexDayDirs(
	root string, fn func(dayPath string) bool,
) {
	years, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, year := range years {
		if !year.IsDir() || !isDigits(year.Name()) {
			continue
		}
		yearPath := filepath.Join(root, year.Name())
		months, err := os.ReadDir(yearPath)
		if err != nil {
			continue
		}
		for _, month := range months {
			if !month.IsDir() || !isDigits(month.Name()) {
				continue
			}
			monthPath := filepath.Join(yearPath, month.Name())
			days, err := os.ReadDir(monthPath)
			if err != nil {
				continue
			}
			for _, day := range days {
				if !day.IsDir() || !isDigits(day.Name()) {
					continue
				}
				if !fn(filepath.Join(monthPath, day.Name())) {
					return
				}
			}
		}
	}
}

// extractUUIDFromRollout extracts the UUID from a Codex filename
// like rollout-{timestamp}-{uuid}.jsonl using regex matching on the
// standard 8-4-4-4-12 hex format.
func extractUUIDFromRollout(filename string) string {
	stem := strings.TrimSuffix(filename, ".jsonl")
	match := uuidRe.FindStringSubmatch(stem)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isValidSessionID(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		if !isAlphanumOrDashUnderscore(c) {
			return false
		}
	}
	return true
}

func isAlphanumOrDashUnderscore(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_'
}
