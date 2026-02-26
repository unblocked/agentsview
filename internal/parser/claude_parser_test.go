package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wesm/agentsview/internal/testjsonl"
)

func runClaudeParserTest(t *testing.T, fileName, content string) (ParsedSession, []ParsedMessage) {
	t.Helper()
	if fileName == "" {
		fileName = "test.jsonl"
	}
	path := createTestFile(t, fileName, content)
	results, err := ParseClaudeSession(path, "my_app", "local")
	require.NoError(t, err)
	require.NotEmpty(t, results)
	return results[0].Session, results[0].Messages
}

func TestParseClaudeSession_Basic(t *testing.T) {
	content := loadFixture(t, "claude/valid_session.jsonl")
	sess, msgs := runClaudeParserTest(t, "test.jsonl", content)

	assertMessageCount(t, len(msgs), 4)
	assertMessageCount(t, sess.MessageCount, 4)
	assertSessionMeta(t, &sess, "test", "my_app", AgentClaude)
	assert.Equal(t, "Fix the login bug", sess.FirstMessage)

	assertMessage(t, msgs[0], RoleUser, "")
	assertMessage(t, msgs[1], RoleAssistant, "")
	assert.True(t, msgs[1].HasToolUse)
	assertToolCalls(t, msgs[1].ToolCalls, []ParsedToolCall{{ToolUseID: "toolu_1", ToolName: "Read", Category: "Read", InputJSON: `{"file_path":"src/auth.ts"}`}})
	assert.Equal(t, 0, msgs[0].Ordinal)
	assert.Equal(t, 1, msgs[1].Ordinal)
}

func TestParseClaudeSession_HyphenatedFilename(t *testing.T) {
	content := loadFixture(t, "claude/valid_session.jsonl")
	sess, _ := runClaudeParserTest(t, "my-test-session.jsonl", content)
	assert.Equal(t, "my-test-session", sess.ID)
}

func TestParseClaudeSession_EdgeCases(t *testing.T) {
	t.Run("empty file", func(t *testing.T) {
		sess, msgs := runClaudeParserTest(t, "test.jsonl", "")
		assert.Empty(t, msgs)
		assert.Equal(t, 0, sess.MessageCount)
	})

	t.Run("skips blank content", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserJSON("", tsZero),
			testjsonl.ClaudeUserJSON("  ", tsZeroS1),
			testjsonl.ClaudeUserJSON("actual message", tsZeroS2),
		)
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, 1, sess.MessageCount)
	})

	t.Run("truncates long first message", func(t *testing.T) {
		content := testjsonl.ClaudeUserJSON(generateLargeString(400), tsZero) + "\n"
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, 303, len(sess.FirstMessage))
	})

	t.Run("skips invalid JSON lines", func(t *testing.T) {
		content := "not valid json\n" +
			testjsonl.ClaudeUserJSON("hello", tsZero) + "\n" +
			"also not valid\n"
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, 1, sess.MessageCount)
	})

	t.Run("malformed UTF-8", func(t *testing.T) {
		badUTF8 := `{"type":"user","timestamp":"` + tsZeroS1 + `","message":{"content":"bad ` + string([]byte{0xff, 0xfe}) + `"}}` + "\n"
		content := testjsonl.ClaudeUserJSON("valid message", tsZero) + "\n" + badUTF8
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.GreaterOrEqual(t, sess.MessageCount, 1)
	})

	t.Run("very large message", func(t *testing.T) {
		content := testjsonl.ClaudeUserJSON(generateLargeString(1024*1024), tsZero) + "\n"
		_, msgs := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, 1024*1024, msgs[0].ContentLength)
	})

	t.Run("skips empty lines in file", func(t *testing.T) {
		content := "\n\n" +
			testjsonl.ClaudeUserJSON("msg1", tsZero) +
			"\n   \n\t\n" +
			testjsonl.ClaudeAssistantJSON([]map[string]any{{"type": "text", "text": "reply"}}, tsZeroS1) +
			"\n\n"
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, 2, sess.MessageCount)
	})

	t.Run("skips partial/truncated JSON", func(t *testing.T) {
		content := testjsonl.ClaudeUserJSON("first", tsZero) + "\n" +
			`{"type":"user","truncated` + "\n" +
			testjsonl.ClaudeAssistantJSON([]map[string]any{{"type": "text", "text": "last"}}, tsZeroS2) + "\n"
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, 2, sess.MessageCount)
	})
}

