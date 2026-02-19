<script lang="ts">
  import { ui } from "../../stores/ui.svelte.js";
  import { sessions } from "../../stores/sessions.svelte.js";
  import {
    getGithubConfig,
    setGithubConfig,
    publishSession,
  } from "../../api/client.js";
  import type { PublishResponse } from "../../api/types.js";

  type View = "setup" | "progress" | "success" | "error";

  let view: View = $state("progress");
  let tokenInput: string = $state("");
  let errorMessage: string = $state("");
  let result: PublishResponse | null = $state(null);

  async function init() {
    try {
      const config = await getGithubConfig();
      if (config.configured) {
        await doPublish();
      } else {
        view = "setup";
      }
    } catch {
      view = "setup";
    }
  }

  async function handleSaveToken() {
    const token = tokenInput.trim();
    if (!token) return;

    view = "progress";
    try {
      await setGithubConfig(token);
      await doPublish();
    } catch (err) {
      errorMessage =
        err instanceof Error ? err.message : "Failed to save token";
      view = "error";
    }
  }

  async function doPublish() {
    const id = sessions.activeSessionId;
    if (!id) {
      errorMessage = "No session selected";
      view = "error";
      return;
    }

    view = "progress";
    try {
      result = await publishSession(id);
      view = "success";
    } catch (err) {
      errorMessage =
        err instanceof Error ? err.message : "Publish failed";
      view = "error";
    }
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
  }

  function handleOverlayClick(e: MouseEvent) {
    if (
      (e.target as HTMLElement).classList.contains(
        "publish-overlay",
      )
    ) {
      ui.closePublishModal();
    }
  }

  init();
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="publish-overlay"
  onclick={handleOverlayClick}
  onkeydown={(e) => {
    if (e.key === "Escape") ui.closePublishModal();
  }}
>
  <div class="publish-modal">
    <div class="modal-header">
      <h3 class="modal-title">Publish to GitHub Gist</h3>
      <button
        class="close-btn"
        onclick={() => ui.closePublishModal()}
      >
        &times;
      </button>
    </div>

    <div class="modal-body">
      {#if view === "setup"}
        <p class="setup-text">
          Enter a GitHub personal access token with the
          <code>gist</code> scope.
        </p>
        <input
          class="token-input"
          type="password"
          placeholder="ghp_..."
          bind:value={tokenInput}
          onkeydown={(e) => {
            if (e.key === "Enter") handleSaveToken();
          }}
        />
        <div class="setup-actions">
          <a
            class="token-link"
            href="https://github.com/settings/tokens/new?scopes=gist"
            target="_blank"
            rel="noopener noreferrer"
          >
            Create token on GitHub
          </a>
          <button
            class="btn btn-primary"
            onclick={handleSaveToken}
            disabled={!tokenInput.trim()}
          >
            Save & Publish
          </button>
        </div>

      {:else if view === "progress"}
        <div class="progress-view">
          <div class="spinner"></div>
          <p>Creating GitHub Gist...</p>
        </div>

      {:else if view === "success" && result}
        <div class="success-view">
          <div class="url-field">
            <label class="url-label" for="publish-view-url">
              View URL
            </label>
            <div class="url-row">
              <input
                id="publish-view-url"
                class="url-input"
                type="text"
                readonly
                value={result.view_url}
              />
              <button
                class="btn btn-copy"
                onclick={() => copyToClipboard(result!.view_url)}
              >
                Copy
              </button>
            </div>
          </div>
          <div class="url-field">
            <label class="url-label" for="publish-gist-url">
              Gist URL
            </label>
            <div class="url-row">
              <input
                id="publish-gist-url"
                class="url-input"
                type="text"
                readonly
                value={result.gist_url}
              />
              <button
                class="btn btn-copy"
                onclick={() => copyToClipboard(result!.gist_url)}
              >
                Copy
              </button>
            </div>
          </div>
          <div class="success-actions">
            <button
              class="btn btn-primary"
              onclick={() => window.open(result!.view_url, "_blank")}
            >
              Open in Browser
            </button>
            <button
              class="btn"
              onclick={() => ui.closePublishModal()}
            >
              Close
            </button>
          </div>
        </div>

      {:else if view === "error"}
        <div class="error-view">
          <p class="error-message">{errorMessage}</p>
          <div class="error-actions">
            <button class="btn btn-primary" onclick={doPublish}>
              Retry
            </button>
            <button
              class="btn"
              onclick={() => ui.closePublishModal()}
            >
              Close
            </button>
          </div>
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  .publish-overlay {
    position: fixed;
    inset: 0;
    background: var(--overlay-bg);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .publish-modal {
    width: 440px;
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md);
    overflow: hidden;
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-default);
  }

  .modal-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .close-btn {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 16px;
    color: var(--text-muted);
    border-radius: var(--radius-sm);
  }

  .close-btn:hover {
    background: var(--bg-surface-hover);
    color: var(--text-primary);
  }

  .modal-body {
    padding: 16px;
  }

  .setup-text {
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 12px;
  }

  .setup-text code {
    font-family: var(--font-mono);
    background: var(--bg-inset);
    padding: 1px 4px;
    border-radius: var(--radius-sm);
  }

  .token-input {
    width: 100%;
    height: 32px;
    padding: 0 8px;
    background: var(--bg-inset);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-sm);
    font-size: 12px;
    font-family: var(--font-mono);
    color: var(--text-primary);
    margin-bottom: 12px;
  }

  .token-input:focus {
    outline: none;
    border-color: var(--accent-blue);
  }

  .setup-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .token-link {
    font-size: 11px;
    color: var(--accent-blue);
    text-decoration: none;
  }

  .token-link:hover {
    text-decoration: underline;
  }

  .progress-view {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 24px 0;
    color: var(--text-secondary);
    font-size: 12px;
  }

  .spinner {
    width: 24px;
    height: 24px;
    border: 2px solid var(--border-default);
    border-top-color: var(--accent-blue);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .success-view {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .url-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .url-label {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .url-row {
    display: flex;
    gap: 4px;
  }

  .url-input {
    flex: 1;
    height: 28px;
    padding: 0 8px;
    background: var(--bg-inset);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-sm);
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--text-secondary);
    min-width: 0;
  }

  .btn {
    height: 28px;
    padding: 0 12px;
    border-radius: var(--radius-sm);
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    background: var(--bg-surface-hover);
    color: var(--text-secondary);
    border: 1px solid var(--border-default);
  }

  .btn:hover {
    background: var(--bg-inset);
    color: var(--text-primary);
  }

  .btn-primary {
    background: var(--accent-blue);
    color: white;
    border-color: var(--accent-blue);
  }

  .btn-primary:hover {
    opacity: 0.9;
  }

  .btn-primary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-copy {
    flex-shrink: 0;
  }

  .success-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
    margin-top: 4px;
  }

  .error-view {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .error-message {
    font-size: 12px;
    color: var(--accent-red, #f85149);
    background: var(--bg-inset);
    padding: 8px 12px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--accent-red, #f85149);
    word-break: break-word;
  }

  .error-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }
</style>
