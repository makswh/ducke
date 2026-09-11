<script lang="ts">
  import {
    Library,
    Download,
    Settings,
    Gamepad2,
    Layers,
    Magnet,
    Tv,
    Check,
    Bookmark
  } from 'lucide-svelte';

  let {
    activeTab = $bindable<'catalog' | 'torrents' | 'favorites' | 'downloads' | 'settings'>('catalog'),
    activeDownloadsCount = 0,
    downloadProgress = { isDownloading: false, percent: 0, title: '', speed: '' },
    hasFtpServers = false,
    hasTorrentSources = false,
    isGamepadConnected = false,
    isConnected = false,
    serverName = '',
    onRefresh = () => {},
    isRefreshing = false,
    metadataProgress = { isSyncing: false, current: 0, total: 0, currentGame: '' },
    isCatalogLoading = false,
    onToggleBigPicture = () => {}
  } = $props();

  let isCompletedRecently = $state(false);
  let completedTimeout: any = null;
  let wasSyncing = $state(false);

  $effect(() => {
    const isSync = metadataProgress.isSyncing || isRefreshing;
    if (wasSyncing && !isSync && metadataProgress.total > 0) {
      isCompletedRecently = true;
      if (completedTimeout) clearTimeout(completedTimeout);
      completedTimeout = setTimeout(() => {
        isCompletedRecently = false;
      }, 3500);
    }
    wasSyncing = isSync;
  });

  let percent = $derived.by(() => {
    if (!metadataProgress.total || metadataProgress.total <= 0) return 0;
    const p = Math.round((metadataProgress.current / metadataProgress.total) * 100);
    return Math.min(100, Math.max(0, p));
  });

  const radius = 11;
  const circumference = 2 * Math.PI * radius; // 69.115
  let strokeDashoffset = $derived(circumference - (percent / 100) * circumference);

  let dlPercent = $derived(Math.min(100, Math.max(0, downloadProgress.percent || 0)));
  let dlStrokeDashoffset = $derived(circumference - (dlPercent / 100) * circumference);
</script>

