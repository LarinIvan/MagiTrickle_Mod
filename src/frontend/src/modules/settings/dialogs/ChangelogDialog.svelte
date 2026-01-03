<script lang="ts">
  import { Dialog } from "bits-ui";
  import { scale } from "svelte/transition";
  import { X } from "lucide-svelte";
  import { t } from "../../../data/locale.svelte";
  import Button from "../../../components/ui/Button.svelte";
  import { fetcher } from "../../../utils/fetcher";

  type Props = {
    open: boolean;
  };

  let { open = $bindable() }: Props = $props();

  let loading = $state(false);
  let content = $state("");
  let error = $state(false);

  // Production URL (Future)
  const REPO_URL =
    "https://raw.githubusercontent.com/LarinIvan/MagiTrickle_Mod/changelog/CHANGELOG.md";

  async function loadChangelog() {
    loading = true;
    error = false;
    try {
      const res = await fetch(REPO_URL);
      if (!res.ok) throw new Error("Failed to load");
      content = await res.text();
    } catch (e) {
      console.error(e);
      error = true;
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    if (open && !content) {
      loadChangelog();
    }
  });

  // Simple Markdown Parser (Headers, Lists, Bold, Code)
  function parseMarkdown(text: string) {
    if (!text) return "";
    let html = text
      .replace(/^### (.*$)/gim, "<h3>$1</h3>")
      .replace(/^## (.*$)/gim, "<h2>$1</h2>")
      .replace(/^# (.*$)/gim, "<h1>$1</h1>")
      .replace(/^\- (.*$)/gim, "<li>$1</li>")
      .replace(/\*\*(.*)\*\*/gim, "<b>$1</b>")
      .replace(/\`(.*)\`/gim, "<code>$1</code>");

    // Wrap lists
    // Simple hack: if line starts with <li>, and previous didn't, add <ul>?
    // Easier: Just replace explicit double newlines with <br> and wrap everything in a div
    return html.replace(/\n/g, "<br />");
  }

  // A slightly better parser for lists
  function renderMarkdown(text: string) {
    const lines = text.split("\n");
    let output = "";
    let inList = false;

    for (let line of lines) {
      line = line.trim();

      // Headers
      if (line.startsWith("### ")) {
        if (inList) {
          output += "</ul>";
          inList = false;
        }
        output += `<h3>${parseInline(line.slice(4))}</h3>`;
      } else if (line.startsWith("## ")) {
        if (inList) {
          output += "</ul>";
          inList = false;
        }
        output += `<h2>${parseInline(line.slice(3))}</h2>`;
      } else if (line.startsWith("# ")) {
        if (inList) {
          output += "</ul>";
          inList = false;
        }
        output += `<h1>${parseInline(line.slice(2))}</h1>`;
      }
      // List items
      else if (line.startsWith("- ")) {
        if (!inList) {
          output += '<ul class="changelog-list">';
          inList = true;
        }
        output += `<li>${parseInline(line.slice(2))}</li>`;
      }
      // Empty lines
      else if (line === "") {
        if (inList) {
          output += "</ul>";
          inList = false;
        }
        // output += '<br/>'; // Optional: add spacing
      }
      // Paragraphs
      else {
        if (inList) {
          output += "</ul>";
          inList = false;
        }
        output += `<p>${parseInline(line)}</p>`;
      }
    }
    if (inList) output += "</ul>";
    return output;
  }

  function parseInline(text: string) {
    return text
      .replace(/\*\*(.*?)\*\*/g, "<b>$1</b>")
      .replace(/\`(.*?)\`/g, '<code class="inline-code">$1</code>');
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Portal>
    <Dialog.Overlay class="dialog-overlay" />
    <Dialog.Content class="dialog-content changelog-modal">
      <div class="dialog-header">
        <Dialog.Title class="dialog-title">{t("What's New")}</Dialog.Title>
        <Dialog.Close class="close-btn">
          <X size={20} />
        </Dialog.Close>
      </div>

      <div class="dialog-body">
        {#if loading}
          <div class="loading-state">
            <div class="spinner"></div>
            <p>{t("Loading changelog...")}</p>
          </div>
        {:else if error}
          <div class="error-state">
            <p class="error-text">{t("Failed to load changelog")}</p>
            <Button small onclick={loadChangelog}>{t("Retry")}</Button>
          </div>
        {:else}
          <div class="changelog-content">
            <!-- eslint-disable-next-line svelte/no-at-html-tags -->
            {@html renderMarkdown(content)}
          </div>
        {/if}
      </div>
    </Dialog.Content>
  </Dialog.Portal>
</Dialog.Root>

<style>
  :global(.dialog-overlay) {
    position: fixed;
    inset: 0;
    z-index: 50;
    background-color: rgba(0, 0, 0, 0.8);
    backdrop-filter: blur(4px);
  }

  :global(.dialog-content) {
    position: fixed;
    left: 50%;
    top: 50%;
    z-index: 50;
    width: 90%;
    max-width: 600px;
    max-height: 85vh;
    transform: translate(-50%, -50%);
    background-color: var(--bg-dark);
    border: 1px solid var(--border-light);
    border-radius: 1rem;
    box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.25);
    display: flex;
    flex-direction: column;
  }

  .dialog-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1rem 1.5rem;
    border-bottom: 1px solid var(--bg-light-extra);
  }

  :global(.dialog-title) {
    font-size: 1.25rem;
    font-weight: 600;
    color: var(--text);
  }

  :global(.close-btn) {
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    color: var(--text-2);
    cursor: pointer;
    padding: 0.25rem;
    border-radius: 0.25rem;
    transition: all 0.2s;
  }

  :global(.close-btn:hover) {
    background-color: var(--bg-light);
    color: var(--text);
  }

  .dialog-body {
    padding: 1.5rem;
    overflow-y: auto;
    color: var(--text);
  }

  /* Changelog Typography */
  .changelog-content :global(h1),
  .changelog-content :global(h2) {
    font-size: 1.5rem;
    font-weight: 700;
    margin-bottom: 1rem;
    margin-top: 1.5rem;
    color: var(--accent);
    border-bottom: 1px solid var(--bg-light-extra);
    padding-bottom: 0.5rem;
  }

  .changelog-content :global(h1):first-child {
    margin-top: 0;
  }

  .changelog-content :global(h3) {
    font-size: 1.1rem;
    font-weight: 600;
    margin-top: 1.25rem;
    margin-bottom: 0.5rem;
    color: var(--text);
  }

  .changelog-content :global(p) {
    margin-bottom: 0.75rem;
    line-height: 1.6;
    color: var(--text-2);
  }

  .changelog-content :global(ul) {
    margin-bottom: 1rem;
    padding-left: 1.5rem;
  }

  .changelog-content :global(li) {
    margin-bottom: 0.4rem;
    line-height: 1.5;
    list-style-type: disc;
    color: var(--text-2);
  }

  .changelog-content :global(b) {
    color: var(--text);
    font-weight: 600;
  }

  .changelog-content :global(.inline-code) {
    background: var(--bg-light);
    padding: 0.1rem 0.3rem;
    border-radius: 0.3rem;
    font-family: monospace;
    font-size: 0.9em;
  }

  /* Loading State */
  .loading-state,
  .error-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 3rem 0;
    gap: 1rem;
  }

  .spinner {
    width: 30px;
    height: 30px;
    border: 3px solid var(--bg-light-extra);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .error-text {
    color: var(--red);
  }
</style>
