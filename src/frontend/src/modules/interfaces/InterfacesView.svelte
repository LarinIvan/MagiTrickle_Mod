<script lang="ts">
  import { onMount } from "svelte";
  import { INTERFACES } from "../../data/interfaces.svelte";
  import { aliases } from "../../data/aliases.svelte";
  import { t } from "../../data/locale.svelte";
  import Button from "../../components/ui/Button.svelte";
  import Tooltip from "../../components/ui/Tooltip.svelte";
  import { Save, Upload, Download, Globe, Gauge } from "../../components/ui/icons";
  import { toast } from "../../utils/events";
  import { fetcher } from "../../utils/fetcher";
  import { groupsStore } from "../../data/groups.svelte";
  import { settings } from "../../data/settings.svelte";
  import SpeedtestModal from "../../modules/diagnostics/SpeedtestModal.svelte";

  let { visible = false } = $props();

  let interfaceList = $state<{ id: string; alias: string; active?: boolean; ip?: string }[]>([]);
  let hasChanges = $state(false);
  let fileInput: HTMLInputElement;

  let externalIPs = $state<Record<string, string | "loading" | "error">>({});
  let globalLoading = $state(false);
  let hasInitialCheck = $state(false);
  let showSpeedtest = $state<string | null>(null);

  // Stats
  let stats = $derived(groupsStore.stats);

  onMount(async () => {
    await aliases.load();
    if (groupsStore.all.length === 0 && !groupsStore.loading) {
      await groupsStore.load();
    }
    refreshList();
  });

  $effect(() => {
    if (visible && !hasInitialCheck && settings.config.show_interface_ips) {
      hasInitialCheck = true;
      checkExternalIPs();
    }
  });

  async function checkExternalIPs() {
    globalLoading = true;

    const activeInterfaces = interfaceList.filter(
      (i) => (i.active && i.ip) || (i.active && i.id === "blackhole"),
    );
    activeInterfaces.forEach((i) => {
      externalIPs[i.id] = "loading";
    });

    const promises = activeInterfaces.map(async (i) => {
      try {
        const res = await fetcher.get<{ ip: string }>(`/system/interfaces/${i.id}/external-ip`);
        if (res.ip) {
          externalIPs[i.id] = res.ip;
        } else {
          externalIPs[i.id] = "error";
        }
      } catch (e) {
        externalIPs[i.id] = "error";
      }
    });

    try {
      await Promise.allSettled(promises);
      toast.success(t("External IPs updated"));
    } finally {
      globalLoading = false;
    }
  }

  function refreshList() {
    const systemIds = new Set(INTERFACES.map((i) => i.id));
    const aliasIds = Object.keys(aliases.all);

    // known interfaces in system order
    const known = INTERFACES.map((item) => ({
      id: item.id,
      alias: aliases.all[item.id] || "",
      active: item.active,
      ip: item.ip,
    }));

    // extra interfaces (orphaned aliases)
    const extra = aliasIds
      .filter((id) => !systemIds.has(id))
      .sort()
      .map((id) => ({
        id,
        alias: aliases.all[id],
      }));

    interfaceList = [...known, ...extra];
  }

  function handleChange() {
    hasChanges = true;
  }

  async function save() {
    const newAliases: Record<string, string> = {};
    interfaceList.forEach((item) => {
      if (item.alias.trim()) {
        newAliases[item.id] = item.alias.trim();
      }
    });

    const success = await aliases.save(newAliases);
    if (success) {
      toast.success(t("Aliases saved successfully"));
      hasChanges = false;
    } else {
      toast.error(t("Failed to save aliases"));
    }
  }

  function exportConfig() {
    const data = JSON.stringify(aliases.all, null, 2);
    const blob = new Blob([data], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "config_interfaces.mtrickle";
    a.click();
    URL.revokeObjectURL(url);
  }

  function triggerImport() {
    fileInput.click();
  }

  async function importConfig(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;

    const text = await file.text();
    try {
      const parsed = JSON.parse(text);

      const systemIds = new Set(INTERFACES.map((i) => i.id));
      const aliasIds = Object.keys(parsed);
      const known = INTERFACES.map((item) => ({
        id: item.id,
        alias: parsed[item.id] || "",
        active: item.active,
        ip: item.ip,
      }));
      const extra = aliasIds
        .filter((id) => !systemIds.has(id))
        .sort()
        .map((id) => ({
          id,
          alias: parsed[id],
        }));
      interfaceList = [...known, ...extra];

      hasChanges = true;
      toast.success(t("Config imported"));
    } catch (err) {
      console.error(err);
      toast.error(t("Invalid config file"));
    }
    (e.target as HTMLInputElement).value = "";
  }
</script>

<div class="interfaces-view">
  <div class="header">
    <h2>{t("Interface Aliases")}</h2>
    <div class="actions">
      {#if settings.config.show_interface_ips}
        <Tooltip value={t("Check External IPs")}>
          <Button small onclick={checkExternalIPs} disabled={globalLoading}>
            <Globe size={20} class={globalLoading ? "spin" : ""} />
          </Button>
        </Tooltip>
      {/if}
      <Tooltip value={t("Import Interface Config")}>
        <Button small onclick={triggerImport}>
          <Upload size={20} />
        </Button>
      </Tooltip>
      <Tooltip value={t("Export Interface Config")}>
        <Button small onclick={exportConfig}>
          <Download size={20} />
        </Button>
      </Tooltip>
      {#if hasChanges}
        <Tooltip value={t("Save Changes")}>
          <Button small onclick={save}>
            <Save size={20} />
          </Button>
        </Tooltip>
      {/if}
    </div>
  </div>

  <!-- Global Stats Header Row -->
  <div class="stats-header-row">
    <div class="header-left">
      <div class="stat-header-col">
        <span class="stat-label">{t("All Interfaces / Used")}</span>
        <span class="stat-value"
          >{interfaceList.length} <span class="stat-sep">/</span>
          <span class="stat-active">{stats.global.usedInterfaces}</span></span
        >
      </div>
    </div>

    <!-- Right side aligns exactly with .interface-stats (300px grid) -->
    <div class="header-right">
      <div class="stat-header-col">
        <span class="stat-label">{t("Groups / Active")}</span>
        <span class="stat-value"
          >{stats.global.groups} <span class="stat-sep">/</span>
          <span class="stat-active">{stats.global.activeGroups}</span></span
        >
      </div>
      <div class="stat-header-col">
        <span class="stat-label">{t("Rules / Active")}</span>
        <span class="stat-value"
          >{stats.global.rules} <span class="stat-sep">/</span>
          <span class="stat-active">{stats.global.activeRules}</span></span
        >
      </div>
    </div>
  </div>

  <input
    type="file"
    bind:this={fileInput}
    onchange={importConfig}
    style="display: none"
    accept=".json,.mtrickle"
  />

  <div class="list">
    {#each interfaceList as item}
      <div class="interface-row">
        <div class="interface-info">
          <!-- Status Dot -->
          <div
            class="status-dot"
            class:active={item.active}
            title={item.active ? t("interface.active") : t("interface.inactive")}
          ></div>

          <div class="interface-details">
            <div class="interface-id">{item.id}</div>
            {#if settings.config.show_interface_ips}
              {#if item.ip}
                <div class="interface-ip" title={t("interface.ip")}>{item.ip}</div>
              {:else if item.id === "blackhole"}
                <div class="interface-ip" title={t("interface.ip")}>0.0.0.0</div>
              {/if}

              {#if externalIPs[item.id] === "loading"}
                <div class="interface-external-ip loading">
                  🌐 <span class="shimmer">{t("checking...")}</span>
                </div>
              {:else if externalIPs[item.id] === "error"}
                <div
                  class="interface-external-ip error"
                  title={t("Failed to look up external IP for this interface")}
                >
                  🌐 {t("Failed to get IP")}
                </div>
              {:else if externalIPs[item.id]}
                <div class="interface-external-ip success">
                  🌐 {externalIPs[item.id]}
                </div>
              {:else}
                <!-- Undefined/Null case: do nothing or show checking if wanted -->
              {/if}
            {/if}
          </div>

          <input
            type="text"
            placeholder={t("Alias (optional)")}
            bind:value={item.alias}
            oninput={handleChange}
            class="alias-input"
          />
        </div>

        <div class="interface-actions-row">
          <Tooltip value={t("Speed Test")}>
            <Button small variant="ghost" onclick={() => (showSpeedtest = item.id)}>
              <Gauge size={18} />
            </Button>
          </Tooltip>
        </div>

        <div class="interface-stats">
          {#if stats.perInterface[item.id]}
            <div class="stat-cell" title={t("Groups bound to this interface")}>
              <span class="stat-label-mini">{t("Groups / Active")}:</span>
              <span class="stat-value-mini"
                >{stats.perInterface[item.id].groups} <span class="stat-sep">/</span>
                <span class="stat-active">{stats.perInterface[item.id].activeGroups}</span></span
              >
            </div>
            <div class="stat-cell" title={t("Total rules in these groups")}>
              <span class="stat-label-mini">{t("Rules / Active")}:</span>
              <span class="stat-value-mini"
                >{stats.perInterface[item.id].rules} <span class="stat-sep">/</span>
                <span class="stat-active">{stats.perInterface[item.id].activeRules}</span></span
              >
            </div>
          {:else}
            <span class="no-stats">{t("Unused")}</span>
          {/if}
        </div>
      </div>
    {/each}
  </div>

  {#if showSpeedtest}
    {@const selectedInterface = interfaceList.find((i) => i.id === showSpeedtest)}
    <SpeedtestModal
      interfaceId={showSpeedtest}
      interfaceName={selectedInterface
        ? selectedInterface.alias
          ? `${selectedInterface.alias} [${selectedInterface.id}]`
          : selectedInterface.id
        : showSpeedtest}
      interfaceIP={externalIPs[showSpeedtest] &&
      externalIPs[showSpeedtest] !== "loading" &&
      externalIPs[showSpeedtest] !== "error"
        ? externalIPs[showSpeedtest]
        : undefined}
      onClose={() => (showSpeedtest = null)}
    />
  {/if}
</div>

<style>
  .interfaces-view {
    padding: 1rem;
    padding: 1rem;
    max-width: 900px;
    margin: 0 auto;
    width: 100%;
  }

  /* Header Row Alignment */
  .stats-header-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 1rem; /* Match list padding */
    margin-bottom: 1rem;
    gap: 1rem;
    color: var(--text-secondary);
  }

  .header-left {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 2rem; /* More spacing for distinct columns */
    padding-left: 1rem; /* Align/Pad similarly */
  }

  .header-right {
    width: 300px; /* EXACT match of .interface-stats width */
    display: grid;
    grid-template-columns: 1fr 1fr; /* 2 columns now */
    gap: 0.5rem;
    flex-shrink: 0;
    padding-left: 1rem;
    /* No border here in header to keep it clean */
  }

  .stat-header-col {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    line-height: 1.2;
  }

  .stat-label {
    font-weight: 500;
    font-size: 0.7rem; /* Smaller label to fit header */
    text-transform: uppercase;
    letter-spacing: 0.05em;
    opacity: 0.7;
    white-space: nowrap;
  }

  .stat-value {
    font-weight: 700;
    color: var(--text);
    font-size: 1rem;
  }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .actions {
    display: flex;
    gap: 0.5rem;
  }

  h2 {
    font-size: 1.5rem;
    font-weight: 600;
    color: var(--text);
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .interface-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 1rem;
    background-color: var(--bg-light);
    border-radius: 0.5rem;
    border: 1px solid var(--bg-light-extra);
    transition: border-color 0.2s;
  }

  .interface-row:hover {
    border-color: var(--accent);
  }

  .interface-info {
    display: flex;
    align-items: center;
    gap: 1rem;
    flex: 1;
  }

  .interface-id {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--accent);
    /* width: 150px; Removed fixed width to accommodate flex column */
    flex-shrink: 0;
  }

  .interface-details {
    display: flex;
    flex-direction: column;
    width: 150px;
    flex-shrink: 0;
  }

  .interface-ip {
    font-size: 0.8rem;
    color: var(--text-2);
    font-family: monospace;
  }

  .interface-external-ip {
    font-size: 0.8rem;
    font-family: monospace;
    margin-top: 0.1rem;
    display: flex;
    align-items: center;
    gap: 0.3rem;
  }

  .interface-external-ip.success {
    color: #10b981; /* Green */
  }

  .interface-external-ip.error {
    color: var(--danger);
    font-size: 0.7rem; /* Slightly smaller for error message */
  }

  .interface-external-ip.loading {
    color: var(--text-2);
  }

  .shimmer {
    background: linear-gradient(90deg, var(--text-2) 25%, var(--text) 50%, var(--text-2) 75%);
    background-size: 200% 100%;
    background-clip: text;
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    animation: shimmer 1.5s infinite;
  }

  @keyframes shimmer {
    0% {
      background-position: 200% 0;
    }
    100% {
      background-position: -200% 0;
    }
  }

  .status-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background-color: var(--bg-light-extra);
    border: 1px solid var(--text-2);
    flex-shrink: 0;
  }
  .status-dot.active {
    background-color: #10b981; /* Green */
    border-color: #059669;
    box-shadow: 0 0 5px rgba(16, 185, 129, 0.4);
  }

  .alias-input {
    border: none;
    background-color: transparent;
    font-size: 1.3rem;
    font-weight: 600;
    font-family: var(--font);
    color: var(--text);
    border-bottom: 1px solid transparent;
    flex: 1;
    padding: 0.2rem 0.5rem;
    min-width: 0;

    &:focus {
      outline: none;
      border-bottom: 1px solid var(--accent);
    }

    &::placeholder {
      font-weight: 400;
      font-size: 1rem;
      opacity: 0.5;
    }
  }

  .interface-actions-row {
    margin-left: auto;
    padding-right: 1rem;
  }

  .interface-stats {
    display: grid;
    grid-template-columns: 1fr 1fr; /* 2 columns now */
    width: 300px;
    gap: 0.5rem;
    align-items: center;
    padding: 0 0.5rem; /* Balanced padding */
    border-left: 1px solid var(--bg-light-extra);
    flex-shrink: 0;
  }

  .stat-cell {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.1rem;
    background: color-mix(in oklab, var(--bg-dark) 50%, transparent);
    padding: 0.3rem 0.4rem;
    border-radius: 0.4rem;
    font-size: 0.85rem;
    border: 1px solid var(--bg-light-extra);
    white-space: nowrap;
    height: 100%;
    text-align: center; /* Ensure text centering */
  }

  .stat-label-mini {
    color: var(--text-secondary, #9ca3af);
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .stat-value-mini {
    font-weight: 600;
    color: var(--text);
    font-size: 0.95rem;
  }

  .no-stats {
    grid-column: 1 / -1;
    text-align: center;
    font-size: 0.85rem;
    color: var(--text-secondary, #9ca3af);
    font-style: italic;
    padding: 0 0.5rem;
  }
</style>
