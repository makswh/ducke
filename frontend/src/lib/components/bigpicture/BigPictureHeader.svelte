<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Home,
    Gamepad2,
    Search,
    Monitor,
    X,
    Compass,
    Magnet,
    Heart,
    Download,
    Settings,
    Clock
  } from 'lucide-svelte';
  import { sound } from '../../navigation/audio';

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

  function updateTime() {
    const now = new Date();
    currentTime = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  onMount(() => {
    updateTime();
    timeInterval = setInterval(updateTime, 1000);
  });

  onDestroy(() => {
    if (timeInterval) clearInterval(timeInterval);
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
      <img src="/appicon.png" alt="Ducke" class="w-6 h-6 rounded-md object-cover border border-white/10 shadow-sm" />
      <span class="text-xs font-black tracking-widest text-white uppercase hidden md:inline">DUCKE</span>
    </button>

    <!-- Clock -->
    <div class="flex items-center gap-1.5 text-xs font-mono font-bold text-[#8e95a2] pl-2 border-l border-white/[0.08]">
      <Clock class="w-3.5 h-3.5 text-[#64748b]" />
      <span>{currentTime}</span>
    </div>

    <!-- Controller Connected Indicator -->
    {#if isGamepadConnected}
      <div class="hidden sm:flex items-center gap-1.5 text-xs text-emerald-400 pl-2 border-l border-white/[0.08]" title="Геймпад подключен">
        <Gamepad2 class="w-3.5 h-3.5" />
        <span class="text-[11px] font-medium hidden lg:inline">Геймпад</span>
      </div>
    {/if}
  </div>

  <!-- Center: Console System Tray (Top Tray) -->
  <nav class="flex items-center gap-1 sm:gap-1.5">
    <!-- LB Badge -->
    <span class="px-1.5 py-0.5 rounded bg-white/[0.04] border border-white/[0.08] text-[9px] font-bold text-[#64748b] hidden xl:inline">LB</span>

    <!-- Tab 1: Home (Multi-tier shelves) -->
    <button
      data-nav-item
      type="button"
      class="px-2.5 sm:px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'home' ? 'bg-white/15 text-white border border-white/20 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05] border border-transparent'}"
      onclick={() => {
        sound.playTab();
        onTabChange('home');
      }}
    >
      <Home class="w-3.5 h-3.5 {activeTab === 'home' ? 'text-sky-400' : 'text-[#64748b]'}" />
      <span class="hidden sm:inline">Главная</span>
    </button>

    <!-- Tab 2: Catalog (if FTP servers are configured) -->
    {#if hasFtpServers}
      <button
        data-nav-item
        type="button"
        class="px-2.5 sm:px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'catalog' ? 'bg-white/15 text-white border border-white/20 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05] border border-transparent'}"
        onclick={() => {
          sound.playTab();
          onTabChange('catalog');
        }}
      >
        <Compass class="w-3.5 h-3.5 {activeTab === 'catalog' ? 'text-sky-400' : 'text-[#64748b]'}" />
        <span class="hidden sm:inline">Каталог</span>
      </button>
    {/if}

    <!-- Tab 3: Torrents Tab (Conditional) -->
    {#if hasTorrentSources}
      <button
        data-nav-item
        type="button"
        class="px-2.5 sm:px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'torrents' ? 'bg-white/15 text-white border border-white/20 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05] border border-transparent'}"
        onclick={() => {
          sound.playTab();
          onTabChange('torrents');
        }}
      >
        <Magnet class="w-3.5 h-3.5 {activeTab === 'torrents' ? 'text-sky-400' : 'text-[#64748b]'}" />
        <span class="hidden sm:inline">Торренты</span>
      </button>
    {/if}

    <!-- Tab 4: Favorites -->
    <button
      data-nav-item
      type="button"
      class="px-2.5 sm:px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'favorites' ? 'bg-white/15 text-white border border-white/20 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05] border border-transparent'}"
      onclick={() => {
        sound.playTab();
        onTabChange('favorites');
      }}
    >
      <Heart class="w-3.5 h-3.5 {activeTab === 'favorites' ? 'text-rose-400 fill-rose-400/20' : 'text-[#64748b]'}" />
      <span class="hidden sm:inline">Избранное</span>
    </button>

    <!-- Tab 5: Downloads -->
    <button
      data-nav-item
      type="button"
      class="px-2.5 sm:px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'downloads' ? 'bg-white/15 text-white border border-white/20 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05] border border-transparent'}"
      onclick={() => {
        sound.playTab();
        onTabChange('downloads');
      }}
    >
      <Download class="w-3.5 h-3.5 {activeTab === 'downloads' ? 'text-sky-400' : 'text-[#64748b]'}" />
      <span class="hidden sm:inline">Загрузки</span>
      {#if activeDownloadsCount > 0}
        <span class="px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-sky-500 text-black font-black">
          {activeDownloadsCount}
        </span>
      {/if}
    </button>

    <!-- Tab 6: Settings -->
    <button
      data-nav-item
      type="button"
      class="px-2.5 sm:px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'settings' ? 'bg-white/15 text-white border border-white/20 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05] border border-transparent'}"
      onclick={() => {
        sound.playTab();
        onTabChange('settings');
      }}
    >
      <Settings class="w-3.5 h-3.5 {activeTab === 'settings' ? 'text-sky-400' : 'text-[#64748b]'}" />
      <span class="hidden sm:inline">Настройки</span>
    </button>

    <!-- RB Badge -->
    <span class="px-1.5 py-0.5 rounded bg-white/[0.04] border border-white/[0.08] text-[9px] font-bold text-[#64748b] hidden xl:inline">RB</span>
  </nav>

  <!-- Right: Quick Search, Desktop Mode Switcher, Close -->
  <div class="flex items-center gap-1.5 sm:gap-2">
    <!-- Quick Search button (X) -->
    <button
      data-nav-item
      type="button"
      class="px-2.5 sm:px-3 py-1.5 rounded-lg bg-white/[0.05] hover:bg-white/[0.1] text-white text-xs font-semibold flex items-center gap-1.5 cursor-pointer transition-colors border border-white/[0.06]"
      onclick={() => onToggleSearch()}
      title="Поиск [X]"
    >
      <Search class="w-3.5 h-3.5 text-sky-400" />
      <span class="hidden sm:inline">Поиск</span>
      <span class="px-1 py-0.2 rounded bg-sky-500 text-black text-[9px] font-black leading-none">X</span>
    </button>

    <!-- Switch to Desktop Mode -->
    <button
      data-nav-item
      type="button"
      class="px-2.5 sm:px-3 py-1.5 rounded-lg bg-white/[0.05] hover:bg-white/[0.1] text-[#8e95a2] hover:text-white text-xs font-medium flex items-center gap-1.5 cursor-pointer transition-colors border border-white/[0.06]"
      onclick={() => onSwitchToDesktop()}
      title="Переключить в режим рабочего стола"
    >
      <Monitor class="w-3.5 h-3.5" />
      <span class="hidden lg:inline">Рабочий стол</span>
    </button>

    <!-- Close / Exit -->
    <button
      data-nav-item
      type="button"
      class="p-2 rounded-lg text-[#8e95a2] hover:text-white hover:bg-[#e81123] transition-colors cursor-pointer border border-transparent hover:border-red-500/20"
      onclick={() => onCloseApp()}
      title="Закрыть приложение"
    >
      <X class="w-4 h-4" />
    </button>
  </div>
</header>
