<script lang="ts">
  import BigPictureHeader from './BigPictureHeader.svelte';
  import BigPictureHomeShelves from './BigPictureHomeShelves.svelte';
  import BigPictureLibrary from './BigPictureLibrary.svelte';
  import BigPictureGameDetail from './BigPictureGameDetail.svelte';
  import BigPictureFavorites from './BigPictureFavorites.svelte';
  import BigPictureDownloads from './BigPictureDownloads.svelte';
  import BigPictureSettings from './BigPictureSettings.svelte';
  import { sound } from '../../navigation/audio';

  type TabType = 'home' | 'catalog' | 'torrents' | 'collections' | 'favorites' | 'downloads' | 'settings';

  let {
    activeTab = $bindable<TabType>('home'),
    games = [] as any[],
    torrentGames = [] as any[],
    hasFtpServers = false,
    hasTorrentSources = false,
    activeDownloads = [] as any[],
    downloadHistory = [] as any[],
    settings = null as any,
    isGamepadConnected = false,
    isCatalogLoading = false,
    isTorrentsLoading = false,
    onStartDownload = (gameId: number, targetPath: string) => {},
    onPauseDownload = (id: string) => {},
    onResumeDownload = (id: string) => {},
    onCancelDownload = (id: string) => {},
    onPauseAll = () => {},
    onResumeAll = () => {},
    onClearCompleted = () => {},
    onDeleteRecord = (id: string, removeFiles: boolean) => {},
    onSaveSettings = (s: any) => {},
    onImportXML = (xml: string) => {},
    onImportFile = async () => {},
    onTestConnection = async (srv: any) => ({ success: false }),
    onSelectFolder = async () => '',
    onUpdateDownloadPath = (path: string) => {},
    onOpenFolder = (path: string) => {},
    onOpenConfigFolder = async () => {},
    onClearMetadataCache = async (): Promise<number> => 0,
    onSwitchToDesktop = () => {},
    onCloseApp = () => {}
  } = $props();

  import { gamepad } from '../../navigation/gamepad';
  import { onMount, onDestroy } from 'svelte';

  let selectedGame = $state<any | null>(null);
  let isSearchOpen = $state<boolean>(false);
  let searchQuery = $state<string>('');
  let isTheaterActive = $state<boolean>(false);

  $effect(() => {
    if (!selectedGame && isTheaterActive) {
      isTheaterActive = false;
    }
  });

  function handleSelectGame(game: any) {
    selectedGame = game;
  }

  function handleBackToLibrary() {
    selectedGame = null;
    isTheaterActive = false;
    gamepad.retryFocusZone('grid');
  }

  onMount(() => {
    const handleGoBack = (e: CustomEvent) => {
      if (e.defaultPrevented) return;

      if (isSearchOpen) {
        isSearchOpen = false;
        e.preventDefault();
        return;
      }
      if (selectedGame) {
        handleBackToLibrary();
        e.preventDefault();
        return;
      }
      if (activeTab !== 'home') {
        activeTab = 'home';
        e.preventDefault();
        gamepad.retryFocusZone('grid');
        return;
      }
    };

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.defaultPrevented) return;
      const target = e.target as HTMLElement | null;
      const isInput = target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA');
      if (isInput) return;

      if (e.key === 'Escape' || e.key === 'Backspace') {
        const ev = new CustomEvent('app:go-back', { cancelable: true });
        window.dispatchEvent(ev);
        e.preventDefault();
      }
    };

    const handleToggleMenu = () => {
      if (activeTab === 'settings') {
        activeTab = 'home';
      } else {
        selectedGame = null;
        isSearchOpen = false;
        activeTab = 'settings';
      }
      gamepad.retryFocusZone('list');
    };

    const handleToggleSearch = () => {
      if (activeTab === 'home') {
        activeTab = hasFtpServers ? 'catalog' : (hasTorrentSources ? 'torrents' : 'catalog');
      } else if ((!hasFtpServers || activeTab !== 'catalog') && activeTab !== 'torrents') {
        activeTab = hasFtpServers ? 'catalog' : 'torrents';
      }
      selectedGame = null;
      isSearchOpen = !isSearchOpen;
    };

    const handleTheaterMode = (e: CustomEvent) => {
      isTheaterActive = !!e.detail?.active;
    };

    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('app:go-back', handleGoBack as EventListener);
    window.addEventListener('app:toggle-menu', handleToggleMenu);
    window.addEventListener('app:toggle-search', handleToggleSearch);
    window.addEventListener('app:theater-mode', handleTheaterMode as EventListener);

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('app:go-back', handleGoBack as EventListener);
      window.removeEventListener('app:toggle-menu', handleToggleMenu);
      window.removeEventListener('app:toggle-search', handleToggleSearch);
      window.removeEventListener('app:theater-mode', handleTheaterMode as EventListener);
    };
  });
