<script lang="ts">
  import { t } from "../../data/locale.svelte";
  import { X } from "../../components/ui/icons";
  import { fade, scale, fly } from "svelte/transition";
  import { tweened } from "svelte/motion";
  import { cubicOut, linear } from "svelte/easing";
  import { API_BASE, fetcher } from "../../utils/fetcher";
  import { untrack } from "svelte";
  import { toast } from "../../utils/events";
  import { INTERFACES } from "../../data/interfaces.svelte";
  import { aliases } from "../../data/aliases.svelte";

  type Props = {
    interfaceId: string;
    interfaceName: string;
    interfaceIP?: string;
    onClose: () => void;
  };

  let { interfaceId, interfaceName, interfaceIP, onClose }: Props = $props();

  let currentInterfaceId = $state(interfaceId);
  let currentInterfaceName = $state(interfaceName);
  let currentInterfaceIP = $state(interfaceIP);

  let phase = $state<"init" | "ping" | "download" | "upload" | "loss" | "done">("init");
  let error = $state<string | null>(null);

  // Interface Selection
  let isInterfaceSelectOpen = $state(false);
  let interfaceList = $derived.by(() => {
    const systemIds = new Set(INTERFACES.map((i) => i.id));
    const aliasIds = Object.keys(aliases.all);
    const known = INTERFACES.map((item) => ({
      id: item.id,
      name: aliases.all[item.id] || item.id,
      active: item.active,
    }));
    const extra = aliasIds
      .filter((id) => !systemIds.has(id))
      .sort()
      .map((id) => ({
        id,
        name: aliases.all[id] || id,
        active: true, // assume active if alias exists
      }));
    return [...known, ...extra];
  });

  function selectInterface(iface: { id: string; name: string }) {
    if (iface.id === currentInterfaceId) {
      isInterfaceSelectOpen = false;
      return;
    }

    currentInterfaceId = iface.id;
    currentInterfaceName = iface.name;
    currentInterfaceIP = undefined; // clear pending fetch
    manualIP = null;
    isInterfaceSelectOpen = false;

    // Reset test
    closeEventSource();
    phase = "init";

    // Reset animations instantly
    // Reset animations instantly
    currentSpeed.set(0, { duration: 0 });
    tickProgressDegree.set(-10, { duration: 0 });

    pingMs = null;
    jitterMs = null;
    packetLoss = null;
    downloadMbps = null;
    uploadMbps = null;
    serverInfo = null;
    selectedServerId = null;
    error = null;
    clientInfo = null;

    fetchExternalIP();
  }

  let currentSpeed = tweened(0, {
    duration: 800,
    easing: cubicOut,
  });

  // Ticks animation: -10 -> 280 degrees (reveal), 280 -> -10 (hide)
  // Animate by DEGREE for linear spatial effect.
  // 0 deg = start, 270 deg = end.
  let tickProgressDegree = tweened(-10, {
    duration: 1500,
    easing: linear,
  });

  $effect(() => {
    if (phase === "download") {
      // Reveal linearly start at DOWNLOAD phase.
      tickProgressDegree.set(280, { duration: 1500 });
    } else if (phase === "upload" && uploadMbps !== null) {
      // Hide at end of upload (when result is known, during the pause)
      tickProgressDegree.set(-10, { duration: 1500 });
    } else if (phase === "done" || phase === "loss") {
      // Ensure hidden if missed
      tickProgressDegree.set(-10, { duration: 1500 });
    } else if (phase === "init") {
      // Reset instantly
      tickProgressDegree.set(-10, { duration: 0 });
    }
  });

  let pingMs = $state<number | null>(null);
  let jitterMs = $state<number | null>(null);
  let packetLoss = $state<number | null>(null);
  let downloadMbps = $state<number | null>(null);
  let uploadMbps = $state<number | null>(null);
  let serverInfo = $state<{ id?: string; name: string; country: string; sponsor?: string } | null>(
    null,
  );
  let clientInfo = $state<{ ip: string; isp: string } | null>(null);

  let manualIP = $state<string | null>(null);
  let isTestDone = false;
  let isPaused = false;

  let displayIP = $derived(clientInfo?.ip || manualIP || currentInterfaceIP || "...");
  let innerWidth = $state(typeof window !== "undefined" ? window.innerWidth : 1000);
  let isMobile = $derived(innerWidth <= 700);

  let eventSource: EventSource | null = null;
  let statusText = $state("");

  let parallelLossEnabled = $state(true);

  // Server Selection
  let servers = $state<any[]>([]);
  let selectedServerId = $state<string | null>(null);
  let isServerSelectOpen = $state(false);
  let loadingServers = $state(false);

  let serverSearch = $state("");
  let searchTimeout: any;

  async function fetchServers(query?: string) {
    if (!query && servers.length > 0) return;
    loadingServers = true;
    loadingServers = true;
    try {
      let url = "/diagnostics/speedtest/servers?interface=" + (currentInterfaceId || "");
      if (query) url += "&search=" + encodeURIComponent(query);
      const res = await fetcher.get<any[]>(url);
      servers = res;
    } catch (e) {
      console.error(e);
      toast.error("Failed to fetch servers");
    } finally {
      loadingServers = false;
    }
  }

  function handleSearch(e: Event) {
    const val = (e.target as HTMLInputElement).value;
    serverSearch = val;
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      fetchServers(val);
    }, 500);
  }

  function selectServer(server: any) {
    selectedServerId = server.id || server.ID;
    serverInfo = {
      name: server.name || server.Name,
      country: server.country || server.Country,
      sponsor: server.sponsor || server.Sponsor,
      id: server.id || server.ID,
    };
    isServerSelectOpen = false;
  }

  // Gauge Logic
  // Ticks: 0, 5, 10, 50, 100, 250, 500, 750, 1000
  // Total Arc: 270 degrees.
  // We want UNIFORM visual spacing between these specific 9 ticks.
  // Step = 270 / (9 - 1) = 33.75 degrees.
  const scaleTable = [
    { degree: 0, value: 0 },
    { degree: 33.75, value: 5 },
    { degree: 67.5, value: 10 },
    { degree: 101.25, value: 50 },
    { degree: 135, value: 100 }, // Top Center (Aligned)
    { degree: 168.75, value: 250 },
    { degree: 202.5, value: 500 },
    { degree: 236.25, value: 750 },
    { degree: 270, value: 1000 },
  ];

  /* 
    Helper to calculate tick position
    Center: 200, 200
    Radius: 105 (Inside the 140 arc with 30 width)
    Angle offset: +135 degrees to map 0 to Start
  */
  function getTickPos(deg: number) {
    const r = 105;
    const angleRad = (deg + 135) * (Math.PI / 180);
    const x = 200 + r * Math.cos(angleRad);
    const y = 200 + r * Math.sin(angleRad);
    return { x, y };
  }

  function getNonlinearDegree(c: number) {
    if (c <= 0) return 0;
    if (c >= 1000) return 270;

    for (let i = 1; i < scaleTable.length; i++) {
      if (c < scaleTable[i].value) {
        const prev = scaleTable[i - 1];
        const curr = scaleTable[i];
        const ratio = (c - prev.value) / (curr.value - prev.value);
        return prev.degree + ratio * (curr.degree - prev.degree);
      }
    }
    return 270;
  }

  $effect(() => {
    untrack(() => {
      fetchExternalIP();
    });
    // Lock body scroll
    if (typeof document !== "undefined") {
      document.body.style.overflow = "hidden";
    }
    return () => {
      closeEventSource();
      // Unlock body scroll
      if (typeof document !== "undefined") {
        document.body.style.overflow = "";
      }
    };
  });

  async function fetchExternalIP() {
    if (currentInterfaceIP && !isPrivateIP(currentInterfaceIP)) return;
    try {
      const res = await fetcher.get<{ ip: string }>(
        `/system/interfaces/${currentInterfaceId}/external-ip`,
      );
      if (res.ip) manualIP = res.ip;
    } catch (e) {
      console.error(e);
    }
  }

  function isPrivateIP(ip: string) {
    return (
      ip.startsWith("192.168.") ||
      ip.startsWith("10.") ||
      ip.startsWith("172.16.") ||
      ip === "127.0.0.1"
    );
  }

  function closeEventSource() {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  }

  function handleEventData(data: string, callback: (parsed: any) => void) {
    try {
      if (!data || data === "undefined") throw new Error("Invalid data");
      const parsed = JSON.parse(data);
      callback(parsed);
    } catch (e) {
      retryTest();
    }
  }

  function retryTest() {
    // If we already have upload results, consider it done instead of retrying
    if (uploadMbps !== null) {
      phase = "done";
      isTestDone = true;
      currentSpeed.set(0);
      closeEventSource();
      return;
    }

    closeEventSource();
    toast.error(t("Speedtest failed, retrying..."));
    setTimeout(() => {
      if (onClose) startTest();
    }, 1500);
  }

  function startTest() {
    console.log("Starting speedtest...");
    error = null;
    phase = "ping";
    isTestDone = false;
    isPaused = false;
    currentSpeed.set(0);
    pingMs = null;
    jitterMs = null;
    packetLoss = null;
    downloadMbps = null;
    uploadMbps = null;

    closeEventSource();

    closeEventSource();

    let url = `${API_BASE}/diagnostics/speedtest?interface=${currentInterfaceId}`;
    if (selectedServerId) url += `&server_id=${selectedServerId}`;
    if (parallelLossEnabled) url += `&parallel_loss=true`;
    eventSource = new EventSource(url);

    eventSource.addEventListener("status", (e) => handleEventData(e.data, (d) => (statusText = d)));
    eventSource.addEventListener("error", (e) => {
      try {
        if (!e.data || e.data === "undefined") {
          retryTest();
          return;
        }
        error = JSON.parse(e.data);
        closeEventSource();
        phase = "done";
      } catch (err) {
        retryTest();
      }
    });
    eventSource.addEventListener("server_info", (e) =>
      handleEventData(e.data, (d) => {
        // If we already have info for this server (e.g. from manual selection),
        // and the new info is missing country/sponsor (e.g. from FetchServerByID quirk),
        // preserve the existing info.
        if (serverInfo && serverInfo.id === d.id) {
          serverInfo = {
            ...d,
            country: d.country || serverInfo.country,
            sponsor: d.sponsor || serverInfo.sponsor,
            name: d.name || serverInfo.name,
          };
        } else {
          serverInfo = d;
        }

        if (d.id && !selectedServerId) selectedServerId = d.id;
      }),
    );
    eventSource.addEventListener("client_info", (e) =>
      handleEventData(e.data, (d) => (clientInfo = d)),
    );
    eventSource.addEventListener("stage", (e) => {
      handleEventData(e.data, (d) => {
        phase = d;
        isPaused = false;
        if (phase === "download" || phase === "upload" || phase === "loss") currentSpeed.set(0);
      });
    });
    eventSource.addEventListener("speed", (e) => {
      if (isTestDone || isPaused) return;
      handleEventData(e.data, (d) => currentSpeed.set(d.mbps));
    });
    eventSource.addEventListener("result_ping", (e) =>
      handleEventData(e.data, (d) => (pingMs = d)),
    );
    eventSource.addEventListener("result_jitter", (e) =>
      handleEventData(e.data, (d) => (jitterMs = d)),
    );
    eventSource.addEventListener("result_packet_loss", (e) =>
      handleEventData(e.data, (d) => (packetLoss = d)),
    );
    eventSource.addEventListener("result_download", (e) =>
      handleEventData(e.data, (d) => {
        downloadMbps = d;
        isPaused = true;
        currentSpeed.set(0);
      }),
    );
    eventSource.addEventListener("result_upload", (e) =>
      handleEventData(e.data, (d) => {
        uploadMbps = d;
        isPaused = true;
        currentSpeed.set(0);
      }),
    );
    eventSource.addEventListener("done", (e) => {
      currentSpeed.set(0);
      setTimeout(() => {
        phase = "done";
        isTestDone = true;
        closeEventSource();
      }, 1000);
    });
    eventSource.onerror = (e) => {
      if (eventSource?.readyState === EventSource.CLOSED) return;
      retryTest();
    };
  }

  function formatVal(val: number | null, fixed = 1) {
    if (val === null) return "—";
    return val.toFixed(fixed);
  }
