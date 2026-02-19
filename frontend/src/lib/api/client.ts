import type {
  SessionPage,
  Session,
  MessagesResponse,
  MinimapResponse,
  SearchResponse,
  ProjectsResponse,
  MachinesResponse,
  Stats,
  SyncStatus,
  SyncProgress,
  SyncStats,
  PublishResponse,
  GithubConfig,
  SetGithubConfigResponse,
  AnalyticsSummary,
  ActivityResponse,
  HeatmapResponse,
  ProjectsAnalyticsResponse,
} from "./types.js";

const BASE = "/api/v1";

async function fetchJSON<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const res = await fetch(`${BASE}${path}`, init);
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`API ${res.status}: ${body}`);
  }
  return res.json() as Promise<T>;
}

/* Sessions */

export interface ListSessionsParams {
  project?: string;
  machine?: string;
  agent?: string;
  date?: string;
  date_from?: string;
  date_to?: string;
  min_messages?: number;
  max_messages?: number;
  cursor?: string;
  limit?: number;
}

export function listSessions(
  params: ListSessionsParams = {},
): Promise<SessionPage> {
  const q = new URLSearchParams();
  if (params.project) q.set("project", params.project);
  if (params.machine) q.set("machine", params.machine);
  if (params.agent) q.set("agent", params.agent);
  if (params.date) q.set("date", params.date);
  if (params.date_from) q.set("date_from", params.date_from);
  if (params.date_to) q.set("date_to", params.date_to);
  if (params.min_messages) q.set("min_messages", String(params.min_messages));
  if (params.max_messages) q.set("max_messages", String(params.max_messages));
  if (params.cursor) q.set("cursor", params.cursor);
  if (params.limit) q.set("limit", String(params.limit));
  const qs = q.toString();
  return fetchJSON(`/sessions${qs ? `?${qs}` : ""}`);
}

export function getSession(id: string): Promise<Session> {
  return fetchJSON(`/sessions/${id}`);
}

/* Messages */

export interface GetMessagesParams {
  from?: number;
  limit?: number;
  direction?: "asc" | "desc";
}

export function getMessages(
  sessionId: string,
  params: GetMessagesParams = {},
): Promise<MessagesResponse> {
  const q = new URLSearchParams();
  if (params.from !== undefined) q.set("from", String(params.from));
  if (params.limit) q.set("limit", String(params.limit));
  if (params.direction) q.set("direction", params.direction);
  const qs = q.toString();
  return fetchJSON(`/sessions/${sessionId}/messages${qs ? `?${qs}` : ""}`);
}

export interface GetMinimapParams {
  from?: number;
  max?: number;
}

export function getMinimap(
  sessionId: string,
  params: GetMinimapParams = {},
): Promise<MinimapResponse> {
  const q = new URLSearchParams();
  if (params.from !== undefined) q.set("from", String(params.from));
  if (params.max !== undefined) q.set("max", String(params.max));
  const qs = q.toString();
  return fetchJSON(`/sessions/${sessionId}/minimap${qs ? `?${qs}` : ""}`);
}

/* Search */

export function search(
  query: string,
  params: { project?: string; limit?: number; cursor?: number } = {},
): Promise<SearchResponse> {
  const q = new URLSearchParams();
  q.set("q", query);
  if (params.project) q.set("project", params.project);
  if (params.limit) q.set("limit", String(params.limit));
  if (params.cursor) q.set("cursor", String(params.cursor));
  return fetchJSON(`/search?${q.toString()}`);
}

/* Metadata */

export function getProjects(): Promise<ProjectsResponse> {
  return fetchJSON("/projects");
}

export function getMachines(): Promise<MachinesResponse> {
  return fetchJSON("/machines");
}

export function getStats(): Promise<Stats> {
  return fetchJSON("/stats");
}

/* Sync */

export function getSyncStatus(): Promise<SyncStatus> {
  return fetchJSON("/sync/status");
}

export interface SyncEventCallbacks {
  onProgress?: (p: SyncProgress) => void;
  onDone?: (s: SyncStats) => void;
  onError?: (e: Error) => void;
}

