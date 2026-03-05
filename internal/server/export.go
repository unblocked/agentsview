package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/wesm/agentsview/internal/db"
)

// getSessionWithMessages fetches a session and its messages by ID,
// writing appropriate HTTP errors on failure. Returns false if the
// response has already been written.
func (s *Server) getSessionWithMessages(
	w http.ResponseWriter, r *http.Request,
) (*db.Session, []db.Message, bool) {
	id := r.PathValue("id")
	session, err := s.db.GetSession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return nil, nil, false
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return nil, nil, false
	}

	msgs, err := s.db.GetAllMessages(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return nil, nil, false
	}
	return session, msgs, true
}

func (s *Server) handleExportSession(
	w http.ResponseWriter, r *http.Request,
) {
	session, msgs, ok := s.getSessionWithMessages(w, r)
	if !ok {
		return
	}

	htmlContent := generateExportHTML(session, msgs)
	filename := sanitizeFilename(
		session.Project + "-" + formatDateShort(session.StartedAt) + ".html",
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set(
		"Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, filename),
	)
	_, _ = io.WriteString(w, htmlContent)
}

func (s *Server) handlePublishSession(
	w http.ResponseWriter, r *http.Request,
) {
	token := s.githubToken()
	if token == "" {
		writeError(w, http.StatusUnauthorized,
			"GitHub token not configured")
		return
	}

	session, msgs, ok := s.getSessionWithMessages(w, r)
	if !ok {
		return
	}

	htmlContent := generateExportHTML(session, msgs)
	filename := session.Project + "-" +
		formatDateShort(session.StartedAt) + ".html"

	first := ""
	if session.FirstMessage != nil {
		first = truncateStr(*session.FirstMessage, 100)
	}
	description := fmt.Sprintf("Agent session: %s - %s",
		session.Project, first)

	gist, err := createGist(
		r.Context(), token, filename, description, htmlContent,
	)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if gist.ID == "" || gist.HTMLURL == "" {
		writeError(w, http.StatusBadGateway,
			"GitHub API returned incomplete gist data")
		return
	}
	encoded := url.PathEscape(filename)
	rawURL := fmt.Sprintf(
		"https://gist.githubusercontent.com/%s/%s/raw/%s",
		gist.Owner.Login, gist.ID, encoded,
	)
	viewURL := "https://htmlpreview.github.io/?" + rawURL

	writeJSON(w, http.StatusOK, map[string]any{
		"gist_id":  gist.ID,
		"gist_url": gist.HTMLURL,
		"view_url": viewURL,
		"raw_url":  rawURL,
	})
}

func (s *Server) handleGetGithubConfig(
	w http.ResponseWriter, r *http.Request,
) {
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": s.githubToken() != "",
	})
}

func (s *Server) handleSetGithubConfig(
	w http.ResponseWriter, r *http.Request,
) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeError(w, http.StatusBadRequest, "token required")
		return
	}

	// Validate token
	username, err := validateGithubToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	s.mu.Lock()
	err = s.cfg.SaveGithubToken(token)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			"failed to save token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"username": username,
	})
}

// gistResponse represents the relevant fields from GitHub's
// Create Gist API response.
type gistResponse struct {
	ID      string `json:"id"`
	HTMLURL string `json:"html_url"`
	Owner   struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func createGist(
	ctx context.Context,
	token, filename, description, content string,
) (*gistResponse, error) {
	return createGistWithURL(
		ctx,
		"https://api.github.com/gists",
		token, filename, description, content,
	)
}

func createGistWithURL(
	ctx context.Context,
	apiURL, token, filename, description, content string,
) (*gistResponse, error) {
	payload, err := json.Marshal(map[string]any{
		"description": description,
		"public":      true,
		"files": map[string]any{
			filename: map[string]string{"content": content},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling gist payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		apiURL,
		strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("creating gist request: %w", err)
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "agentsview")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
		if err != nil {
			return nil, fmt.Errorf("github API error: %d: reading body: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("github API error: %d: %s",
			resp.StatusCode, string(body))
	}

	var result gistResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing github response: %w", err)
	}
	return &result, nil
}

func validateGithubToken(ctx context.Context, token string) (string, error) {
	return validateGithubTokenWithURL(
		ctx, "https://api.github.com/user", token,
	)
}

func validateGithubTokenWithURL(
	ctx context.Context,
	apiURL, token string,
) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating validation request: %w", err)
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "agentsview")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("validating token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return "", fmt.Errorf("invalid GitHub token")
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("GitHub API error: %d", resp.StatusCode)
	}

	var user struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", fmt.Errorf("parsing user response: %w", err)
	}
	return user.Login, nil
}

// --------------- Export data types ---------------

type exportData struct {
	Project      string
	Agent        string
	MessageCount int
	StartedAt    string
	Duration     string
	TokenStats   []exportModelStats
	TotalCost    string
	Items        []template.HTML
}

type exportModelStats struct {
	Model      string
	Input      string
	Output     string
	CacheRead  string
	CacheWrite string
	Cost       string
}

// --------------- Template ---------------

var exportTmpl = template.Must(
	template.New("export").Parse(exportTemplateStr))

