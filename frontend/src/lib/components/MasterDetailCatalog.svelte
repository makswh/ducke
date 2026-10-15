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
    ArrowLeft,
    ArrowsDownUp,
    ArrowDown
  } from 'phosphor-svelte';
  import type { GameEntity } from '../types/game';
  import { deduplicateGames } from '../utils/gameDeduplication';
  import { getDisplayTitle } from '../utils/titleUtils';
  import { getGameAddedTime, getGameUpdatedTime } from '../utils/dateUtils';
  import { downloadsStore } from '../stores/downloads.svelte';
  import GameDetailView from './GameDetailView.svelte';
  import GameListSidebar from './GameListSidebar.svelte';
  import {
    matchesSourceFilter,
    SOURCE_FILTER_OPTIONS,
    type SourceFilterType
  } from '../utils/sourceUtils';

  let {
    games = [] as GameEntity[],
    searchQuery = $bindable<string>(''),
    downloadPath = '',
    isLoading = false,
    loadingStatusText = '',
    onStartDownload = (gameId: number, targetPath: string) => {},
    onSelectFolder = async (): Promise<string> => '',
    onUpdateDownloadPath = (path: string) => {}
  } = $props();

  const SAVED_SORT_KEY = 'ducke_catalog_selected_sort';
  const SAVED_GENRE_KEY = 'ducke_catalog_selected_genre';
  const SAVED_TAGS_KEY = 'ducke_catalog_selected_tags';
  const SAVED_RATING_KEY = 'ducke_catalog_selected_rating';
  const SAVED_SIZE_KEY = 'ducke_catalog_selected_size';
  const SAVED_YEAR_KEY = 'ducke_catalog_selected_year';
  const SAVED_SOURCE_KEY = 'ducke_catalog_selected_source';

  function getStoredFilter(key: string, fallback: string): string {
    try {
      if (typeof localStorage !== 'undefined') {
        const v = localStorage.getItem(key);
        if (v) return v;
      }
    } catch {}
    return fallback;
  }

  type CatalogSortType = 'added_desc' | 'updated_desc' | 'date_desc' | 'name' | 'size_desc' | 'size_asc' | 'rating_desc' | 'popular_desc';
  let selectedSort = $state<CatalogSortType>(
    getStoredFilter(SAVED_SORT_KEY, 'added_desc') as any
  );
  let selectedSource = $state<SourceFilterType>(
    getStoredFilter(SAVED_SOURCE_KEY, 'all') as SourceFilterType
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
  let isSearchOpen = $state<boolean>(false);
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
        localStorage.setItem(SAVED_SOURCE_KEY, selectedSource);
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
    if (selectedSource !== 'all') count++;
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
    selectedSource = 'all';
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

  function getSourceLabel(val: string): string {
    const opt = SOURCE_FILTER_OPTIONS.find((o) => o.id === val);
    return opt ? opt.label : val;
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
    // If games are already deduplicated from backend (variants field is populated), skip redundant 20k DSU pass
    const isAlreadyDeduplicated = rawGames.length > 0 && Array.isArray(rawGames[0]?.variants);
    const list = isAlreadyDeduplicated ? rawGames : deduplicateGames(rawGames);
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
    const selSource = selectedSource;
    const selGenre = selectedGenre && selectedGenre !== 'all' ? selectedGenre.toLowerCase() : null;
    const rawTags = Array.isArray(selectedTags) ? selectedTags : [];
    const selTags = rawTags.map((t) => (typeof t === 'string' ? t.toLowerCase() : '')).filter(Boolean);
    const selRating = selectedRating;
    const selSize = selectedSize;
    const selYear = selectedYear;
    const reqController = filterController;
    const reqCollections = filterCollections;

    let result = deduplicatedList.filter((g) => {
      // 0. Source filter match
      if (!matchesSourceFilter(g, selSource)) {
        return false;
      }

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

    if (selectedSort === 'added_desc') {
      result.sort((a, b) => {
        const da = getGameAddedTime(a);
        const db = getGameAddedTime(b);
        if (da !== db) return db - da;
        return (b.id || 0) - (a.id || 0);
      });
    } else if (selectedSort === 'updated_desc') {
      result.sort((a, b) => {
        const da = getGameUpdatedTime(a);
        const db = getGameUpdatedTime(b);
        if (da !== db) return db - da;
        return (b.id || 0) - (a.id || 0);
      });
    } else if (selectedSort === 'size_desc') {
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
      // date_desc: sort by Steam release year descending; games without year go to the end
      result.sort((a, b) => {
        const ay = a._releaseYear;
        const by_ = b._releaseYear;
        if (ay !== null && by_ !== null) return by_ - ay;
        if (ay !== null) return -1;
        if (by_ !== null) return 1;
        return (b.id || 0) - (a.id || 0);
      });
    }

    return result;
  });

  let sidebarRef = $state<any>(null);

  function ensureGameVisible(gameId: number) {
    sidebarRef?.ensureGameVisible(gameId);
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


  let recentGame = $derived.by(() => {
    if (downloadsStore.currentDownload) {
      const curGame = deduplicatedList.find((g) => g.id === downloadsStore.currentDownload?.gameId);
      if (curGame) return curGame;
    }
    try {
      if (typeof localStorage !== 'undefined') {
        const lastLaunchedId = localStorage.getItem('ducke_last_launched_game_id');
        if (lastLaunchedId) {
          const launched = deduplicatedList.find((g) => g.id === Number(lastLaunchedId));
          if (launched) return launched;
        }
      }
    } catch {}
    const fav = deduplicatedList.find((g) => g.favoriteStatus === 'favorite');
    if (fav) return fav;
    return null;
  });

  onMount(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isFilterHubOpen) {
        isFilterHubOpen = false;
        e.stopPropagation();
      }
    };
    window.addEventListener('keydown', handleKeyDown);

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  });
</script>

