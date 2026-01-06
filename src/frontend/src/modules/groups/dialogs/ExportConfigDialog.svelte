<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import GenericDialog from "../../../components/ui/GenericDialog.svelte";
  import Button from "../../../components/ui/Button.svelte";
  import Checkbox from "../../../components/ui/Checkbox.svelte";
  import type { Group } from "../../../types";
  import { t } from "../../../data/locale.svelte";

  export let open = false;
  export let groups: Group[] = [];

  const dispatch = createEventDispatcher<{
    close: void;
    export: { groups: Group[] };
  }>();

  let triedSubmit = false;
  let selection = new Set<number>();
  let wasOpen = false;

  $: if (open && !wasOpen) {
    // Default select all
    selection = new Set(groups.map((_, index) => index));
    triedSubmit = false;
  }

  $: wasOpen = open;
  $: selectedCount = selection.size;

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

    dispatch("export", { groups: selectedGroups });
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
  title={t("Export Config")}
  textareaValue=""
  textareaPlaceholder=""
  {triedSubmit}
  maxWidth={600}
  on:close={close}
  on:submit={submit}
>
  <div slot="body" class="export-config">
    <p class="summary-text">
      {t("Select groups to export")}
    </p>

    <div class="list-header">
      <div class="select-all-container">
        <Checkbox checked={allSelected} on:change={handleSelectAll} />
        <span class="select-all-label" onclick={() => selectAll(!allSelected)}
          >{t("Select all")}</span
        >
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
  </div>
  <div slot="actions" class="dialog-actions">
    <div class="selected-count">
      {selectedCount} / {groups.length}
    </div>
    <div class="buttons">
      <Button type="button" onclick={close}>{t("Cancel")}</Button>
      <Button type="submit">{t("Export")}</Button>
    </div>
  </div>
</GenericDialog>

<style>
  .export-config {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding-top: 0.25rem;
  }

  .summary-text {
    font-size: 0.95rem;
    color: var(--text-2);
    margin: 0;
    line-height: 1.4;
    margin-bottom: 0.5rem;
  }

  .list-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 1rem;
    margin-top: 0.5rem;
  }

  .select-all-container {
    display: flex;
    align-items: center;
    gap: 0.75rem;
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
    max-height: 50vh;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding: 0 1rem 0.25rem 0;
    scrollbar-gutter: stable;
  }

  @media (max-height: 700px) {
    .group-list {
      max-height: 40vh;
    }
    .export-config {
      padding-top: 0;
      gap: 0.25rem;
    }
  }

  .group-list::-webkit-scrollbar {
    width: 10px;
  }

  .group-list::-webkit-scrollbar-button {
    display: none;
  }

  .group-list::-webkit-scrollbar-track {
    background-color: rgba(0, 0, 0, 0.2);
    border-radius: 5px;
  }

  .group-list::-webkit-scrollbar-thumb {
    background-color: var(--text-2);
    border-radius: 5px;
    border: 2px solid transparent;
    background-clip: content-box;
  }

  .group-list::-webkit-scrollbar-thumb:hover {
    background-color: var(--text);
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
</style>
