<script lang="ts">
  import { onMount } from 'svelte';
  import { Minus, Square, Copy, X, TelevisionSimple } from 'phosphor-svelte';
  import TopMenuBar from './TopMenuBar.svelte';
  import {
    WindowMinimise,
    WindowToggleMaximise,
    WindowIsMaximised,
    Quit
  } from '../../../wailsjs/runtime/runtime';

  let {
    onToggleBigPicture = () => {},
    activeTab = 'catalog',
    downloadPath = 'C:\\Ducke',
    hasActiveDownloads = false,
    hasPausedDownloads = false,
    onOpenSettingsSubTab = (subTab: string) => {},
    onOpenFavoritesFilter = (filter: string) => {},
    onOpenFolder = (path: string) => {},
    onOpenConfigFolder = () => {},
    onRefreshCatalog = () => {},
    onClearMetadataCache = () => {},
    onPauseAllDownloads = () => {},
    onResumeAllDownloads = () => {},
    onClearCompletedDownloads = () => {},
    onImportFileZilla = () => {},
    onTestConnection = () => {},
    onCheckForUpdates = () => {},
    onExportLogs = () => {},
    onOpenShortcutsHelp = () => {}
  } = $props();

  let isMaximised = $state<boolean>(false);

  async function checkMaximised() {
    try {
      isMaximised = await WindowIsMaximised();
    } catch {
      // ignore
    }
  }

  function handleMinimise() {
    try {
      WindowMinimise();
    } catch (e) {
      console.error(e);
    }
  }

  function handleToggleMaximise() {
    try {
      WindowToggleMaximise();
      setTimeout(checkMaximised, 100);
    } catch (e) {
      console.error(e);
    }
  }

  function handleClose() {
    try {
      Quit();
    } catch (e) {
      console.error(e);
    }
  }

  onMount(() => {
    checkMaximised();
    window.addEventListener('resize', checkMaximised);
    return () => {
      window.removeEventListener('resize', checkMaximised);
    };
  });
</script>

<!-- Custom Seamless Window Titlebar (Modern 48px TopBar) -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<header
  class="h-12 w-full bg-[#0F1218] flex items-center justify-between select-none z-50 flex-shrink-0 px-3"
  style="--wails-draggable: drag;"
  ondblclick={handleToggleMaximise}
>
  <!-- Left: Steam Menu Bar with Integrated Logo & Breadcrumb -->
  <div class="flex items-center gap-2">
    <TopMenuBar
      {downloadPath}
      {hasActiveDownloads}
      {hasPausedDownloads}
      {onOpenSettingsSubTab}
      {onOpenFavoritesFilter}
      {onOpenFolder}
      {onOpenConfigFolder}
      {onToggleBigPicture}
      {onRefreshCatalog}
      {onClearMetadataCache}
      {onPauseAllDownloads}
      {onResumeAllDownloads}
      {onClearCompletedDownloads}
      {onImportFileZilla}
      {onTestConnection}
      {onCheckForUpdates}
      {onExportLogs}
      {onOpenShortcutsHelp}
      onQuit={handleClose}
    />
  </div>

  <!-- Center: Draggable Area -->
  <div class="flex-1 h-full" style="--wails-draggable: drag;"></div>

  <!-- Right: Window Action Buttons & Big Picture Switcher -->
  <div class="flex items-center h-full gap-1.5" style="--wails-draggable: no-drag;">
    <!-- Big Picture Console Mode Button (Modern Squircle Pill) -->
    <button
      type="button"
      class="h-8 px-3 rounded-[12px] flex items-center gap-2 text-[11px] font-medium text-[#94a3b8] hover:text-white bg-white/[0.03] hover:bg-white/[0.08] transition-all cursor-pointer mr-1"
      onclick={onToggleBigPicture}
      title="Режим Big Picture (Консольный вид)"
      aria-label="Режим Big Picture"
    >
      <TelevisionSimple size={15} weight="bold" class="text-sky-400" />
      <span class="hidden sm:inline">Big Picture</span>
    </button>

    <!-- Minimize (Squircle button) -->
    <button
      type="button"
      class="h-8 w-8 rounded-[8px] flex items-center justify-center text-[#8e95a2] hover:text-white hover:bg-white/[0.08] active:bg-white/[0.12] transition-colors cursor-pointer"
      onclick={handleMinimise}
      title="Свернуть"
      aria-label="Свернуть"
    >
      <Minus size={13} weight="bold" />
    </button>

    <!-- Maximize / Restore -->
    <button
      type="button"
      class="h-8 w-8 rounded-[8px] flex items-center justify-center text-[#8e95a2] hover:text-white hover:bg-white/[0.08] active:bg-white/[0.12] transition-colors cursor-pointer"
      onclick={handleToggleMaximise}
      title={isMaximised ? "Восстановить" : "Развернуть"}
      aria-label={isMaximised ? "Восстановить" : "Развернуть"}
    >
      {#if isMaximised}
        <Copy size={13} weight="regular" class="rotate-180" />
      {:else}
        <Square size={13} weight="regular" />
      {/if}
    </button>

    <!-- Close -->
    <button
      type="button"
      class="h-8 w-8 rounded-[8px] flex items-center justify-center text-[#8e95a2] hover:text-white hover:bg-[#e81123] active:bg-[#c4101f] transition-colors cursor-pointer"
      onclick={handleClose}
      title="Закрыть"
      aria-label="Закрыть"
    >
      <X size={13} weight="bold" />
    </button>
  </div>
</header>
