import { expect, type Locator, type Page } from "@playwright/test";

/**
 * Page object for the sessions view.
 * Encapsulates selectors and common navigation actions
 * shared across E2E specs.
 */
export class SessionsPage {
  readonly sessionItems: Locator;
  readonly messageRows: Locator;
  readonly scroller: Locator;
  readonly minimap: Locator;

  readonly sortButton: Locator;
  readonly projectSelect: Locator;
  readonly sessionListHeader: Locator;

  constructor(readonly page: Page) {
    this.sessionItems = page.locator("button.session-item");
    this.messageRows = page.locator(".virtual-row");
    this.scroller = page.locator(".message-list-scroll");
    this.minimap = page.locator(".minimap-container");
    this.sortButton = page.getByLabel("Toggle sort order");
    this.projectSelect = page.locator("select.project-select");
    this.sessionListHeader = page.locator(".session-list-header");
  }

  async goto() {
    await this.page.goto("/");
    await expect(this.sessionItems.first()).toBeVisible({
      timeout: 5_000,
    });
  }

  async selectSession(index: number = 0) {
    await this.sessionItems.nth(index).click();
    await expect(this.messageRows.first()).toBeVisible({
      timeout: 3_000,
    });
  }

  async selectFirstSession() {
    await this.selectSession(0);
  }

  async selectLastSession() {
    await this.sessionItems.last().click();
    await expect(this.messageRows.first()).toBeVisible({
      timeout: 3_000,
    });
  }

  async toggleSortOrder(times: number = 1) {
    for (let i = 0; i < times; i++) {
      await this.sortButton.click();
    }
  }

  async filterByProject(project: string) {
    await this.projectSelect.selectOption(project);
  }

  async clearProjectFilter() {
    await this.projectSelect.selectOption("");
  }

  async minimapBoundingBox() {
    await expect(this.minimap).toBeVisible({ timeout: 10_000 });
    const box = await this.minimap.boundingBox();
    expect(box).toBeTruthy();
    return box!;
  }

  minimapCenter(box: { x: number; y: number; width: number; height: number }) {
    return {
      x: box.x + box.width / 2,
      y: box.y + box.height / 2,
    };
  }
}

/**
 * Scrolls down inside a scroller element and verifies that
 * the scroll position increases monotonically (no large
 * backward jumps). Returns the final scroll position.
 */
export async function verifySmoothScroll(
  page: Page,
  scroller: Locator,
  steps: number = 24,
): Promise<{ backwardJumps: number; finalPosition: number }> {
  const el = await scroller.elementHandle();
  expect(el).toBeTruthy();

  let prev = await el!.evaluate((e) => e.scrollTop);
  let backwardJumps = 0;

  for (let i = 0; i < steps; i++) {
    await page.mouse.wheel(0, 220);
    await page.waitForTimeout(20);
    const next = await el!.evaluate((e) => e.scrollTop);
    if (next < prev - 20) {
      backwardJumps++;
    }
    prev = next;
  }

  return { backwardJumps, finalPosition: prev };
}
