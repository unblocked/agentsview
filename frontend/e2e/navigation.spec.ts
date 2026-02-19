import { test, expect } from "@playwright/test";

test.describe("Navigation", () => {
  test("minimap renders with non-zero canvas", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });

    await items.first().click();

    // Wait for messages to load so minimap has data
    const messages = page.locator(".virtual-row");
    await expect(messages.first()).toBeVisible({
      timeout: 5_000,
    });

    const canvas = page.locator("canvas");
    await expect(canvas).toBeVisible();

    const box = await canvas.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.width).toBeGreaterThan(0);
    expect(box!.height).toBeGreaterThan(0);
  });

  test("keyboard ] navigates to next session", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });

    // Select first session
    await items.first().click();
    await expect(items.first()).toHaveClass(/active/);

    // Press ] to go to next session
    await page.keyboard.press("]");
    await expect(items.nth(1)).toHaveClass(/active/);
  });

  test("keyboard [ navigates to previous session", async ({
    page,
  }) => {
    await page.goto("/");
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });

    // Select second session
    await items.nth(1).click();
    await expect(items.nth(1)).toHaveClass(/active/);

    // Press [ to go back
    await page.keyboard.press("[");
    await expect(items.first()).toHaveClass(/active/);
  });

  test("empty state shows when no session selected", async ({
    page,
  }) => {
    await page.goto("/");

    // Wait for sessions to load
    const items = page.locator("button.session-item");
    await expect(items.first()).toBeVisible({ timeout: 10_000 });

    // No session selected — empty state should show
    const empty = page.locator(".empty-state");
    await expect(empty).toBeVisible();
    await expect(empty).toContainText("Select a session");
  });
});
