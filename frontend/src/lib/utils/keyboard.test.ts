import {
  describe,
  it,
  expect,
  vi,
  beforeEach,
  afterEach,
} from "vitest";
import { ui } from "../stores/ui.svelte.js";
import { sessions } from "../stores/sessions.svelte.js";
import { registerShortcuts } from "./keyboard.js";

function fireKey(
  key: string,
  opts: Partial<KeyboardEventInit> = {},
) {
  const event = new KeyboardEvent("keydown", {
    key,
    bubbles: true,
    ...opts,
  });
  document.dispatchEvent(event);
}

describe("registerShortcuts", () => {
  let cleanup: () => void;
  let navigateMessage: (delta: number) => void;

  beforeEach(() => {
    ui.activeModal = null;
    ui.selectedOrdinal = null;
    navigateMessage = vi.fn();
    cleanup = registerShortcuts({ navigateMessage });
  });

  afterEach(() => {
    cleanup();
  });

  describe("Cmd+K modal toggle", () => {
    it("should open command palette on Cmd+K", () => {
      fireKey("k", { metaKey: true });
      expect(ui.activeModal).toBe("commandPalette");
    });

    it("should close command palette on second Cmd+K", () => {
      fireKey("k", { metaKey: true });
      expect(ui.activeModal).toBe("commandPalette");

      fireKey("k", { metaKey: true });
      expect(ui.activeModal).toBeNull();
    });

    it("should replace other modal with command palette", () => {
      ui.activeModal = "shortcuts";
      fireKey("k", { metaKey: true });
      expect(ui.activeModal).toBe("commandPalette");
    });

    it("should work with Ctrl+K", () => {
      fireKey("k", { ctrlKey: true });
      expect(ui.activeModal).toBe("commandPalette");
    });
  });

  describe("Escape handling", () => {
    it("should close active modal on Escape", () => {
      ui.activeModal = "commandPalette";
      fireKey("Escape");
      expect(ui.activeModal).toBeNull();
    });

    it("should close shortcuts modal on Escape", () => {
      ui.activeModal = "shortcuts";
      fireKey("Escape");
      expect(ui.activeModal).toBeNull();
    });

    it("should close publish modal on Escape", () => {
      ui.activeModal = "publish";
      fireKey("Escape");
      expect(ui.activeModal).toBeNull();
    });

    it("should deselect session when no modal is open", () => {
      sessions.activeSessionId = "s1";
      fireKey("Escape");
      expect(sessions.activeSessionId).toBeNull();
    });

    it("should prioritize closing modal over deselecting session", () => {
      ui.activeModal = "commandPalette";
      sessions.activeSessionId = "s1";

      fireKey("Escape");

      expect(ui.activeModal).toBeNull();
      expect(sessions.activeSessionId).toBe("s1");
    });
  });

  describe("modal blocks other shortcuts", () => {
    it("should block navigation when modal is open", () => {
      ui.activeModal = "commandPalette";
      fireKey("j");
      expect(navigateMessage).not.toHaveBeenCalled();
    });

    it("should allow navigation when no modal is open", () => {
      fireKey("j");
      expect(navigateMessage).toHaveBeenCalledWith(1);
    });
  });

  describe("? opens shortcuts modal", () => {
    it("should open shortcuts modal", () => {
      fireKey("?");
      expect(ui.activeModal).toBe("shortcuts");
    });
  });

  describe("cleanup removes listener", () => {
    it("should stop handling keys after cleanup", () => {
      cleanup();
      fireKey("k", { metaKey: true });
      expect(ui.activeModal).toBeNull();
    });
  });
});
