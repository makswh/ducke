<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Bookmark,
    Search,
    Clock,
    Gamepad2,
    CheckCircle2,
    Layers,
    X,
    Folder,
    FolderOpen,
    Disc,
    Star,
    ChevronDown,
    Check,
    ArrowUpDown
  } from 'lucide-svelte';


  import GameDetailView from './GameDetailView.svelte';
  import { GetFavorites, SetFavoriteLaunchConfig, SelectGameExeFile, LaunchGameWithCustomConfig } from '../../../wailsjs/go/main/App';
  import { EventsOn } from '../../../wailsjs/runtime/runtime';


  let {
    downloadPath = 'C:\\Ducke',
    onStartDownload = (gameId: number, targetPath: string) => {},
    onSelectFolder = () => Promise.resolve(''),
    onOpenCatalog = () => {}
  } = $props();

  let favorites = $state<any[]>([]);
  let isLoading = $state<boolean>(true);
  let selectedTab = $state<'all' | 'planned' | 'playing' | 'completed'>('all');
  let searchQuery = $state<string>('');
  let selectedSort = $state<'recent' | 'rating' | 'name' | 'size'>('recent');
  let selectedGameId = $state<number | null>(null);
  let isStatusMenuOpen = $state<boolean>(false);
  let isSortMenuOpen = $state<boolean>(false);

  function getSortLabel(sort: string) {
    switch (sort) {
      case 'rating': return 'По оценке Steam';
      case 'name': return 'По названию (А-Я)';
      case 'size': return 'По размеру';
      default: return 'Недавние';
    }
  }

  async function loadFavorites() {
    try {
      isLoading = true;
      const res = await GetFavorites();
      favorites = Array.isArray(res) ? res : [];
      if (favorites.length > 0 && selectedGameId === null) {
        selectedGameId = favorites[0]?.game?.id || null;
      }
    } catch (e) {
      console.error('Failed to load favorites:', e);
    } finally {
      isLoading = false;
    }
  }

  let unsubFavorites: any = null;

  onMount(() => {
    loadFavorites();
    unsubFavorites = EventsOn('favorites:updated', () => {
      loadFavorites();
    });
  });

  onDestroy(() => {
    if (unsubFavorites) unsubFavorites();
  });

  // Filter and sort items
  let filteredItems = $derived.by(() => {
    let list = (favorites || []).slice();

    // Tab filter
    if (selectedTab !== 'all') {
      list = list.filter((item) => item.status === selectedTab);
    }

    // Search filter
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim();
      list = list.filter((item) => {
        const title = (item.game?.cleanTitle || item.game?.rawName || item.game?.steamTitle || '').toLowerCase();
        return title.includes(q);
      });
    }

    // Sort
    if (selectedSort === 'rating') {
      list.sort((a, b) => (b.game?.reviewPercent || 0) - (a.game?.reviewPercent || 0));
    } else if (selectedSort === 'name') {
      list.sort((a, b) => (a.game?.cleanTitle || '').localeCompare(b.game?.cleanTitle || ''));
    } else if (selectedSort === 'size') {
      list.sort((a, b) => (b.game?.sizeBytes || 0) - (a.game?.sizeBytes || 0));
    }
    // 'recent' keeps original order from database (ORDER BY updated_at DESC)

    return list;
  });

  // Counts for tabs
  let counts = $derived.by(() => {
    const res = { all: 0, planned: 0, playing: 0, completed: 0 };
    for (const item of favorites || []) {
      res.all++;
      if (item.status === 'planned') res.planned++;
      else if (item.status === 'playing') res.playing++;
      else if (item.status === 'completed') res.completed++;
    }
    return res;
  });

  let selectedGame = $derived.by(() => {
    if (!selectedGameId) return filteredItems[0]?.game || null;
    const found = filteredItems.find((item) =>
      item.game?.id === selectedGameId ||
      (item.game?.variants && item.game.variants.some((v: any) => v.id === selectedGameId))
    );
    return found ? found.game : (filteredItems[0]?.game || null);
  });

  let selectedItem = $derived.by(() => {
    if (!selectedGameId) return filteredItems[0] || null;
    const found = filteredItems.find((item) =>
      item.game?.id === selectedGameId ||
      (item.game?.variants && item.game.variants.some((v: any) => v.id === selectedGameId))
    );
    return found || (filteredItems[0] || null);
  });

  // Launch config local state — synced when selected item changes
  let configExePath = $state<string>('');
  let configLaunchArgs = $state<string>('');
  let isSaving = $state<boolean>(false);
  let saveSuccess = $state<boolean>(false);

  // Sync fields when selected item changes
  $effect(() => {
    if (selectedItem) {
      configExePath = selectedItem.customExePath || '';
      configLaunchArgs = selectedItem.launchArguments || '';
    }
  });

  async function browseExeFile() {
    const path = await SelectGameExeFile();
    if (path) configExePath = path;
  }

  async function saveLaunchConfig() {
    if (!selectedGame) return;
    isSaving = true;
    saveSuccess = false;
    try {
      await SetFavoriteLaunchConfig(selectedGame.id, configExePath.trim(), configLaunchArgs.trim());
      saveSuccess = true;
      setTimeout(() => (saveSuccess = false), 2000);
    } catch (e) {
      console.error('Failed to save launch config:', e);
    } finally {
      isSaving = false;
    }
  }

  async function launchWithCustomConfig() {
    if (!selectedGame) return;
    await LaunchGameWithCustomConfig(selectedGame.id);
  }

  function getStatusLabel(status: string) {

    if (status === 'playing') return 'Прохожу';
    if (status === 'completed') return 'Прошел';
    return 'В планах';
  }

  function getStatusBadgeStyle(status: string) {
    if (status === 'playing') return 'bg-amber-500/15 text-amber-300 border-amber-500/30';
    if (status === 'completed') return 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30';
    return 'bg-sky-500/15 text-sky-300 border-sky-500/30';
  }
