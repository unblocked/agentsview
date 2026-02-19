import { describe, it, expect } from "vitest";
import { sanitizeSnippet } from "./format.js";

describe("sanitizeSnippet", () => {
  it("preserves <mark> tags", () => {
    const input = "hello <mark>world</mark> end";
    expect(sanitizeSnippet(input)).toBe(
      "hello <mark>world</mark> end",
    );
  });

  it("escapes other HTML tags", () => {
    const input = '<script>alert("xss")</script>';
    expect(sanitizeSnippet(input)).toBe(
      '&lt;script&gt;alert("xss")&lt;/script&gt;',
    );
  });

  it("escapes img tags", () => {
    const input = '<img src=x onerror=alert(1)>';
    expect(sanitizeSnippet(input)).toBe(
      "&lt;img src=x onerror=alert(1)&gt;",
    );
  });

  it("handles mixed mark and other tags", () => {
    const input =
      '<b>bold</b> <mark>highlighted</mark> <i>italic</i>';
    expect(sanitizeSnippet(input)).toBe(
      "&lt;b&gt;bold&lt;/b&gt; " +
        "<mark>highlighted</mark> " +
        "&lt;i&gt;italic&lt;/i&gt;",
    );
  });

  it("handles case-insensitive mark tags", () => {
    const input = "<MARK>upper</MARK> <Mark>mixed</Mark>";
    expect(sanitizeSnippet(input)).toBe(
      "<mark>upper</mark> <mark>mixed</mark>",
    );
  });

  it("handles multiple mark spans", () => {
    const input =
      "<mark>first</mark> gap <mark>second</mark>";
    expect(sanitizeSnippet(input)).toBe(
      "<mark>first</mark> gap <mark>second</mark>",
    );
  });

  it("returns empty string for empty input", () => {
    expect(sanitizeSnippet("")).toBe("");
  });

  it("handles plain text without tags", () => {
    const input = "no tags here";
    expect(sanitizeSnippet(input)).toBe("no tags here");
  });

  it("escapes angle brackets in content", () => {
    const input = "x < y > z";
    expect(sanitizeSnippet(input)).toBe("x &lt; y &gt; z");
  });

  it("handles nested mark tags gracefully", () => {
    const input = "<mark>outer <mark>inner</mark></mark>";
    expect(sanitizeSnippet(input)).toBe(
      "<mark>outer <mark>inner</mark></mark>",
    );
  });

  it("escapes event handler attributes in mark-like tags", () => {
    const input = '<mark onload=alert(1)>text</mark>';
    // The opening tag has attributes so it won't match <mark>
    // exactly — it should be escaped
    expect(sanitizeSnippet(input)).toBe(
      "&lt;mark onload=alert(1)&gt;text</mark>",
    );
  });
});