</script>

<svelte:window onresize={() => (innerWidth = window.innerWidth)} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-overlay" onclick={onClose} transition:fade={{ duration: 200 }}>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-content" onclick={(e) => e.stopPropagation()} transition:scale={{ start: 0.9 }}>
    <button class="close-btn" onclick={onClose}>
      <X size={24} />
    </button>

    <div class="st-container">
      <div class="st-stats" class:result-mode={phase === "done"}>
        <div class="st-stat-item">
          <div class="st-stat-label">
            <svg class="st-icon" viewBox="0 0 24 24" style="fill: #fff38e;">
              <path
                d="M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2M12,4A8,8 0 0,1 20,12A8,8 0 0,1 12,20A8,8 0 0,1 4,12A8,8 0 0,1 12,4M12,6A6,6 0 0,0 6,12A6,6 0 0,0 12,18A6,6 0 0,0 18,12A6,6 0 0,0 12,6M12,8A4,4 0 0,1 16,12A4,4 0 0,1 12,16A4,4 0 0,1 8,12A4,4 0 0,1 12,8Z"
              />
            </svg>
            <span>PING</span> <span class="unit">ms</span>
          </div>
          <div class="st-stat-value">
            {#if pingMs !== null}
              <div in:fly={{ y: 10, duration: 400 }}>{formatVal(pingMs, 0)}</div>
            {:else}
              —
            {/if}
          </div>
        </div>

        <div class="st-stat-item">
          <div class="st-stat-label">
            <svg class="st-icon" viewBox="0 0 24 24" style="fill: #ffafcc;">
              <path
                d="M5,12L12,5L19,12H16V16H8V12H5M15,12V10H13V12H15M11,10V12H9V10H11M16,14.5A2.5,2.5 0 0,1 18.5,17A2.5,2.5 0 0,1 16,19.5A2.5,2.5 0 0,1 13.5,17A2.5,2.5 0 0,1 16,14.5M8,14.5A2.5,2.5 0 0,1 10.5,17A2.5,2.5 0 0,1 8,19.5A2.5,2.5 0 0,1 5.5,17A2.5,2.5 0 0,1 8,14.5Z"
              />
            </svg>
            <span>JITTER</span> <span class="unit">ms</span>
          </div>
          <div class="st-stat-value">
            {#if jitterMs !== null}
              <div in:fly={{ y: 10, duration: 400 }}>{formatVal(jitterMs, 0)}</div>
            {:else}
              —
            {/if}
          </div>
        </div>

        <div class="st-stat-item">
          <div class="st-stat-label">
            <svg class="st-icon" viewBox="0 0 24 24" style="fill: #ffafcc;">
              <path
                d="M13,13H11V7H13M13,17H11V15H13M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2Z"
              />
            </svg>
            <span>LOSS</span> <span class="unit">%</span>
          </div>
          <div class="st-stat-value">
            {#if packetLoss !== null}
              <div in:fly={{ y: 10, duration: 400 }}>
                {packetLoss !== null && packetLoss >= 0 ? formatVal(packetLoss) + "%" : "—"}
              </div>{:else}
              —
            {/if}
          </div>
        </div>
        <div class="st-stat-item">
          <div class="st-stat-label">
            <svg class="st-icon" viewBox="0 0 24 24" style="fill: var(--pool-blue);">
              <path d="M5,20H19V18H5M19,9H15V3H9V9H5L12,16L19,9Z" />
            </svg>
            <span>DOWNLOAD</span> <span class="unit">Mbps</span>
          </div>
          <div class="st-stat-value">
            {#if downloadMbps !== null}
              <div in:fly={{ y: 10, duration: 400 }}>{formatVal(downloadMbps)}</div>
            {:else}
              —
            {/if}
          </div>
        </div>
        <div class="st-stat-item">
          <div class="st-stat-label">
            <svg class="st-icon" viewBox="0 0 24 24" style="fill: #bf71ff;">
              <path d="M9,16V10H5L12,3L19,10H15V16H9M5,20V18H19V20H5Z" />
            </svg>
            <span>UPLOAD</span> <span class="unit">Mbps</span>
          </div>
          <div class="st-stat-value">
            {#if uploadMbps !== null}
              <div in:fly={{ y: 10, duration: 400 }}>{formatVal(uploadMbps)}</div>
            {:else}
              —
            {/if}
          </div>
        </div>
      </div>

      <div class="st-main">
        <div class="gauge-container">
          <!-- Gauge SVG (Always rendered, opacity changes) -->
          <div
            class="gauge-wrapper"
            style:opacity={phase === "init" || phase === "done" ? "0" : "1"}
            style:transition="opacity 0.4s"
          >
            <svg class="gauge-svg" viewBox="0 0 400 400">
              <defs>
                <linearGradient id="gaugeGradient" x1="0%" y1="0%" x2="100%" y2="0%">
                  <stop offset="0%" stop-color="#4fd1c5" />
                  <stop offset="100%" stop-color="#6afff3" />
                </linearGradient>
                <linearGradient id="gaugeGradientUpload" x1="0%" y1="0%" x2="100%" y2="0%">
                  <stop offset="0%" stop-color="#e0aaff" />
                  <stop offset="100%" stop-color="#bf71ff" />
                </linearGradient>
                <linearGradient id="gaugeGradientPing" x1="0%" y1="0%" x2="100%" y2="0%">
                  <stop offset="0%" stop-color="#fffbea" />
                  <stop offset="100%" stop-color="#fff38e" />
                </linearGradient>
                <linearGradient id="gaugeGradientLoss" x1="0%" y1="0%" x2="100%" y2="0%">
                  <stop offset="0%" stop-color="#ffeef5" />
                  <stop offset="100%" stop-color="#ffafcc" />
                </linearGradient>
              </defs>
              <!-- Thick Background Arc -->
              <!-- 
                DashArray: Circ ~ 880. Arc 270 deg = 660.
                Rotate 135 deg to start at bottom left.
              -->
              <circle
                cx="200"
                cy="200"
                r="140"
                fill="none"
                stroke="#26273b"
                stroke-width="30"
                stroke-linecap="round"
                stroke-dasharray="660 1000"
                transform="rotate(135 200 200)"
              />

              <!-- Value Arc -->
              <circle
                cx="200"
                cy="200"
                r="140"
                fill="none"
                stroke={phase === "upload" || phase === "done"
                  ? "url(#gaugeGradientUpload)"
                  : phase === "ping"
                    ? "url(#gaugeGradientPing)"
                    : phase === "loss"
                      ? "url(#gaugeGradientLoss)"
                      : "url(#gaugeGradient)"}
                stroke-width="30"
                stroke-linecap="round"
                transform="rotate(135 200 200)"
                class:gauge-pulse={phase === "ping" || phase === "loss"}
                style="stroke-dasharray: 660 1000; stroke-dashoffset: {660 -
                  660 *
                    ($currentSpeed > 0
                      ? getNonlinearDegree($currentSpeed) / 270
                      : phase === 'ping' || phase === 'loss'
                        ? 1
                        : 0)}; transition: stroke-dashoffset 0.4s linear;"
              />
              <!-- Center Value (Hidden in init/done) -->
              {#if phase !== "init" && phase !== "done"}
                <text
                  x="200"
                  y="200"
                  class="gauge-center-val"
                  text-anchor="middle"
                  dominant-baseline="middle"
                  in:fade
                >
                  {$currentSpeed.toFixed(1)}
                </text>
                <text
                  x="200"
                  y="240"
                  class="gauge-center-status"
                  text-anchor="middle"
                  style="fill: {phase === 'ping'
                    ? '#fff38e'
                    : phase === 'download'
                      ? 'var(--pool-blue)'
                      : phase === 'upload'
                        ? '#e0aaff'
                        : phase === 'loss'
                          ? '#ffafcc'
                          : 'var(--pool-blue)'}"
                  in:fade
                >
                  {phase}
                </text>
              {/if}

              <!-- Ticks -->
              {#each scaleTable as tick}
                {@const pos = getTickPos(tick.degree)}
                <text
                  x={pos.x}
                  y={pos.y}
                  class="gauge-tick"
                  text-anchor="middle"
                  dominant-baseline="middle"
                  style:opacity={tick.degree <= $tickProgressDegree ? 1 : 0}
                  style:transition="opacity 0.1s"
                >
                  {tick.value}
                </text>
              {/each}

              <!-- Active Icon -->
              {#if phase === "ping"}
                <circle cx="200" cy="265" r="5" fill="#fff38e" class="blink" />
              {:else if phase === "download"}
                <path d="M190 260 L210 260 L200 270 Z" fill="var(--pool-blue)" class="blink" />
              {:else if phase === "upload"}
                <path d="M190 270 L210 270 L200 260 Z" fill="#bf71ff" class="blink" />
              {:else if phase === "loss"}
                <path
                  d="M12,2L1,21H23M12,6L19.53,19H4.47M11,10V14H13V10M11,16V18H13V16"
                  transform="translate(188, 248) scale(1)"
                  fill="#ffafcc"
                  class="blink"
                />
              {/if}
            </svg>
          </div>

          <!-- GO Button Overlay -->
          {#if phase === "init" || phase === "done"}
            <div class="go-button-overlay" transition:scale>
              <div class="go-ring"></div>
              <button class="go-button" onclick={startTest}>GO</button>
            </div>
          {/if}
        </div>
      </div>

      <div class="st-footer">
        <div class="st-info-left">
          <div class="st-user-icon">
            <svg viewBox="0 0 24 24"
              ><path
                d="M12,4A4,4 0 0,1 16,8A4,4 0 0,1 12,12A4,4 0 0,1 8,8A4,4 0 0,1 12,4M12,14C16.42,14 20,15.79 20,18V20H4V18C4,15.79 7.58,14 12,14Z"
              /></svg
            >
          </div>
          <div
            class="st-info-text selectable"
            onclick={() => {
              if (phase === "init" || phase === "done") isInterfaceSelectOpen = true;
            }}
            role="button"
            tabindex="0"
            onkeydown={(e) => {
              if ((e.key === "Enter" || e.key === " ") && (phase === "init" || phase === "done")) {
                isInterfaceSelectOpen = true;
              }
            }}
          >
            <div class="st-label">
              {currentInterfaceName}{currentInterfaceName !== currentInterfaceId
                ? ` [${currentInterfaceId}]`
                : ""}
            </div>
            <div class="st-sub">{displayIP}</div>
          </div>
        </div>

        <label class="loss-toggle">
          <span>Parallel Loss</span>
          <div class="switch">
            <input
              type="checkbox"
              bind:checked={parallelLossEnabled}
              disabled={phase !== "init" && phase !== "done"}
            />
            <span class="slider"></span>
          </div>
        </label>

        <div
          class="st-info-right"
          class:clickable={phase === "init" || phase === "done"}
          onclick={() => {
            if (phase === "init" || phase === "done") {
              isServerSelectOpen = true;
              servers = [];
              serverSearch = "";
              clearTimeout(searchTimeout);
              fetchServers();
            }
          }}
          onkeydown={(e) => {
            if ((e.key === "Enter" || e.key === " ") && (phase === "init" || phase === "done")) {
              isServerSelectOpen = true;
              servers = [];
              serverSearch = "";
              clearTimeout(searchTimeout);
              fetchServers();
            }
          }}
          role="button"
          tabindex="0"
        >
          <div class="st-server-icon">
            <svg viewBox="0 0 24 24"
              ><path
                d="M17.9,17.39C17.64,16.59 16.89,16 16,16H15V13A1,1 0 0,0 14,12H8V10H10A1,1 0 0,0 11,9V7H13A2,2 0 0,0 15,5V4.59C17.93,5.77 20,8.64 20,12C20,14.08 19.2,15.97 17.9,17.39M11,19.93C7.05,19.44 4,16.08 4,12C4,11.38 4.08,10.78 4.21,10.21L9,15V16A2,2 0 0,0 11,18M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2Z"
              /></svg
            >
          </div>
          <div class="st-info-text">
            <div class="st-label">
              {#if serverInfo}
                {serverInfo.name} <span class="st-sponsor">({serverInfo.sponsor})</span>
              {:else}
                ...
              {/if}
            </div>
            <div class="st-sub">
              {#if serverInfo}
                {serverInfo.country}
                {#if serverInfo.id}
                  (ID: {serverInfo.id}){/if}
              {:else}
                {statusText || "Finding server..."}
              {/if}
            </div>
          </div>
        </div>
      </div>

      {#if error}
        <div class="st-error">{error}</div>
      {/if}

      {#if isServerSelectOpen}
        <div class="server-select-overlay" transition:fade={{ duration: 150 }}>
          <div class="server-select-header">
            <h3>Search</h3>
            <div class="server-search">
              <input
                type="text"
                placeholder="by name, city, country..."
                bind:value={serverSearch}
                oninput={handleSearch}
              />
            </div>
            <button class="close-btn-overlay" onclick={() => (isServerSelectOpen = false)}
              ><X size={20} /></button
            >
          </div>
          <div class="server-list">
            {#if loadingServers}
              <div class="loading">Loading servers...</div>
            {:else}
              {#each servers as server}
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <div
                  class="server-item"
                  onclick={() => selectServer(server)}
                  role="button"
                  tabindex="0"
                >
                  <div class="server-name">
                    {server.name} <span class="server-sponsor">({server.sponsor})</span>
                  </div>
                  <div class="server-meta">
                    [#{server.id}] {server.country} • {(server.distance / 1000).toFixed(0)} km
                  </div>
                </div>
              {/each}
            {/if}
          </div>
        </div>
      {/if}

      {#if isInterfaceSelectOpen}
        <div class="server-select-overlay" transition:fade={{ duration: 150 }}>
          <div class="server-select-header">
            <h3>Select Interface</h3>
            <button class="close-btn-overlay" onclick={() => (isInterfaceSelectOpen = false)}
              ><X size={20} /></button
            >
          </div>
          <div class="server-list">
            {#each interfaceList as iface}
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <div
                class="server-item"
                class:active={iface.id === currentInterfaceId}
                onclick={() => selectInterface(iface)}
                role="button"
                tabindex="0"
              >
                <div class="server-name">
                  {iface.name}
                </div>
                <div class="server-meta">
                  {iface.id}
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  :root {
    --primary-blue: #141526;
    --secondary-blue: #26273b;
    --pool-blue: #6afff3;
    --light-blue: #1cbfff;
    --purple: #bf71ff;
    --white: #ffffff;
    --gray: #9193a8;
  }

  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(10, 10, 20, 0.95);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 2000;
  }
  .modal-content {
    background: var(--primary-blue, #141526);
    width: 95%;
    max-width: 800px;
    height: 500px;
    border-radius: 8px;
    position: relative;
    box-shadow: 0 0 50px rgba(0, 0, 0, 0.5);
    font-family:
      system-ui,
      -apple-system,
      sans-serif;
    color: white;
    display: flex;
    flex-direction: column;
  }
  .close-btn {
    position: absolute;
    top: -40px;
    right: 0px;
    width: 40px;
    height: 40px;
    background: none;
    border: none;
    color: var(--gray);
    cursor: pointer;
    z-index: 200;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .close-btn:hover {
    color: white;
  }

  .st-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    flex-direction: column;
    height: 100%;
    padding: 20px 40px;
    box-sizing: border-box;
  }
  .st-stats {
    display: flex;
    justify-content: center;
    gap: 40px;
    margin-bottom: 20px;
    height: 80px;
    flex-shrink: 0;
    transition: transform 0.6s cubic-bezier(0.34, 1.56, 0.64, 1);
  }
  /* Removed .st-stats.result-mode scaling per user request */
  .st-stat-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    min-width: 100px;
  }
  .st-stat-label {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    font-weight: bold;
    color: white;
    text-transform: uppercase;
  }
  .st-stat-label .unit {
    color: var(--gray);
  }
  .st-stat-value {
    font-family: "Consolas", monospace;
    font-size: 32px;
    color: white;
    margin-top: 5px;
    font-weight: bold;
  }
  .st-icon {
    width: 16px;
    height: 16px;
    fill: white;
  }

  .st-main {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    min-height: 0;
  }

  .go-button-overlay {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10;
  }
  .gauge-wrapper {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .st-info-text.selectable {
    cursor: pointer;
  }
  .st-info-text.selectable:hover .st-label {
    text-decoration: underline;
    color: var(--pool-blue);
  }

  .st-info-right.clickable {
    cursor: pointer;
  }
  .st-info-right.clickable:hover .st-label {
    text-decoration: underline;
    color: var(--pool-blue);
  }

  .server-select-overlay {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: var(--primary-blue);
    z-index: 100;
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    padding: 20px;
    box-sizing: border-box;
  }
  .server-select-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
    color: white;
  }
  .server-select-header h3 {
    margin: 0;
    font-size: 1.2rem;
    white-space: nowrap;
  }
  .server-search {
    flex: 1;
    margin: 0 15px;
    min-width: 0;
  }
  .server-search input {
    width: 100%;
    background: var(--secondary-blue);
    border: 1px solid var(--gray);
    border-radius: 4px;
    padding: 6px 12px;
    color: white;
    font-size: 0.9rem;
    box-sizing: border-box;
  }
  .server-search input:focus {
    outline: none;
    border-color: var(--light-blue);
    box-shadow: 0 0 0 1px var(--light-blue);
  }
  .close-btn-overlay {
    background: none;
    border: none;
    color: var(--gray);
    cursor: pointer;
  }
  .close-btn-overlay:hover {
    color: white;
  }
  .server-list {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  /* Custom Scrollbar */
  .server-list::-webkit-scrollbar {
    width: 6px;
  }
  .server-list::-webkit-scrollbar-track {
    background: rgba(20, 21, 38, 0.5); /* dark variant of primary-blue */
    border-radius: 4px;
    margin: 4px 0;
  }
  .server-list::-webkit-scrollbar-thumb {
    background: var(--secondary-blue);
    border-radius: 4px;
  }
  .server-list::-webkit-scrollbar-thumb:hover {
    background: var(--light-blue);
  }
  .server-item {
    padding: 10px;
    border-radius: 6px;
    background: var(--secondary-blue);
    cursor: pointer;
    transition: background 0.2s;
  }
  .server-item.active {
    background: var(--light-blue);
    color: var(--primary-blue);
  }
  .server-item.active .server-meta {
    color: var(--primary-blue); /* High contrast on light blue (black-ish blue) */
    opacity: 0.8;
  }
  .server-item:hover {
    background: var(--light-blue);
    color: var(--primary-blue);
  }
  .server-item:hover .server-sponsor,
  .server-item:hover .server-meta {
    color: rgba(20, 21, 38, 0.7);
  }
  .server-name {
    font-weight: bold;
    font-size: 0.95rem;
  }
  .server-sponsor {
    font-weight: normal;
    color: var(--gray);
    font-size: 0.85rem;
  }
  .server-meta {
    font-size: 0.8rem;
    color: var(--gray);
    margin-top: 4px;
  }
  .go-button {
    width: 180px;
    height: 180px;
    border-radius: 50%;
    background: rgba(0, 0, 0, 0.2);
    border: 2px solid var(--pool-blue);
    color: var(--pool-blue);
    font-size: 48px;
    font-weight: bold;
    cursor: pointer;
    transition: all 0.3s;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    z-index: 2;
  }
  .go-button:hover {
    background: rgba(106, 255, 243, 0.1);
    color: white;
    border-color: white;
  }
  .go-ring {
    position: absolute;
    width: 180px;
    height: 180px;
    border-radius: 50%;
    border: 1px solid var(--pool-blue);
    opacity: 0;
    animation: pulse-ring 2s infinite;
  }
  @keyframes pulse-ring {
    0% {
      transform: scale(1);
      opacity: 0.8;
    }
    100% {
      transform: scale(1.5);
      opacity: 0;
    }
  }

  .gauge-container {
    width: 100%;
    height: 100%;
    display: flex;
    justify-content: center;
    align-items: center;
  }
  .gauge-svg {
    width: 100%;
    height: 100%;
    max-width: 400px;
    max-height: 350px;
  }
  .gauge-center-val {
    font-family: "Consolas", monospace;
    font-size: 56px;
    fill: white;
    font-weight: bold;
    letter-spacing: -2px;
  }
  .gauge-center-status {
    font-size: 14px;
    fill: var(--pool-blue);
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .gauge-tick {
    font-size: 14px;
    fill: var(--gray);
    font-weight: 500;
  }

  .blink {
    animation: blinker 1s linear infinite;
  }
  @keyframes blinker {
    50% {
      opacity: 0;
    }
  }

  .gauge-pulse {
    animation: gaugePulse 1.5s infinite ease-in-out;
  }
  @keyframes gaugePulse {
    0% {
      opacity: 0.6;
    }
    50% {
      opacity: 0.2;
    }
    100% {
      opacity: 0.6;
    }
  }

  .st-footer {
    display: flex;
    justify-content: space-between;
    margin-top: 10px;
    border-top: 1px solid #333;
    padding-top: 20px;
    flex-shrink: 0;
  }
  .st-info-left,
  .st-info-right {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .st-user-icon,
  .st-server-icon {
    width: 24px;
    height: 24px;
    fill: var(--gray);
  }
  .st-user-icon svg,
  .st-server-icon svg {
    width: 100%;
    height: 100%;
    fill: inherit;
  }
  .st-info-text {
    display: flex;
    flex-direction: column;
  }
  .st-label {
    font-size: 14px;
    font-weight: bold;
    color: var(--pool-blue);
  }
  .st-sub {
    font-size: 12px;
    color: var(--gray);
  }
  .st-error {
    color: #ff4444;
    position: absolute;
    bottom: 80px;
    width: 100%;
    text-align: center;
  }

  /* Toggle Switch */
  .loss-toggle {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-right: 15px;
    cursor: pointer;
    font-size: 12px;
    color: var(--gray);
  }
  .switch {
    position: relative;
    display: inline-block;
    width: 32px;
    height: 18px;
  }
  .switch input {
    opacity: 0;
    width: 0;
    height: 0;
  }
  .slider {
    position: absolute;
    cursor: pointer;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: #333;
    transition: 0.4s;
    border-radius: 18px;
  }
  .slider:before {
    position: absolute;
    content: "";
    height: 14px;
    width: 14px;
    left: 2px;
    bottom: 2px;
    background-color: white;
    transition: 0.4s;
    border-radius: 50%;
  }
  input:checked + .slider {
    background-color: #ffafcc;
  }
  input:checked + .slider:before {
    transform: translateX(14px);
  }

  @media (max-width: 850px) {
    .modal-content {
      height: auto;
      margin-top: 50px;
      /* removed overflow/max-height to fix close button clipping */
    }
    .st-main {
      flex: 1 1 auto;
      min-height: 0;
    }
    .gauge-svg {
      max-width: none;
      max-height: 40vh; /* Constrain height relative to viewport */
      width: 100%;
      height: 100%;
      min-height: 200px;
    }
    .go-button {
      width: 140px;
      height: 140px;
      font-size: 32px;
    }
    .go-ring {
      width: 140px;
      height: 140px;
    }

    .st-stats {
      display: grid;
      grid-template-columns: repeat(6, 1fr);
      height: auto;
      gap: 15px;
      margin-bottom: 30px;
    }
    .st-stat-item {
      min-width: 0;
      width: auto !important;
      margin-top: 0 !important;
    }
    /* Top row: Ping, Jitter, Loss (3 items -> span 2) */
    .st-stat-item:nth-child(1),
    .st-stat-item:nth-child(2),
    .st-stat-item:nth-child(3) {
      grid-column: span 2;
    }
    .st-stat-item:nth-child(1) .st-stat-value,
    .st-stat-item:nth-child(2) .st-stat-value,
    .st-stat-item:nth-child(3) .st-stat-value {
      font-size: 16px;
    }
    /* Bottom row: Download, Upload (2 items -> span 3) */
    .st-stat-item:nth-child(4),
    .st-stat-item:nth-child(5) {
      grid-column: span 3;
    }

    .st-footer {
      flex-direction: column;
      align-items: flex-start;
      gap: 15px;
      padding-top: 15px;
    }
    .st-info-left {
      width: 100%;
      order: 1;
    }
    .st-info-right {
      width: 100%;
      order: 2;
    }
    .loss-toggle {
      width: 100%;
      order: 3;
      margin-right: 0;
      justify-content: space-between;
      border-top: 1px solid rgba(255, 255, 255, 0.1);
      padding-top: 10px;
    }
  }
</style>
