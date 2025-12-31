import { fetcher } from "../utils/fetcher";
import { toast } from "../utils/events";
import { t } from "./locale.svelte";

class UpdaterStore {
    newVersion = $state("");
    checking = $state(false);

    async check(silent = false) {
        if (this.checking) return;
        this.checking = true;
        try {
            // Using window.location.origin not strictly needed if fetcher handles it, but good for safety
            const res = await fetcher.get("/system/update/check");
            if (res && res.available_version) {
                this.newVersion = res.available_version;
                // Notify even if silent (auto-check), so user knows on startup
                toast.success(`${t("New version available:")} ${this.newVersion}`);
            } else {
                this.newVersion = "";
                if (!silent) {
                    toast.info(t("You have the latest version"));
                }
            }
        } catch (e) {
            console.error("Update check failed", e);
            if (!silent) {
                toast.error(t("Failed to check for updates"));
            }
        } finally {
            this.checking = false;
        }
    }

    async runUpdate() {
        try {
            await fetcher.post("/system/update/run", {});
            toast.success(t("Update started. The service will restart shortly..."));
            // Optionally reload page after some time
            setTimeout(() => {
                window.location.reload();
            }, 10000);
        } catch (e) {
            toast.error(t("Failed to start update"));
        }
    }
}

export const updater = new UpdaterStore();
