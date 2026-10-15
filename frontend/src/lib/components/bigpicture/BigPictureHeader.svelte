<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    House,
    GameController,
    MagnifyingGlass,
    Monitor,
    X,
    Compass,
    Magnet,
    BookmarkSimple,
    DownloadSimple,
    Gear,
    Clock,
    ArrowCircleUp
  } from 'phosphor-svelte';
  import { sound } from '../../navigation/audio';
  import { EventsOn } from '../../../../wailsjs/runtime/runtime';

  type TabType = 'home' | 'catalog' | 'torrents' | 'collections' | 'favorites' | 'downloads' | 'settings';

  let {
    activeTab = 'home' as TabType,
    onTabChange = (tab: TabType) => {},
    hasFtpServers = false,
    hasTorrentSources = false,
    activeDownloadsCount = 0,
    isGamepadConnected = false,
    onToggleSearch = () => {},
    onSwitchToDesktop = () => {},
    onCloseApp = () => {}
  } = $props();

  let currentTime = $state<string>('');
  let timeInterval: any;
  let hasUpdateAvailable = $state<boolean>(false);
  let unsubUpdate: any;

  function updateTime() {
    const now = new Date();
    currentTime = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  onMount(() => {
    updateTime();
    timeInterval = setInterval(updateTime, 1000);

    unsubUpdate = EventsOn('update:available', (info: any) => {
      if (info && info.hasUpdate) {
        hasUpdateAvailable = true;
      }
    });
  });

  onDestroy(() => {
    if (timeInterval) clearInterval(timeInterval);
    if (typeof unsubUpdate === 'function') unsubUpdate();
  });
</script>

<header data-nav-zone="header" class="h-14 sm:h-16 w-full bg-[#07080a] border-b border-white/[0.06] flex items-center justify-between px-4 sm:px-6 select-none flex-shrink-0 z-40">
  <!-- Left: Logo, Clock & Controller Status -->
  <div class="flex items-center gap-3 sm:gap-4 min-w-0">
    <!-- Ducke Logo Badge -->
    <button
      data-nav-item
      type="button"
      class="flex items-center gap-2 cursor-pointer focus:outline-none"
      onclick={() => {
        sound.playTab();
        onTabChange('home');
      }}
      title="На главную"
    >
      <div class="w-7 h-7 rounded-[10px] bg-white/[0.04] flex items-center justify-center overflow-hidden border border-white/10">
        <img src="/appicon.png" alt="Ducke" class="w-5 h-5 object-contain" />
      </div>
      <span class="text-xs font-black tracking-widest text-white uppercase hidden md:inline">DUCKE</span>
    </button>

    <!-- Clock -->
    <div class="flex items-center gap-1.5 text-xs font-mono font-bold text-[#8e95a2] pl-2 border-l border-white/[0.08]">
      <Clock size={14} class="text-[#64748b]" />
      <span>{currentTime}</span>
    </div>

    <!-- Controller Connected Indicator -->
    {#if isGamepadConnected}
      <div class="hidden sm:flex items-center gap-1.5 text-xs text-emerald-400 pl-2 border-l border-white/[0.08]" title="Геймпад подключен">
        <GameController size={14} />
        <span class="text-[11px] font-medium hidden lg:inline">Геймпад</span>
      </div>
    {/if}
  </div>

  <!-- Center: Console System Tray (Top Tray) -->
  <nav class="flex items-center gap-1 sm:gap-1.5">
    <!-- LB Badge -->
    <span class="px-1.5 py-0.5 rounded-[6px] bg-white/[0.04] border border-white/[0.08] text-[9px] font-mono font-bold text-[#8e95a2] hidden xl:inline">LB</span>

    <!-- Tab 1: Home (Multi-tier shelves) -->
    <button
      data-nav-item
      type="button"
      class="px-3 sm:px-3.5 py-2 rounded-[12px] text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'home' ? 'bg-white/[0.08] text-white border border-white/10 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
      onclick={() => {
        sound.playTab();
        onTabChange('home');
      }}
    >
      <House size={15} class={activeTab === 'home' ? 'text-[#4C7DFF]' : 'text-[#64748b]'} />
      <span class="hidden sm:inline">Главная</span>
    </button>

    <!-- Tab 2: Catalog (if FTP servers are configured) -->
    {#if hasFtpServers}
      <button
        data-nav-item
        type="button"
        class="px-3 sm:px-3.5 py-2 rounded-[12px] text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'catalog' ? 'bg-white/[0.08] text-white border border-white/10 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
        onclick={() => {
          sound.playTab();
          onTabChange('catalog');
        }}
      >
        <Compass size={15} class={activeTab === 'catalog' ? 'text-[#4C7DFF]' : 'text-[#64748b]'} />
        <span class="hidden sm:inline">Каталог</span>
      </button>
    {/if}

    <!-- Tab 3: Torrents Tab (Conditional) -->
    {#if hasTorrentSources}
      <button
        data-nav-item
        type="button"
        class="px-3 sm:px-3.5 py-2 rounded-[12px] text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'torrents' ? 'bg-white/[0.08] text-white border border-white/10 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
        onclick={() => {
          sound.playTab();
          onTabChange('torrents');
        }}
      >
        <Magnet size={15} class={activeTab === 'torrents' ? 'text-[#4C7DFF]' : 'text-[#64748b]'} />
        <span class="hidden sm:inline">Торренты</span>
      </button>
    {/if}

    <!-- Tab 4: Favorites -->
    <button
      data-nav-item
      type="button"
      class="px-3 sm:px-3.5 py-2 rounded-[12px] text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'favorites' ? 'bg-white/[0.08] text-white border border-white/10 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
      onclick={() => {
        sound.playTab();
        onTabChange('favorites');
      }}
    >
      <BookmarkSimple size={15} weight={activeTab === 'favorites' ? 'fill' : 'regular'} class={activeTab === 'favorites' ? 'text-amber-400' : 'text-[#64748b]'} />
      <span class="hidden sm:inline">Избранное</span>
    </button>

    <!-- Tab 5: Downloads -->
    <button
      data-nav-item
      type="button"
      class="px-3 sm:px-3.5 py-2 rounded-[12px] text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'downloads' ? 'bg-white/[0.08] text-white border border-white/10 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
      onclick={() => {
        sound.playTab();
        onTabChange('downloads');
      }}
    >
      <DownloadSimple size={15} class={activeTab === 'downloads' ? 'text-[#4C7DFF]' : 'text-[#64748b]'} />
      <span class="hidden sm:inline">Загрузки</span>
      {#if activeDownloadsCount > 0}
        <span class="px-1.5 py-0.5 rounded-[6px] text-[10px] font-mono bg-[#2563eb] text-white font-bold">
          {activeDownloadsCount}
        </span>
      {/if}
    </button>

    <!-- Tab 6: Settings -->
    <button
      data-nav-item
      type="button"
      class="px-3 sm:px-3.5 py-2 rounded-[12px] text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'settings' ? 'bg-white/[0.08] text-white border border-white/10 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
      onclick={() => {
        sound.playTab();
        onTabChange('settings');
      }}
    >
      <Gear size={15} class={activeTab === 'settings' ? 'text-[#4C7DFF]' : 'text-[#64748b]'} />
      <span class="hidden sm:inline">Настройки</span>
      {#if hasUpdateAvailable}
        <span class="w-2 h-2 rounded-full bg-[#4C7DFF] animate-pulse" title="Доступно обновление"></span>
      {/if}
    </button>

    <!-- RB Badge -->
    <span class="px-1.5 py-0.5 rounded-[6px] bg-white/[0.04] border border-white/[0.08] text-[9px] font-mono font-bold text-[#8e95a2] hidden xl:inline">RB</span>
  </nav>

  <!-- Right: Quick Search, Desktop Mode Switcher, Close -->
  <div class="flex items-center gap-1.5 sm:gap-2">
    <!-- Quick Search button (X) -->
    <button
      data-nav-item
      type="button"
      class="px-3 sm:px-3.5 py-2 rounded-[12px] bg-white/[0.04] hover:bg-white/[0.08] text-white text-xs font-semibold flex items-center gap-1.5 cursor-pointer transition-colors border border-white/[0.06]"
      onclick={() => onToggleSearch()}
      title="Поиск [X]"
    >
      <MagnifyingGlass size={15} class="text-[#4C7DFF]" />
      <span class="hidden sm:inline">Поиск</span>
      <span class="w-4 h-4 rounded-full bg-sky-400 text-black text-[10px] font-black flex items-center justify-center leading-none">X</span>
    </button>

    <!-- Switch to Desktop Mode -->
    <button
      data-nav-item
      type="button"
      class="px-3 sm:px-3.5 py-2 rounded-[12px] bg-white/[0.04] hover:bg-white/[0.08] text-[#8e95a2] hover:text-white text-xs font-medium flex items-center gap-1.5 cursor-pointer transition-colors border border-white/[0.06]"
      onclick={() => onSwitchToDesktop()}
      title="Переключить в режим рабочего стола"
    >
      <Monitor size={15} />
      <span class="hidden lg:inline">Рабочий стол</span>
    </button>

    <!-- Close / Exit -->
    <button
      data-nav-item
      type="button"
      class="p-2 rounded-[8px] text-[#8e95a2] hover:text-white hover:bg-[#e81123] transition-colors cursor-pointer border border-transparent hover:border-red-500/20"
      onclick={() => onCloseApp()}
      title="Закрыть приложение"
    >
      <X size={15} weight="bold" />
    </button>
  </div>
</header>

