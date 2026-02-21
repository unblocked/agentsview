import { describe, it, expect, vi, beforeEach } from 'vitest';
import { messages } from './messages.svelte.js';
import * as api from '../api/client.js';
import type {
  Message,
  MessagesResponse,
  MinimapResponse,
  Session,
} from '../api/types.js';

// Mock the API client
vi.mock('../api/client.js', () => ({
  getMessages: vi.fn(),
  getMinimap: vi.fn(),
  getSession: vi.fn(),
}));

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function makeSession(
  id: string,
  messageCount: number,
): Session {
  return {
    id,
    project: 'project-alpha',
    machine: 'test-machine',
    agent: 'test-agent',
    first_message: null,
    started_at: null,
    ended_at: null,
    message_count: messageCount,
    created_at: new Date(0).toISOString(),
  };
}

function makeMessage(ordinal: number): Message {
  return {
    id: ordinal + 1,
    session_id: 's1',
    ordinal,
    role: ordinal % 2 === 0 ? 'user' : 'assistant',
    content: `msg ${ordinal}`,
    timestamp: new Date(ordinal * 1000).toISOString(),
    has_thinking: false,
    has_tool_use: false,
    content_length: 6,
  };
}

function makeMessagesResponse(
  rows: Message[],
): MessagesResponse {
  return {
    messages: rows,
    count: rows.length,
  };
}

function emptyMinimap(): MinimapResponse {
  return {
    entries: [],
    count: 0,
  };
}

async function setupSession(
  sessionId: string,
  messageCount: number,
  msgs: Message[] = [],
) {
  vi.mocked(api.getSession).mockResolvedValue(
    makeSession(sessionId, messageCount),
  );
  vi.mocked(api.getMessages).mockResolvedValue(
    makeMessagesResponse(msgs),
  );
  vi.mocked(api.getMinimap).mockResolvedValue(emptyMinimap());
  await messages.loadSession(sessionId);
}

