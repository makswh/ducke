<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Compass,
    Search,
    RefreshCw,
    ArrowLeft,
    Check,
    Download,
    ExternalLink,
    Clock,
    User,
    MessageSquare,
    Gamepad2,
    X,
    Filter,
    ChevronLeft,
    ChevronRight,
    Star,
    Info
  } from 'lucide-svelte';

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

  // Subtle fan tilt offsets for overlapping posters
  function getFanTransform(idx: number, total: number): string {
    if (total <= 1) return 'rotate(0deg)';
    if (total === 2) {
      return idx === 0 ? 'rotate(-6deg) translateX(-14px)' : 'rotate(6deg) translateX(14px)';
    }
    if (total === 3) {
      if (idx === 0) return 'rotate(-8deg) translateX(-26px) translateY(3px)';
      if (idx === 1) return 'rotate(0deg) translateY(-5px) scale(1.04)';
      return 'rotate(8deg) translateX(26px) translateY(3px)';
    }
    if (total === 4) {
      if (idx === 0) return 'rotate(-10deg) translateX(-34px) translateY(5px)';
      if (idx === 1) return 'rotate(-3deg) translateX(-11px) translateY(0px)';
      if (idx === 2) return 'rotate(3deg) translateX(11px) translateY(0px)';
      return 'rotate(10deg) translateX(34px) translateY(5px)';
    }
    const transforms = [
      'rotate(-11deg) translateX(-42px) translateY(6px)',
      'rotate(-5deg) translateX(-21px) translateY(1px)',
      'rotate(0deg) translateX(0px) translateY(-6px) scale(1.06)',
      'rotate(5deg) translateX(21px) translateY(1px)',
      'rotate(11deg) translateX(42px) translateY(6px)'
    ];
    return transforms[Math.min(idx, 4)];
  }

  function getFanZIndex(idx: number, total: number): number {
    if (total === 5) {
      return [1, 2, 5, 2, 1][idx] || 1;
    }
    if (total === 3) {
      return [1, 4, 1][idx] || 1;
    }
    if (total === 4) {
      return [1, 3, 3, 1][idx] || 1;
    }
    return idx + 1;
  }

  function getCompilationBackdrop(comp: StopGameCompilation): string | null {
    if (!comp.previewImages || comp.previewImages.length === 0) return null;
    let hash = 0;
    const str = String(comp.id || comp.title || '');
    for (let i = 0; i < str.length; i++) {
      hash = (hash * 31 + str.charCodeAt(i)) >>> 0;
    }
    const idx = hash % comp.previewImages.length;
    return comp.previewImages[idx] || comp.previewImages[0];
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
          class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[#0d1117] hover:bg-white/10 border border-white/10 text-xs font-medium text-slate-200 transition-colors cursor-pointer"
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
      <header class="px-6 py-4 border-b border-white/[0.06] bg-[#07080a] flex flex-col gap-3 flex-shrink-0">
        <div class="flex items-center justify-between gap-4 flex-wrap">
          <div class="flex items-center gap-3">
            <div class="w-8 h-8 rounded-lg bg-white/[0.05] border border-white/10 flex items-center justify-center text-slate-300">
              <Compass class="w-4 h-4" />
            </div>
            <div>
              <h1 class="text-base font-semibold text-white tracking-wide">Подборки игр</h1>
              <p class="text-[11px] text-[#8e95a2]">Тематические сборники StopGame с сопоставлением в библиотеке Ducke</p>
            </div>
          </div>

          <!-- Search compilations -->
          <div class="relative w-72">
            <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-[#8e95a2]" />
            <input
              type="text"
              placeholder="Поиск по подборкам и авторам..."
              bind:value={catalogSearch}
              class="w-full h-8 pl-8 pr-3 text-xs bg-[#0d1117] border border-white/10 rounded-lg text-white placeholder-[#8e95a2] focus:outline-none focus:border-white/25 transition-colors"
            />
            {#if catalogSearch}
              <button
                type="button"
                onclick={() => (catalogSearch = '')}
                class="absolute right-2.5 top-1/2 -translate-y-1/2 text-[#8e95a2] hover:text-white"
              >
                <X class="w-3 h-3" />
              </button>
            {/if}
          </div>
        </div>

        <!-- Sorter navigation tabs -->
        <div class="flex items-center gap-1.5 overflow-x-auto pt-1 no-scrollbar">
          {#each sortTabs as tab}
            <button
              type="button"
              onclick={() => loadCompilations(1, tab.id)}
              class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors whitespace-nowrap cursor-pointer {sortOption === tab.id ? 'bg-white/10 text-white border border-white/15' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04] border border-transparent'}"
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
              <div class="bg-[#0d1117] border border-white/[0.06] rounded-xl p-4 flex flex-col gap-3 animate-pulse">
                <div class="w-full h-28 bg-white/[0.04] rounded-lg"></div>
                <div class="h-4 bg-white/[0.06] rounded w-3/4"></div>
                <div class="h-3 bg-white/[0.04] rounded w-1/2"></div>
                <div class="h-10 bg-white/[0.02] rounded w-full mt-1"></div>
              </div>
            {/each}
          </div>
        {:else if catalogError}
          <div class="h-full flex flex-col items-center justify-center text-center p-6 gap-3">
            <div class="w-12 h-12 rounded-xl bg-red-500/10 border border-red-500/20 flex items-center justify-center text-red-400">
              <Compass class="w-6 h-6" />
            </div>
            <p class="text-sm text-slate-300 font-medium max-w-md">{catalogError}</p>
            <button
              type="button"
              onclick={() => loadCompilations(currentPage, sortOption)}
              class="mt-2 px-4 py-2 bg-white/10 hover:bg-white/15 text-white text-xs font-medium rounded-lg border border-white/10 flex items-center gap-2 transition-colors cursor-pointer"
            >
              <RefreshCw class="w-3.5 h-3.5" />
              Повторить попытку
            </button>
          </div>
        {:else if filteredCompilations.length === 0}
          <div class="h-full flex flex-col items-center justify-center text-center p-6 text-[#8e95a2]">
            <Compass class="w-10 h-10 stroke-[1.25] mb-2 opacity-50" />
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
              {@const backdrop = getCompilationBackdrop(comp)}
              <div
                role="button"
                tabindex="0"
                onclick={() => openCompilation(comp.id)}
                onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') openCompilation(comp.id); }}
                class="group relative overflow-hidden bg-[#0d1117] hover:bg-[#121620] border border-white/[0.06] hover:border-white/15 rounded-xl p-4 flex flex-col justify-between transition-all duration-200 cursor-pointer shadow-sm hover:shadow-md"
              >
                <!-- Ambient blurred collection cover backdrop -->
                {#if backdrop}
                  <div class="absolute inset-0 overflow-hidden pointer-events-none z-0 select-none">
                    <img
                      src={backdrop}
                      alt=""
                      aria-hidden="true"
                      loading="lazy"
                      decoding="async"
                      class="w-full h-full object-cover scale-125 blur-2xl brightness-[0.2] contrast-125 transition-transform duration-500 group-hover:scale-135 opacity-75"
                      onerror={(e) => { (e.currentTarget as HTMLElement).style.display = 'none'; }}
                    />
                    <div class="absolute inset-0 bg-gradient-to-t from-[#0d1117] via-[#0d1117]/85 to-[#0d1117]/50"></div>
                  </div>
                {/if}

                <div class="relative z-10">
                  <!-- Fan of tilted posters (Clean: no extra borders or background boxes) -->
                  <div class="relative w-full h-36 mb-3 flex items-center justify-center overflow-hidden">
                    {#if comp.previewImages && comp.previewImages.length > 0}
                      <div class="relative w-full h-full flex items-center justify-center">
                        {#each comp.previewImages.slice(0, 5) as imgUrl, idx}
                          <img
                            src={imgUrl}
                            alt=""
                            loading="lazy"
                            decoding="async"
                            class="absolute w-20 h-28 sm:w-22 sm:h-30 object-cover rounded-md shadow-lg shadow-black/80 transition-transform duration-300 ease-out pointer-events-none"
                            style="transform: {getFanTransform(idx, Math.min(comp.previewImages.length, 5))}; z-index: {getFanZIndex(idx, Math.min(comp.previewImages.length, 5))};"
                            onerror={(e) => { (e.currentTarget as HTMLElement).style.display = 'none'; }}
                          />
                        {/each}
                      </div>
                    {:else}
                      <div class="text-[#8e95a2] text-xs flex items-center gap-1.5 opacity-40">
                        <Gamepad2 class="w-5 h-5" />
                        <span>{comp.gamesCount} игр</span>
                      </div>
                    {/if}
                  </div>

                  <!-- Badges row -->
                  <div class="flex items-center gap-1.5 mb-2 flex-wrap text-[10px]">
                    <span class="px-2 py-0.5 rounded bg-white/[0.06] text-slate-300 font-mono flex items-center gap-1">
                      <Gamepad2 class="w-3 h-3 text-[#8e95a2]" />
                      {comp.gamesCount} игр
                    </span>
                    {#if comp.rating}
                      <span class="px-2 py-0.5 rounded bg-white/[0.06] text-slate-300 font-mono">
                        {comp.rating}
                      </span>
                    {/if}
                    {#if comp.commentsCount > 0}
                      <span class="px-2 py-0.5 rounded bg-white/[0.04] text-[#8e95a2] font-mono flex items-center gap-1">
                        <MessageSquare class="w-2.5 h-2.5" />
                        {comp.commentsCount}
                      </span>
                    {/if}
                  </div>

                  <!-- Title -->
                  <h2 class="text-sm font-semibold text-white group-hover:text-sky-300 transition-colors line-clamp-2 mb-1.5 leading-snug">
                    {comp.title}
                  </h2>

                  <!-- Description snippet -->
                  {#if comp.description}
                    <p class="text-[11px] text-[#8e95a2] line-clamp-2 leading-relaxed mb-3">
                      {comp.description}
                    </p>
                  {/if}
                </div>

                <!-- Footer: Author info -->
                <div class="relative z-10 pt-2.5 border-t border-white/[0.06] flex items-center justify-between text-[11px] text-[#8e95a2]">
                  <div class="flex items-center gap-1.5 truncate pr-2">
                    {#if comp.authorAvatar}
                      <img
                        src={comp.authorAvatar}
                        alt=""
                        class="w-4 h-4 rounded-full object-cover flex-shrink-0"
                        onerror={(e) => { (e.currentTarget as HTMLElement).style.display = 'none'; }}
                      />
                    {:else}
                      <User class="w-3.5 h-3.5 opacity-60 flex-shrink-0" />
                    {/if}
                    <span class="truncate">{comp.authorName || 'StopGame'}</span>
                  </div>
                  <span class="text-white/40 group-hover:text-white transition-colors text-xs font-mono">→</span>
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
                class="px-3 py-1.5 rounded-lg bg-[#0d1117] border border-white/10 text-[#8e95a2] hover:text-white hover:bg-white/[0.05] disabled:opacity-30 disabled:pointer-events-none flex items-center gap-1.5 transition-colors cursor-pointer"
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
                class="px-3 py-1.5 rounded-lg bg-[#0d1117] border border-white/10 text-[#8e95a2] hover:text-white hover:bg-white/[0.05] disabled:opacity-30 disabled:pointer-events-none flex items-center gap-1.5 transition-colors cursor-pointer"
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
          class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[#0d1117] hover:bg-white/10 border border-white/10 text-xs font-medium text-slate-200 transition-colors cursor-pointer"
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
            <button
              type="button"
              onclick={() => selectedCompId && openCompilation(selectedCompId, true)}
              title="Обновить подборку с сайта StopGame"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#0d1117] hover:bg-white/10 border border-white/10 text-xs font-medium text-[#8e95a2] hover:text-white transition-colors cursor-pointer"
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
              class="px-4 py-2 bg-white/10 hover:bg-white/15 text-white text-xs font-medium rounded-lg border border-white/10 transition-colors cursor-pointer"
            >
              Повторить
            </button>
          </div>
        {:else if detail}
          <!-- Attribution Disclaimer & Source Link -->
          <div class="flex flex-wrap items-center justify-between gap-3 px-3.5 py-2 rounded-lg bg-white/[0.02] border border-white/[0.05] text-[11px] text-[#8e95a2] mb-5">
            <div class="flex items-center gap-2 min-w-0">
              <Info class="w-3.5 h-3.5 text-[#64748b] flex-shrink-0" />
              <span class="leading-normal">
                Материалы получены из открытых общедоступных источников <strong class="text-slate-300 font-medium">StopGame.ru</strong> в некоммерческих ознакомительных целях, без извлечения выгоды.
              </span>
            </div>
            {#if detail.id}
              <button
                type="button"
                onclick={() => openExternalUrl(`https://stopgame.ru/games/compilation/${detail?.id}`)}
                class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-white/[0.04] hover:bg-white/[0.08] text-sky-400 hover:text-sky-300 font-medium transition-colors cursor-pointer flex-shrink-0"
                title="Перейти к оригинальной подборке на StopGame.ru"
              >
                <span>Оригинал на StopGame.ru</span>
                <ExternalLink class="w-3 h-3" />
              </button>
            {/if}
          </div>

          <!-- Compilation Header / Hero (No border, no background, larger text) -->
          <section class="mb-6 flex flex-col gap-3.5">
            <div class="flex items-start justify-between gap-6 flex-wrap">
              <div class="flex-1 min-w-[280px]">
                <h1 class="text-2xl sm:text-3xl font-bold text-white tracking-tight leading-tight mb-2.5">
                  {detail.title}
                </h1>
                <div class="flex items-center gap-3.5 flex-wrap text-xs text-[#8e95a2]">
                  {#if detail.authorName}
                    <div class="flex items-center gap-1.5">
                      {#if detail.authorAvatar}
                        <img src={detail.authorAvatar} alt="" class="w-4 h-4 rounded-full object-cover" />
                      {:else}
                        <User class="w-3.5 h-3.5" />
                      {/if}
                      <span class="text-slate-200">{detail.authorName}</span>
                    </div>
                  {/if}
                  {#if detail.rating}
                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-white/[0.06] text-slate-200 font-mono text-[11px]">
                      <Star class="w-3 h-3 fill-amber-400 text-amber-400" />
                      <span>Рейтинг: {detail.rating}</span>
                    </span>
                  {/if}
                  {#if detail.lastUpdated}
                    <span class="flex items-center gap-1 text-[11px]">
                      <Clock class="w-3 h-3 text-[#8e95a2]" />
                      {detail.lastUpdated}
                    </span>
                  {/if}
                </div>
              </div>

              <!-- Library Match Status Pill -->
              <div class="px-4 py-2.5 rounded-xl bg-white/[0.02] border border-white/[0.06] flex flex-col items-end text-right min-w-[170px]">
                <span class="text-[10px] uppercase font-bold tracking-wider text-[#8e95a2]">В библиотеке Ducke</span>
                <div class="text-sm font-mono font-bold text-white mt-0.5">
                  <span class="text-emerald-400">{inLibraryCount}</span>
                  <span class="text-[#8e95a2]"> / {detail.games.length}</span>
                  <span class="text-[11px] font-normal text-[#8e95a2] ml-1">({matchPercent}%)</span>
                </div>
                <!-- Slim progress bar -->
                <div class="w-full h-1 bg-white/10 rounded-full mt-2 overflow-hidden">
                  <div class="h-full bg-emerald-400 rounded-full transition-all duration-300" style="width: {matchPercent}%;"></div>
                </div>
              </div>
            </div>

            <!-- Description (no background, no border, larger typography) -->
            {#if detail.description}
              <p class="text-sm sm:text-[15px] text-slate-300 leading-relaxed font-normal whitespace-pre-line max-w-4xl select-text pt-1">
                {detail.description}
              </p>
            {/if}
          </section>

          <!-- Search & Filter Controls -->
          <div class="flex items-center justify-between gap-3 mb-4 flex-wrap">
            <div class="flex items-center gap-1.5 bg-[#0d1117] p-1 rounded-lg border border-white/[0.06]">
              <button
                type="button"
                onclick={() => (detailFilter = 'all')}
                class="px-3 py-1 rounded-md text-xs font-medium transition-colors cursor-pointer {detailFilter === 'all' ? 'bg-white/10 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white'}"
              >
                Все ({detail.games.length})
              </button>
              <button
                type="button"
                onclick={() => (detailFilter = 'in_library')}
                class="px-3 py-1 rounded-md text-xs font-medium transition-colors cursor-pointer flex items-center gap-1.5 {detailFilter === 'in_library' ? 'bg-white/10 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white'}"
              >
                <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                В библиотеке ({inLibraryCount})
              </button>
              <button
                type="button"
                onclick={() => (detailFilter = 'missing')}
                class="px-3 py-1 rounded-md text-xs font-medium transition-colors cursor-pointer {detailFilter === 'missing' ? 'bg-white/10 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white'}"
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
                class="w-full h-8 pl-8 pr-3 text-xs bg-[#0d1117] border border-white/10 rounded-lg text-white placeholder-[#8e95a2] focus:outline-none focus:border-white/25 transition-colors"
              />
              {#if detailSearch}
                <button
                  type="button"
                  onclick={() => (detailSearch = '')}
                  class="absolute right-2.5 top-1/2 -translate-y-1/2 text-[#8e95a2] hover:text-white"
                >
                  <X class="w-3 h-3" />
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
                  class="group relative bg-[#0d1117] hover:bg-[#131826] border border-white/[0.06] hover:border-white/20 rounded-xl overflow-hidden flex flex-col transition-all duration-150 cursor-pointer select-none text-left focus:outline-none focus:border-sky-500 active:scale-[0.99]"
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

                    <!-- Top controls / badges row -->
                    <div class="absolute top-2 left-2 right-2 flex items-center justify-between gap-1.5 pointer-events-none">
                      <!-- Left: StopGame Article Link (only if game has url) -->
                      {#if game.url}
                        <button
                          type="button"
                          onclick={(e) => {
                            e.stopPropagation();
                            const fullUrl = game.url.startsWith('http') ? game.url : `https://stopgame.ru${game.url}`;
                            openExternalUrl(fullUrl);
                          }}
                          class="pointer-events-auto p-1 rounded-md bg-black/75 hover:bg-black/95 backdrop-blur-sm border border-white/10 text-[#8e95a2] hover:text-white transition-all opacity-0 group-hover:opacity-100"
                          title="Открыть статью об игре на StopGame.ru"
                        >
                          <ExternalLink class="w-3 h-3" />
                        </button>
                      {:else}
                        <div></div>
                      {/if}

                      <!-- Right: StopGame score chip -->
                      {#if game.stopGameScore && game.stopGameScore !== '-'}
                        <div class="px-1.5 py-0.5 rounded-md bg-black/80 backdrop-blur-sm border border-white/10 text-white font-mono font-bold text-[10px] flex items-center gap-1 shadow">
                          <Star class="w-2.5 h-2.5 fill-amber-400 text-amber-400" />
                          <span>{game.stopGameScore}</span>
                        </div>
                      {/if}
                    </div>
                  </div>

                  <!-- Content info (Tactile Steam/Ducke style like MasterDetailCatalog) -->
                  <div class="p-2.5 px-3 flex-1 flex flex-col justify-between gap-1.5 min-h-[64px]">
                    <h3 class="text-xs sm:text-[13px] font-semibold leading-snug truncate text-[#d1d5db] group-hover:text-white" title={game.title}>
                      {game.title}
                    </h3>

                    <div class="flex items-center justify-between text-[10px] pt-0.5">
                      {#if game.inLibrary && game.duckeGame}
                        <span class="inline-flex items-center gap-1.5 text-emerald-400 font-medium truncate">
                          <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 flex-shrink-0"></span>
                          <span class="truncate">В библиотеке</span>
                        </span>
                        {#if game.duckeGame.sizeDisplay}
                          <span class="font-mono text-[10px] text-[#94a3b8] px-1.5 py-0.5 rounded bg-black/60 border border-white/[0.06] flex-shrink-0 ml-1">
                            {game.duckeGame.sizeDisplay}
                          </span>
                        {/if}
                      {:else}
                        <span class="inline-flex items-center gap-1 text-[#8e95a2] group-hover:text-sky-300 transition-colors truncate">
                          <Search class="w-3 h-3 flex-shrink-0" />
                          <span class="truncate">Искать в Ducke</span>
                        </span>
                        <span class="font-mono text-[9px] text-[#525a6c] uppercase tracking-wider flex-shrink-0 ml-1">
                          Каталог
                        </span>
                      {/if}
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        {/if}
      </main>
    </div>
  {/if}
</div>

