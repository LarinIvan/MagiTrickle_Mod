<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import GenericDialog from "../../../components/ui/GenericDialog.svelte";
  import Button from "../../../components/ui/Button.svelte";
  import Checkbox from "../../../components/ui/Checkbox.svelte";
  import Switch from "../../../components/ui/Switch.svelte";
  import { Add } from "../../../components/ui/icons";
  import type { Group } from "../../../types";
  import { t } from "../../../data/locale.svelte";

  export let open = false;
  export let groups: Group[] = [];
  export let fileName = "";

  const dispatch = createEventDispatcher<{
    close: void;
    import: { groups: Group[]; replace: boolean };
    file: { file: File };
  }>();

  let triedSubmit = false;
  let selection = new Set<number>();
  let wasOpen = false;
  let replaceConfig = false;

  $: if (open && !wasOpen) {
    selection = new Set(groups.map((_, index) => index));
    triedSubmit = false;
    replaceConfig = false;
  }

  $: wasOpen = open;
  $: selectedCount = selection.size;
  $: hasGroups = groups.length > 0;

  let isDragging = false;
  let fileInput: HTMLInputElement;

  function handleFileSelect(event: Event) {
    const input = event.target as HTMLInputElement;
    if (input.files?.length) {
      dispatch("file", { file: input.files[0] });
      input.value = ""; // Reset
    }
  }

  function onDragOver(e: DragEvent) {
    e.preventDefault();
    isDragging = true;
  }

  function onDragLeave(e: DragEvent) {
    e.preventDefault();
    isDragging = false;
  }

  function onDrop(e: DragEvent) {
    e.preventDefault();
    isDragging = false;
    if (e.dataTransfer?.files.length) {
      dispatch("file", { file: e.dataTransfer.files[0] });
    }
  }

  function toggleGroup(index: number, checked: boolean) {
    const next = new Set(selection);
    if (checked) {
      next.add(index);
    } else {
      next.delete(index);
    }
    selection = next;
  }

  function selectAll(value: boolean) {
    selection = value ? new Set(groups.map((_, index) => index)) : new Set();
  }

  function submit() {
    const selectedGroups = groups.filter((_, index) => selection.has(index));

    if (!selectedGroups.length) {
      triedSubmit = true;
      return;
    }

    dispatch("import", { groups: selectedGroups, replace: replaceConfig });
    close();
  }

  function close() {
    dispatch("close");
  }

  function handleSelectAll(e: CustomEvent<{ checked: boolean }>) {
    selectAll(e.detail.checked);
  }

  $: allSelected = groups.length > 0 && selection.size === groups.length;
</script>

<GenericDialog
  {open}
  title={t("Import Config")}
  textareaValue=""
  textareaPlaceholder=""
  {triedSubmit}
  maxWidth={600}
  on:close={close}
  on:submit={submit}