describe('MessagesStore', () => {
  beforeEach(() => {
    messages.clear();
    vi.clearAllMocks();
  });

  it('should clear reload state when loading a new session', async () => {
    await setupSession('s1', 10);
    expect(messages.sessionId).toBe('s1');

    // Trigger a reload that hangs
    const { promise: pendingReload, resolve: resolveReload } =
      createDeferred<Session>();
    vi.mocked(api.getSession).mockReturnValue(pendingReload);

    const p1 = messages.reload();

    // Switch to session s2
    vi.mocked(api.getSession).mockResolvedValue(
      makeSession('s2', 5),
    );
    await messages.loadSession('s2');

    expect(messages.sessionId).toBe('s2');

    // A new reload should create a fresh promise, not reuse p1
    const { promise: s2Reload, resolve: resolveS2 } =
      createDeferred<Session>();
    vi.mocked(api.getSession).mockReturnValue(s2Reload);
    const p2 = messages.reload();
    expect(p2).not.toBe(p1);

    // Resolve dangling promises to clean up
    resolveReload(makeSession('s1', 10));
    resolveS2(makeSession('s2', 5));
    await Promise.all([p1, p2]);
  });

  it('should not reuse reload promise from different session', async () => {
    await setupSession('s1', 10);

    // Start reload for s1
    const { promise: s1Promise, resolve: resolveS1 } =
      createDeferred<Session>();
    vi.mocked(api.getSession).mockReturnValue(s1Promise);

    const p1 = messages.reload();

    // Switch to s2
    vi.mocked(api.getSession).mockResolvedValue(
      makeSession('s2', 5),
    );
    await messages.loadSession('s2');

    // Start reload for s2 — must be a new promise
    const { promise: s2Promise, resolve: resolveS2 } =
      createDeferred<Session>();
    vi.mocked(api.getSession).mockReturnValue(s2Promise);

    const p2 = messages.reload();

    expect(p2).not.toBe(p1);

    resolveS1(makeSession('s1', 10));
    resolveS2(makeSession('s2', 5));
    await Promise.all([p1, p2]);
  });

  it('should coalesce reloads for the same session', async () => {
    await setupSession('s1', 10);

    // Start reload
    const { promise: s1Promise, resolve: resolveS1 } =
      createDeferred<Session>();
    vi.mocked(api.getSession)
      .mockReturnValueOnce(s1Promise)
      .mockResolvedValue(makeSession('s1', 10));

    const p1 = messages.reload();
    const p2 = messages.reload();

    // Coalesced: same promise returned
    expect(p1).toBe(p2);

    resolveS1(makeSession('s1', 10));
    await p1;
  });

  it('should no-op ensureOrdinalLoaded when full session is already loaded', async () => {
    vi.mocked(api.getSession).mockResolvedValue(
      makeSession('s1', 20),
    );
    vi.mocked(api.getMinimap).mockResolvedValue(emptyMinimap());
    vi.mocked(api.getMessages).mockResolvedValueOnce(
      makeMessagesResponse(
        Array.from(
          { length: 20 },
          (_, i) => makeMessage(i),
        ),
      ),
    );

    await messages.loadSession('s1');

    expect(messages.messages.length).toBe(20);
    expect(messages.messages[0]).toBeDefined();
    expect(messages.messages[0]!.ordinal).toBe(0);
    expect(messages.hasOlder).toBe(false);

    await messages.ensureOrdinalLoaded(5);

    expect(vi.mocked(api.getMessages)).toHaveBeenCalledTimes(1);
    expect(messages.messages.length).toBe(20);
    expect(messages.messages[0]).toBeDefined();
    expect(messages.messages[0]!.ordinal).toBe(0);
  });

  it('should not clear pending reload of a new session when old session reload finishes', async () => {
    // 1. Setup Session A
    await setupSession('s1', 10);

    // 2. Start Reload for Session A (P1) — hangs
    const { promise: p1Promise, resolve: resolveP1 } =
      createDeferred<Session>();
    vi.mocked(api.getSession).mockReturnValue(p1Promise);

    const p1 = messages.reload();

    // 3. Switch to Session B
    vi.mocked(api.getSession).mockResolvedValue(
      makeSession('s2', 5),
    );
    await messages.loadSession('s2');

    // 4. Start Reload for Session B (P2) — hangs
    const { promise: p2Promise, resolve: resolveP2 } =
      createDeferred<Session>();
    vi.mocked(api.getSession).mockReturnValue(p2Promise);

    const p2 = messages.reload();

    // 5. Coalesced reload for Session B
    const p3 = messages.reload();
    expect(p3).toBe(p2); // Should reuse P2

    // 6. Resolve P1 (Session A).
    // This should NOT interfere with Session B's pending reload.
    resolveP1(makeSession('s1', 10));
    await new Promise(resolve => setTimeout(resolve, 0));

    // Verify: calling reload() again still returns the
    // same coalesced promise (pending reload intact)
    const p4 = messages.reload();
    expect(p4).toBe(p2);

    // 7. Resolve P2 (Session B).
    // The pending reload should trigger automatically.
    vi.mocked(api.getSession).mockResolvedValue(
      makeSession('s2', 6),
    );
    resolveP2(makeSession('s2', 5));

    await p2;

    // After pending reload fires and completes, a new
    // reload() call should create a fresh promise
    // (no stale coalescing)
    vi.mocked(api.getSession).mockResolvedValue(
      makeSession('s2', 6),
    );
    const p5 = messages.reload();
    expect(p5).not.toBe(p2);
    await p5;
  });

  it('should fallback to full reload if incremental fetch is out of sync', async () => {
    // 1. Initial State: Session 's1' with 2 messages
    vi.mocked(api.getSession).mockResolvedValue(
      makeSession('s1', 2),
    );
    vi.mocked(api.getMinimap).mockResolvedValue(emptyMinimap());
    vi.mocked(api.getMessages).mockResolvedValueOnce(
      makeMessagesResponse([makeMessage(0), makeMessage(1)]),
    );

    await messages.loadSession('s1');
    expect(messages.messageCount).toBe(2);

    // 2. Prepare for Reload
    // New state on server: count=4.
    // Incremental fetch returns only [2], missing [3].
    // This mismatch should trigger full reload.

    vi.mocked(api.getSession).mockResolvedValueOnce(
      makeSession('s1', 4),
    );

    vi.mocked(api.getMessages).mockResolvedValueOnce(
      makeMessagesResponse([makeMessage(2)]),
    );

    vi.mocked(api.getMessages).mockResolvedValueOnce(
      makeMessagesResponse([
        makeMessage(1),
        makeMessage(0),
        makeMessage(2),
        makeMessage(3),
      ]),
    );

    await messages.reload();

    expect(messages.messageCount).toBe(4);
    expect(messages.messages.length).toBe(4);
    expect(messages.messages[3]!.ordinal).toBe(3);

    expect(vi.mocked(api.getMessages)).toHaveBeenLastCalledWith(
      's1',
      expect.objectContaining({
        from: 0,
        limit: 1000,
        direction: 'asc',
      }),
      expect.objectContaining({
        signal: expect.any(AbortSignal),
      }),
    );
  });

  it('should not update messageCount prematurely if incremental fetch fails and triggers full reload', async () => {
    // 1. Initial State: Session 's1' with 2 messages
    vi.mocked(api.getSession).mockResolvedValue(
      makeSession('s1', 2),
    );
    vi.mocked(api.getMinimap).mockResolvedValue(emptyMinimap());
    vi.mocked(api.getMessages).mockResolvedValueOnce(
      makeMessagesResponse([makeMessage(0), makeMessage(1)]),
    );

    await messages.loadSession('s1');
    expect(messages.messageCount).toBe(2);

    vi.mocked(api.getSession).mockResolvedValueOnce(
      makeSession('s1', 4),
    );

    vi.mocked(api.getMessages).mockResolvedValueOnce(
      makeMessagesResponse([makeMessage(2)]),
    );

    // Full reload — delayed via deferred
    const { promise: fullReload, resolve: resolveFullReload } =
      createDeferred<MessagesResponse>();
    vi.mocked(api.getMessages).mockReturnValueOnce(
      fullReload as ReturnType<typeof api.getMessages>,
    );

    const reloadPromise = messages.reload();

    // Wait for the full reload call to be initiated
    await vi.waitFor(() => {
      expect(
        vi.mocked(api.getMessages),
      ).toHaveBeenCalledTimes(3);
    });

    // messageCount should still be 2 until full reload
    // completes
    expect(messages.messageCount).toBe(2);

    resolveFullReload(
      makeMessagesResponse([
        makeMessage(0),
        makeMessage(1),
        makeMessage(2),
        makeMessage(3),
      ]),
    );

    await reloadPromise;

    expect(messages.messageCount).toBe(4);
  });
});
