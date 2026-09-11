<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Bookmark,
    BookmarkCheck,
    Search,
    Download,
    Trash2,
    Check,
    Clock,
    Gamepad2,
    CheckCircle2,
    Layers,
    LayoutGrid,
    List,
    X,
    Folder,
    Disc,
    Star,
    ExternalLink,
    ChevronDown,
    WifiOff
  } from 'lucide-svelte';

  import GameDetailView from './GameDetailView.svelte';
  import {
    GetFavorites,
    SetFavoriteStatus,
    RemoveFromFavorites
  } from '../../../wailsjs/go/main/App';
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
  let viewMode = $state<'grid' | 'master_detail'>('grid');
  let selectedGameId = $state<number | null>(null);
  let activeDropdownGameId = $state<number | null>(null);

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

  async function handleSetStatus(gameId: number, status: string) {
    try {
      activeDropdownGameId = null;
      await SetFavoriteStatus(gameId, status);
      await loadFavorites();
    } catch (e) {
      console.error('Failed to update favorite status:', e);
    }
  }

  async function handleRemove(gameId: number) {
    try {
      activeDropdownGameId = null;
      await RemoveFromFavorites(gameId);
      await loadFavorites();
    } catch (e) {
      console.error('Failed to remove from favorites:', e);
    }
  }

  // Close dropdown on outside click
  function handleWindowClick(e: MouseEvent) {
    if (activeDropdownGameId !== null) {
      const target = e.target as HTMLElement;
      if (!target.closest('.status-dropdown-container')) {
        activeDropdownGameId = null;
      }
    }
  }

  let unsubFavorites: any = null;

  onMount(() => {
    loadFavorites();
    window.addEventListener('click', handleWindowClick);
    unsubFavorites = EventsOn('favorites:updated', () => {
      loadFavorites();
    });
  });

  onDestroy(() => {
    window.removeEventListener('click', handleWindowClick);
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
    const found = filteredItems.find((item) => item.game?.id === selectedGameId);
    return found ? found.game : (filteredItems[0]?.game || null);
  });

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

