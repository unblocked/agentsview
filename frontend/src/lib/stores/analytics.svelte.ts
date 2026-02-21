import type {
  AnalyticsSummary,
  ActivityResponse,
  HeatmapResponse,
  ProjectsAnalyticsResponse,
  HourOfWeekResponse,
  SessionShapeResponse,
  VelocityResponse,
  ToolsAnalyticsResponse,
  TopSessionsResponse,
} from "../api/types.js";
import {
  getAnalyticsSummary,
  getAnalyticsActivity,
  getAnalyticsHeatmap,
  getAnalyticsProjects,
  getAnalyticsHourOfWeek,
  getAnalyticsSessionShape,
  getAnalyticsVelocity,
  getAnalyticsTools,
  getAnalyticsTopSessions,
  type AnalyticsParams,
} from "../api/client.js";


function localDateStr(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function daysAgo(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return localDateStr(d);
}

function today(): string {
  return localDateStr(new Date());
}

type Panel =
  | "summary"
  | "activity"
  | "heatmap"
  | "projects"
  | "hourOfWeek"
  | "sessionShape"
  | "velocity"
  | "tools"
  | "topSessions";

export type Granularity = "day" | "week" | "month";
export type HeatmapMetric = "messages" | "sessions";
export type TopSessionsMetric = "messages" | "duration";

class AnalyticsStore {
  from: string = $state(daysAgo(30));
  to: string = $state(today());
  granularity: Granularity = $state("day");
  metric: HeatmapMetric = $state("messages");
  selectedDate: string | null = $state(null);
  project: string = $state("");

  summary = $state<AnalyticsSummary | null>(null);
  activity = $state<ActivityResponse | null>(null);
  heatmap = $state<HeatmapResponse | null>(null);
  projects = $state<ProjectsAnalyticsResponse | null>(null);
  hourOfWeek = $state<HourOfWeekResponse | null>(null);
  sessionShape = $state<SessionShapeResponse | null>(null);
  velocity = $state<VelocityResponse | null>(null);
  tools = $state<ToolsAnalyticsResponse | null>(null);
  topSessions = $state<TopSessionsResponse | null>(null);
  topMetric: TopSessionsMetric = $state("messages");

  loading = $state({
    summary: false,
    activity: false,
    heatmap: false,
    projects: false,
    hourOfWeek: false,
    sessionShape: false,
    velocity: false,
    tools: false,
    topSessions: false,
  });

  errors = $state<Record<Panel, string | null>>({
    summary: null,
    activity: null,
    heatmap: null,
    projects: null,
    hourOfWeek: null,
    sessionShape: null,
    velocity: null,
    tools: null,
    topSessions: null,
  });

  private versions: Record<Panel, number> = {
    summary: 0,
    activity: 0,
    heatmap: 0,
    projects: 0,
    hourOfWeek: 0,
    sessionShape: 0,
    velocity: 0,
    tools: 0,
    topSessions: 0,
  };

  private baseParams(
    opts: { includeProject?: boolean } = {},
  ): AnalyticsParams {
    const includeProject = opts.includeProject ?? true;
    const p: AnalyticsParams = {
      from: this.from,
      to: this.to,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    };
    if (includeProject && this.project) {
      p.project = this.project;
    }
    return p;
  }

  private filterParams(
    opts: { includeProject?: boolean } = {},
  ): AnalyticsParams {
    const includeProject = opts.includeProject ?? true;
    if (this.selectedDate) {
      const p: AnalyticsParams = {
        from: this.selectedDate,
        to: this.selectedDate,
        timezone:
          Intl.DateTimeFormat().resolvedOptions().timeZone,
      };
      if (includeProject && this.project) {
        p.project = this.project;
      }
      return p;
    }
    return this.baseParams({ includeProject });
  }

  private async executeFetch<T>(
    panel: Panel,
    fetchRequest: () => Promise<T>,
    onSuccess: (data: T) => void,
  ) {
    const v = ++this.versions[panel];
    this.loading[panel] = true;
    this.errors[panel] = null;
    try {
      const data = await fetchRequest();
      if (this.versions[panel] === v) {
        onSuccess(data);
      }
    } catch (e) {
      if (this.versions[panel] === v) {
        this.errors[panel] =
          e instanceof Error ? e.message : "Failed to load";
      }
    } finally {
      if (this.versions[panel] === v) {
        this.loading[panel] = false;
      }
    }
  }

  async fetchAll() {
    await Promise.all([
      this.fetchSummary(),
      this.fetchActivity(),
      this.fetchHeatmap(),
      this.fetchProjects(),
      this.fetchHourOfWeek(),
      this.fetchSessionShape(),
      this.fetchVelocity(),
      this.fetchTools(),
      this.fetchTopSessions(),
    ]);
  }

  async fetchSummary() {
    await this.executeFetch(
      "summary",
      () => getAnalyticsSummary(this.filterParams()),
      (data) => { this.summary = data; },
    );
  }

  async fetchActivity() {
    await this.executeFetch(
      "activity",
      () => getAnalyticsActivity({
        ...this.filterParams(),
        granularity: this.granularity,
      }),
      (data) => { this.activity = data; },
    );
  }

  async fetchHeatmap() {
    await this.executeFetch(
      "heatmap",
      () => getAnalyticsHeatmap({
        ...this.baseParams(),
        metric: this.metric,
      }),
      (data) => { this.heatmap = data; },
    );
  }

  // Projects chart always shows all projects (no project
  // filter) so the selected project can be highlighted in
  // context rather than shown in isolation.
  async fetchProjects() {
    await this.executeFetch(
      "projects",
      () => getAnalyticsProjects(
        this.filterParams({ includeProject: false }),
      ),
      (data) => { this.projects = data; },
    );
  }

  async fetchHourOfWeek() {
    await this.executeFetch(
      "hourOfWeek",
      () => getAnalyticsHourOfWeek(this.baseParams()),
      (data) => { this.hourOfWeek = data; },
    );
  }

  async fetchSessionShape() {
    await this.executeFetch(
      "sessionShape",
      () => getAnalyticsSessionShape(this.filterParams()),
      (data) => { this.sessionShape = data; },
    );
  }

  async fetchVelocity() {
    await this.executeFetch(
      "velocity",
      () => getAnalyticsVelocity(this.filterParams()),
      (data) => { this.velocity = data; },
    );
  }

  async fetchTools() {
    await this.executeFetch(
      "tools",
      () => getAnalyticsTools(this.filterParams()),
      (data) => { this.tools = data; },
    );
  }

  async fetchTopSessions() {
    await this.executeFetch(
      "topSessions",
      () => getAnalyticsTopSessions({
        ...this.filterParams(),
        metric: this.topMetric,
      }),
      (data) => { this.topSessions = data; },
    );
  }

  setTopMetric(m: TopSessionsMetric) {
    this.topMetric = m;
    this.fetchTopSessions();
  }

  setDateRange(from: string, to: string) {
    this.from = from;
    this.to = to;
    this.selectedDate = null;
    this.fetchAll();
  }

  selectDate(date: string) {
    if (this.selectedDate === date) {
      this.selectedDate = null;
    } else {
      this.selectedDate = date;
    }
    this.fetchSummary();
    this.fetchActivity();
    this.fetchProjects();
    this.fetchSessionShape();
    this.fetchVelocity();
    this.fetchTools();
    this.fetchTopSessions();
  }

  setGranularity(g: Granularity) {
    this.granularity = g;
    this.fetchActivity();
  }

  setMetric(m: HeatmapMetric) {
    this.metric = m;
    this.fetchHeatmap();
  }

  setProject(name: string) {
    if (this.project === name) {
      this.project = "";
    } else {
      this.project = name;
    }
    this.fetchAll();
  }
}

export const analytics = new AnalyticsStore();
