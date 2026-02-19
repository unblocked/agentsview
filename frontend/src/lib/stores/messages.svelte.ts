import * as api from "../api/client.js";
import type { Message, MinimapEntry } from "../api/types.js";

const FIRST_BATCH = 1000;
const BATCH_SIZE = 500;
const MINIMAP_MAX_ENTRIES = 1200;

class MessagesStore {
  messages: Message[] = $state([]);
  minimap: MinimapEntry[] = $state([]);
  loading: boolean = $state(false);
  sessionId: string | null = $state(null);
  messageCount: number = $state(0);
  hasOlder: boolean = $state(false);
  loadingOlder: boolean = $state(false);
  private reloadPromise: Promise<void> | null = null;
  private reloadSessionId: string | null = null;
  private pendingReload: boolean = false;

  async loadSession(id: string) {
    if (
      this.sessionId === id &&
      (this.messages.length > 0 || this.loading)
    ) {
      return;
    }
    this.sessionId = id;
    this.loading = true;
    this.messages = [];
    this.minimap = [];
    this.messageCount = 0;
    this.hasOlder = false;
    this.loadingOlder = false;
    this.reloadPromise = null;
    this.reloadSessionId = null;
    this.pendingReload = false;

    try {
      await this.loadProgressively(id);
    } catch {
      // Non-fatal. Active session may have changed or the
      // source file may be mid-write during sync.
    } finally {
      if (this.sessionId === id) {
        this.loading = false;
      }
    }
  }

  reload(): Promise<void> {
    if (!this.sessionId) return Promise.resolve();
    
    // Use the session ID of the current reload to ensure we don't return
    // a promise for a previous session.
    if (this.reloadPromise && this.reloadSessionId === this.sessionId) {
      this.pendingReload = true;
      return this.reloadPromise;
    }

    const id = this.sessionId;
    this.reloadSessionId = id;

    const promise = this.reloadNow(id).finally(async () => {
      if (this.reloadPromise === promise) {
        this.reloadPromise = null;
        this.reloadSessionId = null;
      }
      if (this.pendingReload && this.sessionId === id) {
        this.pendingReload = false;
        await this.reload();
      }
    });
    this.reloadPromise = promise;
    return promise;
  }

  clear() {
    this.messages = [];
    this.minimap = [];
    this.sessionId = null;
    this.loading = false;
    this.messageCount = 0;
    this.hasOlder = false;
    this.loadingOlder = false;
    this.reloadPromise = null;
    this.reloadSessionId = null;
    this.pendingReload = false;
  }

  private async loadProgressively(id: string) {
    const [firstRes, minimapRes, sess] = await Promise.all([
      api.getMessages(id, {
        limit: FIRST_BATCH,
        direction: "desc",
      }),
      api.getMinimap(id, { max: MINIMAP_MAX_ENTRIES }),
      api.getSession(id),
    ]);

    if (this.sessionId !== id) return;
    // Keep in ascending ordinal order in store for simpler append
    // and stable ordinal math; UI handles newest-first presentation.
    this.messages = [...firstRes.messages].reverse();
    this.minimap = minimapRes.entries ?? [];
    this.messageCount = sess.message_count ?? this.messages.length;
    const oldest = this.messages[0]?.ordinal;
    if (oldest !== undefined) {
      this.hasOlder = oldest > 0;
    } else {
      this.hasOlder = false;
    }
  }

  private async loadFrom(id: string, from: number) {
    for (;;) {
      if (this.sessionId !== id) return;

      const res = await api.getMessages(id, {
        from,
        limit: BATCH_SIZE,
        direction: "asc",
      });

      if (this.sessionId !== id) return;
      if (res.messages.length === 0) break;

      this.messages.push(...res.messages);

      if (res.messages.length < BATCH_SIZE) break;
      from =
        res.messages[res.messages.length - 1]!.ordinal + 1;
    }
  }

  async loadOlder() {
    if (
      !this.sessionId ||
      this.loadingOlder ||
      !this.hasOlder ||
      this.messages.length === 0
    ) return;
    const id = this.sessionId;
    const oldest = this.messages[0]!.ordinal;
    if (oldest <= 0) {
      this.hasOlder = false;
      return;
    }

    this.loadingOlder = true;
    try {
      const res = await api.getMessages(id, {
        from: oldest - 1,
        limit: BATCH_SIZE,
        direction: "desc",
      });
      if (this.sessionId !== id) return;
      if (res.messages.length === 0) {
        this.hasOlder = false;
        return;
      }
      const chunk = [...res.messages].reverse();
      this.messages.unshift(...chunk);
      this.hasOlder = chunk[0]!.ordinal > 0;
    } finally {
      if (this.sessionId === id) {
        this.loadingOlder = false;
      }
    }
  }

  async ensureOrdinalLoaded(targetOrdinal: number) {
    for (;;) {
      if (!this.sessionId || !this.hasOlder || this.messages.length === 0) {
        return;
      }
      const oldest = this.messages[0]!.ordinal;
      if (oldest <= targetOrdinal) {
        return;
      }
      await this.loadOlder();
      if (this.messages.length === 0) {
        return;
      }
      // Safety: stop if we failed to move the lower bound.
      if (this.messages[0]!.ordinal >= oldest) {
        return;
      }
    }
  }

  private async reloadNow(id: string) {
    try {
      const sess = await api.getSession(id);
      if (this.sessionId !== id) return;

      const newCount = sess.message_count ?? 0;
      const oldCount = this.messageCount;
      if (newCount === oldCount) return;

      // Fast path: append only new messages and refresh a
      // sampled minimap snapshot.
      if (newCount > oldCount && this.messages.length > 0) {
        const lastOrdinal =
          this.messages[this.messages.length - 1]!.ordinal;
        await this.loadFrom(id, lastOrdinal + 1);
        if (this.sessionId !== id) return;

        const minimapRes = await api.getMinimap(id, {
          max: MINIMAP_MAX_ENTRIES,
        });
        if (this.sessionId !== id) return;
        this.minimap = minimapRes.entries ?? [];

        // If incremental fetch fell out of sync, repair once.
        const newest = this.messages[this.messages.length - 1];
        if (newest && newest.ordinal !== newCount - 1) {
          await this.fullReload(id);
          return;
        }

        this.messageCount = newCount;
        return;
      }

      // Message count shrank (session rewrite) or we have no local
      // data yet: do a full reload.
      await this.fullReload(id);
    } catch {
      // Non-fatal. SSE watch should keep working and retry on the
      // next update tick.
    }
  }

  private async fullReload(id: string) {
    this.loading = true;
    try {
      await this.loadProgressively(id);
    } finally {
      if (this.sessionId === id) {
        this.loading = false;
      }
    }
  }
}

export const messages = new MessagesStore();