const gearIconSVG = `<svg width="12" height="12" viewBox="0 0 16 16" fill="var(--accent-amber)">` +
	`<path d="M8 4.754a3.246 3.246 0 100 6.492 3.246 3.246 0 000-6.492zM5.754 8a2.246 2.246 0 114.492 0 2.246 2.246 0 01-4.492 0z"/>` +
	`<path d="M9.796 1.343c-.527-1.79-3.065-1.79-3.592 0l-.094.319a.873.873 0 01-1.255.52l-.292-.16c-1.64-.892-3.433.902-2.54 2.541l.159.292a.873.873 0 01-.52 1.255l-.319.094c-1.79.527-1.79 3.065 0 3.592l.319.094a.873.873 0 01.52 1.255l-.16.292c-.892 1.64.901 3.434 2.541 2.54l.292-.159a.873.873 0 011.255.52l.094.319c.527 1.79 3.065 1.79 3.592 0l.094-.319a.873.873 0 011.255-.52l.292.16c1.64.893 3.434-.902 2.54-2.541l-.159-.292a.873.873 0 01.52-1.255l.319-.094c1.79-.527 1.79-3.065 0-3.592l-.319-.094a.873.873 0 01-.52-1.255l.16-.292c.893-1.64-.902-3.433-2.541-2.54l-.292.159a.873.873 0 01-1.255-.52l-.094-.319zm-2.633.283a.909.909 0 011.674 0l.094.319a1.873 1.873 0 002.693 1.115l.291-.16a.909.909 0 011.18 1.18l-.159.292a1.873 1.873 0 001.116 2.692l.318.094a.909.909 0 010 1.674l-.319.094a1.873 1.873 0 00-1.115 2.693l.16.291a.909.909 0 01-1.18 1.18l-.292-.159a1.873 1.873 0 00-2.692 1.116l-.094.318a.909.909 0 01-1.674 0l-.094-.319a1.873 1.873 0 00-2.693-1.115l-.291.16a.909.909 0 01-1.18-1.18l.159-.292a1.873 1.873 0 00-1.116-2.692l-.318-.094a.909.909 0 010-1.674l.319-.094a1.873 1.873 0 001.115-2.693l-.16-.291a.909.909 0 011.18-1.18l.292.159a1.873 1.873 0 002.692-1.116l.094-.318z"/>` +
	`</svg>`

const unblockedIconSVG = `<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">` +
	`<rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>` +
	`<path d="M7 11V7a5 5 0 0 1 9.9-1"></path></svg>`

const exportTemplateStr = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Project}} - Agent Session</title>
<style>
:root {
  --bg-primary: #f7f7fa;
  --bg-surface: #ffffff;
  --bg-inset: #edeef3;
  --border-default: #dfe1e8;
  --border-muted: #e8eaf0;
  --text-primary: #1a1d26;
  --text-secondary: #5a6070;
  --text-muted: #8b92a0;
  --accent-blue: #2563eb;
  --accent-purple: #7c3aed;
  --accent-amber: #d97706;
  --accent-green: #16a34a;
  --user-bg: #eef2ff;
  --assistant-bg: #faf9ff;
  --system-bg: #f0fdf4;
  --thinking-bg: #f5f3ff;
  --tool-bg: #fffbf0;
  --code-bg: #1e1e2e;
  --code-text: #cdd6f4;
  --radius-sm: 4px;
  --radius-md: 6px;
  --font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI",
    "Noto Sans", Helvetica, Arial, sans-serif;
  --font-mono: "JetBrains Mono", "SF Mono", "Fira Code",
    "Fira Mono", Menlo, Consolas, monospace;
  color-scheme: light;
}
:root.dark {
  --bg-primary: #0c0c10;
  --bg-surface: #15151b;
  --bg-inset: #101015;
  --border-default: #2a2a35;
  --border-muted: #222230;
  --text-primary: #e2e4e9;
  --text-secondary: #9ca3af;
  --text-muted: #6b7280;
  --accent-blue: #60a5fa;
  --accent-purple: #a78bfa;
  --accent-amber: #fbbf24;
  --accent-green: #4ade80;
  --user-bg: #111827;
  --assistant-bg: #141220;
  --system-bg: #052e16;
  --thinking-bg: #1a1530;
  --tool-bg: #1a1508;
  --code-bg: #0d0d14;
  --code-text: #cdd6f4;
  color-scheme: dark;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: var(--font-sans);
  font-size: 14px;
  background: var(--bg-primary);
  color: var(--text-primary);
  line-height: 1.5;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}
button { background: none; border: none; cursor: pointer; font: inherit; }
header {
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-default);
  padding: 12px 24px;
  position: sticky; top: 0; z-index: 100;
}
.header-content {
  max-width: 900px; margin: 0 auto;
  display: flex; align-items: center;
  justify-content: space-between; gap: 12px;
}
h1 { font-size: 14px; font-weight: 600; }
.session-meta {
  font-size: 11px; color: var(--text-muted);
  display: flex; gap: 12px; flex-wrap: wrap;
}
.controls { display: flex; gap: 8px; }
.stats-bar {
  max-width: 900px; margin: 0 auto;
  padding: 8px 24px;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-muted);
}
.stats-table {
  width: 100%; border-collapse: collapse;
  font-size: 10px; font-variant-numeric: tabular-nums;
}
.stats-table th {
  color: var(--text-muted); font-weight: 500;
  text-align: right; padding: 1px 8px 2px 0;
}
.stats-table td {
  padding: 1px 8px 1px 0; color: var(--text-secondary);
}
.stats-table .col-model {
  text-align: left; white-space: nowrap;
  font-family: var(--font-mono); font-size: 9.5px;
  color: var(--text-muted);
}
.stats-table .col-num { text-align: right; }
.stats-table .col-cost {
  text-align: right; font-weight: 550;
  color: var(--text-primary);
}
main { max-width: 900px; margin: 0 auto; padding: 16px; }
.messages {
  display: flex; flex-direction: column; gap: 8px;
}

