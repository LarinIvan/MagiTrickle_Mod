<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { slide } from "svelte/transition";
  import GenericDialog from "../../../components/ui/GenericDialog.svelte";
  import Button from "../../../components/ui/Button.svelte";
  import Select from "../../../components/ui/Select.svelte";
  import { RULE_TYPES } from "../../../types";
  import { defaultRule } from "../../../utils/defaults";
  import {
    isValidSubnet,
    isValidSubnet6,
    isValidNamespace,
    isValidDomain,
    isValidWildcard,
    isValidRegex,
    VALIDATOP_MAP,
  } from "../../../utils/rule-validators";
  import type { Rule } from "../../../types";
  import { toast } from "../../../utils/events";
  import { t } from "../../../data/locale.svelte";
  import { Search, Loader2, X } from "lucide-svelte";

  export let open = false;
  export let group_index: number | null = null;

  const dispatch = createEventDispatcher();
  let import_rules_text = "";
  let triedSubmit = false;

  const RULE_TYPE_SELECT = [{ value: "auto", label: "Auto" }, ...RULE_TYPES];
  type RuleTypeValue = (typeof RULE_TYPE_SELECT)[number]["value"];
  let selectedRuleType: RuleTypeValue = "auto";

  // --- List Download Feature ---
  let searchTerm = "";
  let allFiles: string[] = [];
  let filteredFiles: string[] = [];
  let isLoadingList = false;
  let isLoadingContent = false;
  let showResults = false;
  let searchContainer: HTMLElement; // Ref for click outside

  async function fetchFileList() {
    if (allFiles.length > 0) return; // Cache
    isLoadingList = true;
    try {
      // Use Tree API
      // v2fly/domain-list-community/data folder
      const response = await fetch(
        "https://api.github.com/repos/v2fly/domain-list-community/git/trees/master?recursive=1",
      );
      if (!response.ok) throw new Error("Failed to fetch list");
      const data = await response.json();

      allFiles = data.tree
        .filter((item: any) => item.path.startsWith("data/") && item.type === "blob")
        .map((item: any) => item.path.replace("data/", ""));
    } catch (e) {
      console.error(e);
      toast.error(t("Failed to load community lists"));
    } finally {
      isLoadingList = false;
    }
  }

  // Click Outside Handler
  function handleWindowClick(e: MouseEvent) {
    if (showResults && searchContainer && !searchContainer.contains(e.target as Node)) {
      showResults = false;
    }
  }

  async function handleFocus() {
    if (allFiles.length === 0 && !isLoadingList) {
      await fetchFileList();
    }
    filterFiles();
  }

  function handleSearchInput() {
    if (allFiles.length === 0 && !isLoadingList) {
      fetchFileList().then(() => filterFiles());
    } else {
      filterFiles();
    }
  }

  function filterFiles() {
    const term = searchTerm.toLowerCase();

    if (!term.trim()) {
      filteredFiles = allFiles; // No limit
      showResults = true;
      return;
    }

    // Improve scoring/sorting
    filteredFiles = allFiles
      .filter((f) => f.toLowerCase().includes(term))
      .sort((a, b) => {
        const lowerA = a.toLowerCase();
        const lowerB = b.toLowerCase();

        // Exact match puts first
        if (lowerA === term && lowerB !== term) return -1;
        if (lowerB === term && lowerA !== term) return 1;

        // Starts with puts higher
        const startA = lowerA.startsWith(term);
        const startB = lowerB.startsWith(term);
        if (startA && !startB) return -1;
        if (startB && !startA) return 1;

        return lowerA.localeCompare(lowerB);
      });

    showResults = true;
  }

  async function loadListContent(filename: string) {
    isLoadingContent = true;
    showResults = false; // Hide results immediately
    searchTerm = filename; // Update input to show selected
    try {
      const response = await fetch(
        `https://raw.githubusercontent.com/v2fly/domain-list-community/master/data/${filename}`,
      );
      if (!response.ok) throw new Error("Failed to fetch content");
      const text = await response.text();
      import_rules_text = text;
      toast.success(t("Loaded list: ") + filename);
    } catch (e) {
      console.error(e);
      toast.error(t("Failed to download list content"));
    } finally {
      isLoadingContent = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      e.preventDefault();
      submit();
    }
  }

  function detectRuleType(pattern: string): keyof typeof VALIDATOP_MAP {
    const p = pattern.trim();

    if (isValidSubnet6(p)) return "subnet6";
    if (isValidSubnet(p)) return "subnet";
    if (p.split(".").length >= 3 && isValidDomain(p)) return "domain";
    if (isValidNamespace(p)) return "namespace";
    if (isValidRegex(p)) return "regex";
    if (isValidWildcard(p)) return "wildcard";

    return "domain";
  }

  function processLine(
    rawLine: string,
  ): { rule: string; type: RuleTypeValue; name: string } | null {
    // Ignore include: (reserved for future)
    if (rawLine.includes("include:")) return null;

    // Ignore lines starting with http
    if (rawLine.trim().startsWith("http")) return null;

    let line = rawLine;
    let comment = "";
    let attributes = "";
    let prefix = "";

    // Extract comments (# and everything after)
    const commentSplit = line.split("#");
    if (commentSplit.length > 1) {
      // Keep everything after the first # as the comment part
      comment = "#" + commentSplit.slice(1).join("#");
      line = commentSplit[0];
    }
    line = line.trim();

    if (!line) return null;

    // Extract attributes (anything starting with @)
    const atIndex = line.indexOf("@");
    if (atIndex !== -1) {
      attributes = line.slice(atIndex).trim();
      line = line.slice(0, atIndex).trim();
    }

    if (!line) return null;

    let type: RuleTypeValue = "auto";
    let rule = line;
    let prefixType: RuleTypeValue | null = null;

    // Extract Prefixes and Determine Type
    if (line.startsWith("full:")) {
      prefixType = "domain";
      prefix = "full:";
      rule = line.slice(5);
    } else if (line.startsWith("domain:")) {
      prefixType = "namespace";
      prefix = "domain:";
      rule = line.slice(7);
    } else if (line.startsWith("regexp:")) {
      prefixType = "regex";
      prefix = "regexp:";
      rule = line.slice(7);
    }
    // Generic Check for unknown prefixes (e.g. hello:super.com)
    // Must NOT match IPv6 addresses
    else if (line.includes(":") && !isValidSubnet6(line)) {
      const colonIndex = line.indexOf(":");
      const possibleTag = line.slice(0, colonIndex);

      if (/^[a-zA-Z0-9_-]+$/.test(possibleTag)) {
        prefix = possibleTag + ":";
        rule = line.slice(colonIndex + 1);
      }
    }

    // Determine final type
    if (selectedRuleType === "auto") {
      type = prefixType || detectRuleType(rule);
    } else {
      type = selectedRuleType;
    }

    // Construct Name from collected metadata
    const nameParts = [prefix, attributes, comment].map((p) => p.trim()).filter(Boolean);
    const ruleName = nameParts.join(" ");

    return { rule, type, name: ruleName };
  }

  function submit() {
    triedSubmit = true;

    if (!import_rules_text.trim() || group_index === null) return;

    const seen = new Set<string>();
    const lines = import_rules_text.split(/[\n,]+/);
    const validRules: { rule: string; type: RuleTypeValue; name: string }[] = [];

    for (const rawLine of lines) {
      const result = processLine(rawLine);
      if (!result) continue;

      const { rule, type, name } = result;

      // Additional safety check: skip empty rules after processing
      if (!rule) continue;

      const key = `${type}|${rule}`;
      if (seen.has(key)) continue;
      seen.add(key);

      validRules.push({ rule, type, name });
    }

    if (!validRules.length) {
      toast.info(t("No valid rules to import"));
      return;
    }

    const rules: Rule[] = validRules.map(({ rule, type, name }) => ({
      ...defaultRule(),
      rule: rule,
      type: type,
      name: name,
    }));

    dispatch("import", { group_index, rules });
    toast.success(t("Imported rules: ") + rules.length);
    close();
  }

  function close() {
    import_rules_text = "";
    triedSubmit = false;
    selectedRuleType = "auto";
    searchTerm = "";
    showResults = false;
    dispatch("close");
  }

  function getHighlightedSegments(text: string, term: string) {
    if (!term) return [{ text, isMatch: false }];
    // Escape regex special characters
    const escapedTerm = term.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const regex = new RegExp(`(${escapedTerm})`, "gi");
    return text
      .split(regex)
      .map((part) => ({
        text: part,
        isMatch: part.toLowerCase() === term.toLowerCase(),
      }))
      .filter((p) => p.text);
  }
