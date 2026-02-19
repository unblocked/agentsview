import type {
  AnalyticsSummary,
  ActivityResponse,
  HeatmapResponse,
  ProjectsAnalyticsResponse,
} from "../api/types.js";
import {
  getAnalyticsSummary,
  getAnalyticsActivity,
  getAnalyticsHeatmap,
  getAnalyticsProjects,
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

type Panel = "summary" | "activity" | "heatmap" | "projects";

class AnalyticsStore {
  from: string = $state(daysAgo(30));
  to: string = $state(today());
  granularity: string = $state("day");
  metric: string = $state("messages");

  summary = $state<AnalyticsSummary | null>(null);
  activity = $state<ActivityResponse | null>(null);
  heatmap = $state<HeatmapResponse | null>(null);
  projects = $state<ProjectsAnalyticsResponse | null>(null);

  loading = $state({
    summary: false,
    activity: false,
    heatmap: false,
    projects: false,
  });

  errors = $state<Record<Panel, string | null>>({
    summary: null,
    activity: null,
    heatmap: null,
    projects: null,
  });

  // Per-panel version counters to avoid cross-panel conflicts.
  private versions = {
    summary: 0,
    activity: 0,
    heatmap: 0,
    projects: 0,
  };

  private baseParams(): AnalyticsParams {
    return {
      from: this.from,
      to: this.to,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    };
  }

  async fetchAll() {
    await Promise.all([
      this.fetchSummary(),
      this.fetchActivity(),
      this.fetchHeatmap(),
      this.fetchProjects(),
    ]);
  }

  async fetchSummary() {
    const v = ++this.versions.summary;
    this.loading.summary = true;
    this.errors.summary = null;
    try {
      const data = await getAnalyticsSummary(this.baseParams());
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
        ...this.baseParams(),
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
        this.baseParams(),
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

  setDateRange(from: string, to: string) {
    this.from = from;
    this.to = to;
    router.navigate("analytics", { from, to });
    this.fetchAll();
  }

  setGranularity(g: string) {
    this.granularity = g;
    this.fetchActivity();
  }

  setMetric(m: string) {
    this.metric = m;
    this.fetchHeatmap();
  }

  initFromParams(params: Record<string, string>) {
    if (params["from"]) this.from = params["from"];
    if (params["to"]) this.to = params["to"];
  }
}

export const analytics = new AnalyticsStore();