/* --- Messages --- */
.message {
  border-left: 4px solid;
  padding: 14px 20px;
  border-radius: 0 var(--radius-md) var(--radius-md) 0;
}
.message.user {
  background: var(--user-bg);
  border-left-color: var(--accent-blue);
}
.message.assistant {
  background: var(--assistant-bg);
  border-left-color: var(--accent-purple);
}
.message.system {
  background: var(--system-bg);
  border-left-color: var(--text-muted);
  border-left-style: dashed;
  opacity: 0.7;
}
.message.thinking-only { display: none; }
.message-header {
  display: flex; align-items: center; gap: 8px;
  margin-bottom: 10px;
}
.role-icon {
  width: 22px; height: 22px;
  border-radius: 50%;
  display: inline-flex; align-items: center; justify-content: center;
  font-size: 11px; font-weight: 700;
  color: white; flex-shrink: 0; line-height: 1;
}
.role-icon.user { background: var(--accent-blue); }
.role-icon.assistant { background: var(--accent-purple); }
.role-icon.system { background: var(--text-muted); }
.role-label {
  font-size: 13px; font-weight: 600;
  letter-spacing: 0.01em;
}
.role-label.user { color: var(--accent-blue); }
.role-label.assistant { color: var(--accent-purple); }
.role-label.system { color: var(--text-muted); }
.timestamp {
  font-size: 12px; color: var(--text-muted);
  margin-left: auto;
}
.message-body {
  display: flex; flex-direction: column; gap: 8px;
}
.text-content {
  font-size: 14px; line-height: 1.7;
  color: var(--text-primary);
  white-space: pre-wrap; word-break: break-word;
}
.text-content pre {
  background: var(--code-bg);
  color: var(--code-text);
  border-radius: var(--radius-md);
  padding: 12px 16px; overflow-x: auto;
  margin: 0.5em 0; white-space: pre;
}
.text-content code {
  font-family: var(--font-mono); font-size: 0.85em;
  background: var(--bg-inset);
  border: 1px solid var(--border-muted);
  border-radius: 4px; padding: 0.15em 0.4em;
}
.text-content pre code {
  background: none; border: none;
  padding: 0; font-size: 13px; color: inherit;
}

/* --- Thinking blocks --- */
.thinking-wrapper { display: none; }
details.thinking-block {
  border-left: 2px solid var(--accent-purple);
  background: var(--thinking-bg);
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
}
details.thinking-block > summary {
  list-style: none; cursor: pointer;
  display: flex; align-items: center; gap: 6px;
  padding: 6px 10px;
  font-size: 12px; font-weight: 600;
  color: var(--accent-purple);
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
}
details.thinking-block > summary::-webkit-details-marker { display: none; }
details.thinking-block > summary:hover {
  background: rgba(128,128,128,0.05);
}
.thinking-chevron {
  display: inline-block; font-size: 10px;
  transition: transform 0.15s; color: var(--text-muted);
}
details.thinking-block[open] > summary .thinking-chevron {
  transform: rotate(90deg);
}
.thinking-label { pointer-events: none; }
.thinking-content {
  padding: 8px 14px 12px;
  font-size: 13px; font-style: italic;
  color: var(--text-secondary);
  white-space: pre-wrap; word-wrap: break-word;
  line-height: 1.65;
  border-top: 1px solid var(--border-muted);
}

/* --- Tool groups --- */
.tool-group {
  border-left: 3px solid var(--accent-amber);
  background: var(--tool-bg);
  border-radius: 0 var(--radius-md) var(--radius-md) 0;
  padding: 8px 12px;
}
.tool-group.has-unblocked {
  border-left-color: var(--accent-purple);
  background: color-mix(in srgb, var(--accent-purple) 4%, var(--tool-bg, transparent));
}
.tool-group-header {
  display: flex; align-items: center; gap: 8px;
  margin-bottom: 6px;
}
.gear-icon {
  display: flex; align-items: center; flex-shrink: 0;
}
.unblocked-icon {
  display: flex; align-items: center; flex-shrink: 0;
  color: var(--accent-purple);
}
.group-label {
  font-size: 12px; font-weight: 600;
  color: var(--accent-amber);
}
.has-unblocked .group-label {
  color: var(--accent-purple);
}
.group-timestamp {
  font-size: 12px; color: var(--text-muted);
  margin-left: auto;
}
.tool-group-body {
  display: flex; flex-direction: column; gap: 2px;
}
.tool-group-body > details.tool-block {
  margin: 0; border-left: none; border-radius: 0;
}
.tool-group-body > details.tool-block.unblocked {
  border-left: 3px solid var(--accent-purple);
}

