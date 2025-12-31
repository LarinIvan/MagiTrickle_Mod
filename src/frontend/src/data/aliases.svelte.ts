import { fetcher } from "../utils/fetcher";

let _aliases = $state<Record<string, string>>({});

export const aliases = {
    get all() { return _aliases },
    async load() {
        try {
            const res = await fetcher.get<Record<string, string>>("/system/interfaces/aliases");
            if (res) { // Fetcher returns data directly? Or Response? 
                       // check interfaces.svelte.ts: await fetcher.get(...).then(...)
                       // interfaces.svelte.ts uses .then(data => ...).
                       // So fetcher.get returns Promise<T>.
                _aliases = res;
            }
        } catch (e) {
            console.error("Failed to load aliases", e);
        }
    },
    async save(newAliases: Record<string, string>) {
        try {
            // Post might return raw response or data.
            // handlers.go SaveInterfaceAliases writes nil.
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
