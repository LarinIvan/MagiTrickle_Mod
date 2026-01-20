import { fetcher } from "../utils/fetcher";
import { toast } from "../utils/events";

class SettingsStore {
    /** @type {import("../types").SettingsConfig} */
    config = $state({
        enable_regexp: false,
        enable_wildcard: true,
        log_level: "info",
        show_interface_ips: true,
        auto_check_updates: false
    });

    async load() {
        try {
            const res = await fetcher.get<import("../types").SettingsConfig>("/system/settings");
            if (res) {
                this.config = res;
            }
        } catch (e) {
            console.error("Failed to load settings:", e);
        }
    }

    async save(newSettings: Partial<import("../types").SettingsConfig>) {
        try {
            this.config = { ...this.config, ...newSettings };
            await fetcher.post("/system/settings?save=true", this.config);
            return true;
        } catch (e) {
            console.error("Failed to save settings:", e);
            toast.error("Failed to save settings");
            return false;
        }
    }
}

export const settings = new SettingsStore();
