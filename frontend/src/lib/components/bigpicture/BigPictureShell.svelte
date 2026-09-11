<script lang="ts">
  import BigPictureHeader from './BigPictureHeader.svelte';
  import BigPictureLibrary from './BigPictureLibrary.svelte';
  import BigPictureGameDetail from './BigPictureGameDetail.svelte';
  import BigPictureFavorites from './BigPictureFavorites.svelte';
  import BigPictureDownloads from './BigPictureDownloads.svelte';
  import BigPictureSettings from './BigPictureSettings.svelte';
  import { sound } from '../../navigation/audio';

  type TabType = 'catalog' | 'torrents' | 'favorites' | 'downloads' | 'settings';

  let {
    activeTab = $bindable<TabType>('catalog'),
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

  function handleSelectGame(game: any) {
    selectedGame = game;
  }

  function handleBackToLibrary() {
    selectedGame = null;
    setTimeout(() => {
      gamepad.focusFirstInZone('grid');
    }, 60);
  }

  onMount(() => {
    const handleGoBack = (e: CustomEvent) => {
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
      const defaultTab = hasFtpServers ? 'catalog' : (hasTorrentSources ? 'torrents' : 'settings');
      if (activeTab !== defaultTab && activeTab !== 'torrents' && activeTab !== 'favorites') {
        activeTab = defaultTab;
        e.preventDefault();
        setTimeout(() => {
          gamepad.focusFirstInZone('grid');
        }, 60);
        return;
      }
    };

    const handleToggleMenu = () => {
      const defaultTab = hasFtpServers ? 'catalog' : (hasTorrentSources ? 'torrents' : 'settings');
      if (activeTab === 'settings') {
        activeTab = defaultTab;
      } else {
        selectedGame = null;
        isSearchOpen = false;
        activeTab = 'settings';
      }
      setTimeout(() => {
        gamepad.focusFirstInZone('list') || gamepad.focusFirstInZone('grid');
      }, 60);
    };

    const handleToggleSearch = () => {
      if ((!hasFtpServers || activeTab !== 'catalog') && activeTab !== 'torrents') {
        activeTab = hasFtpServers ? 'catalog' : 'torrents';
      }
      selectedGame = null;
      isSearchOpen = !isSearchOpen;
    };

    window.addEventListener('app:go-back', handleGoBack as EventListener);
    window.addEventListener('app:toggle-menu', handleToggleMenu);
    window.addEventListener('app:toggle-search', handleToggleSearch);

    return () => {
      window.removeEventListener('app:go-back', handleGoBack as EventListener);
      window.removeEventListener('app:toggle-menu', handleToggleMenu);
      window.removeEventListener('app:toggle-search', handleToggleSearch);
    };
  });
</script>

<div class="h-screen w-screen flex flex-col overflow-hidden bg-[#07080a] text-white selection:bg-[#38bdf8] selection:text-black font-sans-ui select-none">
  
  <!-- SteamOS Top Header -->
  <BigPictureHeader
    {activeTab}
    {hasFtpServers}
    {hasTorrentSources}
    onTabChange={(t) => {
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

  <!-- Main Stage -->
  <div class="flex-1 flex overflow-hidden min-h-0 relative">
    {#if activeTab === 'catalog'}
      <div class="w-full h-full flex flex-col {selectedGame ? 'hidden' : ''}">
        <BigPictureLibrary
          {games}
          isLoading={isCatalogLoading}
          {activeDownloads}
          bind:searchQuery
          onSelectGame={handleSelectGame}
          {isSearchOpen}
          onCloseSearch={() => (isSearchOpen = false)}
        />
      </div>
      {#if selectedGame}
        <BigPictureGameDetail
          game={selectedGame}
          downloadPath={settings?.downloadPath || 'C:\\Ducke'}
          onBack={handleBackToLibrary}
          {onStartDownload}
          {onSelectFolder}
          {activeDownloads}
        />
      {/if}
    {:else if activeTab === 'torrents'}
      <div class="w-full h-full flex flex-col {selectedGame ? 'hidden' : ''}">
        <BigPictureLibrary
          games={torrentGames}
          isLoading={isTorrentsLoading}
          {activeDownloads}
          bind:searchQuery
          onSelectGame={handleSelectGame}
          {isSearchOpen}
          onCloseSearch={() => (isSearchOpen = false)}
        />
      </div>
      {#if selectedGame}
        <BigPictureGameDetail
          game={selectedGame}
          downloadPath={settings?.downloadPath || 'C:\\Ducke'}
          onBack={handleBackToLibrary}
          {onStartDownload}
          {onSelectFolder}
          {activeDownloads}
        />
      {/if}
    {:else if activeTab === 'favorites'}
      <div class="w-full h-full flex flex-col {selectedGame ? 'hidden' : ''}">
        <BigPictureFavorites
          onSelectGame={handleSelectGame}
          onExploreCatalog={() => {
            activeTab = hasFtpServers ? 'catalog' : 'torrents';
            selectedGame = null;
          }}
        />
      </div>
      {#if selectedGame}
        <BigPictureGameDetail
          game={selectedGame}
          downloadPath={settings?.downloadPath || 'C:\\Ducke'}
          onBack={handleBackToLibrary}
          {onStartDownload}
          {onSelectFolder}
          {activeDownloads}
        />
      {/if}
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
          activeTab = hasFtpServers ? 'catalog' : 'torrents';
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
    <div class="flex items-center gap-6">
      <div class="flex items-center gap-1.5">
        <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">A</span>
        <span>Выбрать</span>
      </div>
      <div class="flex items-center gap-1.5">
        <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">B</span>
        <span>Назад</span>
      </div>
      {#if (hasFtpServers && activeTab === 'catalog') || activeTab === 'torrents'}
        <div class="flex items-center gap-1.5">
          <span class="w-4 h-4 rounded-full bg-white/10 text-white font-bold text-[10px] flex items-center justify-center border border-white/20">X</span>
          <span>Поиск</span>
        </div>
      {/if}
      <div class="flex items-center gap-1.5">
        <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-bold text-[10px] border border-white/20">LB / RB</span>
        <span>Вкладки</span>
      </div>
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
