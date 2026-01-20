<script lang="ts">
  import { t } from "../../data/locale.svelte";
  import { settings } from "../../data/settings.svelte";
  import { consoleStore } from "../../data/console.svelte";
  import { updater } from "../../data/updater.svelte";
  import { fetcher } from "../../utils/fetcher";
  import { toast } from "../../utils/events";
  import Button from "../../components/ui/Button.svelte";
  import Switch from "../../components/ui/Switch.svelte";
  import Select from "../../components/ui/Select.svelte";
  import {
    Settings,
    RefreshCw,
    TerminalSquare,
    Regex,
    Network,
    ArrowUpCircle,
  } from "../../components/ui/icons";
  import ChangelogDialog from "./dialogs/ChangelogDialog.svelte";

  let isRestarting = $state(false);
  let changelogOpen = $state(false);

  const logLevels = [
    { value: "debug", label: "Log Level Debug" },
    { value: "info", label: "Log Level Info" },
    { value: "warn", label: "Log Level Warn" },
    { value: "error", label: "Log Level Error" },
  ];

  async function handleRestart() {
    if (!confirm(t("Restart Confirmation"))) return;

    isRestarting = true;
    try {
      await fetcher.post("/system/restart", {});
      toast.success(t("Service restart initiated"));
    } catch (e: any) {
      toast.error(e.message);
    } finally {
      setTimeout(() => {
        isRestarting = false;
        window.location.reload();
      }, 10000);
    }
  }

  function handleLogLevelChange(val: string) {
    console.log("Log Level changed to:", val);
    settings.save({ ...settings.config, log_level: val });
  }

  function handleRegexpChange(checked: boolean) {
    console.log("Regexp toggle changed:", checked);
    settings.save({ ...settings.config, enable_regexp: checked });
  }

  function handleWildcardChange(checked: boolean) {
    console.log("Wildcard toggle changed:", checked);
    settings.save({ ...settings.config, enable_wildcard: checked });
  }

  function handleShowIPsChange(checked: boolean) {
    settings.save({ ...settings.config, show_interface_ips: checked });
  }

  function handleAutoCheckChange(checked: boolean) {
    settings.save({ ...settings.config, auto_check_updates: checked });
  }
</script>

