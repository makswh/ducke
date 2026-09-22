<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Compass,
    MagnifyingGlass,
    MagnifyingGlass as Search,
    ArrowClockwise,
    ArrowClockwise as RefreshCw,
    ArrowLeft,
    ArrowSquareOut,
    ArrowSquareOut as ExternalLink,
    Clock,
    GameController,
    GameController as Gamepad2,
    X,
    CaretLeft,
    CaretLeft as ChevronLeft,
    CaretRight,
    CaretRight as ChevronRight,
    Star
  } from 'phosphor-svelte';

  import type {
    CompilationSummary,
    CompilationsResponse,
    CompilationDetail,
    CompilationGame
  } from '../types/collections';
  import type { GameEntity } from '../types/game';
  import GameDetailView from './GameDetailView.svelte';
  import {
    GetStopGameCompilations,
    GetStopGameCompilationDetail
  } from '../../../wailsjs/go/main/App';
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';

  let {
    downloadPath = 'C:\\Ducke',
    onStartDownload = (gameId: number, targetPath: string) => {},
    onSelectFolder = () => Promise.resolve(''),
    onSearchInCatalog = (query: string) => {}
  } = $props();

  function openExternalUrl(url: string) {
    if (!url) return;
    try {
      BrowserOpenURL(url);
    } catch {
      window.open(url, '_blank');
    }
  }

  // State: Catalog
  let sortOption = $state<string>('best');
  let currentPage = $state<number>(1);
  let totalPages = $state<number>(1);
  let compilations = $state<CompilationSummary[]>([]);
  let isCatalogLoading = $state<boolean>(true);
  let catalogError = $state<string>('');
  let catalogSearch = $state<string>('');

  // State: Detail
  let selectedCompId = $state<string | null>(null);
  let detail = $state<CompilationDetail | null>(null);
  let isDetailLoading = $state<boolean>(false);
  let detailError = $state<string>('');
  let detailFilter = $state<'all' | 'in_library' | 'missing'>('all');
  let detailSearch = $state<string>('');
  let isDescriptionExpanded = $state<boolean>(false);

  // State: Full-page Game Detail (NO MODAL)
  let selectedGame = $state<GameEntity | null>(null);

  const sortTabs = [
    { id: 'best', label: 'Лучшие за всё время' },
    { id: 'month-top', label: 'Лучшие за месяц' },
    { id: 'week-top', label: 'Лучшие за неделю' },
    { id: 'default', label: 'Свежие' },
    { id: 'stopgame', label: 'От редакции' }
  ];

  async function loadCompilations(page = 1, sort = sortOption) {
    try {
      isCatalogLoading = true;
      catalogError = '';
      currentPage = page;
      sortOption = sort;

      const res: CompilationsResponse = await GetStopGameCompilations(sort, page);
      if (res && Array.isArray(res.items)) {
        compilations = res.items;
        totalPages = res.totalPages || 1;
      } else {
        compilations = [];
      }
    } catch (err: any) {
      console.error('Failed to load compilations:', err);
      catalogError = 'Не удалось загрузить подборки. Проверьте подключение к интернету.';
    } finally {
      isCatalogLoading = false;
    }
  }

  async function openCompilation(id: string, forceRefresh = false) {
    try {
      selectedCompId = id;
      selectedGame = null;
      isDetailLoading = true;
      detailError = '';
      detailFilter = 'all';
      detailSearch = '';
      isDescriptionExpanded = false;

      const res: CompilationDetail = await GetStopGameCompilationDetail(id, forceRefresh);
      if (res && res.id) {
        detail = res;
      } else {
        detailError = 'Не удалось получить данные подборки.';
      }
    } catch (err: any) {
      console.error('Failed to load compilation detail:', err);
      detailError = 'Ошибка загрузки подборки StopGame.';
    } finally {
      isDetailLoading = false;
    }
  }

  function backToCatalog() {
    selectedCompId = null;
    detail = null;
    detailError = '';
    selectedGame = null;
    isDescriptionExpanded = false;
  }

  onMount(() => {
    loadCompilations(1, 'best');

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (selectedGame) {
          selectedGame = null;
          e.stopPropagation();
        } else if (selectedCompId) {
          backToCatalog();
          e.stopPropagation();
        }
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  });

  // Filtered compilations in overview
  let filteredCompilations = $derived.by(() => {
    if (!catalogSearch.trim()) return compilations;
    const q = catalogSearch.toLowerCase().trim();
    return compilations.filter(
      (c) =>
        c.title.toLowerCase().includes(q) ||
        (c.authorName && c.authorName.toLowerCase().includes(q)) ||
        (c.description && c.description.toLowerCase().includes(q))
    );
  });

  // Filtered games in detailed view
  let filteredGames = $derived.by(() => {
    if (!detail || !Array.isArray(detail.games)) return [];
    let list = detail.games;

    if (detailFilter === 'in_library') {
      list = list.filter((g) => g.inLibrary);
    } else if (detailFilter === 'missing') {
      list = list.filter((g) => !g.inLibrary);
    }

    if (detailSearch.trim()) {
      const q = detailSearch.toLowerCase().trim();
      list = list.filter((g) => g.title.toLowerCase().includes(q));
    }

    return list;
  });

  let inLibraryCount = $derived.by(() => {
    if (!detail || !detail.games) return 0;
    return detail.games.filter((g) => g.inLibrary).length;
  });

  let missingCount = $derived.by(() => {
    if (!detail || !detail.games) return 0;
    return detail.games.filter((g) => !g.inLibrary).length;
  });

  let matchPercent = $derived.by(() => {
    if (!detail || !detail.games || detail.games.length === 0) return 0;
    return Math.round((inLibraryCount / detail.games.length) * 100);
  });

  function getBackgroundCovers(previewImages?: string[], targetCount = 16): string[] {
    if (!previewImages || previewImages.length === 0) return [];
    const valid = previewImages.filter(Boolean);
    if (valid.length === 0) return [];
    const result: string[] = [];
    while (result.length < targetCount) {
      for (const img of valid) {
        result.push(img);
        if (result.length >= targetCount) break;
      }
    }
    return result;
  }
