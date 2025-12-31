<script lang="ts">
  import "./app.css";
  import AppLayout from "./components/layout/AppLayout.svelte";
  import { settings } from "./data/settings.svelte";
  import { updater } from "./data/updater.svelte";

  let ready = $state(false);

  $effect(() => {
    settings.load().then(() => {
      ready = true;
      if (settings.config.auto_check_updates) {
        updater.check(true); // silent check
      }
    });
  });
</script>

<AppLayout />
