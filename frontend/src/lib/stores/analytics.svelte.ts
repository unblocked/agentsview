import type {
  AnalyticsSummary,
  ActivityResponse,
  HeatmapResponse,
  ProjectsAnalyticsResponse,
  HourOfWeekResponse,
  SessionShapeResponse,
  VelocityResponse,
  ToolsAnalyticsResponse,
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
  type AnalyticsParams,
} from "../api/client.js";
import { router } from "./router.svelte.js";

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
  | "tools";

class AnalyticsStore {
  from: string = $state(daysAgo(30));
  to: string = $state(today());
  granularity: string = $state("day");
  metric: string = $state("messages");
  selectedDate: string | null = $state(null);

  summary = $state<AnalyticsSummary | null>(null);
  activity = $state<ActivityResponse | null>(null);
  heatmap = $state<HeatmapResponse | null>(null);
  projects = $state<ProjectsAnalyticsResponse | null>(null);
  hourOfWeek = $state<HourOfWeekResponse | null>(null);
  sessionShape = $state<SessionShapeResponse | null>(null);
  velocity = $state<VelocityResponse | null>(null);
  tools = $state<ToolsAnalyticsResponse | null>(null);

  loading = $state({
    summary: false,
    activity: false,
    heatmap: false,
    projects: false,
    hourOfWeek: false,
    sessionShape: false,
    velocity: false,
    tools: false,
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
  });

  // Per-panel version counters to avoid cross-panel conflicts.
  private versions = {
    summary: 0,
    activity: 0,
    heatmap: 0,
    projects: 0,
    hourOfWeek: 0,
    sessionShape: 0,
    velocity: 0,
    tools: 0,
  };

  private baseParams(): AnalyticsParams {
    return {
      from: this.from,
      to: this.to,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    };
  }

