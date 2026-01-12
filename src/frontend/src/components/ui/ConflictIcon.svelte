<script lang="ts">
  import { AlertTriangle } from "lucide-svelte";
  import { conflictsStore, type Conflict } from "../../modules/groups/conflicts.svelte";
  import { t } from "../../data/locale.svelte";
  import Tooltip from "./Tooltip.svelte";

  let {
    conflicts,
    ruleId,
    onJump,
    isGroupContext = false,
    allGroups = [],
  }: {
    conflicts: Conflict[];
    ruleId: string;
    onJump: (conflict: Conflict) => void;
    isGroupContext?: boolean;
    allGroups?: any[];
  } = $props();

  let isOpen = $state(false);
  let readyToShow = $state(false);

  $effect(() => {
    if (conflictsStore.activePopoverId !== null && conflictsStore.activePopoverId !== ruleId) {
      isOpen = false;
    }
  });

  function togglePopover() {
    isOpen = !isOpen;
    if (isOpen) {
      conflictsStore.activePopoverId = ruleId;
    } else {
      conflictsStore.activePopoverId = null;
    }
  }

  function closePopover() {
    isOpen = false;
    conflictsStore.activePopoverId = null;
  }

  function handleClickOutside(event: MouseEvent) {
    const target = event.target as HTMLElement;
    const popover = document.querySelector(`.popover[data-rule-id="${ruleId}"]`);
    const icon = document.querySelector(`.conflict-icon[data-rule-id="${ruleId}"]`);

    if (popover && !popover.contains(target) && icon && !icon.contains(target)) {
      closePopover();
    }
  }

  $effect(() => {
    if (isOpen) {
      document.addEventListener(`click`, handleClickOutside);

      requestAnimationFrame(() => {
        const popover = document.querySelector(`.popover[data-rule-id="${ruleId}"]`) as HTMLElement;
        const icon = document.querySelector(
          `.conflict-icon[data-rule-id="${ruleId}"]`,
        ) as HTMLElement;

        if (popover && icon) {
          popover.style.maxWidth = "";

          let rect = popover.getBoundingClientRect();
          const viewportWidth = document.documentElement.clientWidth;
          const viewportHeight = window.innerHeight;
          const margin = 32;

          if (rect.width > viewportWidth - margin * 2) {
            popover.style.maxWidth = `${viewportWidth - margin * 2}px`;
            rect = popover.getBoundingClientRect();
          }
          if (rect.right > viewportWidth - margin) {
            const overflow = rect.right - (viewportWidth - margin);
            const currentLeft = popover.offsetLeft;
            popover.style.left = `${currentLeft - overflow}px`;
            rect = popover.getBoundingClientRect();
          }
          if (rect.left < margin) {
            const currentLeft = popover.offsetLeft;
            const underflow = margin - rect.left;
            popover.style.left = `${currentLeft + underflow}px`;
            rect = popover.getBoundingClientRect();
          }

          if (rect.bottom > viewportHeight) {
            const iconRect = icon.getBoundingClientRect();
            const spaceAbove = iconRect.top;
            const popoverHeight = rect.height;

            if (spaceAbove > popoverHeight || spaceAbove > viewportHeight - iconRect.bottom) {
              popover.style.top = "auto";
              popover.style.bottom = "100%";
              popover.style.marginTop = "0";
              popover.style.marginBottom = "8px";
              popover.classList.add("open-up");
            }
          }
          readyToShow = true;
        }
      });

      return () => {
        document.removeEventListener(`click`, handleClickOutside);
        readyToShow = false;
      };
    }
  });

  function handleJump(conflict: Conflict) {
    closePopover();
    onJump(conflict);
  }

  type AggregatedConflict = {
    ruleValue: string;
    groupName: string;
    groupId: string;
    count: number;
    type: "exact" | "overlap";
    firstConflict: Conflict;
  };

  let aggregatedConflicts = $derived.by(() => {
    if (!isGroupContext || conflicts.length === 0) return [];

    const uniqueRuleValues = new Set<string>();
    let conflictType: "exact" | "overlap" = "exact";

    for (const conflict of conflicts) {
      uniqueRuleValues.add(conflict.targetRule.rule);
      conflictType = conflict.type;
    }

    const result: AggregatedConflict[] = [];

    for (const ruleValue of uniqueRuleValues) {
      const groupStats = new Map<
        string,
        { name: string; count: number; firstConflict: Conflict | null }
      >();

      for (const group of allGroups) {
        if (!group.enable) continue;

        let count = 0;
        let firstConflict: Conflict | null = null;

        for (const rule of group.rules) {
          if (rule.enable && rule.rule === ruleValue) {
            count++;
            if (!firstConflict) {
              firstConflict = {
                id: `${ruleId}-${rule.id}`,
                sourceRule: rule,
                sourceGroupId: group.id,
                targetRule: rule,
                targetGroupId: group.id,
                targetGroupName: group.name,
                type: conflictType,
              };
            }
          }
        }

        if (count > 0) {
          groupStats.set(group.id, {
            name: group.name,
            count,
            firstConflict,
          });
        }
      }

      for (const [groupId, stats] of groupStats) {
        result.push({
          ruleValue,
          groupName: stats.name,
          groupId,
          count: stats.count,
          type: conflictType,
          firstConflict: stats.firstConflict!,
        });
      }
    }

    return result;
  });
