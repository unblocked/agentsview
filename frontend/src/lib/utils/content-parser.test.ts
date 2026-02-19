import { describe, it, expect } from "vitest";
import { parseContent } from "./content-parser.js"; // Use .js extension for imports if needed, or .ts? The project uses .js in imports.

describe("parseContent", () => {
  it("preserves leading whitespace in plain text (no blocks)", () => {
    const text = "  - Indented list item";
    const segments = parseContent(text);
    expect(segments).toHaveLength(1);
    expect(segments[0]!.type).toBe("text");
    expect(segments[0]!.content).toBe("  - Indented list item");
  });

  it("removes trailing whitespace in plain text", () => {
    const text = "Text with trailing space   \n";
    const segments = parseContent(text);
    expect(segments).toHaveLength(1);
    expect(segments[0]!.type).toBe("text");
    expect(segments[0]!.content).toBe("Text with trailing space");
  });

  it("preserves leading whitespace in text segments before blocks", () => {
    const text = "  Indented text\n[Thinking]\n...";
    // The thinking block regex: /\[Thinking\]\n?([\s\S]*?)(?=\n\[|\n\n\[|$)/g;
    // It should match [Thinking]\n... at the end.
    // The gap before it is "  Indented text\n".
    
    const segments = parseContent(text);
    // Expect: text, thinking
    expect(segments[0]!.type).toBe("text");
    expect(segments[0]!.content).toBe("  Indented text");
    expect(segments[1]!.type).toBe("thinking");
  });

  it("handles whitespace correctly in gaps between blocks", () => {
    // We need two blocks to have a gap.
    // [Thinking]... [Tool]...
    // But Thinking consumes until next [
    // regex: (?=\n\[|\n\n\[|$)
    // So we need \n[ to stop the thinking block.
    
    const text = "[Thinking]\nfoo\n[Bash]\necho hi";
    // Thinking matches "[Thinking]\nfoo".
    // Tool matches "[Bash]\necho hi".
    // Gap is "\n".
    // trimEnd("\n") is "". So no text segment.
    
    const segments = parseContent(text);
    expect(segments.map(s => s.type)).toEqual(["thinking", "tool"]);
  });
  
  it("preserves leading whitespace in tail text", () => {
      // The code block regex requires a newline after the language identifier: ```(\w*)\n
      const text = "```code\ncontent```\n  Trailing text";
      
      const segments = parseContent(text);
      expect(segments).toHaveLength(2);
      expect(segments[0]!.type).toBe("code");
      expect(segments[1]!.type).toBe("text");
      // "  Trailing text" preserves leading whitespace.
      // But we need to be careful about what trimEnd() does to the newline.
      // text.slice(pos) includes the newline before "  Trailing text" if it wasn't consumed.
      // The code block match ends at ```.
      // So tail starts at \n.
      // "\n  Trailing text".trimEnd() -> "\n  Trailing text".
      expect(segments[1]!.content).toBe("\n  Trailing text");
  });
});
