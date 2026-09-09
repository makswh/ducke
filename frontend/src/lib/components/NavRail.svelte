<script lang="ts">
  import {
    Library,
    Download,
    Settings,
    Server,
    Gamepad2,
    RefreshCw,
    HardDrive,
    Layers,
    Tv
  } from 'lucide-svelte';

  let {
    activeTab = $bindable<'catalog' | 'downloads' | 'settings'>('catalog'),
    activeDownloadsCount = 0,
    isGamepadConnected = false,
    isConnected = false,
    serverName = '',
    onRefresh = () => {},
    isRefreshing = false,
    onToggleBigPicture = () => {}
  } = $props();
</script>

<aside data-nav-zone="sidebar" class="w-[60px] flex flex-col items-center justify-between py-4 flex-shrink-0 z-30 select-none bg-[#08090d] border-r border-white/[0.06]">
  <!-- Navigation Tabs (Vertical Icons) -->
  <div class="flex flex-col items-center gap-2 w-full">
    <nav class="flex flex-col items-center gap-2.5 w-full">
      <!-- Catalog Tab -->
      <div class="relative w-full flex items-center justify-center">
        {#if activeTab === 'catalog'}
          <div class="absolute left-0 top-2 bottom-2 w-[3px] rounded-r bg-white"></div>
        {/if}
        <button
          data-nav-item
          class="w-10 h-10 rounded-xl flex items-center justify-center transition-colors cursor-pointer {activeTab === 'catalog' ? 'bg-white/[0.08] text-white border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
          onclick={() => (activeTab = 'catalog')}
          title="Каталог игр"
        >
          <Layers class="w-5 h-5 stroke-[1.75] {activeTab === 'catalog' ? 'text-white' : ''}" />
        </button>
      </div>

      <!-- Downloads Tab -->
      <div class="relative w-full flex items-center justify-center">
        {#if activeTab === 'downloads'}
          <div class="absolute left-0 top-2 bottom-2 w-[3px] rounded-r bg-white"></div>
        {/if}
        <button
          data-nav-item
          class="relative w-10 h-10 rounded-xl flex items-center justify-center transition-colors cursor-pointer {activeTab === 'downloads' ? 'bg-white/[0.08] text-white border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
          onclick={() => (activeTab = 'downloads')}
          title="Загрузки"
        >
          <Download class="w-5 h-5 stroke-[1.75] {activeTab === 'downloads' ? 'text-white' : ''}" />
          {#if activeDownloadsCount > 0}
            <span class="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1 rounded-full bg-sky-400 text-slate-950 text-[9px] font-bold font-mono flex items-center justify-center pointer-events-none">
              {activeDownloadsCount}
            </span>
          {/if}
        </button>
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

  <!-- Bottom: Refresh, Gamepad & Server Connection Status -->
  <div class="flex flex-col items-center gap-2 w-full">
    <div class="w-6 h-px bg-white/[0.06] mb-1"></div>

    <!-- Refresh Button -->
    <button
      data-nav-item
      class="w-9 h-9 rounded-lg flex items-center justify-center text-[#8e95a2] hover:text-white hover:bg-white/[0.06] transition-colors cursor-pointer disabled:opacity-40"
      onclick={onRefresh}
      disabled={isRefreshing}
      title="Обновить каталог"
    >
      <RefreshCw class="w-4 h-4 stroke-[1.75] {isRefreshing ? 'animate-spin text-white' : ''}" />
    </button>

    <!-- Gamepad Indicator -->
    <div
      class="w-9 h-9 rounded-lg flex items-center justify-center transition-colors {isGamepadConnected ? 'text-sky-400 bg-sky-400/10 border border-sky-400/20' : 'text-[#4b5563]'}"
      title={isGamepadConnected ? 'Контроллер подключен' : 'Контроллер не обнаружен'}
    >
      <Gamepad2 class="w-4 h-4 stroke-[1.75]" />
    </div>

    <!-- Server Connection Dot -->
    <div
      class="w-9 h-9 rounded-lg flex items-center justify-center hover:bg-white/[0.04] transition-colors cursor-default"
      title={serverName ? `Сервер: ${serverName}` : isConnected ? 'Сервер подключен' : 'Сервер не подключен'}
    >
      <span class="w-2 h-2 rounded-full {isConnected ? 'bg-emerald-400' : 'bg-[#475569]'}"></span>
    </div>
  </div>
</aside>