<div class="settings-container">
  <div class="header">
    <div class="icon">
      <Settings size={28} />
    </div>
    <h2>{t("System Settings")}</h2>
  </div>

  <div class="settings-grid">
    <div class="setting-card grouped">
      <div class="setting-icon text-yellow-400">
        <Regex size={24} />
      </div>

      <div class="setting-group-content">
        <div class="setting-row">
          <div class="setting-title-row">
            <h3>{t("settings.enable_wildcard")}</h3>
            <div class="setting-control switch-control">
              <Switch
                checked={settings.config?.enable_wildcard ?? true}
                onCheckedChange={handleWildcardChange}
              />
            </div>
          </div>
          <p class="setting-description">{t("settings.enable_wildcard_desc")}</p>
        </div>

        <div class="setting-separator"></div>

        <div class="setting-row">
          <div class="setting-title-row">
            <h3>{t("Use Regexp")}</h3>
            <div class="setting-control switch-control">
              <Switch
                checked={settings.config?.enable_regexp ?? false}
                onCheckedChange={handleRegexpChange}
              />
            </div>
          </div>
          <p class="setting-description">{t("Use Regexp Description")}</p>
        </div>
      </div>
    </div>

    <div class="setting-card">
      <div class="setting-icon">
        <Network size={24} />
      </div>
      <div class="setting-group-content">
        <div class="setting-row">
          <div class="setting-title-row">
            <h3>{t("Show IPs")}</h3>
            <div class="setting-control switch-control">
              <Switch
                checked={settings.config?.show_interface_ips ?? true}
                onCheckedChange={handleShowIPsChange}
              />
            </div>
          </div>
          <p class="setting-description">{t("Show IPs Description")}</p>
        </div>
      </div>
    </div>

    <div class="setting-card grouped">
      <div class="setting-icon text-blue-400">
        <ArrowUpCircle size={24} />
      </div>

      <div class="setting-group-content">
        <div class="setting-row">
          <div class="setting-title-row">
            <div>
              <h3>{t("Version & Updates")}</h3>
              <div class="text-sm opacity-70 mt-1 version-display">
                <div class="version-item">
                  <span>{t("Current Version")}:</span>
                  <span class="font-bold">{import.meta.env.VITE_PKG_VERSION || "0.0.0"}</span>
                </div>
                {#if updater.newVersion}
                  <div class="version-item new-version text-green-400">
                    <span>{t("New version available:")}</span>
                    <span class="font-bold">{updater.newVersion}</span>
                  </div>
                {/if}
              </div>
            </div>
            <div class="setting-control">
              <Button
                small
                class={updater.newVersion ? "new-version-btn" : ""}
                onclick={() => (changelogOpen = true)}
              >
                {t("What's New")}
              </Button>
            </div>
          </div>
        </div>

        <ChangelogDialog bind:open={changelogOpen} />

        <div class="setting-row">
          <div class="setting-title-row">
            <h3>{t("Notify about new version")}</h3>
            <div class="setting-control switch-control">
              <Switch
                checked={settings.config?.auto_check_updates ?? false}
                onCheckedChange={handleAutoCheckChange}
              />
            </div>
          </div>
        </div>

        <div class="setting-separator"></div>

        <div class="setting-row">
          <div class="mobile-actions-row">
            <Button class="check-btn" onclick={() => updater.check()} disabled={updater.checking}>
              {updater.checking ? t("Checking...") : t("Check for Updates")}
            </Button>
            {#if updater.newVersion}
              <Button
                class="update-btn"
                onclick={() => updater.runUpdate()}
                disabled={updater.checking}
              >
                {t("Update Now")}
              </Button>
            {/if}
          </div>
        </div>
      </div>
    </div>

    <div class="setting-card grouped">
      <div class="setting-icon text-purple-400">
        <TerminalSquare size={24} />
      </div>

      <div class="setting-group-content">
        <div class="setting-row">
          <div class="setting-title-row stack-mobile">
            <h3>{t("Debug Console")}</h3>
            <div class="setting-control">
              <Button
                onclick={() => setTimeout(() => consoleStore.toggle(), 0)}
                class="console-btn"
              >
                {consoleStore.isOpen ? t("Close Console") : t("Open Console")}
              </Button>
            </div>
          </div>
          <p class="setting-description">{t("Debug Console Description")}</p>
        </div>

        <div class="setting-separator"></div>

        <div class="setting-row">
          <div class="setting-title-row stack-mobile">
            <h3>{t("Log Level")}</h3>
            <div class="setting-control">
              <div class="select-wrapper-dark">
                <Select
                  options={logLevels.map((l) => ({ value: l.value, label: t(l.label) }))}
                  selected={settings.config?.log_level || "info"}
                  onValueChange={handleLogLevelChange}
                />
              </div>
            </div>
          </div>
          <p class="setting-description">{t("Log Level Description")}</p>
        </div>
      </div>
    </div>

    <div class="setting-card danger-zone">
      <div class="setting-icon">
        <div class={isRestarting ? "spin" : ""}>
          <RefreshCw size={24} />
        </div>
      </div>
      <div class="setting-group-content">
        <div class="setting-row">
          <div class="setting-title-row stack-mobile">
            <h3 class="danger-text">{t("Restart Service")}</h3>
            <div class="setting-control">
              <Button class="restart-btn" onclick={handleRestart} disabled={isRestarting}>
                {isRestarting ? t("Restarting...") : t("Restart Service")}
              </Button>
            </div>
          </div>
          <p class="setting-description">{t("Restart Service Description")}</p>
        </div>
      </div>
    </div>
  </div>
</div>

<style>
  .settings-container {
    padding: 0.5rem;
    max-width: 900px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }

  .header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid var(--bg-light-extra);
    color: var(--text);
  }

  .header h2 {
    font-size: 1.5rem;
    font-weight: 600;
  }

  .header .icon {
    color: var(--accent);
    display: flex;
  }

  .settings-grid {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .setting-card {
    display: grid;
    grid-template-columns: auto 1fr auto;
    gap: 1rem;
    align-items: center;
    background-color: var(--bg-light);
    border: 1px solid var(--bg-light-extra);
    border-radius: 0.75rem;
    padding: 1.5rem;
    transition:
      transform 0.2s ease,
      box-shadow 0.2s ease;
  }

  .setting-card:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
    border-color: var(--accent-dim);
  }

  .setting-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 3.5rem;
    height: 3.5rem;
    background-color: var(--bg-dark);
    border-radius: 0.75rem;
    color: var(--text-2);
  }

  .setting-control {
    display: flex;
    align-items: center;
  }

  .version-item {
    display: flex;
    flex-direction: row;
    gap: 0.5ch;
  }

  .new-version {
    margin-top: 0.25rem;
  }

  @media (max-width: 600px) {
    .version-item {
      flex-direction: column;
      align-items: flex-start;
      margin-bottom: 0.25rem;
    }
  }

  .switch-control :global([data-switch-root]) {
    width: 52px !important;
    height: 30px !important;
  }
  .switch-control :global([data-switch-thumb]) {
    width: 24px !important;
    height: 24px !important;
  }
  .switch-control :global([data-switch-root][data-state="checked"] [data-switch-thumb]) {
    transform: translateX(22px) !important;
  }

  .select-wrapper-dark :global([data-select-trigger]) {
    background-color: var(--bg-dark) !important;
    border: 1px solid var(--bg-light-extra) !important;
    padding: 0.5rem 1rem !important;
    font-size: 1rem !important;
    min-width: 150px;
    justify-content: space-between;
  }
  .select-wrapper-dark :global([data-select-trigger]:hover) {
    border-color: var(--accent-dim) !important;
  }

  :global(button.restart-btn) {
    background-color: color-mix(in oklab, var(--red) 15%, var(--bg-dark)) !important;
    border: 1px solid color-mix(in oklab, var(--red) 30%, transparent) !important;
    color: var(--text) !important;
    font-weight: 500;
    padding: 0.6rem 1.2rem !important;
    font-size: 1rem;
  }
  :global(button.restart-btn:hover) {
    background-color: color-mix(in oklab, var(--red) 25%, var(--bg-dark)) !important;
    border-color: var(--red) !important;
  }

  :global(button.console-btn) {
    background-color: color-mix(in oklab, var(--accent) 15%, var(--bg-dark)) !important;
    border: 1px solid color-mix(in oklab, var(--accent) 30%, transparent) !important;
    color: var(--text) !important;
    font-weight: 500;
    padding: 0.6rem 1.2rem !important;
    font-size: 1rem;
    min-width: 150px;
  }
  :global(button.console-btn:hover) {
    background-color: color-mix(in oklab, var(--accent) 25%, var(--bg-dark)) !important;
    border-color: var(--accent) !important;
  }

  :global(button.check-btn) {
    background-color: color-mix(in oklab, var(--bg-light-extra) 15%, var(--bg-dark)) !important;
    border: 1px solid var(--bg-light-extra) !important;
    color: var(--text) !important;
    font-weight: 500;
    padding: 0.6rem 1.2rem !important;
    font-size: 1rem;
    min-width: 150px;
  }
  :global(button.check-btn:hover) {
    background-color: var(--bg-light-extra) !important;
    border-color: var(--accent) !important;
  }

  :global(.new-version-btn) {
    background-color: var(--orange) !important;
    color: white !important;
    border: 1px solid rgba(255, 255, 255, 0.1) !important;
  }

  :global(.new-version-btn:hover) {
    filter: brightness(1.1);
  }

  :global(button.update-btn) {
    background-color: color-mix(in oklab, var(--green) 80%, black) !important;
    border: 1px solid var(--green) !important;
    color: white !important;
    font-weight: 600;
    padding: 0.6rem 1.2rem !important;
    font-size: 1rem;
    min-width: 150px;
  }
  :global(button.update-btn:hover) {
    background-color: var(--green) !important;
    box-shadow: 0 0 10px rgba(74, 222, 128, 0.4);
  }

  .text-blue-400 {
    color: #60a5fa;
  }
  .text-green-400 {
    color: #4ade80;
  }
  .text-yellow-400 {
    color: #facc15;
  }
  .font-bold {
    font-weight: 700;
  }

  .danger-zone {
    border-color: color-mix(in oklab, var(--red) 30%, transparent);
    background-color: color-mix(in oklab, var(--red) 5%, var(--bg-light));
  }
  .danger-zone:hover {
    border-color: var(--red);
  }
  .danger-zone .setting-icon {
    color: var(--red);
    background-color: color-mix(in oklab, var(--red) 10%, var(--bg-dark));
  }
  .danger-zone .setting-icon > div {
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .danger-text {
    color: var(--red) !important;
  }

  .spin {
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

  .setting-card.grouped {
    display: flex;
    gap: 1.5rem;
    align-items: flex-start;
  }

  .setting-group-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }

  .setting-row {
    display: flex;
    flex-direction: column;
    width: 100%;
    gap: 0.25rem;
  }

  .setting-title-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
    gap: 1rem;
  }

  .setting-title-row .setting-control {
    margin-left: auto;
  }

  .setting-separator {
    height: 1px;
    background-color: var(--bg-light-extra);
    width: 100%;
  }

  .setting-description {
    font-size: 0.95rem;
    color: var(--text-2);
    line-height: 1.5;
    white-space: pre-line;
    margin: 0;
  }

  .mobile-actions-row {
    display: flex;
    gap: 0.5rem;
    width: 100%;
  }

  @media (max-width: 600px) {
    .setting-card {
      display: flex;
      flex-direction: column;
      gap: 0.5rem;
      align-items: stretch;
      padding: 1rem;
    }

    .setting-card.grouped {
      flex-direction: column;
      gap: 1rem;
      align-items: stretch !important;
    }
    .setting-group-content {
      gap: 1.25rem !important;
    }

    .setting-separator {
      display: none !important;
    }

    .setting-icon {
      width: 1.75rem;
      height: 1.75rem;
      flex-shrink: 0;
      border-radius: 0.4rem;
    }

    .setting-icon :global(svg) {
      width: 1.25rem;
      height: 1.25rem;
    }

    .setting-card h3 {
      margin: 0 !important;
      line-height: 1.2 !important;
      font-size: 1.1rem !important;
    }

    .setting-title-row {
      width: 100% !important;
    }
    .setting-title-row:not(.stack-mobile) {
      justify-content: space-between !important;
      align-items: center !important;
    }

    .setting-title-row.stack-mobile {
      flex-direction: column;
      align-items: flex-start;
      gap: 0.5rem;
    }
    .setting-title-row.stack-mobile .setting-control {
      width: 100%;
      margin-left: 0;
      display: block;
    }

    .setting-title-row.stack-mobile :global(button),
    .mobile-actions-row :global(button) {
      width: 100% !important;
      max-width: none !important;
      min-width: 0 !important;
      display: flex !important;
      justify-content: center !important;
    }

    .setting-title-row.stack-mobile .select-wrapper-dark {
      display: block;
      width: 100% !important;
    }
    .setting-title-row.stack-mobile :global([data-select-trigger]) {
      display: flex !important;
      justify-content: space-between !important;
      width: 100% !important;
      max-width: none !important;
    }
    .setting-title-row.stack-mobile .select-wrapper-dark :global(.select-wrap) {
      width: 100% !important;
      max-width: none !important;
      display: block !important;
    }
    .setting-title-row.stack-mobile .select-wrapper-dark :global(.selected) {
      width: 100% !important;
      justify-content: space-between !important;
      display: flex !important;
    }

    .mobile-actions-row {
      flex-direction: column;
      align-items: stretch !important;
      gap: 0.5rem;
    }
  }
</style>
