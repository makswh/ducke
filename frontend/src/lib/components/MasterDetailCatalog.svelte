<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Gamepad2,
    HardDrive,
    Search,
    X,
    Layers,
    Tag,
    ChevronDown,
    FolderOpen,
    Check
  } from 'lucide-svelte';
  import type { GameEntity } from '../types/game';
  import { deduplicateGames } from '../utils/gameDeduplication';
  import GameDetailView from './GameDetailView.svelte';

  let {
    games = [] as GameEntity[],
    searchQuery = $bindable<string>(''),
    downloadPath = '',
    isLoading = false,
    loadingStatusText = '',
    onStartDownload = (gameId: number, targetPath: string) => {},
    onSelectFolder = async (): Promise<string> => ''
  } = $props();

  const SAVED_FILTER_KEY = 'ducke_catalog_selected_filter';
  const SAVED_GENRE_KEY = 'ducke_catalog_selected_genre';
  const SAVED_SORT_KEY = 'ducke_catalog_selected_sort';

  function getStoredFilter(key: string, fallback: string): string {
    try {
      if (typeof localStorage !== 'undefined') {
        const v = localStorage.getItem(key);
        if (v) return v;
      }
    } catch {}
    return fallback;
  }

  let selectedFilter = $state<string>(getStoredFilter(SAVED_FILTER_KEY, 'all'));
  let selectedGenre = $state<string>(getStoredFilter(SAVED_GENRE_KEY, 'all'));
  let genreSearchQuery = $state<string>('');
  let isGenreMenuOpen = $state<boolean>(false);
  let selectedSort = $state<'date_desc' | 'name' | 'size_desc' | 'size_asc' | 'rating_desc' | 'popular_desc'>(
    getStoredFilter(SAVED_SORT_KEY, 'date_desc') as any
  );

  $effect(() => {
    try {
      if (typeof localStorage !== 'undefined') {
        localStorage.setItem(SAVED_FILTER_KEY, selectedFilter);
      }
    } catch {}
  });

  $effect(() => {
    try {
      if (typeof localStorage !== 'undefined') {
        localStorage.setItem(SAVED_GENRE_KEY, selectedGenre);
      }
    } catch {}
  });

  $effect(() => {
    try {
      if (typeof localStorage !== 'undefined') {
        localStorage.setItem(SAVED_SORT_KEY, selectedSort);
      }
    } catch {}
  });
  let selectedGameId = $state<number | null>(null);
  let imageLoadFailed = $state<Record<string, boolean>>({});

  // Debounced search query (150ms) to keep input instantaneous on 10k+ libraries
  let debouncedSearchQuery = $state<string>('');
  let debounceTimer: any = null;

  $effect(() => {
    const raw = searchQuery;
    if (debounceTimer) clearTimeout(debounceTimer);
    if (!raw) {
      debouncedSearchQuery = '';
    } else {
      debounceTimer = setTimeout(() => {
        debouncedSearchQuery = raw;
      }, 150);
    }
  });

  onDestroy(() => {
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
  });

  // Deduplicate only when raw 'games' array reference changes and precompute search corpus & flags
  let deduplicatedList = $derived.by(() => {
    const list = deduplicateGames(games || []);
    return list.map((g) => {
      const gList = (g.genres || []).map((x: string) => x.toLowerCase().trim()).filter(Boolean);
      const tList = (g.tags || []).map((x: string) => x.toLowerCase().trim()).filter(Boolean);
      const pList = (g.publishers || []).map((x: string) => x.toLowerCase().trim()).filter(Boolean);
      const vList = (g.variants || []).map((v: any) => `${v.rawName || ''} ${v.torrentSource || ''}`.toLowerCase().trim()).filter(Boolean);
      const searchCorpus = `${g.cleanTitle || ''} ${g.steamTitle || ''} ${gList.join(' ')} ${tList.join(' ')} ${pList.join(' ')} ${vList.join(' ')}`.toLowerCase();

      const hasController = g.controllerSupport === 'full' || g.controllerSupport === 'partial';
      const isRpg = gList.some((x: string) => x.includes('rpg') || x.includes('ролев'));
      const isAction = gList.some((x: string) => x.includes('action') || x.includes('экшен'));
      const isCollection = !!(g.isCollection || (g.parentPath && g.parentPath !== ''));
      const isUnder10gb = (g.sizeBytes || 0) > 0 && (g.sizeBytes || 0) <= 10 * 1024 * 1024 * 1024;
      const isOver50gb = (g.sizeBytes || 0) >= 50 * 1024 * 1024 * 1024;

      return {
        ...g,
        _searchCorpus: searchCorpus,
        _genreLowerSet: new Set(gList),
        _hasController: hasController,
        _isRpg: isRpg,
        _isAction: isAction,
        _isCollection: isCollection,
        _isUnder10gb: isUnder10gb,
        _isOver50gb: isOver50gb
      };
    });
  });

  let availableGenres = $derived.by(() => {
    const counts = new Map<string, number>();
    for (const g of deduplicatedList) {
      if (g.genres && Array.isArray(g.genres)) {
        for (const raw of g.genres) {
          const genre = raw.trim();
          if (genre) {
            counts.set(genre, (counts.get(genre) || 0) + 1);
          }
        }
      }
    }
    return Array.from(counts.entries())
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count);
  });

  let filteredGenreList = $derived.by(() => {
    if (!genreSearchQuery.trim()) return availableGenres;
    const q = genreSearchQuery.toLowerCase();
    return availableGenres.filter((item) => item.name.toLowerCase().includes(q));
  });

  function getFilterLabel(filter: string): string {
    switch (filter) {
      case 'controller': return 'С геймпадом';
      case 'rpg': return 'RPG / Ролевые';
      case 'action': return 'Экшены';
      case 'collections': return 'Саги / Сборники';
      case 'under10gb': return '< 10 ГБ';
      case 'over50gb': return '> 50 ГБ';
      default: return 'Все жанры';
    }
  }

  // Single-pass filter over deduplicated list
  let filteredGames = $derived.by(() => {
    const q = debouncedSearchQuery.trim().toLowerCase();
    const selGenre = selectedGenre !== 'all' ? selectedGenre.toLowerCase() : null;
    const selFilter = selectedFilter;

    let result = deduplicatedList.filter((g) => {
      // 1. Fast search match using precomputed corpus
      if (q && !g._searchCorpus.includes(q)) {
        return false;
      }

      // 2. Fast genre match using Set O(1)
      if (selGenre && !g._genreLowerSet.has(selGenre)) {
        return false;
      }

      // 3. Fast filter presets using precomputed booleans
      if (selFilter === 'controller') {
        return g._hasController;
      }
      if (selFilter === 'rpg') {
        return g._isRpg;
      }
      if (selFilter === 'action') {
        return g._isAction;
      }
      if (selFilter === 'collections') {
        return g._isCollection;
      }
      if (selFilter === 'under10gb') {
        return g._isUnder10gb;
      }
      if (selFilter === 'over50gb') {
        return g._isOver50gb;
      }

      return true;
    });

    if (selectedSort === 'size_desc') {
      result.sort((a, b) => (b.sizeBytes || 0) - (a.sizeBytes || 0));
    } else if (selectedSort === 'size_asc') {
      result.sort((a, b) => (a.sizeBytes || 0) - (b.sizeBytes || 0));
    } else if (selectedSort === 'name') {
      result.sort((a, b) => (a.cleanTitle || '').localeCompare(b.cleanTitle || ''));
    } else if (selectedSort === 'rating_desc') {
      result.sort((a, b) => {
        const diff = (b.reviewPercent || 0) - (a.reviewPercent || 0);
        if (diff !== 0) return diff;
        return (b.totalReviews || 0) - (a.totalReviews || 0);
      });
    } else if (selectedSort === 'popular_desc') {
      result.sort((a, b) => {
        const scoreA = Math.log10((a.totalReviews || 0) + 1) * ((a.reviewPercent || 0) / 100);
        const scoreB = Math.log10((b.totalReviews || 0) + 1) * ((b.reviewPercent || 0) / 100);
        return scoreB - scoreA;
      });
    } else {
      result.sort((a, b) => (b.id || 0) - (a.id || 0));
    }

    return result;
  });

  function ensureGameVisible(gameId: number) {
    if (!listContainer || filteredGames.length === 0) return;
    const idx = filteredGames.findIndex((g) => g.id === gameId);
    if (idx < 0) return;
    const itemTop = idx * ITEM_SLOT;
    const itemBottom = itemTop + ITEM_SLOT;
    const curTop = listContainer.scrollTop;
    const curBottom = curTop + containerHeight;

    if (itemTop < curTop) {
      listContainer.scrollTop = itemTop;
    } else if (itemBottom > curBottom) {
      listContainer.scrollTop = itemBottom - containerHeight;
    }
  }

  // Auto-select first game when list loads, and track merged games
  $effect(() => {
    if (filteredGames.length > 0 && selectedGameId === null) {
      selectedGameId = filteredGames[0].id;
      return;
    }

    if (selectedGameId !== null && deduplicatedList.length > 0) {
      const existsDirectly = deduplicatedList.some((g) => g.id === selectedGameId);
      if (!existsDirectly) {
        // Game was merged as a variant into a primary parent game!
        const parent = deduplicatedList.find((g) =>
          g.variants && g.variants.some((v: any) => v.id === selectedGameId)
        );
        if (parent) {
          selectedGameId = parent.id;
          ensureGameVisible(parent.id);
        }
      }
    }
  });

  let selectedGame = $derived.by(() => {
    if (filteredGames.length === 0) return null;
    if (!selectedGameId) return filteredGames[0];

    // 1. Direct top-level match
    const direct = filteredGames.find((g) => g.id === selectedGameId);
    if (direct) return direct;

    // 2. Merged variant match in filteredGames
    const mergedInFiltered = filteredGames.find((g) =>
      g.variants && g.variants.some((v: any) => v.id === selectedGameId)
    );
    if (mergedInFiltered) return mergedInFiltered;

    // 3. Merged variant match in entire deduplicated list
    const mergedInAll = deduplicatedList.find((g) =>
      g.id === selectedGameId || (g.variants && g.variants.some((v: any) => v.id === selectedGameId))
    );
    if (mergedInAll) return mergedInAll;

    // 4. Fallback to first filtered game
    return filteredGames[0];
  });

  function isPlaceholderTitle(title: string | undefined | null): boolean {
    if (!title) return true;
    const t = title.trim();
    return t === '' || /^Steam App \d+$/i.test(t);
  }

  function getDisplayTitle(game: GameEntity | null): string {
    if (!game) return '';
    const raw = (!isPlaceholderTitle(game.steamTitle))
      ? game.steamTitle!
      : (game.cleanTitle && game.cleanTitle.trim() !== '' ? game.cleanTitle : (game.rawName || ''));
    return raw
      .replace(/^[\{\[\(]\s*(linux|win|windows|mac|macos|pc|gog|steam|portable|repack|native|unpack|unpacked)\s*[\}\]\)]\s*/gi, '')
      .replace(/\s*[\{\[\(]\s*(linux|win|windows|mac|macos|pc|gog|steam|portable|repack|native|unpack|unpacked)\s*[\}\]\)]$/gi, '')
      .replace(/[\{\}]/g, '')
      .replace(/[\s\-_]+(?:\[|\()?(\d+([.,]\d+)?\s*(?:gb|mb|tb|гб|мб|тб|g|m|t))(?:\)|\])?$/i, '')
      .replace(/(?:\[|\()?(\d+([.,]\d+)?\s*(?:gb|mb|tb|гб|мб|тб))(?:\)|\])?$/i, '')
      .trim();
  }

  function formatSizeDisplay(game: GameEntity | null): string {
    if (!game) return '';
    if (game.sizeDisplay && game.sizeDisplay.trim() !== '' && game.sizeDisplay.toLowerCase() !== 'unknown') {
      return game.sizeDisplay;
    }
    if (game.sizeBytes && game.sizeBytes > 0) {
      const gb = game.sizeBytes / (1024 * 1024 * 1024);
      if (gb >= 1) return `${gb.toFixed(1)} GB`;
      const mb = game.sizeBytes / (1024 * 1024);
      return `${mb.toFixed(0)} MB`;
    }
    return '';
  }

  // Virtualization constants & calculations
  const ITEM_HEIGHT = 58;
  const GAP = 4;
  const ITEM_SLOT = ITEM_HEIGHT + GAP;
  const OVERSCAN = 6;

  let listContainer = $state<HTMLDivElement | null>(null);
  let scrollTop = $state<number>(0);
  let containerHeight = $state<number>(600);

  function handleScroll() {
    if (listContainer) {
      scrollTop = listContainer.scrollTop;
    }
  }

  let totalListHeight = $derived.by(() => {
    return filteredGames.length > 0 ? filteredGames.length * ITEM_SLOT - GAP : 0;
  });

  let startIndex = $derived.by(() => {
    if (filteredGames.length === 0) return 0;
    const idx = Math.floor(scrollTop / ITEM_SLOT);
    return Math.max(0, idx - OVERSCAN);
  });

  let endIndex = $derived.by(() => {
    if (filteredGames.length === 0) return 0;
    const idx = Math.ceil((scrollTop + containerHeight) / ITEM_SLOT);
    return Math.min(filteredGames.length, idx + OVERSCAN);
  });

  let offsetY = $derived(startIndex * ITEM_SLOT);

  let visibleGames = $derived.by(() => {
    return filteredGames.slice(startIndex, endIndex);
  });

  // Reset scroll position when filter, genre, sort, or search changes
  $effect(() => {
    const _ = [debouncedSearchQuery, selectedGenre, selectedFilter, selectedSort];
    if (listContainer) {
      listContainer.scrollTop = 0;
      scrollTop = 0;
    }
  });

  onMount(() => {
    if (!listContainer) return;
    containerHeight = listContainer.clientHeight || 600;

    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.target === listContainer) {
          containerHeight = listContainer.clientHeight || 600;
        }
      }
    });
    ro.observe(listContainer);

    return () => ro.disconnect();
  });
