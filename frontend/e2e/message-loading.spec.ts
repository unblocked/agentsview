import { test, expect } from "@playwright/test";

test.describe("Message loading", () => {
  test("clicking session shows messages", async ({ page }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });

    // Click the first session (ordered by ended_at desc,
    // so xlarge-5500 is first)
    await items.first().click();

    // Messages should appear
    const messages = page.locator(".virtual-row");
    await expect(messages.first()).toBeVisible({
      timeout: 5_000,
    });
  });

  test("no request spam on session click", async ({ page }) => {
    const messageRequests: string[] = [];
    page.on("request", (req) => {
      if (req.url().includes("/messages")) {
        messageRequests.push(req.url());
      }
    });

    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });

    // Click a session and wait for messages to appear
    await items.first().click();
    const messages = page.locator(".virtual-row");
    await expect(messages.first()).toBeVisible({
      timeout: 5_000,
    });

    // Wait a bit for any straggling requests
    await page.waitForTimeout(1000);

    // For the 5500-message session: 1000 first batch + 9x500
    // remaining = 10 requests max. With the reactive loop bug,
    // this would be dozens of parallel requests.
    expect(messageRequests.length).toBeLessThanOrEqual(15);
  });

  test("large session requests capped minimap payload", async ({
    page,
  }) => {
    const minimapRequests: string[] = [];
    page.on("request", (req) => {
      if (req.url().includes("/minimap")) {
        minimapRequests.push(req.url());
      }
    });

    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });

    await items.first().click();
    await expect(page.locator(".virtual-row").first()).toBeVisible({
      timeout: 5_000,
    });

    await page.waitForTimeout(500);

    const largeReq = minimapRequests.find((u) =>
      u.includes("/sessions/test-session-xlarge-5500/minimap"),
    );
    expect(largeReq).toBeTruthy();
    const parsed = new URL(largeReq!);
    const max = parsed.searchParams.get("max");
    expect(max).toBeTruthy();
    expect(Number(max)).toBe(1200);
  });

  test("small session loads fast", async ({ page }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });

    // Click the last session (smallest, 2 messages)
    await items.last().click();

    const messages = page.locator(".virtual-row");
    await expect(messages.first()).toBeVisible({
      timeout: 2_000,
    });
  });

  test(
    "large session shows first batch quickly",
    async ({ page }) => {
      await page.goto("/");
      const items = page.locator("button.session-item");
      await expect(items.first()).toBeVisible({
        timeout: 10_000,
      });

      // First session is the largest (5500 messages)
      await items.first().click();

      // First batch should render within 3s
      const messages = page.locator(".virtual-row");
      await expect(messages.first()).toBeVisible({
        timeout: 3_000,
      });
    },
  );

  test(
    "scroll does not reset to top during loading",
    async ({ page }) => {
      await page.goto("/");
      const items = page.locator("button.session-item");
      await expect(items.first()).toBeVisible({
        timeout: 10_000,
      });

      // Click the largest session (5500 messages)
      await items.first().click();

      const scroller = page.locator(".message-list-scroll");
      await expect(
        page.locator(".virtual-row").first(),
      ).toBeVisible({ timeout: 5_000 });

      // Wait for all progressive loading to finish
      await page.waitForTimeout(4000);

      // Scroll down
      await scroller.evaluate((el) => {
        el.scrollTop = 3000;
      });
      await page.waitForTimeout(500);

      const scrollPos = await scroller.evaluate(
        (el) => el.scrollTop,
      );

      // Scroll should NOT have reset to 0.
      // Some drift is expected (variable-height items get
      // measured, changing the total height and shifting
      // content) but the position must stay well above 0.
      expect(scrollPos).toBeGreaterThan(500);
    },
  );

  test(
    "minimap jump to middle does not fight follow-up scrolling",
    async ({ page }) => {
      test.setTimeout(45_000);

      await page.goto("/");
      const items = page.locator("button.session-item");
      await expect(items.first()).toBeVisible({
        timeout: 10_000,
      });

      // Largest fixture session (5500 messages).
      await items.first().click();
      const scroller = page.locator(".message-list-scroll");
      await expect(
        page.locator(".virtual-row").first(),
      ).toBeVisible({ timeout: 5_000 });

      const minimap = page.locator(".minimap-container");
      await expect(minimap).toBeVisible();

      const box = await minimap.boundingBox();
      expect(box).toBeTruthy();

      const beforeScroll = await scroller.evaluate((el) => el.scrollTop);

      await page.mouse.click(
        box!.x + box!.width / 2,
        box!.y + box!.height / 2,
      );

      await page.waitForTimeout(300);
      await scroller.hover();
      const scrollerEl = await scroller.elementHandle();
      expect(scrollerEl).toBeTruthy();

      const afterJump = await scrollerEl!.evaluate((el) => el.scrollTop);
      // It should have moved significantly from 0 (or near 0)
      expect(Math.abs(afterJump - beforeScroll)).toBeGreaterThan(100);

      let prev = afterJump;
      let significantBackward = 0;

      for (let i = 0; i < 24; i++) {
        await page.mouse.wheel(0, 220);
        await page.waitForTimeout(20);
        const next = await scrollerEl!.evaluate((el) => el.scrollTop);
        if (next < prev - 20) {
          significantBackward++;
        }
        prev = next;
      }

      expect(significantBackward).toBeLessThanOrEqual(1);
      expect(prev).toBeGreaterThan(1000);
    },
  );

  test(
    "minimap top click maps to newest in default sort",
    async ({ page }) => {
      test.setTimeout(45_000);

      await page.goto("/");
      const items = page.locator("button.session-item");
      await expect(items.first()).toBeVisible({
        timeout: 10_000,
      });

      // Largest fixture session (5500 messages).
      await items.first().click();
      const scroller = page.locator(".message-list-scroll");
      await expect(
        page.locator(".virtual-row").first(),
      ).toBeVisible({ timeout: 5_000 });

      await scroller.hover();
      for (let i = 0; i < 12; i++) {
        await page.mouse.wheel(0, 220);
        await page.waitForTimeout(20);
      }

      const beforeTopClick = await scroller.evaluate(
        (el) => el.scrollTop,
      );
      expect(beforeTopClick).toBeGreaterThan(300);

      const minimap = page.locator(".minimap-container");
      await expect(minimap).toBeVisible();
      const box = await minimap.boundingBox();
      expect(box).toBeTruthy();

      // In newest-first mode, top of minimap should map to newest
      // messages (small scrollTop).
      await page.mouse.click(
        box!.x + box!.width / 2,
        box!.y + 2,
      );

      await expect
        .poll(
          async () => await scroller.evaluate((el) => el.scrollTop),
          { timeout: 3_000 },
        )
        .toBeLessThan(beforeTopClick - 150);
    },
  );

  test(
    "minimap jump works with reversed sort (oldest first)",
    async ({ page }) => {
      test.setTimeout(45_000);

      await page.goto("/");
      const items = page.locator("button.session-item");
      await expect(items.first()).toBeVisible({
        timeout: 10_000,
      });

      // Largest fixture session (5500 messages).
      await items.first().click();
      const scroller = page.locator(".message-list-scroll");
      await expect(
        page.locator(".virtual-row").first(),
      ).toBeVisible({ timeout: 5_000 });

      // Toggle sort to oldest first
      const sortButton = page.getByLabel("Toggle sort order");
      await sortButton.click();
      // Wait for re-render
      await page.waitForTimeout(500);

      const minimap = page.locator(".minimap-container");
      await expect(minimap).toBeVisible();

      const box = await minimap.boundingBox();
      expect(box).toBeTruthy();

      const beforeScroll = await scroller.evaluate((el) => el.scrollTop);

      // Click middle of minimap
      await page.mouse.click(
        box!.x + box!.width / 2,
        box!.y + box!.height / 2,
      );

      await page.waitForTimeout(500); // Wait for jump to settle

      const scrollerEl = await scroller.elementHandle();
      const afterJump = await scrollerEl!.evaluate((el) => el.scrollTop);

      // It should have moved significantly
      expect(Math.abs(afterJump - beforeScroll)).toBeGreaterThan(100);

      // Verify follow-up scrolling doesn't jump
      let prev = afterJump;
      let significantJumps = 0;

      for (let i = 0; i < 24; i++) {
        await page.mouse.wheel(0, 220);
        await page.waitForTimeout(20);
        const next = await scrollerEl!.evaluate((el) => el.scrollTop);
        // In oldest-first, scrolling down increases scrollTop
        // A jump "backward" (up) would be bad
        if (next < prev - 20) {
          significantJumps++;
        }
        prev = next;
      }

      expect(significantJumps).toBeLessThanOrEqual(1);
    },
  );
});
