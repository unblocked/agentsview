import type { Locator } from "@playwright/test";
import { expect } from "@playwright/test";
import type { SessionsPage } from "../pages/sessions-page";

type ScrollPosition = "top" | "bottom" | "middle" | number;

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
 * Polls a value-producing function until the value stays
 * constant for `stableDurationMs`. Throws if the value does
 * not stabilize within `maxWaitMs`.
 */
export async function waitForStableValue<T>(
  fn: () => Promise<T> | T,
  stableDurationMs: number,
  pollIntervalMs: number = 100,
  maxWaitMs: number = stableDurationMs * 3,
): Promise<T> {
  const deadline = Date.now() + maxWaitMs;
  let lastValue = await fn();
  let stableStart = Date.now();

  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, pollIntervalMs));
    const current = await fn();

    if (current !== lastValue) {
      lastValue = current;
      stableStart = Date.now();
    } else if (Date.now() - stableStart >= stableDurationMs) {
      return current;
    }
  }
  throw new Error(
    `Value did not stabilize within ${maxWaitMs}ms.` +
      ` Last value: ${lastValue}`,
  );
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
