<script lang="ts">
  import BigPictureHeader from './BigPictureHeader.svelte';
  import BigPictureLibrary from './BigPictureLibrary.svelte';
  import BigPictureGameDetail from './BigPictureGameDetail.svelte';
  import BigPictureDownloads from './BigPictureDownloads.svelte';
  import BigPictureSettings from './BigPictureSettings.svelte';
  import { sound } from '../../navigation/audio';

  type TabType = 'catalog' | 'downloads' | 'settings';

  let {
    activeTab = $bindable<TabType>('catalog'),
    games = [] as any[],
    activeDownloads = [] as any[],
    downloadHistory = [] as any[],
    settings = null as any,
    isGamepadConnected = false,
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
      if (activeTab !== 'catalog') {
        activeTab = 'catalog';
        e.preventDefault();
        setTimeout(() => {
          gamepad.focusFirstInZone('grid');
        }, 60);
        return;
      }
    };

    const handleToggleMenu = () => {
      if (activeTab === 'settings') {
        activeTab = 'catalog';
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
      if (activeTab !== 'catalog') {
        activeTab = 'catalog';
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
    onTabChange={(t) => {
      activeTab = t;
      selectedGame = null;
    }}
    activeDownloadsCount={(activeDownloads || []).filter((d) => d && (d.status === 'downloading' || d.status === 'queued')).length}
    {isGamepadConnected}
    onToggleSearch={() => {
      activeTab = 'catalog';
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
          activeTab = 'catalog';
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
</div>
