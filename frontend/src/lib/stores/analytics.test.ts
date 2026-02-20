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
  HourOfWeekResponse,
  SessionShapeResponse,
  VelocityResponse,
  ToolsAnalyticsResponse,
} from "../api/types.js";

vi.mock("../api/client.js", () => ({
  getAnalyticsSummary: vi.fn(),
  getAnalyticsActivity: vi.fn(),
  getAnalyticsHeatmap: vi.fn(),
  getAnalyticsProjects: vi.fn(),
  getAnalyticsHourOfWeek: vi.fn(),
  getAnalyticsSessionShape: vi.fn(),
  getAnalyticsVelocity: vi.fn(),
  getAnalyticsTools: vi.fn(),
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

function makeHourOfWeek(): HourOfWeekResponse {
  return { cells: [] };
}

function makeSessionShape(): SessionShapeResponse {
  return {
    count: 0,
    length_distribution: [],
    duration_distribution: [],
    autonomy_distribution: [],
  };
}

function makeVelocity(): VelocityResponse {
  return {
    overall: {
      turn_cycle_sec: { p50: 0, p90: 0 },
      first_response_sec: { p50: 0, p90: 0 },
      msgs_per_active_min: 0,
      chars_per_active_min: 0,
      tool_calls_per_active_min: 0,
    },
    by_agent: [],
    by_complexity: [],
  };
}

function makeTools(): ToolsAnalyticsResponse {
  return {
    total_calls: 0,
    by_category: [],
    by_agent: [],
    trend: [],
  };
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
  vi.mocked(api.getAnalyticsHourOfWeek).mockResolvedValue(
    makeHourOfWeek(),
  );
  vi.mocked(api.getAnalyticsSessionShape).mockResolvedValue(
    makeSessionShape(),
  );
  vi.mocked(api.getAnalyticsVelocity).mockResolvedValue(
    makeVelocity(),
  );
  vi.mocked(api.getAnalyticsTools).mockResolvedValue(
    makeTools(),
  );
}

function resetStore() {
  analytics.selectedDate = null;
  analytics.from = "2024-01-01";
  analytics.to = "2024-01-31";
}

// Assert the most recent call to a mocked API function used
// the expected from/to params. Uses lastCall so it reads the
// right invocation even if the mock was called multiple times.
function assertParams(
  fn: ReturnType<typeof vi.fn>,
  from: string,
  to: string,
) {
  const mock = vi.mocked(fn);
  expect(mock).toHaveBeenCalled();
  const params = mock.mock.lastCall![0];
  expect(params.from).toBe(from);
  expect(params.to).toBe(to);
}

// Note: selectDate and setDateRange invoke API mocks
// synchronously (the mock call is recorded before the first
// await inside fetchSummary/etc.), so no async flushing is
// needed for call-count or call-param assertions.

describe("AnalyticsStore.selectDate", () => {
  beforeEach(() => {
    resetStore();
    vi.clearAllMocks();
    mockAllAPIs();
  });

  it("should set selectedDate on first click", () => {
    analytics.selectDate("2024-01-15");
    expect(analytics.selectedDate).toBe("2024-01-15");
  });

  it("should deselect when clicking the same date", () => {
    analytics.selectDate("2024-01-15");
    analytics.selectDate("2024-01-15");
    expect(analytics.selectedDate).toBeNull();
  });

  it("should switch to a different date", () => {
    analytics.selectDate("2024-01-15");
    analytics.selectDate("2024-01-20");
    expect(analytics.selectedDate).toBe("2024-01-20");
  });

  it("should fetch filtered panels but not heatmap/hourOfWeek", () => {
    analytics.selectDate("2024-01-15");

    expect(api.getAnalyticsSummary).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsActivity).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsProjects).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsSessionShape).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsVelocity).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsTools).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsHeatmap).not.toHaveBeenCalled();
    expect(api.getAnalyticsHourOfWeek).not.toHaveBeenCalled();
  });

  it("should pass selected date as from/to for filtered panels", () => {
    analytics.selectDate("2024-01-15");

    assertParams(
      api.getAnalyticsSummary, "2024-01-15", "2024-01-15",
    );
    assertParams(
      api.getAnalyticsActivity, "2024-01-15", "2024-01-15",
    );
    assertParams(
      api.getAnalyticsProjects, "2024-01-15", "2024-01-15",
    );
  });

  it("should use full range after deselecting", () => {
    analytics.selectDate("2024-01-15");
    vi.clearAllMocks();
    mockAllAPIs();

    analytics.selectDate("2024-01-15"); // deselect

    assertParams(
      api.getAnalyticsSummary, "2024-01-01", "2024-01-31",
    );
    assertParams(
      api.getAnalyticsActivity, "2024-01-01", "2024-01-31",
    );
    assertParams(
      api.getAnalyticsProjects, "2024-01-01", "2024-01-31",
    );
  });
});

