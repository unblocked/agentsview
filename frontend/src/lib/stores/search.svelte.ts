import * as api from "../api/client.js";
import type { SearchResult } from "../api/types.js";

class SearchStore {
  query: string = $state("");
  project: string = $state("");
  results: SearchResult[] = $state([]);
  isSearching: boolean = $state(false);

  private debounceTimer: ReturnType<typeof setTimeout> | null = null;

  search(q: string, project?: string) {
    this.query = q;
    if (project !== undefined) this.project = project;

    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
    }

    if (!q.trim()) {
      this.results = [];
      this.isSearching = false;
      return;
    }

    this.debounceTimer = setTimeout(() => {
      this.executeSearch(q);
    }, 300);
  }

  clear() {
    this.query = "";
    this.results = [];
    this.isSearching = false;
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = null;
    }
  }

  private async executeSearch(q: string) {
    this.isSearching = true;
    try {
      const res = await api.search(q, {
        project: this.project || undefined,
        limit: 30,
      });
      // Only apply if query hasn't changed during search
      if (this.query === q) {
        this.results = res.results;
      }
    } catch {
      if (this.query === q) {
        this.results = [];
      }
    } finally {
      if (this.query === q) {
        this.isSearching = false;
      }
    }
  }
}

export const searchStore = new SearchStore();
