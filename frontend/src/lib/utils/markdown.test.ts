// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { renderMarkdown } from "./markdown.js";

describe("renderMarkdown", () => {
  it("renders bold text", () => {
    expect(renderMarkdown("**bold**")).toContain(
      "<strong>bold</strong>",
    );
  });

  it("renders italic text", () => {
    expect(renderMarkdown("*italic*")).toContain(
      "<em>italic</em>",
    );
  });

  it("renders headings", () => {
    const result = renderMarkdown("## Heading 2");
    expect(result).toContain("<h2");
    expect(result).toContain("Heading 2</h2>");
  });

  it("renders inline code", () => {
    expect(renderMarkdown("`code`")).toContain(
      "<code>code</code>",
    );
  });

  it("renders links", () => {
    const result = renderMarkdown("[text](https://example.com)");
    expect(result).toContain("<a ");
    expect(result).toContain('href="https://example.com"');
    expect(result).toContain("text</a>");
  });

  it("renders unordered lists", () => {
    const result = renderMarkdown("- item one\n- item two");
    expect(result).toContain("<ul>");
    expect(result).toContain("<li>item one</li>");
    expect(result).toContain("<li>item two</li>");
  });

  it("renders ordered lists", () => {
    const result = renderMarkdown("1. first\n2. second");
    expect(result).toContain("<ol>");
    expect(result).toContain("<li>first</li>");
    expect(result).toContain("<li>second</li>");
  });

  it("renders blockquotes", () => {
    const result = renderMarkdown("> quoted text");
    expect(result).toContain("<blockquote>");
    expect(result).toContain("quoted text");
  });

  it("renders tables", () => {
    const md = "| A | B |\n| --- | --- |\n| 1 | 2 |";
    const result = renderMarkdown(md);
    expect(result).toContain("<table>");
    expect(result).toContain("<th>A</th>");
    expect(result).toContain("<td>1</td>");
  });

  it("renders horizontal rules", () => {
    expect(renderMarkdown("---")).toContain("<hr");
  });

  it("converts single newlines to <br>", () => {
    const result = renderMarkdown("line one\nline two");
    expect(result).toContain("<br>");
  });

  it("strips script tags (XSS)", () => {
    const result = renderMarkdown(
      '<script>alert("xss")</script>',
    );
    expect(result).not.toContain("<script");
  });

  it("strips event handlers (XSS)", () => {
    const result = renderMarkdown(
      '<img src=x onerror="alert(1)">',
    );
    expect(result).not.toContain("onerror");
  });

  it("strips javascript: URLs (XSS)", () => {
    const result = renderMarkdown(
      '[click](javascript:alert(1))',
    );
    expect(result).not.toContain("javascript:");
  });

  it("returns empty string for empty input", () => {
    expect(renderMarkdown("")).toBe("");
  });

  it("passes through plain text", () => {
    const result = renderMarkdown("just plain text");
    expect(result).toContain("just plain text");
  });

  it("removes trailing newlines to prevent extra height", () => {
    const result = renderMarkdown("text\n\n");
    expect(result).not.toContain("<br>");
    expect(result).toContain("text");
    // Should be just <p>text</p> basically, no trailing <br>
    expect(result.endsWith("<br>")).toBe(false);
  });
});
