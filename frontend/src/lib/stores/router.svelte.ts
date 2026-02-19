type Route = "sessions" | "analytics";

function parseHash(): { route: Route; params: Record<string, string> } {
  const hash = window.location.hash.slice(1); // remove #
  if (!hash || hash === "/") {
    return { route: "sessions", params: {} };
  }

  const qIdx = hash.indexOf("?");
  const path = qIdx >= 0 ? hash.slice(0, qIdx) : hash;
  const params: Record<string, string> = {};

  if (qIdx >= 0) {
    try {
      const sp = new URLSearchParams(hash.slice(qIdx + 1));
      for (const [k, v] of sp) {
        params[k] = v;
      }
    } catch {
      // Malformed query string — ignore params
    }
  }

  const clean = path.replace(/^\//, "");
  if (clean === "analytics") {
    return { route: "analytics", params };
  }
  return { route: "sessions", params };
}

class RouterStore {
  route: Route = $state("sessions");
  params: Record<string, string> = $state({});

  constructor() {
    const initial = parseHash();
    this.route = initial.route;
    this.params = initial.params;

    $effect.root(() => {
      const handler = () => {
        const parsed = parseHash();
        this.route = parsed.route;
        this.params = parsed.params;
      };
      window.addEventListener("hashchange", handler);
      return () => window.removeEventListener("hashchange", handler);
    });
  }

  navigate(route: Route, params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const hash = qs ? `#/${route}?${qs}` : `#/${route}`;
    window.location.hash = hash;
  }
}

export const router = new RouterStore();