export function triggerSync(
  callbacks: SyncEventCallbacks,
): AbortController {
  const controller = new AbortController();

  fetch(`${BASE}/sync`, {
    method: "POST",
    signal: controller.signal,
  })
    .then(async (res) => {
      if (!res.ok || !res.body) {
        throw new Error(`Sync request failed: ${res.status}`);
      }
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      let finished = false;

      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        // Normalize CRLF to LF
        buf = buf.replaceAll("\r\n", "\n");

        if (processFrames(buf, callbacks)) {
          finished = true;
          reader.cancel();
          break;
        }
        // Remove fully consumed frames
        const last = buf.lastIndexOf("\n\n");
        if (last !== -1) buf = buf.slice(last + 2);
      }

      // Process any trailing frame on EOF
      if (!finished && buf.trim()) {
        processFrame(buf, callbacks);
      }
    })
    .catch((err) => {
      if (err instanceof DOMException && err.name === "AbortError") return;
      callbacks.onError?.(err instanceof Error ? err : new Error(String(err)));
    });

  return controller;
}

/** Parse all complete SSE frames in buf. Returns true if "done" was received. */
function processFrames(
  buf: string, callbacks: SyncEventCallbacks,
): boolean {
  let idx: number;
  let start = 0;
  while ((idx = buf.indexOf("\n\n", start)) !== -1) {
    const frame = buf.slice(start, idx);
    start = idx + 2;
    if (processFrame(frame, callbacks)) return true;
  }
  return false;
}

/** Dispatch a single SSE frame. Returns true if it was a "done" event. */
function processFrame(
  frame: string, callbacks: SyncEventCallbacks,
): boolean {
  let event = "";
  const dataLines: string[] = [];
  for (const line of frame.split("\n")) {
    if (line.startsWith("event: ")) {
      event = line.slice(7);
    } else if (line.startsWith("data: ")) {
      dataLines.push(line.slice(6));
    } else if (line.startsWith("data:")) {
      dataLines.push(line.slice(5));
    }
  }
  const data = dataLines.join("\n");
  if (!data) return false;

  if (event === "progress") {
    callbacks.onProgress?.(JSON.parse(data) as SyncProgress);
  } else if (event === "done") {
    callbacks.onDone?.(JSON.parse(data) as SyncStats);
    return true;
  }
  return false;
}

/** Watch a session for live updates via SSE */
export function watchSession(
  sessionId: string,
  onUpdate: () => void,
): EventSource {
  const es = new EventSource(
    `${BASE}/sessions/${sessionId}/watch`,
  );

  es.addEventListener("session_updated", () => {
    onUpdate();
  });

  es.onerror = () => {
    // Connection will auto-retry via EventSource spec
  };

  return es;
}

/** Get the export URL for a session */
export function getExportUrl(sessionId: string): string {
  return `${BASE}/sessions/${sessionId}/export`;
}

/* Publish / GitHub config */

export function publishSession(
  sessionId: string,
): Promise<PublishResponse> {
  return fetchJSON(`/sessions/${sessionId}/publish`, {
    method: "POST",
  });
}

export function getGithubConfig(): Promise<GithubConfig> {
  return fetchJSON("/config/github");
}

export function setGithubConfig(
  token: string,
): Promise<SetGithubConfigResponse> {
  return fetchJSON("/config/github", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
}

/* Analytics */

export interface AnalyticsParams {
  from?: string;
  to?: string;
  timezone?: string;
  machine?: string;
}

function analyticsQuery(
  params: AnalyticsParams,
  extra?: Record<string, string>,
): string {
  const q = new URLSearchParams();
  if (params.from) q.set("from", params.from);
  if (params.to) q.set("to", params.to);
  if (params.timezone) q.set("timezone", params.timezone);
  if (params.machine) q.set("machine", params.machine);
  if (extra) {
    for (const [k, v] of Object.entries(extra)) {
      q.set(k, v);
    }
  }
  const qs = q.toString();
  return qs ? `?${qs}` : "";
}

export function getAnalyticsSummary(
  params: AnalyticsParams,
): Promise<AnalyticsSummary> {
  return fetchJSON(
    `/analytics/summary${analyticsQuery(params)}`,
  );
}

export function getAnalyticsActivity(
  params: AnalyticsParams & { granularity?: string },
): Promise<ActivityResponse> {
  const { granularity, ...base } = params;
  const extra: Record<string, string> = {};
  if (granularity) extra["granularity"] = granularity;
  return fetchJSON(
    `/analytics/activity${analyticsQuery(base, extra)}`,
  );
}

export function getAnalyticsHeatmap(
  params: AnalyticsParams & { metric?: string },
): Promise<HeatmapResponse> {
  const { metric, ...base } = params;
  const extra: Record<string, string> = {};
  if (metric) extra["metric"] = metric;
  return fetchJSON(
    `/analytics/heatmap${analyticsQuery(base, extra)}`,
  );
}

export function getAnalyticsProjects(
  params: AnalyticsParams,
): Promise<ProjectsAnalyticsResponse> {
  return fetchJSON(
    `/analytics/projects${analyticsQuery(params)}`,
  );
}
