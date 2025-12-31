<script lang="ts">
  import { t } from "../../data/locale.svelte";
  import { X } from "../../components/ui/icons";
  import { fade, scale } from "svelte/transition";
  import { API_BASE, fetcher } from "../../utils/fetcher";
  import { untrack } from "svelte";

  type Props = {
    interfaceId: string;
    interfaceName: string;
    interfaceIP?: string;
    onClose: () => void;
  };

  let { interfaceId, interfaceName, interfaceIP, onClose }: Props = $props();

  let phase = $state<"init" | "ping" | "download" | "upload" | "done">("init");
  let error = $state<string | null>(null);

  // Real-time values
  let currentSpeed = $state(0);
  let pingMs = $state<number | null>(null);
  let downloadMbps = $state<number | null>(null);
  let uploadMbps = $state<number | null>(null);
  let serverInfo = $state<{ name: string; country: string; sponsor?: string } | null>(null);
  let clientInfo = $state<{ ip: string; isp: string } | null>(null);
  let manualIP = $state<string | null>(null);
  let isTestDone = false; // Non-reactive flag to prevent late updates

  let displayIP = $derived(clientInfo?.ip || manualIP || interfaceIP || "...");

  let eventSource: EventSource | null = null;
  let statusText = $state(t("Initializing..."));

  // Gauge Logic from OpenSpeedTest
  const scaleTable = [
    { degree: 680, value: 0 },
    { degree: 570, value: 0.5 },
    { degree: 460, value: 1 },
    { degree: 337, value: 10 },
    { degree: 220, value: 100 },
    { degree: 115, value: 500 },
    { degree: 0, value: 1e3 }, // 1000+
  ];

  function getNonlinearDegree(c: number) {
    var d = 0;
    if (0 == c || 0 >= c || isNaN(c)) return 680; // Default to empty (680)
    for (; d < scaleTable.length; )
      if (c > scaleTable[d].value) d++;
      else
        return (
          scaleTable[d - 1].degree +
          ((c - scaleTable[d - 1].value) * (scaleTable[d].degree - scaleTable[d - 1].degree)) /
            (scaleTable[d].value - scaleTable[d - 1].value)
        );
    return scaleTable[scaleTable.length - 1].degree;
  }

  let gaugeOffset = $derived(getNonlinearDegree(currentSpeed));

  // Derived UI states
  // ... (keep derived as is, line 60-62 are fine)
  let isDownload = $derived(phase === "download");
  let isUpload = $derived(phase === "upload");
  let isPing = $derived(phase === "ping");

  $effect(() => {
    // We track interfaceId explicitly. If it changes, we restart.
    const id = interfaceId;

    // We untrack the rest to prevent restart when props like interfaceIP change (causing blinking)
    untrack(() => {
      startTest();
      fetchExternalIP();
    });

    return () => {
      if (eventSource) {
        eventSource.close();
      }
    };
  });

  async function fetchExternalIP() {
    // If we already have a likely external IP from props, maybe skip?
    if (interfaceIP && !isPrivateIP(interfaceIP)) return;

    try {
      const res = await fetcher.get<{ ip: string }>(
        `/system/interfaces/${interfaceId}/external-ip`,
      );
      if (res.ip) {
        manualIP = res.ip;
      }
    } catch (e) {
      console.error("Failed to check external IP inside modal", e);
    }
  }

  function isPrivateIP(ip: string) {
    // ...
    return (
      ip.startsWith("192.168.") ||
      ip.startsWith("10.") ||
      ip.startsWith("172.16.") ||
      ip === "127.0.0.1"
    );
  }

  function startTest() {
    error = null;
    phase = "init";
    isTestDone = false;
    statusText = t("Finding server...");

    // Construct URL with query param
    const url = `${API_BASE}/diagnostics/speedtest?interface=${interfaceId}`;

    eventSource = new EventSource(url);

    // ... listeners ...

    eventSource.addEventListener("status", (e) => {
      statusText = JSON.parse(e.data);
    });

    eventSource.addEventListener("error", (e) => {
      error = JSON.parse(e.data);
      eventSource?.close();
    });

    eventSource.addEventListener("server_info", (e) => {
      serverInfo = JSON.parse(e.data);
    });

    eventSource.addEventListener("client_info", (e) => {
      clientInfo = JSON.parse(e.data);
    });

    eventSource.addEventListener("stage", (e) => {
      phase = JSON.parse(e.data);
      currentSpeed = 0; // Reset gauge on phase change
    });

    eventSource.addEventListener("speed", (e) => {
      if (isTestDone) return;
      const data = JSON.parse(e.data);
      currentSpeed = data.mbps;
    });

    eventSource.addEventListener("result_ping", (e) => {
      pingMs = JSON.parse(e.data);
    });

    eventSource.addEventListener("result_download", (e) => {
      downloadMbps = JSON.parse(e.data);
    });

    eventSource.addEventListener("result_upload", (e) => {
      uploadMbps = JSON.parse(e.data);
    });

    eventSource.addEventListener("done", (e) => {
      phase = "done";
      isTestDone = true;
      statusText = t("Test complete");

      // Reset gauge to 0 immediately to prevent freeze
      currentSpeed = 0;

      eventSource?.close();
    });

    eventSource.onerror = (e) => {
      if (eventSource?.readyState === EventSource.CLOSED) return;
    };
  }

  function formatVal(val: number | null, fixed = 1) {
    if (val === null) return "---";
    return val.toFixed(fixed);
  }
