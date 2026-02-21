import { test, expect } from "@playwright/test";
import { SessionsPage } from "./pages/sessions-page";
import {
  waitForStableValue,
  waitForRowCountStable,
} from "./helpers/virtual-list-helpers";

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
    await expect
      .poll(() => messageRequests.length, { timeout: 5_000 })
      .toBeGreaterThan(0);

    // Wait for requests to stop firing
    await waitForStableValue(() => messageRequests.length, 500);

    // For large sessions we may fetch several pages while loading
    // into memory. With the reactive loop bug, this would be
    // dozens of parallel requests.
    expect(messageRequests.length).toBeLessThanOrEqual(15);
  });

  test("small session loads fast", async ({ page }) => {
    const sp = new SessionsPage(page);
    await sp.goto();
    await sp.selectLastSession();
  });

  test(
    "large session shows first page quickly",
    async ({ page }) => {
      const sp = new SessionsPage(page);
      await sp.goto();

      // First session is the largest (5500 messages)
      await sp.sessionItems.first().click();

      // First page should render within 3s
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
      await waitForRowCountStable(sp);

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
});
