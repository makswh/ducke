<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import {
    HardDrive,
    Server,
    DownloadCloud,
    Info,
    Check,
    Star,
    RefreshCw,
    FolderOpen,
    Folder,
    Monitor,
    Power,
    Eye,
    EyeOff,
    FileCode,
    Plus,
    Trash2,
    CheckCircle2,
    Terminal,
    Download,
    X
  } from 'lucide-svelte';
  import { sound } from '../../navigation/audio';
  import * as AppAPI from '../../../../wailsjs/go/main/App';
  import { EventsOn, EventsOff } from '../../../../wailsjs/runtime/runtime';

  interface StorageDrive {
    id: string;
    label: string;
    path: string;
    type: string;
    freeBytes: number;
    totalBytes: number;
    usedBytes: number;
    duckeBytes: number;
    freeGB: string;
    totalGB: string;
    usedGB: string;
    duckeGB: string;
    isDefault: boolean;
  }

  interface ServerConfig {
    id: string;
    name: string;
    host: string;
    port: number;
    protocol: string;
    user: string;
    password?: string;
    remoteDir: string;
    isActive: boolean;
  }

  interface AppSettings {
    downloadPath: string;
    maxConcurrentFiles: number;
    maxSpeedKBps: number;
    theme: string;
    steamDeckMode: boolean;
    steamApiKey: string;
    enableLogs?: boolean;
    activeServer?: ServerConfig;
    savedServers: ServerConfig[];
  }

  let {
    settings = null as AppSettings | null,
    onSaveSettings = (s: AppSettings) => {},
    onImportFile = async () => {},
    onTestConnection = async (srv: ServerConfig): Promise<{ success: boolean; protocolUsed?: string; errorMessage?: string }> => ({ success: false }),
    onSelectFolder = async (): Promise<string> => '',
    onOpenFolder = (path: string) => {},
    onOpenConfigFolder = async () => {},
    onClearMetadataCache = async (): Promise<number> => 0,
    onSwitchToDesktop = () => {},
    onCloseApp = () => {}
  } = $props();

  type SettingsCategory = 'storage' | 'server' | 'downloads' | 'logs' | 'about';
  let activeCategory = $state<SettingsCategory>('storage');

  let localSettings = $state<AppSettings>({
    downloadPath: 'C:\\Ducke',
    maxConcurrentFiles: 4,
    maxSpeedKBps: 0,
    theme: 'dark',
    steamDeckMode: true,
    steamApiKey: '',
    enableLogs: false,
    savedServers: [],
    activeServer: {
      id: 'srv_1',
      name: 'Основной сервер',
      host: '',
      port: 2022,
      protocol: 'sftp',
      user: '',
      password: '',
      remoteDir: '/public',
      isActive: true
    }
  });

  let selectedServerId = $state<string>('');
  let appInfo = $state<{ name: string; version: string }>({ name: 'Ducke', version: '1.0.0' });
  let storageDrives = $state<StorageDrive[]>([]);
  let isTestingConnection = $state<boolean>(false);
  let testResult = $state<{ success: boolean; message: string } | null>(null);
  let saveFeedbackVisible = $state<boolean>(false);
  let showPassword = $state<boolean>(false);
  let isClearingCache = $state<boolean>(false);
  let cacheClearMessage = $state<string | null>(null);

  // Logging state
  interface LogEntry {
    id: number;
    timestamp: string;
    level: string;
    source: string;
    message: string;
  }

  let logEntries = $state<LogEntry[]>([]);
  let selectedLogLevel = $state<string>('ALL');
  let logSearchQuery = $state<string>('');
  let autoScrollLogs = $state<boolean>(true);
  let logContainerEl = $state<HTMLDivElement | null>(null);
  let exportMessage = $state<string | null>(null);

  async function loadLogs() {
    try {
      const entries = await AppAPI.GetLogs();
      logEntries = Array.isArray(entries) ? entries : [];
      if (autoScrollLogs && logContainerEl) {
        setTimeout(() => {
          logContainerEl?.scrollTo({ top: logContainerEl.scrollHeight });
        }, 50);
      }
    } catch {
      logEntries = [];
    }
  }

  async function handleClearLogs() {
    sound.playSelect();
    try {
      await AppAPI.ClearLogs();
      logEntries = [];
    } catch (e) {
      console.error(e);
    }
  }

  async function handleExportLogs() {
    sound.playSelect();
    try {
      const exportedPath = await AppAPI.ExportLogs('');
      if (exportedPath) {
        exportMessage = `Сохранено в: ${exportedPath}`;
        setTimeout(() => (exportMessage = null), 5000);
        return;
      }
    } catch {
      // Fallback: download via browser blob
      try {
        const text = await AppAPI.GetLogsText();
        const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `ducke_logs_${new Date().toISOString().slice(0, 10)}.txt`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
        exportMessage = 'Логи успешно скачаны';
        setTimeout(() => (exportMessage = null), 4000);
      } catch (e: any) {
        exportMessage = 'Ошибка экспорта: ' + (e?.message || e);
      }
    }
  }

  let displayedLogs = $derived.by(() => {
    let list = logEntries;
    if (selectedLogLevel !== 'ALL') {
      list = list.filter((e) => e.level === selectedLogLevel);
    }
    if (logSearchQuery.trim()) {
      const q = logSearchQuery.toLowerCase().trim();
      list = list.filter((e) =>
        e.message.toLowerCase().includes(q) ||
        e.source.toLowerCase().includes(q) ||
        e.timestamp.toLowerCase().includes(q)
      );
    }
    return list;
  });

  $effect(() => {
    if (settings) {
      const current = settings;
      untrack(() => {
        const savedServers = Array.isArray(current.savedServers) && current.savedServers.length > 0
          ? current.savedServers.map(s => ({ ...s }))
          : current.activeServer
            ? [{ ...current.activeServer, isActive: true }]
            : [];

        const activeServer = current.activeServer
          ? { ...current.activeServer }
          : savedServers.length > 0
            ? { ...savedServers[0] }
            : {
                id: 'srv_1',
                name: 'Основной сервер',
                host: '',
                port: 2022,
                protocol: 'sftp',
                user: '',
                password: '',
                remoteDir: '/public',
                isActive: true
              };

        localSettings = {
          downloadPath: current.downloadPath || 'C:\\Ducke',
          maxConcurrentFiles: current.maxConcurrentFiles || 4,
          maxSpeedKBps: current.maxSpeedKBps || 0,
          theme: current.theme || 'dark',
          steamDeckMode: current.steamDeckMode ?? true,
          steamApiKey: current.steamApiKey || '',
          enableLogs: !!current.enableLogs,
          savedServers,
          activeServer
        };

        if (!selectedServerId && activeServer) {
          selectedServerId = activeServer.id;
        }
      });
    }
  });

  let selectedServer = $derived.by<ServerConfig | null>(() => {
    const servers = localSettings.savedServers;
    if (servers.length === 0) return localSettings.activeServer || null;
    const found = servers.find(s => s.id === selectedServerId);
    return found || servers[0] || localSettings.activeServer || null;
  });

  async function loadStorageDrives() {
    try {
      if (typeof (AppAPI as any)?.GetStorageDrives === 'function') {
        const drives = await (AppAPI as any).GetStorageDrives();
        if (Array.isArray(drives) && drives.length > 0) storageDrives = drives;
      } else if (typeof (window as any)?.go?.main?.App?.GetStorageDrives === 'function') {
        const drives = await (window as any).go.main.App.GetStorageDrives();
        if (Array.isArray(drives) && drives.length > 0) storageDrives = drives;
      }
    } catch (e) {
      console.warn('[BigPictureSettings] Failed to load drives:', e);
    }
  }

  onMount(async () => {
    try {
      if (typeof (AppAPI as any)?.GetAppInfo === 'function') {
        const info = await (AppAPI as any).GetAppInfo();
        if (info) appInfo = info;
      } else if (typeof (window as any)?.go?.main?.App?.GetAppInfo === 'function') {
        const info = await (window as any).go.main.App.GetAppInfo();
        if (info) appInfo = info;
      }
    } catch (e) {
      console.warn('[BigPictureSettings] Failed to get app info:', e);
    }

    loadStorageDrives();

    if (localSettings.enableLogs) {
      loadLogs();
    }

    EventsOn('log:entry', (entry: any) => {
      if (entry && entry.id) {
        logEntries = [...logEntries.slice(-1999), entry];
        if (autoScrollLogs && logContainerEl) {
          requestAnimationFrame(() => {
            logContainerEl?.scrollTo({ top: logContainerEl.scrollHeight });
          });
        }
      }
    });

    return () => {
      EventsOff('log:entry');
    };
  });

  function triggerSave() {
    handleServerFieldChange();
    onSaveSettings(localSettings);
    saveFeedbackVisible = true;
    setTimeout(() => {
      saveFeedbackVisible = false;
    }, 2000);
  }

  async function handleBrowseFolder() {
    sound.playSelect();
    const selected = await onSelectFolder();
    if (selected) {
      localSettings.downloadPath = selected;
      triggerSave();
      loadStorageDrives();
    }
  }

  function handleSetDriveAsDefault(drive: StorageDrive) {
    sound.playSelect();
    let target = drive.path;
    if (drive.type === 'internal' && (target === 'C:\\' || target === 'C:')) {
      target = 'C:\\Ducke';
    } else if (drive.type === 'internal' && (target === '/home' || target === '/' || target.startsWith('/home/'))) {
      target = '/home/deck/Games/Ducke';
    } else if (drive.type === 'sdcard' || drive.type === 'removable') {
      target = `${drive.path.replace(/\/+$/, '')}/Games/Ducke`;
    } else {
      target = drive.path;
    }
    localSettings.downloadPath = target;
    triggerSave();
    loadStorageDrives();
  }

  function handleAddServer() {
    sound.playSelect();
    const newId = `srv_${Date.now()}`;
    const newServer: ServerConfig = {
      id: newId,
      name: `Сервер ${localSettings.savedServers.length + 1}`,
      host: '',
      port: 2022,
      protocol: 'sftp',
      user: '',
      password: '',
      remoteDir: '/public',
      isActive: localSettings.savedServers.length === 0
    };

    localSettings.savedServers = [...localSettings.savedServers, newServer];
    selectedServerId = newId;
    if (newServer.isActive) {
      localSettings.activeServer = { ...newServer };
    }
    triggerSave();
  }

  function handleSelectServer(id: string) {
    sound.playFocus();
    selectedServerId = id;
    testResult = null;
  }

  function handleMakeServerActive(srv: ServerConfig) {
    sound.playSelect();
    localSettings.savedServers = localSettings.savedServers.map(s => ({
      ...s,
      isActive: s.id === srv.id
    }));
    localSettings.activeServer = { ...srv, isActive: true };
    triggerSave();
  }

  function handleDeleteServer(srv: ServerConfig) {
    if (localSettings.savedServers.length <= 1) return;
    sound.playBack();
    const remaining = localSettings.savedServers.filter(s => s.id !== srv.id);
    localSettings.savedServers = remaining;

    if (localSettings.activeServer?.id === srv.id && remaining.length > 0) {
      remaining[0].isActive = true;
      localSettings.activeServer = { ...remaining[0] };
    }

    selectedServerId = remaining[0]?.id || '';
    triggerSave();
  }

  function handleServerFieldChange() {
    if (!selectedServer) return;
    const idx = localSettings.savedServers.findIndex(s => s.id === selectedServer.id);
    if (idx !== -1) {
      localSettings.savedServers[idx] = { ...selectedServer };
    }
    if (localSettings.activeServer?.id === selectedServer.id) {
      localSettings.activeServer = { ...selectedServer, isActive: true };
    }
  }

  async function handleTestConn() {
    if (!selectedServer) return;
    sound.playSelect();
    isTestingConnection = true;
    testResult = null;
    try {
      const res = await onTestConnection(selectedServer);
      if (res.success) {
        testResult = {
          success: true,
          message: `Соединение успешно (${(res.protocolUsed || selectedServer.protocol).toUpperCase()})`
        };
      } else {
        testResult = {
          success: false,
          message: res.errorMessage || 'Не удалось подключиться к серверу'
        };
      }
    } catch (e: any) {
      testResult = {
        success: false,
        message: e?.message || 'Ошибка соединения'
      };
    } finally {
      isTestingConnection = false;
    }
  }

  async function handleClearCache() {
    sound.playSelect();
    isClearingCache = true;
    cacheClearMessage = null;
    try {
      const count = await onClearMetadataCache();
      cacheClearMessage = `Кэш очищен (${count} записей перепроверено)`;
      setTimeout(() => {
        cacheClearMessage = null;
      }, 4000);
    } catch (e: any) {
      cacheClearMessage = `Ошибка: ${e?.message || e}`;
    } finally {
      isClearingCache = false;
    }
  }

  function adjustConcurrent(delta: number) {
    sound.playSelect();
    const current = localSettings.maxConcurrentFiles || 4;
    const updated = Math.min(8, Math.max(1, current + delta));
    localSettings.maxConcurrentFiles = updated;
    triggerSave();
  }

  function setSpeedLimit(kbps: number) {
    sound.playSelect();
    localSettings.maxSpeedKBps = kbps;
    triggerSave();
  }

  function formatBytesReadable(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 МБ';
    const gb = bytes / (1024 * 1024 * 1024);
    if (gb >= 1) return `${gb.toFixed(1)} ГБ`;
    const mb = bytes / (1024 * 1024);
    return `${mb.toFixed(0)} МБ`;
  }
