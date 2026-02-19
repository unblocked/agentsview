import { ui } from "../stores/ui.svelte.js";
import { sessions } from "../stores/sessions.svelte.js";
import { sync } from "../stores/sync.svelte.js";
import { getExportUrl } from "../api/client.js";

function isInputFocused(): boolean {
  const el = document.activeElement;
  if (!el) return false;
  const tag = el.tagName;
  return (
    tag === "INPUT" ||
    tag === "TEXTAREA" ||
    tag === "SELECT" ||
    (el as HTMLElement).isContentEditable
  );
}

interface ShortcutOptions {
  navigateMessage: (delta: number) => void;
}

/**
 * Register global keyboard shortcuts.
 * Returns a cleanup function to remove the listener.
 */
export function registerShortcuts(
  opts: ShortcutOptions,
): () => void {
  function handler(e: KeyboardEvent) {
    const meta = e.metaKey || e.ctrlKey;

    // Cmd+K — always works
    if (meta && e.key === "k") {
      e.preventDefault();
      if (ui.commandPaletteOpen) {
        ui.closeCommandPalette();
      } else {
        ui.openCommandPalette();
      }
      return;
    }

    // Esc — always works
    if (e.key === "Escape") {
      if (ui.commandPaletteOpen) {
        ui.closeCommandPalette();
      } else if (ui.publishModalOpen) {
        ui.closePublishModal();
      } else if (ui.shortcutsModalOpen) {
        ui.closeShortcutsModal();
      }
      return;
    }

    // All other shortcuts: skip when modal open or input focused
    if (
      ui.commandPaletteOpen ||
      ui.publishModalOpen ||
      isInputFocused()
    ) return;

    switch (e.key) {
      case "j":
      case "ArrowDown":
        e.preventDefault();
        opts.navigateMessage(1);
        break;
      case "k":
      case "ArrowUp":
        e.preventDefault();
        opts.navigateMessage(-1);
        break;
      case "]":
        e.preventDefault();
        sessions.navigateSession(1);
        break;
      case "[":
        e.preventDefault();
        sessions.navigateSession(-1);
        break;
      case "o":
        e.preventDefault();
        ui.toggleSort();
        break;
      case "t":
        e.preventDefault();
        ui.toggleThinking();
        break;
      case "r":
        e.preventDefault();
        sync.triggerSync(() => {
          sessions.load();
        });
        break;
      case "e":
        e.preventDefault();
        if (sessions.activeSessionId) {
          window.open(
            getExportUrl(sessions.activeSessionId),
            "_blank",
          );
        }
        break;
      case "p":
        e.preventDefault();
        if (sessions.activeSessionId) {
          ui.openPublishModal();
        }
        break;
      case "?":
        e.preventDefault();
        ui.openShortcutsModal();
        break;
    }
  }

  document.addEventListener("keydown", handler);
  return () => document.removeEventListener("keydown", handler);
}
