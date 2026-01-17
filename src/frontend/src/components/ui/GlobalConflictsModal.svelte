<script lang="ts">
  import { AlertTriangle, Trash2 } from "lucide-svelte";
  import { conflictsStore } from "../../modules/groups/conflicts.svelte";
  import { t } from "../../data/locale.svelte";
  import Switch from "./Switch.svelte";
  import type { Group, Rule } from "../../types";

  type Props = {
    open: boolean;
    groups: Group[];
    onClose: () => void;
    onToggleRule: (groupId: string, ruleId: string, enabled: boolean) => void;
    onDeleteRule: (groupId: string, ruleId: string) => void;
    onJump: (groupId: string, ruleId: string) => void;
  };

  let { open = $bindable(), groups, onClose, onToggleRule, onDeleteRule, onJump }: Props = $props();

  let staticClusters = $state<
    { rules: Array<{ rule: Rule; groupId: string; groupName: string }> }[]
  >([]);

  $effect(() => {
    if (open) {
      if (staticClusters.length === 0) {
        staticClusters = conflictsStore.getConflictClusters();
      }
    } else {
      staticClusters = [];
    }
  });

  let groupNameMap = $derived.by(() => {
    const map = new Map<string, string>();
    for (const g of groups) {
      map.set(g.id, g.name);
    }
    return map;
  });

  function isRuleEnabled(groupId: string, ruleId: string): boolean {
    const group = groups.find((g) => g.id === groupId);
    if (!group) return false;
    const rule = group.rules.find((r) => r.id === ruleId);
    return rule?.enable ?? false;
  }

  function handleToggle(groupId: string, ruleId: string, enabled: boolean) {
    onToggleRule(groupId, ruleId, enabled);
  }

  function handleDelete(groupId: string, ruleId: string) {
    onDeleteRule(groupId, ruleId);

    staticClusters = staticClusters
      .map((cluster) => ({
        ...cluster,
        rules: cluster.rules.filter((r) => r.rule && r.rule.id !== ruleId),
      }))
      .filter((c) => c.rules.length > 0);
  }

  function handleJump(groupId: string, ruleId: string) {
    onClose();
    onJump(groupId, ruleId);
  }

  function handleBackdropClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains(`modal-backdrop`)) {
      onClose();
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") onClose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div class="modal-backdrop" role="presentation" onclick={handleBackdropClick}>
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div class="modal" role="dialog" aria-modal="true" onclick={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <AlertTriangle size={20} class="header-icon" />
        <span>{t(`conflict.global_header`)}</span>
        <button class="close-btn" onclick={onClose}>×</button>
      </div>

      <div class="modal-content">
        {#if staticClusters.length === 0}
          <div class="no-conflicts">{t(`conflict.none`)}</div>
        {:else}
          <table class="conflict-table">
            <thead>
              <tr>
                <th class="col-rule">{t(`conflict.table.rule`)}</th>
                <th class="col-group">{t(`conflict.table.group`)}</th>
                <th class="col-type">{t(`conflict.table.type`)}</th>
                <th class="col-action">{t(`conflict.table.action`)}</th>
              </tr>
            </thead>
            <tbody>
              {#each staticClusters as cluster, clusterIndex (clusterIndex)}
                {@const clusterEnabledCount = cluster.rules.reduce(
                  (acc, item) => acc + (isRuleEnabled(item.groupId, item.rule.id) ? 1 : 0),
                  0,
                )}

                {#each cluster.rules as item, itemIndex (item.rule.id)}
                  {@const enabled = isRuleEnabled(item.groupId, item.rule.id)}
                  {@const isLastInCluster = itemIndex === cluster.rules.length - 1}
                  {@const isLastCluster = clusterIndex === staticClusters.length - 1}
                  {@const hasConflict = enabled && clusterEnabledCount > 1}

                  <tr
                    class:disabled={!enabled}
                    class:cluster-separator={isLastInCluster && !isLastCluster}
                  >
                    <td class="rule-cell" title={item.rule.rule}>{item.rule.rule}</td>
                    <td>
                      <span
                        class="group-name"
                        title={groupNameMap.get(item.groupId) || item.groupName}
                        >{groupNameMap.get(item.groupId) || item.groupName}</span
                      >
                    </td>
                    <td class="type-cell">{item.rule.type}</td>
                    <td class="actions-cell">
                      {#if hasConflict}
                        <div class="warning-icon">
                          <AlertTriangle size={16} />
                        </div>
                      {/if}
                      <Switch
                        checked={enabled}
                        onCheckedChange={(v: boolean) =>
                          handleToggle(item.groupId, item.rule.id, v)}
                      />
                      <button
                        class="delete-btn"
                        onclick={() => handleDelete(item.groupId, item.rule.id)}
                      >
                        <Trash2 size={16} />
                      </button>
                      <button
                        class="jump-btn"
                        onclick={() => handleJump(item.groupId, item.rule.id)}
                      >
                        {t(`conflict.action.go`)}
                      </button>
                    </td>
                  </tr>
                {/each}
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10000;
    backdrop-filter: blur(2px);
  }

  .modal {
    background: #1e1e1e;
    border: 1px solid #333;
    border-radius: 12px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
    width: 90vw;
    max-width: 900px;
    max-height: 80vh;
    display: flex;
    flex-direction: column;
    animation: fadeIn 0.2s ease-out;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: scale(0.95);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  .modal-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    background: #252525;
    border-bottom: 1px solid #333;
    border-radius: 12px 12px 0 0;
    font-weight: 600;
    color: #ffd000;
    flex-shrink: 0;
  }

  :global(.header-icon) {
    color: #ffd000;
  }

  .close-btn {
    margin-left: auto;
    background: transparent;
    border: none;
    color: #888;
    font-size: 24px;
    cursor: pointer;
    padding: 0 4px;
    line-height: 1;
  }

  .close-btn:hover {
    color: #fff;
  }

  .modal-content {
    padding: 0;
    overflow-y: auto;
    flex: 1;
    display: flex;
    flex-direction: column;
    overscroll-behavior: contain;
    overflow-x: hidden;
  }

  .modal-content::-webkit-scrollbar {
    width: 8px;
  }

  .modal-content::-webkit-scrollbar-track {
    background: #1a1a1a;
    border-radius: 4px;
  }

  .modal-content::-webkit-scrollbar-thumb {
    background: #404040;
    border-radius: 4px;
  }

  .conflict-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
    table-layout: fixed;
  }

  .conflict-table th {
    text-align: left;
    padding: 10px 16px;
    color: #888;
    font-weight: normal;
    background: #222;
    border-bottom: 1px solid #333;
    position: sticky;
    top: 0;
    z-index: 10;
  }

  .col-rule {
    width: 35%;
  }
  .col-group {
    width: 20%;
  }
  .col-type {
    width: 15%;
  }
  .col-action {
    width: 30%;
  }

  .conflict-table th.col-action {
    text-align: right;
    padding-right: 24px;
  }

  .conflict-table td {
    padding: 10px 16px;
    border-bottom: none;
    color: #eee;
    vertical-align: middle;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .conflict-table tbody tr {
    border-bottom: 1px solid #2a2a2a;
  }
  .conflict-table tr.cluster-separator {
    border-bottom: 2px solid #555;
  }

  .conflict-table tbody tr:last-child {
    border-bottom: none;
  }

  .conflict-table tr.disabled td {
    opacity: 0.5;
  }

  .rule-cell {
    font-family: monospace;
    color: #ffd000;
  }

  .group-name {
    font-weight: 500;
    color: #fff;
  }

  .type-cell {
    color: #888;
    font-size: 12px;
  }

  .actions-cell {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    padding-right: 24px;
  }

  .delete-btn {
    background: transparent;
    border: none;
    color: #ff4d4f;
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .warning-icon {
    display: flex;
    align-items: center;
    color: #ffd000;
    margin-right: 8px;
    animation: fadeIn 0.2s ease-out;
  }

  .delete-btn:hover {
    background: rgba(255, 77, 79, 0.1);
  }

  .jump-btn {
    background: #3a3a3a;
    border: none;
    color: #8ab4f8;
    padding: 4px 10px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
    transition: all 0.2s;
  }

  .jump-btn:hover {
    background: #444;
    color: #fff;
  }

  .no-conflicts {
    text-align: center;
    color: #888;
    padding: 32px;
    margin: auto;
  }

  @media (max-width: 700px) {
    .modal {
      width: 95vw;
      max-height: 90vh;
      overflow-x: hidden;
    }

    .conflict-table thead {
      display: none;
    }

    .conflict-table,
    .conflict-table tbody,
    .conflict-table tr,
    .conflict-table td {
      display: block;
      width: 100%;
      box-sizing: border-box;
    }

    .conflict-table tr {
      display: grid;
      grid-template-columns: 1fr auto;
      grid-template-rows: auto auto auto;
      gap: 2px;
      padding: 10px 4px 10px 12px;
      border-bottom: 1px solid #333;
      background: #1e1e1e;
    }

    .conflict-table tr.cluster-separator {
      border-bottom: 4px solid #111;
      margin-bottom: 0;
    }

    .conflict-table tr:last-child {
      border-bottom: none;
    }

    .conflict-table td {
      padding: 0;
      border: none !important;
      white-space: normal;
    }

    .rule-cell {
      grid-row: 1;
      grid-column: 1 / -1;
      font-size: 14px;
      margin-bottom: 4px;
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .conflict-table td:has(.group-name) {
      grid-row: 2;
      grid-column: 1 / -1;
      color: #888;
      font-size: 13px;
      margin-bottom: 2px;
    }

    .group-name {
      color: #aaa;
      font-weight: normal;
    }

    .type-cell {
      display: block;
      grid-row: 3;
      grid-column: 1;
      justify-self: start;
      align-self: center;
      color: #666;
      font-size: 11px;
    }

    .actions-cell {
      grid-row: 3;
      grid-column: 2;
      justify-self: end;
      align-self: center;

      display: flex !important;
      flex-direction: row !important;
      align-items: center;
      gap: 6px;
      padding-right: 0;
    }

    .delete-btn {
      padding: 6px;
    }

    .jump-btn {
      padding: 6px 10px;
    }
  }
</style>
