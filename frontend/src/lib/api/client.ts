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
): EventSource {
  const es = new EventSource(`${BASE}/sync`);

  es.addEventListener("progress", (e: MessageEvent) => {
    callbacks.onProgress?.(JSON.parse(e.data) as SyncProgress);
  });

  es.addEventListener("done", (e: MessageEvent) => {
    callbacks.onDone?.(JSON.parse(e.data) as SyncStats);
    es.close();
  });

  es.onerror = () => {
    callbacks.onError?.(new Error("Sync stream error"));
    es.close();
  };

  return es;
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