func TestParseClaudeSession_SystemMessages(t *testing.T) {
	t.Run("isMeta user messages tagged as system", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeMetaUserJSON("meta context", tsZero, true, false),
			testjsonl.ClaudeUserJSON("real question", tsZeroS1),
		)
		sess, msgs := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, 2, sess.MessageCount)
		assert.Equal(t, 1, sess.UserMessageCount)
		assert.Equal(t, RoleSystem, msgs[0].Role)
		assert.Equal(t, RoleUser, msgs[1].Role)
		assert.Equal(t, "real question", sess.FirstMessage)
	})

	t.Run("isCompactSummary user messages tagged as system", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeMetaUserJSON("summary of prior turns", tsZero, false, true),
			testjsonl.ClaudeUserJSON("actual prompt", tsZeroS1),
		)
		sess, msgs := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, 2, sess.MessageCount)
		assert.Equal(t, 1, sess.UserMessageCount)
		assert.Equal(t, RoleSystem, msgs[0].Role)
		assert.Equal(t, RoleUser, msgs[1].Role)
		assert.Equal(t, "actual prompt", sess.FirstMessage)
	})

	t.Run("content-heuristic system messages tagged as system", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserJSON("This session is being continued from a previous conversation.", tsZero),
			testjsonl.ClaudeUserJSON("[Request interrupted by user]", tsZeroS1),
			testjsonl.ClaudeUserJSON("<task-notification>data</task-notification>", tsZeroS2),
			testjsonl.ClaudeUserJSON("<command-message>x</command-message>", "2024-01-01T00:00:03Z"),
			testjsonl.ClaudeUserJSON("<command-name>commit</command-name>", "2024-01-01T00:00:04Z"),
			testjsonl.ClaudeUserJSON("<local-command-result>ok</local-command-result>", "2024-01-01T00:00:05Z"),
			testjsonl.ClaudeUserJSON("Stop hook feedback: rejected", "2024-01-01T00:00:06Z"),
			testjsonl.ClaudeUserJSON("real user message", "2024-01-01T00:00:07Z"),
		)
		sess, msgs := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, 8, sess.MessageCount)
		assert.Equal(t, 1, sess.UserMessageCount)
		for i := 0; i < 7; i++ {
			assert.Equal(t, RoleSystem, msgs[i].Role, "msgs[%d] should be system", i)
		}
		assert.Equal(t, RoleUser, msgs[7].Role)
		assert.Equal(t, "real user message", msgs[7].Content)
		assert.Equal(t, "real user message", sess.FirstMessage)
	})

	t.Run("assistant with system-like content not tagged as system", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserJSON("hello", tsZero),
			testjsonl.ClaudeAssistantJSON([]map[string]any{
				{"type": "text", "text": "This session is being continued from a previous conversation."},
			}, tsZeroS1),
		)
		sess, msgs := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, 2, sess.MessageCount)
		assert.Equal(t, RoleUser, msgs[0].Role)
		assert.Equal(t, RoleAssistant, msgs[1].Role)
	})

	t.Run("firstMsg from first non-system user message", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeMetaUserJSON("context data", tsZero, true, false),
			testjsonl.ClaudeUserJSON("This session is being continued from a previous conversation.", tsZeroS1),
			testjsonl.ClaudeUserJSON("Fix the auth bug", tsZeroS2),
		)
		sess, msgs := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, 3, sess.MessageCount)
		assert.Equal(t, 1, sess.UserMessageCount)
		assert.Equal(t, RoleSystem, msgs[0].Role)
		assert.Equal(t, RoleSystem, msgs[1].Role)
		assert.Equal(t, RoleUser, msgs[2].Role)
		assert.Equal(t, "Fix the auth bug", sess.FirstMessage)
	})
}