</script>

<div class="h-screen w-screen flex flex-col overflow-hidden bg-[#07080a] text-white selection:bg-[#38bdf8] selection:text-black font-sans-ui select-none">
  
  <!-- SteamOS Top Header -->
  {#if !isTheaterActive}
    <BigPictureHeader
      activeTab={activeTab as TabType}
      {hasFtpServers}
      {hasTorrentSources}
      onTabChange={(t: TabType) => {
        activeTab = t;
        selectedGame = null;
      }}
      activeDownloadsCount={(activeDownloads || []).filter((d) => d && (d.status === 'downloading' || d.status === 'queued')).length}
      {isGamepadConnected}
      onToggleSearch={() => {
        if ((!hasFtpServers || activeTab !== 'catalog') && activeTab !== 'torrents') {
          activeTab = hasFtpServers ? 'catalog' : 'torrents';
        }
        selectedGame = null;
        isSearchOpen = !isSearchOpen;
      }}
      {onSwitchToDesktop}
      {onCloseApp}
    />
  {/if}

  <!-- Main Stage -->
  <div class="flex-1 flex overflow-hidden min-h-0 relative">
    {#if selectedGame}
      <BigPictureGameDetail
        game={selectedGame}
        downloadPath={settings?.downloadPath || 'C:\\Ducke'}
        onBack={handleBackToLibrary}
        {onStartDownload}
        {onSelectFolder}
        {onUpdateDownloadPath}
        {activeDownloads}
      />
    {:else if activeTab === 'home'}
      <BigPictureHomeShelves
        {games}
        {torrentGames}
        {activeDownloads}
        {hasFtpServers}
        {hasTorrentSources}
        {downloadHistory}
        downloadPath={settings?.downloadPath || 'C:\\Ducke'}
        onSelectGame={handleSelectGame}
        {onStartDownload}
        onGoToCatalog={() => {
          activeTab = 'catalog';
          selectedGame = null;
        }}
        onGoToTorrents={() => {
          activeTab = 'torrents';
          selectedGame = null;
        }}
        onGoToFavorites={() => {
          activeTab = 'favorites';
          selectedGame = null;
        }}
        onGoToDownloads={() => {
          activeTab = 'downloads';
          selectedGame = null;
        }}
      />
    {:else if activeTab === 'catalog'}
      <BigPictureLibrary
        {games}
        isLoading={isCatalogLoading}
        {activeDownloads}
        bind:searchQuery
        onSelectGame={handleSelectGame}
        {isSearchOpen}
        onCloseSearch={() => (isSearchOpen = false)}
      />
    {:else if activeTab === 'torrents'}
      <BigPictureLibrary
        games={torrentGames}
        isLoading={isTorrentsLoading}
        {activeDownloads}
        bind:searchQuery
        onSelectGame={handleSelectGame}
        {isSearchOpen}
        onCloseSearch={() => (isSearchOpen = false)}
      />
    {:else if activeTab === 'favorites'}
      <BigPictureFavorites
        {games}
        {torrentGames}
        {activeDownloads}
        onSelectGame={handleSelectGame}
        onGoToCatalog={() => {
          activeTab = hasFtpServers ? 'catalog' : (hasTorrentSources ? 'torrents' : 'home');
          selectedGame = null;
        }}
      />
    {:else if activeTab === 'downloads'}
      <BigPictureDownloads
        {games}
        {activeDownloads}
        {downloadHistory}
        downloadPath={settings?.downloadPath || ''}
        onPause={onPauseDownload}
        onResume={onResumeDownload}
        onCancel={onCancelDownload}
        {onPauseAll}
        {onResumeAll}
        {onClearCompleted}
        {onDeleteRecord}
        {onOpenFolder}
        onGoToCatalog={() => {
          activeTab = hasFtpServers ? 'catalog' : (hasTorrentSources ? 'torrents' : 'home');
          selectedGame = null;
        }}
      />
    {:else if activeTab === 'settings'}
      <BigPictureSettings
        {settings}
        {onSaveSettings}
        {onImportFile}
        {onTestConnection}
        {onSelectFolder}
        {onOpenFolder}
        {onOpenConfigFolder}
        {onClearMetadataCache}
        {onSwitchToDesktop}
        {onCloseApp}
      />
    {/if}
  </div>

  <!-- SteamOS Gamepad Footer HUD -->
  <footer class="h-10 px-8 flex items-center justify-between bg-[#050608] border-t border-white/[0.06] text-[#8e95a2] text-xs font-medium z-30 select-none flex-shrink-0">
    <div class="flex items-center gap-5 sm:gap-6 flex-wrap">
      {#if isTheaterActive}
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">A</span>
          <span>Пауза</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">B</span>
          <span>Выход</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">X</span>
          <span>Звук</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-bold text-[10px] border border-white/20">LB / RB</span>
          <span>Трейлеры</span>
        </div>
      {:else if selectedGame}
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">A</span>
          <span>Скачать / Играть</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">B</span>
          <span>Назад</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">X</span>
          <span>Версии / Папка</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">Y</span>
          <span>В избранное</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-bold text-[10px] border border-white/20">LB / RB</span>
          <span>Разделы</span>
        </div>
      {:else if activeTab === 'home'}
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">A</span>
          <span>Подробнее</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">B</span>
          <span>Назад</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">X</span>
          <span>Скачать / Играть</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">Y</span>
          <span>Избранное</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-bold text-[10px] border border-white/20">LB / RB</span>
          <span>Вкладки</span>
        </div>
      {:else if (hasFtpServers && activeTab === 'catalog') || activeTab === 'torrents'}
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">A</span>
          <span>Открыть</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">B</span>
          <span>Главная</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">X</span>
          <span>Поиск</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">Y</span>
          <span>Сортировка</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-bold text-[10px] border border-white/20">LB / RB</span>
          <span>Вкладки</span>
        </div>
      {:else if activeTab === 'favorites'}
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">A</span>
          <span>Открыть</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">B</span>
          <span>Главная</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">Y</span>
          <span>Сменить статус</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-bold text-[10px] border border-white/20">LB / RB</span>
          <span>Вкладки</span>
        </div>
      {:else if activeTab === 'downloads'}
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">A</span>
          <span>Действие</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">B</span>
          <span>Главная</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-bold text-[10px] border border-white/20">LB / RB</span>
          <span>Вкладки</span>
        </div>
      {:else if activeTab === 'settings'}
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">A</span>
          <span>Выбрать</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">B</span>
          <span>Главная</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-bold text-[10px] border border-white/20">← →</span>
          <span>Сайдбар / Настройки</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-bold text-[10px] border border-white/20">LB / RB</span>
          <span>Вкладки</span>
        </div>
      {/if}
    </div>
    
    <div class="flex items-center gap-4 text-[11px] text-[#64748b]">
      <span>Ducke Console</span>
      {#if isGamepadConnected}
        <span class="inline-flex items-center gap-1.5 text-emerald-400 font-semibold">
          <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
          Геймпад подключен
        </span>
      {/if}
    </div>
  </footer>
</div>
