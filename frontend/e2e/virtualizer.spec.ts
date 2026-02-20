import { test, expect, type Locator, type Page } from "@playwright/test";

// Session list is ordered by ended_at DESC.
// We select sessions by stable properties (project + message count)
// rather than unstable indices.
const ESTIMATE_PX = 120;

const SESSIONS = {
  ALPHA_5: { project: "project-alpha", count: 5, displayRows: 5 },
  ALPHA_2: { project: "project-alpha", count: 2, displayRows: 2 },
  BETA_6: { project: "project-beta", count: 6, displayRows: 5 },
};

function getSessionItem(page: Page, project: string, count: number) {
  return page
    .locator(".session-item")
    .filter({
      has: page.locator(`.session-project:text-is("${project}")`),
    })
    .filter({
      has: page.locator(`.session-count:text-is("${count}")`),
    });
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
    // Wait for the virtual list to settle on the target session's
    // item count (assumes small item counts fit within
    // viewport+overscan)
    await expect(page.locator(".virtual-row")).toHaveCount(
      expectedRows,
    );
  } else {
    // Wait for the list to render at least one item
    await expect(
      page.locator(".virtual-row").first(),
    ).toBeVisible();
  }
}

/**
 * Wait until the virtualizer has measured items and the container
 * height no longer equals the initial estimate.
 */
async function waitForLayoutSettle(page: Page, itemCount: number) {
  const container = page
    .locator(".message-list-scroll > div")
    .first();
  await expect
    .poll(async () => {
      return container.evaluate(
        (el) => el.getBoundingClientRect().height,
      );
    })
    .not.toBe(itemCount * ESTIMATE_PX);
  return container;
}

/**
 * Verify there are no vertical gaps (> 1px) between consecutive
 * virtual rows by checking translateY positions against heights.
 */
async function verifyNoVerticalGaps(rows: Locator) {
  const positions = await rows.evaluateAll((els) =>
    els.map((el) => {
      const style = el.style.transform;
      const match = style.match(
        /translateY\((\d+(?:\.\d+)?)px\)/,
      );
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
}

test.describe("Virtualizer measurement", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await expect(
      page.locator("button.session-item").first(),
    ).toBeVisible({ timeout: 5_000 });
  });

  test("items are measured to actual DOM height", async ({
    page,
  }) => {
    const { project, count, displayRows } = SESSIONS.ALPHA_5;
    await clickSession(page, project, count, displayRows);

    const rows = page.locator(".virtual-row");
    const container = await waitForLayoutSettle(
      page,
      displayRows,
    );

    const totalHeight = await container.evaluate(
      (el) => el.getBoundingClientRect().height,
    );
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
    const { project, count, displayRows } = SESSIONS.ALPHA_5;
    await clickSession(page, project, count, displayRows);

    const rows = page.locator(".virtual-row");
    await waitForLayoutSettle(page, displayRows);
    await verifyNoVerticalGaps(rows);
  });

  test("total container height matches sum of items", async ({
    page,
  }) => {
    const { project, count, displayRows } = SESSIONS.ALPHA_5;
    await clickSession(page, project, count, displayRows);

    const rows = page.locator(".virtual-row");
    const container = await waitForLayoutSettle(
      page,
      displayRows,
    );

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
    const sessionA = SESSIONS.ALPHA_5;
    await clickSession(
      page,
      sessionA.project,
      sessionA.count,
      sessionA.displayRows,
    );

    const container = await waitForLayoutSettle(
      page,
      sessionA.displayRows,
    );

    const heightA = await container.evaluate(
      (el) => el.getBoundingClientRect().height,
    );
    expect(heightA).toBeGreaterThan(0);

    const sessionB = SESSIONS.ALPHA_2;
    await clickSession(
      page,
      sessionB.project,
      sessionB.count,
      sessionB.displayRows,
    );

    await waitForLayoutSettle(page, sessionB.displayRows);

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
    const { project, count, displayRows } = SESSIONS.ALPHA_5;
    await clickSession(page, project, count, displayRows);
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
    const sessionA = SESSIONS.ALPHA_2;
    await clickSession(
      page,
      sessionA.project,
      sessionA.count,
      sessionA.displayRows,
    );

    const sessionB = SESSIONS.ALPHA_5;
    const targetItem = getSessionItem(
      page,
      sessionB.project,
      sessionB.count,
    );
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

    // Prove the messages store actually transitioned away from
    // the previous session before asserting the final loaded
    // state.
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

    await expect(
      page.locator(".virtual-row").first(),
    ).toBeVisible();

    const rows = page.locator(".virtual-row");
    await expect(rows).toHaveCount(sessionB.displayRows);
  });
});

test.describe("Mixed content rendering", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await expect(
      page.locator("button.session-item").first(),
    ).toBeVisible({ timeout: 5_000 });
  });

  test("tool group renders for consecutive tool-only messages", async ({
    page,
  }) => {
    const { project, count, displayRows } = SESSIONS.BETA_6;
    await clickSession(page, project, count, displayRows);

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
    const { project, count, displayRows } = SESSIONS.BETA_6;
    await clickSession(page, project, count, displayRows);

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
