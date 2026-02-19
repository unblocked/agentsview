import * as api from "../api/client.js";
import type { Session, ProjectInfo } from "../api/types.js";

class SessionsStore {
  sessions: Session[] = $state([]);
  projects: ProjectInfo[] = $state([]);
  activeSessionId: string | null = $state(null);
  nextCursor: string | null = $state(null);
  total: number = $state(0);
  loading: boolean = $state(false);
  projectFilter: string = $state("");
  dateFilter: string = $state("");
  dateFromFilter: string = $state("");
  dateToFilter: string = $state("");
  minMessagesFilter: number = $state(0);
  maxMessagesFilter: number = $state(0);

  private loadVersion: number = 0;

  get activeSession(): Session | undefined {
    return this.sessions.find(
      (s) => s.id === this.activeSessionId,
    );
  }

  initFromParams(params: Record<string, string>) {
    const project = params["project"] ?? "";
    const date = params["date"] ?? "";
    const dateFrom = params["date_from"] ?? "";
    const dateTo = params["date_to"] ?? "";
    const minMsgs = parseInt(params["min_messages"] ?? "", 10);
    const maxMsgs = parseInt(params["max_messages"] ?? "", 10);

    this.projectFilter = project;
    this.dateFilter = date;
    this.dateFromFilter = dateFrom;
    this.dateToFilter = dateTo;
    this.minMessagesFilter = Number.isFinite(minMsgs) ? minMsgs : 0;
    this.maxMessagesFilter = Number.isFinite(maxMsgs) ? maxMsgs : 0;
    this.activeSessionId = null;
    this.sessions = [];
    this.nextCursor = null;
    this.total = 0;
  }

  async load() {
    const version = ++this.loadVersion;
    this.loading = true;
    try {
      const page = await api.listSessions({
        project: this.projectFilter || undefined,
        date: this.dateFilter || undefined,
        date_from: this.dateFromFilter || undefined,
        date_to: this.dateToFilter || undefined,
        min_messages: this.minMessagesFilter || undefined,
        max_messages: this.maxMessagesFilter || undefined,
        limit: 200,
      });
      if (this.loadVersion !== version) return;
      this.sessions = page.sessions;
      this.nextCursor = page.next_cursor ?? null;
      this.total = page.total;
    } finally {
      if (this.loadVersion === version) {
        this.loading = false;
      }
    }
  }

  async loadMore() {
    if (!this.nextCursor || this.loading) return;
    const version = ++this.loadVersion;
    this.loading = true;
    try {
      const page = await api.listSessions({
        project: this.projectFilter || undefined,
        date: this.dateFilter || undefined,
        date_from: this.dateFromFilter || undefined,
        date_to: this.dateToFilter || undefined,
        min_messages: this.minMessagesFilter || undefined,
        max_messages: this.maxMessagesFilter || undefined,
        cursor: this.nextCursor,
        limit: 200,
      });
      if (this.loadVersion !== version) return;
      this.sessions.push(...page.sessions);
      this.nextCursor = page.next_cursor ?? null;
      this.total = page.total;
    } finally {
      if (this.loadVersion === version) {
        this.loading = false;
      }
    }
  }

  async loadProjects() {
    const res = await api.getProjects();
    this.projects = res.projects;
  }

  selectSession(id: string) {
    this.activeSessionId = id;
  }

  navigateSession(delta: number) {
    const idx = this.sessions.findIndex(
      (s) => s.id === this.activeSessionId,
    );
    const next = idx + delta;
    if (next >= 0 && next < this.sessions.length) {
      this.activeSessionId = this.sessions[next]!.id;
    }
  }

  setProjectFilter(project: string) {
    this.projectFilter = project;
    this.dateFilter = "";
    this.dateFromFilter = "";
    this.dateToFilter = "";
    this.minMessagesFilter = 0;
    this.maxMessagesFilter = 0;
    this.activeSessionId = null;
    this.sessions = [];
    this.nextCursor = null;
    this.total = 0;
    this.load();
  }
}

export const sessions = new SessionsStore();
