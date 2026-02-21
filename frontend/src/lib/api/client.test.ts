import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { triggerSync } from "./client.js";
import type { SyncProgress, SyncStats } from "./types.js";

/**
 * Create a ReadableStream that yields the given chunks as
 * Uint8Array values, then closes.
 */
function makeSSEStream(
  chunks: string[],
): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  let i = 0;
  return new ReadableStream({
    pull(controller) {
      if (i < chunks.length) {
        controller.enqueue(encoder.encode(chunks[i]!));
        i++;
      } else {
        controller.close();
      }
    },
  });
}

function mockFetchWithStream(
  chunks: string[],
): void {
  const stream = makeSSEStream(chunks);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
    ok: true,
    body: stream,
  }));
}

interface SyncResult {
  progress: SyncProgress[];
  done: SyncStats[];
  errors: Error[];
}

describe("triggerSync SSE parsing", () => {
  let activeControllers: AbortController[] = [];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    for (const c of activeControllers) c.abort();
    activeControllers = [];
  });

  async function executeSync(
    chunks: string[],
    waitFor: (result: SyncResult) => void,
  ): Promise<SyncResult> {
    mockFetchWithStream(chunks);
    const result: SyncResult = {
      progress: [],
      done: [],
      errors: [],
    };

    const controller = triggerSync({
      onProgress: (p) => result.progress.push(p),
      onDone: (s) => result.done.push(s),
      onError: (e) => result.errors.push(e),
    });
    activeControllers.push(controller);

    await vi.waitFor(() => waitFor(result));
    return result;
  }

  it("should parse CRLF-terminated SSE frames", async () => {
    const { progress, done } = await executeSync(
      [
        "event: progress\r\ndata: {\"phase\":\"scanning\",\"projects_total\":1,\"projects_done\":0,\"sessions_total\":0,\"sessions_done\":0,\"messages_indexed\":0}\r\n\r\n",
        "event: done\r\ndata: {\"total_sessions\":5,\"synced\":3,\"skipped\":2}\r\n\r\n",
      ],
      (r) => expect(r.done.length).toBe(1),
    );

    expect(progress.length).toBe(1);
    expect(progress[0]!.phase).toBe("scanning");
    expect(done[0]!.total_sessions).toBe(5);
    expect(done[0]!.synced).toBe(3);
  });

  it("should handle multi-line data: payloads", async () => {
    const { progress } = await executeSync(
      [
        'event: progress\ndata: {"phase":"scanning",\ndata: "projects_total":2,"projects_done":1,\ndata: "sessions_total":10,"sessions_done":5,"messages_indexed":50}\n\n',
        'event: done\ndata: {"total_sessions":10,"synced":5,"skipped":5}\n\n',
      ],
      (r) => expect(r.done.length).toBe(1),
    );

    expect(progress.length).toBe(1);
    expect(progress[0]!.projects_total).toBe(2);
    expect(progress[0]!.sessions_done).toBe(5);
  });

  it("should process trailing frame on EOF", async () => {
    const { done } = await executeSync(
      [
        'event: done\ndata: {"total_sessions":1,"synced":1,"skipped":0}',
      ],
      (r) => expect(r.done.length).toBe(1),
    );

    expect(done[0]!.total_sessions).toBe(1);
    expect(done[0]!.synced).toBe(1);
  });

  it("should trigger onDone once and stop processing after done", async () => {
    const { progress, done } = await executeSync(
      [
        'event: done\ndata: {"total_sessions":1,"synced":1,"skipped":0}\n\n',
        'event: progress\ndata: {"phase":"extra","projects_total":0,"projects_done":0,"sessions_total":0,"sessions_done":0,"messages_indexed":0}\n\n',
      ],
      (r) => expect(r.done.length).toBe(1),
    );

    // Small delay to ensure no further processing happens
    await new Promise((r) => setTimeout(r, 50));

    expect(done.length).toBe(1);
    expect(progress.length).toBe(0);
  });

  it("should handle data: without space after colon", async () => {
    const { done } = await executeSync(
      [
        'event: done\ndata:{"total_sessions":3,"synced":2,"skipped":1}\n\n',
      ],
      (r) => expect(r.done.length).toBe(1),
    );

    expect(done[0]!.total_sessions).toBe(3);
  });

  it("should call onError for non-ok responses", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      body: null,
    }));

    const result: SyncResult = {
      progress: [],
      done: [],
      errors: [],
    };

    const controller = triggerSync({
      onError: (e) => result.errors.push(e),
    });
    activeControllers.push(controller);

    await vi.waitFor(() => {
      expect(result.errors.length).toBe(1);
    });

    expect(result.errors[0]!.message).toContain("500");
  });

  it("should handle chunks split across frame boundaries", async () => {
    const { progress } = await executeSync(
      [
        'event: progress\ndata: {"phase":"scan',
        'ning","projects_total":1,"projects_done":0,"sessions_total":0,"sessions_done":0,"messages_indexed":0}\n\nevent: done\ndata: {"total_sessions":1,"synced":1,"skipped":0}\n\n',
      ],
      (r) => expect(r.done.length).toBe(1),
    );

    expect(progress.length).toBe(1);
    expect(progress[0]!.phase).toBe("scanning");
  });
});