  // Returns params narrowed to selectedDate when one is active.
  // Used by summary, activity, and projects — but not heatmap.
  private filterParams(): AnalyticsParams {
    if (this.selectedDate) {
      return {
        from: this.selectedDate,
        to: this.selectedDate,
        timezone:
          Intl.DateTimeFormat().resolvedOptions().timeZone,
      };
    }
    return this.baseParams();
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
    ]);
  }

  async fetchSummary() {
    const v = ++this.versions.summary;
    this.loading.summary = true;
    this.errors.summary = null;
    try {
      const data = await getAnalyticsSummary(this.filterParams());
      if (this.versions.summary === v) {
        this.summary = data;
      }
    } catch (e) {
      if (this.versions.summary === v) {
        this.errors.summary =
          e instanceof Error ? e.message : "Failed to load";
      }
    } finally {
      if (this.versions.summary === v) {
        this.loading.summary = false;
      }
    }
  }

  async fetchActivity() {
    const v = ++this.versions.activity;
    this.loading.activity = true;
    this.errors.activity = null;
    try {
      const data = await getAnalyticsActivity({
        ...this.filterParams(),
        granularity: this.granularity,
      });
      if (this.versions.activity === v) {
        this.activity = data;
      }
    } catch (e) {
      if (this.versions.activity === v) {
        this.errors.activity =
          e instanceof Error ? e.message : "Failed to load";
      }
    } finally {
      if (this.versions.activity === v) {
        this.loading.activity = false;
      }
    }
  }

  async fetchHeatmap() {
    const v = ++this.versions.heatmap;
    this.loading.heatmap = true;
    this.errors.heatmap = null;
    try {
      const data = await getAnalyticsHeatmap({
        ...this.baseParams(),
        metric: this.metric,
      });
      if (this.versions.heatmap === v) {
        this.heatmap = data;
      }
    } catch (e) {
      if (this.versions.heatmap === v) {
        this.errors.heatmap =
          e instanceof Error ? e.message : "Failed to load";
      }
    } finally {
      if (this.versions.heatmap === v) {
        this.loading.heatmap = false;
      }
    }
  }

  async fetchProjects() {
    const v = ++this.versions.projects;
    this.loading.projects = true;
    this.errors.projects = null;
    try {
      const data = await getAnalyticsProjects(
        this.filterParams(),
      );
      if (this.versions.projects === v) {
        this.projects = data;
      }
    } catch (e) {
      if (this.versions.projects === v) {
        this.errors.projects =
          e instanceof Error ? e.message : "Failed to load";
      }
    } finally {
      if (this.versions.projects === v) {
        this.loading.projects = false;
      }
    }
  }

  async fetchHourOfWeek() {
    const v = ++this.versions.hourOfWeek;
    this.loading.hourOfWeek = true;
    this.errors.hourOfWeek = null;
    try {
      const data = await getAnalyticsHourOfWeek(
        this.baseParams(),
      );
      if (this.versions.hourOfWeek === v) {
        this.hourOfWeek = data;
      }
    } catch (e) {
      if (this.versions.hourOfWeek === v) {
        this.errors.hourOfWeek =
          e instanceof Error ? e.message : "Failed to load";
      }
    } finally {
      if (this.versions.hourOfWeek === v) {
        this.loading.hourOfWeek = false;
      }
    }
  }

  async fetchSessionShape() {
    const v = ++this.versions.sessionShape;
    this.loading.sessionShape = true;
    this.errors.sessionShape = null;
    try {
      const data = await getAnalyticsSessionShape(
        this.filterParams(),
      );
      if (this.versions.sessionShape === v) {
        this.sessionShape = data;
      }
    } catch (e) {
      if (this.versions.sessionShape === v) {
        this.errors.sessionShape =
          e instanceof Error ? e.message : "Failed to load";
      }
    } finally {
      if (this.versions.sessionShape === v) {
        this.loading.sessionShape = false;
      }
    }
  }

  async fetchVelocity() {
    const v = ++this.versions.velocity;
    this.loading.velocity = true;
    this.errors.velocity = null;
    try {
      const data = await getAnalyticsVelocity(
        this.filterParams(),
      );
      if (this.versions.velocity === v) {
        this.velocity = data;
      }
    } catch (e) {
      if (this.versions.velocity === v) {
        this.errors.velocity =
          e instanceof Error ? e.message : "Failed to load";
      }
    } finally {
      if (this.versions.velocity === v) {
        this.loading.velocity = false;
      }
    }
  }

  async fetchTools() {
    const v = ++this.versions.tools;
    this.loading.tools = true;
    this.errors.tools = null;
    try {
      const data = await getAnalyticsTools(
        this.filterParams(),
      );
      if (this.versions.tools === v) {
        this.tools = data;
      }
    } catch (e) {
      if (this.versions.tools === v) {
        this.errors.tools =
          e instanceof Error ? e.message : "Failed to load";
      }
    } finally {
      if (this.versions.tools === v) {
        this.loading.tools = false;
      }
    }
  }

  private updateURL() {
    const params: Record<string, string> = {
      from: this.from,
      to: this.to,
    };
    if (this.granularity !== "day") {
      params["granularity"] = this.granularity;
    }
    if (this.metric !== "messages") {
      params["metric"] = this.metric;
    }
    if (this.selectedDate) {
      params["selected"] = this.selectedDate;
    }
    router.navigate("analytics", params);
  }

  setDateRange(from: string, to: string) {
    this.from = from;
    this.to = to;
    this.selectedDate = null;
    this.updateURL();
    this.fetchAll();
  }

  selectDate(date: string) {
    if (this.selectedDate === date) {
      this.selectedDate = null;
    } else {
      this.selectedDate = date;
    }
    this.updateURL();
    this.fetchSummary();
    this.fetchActivity();
    this.fetchProjects();
    this.fetchSessionShape();
    this.fetchVelocity();
    this.fetchTools();
  }

  setGranularity(g: string) {
    this.granularity = g;
    this.updateURL();
    this.fetchActivity();
  }

  setMetric(m: string) {
    this.metric = m;
    this.updateURL();
    this.fetchHeatmap();
  }

  initFromParams(params: Record<string, string>) {
    if (params["from"]) this.from = params["from"];
    if (params["to"]) this.to = params["to"];
    if (params["granularity"]) {
      this.granularity = params["granularity"];
    }
    if (params["metric"]) this.metric = params["metric"];
    if (params["selected"]) {
      this.selectedDate = params["selected"];
    }
  }
}

export const analytics = new AnalyticsStore();