>
  <div slot="body" class="import-config">
    <p class="file-name">
      {#if hasGroups}
        {#if fileName}
          {t("Select groups to import")} — {fileName}
        {:else}
          {t("Select groups to import")}
        {/if}
      {:else}
        {t("Select config file")}
      {/if}
    </p>

    {#if hasGroups}
      <div class="mode-switch-container">
        <div class="mode-switch">
          <Switch bind:checked={replaceConfig} />
          <button class="mode-label" type="button" onclick={() => (replaceConfig = !replaceConfig)}>
            {t("Replace current configuration")}
          </button>
        </div>
        <p class="mode-description" class:replace={replaceConfig}>
          {#if replaceConfig}
            {t("Replace Mode Description")}
          {:else}
            {t("Merge Mode Description")}
          {/if}
        </p>
      </div>

      <div class="list-header">
        <div class="select-all-container">
          <Checkbox checked={allSelected} on:change={handleSelectAll} />
          <span
            class="select-all-label"
            role="button"
            tabindex="0"
            onclick={() => selectAll(!allSelected)}
            onkeydown={(e) => (e.key === "Enter" || e.key === " ") && selectAll(!allSelected)}
          >
            {t("Select all")}
          </span>
        </div>
        <span class="groups-label">{t("Groups")}</span>
      </div>

      <div class="group-list">
        {#each groups as group, index (index)}
          <label class="group-option">
            <Checkbox
              checked={selection.has(index)}
              on:change={(event) => toggleGroup(index, event.detail.checked)}
            />
            <div class="group-info">
              <span class="group-name">{group.name || `${t("Group")} ${index + 1}`}</span>
              <span class="group-meta">#{group.rules.length}</span>
            </div>
          </label>
        {/each}
      </div>
      {#if triedSubmit && !selectedCount}
        <div class="validation">{t("Select at least one group")}</div>
      {/if}
    {:else}
      <!-- File Selection Mode -->
      <div
        class="dropzone"
        class:dragging={isDragging}
        role="button"
        tabindex="0"
        ondragover={onDragOver}
        ondragleave={onDragLeave}
        ondrop={onDrop}
        onclick={() => fileInput?.click()}
        onkeydown={(e) => (e.key === "Enter" || e.key === " ") && fileInput?.click()}
      >
        <input
          type="file"
          hidden
          accept=".mtrickle"
          bind:this={fileInput}
          onchange={handleFileSelect}
        />
        <div class="dropzone-icon">
          <Add size={48} />
        </div>
        <p class="dropzone-text">{t("Drop file here or click to upload")}</p>
      </div>
    {/if}
  </div>
  <div slot="actions" class="dialog-actions">
    {#if hasGroups}
      <div class="selected-count">
        {selectedCount} / {groups.length}
      </div>
      <div class="buttons">
        <Button type="button" onclick={close}>{t("Cancel")}</Button>
        <Button type="submit">{t("Import")}</Button>
      </div>
    {:else}
      <div class="spacer"></div>
      <div class="buttons">
        <Button type="button" onclick={close}>{t("Cancel")}</Button>
      </div>
    {/if}
  </div>
</GenericDialog>

<style>
  .import-config {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding-top: 0.25rem;
  }

  .file-name {
    font-size: 0.95rem;
    color: var(--text-2);
    margin: 0;
    line-height: 1.4;
    margin-bottom: 0.5rem;
  }

  .mode-switch-container {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .mode-switch {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .mode-label {
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    font-size: 0.9rem;
    color: var(--text);
    cursor: pointer;
    user-select: none;
  }

  .mode-description {
    margin: 0;
    font-size: 0.8rem;
    line-height: 1.3;
    transition: color 0.15s ease;
    padding-left: 3rem; /* Align with text start approximately */
  }

  .mode-description:not(.replace) {
    color: #4ade80; /* Green */
  }

  .mode-description.replace {
    color: #fb923c; /* Orange */
  }

  .list-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 1rem;
    margin-top: 0.5rem;
    /* Removed negative margin to prevent overlap issues */
  }

  .select-all-container {
    display: flex;
    align-items: center;
    gap: 0.75rem; /* Match group-option gap */
  }

  .select-all-label {
    font-size: 0.9rem;
    color: var(--text);
    cursor: pointer;
    user-select: none;
  }

  .groups-label {
    font-size: 0.8rem;
    color: var(--text-2);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-weight: 600;
  }

  .group-list {
    max-height: 50vh; /* Responsive height instead of fixed 400px */
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding: 0 1rem 0.25rem 0;
    scrollbar-gutter: stable;
  }

  @media (max-height: 700px) {
    .group-list {
      max-height: 40vh; /* Even smaller on short screens */
    }
    .import-config {
      padding-top: 0;
      gap: 0.25rem;
    }
  }

  .group-list::-webkit-scrollbar {
    width: 10px; /* Standard width */
  }

  .group-list::-webkit-scrollbar-button {
    display: none;
  }

  .group-list::-webkit-scrollbar-track {
    background-color: rgba(0, 0, 0, 0.2); /* Visible dark track */
    border-radius: 5px;
  }

  .group-list::-webkit-scrollbar-thumb {
    background-color: var(--text-2); /* High visibility (grey) */
    border-radius: 5px;
    border: 2px solid transparent;
    background-clip: content-box;
  }

  .group-list::-webkit-scrollbar-thumb:hover {
    background-color: var(--text); /* White on hover */
  }

  .group-option {
    display: flex;
    gap: 0.75rem;
    align-items: center;
    padding: 0.75rem 1rem;
    border-radius: 0.55rem;
    border: 1px solid var(--bg-light-extra);
    background-color: var(--bg-light);
    position: relative;
    cursor: pointer;
    transition:
      border-color 0.15s ease,
      background-color 0.15s ease;
  }

  .group-option:hover {
    border-color: color-mix(in oklab, var(--bg-light-extra) 85%, var(--accent) 15%);
  }

  .group-info {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    width: 100%;
  }

  .group-name {
    font-weight: 600;
    color: var(--text);
    word-break: break-word;
    flex: 1;
  }

  .group-meta {
    font-size: 0.8rem;
    color: var(--text-2);
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }

  .validation {
    color: var(--danger);
    font-size: 0.85rem;
  }

  .dialog-actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: 0.75rem;
  }

  .buttons {
    display: flex;
    gap: 0.5rem;
  }

  .selected-count {
    font-size: 0.85rem;
    color: var(--text-2);
  }

  .dropzone {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    border: 2px dashed var(--bg-light-extra);
    border-radius: 1rem;
    padding: 3rem 1rem;
    background-color: var(--bg-light);
    cursor: pointer;
    transition: all 0.2s ease;
    margin-top: 0.5rem;
    user-select: none;
  }

  .dropzone:hover,
  .dropzone.dragging {
    border-color: var(--accent);
    background-color: color-mix(in oklab, var(--bg-light) 95%, var(--accent) 5%);
  }

  .dropzone-icon {
    color: var(--text-2);
    transition: color 0.2s ease;
  }

  .dropzone:hover .dropzone-icon {
    color: var(--accent);
  }

  .dropzone-text {
    font-size: 1rem;
    color: var(--text-2);
    text-align: center;
    margin: 0;
  }

  .spacer {
    flex: 1;
  }
</style>
