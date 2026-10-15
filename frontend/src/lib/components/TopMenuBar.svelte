<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Gear,
    FolderOpen,
    TelevisionSimple,
    SignOut,
    ArrowClockwise,
    Trash,
    Pause,
    Play,
    Sliders,
    Info,
    CheckCircle,
    DownloadSimple,
    Terminal,
    FileText,
    BookmarkSimple,
    Folder,
    Command,
    HardDrive,
    Database,
    Magnet
  } from 'phosphor-svelte';

  let {
    downloadPath = 'C:\\Ducke',
    hasActiveDownloads = false,
    hasPausedDownloads = false,
    onOpenSettingsSubTab = (subTab: string) => {},
    onOpenFavoritesFilter = (filter: string) => {},
    onOpenFolder = (path: string) => {},
    onOpenConfigFolder = () => {},
    onToggleBigPicture = () => {},
    onRefreshCatalog = () => {},
    onClearMetadataCache = () => {},
    onPauseAllDownloads = () => {},
    onResumeAllDownloads = () => {},
    onClearCompletedDownloads = () => {},
    onImportFileZilla = () => {},
    onTestConnection = () => {},
    onCheckForUpdates = () => {},
    onExportLogs = () => {},
    onOpenShortcutsHelp = () => {},
    onQuit = () => {}
  } = $props();

  type MenuId = 'ducke' | 'view' | 'tools' | 'help' | null;
  let activeMenu = $state<MenuId>(null);
  let menuBarContainer: HTMLDivElement | null = null;

  function toggleMenu(id: MenuId) {
    activeMenu = activeMenu === id ? null : id;
  }

  function handleMenuHover(id: MenuId) {
    if (activeMenu !== null) {
      activeMenu = id;
    }
  }

  function closeMenu() {
    activeMenu = null;
  }

  function handleAction(action: () => void) {
    closeMenu();
    action();
  }

  function handleWindowClick(e: MouseEvent) {
    if (menuBarContainer && !menuBarContainer.contains(e.target as Node)) {
      closeMenu();
    }
  }

  function handleWindowKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape' && activeMenu !== null) {
      closeMenu();
      e.stopPropagation();
    }
  }

  onMount(() => {
    window.addEventListener('click', handleWindowClick);
    window.addEventListener('keydown', handleWindowKeyDown);
    return () => {
      window.removeEventListener('click', handleWindowClick);
      window.removeEventListener('keydown', handleWindowKeyDown);
    };
  });
</script>

<!-- Steam Desktop Top Menu Bar (Integrated in TitleBar) -->
<nav
  bind:this={menuBarContainer}
  class="flex items-center h-full gap-0.5 relative select-none font-sans"
  style="--wails-draggable: no-drag;"
