export interface LogEntry {
    id: string; // unique id for keying
    time: string;
    level: string;
    message: string;
    kvPairs: { key: string; value: string }[];
    raw: string; // fallback
    isError: boolean;
}

class ConsoleStore {
    isOpen = $state(false);
    isMinimized = $state(false);
    logs = $state<LogEntry[]>([]);
    maxLogs = 1000;
    eventSource: EventSource | null = null;
    unreadCount = $state(0);

    // Position and Size
    x = $state(50);
    y = $state(50);
    width = $state(600);
    height = $state(400);

    toggle() {
        this.isOpen = !this.isOpen;
        if (this.isOpen) {
            this.isMinimized = false;
            this.connect();
        } else {
            this.disconnect();
        }
    }

    toggleMinimize() {
        this.isMinimized = !this.isMinimized;
        if (!this.isMinimized) {
            this.unreadCount = 0;
        }
    }

    connect() {
        if (this.eventSource) return;
        if (typeof window === 'undefined') return; // Guard against SSR if applicable

        this.addLog("--- Connecting to log stream ---");

        this.eventSource = new EventSource('/api/v1/system/logs/stream');

        this.eventSource.onopen = () => {
            this.addLog("--- Connected ---");
        };

        this.eventSource.onmessage = (event) => {
            this.addLog(event.data);
        };

        this.eventSource.onerror = (err) => {
            // console.error("SSE Error", err);
            this.eventSource?.close();
            this.eventSource = null;
            this.addLog("--- Connection lost. Reconnecting... ---");
            setTimeout(() => this.connect(), 2000);
        };
    }

    disconnect() {
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
            this.addLog("--- Disconnected ---");
        }
    }

    addLog(msg: string) {
        if (!msg) return;

        const entry: LogEntry = {
            id: Math.random().toString(36),
            time: '',
            level: 'INFO',
            message: msg,
            kvPairs: [],
            raw: msg,
            isError: false
        };

        try {
            if (msg.startsWith('{') && msg.includes('level')) {
                const parsed = JSON.parse(msg);

                entry.time = parsed.time ? new Date(parsed.time).toLocaleTimeString([], { hour12: false }) : new Date().toLocaleTimeString([], { hour12: false });
                entry.level = (parsed.level || 'INFO').toUpperCase().padEnd(3).slice(0, 3);

                const lvlMap: Record<string, string> = {
                    'info': 'INF',
                    'warn': 'WRN',
                    'error': 'ERR',
                    'debug': 'DBG',
                    'fatal': 'FTL',
                    'trace': 'TRC'
                };
                if (parsed.level && lvlMap[parsed.level]) {
                    entry.level = lvlMap[parsed.level];
                }

                entry.message = parsed.message || parsed.msg || '';

                if (entry.level === 'ERR' || entry.level === 'FTL') {
                    entry.isError = true;
                }

                for (const [key, value] of Object.entries(parsed)) {
                    if (['time', 'level', 'message', 'msg', 'error', 'err'].includes(key)) continue;

                    let valStr = '';
                    if (typeof value === 'object') {
                        valStr = JSON.stringify(value);
                    } else {
                        valStr = String(value);
                    }
                    entry.kvPairs.push({ key, value: valStr });
                }

                if (parsed.error || parsed.err) {
                    entry.kvPairs.push({ key: 'error', value: parsed.error || parsed.err });
                    entry.isError = true;
                }
            } else {
                entry.time = new Date().toLocaleTimeString([], { hour12: false });
            }
        } catch (e) {
            entry.time = new Date().toLocaleTimeString([], { hour12: false });
        }

        if (!entry.time) entry.time = new Date().toLocaleTimeString([], { hour12: false });
        if (!entry.level) entry.level = "INFO";
        if (!entry.message) entry.message = "";

        this.logs.push(entry);
        if (this.logs.length > this.maxLogs) {
            this.logs.shift();
        }

        if (this.isMinimized) {
            this.unreadCount++;
        }
    }

    clear() {
        this.logs = [];
    }
}

export const consoleStore = new ConsoleStore();
