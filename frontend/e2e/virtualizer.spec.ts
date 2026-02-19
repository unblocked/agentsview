import { test, expect, type Page } from "@playwright/test";

// Session list is ordered by ended_at DESC.
// We select sessions by stable properties (project + message count)
// rather than unstable indices.
const ESTIMATE_PX = 120;

function getSessionItem(page: Page, project: string, count: number) {
  return page
    .locator(".session-item")
    .filter({ has: page.locator(`.session-project:text-is("${project}")`) })
    .filter({ has: page.locator(`.session-count:text-is("${count}")`) });
}

async function clickSession(
  page: Page,
  project: string,
  count: number,
  expectedRows?: number,
) {
  const item = getSessionItem(page, project, count);

  // Get the session ID from the item to ensure robust waiting
  const sessionId = await item.getAttribute("data-session-id");
  expect(sessionId).toBeTruthy();

  await item.click();
  await expect(item).toHaveClass(/active/);

  // Wait for the message list to show this session and finish loading.
  // We check data-messages-session-id (set by the messages store) rather
  // than just data-session-id (set by activeSessionId) to avoid a race
  // where activeSessionId updates synchronously but loadSession() runs
  // in a deferred $effect.
  const messageList = page.locator(".message-list-scroll");
  await expect(messageList).toHaveAttribute(
    "data-session-id",
    sessionId!,
  );
  await expect(messageList).toHaveAttribute(
    "data-messages-session-id",
    sessionId!,
  );
  await expect(messageList).toHaveAttribute("data-loaded", "true");

  if (expectedRows !== undefined) {
    // Wait for the virtual list to settle on the target session's item count
    // (assumes small item counts fit within viewport+overscan)
    await expect(page.locator(".virtual-row")).toHaveCount(expectedRows);
  } else {
    // Wait for the list to render at least one item to ensure loading started
    // We already confirmed the session ID, so this check is safer now
    await expect(page.locator(".virtual-row").first()).toBeVisible();
  }
}

