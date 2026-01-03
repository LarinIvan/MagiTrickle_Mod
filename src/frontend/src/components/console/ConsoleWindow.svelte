<script lang="ts">
  import { consoleStore } from "../../data/console.svelte";
  import {
    X,
    Minus,
    Square,
    Terminal,
    Copy,
    ArrowUpLeftFromCircle,
    ChevronsUp,
    ChevronsDown,
  } from "lucide-svelte";
  import { onMount, tick } from "svelte";

  let windowEl = $state<HTMLElement>();
  let logContentEl = $state<HTMLElement>();
  let isDragging = false;
  let isResizing = false;
  let dragOffset = { x: 0, y: 0 };
  let startDim = { w: 0, h: 0, x: 0, y: 0 };

  // Mobile State
  let innerWidth = $state(typeof window !== "undefined" ? window.innerWidth : 1000);
  let isMobile = $derived(innerWidth <= 700);
  let mobileFull = $state(false); // false = 50%, true = 100%

  // Scroll to bottom on new logs
  $effect(() => {
    if (consoleStore.logs.length && logContentEl) {
      // Only auto-scroll if already near bottom
      const isNearBottom =
        logContentEl.scrollHeight - logContentEl.scrollTop - logContentEl.clientHeight < 50;
      if (isNearBottom) {
        tick().then(() => {
          if (logContentEl) logContentEl.scrollTop = logContentEl.scrollHeight;
        });
      }
    }
  });

  function startDrag(e: PointerEvent) {
    if (isMobile) return; // No drag on mobile
    const target = e.target as Element;
    // Explicitly ignore buttons and controls to prevent drag start
    if (target.closest && (target.closest(".window-controls") || target.closest("button"))) return;

    isDragging = true;
    dragOffset.x = e.clientX - consoleStore.x;
    dragOffset.y = e.clientY - consoleStore.y;
    // Use windowEl check
    if (windowEl) windowEl.setPointerCapture(e.pointerId);
  }

  function onDrag(e: PointerEvent) {
    if (isDragging) {
      consoleStore.x = Math.max(0, e.clientX - dragOffset.x);
      consoleStore.y = Math.max(0, e.clientY - dragOffset.y);
    } else if (isResizing) {
      consoleStore.width = Math.max(300, startDim.w + (e.clientX - startDim.x));
      consoleStore.height = Math.max(200, startDim.h + (e.clientY - startDim.y));
    }
  }

  function stopDrag(e: PointerEvent) {
    isDragging = false;
    isResizing = false;
    if (windowEl && windowEl.hasPointerCapture(e.pointerId)) {
      windowEl.releasePointerCapture(e.pointerId);
    }
  }

  function startResize(e: PointerEvent) {
    if (isMobile) return;
    isResizing = true;
    startDim = { w: consoleStore.width, h: consoleStore.height, x: e.clientX, y: e.clientY };
    e.stopPropagation(); // prevent drag
    if (windowEl) windowEl.setPointerCapture(e.pointerId);
  }

  function handleCopy() {
    const text = consoleStore.logs
      .map((l) => {
        const time = l.time || "";
        const level = l.level || "INFO";
        const msg = l.message || "";
        let line = `${time} [${level}] ${msg}`;
        if (l.kvPairs?.length) {
          line += " " + l.kvPairs.map((k) => `${k.key}=${k.value}`).join(" ");
        }
        return line;
      })
      .join("\n");

    if (!text) return;

    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text);
    } else {
      // Fallback for non-secure contexts (HTTP)
      const textArea = document.createElement("textarea");
      textArea.value = text;
      textArea.style.position = "fixed";
      textArea.style.left = "-9999px";
      textArea.style.top = "0";
      document.body.appendChild(textArea);
      textArea.focus();
      textArea.select();
      try {
        document.execCommand("copy");
      } catch (err) {
        console.error("Fallback: Oops, unable to copy", err);
      }
      document.body.removeChild(textArea);
    }
  }

  // Helper for Mobile Logic
  function handleMobileExpand() {
    if (consoleStore.isMinimized) {
      // Minimized -> Half
      consoleStore.isMinimized = false;
      mobileFull = false;
    } else if (!mobileFull) {
      // Half -> Full
      mobileFull = true;
    } else {
      // Full -> Half
      mobileFull = false;
    }
  }

  // Action to prevent background scroll on the window container
  function preventTouch(node: HTMLElement) {
    const handler = (e: TouchEvent) => {
      // Prevent default browser scroll processing
      e.preventDefault();
    };
    // Non-passive is required to use preventDefault
    node.addEventListener("touchmove", handler, { passive: false });
    return {
      destroy() {
        node.removeEventListener("touchmove", handler);
      },
    };
  }

  // Action to allow internal scroll but contain it
  function isolateScroll(node: HTMLElement) {
    const handler = (e: TouchEvent) => {
      // If content is scrollable
      if (node.scrollHeight > node.clientHeight) {
        // Stop propagation so the parent (preventTouch) doesn't see it
        e.stopPropagation();
      } else {
        // If not scrollable, prevent default to stop background scroll
        e.preventDefault();
      }
    };
    node.addEventListener("touchmove", handler, { passive: false });
    return {
      destroy() {
        node.removeEventListener("touchmove", handler);
      },
    };
  }
