<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    GameController,
    GameController as Gamepad2,
    HardDrive,
    MagnifyingGlass,
    MagnifyingGlass as Search,
    X,
    SquaresFour,
    SquaresFour as Layers,
    Tag,
    CaretDown,
    FolderOpen,
    Check,
    Sliders,
    Star,
    Calendar,
    ArrowLeft
  } from 'phosphor-svelte';
  import type { GameEntity } from '../types/game';
  import { deduplicateGames } from '../utils/gameDeduplication';
  import { getDisplayTitle } from '../utils/titleUtils';
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

  const SAVED_SORT_KEY = 'ducke_catalog_selected_sort';
  const SAVED_GENRE_KEY = 'ducke_catalog_selected_genre';
  const SAVED_TAGS_KEY = 'ducke_catalog_selected_tags';
  const SAVED_RATING_KEY = 'ducke_catalog_selected_rating';
  const SAVED_SIZE_KEY = 'ducke_catalog_selected_size';
  const SAVED_YEAR_KEY = 'ducke_catalog_selected_year';

  function getStoredFilter(key: string, fallback: string): string {
    try {
      if (typeof localStorage !== 'undefined') {
        const v = localStorage.getItem(key);
        if (v) return v;
      }
    } catch {}
    return fallback;
  }

  let selectedSort = $state<'date_desc' | 'name' | 'size_desc' | 'size_asc' | 'rating_desc' | 'popular_desc'>(
    getStoredFilter(SAVED_SORT_KEY, 'date_desc') as any
  );
  let selectedGenre = $state<string>(getStoredFilter(SAVED_GENRE_KEY, 'all'));
  let selectedTags = $state<string[]>(() => {
    try {
      const v = localStorage.getItem(SAVED_TAGS_KEY);
      if (v) {
        const parsed = JSON.parse(v);
        if (Array.isArray(parsed)) return parsed.filter((x) => typeof x === 'string');
      }
    } catch {}
    return [];
  });
  let selectedRating = $state<string>(getStoredFilter(SAVED_RATING_KEY, 'all'));
  let selectedSize = $state<string>(getStoredFilter(SAVED_SIZE_KEY, 'all'));
  let selectedYear = $state<string>(getStoredFilter(SAVED_YEAR_KEY, 'all'));
  let filterController = $state<boolean>(false);
  let filterCollections = $state<boolean>(false);

  // Очищаем сохраненные ключи, чтобы фильтры гарантированно не включались по умолчанию
  try {
    if (typeof localStorage !== 'undefined') {
      localStorage.removeItem('ducke_catalog_filter_controller');
      localStorage.removeItem('ducke_catalog_filter_collections');
    }
  } catch {}

  let isFilterHubOpen = $state<boolean>(false);
  let genreSearchQuery = $state<string>('');
  let tagSearchQuery = $state<string>('');

  const ratingOptions = [
    { id: 'all', label: 'Любая оценка' },
    { id: '95', label: 'Крайне положительные (95%+)' },
    { id: '80', label: 'Очень положительные (80%+)' },
    { id: '70', label: 'В основном положительные (70%+)' },
    { id: 'mixed', label: 'Смешанные (40–69%)' },
    { id: 'negative', label: 'Отрицательные (< 40%)' }
  ];

  const sizeOptions = [
    { id: 'all', label: 'Любой размер' },
    { id: 'under5gb', label: 'До 5 ГБ' },
    { id: '5to20gb', label: '5 — 20 ГБ' },
    { id: '20to50gb', label: '20 — 50 ГБ' },
    { id: 'over50gb', label: 'Более 50 ГБ' }
  ];

  const yearOptions = [
    { id: 'all', label: 'Любой год' },
    { id: '2024_plus', label: '2024 — 2025' },
    { id: '2020_2023', label: '2020 — 2023' },
    { id: '2015_2019', label: '2015 — 2019' },
    { id: 'before_2015', label: 'До 2015' }
  ];

  $effect(() => {
    try {
      if (typeof localStorage !== 'undefined') {
        localStorage.setItem(SAVED_SORT_KEY, selectedSort);
        localStorage.setItem(SAVED_GENRE_KEY, selectedGenre);
        localStorage.setItem(SAVED_TAGS_KEY, JSON.stringify(selectedTags));
        localStorage.setItem(SAVED_RATING_KEY, selectedRating);
        localStorage.setItem(SAVED_SIZE_KEY, selectedSize);
        localStorage.setItem(SAVED_YEAR_KEY, selectedYear);
      }
    } catch {}
  });

  let activeFilterCount = $derived.by(() => {
    let count = 0;
    if (selectedGenre !== 'all') count++;
    if (Array.isArray(selectedTags) && selectedTags.length > 0) count += selectedTags.length;
    if (selectedRating !== 'all') count++;
    if (selectedSize !== 'all') count++;
    if (selectedYear !== 'all') count++;
    if (filterController) count++;
    if (filterCollections) count++;
    return count;
  });

  function resetAllFilters() {
    selectedGenre = 'all';
    selectedTags = [];
    selectedRating = 'all';
    selectedSize = 'all';
    selectedYear = 'all';
    filterController = false;
    filterCollections = false;
    genreSearchQuery = '';
    tagSearchQuery = '';
  }

  function toggleTag(tag: string) {
    const list = Array.isArray(selectedTags) ? selectedTags : [];
    if (list.includes(tag)) {
      selectedTags = list.filter((t) => t !== tag);
    } else {
      selectedTags = [...list, tag];
    }
  }

  function getRatingLabel(val: string): string {
    const opt = ratingOptions.find((o) => o.id === val);
    return opt ? opt.label : val;
  }

  function getSizeLabel(val: string): string {
    const opt = sizeOptions.find((o) => o.id === val);
    return opt ? opt.label : val;
  }

  function getYearLabel(val: string): string {
    const opt = yearOptions.find((o) => o.id === val);
    return opt ? opt.label : val;
  }

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
    const rawGames = Array.isArray(games) ? games : [];
    const list = deduplicateGames(rawGames);
    return list.map((g) => {
      const gList = (Array.isArray(g.genres) ? g.genres : []).map((x: any) => typeof x === 'string' ? x.toLowerCase().trim() : '').filter(Boolean);
      const tList = (Array.isArray(g.tags) ? g.tags : []).map((x: any) => typeof x === 'string' ? x.toLowerCase().trim() : '').filter(Boolean);
      const pList = (Array.isArray(g.publishers) ? g.publishers : []).map((x: any) => typeof x === 'string' ? x.toLowerCase().trim() : '').filter(Boolean);
      const vList = (Array.isArray(g.variants) ? g.variants : []).map((v: any) => `${v?.rawName || ''} ${v?.torrentSource || ''}`.toLowerCase().trim()).filter(Boolean);
      const searchCorpus = `${g.cleanTitle || ''} ${g.steamTitle || ''} ${gList.join(' ')} ${tList.join(' ')} ${pList.join(' ')} ${vList.join(' ')}`.toLowerCase();

      const hasController = g.controllerSupport === 'full' || g.controllerSupport === 'partial';
      const isCollection = !!(g.isCollection || (g.parentPath && g.parentPath !== ''));
      
      let releaseYear: number | null = null;
      if (g.releaseDate) {
        const m = g.releaseDate.match(/\b(19\d\d|20\d\d)\b/);
        if (m) releaseYear = parseInt(m[1], 10);
      }

      return {
        ...g,
        _searchCorpus: searchCorpus,
        _genreLowerSet: new Set(gList),
        _tagLowerSet: new Set(tList),
        _hasController: hasController,
        _isCollection: isCollection,
        _releaseYear: releaseYear,
        _reviewPercent: g.reviewPercent || 0,
        _sizeBytes: g.sizeBytes || 0
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
    const q = genreSearchQuery.toLowerCase().trim();
    return availableGenres.filter((item) => item.name.toLowerCase().includes(q));
  });

  let availableTags = $derived.by(() => {
    const counts = new Map<string, number>();
    for (const g of deduplicatedList) {
      if (g.tags && Array.isArray(g.tags)) {
        for (const raw of g.tags) {
          const tag = raw.trim();
          if (tag) {
            counts.set(tag, (counts.get(tag) || 0) + 1);
          }
        }
      }
    }
    return Array.from(counts.entries())
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count);
  });

  let filteredTagList = $derived.by(() => {
    if (!tagSearchQuery.trim()) {
      return availableTags.slice(0, 30);
    }
    const q = tagSearchQuery.toLowerCase().trim();
    return availableTags.filter((item) => item.name.toLowerCase().includes(q)).slice(0, 50);
  });

  // Single-pass filter over deduplicated list
  let filteredGames = $derived.by(() => {
    const q = debouncedSearchQuery.trim().toLowerCase();
    const selGenre = selectedGenre && selectedGenre !== 'all' ? selectedGenre.toLowerCase() : null;
    const rawTags = Array.isArray(selectedTags) ? selectedTags : [];
    const selTags = rawTags.map((t) => (typeof t === 'string' ? t.toLowerCase() : '')).filter(Boolean);
    const selRating = selectedRating;
    const selSize = selectedSize;
    const selYear = selectedYear;
    const reqController = filterController;
    const reqCollections = filterCollections;

    let result = deduplicatedList.filter((g) => {
      // 1. Fast search match using precomputed corpus
      if (q && !g._searchCorpus.includes(q)) {
        return false;
      }

      // 2. Genre match using Set O(1)
      if (selGenre && !g._genreLowerSet.has(selGenre)) {
        return false;
      }

      // 3. Tags match using Set O(1)
      if (selTags.length > 0) {
        for (const t of selTags) {
          if (!g._tagLowerSet.has(t)) return false;
        }
      }

      // 4. Steam Rating match
      if (selRating === '95') {
        if (g._reviewPercent < 95 || (g.totalReviews || 0) < 10) return false;
      } else if (selRating === '80') {
        if (g._reviewPercent < 80 || (g.totalReviews || 0) < 5) return false;
      } else if (selRating === '70') {
        if (g._reviewPercent < 70) return false;
      } else if (selRating === 'mixed') {
        if (g._reviewPercent < 40 || g._reviewPercent >= 70) return false;
      } else if (selRating === 'negative') {
        if (g._reviewPercent >= 40 || (g.totalReviews || 0) === 0) return false;
      }

      // 5. Size Range match
      if (selSize === 'under5gb') {
        if (g._sizeBytes <= 0 || g._sizeBytes > 5 * 1024 * 1024 * 1024) return false;
      } else if (selSize === '5to20gb') {
        if (g._sizeBytes < 5 * 1024 * 1024 * 1024 || g._sizeBytes > 20 * 1024 * 1024 * 1024) return false;
      } else if (selSize === '20to50gb') {
        if (g._sizeBytes < 20 * 1024 * 1024 * 1024 || g._sizeBytes > 50 * 1024 * 1024 * 1024) return false;
      } else if (selSize === 'over50gb') {
        if (g._sizeBytes < 50 * 1024 * 1024 * 1024) return false;
      }

      // 6. Release Year match
      if (selYear === '2024_plus') {
        if (!g._releaseYear || g._releaseYear < 2024) return false;
      } else if (selYear === '2020_2023') {
        if (!g._releaseYear || g._releaseYear < 2020 || g._releaseYear > 2023) return false;
      } else if (selYear === '2015_2019') {
        if (!g._releaseYear || g._releaseYear < 2015 || g._releaseYear > 2019) return false;
      } else if (selYear === 'before_2015') {
        if (!g._releaseYear || g._releaseYear >= 2015) return false;
      }

      // 7. Features
      if (reqController && !g._hasController) return false;
      if (reqCollections && !g._isCollection) return false;

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

  // Reset scroll position when any filter, sort, or search changes
  $effect(() => {
    const _ = [
      debouncedSearchQuery,
      selectedGenre,
      selectedTags.length,
      selectedRating,
      selectedSize,
      selectedYear,
      filterController,
      filterCollections,
      selectedSort
    ];
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

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isFilterHubOpen) {
        isFilterHubOpen = false;
        e.stopPropagation();
      }
    };
    window.addEventListener('keydown', handleKeyDown);

    return () => {
      ro.disconnect();
      window.removeEventListener('keydown', handleKeyDown);
    };
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
        <MagnifyingGlass size={14} class="text-[#6b7280] absolute left-2.5 pointer-events-none" />
        <input
          data-nav-item
          data-nav-search
          type="text"
          bind:value={searchQuery}
          placeholder="Поиск в библиотеке..."
          class="w-full bg-[#07080a] text-[#ededed] placeholder-[#5a6170] text-xs rounded pl-8 pr-7 py-1.5 border border-white/[0.08] focus:border-white/20 focus:outline-none transition-colors"
        />
        {#if searchQuery}
          <button
            class="absolute right-2 text-[#6b7280] hover:text-white cursor-pointer"
            onclick={() => (searchQuery = '')}
          >
            <X size={14} />
          </button>
        {/if}
      </div>

      <!-- Filter & Sort Controls -->
      <div class="flex items-center gap-1.5 relative">
        <!-- Filter Button opening Offcanvas Drawer -->
        <button
          type="button"
          data-nav-item
          data-nav-filter
          class="flex-1 h-8 flex items-center justify-between px-2.5 rounded border text-[11px] font-medium transition-colors cursor-pointer truncate {activeFilterCount > 0 ? 'bg-white/10 border-white/20 text-white shadow-sm' : 'bg-[#07080a] hover:bg-white/[0.04] border-white/[0.08] text-[#9ca3af] hover:text-white'}"
          onclick={() => {
            isFilterHubOpen = true;
          }}
        >
          <div class="flex items-center gap-1.5 truncate">
            <Sliders size={12} class="flex-shrink-0 {activeFilterCount > 0 ? 'text-white' : 'text-[#6b7280]'}" />
            <span class="truncate">Фильтры</span>
            {#if activeFilterCount > 0}
              <span class="px-1.5 py-0.2 rounded bg-white/15 text-white font-mono text-[10px] font-semibold">
                {activeFilterCount}
              </span>
            {/if}
          </div>
          <CaretDown size={12} class="text-[#6b7280] flex-shrink-0 ml-1" />
        </button>

        <!-- Sort selector -->
        <select
          data-nav-item
          bind:value={selectedSort}
          class="flex-1 h-8 bg-[#07080a] text-[#9ca3af] hover:text-white text-[11px] font-medium px-2 rounded border border-white/[0.08] focus:outline-none focus:border-white/20 cursor-pointer truncate transition-colors"
        >
          <option value="date_desc">Новые</option>
          <option value="popular_desc">Популярные</option>
          <option value="rating_desc">Оценка Steam</option>
          <option value="name">А — Я</option>
          <option value="size_desc">Большие</option>
          <option value="size_asc">Лёгкие</option>
        </select>
      </div>

      <!-- Active Filter Chips Tray (Strictly NO horizontal scroll: wrapped flex layout) -->
      {#if activeFilterCount > 0}
        <div class="flex flex-wrap items-center gap-1.5 pt-0.5">
          {#if selectedGenre !== 'all'}
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-white/[0.06] border border-white/10 text-[10px] font-medium text-slate-200">
              <span class="truncate max-w-[120px]">{selectedGenre}</span>
              <button
                type="button"
                class="hover:text-rose-400 cursor-pointer ml-0.5"
                onclick={() => (selectedGenre = 'all')}
                title="Сбросить жанр"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          {#each (Array.isArray(selectedTags) ? selectedTags : []) as tag}
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-white/[0.06] border border-white/10 text-[10px] font-medium text-slate-200">
              <span class="truncate max-w-[120px]">{tag}</span>
              <button
                type="button"
                class="hover:text-rose-400 cursor-pointer ml-0.5"
                onclick={() => toggleTag(tag)}
                title="Сбросить тег"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/each}

          {#if selectedRating !== 'all'}
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-white/[0.06] border border-white/10 text-[10px] font-medium text-slate-200">
              <span>{getRatingLabel(selectedRating)}</span>
              <button
                type="button"
                class="hover:text-rose-400 cursor-pointer ml-0.5"
                onclick={() => (selectedRating = 'all')}
                title="Сбросить оценку"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          {#if selectedSize !== 'all'}
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-white/[0.06] border border-white/10 text-[10px] font-medium text-slate-200">
              <span>{getSizeLabel(selectedSize)}</span>
              <button
                type="button"
                class="hover:text-rose-400 cursor-pointer ml-0.5"
                onclick={() => (selectedSize = 'all')}
                title="Сбросить размер"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          {#if selectedYear !== 'all'}
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-white/[0.06] border border-white/10 text-[10px] font-medium text-slate-200">
              <span>{getYearLabel(selectedYear)}</span>
              <button
                type="button"
                class="hover:text-rose-400 cursor-pointer ml-0.5"
                onclick={() => (selectedYear = 'all')}
                title="Сбросить год"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          {#if filterController}
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-white/[0.06] border border-white/10 text-[10px] font-medium text-slate-200">
              <span>Геймпад</span>
              <button
                type="button"
                class="hover:text-rose-400 cursor-pointer ml-0.5"
                onclick={() => (filterController = false)}
                title="Сбросить фильтр"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          {#if filterCollections}
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-white/[0.06] border border-white/10 text-[10px] font-medium text-slate-200">
              <span>Сборники</span>
              <button
                type="button"
                class="hover:text-rose-400 cursor-pointer ml-0.5"
                onclick={() => (filterCollections = false)}
                title="Сбросить фильтр"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          <button
            type="button"
            class="text-[10px] text-[#8e95a2] hover:text-rose-400 underline transition-colors cursor-pointer ml-1"
            onclick={resetAllFilters}
          >
            Сбросить всё
          </button>
        </div>
      {/if}
    </div>

    <!-- Offcanvas Filter Panel (Exact same width as sidebar, slides over) -->
    {#if isFilterHubOpen}
      <div
        class="absolute inset-0 z-40 bg-[#07080a] flex flex-col border-r border-white/[0.08] animate-in fade-in slide-in-from-left-2 duration-150"
      >
        <!-- Offcanvas Header -->
        <div class="px-4 py-3 border-b border-white/[0.08] flex items-center justify-between bg-[#0b0e14] flex-shrink-0">
          <div class="flex items-center gap-2 min-w-0">
            <button
              type="button"
              onclick={() => (isFilterHubOpen = false)}
              class="flex items-center gap-1.5 px-2 py-1 -ml-1 rounded-sm text-xs text-[#8e95a2] hover:text-white hover:bg-white/[0.06] transition-colors cursor-pointer"
            >
              <ArrowLeft class="w-3.5 h-3.5" />
              <span>Назад</span>
            </button>
            <span class="text-white/20">/</span>
            <span class="text-xs font-semibold text-white truncate">Фильтры</span>
            {#if activeFilterCount > 0}
              <span class="px-1.5 py-0.2 rounded bg-white/10 text-white font-mono text-[10px]">
                {activeFilterCount}
              </span>
            {/if}
          </div>

          {#if activeFilterCount > 0}
            <button
              type="button"
              onclick={resetAllFilters}
              class="text-[11px] text-[#8e95a2] hover:text-rose-400 transition-colors cursor-pointer flex-shrink-0"
            >
              Сбросить
            </button>
          {/if}
        </div>

        <!-- Offcanvas Scrollable Content (No horizontal scroll, clean Steam aesthetic) -->
        <div class="flex-1 overflow-y-auto overflow-x-hidden p-4 space-y-5 text-xs select-none">
          
          <!-- 1. Оценка Steam -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <span class="text-[10px] font-bold uppercase tracking-wider text-[#64748b]">Оценка Steam</span>
              {#if selectedRating !== 'all'}
                <button
                  type="button"
                  onclick={() => (selectedRating = 'all')}
                  class="text-[10px] text-[#8e95a2] hover:text-white cursor-pointer"
                >
                  Сброс
                </button>
              {/if}
            </div>
            <div class="space-y-1">
              {#each ratingOptions as opt}
                <button
                  type="button"
                  onclick={() => (selectedRating = opt.id)}
                  class="w-full flex items-center justify-between px-3 py-1.5 rounded-sm text-left transition-colors cursor-pointer {selectedRating === opt.id ? 'bg-white/10 text-white font-medium border border-white/15' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.03] border border-transparent'}"
                >
                  <span>{opt.label}</span>
                  {#if selectedRating === opt.id}
                    <Check class="w-3.5 h-3.5 text-white" />
                  {/if}
                </button>
              {/each}
            </div>
          </div>

          <!-- 2. Размер на диске -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <span class="text-[10px] font-bold uppercase tracking-wider text-[#64748b]">Размер на диске</span>
              {#if selectedSize !== 'all'}
                <button
                  type="button"
                  onclick={() => (selectedSize = 'all')}
                  class="text-[10px] text-[#8e95a2] hover:text-white cursor-pointer"
                >
                  Сброс
                </button>
              {/if}
            </div>
            <div class="grid grid-cols-2 gap-1.5">
              {#each sizeOptions as opt}
                <button
                  type="button"
                  onclick={() => (selectedSize = opt.id)}
                  class="px-2.5 py-1.5 rounded-sm text-left transition-colors cursor-pointer truncate {selectedSize === opt.id ? 'bg-white/10 text-white font-medium border border-white/15' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.03] border border-white/[0.05]'}"
                >
                  <span class="truncate">{opt.label}</span>
                </button>
              {/each}
            </div>
          </div>

          <!-- 3. Дата выхода -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <span class="text-[10px] font-bold uppercase tracking-wider text-[#64748b]">Дата выхода</span>
              {#if selectedYear !== 'all'}
                <button
                  type="button"
                  onclick={() => (selectedYear = 'all')}
                  class="text-[10px] text-[#8e95a2] hover:text-white cursor-pointer"
                >
                  Сброс
                </button>
              {/if}
            </div>
            <div class="grid grid-cols-2 gap-1.5">
              {#each yearOptions as opt}
                <button
                  type="button"
                  onclick={() => (selectedYear = opt.id)}
                  class="px-2.5 py-1.5 rounded-sm text-left transition-colors cursor-pointer truncate {selectedYear === opt.id ? 'bg-white/10 text-white font-medium border border-white/15' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.03] border border-white/[0.05]'}"
                >
                  <span class="truncate">{opt.label}</span>
                </button>
              {/each}
            </div>
          </div>

          <!-- 4. Жанры -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <span class="text-[10px] font-bold uppercase tracking-wider text-[#64748b]">Жанры</span>
              {#if selectedGenre !== 'all'}
                <button
                  type="button"
                  onclick={() => (selectedGenre = 'all')}
                  class="text-[10px] text-[#8e95a2] hover:text-white cursor-pointer truncate max-w-[150px]"
                >
                  Сброс ({selectedGenre})
                </button>
              {/if}
            </div>

            <!-- Search genre -->
            <div class="relative mb-2">
              <Search class="w-3.5 h-3.5 text-[#6b7280] absolute left-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
              <input
                type="text"
                bind:value={genreSearchQuery}
                placeholder="Поиск жанра..."
                class="w-full bg-[#0d1117] text-white text-xs placeholder-[#5a6170] rounded pl-8 pr-7 py-1.5 border border-white/10 focus:border-white/20 focus:outline-none"
              />
              {#if genreSearchQuery}
                <button
                  type="button"
                  class="absolute right-2 top-1/2 -translate-y-1/2 text-[#6b7280] hover:text-white cursor-pointer"
                  onclick={() => (genreSearchQuery = '')}
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              {/if}
            </div>

            <!-- Vertical genre list -->
            <div class="max-h-[160px] overflow-y-auto space-y-0.5 pr-1 no-scrollbar">
              <button
                type="button"
                onclick={() => (selectedGenre = 'all')}
                class="w-full flex items-center justify-between px-2.5 py-1.5 rounded-sm text-left transition-colors cursor-pointer {selectedGenre === 'all' ? 'bg-white/10 text-white font-semibold' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.03]'}"
              >
                <span>Все жанры</span>
                <span class="text-[10px] font-mono text-[#64748b]">{deduplicatedList.length}</span>
              </button>
              {#each filteredGenreList as item}
                {@const isCurrent = selectedGenre.toLowerCase() === item.name.toLowerCase()}
                <button
                  type="button"
                  onclick={() => (selectedGenre = item.name)}
                  class="w-full flex items-center justify-between px-2.5 py-1.5 rounded-sm text-left transition-colors cursor-pointer {isCurrent ? 'bg-white/10 text-white font-medium border border-white/15' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.03]'}"
                >
                  <div class="flex items-center gap-2 truncate">
                    {#if isCurrent}
                      <Check class="w-3.5 h-3.5 text-white flex-shrink-0" />
                    {/if}
                    <span class="truncate">{item.name}</span>
                  </div>
                  <span class="text-[10px] font-mono text-[#64748b] ml-1 flex-shrink-0">{item.count}</span>
                </button>
              {/each}
            </div>
          </div>

          <!-- 5. Теги Steam -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <span class="text-[10px] font-bold uppercase tracking-wider text-[#64748b]">Теги Steam</span>
              {#if Array.isArray(selectedTags) && selectedTags.length > 0}
                <button
                  type="button"
                  onclick={() => (selectedTags = [])}
                  class="text-[10px] text-[#8e95a2] hover:text-white cursor-pointer"
                >
                  Сброс ({selectedTags.length})
                </button>
              {/if}
            </div>

            <!-- Search tag -->
            <div class="relative mb-2">
              <Search class="w-3.5 h-3.5 text-[#6b7280] absolute left-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
              <input
                type="text"
                bind:value={tagSearchQuery}
                placeholder="Поиск тега..."
                class="w-full bg-[#0d1117] text-white text-xs placeholder-[#5a6170] rounded pl-8 pr-7 py-1.5 border border-white/10 focus:border-white/20 focus:outline-none"
              />
              {#if tagSearchQuery}
                <button
                  type="button"
                  class="absolute right-2 top-1/2 -translate-y-1/2 text-[#6b7280] hover:text-white cursor-pointer"
                  onclick={() => (tagSearchQuery = '')}
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              {/if}
            </div>

            <!-- Vertical tags list -->
            <div class="max-h-[160px] overflow-y-auto space-y-0.5 pr-1 no-scrollbar">
              {#each filteredTagList as item}
                {@const isSelected = Array.isArray(selectedTags) && selectedTags.includes(item.name)}
                <button
                  type="button"
                  onclick={() => toggleTag(item.name)}
                  class="w-full flex items-center justify-between px-2.5 py-1.5 rounded-sm text-left transition-colors cursor-pointer {isSelected ? 'bg-white/10 text-white font-medium border border-white/15' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.03]'}"
                >
                  <div class="flex items-center gap-2 truncate">
                    <span class="w-3.5 h-3.5 rounded border flex items-center justify-center flex-shrink-0 {isSelected ? 'bg-white text-black border-white' : 'border-white/20 bg-transparent'}">
                      {#if isSelected}
                        <Check class="w-2.5 h-2.5" />
                      {/if}
                    </span>
                    <span class="truncate">{item.name}</span>
                  </div>
                  <span class="text-[10px] font-mono text-[#64748b] ml-1 flex-shrink-0">{item.count}</span>
                </button>
              {/each}
            </div>
          </div>

          <!-- 6. Особенности -->
          <div>
            <div class="text-[10px] font-bold uppercase tracking-wider text-[#64748b] mb-2">
              Особенности
            </div>
            <div class="space-y-1.5">
              <button
                type="button"
                onclick={() => (filterController = !filterController)}
                class="w-full flex items-center justify-between px-3 py-2 rounded-sm text-left transition-colors cursor-pointer {filterController ? 'bg-white/10 border border-white/15 text-white font-medium' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.03] border border-white/[0.04]'}"
              >
                <div class="flex items-center gap-2">
                  <Gamepad2 class="w-3.5 h-3.5 text-[#8e95a2]" />
                  <span>С поддержкой геймпада</span>
                </div>
                <span class="w-3.5 h-3.5 rounded border flex items-center justify-center flex-shrink-0 {filterController ? 'bg-white text-black border-white' : 'border-white/20 bg-transparent'}">
                  {#if filterController}
                    <Check class="w-2.5 h-2.5" />
                  {/if}
                </span>
              </button>

              <button
                type="button"
                onclick={() => (filterCollections = !filterCollections)}
                class="w-full flex items-center justify-between px-3 py-2 rounded-sm text-left transition-colors cursor-pointer {filterCollections ? 'bg-white/10 border border-white/15 text-white font-medium' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.03] border border-white/[0.04]'}"
              >
                <div class="flex items-center gap-2">
                  <FolderOpen class="w-3.5 h-3.5 text-[#8e95a2]" />
                  <span>Саги и сборники</span>
                </div>
                <span class="w-3.5 h-3.5 rounded border flex items-center justify-center flex-shrink-0 {filterCollections ? 'bg-white text-black border-white' : 'border-white/20 bg-transparent'}">
                  {#if filterCollections}
                    <Check class="w-2.5 h-2.5" />
                  {/if}
                </span>
              </button>
            </div>
          </div>

        </div>

        <!-- Offcanvas Sticky Footer -->
        <div class="p-3 border-t border-white/[0.08] bg-[#0b0e14] flex items-center gap-2 flex-shrink-0">
          <button
            type="button"
            onclick={() => (isFilterHubOpen = false)}
            class="w-full py-2 rounded bg-white/10 hover:bg-white/15 text-white text-xs font-semibold text-center transition-colors cursor-pointer border border-white/15"
          >
            Показать {filteredGames.length} игр
          </button>
        </div>
      </div>
    {/if}

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
            <div class="h-[58px] rounded sk-block p-2.5 px-3 flex flex-col justify-between" style="animation-delay: {i * 60}ms">
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
                class="group relative h-[58px] overflow-hidden rounded cursor-pointer transition-all duration-150 border {isSelected ? 'border-white/20 bg-white/[0.08] before:absolute before:left-0 before:top-2 before:bottom-2 before:w-[3px] before:rounded-r before:bg-white' : 'border-white/[0.04] bg-white/[0.02] hover:bg-white/[0.06] hover:border-white/10'}"
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
    bind:game={selectedGame}
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
