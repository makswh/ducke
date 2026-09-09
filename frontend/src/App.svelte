<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import TitleBar from './lib/components/TitleBar.svelte';
  import NavRail from './lib/components/NavRail.svelte';
  import BigPictureShell from './lib/components/bigpicture/BigPictureShell.svelte';
  import MasterDetailCatalog from './lib/components/MasterDetailCatalog.svelte';
  import DownloadsView from './lib/components/DownloadsView.svelte';
  import SettingsView from './lib/components/SettingsView.svelte';
  import GamepadHUD from './lib/components/GamepadHUD.svelte';
  import { gamepad } from './lib/navigation/gamepad';

  import {
    GetCatalog,
    GetGameDetails,
    UpdateSteamAppID,
    StartDownload,
    PauseDownload,
    ResumeDownload,
    CancelDownload,
    GetDownloads,
    GetDownloadHistory,
    GetSettings,
    SaveSettings,
    ImportFileZillaXML,
    TestConnection,
    SelectDirectory,
    OpenLocalFolder,
    TriggerSteamOSKeyboard
  } from '../wailsjs/go/main/App';

  import { EventsOn, EventsOff, Quit, WindowFullscreen, WindowUnfullscreen } from '../wailsjs/runtime/runtime';

  // Application State (Svelte 5 Runes)
  let displayMode = $state<'desktop' | 'bigpicture'>('desktop');
  let activeTab = $state<'catalog' | 'downloads' | 'settings'>('catalog');

  function switchToBigPicture() {
    displayMode = 'bigpicture';
    try {
      WindowFullscreen();
    } catch (e) {
      console.warn('Failed to enter fullscreen:', e);
    }
  }

  function switchToDesktop() {
    displayMode = 'desktop';
    try {
      WindowUnfullscreen();
    } catch (e) {
      console.warn('Failed to exit fullscreen:', e);
    }
  }
  let searchQuery = $state<string>('');
  let games = $state<any[]>([]);
  let activeDownloads = $state<any[]>([]);
  let downloadHistory = $state<any[]>([]);
  let settings = $state<any | null>(null);
  let isRefreshing = $state<boolean>(false);
  let isGamepadConnected = $state<boolean>(false);
  let toastMessage = $state<string>('');

  function showToast(msg: string) {
    toastMessage = msg;
    setTimeout(() => {
      if (toastMessage === msg) toastMessage = '';
    }, 3000);
  }

  async function loadInitialData() {
    try {
      settings = await GetSettings().catch(() => null);
      const [fetchedGames, fetchedDownloads, fetchedHistory] = await Promise.all([
        GetCatalog(false).catch(() => []),
        GetDownloads().catch(() => []),
        GetDownloadHistory().catch(() => [])
      ]);

      games = Array.isArray(fetchedGames) ? fetchedGames : [];
      activeDownloads = Array.isArray(fetchedDownloads) ? fetchedDownloads : [];
      downloadHistory = Array.isArray(fetchedHistory) ? fetchedHistory : [];

      if (settings && settings.steamDeckMode) {
        switchToBigPicture();
      }
    } catch (e: any) {
      console.error('Failed to load initial state:', e);
    }
  }

  async function handleRefreshCatalog() {
    if (isRefreshing) return;
    isRefreshing = true;
    try {
      const res = await GetCatalog(true);
      games = Array.isArray(res) ? res : [];
      showToast('Каталог обновлен');
    } catch (e: any) {
      showToast('Ошибка обновления: ' + (e?.message || e));
    } finally {
      isRefreshing = false;
    }
  }

  async function handleStartDownload(gameId: number, targetPath: string) {
    try {
      const dlId = await StartDownload(gameId, targetPath);
      activeTab = 'downloads';
      showToast('Загрузка добавлена в очередь');
      const dls = await GetDownloads();
      activeDownloads = Array.isArray(dls) ? dls : [];
    } catch (e: any) {
      showToast('Не удалось начать загрузку: ' + (e?.message || e));
    }
  }

  async function handlePauseDownload(id: string) {
    try {
      await PauseDownload(id);
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleResumeDownload(id: string) {
    try {
      await ResumeDownload(id);
      showToast('Загрузка возобновлена');
      const dls = await GetDownloads();
      activeDownloads = Array.isArray(dls) ? dls : [];
      const hist = await GetDownloadHistory();
      downloadHistory = Array.isArray(hist) ? hist : [];
    } catch (e: any) {
      showToast('Ошибка возобновления: ' + (e?.message || e));
    }
  }

  async function handleCancelDownload(id: string) {
    try {
      await CancelDownload(id);
      activeDownloads = (activeDownloads || []).filter((d) => d && d.downloadId !== id);
      const hist = await GetDownloadHistory();
      downloadHistory = Array.isArray(hist) ? hist : [];
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleOpenFolder(folderPath: string) {
    try {
      await OpenLocalFolder(folderPath);
    } catch (e: any) {
      showToast('Не удалось открыть папку: ' + (e?.message || e));
    }
  }

  async function handlePauseAll() {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.PauseAllDownloads) {
        await app.PauseAllDownloads();
      }
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleResumeAll() {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.ResumeAllDownloads) {
        await app.ResumeAllDownloads();
        showToast('Все загрузки возобновлены');
        const dls = await GetDownloads();
        activeDownloads = Array.isArray(dls) ? dls : [];
      }
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleClearCompleted() {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.ClearCompletedDownloads) {
        await app.ClearCompletedDownloads();
        activeDownloads = (activeDownloads || []).filter((d) => d && d.status !== 'completed' && d.status !== 'cancelled');
        const hist = await GetDownloadHistory();
        downloadHistory = Array.isArray(hist) ? hist : [];
        showToast('Завершенные загрузки очищены');
      }
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleDeleteRecord(id: string, removeFiles: boolean = false) {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.DeleteDownloadRecord) {
        await app.DeleteDownloadRecord(id, removeFiles);
        activeDownloads = (activeDownloads || []).filter((d) => d && d.downloadId !== id);
        const hist = await GetDownloadHistory();
        downloadHistory = Array.isArray(hist) ? hist : [];
      }
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleLaunchGame(path: string) {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.LaunchGame) {
        await app.LaunchGame(path);
      } else {
        handleOpenFolder(path);
      }
    } catch (e: any) {
      console.warn('Failed to launch game executable, falling back to folder:', e);
      handleOpenFolder(path);
    }
  }

  async function handleSaveSettings(newSettings: any) {
    try {
      await SaveSettings(newSettings);
      settings = newSettings;
      showToast('Настройки успешно сохранены');
    } catch (e: any) {
      showToast('Ошибка сохранения: ' + (e?.message || e));
    }
  }

  async function handleImportXML(xml: string) {
    try {
      const servers = await ImportFileZillaXML(xml);
      settings = await GetSettings();
      showToast(`Импортировано серверов: ${servers ? servers.length : 0}`);
      handleRefreshCatalog();
    } catch (e: any) {
      showToast('Ошибка импорта: ' + (e?.message || e));
    }
  }

  async function handleImportXMLFile() {
    try {
      let servers: any;
      if (typeof (window as any)?.go?.main?.App?.ImportFileZillaFile === 'function') {
        servers = await (window as any).go.main.App.ImportFileZillaFile();
      }
      if (servers && servers.length > 0) {
        settings = await GetSettings();
        showToast(`Импортировано серверов: ${servers.length}`);
        handleRefreshCatalog();
      }
    } catch (e: any) {
      if (e) showToast('Ошибка импорта: ' + (e?.message || e));
    }
  }

  async function handleOpenConfigFolder() {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.OpenConfigFolder) {
        await app.OpenConfigFolder();
      }
    } catch (e: any) {
      showToast('Не удалось открыть папку: ' + (e?.message || e));
    }
  }

  async function handleClearMetadataCache(): Promise<number> {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.ClearMetadataCache) {
        const count = await app.ClearMetadataCache();
        showToast(`Кэш очищен (${count} записей)`);
        handleRefreshCatalog();
        return count;
      }
      return 0;
    } catch (e: any) {
      showToast('Ошибка очистки кэша: ' + (e?.message || e));
      return 0;
    }
  }

  onMount(() => {
    loadInitialData();

    // Listen to real-time settings updates
    EventsOn('settings:updated', (updated: any) => {
      if (updated) {
        settings = updated;
      }
    });

    // Listen to real-time download progress events
    EventsOn('download:progress', async (event: any) => {
      if (!event) return;
      const list = activeDownloads || [];
      const existingIdx = list.findIndex((d) => d && d.downloadId === event.downloadId);
      if (existingIdx >= 0) {
        list[existingIdx] = event;
        activeDownloads = [...list];
      } else {
        activeDownloads = [...list, event];
      }

      if (event.status === 'completed' || event.status === 'cancelled') {
        try {
          const hist = await GetDownloadHistory();
          downloadHistory = Array.isArray(hist) ? hist : [];
        } catch (e) {
          console.error(e);
        }
      }
    });

    // Listen to progressive Steam enrichment events
    EventsOn('game:enriched', (enrichedGame: any) => {
      if (!enrichedGame) return;
      games = (games || []).map((g) => (g && g.id === enrichedGame.id ? enrichedGame : g));
    });

    // Listen to real-time Steam review summary updates
    EventsOn('game:reviews-updated', (event: any) => {
      if (!event) return;
      games = (games || []).map((g) => {
        if (g && (g.id === event.gameId || (event.steamAppId > 0 && g.steamAppId === event.steamAppId))) {
          return {
            ...g,
            reviewScoreDesc: event.reviewScoreDesc,
            reviewPercent: event.reviewPercent,
            totalReviews: event.totalReviews,
          };
        }
        return g;
      });
    });

    // Setup Gamepad Navigation
    gamepad.onTabChange = (dir) => {
      const tabs: ('catalog' | 'downloads' | 'settings')[] = ['catalog', 'downloads', 'settings'];
      const curIdx = tabs.indexOf(activeTab);
      if (dir === 'NEXT') {
        activeTab = tabs[(curIdx + 1) % tabs.length];
      } else {
        activeTab = tabs[(curIdx - 1 + tabs.length) % tabs.length];
      }
      setTimeout(() => {
        gamepad.focusFirstInZone('grid') || gamepad.focusFirstInZone('list') || gamepad.focusFirstInZone('detail');
      }, 100);
    };

    gamepad.onSearch = () => {
      activeTab = 'catalog';
      TriggerSteamOSKeyboard();
    };

    gamepad.onBack = () => {
      if (activeTab !== 'catalog') {
        activeTab = 'catalog';
        setTimeout(() => {
          gamepad.focusFirstInZone('grid') || gamepad.focusFirstInZone('list');
        }, 100);
      }
    };

    gamepad.onControllerStateChange = (connected) => {
      isGamepadConnected = connected;
    };

    gamepad.start();
  });

  onDestroy(() => {
    gamepad.stop();
    EventsOff('download:progress');
    EventsOff('game:enriched');
  });
