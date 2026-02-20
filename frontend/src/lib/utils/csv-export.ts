import type {
  AnalyticsSummary,
  ActivityResponse,
  ProjectsAnalyticsResponse,
  ToolsAnalyticsResponse,
  VelocityResponse,
} from "../api/types.js";

interface AnalyticsData {
  from: string;
  to: string;
  summary: AnalyticsSummary | null;
  activity: ActivityResponse | null;
  projects: ProjectsAnalyticsResponse | null;
  tools: ToolsAnalyticsResponse | null;
  velocity: VelocityResponse | null;
}

function escapeCSV(value: string): string {
  if (
    value.includes(",") ||
    value.includes('"') ||
    value.includes("\n")
  ) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}

function row(cells: (string | number)[]): string {
  return cells.map((c) => escapeCSV(String(c))).join(",");
}

function buildSummarySection(
  summary: AnalyticsSummary,
): string {
  const lines = [
    "Summary",
    row(["Metric", "Value"]),
    row(["Sessions", summary.total_sessions]),
    row(["Messages", summary.total_messages]),
    row(["Active Projects", summary.active_projects]),
    row(["Active Days", summary.active_days]),
    row(["Avg Messages/Session", summary.avg_messages]),
    row([
      "Median Messages/Session",
      summary.median_messages,
    ]),
    row(["P90 Messages/Session", summary.p90_messages]),
    row([
      "Most Active Project",
      summary.most_active_project,
    ]),
    row([
      "Concentration",
      (summary.concentration * 100).toFixed(1) + "%",
    ]),
  ];
  return lines.join("\n");
}

function buildActivitySection(
  activity: ActivityResponse,
): string {
  const lines = [
    "Activity",
    row([
      "Date",
      "Sessions",
      "Messages",
      "User Messages",
      "Assistant Messages",
      "Tool Calls",
      "Thinking Messages",
    ]),
  ];
  for (const e of activity.series) {
    lines.push(
      row([
        e.date,
        e.sessions,
        e.messages,
        e.user_messages,
        e.assistant_messages,
        e.tool_calls,
        e.thinking_messages,
      ]),
    );
  }
  return lines.join("\n");
}

function buildProjectsSection(
  projects: ProjectsAnalyticsResponse,
): string {
  const lines = [
    "Projects",
    row([
      "Name",
      "Sessions",
      "Messages",
      "Avg Messages",
      "Median Messages",
    ]),
  ];
  for (const p of projects.projects) {
    lines.push(
      row([
        p.name,
        p.sessions,
        p.messages,
        p.avg_messages,
        p.median_messages,
      ]),
    );
  }
  return lines.join("\n");
}

function buildToolsSection(
  tools: ToolsAnalyticsResponse,
): string {
  const lines = [
    "Tool Usage",
    row(["Category", "Count", "Percentage"]),
  ];
  for (const c of tools.by_category) {
    lines.push(row([c.category, c.count, c.pct + "%"]));
  }
  return lines.join("\n");
}

function buildVelocitySection(
  velocity: VelocityResponse,
): string {
  const o = velocity.overall;
  const lines = [
    "Velocity",
    row(["Metric", "P50", "P90"]),
    row([
      "Turn Cycle (sec)",
      o.turn_cycle_sec.p50,
      o.turn_cycle_sec.p90,
    ]),
    row([
      "First Response (sec)",
      o.first_response_sec.p50,
      o.first_response_sec.p90,
    ]),
    row(["Msgs / Active Min", o.msgs_per_active_min, ""]),
    row([
      "Chars / Active Min",
      o.chars_per_active_min,
      "",
    ]),
    row([
      "Tools / Active Min",
      o.tool_calls_per_active_min,
      "",
    ]),
  ];
  return lines.join("\n");
}

export function exportAnalyticsCSV(data: AnalyticsData) {
  const sections: string[] = [];

  if (data.summary) {
    sections.push(buildSummarySection(data.summary));
  }
  if (data.activity) {
    sections.push(buildActivitySection(data.activity));
  }
  if (data.projects) {
    sections.push(buildProjectsSection(data.projects));
  }
  if (data.tools) {
    sections.push(buildToolsSection(data.tools));
  }
  if (data.velocity) {
    sections.push(buildVelocitySection(data.velocity));
  }

  if (sections.length === 0) return;

  const csv = sections.join("\n\n");
  const blob = new Blob([csv], { type: "text/csv" });
  const url = URL.createObjectURL(blob);
  const filename =
    `analytics-${data.from}-to-${data.to}.csv`;

  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