<div class="flex-1 flex h-full overflow-hidden bg-gradient-to-b from-[#0F1218] to-[#0B0F1E]">
  <!-- 1. LEFT SIDEBAR: Reusable Dark Tactile Game List Sidebar -->
  <GameListSidebar
    bind:this={sidebarRef}
    title="Библиотека"
    games={filteredGames}
    bind:selectedGameId
    bind:searchQuery
    bind:selectedSort
    hasFilters={true}
    activeFilterCount={activeFilterCount}
    onOpenFilters={() => (isFilterHubOpen = true)}
    recentGame={recentGame}
    isLoading={isLoading}
    loadingStatusText={loadingStatusText}
  >
    {#snippet filterChipsSnippet()}
      <!-- Active Filter Chips Tray -->
      {#if activeFilterCount > 0}
        <div class="flex flex-wrap items-center gap-[6px] pt-0.5">
          {#if selectedGenre !== 'all'}
            <span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-[12px] bg-white/[0.05] border border-white/10 text-[11px] font-light text-white/70">
              <span class="truncate max-w-[120px]">{selectedGenre}</span>
              <button
                type="button"
                class="hover:text-white cursor-pointer ml-0.5"
                onclick={() => (selectedGenre = 'all')}
                title="Сбросить жанр"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          {#each (Array.isArray(selectedTags) ? selectedTags : []) as tag}
            <span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-[12px] bg-white/[0.05] border border-white/10 text-[11px] font-light text-white/70">
              <span class="truncate max-w-[120px]">{tag}</span>
              <button
                type="button"
                class="hover:text-white cursor-pointer ml-0.5"
                onclick={() => toggleTag(tag)}
                title="Сбросить тег"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/each}

          {#if selectedSource !== 'all'}
            <span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-[12px] bg-white/[0.05] border border-white/10 text-[11px] font-light text-white/70">
              <span>{getSourceLabel(selectedSource)}</span>
              <button
                type="button"
                class="hover:text-white cursor-pointer ml-0.5"
                onclick={() => (selectedSource = 'all')}
                title="Сбросить источник"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          {#if selectedRating !== 'all'}
            <span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-[12px] bg-white/[0.05] border border-white/10 text-[11px] font-light text-white/70">
              <span>{getRatingLabel(selectedRating)}</span>
              <button
                type="button"
                class="hover:text-white cursor-pointer ml-0.5"
                onclick={() => (selectedRating = 'all')}
                title="Сбросить оценку"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          {#if selectedSize !== 'all'}
            <span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-[12px] bg-white/[0.05] border border-white/10 text-[11px] font-light text-white/70">
              <span>{getSizeLabel(selectedSize)}</span>
              <button
                type="button"
                class="hover:text-white cursor-pointer ml-0.5"
                onclick={() => (selectedSize = 'all')}
                title="Сбросить размер"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          {#if selectedYear !== 'all'}
            <span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-[12px] bg-white/[0.05] border border-white/10 text-[11px] font-light text-white/70">
              <span>{getYearLabel(selectedYear)}</span>
              <button
                type="button"
                class="hover:text-white cursor-pointer ml-0.5"
                onclick={() => (selectedYear = 'all')}
                title="Сбросить год"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          {#if filterController}
            <span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-[12px] bg-white/[0.05] border border-white/10 text-[11px] font-light text-white/70">
              <span>Геймпад</span>
              <button
                type="button"
                class="hover:text-white cursor-pointer ml-0.5"
                onclick={() => (filterController = false)}
                title="Сбросить фильтр"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          {#if filterCollections}
            <span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-[12px] bg-white/[0.05] border border-white/10 text-[11px] font-light text-white/70">
              <span>Сборники</span>
              <button
                type="button"
                class="hover:text-white cursor-pointer ml-0.5"
                onclick={() => (filterCollections = false)}
                title="Сбросить фильтр"
              >
                <X class="w-3 h-3" />
              </button>
            </span>
          {/if}

          <button
            type="button"
            class="text-[10px] text-white/40 hover:text-white underline transition-colors cursor-pointer ml-1 font-light"
            onclick={resetAllFilters}
          >
            Сбросить всё
          </button>
        </div>
      {/if}
    {/snippet}

    {#snippet filterPanelSnippet()}
      <!-- Offcanvas Filter Panel (Exact same width as sidebar, slides over) -->
      {#if isFilterHubOpen}
        <div
          class="absolute inset-0 z-40 bg-gradient-to-b from-[#0F1218] to-[#0B0F1E] flex flex-col animate-in fade-in slide-in-from-left-2 duration-150"
        >
          <!-- Offcanvas Header -->
          <div class="px-4 py-3 flex items-center justify-between bg-transparent flex-shrink-0">
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
                <span class="px-1.5 py-0.5 rounded-[6px] bg-white/10 text-white font-mono text-[10px]">
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
            
            <!-- 0. Источник загрузки -->
            <div>
              <div class="flex items-center justify-between mb-2">
                <span class="text-[10px] font-bold uppercase tracking-wider text-[#64748b]">Источник</span>
                {#if selectedSource !== 'all'}
                  <button
                    type="button"
                    onclick={() => (selectedSource = 'all')}
                    class="text-[10px] text-[#8e95a2] hover:text-white cursor-pointer"
                  >
                    Сброс
                  </button>
                {/if}
              </div>
              <div class="space-y-1">
                {#each SOURCE_FILTER_OPTIONS as opt}
                  <button
                    type="button"
                    onclick={() => (selectedSource = opt.id)}
                    class="w-full flex items-center justify-between px-3 py-1.5 rounded-sm text-left transition-colors cursor-pointer {selectedSource === opt.id ? 'bg-white/10 text-white font-medium border border-white/15' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.03] border border-transparent'}"
                  >
                    <span>{opt.label}</span>
                    {#if selectedSource === opt.id}
                      <Check class="w-3.5 h-3.5 text-white" />
                    {/if}
                  </button>
                {/each}
              </div>
            </div>

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
          <div class="p-3 bg-transparent flex items-center gap-2 flex-shrink-0">
            <button
              type="button"
              onclick={() => (isFilterHubOpen = false)}
              class="w-full h-11 rounded-[16px] bg-gradient-to-b from-white/[0.16] to-white/[0.08] hover:from-white/[0.22] hover:to-white/[0.12] text-white text-xs font-normal text-center transition-colors cursor-pointer"
            >
              Показать {filteredGames.length} игр
            </button>
          </div>
        </div>
      {/if}
    {/snippet}
  </GameListSidebar>

  <!-- 2. MAIN DETAIL VIEW -->
  <GameDetailView
    bind:game={selectedGame}
    {isLoading}
    {loadingStatusText}
    {downloadPath}
    {onStartDownload}
    {onSelectFolder}
    {onUpdateDownloadPath}
    onSelectGenre={(g: string) => {
      selectedGenre = g;
    }}
    onSelectTag={(t: string) => {
      searchQuery = t;
    }}
  />
</div>

<style>
  :global(.no-scrollbar::-webkit-scrollbar) {
    display: none !important;
    width: 0 !important;
    height: 0 !important;
  }
  :global(.no-scrollbar) {
    -ms-overflow-style: none !important;
    scrollbar-width: none !important;
  }
</style>