</script>

<div class="flex-1 flex h-full overflow-hidden bg-[#07080a]">
  <!-- 1. LEFT SIDEBAR: Master Game List -->
  <aside data-nav-zone="list" class="panel-master relative w-[310px] lg:w-[340px] flex flex-col flex-shrink-0 h-full select-none border-r border-white/[0.06] bg-[#07080a]">
    <!-- Search & Filter Controls -->
    <div class="p-3 pb-2.5 border-b border-white/[0.06] space-y-2">
      <div class="flex items-center justify-between">
        <h1 class="text-xs font-bold uppercase tracking-wider text-[#cbd5e1]">Библиотека</h1>
        <span class="text-[10px] font-mono font-semibold text-[#8e95a2] bg-white/[0.04] px-2 py-0.5 rounded border border-white/[0.06]">
          {filteredGames.length}
        </span>
      </div>

      <!-- Search Input -->
      <div class="relative flex items-center">
        <Search class="w-3.5 h-3.5 text-[#6b7280] absolute left-2.5 pointer-events-none" />
        <input
          data-nav-item
          data-nav-search
          type="text"
          bind:value={searchQuery}
          placeholder="Поиск в библиотеке..."
          class="w-full bg-[#07080a] text-[#ededed] placeholder-[#5a6170] text-xs rounded-lg pl-8 pr-7 py-1.5 border border-white/[0.08] focus:border-white/20 focus:outline-none transition-colors"
        />
        {#if searchQuery}
          <button
            class="absolute right-2 text-[#6b7280] hover:text-white cursor-pointer"
            onclick={() => (searchQuery = '')}
          >
            <X class="w-3.5 h-3.5" />
          </button>
        {/if}
      </div>

      <!-- Filter & Sort Controls -->
      <div class="flex items-center gap-1.5 relative">
        <!-- Genre / Category Selector Dropdown Button & Anchored Menu -->
        <div class="relative flex-1 min-w-0">
          <button
            type="button"
            data-nav-item
            data-nav-filter
            class="w-full h-8 flex items-center justify-between bg-[#07080a] hover:bg-white/[0.04] text-[#9ca3af] hover:text-white text-[11px] font-medium px-2.5 rounded-lg border {selectedGenre !== 'all' || selectedFilter !== 'all' ? 'border-sky-500/60 text-white' : 'border-white/[0.08]'} transition-colors cursor-pointer truncate"
            onclick={() => {
              isGenreMenuOpen = !isGenreMenuOpen;
              genreSearchQuery = '';
            }}
          >
            <div class="flex items-center gap-1.5 truncate">
              <Tag class="w-3 h-3 text-sky-400 flex-shrink-0" />
              <span class="truncate">
                {selectedGenre !== 'all' ? selectedGenre : (selectedFilter !== 'all' ? getFilterLabel(selectedFilter) : 'Все жанры')}
              </span>
            </div>
            <ChevronDown class="w-3 h-3 text-[#6b7280] flex-shrink-0 ml-1 transition-transform {isGenreMenuOpen ? 'rotate-180' : ''}" />
          </button>

          <!-- Genre Dropdown Menu Popup - Anchored Under the Button -->
          {#if isGenreMenuOpen}
            <!-- Backdrop to click outside -->
            <button
              type="button"
              aria-label="Закрыть меню жанров"
              class="fixed inset-0 z-30 cursor-default bg-transparent border-none p-0 m-0 w-full h-full"
              onclick={() => (isGenreMenuOpen = false)}
            ></button>

            <!-- Popup Container -->
            <div class="absolute left-0 top-full mt-1.5 w-[280px] z-40 bg-[#0e1219] border border-white/10 rounded-xl shadow-2xl overflow-hidden flex flex-col max-h-[340px]">
              {#if availableGenres.length > 5}
                <div class="p-2 border-b border-white/[0.06] bg-black/40">
                  <div class="relative flex items-center">
                    <Search class="w-3 h-3 text-[#6b7280] absolute left-2.5 pointer-events-none" />
                    <input
                      type="text"
                      bind:value={genreSearchQuery}
                      placeholder="Поиск жанра..."
                      class="w-full bg-[#07080a] text-white text-[11px] placeholder-[#5a6170] rounded-md pl-7 pr-7 py-1 border border-white/10 focus:border-sky-500 focus:outline-none"
                      onclick={(e) => e.stopPropagation()}
                    />
                    {#if genreSearchQuery}
                      <button
                        type="button"
                        class="absolute right-2 text-[#6b7280] hover:text-white cursor-pointer"
                        onclick={(e) => {
                          e.stopPropagation();
                          genreSearchQuery = '';
                        }}
                      >
                        <X class="w-3 h-3" />
                      </button>
                    {/if}
                  </div>
                </div>
              {/if}

              <!-- Scrollable list -->
              <div class="overflow-y-auto p-1.5 space-y-0.5 max-h-[280px] text-xs">
                <!-- All games option -->
                <button
                  type="button"
                  class="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer {selectedGenre === 'all' && selectedFilter === 'all' ? 'bg-white/10 text-white font-semibold' : 'text-[#9ca3af] hover:text-white hover:bg-white/[0.04]'}"
                  onclick={() => {
                    selectedGenre = 'all';
                    selectedFilter = 'all';
                    isGenreMenuOpen = false;
                  }}
                >
                  <div class="flex items-center gap-2">
                    <Layers class="w-3.5 h-3.5 {selectedGenre === 'all' && selectedFilter === 'all' ? 'text-sky-400' : 'text-[#6b7280]'}" />
                    <span>Все игры</span>
                  </div>
                  <span class="text-[10px] text-[#8e95a2] font-mono px-1.5 py-0.5 rounded bg-white/[0.04]">{deduplicatedList.length}</span>
                </button>

                <!-- Dynamic genres -->
                {#if filteredGenreList.length > 0}
                  <div class="px-2 pt-2.5 pb-1 text-[9px] font-bold uppercase tracking-wider text-[#6b7280]">Жанры ({filteredGenreList.length})</div>
                  {#each filteredGenreList as item}
                    {@const isCurrent = selectedGenre.toLowerCase() === item.name.toLowerCase()}
                    <button
                      type="button"
                      class="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer {isCurrent ? 'bg-white/10 text-white font-semibold' : 'text-[#9ca3af] hover:text-white hover:bg-white/[0.04]'}"
                      onclick={() => {
                        selectedGenre = item.name;
                        isGenreMenuOpen = false;
                      }}
                    >
                      <div class="flex items-center gap-1.5 truncate">
                        {#if isCurrent}
                          <Check class="w-3 h-3 text-sky-400 flex-shrink-0" />
                        {/if}
                        <span class="truncate">{item.name}</span>
                      </div>
                      <span class="text-[10px] text-[#8e95a2] font-mono px-1.5 py-0.5 rounded bg-white/[0.04] ml-2 flex-shrink-0">{item.count}</span>
                    </button>
                  {/each}
                {/if}

                <!-- Special Presets Section -->
                <div class="px-2 pt-2.5 pb-1 text-[9px] font-bold uppercase tracking-wider text-[#6b7280]">Особенности</div>
                <button
                  type="button"
                  class="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer {selectedFilter === 'controller' ? 'bg-white/10 text-white font-semibold' : 'text-[#9ca3af] hover:text-white hover:bg-white/[0.04]'}"
                  onclick={() => {
                    selectedFilter = 'controller';
                    selectedGenre = 'all';
                    isGenreMenuOpen = false;
                  }}
                >
                  <div class="flex items-center gap-2">
                    <Gamepad2 class="w-3.5 h-3.5 text-[#9ca3af]" />
                    <span>С геймпадом</span>
                  </div>
                </button>
                <button
                  type="button"
                  class="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer {selectedFilter === 'collections' ? 'bg-white/10 text-white font-semibold' : 'text-[#9ca3af] hover:text-white hover:bg-white/[0.04]'}"
                  onclick={() => {
                    selectedFilter = 'collections';
                    selectedGenre = 'all';
                    isGenreMenuOpen = false;
                  }}
                >
                  <div class="flex items-center gap-2">
                    <FolderOpen class="w-3.5 h-3.5 text-[#9ca3af]" />
                    <span>Саги / Сборники</span>
                  </div>
                </button>
                <button
                  type="button"
                  class="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer {selectedFilter === 'under10gb' ? 'bg-white/10 text-white font-semibold' : 'text-[#9ca3af] hover:text-white hover:bg-white/[0.04]'}"
                  onclick={() => {
                    selectedFilter = 'under10gb';
                    selectedGenre = 'all';
                    isGenreMenuOpen = false;
                  }}
                >
                  <div class="flex items-center gap-2">
                    <HardDrive class="w-3.5 h-3.5 text-[#9ca3af]" />
                    <span>&lt; 10 ГБ</span>
                  </div>
                </button>
                <button
                  type="button"
                  class="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer {selectedFilter === 'over50gb' ? 'bg-white/10 text-white font-semibold' : 'text-[#9ca3af] hover:text-white hover:bg-white/[0.04]'}"
                  onclick={() => {
                    selectedFilter = 'over50gb';
                    selectedGenre = 'all';
                    isGenreMenuOpen = false;
                  }}
                >
                  <div class="flex items-center gap-2">
                    <HardDrive class="w-3.5 h-3.5 text-[#9ca3af]" />
                    <span>&gt; 50 ГБ</span>
                  </div>
                </button>
              </div>
            </div>
          {/if}
        </div>

        <!-- Sort selector -->
        <select
          data-nav-item
          bind:value={selectedSort}
          class="h-8 bg-[#07080a] text-[#9ca3af] hover:text-white text-[11px] font-medium px-2 rounded-lg border border-white/[0.08] focus:outline-none focus:border-white/20 cursor-pointer flex-shrink-0 transition-colors"
        >
          <option value="date_desc">Новые</option>
          <option value="popular_desc">По популярности</option>
          <option value="rating_desc">По оценке Steam</option>
          <option value="name">А — Я</option>
          <option value="size_desc">Большие</option>
          <option value="size_asc">Лёгкие</option>
        </select>
      </div>

      <!-- Active Filter Pill -->
      {#if selectedGenre !== 'all' || selectedFilter !== 'all'}
        <div class="flex items-center gap-1.5 flex-wrap pt-0.5">
          {#if selectedGenre !== 'all'}
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-white/[0.06] border border-white/10 text-[10px] font-medium text-white">
              <span>{selectedGenre}</span>
              <button
                type="button"
                class="hover:text-rose-400 cursor-pointer ml-0.5"
                onclick={() => (selectedGenre = 'all')}
                title="Сбросить фильтр по жанру"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}
          {#if selectedFilter !== 'all'}
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-white/[0.06] border border-white/10 text-[10px] font-medium text-white">
              <span>{getFilterLabel(selectedFilter)}</span>
              <button
                type="button"
                class="hover:text-rose-400 cursor-pointer ml-0.5"
                onclick={() => (selectedFilter = 'all')}
                title="Сбросить фильтр"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}
        </div>
      {/if}
    </div>

    <!-- Master Game List Items (Virtual List) -->
    <div
      bind:this={listContainer}
      onscroll={handleScroll}
      class="flex-1 overflow-y-auto p-2 select-none"
    >
      {#if isLoading && filteredGames.length === 0}
        <!-- Minimal loading indicator in sidebar -->
        <div class="px-3 py-2 mb-1.5 text-[11px] text-[#525a6c] font-mono tracking-wider uppercase truncate">
          {loadingStatusText || 'Загрузка...'}
        </div>
        <div class="flex flex-col gap-1 w-full">
          {#each Array(9) as _, i}
            <div class="h-[58px] rounded-xl sk-block p-2.5 px-3 flex flex-col justify-between" style="animation-delay: {i * 60}ms">
              <div class="sk-line h-3 w-3/4"></div>
              <div class="flex justify-between items-center">
                <div class="sk-line h-2 w-16"></div>
                <div class="sk-line h-2 w-12"></div>
              </div>
            </div>
          {/each}
        </div>
      {:else if filteredGames.length === 0}
        <div class="p-8 text-center text-xs text-[#6b7280] space-y-2">
          <Layers class="w-6 h-6 mx-auto text-[#4b5563]" />
          <p>Игр не найдено</p>
        </div>
      {:else}
        <!-- Virtual total height spacer -->
        <div style="height: {totalListHeight}px; position: relative; width: 100%;">
          <!-- Rendered visible games slice shifted via translateY -->
          <div
            style="transform: translateY({offsetY}px); will-change: transform;"
            class="flex flex-col gap-1 w-full"
          >
            {#each visibleGames as game (game.id)}
              {@const isSelected = selectedGame?.id === game.id}
              {@const artUrl = game.headerImage || game.capsuleImage || game.backgroundImage}

              <div
                data-nav-item
                role="button"
                tabindex="0"
                class="group relative h-[58px] overflow-hidden rounded-xl cursor-pointer transition-colors duration-150 border-2 {isSelected ? 'border-sky-500 bg-[#131722]' : 'border-white/[0.04] bg-white/[0.02] hover:bg-white/[0.06] hover:border-white/10'}"
                onclick={() => {
                  selectedGameId = game.id;
                }}
                onkeydown={(e) => {
                  if (e.key === 'Enter') {
                    selectedGameId = game.id;
                  }
                }}
              >
                <!-- Background Cover -->
                {#if artUrl && !imageLoadFailed[artUrl]}
                  <div
                    class="absolute inset-0 bg-cover bg-center transition-opacity duration-200 pointer-events-none {isSelected ? 'opacity-30' : 'opacity-15 group-hover:opacity-25'}"
                    style="background-image: url('{artUrl}');"
                  ></div>
                  <div class="absolute inset-0 bg-gradient-to-r from-[#07080a]/95 via-[#07080a]/80 to-[#07080a]/60 pointer-events-none"></div>
                {/if}

                <!-- Card Content -->
                <div class="relative z-10 p-2.5 px-3 h-full flex flex-col justify-between">
                  <div class="flex items-center gap-2 min-w-0">
                    {#if game.iconUrl && !imageLoadFailed[game.iconUrl]}
                      <img
                        src={game.iconUrl}
                        alt=""
                        decoding="async"
                        class="w-5 h-5 rounded flex-shrink-0 object-contain"
                        onerror={() => {
                          if (game.iconUrl) {
                            imageLoadFailed[game.iconUrl] = true;
                          }
                        }}
                      />
                    {/if}
                    <h3 class="text-xs font-semibold leading-snug truncate {isSelected ? 'text-white' : 'text-[#d1d5db] group-hover:text-white'}">
                      {getDisplayTitle(game)}
                    </h3>
                  </div>

                  <div class="flex items-center justify-between text-[10px]">
                    <span class="text-[#8e95a2] truncate font-mono">
                      {game.releaseDate || ''}
                    </span>
                    {#if formatSizeDisplay(game)}
                      <span class="font-mono text-[#94a3b8] px-1.5 py-0.5 rounded bg-black/60 border border-white/[0.06] flex-shrink-0 ml-2">
                        {formatSizeDisplay(game)}
                      </span>
                    {/if}
                  </div>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </div>
  </aside>

  <!-- 2. MAIN DETAIL VIEW -->
  <GameDetailView
    game={selectedGame}
    {isLoading}
    {loadingStatusText}
    {downloadPath}
    {onStartDownload}
    {onSelectFolder}
    onSelectGenre={(g: string) => {
      selectedGenre = g;
    }}
    onSelectTag={(t: string) => {
      searchQuery = t;
    }}
  />
</div>
