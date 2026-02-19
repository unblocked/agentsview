import { describe, it, expect } from "vitest";
import { isToolOnly } from "./content-parser.js";
import { buildDisplayItems } from "./display-items.js";
import type { Message } from "../api/types.js";

function msg(
  overrides: Partial<Message> & { content: string },
): Message {
  return {
    id: 1,
    session_id: "s1",
    ordinal: 0,
    role: "assistant",
    timestamp: "2025-02-17T21:04:00Z",
    has_thinking: false,
    has_tool_use: false,
    content_length: overrides.content.length,
    ...overrides,
  };
}

describe("isToolOnly", () => {
  it("returns true for assistant with only tool blocks", () => {
    const m = msg({
      content: "[Bash]\n$ ls",
      has_tool_use: true,
    });
    expect(isToolOnly(m)).toBe(true);
  });

  it("returns true for multiple tool blocks", () => {
    const m = msg({
      content: "[Read]\nfile.ts\n\n[Edit]\nchanges",
      has_tool_use: true,
    });
    expect(isToolOnly(m)).toBe(true);
  });

  it("returns false when text accompanies tools", () => {
    const m = msg({
      content: "Here are the results.\n\n[Bash]\n$ ls",
      has_tool_use: true,
    });
    expect(isToolOnly(m)).toBe(false);
  });

  it("returns false for user messages", () => {
    const m = msg({
      role: "user",
      content: "[Bash]\n$ ls",
      has_tool_use: true,
    });
    expect(isToolOnly(m)).toBe(false);
  });

  it("returns false when has_tool_use is false", () => {
    const m = msg({
      content: "[Bash]\n$ ls",
      has_tool_use: false,
    });
    expect(isToolOnly(m)).toBe(false);
  });

  it("returns false for plain text assistant messages", () => {
    const m = msg({ content: "Hello, how can I help?" });
    expect(isToolOnly(m)).toBe(false);
  });

  it("handles thinking + tool blocks (no text)", () => {
    const m = msg({
      content: "[Thinking]\nLet me check\n\n[Bash]\n$ ls",
      has_tool_use: true,
      has_thinking: true,
    });
    expect(isToolOnly(m)).toBe(true);
  });
});

describe("buildDisplayItems", () => {
  it("returns empty array for empty input", () => {
    expect(buildDisplayItems([])).toEqual([]);
  });

  it("wraps all text messages as individual items", () => {
    const msgs = [
      msg({ ordinal: 0, content: "Hello" }),
      msg({ ordinal: 1, role: "user", content: "Hi" }),
      msg({ ordinal: 2, content: "How can I help?" }),
    ];
    const items = buildDisplayItems(msgs);
    expect(items).toHaveLength(3);
    expect(items.every((i) => i.kind === "message")).toBe(true);
  });

  it("groups all tool-only messages into one group", () => {
    const msgs = [
      msg({
        ordinal: 0,
        content: "[Bash]\n$ ls",
        has_tool_use: true,
      }),
      msg({
        ordinal: 1,
        content: "[Read]\nfile.ts",
        has_tool_use: true,
      }),
      msg({
        ordinal: 2,
        content: "[Edit]\nchanges",
        has_tool_use: true,
      }),
    ];
    const items = buildDisplayItems(msgs);
    expect(items).toHaveLength(1);
    expect(items[0]!.kind).toBe("tool-group");
    if (items[0]!.kind === "tool-group") {
      expect(items[0]!.messages).toHaveLength(3);
      expect(items[0]!.ordinals).toEqual([0, 1, 2]);
    }
  });

  it("handles mixed text and tool messages", () => {
    const msgs = [
      msg({ ordinal: 0, content: "Let me check" }),
      msg({
        ordinal: 1,
        content: "[Bash]\n$ ls",
        has_tool_use: true,
      }),
      msg({
        ordinal: 2,
        content: "[Read]\nfile.ts",
        has_tool_use: true,
      }),
      msg({ ordinal: 3, content: "Here are the results" }),
      msg({
        ordinal: 4,
        content: "[Edit]\nchanges",
        has_tool_use: true,
      }),
    ];
    const items = buildDisplayItems(msgs);
    expect(items).toHaveLength(4);
    expect(items[0]!.kind).toBe("message");
    expect(items[1]!.kind).toBe("tool-group");
    if (items[1]!.kind === "tool-group") {
      expect(items[1]!.messages).toHaveLength(2);
      expect(items[1]!.ordinals).toEqual([1, 2]);
    }
    expect(items[2]!.kind).toBe("message");
    expect(items[3]!.kind).toBe("tool-group");
    if (items[3]!.kind === "tool-group") {
      expect(items[3]!.messages).toHaveLength(1);
      expect(items[3]!.ordinals).toEqual([4]);
    }
  });

  it("keeps messages with text + tools as single messages", () => {
    const m = msg({
      ordinal: 0,
      content: "Let me explain the output.\n\n[Bash]\n$ ls",
      has_tool_use: true,
    });
    const items = buildDisplayItems([m]);
    expect(items).toHaveLength(1);
    expect(items[0]!.kind).toBe("message");
  });

  it("user messages are always individual items", () => {
    const msgs = [
      msg({
        ordinal: 0,
        role: "user",
        content: "[Bash]\n$ ls",
        has_tool_use: true,
      }),
    ];
    const items = buildDisplayItems(msgs);
    expect(items).toHaveLength(1);
    expect(items[0]!.kind).toBe("message");
  });

  it("single tool-only message becomes a tool-group", () => {
    const msgs = [
      msg({
        ordinal: 5,
        content: "[Bash]\n$ ls",
        has_tool_use: true,
      }),
    ];
    const items = buildDisplayItems(msgs);
    expect(items).toHaveLength(1);
    expect(items[0]!.kind).toBe("tool-group");
    if (items[0]!.kind === "tool-group") {
      expect(items[0]!.ordinals).toEqual([5]);
    }
  });

  it("uses first message timestamp for tool group", () => {
    const msgs = [
      msg({
        ordinal: 0,
        content: "[Bash]\n$ ls",
        has_tool_use: true,
        timestamp: "2025-02-17T21:04:00Z",
      }),
      msg({
        ordinal: 1,
        content: "[Read]\nfile.ts",
        has_tool_use: true,
        timestamp: "2025-02-17T21:05:00Z",
      }),
    ];
    const items = buildDisplayItems(msgs);
    if (items[0]!.kind === "tool-group") {
      expect(items[0]!.timestamp).toBe("2025-02-17T21:04:00Z");
    }
  });
});