</script>

<div class="flex-1 flex flex-col h-full bg-[#07080a] text-slate-200 select-none overflow-hidden">
  {#if selectedGame}
    <!-- ========================================== -->
    <!-- VIEW 3: NATIVE FULL GAME PAGE (NO MODAL)   -->
    <!-- ========================================== -->
    <div class="flex-1 flex flex-col h-full overflow-hidden">
      <!-- Breadcrumb Navigation Bar -->
      <header class="px-6 py-3 border-b border-white/[0.06] bg-[#07080a] flex items-center justify-between gap-4 flex-shrink-0">
        <button
          type="button"
          onclick={() => (selectedGame = null)}
          class="flex items-center gap-2 px-3 py-1.5 rounded bg-[#0d1117] hover:bg-white/10 border border-white/10 text-xs font-medium text-slate-200 transition-colors cursor-pointer"
        >
          <ArrowLeft class="w-3.5 h-3.5" />
          <span>Назад к подборке: <strong class="text-white font-semibold">{detail?.title || 'Подборка'}</strong></span>
        </button>

        <div class="flex items-center gap-2 text-xs text-[#8e95a2]">
          <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
          <span>В библиотеке Ducke</span>
        </div>
      </header>

      <!-- Full Native Game View -->
      <div class="flex-1 overflow-hidden">
        <GameDetailView
          game={selectedGame}
          {downloadPath}
          {onStartDownload}
          {onSelectFolder}
        />
      </div>
    </div>
  {:else if !selectedCompId}
    <!-- ========================================== -->
    <!-- VIEW 1: BROWSE COMPILATIONS CATALOG        -->
    <!-- ========================================== -->
    <div class="flex-1 flex flex-col h-full overflow-hidden">
      <!-- Sub-header bar with tabs and search -->
      <header class="px-6 py-3.5 border-b border-white/[0.06] bg-[#07080a] flex flex-col gap-2.5 flex-shrink-0">
        <div class="flex items-center justify-between gap-4 flex-wrap">
          <div class="flex items-center gap-2.5">
            <Compass size={16} weight="bold" class="text-sky-400" />
            <h1 class="text-xs font-bold uppercase tracking-wider text-[#cbd5e1]">Подборки</h1>
          </div>

          <!-- Search compilations -->
          <div class="relative w-64">
            <MagnifyingGlass size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-[#64748b]" />
            <input
              type="text"
              placeholder="Поиск по подборкам..."
              bind:value={catalogSearch}
              class="w-full h-8 pl-8 pr-3 text-xs bg-[#0d1117] border border-white/10 rounded text-white placeholder-[#64748b] focus:outline-none focus:border-white/25 transition-colors"
            />
            {#if catalogSearch}
              <button
                type="button"
                onclick={() => (catalogSearch = '')}
                class="absolute right-2.5 top-1/2 -translate-y-1/2 text-[#8e95a2] hover:text-white"
              >
                <X size={12} />
              </button>
            {/if}
          </div>
        </div>

        <!-- Sorter navigation tabs -->
        <div class="flex items-center gap-1.5 overflow-x-auto pt-0.5 no-scrollbar text-xs">
          {#each sortTabs as tab}
            <button
              type="button"
              onclick={() => loadCompilations(1, tab.id)}
              class="px-3 py-1.5 rounded text-xs font-medium transition-colors whitespace-nowrap cursor-pointer {sortOption === tab.id ? 'bg-white/10 text-white font-semibold border border-white/10 shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
            >
              {tab.label}
            </button>
          {/each}
        </div>
      </header>

      <!-- Main compilations list container -->
      <main class="flex-1 overflow-y-auto px-6 py-5">
        {#if isCatalogLoading}
          <!-- Loading skeleton grid -->
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            {#each Array(8) as _}
              <div class="bg-[#0d1117] border border-white/[0.06] rounded p-4 flex flex-col gap-3 animate-pulse">
                <div class="w-full h-28 bg-white/[0.04] rounded-sm"></div>
                <div class="h-4 bg-white/[0.06] rounded w-3/4"></div>
                <div class="h-3 bg-white/[0.04] rounded w-1/2"></div>
                <div class="h-10 bg-white/[0.02] rounded w-full mt-1"></div>
              </div>
            {/each}
          </div>
        {:else if catalogError}
          <div class="h-full flex flex-col items-center justify-center text-center p-6 gap-3">
            <div class="w-12 h-12 rounded bg-red-500/10 border border-red-500/20 flex items-center justify-center text-red-400">
              <Compass size={24} />
            </div>
            <p class="text-sm text-slate-300 font-medium max-w-md">{catalogError}</p>
            <button
              type="button"
              onclick={() => loadCompilations(currentPage, sortOption)}
              class="mt-2 px-4 py-2 bg-white/10 hover:bg-white/15 text-white text-xs font-medium rounded border border-white/10 flex items-center gap-2 transition-colors cursor-pointer"
            >
              <ArrowClockwise size={14} />
              Повторить попытку
            </button>
          </div>
        {:else if filteredCompilations.length === 0}
          <div class="h-full flex flex-col items-center justify-center text-center p-6 text-[#8e95a2]">
            <Compass size={40} class="mb-2 opacity-50" />
            <p class="text-sm">Подборки не найдены</p>
            {#if catalogSearch}
              <button
                type="button"
                onclick={() => (catalogSearch = '')}
                class="mt-3 text-xs text-sky-400 hover:underline"
              >
                Сбросить поисковый фильтр
              </button>
            {/if}
          </div>
        {:else}
          <!-- Grid of Compilations -->
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            {#each filteredCompilations as comp (comp.id)}
              <div
                role="button"
                tabindex="0"
                onclick={() => openCompilation(comp.id)}
                onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') openCompilation(comp.id); }}
                class="group relative h-48 sm:h-52 rounded overflow-hidden bg-[#0d1117] border border-white/[0.06] hover:border-white/20 transition-all duration-200 cursor-pointer shadow-sm hover:shadow-md flex flex-col justify-between p-4"
              >
                <!-- 45-degree diagonal background grid of covers -->
                <div class="absolute -inset-20 flex items-center justify-center pointer-events-none overflow-hidden select-none">
                  {#if comp.previewImages && comp.previewImages.length > 0}
                    <div
                      class="grid grid-cols-4 gap-2 w-[180%] h-[180%] rotate-[-45deg] opacity-20 group-hover:opacity-30 grayscale-[25%] transition-opacity duration-300"
                    >
                      {#each getBackgroundCovers(comp.previewImages) as imgUrl}
                        <div class="aspect-[2/3] rounded-sm overflow-hidden bg-black/40 border border-white/[0.04]">
                          <img
                            src={imgUrl}
                            alt=""
                            loading="lazy"
                            decoding="async"
                            class="w-full h-full object-cover pointer-events-none"
                            onerror={(e) => { (e.currentTarget as HTMLElement).style.display = 'none'; }}
                          />
                        </div>
                      {/each}
                    </div>
                  {/if}
                </div>

                <!-- Pure charcoal vignette matching Ducke palette (#0d1117 / #07080a) -->
                <div class="absolute inset-0 bg-gradient-to-t from-[#0d1117] via-[#0d1117]/85 to-[#0d1117]/55 pointer-events-none"></div>

                <!-- Card Content Layer (Pure collection info, NO personal data, NO rainbow badges) -->
                <div class="relative z-10 flex flex-col justify-between h-full">
                  <!-- Top Row: Clean Neutral Metadata -->
                  <div class="flex items-center justify-between text-xs text-[#8e95a2] font-mono">
                    <span>{comp.gamesCount} игр</span>
                    {#if comp.rating}
                      <span class="text-[#8e95a2]">★ {comp.rating}</span>
                    {/if}
                  </div>

                  <!-- Middle: Title & Description -->
                  <div class="space-y-1 my-auto">
                    <h2 class="text-sm font-semibold text-white group-hover:text-sky-300 transition-colors line-clamp-2 leading-snug">
                      {comp.title}
                    </h2>
                    {#if comp.description}
                      <p class="text-[11px] text-[#8e95a2] line-clamp-2 leading-relaxed">
                        {comp.description}
                      </p>
                    {/if}
                  </div>
                </div>
              </div>
            {/each}
          </div>

          <!-- Pagination Bar -->
          {#if totalPages > 1}
            <div class="flex items-center justify-center gap-3 pt-8 pb-4 text-xs">
              <button
                type="button"
                disabled={currentPage <= 1}
                onclick={() => loadCompilations(currentPage - 1, sortOption)}
                class="px-3 py-1.5 rounded bg-[#0d1117] border border-white/10 text-[#8e95a2] hover:text-white hover:bg-white/[0.05] disabled:opacity-30 disabled:pointer-events-none flex items-center gap-1.5 transition-colors cursor-pointer"
              >
                <ChevronLeft class="w-3.5 h-3.5" />
                Назад
              </button>

              <span class="text-[#8e95a2] font-mono text-[11px]">
                Страница <strong class="text-white">{currentPage}</strong> из <strong class="text-white">{totalPages}</strong>
              </span>

              <button
                type="button"
                disabled={currentPage >= totalPages}
                onclick={() => loadCompilations(currentPage + 1, sortOption)}
                class="px-3 py-1.5 rounded bg-[#0d1117] border border-white/10 text-[#8e95a2] hover:text-white hover:bg-white/[0.05] disabled:opacity-30 disabled:pointer-events-none flex items-center gap-1.5 transition-colors cursor-pointer"
              >
                Вперед
                <ChevronRight class="w-3.5 h-3.5" />
              </button>
            </div>
          {/if}
        {/if}
      </main>
    </div>
  {:else}
    <!-- ========================================== -->
    <!-- VIEW 2: COMPILATION DETAIL PAGE            -->
    <!-- ========================================== -->
    <div class="flex-1 flex flex-col h-full overflow-hidden">
      <!-- Top Action Navigation Bar -->
      <header class="px-6 py-3 border-b border-white/[0.06] bg-[#07080a] flex items-center justify-between gap-4 flex-shrink-0">
        <button
          type="button"
          onclick={backToCatalog}
          class="flex items-center gap-2 px-3 py-1.5 rounded bg-[#0d1117] hover:bg-white/10 border border-white/10 text-xs font-medium text-slate-200 transition-colors cursor-pointer"
        >
          <ArrowLeft class="w-3.5 h-3.5" />
          Все подборки
        </button>

        <div class="flex items-center gap-2">
          {#if isDetailLoading}
            <div class="flex items-center gap-1.5 text-xs text-sky-400 font-medium animate-pulse">
              <RefreshCw class="w-3.5 h-3.5 animate-spin" />
              Загрузка подборки...
            </div>
          {:else}
            {#if detail?.id}
              <button
                type="button"
                onclick={() => openExternalUrl(`https://stopgame.ru/games/compilation/${detail?.id}`)}
                class="flex items-center gap-1.5 px-3 py-1.5 rounded bg-[#0d1117] hover:bg-white/10 border border-white/10 text-xs font-medium text-[#8e95a2] hover:text-white transition-colors cursor-pointer"
                title="Перейти к оригинальной подборке на StopGame.ru"
              >
                <span>Оригинал на StopGame</span>
                <ExternalLink class="w-3.5 h-3.5" />
              </button>
            {/if}
            <button
              type="button"
              onclick={() => selectedCompId && openCompilation(selectedCompId, true)}
              title="Обновить подборку с сайта StopGame"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded bg-[#0d1117] hover:bg-white/10 border border-white/10 text-xs font-medium text-[#8e95a2] hover:text-white transition-colors cursor-pointer"
            >
              <RefreshCw class="w-3.5 h-3.5" />
              Обновить
            </button>
          {/if}
        </div>
      </header>

      <!-- Main Detail Scroll Area -->
      <main class="flex-1 overflow-y-auto px-6 py-5">
        {#if isDetailLoading && !detail}
          <div class="h-full flex items-center justify-center text-xs text-[#8e95a2] gap-2">
            <RefreshCw class="w-4 h-4 animate-spin text-sky-400" />
            <span>Загрузка данных подборки и сопоставление с библиотекой...</span>
          </div>
        {:else if detailError}
          <div class="h-full flex flex-col items-center justify-center text-center p-6 gap-3">
            <p class="text-sm text-red-400 font-medium">{detailError}</p>
            <button
              type="button"
              onclick={() => selectedCompId && openCompilation(selectedCompId, true)}
              class="px-4 py-2 bg-white/10 hover:bg-white/15 text-white text-xs font-medium rounded border border-white/10 transition-colors cursor-pointer"
            >
              Повторить
            </button>
          </div>
        {:else if detail}
          <!-- Compilation Header / Hero -->
          <section class="mb-5 flex flex-col gap-3">
            <div class="flex items-start justify-between gap-4 flex-wrap">
              <div class="flex-1 min-w-[280px]">
                <h1 class="text-2xl sm:text-3xl font-bold text-white tracking-tight leading-tight mb-2">
                  {detail.title}
                </h1>
                <div class="flex items-center gap-3 flex-wrap text-xs text-[#8e95a2]">
                  {#if detail.rating}
                    <span class="inline-flex items-center gap-1 text-slate-200 font-mono text-xs">
                      <Star class="w-3 h-3 fill-amber-400 text-amber-400" />
                      <span>{detail.rating}</span>
                    </span>
                  {/if}
                  {#if detail.lastUpdated}
                    <span class="text-white/20">•</span>
                    <span class="flex items-center gap-1 text-xs">
                      <Clock class="w-3 h-3 text-[#8e95a2]" />
                      {detail.lastUpdated}
                    </span>
                  {/if}
                </div>
              </div>

              <!-- Compact Library Match Status -->
              <div class="flex items-center gap-3 px-3.5 py-2 rounded bg-white/[0.03] border border-white/[0.08] text-xs self-start">
                <span class="text-[#8e95a2]">В библиотеке:</span>
                <span class="font-mono text-white">
                  <strong class="text-emerald-400 font-semibold">{inLibraryCount}</strong>
                  <span class="text-[#8e95a2]"> / {detail.games.length}</span>
                </span>
                <span class="text-[11px] font-mono text-emerald-400 bg-emerald-400/10 px-1.5 py-0.5 rounded">
                  {matchPercent}%
                </span>
              </div>
            </div>

            <!-- Description (collapsible to avoid wall of text pushing games down) -->
            {#if detail.description}
              <div class="max-w-4xl pt-1">
                <p class="text-xs sm:text-sm text-slate-300 leading-relaxed whitespace-pre-line select-text {isDescriptionExpanded ? '' : 'line-clamp-2'}">
                  {detail.description}
                </p>
                {#if detail.description.length > 140}
                  <button
                    type="button"
                    onclick={() => (isDescriptionExpanded = !isDescriptionExpanded)}
                    class="mt-1 text-xs text-sky-400 hover:text-sky-300 font-medium cursor-pointer transition-colors"
                  >
                    {isDescriptionExpanded ? 'Свернуть' : 'Развернуть описание...'}
                  </button>
                {/if}
              </div>
            {/if}
          </section>

          <!-- Search & Filter Controls -->
          <div class="flex items-center justify-between gap-3 mb-4 flex-wrap">
            <div class="flex items-center gap-1.5 bg-[#0d1117] p-1 rounded border border-white/[0.06]">
              <button
                type="button"
                onclick={() => (detailFilter = 'all')}
                class="px-3 py-1 rounded-sm text-xs font-medium transition-colors cursor-pointer {detailFilter === 'all' ? 'bg-white/10 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white'}"
              >
                Все ({detail.games.length})
              </button>
              <button
                type="button"
                onclick={() => (detailFilter = 'in_library')}
                class="px-3 py-1 rounded-sm text-xs font-medium transition-colors cursor-pointer flex items-center gap-1.5 {detailFilter === 'in_library' ? 'bg-white/10 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white'}"
              >
                <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                В библиотеке ({inLibraryCount})
              </button>
              <button
                type="button"
                onclick={() => (detailFilter = 'missing')}
                class="px-3 py-1 rounded-sm text-xs font-medium transition-colors cursor-pointer {detailFilter === 'missing' ? 'bg-white/10 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white'}"
              >
                Отсутствуют ({missingCount})
              </button>
            </div>

            <div class="relative w-64">
              <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-[#8e95a2]" />
              <input
                type="text"
                placeholder="Поиск по играм подборки..."
                bind:value={detailSearch}
                class="w-full h-8 pl-8 pr-3 text-xs bg-[#0d1117] border border-white/10 rounded text-white placeholder-[#8e95a2] focus:outline-none focus:border-white/25 transition-colors"
              />
              {#if detailSearch}
                <button
                  type="button"
                  onclick={() => (detailSearch = '')}
                  class="absolute right-2.5 top-1/2 -translate-y-1/2 text-[#8e95a2] hover:text-white"
                >
                  <X size={12} />
                </button>
              {/if}
            </div>
          </div>

          <!-- Games Cards Grid -->
          {#if filteredGames.length === 0}
            <div class="py-12 flex flex-col items-center justify-center text-center text-[#8e95a2] gap-1">
              <p class="text-xs">Игры не найдены по выбранному фильтру</p>
            </div>
          {:else}
            <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3.5">
              {#each filteredGames as game (game.stopGameId || game.title)}
                {@const posterSrc = game.duckeGame?.capsuleImage || game.posterUrl || game.duckeGame?.headerImage}
                <div
                  data-nav-item
                  role="button"
                  tabindex="0"
                  onclick={() => {
                    if (game.inLibrary && game.duckeGame) {
                      selectedGame = game.duckeGame;
                    } else {
                      onSearchInCatalog(game.title);
                    }
                  }}
                  onkeydown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      if (game.inLibrary && game.duckeGame) {
                        selectedGame = game.duckeGame;
                      } else {
                        onSearchInCatalog(game.title);
                      }
                    }
                  }}
                  class="group relative bg-[#0d1117] hover:bg-[#131826] border border-white/[0.06] hover:border-white/20 rounded overflow-hidden flex flex-col transition-all duration-150 cursor-pointer select-none text-left focus:outline-none focus:border-sky-500 active:scale-[0.99]"
                >
                  <!-- Poster image container -->
                  <div class="relative w-full aspect-[3/4] bg-[#07080a] overflow-hidden">
                    {#if posterSrc}
                      <img
                        src={posterSrc}
                        alt={game.title}
                        loading="lazy"
                        decoding="async"
                        class="w-full h-full object-cover transition-transform duration-200 group-hover:scale-105"
                        onerror={(e) => {
                          const target = e.currentTarget as HTMLImageElement;
                          target.src = 'https://images.stopgame.ru/games/logos/empty.jpg';
                        }}
                      />
                    {:else}
                      <div class="w-full h-full flex items-center justify-center text-[#8e95a2]">
                        <Gamepad2 class="w-8 h-8 opacity-30" />
                      </div>
                    {/if}

                    <!-- Gradient vignette at the bottom of the poster -->
                    <div class="absolute inset-0 bg-gradient-to-t from-[#0d1117] via-transparent to-transparent opacity-80 pointer-events-none"></div>

                    <!-- Top controls / badges (clean: visible on hover) -->
                    <div class="absolute top-2 left-2 right-2 flex items-center justify-between gap-1.5 pointer-events-none opacity-0 group-hover:opacity-100 transition-opacity duration-150">
                      <!-- Left: StopGame Article Link (only if game has url) -->
                      {#if game.url}
                        <button
                          type="button"
                          onclick={(e) => {
                            e.stopPropagation();
                            const fullUrl = game.url.startsWith('http') ? game.url : `https://stopgame.ru${game.url}`;
                            openExternalUrl(fullUrl);
                          }}
                          class="pointer-events-auto p-1 rounded-sm bg-[#07080a]/90 hover:bg-black border border-white/10 text-[#8e95a2] hover:text-white transition-all"
                          title="Открыть статью об игре на StopGame.ru"
                        >
                          <ExternalLink class="w-3 h-3" />
                        </button>
                      {:else}
                        <div></div>
                      {/if}

                      <!-- Right: StopGame score chip -->
                      {#if game.stopGameScore && game.stopGameScore !== '-'}
                        <div class="px-1.5 py-0.5 rounded-sm bg-[#07080a]/90 border border-white/10 text-slate-200 font-mono text-[10px] flex items-center gap-1 shadow">
                          <Star class="w-2.5 h-2.5 fill-amber-400 text-amber-400" />
                          <span>{game.stopGameScore}</span>
                        </div>
                      {/if}
                    </div>
                  </div>

                  <!-- Content info -->
                  <div class="p-2.5 px-3 flex-1 flex flex-col justify-between gap-1.5 min-h-[58px]">
                    <h3 class="text-xs sm:text-[13px] font-medium leading-snug truncate text-slate-200 group-hover:text-white" title={game.title}>
                      {game.title}
                    </h3>

                    <div class="flex items-center justify-between text-[10px] pt-0.5 text-[#8e95a2]">
                      {#if game.inLibrary && game.duckeGame}
                        <span class="inline-flex items-center gap-1.5 text-emerald-400 font-medium truncate">
                          <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 flex-shrink-0"></span>
                          <span class="truncate">В библиотеке</span>
                        </span>
                        {#if game.duckeGame.sizeDisplay}
                          <span class="font-mono text-[10px] text-[#8e95a2] flex-shrink-0 ml-1">
                            {game.duckeGame.sizeDisplay}
                          </span>
                        {/if}
                      {:else}
                        <span class="inline-flex items-center gap-1 text-[#8e95a2] group-hover:text-sky-300 transition-colors truncate">
                          <Search class="w-3 h-3 flex-shrink-0" />
                          <span class="truncate">Искать в Ducke</span>
                        </span>
                      {/if}
                    </div>
                  </div>
                </div>
              {/each}
            </div>

            <!-- Attribution footnote -->
            <div class="pt-8 pb-4 text-center text-[11px] text-[#8e95a2]/50">
              Материалы подборок получены из открытых источников StopGame.ru в ознакомительных целях
            </div>
          {/if}
        {/if}
      </main>
    </div>
  {/if}
</div>