</script>

<svelte:window onpointerup={stopDrag} onresize={() => (innerWidth = window.innerWidth)} />

{#if consoleStore.isOpen}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="console-window"
    class:collapsed={consoleStore.isMinimized}
    class:mobile={isMobile}
    style:left={isMobile ? "0" : `${consoleStore.x}px`}
    style:top={isMobile ? "auto" : `${consoleStore.y}px`}
    style:width={isMobile ? "100%" : `${consoleStore.width}px`}
    style:height={isMobile
      ? consoleStore.isMinimized
        ? "auto"
        : mobileFull
          ? "calc(100vh - 65px)"
          : "50vh"
      : consoleStore.isMinimized
        ? "auto"
        : `${consoleStore.height}px`}
    style:bottom={isMobile ? "60px" : "auto"}
    bind:this={windowEl}
    onpointermove={onDrag}
    onpointerup={stopDrag}
    onpointerdown={(e) => e.stopPropagation()}
    onmousedown={(e) => e.stopPropagation()}
    onwheel={(e) => e.stopPropagation()}
    use:preventTouch
  >
    <div class="window-header" onpointerdown={startDrag}>
      <div class="window-title">
        <Terminal size={14} class="mr-2 text-primary-400" />
        {#if consoleStore.isMinimized && consoleStore.unreadCount > 0}
          <span class="badge mr-2">{consoleStore.unreadCount}</span>
        {/if}
        <span>Debug Console</span>
      </div>
      <div class="window-controls">
        <button
          onclick={(e) => {
            e.stopPropagation();
            handleCopy();
          }}
          class="control-btn"
          title="Copy All"
        >
          <Copy size={12} />
        </button>
        <button
          onclick={(e) => {
            e.stopPropagation();
            consoleStore.clear();
          }}
          class="control-btn"
          title="Clear Logs"
        >
          <span class="text-xs uppercase font-bold text-gray-400 hover:text-white">CLR</span>
        </button>

        {#if isMobile}
          <!-- Mobile Controls -->
          {#if !consoleStore.isMinimized}
            <button
              onclick={(e) => {
                e.stopPropagation();
                consoleStore.isMinimized = true;
              }}
              class="control-btn"
              title="Collapse"
            >
              <Minus size={14} />
            </button>
          {/if}

          <button
            onclick={(e) => {
              e.stopPropagation();
              handleMobileExpand();
            }}
            class="control-btn"
            title={consoleStore.isMinimized ? "Open" : mobileFull ? "Restore" : "Full Screen"}
          >
            {#if consoleStore.isMinimized}
              <ChevronsUp size={14} />
            {:else if mobileFull}
              <ChevronsDown size={14} />
            {:else}
              <ChevronsUp size={14} />
            {/if}
          </button>
        {:else}
          <!-- Desktop Controls -->
          <button
            onclick={(e) => {
              e.stopPropagation();
              consoleStore.toggleMinimize();
            }}
            class="control-btn"
            title={consoleStore.isMinimized ? "Expand" : "Collapse"}
          >
            {#if consoleStore.isMinimized}
              <Square size={14} />
            {:else}
              <Minus size={14} />
            {/if}
          </button>
        {/if}

        <button
          onclick={(e) => {
            e.stopPropagation();
            consoleStore.toggle();
          }}
          class="control-btn close"
          title="Close"
        >
          <X size={14} />
        </button>
      </div>
    </div>

    {#if !consoleStore.isMinimized}
      <div class="window-body" bind:this={logContentEl} use:isolateScroll>
        {#each consoleStore.logs as log (log.id)}
          <div class="log-line">
            <span class="log-time">{log.time}</span>
            <span class="log-level" data-level={log.level}>{log.level}</span>
            <span class="log-msg" class:error-msg={log.isError}>{log.message}</span>
            {#each log.kvPairs as kv}
              <span class="log-kv">
                <span class="key">{kv.key}=</span><span class="val">{kv.value}</span>
              </span>
            {/each}
          </div>
        {/each}
        {#if consoleStore.logs.length === 0}
          <div class="text-gray-500 italic p-2 opacity-50 text-xs">Waiting for logs...</div>
          <div class="text-gray-600 text-[10px] px-2">
            Stream connected: {consoleStore.eventSource?.readyState === 1 ? "Yes" : "No"}
          </div>
        {/if}
      </div>

      <div class="resize-handle" onpointerdown={startResize}></div>
    {/if}
  </div>
{/if}

<style>
  .console-window {
    position: fixed;
    background: rgba(10, 10, 12, 0.98); /* Almost black, opaque */
    border: 1px solid #333;
    border-radius: 6px;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.8);
    display: flex;
    flex-direction: column;
    z-index: 10000; /* Top most */
    overflow: hidden;
    font-family: "JetBrains Mono", "Consolas", monospace;
  }

  /* Collapsed state overrides */
  .console-window.collapsed {
    height: auto !important;
    border-bottom-left-radius: 6px;
    border-bottom-right-radius: 6px;
  }

  .window-header {
    height: 32px;
    background: #1a1a1e;
    border-bottom: 1px solid #333;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 8px;
    cursor: move;
    user-select: none;
  }
  .console-window.collapsed .window-header {
    border-bottom: none;
  }

  .window-title {
    display: flex;
    align-items: center;
    font-size: 0.75rem;
    font-weight: 700;
    color: #e2e8f0;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .badge {
    background: #ef4444;
    color: white;
    font-size: 0.65rem;
    padding: 0 5px;
    border-radius: 4px;
    height: 16px;
    display: flex;
    align-items: center;
  }

  .window-controls {
    display: flex;
    gap: 4px;
    height: 100%;
    align-items: center;
  }

  .control-btn {
    background: transparent;
    border: none;
    color: #94a3b8;
    cursor: pointer;
    width: 24px;
    height: 24px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.15s;
    -webkit-app-region: no-drag; /* For Tauri/Electron context if compatible, ignored in web */
  }

  /* Only apply hover on devices that support it to avoid sticky states on mobile */
  @media (hover: hover) {
    .control-btn:hover {
      background: rgba(255, 255, 255, 0.1);
      color: white;
    }
    .control-btn.close:hover {
      background: #ef4444;
      color: white;
    }
  }

  /* Active state for immediate feedback on both desktop and mobile */
  .control-btn:active {
    background: rgba(255, 255, 255, 0.2);
    color: white;
    transform: scale(0.95);
  }

  .window-body {
    flex: 1;
    background: #000000; /* Pure black for console content */
    overflow-y: auto;
    padding: 4px 6px;
    font-size: 0.75rem; /* Small terminal font */
    line-height: 1.35;
    color: #cbd5e1;
    white-space: pre-wrap;
    user-select: text;
    cursor: text;
    scrollbar-width: thin;
    scrollbar-color: #333 #000;
    overscroll-behavior: contain; /* Prevent scroll chaining */
  }

  .window-body::-webkit-scrollbar {
    width: 8px;
  }
  .window-body::-webkit-scrollbar-track {
    background: #000;
  }
  .window-body::-webkit-scrollbar-thumb {
    background-color: #333;
    border-radius: 4px;
    border: 1px solid #000;
  }
  .window-body::-webkit-scrollbar-thumb:hover {
    background-color: #555;
  }

  .log-line {
    display: block;
    margin-bottom: 1px;
    word-break: break-all;
  }

  .log-time {
    color: #64748b;
    margin-right: 6px;
  }

  .log-level {
    font-weight: bold;
    margin-right: 6px;
  }
  .log-level[data-level="INF"] {
    color: #22c55e;
  } /* Green */
  .log-level[data-level="WRN"] {
    color: #eab308;
  } /* Yellow */
  .log-level[data-level="ERR"] {
    color: #ef4444;
  } /* Red */
  .log-level[data-level="DBG"] {
    color: #3b82f6;
  } /* Blue */
  .log-level[data-level="TRC"] {
    color: #a855f7;
  } /* Purple */

  .log-msg {
    color: #e2e8f0;
    margin-right: 6px;
  }
  .log-msg.error-msg {
    color: #fca5a5;
  }

  .log-kv {
    margin-right: 6px;
    font-size: 0.7rem;
    opacity: 0.9;
  }
  .log-kv .key {
    color: #06b6d4; /* Cyan */
  }
  .log-kv .val {
    color: #94a3b8;
    margin-left: 2px;
  }

  .resize-handle {
    position: absolute;
    bottom: 0;
    right: 0;
    width: 12px;
    height: 12px;
    cursor: nwse-resize;
    z-index: 20;
    background: linear-gradient(135deg, transparent 50%, #475569 50%);
    opacity: 0.5;
  }
  .resize-handle:hover {
    opacity: 1;
  }

  /* Mobile Overrides */
  .console-window.mobile {
    border-radius: 12px 12px 0 0 !important;
    border: 1px solid #333;
    border-bottom: none;
    transition: height 0.3s cubic-bezier(0.2, 0, 0.2, 1);
    /* CRITICAL: Prevent browser scroll handling on the container itself */
    touch-action: none;
    /* Fix width overflow issues */
    width: auto !important;
    left: 0 !important;
    right: 0 !important;
    box-sizing: border-box; /* Ensure border is inside width */
  }

  /* Make sure the body allows scrolling */
  .console-window.mobile .window-body {
    touch-action: pan-y;
  }

  .console-window.mobile.collapsed {
    border-bottom: 1px solid #333;
    border-radius: 6px 6px 0 0 !important;
  }
</style>
