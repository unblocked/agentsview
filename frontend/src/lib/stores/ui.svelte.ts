type Theme = "light" | "dark";

class UIStore {
  theme: Theme = $state(
    (localStorage.getItem("theme") as Theme) || "light",
  );
  showThinking: boolean = $state(true);
  sortNewestFirst: boolean = $state(true);
  commandPaletteOpen: boolean = $state(false);
  shortcutsModalOpen: boolean = $state(false);
  publishModalOpen: boolean = $state(false);
  selectedOrdinal: number = $state(-1);
  pendingScrollOrdinal: number = $state(-1);

  constructor() {
    $effect.root(() => {
      $effect(() => {
        const root = document.documentElement;
        if (this.theme === "dark") {
          root.classList.add("dark");
        } else {
          root.classList.remove("dark");
        }
        localStorage.setItem("theme", this.theme);
      });
    });
  }

  toggleTheme() {
    this.theme = this.theme === "light" ? "dark" : "light";
  }

  toggleThinking() {
    this.showThinking = !this.showThinking;
  }

  toggleSort() {
    this.sortNewestFirst = !this.sortNewestFirst;
  }

  openCommandPalette() {
    this.commandPaletteOpen = true;
  }

  closeCommandPalette() {
    this.commandPaletteOpen = false;
  }

  openShortcutsModal() {
    this.shortcutsModalOpen = true;
  }

  closeShortcutsModal() {
    this.shortcutsModalOpen = false;
  }

  openPublishModal() {
    this.publishModalOpen = true;
  }

  closePublishModal() {
    this.publishModalOpen = false;
  }

  selectOrdinal(ordinal: number) {
    this.selectedOrdinal = ordinal;
  }

  clearSelection() {
    this.selectedOrdinal = -1;
  }

  scrollToOrdinal(ordinal: number) {
    this.selectedOrdinal = ordinal;
    this.pendingScrollOrdinal = ordinal;
  }

  closeAll() {
    this.commandPaletteOpen = false;
    this.shortcutsModalOpen = false;
    this.publishModalOpen = false;
  }
}

export const ui = new UIStore();
