import { test, expect } from "@playwright/test";

test.describe("Session list", () => {
  test("sessions load and display", async ({ page }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });
    expect(await items.count()).toBe(8);
  });

  test("session count header is visible", async ({ page }) => {
    await page.goto("/");
    const header = page.locator(".session-list-header");
    await expect(header).toBeVisible({ timeout: 10_000 });
    await expect(header).toContainText("sessions");
  });

  test("clicking a session marks it active", async ({ page }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });

    await items.first().click();
    await expect(items.first()).toHaveClass(/active/);
  });

  test("project filter changes do not blank virtualized list", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    const header = page.locator(".session-list-header");
    const projectSelect = page.locator("select.project-select");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });

    await projectSelect.selectOption("project-alpha");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });
    await expect(header).toContainText("2 sessions");
    await expect(items).toHaveCount(2);

    await projectSelect.selectOption("project-beta");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });
    await expect(header).toContainText("3 sessions");
    await expect(items).toHaveCount(3);

    await projectSelect.selectOption("");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });
    await expect(header).toContainText("8 sessions");
    await expect(items).toHaveCount(8);
  });
});
