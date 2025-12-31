<script lang="ts">
  import { Tabs } from "bits-ui";
  import { t } from "../../data/locale.svelte";
  import GroupsView from "../../modules/groups/GroupsView.svelte";
  import InterfacesView from "../../modules/interfaces/InterfacesView.svelte";
  import SettingsView from "../../modules/settings/SettingsView.svelte";
  import { aliases } from "../../data/aliases.svelte";
  // import LogsPanel from "../../modules/logs/LogsPanel.svelte";
  import Overlay from "../feedback/Overlay.svelte";
  import SnowField from "../feedback/SnowField.svelte";
  import Toast from "../feedback/Toast.svelte";
  import ScrollToTop from "../feedback/ScrollToTop.svelte";
  import ConsoleWindow from "../console/ConsoleWindow.svelte";
  import HeaderSettings from "./HeaderSettings.svelte";
  import { settings } from "../../data/settings.svelte";
  import { onMount } from "svelte";

  let active_tab = $state("groups");

  onMount(() => {
    aliases.load();
    settings.load();
  });
</script>

<!-- TODO: add locales -->
<!-- TODO: add white/dark themes -->

<Toast />
<Overlay />
<ScrollToTop />
<ConsoleWindow />
<SnowField variant="front" />

<main>
  <Tabs.Root bind:value={active_tab}>
    <nav>
      <Tabs.List>
        <Tabs.Trigger value="groups">{t("Groups")}</Tabs.Trigger>
        <Tabs.Trigger value="interfaces">{t("Interfaces")}</Tabs.Trigger>
        <Tabs.Trigger value="settings">{t("Settings")}</Tabs.Trigger>
      </Tabs.List>
      <div class="header-controls">
        <HeaderSettings />
      </div>
    </nav>
    <article>
      <!-- Keep-alive implementation using display style -->
      <div style:display={active_tab === "groups" ? "block" : "none"}>
        <GroupsView />
      </div>
      <div style:display={active_tab === "interfaces" ? "block" : "none"}>
        <InterfacesView visible={active_tab === "interfaces"} />
      </div>
      <div style:display={active_tab === "settings" ? "block" : "none"}>
        <SettingsView />
      </div>

      <!-- <Tabs.Content value="settings">
        <SettingsPanel />
      </Tabs.Content> -->
      <!-- <Tabs.Content value="logs">
        <LogsPanel />
      </Tabs.Content> -->
    </article>
  </Tabs.Root>
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
    align-items: center;
    margin-bottom: 2rem;
    padding: 0.3rem;
    position: relative;
    z-index: 1;
  }

  nav {
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
  }

  .header-controls {
    display: flex;
    align-items: center;
    gap: 1.5rem;
  }

  :global {
    [data-tabs-root] {
      max-width: 1000px;
      width: 100%;
    }

    [data-tabs-list] {
      display: flex;
      flex-direction: row;
      gap: 1rem;
    }

    [data-tabs-trigger] {
      & {
        padding: 0.5rem 1rem;
        border: none;
        border-bottom: 2px solid transparent;
        font-size: 1.5rem;
        font-family: var(--font);
        background-color: transparent;
        color: var(--text);
      }

      &[data-state="active"] {
        color: var(--accent);
        border-color: var(--accent);
      }

      &[data-state="inactive"]:hover {
        border-color: var(--text);
      }
    }

    [data-tabs-content] {
      padding-top: 1rem;
    }
  }

  @media (max-width: 700px) {
    :global {
      [data-tabs-root] {
        width: 100%;
      }
      [data-tabs-trigger] {
        font-size: 1.2rem;
        padding: 0.5rem 0.5rem;
      }
      [data-tabs-list] {
        gap: 0.5rem;
      }
    }
  }
</style>
