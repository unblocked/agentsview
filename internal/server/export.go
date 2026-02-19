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

	req, err := http.NewRequestWithContext(ctx, "POST",
		apiURL,
		strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("creating gist request: %w", err)
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "agentsv")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
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
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating validation request: %w", err)
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "agentsv")

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

type exportData struct {
	Project      string
	Agent        string
	MessageCount int
	StartedAt    string
	Messages     []exportMessage
}

type exportMessage struct {
	RoleClass    string
	ExtraClass   string
	Role         string
	Timestamp    string
	ContentHTML  template.HTML
}

var exportTmpl = template.Must(
	template.New("export").Parse(exportTemplateStr))

const exportTemplateStr = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Project}} - Agent Session</title>
<style>
:root {
  --bg: #0d1117; --surface: #161b22; --border: #30363d;
  --text: #e6edf3; --text-muted: #8b949e;
  --accent: #58a6ff; --agent-accent: #9d7cd8;
  --user-bg: #1c2128; --assistant-bg: #1a1f26;
  --tool-bg: #1a2332; --thinking-bg: #1f1a24;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
  background: var(--bg); color: var(--text); line-height: 1.5;
}
header {
  background: var(--surface); border-bottom: 1px solid var(--border);
  padding: 16px 24px; position: sticky; top: 0; z-index: 100;
}
.header-content {
  max-width: 900px; margin: 0 auto;
  display: flex; align-items: center;
  justify-content: space-between; gap: 12px;
}
h1 { font-size: 1.1rem; font-weight: 600; }
.session-meta {
  font-size: 0.8rem; color: var(--text-muted);
  display: flex; gap: 12px;
}
main { max-width: 900px; margin: 0 auto; padding: 24px; }
.messages { display: flex; flex-direction: column; gap: 16px; }
.message {
  padding: 16px; border-radius: 8px;
  border: 1px solid var(--border);
}
.message.user {
  background: var(--user-bg); border-left: 3px solid var(--accent);
}
.message.assistant {
  background: var(--assistant-bg);
  border-left: 3px solid var(--agent-accent);
}
.message-header {
  display: flex; justify-content: space-between;
  margin-bottom: 8px; font-size: 0.8rem;
}
.message-role {
  font-weight: 600; text-transform: uppercase;
  letter-spacing: 0.5px;
}
.message.user .message-role { color: var(--accent); }
.message.assistant .message-role { color: var(--agent-accent); }
.message-time { color: var(--text-muted); }
.message-content {
  white-space: pre-wrap; word-break: break-word; font-size: 0.9rem;
}
.message-content pre {
  background: var(--bg); padding: 12px;
  border-radius: 6px; overflow-x: auto; margin: 12px 0;
}
.thinking-block {
  background: var(--thinking-bg);
  border-left: 2px solid #8b5cf6;
  padding: 12px; margin: 8px 0; border-radius: 4px;
  font-style: italic; color: var(--text-muted); display: none;
}
.thinking-label {
  font-size: 0.75rem; font-weight: 600; color: #8b5cf6;
  text-transform: uppercase; margin-bottom: 4px; font-style: normal;
}
.message.thinking-only { display: none; }
#thinking-toggle:checked ~ main .thinking-block { display: block; }
#thinking-toggle:checked ~ main .message.thinking-only {
  display: block;
}
.tool-block {
  background: var(--tool-bg);
  border-left: 2px solid #d29922;
  padding: 8px 12px; margin: 8px 0; border-radius: 4px;
  font-size: 0.85rem;
}
#sort-toggle:checked ~ main .messages {
  flex-direction: column-reverse;
}
.toggle-input { position: absolute; opacity: 0; pointer-events: none; }
.toggle-label {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 6px 12px; background: #21262d;
  border: 1px solid var(--border); border-radius: 6px;
  color: var(--text); cursor: pointer; font-size: 0.85rem;
}
.toggle-input:checked + .toggle-label {
  background: #388bfd; border-color: var(--accent);
}
footer {
  max-width: 900px; margin: 40px auto; padding: 16px 24px;
  border-top: 1px solid var(--border); font-size: 0.8rem;
  color: var(--text-muted); text-align: center;
}
footer a { color: var(--accent); text-decoration: none; }
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
  </div>
</div>
<div style="display:flex;gap:12px">
  <label for="thinking-toggle" class="toggle-label">Thinking</label>
  <label for="sort-toggle" class="toggle-label">Newest first</label>
</div>
</div>
</header>
<main><div class="messages">
{{- range .Messages}}
<div class="message {{.RoleClass}}{{.ExtraClass}}"><div class="message-header"><span class="message-role">{{.Role}}</span><span class="message-time">{{.Timestamp}}</span></div><div class="message-content">{{.ContentHTML}}</div></div>
{{- end}}
</div></main>
<footer>Exported from <a href="https://github.com/wesm/agentsview">Agent Session Viewer</a></footer>
</body></html>`

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

	data := exportData{
		Project:      session.Project,
		Agent:        agentDisplay,
		MessageCount: session.MessageCount,
		StartedAt:    startedAt,
		Messages:     make([]exportMessage, len(msgs)),
	}

	for i, m := range msgs {
		roleClass := "unknown"
		if m.Role == "user" || m.Role == "assistant" {
			roleClass = m.Role
		}
		extraClass := ""
		if m.Role == "assistant" && isThinkingOnly(m.Content) {
			extraClass = " thinking-only"
		}

		data.Messages[i] = exportMessage{
			RoleClass:   roleClass,
			ExtraClass:  extraClass,
			Role:        m.Role,
			Timestamp:   formatTimestamp(m.Timestamp),
			ContentHTML: template.HTML(formatContentForExport(m.Content)),
		}
	}

	var b strings.Builder
	if err := exportTmpl.Execute(&b, data); err != nil {
		return fmt.Sprintf("template error: %s", err)
	}
	return b.String()
}

var (
	codeBlockRe  = regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```")
	inlineCodeRe = regexp.MustCompile("`([^`]+)`")
	thinkingRe   = regexp.MustCompile(
		`(?s)\[Thinking\]\n?(.*?)(?:\n\[|\n\n\[|$)`)
	toolBlockRe = regexp.MustCompile(
		`(?s)\[(Tool|Read|Write|Edit|Bash|Glob|Grep|Task|` +
			`Question|Todo List|Entering Plan Mode|` +
			`Exiting Plan Mode)([^\]]*)\](.*?)(?:\n\[|\n\n|$)`)
)

func formatContentForExport(text string) string {
	s := html.EscapeString(text)
	s = codeBlockRe.ReplaceAllString(s, "<pre><code>$2</code></pre>")
	s = inlineCodeRe.ReplaceAllString(s, "<code>$1</code>")
	s = thinkingRe.ReplaceAllString(s,
		`<div class="thinking-block">`+
			`<div class="thinking-label">Thinking</div>$1</div>`)
	s = toolBlockRe.ReplaceAllString(s,
		`<div class="tool-block">[$1$2]$3</div>`)
	return s
}

func isThinkingOnly(content string) bool {
	if content == "" {
		return false
	}
	without := thinkingRe.ReplaceAllString(content, "")
	return strings.TrimSpace(without) == ""
}

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
	return t.Format("2006-01-02 15:04:05")
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