<div data-nav-zone="favorites" class="flex-1 flex flex-col h-full overflow-hidden bg-[#07080a] text-white">
  <!-- Top Header & Filter Controls -->
  <header class="p-4 sm:p-5 border-b border-white/[0.06] bg-[#090b10] flex-shrink-0 space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <!-- Title & Offline Status -->
      <div class="flex items-center gap-3">
        <div class="w-9 h-9 rounded-xl bg-white/[0.05] border border-white/10 flex items-center justify-center text-sky-400">
          <Bookmark class="w-5 h-5 stroke-[2]" />
        </div>
        <div>
          <h1 class="text-base sm:text-lg font-bold text-white tracking-wide flex items-center gap-2">
            <span>Избранное и бэклог</span>
            <span class="text-xs font-normal text-[#6b7280] font-mono">({counts.all})</span>
          </h1>
          <p class="text-[11px] text-[#8e95a2] flex items-center gap-1.5">
            <span>Метаданные и ссылки сохранены для автономного доступа</span>
          </p>
        </div>
      </div>

      <!-- View Mode & Action Buttons -->
      <div class="flex items-center gap-2">
        <div class="flex items-center bg-[#0d1117] border border-white/[0.08] rounded-lg p-0.5">
          <button
            data-nav-item
            type="button"
            class="h-7 px-2.5 rounded text-xs flex items-center gap-1.5 transition-colors cursor-pointer {viewMode === 'grid' ? 'bg-white/10 text-white font-semibold' : 'text-[#8e95a2] hover:text-white'}"
            onclick={() => (viewMode = 'grid')}
            title="Вид сеткой"
          >
            <LayoutGrid class="w-3.5 h-3.5" />
            <span class="hidden sm:inline">Сетка</span>
          </button>
          <button
            data-nav-item
            type="button"
            class="h-7 px-2.5 rounded text-xs flex items-center gap-1.5 transition-colors cursor-pointer {viewMode === 'master_detail' ? 'bg-white/10 text-white font-semibold' : 'text-[#8e95a2] hover:text-white'}"
            onclick={() => (viewMode = 'master_detail')}
            title="Вид списком с деталями"
          >
            <List class="w-3.5 h-3.5" />
            <span class="hidden sm:inline">Список</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Category Tabs & Search Row -->
    <div class="flex flex-wrap items-center justify-between gap-3 pt-1">
      <!-- Status Tabs -->
      <div class="flex items-center gap-1.5 overflow-x-auto no-scrollbar pb-1 sm:pb-0">
        <button
          data-nav-item
          type="button"
          class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer border {selectedTab === 'all' ? 'bg-white text-slate-950 font-bold border-white' : 'bg-white/[0.04] text-[#8e95a2] hover:text-white border-white/[0.06] hover:border-white/15'}"
          onclick={() => (selectedTab = 'all')}
        >
          Все ({counts.all})
        </button>
        <button
          data-nav-item
          type="button"
          class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer border flex items-center gap-1.5 {selectedTab === 'planned' ? 'bg-sky-400 text-slate-950 font-bold border-sky-400' : 'bg-white/[0.04] text-[#8e95a2] hover:text-white border-white/[0.06] hover:border-white/15'}"
          onclick={() => (selectedTab = 'planned')}
        >
          <Clock class="w-3.5 h-3.5" />
          <span>В планах ({counts.planned})</span>
        </button>
        <button
          data-nav-item
          type="button"
          class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer border flex items-center gap-1.5 {selectedTab === 'playing' ? 'bg-amber-400 text-slate-950 font-bold border-amber-400' : 'bg-white/[0.04] text-[#8e95a2] hover:text-white border-white/[0.06] hover:border-white/15'}"
          onclick={() => (selectedTab = 'playing')}
        >
          <Gamepad2 class="w-3.5 h-3.5" />
          <span>Прохожу ({counts.playing})</span>
        </button>
        <button
          data-nav-item
          type="button"
          class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer border flex items-center gap-1.5 {selectedTab === 'completed' ? 'bg-emerald-400 text-slate-950 font-bold border-emerald-400' : 'bg-white/[0.04] text-[#8e95a2] hover:text-white border-white/[0.06] hover:border-white/15'}"
          onclick={() => (selectedTab = 'completed')}
        >
          <CheckCircle2 class="w-3.5 h-3.5" />
          <span>Прошел ({counts.completed})</span>
        </button>
      </div>

      <!-- Search & Sort -->
      <div class="flex items-center gap-2 flex-1 sm:flex-initial justify-end">
        <div class="relative min-w-[180px] max-w-[260px] flex-1">
          <Search class="w-3.5 h-3.5 text-[#6b7280] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
          <input
            data-nav-item
            type="text"
            bind:value={searchQuery}
            placeholder="Поиск в избранном..."
            class="w-full bg-[#0d1017] text-white text-xs pl-8 pr-3 py-1.5 rounded-lg border border-white/[0.08] focus:outline-none focus:border-white/20 placeholder-[#6b7280]"
          />
        </div>

        <select
          data-nav-item
          bind:value={selectedSort}
          class="h-8 bg-[#0d1017] text-[#9ca3af] hover:text-white text-xs px-2.5 rounded-lg border border-white/[0.08] focus:outline-none focus:border-white/20 cursor-pointer flex-shrink-0"
        >
          <option value="recent">Недавние</option>
          <option value="rating">По оценке Steam</option>
          <option value="name">А — Я</option>
          <option value="size">По размеру</option>
        </select>
      </div>
    </div>
  </header>

  <!-- Content Stage -->
  {#if isLoading}
    <div class="flex-1 flex items-center justify-center p-8 text-xs text-[#8e95a2]">
      <span>Загрузка избранного...</span>
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
  {:else if filteredItems.length === 0}
    <!-- Search / Filter Zero Results -->
    <div class="flex-1 flex flex-col items-center justify-center p-8 text-center space-y-2">
      <p class="text-xs text-[#8e95a2]">По вашему запросу ничего не найдено</p>
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
    <!-- VIEW MODE: GRID -->
    {#if viewMode === 'grid'}
      <div class="flex-1 overflow-y-auto p-4 sm:p-6 select-none">
        <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4 sm:gap-5">
          {#each filteredItems as item (item.gameId)}
            {@const g = item.game}
            {@const artUrl = g?.capsuleImage || g?.headerImage || g?.backgroundImage}
            {@const isDropdownOpen = activeDropdownGameId === item.gameId}

            <div
              data-nav-item
              class="group relative bg-[#0e121a] rounded-2xl border border-white/[0.06] hover:border-white/20 transition-all flex flex-col overflow-hidden shadow-lg hover:shadow-2xl"
            >
              <!-- Poster Aspect 2:3 -->
              <div
                role="button"
                tabindex="0"
                class="relative aspect-[2/3] w-full overflow-hidden bg-black/60 cursor-pointer"
                onclick={() => {
                  selectedGameId = g.id;
                  viewMode = 'master_detail';
                }}
                onkeydown={(e) => {
                  if (e.key === 'Enter') {
                    selectedGameId = g.id;
                    viewMode = 'master_detail';
                  }
                }}
              >
                {#if artUrl}
                  <img
                    src={artUrl}
                    alt={g.cleanTitle || g.rawName}
                    class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
                  />
                {:else}
                  <div class="w-full h-full flex flex-col items-center justify-center p-3 text-center text-[#6b7280]">
                    <Disc class="w-8 h-8 opacity-40 mb-2" />
                    <span class="text-[11px] font-semibold line-clamp-3">{g.cleanTitle || g.rawName}</span>
                  </div>
                {/if}

                <!-- Status Badge Overlay Top Left -->
                <div class="absolute top-2 left-2 z-10">
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-bold border backdrop-blur-md {getStatusBadgeStyle(item.status)}">
                    {#if item.status === 'playing'}
                      <Gamepad2 class="w-3 h-3" />
                    {:else if item.status === 'completed'}
                      <CheckCircle2 class="w-3 h-3" />
                    {:else}
                      <Clock class="w-3 h-3" />
                    {/if}
                    <span>{getStatusLabel(item.status)}</span>
                  </span>
                </div>

                <!-- Rating badge Top Right if present -->
                {#if g.reviewPercent > 0}
                  <div class="absolute top-2 right-2 z-10">
                    <span class="px-1.5 py-0.5 rounded bg-black/80 text-emerald-400 text-[10px] font-mono font-bold border border-emerald-500/30 backdrop-blur-md">
                      {g.reviewPercent}%
                    </span>
                  </div>
                {/if}
              </div>

              <!-- Card Body -->
              <div class="p-3 flex-1 flex flex-col justify-between gap-2.5">
                <div
                  role="button"
                  tabindex="0"
                  class="cursor-pointer"
                  onclick={() => {
                    selectedGameId = g.id;
                    viewMode = 'master_detail';
                  }}
                  onkeydown={(e) => {
                    if (e.key === 'Enter') {
                      selectedGameId = g.id;
                      viewMode = 'master_detail';
                    }
                  }}
                >
                  <h3 class="text-xs font-bold text-white line-clamp-1 group-hover:text-sky-300 transition-colors" title={g.cleanTitle || g.rawName}>
                    {g.cleanTitle || g.rawName}
                  </h3>
                  <div class="text-[10px] font-mono text-[#8e95a2] mt-0.5 flex items-center justify-between">
                    <span>{g.sizeDisplay || '—'}</span>
                    {#if g.sourceType === 'torrent'}
                      <span class="text-amber-400/80">Торрент</span>
                    {:else}
                      <span>FTP</span>
                    {/if}
                  </div>
                </div>

                <!-- Action Controls Row -->
                <div class="relative status-dropdown-container pt-1 border-t border-white/[0.04] flex items-center justify-between gap-1.5">
                  <!-- Status Switcher Dropdown Button -->
                  <div class="relative flex-1">
                    <button
                      data-nav-item
                      type="button"
                      class="w-full text-left flex items-center justify-between px-2 py-1 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] text-[10px] font-medium text-[#cbd5e1] border border-white/[0.06] hover:border-white/15 transition-colors cursor-pointer"
                      onclick={(e) => {
                        e.stopPropagation();
                        activeDropdownGameId = isDropdownOpen ? null : item.gameId;
                      }}
                    >
                      <span class="truncate">{getStatusLabel(item.status)}</span>
                      <ChevronDown class="w-3 h-3 text-[#8e95a2] flex-shrink-0 ml-1 {isDropdownOpen ? 'rotate-180' : ''}" />
                    </button>

                    <!-- Status Dropdown Menu -->
                    {#if isDropdownOpen}
                      <div class="absolute left-0 bottom-full mb-1.5 z-50 w-36 rounded-xl bg-[#121620] border border-white/15 shadow-2xl p-1 space-y-0.5 text-xs backdrop-blur-md">
                        <button
                          type="button"
                          class="w-full text-left px-2.5 py-1.5 rounded-lg flex items-center gap-2 hover:bg-white/10 cursor-pointer {item.status === 'planned' ? 'text-sky-400 font-bold bg-white/5' : 'text-[#cbd5e1]'}"
                          onclick={() => handleSetStatus(item.gameId, 'planned')}
                        >
                          <Clock class="w-3.5 h-3.5" />
                          <span>В планах</span>
                        </button>
                        <button
                          type="button"
                          class="w-full text-left px-2.5 py-1.5 rounded-lg flex items-center gap-2 hover:bg-white/10 cursor-pointer {item.status === 'playing' ? 'text-amber-400 font-bold bg-white/5' : 'text-[#cbd5e1]'}"
                          onclick={() => handleSetStatus(item.gameId, 'playing')}
                        >
                          <Gamepad2 class="w-3.5 h-3.5" />
                          <span>Прохожу</span>
                        </button>
                        <button
                          type="button"
                          class="w-full text-left px-2.5 py-1.5 rounded-lg flex items-center gap-2 hover:bg-white/10 cursor-pointer {item.status === 'completed' ? 'text-emerald-400 font-bold bg-white/5' : 'text-[#cbd5e1]'}"
                          onclick={() => handleSetStatus(item.gameId, 'completed')}
                        >
                          <CheckCircle2 class="w-3.5 h-3.5" />
                          <span>Прошел</span>
                        </button>
                      </div>
                    {/if}
                  </div>

                  <!-- Remove from favorites button -->
                  <button
                    data-nav-item
                    type="button"
                    class="p-1.5 rounded-lg text-[#8e95a2] hover:text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
                    onclick={() => handleRemove(item.gameId)}
                    title="Удалить из избранного"
                  >
                    <Trash2 class="w-3.5 h-3.5 stroke-[1.75]" />
                  </button>
                </div>
              </div>
            </div>
          {/each}
        </div>
      </div>

    <!-- VIEW MODE: MASTER-DETAIL -->
    {:else}
      <div class="flex-1 flex overflow-hidden min-h-0">
        <!-- Left Master List (320px) -->
        <div class="w-72 sm:w-80 border-r border-white/[0.06] flex flex-col bg-[#090b10] flex-shrink-0 select-none overflow-y-auto p-2 space-y-1">
          {#each filteredItems as item (item.gameId)}
            {@const g = item.game}
            {@const isSelected = selectedGame?.id === g.id}
            {@const artUrl = g.headerImage || g.capsuleImage || g.backgroundImage}

            <div
              data-nav-item
              role="button"
              tabindex="0"
              class="group relative h-[58px] overflow-hidden rounded-xl cursor-pointer transition-colors duration-150 border-2 {isSelected ? 'border-sky-500 bg-[#131722]' : 'border-white/[0.04] bg-[#0c0e14]/90 hover:bg-[#11141c] hover:border-white/10'}"
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
        </div>

        <!-- Right Game Detail Pane -->
        <div class="flex-1 flex overflow-hidden min-w-0">
          {#if selectedGame}
            <GameDetailView
              game={selectedGame}
              {downloadPath}
              {onStartDownload}
              {onSelectFolder}
            />
          {:else}
            <div class="flex-1 flex items-center justify-center text-xs text-[#8e95a2]">
              <span>Выберите игру из списка</span>
            </div>
          {/if}
        </div>
      </div>
    {/if}
  {/if}
</div>
