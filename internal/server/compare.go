package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type compareRequest struct {
	SessionIDsA []string `json:"session_ids_a"`
	SessionIDsB []string `json:"session_ids_b"`
}

func (s *Server) handleCompareGenerate(
	w http.ResponseWriter, r *http.Request,
) {
	var req compareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest,
			"invalid JSON body")
		return
	}

	if len(req.SessionIDsA) == 0 || len(req.SessionIDsB) == 0 {
		writeError(w, http.StatusBadRequest,
			"both session_ids_a and session_ids_b are required")
		return
	}

	stream, err := NewSSEStream(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			"streaming not supported")
		return
	}

	stream.SendJSON("status", map[string]string{
		"phase": "building prompt",
	})

	prompt, err := s.buildComparePrompt(r.Context(), req)
	if err != nil {
		log.Printf("compare prompt error: %v", err)
		stream.SendJSON("error", map[string]string{
			"message": fmt.Sprintf(
				"failed to build prompt: %v", err,
			),
		})
		return
	}

	stream.SendJSON("status", map[string]string{
		"phase": "generating comparison",
	})

	genCtx, cancel := context.WithTimeout(
		r.Context(), 10*time.Minute,
	)
	defer cancel()

	result, err := s.generateFunc(genCtx, "claude", prompt)
	if err != nil {
		log.Printf("compare generate error: %v", err)
		stream.SendJSON("error", map[string]string{
			"message": fmt.Sprintf(
				"generation failed: %v", err,
			),
		})
		return
	}

	if strings.TrimSpace(result.Content) == "" {
		stream.SendJSON("error", map[string]string{
			"message": "agent returned empty content",
		})
		return
	}

	stream.SendJSON("done", map[string]string{
		"content": result.Content,
	})
}

const maxCompareMessages = 50

func (s *Server) buildComparePrompt(
	ctx context.Context, req compareRequest,
) (string, error) {
	var b strings.Builder

	b.WriteString(
		"You are comparing two groups of AI coding sessions " +
			"to determine which produced better results.\n\n",
	)

	if err := s.writeGroupSummary(
		ctx, &b, "A", req.SessionIDsA,
	); err != nil {
		return "", err
	}

	if err := s.writeGroupSummary(
		ctx, &b, "B", req.SessionIDsB,
	); err != nil {
		return "", err
	}

	b.WriteString("## Instructions\n\n")
	b.WriteString("Compare these two session groups. Analyze:\n")
	b.WriteString("1. What task was each group trying to accomplish?\n")
	b.WriteString("2. What approach did each take?\n")
	b.WriteString("3. Which produced better results and why?\n")
	b.WriteString("4. Which was more efficient (cost, speed, token usage)?\n\n")
	b.WriteString("Provide a clear verdict on which group was better overall, with reasoning.\n")
	b.WriteString("Use markdown formatting for your response.\n")

	return b.String(), nil
}

func (s *Server) writeGroupSummary(
	ctx context.Context,
	b *strings.Builder,
	label string,
	sessionIDs []string,
) error {
	fmt.Fprintf(b, "## Session Group %s\n\n", label)

	var totalMessages int
	var totalInputTokens, totalOutputTokens int64
	var totalCacheWrite, totalCacheRead int64
	var sessionCount int

	for _, sid := range sessionIDs {
		sess, err := s.db.GetSession(ctx, sid)
		if err != nil {
			return fmt.Errorf("getting session %s: %w", sid, err)
		}
		if sess == nil {
			continue
		}
		sessionCount++
		totalMessages += sess.MessageCount
		totalInputTokens += sess.InputTokens
		totalOutputTokens += sess.OutputTokens
		totalCacheWrite += sess.CacheCreationInputTokens
		totalCacheRead += sess.CacheReadInputTokens

		fmt.Fprintf(b, "### Session: %s\n", sess.Project)
		fmt.Fprintf(b, "- Agent: %s\n", sess.Agent)
		fmt.Fprintf(b, "- Messages: %d\n", sess.MessageCount)
		fmt.Fprintf(b, "- User prompts: %d\n", sess.UserMessageCount)
		if sess.StartedAt != nil {
			fmt.Fprintf(b, "- Started: %s\n", *sess.StartedAt)
		}
		if sess.EndedAt != nil {
			fmt.Fprintf(b, "- Ended: %s\n", *sess.EndedAt)
		}
		fmt.Fprintf(b,
			"- Tokens: %d input, %d output\n",
			sess.InputTokens, sess.OutputTokens,
		)

		msgs, err := s.db.GetMessages(
			ctx, sid, 0, maxCompareMessages, true,
		)
		if err != nil {
			return fmt.Errorf(
				"getting messages for %s: %w", sid, err,
			)
		}

		if len(msgs) > 0 {
			b.WriteString("\n#### Key Messages:\n\n")
			for _, msg := range msgs {
				if msg.Role != "user" && msg.Role != "assistant" {
					continue
				}
				content := msg.Content
				if len(content) > 500 {
					content = content[:500] + "..."
				}
				fmt.Fprintf(b, "**[%s]**: %s\n\n",
					msg.Role, content,
				)
			}
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(b,
		"**Group %s Summary**: %d sessions, %d messages, "+
			"%d input tokens, %d output tokens\n\n",
		label, sessionCount, totalMessages,
		totalInputTokens, totalOutputTokens,
	)

	return nil
}
