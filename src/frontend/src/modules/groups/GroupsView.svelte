<script lang="ts">
  import { scale } from "svelte/transition";
  import { onDestroy, onMount, untrack, tick } from "svelte";
  import { loaderState } from "svelte-infinite";

  import { parseConfig, type Group, type Rule } from "../../types";
  import { defaultGroup, defaultRule, randomId } from "../../utils/defaults";
  import { fetcher } from "../../utils/fetcher";
  import { overlay, toast } from "../../utils/events";
  import { persistedState } from "../../utils/persisted-state.svelte";
  import Button from "../../components/ui/Button.svelte";
  import Select from "../../components/ui/Select.svelte";
  import Switch from "../../components/ui/Switch.svelte";
  import Tooltip from "../../components/ui/Tooltip.svelte";
  import {
    Add,
    Upload,
    Download,
    Save,
    CollapseAll,
    ExpandAll,
    Delete,
    MoveUp,
    MoveDown,
  } from "../../components/ui/icons";
  import { t } from "../../data/locale.svelte";
  import { droppable } from "../../lib/dnd";

  import { INTERFACES } from "../../data/interfaces.svelte";
  import { getInterfaceLabel } from "../../data/aliases.svelte";

  import GroupPanel from "./components/GroupPanel.svelte";
  import ImportRulesDialog from "./dialogs/ImportRulesDialog.svelte";
  import ImportConfigDialog from "./dialogs/ImportConfigDialog.svelte";
  import ExportConfigDialog from "./dialogs/ExportConfigDialog.svelte";

  import { groupsStore } from "../../data/groups.svelte";

  function handleSaveShortcut(event: KeyboardEvent) {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
      if (counter > 0 && valid_rules) {
        event.preventDefault();
        saveChanges();
      }
    }
  }

  const INITIAL_RULES_LIMIT = 30 as const;
  const INCREMENT_RULES_LIMIT = 40 as const;

  let showed_limit: number[] = $state([]);
  let counter = $state(-2); // skip first update on init
  let valid_rules = $state(true);
  let open_state = persistedState<Record<string, boolean>>("group_open_state", {});

  let importRulesModal = $state<{ open: boolean; groupIndex: number | null }>({
    open: false,
    groupIndex: null,
  });

  let importConfigModal = $state<{ open: boolean; groups: Group[]; fileName: string }>({
    open: false,
    groups: [],
    fileName: "",
  });
  function resetImportConfigModal() {
    importConfigModal = { open: false, groups: [], fileName: "" };
  }

  let exportConfigModal = $state<{ open: boolean }>({ open: false });
  function resetExportConfigModal() {
    exportConfigModal = { open: false };
  }
  let selectedGroupIds = $state<Set<string>>(new Set());
  let selectionBox = $state<{
    startX: number;
    startY: number;
    currentX: number;
    currentY: number;
    active: boolean;
  } | null>(null);
  let isSelecting = $state(false);

  let client_width = $state<number>(0);
  let is_desktop = $derived(client_width > 700);

  let commonInterface = $derived.by(() => {
    if (selectedGroupIds.size === 0) return "";
    let firstInterface: string | null = null;
    for (const id of selectedGroupIds) {
      const group = groupsStore.all.find((g) => g.id === id);
      if (!group) continue;
      if (firstInterface === null) {
        firstInterface = group.interface;
      } else if (firstInterface !== group.interface) {
        return ""; // Mixed interfaces
      }
    }
    return firstInterface || "";
  });

  function handleGroupSelect(event: CustomEvent<{ originalEvent: MouseEvent; id: string }>) {
    const { originalEvent: e, id } = event.detail;
    const index = groupsStore.all.findIndex((g) => g.id === id);
    if (index === -1) return;

    if (e.ctrlKey || e.metaKey || !is_desktop) {
      // Toggle selection (Desktop Ctrl+Click OR Mobile default)
      if (selectedGroupIds.has(id)) {
        selectedGroupIds.delete(id);
      } else {
        selectedGroupIds.add(id);
      }
      selectedGroupIds = new Set(selectedGroupIds); // Trigger reactivity
    } else if (e.shiftKey) {
      // Range selection
      // Find last selected index (or 0 if none)
      let lastIndex = -1;
      const ids = Array.from(selectedGroupIds);
      if (ids.length > 0) {
        const firstSelectedIdx = groupsStore.all.findIndex((g) => g.id === ids[0]); // simplistic
        const start = Math.min(firstSelectedIdx, index);
        const end = Math.max(firstSelectedIdx, index);

        // Add range
        for (let i = start; i <= end; i++) {
          selectedGroupIds.add(groupsStore.all[i].id);
        }
        selectedGroupIds = new Set(selectedGroupIds);
      } else {
        selectedGroupIds.add(id);
        selectedGroupIds = new Set(selectedGroupIds);
      }
    } else {
      // Single select (replace)
      if (!selectedGroupIds.has(id) || selectedGroupIds.size > 1) {
        selectedGroupIds.clear();
        selectedGroupIds.add(id);
        selectedGroupIds = new Set(selectedGroupIds);
      }
    }
  }

  function clearSelection() {
    if (selectedGroupIds.size > 0) {
      selectedGroupIds.clear();
      selectedGroupIds = new Set(selectedGroupIds);
    }
  }

  function cloneGroupWithNewIds(group: Group): Group {
    return {
      ...group,
      id: randomId(),
      rules: group.rules.map((rule) => ({
        ...rule,
        id: randomId(),
      })),
    };
  }

  type VisibleGroup = {
    group_index: number;
    ruleIndices: number[] | null;
  };

  type GroupDragData = {
    group_id: string;
    group_index: number;
    name: string;
    color: string;
    count: number;
  };

  type GroupDropSlotData = {
    group_index: number;
    insert: "before" | "after";
  };

  function handleGroupSlotDrop(source: GroupDragData, target: GroupDropSlotData) {
    const { group_index: from_index, group_id } = source;
    const { group_index: to_index, insert } = target;

    // If source is in selection, we do bulk move
    if (selectedGroupIds.has(group_id)) {
      moveSelectedGroups(to_index, insert);
      return;
    }

    if (from_index === to_index && insert !== "after") return;
    changeGroupIndex(from_index, to_index, insert);
  }

  function moveSelectedGroups(targetIndex: number, insert: "before" | "after") {
    const selectedIndices = groupsStore.all
      .map((g, i) => (selectedGroupIds.has(g.id) ? i : -1))
      .filter((i) => i !== -1);

    if (selectedIndices.length === 0) return;

    // 1. Separate data
    const selectedGroups: Group[] = [];
    const selectedLimits: number[] = [];

    const remainingData: Group[] = [];
    const remainingLimits: number[] = [];

    groupsStore.all.forEach((g, i) => {
      if (selectedGroupIds.has(g.id)) {
        selectedGroups.push(g);
        selectedLimits.push(showed_limit[i]);
      } else {
        remainingData.push(g);
        remainingLimits.push(showed_limit[i]);
      }
    });

    // 2. Calculate insertion index in 'remainingData'
    let insertionIndex = 0;

    const targetGroup = groupsStore.all[targetIndex];

    // Find where targetGroup is in remainingData
    let targetInRemaining = remainingData.findIndex((g) => g.id === targetGroup.id);

    if (targetInRemaining === -1) {
      return;
    }

    if (insert === "before") {
      insertionIndex = targetInRemaining;
    } else {
      insertionIndex = targetInRemaining + 1;
    }

    // 3. Insert
    remainingData.splice(insertionIndex, 0, ...selectedGroups);
    remainingLimits.splice(insertionIndex, 0, ...selectedLimits);

    groupsStore.all = remainingData;
    showed_limit = remainingLimits;
    recomputeVisibleGroups();
  }

  function moveSelectedGroupsStep(direction: "up" | "down") {
    // Create a set of indices to move
    const indices = groupsStore.all
      .map((g, i) => (selectedGroupIds.has(g.id) ? i : -1))
      .filter((i) => i !== -1);

    if (indices.length === 0) return;

    if (direction === "up") {
      const newData = [...groupsStore.all];
      const newLimits = [...showed_limit];
      let moved = false;

      for (let i = 1; i < newData.length; i++) {
        if (selectedGroupIds.has(newData[i].id) && !selectedGroupIds.has(newData[i - 1].id)) {
          // Swap with previous
          [newData[i], newData[i - 1]] = [newData[i - 1], newData[i]];
          [newLimits[i], newLimits[i - 1]] = [newLimits[i - 1], newLimits[i]];
          moved = true;
        }
      }
      if (moved) {
        groupsStore.all = newData;
        showed_limit = newLimits;
        recomputeVisibleGroups();
      }
    } else {
      // Down: Iterate from L-2 to 0
      const newData = [...groupsStore.all];
      const newLimits = [...showed_limit];
      let moved = false;

      for (let i = newData.length - 2; i >= 0; i--) {
        if (selectedGroupIds.has(newData[i].id) && !selectedGroupIds.has(newData[i + 1].id)) {
          // Swap with next
          [newData[i], newData[i + 1]] = [newData[i + 1], newData[i]];
          [newLimits[i], newLimits[i + 1]] = [newLimits[i + 1], newLimits[i]];
          moved = true;
        }
      }
      if (moved) {
        groupsStore.all = newData;
        showed_limit = newLimits;
        recomputeVisibleGroups();
      }
    }
  }

  let searchQuery = $state("");
  let normalizedSearch = $derived(searchQuery.trim().toLowerCase());
  let searchActive = $derived(Boolean(normalizedSearch));
  let visibleGroups: VisibleGroup[] = $state([]);

  function recomputeVisibleGroups() {
    if (!normalizedSearch) {
      visibleGroups = groupsStore.all.map(
        (_, index): VisibleGroup => ({ group_index: index, ruleIndices: null }),
      );
      return;
    }

    visibleGroups = groupsStore.all
      .map<VisibleGroup | null>((group, index) => {
        if (!group) return null;
        const query = normalizedSearch;
        const groupMatch =
          (group.name?.toLowerCase() ?? "").includes(query) ||
          (group.interface?.toLowerCase() ?? "").includes(query);

        const matchedRuleIndices = group.rules
          .map((rule, ruleIndex) => {
            const ruleName = rule.name?.toLowerCase() ?? "";
            const rulePattern = rule.rule?.toLowerCase() ?? "";
            const ruleType = rule.type?.toLowerCase() ?? "";
            const match =
              ruleName.includes(query) || rulePattern.includes(query) || ruleType.includes(query);
            return match ? ruleIndex : -1;
          })
          .filter((idx) => idx !== -1);

        if (groupMatch || matchedRuleIndices.length > 0) {
          return {
            group_index: index,
            ruleIndices: groupMatch && matchedRuleIndices.length === 0 ? null : matchedRuleIndices,
          };
        }

        return null;
      })
      .filter(Boolean) as VisibleGroup[];
  }

  $effect(recomputeVisibleGroups);

  let noVisibleGroups = $derived(searchActive && visibleGroups.length === 0);

  function saveChanges() {
    if (counter === 0) return;
    overlay.show(t("saving changes..."));

    groupsStore
      .save(groupsStore.all)
      .then(() => {
        counter = 0;
        overlay.hide();
      })
      .catch(() => {
        overlay.hide();
      });
  }

  function checkRulesValidityState() {
    valid_rules = !document.querySelector(".rule input.invalid");
  }

  function initOpenState() {
    for (const group of groupsStore.all) {
      if (!open_state.current[group.id]) {
        open_state.current[group.id] = false;
      }
    }
  }

  function cleanOrphanedOpenState() {
    for (const key of Object.keys(open_state.current)) {
      if (!groupsStore.all.some((group) => group.id === key)) {
        delete open_state.current[key];
      }
    }
  }

  onMount(async () => {
    if (groupsStore.all.length === 0 && !groupsStore.loading) {
      await groupsStore.load();
    }

    // Resize limit array if needed
    if (showed_limit.length !== groupsStore.all.length) {
      showed_limit = groupsStore.all.map((group) =>
        group.rules.length > INITIAL_RULES_LIMIT ? INITIAL_RULES_LIMIT : group.rules.length,
      );
    }

    initOpenState();
    setTimeout(cleanOrphanedOpenState, 5000);
    window.addEventListener("keydown", handleSaveShortcut);
  });

  onDestroy(() => {
    window.removeEventListener("keydown", handleSaveShortcut);
  });

  $effect(() => {
    const value = $state.snapshot(groupsStore.all);
    const new_count = untrack(() => counter) + 1;
    counter = new_count;
    if (new_count == 0) return;
    setTimeout(checkRulesValidityState, 10);
  });

  async function addRuleToGroup(group_index: number, rule: Rule, focus = false) {
    groupsStore.all[group_index].rules.unshift(rule);
    showed_limit[group_index]++;
    recomputeVisibleGroups();
    if (!focus) return;
    await tick();
    const el = document.querySelector(`.rule[data-group-index="${group_index}"][data-index="0"]`);
    if (el) {
      requestAnimationFrame(() => {
        el.querySelector<HTMLInputElement>("div.name input")?.focus();
        el.querySelector<HTMLInputElement>("div.pattern input")?.classList.add("invalid");
      });
    }
  }

  function deleteRuleFromGroup(group_index: number, rule_index: number) {
    groupsStore.all[group_index].rules.splice(rule_index, 1);
    recomputeVisibleGroups();
  }

  function changeRuleIndex(
    from_group_index: number,
    from_rule_index: number,
    to_group_index: number,
    to_rule_index: number,
    to_rule_id?: string,
    insert: "before" | "after" = "before",
  ) {
    const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value));

    const sourceGroup = groupsStore.all[from_group_index];
    const targetGroup = groupsStore.all[to_group_index];

    if (!sourceGroup || !targetGroup) return;

    const isSameGroup = from_group_index === to_group_index;

    const sourceRulesNext = [...sourceGroup.rules];
    if (!sourceRulesNext.length) return;

    const fromIndex = clamp(from_rule_index, 0, sourceRulesNext.length - 1);
    const [movedRule] = sourceRulesNext.splice(fromIndex, 1);
    if (!movedRule) return;

    const targetRulesNext = isSameGroup ? sourceRulesNext : [...targetGroup.rules];

    let anchorIndex =
      to_rule_id && to_rule_id.length > 0
        ? targetRulesNext.findIndex((r) => r.id === to_rule_id)
        : -1;

    if (anchorIndex === -1 && targetRulesNext.length > 0) {
      anchorIndex = clamp(to_rule_index, 0, targetRulesNext.length - 1);
    }

    let insertIndex: number;
    if (anchorIndex === -1) {
      insertIndex = insert === "after" ? targetRulesNext.length : 0;
    } else {
      insertIndex = insert === "after" ? anchorIndex + 1 : anchorIndex;
    }

    if (isSameGroup && insertIndex > fromIndex) {
      insertIndex -= 1;
    }

    insertIndex = clamp(insertIndex, 0, targetRulesNext.length);

    targetRulesNext.splice(insertIndex, 0, movedRule);

    const nextData = [...groupsStore.all];

    if (isSameGroup) {
      nextData[from_group_index] = { ...sourceGroup, rules: targetRulesNext };
    } else {
      nextData[from_group_index] = { ...sourceGroup, rules: sourceRulesNext };
      nextData[to_group_index] = { ...targetGroup, rules: targetRulesNext };
    }

    groupsStore.all = nextData;

    if (!isSameGroup) {
      showed_limit[from_group_index] = Math.min(
        showed_limit[from_group_index],
        sourceRulesNext.length,
      );
    }

    const ensureVisibleCount = isSameGroup
      ? Math.min(insertIndex + 1, targetRulesNext.length)
      : Math.min(
          targetRulesNext.length,
          Math.max(insertIndex + 1, showed_limit[to_group_index] + 1),
        );
    if (showed_limit[to_group_index] < ensureVisibleCount) {
      showed_limit[to_group_index] = ensureVisibleCount;
    }

    showed_limit = [...showed_limit];
    recomputeVisibleGroups();
  }

  function changeGroupIndex(
    from_index: number,
    to_index: number,
    insert: "before" | "after" = "before",
  ) {
    if (from_index === to_index && insert !== "after") return;

    if (from_index < 0 || from_index >= groupsStore.all.length) return;

    const g = groupsStore.all[from_index];
    const lim = showed_limit[from_index];
    if (!g) return;

    groupsStore.all.splice(from_index, 1);
    showed_limit.splice(from_index, 1);

    let target = insert === "after" ? to_index + 1 : to_index;

    if (from_index < target) target -= 1;

    if (target < 0) target = 0;
    if (target > groupsStore.all.length) target = groupsStore.all.length;

    groupsStore.all.splice(target, 0, g);
    showed_limit.splice(target, 0, lim);
    recomputeVisibleGroups();
  }

  async function addGroup() {
    groupsStore.all.unshift(defaultGroup());
    showed_limit.unshift(INITIAL_RULES_LIMIT);
    open_state.current[groupsStore.all[0].id] = true;
    recomputeVisibleGroups();
    await addRuleToGroup(0, defaultRule(), false);
    await tick();
    const el = document.querySelector(`.group-header[data-group-index="0"]`);
    el?.querySelector<HTMLInputElement>("input.group-name")?.focus();
  }

  function deleteGroup(index: number) {
    if (!confirm(t("Delete this group?"))) return;
    groupsStore.all.splice(index, 1);
    showed_limit.splice(index, 1);
    recomputeVisibleGroups();
  }

  function moveGroupUp(index: number) {
    if (index > 0) {
      changeGroupIndex(index, index - 1, "before");
    }
  }

  function moveGroupDown(index: number) {
    if (index < groupsStore.all.length - 1) {
      changeGroupIndex(index, index + 1, "after");
    }
  }

  function exportConfig() {
    exportConfigModal = { open: true };
  }

  function handleExport(e: CustomEvent<{ groups: Group[] }>) {
    const blob = new Blob([JSON.stringify({ groups: e.detail.groups })], {
      type: "application/json",
    });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    const now = new Date();
    const pad = (n: number) => n.toString().padStart(2, "0");
    const dateStr = `${pad(now.getDate())}-${pad(now.getMonth() + 1)}-${now.getFullYear()}`;
    const timeStr = `${pad(now.getHours())}-${pad(now.getMinutes())}`;
    link.download = `config_${dateStr}_${timeStr}.mtrickle`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }

  function processConfigFile(file: File) {
    if (!file) {
      alert(t("Please select a CONFIG file to load."));
      return;
    }

    const reader = new FileReader();
    reader.onload = (event) => {
      try {
        const { groups } = parseConfig(event.target?.result as string);

        if (!groups?.length) {
          toast.error(t("Invalid config file"));
          return;
        }

        importConfigModal = {
          open: true,
          groups,
          fileName: file.name,
        };
      } catch (error) {
        console.error("Error parsing CONFIG:", error);
        toast.error(t("Invalid config file"));
      }
    };
    reader.onerror = (event) => {
      console.error("Error reading file:", event.target?.error);
      toast.error(t("Invalid config file"));
    };
    reader.readAsText(file);
  }

  async function loadMore(group_index: number): Promise<void> {
    if (showed_limit[group_index] >= groupsStore.all[group_index].rules.length) return;
    showed_limit[group_index] += INCREMENT_RULES_LIMIT;
    if (showed_limit[group_index] > groupsStore.all[group_index].rules.length) {
      showed_limit[group_index] = groupsStore.all[group_index].rules.length;
      return;
    }
    loaderState.loaded();
  }

  function openImportRulesModal(groupIndex: number) {
    importRulesModal = { open: true, groupIndex };
  }

  function closeImportRulesModal() {
    importRulesModal = { open: false, groupIndex: null };
  }
  function startSelection(e: MouseEvent) {
    // Ignore if clicking on interactive elements or scrollbars
    if (
      (e.target as HTMLElement).closest(
        "button, input, label, a, .group-header, .rule, .bulk-actions, select, option, .console-window",
      )
    )
      return;

    isSelecting = true;
    document.body.classList.add("is-selecting");
    selectionBox = {
      startX: e.clientX,
      startY: e.clientY + window.scrollY,
      currentX: e.clientX,
      currentY: e.clientY + window.scrollY,
      active: true,
    };

    // If not holding Ctrl/Shift, clicking background clears selection
    if (!e.ctrlKey && !e.shiftKey && !e.metaKey) {
      clearSelection();
    }
  }

  function collapseAll() {
    for (const group of groupsStore.all) {
      open_state.current[group.id] = false;
    }
  }

  function expandAll() {
    for (const group of groupsStore.all) {
      open_state.current[group.id] = true;
    }
  }

  function updateSelection(e: MouseEvent) {
    if (!isSelecting || !selectionBox) return;

    selectionBox.currentX = e.clientX;
    selectionBox.currentY = e.clientY + window.scrollY;

    // Calculate intersection with groups
    const boxRect = {
      left: Math.min(selectionBox.startX, selectionBox.currentX),
      top: Math.min(selectionBox.startY, selectionBox.currentY),
      right: Math.max(selectionBox.startX, selectionBox.currentX),
      bottom: Math.max(selectionBox.startY, selectionBox.currentY),
    };

    // Query all visible groups
    const groupElements = document.querySelectorAll(".group[data-uuid]");
    const newSelection = new Set(selectedGroupIds);

    // If we provided a "base" selection before drag, we should handle that.
    // For now, simpler: re-evaluate intersections.
    // If Ctrl is held, we Toggle? Or Add?
    // Windows behavior: Dragging box inverts? Or adds? Usually adds.

    groupElements.forEach((el) => {
      const rect = el.getBoundingClientRect();
      const absoluteTop = rect.top + window.scrollY; // Adjust for scroll

      // Check collision
      const intersect = !(
        boxRect.right < rect.left ||
        boxRect.left > rect.right ||
        boxRect.bottom < absoluteTop ||
        boxRect.top > absoluteTop + rect.height
      );

      const id = el.getAttribute("data-uuid");
      if (id) {
        if (intersect) {
          newSelection.add(id);
        } else if (!e.ctrlKey) {
          // If NOT ctrl key, and not intersecting, we might remove it if it wasn't pre-selected?
          // Complex behavior. Let's stick to "Add to selection" for simplicity first.
          // Actually, if I just drag box, I expect it to Select ONLY what is in box (unless Ctrl).
          if (!selectedGroupIds.has(id)) {
            // It wasn't selected before, so it shouldn't be now.
          }
        }
      }
    });

    // A simpler approach for "in-flight" selection:
    // Just track what is currently in box.
    // But we need to persist what was already selected if Ctrl.
  }

  function endSelection() {
    if (isSelecting && selectionBox) {
      // Finalize selection logic
      const boxRect = {
        left: Math.min(selectionBox.startX, selectionBox.currentX),
        top: Math.min(selectionBox.startY, selectionBox.currentY),
        right: Math.max(selectionBox.startX, selectionBox.currentX),
        bottom: Math.max(selectionBox.startY, selectionBox.currentY),
      };

      // Minimal drag check to avoid clearing on simple clicks handled by click handlers
      const dragDist = Math.hypot(
        selectionBox.currentX - selectionBox.startX,
        selectionBox.currentY - selectionBox.startY,
      );
      if (dragDist > 5) {
        const groupElements = document.querySelectorAll(".group[data-uuid]");
        groupElements.forEach((el) => {
          const rect = el.getBoundingClientRect();
          const absoluteTop = rect.top + window.scrollY;

          const intersect = !(
            boxRect.right < rect.left ||
            boxRect.left > rect.right ||
            boxRect.bottom < absoluteTop ||
            boxRect.top > absoluteTop + rect.height
          );

          const id = el.getAttribute("data-uuid");
          if (id && intersect) {
            selectedGroupIds.add(id);
          }
        });
        selectedGroupIds = new Set(selectedGroupIds);
      }
    }
    isSelecting = false;
    document.body.classList.remove("is-selecting");
    selectionBox = null;
  }

  function deleteSelectedGroups() {
    if (!confirm(t("Delete selected groups?"))) return;

    const newData: Group[] = [];
    const newShowedLimit: number[] = [];

    groupsStore.all.forEach((g, i) => {
      if (!selectedGroupIds.has(g.id)) {
        newData.push(g);
        newShowedLimit.push(showed_limit[i]);
      }
    });

    groupsStore.all = newData;
    showed_limit = newShowedLimit;
    clearSelection();
    recomputeVisibleGroups();
  }