</script>

<div class="modal-overlay" onclick={onClose} transition:fade={{ duration: 200 }}>
  <div class="modal-content" onclick={(e) => e.stopPropagation()} transition:scale={{ start: 0.9 }}>
    <button class="close-btn" onclick={onClose}>
      <X size={24} />
    </button>

    <!-- OpenSpeedTest Style Interface (Dark Mode) -->
    <div class="ost-wrapper">
      <!-- Increased Height to 400 for Stacked Layout -->
      <svg
        version="1.1"
        id="OpenSpeedtest"
        xmlns="http://www.w3.org/2000/svg"
        xmlns:xlink="http://www.w3.org/1999/xlink"
        viewBox="0 0 586 400"
        preserveAspectRatio="xMidYMid meet"
        class="ost-svg"
      >
        <defs>
          <linearGradient id="gradient" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="#56c4fb" />
            <stop offset="100%" stop-color="#0baeff" />
          </linearGradient>
          <symbol id="mainGaugebg" viewBox="0 0 295.93 270.18">
            <path
              fill="none"
              d="M64.32,256.41a137,137,0,1,1,192.09-24.8l-.07.09a134.45,134.45,0,0,1-24.71,24.71"
            />
          </symbol>
          <symbol id="mainGaugeBlue" viewBox="0 0 295.93 270.18">
            <path
              fill="none"
              d="M64.32,256.41a137,137,0,1,1,192.09-24.8l-.07.09a134.45,134.45,0,0,1-24.71,24.71"
            />
          </symbol>
          <symbol id="mainGaugeWhite" viewBox="0 0 295.93 270.18">
            <path
              fill="none"
              d="M64.32,256.41a137,137,0,1,1,192.09-24.8l-.07.09a134.45,134.45,0,0,1-24.71,24.71"
            />
          </symbol>
          <symbol id="oDoMeter" viewBox="0 0 295.93 270.18">
            <text transform="translate(76.34 264.28) scale(0.9 1)" class="oDo-Meter">0</text>
            <text transform="translate(23.81 164.6) scale(0.9 1)" class="oDo-Meter">.5</text>
            <text transform="translate(50.01 80.11) scale(0.9 1)" class="oDo-Meter">1</text>
            <text transform="translate(140.58 36.04) scale(0.9 1)" class="oDo-Meter">10</text>
            <text transform="translate(220.43 79.11) scale(0.9 1)" class="oDo-Meter">100</text>
            <text transform="translate(246.71 164.6) scale(0.9 1)" class="oDo-Meter">500</text>
            <text transform="translate(220.71 265.99) scale(0.9 1)" class="oDo-Meter">1000+</text>
          </symbol>

          <symbol id="resultCard" viewBox="0 0 278.13 85.49">
            <rect width="278.13" height="85.49" rx="12" ry="12" />
          </symbol>
          <!-- Icons -->
          <symbol id="downSymbol" viewBox="0 0 32.21 41.62"
            ><path
              d="M29.51,15.62H22.66V3.08A2.94,2.94,0,0,0,19.88,0H12.76A2.94,2.94,0,0,0,10,3.08V15.62H3.14l13.18,17.2Z"
            /><rect y="38.21" width="32.21" height="3.41" /></symbol
          >
          <symbol id="upSymbol" viewBox="0 0 32.21 41.62"
            ><path
              d="M2.71,26H9.55V38.55a2.93,2.93,0,0,0,2.78,3.07h7.13a2.94,2.94,0,0,0,2.78-3.07V26h6.84L15.89,8.8Z"
            /><rect width="32.21" height="3.41" /></symbol
          >
          <symbol id="pingSymbol" viewBox="0 0 14.28 14.28"
            ><path
              d="M13.92.64a.55.55,0,0,0-.65.13L12.06,2A7.21,7.21,0,0,0,9.79.52a7,7,0,0,0-5.42,0,7,7,0,0,0-3.8,3.8,7,7,0,0,0,0,5.54,7.23,7.23,0,0,0,1.52,2.28,7.25,7.25,0,0,0,2.28,1.53,6.89,6.89,0,0,0,2.77.56,7.12,7.12,0,0,0,5.5-2.57.31.31,0,0,0,.07-.21.23.23,0,0,0-.09-.19L11.35,10a.36.36,0,0,0-.24-.09.27.27,0,0,0-.21.11,4.61,4.61,0,0,1-1.67,1.37,4.68,4.68,0,0,1-2.09.48,4.75,4.75,0,0,1-3.36-1.39A4.84,4.84,0,0,1,2.76,9a4.68,4.68,0,0,1-.38-1.85A4.63,4.63,0,0,1,2.76,5.3,4.74,4.74,0,0,1,5.3,2.76a4.72,4.72,0,0,1,5.09.9L9.1,4.94A.54.54,0,0,0,9,5.58.56.56,0,0,0,9.52,6h4.17a.61.61,0,0,0,.42-.17.57.57,0,0,0,.17-.42V1.19a.56.56,0,0,0-.36-.55Z"
              style="fill: #14b0fe"
            /></symbol
          >

          <!-- Spinner Symbol -->
          <symbol id="spinner" viewBox="0 0 50 50">
            <path
              fill="currentColor"
              d="M25.251,6.461c-10.318,0-18.683,8.365-18.683,18.683h4.068c0-8.071,6.543-14.615,14.615-14.615V6.461z"
              transform="rotate(270 25 25)"
            >
              <animateTransform
                attributeType="xml"
                attributeName="transform"
                type="rotate"
                from="0 25 25"
                to="360 25 25"
                dur="1.2s"
                repeatCount="indefinite"
              />
            </path>
          </symbol>

          <!-- User Icon (Small person) -->
          <symbol
            id="userIcon"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
            <circle cx="12" cy="7" r="4"></circle>
          </symbol>

          <!-- Server Icon (Globe) -->
          <symbol
            id="serverIcon"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <circle cx="12" cy="12" r="10"></circle>
            <line x1="2" y1="12" x2="22" y2="12"></line>
            <path
              d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"
            ></path>
          </symbol>
        </defs>

        <g id="UI-Desk">
          <!-- Gauge Background -->
          <use
            class="main-Gaugebg"
            xlink:href="#mainGaugebg"
            x="10.28"
            y="36.11"
            width="273.94"
            height="245.39"
          ></use>
          <use xlink:href="#oDoMeter" x="10.28" y="36.11" width="273.94" height="245.39"></use>

          <!-- Active Gauge -->
          <use
            class="main-GaugeBlue"
            xlink:href="#mainGaugeBlue"
            x="10.28"
            y="36.11"
            width="273.94"
            height="245.39"
            style="stroke-dashoffset: {gaugeOffset}; stroke-opacity: {currentSpeed > 0 ? 1 : 0};"
          ></use>
          <use
            class="main-GaugeWhite"
            xlink:href="#mainGaugeWhite"
            x="10.28"
            y="36.11"
            width="273.94"
            height="245.39"
            style="stroke-dashoffset: {gaugeOffset + 1}; stroke-opacity: {currentSpeed > 0
              ? 1
              : 0};"
          ></use>

          <!-- Cards -->
          <use xlink:href="#resultCard" class="Cards" x="307.4" y="31" width="278.1" height="85.5"
          ></use>
          <use
            xlink:href="#resultCard"
            class="Cards"
            x="307.4"
            y="129.3"
            width="278.1"
            height="85.5"
          ></use>
          <use
            xlink:href="#resultCard"
            class="Cards"
            x="307.4"
            y="228.5"
            width="278.1"
            height="85.5"
          ></use>

          <!-- Static Icons & Active Spinners -->

          <!-- DOWNLOAD -->
          <!-- x: ~325 (left of card), y: ~60 (centered vertically in card), increased size -->
          <use xlink:href="#downSymbol" class="Symbol" x="325" y="58" width="24" height="30"></use>
          <text x="446" y="58" class="rtext" style="text-anchor: middle;">DOWNLOAD</text>
          {#if isDownload}
            <use xlink:href="#spinner" x="550" y="40" width="20" height="20" style="fill: #14b0fe;"
            ></use>
          {/if}

          <!-- UPLOAD -->
          <!-- x: ~325, y: ~158 (centered) -->
          <use xlink:href="#upSymbol" class="Symbol" x="325" y="156" width="24" height="30"></use>
          <text x="446" y="156.3" class="rtext" style="text-anchor: middle;">UPLOAD</text>
          {#if isUpload}
            <use xlink:href="#spinner" x="550" y="138" width="20" height="20" style="fill: #14b0fe;"
            ></use>
          {/if}

          <!-- PING -->
          <!-- x: ~325, y: ~255 (centered) -->
          <use xlink:href="#pingSymbol" x="325" y="255" width="26" height="26"></use>
          <text x="446" y="255.5" class="rtext" style="text-anchor: middle;">PING</text>

          <!-- Gauge Values -->
          <text class="oDoLive-Speed" x="147" y="180">{currentSpeed.toFixed(1)}</text>
          <text class="oDoLive-Status" x="147" y="210">{statusText}</text>

          <!-- CLIENT & SERVER INFO (Stacked on Left) -->

          <!-- Client Info Group (Top) -->
          <use
            xlink:href="#userIcon"
            x="20"
            y="300"
            width="24"
            height="24"
            style="stroke: #333; fill: none; opacity: 0.5"
          ></use>
          <!-- Interface Name -->
          <text class="info-label" x="53" y="310" style="text-anchor: start;">
            {interfaceName}
          </text>
          <!-- IP Address -->
          <text class="info-label-sub" x="53" y="330" style="text-anchor: start;">
            {displayIP}
          </text>

          <!-- Server Info Group (Bottom) -->
          <use
            xlink:href="#serverIcon"
            x="20"
            y="350"
            width="24"
            height="24"
            style="stroke: #333; fill: none; opacity: 0.5"
          ></use>
          <!-- Server Name -->
          <text class="info-label" x="53" y="360" style="text-anchor: start;">
            {serverInfo ? serverInfo.sponsor || serverInfo.name : "..."}
          </text>
          <!-- Location -->
          <text class="info-label-sub" x="53" y="380" style="text-anchor: start;">
            {serverInfo ? serverInfo.country : ""}
          </text>

          <!-- Results -->
          <text class="rtextnum" x="446" y="90">{formatVal(downloadMbps)}</text>
          <text class="rtextmbms" x="446" y="105">Mbps</text>

          <text class="rtextnum" x="446" y="190">{formatVal(uploadMbps)}</text>
          <text class="rtextmbms" x="446" y="206">Mbps</text>

          <text class="rtextnum" x="446" y="285">{formatVal(pingMs, 0)}</text>
          <text class="rtextmbms" x="446" y="300">ms</text>
        </g>
      </svg>
    </div>

    {#if error}
      <div class="error-msg">{error}</div>
    {/if}
  </div>
</div>

<style>
  @import url("https://fonts.googleapis.com/css2?family=Roboto:wght@400;500;700&display=swap");

  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(0, 0, 0, 0.85); /* Darker backdrop */
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 2000;
  }

  .modal-content {
    background: #000000; /* Pure black / Dark Theme */
    border: 1px solid #333;
    border-radius: 20px;
    width: 95%;
    max-width: 850px;
    position: relative;
    padding: 30px;
    box-shadow: 0 30px 60px rgba(0, 0, 0, 0.8);
    font-family: "Roboto", sans-serif;
  }

  .close-btn {
    position: absolute;
    top: 15px;
    right: 20px;
    background: none;
    border: none;
    cursor: pointer;
    z-index: 100;
    color: #666;
    transition: color 0.2s;
  }
  .close-btn:hover {
    color: #fff;
  }

  .ost-wrapper {
    position: relative;
    width: 100%;
    max-width: 700px;
    margin: 0 auto;
    aspect-ratio: 586/400; /* Increased aspect ratio for height */
  }

  .ost-svg {
    width: 100%;
    height: 100%;
  }

  /* SVG STYLES (Dark Theme) */
  .main-Gaugebg {
    fill: none;
    stroke: #1a1a1a;
    stroke-linecap: round;
    stroke-linejoin: round;
    stroke-width: 22px;
    stroke-dasharray: 681;
  }

  .main-GaugeBlue {
    fill: none;
    stroke: url(#gradient);
    stroke-linecap: round;
    stroke-linejoin: round;
    stroke-width: 22px;
    stroke-dasharray: 681;
    transition: stroke-dashoffset 0.1s linear;
  }

  .main-GaugeWhite {
    fill: none;
    stroke: #ffffff;
    stroke-linecap: round;
    stroke-linejoin: round;
    stroke-width: 15px;
    stroke-dasharray: 0, 681;
    transition: stroke-dashoffset 0.1s linear;
  }

  .Cards {
    fill: #111111; /* Dark Card Bg */
  }

  .Symbol {
    fill: url(#gradient);
  }

  /* Typography */
  .oDo-Meter {
    font-size: 16.63px;
    fill: #cecece; /* Light Grey for Dark Mode */
    font-family: "Roboto", sans-serif;
    font-weight: 500;
  }

  .oDoLive-Speed {
    font-size: 48px;
    fill: #ffffff;
    font-family: "Roboto", sans-serif;
    font-weight: 500;
    text-anchor: middle;
  }

  .info-label {
    font-size: 14px;
    fill: #14b0fe; /* Blue accent for Server/Interface name */
    font-family: "Roboto", sans-serif;
    font-weight: 700; /* Bold */
    text-anchor: middle;
  }

  .info-label-sub {
    font-size: 12px;
    fill: #666; /* Grey for IP/Country */
    font-family: "Roboto", sans-serif;
    font-weight: 400;
    text-anchor: middle;
  }

  .oDoLive-Status {
    font-size: 14px;
    fill: #888;
    font-family: "Roboto", sans-serif;
    font-weight: 500;
    text-anchor: middle;
  }

  .rtext {
    font-size: 12px;
    fill: #888;
    font-family: "Roboto", sans-serif;
    font-weight: 500;
  }

  .rtextnum {
    font-size: 26px; /* Slightly larger for readability */
    fill: #ffffff;
    font-family: "Roboto", sans-serif;
    font-weight: 500;
    text-anchor: middle;
  }

  .rtextmbms {
    font-size: 12px;
    fill: #666;
    font-family: "Roboto", sans-serif;
    font-weight: 500;
    text-anchor: middle;
  }

  .error-msg {
    text-align: center;
    color: #ff4444;
    margin-top: 15px;
    font-weight: bold;
  }
</style>
