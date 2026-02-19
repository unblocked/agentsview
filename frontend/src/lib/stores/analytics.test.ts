import {
  describe,
  it,
  expect,
  vi,
  beforeEach,
} from "vitest";
import { analytics } from "./analytics.svelte.js";
import * as api from "../api/client.js";
import type {
  AnalyticsSummary,
  ActivityResponse,
  HeatmapResponse,
  ProjectsAnalyticsResponse,
} from "../api/types.js";

vi.mock("../api/client.js", () => ({
  getAnalyticsSummary: vi.fn(),
  getAnalyticsActivity: vi.fn(),
  getAnalyticsHeatmap: vi.fn(),
  getAnalyticsProjects: vi.fn(),
}));

vi.mock("./router.svelte.js", () => ({
  router: { navigate: vi.fn() },
}));

function makeSummary(): AnalyticsSummary {
  return {
    total_sessions: 10,
    total_messages: 100,
    active_projects: 3,
    active_days: 5,
    avg_messages: 10,
    median_messages: 8,
    p90_messages: 20,
    most_active_project: "proj",
    concentration: 0.5,
    agents: {},
  };
}

function makeActivity(): ActivityResponse {
  return { granularity: "day", series: [] };
}

function makeHeatmap(): HeatmapResponse {
  return {
    metric: "messages",
    entries: [],
    levels: { l1: 1, l2: 5, l3: 10, l4: 20 },
  };
}

function makeProjects(): ProjectsAnalyticsResponse {
  return { projects: [] };
}

function mockAllAPIs() {
  vi.mocked(api.getAnalyticsSummary).mockResolvedValue(
    makeSummary(),
  );
  vi.mocked(api.getAnalyticsActivity).mockResolvedValue(
    makeActivity(),
  );
  vi.mocked(api.getAnalyticsHeatmap).mockResolvedValue(
    makeHeatmap(),
  );
  vi.mocked(api.getAnalyticsProjects).mockResolvedValue(
    makeProjects(),
  );
}

function resetStore() {
  analytics.selectedDate = null;
  analytics.from = "2024-01-01";
  analytics.to = "2024-01-31";
}

describe("AnalyticsStore.selectDate", () => {
  beforeEach(() => {
    resetStore();
    vi.clearAllMocks();
    mockAllAPIs();
  });

  it("should set selectedDate on first click", async () => {
    analytics.selectDate("2024-01-15");
    await Promise.resolve();

    expect(analytics.selectedDate).toBe("2024-01-15");
  });

  it("should deselect when clicking the same date", async () => {
    analytics.selectDate("2024-01-15");
    await Promise.resolve();
    analytics.selectDate("2024-01-15");
    await Promise.resolve();

    expect(analytics.selectedDate).toBeNull();
  });

  it("should switch to a different date", async () => {
    analytics.selectDate("2024-01-15");
    await Promise.resolve();
    analytics.selectDate("2024-01-20");
    await Promise.resolve();

    expect(analytics.selectedDate).toBe("2024-01-20");
  });

  it("should fetch summary, activity, projects but not heatmap", async () => {
    analytics.selectDate("2024-01-15");
    await Promise.resolve();

    expect(api.getAnalyticsSummary).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsActivity).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsProjects).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsHeatmap).not.toHaveBeenCalled();
  });

  it("should pass selected date as from/to for filtered panels", async () => {
    analytics.selectDate("2024-01-15");
    await Promise.resolve();

    const summaryParams = vi.mocked(
      api.getAnalyticsSummary,
    ).mock.calls[0]![0];
    expect(summaryParams.from).toBe("2024-01-15");
    expect(summaryParams.to).toBe("2024-01-15");

    const activityParams = vi.mocked(
      api.getAnalyticsActivity,
    ).mock.calls[0]![0];
    expect(activityParams.from).toBe("2024-01-15");
    expect(activityParams.to).toBe("2024-01-15");

    const projectsParams = vi.mocked(
      api.getAnalyticsProjects,
    ).mock.calls[0]![0];
    expect(projectsParams.from).toBe("2024-01-15");
    expect(projectsParams.to).toBe("2024-01-15");
  });

  it("should use full range after deselecting", async () => {
    analytics.selectDate("2024-01-15");
    await Promise.resolve();
    vi.clearAllMocks();
    mockAllAPIs();

    analytics.selectDate("2024-01-15"); // deselect
    await Promise.resolve();

    const summaryParams = vi.mocked(
      api.getAnalyticsSummary,
    ).mock.calls[0]![0];
    expect(summaryParams.from).toBe("2024-01-01");
    expect(summaryParams.to).toBe("2024-01-31");
  });
});

describe("AnalyticsStore.setDateRange", () => {
  beforeEach(() => {
    resetStore();
    vi.clearAllMocks();
    mockAllAPIs();
  });

  it("should clear selectedDate", async () => {
    analytics.selectDate("2024-01-15");
    await Promise.resolve();

    analytics.setDateRange("2024-02-01", "2024-02-28");
    await Promise.resolve();

    expect(analytics.selectedDate).toBeNull();
  });

  it("should fetch all panels including heatmap", async () => {
    analytics.setDateRange("2024-02-01", "2024-02-28");
    await Promise.resolve();

    expect(api.getAnalyticsSummary).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsActivity).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsHeatmap).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsProjects).toHaveBeenCalledTimes(1);
  });
});

describe("AnalyticsStore heatmap uses full range", () => {
  beforeEach(() => {
    resetStore();
    vi.clearAllMocks();
    mockAllAPIs();
  });

  it("should use base from/to for heatmap even with selectedDate", async () => {
    analytics.selectDate("2024-01-15");
    await Promise.resolve();
    vi.clearAllMocks();
    mockAllAPIs();

    // fetchHeatmap should use the full date range
    await analytics.fetchHeatmap();

    const heatmapParams = vi.mocked(
      api.getAnalyticsHeatmap,
    ).mock.calls[0]![0];
    expect(heatmapParams.from).toBe("2024-01-01");
    expect(heatmapParams.to).toBe("2024-01-31");
  });
});
