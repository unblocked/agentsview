import type { Locator } from "@playwright/test";
import { expect } from "@playwright/test";
import type { SessionsPage } from "../pages/sessions-page";
import { waitForStableValue } from "../../src/lib/utils/poll.js";

export { waitForStableValue };

type ScrollPosition = "top" | "bottom" | "middle" | number;

/** Returns the current scrollTop of a scrollable container. */
export function getScrollTop(locator: Locator): Promise<number> {
  return locator.evaluate((el) => el.scrollTop);
}

/**
 * Scrolls a virtual list container to the given position
 * and dispatches a scroll event to trigger virtualizer updates.
 */
export async function scrollListTo(
  locator: Locator,
  position: ScrollPosition,
): Promise<void> {
  await locator.evaluate((el, pos) => {
    if (pos === "top") {
      el.scrollTop = 0;
    } else if (pos === "bottom") {
      el.scrollTop = el.scrollHeight;
    } else if (pos === "middle") {
      el.scrollTop = (el.scrollHeight - el.clientHeight) / 2;
    } else {
      el.scrollTop = pos;
    }
    el.dispatchEvent(new Event("scroll"));
  }, position);
}

/**
 * Waits for the virtual row count (via SessionsPage POM)
 * to stabilize, indicating progressive loading is complete.
 */
export async function waitForRowCountStable(
  sp: SessionsPage,
  durationMs: number = 800,
): Promise<void> {
  await expect
    .poll(() => sp.messageRows.count(), { timeout: 5_000 })
    .toBeGreaterThan(0);

  await waitForStableValue(
    () => sp.messageRows.count(),
    durationMs,
    200,
  );
}