</script>

<svelte:window on:keydown={handleKeydown} on:click={handleWindowClick} />
<GenericDialog
  {open}
  {triedSubmit}
  title={t("Import Rule List")}
  on:close={close}
  on:submit={submit}
  maxWidth={600}
>
  <div slot="body" class="dialog-body-custom">
    <div class="search-container" bind:this={searchContainer}>
      <div class="input-wrapper">
        {#if isLoadingList || isLoadingContent}
          <div class="icon-wrapper">
            <Loader2 class="search-icon animate-spin" size="18" />
          </div>
        {:else}
          <div class="icon-wrapper">
            <Search class="search-icon" size="18" />
          </div>
        {/if}
        <input
          type="text"
          placeholder={t("Search community lists (e.g. 'google', 'telegram')...")}
          bind:value={searchTerm}
          on:input={handleSearchInput}
          on:click={handleFocus}
          on:focus={handleFocus}
          class="search-input"
        />
        {#if searchTerm || showResults}
          <button
            type="button"
            class="clear-btn"
            on:click={() => {
              searchTerm = "";
              showResults = false;
            }}
          >
            <X size="16" />
          </button>
        {/if}
      </div>

      {#if showResults && filteredFiles.length > 0}
        <div class="results-dropdown" transition:slide={{ duration: 550, axis: "y" }}>
          {#each filteredFiles as file}
            <button type="button" class="result-item" on:click={() => loadListContent(file)}>
              <div class="file-icon">📄</div>
              <span class="file-name">
                {#each getHighlightedSegments(file, searchTerm) as segment}
                  <span class:highlight={segment.isMatch}>{segment.text}</span>
                {/each}
              </span>
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <textarea
      bind:value={import_rules_text}
      placeholder={t("Insert a list of IPs or domains, one per line")}
      class:invalid={triedSubmit && !import_rules_text.trim()}
      class="rules-textarea"
    ></textarea>
  </div>

  <div slot="actions" class="rule-type-select">
    <Select options={RULE_TYPE_SELECT} bind:selected={selectedRuleType} />
    <Button type="submit" on:click={submit}>{t("Import")}</Button>
  </div>
</GenericDialog>

<style>
  .dialog-body-custom {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    width: 100%;
    height: 70vh;
  }

  .rules-textarea {
    width: 100%;
    flex: 1;
    resize: none;
    font: inherit;
    padding: 0.75rem 1rem;
    border-radius: 0.5rem;
    border: 1.5px solid var(--bg-light-extra);
    background: var(--bg-light);
    color: var(--text);
    box-sizing: border-box;
    transition:
      border 0.15s,
      box-shadow 0.15s;
  }
  .rules-textarea:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-light, #aaf2ff33);
  }
  .rules-textarea.invalid {
    border-color: var(--danger) !important;
  }

  .search-container {
    position: relative;
    width: 100%;
    z-index: 10;
    flex-shrink: 0;
  }

  .results-dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    background-color: var(--bg-light);
    border: 1px solid var(--border);
    border-radius: 0.5rem;
    margin-top: 0.5rem;
    max-height: 65vh;
    overflow-y: auto;
    z-index: 100;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  }

  .input-wrapper {
    position: relative;
    display: flex;
    align-items: center;
    background-color: var(--bg-light);
    border: 1px solid var(--border);
    border-radius: 0.5rem;
    padding: 0 0.75rem;
    height: 2.75rem;
    transition: border-color 0.2s;
  }

  .input-wrapper:focus-within {
    border-color: var(--primary);
    box-shadow: 0 0 0 2px rgba(var(--primary-rgb), 0.1);
  }

  .search-input {
    flex: 1;
    background: transparent;
    border: none;
    color: var(--text);
    font-size: 0.9rem;
    padding: 0 0.5rem;
    outline: none;
    min-width: 0;
  }

  .icon-wrapper {
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-secondary);
  }

  .icon-wrapper :global(.search-icon) {
    color: currentColor;
  }

  .icon-wrapper :global(.search-icon.animate-spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }

  .clear-btn {
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 0.9rem;
    padding: 0.25rem;
    line-height: 1;
    border-radius: 50%;
  }
  .clear-btn:hover {
    background-color: var(--bg-hover);
    color: var(--text);
  }

  .results-dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    background-color: var(--bg-dark);
    border: 1px solid var(--border);
    border-radius: 0.5rem;
    margin-top: 0.5rem;
    max-height: 70vh;
    height: 70vh;
    overflow-y: auto;
    z-index: 9999;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
  }

  .result-item {
    display: flex;
    align-items: center;
    width: 100%;
    padding: 0.75rem 1rem;
    background: none;
    border: none;
    text-align: left;
    cursor: pointer;
    color: var(--text);
    transition: background-color 0.15s;
    gap: 0.75rem;
  }

  .result-item:hover {
    background-color: var(--bg-light-extra);
  }

  .result-item:not(:last-child) {
    border-bottom: 1px solid var(--border-light);
  }

  .file-icon {
    color: var(--text-secondary);
    opacity: 0.7;
  }

  .file-name {
    font-size: 0.9rem;
    font-family: monospace;
  }

  .highlight {
    font-weight: 700;
    color: var(--primary);
    background-color: rgba(var(--primary-rgb), 0.1);
    border-radius: 2px;
  }

  .rule-type-select :global(.select-wrap),
  .rule-type-select :global([data-select-root]),
  .rule-type-select :global([data-select-trigger]) {
    display: block !important;
    width: 100% !important;
    box-sizing: border-box;
  }
  .rule-type-select :global([data-select-trigger] .selected) {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100% !important;
  }
  .rule-type-select :global([data-select-trigger] .selected-value) {
    color: var(--text-2);
  }
  .rule-type-select :global([data-select-content]) {
    min-width: 100% !important;
    width: auto;
  }
  .rule-type-select {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.5rem;
    width: 100%;
  }
  .rule-type-select :global(.select-root),
  .rule-type-select :global([data-select-trigger]) {
    width: 100% !important;
    justify-content: space-between;
  }
  .rule-type-select :global(.select-root),
  .rule-type-select :global(button) {
    height: 2.5rem;
    font-size: 0.95rem;
    border: 1px solid var(--bg-light-extra);
    border-radius: 0.5rem;
    background-color: var(--bg-light);
  }
  .rule-type-select :global(button) {
    padding: 0 1rem;
  }
  .rule-type-select :global([data-select-trigger]:hover),
  .rule-type-select :global(button:hover) {
    background-color: var(--bg-light-extra);
  }
  @media (max-width: 315px) {
    .rule-type-select {
      grid-template-columns: 1fr;
    }
  }

  .results-dropdown::-webkit-scrollbar,
  .rules-textarea::-webkit-scrollbar {
    width: 10px;
  }
  .results-dropdown::-webkit-scrollbar-button,
  .rules-textarea::-webkit-scrollbar-button {
    display: none;
  }
  .results-dropdown::-webkit-scrollbar-track,
  .rules-textarea::-webkit-scrollbar-track {
    background-color: rgba(0, 0, 0, 0.2);
    border-radius: 5px;
  }
  .results-dropdown::-webkit-scrollbar-thumb,
  .rules-textarea::-webkit-scrollbar-thumb {
    background-color: var(--text-2);
    border-radius: 5px;
    border: 2px solid transparent;
    background-clip: content-box;
  }
  .results-dropdown::-webkit-scrollbar-thumb:hover,
  .rules-textarea::-webkit-scrollbar-thumb:hover {
    background-color: var(--text);
  }
</style>
