<script lang="ts">
  import { t } from "../../data/locale.svelte";
  import { Layers, Network, Settings } from "../ui/icons";

  let { active = $bindable("groups") } = $props();

  function setActive(tab: string) {
    active = tab;
  }
</script>

<nav class="bottom-nav">
  <button
    class:active={active === "groups"}
    onclick={() => setActive("groups")}
    aria-label={t("Groups")}
  >
    <div class="icon">
      <Layers size={24} />
    </div>
    <span class="label">{t("Groups")}</span>
  </button>

  <button
    class:active={active === "interfaces"}
    onclick={() => setActive("interfaces")}
    aria-label={t("Interfaces")}
  >
    <div class="icon">
      <Network size={24} />
    </div>
    <span class="label">{t("Interfaces")}</span>
  </button>

  <button
    class:active={active === "settings"}
    onclick={() => setActive("settings")}
    aria-label={t("Settings")}
  >
    <div class="icon">
      <Settings size={24} />
    </div>
    <span class="label">{t("Settings")}</span>
  </button>
</nav>

<style>
  .bottom-nav {
    display: none; /* Hidden by default (desktop) */
    position: fixed;
    bottom: 0;
    left: 0;
    width: 100%;
    height: 60px;
    background-color: var(--bg-main);
    border-top: 1px solid var(--border);
    justify-content: space-around;
    align-items: center;
    z-index: 1000;
    backdrop-filter: blur(10px);
    background-color: rgba(var(--bg-main-rgb), 0.95);
    padding-bottom: env(safe-area-inset-bottom);
  }

  @media (max-width: 700px) {
    .bottom-nav {
      display: flex; /* Show on mobile */
    }
  }

  button {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    padding: 0.5rem 0;
    gap: 0.25rem;
    color: var(--text-2);
    cursor: pointer;
    transition:
      color 0.1s ease,
      transform 0.1s;
    height: 100%;
    -webkit-tap-highlight-color: transparent;
  }

  button:active {
    transform: scale(0.95);
  }

  .icon {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .label {
    font-size: 0.7rem;
    font-weight: 500;
  }

  button.active {
    color: var(--accent);
  }
</style>