</script>

<!-- SteamOS 10-Foot Console Settings Shell -->
<div class="flex-1 flex h-full overflow-hidden bg-[#07080a] text-white select-none">

  <!-- Left Column: Settings Categories (Gamepad D-pad accessible) -->
  <aside data-nav-zone="list" class="w-72 lg:w-80 flex flex-col flex-shrink-0 h-full border-r border-white/[0.06] bg-[#090b10]">
    <div class="px-8 pt-8 pb-6 border-b border-white/[0.04]">
      <h2 class="text-xs font-black uppercase tracking-widest text-[#8e95a2] font-mono">Настройки</h2>
    </div>

    <nav class="flex-1 px-4 py-4 space-y-1.5 overflow-y-auto">
      <button
        data-nav-item
        class="w-full flex items-center gap-3.5 px-4 py-3.5 rounded-xl text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'storage' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          activeCategory = 'storage';
        }}
      >
        <HardDrive class="w-5 h-5 flex-shrink-0 {activeCategory === 'storage' ? 'text-sky-400' : ''}" />
        <span>Хранилище</span>
      </button>

      <button
        data-nav-item
        class="w-full flex items-center gap-3.5 px-4 py-3.5 rounded-xl text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'server' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          activeCategory = 'server';
        }}
      >
        <Server class="w-5 h-5 flex-shrink-0 {activeCategory === 'server' ? 'text-sky-400' : ''}" />
        <span>Серверы</span>
      </button>

      <button
        data-nav-item
        class="w-full flex items-center gap-3.5 px-4 py-3.5 rounded-xl text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'downloads' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          activeCategory = 'downloads';
        }}
      >
        <DownloadCloud class="w-5 h-5 flex-shrink-0 {activeCategory === 'downloads' ? 'text-sky-400' : ''}" />
        <span>Загрузки</span>
      </button>

      {#if localSettings.enableLogs}
        <button
          data-nav-item
          class="w-full flex items-center gap-3.5 px-4 py-3.5 rounded-xl text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'logs' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
          onclick={() => {
            sound.playFocus();
            activeCategory = 'logs';
            loadLogs();
          }}
        >
          <Terminal class="w-5 h-5 flex-shrink-0 {activeCategory === 'logs' ? 'text-sky-400' : ''}" />
          <span>Логи</span>
          {#if logEntries.length > 0}
            <span class="ml-auto text-xs px-2 py-0.5 rounded-md bg-white/[0.08] text-[#8e95a2] font-mono">
              {logEntries.length}
            </span>
          {/if}
        </button>
      {/if}

      <button
        data-nav-item
        class="w-full flex items-center gap-3.5 px-4 py-3.5 rounded-xl text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'about' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          activeCategory = 'about';
        }}
      >
        <Info class="w-5 h-5 flex-shrink-0 {activeCategory === 'about' ? 'text-sky-400' : ''}" />
        <span>О системе</span>
      </button>
    </nav>

    <!-- Save Indicator -->
    {#if saveFeedbackVisible}
      <div class="m-4 p-3 rounded-xl bg-emerald-500/20 border border-emerald-500/40 text-emerald-300 text-xs font-mono font-bold flex items-center justify-center gap-2">
        <Check class="w-4 h-4" />
        <span>СОХРАНЕНО</span>
      </div>
    {/if}
  </aside>

  <!-- Right Content Area (10-Foot Console Layout) -->
  <main data-nav-zone="detail" class="flex-1 flex flex-col h-full overflow-y-auto p-8 lg:p-12 space-y-8 max-w-4xl">

    <!-- ============================================================ -->
    <!-- 1. STORAGE                                                    -->
    <!-- ============================================================ -->
    {#if activeCategory === 'storage'}
      <div class="space-y-8">
        <div class="border-b border-white/[0.06] pb-4">
          <h1 class="text-xl font-bold uppercase tracking-wider text-white font-mono">Управление хранилищем</h1>
          <p class="text-xs text-[#8e95a2] mt-1 font-mono">Выберите накопитель и каталог для сохранения игр</p>
        </div>

        {#if storageDrives.length === 0}
          <div class="p-6 rounded-2xl bg-[#0d1017] border border-white/[0.06] space-y-4">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <HardDrive class="w-6 h-6 text-sky-400" />
                <span class="text-base font-bold text-white font-mono">Каталог загрузки</span>
              </div>
              <span class="text-xs font-mono text-[#8e95a2]">{localSettings.downloadPath}</span>
            </div>
            <button
              data-nav-item
              class="px-5 py-2.5 rounded-xl bg-white/10 hover:bg-white/20 focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors"
              onclick={handleBrowseFolder}
            >
              Выбрать другую папку
            </button>
          </div>
        {:else}
          <div class="space-y-6">
            {#each storageDrives as drive}
              {@const total = drive.totalBytes || 1}
              {@const duckePct = Math.min(100, Math.max(0, (drive.duckeBytes / total) * 100))}
              {@const otherBytes = Math.max(0, (drive.usedBytes - drive.duckeBytes))}
              {@const otherPct = Math.min(100 - duckePct, Math.max(0, (otherBytes / total) * 100))}

              <div class="p-6 rounded-2xl bg-[#0d1017] border border-white/[0.08] space-y-5">
                <!-- Header -->
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-3">
                    <HardDrive class="w-6 h-6 text-sky-400" />
                    <div>
                      <div class="flex items-center gap-2">
                        <span class="text-base font-bold text-white font-mono">{drive.label}</span>
                        {#if drive.isDefault}
                          <span class="px-2 py-0.5 rounded text-[10px] font-mono font-bold bg-sky-500/20 text-sky-400 border border-sky-500/30">
                            Основной
                          </span>
                        {/if}
                      </div>
                      <span class="text-xs font-mono text-[#64748b]">{drive.isDefault ? localSettings.downloadPath : drive.path}</span>
                    </div>
                  </div>

                  <div class="text-xs font-mono font-bold uppercase tracking-wider text-white">
                    {drive.freeGB} свободно из {drive.totalGB}
                  </div>
                </div>

                <!-- Steam Storage Bar -->
                <div class="w-full h-3 rounded-full bg-[#1e293b] overflow-hidden flex">
                  {#if duckePct > 0}
                    <div class="h-full bg-sky-400 transition-all duration-300" style="width: {duckePct}%;"></div>
                  {/if}
                  {#if otherPct > 0}
                    <div class="h-full bg-amber-400 transition-all duration-300" style="width: {otherPct}%;"></div>
                  {/if}
                </div>

                <!-- Storage Legend -->
                <div class="flex flex-wrap items-center gap-x-6 gap-y-2 text-xs font-mono uppercase tracking-wider">
                  <div class="flex items-center gap-2 text-sky-400">
                    <span class="text-sm leading-none">●</span>
                    <span class="text-white font-bold">DUCKE</span>
                    <span class="text-[#94a3b8] font-normal lowercase">{drive.duckeGB}</span>
                  </div>

                  <div class="flex items-center gap-2 text-amber-400">
                    <span class="text-sm leading-none">●</span>
                    <span class="text-white font-bold">ДРУГИЕ ДАННЫЕ</span>
                    <span class="text-[#94a3b8] font-normal lowercase">{formatBytesReadable(otherBytes)}</span>
                  </div>

                  <div class="flex items-center gap-2 text-[#64748b]">
                    <span class="text-sm leading-none">●</span>
                    <span class="text-white font-bold">СВОБОДНО</span>
                    <span class="text-[#94a3b8] font-normal lowercase">{drive.freeGB}</span>
                  </div>
                </div>

                <!-- Actions -->
                <div class="pt-2 flex flex-wrap items-center gap-3 border-t border-white/[0.04]">
                  {#if !drive.isDefault}
                    <button
                      data-nav-item
                      class="px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors"
                      onclick={() => handleSetDriveAsDefault(drive)}
                    >
                      Сделать основным
                    </button>
                  {/if}

                  <button
                    data-nav-item
                    class="px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors"
                    onclick={handleBrowseFolder}
                  >
                    Выбрать другую папку
                  </button>

                  <button
                    data-nav-item
                    class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 focus:ring-2 focus:ring-white focus:outline-none text-[#8e95a2] hover:text-white text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-1.5"
                    onclick={() => {
                      sound.playSelect();
                      onOpenFolder(drive.isDefault ? localSettings.downloadPath : drive.path);
                    }}
                  >
                    <FolderOpen class="w-3.5 h-3.5" />
                    <span>Открыть в проводнике</span>
                  </button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>

    <!-- ============================================================ -->
    <!-- 2. SERVERS (Multi-Server Management)                         -->
    <!-- ============================================================ -->
    {:else if activeCategory === 'server'}
      <div class="space-y-6">
        <div class="flex items-center justify-between border-b border-white/[0.06] pb-4">
          <div>
            <h1 class="text-xl font-bold uppercase tracking-wider text-white font-mono">FTP / SFTP Серверы</h1>
            <p class="text-xs text-[#8e95a2] mt-1 font-mono">Управление удаленными хранилищами с играми</p>
          </div>

          <div class="flex items-center gap-3">
            <button
              data-nav-item
              class="px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold flex items-center gap-2 cursor-pointer transition-colors"
              onclick={() => {
                sound.playSelect();
                onImportFile();
              }}
            >
              <FileCode class="w-3.5 h-3.5" />
              <span>Импорт XML</span>
            </button>

            <button
              data-nav-item
              class="px-4 py-2 rounded-xl bg-sky-500 hover:bg-sky-400 focus:ring-2 focus:ring-white focus:outline-none text-slate-950 text-xs font-mono font-bold flex items-center gap-1.5 cursor-pointer transition-colors shadow-md"
              onclick={handleAddServer}
            >
              <Plus class="w-3.5 h-3.5" />
              <span>Добавить</span>
            </button>
          </div>
        </div>

        <!-- Server Selector Tabs -->
        {#if localSettings.savedServers.length > 0}
          <div class="flex items-center gap-2.5 overflow-x-auto pb-2">
            {#each localSettings.savedServers as srv (srv.id)}
              {@const isSelected = selectedServer?.id === srv.id}
              {@const isActive = localSettings.activeServer?.id === srv.id}
              <button
                data-nav-item
                class="px-4 py-2.5 rounded-xl text-xs font-mono border transition-all cursor-pointer flex items-center gap-2.5 flex-shrink-0 focus:ring-2 focus:ring-white focus:outline-none {isSelected ? 'bg-white/15 border-white/30 text-white font-bold' : 'bg-[#0d1017] border-white/[0.08] text-[#8e95a2] hover:text-white'}"
                onclick={() => handleSelectServer(srv.id)}
              >
                <span>{srv.name || srv.host || 'Сервер'}</span>
                {#if isActive}
                  <span class="w-2 h-2 rounded-full bg-emerald-400"></span>
                {/if}
              </button>
            {/each}
          </div>
        {/if}

        {#if selectedServer}
          <div class="p-6 rounded-2xl bg-[#0d1017] border border-white/[0.08] space-y-5">
            <!-- Active Toggle & Delete -->
            <div class="flex items-center justify-between pb-3 border-b border-white/[0.06]">
              <div>
                {#if localSettings.activeServer?.id === selectedServer.id}
                  <div class="flex items-center gap-1.5 px-3 py-1 rounded-xl bg-emerald-500/15 border border-emerald-500/30 text-emerald-400 text-xs font-mono font-bold">
                    <CheckCircle2 class="w-3.5 h-3.5" />
                    <span>АКТИВНЫЙ СЕРВЕР</span>
                  </div>
                {:else}
                  <button
                    data-nav-item
                    class="px-4 py-1.5 rounded-xl bg-sky-500/15 text-sky-400 hover:bg-sky-500/25 border border-sky-500/30 focus:ring-2 focus:ring-white focus:outline-none text-xs font-mono font-bold cursor-pointer transition-colors"
                    onclick={() => handleMakeServerActive(selectedServer)}
                  >
                    Сделать активным
                  </button>
                {/if}
              </div>

              {#if localSettings.savedServers.length > 1}
                <button
                  data-nav-item
                  class="px-3 py-1.5 rounded-xl text-xs font-mono text-[#8e95a2] hover:text-rose-400 hover:bg-rose-500/10 focus:ring-2 focus:ring-rose-400 focus:outline-none transition-colors cursor-pointer flex items-center gap-1.5"
                  onclick={() => handleDeleteServer(selectedServer)}
                >
                  <Trash2 class="w-3.5 h-3.5" />
                  <span>Удалить</span>
                </button>
              {/if}
            </div>

            <!-- Server Name -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
              <span class="text-sm font-bold text-white font-mono">Название</span>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.name}
                onchange={triggerSave}
                placeholder="Основной SFTP"
                class="w-72 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-xl focus:border-sky-400 focus:ring-2 focus:ring-white focus:outline-none"
              />
            </div>

            <!-- Protocol Toggle -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
              <span class="text-sm font-bold text-white font-mono">Протокол</span>
              <div class="flex items-center gap-2">
                <button
                  data-nav-item
                  type="button"
                  class="px-5 py-2 rounded-xl text-xs font-mono font-bold cursor-pointer transition-all focus:ring-2 focus:ring-white focus:outline-none {selectedServer.protocol === 'sftp' ? 'bg-sky-500 text-slate-950 font-bold' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                  onclick={() => {
                    sound.playSelect();
                    if (selectedServer) {
                      selectedServer.protocol = 'sftp';
                      if (selectedServer.port === 21) selectedServer.port = 2022;
                      triggerSave();
                    }
                  }}
                >
                  SFTP (2022)
                </button>

                <button
                  data-nav-item
                  type="button"
                  class="px-5 py-2 rounded-xl text-xs font-mono font-bold cursor-pointer transition-all focus:ring-2 focus:ring-white focus:outline-none {selectedServer.protocol === 'ftp' ? 'bg-sky-500 text-slate-950 font-bold' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                  onclick={() => {
                    sound.playSelect();
                    if (selectedServer) {
                      selectedServer.protocol = 'ftp';
                      if (selectedServer.port === 2022) selectedServer.port = 21;
                      triggerSave();
                    }
                  }}
                >
                  FTP (21)
                </button>
              </div>
            </div>

            <!-- Host -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
              <span class="text-sm font-bold text-white font-mono">Хост / IP-адрес</span>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.host}
                onchange={triggerSave}
                placeholder="192.168.1.100"
                class="w-72 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-xl focus:border-sky-400 focus:ring-2 focus:ring-white focus:outline-none"
              />
            </div>

            <!-- Port -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
              <span class="text-sm font-bold text-white font-mono">Порт</span>
              <input
                data-nav-item
                type="number"
                bind:value={selectedServer.port}
                onchange={triggerSave}
                class="w-32 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-xl focus:border-sky-400 focus:ring-2 focus:ring-white focus:outline-none"
              />
            </div>

            <!-- User -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
              <span class="text-sm font-bold text-white font-mono">Пользователь</span>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.user}
                onchange={triggerSave}
                placeholder="anonymous"
                class="w-72 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-xl focus:border-sky-400 focus:ring-2 focus:ring-white focus:outline-none"
              />
            </div>

            <!-- Password -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
              <span class="text-sm font-bold text-white font-mono">Пароль</span>
              <div class="flex items-center gap-2">
                <button
                  data-nav-item
                  type="button"
                  class="p-2 rounded-xl text-[#8e95a2] hover:text-white cursor-pointer focus:ring-2 focus:ring-white focus:outline-none"
                  onclick={() => (showPassword = !showPassword)}
                  title={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
                >
                  {#if showPassword}
                    <EyeOff class="w-4 h-4" />
                  {:else}
                    <Eye class="w-4 h-4" />
                  {/if}
                </button>
                <input
                  data-nav-item
                  type={showPassword ? 'text' : 'password'}
                  bind:value={selectedServer.password}
                  onchange={triggerSave}
                  placeholder="••••••••"
                  class="w-64 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-xl focus:border-sky-400 focus:ring-2 focus:ring-white focus:outline-none"
                />
              </div>
            </div>

            <!-- Remote Dir -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
              <span class="text-sm font-bold text-white font-mono">Удаленный каталог</span>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.remoteDir}
                onchange={triggerSave}
                placeholder="/public"
                class="w-72 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-xl focus:border-sky-400 focus:ring-2 focus:ring-white focus:outline-none"
              />
            </div>

            <!-- Connection Test -->
            <div class="pt-3 flex items-center justify-between">
              <button
                data-nav-item
                disabled={isTestingConnection}
                class="px-5 py-2.5 rounded-xl bg-white/10 hover:bg-white/20 text-white text-xs font-mono font-bold cursor-pointer flex items-center gap-2.5 transition-colors disabled:opacity-50 focus:ring-2 focus:ring-white focus:outline-none"
                onclick={handleTestConn}
              >
                {#if isTestingConnection}
                  <RefreshCw class="w-4 h-4 animate-spin text-sky-400" />
                  <span>Проверка подключения...</span>
                {:else}
                  <Server class="w-4 h-4 text-sky-400" />
                  <span>Проверить соединение</span>
                {/if}
              </button>

              {#if testResult}
                <div class="text-xs font-mono font-bold flex items-center gap-2 {testResult.success ? 'text-emerald-400' : 'text-rose-400'}">
                  <span class="w-2 h-2 rounded-full {testResult.success ? 'bg-emerald-400' : 'bg-rose-400'}"></span>
                  <span>{testResult.message}</span>
                </div>
              {/if}
            </div>
          </div>
        {/if}
      </div>

    <!-- ============================================================ -->
    <!-- 3. DOWNLOADS                                                  -->
    <!-- ============================================================ -->
    {:else if activeCategory === 'downloads'}
      <div class="space-y-6">
        <div class="border-b border-white/[0.06] pb-4">
          <h1 class="text-xl font-bold uppercase tracking-wider text-white font-mono">Параметры загрузок</h1>
          <p class="text-xs text-[#8e95a2] mt-1 font-mono">Ограничения скорости и многопоточность</p>
        </div>

        <div class="p-6 rounded-2xl bg-[#0d1017] border border-white/[0.08] space-y-6">
          <!-- Concurrent Files Stepper -->
          <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
            <div>
              <span class="text-sm font-bold text-white font-mono">Одновременных файлов</span>
              <p class="text-xs text-[#64748b] font-mono">Количество файлов, загружаемых параллельно</p>
            </div>

            <div class="flex items-center gap-3">
              <button
                data-nav-item
                class="w-10 h-10 rounded-xl bg-white/10 hover:bg-white/20 focus:ring-2 focus:ring-white focus:outline-none text-white flex items-center justify-center font-bold text-lg cursor-pointer transition-colors"
                onclick={() => adjustConcurrent(-1)}
                title="Уменьшить"
              >
                −
              </button>
              <span class="w-8 text-center text-lg font-mono font-bold text-sky-400">{localSettings.maxConcurrentFiles || 4}</span>
              <button
                data-nav-item
                class="w-10 h-10 rounded-xl bg-white/10 hover:bg-white/20 focus:ring-2 focus:ring-white focus:outline-none text-white flex items-center justify-center font-bold text-lg cursor-pointer transition-colors"
                onclick={() => adjustConcurrent(1)}
                title="Увеличить"
              >
                +
              </button>
            </div>
          </div>

          <!-- Speed Limit Presets -->
          <div class="space-y-3 py-2">
            <div>
              <span class="text-sm font-bold text-white font-mono">Ограничение скорости</span>
              <p class="text-xs text-[#64748b] font-mono">Выберите лимит входящего трафика</p>
            </div>

            <div class="grid grid-cols-2 sm:grid-cols-5 gap-2.5">
              <button
                data-nav-item
                class="py-3 px-3 rounded-xl text-xs font-mono font-bold cursor-pointer transition-all focus:ring-2 focus:ring-white focus:outline-none {localSettings.maxSpeedKBps === 0 ? 'bg-sky-500 text-slate-950 shadow-md' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                onclick={() => setSpeedLimit(0)}
              >
                Без лимита
              </button>

              <button
                data-nav-item
                class="py-3 px-3 rounded-xl text-xs font-mono font-bold cursor-pointer transition-all focus:ring-2 focus:ring-white focus:outline-none {localSettings.maxSpeedKBps === 10240 ? 'bg-sky-500 text-slate-950 shadow-md' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                onclick={() => setSpeedLimit(10240)}
              >
                10 МБ/с
              </button>

              <button
                data-nav-item
                class="py-3 px-3 rounded-xl text-xs font-mono font-bold cursor-pointer transition-all focus:ring-2 focus:ring-white focus:outline-none {localSettings.maxSpeedKBps === 25600 ? 'bg-sky-500 text-slate-950 shadow-md' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                onclick={() => setSpeedLimit(25600)}
              >
                25 МБ/с
              </button>

              <button
                data-nav-item
                class="py-3 px-3 rounded-xl text-xs font-mono font-bold cursor-pointer transition-all focus:ring-2 focus:ring-white focus:outline-none {localSettings.maxSpeedKBps === 51200 ? 'bg-sky-500 text-slate-950 shadow-md' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                onclick={() => setSpeedLimit(51200)}
              >
                50 МБ/с
              </button>

              <button
                data-nav-item
                class="py-3 px-3 rounded-xl text-xs font-mono font-bold cursor-pointer transition-all focus:ring-2 focus:ring-white focus:outline-none {localSettings.maxSpeedKBps === 102400 ? 'bg-sky-500 text-slate-950 shadow-md' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                onclick={() => setSpeedLimit(102400)}
              >
                100 МБ/с
              </button>
            </div>
          </div>
        </div>
      </div>

    <!-- ============================================================ -->
    <!-- 4. ABOUT & SYSTEM                                             -->
    <!-- ============================================================ -->
    {:else if activeCategory === 'about'}
      <div class="space-y-6">
        <div class="border-b border-white/[0.06] pb-4">
          <h1 class="text-xl font-bold uppercase tracking-wider text-white font-mono">О системе</h1>
          <p class="text-xs text-[#8e95a2] mt-1 font-mono">Информация о Ducke и управление приложением</p>
        </div>

        <div class="p-6 rounded-2xl bg-[#0d1017] border border-white/[0.08] space-y-4">
          <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
            <span class="text-sm font-bold text-white font-mono">Приложение</span>
            <span class="text-sm font-mono font-bold text-sky-400">{appInfo.name}</span>
          </div>

          <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
            <span class="text-sm font-bold text-white font-mono">Версия</span>
            <span class="text-sm font-mono text-[#8e95a2]">v{appInfo.version}</span>
          </div>

          <!-- Open Config Folder -->
          <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
            <div>
              <span class="text-sm font-bold text-white font-mono">Папка конфигурации и БД</span>
              <p class="text-xs text-[#64748b] font-mono">Файлы config.json и база данных ducke.db</p>
            </div>
            <button
              data-nav-item
              type="button"
              class="px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-1.5"
              onclick={() => {
                sound.playSelect();
                onOpenConfigFolder();
              }}
            >
              <FolderOpen class="w-3.5 h-3.5" />
              <span>Открыть</span>
            </button>
          </div>

          <!-- Clear Metadata Cache -->
          <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
            <div>
              <span class="text-sm font-bold text-white font-mono">Очистить кэш постеров</span>
              <p class="text-xs text-[#64748b] font-mono">Сбросить неверные сопоставления и перезапустить поиск</p>
            </div>
            <div class="flex items-center gap-3">
              {#if cacheClearMessage}
                <span class="text-xs font-mono text-emerald-400">{cacheClearMessage}</span>
              {/if}
              <button
                data-nav-item
                type="button"
                disabled={isClearingCache}
                class="px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-1.5 disabled:opacity-50"
                onclick={handleClearCache}
              >
                <RefreshCw class="w-3.5 h-3.5 {isClearingCache ? 'animate-spin' : ''}" />
                <span>Очистить кэш</span>
              </button>
            </div>
          </div>

          <!-- Logging toggle -->
          <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
            <div>
              <span class="text-sm font-bold text-white font-mono">Ведение журнала и логов</span>
              <p class="text-xs text-[#64748b] font-mono">Записывать диагностические сообщения и активировать вкладку «Логи»</p>
            </div>
            <button
              data-nav-item
              type="button"
              aria-label="Включить ведение логов"
              title="Включить ведение логов"
              class="w-12 h-6 rounded-full transition-colors cursor-pointer relative focus:ring-2 focus:ring-white focus:outline-none {localSettings.enableLogs ? 'bg-sky-500' : 'bg-white/10'}"
              onclick={() => {
                sound.playSelect();
                localSettings.enableLogs = !localSettings.enableLogs;
                triggerSave();
                if (localSettings.enableLogs) {
                  loadLogs();
                }
              }}
            >
              <span class="w-4 h-4 rounded-full bg-white transition-transform absolute top-1 {localSettings.enableLogs ? 'left-7' : 'left-1'}"></span>
            </button>
          </div>

          <!-- Actions -->
          <div class="pt-4 flex items-center gap-4">
            <button
              data-nav-item
              class="px-5 py-3 rounded-xl bg-white/10 hover:bg-white/20 focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold flex items-center gap-2 cursor-pointer transition-colors"
              onclick={() => {
                sound.playSelect();
                onSwitchToDesktop();
              }}
            >
              <Monitor class="w-4 h-4 text-sky-400" />
              <span>Режим рабочего стола</span>
            </button>

            <button
              data-nav-item
              class="px-5 py-3 rounded-xl bg-rose-500/20 hover:bg-rose-500/30 focus:ring-2 focus:ring-rose-400 focus:outline-none text-rose-300 text-xs font-mono font-bold flex items-center gap-2 cursor-pointer transition-colors"
              onclick={() => {
                sound.playBack();
                onCloseApp();
              }}
            >
              <Power class="w-4 h-4" />
              <span>Закрыть Ducke</span>
            </button>
          </div>
        </div>
      </div>

    <!-- ============================================================ -->
    <!-- 5. LOGS                                                       -->
    <!-- ============================================================ -->
    {:else if activeCategory === 'logs'}
      <div class="space-y-6 flex flex-col h-full min-h-[500px]">
        <div class="flex items-center justify-between border-b border-white/[0.06] pb-4">
          <div>
            <h1 class="text-xl font-bold uppercase tracking-wider text-white font-mono flex items-center gap-3">
              <Terminal class="w-6 h-6 text-sky-400" />
              <span>Журнал работы Ducke</span>
            </h1>
            <p class="text-xs text-[#8e95a2] mt-1 font-mono">Диагностические логи SFTP, загрузок и метаданных</p>
          </div>

          <div class="flex items-center gap-3">
            <button
              data-nav-item
              type="button"
              class="px-4 py-2.5 rounded-xl bg-white/10 hover:bg-white/20 focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-2"
              onclick={handleClearLogs}
            >
              <X class="w-4 h-4 text-rose-400" />
              <span>Очистить</span>
            </button>

            <button
              data-nav-item
              type="button"
              class="px-5 py-2.5 rounded-xl bg-sky-600 hover:bg-sky-500 focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-2 shadow-md"
              onclick={handleExportLogs}
            >
              <Download class="w-4 h-4" />
              <span>Экспорт в файл</span>
            </button>
          </div>
        </div>

        {#if exportMessage}
          <div class="p-3 rounded-xl bg-sky-500/10 border border-sky-500/30 text-xs font-mono text-sky-300 flex items-center justify-between">
            <span>{exportMessage}</span>
            <button onclick={() => (exportMessage = null)} class="text-white/40 hover:text-white p-1">
              <X class="w-4 h-4" />
            </button>
          </div>
        {/if}

        <!-- Filter & Search Controls (Gamepad Navigable) -->
        <div class="p-3 rounded-2xl bg-[#0d1017] border border-white/[0.08] flex flex-wrap items-center justify-between gap-3">
          <!-- Level Buttons -->
          <div class="flex items-center gap-2">
            {#each ['ALL', 'INFO', 'WARN', 'ERROR', 'DEBUG'] as lvl}
              <button
                data-nav-item
                type="button"
                class="px-3.5 py-1.5 rounded-lg text-xs font-mono font-bold uppercase transition-colors cursor-pointer focus:ring-2 focus:ring-white focus:outline-none {selectedLogLevel === lvl ? 'bg-white/20 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/5'}"
                onclick={() => {
                  sound.playFocus();
                  selectedLogLevel = lvl;
                }}
              >
                {lvl}
              </button>
            {/each}
          </div>

          <!-- Search & Auto-scroll -->
          <div class="flex items-center gap-3">
            <div class="relative w-48 sm:w-64">
              <input
                data-nav-item
                type="text"
                bind:value={logSearchQuery}
                placeholder="Поиск..."
                class="w-full bg-[#111622] border border-white/[0.08] px-3 py-1.5 text-xs font-mono text-white rounded-xl placeholder:text-white/30 focus:outline-none focus:ring-2 focus:ring-sky-400"
              />
              {#if logSearchQuery}
                <button
                  type="button"
                  class="absolute right-2 top-1/2 -translate-y-1/2 text-white/40 hover:text-white"
                  onclick={() => (logSearchQuery = '')}
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              {/if}
            </div>

            <button
              data-nav-item
              type="button"
              class="px-3 py-1.5 rounded-xl border border-white/10 text-xs font-mono transition-colors cursor-pointer focus:ring-2 focus:ring-white focus:outline-none {autoScrollLogs ? 'bg-sky-500/20 text-sky-300 border-sky-500/40' : 'bg-white/5 text-[#8e95a2]'}"
              onclick={() => {
                sound.playSelect();
                autoScrollLogs = !autoScrollLogs;
              }}
            >
              Автоскролл: {autoScrollLogs ? 'ВКЛ' : 'ВЫКЛ'}
            </button>
          </div>
        </div>

        <!-- Terminal Console View -->
        <div
          bind:this={logContainerEl}
          class="flex-1 min-h-[380px] max-h-[calc(100vh-340px)] overflow-y-auto bg-[#06080c] border border-white/[0.08] rounded-2xl p-4 font-mono text-xs select-text space-y-1"
        >
          {#if displayedLogs.length === 0}
            <div class="h-full min-h-[260px] flex flex-col items-center justify-center text-center text-[#64748b]">
              <Terminal class="w-10 h-10 stroke-1 mb-2 opacity-30" />
              <p class="text-sm">Журнал работы пуст</p>
            </div>
          {:else}
            {#each displayedLogs as entry (entry.id)}
              <div class="flex items-start gap-2.5 py-1 px-1.5 rounded-lg hover:bg-white/[0.04] transition-colors leading-relaxed break-all">
                <span class="text-[#64748b] text-xs flex-shrink-0 select-none">{entry.timestamp}</span>
                <span class="px-2 py-0.5 rounded text-[10px] font-bold flex-shrink-0 select-none {entry.level === 'ERROR' ? 'bg-rose-500/20 text-rose-300' : entry.level === 'WARN' ? 'bg-amber-500/20 text-amber-300' : entry.level === 'DEBUG' ? 'bg-purple-500/20 text-purple-300' : 'bg-sky-500/20 text-sky-300'}">
                  {entry.level}
                </span>
                <span class="text-slate-400 text-xs font-bold flex-shrink-0 select-none">[{entry.source}]</span>
                <span class="text-[#cbd5e1] whitespace-pre-wrap">{entry.message}</span>
              </div>
            {/each}
          {/if}
        </div>

        <div class="flex items-center justify-between text-xs font-mono text-[#64748b]">
          <span>Всего записей: {logEntries.length} (показано: {displayedLogs.length})</span>
          <span>Кольцевой буфер: 2000 записей</span>
        </div>
      </div>
    {/if}

  </main>
</div>
