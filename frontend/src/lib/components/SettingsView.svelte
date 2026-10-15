<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import {
    Gear,
    HardDrive,
    Database,
    CloudArrowDown,
    CloudArrowDown as DownloadCloud,
    Sliders,
    Info,
    Check,
    FloppyDisk,
    FloppyDisk as Save,
    Plus,
    Trash,
    Trash as Trash2,
    ArrowClockwise,
    ArrowClockwise as RefreshCw,
    FolderOpen,
    Folder,
    Eye,
    EyeSlash,
    EyeSlash as EyeOff,
    ArrowSquareOut,
    ArrowSquareOut as ExternalLink,
    Warning,
    CheckCircle,
    CheckCircle as CheckCircle2,
    Terminal,
    DownloadSimple,
    DownloadSimple as Download,
    Magnet,
    X
  } from 'phosphor-svelte';
  import * as AppAPI from '../../../wailsjs/go/main/App';
  import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime';
  import TorrentSourcesManager from './TorrentSourcesManager.svelte';

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
    autoExtractArchives?: boolean;
    activeServer?: ServerConfig;
    savedServers: ServerConfig[];
    torrentSources?: any[];
  }

  export type SettingsTab = 'storage' | 'server' | 'torrents' | 'downloads' | 'interface' | 'logs' | 'about';

  let {
    settings = null as AppSettings | null,
    activeSubTab = $bindable('storage' as SettingsTab),
    onSaveSettings = (s: AppSettings) => {},
    onImportFile = async () => {},
    onTestConnection = async (srv: ServerConfig): Promise<{ success: boolean; protocolUsed?: string; errorMessage?: string }> => ({ success: false }),
    onSelectFolder = async (): Promise<string> => '',
    onOpenFolder = (path: string) => {},
    onOpenConfigFolder = async () => {},
    onClearMetadataCache = async (): Promise<number> => 0
  } = $props();

  $effect(() => {
    if (activeSubTab === 'logs') {
      loadLogs();
    }
  });

  let localSettings = $state<AppSettings>({
    downloadPath: 'C:\\Ducke',
    maxConcurrentFiles: 4,
    maxSpeedKBps: 0,
    theme: 'dark',
    steamDeckMode: false,
    steamApiKey: '',
    enableLogs: false,
    autoExtractArchives: true,
    savedServers: [],
    torrentSources: [],
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
  let appInfo = $state<{ name: string; version: string }>({ name: 'Ducke', version: '1.2.0' });
  let storageDrives = $state<StorageDrive[]>([]);
  let isLoadingDrives = $state<boolean>(true);
  let isTestingConnection = $state<boolean>(false);
  let testResult = $state<{ success: boolean; message: string } | null>(null);
  let isSaved = $state<boolean>(false);
  let showPassword = $state<boolean>(false);
  let isClearingCache = $state<boolean>(false);
  let cacheClearMessage = $state<string | null>(null);

  // Updater state
  interface UpdateInfo {
    currentVersion: string;
    latestVersion: string;
    hasUpdate: boolean;
    releaseNotes: string;
    releaseName: string;
    releaseDate?: string;
    releaseUrl?: string;
    downloadUrl?: string;
    assetName?: string;
    assetSize?: number;
  }

  interface UpdateProgress {
    downloadedBytes: number;
    totalBytes: number;
    percent: number;
    speedBytesPerSec: number;
    done: boolean;
    error?: string;
  }

  let isCheckingUpdate = $state<boolean>(false);
  let updateCheckResult = $state<UpdateInfo | null>(null);
  let updateCheckMessage = $state<string | null>(null);
  let updateProgress = $state<UpdateProgress | null>(null);
  let isDownloadingUpdate = $state<boolean>(false);
  let isUpdateReadyToInstall = $state<boolean>(false);
  let updateErrorMessage = $state<string | null>(null);

  async function handleCheckUpdates() {
    isCheckingUpdate = true;
    updateCheckMessage = null;
    updateErrorMessage = null;
    try {
      let res: any = null;
      if (typeof (AppAPI as any)?.CheckForUpdates === 'function') {
        res = await (AppAPI as any).CheckForUpdates();
      } else if (typeof (window as any)?.go?.main?.App?.CheckForUpdates === 'function') {
        res = await (window as any).go.main.App.CheckForUpdates();
      }

      if (res) {
        updateCheckResult = res;
        if (!res.hasUpdate) {
          updateCheckMessage = `Установлена актуальная версия (v${res.currentVersion || appInfo.version})`;
        }
      } else {
        updateCheckMessage = 'Нет доступных обновлений';
      }
    } catch (e: any) {
      console.warn('Check updates error:', e);
      updateErrorMessage = e?.message || 'Не удалось связаться с сервером обновлений';
    } finally {
      isCheckingUpdate = false;
    }
  }

  async function handleStartDownloadUpdate() {
    isDownloadingUpdate = true;
    updateErrorMessage = null;
    updateProgress = {
      downloadedBytes: 0,
      totalBytes: updateCheckResult?.assetSize || 0,
      percent: 0,
      speedBytesPerSec: 0,
      done: false
    };

    try {
      if (typeof (AppAPI as any)?.StartUpdateDownload === 'function') {
        await (AppAPI as any).StartUpdateDownload();
      } else if (typeof (window as any)?.go?.main?.App?.StartUpdateDownload === 'function') {
        await (window as any).go.main.App.StartUpdateDownload();
      }
    } catch (e: any) {
      isDownloadingUpdate = false;
      updateErrorMessage = e?.message || 'Ошибка запуска загрузки';
    }
  }

  async function handleApplyUpdate() {
    try {
      if (typeof (AppAPI as any)?.ApplyUpdateAndRestart === 'function') {
        await (AppAPI as any).ApplyUpdateAndRestart();
      } else if (typeof (window as any)?.go?.main?.App?.ApplyUpdateAndRestart === 'function') {
        await (window as any).go.main.App.ApplyUpdateAndRestart();
      }
    } catch (e: any) {
      updateErrorMessage = e?.message || 'Ошибка применения обновления';
    }
  }

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
          autoExtractArchives: current.autoExtractArchives !== undefined ? !!current.autoExtractArchives : true,
          savedServers,
          activeServer,
          torrentSources: Array.isArray(current.torrentSources) ? current.torrentSources.map((s: any) => ({ ...s })) : []
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
    isLoadingDrives = true;
    try {
      let drives: any = null;
      if (typeof (AppAPI as any)?.GetStorageDrives === 'function') {
        drives = await (AppAPI as any).GetStorageDrives();
      } else if (typeof (window as any)?.go?.main?.App?.GetStorageDrives === 'function') {
        drives = await (window as any).go.main.App.GetStorageDrives();
      }
      if (Array.isArray(drives) && drives.length > 0) {
        storageDrives = drives;
      }
    } catch (e) {
      console.warn('Failed to load storage drives:', e);
    } finally {
      isLoadingDrives = false;
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

    const unsubUpdateAvail = EventsOn('update:available', (info: any) => {
      if (info) {
        updateCheckResult = info;
      }
    });

    const unsubUpdateProg = EventsOn('update:progress', (prog: any) => {
      if (prog) {
        updateProgress = prog;
        if (prog.done) {
          isDownloadingUpdate = false;
          isUpdateReadyToInstall = true;
        }
      }
    });

    const unsubUpdateErr = EventsOn('update:error', (err: any) => {
      isDownloadingUpdate = false;
      updateErrorMessage = typeof err === 'string' ? err : 'Ошибка загрузки обновления';
    });

    return () => {
      EventsOff('log:entry');
      if (typeof unsubUpdateAvail === 'function') unsubUpdateAvail();
      if (typeof unsubUpdateProg === 'function') unsubUpdateProg();
      if (typeof unsubUpdateErr === 'function') unsubUpdateErr();
    };
  });

  async function handleBrowseFolder() {
    let selected = await onSelectFolder();
    if (selected) {
      // Normalize drive roots (e.g. "F:\" -> "F:\Ducke")
      if (/^[a-zA-Z]:\\?$/.test(selected)) {
        selected = `${selected[0].toUpperCase()}:\\Ducke`;
      }
      localSettings.downloadPath = selected;
      handleSave();
      loadStorageDrives();
    }
  }

  function handleSetDriveAsDefault(drive: StorageDrive) {
    let target = drive.path;
    // Normalize Windows drive roots (e.g. "C:\", "D:\", "F:\", "C:")
    if (/^[a-zA-Z]:\\?$/.test(target)) {
      const letter = target[0].toUpperCase();
      target = `${letter}:\\Ducke`;
    } else if (drive.type === 'internal' && (target === '/home' || target === '/' || target.startsWith('/home/'))) {
      target = '/home/deck/Games/Ducke';
    } else if (drive.type === 'sdcard' || drive.type === 'removable') {
      target = `${drive.path.replace(/\/+$/, '')}/Games/Ducke`;
    } else {
      target = `${drive.path.replace(/[\\/]+$/, '')}/Ducke`;
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

  function sanitizeServerInput(srv: ServerConfig | null) {
    if (!srv || !srv.host) return;
    let raw = srv.host.trim();
    if (!raw) return;

    const lower = raw.toLowerCase();
    if (lower.startsWith('ftp://')) {
      srv.protocol = 'ftp';
      if (!srv.port || srv.port === 2022) srv.port = 21;
      raw = raw.slice(6);
    } else if (lower.startsWith('ftps://')) {
      srv.protocol = 'ftps';
      if (!srv.port || srv.port === 2022) srv.port = 990;
      raw = raw.slice(7);
    } else if (lower.startsWith('sftp://')) {
      srv.protocol = 'sftp';
      if (!srv.port || srv.port === 21) srv.port = 2022;
      raw = raw.slice(7);
    } else if (lower.startsWith('http://')) {
      raw = raw.slice(7);
    } else if (lower.startsWith('https://')) {
      raw = raw.slice(8);
    }

    // Extract path if present (e.g. ftp.matrixgames.com/pub/ or server/dir)
    const slashIdx = raw.indexOf('/');
    const backslashIdx = raw.indexOf('\\');
    const splitIdx = slashIdx !== -1 ? slashIdx : backslashIdx;
    if (splitIdx !== -1) {
      const extractedPath = raw.slice(splitIdx).replace(/\\/g, '/');
      raw = raw.slice(0, splitIdx);
      if (!srv.remoteDir || srv.remoteDir === '/' || srv.remoteDir === '.' || srv.remoteDir === '0') {
        srv.remoteDir = extractedPath.replace(/\/+$/, '') || '/';
      }
    }

    // Extract credentials user:pass@host
    const atIdx = raw.lastIndexOf('@');
    if (atIdx !== -1) {
      const creds = raw.slice(0, atIdx);
      raw = raw.slice(atIdx + 1);
      if (!srv.user) {
        const colIdx = creds.indexOf(':');
        if (colIdx !== -1) {
          srv.user = creds.slice(0, colIdx);
          if (!srv.password) srv.password = creds.slice(colIdx + 1);
        } else {
          srv.user = creds;
        }
      }
    }

    // Extract port :21
    const portMatch = raw.match(/:(\d+)$/);
    if (portMatch) {
      srv.port = parseInt(portMatch[1], 10);
      raw = raw.slice(0, -portMatch[0].length);
    }

    srv.host = raw.replace(/^[/\\:]+|[/\\:]+$/g, '').trim();
  }

  let isAnonymous = $derived.by(() => {
    if (!selectedServer) return false;
    const u = (selectedServer.user || '').trim().toLowerCase();
    return u === 'anonymous' || u === 'guest';
  });

  function toggleAnonymous() {
    if (!selectedServer) return;
    if (isAnonymous) {
      selectedServer.user = '';
      selectedServer.password = '';
    } else {
      selectedServer.user = 'anonymous';
      selectedServer.password = 'anonymous@';
    }
    handleServerFieldChange();
  }

  function handleServerFieldChange() {
    if (!selectedServer) return;
    sanitizeServerInput(selectedServer);
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
    sanitizeServerInput(selectedServer);
    isTestingConnection = true;
    testResult = null;
    try {
      const res = await onTestConnection(selectedServer);
      if (res.cleanedHost && selectedServer) {
        selectedServer.host = res.cleanedHost;
      }
      if (res.cleanedDir && selectedServer && (!selectedServer.remoteDir || selectedServer.remoteDir === '/')) {
        selectedServer.remoteDir = res.cleanedDir;
      }
      if (res.cleanedPort && selectedServer) {
        selectedServer.port = res.cleanedPort;
      }
      handleServerFieldChange();

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
<div class="flex-1 flex h-full overflow-hidden bg-gradient-to-b from-[#0F1218] to-[#0B0F1E] text-white font-sans select-none">
  
  <!-- Left Navigation Column: Matches FavoritesView and MasterDetailCatalog -->
  <aside data-nav-zone="list" class="panel-master relative w-[310px] lg:w-[340px] flex flex-col flex-shrink-0 h-full select-none bg-gradient-to-b from-[#0F1218] to-[#0B0F1E]">
    <!-- Top Header matching app standard -->
    <div class="p-3 pb-2.5 flex-shrink-0 bg-transparent">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <Gear size={15} weight="fill" class="text-sky-400" />
          <h1 class="text-xs font-bold uppercase tracking-wider text-[#cbd5e1]">Настройки</h1>
        </div>
      </div>
    </div>

    <!-- Category Nav Items -->
    <nav class="flex-1 p-2 space-y-1 overflow-y-auto">
      <button
        data-nav-item
        class="w-full flex items-center gap-2.5 px-3.5 py-2.5 rounded-[12px] text-xs font-medium transition-colors cursor-pointer text-left {activeSubTab === 'storage' ? 'bg-white/10 text-white font-semibold border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
        onclick={() => (activeSubTab = 'storage')}
      >
        <HardDrive size={15} class="flex-shrink-0 {activeSubTab === 'storage' ? 'text-sky-400' : 'text-[#8e95a2]'}" />
        <span>Хранилище</span>
      </button>

      <button
        data-nav-item
        class="w-full flex items-center gap-2.5 px-3.5 py-2.5 rounded-[12px] text-xs font-medium transition-colors cursor-pointer text-left {activeSubTab === 'server' ? 'bg-white/10 text-white font-semibold border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
        onclick={() => (activeSubTab = 'server')}
      >
        <Database size={15} class="flex-shrink-0 {activeSubTab === 'server' ? 'text-sky-400' : 'text-[#8e95a2]'}" />
        <span>Серверы</span>
      </button>

      <button
        data-nav-item
        class="w-full flex items-center gap-2.5 px-3.5 py-2.5 rounded-[12px] text-xs font-medium transition-colors cursor-pointer text-left {activeSubTab === 'torrents' ? 'bg-white/10 text-white font-semibold border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
        onclick={() => (activeSubTab = 'torrents')}
      >
        <Magnet size={15} class="flex-shrink-0 {activeSubTab === 'torrents' ? 'text-sky-400' : 'text-[#8e95a2]'}" />
        <span>Торренты</span>
        {#if (localSettings.torrentSources || []).length > 0}
          <span class="ml-auto text-[10px] font-mono font-semibold text-[#8e95a2] bg-white/[0.04] px-1.5 py-0.5 rounded-[6px] border border-white/[0.06]">
            {(localSettings.torrentSources || []).length}
          </span>
        {/if}
      </button>

      <button
        data-nav-item
        class="w-full flex items-center gap-2.5 px-3.5 py-2.5 rounded-[12px] text-xs font-medium transition-colors cursor-pointer text-left {activeSubTab === 'downloads' ? 'bg-white/10 text-white font-semibold border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
        onclick={() => (activeSubTab = 'downloads')}
      >
        <DownloadCloud size={15} class="flex-shrink-0 {activeSubTab === 'downloads' ? 'text-sky-400' : 'text-[#8e95a2]'}" />
        <span>Загрузки</span>
      </button>

      <button
        data-nav-item
        class="w-full flex items-center gap-2.5 px-3.5 py-2.5 rounded-[12px] text-xs font-medium transition-colors cursor-pointer text-left {activeSubTab === 'interface' ? 'bg-white/10 text-white font-semibold border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
        onclick={() => (activeSubTab = 'interface')}
      >
        <Sliders size={15} class="flex-shrink-0 {activeSubTab === 'interface' ? 'text-sky-400' : 'text-[#8e95a2]'}" />
        <span>Интерфейс</span>
      </button>

      {#if localSettings.enableLogs}
        <button
          data-nav-item
          class="w-full flex items-center gap-2.5 px-3.5 py-2.5 rounded-[12px] text-xs font-medium transition-colors cursor-pointer text-left {activeSubTab === 'logs' ? 'bg-white/10 text-white font-semibold border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
          onclick={() => {
            activeSubTab = 'logs';
            loadLogs();
          }}
        >
          <Terminal size={15} class="flex-shrink-0 {activeSubTab === 'logs' ? 'text-sky-400' : 'text-[#8e95a2]'}" />
          <span>Логи</span>
          {#if logEntries.length > 0}
            <span class="ml-auto text-[10px] font-mono font-semibold text-[#8e95a2] bg-white/[0.04] px-1.5 py-0.5 rounded-[6px] border border-white/[0.06]">
              {logEntries.length}
            </span>
          {/if}
        </button>
      {/if}

      <button
        data-nav-item
        class="w-full flex items-center gap-2.5 px-3.5 py-2.5 rounded-[12px] text-xs font-medium transition-colors cursor-pointer text-left {activeSubTab === 'about' ? 'bg-white/10 text-white font-semibold border border-white/10' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
        onclick={() => (activeSubTab = 'about')}
      >
        <Info size={15} class="flex-shrink-0 {activeSubTab === 'about' ? 'text-sky-400' : 'text-[#8e95a2]'}" />
        <span>О программе</span>
      </button>
    </nav>

    <!-- Save Action -->
    <div class="p-3 flex-shrink-0 bg-transparent">
      <button
        data-nav-item
        class="w-full py-2.5 px-3 rounded-[12px] text-xs font-mono font-medium flex items-center justify-center gap-2 cursor-pointer transition-colors {isSaved ? 'bg-emerald-500/20 text-emerald-300' : 'bg-white/10 text-white hover:bg-white/15'}"
        onclick={handleSave}
      >
        {#if isSaved}
          <Check size={14} weight="bold" />
          <span>Сохранено</span>
        {:else}
          <Save size={14} weight="bold" />
          <span>Сохранить настройки</span>
        {/if}
      </button>
    </div>
  </aside>

  <!-- Right Settings Content (Pure Flat Layout) -->
  <main data-nav-zone="detail" class="flex-1 flex flex-col h-full overflow-y-auto px-8 py-6 lg:px-10 max-w-4xl bg-transparent">
    
    <!-- 1. STORAGE -->
    {#if activeSubTab === 'storage'}
      <div class="space-y-6">
        <div class="flex items-center justify-between border-b border-white/[0.06] pb-3.5">
          <h2 class="text-xs font-bold uppercase tracking-wider text-[#cbd5e1]">Хранилище игр</h2>
          <button
            data-nav-item
            class="px-3.5 py-1.5 rounded-[10px] bg-white/10 hover:bg-white/15 text-xs text-white border border-white/10 font-medium transition-colors cursor-pointer flex items-center gap-1.5"
            onclick={handleBrowseFolder}
          >
            <Folder class="w-3.5 h-3.5" />
            <span>Выбрать папку</span>
          </button>
        </div>

        <div class="space-y-4">
          {#if isLoadingDrives}
            <div class="p-8 rounded-[18px] bg-[#0d1117] border border-white/[0.06] flex items-center justify-center gap-3 text-xs font-mono text-[#8e95a2] shadow-xl">
              <RefreshCw class="w-4 h-4 animate-spin text-sky-400" />
              <span>Определение накопителей и свободного места...</span>
            </div>
          {:else if storageDrives.length === 0}
            <div class="p-5 rounded-[18px] bg-[#0d1117] border border-white/[0.06] space-y-4 shadow-xl">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2.5">
                  <HardDrive class="w-4 h-4 text-sky-400" />
                  <span class="text-xs font-bold text-white font-mono">Основное хранилище</span>
                  <span class="px-2 py-0.5 rounded-[6px] bg-sky-500/15 text-sky-400 border border-sky-500/25 text-[10px] font-mono font-semibold uppercase">
                    Основной
                  </span>
                </div>
              </div>

              <div class="text-xs font-mono text-[#cbd5e1]">
                Папка для загрузок: <span class="text-white font-bold">{localSettings.downloadPath}</span>
              </div>

              <div class="pt-1 flex items-center gap-3">
                <button
                  data-nav-item
                  class="px-3.5 py-1.5 rounded-[10px] bg-white/10 hover:bg-white/15 text-xs text-white border border-white/10 font-medium transition-colors cursor-pointer flex items-center gap-1.5"
                  onclick={handleBrowseFolder}
                >
                  <Folder class="w-3.5 h-3.5" />
                  <span>Выбрать папку</span>
                </button>

                <button
                  data-nav-item
                  class="px-3.5 py-1.5 rounded-[10px] bg-white/5 hover:bg-white/10 text-xs text-[#8e95a2] hover:text-white border border-white/5 transition-colors cursor-pointer"
                  onclick={() => onOpenFolder(localSettings.downloadPath)}
                >
                  Открыть в проводнике
                </button>
              </div>
            </div>
          {:else}
            {#each storageDrives as drive}
              {@const total = drive.totalBytes || 1}
              {@const duckePct = Math.min(100, Math.max(0, (drive.duckeBytes / total) * 100))}
              {@const otherBytes = Math.max(0, (drive.usedBytes - drive.duckeBytes))}
              {@const otherPct = Math.min(100 - duckePct, Math.max(0, (otherBytes / total) * 100))}

              <div class="p-5 rounded-[18px] bg-[#0d1117] border border-white/[0.06] space-y-3.5 shadow-xl">
                <!-- Header Row -->
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2.5">
                    <HardDrive class="w-4 h-4 text-sky-400" />
                    <span class="text-xs font-bold text-white font-mono">{drive.label}</span>
                    {#if drive.isDefault}
                      <span class="px-2 py-0.5 rounded-[6px] bg-sky-500/15 text-sky-400 border border-sky-500/25 text-[10px] font-mono font-semibold uppercase">
                        Основной
                      </span>
                    {/if}
                  </div>

                  <div class="text-xs font-mono text-[#8e95a2]">
                    <span class="text-white font-medium">{drive.freeGB}</span> свободно из {drive.totalGB}
                  </div>
                </div>

                <!-- Path Label in Steam Style -->
                <div class="text-[11px] font-mono text-[#64748b] truncate">
                  {drive.isDefault ? localSettings.downloadPath : drive.path}
                </div>

                <!-- Steam Single Storage Bar -->
                <div class="w-full h-2 rounded-full bg-white/[0.06] overflow-hidden flex">
                  {#if duckePct > 0}
                    <div class="h-full bg-sky-400" style="width: {duckePct}%;"></div>
                  {/if}
                  {#if otherPct > 0}
                    <div class="h-full bg-slate-500" style="width: {otherPct}%;"></div>
                  {/if}
                </div>

                <!-- Steam Storage Legend -->
                <div class="flex flex-wrap items-center gap-x-5 gap-y-1.5 text-xs font-mono pt-0.5">
                  {#if drive.duckeBytes > 0}
                    <div class="flex items-center gap-1.5 text-sky-400">
                      <span class="text-xs leading-none">●</span>
                      <span class="text-slate-200">Ducke</span>
                      <span class="text-[#8e95a2] lowercase">({drive.duckeGB})</span>
                    </div>
                  {/if}

                  <div class="flex items-center gap-1.5 text-slate-400">
                    <span class="text-xs leading-none">●</span>
                    <span class="text-slate-200">Другие файлы</span>
                    <span class="text-[#8e95a2] lowercase">({formatBytesReadable(otherBytes)})</span>
                  </div>

                  <div class="flex items-center gap-1.5 text-slate-500">
                    <span class="text-xs leading-none">●</span>
                    <span class="text-slate-200">Свободно</span>
                    <span class="text-[#8e95a2] lowercase">({drive.freeGB})</span>
                  </div>
                </div>

                <!-- Action Links -->
                <div class="pt-1 flex items-center gap-4 text-xs font-mono text-[#8e95a2]">
                  {#if !drive.isDefault}
                    <button
                      data-nav-item
                      class="text-sky-400 hover:text-sky-300 hover:underline cursor-pointer"
                      onclick={() => handleSetDriveAsDefault(drive)}
                    >
                      Сделать основным
                    </button>
                  {/if}

                  <button
                    data-nav-item
                    class="hover:text-white hover:underline cursor-pointer"
                    onclick={handleBrowseFolder}
                  >
                    Выбрать папку
                  </button>

                  <button
                    data-nav-item
                    class="hover:text-white hover:underline cursor-pointer"
                    onclick={() => onOpenFolder(drive.isDefault ? localSettings.downloadPath : drive.path)}
                  >
                    Открыть в проводнике
                  </button>
                </div>
              </div>
            {/each}
          {/if}
        </div>
      </div>

    <!-- 2. SERVERS -->
    {:else if activeSubTab === 'server'}
      <div class="space-y-6">
        <div class="flex items-center justify-between border-b border-white/[0.06] pb-3.5">
          <h2 class="text-xs font-bold uppercase tracking-wider text-[#cbd5e1]">FTP / SFTP Серверы</h2>

          <div class="flex items-center gap-2.5">
            <button
              data-nav-item
              class="px-3.5 py-1.5 rounded-[10px] bg-white/5 hover:bg-white/10 text-xs text-[#cbd5e1] hover:text-white border border-white/[0.06] transition-colors cursor-pointer"
              onclick={onImportFile}
            >
              Импорт из FileZilla
            </button>

            <button
              data-nav-item
              class="px-4 py-2 rounded-[12px] bg-sky-500/20 hover:bg-sky-500/30 text-sky-300 border border-sky-500/30 text-xs font-medium transition-colors cursor-pointer flex items-center gap-1.5"
              onclick={handleAddServer}
            >
              <Plus class="w-3.5 h-3.5" />
              <span>Добавить сервер</span>
            </button>
          </div>
        </div>

        <!-- Server Selector Tabs -->
        {#if localSettings.savedServers.length > 0}
          <div class="flex items-center gap-2 overflow-x-auto pb-1">
            {#each localSettings.savedServers as srv (srv.id)}
              {@const isSelected = selectedServer?.id === srv.id}
              {@const isActive = localSettings.activeServer?.id === srv.id}
              <button
                data-nav-item
                class="px-3.5 py-2 rounded-[12px] text-xs font-mono border transition-colors cursor-pointer flex items-center gap-2 flex-shrink-0 {isSelected ? 'bg-white/10 border-white/20 text-white font-medium' : 'bg-[#0d1117] border-white/[0.06] text-[#8e95a2] hover:text-white'}"
                onclick={() => handleSelectServer(srv.id)}
              >
                <span>{srv.name || srv.host || 'Сервер'}</span>
                {#if isActive}
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-400" title="Активный сервер"></span>
                {/if}
              </button>
            {/each}
          </div>
        {/if}

        <!-- Selected Server Editor -->
        {#if selectedServer}
          <div class="p-5 rounded-[18px] bg-[#0d1117] border border-white/[0.06] space-y-4 shadow-xl">
            
            <!-- Top Controls for Selected Server -->
            <div class="flex items-center justify-between pb-3 border-b border-white/[0.06]">
              <div class="flex items-center gap-2">
                {#if localSettings.activeServer?.id === selectedServer.id}
                  <div class="flex items-center gap-1.5 px-2.5 py-1 rounded-[6px] bg-emerald-500/15 border border-emerald-500/25 text-emerald-400 text-[11px] font-mono font-medium">
                    <CheckCircle2 class="w-3.5 h-3.5" />
                    <span>Активный сервер</span>
                  </div>
                {:else}
                  <button
                    data-nav-item
                    class="px-3.5 py-1.5 rounded-[10px] bg-sky-500/15 text-sky-400 hover:bg-sky-500/25 border border-sky-500/25 text-xs font-mono font-medium cursor-pointer transition-colors"
                    onclick={() => handleMakeServerActive(selectedServer)}
                  >
                    Сделать активным
                  </button>
                {/if}
              </div>

              {#if localSettings.savedServers.length > 1}
                <button
                  data-nav-item
                  class="px-3 py-1.5 rounded-[10px] text-xs text-[#8e95a2] hover:text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer flex items-center gap-1.5"
                  onclick={() => handleDeleteServer(selectedServer)}
                  title="Удалить этот сервер"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                  <span>Удалить</span>
                </button>
              {/if}
            </div>

            <!-- Server Name -->
            <div class="flex items-center justify-between py-1.5 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Название</span>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.name}
                onchange={handleSave}
                placeholder="Основной SFTP"
                class="w-72 bg-[#07080a] border border-white/10 rounded-[10px] px-3 py-1.5 text-right text-white font-mono text-xs focus:outline-none focus:border-white/25"
              />
            </div>

            <!-- Protocol -->
            <div class="flex items-center justify-between py-1.5 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Протокол</span>
              <div class="flex items-center gap-1.5">
                <button
                  data-nav-item
                  type="button"
                  class="px-3.5 py-1 text-xs rounded-[8px] cursor-pointer transition-colors font-mono {selectedServer.protocol === 'sftp' ? 'bg-white/15 text-white font-semibold border border-white/20' : 'text-[#8e95a2] hover:text-white bg-white/5 border border-transparent'}"
                  onclick={() => {
                    if (selectedServer) {
                      selectedServer.protocol = 'sftp';
                      if (selectedServer.port === 21) selectedServer.port = 2022;
                      handleSave();
                    }
                  }}
                >
                  SFTP
                </button>
                <button
                  data-nav-item
                  type="button"
                  class="px-3.5 py-1 text-xs rounded-[8px] cursor-pointer transition-colors font-mono {selectedServer.protocol === 'ftp' ? 'bg-white/15 text-white font-semibold border border-white/20' : 'text-[#8e95a2] hover:text-white bg-white/5 border border-transparent'}"
                  onclick={() => {
                    if (selectedServer) {
                      selectedServer.protocol = 'ftp';
                      if (selectedServer.port === 2022) selectedServer.port = 21;
                      handleSave();
                    }
                  }}
                >
                  FTP
                </button>
              </div>
            </div>

            <!-- Host -->
            <div class="flex items-center justify-between py-1.5 border-b border-white/[0.06] text-xs">
              <div class="flex flex-col">
                <span class="text-[#cbd5e1] font-mono">Хост / IP адрес</span>
                <span class="text-[10px] text-[#8e95a2] font-mono">Поддерживает URL (ftp://host/path)</span>
              </div>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.host}
                onchange={handleSave}
                onblur={handleSave}
                placeholder="192.168.1.100 или ftp.example.com"
                class="w-72 bg-[#07080a] border border-white/10 rounded-[10px] px-3 py-1.5 text-right text-white font-mono text-xs focus:outline-none focus:border-white/25"
              />
            </div>

            <!-- Port -->
            <div class="flex items-center justify-between py-1.5 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Порт</span>
              <input
                data-nav-item
                type="number"
                bind:value={selectedServer.port}
                onchange={handleSave}
                class="w-24 bg-[#07080a] border border-white/10 rounded-[10px] px-3 py-1.5 text-right text-white font-mono text-xs focus:outline-none focus:border-white/25"
              />
            </div>

            <!-- Anonymous Login Toggle -->
            <div class="flex items-center justify-between py-1.5 border-b border-white/[0.06] text-xs">
              <div class="flex flex-col">
                <span class="text-[#cbd5e1] font-mono">Анонимный вход</span>
                <span class="text-[10px] text-[#8e95a2] font-mono">Вход без пароля (для открытых FTP)</span>
              </div>
              <button
                data-nav-item
                type="button"
                class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none {isAnonymous ? 'bg-sky-500' : 'bg-white/10'}"
                onclick={() => {
                  toggleAnonymous();
                  handleSave();
                }}
                title="Переключить анонимный вход"
              >
                <span
                  class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow-sm transition duration-200 ease-in-out {isAnonymous ? 'translate-x-4' : 'translate-x-0'}"
                ></span>
              </button>
            </div>

            <!-- User -->
            <div class="flex items-center justify-between py-1.5 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Пользователь</span>
              {#if isAnonymous}
                <div class="flex items-center gap-2">
                  <span class="px-2.5 py-1 rounded-[8px] bg-white/5 border border-white/10 text-white font-mono text-xs">
                    anonymous
                  </span>
                  <span class="text-[11px] text-[#8e95a2] font-mono">(гостевой доступ)</span>
                </div>
              {:else}
                <input
                  data-nav-item
                  type="text"
                  bind:value={selectedServer.user}
                  onchange={handleSave}
                  placeholder="Логин"
                  class="w-72 bg-[#07080a] border border-white/10 rounded-[10px] px-3 py-1.5 text-right text-white font-mono text-xs focus:outline-none focus:border-white/25"
                />
              {/if}
            </div>

            <!-- Password -->
            <div class="flex items-center justify-between py-1.5 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Пароль</span>
              {#if isAnonymous}
                <div class="flex items-center gap-1.5 text-xs font-mono text-[#8e95a2] py-1">
                  <span class="text-white/70">Пароль не требуется</span>
                  <span class="text-[10px] text-white/40 font-mono">(anonymous@)</span>
                </div>
              {:else}
                <div class="flex items-center gap-2">
                  {#if showPassword}
                    <input
                      data-nav-item
                      type="text"
                      bind:value={selectedServer.password}
                      onchange={handleSave}
                      placeholder="••••••••"
                      class="w-64 bg-[#07080a] border border-white/10 rounded-[10px] px-3 py-1.5 text-right text-white font-mono text-xs focus:outline-none focus:border-white/25"
                    />
                  {:else}
                    <input
                      data-nav-item
                      type="password"
                      bind:value={selectedServer.password}
                      onchange={handleSave}
                      placeholder="••••••••"
                      class="w-64 bg-[#07080a] border border-white/10 rounded-[10px] px-3 py-1.5 text-right text-white font-mono text-xs focus:outline-none focus:border-white/25"
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
              {/if}
            </div>

            <!-- Remote Dir -->
            <div class="flex items-center justify-between py-1.5 border-b border-white/[0.06] text-xs">
              <span class="text-[#cbd5e1] font-mono">Удаленный каталог</span>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.remoteDir}
                onchange={handleSave}
                placeholder="/public"
                class="w-72 bg-[#07080a] border border-white/10 rounded-[10px] px-3 py-1.5 text-right text-white font-mono text-xs focus:outline-none focus:border-white/25"
              />
            </div>

            <!-- Connection Test -->
            <div class="pt-2 flex items-center justify-between">
              <button
                data-nav-item
                disabled={isTestingConnection}
                class="px-4 py-2 rounded-[12px] bg-white/10 hover:bg-white/15 active:bg-white/20 text-xs font-mono font-medium text-white border border-white/10 cursor-pointer flex items-center gap-2 disabled:opacity-50 transition-colors"
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

    <!-- TORRENTS -->
    {:else if activeSubTab === 'torrents'}
      <TorrentSourcesManager
        torrentSources={localSettings.torrentSources || []}
        onSourcesChanged={(sources) => {
          localSettings.torrentSources = sources;
        }}
      />

    <!-- 3. DOWNLOADS -->
    {:else if activeSubTab === 'downloads'}
      <div class="space-y-6">
        <h2 class="text-xs font-bold uppercase tracking-wider text-[#cbd5e1] border-b border-white/[0.06] pb-3.5">Параметры загрузок</h2>

        <div class="space-y-5 pt-1">
          <!-- Download Folder -->
          <div class="space-y-2 pb-4 border-b border-white/[0.06]">
            <span class="text-xs text-[#cbd5e1] font-mono">Папка для скачивания по умолчанию</span>
            <div class="flex items-center gap-3">
              <input
                type="text"
                readonly
                value={localSettings.downloadPath}
                class="flex-1 bg-[#0d1117] border border-white/10 px-3.5 py-2 rounded-[10px] text-xs font-mono text-white select-all"
              />
              <button
                data-nav-item
                class="px-4 py-2 rounded-[10px] bg-white/10 hover:bg-white/15 text-xs font-medium text-white border border-white/10 transition-colors cursor-pointer"
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
                  max="1000000"
                  step="512"
                  bind:value={localSettings.maxSpeedKBps}
                  onchange={handleSave}
                  class="w-24 bg-[#0d1117] border border-white/10 px-3 py-1.5 rounded-[10px] text-right text-white font-mono text-xs focus:outline-none focus:border-white/25"
                />
                <span class="text-xs font-mono text-[#8e95a2]">КБ/с</span>
              </div>
            </div>

            <!-- Quick Presets -->
            <div class="flex items-center gap-2 flex-wrap">
              {#each speedPresets as preset}
                <button
                  data-nav-item
                  class="px-3.5 py-1.5 rounded-[10px] text-xs font-mono transition-colors cursor-pointer {localSettings.maxSpeedKBps === preset.value ? 'bg-white/15 border border-white/20 text-white font-medium' : 'bg-[#0d1117] border border-white/[0.06] hover:bg-white/5 text-[#8e95a2] hover:text-white'}"
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
        <h2 class="text-xs font-bold uppercase tracking-wider text-[#cbd5e1] border-b border-white/[0.06] pb-3.5">Интерфейс и интеграции</h2>

        <div class="space-y-5 pt-1">
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
              class="w-full bg-[#0d1117] border border-white/10 px-3.5 py-2 rounded-[10px] text-xs font-mono text-white focus:outline-none focus:border-white/25"
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
                class="px-4 py-2 rounded-[10px] bg-white/10 hover:bg-white/15 active:bg-white/20 text-xs font-mono font-medium text-white border border-white/10 cursor-pointer flex items-center gap-2 disabled:opacity-50 transition-colors"
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

          <div class="flex items-center justify-between py-2 border-b border-white/[0.06] text-xs">
            <div>
              <span class="text-[#cbd5e1] font-mono block">Распаковка архивов</span>
              <span class="text-[11px] text-[#64748b] font-mono">Автоматически распаковывать многотомные архивы (.rar, .zip, .7z) после загрузки</span>
            </div>
            <button
              data-nav-item
              type="button"
              aria-label="Автоматическая распаковка архивов"
              title="Автоматическая распаковка архивов"
              class="w-12 h-6 rounded-full transition-colors cursor-pointer relative {localSettings.autoExtractArchives ? 'bg-sky-500' : 'bg-white/10'}"
              onclick={() => {
                localSettings.autoExtractArchives = !localSettings.autoExtractArchives;
                handleSave();
              }}
            >
              <span class="w-4 h-4 rounded-full bg-white transition-transform absolute top-1 {localSettings.autoExtractArchives ? 'left-7' : 'left-1'}"></span>
            </button>
          </div>
        </div>
      </div>

    <!-- LOGS -->
    {:else if activeSubTab === 'logs'}
      <div class="h-full flex flex-col space-y-4">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-white/[0.06] pb-3.5">
          <div>
            <h2 class="text-xs font-bold uppercase tracking-wider text-[#cbd5e1] flex items-center gap-2">
              <Terminal class="w-4 h-4 text-sky-400" />
              <span>Журнал работы (Логи)</span>
            </h2>
            <p class="text-[11px] text-[#64748b] font-mono mt-0.5">
              Диагностика сети, загрузчика, метаданных и системы
            </p>
          </div>

          <div class="flex items-center gap-2 flex-wrap">
            <button
              data-nav-item
              class="px-3.5 py-1.5 rounded-[10px] bg-white/10 hover:bg-white/15 text-xs text-white border border-white/10 cursor-pointer flex items-center gap-1.5 transition-colors font-medium"
              onclick={handleClearLogs}
              title="Очистить буфер логов"
            >
              <X class="w-3.5 h-3.5 text-rose-400" />
              <span>Очистить</span>
            </button>

            <button
              data-nav-item
              class="px-3.5 py-1.5 rounded-[10px] bg-sky-500/20 hover:bg-sky-500/30 text-sky-300 border border-sky-500/30 text-xs font-medium cursor-pointer flex items-center gap-1.5 transition-colors"
              onclick={handleExportLogs}
              title="Экспорт в текстовый файл"
            >
              <Download class="w-3.5 h-3.5" />
              <span>Экспорт в файл</span>
            </button>
          </div>
        </div>

        {#if exportMessage}
          <div class="p-3 rounded-[10px] bg-white/5 border border-sky-500/30 text-xs font-mono text-sky-300 flex items-center justify-between">
            <span>{exportMessage}</span>
            <button onclick={() => (exportMessage = null)} class="text-white/40 hover:text-white">
              <X class="w-3.5 h-3.5" />
            </button>
          </div>
        {/if}

        <!-- Filter bar -->
        <div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-2.5 bg-[#0d1117] p-2.5 rounded-[14px] border border-white/[0.06]">
          <!-- Level filters -->
          <div class="flex items-center gap-1 overflow-x-auto">
            {#each ['ALL', 'INFO', 'WARN', 'ERROR', 'DEBUG'] as lvl}
              <button
                data-nav-item
                class="px-3 py-1 rounded-[8px] text-[11px] font-mono font-medium uppercase transition-colors cursor-pointer {selectedLogLevel === lvl ? 'bg-white/15 text-white' : 'text-[#8e95a2] hover:text-white hover:bg-white/5'}"
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
                class="w-full bg-[#07080a] border border-white/10 px-3 py-1.5 text-[11px] font-mono text-white rounded-[8px] placeholder:text-[#64748b] focus:outline-none focus:border-white/25"
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
          class="flex-1 min-h-[320px] max-h-[calc(100vh-280px)] overflow-y-auto bg-[#040507] border border-white/[0.08] rounded-[14px] p-3.5 font-mono text-xs select-text space-y-1"
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
              <div class="flex items-start gap-2 py-0.5 px-1.5 rounded-[6px] hover:bg-white/[0.03] transition-colors leading-relaxed break-all">
                <span class="text-[#64748b] text-[11px] flex-shrink-0 select-none">{entry.timestamp}</span>
                <span class="px-1.5 py-0.5 rounded-[6px] text-[10px] font-bold flex-shrink-0 select-none {entry.level === 'ERROR' ? 'bg-rose-500/15 text-rose-300 border border-rose-500/20' : entry.level === 'WARN' ? 'bg-amber-500/15 text-amber-300 border border-amber-500/20' : entry.level === 'DEBUG' ? 'bg-slate-500/15 text-slate-300 border border-slate-500/20' : 'bg-sky-500/15 text-sky-300 border border-sky-500/20'}">
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
        <h2 class="text-xs font-bold uppercase tracking-wider text-[#cbd5e1] border-b border-white/[0.06] pb-3.5">О программе</h2>

        <div class="space-y-3 pt-1">
          <div class="flex items-center justify-between py-2.5 border-b border-white/[0.06] text-xs font-mono">
            <span class="text-[#8e95a2]">Приложение</span>
            <span class="font-bold text-white">{appInfo.name}</span>
          </div>

          <div class="flex items-center justify-between py-2.5 border-b border-white/[0.06] text-xs font-mono">
            <div>
              <span class="text-[#8e95a2] block">Версия</span>
              <span class="text-white font-semibold">v{appInfo.version}</span>
            </div>
            <button
              data-nav-item
              type="button"
              disabled={isCheckingUpdate || isDownloadingUpdate}
              class="px-3.5 py-1.5 rounded-[10px] bg-white/10 hover:bg-white/15 text-white text-xs font-mono font-medium border border-white/10 transition-colors cursor-pointer flex items-center gap-2 disabled:opacity-50"
              onclick={handleCheckUpdates}
            >
              <RefreshCw class="w-3.5 h-3.5 {isCheckingUpdate ? 'animate-spin' : ''}" />
              <span>{isCheckingUpdate ? 'Проверка...' : 'Проверить обновления'}</span>
            </button>
          </div>

          {#if updateCheckMessage}
            <div class="p-3 rounded-[10px] bg-white/[0.04] border border-white/10 text-xs font-mono text-[#cbd5e1] flex items-center justify-between">
              <span>{updateCheckMessage}</span>
              <button onclick={() => (updateCheckMessage = null)} class="text-white/40 hover:text-white p-1">
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          {/if}

          {#if updateErrorMessage}
            <div class="p-3 rounded-[10px] bg-rose-500/10 border border-rose-500/30 text-xs font-mono text-rose-300 flex items-center justify-between">
              <span>{updateErrorMessage}</span>
              <button onclick={() => (updateErrorMessage = null)} class="text-rose-400 hover:text-white p-1">
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          {/if}

          <!-- Update Available Card -->
          {#if updateCheckResult?.hasUpdate}
            <div class="p-4 rounded-[14px] bg-[#0c1017] border border-sky-500/30 space-y-3 shadow-lg">
              <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div class="flex items-center gap-2">
                  <span class="px-2 py-0.5 rounded-[6px] bg-sky-500/20 text-sky-400 text-xs font-mono font-bold">
                    Доступно обновление: {updateCheckResult.latestVersion}
                  </span>
                  {#if updateCheckResult.assetSize}
                    <span class="text-xs font-mono text-[#8e95a2]">
                      ({(updateCheckResult.assetSize / (1024 * 1024)).toFixed(1)} МБ)
                    </span>
                  {/if}
                </div>

                {#if !isDownloadingUpdate && !isUpdateReadyToInstall}
                  <button
                    data-nav-item
                    type="button"
                    class="px-4 py-2 rounded-[10px] bg-sky-500 hover:bg-sky-400 text-black text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-2 shadow-md"
                    onclick={handleStartDownloadUpdate}
                  >
                    <Download class="w-4 h-4 stroke-[2.5]" />
                    <span>Скачать обновление</span>
                  </button>
                {:else if isUpdateReadyToInstall}
                  <button
                    data-nav-item
                    type="button"
                    class="px-4 py-2 rounded-[10px] bg-emerald-500 hover:bg-emerald-400 text-black text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-2 shadow-lg"
                    onclick={handleApplyUpdate}
                  >
                    <Check class="w-4 h-4 stroke-[3]" />
                    <span>Перезапустить и обновить</span>
                  </button>
                {/if}
              </div>

              <!-- Download Progress Bar -->
              {#if isDownloadingUpdate && updateProgress}
                <div class="space-y-1.5 pt-1">
                  <div class="flex items-center justify-between text-xs font-mono text-[#cbd5e1]">
                    <span>Загрузка...</span>
                    <span>{updateProgress.percent.toFixed(0)}% ({((updateProgress.speedBytesPerSec || 0) / (1024 * 1024)).toFixed(1)} МБ/с)</span>
                  </div>
                  <div class="h-2 w-full bg-white/10 rounded-full overflow-hidden">
                    <div class="h-full bg-sky-400 transition-all duration-200" style="width: {updateProgress.percent}%"></div>
                  </div>
                </div>
              {/if}

              <!-- Release Notes Preview -->
              {#if updateCheckResult.releaseNotes}
                <div class="pt-2 border-t border-white/[0.06]">
                  <div class="text-[11px] font-mono text-[#8e95a2] uppercase tracking-wider mb-1">Список изменений:</div>
                  <div class="max-h-36 overflow-y-auto text-xs font-mono text-[#94a3b8] whitespace-pre-wrap leading-relaxed p-2.5 rounded bg-black/40 border border-white/[0.04]">
                    {updateCheckResult.releaseNotes}
                  </div>
                </div>
              {/if}
            </div>
          {/if}

          <div class="flex items-center justify-between py-2.5 border-b border-white/[0.06] text-xs font-mono">
            <div>
              <span class="text-[#8e95a2] block">Папка конфигурации и БД</span>
              <span class="text-[11px] text-[#64748b]">Настройки config.json и база данных ducke.db</span>
            </div>
            <button
              data-nav-item
              class="px-3.5 py-1.5 rounded-[10px] bg-white/10 hover:bg-white/15 text-white text-xs font-medium border border-white/10 transition-colors cursor-pointer flex items-center gap-1.5"
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
