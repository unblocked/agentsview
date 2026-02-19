/** Matches Go Session struct in internal/db/sessions.go */
export interface Session {
  id: string;
  project: string;
  machine: string;
  agent: string;
  first_message: string | null;
  started_at: string | null;
  ended_at: string | null;
  message_count: number;
  file_path?: string;
  file_size?: number;
  file_mtime?: number;
  created_at: string;
}

/** Matches Go SessionPage struct */
export interface SessionPage {
  sessions: Session[];
  next_cursor?: string;
  total: number;
}

/** Matches Go ProjectInfo struct */
export interface ProjectInfo {
  name: string;
  session_count: number;
}

/** Matches Go Message struct in internal/db/messages.go */
export interface Message {
  id: number;
  session_id: string;
  ordinal: number;
  role: string;
  content: string;
  timestamp: string;
  has_thinking: boolean;
  has_tool_use: boolean;
  content_length: number;
}

/** Matches Go MinimapEntry struct */
export interface MinimapEntry {
  ordinal: number;
  role: string;
  content_length: number;
  has_thinking: boolean;
  has_tool_use: boolean;
}

/** Matches Go SearchResult struct in internal/db/search.go */
export interface SearchResult {
  session_id: string;
  project: string;
  ordinal: number;
  role: string;
  timestamp: string;
  snippet: string;
  rank: number;
}

/** Matches Go Stats struct in internal/db/stats.go */
export interface Stats {
  session_count: number;
  message_count: number;
  project_count: number;
  machine_count: number;
}

/** Matches Go Progress struct in internal/sync/progress.go */
export interface SyncProgress {
  phase: string;
  current_project?: string;
  projects_total: number;
  projects_done: number;
  sessions_total: number;
  sessions_done: number;
  messages_indexed: number;
}

/** Matches Go SyncStats struct */
export interface SyncStats {
  total_sessions: number;
  synced: number;
  skipped: number;
}

export interface SyncStatus {
  last_sync: string;
  stats: SyncStats | null;
}

export interface MessagesResponse {
  messages: Message[];
  count: number;
}

export interface MinimapResponse {
  entries: MinimapEntry[];
  count: number;
}

export interface SearchResponse {
  query: string;
  results: SearchResult[];
  count: number;
  next: number;
}

export interface ProjectsResponse {
  projects: ProjectInfo[];
}

export interface MachinesResponse {
  machines: string[];
}

export interface PublishResponse {
  gist_id: string;
  gist_url: string;
  view_url: string;
  raw_url: string;
}

export interface GithubConfig {
  configured: boolean;
}

export interface SetGithubConfigResponse {
  success: boolean;
  username: string;
}

/* Analytics types — match Go structs in internal/db/analytics.go */

export interface AgentSummary {
  sessions: number;
  messages: number;
}

export interface AnalyticsSummary {
  total_sessions: number;
  total_messages: number;
  active_projects: number;
  active_days: number;
  avg_messages: number;
  median_messages: number;
  p90_messages: number;
  most_active_project: string;
  concentration: number;
  agents: Record<string, AgentSummary>;
}

export interface ActivityEntry {
  date: string;
  sessions: number;
  messages: number;
  user_messages: number;
  assistant_messages: number;
  by_agent: Record<string, number>;
}

export interface ActivityResponse {
  granularity: string;
  series: ActivityEntry[];
}

export interface HeatmapEntry {
  date: string;
  value: number;
  level: number;
}

export interface HeatmapLevels {
  l1: number;
  l2: number;
  l3: number;
  l4: number;
}

export interface HeatmapResponse {
  metric: string;
  entries: HeatmapEntry[];
  levels: HeatmapLevels;
}

export interface ProjectAnalytics {
  name: string;
  sessions: number;
  messages: number;
  first_session: string;
  last_session: string;
  avg_messages: number;
  median_messages: number;
  agents: Record<string, number>;
  daily_trend: number;
}

export interface ProjectsAnalyticsResponse {
  projects: ProjectAnalytics[];
}
