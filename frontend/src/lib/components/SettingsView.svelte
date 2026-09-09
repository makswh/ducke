<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import {
    HardDrive,
    Server,
    DownloadCloud,
    Sliders,
    Info,
    Check,
    Save,
    Star,
    Plus,
    Trash2,
    RefreshCw,
    FolderOpen,
    Folder,
    Eye,
    EyeOff,
    ExternalLink,
    AlertCircle,
    CheckCircle2,
    Terminal,
    Download,
    X
  } from 'lucide-svelte';
  import * as AppAPI from '../../../wailsjs/go/main/App';
  import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime';

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
    onClearMetadataCache = async (): Promise<number> => 0
  } = $props();

  type SettingsTab = 'storage' | 'server' | 'downloads' | 'interface' | 'logs' | 'about';
  let activeSubTab = $state<SettingsTab>('storage');

  let localSettings = $state<AppSettings>({
    downloadPath: 'C:\\Ducke',
    maxConcurrentFiles: 4,
    maxSpeedKBps: 0,
    theme: 'dark',
    steamDeckMode: false,
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
  let isSaved = $state<boolean>(false);
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
    try {
      await AppAPI.ClearLogs();
      logEntries = [];
    } catch (e) {
      console.error(e);
    }
  }

  async function handleExportLogs() {
    try {
      const exportedPath = await AppAPI.ExportLogs('');
      if (exportedPath) {
        exportMessage = `Сохранено в: ${exportedPath}`;
        setTimeout(() => (exportMessage = null), 5000);
        return;
      }
    } catch (err) {
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

  // Speed limit presets in KB/s (0 = unlimited)
  const speedPresets = [
    { label: 'Без лимита', value: 0 },
    { label: '10 МБ/с', value: 10240 },
    { label: '25 МБ/с', value: 25600 },
    { label: '50 МБ/с', value: 51200 },
    { label: '100 МБ/с', value: 102400 }
  ];

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
          steamDeckMode: !!current.steamDeckMode,
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
      console.warn('Failed to load storage drives:', e);
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
      console.warn('Failed to get app info:', e);
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

  async function handleBrowseFolder() {
    const selected = await onSelectFolder();
    if (selected) {
      localSettings.downloadPath = selected;
      handleSave();
      loadStorageDrives();
    }
  }

  function handleSetDriveAsDefault(drive: StorageDrive) {
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
    handleSave();
    loadStorageDrives();
  }

  function handleAddServer() {
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
    handleSave();
  }

  function handleSelectServer(id: string) {
    selectedServerId = id;
    testResult = null;
  }

  function handleMakeServerActive(srv: ServerConfig) {
    localSettings.savedServers = localSettings.savedServers.map(s => ({
      ...s,
      isActive: s.id === srv.id
    }));
    localSettings.activeServer = { ...srv, isActive: true };
    handleSave();
  }

  function handleDeleteServer(srv: ServerConfig) {
    if (localSettings.savedServers.length <= 1) return;
    const remaining = localSettings.savedServers.filter(s => s.id !== srv.id);
    localSettings.savedServers = remaining;

    if (localSettings.activeServer?.id === srv.id && remaining.length > 0) {
      remaining[0].isActive = true;
      localSettings.activeServer = { ...remaining[0] };
    }

    selectedServerId = remaining[0]?.id || '';
    handleSave();
  }

  function handleServerFieldChange() {
    if (!selectedServer) return;
    // Keep savedServers and activeServer in sync
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
          message: res.errorMessage || 'Не удалось подключиться'
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
    isClearingCache = true;
    cacheClearMessage = null;
    try {
      const count = await onClearMetadataCache();
      cacheClearMessage = `Кэш успешно очищен (${count} записей перепроверено)`;
      setTimeout(() => {
        cacheClearMessage = null;
      }, 4000);
    } catch (e: any) {
      cacheClearMessage = `Ошибка очистки: ${e?.message || e}`;
    } finally {
      isClearingCache = false;
    }
  }

  function handleSave() {
    handleServerFieldChange();
    onSaveSettings(localSettings);
    isSaved = true;
    setTimeout(() => {
      isSaved = false;
    }, 2000);
  }

  function formatBytesReadable(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 МБ';
    const gb = bytes / (1024 * 1024 * 1024);
    if (gb >= 1) return `${gb.toFixed(2)} ГБ`;
    const mb = bytes / (1024 * 1024);
    return `${mb.toFixed(2)} МБ`;
  }
</script>

<!-- Steam Native Ascetic Settings Shell (Zero AI Cliches, High Contrast) -->
<div class="flex-1 flex h-full overflow-hidden bg-[#07080a] text-white font-sans select-none">
  
  <!-- Left Navigation Column -->
  <aside data-nav-zone="list" class="w-60 lg:w-64 flex flex-col flex-shrink-0 h-full border-r border-white/[0.06] bg-[#090c12]">
    <div class="px-6 py-6 border-b border-white/[0.04]">
      <span class="text-xs font-bold uppercase tracking-widest text-[#8e95a2] font-mono">Настройки Ducke</span>
    </div>

    <!-- Category Nav Items -->
    <nav class="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
      <button
        data-nav-item
        class="w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-xs transition-colors cursor-pointer text-left {activeSubTab === 'storage' ? 'bg-white/10 text-white font-bold' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => (activeSubTab = 'storage')}
      >
        <HardDrive class="w-4 h-4 flex-shrink-0 {activeSubTab === 'storage' ? 'text-sky-400' : ''}" />
        <span>Хранилище</span>
      </button>

      <button
        data-nav-item
        class="w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-xs transition-colors cursor-pointer text-left {activeSubTab === 'server' ? 'bg-white/10 text-white font-bold' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => (activeSubTab = 'server')}
      >
        <Server class="w-4 h-4 flex-shrink-0 {activeSubTab === 'server' ? 'text-sky-400' : ''}" />
        <span>Серверы</span>
      </button>

      <button
        data-nav-item
        class="w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-xs transition-colors cursor-pointer text-left {activeSubTab === 'downloads' ? 'bg-white/10 text-white font-bold' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => (activeSubTab = 'downloads')}
      >
        <DownloadCloud class="w-4 h-4 flex-shrink-0 {activeSubTab === 'downloads' ? 'text-sky-400' : ''}" />
        <span>Загрузки</span>
      </button>

      <button
        data-nav-item
        class="w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-xs transition-colors cursor-pointer text-left {activeSubTab === 'interface' ? 'bg-white/10 text-white font-bold' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => (activeSubTab = 'interface')}
      >
        <Sliders class="w-4 h-4 flex-shrink-0 {activeSubTab === 'interface' ? 'text-sky-400' : ''}" />
        <span>Интерфейс</span>
      </button>

      {#if localSettings.enableLogs}
        <button
          data-nav-item
          class="w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-xs transition-colors cursor-pointer text-left {activeSubTab === 'logs' ? 'bg-white/10 text-white font-bold' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
          onclick={() => {
            activeSubTab = 'logs';
            loadLogs();
          }}
        >
          <Terminal class="w-4 h-4 flex-shrink-0 {activeSubTab === 'logs' ? 'text-sky-400' : ''}" />
          <span>Логи</span>
          {#if logEntries.length > 0}
            <span class="ml-auto text-[10px] px-1.5 py-0.5 rounded-md bg-white/[0.06] text-[#8e95a2] font-mono">
              {logEntries.length}
            </span>
          {/if}
        </button>
      {/if}

      <button
        data-nav-item
        class="w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-xs transition-colors cursor-pointer text-left {activeSubTab === 'about' ? 'bg-white/10 text-white font-bold' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => (activeSubTab = 'about')}
      >
        <Info class="w-4 h-4 flex-shrink-0 {activeSubTab === 'about' ? 'text-sky-400' : ''}" />
        <span>О программе</span>
      </button>
    </nav>

    <!-- Save Feedback / Action -->
    <div class="p-3 border-t border-white/[0.04] space-y-2">
      <button
        data-nav-item
        class="w-full py-2.5 px-3 rounded-xl text-xs font-mono font-bold flex items-center justify-center gap-2 cursor-pointer transition-all active:scale-98 {isSaved ? 'bg-emerald-500 text-slate-950 shadow-md' : 'bg-white/10 text-white hover:bg-white/15'}"
        onclick={handleSave}
      >
        {#if isSaved}
          <Check class="w-4 h-4 stroke-[2.5]" />
          <span>СОХРАНЕНО</span>
        {:else}
          <Save class="w-4 h-4" />
          <span>СОХРАНИТЬ</span>
        {/if}
      </button>
    </div>
  </aside>

  <!-- Right Settings Content (Pure Flat Layout) -->
  <main data-nav-zone="detail" class="flex-1 flex flex-col h-full overflow-y-auto px-8 py-8 lg:px-12 max-w-4xl">
    
    <!-- 1. STORAGE -->
    {#if activeSubTab === 'storage'}
      <div class="space-y-8">
        <div class="flex items-center justify-between border-b border-white/[0.06] pb-4">
          <h1 class="text-base font-black uppercase tracking-wider text-white font-mono">Хранилище игр</h1>
          <button
            data-nav-item
            class="px-3.5 py-1.5 rounded-xl bg-white/5 hover:bg-white/10 text-xs font-mono text-[#cbd5e1] hover:text-white border border-white/[0.06] transition-colors cursor-pointer flex items-center gap-2"
            onclick={handleBrowseFolder}
          >
            <Folder class="w-3.5 h-3.5" />
            <span>Выбрать папку</span>
          </button>
        </div>

        <div class="space-y-8">
          {#each storageDrives as drive}
            {@const total = drive.totalBytes || 1}
            {@const duckePct = Math.min(100, Math.max(0, (drive.duckeBytes / total) * 100))}
            {@const otherBytes = Math.max(0, (drive.usedBytes - drive.duckeBytes))}
            {@const otherPct = Math.min(100 - duckePct, Math.max(0, (otherBytes / total) * 100))}

            <div class="space-y-3.5 pb-6 border-b border-white/[0.06]">
              <!-- Header Row -->
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2.5">
                  <HardDrive class="w-4 h-4 text-white" />
                  <span class="text-sm font-bold text-white font-mono">{drive.label}</span>
                  {#if drive.isDefault}
                    <span class="px-2 py-0.5 rounded-md bg-sky-500/20 text-sky-400 border border-sky-500/30 text-[10px] font-mono font-bold uppercase">
                      Основной
                    </span>
                  {/if}
                </div>

                <div class="text-xs font-mono font-bold uppercase tracking-wider text-[#8e95a2]">
                  <span class="text-white">{drive.freeGB}</span> свободно из {drive.totalGB}
                </div>
              </div>

              <!-- Path Label in Steam Style -->
              <div class="text-[11px] font-mono tracking-wider text-[#64748b]">
                {drive.isDefault ? localSettings.downloadPath : drive.path}
              </div>

              <!-- Steam Single Storage Bar -->
              <div class="w-full h-2 rounded-full bg-[#1e293b] overflow-hidden flex">
                {#if duckePct > 0}
                  <div class="h-full bg-[#38bdf8]" style="width: {duckePct}%;"></div>
                {/if}
                {#if otherPct > 0}
                  <div class="h-full bg-[#eab308]" style="width: {otherPct}%;"></div>
                {/if}
              </div>

              <!-- Steam Storage Legend -->
              <div class="flex flex-wrap items-center gap-x-6 gap-y-2 text-[11px] font-mono uppercase tracking-wider">
                {#if drive.duckeBytes > 0}
                  <div class="flex items-center gap-1.5 text-[#38bdf8]">
                    <span class="text-base leading-none">●</span>
                    <span class="text-white font-bold">DUCKE</span>
                    <span class="text-[#cbd5e1] font-normal lowercase">{drive.duckeGB}</span>
                  </div>
                {/if}

                <div class="flex items-center gap-1.5 text-[#eab308]">
                  <span class="text-base leading-none">●</span>
                  <span class="text-white font-bold">ДРУГИЕ ФАЙЛЫ</span>
                  <span class="text-[#cbd5e1] font-normal lowercase">{formatBytesReadable(otherBytes)}</span>
                </div>

                <div class="flex items-center gap-1.5 text-[#64748b]">
                  <span class="text-base leading-none">●</span>
                  <span class="text-white font-bold">СВОБОДНО</span>
                  <span class="text-[#cbd5e1] font-normal lowercase">{drive.freeGB}</span>
                </div>
              </div>

              <!-- Action Links -->
              <div class="pt-2 flex items-center gap-4 text-xs font-mono text-[#8e95a2]">
                {#if !drive.isDefault}
                  <button
                    data-nav-item
                    class="hover:text-white underline cursor-pointer"
                    onclick={() => handleSetDriveAsDefault(drive)}
                  >
                    Сделать основным
                  </button>
                {/if}

                <button
                  data-nav-item
                  class="hover:text-white underline cursor-pointer"
                  onclick={handleBrowseFolder}
                >
                  Выбрать папку
                </button>

                <button
                  data-nav-item
                  class="hover:text-white underline cursor-pointer"
                  onclick={() => onOpenFolder(drive.isDefault ? localSettings.downloadPath : drive.path)}
                >
                  Открыть в проводнике
                </button>
              </div>
            </div>
          {/each}
        </div>
      </div>

    <!-- 2. SERVERS -->
    {:else if activeSubTab === 'server'}
      <div class="space-y-6">
        <div class="flex items-center justify-between border-b border-white/[0.06] pb-4">
          <h1 class="text-base font-black uppercase tracking-wider text-white font-mono">FTP / SFTP Серверы</h1>

          <div class="flex items-center gap-3">
            <button
              data-nav-item
              class="px-3 py-1.5 rounded-xl bg-white/5 hover:bg-white/10 text-xs font-mono text-[#cbd5e1] hover:text-white border border-white/[0.06] transition-colors cursor-pointer"
              onclick={onImportFile}
            >
              Импорт из FileZilla
            </button>

            <button
              data-nav-item
              class="px-3 py-1.5 rounded-xl bg-sky-500 hover:bg-sky-400 text-slate-950 text-xs font-mono font-bold transition-colors cursor-pointer flex items-center gap-1.5 shadow-sm"
              onclick={handleAddServer}
            >
              <Plus class="w-3.5 h-3.5" />
              <span>Добавить</span>
            </button>
          </div>
        </div>

        <!-- Server Selector Tabs -->
        {#if localSettings.savedServers.length > 0}
          <div class="flex items-center gap-2 overflow-x-auto pb-2">
            {#each localSettings.savedServers as srv (srv.id)}
              {@const isSelected = selectedServer?.id === srv.id}
              {@const isActive = localSettings.activeServer?.id === srv.id}
              <button
                data-nav-item
                class="px-3.5 py-2 rounded-xl text-xs font-mono border transition-all cursor-pointer flex items-center gap-2 flex-shrink-0 {isSelected ? 'bg-white/10 border-white/30 text-white font-bold' : 'bg-[#0c1017] border-white/[0.06] text-[#8e95a2] hover:text-white'}"
                onclick={() => handleSelectServer(srv.id)}
              >
                <span>{srv.name || srv.host || 'Сервер'}</span>
                {#if isActive}
                  <span class="w-2 h-2 rounded-full bg-emerald-400" title="Активный сервер"></span>
                {/if}
              </button>
            {/each}
          </div>
        {/if}

        <!-- Selected Server Editor -->
        {#if selectedServer}
          <div class="space-y-4 pt-1">
            
            <!-- Top Controls for Selected Server -->
            <div class="flex items-center justify-between pb-3 border-b border-white/[0.06]">
              <div class="flex items-center gap-2">
                {#if localSettings.activeServer?.id === selectedServer.id}
                  <div class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-emerald-500/15 border border-emerald-500/30 text-emerald-400 text-xs font-mono font-bold">
                    <CheckCircle2 class="w-3.5 h-3.5" />
                    <span>АКТИВНЫЙ СЕРВЕР</span>
                  </div>
                {:else}
                  <button
                    data-nav-item
                    class="px-3 py-1 rounded-xl bg-sky-500/15 text-sky-400 hover:bg-sky-500/25 border border-sky-500/30 text-xs font-mono font-bold cursor-pointer transition-colors"
                    onclick={() => handleMakeServerActive(selectedServer)}
                  >
                    Сделать активным
                  </button>
                {/if}
              </div>

              {#if localSettings.savedServers.length > 1}
                <button
                  data-nav-item
                  class="px-2.5 py-1 rounded-lg text-xs font-mono text-[#8e95a2] hover:text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer flex items-center gap-1.5"
                  onclick={() => handleDeleteServer(selectedServer)}
                  title="Удалить этот сервер"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                  <span>Удалить</span>
                </button>
              {/if}
            </div>

            <!-- Server Name -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Название</span>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.name}
                onchange={handleServerFieldChange}
                placeholder="Основной SFTP"
                class="w-72 bg-transparent text-right text-white font-mono focus:outline-none focus:text-sky-400"
              />
            </div>

            <!-- Protocol -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Протокол</span>
              <div class="flex items-center gap-2">
                <button
                  data-nav-item
                  type="button"
                  class="px-3 py-1 text-xs rounded-lg cursor-pointer transition-colors font-mono {selectedServer.protocol === 'sftp' ? 'bg-sky-500 text-slate-950 font-bold' : 'text-[#8e95a2] hover:text-white bg-white/5'}"
                  onclick={() => {
                    if (selectedServer) {
                      selectedServer.protocol = 'sftp';
                      if (selectedServer.port === 21) selectedServer.port = 2022;
                      handleServerFieldChange();
                    }
                  }}
                >
                  SFTP
                </button>
                <button
                  data-nav-item
                  type="button"
                  class="px-3 py-1 text-xs rounded-lg cursor-pointer transition-colors font-mono {selectedServer.protocol === 'ftp' ? 'bg-sky-500 text-slate-950 font-bold' : 'text-[#8e95a2] hover:text-white bg-white/5'}"
                  onclick={() => {
                    if (selectedServer) {
                      selectedServer.protocol = 'ftp';
                      if (selectedServer.port === 2022) selectedServer.port = 21;
                      handleServerFieldChange();
                    }
                  }}
                >
                  FTP
                </button>
              </div>
            </div>

            <!-- Host -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Хост / IP адрес</span>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.host}
                onchange={handleServerFieldChange}
                placeholder="192.168.1.100"
                class="w-72 bg-transparent text-right text-white font-mono focus:outline-none focus:text-sky-400"
              />
            </div>

            <!-- Port -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Порт</span>
              <input
                data-nav-item
                type="number"
                bind:value={selectedServer.port}
                onchange={handleServerFieldChange}
                class="w-24 bg-transparent text-right text-white font-mono focus:outline-none focus:text-sky-400"
              />
            </div>

            <!-- User -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Пользователь</span>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.user}
                onchange={handleServerFieldChange}
                placeholder="anonymous"
                class="w-72 bg-transparent text-right text-white font-mono focus:outline-none focus:text-sky-400"
              />
            </div>

            <!-- Password -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Пароль</span>
              <div class="flex items-center gap-2">
                {#if showPassword}
                  <input
                    data-nav-item
                    type="text"
                    bind:value={selectedServer.password}
                    onchange={handleServerFieldChange}
                    placeholder="••••••••"
                    class="w-64 bg-transparent text-right text-white font-mono focus:outline-none focus:text-sky-400"
                  />
                {:else}
                  <input
                    data-nav-item
                    type="password"
                    bind:value={selectedServer.password}
                    onchange={handleServerFieldChange}
                    placeholder="••••••••"
                    class="w-64 bg-transparent text-right text-white font-mono focus:outline-none focus:text-sky-400"
                  />
                {/if}
                <button
                  type="button"
                  class="text-[#8e95a2] hover:text-white cursor-pointer p-1"
                  onclick={() => (showPassword = !showPassword)}
                  title={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
                >
                  {#if showPassword}
                    <EyeOff class="w-4 h-4" />
                  {:else}
                    <Eye class="w-4 h-4" />
                  {/if}
                </button>
              </div>
            </div>

            <!-- Remote Dir -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Удаленный каталог</span>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.remoteDir}
                onchange={handleServerFieldChange}
                placeholder="/public"
                class="w-72 bg-transparent text-right text-white font-mono focus:outline-none focus:text-sky-400"
              />
            </div>

            <!-- Connection Test -->
            <div class="pt-3 flex items-center justify-between">
              <button
                data-nav-item
                disabled={isTestingConnection}
                class="px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 active:bg-white/25 text-xs font-mono font-bold text-white cursor-pointer flex items-center gap-2 disabled:opacity-50 transition-colors"
                onclick={handleTestConn}
              >
                {#if isTestingConnection}
                  <RefreshCw class="w-3.5 h-3.5 animate-spin" />
                  <span>Проверка...</span>
                {:else}
                  <span>Проверить соединение</span>
                {/if}
              </button>

              {#if testResult}
                <span class="text-xs font-mono {testResult.success ? 'text-emerald-400' : 'text-rose-400'}">
                  {testResult.message}
                </span>
              {/if}
            </div>
          </div>
        {/if}
      </div>

    <!-- 3. DOWNLOADS -->
    {:else if activeSubTab === 'downloads'}
      <div class="space-y-6">
        <h1 class="text-base font-black uppercase tracking-wider text-white font-mono border-b border-white/[0.06] pb-4">Параметры загрузок</h1>

        <div class="space-y-6 pt-1">
          <!-- Download Folder -->
          <div class="space-y-2 pb-4 border-b border-white/[0.06]">
            <span class="text-xs text-[#cbd5e1] font-mono">Папка для скачивания по умолчанию</span>
            <div class="flex items-center gap-3">
              <input
                type="text"
                readonly
                value={localSettings.downloadPath}
                class="flex-1 bg-[#0c1017] border border-white/[0.08] px-3.5 py-2 rounded-xl text-xs font-mono text-white select-all"
              />
              <button
                data-nav-item
                class="px-3.5 py-2 rounded-xl bg-white/10 hover:bg-white/20 text-xs font-mono font-bold text-white transition-colors cursor-pointer"
                onclick={handleBrowseFolder}
              >
                Обзор
              </button>
            </div>
          </div>

          <!-- Concurrent Files -->
          <div class="flex items-center justify-between py-2 border-b border-white/[0.06] text-xs">
            <div>
              <span class="text-[#cbd5e1] font-mono block">Одновременных файлов в раздаче</span>
              <span class="text-[11px] text-[#64748b] font-mono">Количество файлов, загружаемых параллельно в рамках игры</span>
            </div>
            <div class="flex items-center gap-3">
              <input
                data-nav-item
                type="range"
                min="1"
                max="8"
                bind:value={localSettings.maxConcurrentFiles}
                onchange={handleSave}
                class="w-36 accent-sky-400 cursor-pointer"
              />
              <span class="font-mono font-bold text-white w-5 text-right text-sm">{localSettings.maxConcurrentFiles}</span>
            </div>
          </div>

          <!-- Speed Limit -->
          <div class="space-y-3 py-2 border-b border-white/[0.06]">
            <div class="flex items-center justify-between text-xs">
              <span class="text-[#cbd5e1] font-mono">Ограничение скорости</span>
              <div class="flex items-center gap-2">
                <input
                  data-nav-item
                  type="number"
                  min="0"
                  step="512"
                  bind:value={localSettings.maxSpeedKBps}
                  onchange={handleSave}
                  class="w-24 bg-[#0c1017] border border-white/[0.08] px-2.5 py-1 rounded-lg text-right text-white font-mono text-xs focus:outline-none focus:border-sky-400"
                />
                <span class="text-xs font-mono text-[#8e95a2]">КБ/с</span>
              </div>
            </div>

            <!-- Quick Presets -->
            <div class="flex items-center gap-2 flex-wrap">
              {#each speedPresets as preset}
                <button
                  data-nav-item
                  class="px-3 py-1.5 rounded-xl text-xs font-mono transition-colors cursor-pointer {localSettings.maxSpeedKBps === preset.value ? 'bg-sky-500 text-slate-950 font-bold' : 'bg-white/5 hover:bg-white/10 text-[#8e95a2] hover:text-white'}"
                  onclick={() => {
                    localSettings.maxSpeedKBps = preset.value;
                    handleSave();
                  }}
                >
                  {preset.label}
                </button>
              {/each}
            </div>

            <p class="text-[11px] font-mono text-[#64748b]">
              Новый лимит применяется динамически ко всем активным сетевым потокам без перезапуска.
            </p>
          </div>
        </div>
      </div>

    <!-- 4. INTERFACE & BEHAVIOR -->
    {:else if activeSubTab === 'interface'}
      <div class="space-y-6">
        <h1 class="text-base font-black uppercase tracking-wider text-white font-mono border-b border-white/[0.06] pb-4">Интерфейс и интеграции</h1>

        <div class="space-y-6 pt-1">
          <!-- Steam Deck / Big Picture Mode -->
          <div class="flex items-center justify-between py-2 border-b border-white/[0.06] text-xs">
            <div>
              <span class="text-[#cbd5e1] font-mono block">Режим Steam Deck</span>
              <span class="text-[11px] text-[#64748b] font-mono">Оптимизировать интерфейс под управление геймпадом</span>
            </div>
            <button
              data-nav-item
              type="button"
              aria-label="Режим Steam Deck"
              title="Режим Steam Deck"
              class="w-12 h-6 rounded-full transition-colors cursor-pointer relative {localSettings.steamDeckMode ? 'bg-sky-500' : 'bg-white/10'}"
              onclick={() => {
                localSettings.steamDeckMode = !localSettings.steamDeckMode;
                handleSave();
              }}
            >
              <span class="w-4 h-4 rounded-full bg-white transition-transform absolute top-1 {localSettings.steamDeckMode ? 'left-7' : 'left-1'}"></span>
            </button>
          </div>

          <!-- Steam Web API Key -->
          <div class="space-y-2 py-2 border-b border-white/[0.06] text-xs">
            <div class="flex items-center justify-between">
              <div>
                <span class="text-[#cbd5e1] font-mono block">Ключ Steam Web API (необязательно)</span>
                <span class="text-[11px] text-[#64748b] font-mono">Для расширенного поиска карточек и фонов сообщества</span>
              </div>
              <a
                href="https://steamcommunity.com/dev/apikey"
                target="_blank"
                class="text-xs font-mono text-sky-400 hover:underline flex items-center gap-1"
              >
                <span>Получить ключ</span>
                <ExternalLink class="w-3 h-3" />
              </a>
            </div>
            <input
              data-nav-item
              type="text"
              bind:value={localSettings.steamApiKey}
              onchange={handleSave}
              placeholder="32-значный ключ Steam API..."
              class="w-full bg-[#0c1017] border border-white/[0.08] px-3.5 py-2 rounded-xl text-xs font-mono text-white focus:outline-none focus:border-sky-400"
            />
          </div>

          <!-- Clear Metadata Cache -->
          <div class="space-y-3 py-2 border-b border-white/[0.06] text-xs">
            <div>
              <span class="text-[#cbd5e1] font-mono block">Кэш метаданных игр</span>
              <span class="text-[11px] text-[#64748b] font-mono">Сбросить несовпадающие сопоставления и перезапустить поиск постеров Steam / SteamGridDB</span>
            </div>
            <div class="flex items-center gap-4">
              <button
                data-nav-item
                disabled={isClearingCache}
                class="px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 active:bg-white/25 text-xs font-mono font-bold text-white cursor-pointer flex items-center gap-2 disabled:opacity-50 transition-colors"
                onclick={handleClearCache}
              >
                <RefreshCw class="w-3.5 h-3.5 {isClearingCache ? 'animate-spin' : ''}" />
                <span>Очистить кэш метаданных</span>
              </button>

              {#if cacheClearMessage}
                <span class="text-xs font-mono text-emerald-400">{cacheClearMessage}</span>
              {/if}
            </div>
          </div>

          <!-- Logging toggle -->
          <div class="flex items-center justify-between py-2 border-b border-white/[0.06] text-xs">
            <div>
              <span class="text-[#cbd5e1] font-mono block">Ведение журнала и логов</span>
              <span class="text-[11px] text-[#64748b] font-mono">Записывать диагностические сообщения и активировать вкладку «Логи»</span>
            </div>
            <button
              data-nav-item
              type="button"
              aria-label="Включить ведение логов"
              title="Включить ведение логов"
              class="w-12 h-6 rounded-full transition-colors cursor-pointer relative {localSettings.enableLogs ? 'bg-sky-500' : 'bg-white/10'}"
              onclick={() => {
                localSettings.enableLogs = !localSettings.enableLogs;
                handleSave();
                if (localSettings.enableLogs) {
                  loadLogs();
                }
              }}
            >
              <span class="w-4 h-4 rounded-full bg-white transition-transform absolute top-1 {localSettings.enableLogs ? 'left-7' : 'left-1'}"></span>
            </button>
          </div>
        </div>
      </div>

    <!-- LOGS -->
    {:else if activeSubTab === 'logs'}
      <div class="h-full flex flex-col space-y-4">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-white/[0.06] pb-4">
          <div>
            <h1 class="text-base font-black uppercase tracking-wider text-white font-mono flex items-center gap-2">
              <Terminal class="w-4 h-4 text-sky-400" />
              <span>Журнал работы (Логи)</span>
            </h1>
            <p class="text-[11px] text-[#64748b] font-mono mt-0.5">
              Диагностика сети, загрузчика, метаданных и системы
            </p>
          </div>

          <div class="flex items-center gap-2 flex-wrap">
            <button
              data-nav-item
              class="px-3 py-1.5 rounded-lg bg-white/10 hover:bg-white/15 text-xs font-mono font-bold text-white cursor-pointer flex items-center gap-1.5 transition-colors"
              onclick={handleClearLogs}
              title="Очистить буфер логов"
            >
              <X class="w-3.5 h-3.5 text-rose-400" />
              <span>Очистить</span>
            </button>

            <button
              data-nav-item
              class="px-3.5 py-1.5 rounded-lg bg-sky-600 hover:bg-sky-500 text-xs font-mono font-bold text-white cursor-pointer flex items-center gap-1.5 transition-colors shadow-sm"
              onclick={handleExportLogs}
              title="Экспорт в текстовый файл"
            >
              <Download class="w-3.5 h-3.5" />
              <span>Экспорт в файл</span>
            </button>
          </div>
        </div>

        {#if exportMessage}
          <div class="p-2.5 rounded-lg bg-white/5 border border-sky-500/30 text-xs font-mono text-sky-300 flex items-center justify-between">
            <span>{exportMessage}</span>
            <button onclick={() => (exportMessage = null)} class="text-white/40 hover:text-white">
              <X class="w-3.5 h-3.5" />
            </button>
          </div>
        {/if}

        <!-- Filter bar -->
        <div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-2.5 bg-[#0c1017] p-2.5 rounded-xl border border-white/[0.06]">
          <!-- Level filters -->
          <div class="flex items-center gap-1 overflow-x-auto">
            {#each ['ALL', 'INFO', 'WARN', 'ERROR', 'DEBUG'] as lvl}
              <button
                data-nav-item
                class="px-2.5 py-1 rounded-md text-[11px] font-mono font-bold uppercase transition-colors cursor-pointer {selectedLogLevel === lvl ? 'bg-white/15 text-white' : 'text-[#8e95a2] hover:text-white hover:bg-white/5'}"
                onclick={() => (selectedLogLevel = lvl)}
              >
                {lvl}
              </button>
            {/each}
          </div>

          <!-- Search & Auto-scroll -->
          <div class="flex items-center gap-2 flex-1 sm:max-w-xs">
            <div class="relative flex-1">
              <input
                data-nav-item
                type="text"
                bind:value={logSearchQuery}
                placeholder="Поиск по логам..."
                class="w-full bg-[#111622] border border-white/[0.08] px-2.5 py-1 text-[11px] font-mono text-white rounded-md placeholder:text-white/30 focus:outline-none focus:border-sky-400"
              />
              {#if logSearchQuery}
                <button
                  class="absolute right-1.5 top-1/2 -translate-y-1/2 text-white/40 hover:text-white p-0.5"
                  onclick={() => (logSearchQuery = '')}
                >
                  <X class="w-3 h-3" />
                </button>
              {/if}
            </div>

            <label class="flex items-center gap-1.5 text-[11px] font-mono text-[#8e95a2] cursor-pointer whitespace-nowrap select-none">
              <input
                type="checkbox"
                bind:checked={autoScrollLogs}
                class="rounded border-white/20 bg-white/5 text-sky-500 focus:ring-0 cursor-pointer"
              />
              <span>Автоскролл</span>
            </label>
          </div>
        </div>

        <!-- Terminal Log Viewer -->
        <div
          bind:this={logContainerEl}
          class="flex-1 min-h-[320px] max-h-[calc(100vh-280px)] overflow-y-auto bg-[#06080c] border border-white/[0.08] rounded-xl p-3 font-mono text-xs select-text space-y-1"
        >
          {#if displayedLogs.length === 0}
            <div class="h-full min-h-[200px] flex flex-col items-center justify-center text-center text-[#64748b]">
              <Terminal class="w-8 h-8 stroke-1 mb-2 opacity-40" />
              <p class="text-xs">Записей в журнале не обнаружено</p>
              {#if logSearchQuery || selectedLogLevel !== 'ALL'}
                <button
                  class="mt-2 text-[11px] text-sky-400 hover:underline cursor-pointer"
                  onclick={() => {
                    logSearchQuery = '';
                    selectedLogLevel = 'ALL';
                  }}
                >
                  Сбросить фильтры
                </button>
              {/if}
            </div>
          {:else}
            {#each displayedLogs as entry (entry.id)}
              <div class="flex items-start gap-2 py-0.5 px-1 rounded hover:bg-white/[0.03] transition-colors leading-relaxed break-all">
                <span class="text-[#64748b] text-[11px] flex-shrink-0 select-none">{entry.timestamp}</span>
                <span class="px-1.5 py-0.2 rounded text-[10px] font-bold flex-shrink-0 select-none {entry.level === 'ERROR' ? 'bg-rose-500/20 text-rose-300' : entry.level === 'WARN' ? 'bg-amber-500/20 text-amber-300' : entry.level === 'DEBUG' ? 'bg-purple-500/20 text-purple-300' : 'bg-sky-500/20 text-sky-300'}">
                  {entry.level}
                </span>
                <span class="text-slate-400 text-[11px] font-bold flex-shrink-0 select-none">[{entry.source}]</span>
                <span class="text-[#cbd5e1] whitespace-pre-wrap">{entry.message}</span>
              </div>
            {/each}
          {/if}
        </div>

        <div class="flex items-center justify-between text-[11px] font-mono text-[#64748b] pt-1">
          <span>Всего записей: {logEntries.length} (показано: {displayedLogs.length})</span>
          <span>Буфер: макс. 2000 записей</span>
        </div>
      </div>

    <!-- 5. ABOUT -->
    {:else if activeSubTab === 'about'}
      <div class="space-y-6">
        <h1 class="text-base font-black uppercase tracking-wider text-white font-mono border-b border-white/[0.06] pb-4">О программе</h1>

        <div class="space-y-4 pt-1">
          <div class="flex items-center justify-between py-3 border-b border-white/[0.06] text-xs font-mono">
            <span class="text-[#8e95a2]">Приложение</span>
            <span class="font-bold text-white">{appInfo.name}</span>
          </div>

          <div class="flex items-center justify-between py-3 border-b border-white/[0.06] text-xs font-mono">
            <span class="text-[#8e95a2]">Версия</span>
            <span class="text-white font-semibold">v{appInfo.version}</span>
          </div>

          <div class="flex items-center justify-between py-3 border-b border-white/[0.06] text-xs font-mono">
            <div>
              <span class="text-[#8e95a2] block">Папка конфигурации и БД</span>
              <span class="text-[11px] text-[#64748b]">Настройки config.json и база данных ducke.db</span>
            </div>
            <button
              data-nav-item
              class="px-3.5 py-1.5 rounded-xl bg-white/10 hover:bg-white/20 text-white text-xs font-mono font-bold transition-colors cursor-pointer flex items-center gap-1.5"
              onclick={onOpenConfigFolder}
            >
              <FolderOpen class="w-3.5 h-3.5" />
              <span>Открыть папку</span>
            </button>
          </div>
        </div>
      </div>
    {/if}

  </main>
</div>