/* --- Tool blocks --- */
details.tool-block {
  border-left: 2px solid var(--accent-amber);
  background: var(--tool-bg);
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
}
details.tool-block.unblocked {
  border-left: 3px solid var(--accent-purple);
  background: color-mix(in srgb, var(--accent-purple) 6%, var(--tool-bg, transparent));
}
details.tool-block > summary {
  list-style: none; cursor: pointer;
  display: flex; align-items: center; gap: 6px;
  padding: 6px 10px;
  font-size: 12px; color: var(--text-secondary);
  min-width: 0;
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
}
details.tool-block > summary::-webkit-details-marker { display: none; }
details.tool-block > summary:hover {
  background: rgba(128,128,128,0.05);
  color: var(--text-primary);
}
.tool-chevron {
  display: inline-block; font-size: 10px;
  transition: transform 0.15s;
  flex-shrink: 0; color: var(--text-muted);
}
details.tool-block[open] > summary .tool-chevron {
  transform: rotate(90deg);
}
.unblocked-tag {
  font-size: 8px; font-weight: 750; letter-spacing: 0.06em;
  color: white; background: var(--accent-purple);
  padding: 1px 5px; border-radius: 3px;
  flex-shrink: 0; line-height: 1.5;
}
.tool-label {
  font-family: var(--font-mono); font-weight: 500;
  font-size: 11px; color: var(--accent-amber);
  white-space: nowrap; flex-shrink: 0;
}
details.tool-block.unblocked .tool-label {
  color: var(--accent-purple); font-weight: 600;
}
.tool-preview {
  font-family: var(--font-mono); font-size: 12px;
  color: var(--text-muted);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  min-width: 0;
}
.tool-result-size {
  font-family: var(--font-mono); font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap; flex-shrink: 0;
  margin-left: auto; opacity: 0.6;
}
.tool-content {
  padding: 8px 14px 10px;
  font-family: var(--font-mono); font-size: 12px;
  color: var(--text-secondary); line-height: 1.5;
  overflow-x: auto; border-top: 1px solid var(--border-muted);
  white-space: pre-wrap; margin: 0;
}
.tool-result-section {
  display: flex; align-items: center; gap: 8px;
  padding: 4px 14px;
  border-top: 1px solid var(--border-muted);
}
.result-label {
  font-size: 10px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.05em;
  color: var(--accent-green);
}
.tool-result {
  border-top: none; color: var(--text-muted);
  max-height: 400px; overflow-y: auto;
}

/* --- Controls --- */
#sort-toggle:checked ~ main .messages {
  flex-direction: column-reverse;
}
.toggle-input {
  position: absolute; opacity: 0; pointer-events: none;
}
.toggle-label {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 4px 10px;
  background: var(--bg-inset);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  cursor: pointer; font-size: 11px;
}
#thinking-toggle:checked ~ header label[for="thinking-toggle"],
#sort-toggle:checked ~ header label[for="sort-toggle"] {
  background: var(--accent-blue); color: #fff;
  border-color: var(--accent-blue);
}
#thinking-toggle:checked ~ main .thinking-wrapper {
  display: block;
}
#thinking-toggle:checked ~ main .message.thinking-only {
  display: block;
}
.theme-btn {
  padding: 4px 10px;
  background: var(--bg-inset);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  cursor: pointer; font-size: 11px;
  font-family: var(--font-sans);
}
.theme-btn:hover { background: var(--border-default); }
footer {
  max-width: 900px; margin: 40px auto; padding: 16px 24px;
  border-top: 1px solid var(--border-default);
  font-size: 11px; color: var(--text-muted);
  text-align: center;
}
footer a {
  color: var(--accent-blue); text-decoration: none;
}
footer a:hover { text-decoration: underline; }
</style>
</head>
<body>
<input type="checkbox" id="thinking-toggle" class="toggle-input">
<input type="checkbox" id="sort-toggle" class="toggle-input">
<header>
<div class="header-content">
<div>
  <h1>{{.Project}}</h1>
  <div class="session-meta">
    <span>{{.Agent}}</span>
    <span>{{.MessageCount}} messages</span>
    <span>{{.StartedAt}}</span>
    {{- if .Duration}}<span>{{.Duration}}</span>{{end}}
    {{- if .TotalCost}}<span>{{.TotalCost}}</span>{{end}}
  </div>
</div>
<div class="controls">
  <label for="thinking-toggle" class="toggle-label">Thinking</label>
  <label for="sort-toggle" class="toggle-label">Newest first</label>
  <button class="theme-btn" onclick="document.documentElement.classList.toggle('dark');this.textContent=document.documentElement.classList.contains('dark')?'Light':'Dark'">Dark</button>
</div>
</div>
</header>
{{- if .TokenStats}}
<div class="stats-bar">
<table class="stats-table">
<thead><tr>
  <th class="col-model">Model</th>
  <th class="col-num">Input</th>
  <th class="col-num">Output</th>
  <th class="col-num">Cache Read</th>
  <th class="col-num">Cache Write</th>
  <th class="col-cost">Cost</th>
</tr></thead>
<tbody>
{{- range .TokenStats}}
<tr>
  <td class="col-model">{{.Model}}</td>
  <td class="col-num">{{.Input}}</td>
  <td class="col-num">{{.Output}}</td>
  <td class="col-num">{{.CacheRead}}</td>
  <td class="col-num">{{.CacheWrite}}</td>
  <td class="col-cost">{{.Cost}}</td>
