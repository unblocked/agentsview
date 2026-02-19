import { test, expect } from "@playwright/test";

test.describe("Virtual list behavior", () => {
  test.beforeEach(async ({ page }) => {
    // Mock the sessions API to return a large list
    await page.route("**/api/v1/sessions*", async (route) => {
      const url = new URL(route.request().url());
      const limit = Number(url.searchParams.get("limit") || "200");
      const cursor = url.searchParams.get("cursor");
      const project = url.searchParams.get("project");

      // Generate 500 mock sessions
      const total = 500;
      const sessions = Array.from({ length: total }, (_, i) => ({
        id: `session-${i}`,
        project: i % 2 === 0 ? "project-alpha" : "project-beta",
        machine: "test-machine",
        agent: "test-agent",
        first_message: `Hello from session ${i}`,
        started_at: new Date().toISOString(),
        ended_at: new Date().toISOString(),
        message_count: 10,
        created_at: new Date().toISOString(),
        file_path: `/tmp/session-${i}.json`,
      }));
      const deepTotal = 2000;
      const deepSessions = Array.from(
        { length: deepTotal },
        (_, i) => ({
          id: `deep-session-${i}`,
          project: "deep",
          machine: "test-machine",
          agent: "test-agent",
          first_message: `Hello from deep session ${i}`,
          started_at: new Date().toISOString(),
          ended_at: new Date().toISOString(),
          message_count: 10,
          created_at: new Date().toISOString(),
          file_path: `/tmp/deep-session-${i}.json`,
        }),
      );

      let filtered = sessions;
      if (project === "tiny") {
        filtered = [sessions[0]];
      } else if (project === "deep") {
        filtered = deepSessions;
      } else if (project) {
        filtered = sessions.filter((s) => s.project === project);
      }

      const startIndex = cursor ? parseInt(cursor, 10) : 0;
      const slice = filtered.slice(startIndex, startIndex + limit);
      const nextCursor =
        startIndex + limit < filtered.length
          ? (startIndex + limit).toString()
          : undefined;

      await route.fulfill({
        json: {
          sessions: slice,
          next_cursor: nextCursor,
          total: filtered.length,
        },
      });
    });

    // Mock projects to include our test project
    await page.route("**/api/v1/projects", async (route) => {
      await route.fulfill({
        json: {
          projects: [
            { name: "project-alpha", session_count: 250 },
            { name: "project-beta", session_count: 250 },
            { name: "tiny", session_count: 1 },
            { name: "deep", session_count: 2000 },
          ],
        },
      });
    });
  });

  test("loads more items when scrolling down", async ({ page }) => {
    await page.goto("/");
    const list = page.locator(".session-list-scroll");

    // Initial load should have some items
    await expect(page.locator("button.session-item").first()).toBeVisible();

    // Wait for the request for more items
    const requestPromise = page.waitForRequest(
      (req) =>
        req.url().includes("/sessions") && req.url().includes("cursor="),
    );

    // Scroll down to the bottom of the list to trigger a load
    await list.evaluate((el) => {
      el.scrollTop = el.scrollHeight;
      el.dispatchEvent(new Event("scroll"));
    });

    const request = await requestPromise;
    expect(request).toBeTruthy();

    // Verify that we are loading the next page (cursor is present)
    const url = new URL(request.url());
    expect(url.searchParams.get("cursor")).toBeTruthy();

    // Verify more items are loaded (check for an item from the end of the list)
    // The mock data has 500 items total, so checking for item 499 confirms we loaded the end
    await expect(page.getByText("Hello from session 499")).toBeVisible();
  });

  test("clamps scroll position when filtering", async ({ page }) => {
    await page.goto("/");
    const list = page.locator(".session-list-scroll");

    // Scroll down significantly
    await list.evaluate((el) => {
      el.scrollTop = 2000;
    });

    // Verify we are scrolled
    await expect.poll(async () => {
        return await list.evaluate((el) => el.scrollTop);
    }).toBeGreaterThan(0);

    // Select "tiny" project which has only 1 item
    const projectSelect = page.locator("select.project-select");
    await projectSelect.selectOption("tiny");

    // Check that scroll top is reset/clamped to 0 (or near 0)
    await expect.poll(async () => {
      return await list.evaluate((el) => el.scrollTop);
    }, { timeout: 2000 }).toBe(0);
  });

  test("keeps loading after dragging into an unloaded middle range", async ({
    page,
  }) => {
    await page.goto("/");
    const list = page.locator(".session-list-scroll");
    const projectSelect = page.locator("select.project-select");

    await projectSelect.selectOption("deep");
    await expect(
      page.getByRole("button", {
        name: /Hello from deep session 0/i,
      }),
    ).toBeVisible();

    await list.evaluate((el) => {
      el.scrollTop = (el.scrollHeight - el.clientHeight) / 2;
      el.dispatchEvent(new Event("scroll"));
    });

    await expect(
      page.getByRole("button", {
        name: /Hello from deep session 1000/i,
      }),
    ).toBeVisible({ timeout: 10000 });
  });

  test("keeps loading after dragging to the end of an unloaded range", async ({
    page,
  }) => {
    await page.goto("/");
    const list = page.locator(".session-list-scroll");
    const projectSelect = page.locator("select.project-select");

    await projectSelect.selectOption("deep");
    await expect(
      page.getByRole("button", {
        name: /Hello from deep session 0/i,
      }),
    ).toBeVisible();

    await list.evaluate((el) => {
      el.scrollTop = el.scrollHeight;
      el.dispatchEvent(new Event("scroll"));
    });

    await expect(
      page.getByRole("button", {
        name: /Hello from deep session 1999/i,
      }),
    ).toBeVisible({ timeout: 15000 });
  });
});
