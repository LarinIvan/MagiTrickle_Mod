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
  import BottomNav from "./BottomNav.svelte";
  import { onMount } from "svelte";

  let active_tab = $state("groups");

  onMount(() => {
    aliases.load();
    settings.load();
  });
</script>

<Toast />
<Overlay />
<ScrollToTop />
<ConsoleWindow />
{#if [11, 0, 1].includes(new Date().getMonth())}
  <SnowField />
{/if}

<main>
  <Tabs.Root bind:value={active_tab}>
    <nav class="top-nav">
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
      <div style:display={active_tab === "groups" ? "block" : "none"}>
        <GroupsView />
      </div>
      <div style:display={active_tab === "interfaces" ? "block" : "none"}>
        <InterfacesView visible={active_tab === "interfaces"} />
      </div>
      <div style:display={active_tab === "settings" ? "block" : "none"}>
        <SettingsView />
      </div>
    </article>
  </Tabs.Root>

  <BottomNav bind:active={active_tab} />
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
    align-items: center;
    margin-bottom: 2rem;
    padding: 0.3rem;
  }

  .top-nav {
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    max-width: 1000px;
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
    main {
      padding-bottom: 80px;
      margin-bottom: 0;
    }

    .top-nav {
      display: none;
    }
  }

  @media (max-width: 700px) {
    :global([data-tabs-list]) {
      display: none;
    }

    .top-nav {
      display: flex;
      justify-content: flex-end;
      padding: 0.5rem 0;
    }

    .header-controls {
      gap: 1rem;
    }
  }
</style>