</script>

{#if displayMode === 'bigpicture'}
  <BigPictureShell
    bind:activeTab
    {games}
    {activeDownloads}
    {downloadHistory}
    {settings}
    {isGamepadConnected}
    onStartDownload={handleStartDownload}
    onPauseDownload={handlePauseDownload}
    onResumeDownload={handleResumeDownload}
    onCancelDownload={handleCancelDownload}
    onPauseAll={handlePauseAll}
    onResumeAll={handleResumeAll}
    onClearCompleted={handleClearCompleted}
    onDeleteRecord={handleDeleteRecord}
    onSaveSettings={handleSaveSettings}
    onImportXML={handleImportXML}
    onImportFile={handleImportXMLFile}
    onTestConnection={TestConnection}
    onSelectFolder={SelectDirectory}
    onOpenFolder={handleOpenFolder}
    onOpenConfigFolder={handleOpenConfigFolder}
    onClearMetadataCache={handleClearMetadataCache}
    onSwitchToDesktop={switchToDesktop}
    onCloseApp={() => Quit()}
  />
{:else}
  <!-- Full-viewport Master-Detail Application Shell (Desktop View) -->
  <div class="h-screen w-screen flex flex-col overflow-hidden bg-[#07080a] text-white selection:bg-[#3b82f6] selection:text-white font-sans-ui">
    <!-- Custom Window Control Bar -->
    <TitleBar onToggleBigPicture={switchToBigPicture} />

    <!-- Main Application Body: Left NavRail + Content Stage -->
    <div class="flex-1 flex overflow-hidden min-h-0">
      <!-- Left Thin Navigation Rail (64px) -->
      <NavRail
        bind:activeTab
        activeDownloadsCount={(activeDownloads || []).filter((d) => d && (d.status === 'downloading' || d.status === 'queued')).length}
        {isGamepadConnected}
        isConnected={!!settings?.activeServer?.host}
        serverName={settings?.activeServer?.name || ''}
        onRefresh={handleRefreshCatalog}
        {isRefreshing}
        onToggleBigPicture={switchToBigPicture}
      />

      <!-- Main Content Stage -->
      <div class="flex-1 flex overflow-hidden">
      {#if activeTab === 'catalog'}
        <MasterDetailCatalog
          {games}
          bind:searchQuery
          downloadPath={settings?.downloadPath || 'C:\\Ducke'}
          onStartDownload={handleStartDownload}
          onSelectFolder={SelectDirectory}
        />
      {:else if activeTab === 'downloads'}
        <DownloadsView
          {games}
          {activeDownloads}
          {downloadHistory}
          {settings}
          downloadPath={settings?.downloadPath || ''}
          onPause={handlePauseDownload}
          onResume={handleResumeDownload}
          onCancel={handleCancelDownload}
          onPauseAll={handlePauseAll}
          onResumeAll={handleResumeAll}
          onClearCompleted={handleClearCompleted}
          onDeleteRecord={handleDeleteRecord}
          onOpenFolder={handleOpenFolder}
          onLaunchGame={handleLaunchGame}
          onGoToCatalog={() => (activeTab = 'catalog')}
          onGoToSettings={() => (activeTab = 'settings')}
          onUpdateSpeedLimit={async (kbps: number) => {
            try {
              const app = (window as any)?.go?.main?.App;
              if (app && typeof app.UpdateSpeedLimit === 'function') {
                await app.UpdateSpeedLimit(kbps);
              } else if (settings) {
                const updated = { ...settings, maxSpeedKBps: kbps };
                await handleSaveSettings(updated);
              }
            } catch (e: any) {
              console.error('Failed to update speed limit:', e);
            }
          }}
        />
      {:else if activeTab === 'settings'}
        <SettingsView
          {settings}
          onSaveSettings={handleSaveSettings}
          onImportFile={handleImportXMLFile}
          onTestConnection={TestConnection}
          onSelectFolder={SelectDirectory}
          onOpenFolder={handleOpenFolder}
          onOpenConfigFolder={handleOpenConfigFolder}
          onClearMetadataCache={handleClearMetadataCache}
        />
      {/if}
      </div>
    </div>

    <!-- Toast Notification (Modern Pro Pill) -->
    {#if toastMessage}
      <div class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 px-4 py-2 rounded-lg bg-[#21252c] border border-white/[0.1] text-white text-xs font-semibold shadow-2xl animate-fade-in flex items-center gap-2">
        <span class="w-2 h-2 rounded-full bg-[#3b82f6]"></span>
        <span>{toastMessage}</span>
      </div>
    {/if}

    <!-- Gamepad HUD bar -->
    <GamepadHUD isVisible={isGamepadConnected} />
  </div>
{/if}
