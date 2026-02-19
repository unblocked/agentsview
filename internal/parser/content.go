package parser

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// ExtractTextContent extracts readable text from message content.
// content can be a string or a JSON array of blocks.
func ExtractTextContent(content gjson.Result) (string, bool, bool) {
	hasThinking := false
	hasToolUse := false

	if content.Type == gjson.String {
		return content.Str, false, false
	}

	if !content.IsArray() {
		return "", false, false
	}

	var parts []string
	content.ForEach(func(_, block gjson.Result) bool {
		blockType := block.Get("type").Str
		switch blockType {
		case "text":
			text := block.Get("text").Str
			if text != "" {
				parts = append(parts, text)
			}
		case "thinking":
			thinking := block.Get("thinking").Str
			if thinking != "" {
				hasThinking = true
				parts = append(parts, "[Thinking]\n"+thinking)
			}
		case "tool_use":
			hasToolUse = true
			parts = append(parts, formatToolUse(block))
		}
		return true
	})

	return strings.Join(parts, "\n"), hasThinking, hasToolUse
}

var todoIcons = map[string]string{
	"completed":   "✓",
	"in_progress": "→",
	"pending":     "○",
}

func formatToolUse(block gjson.Result) string {
	name := block.Get("name").Str
	input := block.Get("input")

	switch name {
	case "AskUserQuestion":
		return formatAskUserQuestion(name, input)
	case "TodoWrite":
		return formatTodoWrite(input)
	case "EnterPlanMode":
		return "[Entering Plan Mode]"
	case "ExitPlanMode":
		return "[Exiting Plan Mode]"
	case "Read":
		return fmt.Sprintf("[Read: %s]", input.Get("file_path").Str)
	case "Glob":
		return formatGlob(input)
	case "Grep":
		return fmt.Sprintf("[Grep: %s]", input.Get("pattern").Str)
	case "Edit":
		return fmt.Sprintf("[Edit: %s]", input.Get("file_path").Str)
	case "Write":
		return fmt.Sprintf("[Write: %s]", input.Get("file_path").Str)
	case "Bash":
		return formatBash(input)
	case "Task":
		return formatTask(input)
	default:
		return fmt.Sprintf("[Tool: %s]", name)
	}
}

func formatAskUserQuestion(
	name string, input gjson.Result,
) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("[Question: %s]", name))
	input.Get("questions").ForEach(func(_, q gjson.Result) bool {
		lines = append(lines, "  "+q.Get("question").Str)
		q.Get("options").ForEach(func(_, opt gjson.Result) bool {
			lines = append(lines, fmt.Sprintf(
				"    - %s: %s",
				opt.Get("label").Str,
				opt.Get("description").Str,
			))
			return true
		})
		return true
	})
	return strings.Join(lines, "\n")
}

func formatTodoWrite(input gjson.Result) string {
	var lines []string
	lines = append(lines, "[Todo List]")
	input.Get("todos").ForEach(func(_, todo gjson.Result) bool {
		status := todo.Get("status").Str
		icon := todoIcons[status]
		if icon == "" {
			icon = "○"
		}
		lines = append(lines, fmt.Sprintf(
			"  %s %s", icon, todo.Get("content").Str,
		))
		return true
	})
	return strings.Join(lines, "\n")
}

func formatGlob(input gjson.Result) string {
	return fmt.Sprintf("[Glob: %s in %s]",
		input.Get("pattern").Str,
		orDefault(input.Get("path").Str, "."))
}

func formatBash(input gjson.Result) string {
	cmd := input.Get("command").Str
	desc := input.Get("description").Str
	if desc != "" {
		return fmt.Sprintf("[Bash: %s]\n$ %s", desc, cmd)
	}
	return fmt.Sprintf("[Bash]\n$ %s", cmd)
}

func formatTask(input gjson.Result) string {
	desc := input.Get("description").Str
	agentType := input.Get("subagent_type").Str
	return fmt.Sprintf("[Task: %s (%s)]", desc, agentType)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