func TestParseClaudeSession_ParentSessionID(t *testing.T) {
	t.Run("sessionId != fileId sets ParentSessionID", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserWithSessionIDJSON("hello", tsZero, "parent-uuid"),
			testjsonl.ClaudeAssistantJSON([]map[string]any{
				{"type": "text", "text": "hi"},
			}, tsZeroS1),
		)
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, "parent-uuid", sess.ParentSessionID)
	})

	t.Run("sessionId == fileId yields empty ParentSessionID", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserWithSessionIDJSON("hello", tsZero, "test"),
		)
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Empty(t, sess.ParentSessionID)
	})

	t.Run("no sessionId field yields empty ParentSessionID", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserJSON("hello", tsZero),
		)
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Empty(t, sess.ParentSessionID)
	})

	t.Run("transcript path reference sets ParentSessionID", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserWithTranscriptRefJSON(
				"Implement the following plan:",
				tsZero,
				"test",
				"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			),
			testjsonl.ClaudeAssistantJSON([]map[string]any{
				{"type": "text", "text": "On it."},
			}, tsZeroS1),
		)
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", sess.ParentSessionID)
	})

	t.Run("transcript ref to self is ignored", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserWithTranscriptRefJSON(
				"Continued session",
				tsZero,
				"test",
				"test",
			),
		)
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Empty(t, sess.ParentSessionID)
	})

	t.Run("sessionId parent takes precedence over transcript ref", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserWithTranscriptRefJSON(
				"Implement plan",
				tsZero,
				"session-parent-via-sid",
				"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			),
		)
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		// sessionId != fileId -> sessionId parent wins
		assert.Equal(t, "session-parent-via-sid", sess.ParentSessionID)
	})
}

func TestParseClaudeSession_TokenUsage(t *testing.T) {
	t.Run("sums token usage from assistant messages", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserJSON("hello", tsZero),
			testjsonl.ClaudeAssistantWithUsageJSON(
				[]map[string]string{{"type": "text", "text": "hi"}},
				tsZeroS1, "msg-1",
				100, 50, 10, 200,
			),
			testjsonl.ClaudeUserJSON("another question", tsZeroS2),
			testjsonl.ClaudeAssistantWithUsageJSON(
				[]map[string]string{{"type": "text", "text": "answer"}},
				tsEarly, "msg-2",
				150, 75, 20, 300,
			),
		)
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, int64(250), sess.InputTokens)
		assert.Equal(t, int64(125), sess.OutputTokens)
		assert.Equal(t, int64(30), sess.CacheCreationInputTokens)
		assert.Equal(t, int64(500), sess.CacheReadInputTokens)
	})

	t.Run("deduplicates streaming lines by messageId", func(t *testing.T) {
		// Simulate streaming: first line has partial usage,
		// second line has final usage for the same message ID.
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserJSON("hello", tsZero),
			testjsonl.ClaudeAssistantWithUsageJSON(
				[]map[string]string{{"type": "text", "text": "partial"}},
				tsZeroS1, "msg-1",
				50, 0, 5, 100,
			),
			testjsonl.ClaudeAssistantWithUsageJSON(
				[]map[string]string{{"type": "text", "text": "complete"}},
				tsZeroS2, "msg-1",
				100, 50, 10, 200,
			),
		)
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		// Should use only the last entry for msg-1.
		assert.Equal(t, int64(100), sess.InputTokens)
		assert.Equal(t, int64(50), sess.OutputTokens)
		assert.Equal(t, int64(10), sess.CacheCreationInputTokens)
		assert.Equal(t, int64(200), sess.CacheReadInputTokens)
	})

	t.Run("zero tokens when no usage data", func(t *testing.T) {
		content := testjsonl.JoinJSONL(
			testjsonl.ClaudeUserJSON("hello", tsZero),
			testjsonl.ClaudeAssistantJSON(
				[]map[string]string{{"type": "text", "text": "hi"}},
				tsZeroS1,
			),
		)
		sess, _ := runClaudeParserTest(t, "test.jsonl", content)
		assert.Equal(t, int64(0), sess.InputTokens)
		assert.Equal(t, int64(0), sess.OutputTokens)
		assert.Equal(t, int64(0), sess.CacheCreationInputTokens)
		assert.Equal(t, int64(0), sess.CacheReadInputTokens)
	})
}

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
