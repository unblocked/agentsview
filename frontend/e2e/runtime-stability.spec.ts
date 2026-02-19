import { expect, test } from "@playwright/test";

test.describe("Runtime stability", () => {
  test("effect update-depth errors stay bounded during core interactions", async ({
    page,
  }) => {
    const runtimeErrors: string[] = [];
    const depthErrorRe =
      /effect_update_depth_exceeded|Maximum update depth exceeded/i;

    page.on("pageerror", (err) => {
      runtimeErrors.push(err.message);
    });

    page.on("console", (msg) => {
      if (msg.type() !== "error") return;
      runtimeErrors.push(msg.text());
    });

    await page.goto("/");
    const sessions = page.locator("button.session-item");
    await expect(sessions.first()).toBeVisible({
      timeout: 10_000,
    });

    // Exercise the highest-churn flows we depend on.
    // Use a smaller session to avoid conflating core
    // interaction stability with large-session perf limits.
    await sessions.nth(6).click();
    await expect(page.locator(".virtual-row").first())
      .toBeVisible({ timeout: 5_000 });

    const sortButton = page.getByLabel("Toggle sort order");
    await sortButton.click();
    await sortButton.click();

    const projectSelect = page.locator("select.project-select");
    await projectSelect.selectOption("project-alpha");
    await expect(page.locator(".session-list-header")).toContainText(
      "2 sessions",
    );
    await projectSelect.selectOption("");
    await expect(page.locator(".session-list-header")).toContainText(
      "8 sessions",
    );

    // Let async observers flush.
    await page.waitForTimeout(500);

    const depthErrors = runtimeErrors.filter((m) =>
      depthErrorRe.test(m),
    );
    // There are still known Svelte depth warnings under rapid
    // virtualizer churn; keep them bounded to prevent regressions.
    expect(depthErrors.length).toBeLessThanOrEqual(4);

    const otherErrors = runtimeErrors.filter((m) => !depthErrorRe.test(m));
    expect(otherErrors).toEqual([]);
  });
});