</tr>
{{- end}}
</tbody>
</table>
</div>
{{- end}}
<main><div class="messages">
{{- range .Items}}
{{.}}
{{- end}}
</div></main>
<footer>Exported from <a href="https://github.com/wesm/agentsview">agentsview</a></footer>
</body></html>`

// --------------- Pricing and stats ---------------

// modelPricing holds per-million-token pricing in USD.
type modelPricing struct {
	input      float64
	output     float64
	cacheWrite float64
	cacheRead  float64
}

var pricingTable = map[string]modelPricing{
	"claude-opus-4-6":            {5, 25, 6.25, 0.5},
	"claude-opus-4-5":            {5, 25, 6.25, 0.5},
	"claude-opus-4-5-20251101":   {5, 25, 6.25, 0.5},
	"claude-opus-4-1":            {15, 75, 18.75, 1.5},
	"claude-opus-4-0-20250514":   {15, 75, 18.75, 1.5},
	"claude-sonnet-4-6":          {3, 15, 3.75, 0.3},
	"claude-sonnet-4-5-20250514": {3, 15, 3.75, 0.3},
	"claude-sonnet-4-5-20250929": {3, 15, 3.75, 0.3},
	"claude-sonnet-4-0-20250514": {3, 15, 3.75, 0.3},
	"claude-sonnet-3-7-20250219": {3, 15, 3.75, 0.3},
	"claude-haiku-4-5-20251001":  {1, 5, 1.25, 0.1},
	"claude-3-5-haiku-20241022":  {0.8, 4, 1, 0.08},
	"claude-3-opus-20240229":     {15, 75, 18.75, 1.5},
	"claude-3-haiku-20240307":    {0.25, 1.25, 0.3, 0.03},
}

func findPricing(modelID string) *modelPricing {
	if p, ok := pricingTable[modelID]; ok {
		return &p
	}
	for key, p := range pricingTable {
		if strings.HasPrefix(modelID, key) {
			return &p
		}
	}
	families := []struct {
		needle string
		key    string
	}{
		{"opus-4-6", "claude-opus-4-6"},
		{"opus-4-5", "claude-opus-4-5"},
		{"opus-4-1", "claude-opus-4-1"},
		{"opus-4-0", "claude-opus-4-0-20250514"},
		{"sonnet-4-6", "claude-sonnet-4-6"},
		{"sonnet-4-5", "claude-sonnet-4-5-20250514"},
		{"sonnet-4-0", "claude-sonnet-4-0-20250514"},
		{"sonnet-3-7", "claude-sonnet-3-7-20250219"},
		{"haiku-4-5", "claude-haiku-4-5-20251001"},
		{"haiku-3-5", "claude-3-5-haiku-20241022"},
		{"opus-3", "claude-3-opus-20240229"},
		{"haiku-3", "claude-3-haiku-20240307"},
	}
	for _, f := range families {
		if strings.Contains(modelID, f.needle) {
			p := pricingTable[f.key]
			return &p
		}
	}
	return nil
}

func shortModelName(modelID string) string {
	name := strings.TrimPrefix(modelID, "claude-")
	// Strip date suffix like -20250514
	if len(name) > 9 && name[len(name)-9] == '-' {
		tail := name[len(name)-8:]
		allDigits := true
		for _, c := range tail {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			name = name[:len(name)-9]
		}
	}
	return name
}

func formatTokenCount(n int64) string {
	if n == 0 {
		return "0"
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatCost(usd float64) string {
	if usd < 0.01 {
		return "<$0.01"
	}
	if usd < 10 {
		return fmt.Sprintf("$%.2f", usd)
	}
	return fmt.Sprintf("$%.1f", usd)
}

func formatDurationMs(ms int64) string {
	if ms <= 0 {
		return ""
	}
	sec := ms / 1000
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	min := sec / 60
	s := sec % 60
	if min < 60 {
		return fmt.Sprintf("%dm %ds", min, s)
	}
	h := min / 60
	m := min % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

// calculateDurationMs computes total assistant response time
// from message timestamps.
func calculateDurationMs(msgs []db.Message) int64 {
	if len(msgs) < 2 {
		return 0
	}
	var totalMs int64
	var turnStart int64
	var lastNonUser int64
	inTurn := false

	for _, m := range msgs {
		t, ok := parseTimestamp(m.Timestamp)
		if !ok {
			continue
		}
		ts := t.UnixMilli()
		if m.Role == "user" {
			if inTurn && lastNonUser > 0 {
				totalMs += lastNonUser - turnStart
			}
			turnStart = ts
			lastNonUser = 0
			inTurn = true
		} else if inTurn {
			lastNonUser = ts
		}
	}
	if inTurn && lastNonUser > 0 {
		totalMs += lastNonUser - turnStart
	}
	return totalMs
}

type modelTokenUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

func buildTokenStats(
	session *db.Session,
) ([]exportModelStats, string) {
	if session.TokenUsageByModel == nil {
		return nil, ""
	}
	var byModel map[string]modelTokenUsage
	if err := json.Unmarshal(
		session.TokenUsageByModel, &byModel,
	); err != nil || len(byModel) == 0 {
		return nil, ""
	}

	var stats []exportModelStats
	var totalCost float64
	for modelID, usage := range byModel {
		if strings.HasPrefix(modelID, "<") {
			continue
		}
		var costStr string
		if p := findPricing(modelID); p != nil {
			mtok := 1_000_000.0
			cost := float64(usage.InputTokens)/mtok*p.input +
				float64(usage.OutputTokens)/mtok*p.output +
				float64(usage.CacheCreationInputTokens)/mtok*p.cacheWrite +
				float64(usage.CacheReadInputTokens)/mtok*p.cacheRead
			costStr = formatCost(cost)
			totalCost += cost
		} else {
			costStr = "\u2014"
		}
		stats = append(stats, exportModelStats{
			Model:      shortModelName(modelID),
			Input:      formatTokenCount(usage.InputTokens),
			Output:     formatTokenCount(usage.OutputTokens),
			CacheRead:  formatTokenCount(usage.CacheReadInputTokens),
			CacheWrite: formatTokenCount(usage.CacheCreationInputTokens),
			Cost:       costStr,
		})
	}
	var totalStr string
	if totalCost > 0 {
		totalStr = formatCost(totalCost)
	}
	return stats, totalStr
}

// --------------- Export generation ---------------

func generateExportHTML(
	session *db.Session, msgs []db.Message,
) string {
	agentDisplay := "Claude"
	if session.Agent == "codex" {
		agentDisplay = "Codex"
	}

	startedAt := ""
	if session.StartedAt != nil {
		startedAt = formatTimestamp(*session.StartedAt)
	}

	tokenStats, totalCost := buildTokenStats(session)
	durationMs := calculateDurationMs(msgs)

	data := exportData{
		Project:      session.Project,
		Agent:        agentDisplay,
		MessageCount: session.MessageCount,
		StartedAt:    startedAt,
		Duration:     formatDurationMs(durationMs),
		TokenStats:   tokenStats,
		TotalCost:    totalCost,
		Items:        buildExportItems(msgs),
	}

	var b strings.Builder
	if err := exportTmpl.Execute(&b, data); err != nil {
		return fmt.Sprintf("template error: %s", err)
	}
	return b.String()
}

// --------------- Display item grouping ---------------

var toolAliases = map[string]string{
	"exec_command":  "Bash",
	"shell_command": "Bash",
	"write_stdin":   "Bash",
	"shell":         "Bash",
	"apply_patch":   "Edit",
}

func applyToolAlias(name string) string {
	if alias, ok := toolAliases[name]; ok {
		return alias
	}
	return name
}

// isToolOnlyExport returns true if the message contains only
// thinking and tool blocks with no visible text.
func isToolOnlyExport(m db.Message) bool {
	if m.Role != "assistant" {
		return false
	}
	if !m.HasToolUse {
		return false
	}
	segs := extractExportSegments(m.Content)
	for _, s := range segs {
		if s.typ == "text" && strings.TrimSpace(s.content) != "" {
			return false
		}
	}
	return true
}

// buildExportItems groups messages into display items matching
// the transcript view: consecutive tool-only assistant messages
// become tool groups, others become individual message items.
func buildExportItems(msgs []db.Message) []template.HTML {
	var items []template.HTML
	var toolAcc []db.Message

	flushTools := func() {
		if len(toolAcc) == 0 {
			return
		}
		items = append(items,
			template.HTML(renderToolGroupItem(toolAcc)))
		toolAcc = nil
	}

	for _, m := range msgs {
		if isToolOnlyExport(m) {
			toolAcc = append(toolAcc, m)
		} else {
			flushTools()
			items = append(items,
				template.HTML(renderMessageItem(m)))
		}
	}
	flushTools()

	return items
}

// --------------- Render message items ---------------

func renderMessageItem(m db.Message) string {
	var b strings.Builder

	roleClass := m.Role
	if roleClass != "user" && roleClass != "assistant" &&
		roleClass != "system" {
		roleClass = "unknown"
	}
	extraClass := ""
	if m.Role == "assistant" && isThinkingOnly(m.Content) {
		extraClass = " thinking-only"
	}

	icon := "A"
	label := "Assistant"
	switch m.Role {
	case "user":
		icon = "U"
		label = "User"
	case "system":
		icon = "S"
		label = "System"
	}

	b.WriteString(fmt.Sprintf(
		`<div class="message %s%s">`, roleClass, extraClass))
	b.WriteString(`<div class="message-header">`)
	b.WriteString(fmt.Sprintf(
		`<span class="role-icon %s">%s</span>`, roleClass, icon))
	b.WriteString(fmt.Sprintf(
		`<span class="role-label %s">%s</span>`,
		roleClass, html.EscapeString(label)))
	b.WriteString(fmt.Sprintf(
		`<span class="timestamp">%s</span>`,
		html.EscapeString(formatTimestamp(m.Timestamp))))
	b.WriteString(`</div>`)
	b.WriteString(`<div class="message-body">`)
	b.WriteString(formatContentForExport(m.Content, m.ToolCalls))
	b.WriteString(`</div></div>`)

	return b.String()
}

// --------------- Render tool groups ---------------

type toolSegInfo struct {
	seg exportSegment
	tc  *db.ToolCall
}

func renderToolGroupItem(msgs []db.Message) string {
	var b strings.Builder

	// Collect all tool segments from grouped messages.
	var segments []toolSegInfo
	for i := range msgs {
		m := &msgs[i]
		segs := extractExportSegments(m.Content)
		tcIdx := 0
		for _, seg := range segs {
			if seg.typ != "tool" {
				continue
			}
			var tc *db.ToolCall
			if tcIdx < len(m.ToolCalls) {
				tc = &m.ToolCalls[tcIdx]
				tcIdx++
			}
			segments = append(segments, toolSegInfo{seg: seg, tc: tc})
		}
	}

	label := "1 tool call"
	if len(segments) != 1 {
		label = fmt.Sprintf("%d tool calls", len(segments))
	}

	hasUnblocked := false
	for _, s := range segments {
		if s.tc != nil &&
			strings.HasPrefix(s.tc.ToolName, "mcp__unblocked__") {
			hasUnblocked = true
			break
		}
	}

	groupClass := "tool-group"
	if hasUnblocked {
		groupClass += " has-unblocked"
	}

	timestamp := ""
	if len(msgs) > 0 {
		timestamp = formatTimestamp(msgs[0].Timestamp)
	}

	b.WriteString(fmt.Sprintf(`<div class="%s">`, groupClass))
	b.WriteString(`<div class="tool-group-header">`)
	if hasUnblocked {
		b.WriteString(`<span class="unblocked-icon">`)
		b.WriteString(unblockedIconSVG)
		b.WriteString(`</span>`)
	} else {
		b.WriteString(`<span class="gear-icon">`)
		b.WriteString(gearIconSVG)
		b.WriteString(`</span>`)
	}
	b.WriteString(fmt.Sprintf(
		`<span class="group-label">%s</span>`,
		html.EscapeString(label)))
	b.WriteString(fmt.Sprintf(
		`<span class="group-timestamp">%s</span>`,
		html.EscapeString(timestamp)))
	b.WriteString(`</div>`)
	b.WriteString(`<div class="tool-group-body">`)
	for _, s := range segments {
		b.WriteString(renderToolBlockHTML(s))
	}
	b.WriteString(`</div></div>`)

	return b.String()
}

// --------------- Render individual tool blocks ---------------

func renderToolBlockHTML(info toolSegInfo) string {
	var b strings.Builder

	toolName := info.seg.label
	isUnblocked := false
	preview := ""
	resultSize := ""
	resultContent := ""
	content := info.seg.content

	if info.tc != nil {
		// Use structured ToolCall data when available.
		if info.tc.ToolName != "" {
			toolName = applyToolAlias(info.tc.ToolName)
		}
		if strings.HasPrefix(info.tc.ToolName, "mcp__unblocked__") {
			isUnblocked = true
			parts := strings.Split(info.tc.ToolName, "__")
			if len(parts) >= 3 {
				toolName = strings.Join(parts[2:], "__")
			}
			if info.tc.InputJSON != "" {
				var input map[string]any
				if json.Unmarshal(
					[]byte(info.tc.InputJSON), &input,
				) == nil {
					if q, ok := input["query"].(string); ok {
						preview = q
					}
				}
			}
		}
		if info.tc.ResultContentLength > 0 {
			if info.tc.ResultContentLength > 1000 {
				resultSize = fmt.Sprintf("%.1fk chars",
					float64(info.tc.ResultContentLength)/1000)
			} else {
				resultSize = fmt.Sprintf("%d chars",
					info.tc.ResultContentLength)
			}
		}
		if info.tc.ResultContent != "" {
			resultContent = info.tc.ResultContent
		}
		// For Bash with multi-line commands, use full command.
		if info.tc.ToolName == "Bash" && info.tc.InputJSON != "" {
			var input map[string]any
			if json.Unmarshal(
				[]byte(info.tc.InputJSON), &input,
			) == nil {
				if cmd, ok := input["command"].(string); ok &&
					strings.Contains(cmd, "\n") {
					content = "$ " + cmd
				}
			}
		}
	}

	if preview == "" && content != "" {
		line := strings.SplitN(content, "\n", 2)[0]
		if len(line) > 100 {
			preview = line[:100]
		} else {
			preview = line
		}
	}

	blockClass := "tool-block"
	if isUnblocked {
		blockClass += " unblocked"
	}

	b.WriteString(fmt.Sprintf(`<details class="%s">`, blockClass))
	b.WriteString(`<summary>`)
	b.WriteString(`<span class="tool-chevron">&#9656;</span>`)
	if isUnblocked {
		b.WriteString(`<span class="unblocked-tag">UNBLOCKED</span>`)
	}
	b.WriteString(fmt.Sprintf(
		`<span class="tool-label">%s</span>`,
		html.EscapeString(toolName)))
	if preview != "" {
		b.WriteString(fmt.Sprintf(
			`<span class="tool-preview">%s</span>`,
			html.EscapeString(preview)))
	}
	if resultSize != "" {
		b.WriteString(fmt.Sprintf(
			`<span class="tool-result-size">%s</span>`,
			html.EscapeString(resultSize)))
	}
	b.WriteString(`</summary>`)

	if content != "" {
		b.WriteString(fmt.Sprintf(
			`<pre class="tool-content">%s</pre>`,
			html.EscapeString(content)))
	}
	if resultContent != "" {
		b.WriteString(
			`<div class="tool-result-section">` +
				`<span class="result-label">Result</span></div>`)
		b.WriteString(fmt.Sprintf(
			`<pre class="tool-content tool-result">%s</pre>`,
			html.EscapeString(resultContent)))
	}

	b.WriteString(`</details>`)
	return b.String()
}