describe("AnalyticsStore.setDateRange", () => {
  beforeEach(() => {
    resetStore();
    vi.clearAllMocks();
    mockAllAPIs();
  });

  it("should clear selectedDate", () => {
    analytics.selectDate("2024-01-15");
    analytics.setDateRange("2024-02-01", "2024-02-28");
    expect(analytics.selectedDate).toBeNull();
  });

  it("should fetch all panels with new range params", () => {
    analytics.setDateRange("2024-02-01", "2024-02-28");

    expect(api.getAnalyticsSummary).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsActivity).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsHeatmap).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsProjects).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsHourOfWeek).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsSessionShape).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsVelocity).toHaveBeenCalledTimes(1);
    expect(api.getAnalyticsTools).toHaveBeenCalledTimes(1);

    assertParams(
      api.getAnalyticsSummary, "2024-02-01", "2024-02-28",
    );
    assertParams(
      api.getAnalyticsActivity, "2024-02-01", "2024-02-28",
    );
    assertParams(
      api.getAnalyticsHeatmap, "2024-02-01", "2024-02-28",
    );
    assertParams(
      api.getAnalyticsProjects, "2024-02-01", "2024-02-28",
    );
    assertParams(
      api.getAnalyticsHourOfWeek, "2024-02-01", "2024-02-28",
    );
    assertParams(
      api.getAnalyticsSessionShape, "2024-02-01", "2024-02-28",
    );
    assertParams(
      api.getAnalyticsVelocity, "2024-02-01", "2024-02-28",
    );
    assertParams(
      api.getAnalyticsTools, "2024-02-01", "2024-02-28",
    );
  });
});

describe("AnalyticsStore.initFromParams", () => {
  beforeEach(() => {
    resetStore();
    analytics.granularity = "day";
    analytics.metric = "messages";
    vi.clearAllMocks();
    mockAllAPIs();
  });

  it("should read granularity, metric, and selected from params", () => {
    analytics.initFromParams({
      from: "2024-03-01",
      to: "2024-03-31",
      granularity: "week",
      metric: "sessions",
      selected: "2024-03-15",
    });
    expect(analytics.from).toBe("2024-03-01");
    expect(analytics.to).toBe("2024-03-31");
    expect(analytics.granularity).toBe("week");
    expect(analytics.metric).toBe("sessions");
    expect(analytics.selectedDate).toBe("2024-03-15");
  });

  it("should keep defaults when params are absent", () => {
    analytics.initFromParams({});
    expect(analytics.granularity).toBe("day");
    expect(analytics.metric).toBe("messages");
    expect(analytics.selectedDate).toBeNull();
  });
});

describe("AnalyticsStore.setGranularity URL sync", () => {
  beforeEach(() => {
    resetStore();
    vi.clearAllMocks();
    mockAllAPIs();
  });

  it("should call navigate with granularity param", async () => {
    const { router } = await import("./router.svelte.js");
    analytics.setGranularity("week");
    expect(router.navigate).toHaveBeenCalledWith(
      "analytics",
      expect.objectContaining({ granularity: "week" }),
    );
  });

  it("should omit default granularity from URL params", async () => {
    const { router } = await import("./router.svelte.js");
    analytics.setGranularity("day");
    expect(router.navigate).toHaveBeenCalledWith(
      "analytics",
      expect.not.objectContaining({ granularity: "day" }),
    );
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
    vi.clearAllMocks();
    mockAllAPIs();

    await analytics.fetchHeatmap();

    assertParams(
      api.getAnalyticsHeatmap, "2024-01-01", "2024-01-31",
    );
  });
});