test.describe("Virtualizer measurement", () => {
  test("items are measured to actual DOM height", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({
      timeout: 10_000,
    });

    // small-5: project-alpha, 5 msgs
    await clickSession(page, "project-alpha", 5, 5);

    const rows = page.locator(".virtual-row");

    const container = page
      .locator(".message-list-scroll > div")
      .first();

    // Wait for measurements to settle (height should not be the estimate sum)
    await expect.poll(async () => {
        return container.evaluate((el) => el.getBoundingClientRect().height);
    }).not.toBe(5 * ESTIMATE_PX);

    const totalHeight = await container.evaluate(
      (el) => el.getBoundingClientRect().height,
    );

    // 5 display items at the default estimate = 600px
    expect(totalHeight).toBeGreaterThan(0);

    // Each row should have a measured (non-estimate) height
    const rowCount = await rows.count();
    for (let i = 0; i < rowCount; i++) {
      const h = await rows.nth(i).evaluate(
        (el) => el.getBoundingClientRect().height,
      );
      expect(h).toBeGreaterThan(0);
      expect(h).not.toBe(ESTIMATE_PX);
    }
  });

  test("no gaps between consecutive virtual rows", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({
      timeout: 10_000,
    });

    await clickSession(page, "project-alpha", 5, 5);

    const rows = page.locator(".virtual-row");
    
    // Wait for layout to stabilize
    const container = page.locator(".message-list-scroll > div").first();
    await expect.poll(async () => {
        return container.evaluate((el) => el.getBoundingClientRect().height);
    }).not.toBe(5 * ESTIMATE_PX);

    const positions = await rows.evaluateAll((els) =>
      els.map((el) => {
        const style = el.style.transform;
        const match = style.match(/translateY\((\d+(?:\.\d+)?)px\)/);
        return {
          translateY: match ? parseFloat(match[1]!) : 0,
          height: el.offsetHeight,
        };
      }),
    );

    for (let i = 0; i < positions.length - 1; i++) {
      const current = positions[i]!;
      const next = positions[i + 1]!;
      const expectedNextStart = current.translateY + current.height;
      expect(Math.abs(expectedNextStart - next.translateY))
        .toBeLessThanOrEqual(1);
    }
  });

  test("total container height matches sum of items", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({
      timeout: 10_000,
    });

    await clickSession(page, "project-alpha", 5, 5);

    const rows = page.locator(".virtual-row");
    
    const container = page
      .locator(".message-list-scroll > div")
      .first();

    // Wait for layout
    await expect.poll(async () => {
        return container.evaluate((el) => el.getBoundingClientRect().height);
    }).not.toBe(5 * ESTIMATE_PX);

    const sumOfHeights = await rows.evaluateAll((els) =>
      els.reduce((sum, el) => sum + el.offsetHeight, 0),
    );

    const totalHeight = await container.evaluate(
      (el) => el.getBoundingClientRect().height,
    );

    // With overscan=5 and only 5 items, all should be in DOM
    expect(Math.abs(totalHeight - sumOfHeights))
      .toBeLessThanOrEqual(5);
  });

  test("measurements update after session switch", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({
      timeout: 10_000,
    });

    // Session A: small-5 (5 display items)
    await clickSession(page, "project-alpha", 5, 5);
    
    const container = page
      .locator(".message-list-scroll > div")
      .first();

    await expect.poll(async () => {
        return container.evaluate((el) => el.getBoundingClientRect().height);
    }).not.toBe(5 * ESTIMATE_PX);

    const heightA = await container.evaluate(
      (el) => el.getBoundingClientRect().height,
    );
    expect(heightA).toBeGreaterThan(0);

    // Session B: small-2 (2 display items)
    // project-alpha, 2 msgs
    await clickSession(page, "project-alpha", 2, 2);
    
    await expect.poll(async () => {
        return container.evaluate((el) => el.getBoundingClientRect().height);
    }).not.toBe(2 * ESTIMATE_PX);

    const heightB = await container.evaluate(
      (el) => el.getBoundingClientRect().height,
    );
    expect(heightB).toBeGreaterThan(0);

    // Heights should differ (different message counts)
    expect(heightA).not.toBe(heightB);
  });

  test("message virtualizer stays populated across sort toggles", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({
      timeout: 10_000,
    });

    await clickSession(page, "project-alpha", 5, 5);
    const rows = page.locator(".virtual-row");

    const sortButton = page.getByLabel("Toggle sort order");
    await sortButton.click();
    await expect(rows.first()).toBeVisible({ timeout: 5_000 });

    await sortButton.click();
    await expect(rows.first()).toBeVisible({ timeout: 5_000 });
  });

  test("session switch without explicit row count wait", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({
      timeout: 10_000,
    });

    // First select a session and wait for its rows to render
    // project-alpha, 2 msgs
    await clickSession(page, "project-alpha", 2, 2);

    // Initiate the switch to the second session and verify a
    // loading cycle occurs for the new session, proving the
    // helper doesn't silently pass on stale state.
    const targetItem = getSessionItem(page, "project-alpha", 5);
    const targetId = await targetItem.getAttribute(
      "data-session-id",
    );
    expect(targetId).toBeTruthy();

    const messageList = page.locator(".message-list-scroll");
    const prevId = await messageList.getAttribute(
      "data-messages-session-id",
    );
    await targetItem.click();
    await expect(targetItem).toHaveClass(/active/);

    // Prove the messages store actually transitioned away from the
    // previous session before asserting the final loaded state.
    // We check that data-messages-session-id is no longer the old
    // value, which confirms a load cycle started regardless of
    // whether the intermediate state shows .empty-state or keeps
    // the .message-list-scroll element in the DOM.
    await expect(messageList).not.toHaveAttribute(
      "data-messages-session-id",
      prevId!,
    );
    await expect(messageList).toHaveAttribute(
      "data-messages-session-id",
      targetId!,
    );
    await expect(messageList).toHaveAttribute(
      "data-loaded",
      "true",
    );

    // Wait for at least one row to appear
    await expect(
      page.locator(".virtual-row").first(),
    ).toBeVisible();

    // Verify we see the target session's content, not stale rows
    const rows = page.locator(".virtual-row");
    await expect(rows).toHaveCount(5);
  });
});

test.describe("Mixed content rendering", () => {
  test("tool group renders for consecutive tool-only messages", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({
      timeout: 10_000,
    });

    // mixed-content-6: project-beta, 6 msgs
    await clickSession(page, "project-beta", 6, 5);

    const toolGroup = page.locator(".tool-group");
    await expect(toolGroup).toBeVisible();
    await expect(toolGroup).toContainText(/tool calls?/i);

    const toolGroupBody = page.locator(".tool-group-body");
    await expect(toolGroupBody).toBeVisible();

    // Should contain exactly 2 tool blocks inside the group
    // (Indices 3 and 4 in the fixture are tool calls)
    const toolBlocks = toolGroupBody.locator(".tool-block");
    await expect(toolBlocks).toHaveCount(2);
  });

  test("thinking block is collapsed by default", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({
      timeout: 10_000,
    });

    await clickSession(page, "project-beta", 6, 5);

    const thinkingBlock = page.locator(".thinking-block");
    await expect(thinkingBlock).toBeVisible();

    // Content should be hidden (collapsed by default)
    const thinkingContent = page.locator(".thinking-content");
    await expect(thinkingContent).not.toBeVisible();

    // Click to expand
    const thinkingHeader = page.locator(".thinking-header");
    await thinkingHeader.click();

    // Content should now be visible
    await expect(thinkingContent).toBeVisible();
    await expect(thinkingContent).toContainText(
      "Let me analyze...",
    );
  });
});
