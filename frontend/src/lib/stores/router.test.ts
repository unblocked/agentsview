import {
  describe,
  it,
  expect,
  vi,
  beforeEach,
  afterEach,
} from "vitest";
import {
  parseHash,
  RouterStore,
} from "./router.svelte.js";

describe("parseHash", () => {
  let originalHash: string;

  beforeEach(() => {
    originalHash = window.location.hash;
  });

  afterEach(() => {
    window.location.hash = originalHash;
  });

  it("returns default route for empty hash", () => {
    window.location.hash = "";
    const result = parseHash();
    expect(result.route).toBe("sessions");
    expect(result.params).toEqual({});
  });

  it("returns default route for bare slash", () => {
    window.location.hash = "#/";
    const result = parseHash();
    expect(result.route).toBe("sessions");
    expect(result.params).toEqual({});
  });

  it("parses #/sessions with query params", () => {
    window.location.hash = "#/sessions?x=1&y=hello";
    const result = parseHash();
    expect(result.route).toBe("sessions");
    expect(result.params).toEqual({ x: "1", y: "hello" });
  });

  it("parses #/sessions without query params", () => {
    window.location.hash = "#/sessions";
    const result = parseHash();
    expect(result.route).toBe("sessions");
    expect(result.params).toEqual({});
  });

  it("falls back to default route for unknown path", () => {
    window.location.hash = "#/unknown";
    const result = parseHash();
    expect(result.route).toBe("sessions");
    expect(result.params).toEqual({});
  });

  it("falls back to default route for unknown path with params", () => {
    window.location.hash = "#/foo?bar=baz";
    const result = parseHash();
    expect(result.route).toBe("sessions");
    expect(result.params).toEqual({ bar: "baz" });
  });

  it("handles path without leading slash", () => {
    window.location.hash = "#sessions?a=1";
    const result = parseHash();
    expect(result.route).toBe("sessions");
    expect(result.params).toEqual({ a: "1" });
  });
});

describe("RouterStore", () => {
  let store: RouterStore;

  afterEach(() => {
    store?.destroy();
    window.location.hash = "";
  });

  it("initializes with parsed hash", () => {
    window.location.hash = "#/sessions?project=test";
    store = new RouterStore();
    expect(store.route).toBe("sessions");
    expect(store.params).toEqual({ project: "test" });
  });

  it("falls back to default on invalid route", () => {
    window.location.hash = "#/bogus";
    store = new RouterStore();
    expect(store.route).toBe("sessions");
  });

  it("destroy removes the hashchange listener", () => {
    window.location.hash = "";
    store = new RouterStore();

    const addSpy = vi.spyOn(window, "removeEventListener");
    store.destroy();
    expect(addSpy).toHaveBeenCalledWith(
      "hashchange",
      expect.any(Function),
    );
    addSpy.mockRestore();
  });

  it("does not accumulate listeners across instances", () => {
    const addSpy = vi.spyOn(window, "addEventListener");

    const store1 = new RouterStore();
    const store2 = new RouterStore();

    const hashChangeCalls = addSpy.mock.calls.filter(
      ([event]) => event === "hashchange",
    );
    expect(hashChangeCalls).toHaveLength(2);

    store1.destroy();
    store2.destroy();
    addSpy.mockRestore();
  });
});