</script>

<div data-nav-zone="favorites" class="flex-1 flex h-full overflow-hidden bg-[#07080a] text-white">
  <!-- 1. LEFT SIDEBAR: Master Game List with Top Header Inside -->
  <aside class="panel-master relative w-[310px] lg:w-[340px] flex flex-col flex-shrink-0 h-full select-none border-r border-white/[0.06] bg-[#07080a]">
    <!-- Top Header & Filter Controls (INSIDE SIDEBAR) -->
    <div class="p-3 pb-2.5 border-b border-white/[0.06] space-y-2 flex-shrink-0 bg-[#07080a]">
      <!-- Title & Count -->
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <Bookmark class="w-3.5 h-3.5 text-sky-400" />
          <h1 class="text-xs font-bold uppercase tracking-wider text-[#cbd5e1]">Избранное</h1>
        </div>
        <span class="text-[10px] font-mono font-semibold text-[#8e95a2] bg-white/[0.04] px-2 py-0.5 rounded border border-white/[0.06]">
          {filteredItems.length}{#if filteredItems.length !== counts.all} / {counts.all}{/if}
        </span>
      </div>

      <!-- Search Input with Clear Button -->
      <div class="relative flex items-center">
        <Search class="w-3.5 h-3.5 text-[#6b7280] absolute left-2.5 pointer-events-none" />
        <input
          data-nav-item
          type="text"
          bind:value={searchQuery}
          placeholder="Поиск в избранном..."
          class="w-full bg-[#07080a] text-[#ededed] placeholder-[#5a6170] text-xs rounded-lg pl-8 pr-7 py-1.5 border border-white/[0.08] focus:border-white/20 focus:outline-none transition-colors"
        />
        {#if searchQuery}
          <button
            type="button"
            class="absolute right-2 text-[#6b7280] hover:text-white cursor-pointer"
            onclick={() => (searchQuery = '')}
          >
            <X class="w-3.5 h-3.5" />
          </button>
        {/if}
      </div>

      <!-- Filter & Sort Controls -->
      <div class="flex items-center gap-1.5 relative">
        <!-- 1. Status Dropdown Button & Menu -->
        <div class="relative flex-1 min-w-0">
          <button
            type="button"
            data-nav-item
            class="w-full h-8 flex items-center justify-between bg-[#07080a] hover:bg-white/[0.04] text-[#ededed] text-[11px] font-medium px-2.5 rounded-lg border {selectedTab !== 'all' ? 'border-sky-500/60 text-white' : 'border-white/[0.08]'} transition-colors cursor-pointer truncate"
            onclick={() => {
              isStatusMenuOpen = !isStatusMenuOpen;
              if (isStatusMenuOpen) isSortMenuOpen = false;
            }}
          >
            <div class="flex items-center gap-1.5 truncate">
              {#if selectedTab === 'planned'}
                <Clock class="w-3 h-3 text-sky-400 flex-shrink-0" />
                <span class="truncate">Планы ({counts.planned})</span>
              {:else if selectedTab === 'playing'}
                <Gamepad2 class="w-3 h-3 text-amber-400 flex-shrink-0" />
                <span class="truncate">Прохожу ({counts.playing})</span>
              {:else if selectedTab === 'completed'}
                <CheckCircle2 class="w-3 h-3 text-emerald-400 flex-shrink-0" />
                <span class="truncate">Прошел ({counts.completed})</span>
              {:else}
                <Bookmark class="w-3 h-3 text-sky-400 flex-shrink-0" />
                <span class="truncate">Все ({counts.all})</span>
              {/if}
            </div>
            <ChevronDown class="w-3 h-3 text-[#6b7280] flex-shrink-0 ml-1 transition-transform {isStatusMenuOpen ? 'rotate-180' : ''}" />
          </button>

          {#if isStatusMenuOpen}
            <!-- Backdrop to click outside -->
            <button
              type="button"
              aria-label="Закрыть меню статусов"
              class="fixed inset-0 z-30 cursor-default bg-transparent border-none p-0 m-0 w-full h-full"
              onclick={() => (isStatusMenuOpen = false)}
            ></button>

            <!-- Popup Container -->
            <div class="absolute left-0 top-full mt-1.5 w-52 z-40 bg-[#0e1219] border border-white/10 rounded-xl shadow-2xl overflow-hidden flex flex-col p-1.5 space-y-0.5">
              <button
                type="button"
                class="w-full flex items-center justify-between px-2.5 py-2 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer {selectedTab === 'all' ? 'bg-white/10 text-white font-semibold' : 'text-[#9ca3af] hover:text-white hover:bg-white/[0.04]'}"
                onclick={() => {
                  selectedTab = 'all';
                  isStatusMenuOpen = false;
                }}
              >
                <div class="flex items-center gap-2">
                  <Bookmark class="w-3.5 h-3.5 {selectedTab === 'all' ? 'text-sky-400' : 'text-[#6b7280]'}" />
                  <span>Все игры</span>
                </div>
                <div class="flex items-center gap-1.5">
                  <span class="text-[10px] text-[#8e95a2] font-mono px-1.5 py-0.5 rounded bg-white/[0.04]">{counts.all}</span>
                  {#if selectedTab === 'all'}
                    <Check class="w-3 h-3 text-sky-400 flex-shrink-0" />
                  {/if}
                </div>
              </button>

              <button
                type="button"
                class="w-full flex items-center justify-between px-2.5 py-2 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer {selectedTab === 'planned' ? 'bg-white/10 text-white font-semibold' : 'text-[#9ca3af] hover:text-white hover:bg-white/[0.04]'}"
                onclick={() => {
                  selectedTab = 'planned';
                  isStatusMenuOpen = false;
                }}
              >
                <div class="flex items-center gap-2">
                  <Clock class="w-3.5 h-3.5 text-sky-400" />
                  <span>В планах</span>
                </div>
                <div class="flex items-center gap-1.5">
                  <span class="text-[10px] text-[#8e95a2] font-mono px-1.5 py-0.5 rounded bg-white/[0.04]">{counts.planned}</span>
                  {#if selectedTab === 'planned'}
                    <Check class="w-3 h-3 text-sky-400 flex-shrink-0" />
                  {/if}
                </div>
              </button>

              <button
                type="button"
                class="w-full flex items-center justify-between px-2.5 py-2 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer {selectedTab === 'playing' ? 'bg-white/10 text-white font-semibold' : 'text-[#9ca3af] hover:text-white hover:bg-white/[0.04]'}"
                onclick={() => {
                  selectedTab = 'playing';
                  isStatusMenuOpen = false;
                }}
              >
                <div class="flex items-center gap-2">
                  <Gamepad2 class="w-3.5 h-3.5 text-amber-400" />
                  <span>Прохожу</span>
                </div>
                <div class="flex items-center gap-1.5">
                  <span class="text-[10px] text-[#8e95a2] font-mono px-1.5 py-0.5 rounded bg-white/[0.04]">{counts.playing}</span>
                  {#if selectedTab === 'playing'}
                    <Check class="w-3 h-3 text-sky-400 flex-shrink-0" />
                  {/if}
                </div>
              </button>

              <button
                type="button"
                class="w-full flex items-center justify-between px-2.5 py-2 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer {selectedTab === 'completed' ? 'bg-white/10 text-white font-semibold' : 'text-[#9ca3af] hover:text-white hover:bg-white/[0.04]'}"
                onclick={() => {
                  selectedTab = 'completed';
                  isStatusMenuOpen = false;
                }}
              >
                <div class="flex items-center gap-2">
                  <CheckCircle2 class="w-3.5 h-3.5 text-emerald-400" />
                  <span>Прошел</span>
                </div>
                <div class="flex items-center gap-1.5">
                  <span class="text-[10px] text-[#8e95a2] font-mono px-1.5 py-0.5 rounded bg-white/[0.04]">{counts.completed}</span>
                  {#if selectedTab === 'completed'}
                    <Check class="w-3 h-3 text-sky-400 flex-shrink-0" />
                  {/if}
                </div>
              </button>
            </div>
          {/if}
        </div>

        <!-- 2. Sort Dropdown Button & Menu -->
        <div class="relative w-36 flex-shrink-0">
          <button
            type="button"
            data-nav-item
            class="w-full h-8 flex items-center justify-between bg-[#07080a] hover:bg-white/[0.04] text-[#9ca3af] hover:text-white text-[11px] font-medium px-2.5 rounded-lg border border-white/[0.08] transition-colors cursor-pointer truncate"
            onclick={() => {
              isSortMenuOpen = !isSortMenuOpen;
              if (isSortMenuOpen) isStatusMenuOpen = false;
            }}
          >
            <div class="flex items-center gap-1.5 truncate">
              <ArrowUpDown class="w-3 h-3 text-sky-400 flex-shrink-0" />
              <span class="truncate">{getSortLabel(selectedSort)}</span>
            </div>
            <ChevronDown class="w-3 h-3 text-[#6b7280] flex-shrink-0 ml-1 transition-transform {isSortMenuOpen ? 'rotate-180' : ''}" />
          </button>

          {#if isSortMenuOpen}
            <!-- Backdrop to click outside -->
            <button
              type="button"
              aria-label="Закрыть меню сортировки"
              class="fixed inset-0 z-30 cursor-default bg-transparent border-none p-0 m-0 w-full h-full"
              onclick={() => (isSortMenuOpen = false)}
            ></button>

            <!-- Popup Container -->
            <div class="absolute right-0 top-full mt-1.5 w-44 z-40 bg-[#0e1219] border border-white/10 rounded-xl shadow-2xl overflow-hidden flex flex-col p-1.5 space-y-0.5">
              {#each [
                { id: 'recent', label: 'Недавние' },
                { id: 'rating', label: 'По оценке Steam' },
                { id: 'name', label: 'По названию (А-Я)' },
                { id: 'size', label: 'По размеру' }
              ] as opt}
                {@const isSel = selectedSort === opt.id}
                <button
                  type="button"
                  class="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer {isSel ? 'bg-white/10 text-white font-semibold' : 'text-[#9ca3af] hover:text-white hover:bg-white/[0.04]'}"
                  onclick={() => {
                    selectedSort = opt.id as any;
                    isSortMenuOpen = false;
                  }}
                >
                  <span>{opt.label}</span>
                  {#if isSel}
                    <Check class="w-3 h-3 text-sky-400 flex-shrink-0 ml-1.5" />
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    </div>

    <!-- Games List Area -->
    <div class="flex-1 overflow-y-auto p-2 space-y-1">
      {#if isLoading}
        <div class="p-6 text-center text-xs text-[#8e95a2]">
          <span>Загрузка избранного...</span>
        </div>
      {:else if favorites.length === 0}
        <div class="p-6 text-center text-xs text-[#8e95a2] space-y-1">
          <p class="font-medium text-[#cbd5e1]">Список пуст</p>
          <p class="text-[11px]">Добавляйте игры со страницы игры</p>
        </div>
      {:else if filteredItems.length === 0}
        <div class="p-6 text-center text-xs text-[#8e95a2] space-y-2">
          <p>Ничего не найдено</p>
          {#if searchQuery}
            <button
              data-nav-item
              type="button"
              class="text-xs text-sky-400 hover:underline cursor-pointer"
              onclick={() => (searchQuery = '')}
            >
              Сбросить поиск
            </button>
          {/if}
        </div>
      {:else}
        {#each filteredItems as item (item.gameId)}
          {@const g = item.game}
          {@const isSelected = selectedGame?.id === g.id}
          {@const artUrl = g.headerImage || g.capsuleImage || g.backgroundImage}

          <div
            data-nav-item
            role="button"
            tabindex="0"
            class="group relative h-[58px] overflow-hidden rounded-xl cursor-pointer transition-colors duration-150 border-2 {isSelected ? 'border-sky-500 bg-[#131722]' : 'border-white/[0.04] bg-white/[0.02] hover:bg-white/[0.06] hover:border-white/10'}"
            onclick={() => (selectedGameId = g.id)}
            onkeydown={(e) => {
              if (e.key === 'Enter') selectedGameId = g.id;
            }}
          >
            {#if artUrl}
              <div
                class="absolute inset-0 bg-cover bg-center transition-opacity duration-200 pointer-events-none {isSelected ? 'opacity-30' : 'opacity-15 group-hover:opacity-25'}"
                style="background-image: url('{artUrl}');"
              ></div>
              <div class="absolute inset-0 bg-gradient-to-r from-[#07080a]/95 via-[#07080a]/80 to-[#07080a]/60 pointer-events-none"></div>
            {/if}

            <div class="relative z-10 p-2.5 px-3 h-full flex flex-col justify-between">
              <div class="flex items-center justify-between gap-2">
                <h3 class="text-xs font-semibold leading-snug truncate {isSelected ? 'text-white' : 'text-[#d1d5db] group-hover:text-white'}">
                  {g.cleanTitle || g.rawName}
                </h3>
                <span class="text-[9px] font-bold px-1.5 py-0.2 rounded border flex-shrink-0 {getStatusBadgeStyle(item.status)}">
                  {getStatusLabel(item.status)}
                </span>
              </div>

              <div class="flex items-center justify-between text-[10px]">
                <span class="text-[#8e95a2] truncate font-mono">
                  {g.releaseDate || ''}
                </span>
                {#if g.sizeDisplay}
                  <span class="font-mono text-[#94a3b8] px-1.5 py-0.5 rounded bg-black/60 border border-white/[0.06] flex-shrink-0 ml-2">
                    {g.sizeDisplay}
                  </span>
                {/if}
              </div>
            </div>
          </div>
        {/each}
      {/if}
    </div>
  </aside>

  <!-- 2. RIGHT DETAIL PANE: Full Window Height -->
  <div class="flex-1 flex overflow-hidden min-w-0 h-full">
    {#if isLoading}
      <div class="flex-1 flex items-center justify-center p-8 text-xs text-[#8e95a2]">
        <span>Загрузка...</span>
      </div>
    {:else if favorites.length === 0}
      <!-- Empty State -->
      <div class="flex-1 flex flex-col items-center justify-center p-8 text-center space-y-4 select-none">
        <div class="w-14 h-14 rounded-2xl bg-white/[0.04] border border-white/10 flex items-center justify-center text-[#64748b]">
          <Bookmark class="w-7 h-7 stroke-[1.5]" />
        </div>
        <div class="space-y-1 max-w-sm">
          <h3 class="text-sm font-bold text-white">Список избранного пуст</h3>
          <p class="text-xs text-[#8e95a2]">
            Добавляйте понравившиеся игры в «В планах», «Прохожу» или «Прошел» со страницы любой игры каталога.
          </p>
        </div>
        <button
          data-nav-item
          type="button"
          class="px-5 py-2.5 rounded-xl bg-white text-slate-950 text-xs font-bold hover:bg-white/90 transition-transform active:scale-95 cursor-pointer flex items-center gap-2"
          onclick={onOpenCatalog}
        >
          <Layers class="w-4 h-4" />
          <span>Перейти в каталог</span>
        </button>
      </div>
    {:else if selectedGame}
      <GameDetailView
        game={selectedGame}
        {downloadPath}
        {onStartDownload}
        {onSelectFolder}
        favoriteItem={selectedItem}
      />
    {:else}
      <div class="flex-1 flex items-center justify-center text-xs text-[#8e95a2]">
        <span>Выберите игру из списка</span>
      </div>
    {/if}
  </div>
</div>

