<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import {
    HardDrive,
    HardDrives as Server,
    Database,
    CloudArrowDown,
    CloudArrowDown as DownloadCloud,
    Info,
    Check,
    Star,
    ArrowClockwise,
    ArrowClockwise as RefreshCw,
    FolderOpen,
    Folder,
    Monitor,
    Power,
    Eye,
    EyeSlash,
    EyeSlash as EyeOff,
    FileCode,
    Plus,
    Trash,
    Trash as Trash2,
    CheckCircle,
    CheckCircle as CheckCircle2,
    Terminal,
    DownloadSimple,
    DownloadSimple as Download,
    Magnet,
    BookOpen,
    X
  } from 'phosphor-svelte';
  import { sound } from '../../navigation/audio';
  import { gamepad } from '../../navigation/gamepad';
  import * as AppAPI from '../../../../wailsjs/go/main/App';
  import { EventsOn, EventsOff } from '../../../../wailsjs/runtime/runtime';
  import TorrentSourcesManager from '../TorrentSourcesManager.svelte';
  import HelpView from '../HelpView.svelte';

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

  type SettingsCategory = 'storage' | 'server' | 'torrents' | 'downloads' | 'logs' | 'help' | 'about';
  let activeCategory = $state<SettingsCategory>('storage');

  let localSettings = $state<AppSettings>({
    downloadPath: 'C:\\Ducke',
    maxConcurrentFiles: 4,
    maxSpeedKBps: 0,
    theme: 'dark',
    steamDeckMode: true,
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
  let saveFeedbackVisible = $state<boolean>(false);
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
    sound.playSelect();
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
          updateCheckMessage = `Установлена последняя версия (v${res.currentVersion || appInfo.version})`;
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
    sound.playSelect();
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
    sound.playSelect();
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
      console.warn('[BigPictureSettings] Failed to load drives:', e);
    } finally {
      isLoadingDrives = false;
    }
  }

  onMount(() => {
    (async () => {
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
    })();

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
          sound.playTab();
        }
      }
    });

    const unsubUpdateErr = EventsOn('update:error', (err: any) => {
      isDownloadingUpdate = false;
      updateErrorMessage = typeof err === 'string' ? err : 'Ошибка загрузки обновления';
    });

    const handleGoBack = (e: CustomEvent) => {
      const focusedEl = document.querySelector('.gamepad-focused');
      const inDetail = focusedEl?.closest('[data-nav-zone="detail"]');
      if (inDetail) {
        sound.playBack();
        e.preventDefault();
        e.stopImmediatePropagation();
        gamepad.focusFirstInZone('list');
      }
    };

    window.addEventListener('app:go-back', handleGoBack as EventListener, true);

    return () => {
      EventsOff('log:entry');
      if (typeof unsubUpdateAvail === 'function') unsubUpdateAvail();
      if (typeof unsubUpdateProg === 'function') unsubUpdateProg();
      if (typeof unsubUpdateErr === 'function') unsubUpdateErr();
      window.removeEventListener('app:go-back', handleGoBack as EventListener, true);
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
    let selected = await onSelectFolder();
    if (selected) {
      // Normalize drive roots (e.g. "F:\" -> "F:\Ducke")
      if (/^[a-zA-Z]:\\?$/.test(selected)) {
        selected = `${selected[0].toUpperCase()}:\\Ducke`;
      }
      localSettings.downloadPath = selected;
      triggerSave();
      loadStorageDrives();
    }
  }

  function handleSetDriveAsDefault(drive: StorageDrive) {
    sound.playSelect();
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
    sound.playSelect();
    if (isAnonymous) {
      selectedServer.user = '';
      selectedServer.password = '';
    } else {
      selectedServer.user = 'anonymous';
      selectedServer.password = 'anonymous@';
    }
    triggerSave();
  }

  function handleServerFieldChange() {
    if (!selectedServer) return;
    sanitizeServerInput(selectedServer);
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
    sound.playSelect();
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
      triggerSave();

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
        class="w-full flex items-center gap-3.5 px-4 py-3 rounded-[12px] text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'storage' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
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
        class="w-full flex items-center gap-3.5 px-4 py-3 rounded-[12px] text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'server' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
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
        class="w-full flex items-center gap-3.5 px-4 py-3 rounded-[12px] text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'torrents' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          activeCategory = 'torrents';
        }}
      >
        <Magnet class="w-5 h-5 flex-shrink-0 {activeCategory === 'torrents' ? 'text-sky-400' : ''}" />
        <span>Торренты</span>
        {#if (localSettings.torrentSources || []).length > 0}
          <span class="ml-auto text-xs px-2 py-0.5 rounded-[6px] bg-white/[0.08] text-[#8e95a2] font-mono">
            {(localSettings.torrentSources || []).length}
          </span>
        {/if}
      </button>

      <button
        data-nav-item
        class="w-full flex items-center gap-3.5 px-4 py-3 rounded-[12px] text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'downloads' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
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
          class="w-full flex items-center gap-3.5 px-4 py-3 rounded-[12px] text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'logs' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
          onclick={() => {
            sound.playFocus();
            activeCategory = 'logs';
            loadLogs();
          }}
        >
          <Terminal class="w-5 h-5 flex-shrink-0 {activeCategory === 'logs' ? 'text-sky-400' : ''}" />
          <span>Логи</span>
          {#if logEntries.length > 0}
            <span class="ml-auto text-xs px-2 py-0.5 rounded-[6px] bg-white/[0.08] text-[#8e95a2] font-mono">
              {logEntries.length}
            </span>
          {/if}
        </button>
      {/if}

      <button
        data-nav-item
        class="w-full flex items-center gap-3.5 px-4 py-3 rounded-[12px] text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'help' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          activeCategory = 'help';
        }}
      >
        <BookOpen class="w-5 h-5 flex-shrink-0 {activeCategory === 'help' ? 'text-sky-400' : ''}" />
        <span>Справка</span>
      </button>

      <button
        data-nav-item
        class="w-full flex items-center gap-3.5 px-4 py-3 rounded-[12px] text-sm font-bold transition-all cursor-pointer text-left {activeCategory === 'about' ? 'bg-white/15 text-white ring-1 ring-white/20' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
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
      <div class="m-4 p-3 rounded-[10px] bg-emerald-500/20 border border-emerald-500/40 text-emerald-300 text-xs font-mono font-bold flex items-center justify-center gap-2">
        <Check class="w-4 h-4" />
        <span>СОХРАНЕНО</span>
      </div>
    {/if}
  </aside>

  <!-- Right Content Area (10-Foot Console Layout) -->
  <main data-nav-zone="detail" class="flex-1 h-full overflow-y-auto">
    <div class="w-full max-w-4xl xl:max-w-5xl mx-auto p-8 lg:p-12 space-y-8">

    <!-- ============================================================ -->
    <!-- 1. STORAGE                                                    -->
    <!-- ============================================================ -->
    {#if activeCategory === 'storage'}
      <div class="space-y-8">
        <div class="border-b border-white/[0.06] pb-4">
          <h1 class="text-xl font-bold uppercase tracking-wider text-white font-mono">Управление хранилищем</h1>
          <p class="text-xs text-[#8e95a2] mt-1 font-mono">Выберите накопитель и каталог для сохранения игр</p>
        </div>

        {#if isLoadingDrives}
          <div class="p-8 rounded-[18px] bg-[#0d1017] border border-white/[0.06] flex items-center justify-center gap-3 text-xs font-mono text-[#8e95a2]">
            <RefreshCw class="w-5 h-5 animate-spin text-sky-400" />
            <span>Определение накопителей и свободного места...</span>
          </div>
        {:else if storageDrives.length === 0}
          <div class="p-6 rounded-[18px] bg-[#0d1017] border border-white/[0.06] space-y-4">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <HardDrive class="w-6 h-6 text-sky-400" />
                <span class="text-base font-bold text-white font-mono">Каталог загрузки</span>
              </div>
              <span class="text-xs font-mono text-[#8e95a2]">{localSettings.downloadPath}</span>
            </div>
            <button
              data-nav-item
              class="px-5 py-2.5 rounded-[10px] bg-white/10 hover:bg-white/20 focus:ring-1 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors"
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

              <div class="p-6 rounded-[18px] bg-[#0d1017] border border-white/[0.08] space-y-5">
                <!-- Header -->
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-3">
                    <HardDrive class="w-6 h-6 text-sky-400" />
                    <div>
                      <div class="flex items-center gap-2">
                        <span class="text-base font-bold text-white font-mono">{drive.label}</span>
                        {#if drive.isDefault}
                          <span class="px-2 py-0.5 rounded-[6px] text-[10px] font-mono font-bold bg-sky-500/20 text-sky-400 border border-sky-500/30">
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
                      class="px-4 py-2 rounded-[10px] bg-white/10 hover:bg-white/20 focus:ring-1 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors"
                      onclick={() => handleSetDriveAsDefault(drive)}
                    >
                      Сделать основным
                    </button>
                  {/if}

                  <button
                    data-nav-item
                    class="px-4 py-2 rounded-[10px] bg-white/10 hover:bg-white/20 focus:ring-1 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors"
                    onclick={handleBrowseFolder}
                  >
                    Выбрать другую папку
                  </button>

                  <button
                    data-nav-item
                    class="px-4 py-2 rounded-[10px] bg-white/5 hover:bg-white/10 focus:ring-1 focus:ring-white focus:outline-none text-[#8e95a2] hover:text-white text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-1.5"
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
              class="px-4 py-2 rounded-[10px] bg-white/10 hover:bg-white/20 focus:ring-1 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold flex items-center gap-2 cursor-pointer transition-colors"
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
              class="px-4 py-2 rounded-[10px] bg-sky-500 hover:bg-sky-400 focus:ring-1 focus:ring-white focus:outline-none text-slate-950 text-xs font-mono font-bold flex items-center gap-1.5 cursor-pointer transition-colors shadow-md"
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
                class="px-4 py-2.5 rounded-[12px] text-xs font-mono border transition-all cursor-pointer flex items-center gap-2.5 flex-shrink-0 focus:ring-1 focus:ring-white focus:outline-none {isSelected ? 'bg-white/15 border-white/30 text-white font-bold' : 'bg-[#0d1017] border-white/[0.08] text-[#8e95a2] hover:text-white'}"
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
          <div class="p-6 rounded-[18px] bg-[#0d1017] border border-white/[0.08] space-y-5">
            <!-- Active Toggle & Delete -->
            <div class="flex items-center justify-between pb-3 border-b border-white/[0.06]">
              <div>
                {#if localSettings.activeServer?.id === selectedServer.id}
                  <div class="flex items-center gap-1.5 px-3 py-1 rounded-[6px] bg-emerald-500/15 border border-emerald-500/30 text-emerald-400 text-xs font-mono font-bold">
                    <CheckCircle2 class="w-3.5 h-3.5" />
                    <span>АКТИВНЫЙ СЕРВЕР</span>
                  </div>
                {:else}
                  <button
                    data-nav-item
                    class="px-4 py-1.5 rounded-[10px] bg-sky-500/15 text-sky-400 hover:bg-sky-500/25 border border-sky-500/30 focus:ring-1 focus:ring-white focus:outline-none text-xs font-mono font-bold cursor-pointer transition-colors"
                    onclick={() => handleMakeServerActive(selectedServer)}
                  >
                    Сделать активным
                  </button>
                {/if}
              </div>

              {#if localSettings.savedServers.length > 1}
                <button
                  data-nav-item
                  class="px-3 py-1.5 rounded-[10px] text-xs font-mono text-[#8e95a2] hover:text-rose-400 hover:bg-rose-500/10 focus:ring-1 focus:ring-rose-400 focus:outline-none transition-colors cursor-pointer flex items-center gap-1.5"
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
                class="w-72 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-[10px] focus:border-sky-400 focus:ring-1 focus:ring-white focus:outline-none"
              />
            </div>

            <!-- Protocol Toggle -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
              <span class="text-sm font-bold text-white font-mono">Протокол</span>
              <div class="flex items-center gap-2">
                <button
                  data-nav-item
                  type="button"
                  class="px-5 py-2 rounded-[10px] text-xs font-mono font-bold cursor-pointer transition-all focus:ring-1 focus:ring-white focus:outline-none {selectedServer.protocol === 'sftp' ? 'bg-sky-500 text-slate-950 font-bold' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
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
                  class="px-5 py-2 rounded-[10px] text-xs font-mono font-bold cursor-pointer transition-all focus:ring-1 focus:ring-white focus:outline-none {selectedServer.protocol === 'ftp' ? 'bg-sky-500 text-slate-950 font-bold' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
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
              <div class="flex flex-col">
                <span class="text-sm font-bold text-white font-mono">Хост / IP-адрес</span>
                <span class="text-[11px] text-[#8e95a2] font-mono">Поддерживает URL (ftp://host/path)</span>
              </div>
              <input
                data-nav-item
                type="text"
                bind:value={selectedServer.host}
                onchange={triggerSave}
                onblur={triggerSave}
                placeholder="192.168.1.100 или ftp.example.com"
                class="w-72 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-[10px] focus:border-sky-400 focus:ring-1 focus:ring-white focus:outline-none"
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
                class="w-32 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-[10px] focus:border-sky-400 focus:ring-1 focus:ring-white focus:outline-none"
              />
            </div>

            <!-- Anonymous Login Toggle -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
              <div class="flex flex-col">
                <span class="text-sm font-bold text-white font-mono">Анонимный вход</span>
                <span class="text-[11px] text-[#8e95a2] font-mono">Вход без пароля (для открытых FTP)</span>
              </div>
              <button
                data-nav-item
                type="button"
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:ring-1 focus:ring-white focus:outline-none {isAnonymous ? 'bg-sky-500' : 'bg-white/10'}"
                onclick={toggleAnonymous}
                title="Переключить анонимный вход"
              >
                <span
                  class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-sm transition duration-200 ease-in-out {isAnonymous ? 'translate-x-5' : 'translate-x-0'}"
                ></span>
              </button>
            </div>

            <!-- User -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
              <span class="text-sm font-bold text-white font-mono">Пользователь</span>
              {#if isAnonymous}
                <div class="flex items-center gap-2">
                  <span class="px-3 py-1.5 rounded-[8px] bg-white/5 border border-white/10 text-white font-mono text-xs font-bold">
                    anonymous
                  </span>
                  <span class="text-xs text-[#8e95a2] font-mono">(гостевой доступ)</span>
                </div>
              {:else}
                <input
                  data-nav-item
                  type="text"
                  bind:value={selectedServer.user}
                  onchange={triggerSave}
                  placeholder="Логин"
                  class="w-72 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-[10px] focus:border-sky-400 focus:ring-1 focus:ring-white focus:outline-none"
                />
              {/if}
            </div>

            <!-- Password -->
            <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
              <span class="text-sm font-bold text-white font-mono">Пароль</span>
              {#if isAnonymous}
                <div class="flex items-center gap-2 text-xs font-mono text-[#8e95a2] py-1.5">
                  <span class="text-white/70">Пароль не требуется</span>
                  <span class="text-[11px] text-white/40 font-mono">(anonymous@)</span>
                </div>
              {:else}
                <div class="flex items-center gap-2">
                  <button
                    data-nav-item
                    type="button"
                    class="p-2 rounded-[8px] text-[#8e95a2] hover:text-white cursor-pointer focus:ring-1 focus:ring-white focus:outline-none"
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
                    class="w-64 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-[10px] focus:border-sky-400 focus:ring-1 focus:ring-white focus:outline-none"
                  />
                </div>
              {/if}
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
                class="w-72 bg-[#07080a] border border-white/10 text-right text-white font-mono text-xs px-3.5 py-2 rounded-[10px] focus:border-sky-400 focus:ring-1 focus:ring-white focus:outline-none"
              />
            </div>

            <!-- Connection Test -->
            <div class="pt-3 flex items-center justify-between">
              <button
                data-nav-item
                disabled={isTestingConnection}
                class="px-5 py-2.5 rounded-[10px] bg-white/10 hover:bg-white/20 text-white text-xs font-mono font-bold cursor-pointer flex items-center gap-2.5 transition-colors disabled:opacity-50 focus:ring-1 focus:ring-white focus:outline-none"
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
    <!-- TORRENTS                                                     -->
    <!-- ============================================================ -->
    {:else if activeCategory === 'torrents'}
      <TorrentSourcesManager
        isBigPicture={true}
        torrentSources={localSettings.torrentSources || []}
        onSourcesChanged={(sources: any) => {
          localSettings.torrentSources = sources;
        }}
      />

    <!-- ============================================================ -->
    <!-- 3. DOWNLOADS                                                  -->
    <!-- ============================================================ -->
    {:else if activeCategory === 'downloads'}
      <div class="space-y-6">
        <div class="border-b border-white/[0.06] pb-4">
          <h1 class="text-xl font-bold uppercase tracking-wider text-white font-mono">Параметры загрузок</h1>
          <p class="text-xs text-[#8e95a2] mt-1 font-mono">Ограничения скорости и многопоточность</p>
        </div>

        <div class="p-6 rounded-[18px] bg-[#0d1017] border border-white/[0.08] space-y-6">
          <!-- Concurrent Files Stepper -->
          <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
            <div>
              <span class="text-sm font-bold text-white font-mono">Одновременных файлов</span>
              <p class="text-xs text-[#64748b] font-mono">Количество файлов, загружаемых параллельно</p>
            </div>

            <div class="flex items-center gap-3">
              <button
                data-nav-item
                class="w-10 h-10 rounded-[10px] bg-white/10 hover:bg-white/20 focus:ring-1 focus:ring-white focus:outline-none text-white flex items-center justify-center font-bold text-lg cursor-pointer transition-colors"
                onclick={() => adjustConcurrent(-1)}
                title="Уменьшить"
              >
                −
              </button>
              <span class="w-8 text-center text-lg font-mono font-bold text-sky-400">{localSettings.maxConcurrentFiles || 4}</span>
              <button
                data-nav-item
                class="w-10 h-10 rounded-[10px] bg-white/10 hover:bg-white/20 focus:ring-1 focus:ring-white focus:outline-none text-white flex items-center justify-center font-bold text-lg cursor-pointer transition-colors"
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
                class="py-3 px-3 rounded-[10px] text-xs font-mono font-bold cursor-pointer transition-all focus:ring-1 focus:ring-white focus:outline-none {localSettings.maxSpeedKBps === 0 ? 'bg-sky-500 text-slate-950 shadow-md' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                onclick={() => setSpeedLimit(0)}
              >
                Без лимита
              </button>

              <button
                data-nav-item
                class="py-3 px-3 rounded-[10px] text-xs font-mono font-bold cursor-pointer transition-all focus:ring-1 focus:ring-white focus:outline-none {localSettings.maxSpeedKBps === 10240 ? 'bg-sky-500 text-slate-950 shadow-md' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                onclick={() => setSpeedLimit(10240)}
              >
                10 МБ/с
              </button>

              <button
                data-nav-item
                class="py-3 px-3 rounded-[10px] text-xs font-mono font-bold cursor-pointer transition-all focus:ring-1 focus:ring-white focus:outline-none {localSettings.maxSpeedKBps === 25600 ? 'bg-sky-500 text-slate-950 shadow-md' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                onclick={() => setSpeedLimit(25600)}
              >
                25 МБ/с
              </button>

              <button
                data-nav-item
                class="py-3 px-3 rounded-[10px] text-xs font-mono font-bold cursor-pointer transition-all focus:ring-1 focus:ring-white focus:outline-none {localSettings.maxSpeedKBps === 51200 ? 'bg-sky-500 text-slate-950 shadow-md' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                onclick={() => setSpeedLimit(51200)}
              >
                50 МБ/с
              </button>

              <button
                data-nav-item
                class="py-3 px-3 rounded-[10px] text-xs font-mono font-bold cursor-pointer transition-all focus:ring-1 focus:ring-white focus:outline-none {localSettings.maxSpeedKBps === 102400 ? 'bg-sky-500 text-slate-950 shadow-md' : 'bg-white/10 text-[#8e95a2] hover:text-white'}"
                onclick={() => setSpeedLimit(102400)}
              >
                100 МБ/с
              </button>
            </div>
          </div>
        </div>
      </div>

    <!-- ============================================================ -->
    <!-- HELP & REPOSITORIES GUIDE                                    -->
    <!-- ============================================================ -->
    {:else if activeCategory === 'help'}
      <HelpView
        isBigPicture={true}
        onNavigateToTorrents={() => {
          sound.playSelect();
          activeCategory = 'torrents';
        }}
      />

    <!-- ============================================================ -->
    <!-- 4. ABOUT & SYSTEM                                             -->
    <!-- ============================================================ -->
    {:else if activeCategory === 'about'}
      <div class="space-y-6">
        <div class="border-b border-white/[0.06] pb-4">
          <h1 class="text-xl font-bold uppercase tracking-wider text-white font-mono">О системе</h1>
          <p class="text-xs text-[#8e95a2] mt-1 font-mono">Информация о Ducke и управление приложением</p>
        </div>

        <div class="p-6 rounded-[18px] bg-[#0d1017] border border-white/[0.08] space-y-4">
          <!-- App Name -->
          <div class="flex items-center justify-between py-2 border-b border-white/[0.06]">
            <span class="text-sm font-bold text-white font-mono">Приложение</span>
            <span class="text-sm font-mono font-bold text-sky-400">{appInfo.name}</span>
          </div>

          <!-- Version & Update Checking -->
          <div class="py-2 border-b border-white/[0.06] space-y-3">
            <div class="flex items-center justify-between">
              <div>
                <span class="text-sm font-bold text-white font-mono block">Версия</span>
                <span class="text-xs font-mono text-[#8e95a2]">v{appInfo.version}</span>
              </div>
              <button
                data-nav-item
                type="button"
                disabled={isCheckingUpdate || isDownloadingUpdate}
                class="px-4 py-2 rounded-[10px] bg-white/10 hover:bg-white/20 focus:ring-1 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-2 disabled:opacity-50"
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
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <span class="px-2 py-0.5 rounded-[6px] bg-sky-500/20 text-sky-400 text-xs font-mono font-bold">
                      Доступна новая версия: {updateCheckResult.latestVersion}
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
                      class="px-4 py-2 rounded-[10px] bg-sky-500 hover:bg-sky-400 text-black text-xs font-mono font-black cursor-pointer transition-colors flex items-center gap-2 shadow-md"
                      onclick={handleStartDownloadUpdate}
                    >
                      <Download class="w-4 h-4 stroke-[2.5]" />
                      <span>Скачать обновление</span>
                      <span class="w-4 h-4 rounded-full bg-black/20 text-black text-[9px] font-black flex items-center justify-center font-mono">A</span>
                    </button>
                  {:else if isUpdateReadyToInstall}
                    <button
                      data-nav-item
                      type="button"
                      class="px-5 py-2.5 rounded-[10px] bg-emerald-500 hover:bg-emerald-400 text-black text-xs font-mono font-black cursor-pointer transition-colors flex items-center gap-2 shadow-lg"
                      onclick={handleApplyUpdate}
                    >
                      <Check class="w-4 h-4 stroke-[3]" />
                      <span>Перезапустить и обновить</span>
                      <span class="w-4 h-4 rounded-full bg-black/20 text-black text-[9px] font-black flex items-center justify-center font-mono">A</span>
                    </button>
                  {/if}
                </div>

                <!-- Download Progress Bar -->
                {#if isDownloadingUpdate && updateProgress}
                  <div class="space-y-1.5 pt-1">
                    <div class="flex items-center justify-between text-xs font-mono text-[#cbd5e1]">
                      <span>Загрузка файла обновления...</span>
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
                    <div class="text-[11px] font-mono text-[#8e95a2] uppercase tracking-wider mb-1">Что нового:</div>
                    <div class="max-h-36 overflow-y-auto text-xs font-mono text-[#94a3b8] whitespace-pre-wrap leading-relaxed p-2.5 rounded bg-black/40 border border-white/[0.04]">
                      {updateCheckResult.releaseNotes}
                    </div>
                  </div>
                {/if}
              </div>
            {/if}
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
              class="px-4 py-2 rounded-[10px] bg-white/10 hover:bg-white/20 focus:ring-1 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-1.5"
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
                class="px-4 py-2 rounded-[10px] bg-white/10 hover:bg-white/20 focus:ring-1 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-1.5 disabled:opacity-50"
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
              class="w-12 h-6 rounded-full transition-colors cursor-pointer relative focus:ring-1 focus:ring-white focus:outline-none {localSettings.enableLogs ? 'bg-sky-500' : 'bg-white/10'}"
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

          <div class="flex items-center justify-between py-3 border-b border-white/[0.06] text-sm">
            <div>
              <p class="text-white font-mono font-medium">Распаковка архивов</p>
              <p class="text-xs text-[#64748b] font-mono">Автоматически распаковывать многотомные архивы (.rar, .zip, .7z) после загрузки</p>
            </div>
            <button
              data-nav-item
              type="button"
              aria-label="Автоматическая распаковка архивов"
              title="Автоматическая распаковка архивов"
              class="w-12 h-6 rounded-full transition-colors cursor-pointer relative focus:ring-1 focus:ring-white focus:outline-none {localSettings.autoExtractArchives ? 'bg-sky-500' : 'bg-white/10'}"
              onclick={() => {
                sound.playSelect();
                localSettings.autoExtractArchives = !localSettings.autoExtractArchives;
                triggerSave();
              }}
            >
              <span class="w-4 h-4 rounded-full bg-white transition-transform absolute top-1 {localSettings.autoExtractArchives ? 'left-7' : 'left-1'}"></span>
            </button>
          </div>

          <!-- Actions -->
          <div class="pt-4 flex items-center gap-4">
            <button
              data-nav-item
              class="px-5 py-3 rounded-[12px] bg-white/10 hover:bg-white/20 focus:ring-1 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold flex items-center gap-2 cursor-pointer transition-colors"
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
              class="px-5 py-3 rounded-[12px] bg-rose-500/20 hover:bg-rose-500/30 focus:ring-1 focus:ring-rose-400 focus:outline-none text-rose-300 text-xs font-mono font-bold flex items-center gap-2 cursor-pointer transition-colors"
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
              class="px-4 py-2.5 rounded-[10px] bg-white/10 hover:bg-white/20 focus:ring-1 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-2"
              onclick={handleClearLogs}
            >
              <X class="w-4 h-4 text-rose-400" />
              <span>Очистить</span>
            </button>

            <button
              data-nav-item
              type="button"
              class="px-5 py-2.5 rounded-[10px] bg-sky-600 hover:bg-sky-500 focus:ring-1 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold cursor-pointer transition-colors flex items-center gap-2 shadow-md"
              onclick={handleExportLogs}
            >
              <Download class="w-4 h-4" />
              <span>Экспорт в файл</span>
            </button>
          </div>
        </div>

        {#if exportMessage}
          <div class="p-3 rounded-[10px] bg-sky-500/10 border border-sky-500/30 text-xs font-mono text-sky-300 flex items-center justify-between">
            <span>{exportMessage}</span>
            <button onclick={() => (exportMessage = null)} class="text-white/40 hover:text-white p-1">
              <X class="w-4 h-4" />
            </button>
          </div>
        {/if}

        <!-- Filter & Search Controls (Gamepad Navigable) -->
        <div class="p-3.5 rounded-[14px] bg-[#0d1017] border border-white/[0.08] flex flex-wrap items-center justify-between gap-3">
          <!-- Level Buttons -->
          <div class="flex items-center gap-2">
            {#each ['ALL', 'INFO', 'WARN', 'ERROR', 'DEBUG'] as lvl}
              <button
                data-nav-item
                type="button"
                class="px-3.5 py-1.5 rounded-[8px] text-xs font-mono font-bold uppercase transition-colors cursor-pointer focus:ring-1 focus:ring-white focus:outline-none {selectedLogLevel === lvl ? 'bg-white/20 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/5'}"
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
                class="w-full bg-[#111622] border border-white/[0.08] px-3 py-1.5 text-xs font-mono text-white rounded-[10px] placeholder:text-white/30 focus:outline-none focus:ring-1 focus:ring-sky-400"
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
              class="px-3 py-1.5 rounded-[10px] border border-white/10 text-xs font-mono transition-colors cursor-pointer focus:ring-1 focus:ring-white focus:outline-none {autoScrollLogs ? 'bg-sky-500/20 text-sky-300 border-sky-500/40' : 'bg-white/5 text-[#8e95a2]'}"
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
          class="flex-1 min-h-[380px] max-h-[calc(100vh-340px)] overflow-y-auto bg-[#06080c] border border-white/[0.08] rounded-[14px] p-4 font-mono text-xs select-text space-y-1"
        >
          {#if displayedLogs.length === 0}
            <div class="h-full min-h-[260px] flex flex-col items-center justify-center text-center text-[#64748b]">
              <Terminal class="w-10 h-10 stroke-1 mb-2 opacity-30" />
              <p class="text-sm">Журнал работы пуст</p>
            </div>
          {:else}
            {#each displayedLogs as entry (entry.id)}
              <div class="flex items-start gap-2.5 py-1 px-1.5 rounded-[6px] hover:bg-white/[0.04] transition-colors leading-relaxed break-all">
                <span class="text-[#64748b] text-xs flex-shrink-0 select-none">{entry.timestamp}</span>
                <span class="px-2 py-0.5 rounded-[6px] text-[10px] font-bold flex-shrink-0 select-none {entry.level === 'ERROR' ? 'bg-rose-500/20 text-rose-300' : entry.level === 'WARN' ? 'bg-amber-500/20 text-amber-300' : entry.level === 'DEBUG' ? 'bg-purple-500/20 text-purple-300' : 'bg-sky-500/20 text-sky-300'}">
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

    </div>
  </main>
</div>