</script>

<svelte:window
  bind:innerWidth={client_width}
  on:mousedown={startSelection}
  on:mousemove={updateSelection}
  on:mouseup={endSelection}
  on:keydown={(e) => {
    if (e.key === "Escape") clearSelection();
    handleSaveShortcut(e);
  }}
/>

<div class="groups-page">
  <div class="group-controls">
    <div class="group-controls-search">
      <input
        type="search"
        placeholder={t("Search groups and rules...")}
        class="group-search-input"
        bind:value={searchQuery}
      />
    </div>
    <div class="group-controls-actions">
      {#if counter > 0 && valid_rules}
        <div transition:scale>
          <Tooltip value={t("Save Changes")}>
            <Button onclick={saveChanges} id="save-changes">
              <Save size={22} />
            </Button>
          </Tooltip>
        </div>
      {/if}
      <Tooltip value={t("Export Config")}>
        <Button onclick={exportConfig}>
          <Download size={22} />
        </Button>
      </Tooltip>
      <Tooltip value={t("Import Config")}>
        <Button
          onclick={() => {
            importConfigModal = { open: true, groups: [], fileName: "" };
          }}
        >
          <Upload size={22} />
        </Button>
      </Tooltip>
      <div class="separator"></div>
      <Tooltip value={t("Collapse All")}>
        <Button onclick={collapseAll}><CollapseAll size="22" /></Button>
      </Tooltip>
      <Tooltip value={t("Expand All")}>
        <Button onclick={expandAll}><ExpandAll size="22" /></Button>
      </Tooltip>
      <div class="separator"></div>
      <Tooltip value={t("Add Group")}>
        <Button onclick={addGroup}><Add size="22" /></Button>
      </Tooltip>
    </div>
  </div>

  {#if noVisibleGroups}
    <div class="no-groups">{t("No matches found")}</div>
  {/if}

  {#if selectionBox && selectionBox.active}
    <div
      class="selection-box"
      style="
          left: {Math.min(selectionBox.startX, selectionBox.currentX)}px;
          top: {Math.min(selectionBox.startY, selectionBox.currentY) - window.scrollY}px;
          width: {Math.abs(selectionBox.currentX - selectionBox.startX)}px;
          height: {Math.abs(selectionBox.currentY - selectionBox.startY)}px;
      "
    ></div>
  {/if}

  {#if selectedGroupIds.size > 0}
    {@const selectedGroups = groupsStore.all.filter((g) => selectedGroupIds.has(g.id))}
    {@const allEnabled = selectedGroups.every((g) => g.enable)}
    {@const anyEnabled = selectedGroups.some((g) => g.enable)}
    <div class="bulk-actions" transition:scale>
      <div class="bulk-header">
        <div class="bulk-count">
          {t("{count} selected").replace("{count}", selectedGroupIds.size.toString())}
        </div>

        <div class="bulk-header-actions">
          <div class="bulk-switch-wrapper" title={t("Toggle Selection")}>
            <Switch
              checked={allEnabled}
              mixed={anyEnabled && !allEnabled}
              onCheckedChange={() => {
                const newState = anyEnabled && !allEnabled ? true : !allEnabled;
                groupsStore.all.forEach((g) => {
                  if (selectedGroupIds.has(g.id)) g.enable = newState;
                });
                groupsStore.all = [...groupsStore.all];
              }}
            />
          </div>

          <Tooltip value={t("Move Up")}>
            <Button small onclick={() => moveSelectedGroupsStep("up")}>
              <MoveUp size={18} />
            </Button>
          </Tooltip>
          <Tooltip value={t("Move Down")}>
            <Button small onclick={() => moveSelectedGroupsStep("down")}>
              <MoveDown size={18} />
            </Button>
          </Tooltip>

          <Tooltip value={t("Delete Selected")}>
            <Button small onclick={deleteSelectedGroups} style="color: #ff4d4f;">
              <Delete size={18} />
            </Button>
          </Tooltip>
        </div>
      </div>

      <div class="bulk-body">
        <div class="bulk-select-interface">
          <Select
            options={[
              ...INTERFACES.map((item) => ({
                value: item.id,
                label: item.active
                  ? getInterfaceLabel(item.id)
                  : `<span style="color: #ff4d4f;">${t("interface.inactive")}</span> ${getInterfaceLabel(item.id)}`,
                html: true,
              })),
            ]}
            selected={commonInterface}
            placeholder={t("Set Interface...")}
            onValueChange={(val) => {
              if (!val) return;
              groupsStore.all.forEach((g) => {
                if (selectedGroupIds.has(g.id)) g.interface = val;
              });
              groupsStore.all = [...groupsStore.all];
            }}
          />
        </div>

        <Button small secondary onclick={clearSelection} class="bulk-cancel-btn"
          >{t("Cancel")}</Button
        >
      </div>
    </div>
  {/if}

  {#each visibleGroups as visible, index (groupsStore.all[visible.group_index]?.id)}
    {#if groupsStore.all[visible.group_index]}
      <div class="group-wrapper">
        {#if index === 0}
          <div
            class="group-drop-slot group-drop-slot--top"
            aria-hidden="true"
            use:droppable={{
              data: { group_index: visible.group_index, insert: "before" } as GroupDropSlotData,
              scope: "group",
              canDrop: (source: GroupDragData, target: GroupDropSlotData) =>
                source.group_index !== target.group_index,
              dropEffect: "move",
              onDrop: handleGroupSlotDrop,
            }}
          ></div>
        {/if}
        <GroupPanel
          bind:group={groupsStore.all[visible.group_index]}
          group_index={visible.group_index}
          bind:total_groups={groupsStore.all.length}
          bind:showed_limit={showed_limit[visible.group_index]}
          bind:open={open_state.current[groupsStore.all[visible.group_index].id]}
          selected={selectedGroupIds.has(groupsStore.all[visible.group_index].id)}
          on:select={handleGroupSelect}
          {deleteGroup}
          {moveGroupUp}
          {moveGroupDown}
          {addRuleToGroup}
          {deleteRuleFromGroup}
          {changeRuleIndex}
          {loadMore}
          {searchActive}
          visibleRuleIndices={visible.ruleIndices}
          on:importRules={() => openImportRulesModal(visible.group_index)}
        />
        <div
          class="group-drop-slot group-drop-slot--bottom"
          aria-hidden="true"
          use:droppable={{
            data: { group_index: visible.group_index, insert: "after" } as GroupDropSlotData,
            scope: "group",
            canDrop: () => true,
            dropEffect: "move",
            onDrop: handleGroupSlotDrop,
          }}
        ></div>
      </div>
    {/if}
  {/each}
</div>

<ImportRulesDialog
  open={importRulesModal.open}
  group_index={importRulesModal.groupIndex}
  on:close={closeImportRulesModal}
  on:import={(e) => {
    const { group_index, rules } = e.detail;
    groupsStore.all[group_index].rules.unshift(...rules);
    if (rules.length > 500) {
      showed_limit[group_index] = Math.max(showed_limit[group_index], 30);
    } else {
      showed_limit[group_index] = Math.min(
        showed_limit[group_index] + rules.length,
        groupsStore.all[group_index].rules.length,
      );
    }
  }}
/>

<ImportConfigDialog
  open={importConfigModal.open}
  groups={importConfigModal.groups}
  fileName={importConfigModal.fileName}
  on:close={resetImportConfigModal}
  on:file={(e) => processConfigFile(e.detail.file)}
  on:import={(e) => {
    const imported = e.detail.groups.map(cloneGroupWithNewIds);
    if (!imported.length) return;

    if (e.detail.replace) {
      groupsStore.all = [];
      showed_limit = [];
    }

    for (let i = imported.length - 1; i >= 0; i--) {
      const group = imported[i];
      groupsStore.all.unshift(group);
      showed_limit.unshift(
        group.rules.length > INITIAL_RULES_LIMIT ? INITIAL_RULES_LIMIT : group.rules.length,
      );
      open_state.current[group.id] = true;
    }

    if (e.detail.replace) {
      toast.success(`${t("Config replaced")}: ${imported.length}`);
    } else {
      toast.success(`${t("Config imported")}: ${imported.length}`);
    }
  }}
/>

<ExportConfigDialog
  open={exportConfigModal.open}
  groups={groupsStore.all}
  on:close={resetExportConfigModal}
  on:export={handleExport}
/>

<style>
  .group-wrapper {
    position: relative;
    margin: 1rem 0;
  }

  .group-wrapper:first-of-type {
    margin-top: 1rem;
  }

  .group-wrapper:last-of-type {
    margin-bottom: 1rem;
  }

  .separator {
    width: 1px;
    height: 24px;
    background: color-mix(in oklab, var(--text) 20%, transparent);
    margin: 0 0.25rem;
  }

  .group-drop-slot {
    position: absolute;
    left: 0;
    right: 0;
    height: 1rem;
    pointer-events: none;
    background: color-mix(in oklab, var(--accent) 28%, transparent);
    box-shadow: inset 0 0 0 2px color-mix(in oklab, var(--accent) 54%, transparent);
    opacity: 0;
    transition: opacity 0.12s ease;
  }

  .group-drop-slot--top {
    top: -1rem;
  }

  .group-drop-slot--bottom {
    bottom: -1rem;
  }

  :global(html[data-dnd-scope="group"]) .group-drop-slot {
    pointer-events: auto;
  }

  :global(html[data-dnd-scope="group"]) .group-drop-slot:global(.dragover) {
    opacity: 1;
  }

  .group-controls {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 0.75rem;
    padding: 0.75rem 0.25rem;
    margin-bottom: 0.75rem;
    position: sticky;
    top: 0;
    z-index: 20;
    background: color-mix(in oklab, var(--bg-dark) 92%, var(--bg-dark-extra) 8%);
  }

  .group-controls-search {
    flex: 1 1 58px;
  }

  @media (max-width: 700px) {
    .group-controls-search {
      flex: 1 1 100%;
    }
  }

  .group-search-input {
    width: 100%;
    padding: 0.7rem 0.85rem;
    border-radius: 0.5rem;
    border: 1px solid var(--bg-light-extra);
    background-color: var(--bg-light);
    color: var(--text);
    font: inherit;
    font-size: 1rem;
    line-height: 1.3;
    min-height: 2.7rem;
    transition:
      border-color 0.12s ease,
      box-shadow 0.18s ease,
      background-color 0.12s ease,
      color 0.12s ease;
  }

  .group-search-input:hover {
    background-color: color-mix(in oklab, var(--bg-light) 92%, var(--bg-light-extra) 8%);
    border-color: color-mix(in oklab, var(--bg-light-extra) 90%, transparent);
    color: var(--text);
  }

  .group-search-input:focus-visible {
    outline: none;
    border-color: var(--accent);
    box-shadow:
      0 0 0 1px color-mix(in oklab, var(--accent) 45%, transparent),
      0 6px 18px -14px color-mix(in oklab, var(--accent) 35%, transparent);
    background-color: var(--bg-light);
    color: var(--text);
  }

  .group-controls-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  @media (max-width: 700px) {
    .group-controls-actions {
      width: 100%;
      justify-content: flex-end;
    }
  }

  .no-groups {
    width: 100%;
    text-align: center;
    padding: 2rem 0;
    color: color-mix(in oklab, var(--text) 75%, transparent);
  }

  /* Bulk Selection Styles */
  .selection-box {
    position: fixed;
    background: rgba(0, 120, 215, 0.2);
    border: 1px solid rgba(0, 120, 215, 0.6);
    pointer-events: none;
    z-index: 9999;
  }

  .bulk-actions {
    position: fixed;
    top: 5rem;
    left: 50%;
    transform: translateX(-50%);
    background: var(--bg-dark);
    border: 1px solid var(--accent);
    padding: 0.75rem 1rem;
    border-radius: 1rem;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    z-index: 100;
    min-width: 340px;
  }

  :global(body.is-selecting) {
    user-select: none;
    -webkit-user-select: none;
  }

  .bulk-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
    margin-bottom: 0.5rem;
  }

  .bulk-header-actions {
    display: flex;
    align-items: center; /* Ensure vertical center alignment */
    /* If height discrepancies exist, we might need a fixed height or line-height adjustments */
    height: 100%;
    gap: 0.25rem;
  }

  :global(.bulk-header-actions button) {
    display: flex;
    align-items: center;
  }

  .bulk-body {
    display: flex;
    align-items: center;
    width: 100%;
    gap: 0.5rem;
  }

  .bulk-select-interface {
    flex: 1;
    min-width: 0; /* Critical for truncation in flex items */
    margin: 0;
    margin-right: 0.25rem;
  }

  /* Force correct width on the wrapper div from Select.svelte */
  :global(.bulk-select-interface .select-wrap) {
    width: 100% !important;
    min-width: 0;
    display: block; /* Ensure it takes width */
  }

  /* Force truncation on the Select trigger inside */
  .bulk-select-interface :global([data-select-trigger]),
  .bulk-select-interface :global(button[data-select-trigger]) {
    width: 100% !important;
    display: flex !important;
    max-width: 100%;
    min-width: 0;
    background-color: var(--bg-light-extra) !important; /* Lighter background */
    border: 1px solid var(--border-light);
    color: var(--text);
  }

  /* Target the internal wrapper 'selected' */
  .bulk-select-interface :global(.selected) {
    width: 100% !important;
    max-width: 100% !important;
    display: flex !important;
    align-items: center;
    justify-content: space-between;
  }

  /* Target the value text */
  .bulk-select-interface :global(.selected-value) {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text);
  }

  .bulk-switch-wrapper {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    height: 100%;
  }

  .bulk-count {
    font-weight: 600;
    color: var(--text);
    white-space: nowrap;
  }

  @media (max-width: 700px) {
    .bulk-actions {
      width: 90%;
      min-width: 0;
    }

    .bulk-body {
      gap: 0.5rem;
      justify-content: space-between;
    }
  }
</style>
