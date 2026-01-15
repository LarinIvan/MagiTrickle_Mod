<script lang="ts">
  import { Tabs } from "bits-ui";
  import { t } from "../../data/locale.svelte";
  import GroupsView from "../../modules/groups/GroupsView.svelte";
  import InterfacesView from "../../modules/interfaces/InterfacesView.svelte";
  import SettingsView from "../../modules/settings/SettingsView.svelte";
  import { aliases } from "../../data/aliases.svelte";
  // import LogsPanel from "../../modules/logs/LogsPanel.svelte";
  import Overlay from "../feedback/Overlay.svelte";
  // import SnowField from "../feedback/SnowField.svelte";
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

<!-- TODO: add locales -->
<!-- TODO: add white/dark themes -->

<Toast />
<Overlay />
<ScrollToTop />
<ConsoleWindow />
<!-- <SnowField variant="front" /> -->

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
    position: relative;
    z-index: 1;
  }

  .top-nav {
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    width: 100%; /* Ensure it spans width */
    max-width: 1000px; /* Match Tabs.Root width constraint */
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
      padding-bottom: 80px; /* Space for BottomNav */
      margin-bottom: 0;
    }

    .top-nav {
      display: none; /* Hide top nav tabs */
    }

    /* Keep header controls visible? Logic says we only want Tabs List hidden. 
       But header-controls usually contains settings, log toggles etc.
       If we hide top-nav, we hide header-controls too. 
       We probably want header-controls to be accessible or moved?
       For now, per instructions: "Вкладки Групп/Интерфейсы/Настройки переместим в меню снизу".
       However, HeaderSettings includes Global Toggle, locale, etc.
       If we hide them, user loses functionality. 
       Let's keep them visible but perhaps styled differently, or just hide the Tabs List.
    */
  }

  @media (max-width: 700px) {
    /* Refined mobile styles */
    :global([data-tabs-list]) {
      display: none; /* Hide Tabs List specifically */
    }

    .top-nav {
      display: flex;
      justify-content: flex-end; /* Only controls remain */
      padding: 0.5rem 0;
    }

    .header-controls {
      gap: 1rem;
    }
  }
</style>