<aside data-nav-zone="sidebar" class="w-[60px] flex flex-col items-center justify-between py-4 flex-shrink-0 z-30 select-none bg-[#08090d] border-r border-white/[0.06]">
  <!-- Navigation Tabs (Vertical Icons) -->
  <div class="flex flex-col items-center gap-2 w-full">
    <nav class="flex flex-col items-center gap-2.5 w-full">
      <!-- Catalog Tab (only shown if FTP servers are configured) -->
      {#if hasFtpServers}
        <div class="relative w-full flex items-center justify-center">
          {#if activeTab === 'catalog'}
            <div class="absolute left-0 top-2 bottom-2 w-[3px] rounded-r bg-white"></div>
          {/if}
          <button
            data-nav-item
            class="relative w-10 h-10 rounded-xl flex items-center justify-center transition-colors cursor-pointer {activeTab === 'catalog' ? 'bg-white/[0.08] text-white border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
            onclick={() => (activeTab = 'catalog')}
            title={isCatalogLoading ? "Каталог игр (загрузка...)" : "Каталог игр"}
          >
            <Layers class="w-5 h-5 stroke-[1.75] {activeTab === 'catalog' ? (isCatalogLoading ? 'text-sky-400 animate-pulse' : 'text-white') : ''}" />
            {#if isCatalogLoading}
              <span class="absolute top-1.5 right-1.5 w-1.5 h-1.5 rounded-full bg-sky-400 animate-ping"></span>
              <span class="absolute top-1.5 right-1.5 w-1.5 h-1.5 rounded-full bg-sky-400"></span>
            {/if}
          </button>
        </div>
      {/if}

      <!-- Torrents Tab (Conditional) -->
      {#if hasTorrentSources}
        <div class="relative w-full flex items-center justify-center">
          {#if activeTab === 'torrents'}
            <div class="absolute left-0 top-2 bottom-2 w-[3px] rounded-r bg-white"></div>
          {/if}
          <button
            data-nav-item
            class="w-10 h-10 rounded-xl flex items-center justify-center transition-colors cursor-pointer {activeTab === 'torrents' ? 'bg-white/[0.08] text-white border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
            onclick={() => (activeTab = 'torrents')}
            title="Торренты"
          >
            <Magnet class="w-5 h-5 stroke-[1.75] {activeTab === 'torrents' ? 'text-white' : ''}" />
          </button>
        </div>
      {/if}

      <!-- Favorites Tab -->
      <div class="relative w-full flex items-center justify-center">
        {#if activeTab === 'favorites'}
          <div class="absolute left-0 top-2 bottom-2 w-[3px] rounded-r bg-white"></div>
        {/if}
        <button
          data-nav-item
          class="w-10 h-10 rounded-xl flex items-center justify-center transition-colors cursor-pointer {activeTab === 'favorites' ? 'bg-white/[0.08] text-white border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
          onclick={() => (activeTab = 'favorites')}
          title="Избранное"
        >
          <Bookmark class="w-5 h-5 stroke-[1.75] {activeTab === 'favorites' ? 'text-white' : ''}" />
        </button>
      </div>

      <!-- Downloads Tab -->
      <div class="relative w-full flex items-center justify-center group/dl">
        {#if activeTab === 'downloads'}
          <div class="absolute left-0 top-2 bottom-2 w-[3px] rounded-r bg-white"></div>
        {/if}
        <button
          data-nav-item
          class="relative w-10 h-10 rounded-xl flex items-center justify-center transition-colors cursor-pointer {activeTab === 'downloads' ? 'bg-white/[0.08] text-white border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
          onclick={() => (activeTab = 'downloads')}
          title={downloadProgress.isDownloading ? `Загрузка: ${dlPercent}%` : 'Загрузки'}
        >
          {#if downloadProgress.isDownloading}
            <!-- Circular Radial Progress (design identical to metadata sync indicator) -->
            <div class="relative w-8 h-8 rounded-full flex items-center justify-center bg-[#0d1017] border border-white/[0.08]">
              <svg class="w-7 h-7 -rotate-90" viewBox="0 0 32 32">
                <circle
                  cx="16"
                  cy="16"
                  r={radius}
                  fill="none"
                  stroke="rgba(255, 255, 255, 0.1)"
                  stroke-width="2.5"
                />
                <circle
                  cx="16"
                  cy="16"
                  r={radius}
                  fill="none"
                  stroke="#38bdf8"
                  stroke-width="2.5"
                  stroke-linecap="round"
                  stroke-dasharray={circumference}
                  stroke-dashoffset={dlStrokeDashoffset}
                  class="transition-all duration-300 ease-out"
                />
              </svg>
              <span class="absolute text-[8.5px] font-mono font-bold text-sky-400 select-none">
                {dlPercent}
              </span>
            </div>
          {:else}
            <Download class="w-5 h-5 stroke-[1.75] {activeTab === 'downloads' ? 'text-white' : ''}" />
            {#if activeDownloadsCount > 0}
              <span class="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1 rounded-full bg-sky-400 text-slate-950 text-[9px] font-bold font-mono flex items-center justify-center pointer-events-none">
                {activeDownloadsCount}
              </span>
            {/if}
          {/if}
        </button>

        <!-- Floating Tooltip Card for Active Download on Hover -->
        {#if downloadProgress.isDownloading}
          <div
            class="pointer-events-none group-hover/dl:opacity-100 group-hover/dl:translate-x-0 opacity-0 -translate-x-1.5 transition-all duration-150 z-50 absolute left-14 px-3 py-2 bg-[#12161f] border border-white/10 rounded-lg shadow-2xl flex flex-col gap-1 min-w-[190px] max-w-[260px]"
          >
            <div class="flex items-center justify-between text-[11px] font-medium text-white">
              <span>Загрузка игры</span>
              <span class="text-sky-400 font-mono text-[10px]">{dlPercent}%</span>
            </div>
            {#if downloadProgress.title}
              <div class="text-[10px] text-white/80 font-medium truncate">
                {downloadProgress.title}
              </div>
            {/if}
            {#if downloadProgress.speed}
              <div class="text-[10px] text-[#8e95a2] font-mono border-t border-white/5 pt-1 mt-0.5">
                Скорость: {downloadProgress.speed}
              </div>
            {/if}
          </div>
        {/if}
      </div>

      <!-- Settings Tab -->
      <div class="relative w-full flex items-center justify-center">
        {#if activeTab === 'settings'}
          <div class="absolute left-0 top-2 bottom-2 w-[3px] rounded-r bg-white"></div>
        {/if}
        <button
          data-nav-item
          class="w-10 h-10 rounded-xl flex items-center justify-center transition-colors cursor-pointer {activeTab === 'settings' ? 'bg-white/[0.08] text-white border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
          onclick={() => (activeTab = 'settings')}
          title="Настройки"
        >
          <Settings class="w-5 h-5 stroke-[1.75] {activeTab === 'settings' ? 'text-white' : ''}" />
        </button>
      </div>
    </nav>
  </div>

  <!-- Bottom: Metadata Loading Indicator & Gamepad -->
  <div class="flex flex-col items-center gap-2 w-full">
    <div class="w-6 h-px bg-white/[0.06] mb-1"></div>

    <!-- Metadata Loading Indicator (Circle) -->
    {#if metadataProgress.isSyncing || isRefreshing || isCompletedRecently}
      <div class="relative group flex items-center justify-center w-9 h-9">
        <div
          class="relative w-8 h-8 rounded-full flex items-center justify-center bg-[#0d1017] border border-white/[0.08]"
        >
          {#if isCompletedRecently}
            <svg class="w-7 h-7" viewBox="0 0 32 32">
              <circle
                cx="16"
                cy="16"
                r={radius}
                fill="none"
                stroke="#10b981"
                stroke-width="2.5"
              />
            </svg>
            <Check class="w-3.5 h-3.5 text-emerald-400 absolute stroke-[2.5]" />
          {:else if metadataProgress.total > 0}
            <!-- Determinate Radial Progress Ring -->
            <svg class="w-7 h-7 -rotate-90" viewBox="0 0 32 32">
              <circle
                cx="16"
                cy="16"
                r={radius}
                fill="none"
                stroke="rgba(255, 255, 255, 0.1)"
                stroke-width="2.5"
              />
              <circle
                cx="16"
                cy="16"
                r={radius}
                fill="none"
                stroke="#38bdf8"
                stroke-width="2.5"
                stroke-linecap="round"
                stroke-dasharray={circumference}
                stroke-dashoffset={strokeDashoffset}
                class="transition-all duration-300 ease-out"
              />
            </svg>
            <span class="absolute text-[8.5px] font-mono font-bold text-sky-400 select-none">
              {percent}
            </span>
          {:else}
            <!-- Indeterminate Spinning Ring -->
            <svg class="w-7 h-7 animate-spin" viewBox="0 0 32 32">
              <circle
                cx="16"
                cy="16"
                r={radius}
                fill="none"
                stroke="rgba(255, 255, 255, 0.1)"
                stroke-width="2.5"
              />
              <circle
                cx="16"
                cy="16"
                r={radius}
                fill="none"
                stroke="#38bdf8"
                stroke-width="2.5"
                stroke-linecap="round"
                stroke-dasharray="24 45"
              />
            </svg>
          {/if}
        </div>

        <!-- Floating Tooltip Card on Hover -->
        <div
          class="pointer-events-none group-hover:opacity-100 group-hover:translate-x-0 opacity-0 -translate-x-1.5 transition-all duration-150 z-50 absolute left-14 px-3 py-2 bg-[#12161f] border border-white/10 rounded-lg shadow-2xl flex flex-col gap-1 min-w-[190px] max-w-[260px]"
        >
          <div class="flex items-center justify-between text-[11px] font-medium text-white">
            <span>{isCompletedRecently ? 'Метаданные обновлены' : 'Загрузка метаданных'}</span>
            {#if !isCompletedRecently && metadataProgress.total > 0}
              <span class="text-sky-400 font-mono text-[10px]">{percent}%</span>
            {/if}
          </div>
          {#if !isCompletedRecently && metadataProgress.total > 0}
            <div class="text-[10px] text-[#8e95a2]">
              {metadataProgress.current} из {metadataProgress.total} игр
            </div>
          {/if}
          {#if !isCompletedRecently && metadataProgress.currentGame}
            <div class="text-[10px] text-white/70 truncate border-t border-white/5 pt-1 mt-0.5" title={metadataProgress.currentGame}>
              {metadataProgress.currentGame}
            </div>
          {/if}
        </div>
      </div>
    {:else}
      <!-- Discreet idle trigger: hover reveals checkmark circle and lets user click to re-sync -->
      <div class="relative group flex items-center justify-center w-9 h-9">
        <button
          type="button"
          class="w-7 h-7 rounded-full flex items-center justify-center text-white/20 hover:text-white/80 hover:bg-white/[0.04] transition-all cursor-pointer opacity-0 group-hover:opacity-100"
          onclick={onRefresh}
          title="Синхронизация метаданных: всё актуально (нажмите для проверки)"
        >
          <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.75" />
            <path d="m9 12 2 2 4-4" stroke="currentColor" stroke-width="2" />
          </svg>
        </button>

        <div
          class="pointer-events-none group-hover:opacity-100 group-hover:translate-x-0 opacity-0 -translate-x-1.5 transition-all duration-150 z-50 absolute left-14 px-2.5 py-1.5 bg-[#12161f] border border-white/10 rounded-lg shadow-2xl flex flex-col gap-0.5 whitespace-nowrap"
        >
          <span class="text-[11px] font-medium text-white">Метаданные актуальны</span>
          <span class="text-[10px] text-[#8e95a2]">Нажмите для проверки обновлений</span>
        </div>
      </div>
    {/if}

    <!-- Gamepad Indicator -->
    <div
      class="w-9 h-9 rounded-lg flex items-center justify-center transition-colors {isGamepadConnected ? 'text-sky-400 bg-sky-400/10 border border-sky-400/20' : 'text-[#4b5563]'}"
      title={isGamepadConnected ? 'Контроллер подключен' : 'Контроллер не обнаружен'}
    >
      <Gamepad2 class="w-4 h-4 stroke-[1.75]" />
    </div>
  </div>
</aside>
