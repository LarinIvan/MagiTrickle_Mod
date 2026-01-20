<script lang="ts">
  import { Collapsible } from "bits-ui";
  import { slide } from "svelte/transition";
  import { InfiniteLoader } from "svelte-infinite";
  import { createEventDispatcher } from "svelte";

  import { type Group, type Rule, type GroupDragData } from "../../../types";
  import { defaultRule } from "../../../utils/defaults";
  import { INTERFACES } from "../../../data/interfaces.svelte";
  import { getInterfaceLabel } from "../../../data/aliases.svelte";
  import { t } from "../../../data/locale.svelte";
  import { droppable, draggable } from "../../../lib/dnd";
  import Button from "../../../components/ui/Button.svelte";
  import DropdownMenu from "../../../components/ui/DropdownMenu.svelte";
  import Select from "../../../components/ui/Select.svelte";
  import Switch from "../../../components/ui/Switch.svelte";
  import Tooltip from "../../../components/ui/Tooltip.svelte";
  import {
    Delete,
    Add,
    GroupExpand,
    GroupCollapse,
    Dots,
    ImportList,
    Grip,
    MoveUp,
    MoveDown,
  } from "../../../components/ui/icons";
  import RuleRow from "./RuleRow.svelte";
  import ConflictIcon from "../../../components/ui/ConflictIcon.svelte";
  import { conflictsStore, type Conflict } from "../conflicts.svelte";

  type Props = {
    group: Group;
    group_index: number;
    total_groups: number;
    showed_limit: number;
    open: boolean;
    selected?: boolean;
    deleteGroup: (index: number) => void;
    addRuleToGroup: (group_index: number, rule: Rule, focus?: boolean) => void;
    deleteRuleFromGroup: (group_index: number, rule_index: number) => void;
    changeRuleIndex: (
      from_group_index: number,
      from_rule_index: number,
      to_group_index: number,
      to_rule_index: number,
    ) => void;
    moveGroupUp: (index: number) => void;
    moveGroupDown: (index: number) => void;
    loadMore: (group_index: number) => Promise<void>;
    searchActive?: boolean;
    visibleRuleIndices?: number[] | null;
    onJump?: (conflict: Conflict) => void;
    allGroups?: Group[];
    [key: string]: any;
  };

  let {
    group = $bindable(),
    group_index,
    total_groups = $bindable(),
    showed_limit = $bindable(),
    open = $bindable(),
    selected = false,
    deleteGroup,
    addRuleToGroup,
    deleteRuleFromGroup,
    changeRuleIndex,
    moveGroupUp,
    moveGroupDown,
    loadMore,
    searchActive = false,
    visibleRuleIndices = null,
    onJump,
    allGroups = [],
    ...rest
  }: Props = $props();

  const dispatch = createEventDispatcher();

  const triggerLoad = () => loadMore(group_index);

  let client_width = $state<number>(Infinity);
  let is_desktop = $derived(client_width > 668);

  let filteredRuleIndices: number[] | null = $state(null);
  let displayedRulesCount = $state(0);

  $effect(() => {
    if (searchActive && Array.isArray(visibleRuleIndices)) {
      filteredRuleIndices = visibleRuleIndices.length ? visibleRuleIndices : [];
    } else {
      filteredRuleIndices = null;
    }

    displayedRulesCount = Array.isArray(filteredRuleIndices)
      ? filteredRuleIndices.length
      : group.rules.length;
  });
  function createGroupDragPreview(
    groupEl: HTMLElement,
    name: string,
    color: string,
    count: number,
  ) {
    const badge = document.createElement("div");
    badge.style.cssText =
      "position:fixed;top:-1000px;left:-1000px;pointer-events:none;z-index:2147483647;transform:translateZ(0);font:600 13px/1.2 var(--font, -apple-system, system-ui, Segoe UI, Roboto, sans-serif);color:var(--text,#e5e7eb);";
    const inner = document.createElement("div");
    inner.style.cssText =
      "display:flex;align-items:center;gap:.5rem;padding:.35rem .6rem;border-radius:.6rem;background:var(--bg-light,rgba(30,30,36,.92));border:1px solid var(--bg-light-extra,rgba(255,255,255,.12));box-shadow:0 6px 18px rgba(0,0,0,.35);backdrop-filter:saturate(120%) blur(6px);";

    if (color) {
      const dot = document.createElement("span");
      dot.style.cssText = `width:10px;height:10px;border-radius:50%;background:${color};display:inline-block;border:1px solid rgba(0,0,0,0.2);`;
      inner.appendChild(dot);
    }

    const label = document.createElement("span");
    label.textContent = name || "Group";
    label.style.cssText =
      "max-width:260px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;";
    inner.appendChild(label);

    if (count !== undefined) {
      const countBadge = document.createElement("span");
      countBadge.textContent = String(count);
      countBadge.style.cssText = "opacity:0.6;font-size:0.85em;margin-left:auto;";
      inner.appendChild(countBadge);
    }

    const gripClone = groupEl.querySelector(".group-grip")?.cloneNode(true) as HTMLElement | null;
    if (gripClone) {
      gripClone.style.cssText += "opacity:.85;display:flex;align-items:center;margin-left:0.5rem;";
      inner.appendChild(gripClone);
    }

    badge.appendChild(inner);
    document.body.appendChild(badge);
    return badge;
  }