</script>

{#if conflicts.length > 0}
  <div class="conflict-icon-wrapper">
    <Tooltip value={t("conflict.found_tooltip").replace("{count}", conflicts.length.toString())}>
      <button
        class="conflict-icon"
        class:active={isOpen}
        data-rule-id={ruleId}
        onclick={togglePopover}
      >
        <AlertTriangle size={18} />
      </button>
    </Tooltip>

    {#if isOpen}
      <div
        class="popover"
        class:animate-in={readyToShow}
        data-rule-id={ruleId}
        role="presentation"
        style:visibility={readyToShow ? "visible" : "hidden"}
        onclick={(e) => e.stopPropagation()}
      >
        <div class="popover-header">
          {t("conflict.found_header").replace(
            "{count}",
            (isGroupContext ? aggregatedConflicts.length : conflicts.length).toString(),
          )}
        </div>
        <div class="popover-content">
          {#if isGroupContext}
            <table class="conflict-table">
              <thead>
                <tr>
                  <th>{t("conflict.table.rule")}</th>
                  <th>{t("conflict.table.group")}</th>
                  <th>{t("conflict.table.count")}</th>
                  <th>{t("conflict.table.conflict")}</th>
                  <th>{t("conflict.table.action")}</th>
                </tr>
              </thead>
              <tbody>
                {#each aggregatedConflicts as agg}
                  <tr>
                    <td class="rule-cell">{agg.ruleValue}</td>
                    <td>
                      <span class="group-name">{agg.groupName}</span>
                    </td>
                    <td class="count-cell">{agg.count}</td>
                    <td class="conflict-type">
                      {agg.type === `exact`
                        ? t("conflict.type.duplicate")
                        : t("conflict.type.overlap")}
                    </td>
                    <td>
                      <button class="jump-btn" onclick={() => handleJump(agg.firstConflict)}>
                        {t("conflict.action.go")}
                      </button>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {:else}
            <table class="conflict-table">
              <thead>
                <tr>
                  <th>{t("conflict.table.rule")}</th>
                  <th>{t("conflict.table.group")}</th>
                  <th>{t("conflict.table.conflict")}</th>
                  <th>{t("conflict.table.action")}</th>
                </tr>
              </thead>
              <tbody>
                {#each conflicts as conflict}
                  <tr>
                    <td class="rule-cell">{conflict.targetRule.rule}</td>
                    <td>
                      <span class="group-name">{conflict.targetGroupName}</span>
                    </td>
                    <td class="conflict-type">
                      {conflict.type === `exact`
                        ? t("conflict.type.duplicate")
                        : t("conflict.type.overlap")}
                    </td>
                    <td>
                      <button class="jump-btn" onclick={() => handleJump(conflict)}>
                        {t("conflict.action.go")}
                      </button>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      </div>
    {/if}
  </div>
{/if}

<style>
  .conflict-icon-wrapper {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .conflict-icon {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    transition: all 0.1s;
    color: #ffd000;
    fill: rgba(255, 208, 0, 0.2);
  }

  @media (hover: hover) {
    .conflict-icon:hover {
      background: rgba(255, 208, 0, 0.1);
      transform: scale(1.1);
    }
  }

  .conflict-icon.active,
  .conflict-icon:active {
    background: rgba(255, 208, 0, 0.1);
    transform: scale(0.95);
    fill: rgba(255, 208, 0, 0.4);
  }

  .popover {
    position: absolute;
    top: 100%;
    left: 0;
    margin-top: 8px;
    background: #1e1e1e;
    border: 1px solid #333;
    border-radius: 8px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
    z-index: 10000;
    width: max-content;
    min-width: 320px;
    overflow: hidden;
    overflow: hidden;
    pointer-events: auto;
  }

  .popover.animate-in {
    animation: fadeIn 0.2s ease-out forwards;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .popover-header {
    background: #252525;
    padding: 8px 12px;
    font-size: 13px;
    font-weight: 600;
    color: #ffd000;
    border-bottom: 1px solid #333;
    white-space: nowrap;
  }

  .popover-content {
    padding: 0;
    max-height: 200px;
    overflow-y: auto;
    overflow-x: hidden;
  }

  .popover-content::-webkit-scrollbar {
    width: 8px;
  }

  .popover-content::-webkit-scrollbar-track {
    background: #1a1a1a;
    border-radius: 4px;
  }

  .popover-content::-webkit-scrollbar-thumb {
    background: #404040;
    border-radius: 4px;
    transition: background 0.2s;
  }

  .popover-content::-webkit-scrollbar-thumb:hover {
    background: #505050;
  }

  .conflict-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }

  .conflict-table th {
    text-align: left;
    padding: 6px 12px;
    color: #888;
    font-weight: normal;
    background: #222;
  }

  .conflict-table td {
    padding: 8px 12px;
    border-top: 1px solid #2a2a2a;
    color: #eee;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    vertical-align: middle;
  }

  .rule-cell {
    font-family: monospace;
    color: #ffd000;
    max-width: 600px;
  }

  .group-name {
    font-weight: bold;
    color: #fff;
    display: inline-block;
    max-width: 400px;
    overflow: hidden;
    text-overflow: ellipsis;
    vertical-align: middle;
    line-height: 1.5;
  }

  .conflict-type {
    color: #888;
  }

  .count-cell {
    text-align: center;
    font-family: monospace;
    color: #8ab4f8;
    font-weight: 600;
  }

  .jump-btn {
    background: #3a3a3a;
    border: none;
    color: #8ab4f8;
    padding: 4px 8px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 11px;
    transition: all 0.2s;
  }

  .jump-btn:hover {
    background: #444;
    color: #fff;
  }

  @media (max-width: 700px) {
    .popover {
      max-width: 90vw !important;
      min-width: auto !important;
      width: 90vw;
    }

    .conflict-table thead {
      display: none;
    }

    .conflict-table tbody {
      display: block;
    }

    .conflict-table tr {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      grid-template-rows: auto auto;
      gap: 2px 8px;
      padding: 8px;
      border-bottom: 1px solid #333;
    }

    .conflict-table td.rule-cell {
      grid-column: 1;
      grid-row: 1;
      padding: 0;
      border: none;
      font-size: 13px;
      font-weight: 500;
      max-width: none;
    }

    .conflict-table td:has(.group-name),
    .conflict-table td:nth-child(2) {
      grid-column: 1;
      grid-row: 2;
      padding: 0;
      border: none;
    }

    .group-name {
      font-size: 11px;
      color: #888;
      font-weight: normal;
      max-width: none;
    }

    .count-cell,
    .conflict-type {
      display: none !important;
    }
    .conflict-table td:last-child {
      grid-column: 2;
      grid-row: 1 / -1;
      align-self: center;
      padding: 0;
      border: none;
      width: auto;
    }

    .jump-btn {
      padding: 6px 12px;
      font-size: 12px;
      background: #333;
    }
  }
</style>
