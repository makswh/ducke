<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Gamepad2,
    Search,
    Monitor,
    X,
    BatteryCharging,
    BatteryMedium,
    Clock
  } from 'lucide-svelte';
  import { sound } from '../../navigation/audio';

  type TabType = 'catalog' | 'torrents' | 'favorites' | 'downloads' | 'settings';

  let {
    activeTab = 'catalog' as TabType,
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

<header data-nav-zone="header" class="h-14 w-full bg-[#07080a] border-b border-white/[0.06] flex items-center justify-between px-6 select-none flex-shrink-0 z-40">
  <!-- Left: Logo, Clock & Controller Status -->
  <div class="flex items-center gap-4">
    <!-- Ducke Logo Badge -->
    <div class="flex items-center gap-2">
      <div class="w-6 h-6 rounded-lg bg-gradient-to-tr from-[#2563eb] to-[#60a5fa] flex items-center justify-center shadow-lg">
        <span class="text-xs font-black text-black leading-none">D</span>
      </div>
      <span class="text-xs font-black tracking-widest text-white uppercase hidden sm:inline">DUCKE</span>
    </div>

    <!-- Clock -->
    <div class="flex items-center gap-1.5 text-xs font-mono font-bold text-[#9ca3af] pl-2 border-l border-white/[0.08]">
      <Clock class="w-3.5 h-3.5 text-[#64748b]" />
      <span>{currentTime}</span>
    </div>

    <!-- Controller Connected Indicator -->
    {#if isGamepadConnected}
      <div class="flex items-center gap-1.5 text-xs text-emerald-400 pl-2 border-l border-white/[0.08]" title="Геймпад подключен">
        <Gamepad2 class="w-4 h-4" />
      </div>
    {/if}
  </div>

  <!-- Center: SteamOS Bumper Tabs -->
  <nav class="flex items-center gap-1.5 sm:gap-2">
    <!-- LB Badge -->
    <span class="px-2 py-0.5 rounded bg-white/[0.06] border border-white/[0.1] text-[10px] font-bold text-[#8e95a2] hidden md:inline">LB</span>

    <!-- Tab 1: Library (only shown if FTP servers are configured) -->
    {#if hasFtpServers}
      <button
        data-nav-item
        class="px-4 py-1.5 rounded-xl text-xs font-bold transition-all cursor-pointer {activeTab === 'catalog' ? 'bg-white/15 text-white shadow-md' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05]'}"
        onclick={() => {
          sound.playTab();
          onTabChange('catalog');
        }}
      >
        Библиотека
      </button>
    {/if}

    <!-- Torrents Tab (Conditional) -->
    {#if hasTorrentSources}
      <button
        data-nav-item
        class="px-4 py-1.5 rounded-xl text-xs font-bold transition-all cursor-pointer {activeTab === 'torrents' ? 'bg-white/15 text-white shadow-md' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05]'}"
        onclick={() => {
          sound.playTab();
          onTabChange('torrents');
        }}
      >
        Торренты
      </button>
    {/if}

    <!-- Tab: Favorites -->
    <button
      data-nav-item
      class="px-4 py-1.5 rounded-xl text-xs font-bold transition-all cursor-pointer {activeTab === 'favorites' ? 'bg-white/15 text-white shadow-md' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05]'}"
      onclick={() => {
        sound.playTab();
        onTabChange('favorites');
      }}
    >
      Избранное
    </button>

    <!-- Tab: Downloads -->
    <button
      data-nav-item
      class="px-4 py-1.5 rounded-xl text-xs font-bold transition-all cursor-pointer flex items-center gap-1.5 {activeTab === 'downloads' ? 'bg-white/15 text-white shadow-md' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05]'}"
      onclick={() => {
        sound.playTab();
        onTabChange('downloads');
      }}
    >
      <span>Загрузки</span>
      {#if activeDownloadsCount > 0}
        <span class="px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-sky-500 text-black font-black">
          {activeDownloadsCount}
        </span>
      {/if}
    </button>

    <!-- Tab 3: Settings -->
    <button
      data-nav-item
      class="px-4 py-1.5 rounded-xl text-xs font-bold transition-all cursor-pointer {activeTab === 'settings' ? 'bg-white/15 text-white shadow-md' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05]'}"
      onclick={() => {
        sound.playTab();
        onTabChange('settings');
      }}
    >
      Настройки
    </button>

    <!-- RB Badge -->
    <span class="px-2 py-0.5 rounded bg-white/[0.06] border border-white/[0.1] text-[10px] font-bold text-[#8e95a2] hidden md:inline">RB</span>
  </nav>

  <!-- Right: Quick Search, Desktop Mode Switcher, Close -->
  <div class="flex items-center gap-2">
    <!-- Quick Search button (X) -->
    <button
      data-nav-item
      class="px-3 py-1.5 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-white text-xs font-semibold flex items-center gap-1.5 cursor-pointer transition-colors"
      onclick={onToggleSearch}
      title="Поиск [X]"
    >
      <Search class="w-3.5 h-3.5 text-sky-400" />
      <span class="hidden sm:inline">Поиск</span>
      <span class="px-1.5 py-0.2 rounded bg-sky-500 text-black text-[9px] font-black">X</span>
    </button>

    <!-- Switch to Desktop Mode -->
    <button
      data-nav-item
      class="px-3 py-1.5 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-[#8e95a2] hover:text-white text-xs font-medium flex items-center gap-1.5 cursor-pointer transition-colors"
      onclick={onSwitchToDesktop}
      title="Переключить в классический вид"
    >
      <Monitor class="w-3.5 h-3.5" />
      <span class="hidden md:inline">Рабочий стол</span>
    </button>

    <!-- Close / Exit -->
    <button
      data-nav-item
      class="p-2 rounded-xl text-[#8e95a2] hover:text-white hover:bg-[#e81123] transition-colors cursor-pointer"
      onclick={onCloseApp}
      title="Закрыть приложение"
    >
      <X class="w-4 h-4" />
    </button>
  </div>
</header>