// --------------- Segment-based content extraction ---------------

type exportSegment struct {
	typ     string // "text", "thinking", "tool", "code"
	content string
	label   string // tool name or code language
}

var (
	codeBlockRe  = regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```")
	inlineCodeRe = regexp.MustCompile("`([^`]+)`")
	// Terminators are in a capturing group so we can find where
	// the terminator starts and exclude it from the match span,
	// preventing it from being consumed.
	thinkingRe = regexp.MustCompile(
		`(?s)\[Thinking\]\n?(.*?)(\n\[|\n\n\[|$)`)
	toolBlockRe = regexp.MustCompile(
		`(?s)\[(Tool|Read|Write|Edit|Bash|Glob|Grep|` +
			`TaskCreate|TaskUpdate|TaskGet|TaskList|Task|` +
			`Skill|SendMessage|` +
			`Question|Todo List|Entering Plan Mode|` +
			`Exiting Plan Mode|exec_command|shell_command|` +
			`write_stdin|apply_patch|shell|parallel|` +
			`view_image|request_user_input|update_plan` +
			`)([^\]]*)\](.*?)(\n\[|\n\n|$)`)
)

type segMatch struct {
	start   int
	end     int
	segment exportSegment
}

// findAllNonConsuming finds all matches of re in text,
// using the terminator group start as the effective end so
// that \n[ terminators are not consumed and the next block
// can be found.
func findAllNonConsuming(
	re *regexp.Regexp, text string, terminatorGroup int,
) [][]int {
	var results [][]int
	pos := 0
	for pos < len(text) {
		m := re.FindStringSubmatchIndex(text[pos:])
		if m == nil {
			break
		}
		// Shift indices to absolute positions.
		for i := range m {
			if m[i] >= 0 {
				m[i] += pos
			}
		}
		results = append(results, m)

		// Advance past the match but before the terminator.
		tgStart := m[terminatorGroup*2]
		if tgStart >= 0 && tgStart > m[0] {
			pos = tgStart
		} else {
			pos = m[1]
		}
	}
	return results
}

func extractExportSegments(text string) []exportSegment {
	var matches []segMatch

	// thinkingRe groups: (1)=content, (2)=terminator
	for _, m := range findAllNonConsuming(thinkingRe, text, 2) {
		end := m[4] // start of terminator group
		if end < 0 {
			end = m[1]
		}
		matches = append(matches, segMatch{
			start: m[0],
			end:   end,
			segment: exportSegment{
				typ:     "thinking",
				content: strings.TrimSpace(text[m[2]:m[3]]),
			},
		})
	}

	// toolBlockRe groups: (1)=name, (2)=args, (3)=content, (4)=terminator
	for _, m := range findAllNonConsuming(toolBlockRe, text, 4) {
		end := m[8] // start of terminator group (group 4)
		if end < 0 {
			end = m[1]
		}
		toolName := text[m[2]:m[3]]
		toolArgs := strings.TrimSpace(text[m[4]:m[5]])
		label := toolName
		if toolArgs != "" {
			label = toolName + toolArgs
		}
		matches = append(matches, segMatch{
			start: m[0],
			end:   end,
			segment: exportSegment{
				typ:     "tool",
				content: strings.TrimSpace(text[m[6]:m[7]]),
				label:   label,
			},
		})
	}

	for _, m := range codeBlockRe.FindAllStringSubmatchIndex(text, -1) {
		start := m[0]
		inside := false
		for _, om := range matches {
			if start >= om.start && start < om.end {
				inside = true
				break
			}
		}
		if inside {
			continue
		}
		matches = append(matches, segMatch{
			start: m[0],
			end:   m[1],
			segment: exportSegment{
				typ:     "code",
				content: text[m[4]:m[5]],
				label:   text[m[2]:m[3]],
			},
		})
	}

	// Sort and deduplicate overlapping matches.
	sortSegMatches(matches)
	var deduped []segMatch
	lastEnd := 0
	for _, m := range matches {
		if m.start < lastEnd {
			continue
		}
		deduped = append(deduped, m)
		lastEnd = m.end
	}

	// Build segments with text gaps.
	var segments []exportSegment
	pos := 0
	for _, m := range deduped {
		if m.start > pos {
			gap := strings.TrimRight(text[pos:m.start], " \t\n")
			if gap != "" {
				segments = append(segments, exportSegment{
					typ:     "text",
					content: gap,
				})
			}
		}
		segments = append(segments, m.segment)
		pos = m.end
	}
	if pos < len(text) {
		tail := strings.TrimRight(text[pos:], " \t\n")
		if tail != "" {
			segments = append(segments, exportSegment{
				typ:     "text",
				content: tail,
			})
		}
	}

	return segments
}

func sortSegMatches(matches []segMatch) {
	for i := 1; i < len(matches); i++ {
		key := matches[i]
		j := i - 1
		for j >= 0 && matches[j].start > key.start {
			matches[j+1] = matches[j]
			j--
		}
		matches[j+1] = key
	}
}

// --------------- Content formatting for messages ---------------

// formatContentForExport renders message content as HTML segments
// for display inside a message body. Tool blocks use <details>
// for collapsibility; thinking blocks are wrapped in a
// thinking-wrapper that is shown/hidden by the thinking toggle.
func formatContentForExport(
	text string, toolCalls []db.ToolCall,
) string {
	if text == "" {
		return ""
	}
	segments := extractExportSegments(text)

	var b strings.Builder
	tcIdx := 0

	for _, seg := range segments {
		switch seg.typ {
		case "thinking":
			b.WriteString(`<div class="thinking-wrapper">`)
			b.WriteString(`<details class="thinking-block">`)
			b.WriteString(`<summary>`)
			b.WriteString(
				`<span class="thinking-chevron">&#9656;</span>`)
			b.WriteString(
				`<span class="thinking-label">Thinking</span>`)
			b.WriteString(`</summary>`)
			b.WriteString(fmt.Sprintf(
				`<div class="thinking-content">%s</div>`,
				html.EscapeString(seg.content)))
			b.WriteString(`</details></div>`)

		case "tool":
			var tc *db.ToolCall
			if tcIdx < len(toolCalls) {
				tc = &toolCalls[tcIdx]
				tcIdx++
			}
			b.WriteString(renderToolBlockHTML(toolSegInfo{
				seg: seg, tc: tc,
			}))

		case "code":
			b.WriteString(`<pre><code>`)
			b.WriteString(html.EscapeString(seg.content))
			b.WriteString(`</code></pre>`)

		case "text":
			escaped := html.EscapeString(seg.content)
			escaped = inlineCodeRe.ReplaceAllString(
				escaped, "<code>$1</code>")
			b.WriteString(fmt.Sprintf(
				`<div class="text-content">%s</div>`, escaped))
		}
	}

	return b.String()
}

func isThinkingOnly(content string) bool {
	if content == "" {
		return false
	}
	without := thinkingRe.ReplaceAllString(content, "")
	return strings.TrimSpace(without) == ""
}

// --------------- Timestamp and filename utilities ---------------

// parseTimestamp tries RFC3339Nano then RFC3339.
func parseTimestamp(ts string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
	}
	return t, err == nil
}

func formatTimestamp(ts string) string {
	if ts == "" {
		return ""
	}
	t, ok := parseTimestamp(ts)
	if !ok {
		return ts
	}
	return t.Format("Jan 2 at 3:04 PM")
}

func formatDateShort(ts *string) string {
	if ts == nil || *ts == "" {
		return "unknown"
	}
	t, ok := parseTimestamp(*ts)
	if !ok {
		return "unknown"
	}
	return t.Format("20060102")
}

func sanitizeFilename(name string) string {
	re := regexp.MustCompile(`[^\w.\-]`)
	return re.ReplaceAllString(name, "_")
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
