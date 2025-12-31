import { fetcher } from "../utils/fetcher";
import { toast } from "../utils/events";
import { t } from "./locale.svelte";
import type { Group } from "../types";

class GroupsStore {
    all = $state<Group[]>([]);
    loading = $state(false);

    // Derived stats for InterfacesView
    stats = $derived.by(() => {
        const global = {
            interfaces: new Set(this.all.map((g) => g.interface || "")).size,
            groups: this.all.length,
            activeGroups: this.all.filter((g) => g.enable).length,
            rules: this.all.reduce((acc, g) => acc + (g.rules?.length || 0), 0),
            activeRules: this.all.reduce((acc, g) => {
                if (!g.enable) return acc;
                return acc + (g.rules?.filter((r) => r.enable).length || 0);
            }, 0),
            usedInterfaces: new Set(this.all.filter(g => g.interface).map(g => g.interface)).size,
        };

        const perInterface: Record<
            string,
            { groups: number; activeGroups: number; rules: number; activeRules: number }
        > = {};

        this.all.forEach((g) => {
            if (!g.interface) return;
            if (!perInterface[g.interface]) {
                perInterface[g.interface] = { groups: 0, activeGroups: 0, rules: 0, activeRules: 0 };
            }
            perInterface[g.interface].groups++;
            if (g.enable) {
                perInterface[g.interface].activeGroups++;
            }

            const ruleCount = g.rules?.length || 0;
            perInterface[g.interface].rules += ruleCount;

            if (g.enable) {
                const activeCount = g.rules?.filter(r => r.enable).length || 0;
                perInterface[g.interface].activeRules += activeCount;
            }
        });

        return { global, perInterface };
    });

    async load() {
        this.loading = true;
        try {
            const res = await fetcher.get<{ groups: Group[] }>("/groups?with_rules=true");
            if (res && res.groups) {
                this.all = res.groups;
            } else {
                this.all = [];
            }
        } catch (err) {
            console.error("Failed to load groups", err);
            toast.error(t("Failed to load groups"));
        } finally {
            this.loading = false;
        }
    }

    async save(newGroups: Group[], silent = false) {
        try {
            await fetcher.put("/groups?save=true", { groups: newGroups });
            this.all = newGroups;
            if (!silent) toast.success(t("Saved"));
        } catch (err) {
            console.error(err);
            toast.error(t("Failed to save groups"));
            throw err;
        }
    }
}

export const groupsStore = new GroupsStore();
