import { test, expect } from "@playwright/test";
import { SessionsPage, verifySmoothScroll } from "./pages/sessions-page";

test.describe("Message loading", () => {
  test("clicking session shows messages", async ({ page }) => {
    const sp = new SessionsPage(page);
    await sp.goto();
    await sp.selectFirstSession();
  });

  test("no request spam on session click", async ({ page }) => {
    const messageRequests: string[] = [];
    page.on("request", (req) => {
      if (req.url().includes("/messages")) {
        messageRequests.push(req.url());
      }
    });

    const sp = new SessionsPage(page);
    await sp.goto();
    await sp.selectFirstSession();

    // Wait for at least one message request to have fired
    expect(messageRequests.length).toBeGreaterThan(0);
    const settled = await stableValue(
      () => messageRequests.length,
      500,
    );
    expect(settled).toBe(true);

    // For the 5500-message session: 1000 first batch + 9x500
    // remaining = 10 requests max. With the reactive loop bug,
    // this would be dozens of parallel requests.
    expect(messageRequests.length).toBeLessThanOrEqual(15);
  });

  test("large session requests capped minimap payload", async ({
    page,
  }) => {
    const sp = new SessionsPage(page);
    await sp.goto();

    // Set up wait before clicking so we don't miss the request
    const minimapPromise = page.waitForRequest((req) =>
      req.url().includes(
        "/sessions/test-session-xlarge-5500/minimap",
      ),
    );
    await sp.selectFirstSession();

    const largeReq = await minimapPromise;
    const largeUrl = largeReq.url();
    const parsed = new URL(largeUrl);
    const max = parsed.searchParams.get("max");
    expect(max).toBeTruthy();
    expect(Number(max)).toBe(1200);
  });

  test("small session loads fast", async ({ page }) => {
    const sp = new SessionsPage(page);
    await sp.goto();
    await sp.selectLastSession();
  });

  test(
    "large session shows first batch quickly",
    async ({ page }) => {
      const sp = new SessionsPage(page);
      await sp.goto();

      // First session is the largest (5500 messages)
      await sp.sessionItems.first().click();

      // First batch should render within 3s
      await expect(sp.messageRows.first()).toBeVisible({
        timeout: 3_000,
      });
    },
  );

  test(
    "scroll does not reset to top during loading",
    async ({ page }) => {
      const sp = new SessionsPage(page);
      await sp.goto();
      await sp.selectFirstSession();

      // Wait for progressive loading to finish by polling
      // the message row count until it stabilizes.
      await waitForRowCountStable(page);

      // Scroll down
      await sp.scroller.evaluate((el) => {
        el.scrollTop = 3000;
      });

      // Wait for scroll position to settle
      await expect
        .poll(
          () => sp.scroller.evaluate((el) => el.scrollTop),
          { timeout: 2_000 },
        )
        .toBeGreaterThan(500);
    },
  );

  test(
    "minimap jump to middle does not fight follow-up scrolling",
    async ({ page }) => {
      const sp = new SessionsPage(page);
      await sp.goto();
      await sp.selectFirstSession();

      const box = await sp.minimapBoundingBox();
      const center = sp.minimapCenter(box);

      const beforeScroll = await sp.scroller.evaluate(
        (el) => el.scrollTop,
      );

      await page.mouse.click(center.x, center.y);

      // Wait for scroll to actually move after the click
      await expect
        .poll(
          () => sp.scroller.evaluate((el) => el.scrollTop),
          { timeout: 5_000 },
        )
        .not.toBe(beforeScroll);

      await sp.scroller.hover();

      const { backwardJumps, finalPosition } =
        await verifySmoothScroll(page, sp.scroller);

      expect(backwardJumps).toBeLessThanOrEqual(1);
      expect(finalPosition).toBeGreaterThan(1000);
    },
  );

  test(
    "minimap top click maps to newest in default sort",
    async ({ page }) => {
      const sp = new SessionsPage(page);
      await sp.goto();
      await sp.selectFirstSession();

      // Scroll down first
      await sp.scroller.hover();
      const { finalPosition } = await verifySmoothScroll(
        page,
        sp.scroller,
        12,
      );
      expect(finalPosition).toBeGreaterThan(300);

      const box = await sp.minimapBoundingBox();

      // In newest-first mode, top of minimap should map to
      // newest messages (small scrollTop).
      await page.mouse.click(
        box.x + box.width / 2,
        box.y + 2,
      );

      await expect
        .poll(
          () => sp.scroller.evaluate((el) => el.scrollTop),
          { timeout: 2_000 },
        )
        .toBeLessThan(finalPosition - 150);
    },
  );

  test(
    "minimap jump works with reversed sort (oldest first)",
    async ({ page }) => {
      const sp = new SessionsPage(page);
      await sp.goto();
      await sp.selectFirstSession();

      // Toggle sort to oldest first
      const sortButton = page.getByLabel("Toggle sort order");
      await sortButton.click();
      // Wait for re-render by checking rows are still visible
      await expect(sp.messageRows.first()).toBeVisible({
        timeout: 5_000,
      });

      const box = await sp.minimapBoundingBox();
      const center = sp.minimapCenter(box);

      const beforeScroll = await sp.scroller.evaluate(
        (el) => el.scrollTop,
      );

      await page.mouse.click(center.x, center.y);

      // Wait for scroll to actually move after the click
      await expect
        .poll(
          () => sp.scroller.evaluate((el) => el.scrollTop),
          { timeout: 5_000 },
        )
        .not.toBe(beforeScroll);

      await sp.scroller.hover();

      const { backwardJumps } = await verifySmoothScroll(
        page,
        sp.scroller,
      );

      expect(backwardJumps).toBeLessThanOrEqual(1);
    },
  );
});

/** Polls a value-producing function until it stays constant. */
async function stableValue(
  fn: () => number,
  durationMs: number,
  pollMs: number = 100,
): Promise<boolean> {
  const deadline = Date.now() + durationMs * 1.5;
  let last = fn();
  let stableStart = Date.now();

  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, pollMs));
    const current = fn();
    if (current !== last) {
      last = current;
      stableStart = Date.now();
    }
    if (Date.now() - stableStart >= durationMs) {
      return true;
    }
  }
  return false;
}

/** Waits for the virtual row count to stabilize (progressive loading done). */
async function waitForRowCountStable(
  page: import("@playwright/test").Page,
  durationMs: number = 800,
) {
  await expect
    .poll(
      async () => {
        const count = await page
          .locator(".virtual-row")
          .count();
        return count;
      },
      { timeout: 5_000 },
    )
    .toBeGreaterThan(0);

  // Wait for count to stop changing
  let lastCount = await page.locator(".virtual-row").count();
  let stableStart = Date.now();
  const deadline = Date.now() + durationMs * 1.5;

  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, 200));
    const current = await page.locator(".virtual-row").count();
    if (current !== lastCount) {
      lastCount = current;
      stableStart = Date.now();
    }
    if (Date.now() - stableStart >= durationMs) {
      return;
    }
  }
}