</script>

<svelte:window bind:innerWidth={client_width} />

<div
  class="group"
  class:selected
  role="listitem"
  data-uuid={group.id}
  use:draggable={{
    data: {
      group_id: group.id,
      group_index,
      name: group.name,
      color: group.color,
      count: group.rules.length,
    } as GroupDragData,
    scope: "group",
    handle: ".group-grip",
    effects: { effectAllowed: "move", dropEffect: "move" },
    dragImage: (node) =>
      createGroupDragPreview(
        (node.querySelector(".group-header") ?? node) as HTMLElement,
        group.name,
        group.color || "",
        group.rules.length,
      ),
  }}
>
  <Collapsible.Root bind:open>
    <div
      class="group-header"
      data-group-index={group_index}
      role="button"
      tabindex="0"
      onkeydown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          dispatch("select", { originalEvent: e, id: group.id });
        }
      }}
      onclick={(e) => {
        // Prevent selecting if clicking interactables
        const target = e.target as HTMLElement;
        const interactable = target.closest('button, input, label, [role="button"]');
        if (interactable && interactable !== e.currentTarget) return;
        dispatch("select", { originalEvent: e, id: group.id });
      }}
      use:droppable={{
        data: { rule_id: "", rule_index: 0, group_id: group.id, group_index },
        scope: "rule",
        canDrop: (src) => src.group_id === group.id,
      }}
    >
      <div class="group-left">
        <label class="group-color" style="background: {group.color}">
          <input type="color" bind:value={group.color} />
        </label>

        <div class="group-grip" title={t("Drag Group")}>
          <Grip />
        </div>

        {#if onJump}
          <div class="group-conflict-icon-wrapper">
            <ConflictIcon
              conflicts={conflictsStore.getConflictsForGroup(group.id)}
              ruleId={group.id}
              {onJump}
              {allGroups}
              isGroupContext={true}
            />
          </div>
        {/if}

        <input
          type="text"
          placeholder={t("group name...")}
          class="group-name"
          bind:value={group.name}
        />
      </div>

      <div class="group-actions">
        <Select
          options={INTERFACES.map((item) => ({
            value: item.id,
            label: item.active
              ? getInterfaceLabel(item.id)
              : `<span style="color: #ff4d4f;">${t("interface.inactive")}</span> ${getInterfaceLabel(item.id)}`,
            html: true,
          }))}
          bind:selected={group.interface}
        />

        <div class="group-enable-switch">
          <Tooltip value={t(group.enable ? "Disable Group" : "Enable Group")}>
            <Switch class="enable-group" bind:checked={group.enable} />
          </Tooltip>
        </div>

        {#if is_desktop}
          <Tooltip value={t("Move Up")}>
            <Button small disabled={group_index === 0} onclick={() => moveGroupUp(group_index)}>
              <MoveUp size={20} />
            </Button>
          </Tooltip>
          <Tooltip value={t("Move Down")}>
            <Button
              small
              disabled={group_index === total_groups - 1}
              onclick={() => moveGroupDown(group_index)}
            >
              <MoveDown size={20} />
            </Button>
          </Tooltip>
          <Tooltip value={t("Delete Group")}>
            <Button small onclick={() => deleteGroup(group_index)}>
              <Delete size={20} />
            </Button>
          </Tooltip>
          <Tooltip value={t("Add Rule")}>
            <Button
              small
              onclick={() => {
                addRuleToGroup(group_index, defaultRule(), true);
                open = true;
              }}
            >
              <Add size={20} />
            </Button>
          </Tooltip>
          <Tooltip value={t("Import Rule List")}>
            <Button small onclick={() => dispatch("importRules")}>
              <ImportList size={20} />
            </Button>
          </Tooltip>

          <Tooltip value={t(open ? "Collapse Group" : "Expand Group")}>
            <Collapsible.Trigger>
              {#if open}
                <GroupCollapse size={20} />
              {:else}
                <GroupExpand size={20} />
              {/if}
            </Collapsible.Trigger>
          </Tooltip>
        {:else}
          <div class="mobile-bottom-actions">
            <DropdownMenu>
              {#snippet trigger()}
                <Dots size={20} />
              {/snippet}
              {#snippet item1()}
                <Button
                  general
                  onclick={() => {
                    addRuleToGroup(group_index, defaultRule(), true);
                    open = true;
                  }}
                >
                  <div class="dd-icon"><Add size={20} /></div>
                  <div class="dd-label">{t("Add Rule")}</div>
                </Button>
              {/snippet}
              {#snippet item2()}
                <Button general onclick={() => dispatch("importRules")}>
                  <div class="dd-icon"><ImportList size={20} /></div>
                  <div class="dd-label">{t("Import Rule List")}</div>
                </Button>
              {/snippet}
              {#snippet item3()}
                <Button
                  general
                  disabled={group_index === 0}
                  onclick={() => moveGroupUp(group_index)}
                >
                  <div class="dd-icon"><MoveUp size={20} /></div>
                  <div class="dd-label">{t("Move Up")}</div>
                </Button>
              {/snippet}
              {#snippet item4()}
                <Button
                  general
                  disabled={group_index === total_groups - 1}
                  onclick={() => moveGroupDown(group_index)}
                >
                  <div class="dd-icon"><MoveDown size={20} /></div>
                  <div class="dd-label">{t("Move Down")}</div>
                </Button>
              {/snippet}
              {#snippet item5()}
                <Button general onclick={() => deleteGroup(group_index)}>
                  <div class="dd-icon"><Delete size={20} /></div>
                  <div class="dd-label">{t("Delete Group")}</div>
                </Button>
              {/snippet}
            </DropdownMenu>

            <Tooltip value={t(open ? "Collapse Group" : "Expand Group")}>
              <Collapsible.Trigger>
                {#if open}
                  <GroupCollapse size={20} />
                {:else}
                  <GroupExpand size={20} />
                {/if}
              </Collapsible.Trigger>
            </Tooltip>
          </div>
        {/if}
      </div>
    </div>

    <Collapsible.Content>
      <div transition:slide>
        {#if displayedRulesCount > 0}
          <div class="group-rules-header">
            <div class="group-rules-header-column total">
              #{displayedRulesCount}
            </div>
            <div class="group-rules-header-column">{t("Name")}</div>
            <div class="group-rules-header-column">{t("Type")}</div>
            <div class="group-rules-header-column">{t("Pattern")}</div>
            <div class="group-rules-header-column">
              {#if displayedRulesCount > 0}
                <div class="master-switch-wrapper">
                  <Tooltip value={t("Toggle All Rules")}>
                    <Switch
                      class="master-switch"
                      checked={group.rules.every((r) => r.enable)}
                      mixed={group.rules.some((r) => r.enable) &&
                        !group.rules.every((r) => r.enable)}
                      onCheckedChange={(v: boolean) => {
                        const allOn = group.rules.every((r) => r.enable);
                        const mixed = group.rules.some((r) => r.enable) && !allOn;
                        const newState = mixed ? true : !allOn;

                        group.rules = group.rules.map((r) => ({ ...r, enable: newState }));
                      }}
                    />
                  </Tooltip>
                </div>
                <Tooltip value={t("Delete All Rules")}>
                  <Button
                    small
                    class="delete-all-rules-btn"
                    onclick={() => {
                      if (confirm(t("Delete all rules in this group?"))) {
                        group.rules = [];
                      }
                    }}
                  >
                    <Delete size={20} />
                  </Button>
                </Tooltip>
              {/if}
            </div>
          </div>
        {/if}
        {#if !is_desktop && displayedRulesCount > 0}
          <div class="mobile-bulk-actions-row">
            <div class="master-switch-wrapper">
              <Tooltip value={t("Toggle All Rules")}>
                <Switch
                  class="master-switch"
                  checked={group.rules.every((r) => r.enable)}
                  mixed={group.rules.some((r) => r.enable) && !group.rules.every((r) => r.enable)}
                  onCheckedChange={(v: boolean) => {
                    const allOn = group.rules.every((r) => r.enable);
                    const mixed = group.rules.some((r) => r.enable) && !allOn;
                    const newState = mixed ? true : !allOn;

                    group.rules = group.rules.map((r) => ({ ...r, enable: newState }));
                  }}
                />
              </Tooltip>
            </div>
            <Button
              small
              class="delete-all-rules-btn"
              onclick={() => {
                if (confirm(t("Delete all rules in this group?"))) {
                  group.rules = [];
                }
              }}
            >
              <Delete size={20} />
            </Button>
          </div>
        {/if}
        <div class="group-rules">
          {#if Array.isArray(filteredRuleIndices)}
            {#each filteredRuleIndices as rule_index, visible_index (group.rules[rule_index]?.id ?? `missing-${visible_index}`)}
              {#if group.rules[rule_index]}
                <RuleRow
                  key={group.rules[rule_index].id}
                  bind:rule={group.rules[rule_index]}
                  {rule_index}
                  {group_index}
                  rule_id={group.rules[rule_index].id}
                  group_id={group.id}
                  onChangeIndex={changeRuleIndex}
                  onDelete={deleteRuleFromGroup}
                  {onJump}
                  style={visible_index % 2 ? "" : "background-color: var(--bg-light)"}
                />
              {/if}
            {/each}
          {:else}
            <InfiniteLoader {triggerLoad} loopDetectionTimeout={10}>
              {#each group.rules.slice(0, showed_limit) as rule, rule_index (rule.id)}
                <RuleRow
                  key={rule.id}
                  bind:rule={group.rules[rule_index]}
                  {rule_index}
                  {group_index}
                  rule_id={rule.id}
                  group_id={group.id}
                  onChangeIndex={changeRuleIndex}
                  onDelete={deleteRuleFromGroup}
                  {onJump}
                  style={rule_index % 2 ? "" : "background-color: var(--bg-light)"}
                />
              {/each}
            </InfiniteLoader>
          {/if}
        </div>
      </div>
    </Collapsible.Content>
  </Collapsible.Root>
</div>

<style>
  .group {
    & {
      background-color: var(--bg-medium);
      border-radius: 0.5rem;
      border: 1px solid var(--bg-light-extra);
      transition:
        transform 0.12s ease,
        opacity 0.12s ease,
        box-shadow 0.12s ease,
        border-color 0.1s ease;
      position: relative;
    }

    &.selected {
      z-index: 1;
      border-color: transparent;
    }

    &.selected::after {
      content: "";
      position: absolute;
      inset: -1px;
      border-radius: inherit;
      border: 1px solid var(--accent);
      box-shadow: 0 0 0 1px color-mix(in oklab, var(--accent) 50%, transparent);
      pointer-events: none;
      z-index: 10;
    }
  }

  .group-header {
    & {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 0.5rem;
      border-radius: 0.5rem;
      background-color: var(--bg-light);
      position: relative;
    }

    &:global(.dragover) {
      outline: 1px solid var(--accent);
      box-shadow: inset 0 0 5px 0 var(--accent);
    }
  }

  .group-left {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex: 1;
    min-width: 0;
  }

  .group-color {
    & {
      display: inline-block;
      width: 2rem;
      height: calc(100% + 1px);
      border-top-left-radius: 0.5rem;
      border-bottom-left-radius: 0.5rem;
      position: absolute;
      left: 0px;
      top: -1px;
      overflow: hidden;
      cursor: pointer;
    }

    & input {
      margin-left: 0.5rem;
    }
  }

  .group-grip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    margin-left: 2.2rem;
    color: var(--text-2);
    cursor: grab;
    user-select: none;
    -webkit-user-select: none;
    -webkit-user-drag: none;
  }
  .group-grip:hover {
    color: var(--text);
  }

  .group-conflict-icon-wrapper {
    margin-left: 0rem;
    position: relative;
  }

  .group-name {
    & {
      border: none;
      background-color: transparent;
      font-size: 1.3rem;
      font-weight: 600;
      font-family: var(--font);
      color: var(--text);
      border-bottom: 1px solid transparent;
      position: relative;
      top: 0.1rem;
      margin-left: 0rem;
      width: 100%;
    }

    &:focus-visible {
      outline: none;
      border-bottom: 1px solid var(--accent);
    }
  }

  .group-actions {
    & {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 0.2rem;
    }
    &:global([data-switch-root]) {
      margin: 0 0.3rem;
    }
  }

  .group-rules-header {
    display: grid;
    grid-template-columns: 4rem 2.1fr 1fr 3fr 1fr;
    justify-content: center;
    align-items: center;

    font-size: 0.9rem;
    color: var(--text-2);
    padding-top: 0.6rem;
    padding-bottom: 0.2rem;
    border-bottom: 1px solid var(--bg-light-extra);
  }

  .group-rules-header-column {
    & {
      display: flex;
      align-items: center;
      justify-content: center;
    }

    &.total {
      justify-content: start;
      margin-left: 0.5rem;
    }

    &.total :global(svg) {
      position: relative;
      top: -1px;
    }
  }

  :global {
    [data-collapsible-trigger] {
      & {
        color: var(--text-2);
        background-color: transparent;
        border: 1px solid transparent;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        padding: 0.4rem;
        border-radius: 0.5rem;
        cursor: pointer;
      }

      &:hover {
        background-color: var(--bg-dark);
        color: var(--text);
        border: 1px solid var(--bg-light-extra);
      }
    }
    .infinite-intersection-target {
      padding-block: 0 !important;
    }
  }

  input[type="color"] {
    -webkit-appearance: none;
    -moz-appearance: none;
    appearance: none;
    background: transparent;
    width: auto;
    height: 0;
    padding: 0;
    border: none;
    cursor: pointer;
  }

  @media (max-width: 700px) {
    .group-header {
      display: flex;
      flex-direction: column;
      align-items: start;
      justify-content: center;
      padding-right: 4rem;
    }

    .group-left {
      & {
        width: 100%;
      }
      & input[type="text"] {
        width: 100%;
        margin-left: 0rem;
      }
      & label {
        height: calc(100% + 1px);
      }
    }

    .group-actions {
      width: calc(100% - 2rem);
      justify-content: stretch;
      gap: 0.25rem;
      margin-left: 2rem;
    }

    :global(.group-actions > *:nth-child(1)) {
      margin-right: auto;
      width: auto;
      max-width: 100%;
      min-width: 0;
      flex: 0 1 auto;
    }

    :global(.group-actions > *:nth-child(1) [data-select-trigger]) {
      max-width: 100%;
    }

    :global(.group-actions > *:nth-child(1) .selected) {
      max-width: 100%;
    }

    :global(.group-actions > *:nth-child(2)) {
      margin-left: auto;
    }

    .group-rules-header {
      height: 1px;
      & .group-rules-header-column {
        display: none;
      }
    }

    :global(.group-enable-switch) {
      position: absolute;
      top: 0.5rem;
      right: 0.5rem;
      z-index: 10;
    }

    :global(.mobile-bottom-actions) {
      position: absolute;
      bottom: 0.5rem;
      right: 0rem;
      display: flex;
      gap: 0;
      z-index: 10;
    }

    .mobile-bulk-actions-row {
      display: flex;
      justify-content: flex-end;
      align-items: center;
      gap: 0.35rem;
      padding: 0.5rem 0.35rem;
      border-bottom: 1px solid var(--bg-light-extra);
    }
  }

  :global(.master-switch[data-state="checked"]) {
    background-color: #22c55e !important;
  }

  :global(.master-switch[data-state="unchecked"]) {
    background-color: #ef4444 !important;
  }

  :global(.master-switch[data-mixed="true"]) {
    background-color: #f97316 !important;
  }

  .group-rules-header-column:last-child {
    justify-content: end;
    gap: 0.5rem;
  }

  :global(.delete-all-rules-btn) {
    color: var(--red) !important;
    position: relative;
    top: -1px;
  }

  :global(.delete-all-rules-btn:hover) {
    background-color: color-mix(in oklab, var(--red) 10%, transparent) !important;
    border: 1px solid var(--red) !important;
  }
</style>
