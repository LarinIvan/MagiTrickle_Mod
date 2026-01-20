import { fetcher } from "../utils/fetcher";

let _aliases = $state<Record<string, string>>({});

export const aliases = {
    get all() { return _aliases },
    async load() {
        try {
            const res = await fetcher.get<Record<string, string>>("/system/interfaces/aliases");
            if (res) {
                _aliases = res;
            }
        } catch (e) {
            console.error("Failed to load aliases", e);
        }
    },
    async save(newAliases: Record<string, string>) {
        try {
            await fetcher.post("/system/interfaces/aliases?save=true", newAliases);
            _aliases = newAliases;
            return true;
        } catch (e) {
            console.error("Failed to save aliases", e);
        }
        return false;
    }
};

export function getInterfaceLabel(id: string) {
    if (!id) return "";
    const alias = _aliases[id];
    if (alias) {
        return `${alias} [${id}]`;
    }
    return id;
}