>
  <!-- 1. Menu: Logo + DUCKE (Triggers Ducke Dropdown) -->
  <div class="relative">
    <button
      type="button"
      class="flex items-center gap-2 px-2 py-1 rounded-[10px] transition-colors cursor-pointer {activeMenu === 'ducke' ? 'bg-white/10 text-white shadow-sm' : 'hover:bg-white/[0.06] text-[#cbd5e1]'}"
      onclick={(e) => {
        e.stopPropagation();
        toggleMenu('ducke');
      }}
      onmouseenter={() => handleMenuHover('ducke')}
      title="Меню Ducke"
    >
      <div class="w-6 h-6 rounded-[8px] bg-white/[0.04] flex items-center justify-center overflow-hidden">
        <img src="/appicon.png" alt="Ducke" class="w-4.5 h-4.5 object-contain" />
      </div>
      <span class="font-bold font-mono text-[11px] tracking-wider uppercase text-white">DUCKE</span>
    </button>

    {#if activeMenu === 'ducke'}
      <div
        class="absolute left-0 top-full mt-1.5 z-50 w-64 rounded-[14px] bg-[#0d1117] border border-white/10 shadow-2xl py-1.5 text-xs text-[#cbd5e1] font-sans divide-y divide-white/[0.06] animate-fade-in"
      >
        <div class="py-1">
          <div class="px-3.5 py-1 text-[10px] font-mono uppercase tracking-wider text-[#64748b] font-semibold">
            Настройки приложения
          </div>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center justify-between hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenSettingsSubTab('storage'))}
          >
            <div class="flex items-center gap-2">
              <HardDrive size={14} class="text-sky-400" />
              <span>Хранилище игр</span>
            </div>
            <kbd class="text-[10px] font-mono text-[#64748b]">Ctrl+,</kbd>
          </button>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center justify-between hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenSettingsSubTab('server'))}
          >
            <div class="flex items-center gap-2">
              <Database size={14} class="text-sky-400" />
              <span>FTP-серверы</span>
            </div>
          </button>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center justify-between hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenSettingsSubTab('torrents'))}
          >
            <div class="flex items-center gap-2">
              <Magnet size={14} class="text-sky-400" />
              <span>Источники торрентов</span>
            </div>
          </button>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center justify-between hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenSettingsSubTab('downloads'))}
          >
            <div class="flex items-center gap-2">
              <Sliders size={14} class="text-sky-400" />
              <span>Сеть и загрузки</span>
            </div>
          </button>
        </div>

        <div class="py-1">
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenFolder(downloadPath))}
            title={downloadPath}
          >
            <FolderOpen size={14} class="text-[#8e95a2]" />
            <span>Папка загрузок игры</span>
          </button>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenConfigFolder())}
          >
            <Folder size={14} class="text-[#8e95a2]" />
            <span>Папка конфигурации Ducke</span>
          </button>
        </div>

        <div class="py-1">
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center justify-between hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onToggleBigPicture())}
          >
            <div class="flex items-center gap-2">
              <TelevisionSimple size={14} class="text-sky-400" />
              <span>Режим Big Picture</span>
            </div>
            <kbd class="text-[10px] font-mono text-[#64748b]">F11</kbd>
          </button>
        </div>

        <div class="py-1">
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center justify-between hover:bg-rose-500/15 text-rose-300 hover:text-rose-200 transition-colors cursor-pointer"
            onclick={() => handleAction(() => onQuit())}
          >
            <div class="flex items-center gap-2">
              <SignOut size={14} />
              <span>Выход из Ducke</span>
            </div>
            <kbd class="text-[10px] font-mono text-rose-400/60">Alt+F4</kbd>
          </button>
        </div>
      </div>
    {/if}
  </div>

  <!-- 2. Menu: Вид (View) -->
  <div class="relative">
    <button
      type="button"
      class="px-2.5 py-1 rounded-[8px] text-[12px] font-medium transition-colors cursor-pointer {activeMenu === 'view' ? 'bg-white/10 text-white font-semibold' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05]'}"
      onclick={(e) => {
        e.stopPropagation();
        toggleMenu('view');
      }}
      onmouseenter={() => handleMenuHover('view')}
    >
      Вид
    </button>

    {#if activeMenu === 'view'}
      <div
        class="absolute left-0 top-full mt-1.5 z-50 w-60 rounded-[14px] bg-[#0d1117] border border-white/10 shadow-2xl py-1.5 text-xs text-[#cbd5e1] font-sans divide-y divide-white/[0.06] animate-fade-in"
      >
        <div class="py-1">
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center justify-between hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onRefreshCatalog())}
          >
            <div class="flex items-center gap-2">
              <ArrowClockwise size={14} class="text-sky-400" />
              <span>Обновить каталоги</span>
            </div>
            <kbd class="text-[10px] font-mono text-[#64748b]">F5</kbd>
          </button>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onClearMetadataCache())}
          >
            <Trash size={14} class="text-[#8e95a2]" />
            <span>Очистить кэш обложек...</span>
          </button>
        </div>

        <div class="py-1">
          <div class="px-3.5 py-1 text-[10px] font-mono uppercase tracking-wider text-[#64748b] font-semibold">
            Быстрый переход в Избранное
          </div>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenFavoritesFilter('planned'))}
          >
            <span class="w-2 h-2 rounded-full bg-amber-400"></span>
            <span>В планах</span>
          </button>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenFavoritesFilter('playing'))}
          >
            <span class="w-2 h-2 rounded-full bg-sky-400"></span>
            <span>Прохожу сейчас</span>
          </button>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenFavoritesFilter('completed'))}
          >
            <span class="w-2 h-2 rounded-full bg-emerald-400"></span>
            <span>Пройденные игры</span>
          </button>
        </div>

        <div class="py-1">
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenSettingsSubTab('logs'))}
          >
            <Terminal size={14} class="text-[#8e95a2]" />
            <span>Журнал событий (Логи)</span>
          </button>
        </div>
      </div>
    {/if}
  </div>

  <!-- 3. Menu: Инструменты (Tools) -->
  <div class="relative">
    <button
      type="button"
      class="px-2.5 py-1 rounded-[8px] text-[12px] font-medium transition-colors cursor-pointer {activeMenu === 'tools' ? 'bg-white/10 text-white font-semibold' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05]'}"
      onclick={(e) => {
        e.stopPropagation();
        toggleMenu('tools');
      }}
      onmouseenter={() => handleMenuHover('tools')}
    >
      Инструменты
    </button>

    {#if activeMenu === 'tools'}
      <div
        class="absolute left-0 top-full mt-1.5 z-50 w-64 rounded-[14px] bg-[#0d1117] border border-white/10 shadow-2xl py-1.5 text-xs text-[#cbd5e1] font-sans divide-y divide-white/[0.06] animate-fade-in"
      >
        <div class="py-1">
          <div class="px-3.5 py-1 text-[10px] font-mono uppercase tracking-wider text-[#64748b] font-semibold">
            Управление очередью загрузок
          </div>
          <button
            type="button"
            disabled={!hasActiveDownloads}
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            onclick={() => handleAction(() => onPauseAllDownloads())}
          >
            <Pause size={14} class="text-amber-400" />
            <span>Приостановить всё</span>
          </button>
          <button
            type="button"
            disabled={!hasPausedDownloads}
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            onclick={() => handleAction(() => onResumeAllDownloads())}
          >
            <Play size={14} class="text-emerald-400" />
            <span>Возобновить всё</span>
          </button>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onClearCompletedDownloads())}
          >
            <Trash size={14} class="text-[#8e95a2]" />
            <span>Очистить завершенные</span>
          </button>
        </div>

        <div class="py-1">
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onImportFileZilla())}
          >
            <FileText size={14} class="text-sky-400" />
            <span>Импорт FileZilla (XML)...</span>
          </button>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onTestConnection())}
          >
            <CheckCircle size={14} class="text-emerald-400" />
            <span>Проверить FTP соединение</span>
          </button>
        </div>
      </div>
    {/if}
  </div>

  <!-- 4. Menu: Справка (Help) -->
  <div class="relative">
    <button
      type="button"
      class="px-2.5 py-1 rounded-[8px] text-[12px] font-medium transition-colors cursor-pointer {activeMenu === 'help' ? 'bg-white/10 text-white font-semibold' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.05]'}"
      onclick={(e) => {
        e.stopPropagation();
        toggleMenu('help');
      }}
      onmouseenter={() => handleMenuHover('help')}
    >
      Справка
    </button>

    {#if activeMenu === 'help'}
      <div
        class="absolute left-0 top-full mt-1.5 z-50 w-60 rounded-[14px] bg-[#0d1117] border border-white/10 shadow-2xl py-1.5 text-xs text-[#cbd5e1] font-sans divide-y divide-white/[0.06] animate-fade-in"
      >
        <div class="py-1">
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center justify-between hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenShortcutsHelp())}
          >
            <div class="flex items-center gap-2">
              <Command size={14} class="text-sky-400" />
              <span>Горячие клавиши</span>
            </div>
            <kbd class="text-[10px] font-mono text-[#64748b]">F1</kbd>
          </button>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onCheckForUpdates())}
          >
            <ArrowClockwise size={14} class="text-[#8e95a2]" />
            <span>Проверить обновления...</span>
          </button>
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onExportLogs())}
          >
            <DownloadSimple size={14} class="text-[#8e95a2]" />
            <span>Экспорт журнала логов...</span>
          </button>
        </div>

        <div class="py-1">
          <button
            type="button"
            class="w-full text-left px-3.5 py-1.5 flex items-center gap-2 hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
            onclick={() => handleAction(() => onOpenSettingsSubTab('about'))}
          >
            <Info size={14} class="text-[#8e95a2]" />
            <span>О программе Ducke</span>
          </button>
        </div>
      </div>
    {/if}
  </div>
</nav>
