<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Flame,
    ArrowLeft,
    CaretLeft,
    CaretRight,
    Star,
    DownloadSimple as Download,
    BookmarkSimple as Bookmark,
    ArrowSquareOut,
    MagnifyingGlass,
    GameController,
    Clock,
    Tag,
    Trophy,
    Check,
    ArrowsClockwise,
    HardDrive,
    Folders
  } from 'phosphor-svelte';

  import type { GameEntity } from '../types/game';
  import type { CompilationSummary, CompilationDetail, CompilationGame } from '../types/collections';
  import GameDetailView from './GameDetailView.svelte';
  import { GetStopGameCompilations, GetStopGameCompilationDetail, GetGameDetails } from '../../../wailsjs/go/main/App';
  import { getDisplayTitle, cleanCompilationTitle } from '../utils/titleUtils';
  import {
    getHeroFeaturedGames,
    getTopRatedGames,
    getRecentGames,
    getRpgHighlights,
    getActionHighlights,
    getIndieHighlights,
    getStrategyHighlights,
    normalizeTitleKey
  } from '../utils/storefrontHelpers';

  let {
    games = [] as GameEntity[],
    isLoading = false,
    downloadPath = 'C:\\Ducke',
    onStartDownload = (gameId: number, targetPath: string) => {},
    onSelectFolder = () => Promise.resolve(''),
    onUpdateDownloadPath = (path: string) => {},
    onSearchInCatalog = (query: string) => {}
  }: {
    games?: GameEntity[];
    isLoading?: boolean;
    downloadPath?: string;
    onStartDownload?: (gameId: number, targetPath: string) => void;
    onSelectFolder?: () => Promise<string>;
    onUpdateDownloadPath?: (path: string) => void;
    onSearchInCatalog?: (query: string) => void;
  } = $props();

  // Navigation state within Storefront
  let selectedGame = $state<GameEntity | null>(null);

  // StopGame Compilations Data & Infinite Scroll
  let topCompilations = $state<CompilationDetail[]>([]); // 2-3 initial shelves before Steam shelves
  let bottomCompilations = $state<CompilationDetail[]>([]); // Infinite scroll shelves loaded below Top-10 tabs
  let stopGamePage = $state<number>(1);
  let totalStopGamePages = $state<number>(10);
  let isStopGameLoading = $state<boolean>(false);
  let isLoadingMore = $state<boolean>(false);
  let seenCompilationIds = new Set<string>();

  // Hero Carousel State
  let heroCompilation = $state<CompilationDetail | null>(null);
  let heroCompilationGames = $state<GameEntity[]>([]);

  let heroGames = $derived.by<GameEntity[]>(() => {
    if (heroCompilationGames.length >= 2) {
      return heroCompilationGames;
    }
    return getHeroFeaturedGames(games, 8);
  });
  let currentHeroIndex = $state<number>(0);
  let heroPreviewIndex = $state<number>(0); // 0 = default (1st screenshot), 1..4 = screenshot previews
  let heroTimer: any = null;
  let isHeroHovered = $state<boolean>(false);
  let heroScreensCache = $state<Record<number, string[]>>({});

  async function loadHeroScreenshots(gamesList: GameEntity[]) {
    for (const g of gamesList) {
      if (!g || !g.id || heroScreensCache[g.id]) continue;
      try {
        const details = await GetGameDetails(g.id);
        if (details && Array.isArray(details.screenshots) && details.screenshots.length > 0) {
          heroScreensCache[g.id] = details.screenshots;
        }
      } catch (err) {
        console.warn('Failed to load hero screenshots for', g.id, err);
      }
    }
  }

  $effect(() => {
    if (heroGames.length > 0) {
      loadHeroScreenshots(heroGames);
    }
  });

  // Ambient Dynamic Backdrop State (Random blurred screenshot of current hero game with smooth cross-fade)
  let bgLayerA = $state<string>('');
  let bgLayerB = $state<string>('');
  let activeLayer = $state<'A' | 'B'>('A');

  // Non-reactive tracking variables to guarantee zero reactivity loops
  let currentBackdropUrl = '';
  const heroChosenScreens = new Map<number, string>();

  function switchBackdrop(url: string) {
    if (!url || url === currentBackdropUrl) return;
    currentBackdropUrl = url;

    if (!bgLayerA && !bgLayerB) {
      bgLayerA = url;
      activeLayer = 'A';
    } else if (activeLayer === 'A') {
      bgLayerB = url;
      activeLayer = 'B';
    } else {
      bgLayerA = url;
      activeLayer = 'A';
    }
  }

  $effect(() => {
    if (heroGames.length === 0) return;
    const curIdx = currentHeroIndex;
    const curHero = heroGames[curIdx];
    if (!curHero) return;

    const cachedScreens = heroScreensCache[curHero.id];
    const screens = (cachedScreens && cachedScreens.length > 0)
      ? cachedScreens
      : (Array.isArray(curHero.screenshots) && curHero.screenshots.length > 0 ? curHero.screenshots : []);

    let chosenUrl = '';
    if (screens.length > 0) {
      let existing = heroChosenScreens.get(curHero.id);
      if (!existing || !screens.includes(existing)) {
        const randIdx = Math.floor(Math.random() * screens.length);
        existing = screens[randIdx];
        heroChosenScreens.set(curHero.id, existing);
      }
      chosenUrl = existing;
    } else if (curHero.backgroundImage) {
      chosenUrl = curHero.backgroundImage;
    } else if (curHero.headerImage) {
      chosenUrl = curHero.headerImage;
    }

    if (chosenUrl && chosenUrl !== currentBackdropUrl) {
      switchBackdrop(chosenUrl);
    }
  });

  // Cross-shelf deduplication tracker
  // We feed seen IDs and titles from Hero -> Top StopGame Shelves -> Top Rated -> Recent -> Tabs -> Bottom StopGame Shelves
  let deduplicatedData = $derived.by(() => {
    const seenIds = new Set<number>();
    const seenKeys = new Set<string>();

    // 1. Register Hero games
    for (const h of heroGames) {
      if (!h) continue;
      seenIds.add(h.id);
      const k = normalizeTitleKey(getDisplayTitle(h));
      if (k) seenKeys.add(k);
    }

    // Helper to filter compilation shelf games: ONLY games that exist in Ducke database, deduplicated
    function filterShelfGames(comp: CompilationDetail): CompilationGame[] {
      if (!comp || !Array.isArray(comp.games) || comp.games.length === 0) return [];
      const res: CompilationGame[] = [];
      for (const g of comp.games) {
        if (!g) continue;
        // 1. Requirement: Show ONLY games that are in our local database
        if (!g.inLibrary || !g.duckeGame) continue;

        const normKey = normalizeTitleKey(g.title);
        const dId = g.duckeGame.id;

        if (dId && seenIds.has(dId)) continue;
        if (normKey && seenKeys.has(normKey)) continue;

        if (dId) seenIds.add(dId);
        if (normKey) seenKeys.add(normKey);
        res.push(g);
      }
      return res;
    }

    // 2. Top StopGame Shelves
    const topShelves = topCompilations
      .map((c) => ({
        id: c.id,
        title: cleanCompilationTitle(c.title),
        games: filterShelfGames(c)
      }))
      .filter((s) => s.games.length > 0);

    // 3. Steam Top Rated (deduplicated against Hero & Top Shelves)
    const topRated = getTopRatedGames(games, 18, seenIds, seenKeys);

    // 4. Steam Recent (deduplicated against Hero, Top Shelves, Top Rated)
    const recent = getRecentGames(games, 18, seenIds, seenKeys);

    // 5. Tabbed Lists (deduplicated against all above)
    const rpg = getRpgHighlights(games, 12, seenIds, seenKeys);
    const indie = getIndieHighlights(games, 12, seenIds, seenKeys);

    // 6. Bottom StopGame Shelves (Infinite Scroll below Tabs, deduplicated against all above)
    const bottomShelves = bottomCompilations
      .map((c) => ({
        id: c.id,
        title: cleanCompilationTitle(c.title),
        games: filterShelfGames(c)
      }))
      .filter((s) => s.games.length > 0);

    return {
      topShelves,
      topRated,
      recent,
      rpg,
      indie,
      bottomShelves
    };
  });

  // Tabbed Lists state
  let activeTabList = $state<'topRated' | 'recent' | 'rpg' | 'indie'>('topRated');
  let currentTabListGames = $derived.by(() => {
    switch (activeTabList) {
      case 'topRated':
        return deduplicatedData.topRated.slice(0, 10);
      case 'recent':
        return deduplicatedData.recent.slice(0, 10);
      case 'rpg':
        return deduplicatedData.rpg.slice(0, 10);
      case 'indie':
        return deduplicatedData.indie.slice(0, 10);
      default:
        return deduplicatedData.topRated.slice(0, 10);
    }
  });

  // Hero Carousel Controls
  function nextHero() {
    if (heroGames.length <= 1) return;
    currentHeroIndex = (currentHeroIndex + 1) % heroGames.length;
    heroPreviewIndex = 0;
  }

  function prevHero() {
    if (heroGames.length <= 1) return;
    currentHeroIndex = (currentHeroIndex - 1 + heroGames.length) % heroGames.length;
    heroPreviewIndex = 0;
  }

  function startHeroTimer() {
    stopHeroTimer();
    heroTimer = setInterval(() => {
      if (!isHeroHovered) {
        nextHero();
      }
    }, 8000);
  }

  function stopHeroTimer() {
    if (heroTimer) {
      clearInterval(heroTimer);
      heroTimer = null;
    }
  }

  // Horizontal Shelf Scroll Helper
  function scrollShelf(shelfId: string, direction: 'left' | 'right') {
    const el = document.getElementById(shelfId);
    if (!el) return;
    const scrollAmount = direction === 'left' ? -el.clientWidth * 0.75 : el.clientWidth * 0.75;
    el.scrollBy({ left: scrollAmount, behavior: 'smooth' });
  }

  // Fisher-Yates shuffle helper
  function shuffleArray<T>(arr: T[]): T[] {
    const copy = [...arr];
    for (let i = copy.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [copy[i], copy[j]] = [copy[j], copy[i]];
    }
    return copy;
  }

  // Initial Load: Fetch first page and populate Hero compilation + top shelves + first batch of bottom shelves
  async function loadInitialCompilations() {
    try {
      isStopGameLoading = true;
      const res = await GetStopGameCompilations('best', 1);
      if (!res || !Array.isArray(res.items) || res.items.length === 0) return;
      if (res.totalPages) totalStopGamePages = res.totalPages;

      const shuffled = shuffleArray(res.items);
      // Fetch details for candidate compilations to pick a random one for Hero and top shelves
      const candidateItems = shuffled.slice(0, 8);
      const details = await Promise.all(
        candidateItems.map((item) => GetStopGameCompilationDetail(item.id, false).catch(() => null))
      );

      const validDetails = details.filter(
        (d): d is CompilationDetail => !!(d && Array.isArray(d.games) && d.games.length > 0)
      );

      // 1. Pick a random compilation for Hero with highest rating games in our library
      const eligibleHeroComps = validDetails
        .map((c) => {
          const libGames = (c.games || [])
            .filter((g) => g.inLibrary && g.duckeGame)
            .map((g) => g.duckeGame!);
          return { comp: c, libGames };
        })
        .filter((x) => x.libGames.length >= 2);

      let chosenHeroId: string | null = null;
      if (eligibleHeroComps.length > 0) {
        const chosen = eligibleHeroComps[Math.floor(Math.random() * eligibleHeroComps.length)];
        chosenHeroId = chosen.comp.id;
        seenCompilationIds.add(chosen.comp.id);
        heroCompilation = chosen.comp;

        // Sort by highest rating
        const sortedGames = chosen.libGames.slice().sort((a, b) => {
          const pctA = a.reviewPercent || 0;
          const pctB = b.reviewPercent || 0;
          const countA = a.totalReviews || 0;
          const countB = b.totalReviews || 0;
          const scoreA = pctA * Math.log10(Math.max(10, countA)) + (a.metacriticScore || 0);
          const scoreB = pctB * Math.log10(Math.max(10, countB)) + (b.metacriticScore || 0);
          if (scoreB !== scoreA) return scoreB - scoreA;
          if (pctB !== pctA) return pctB - pctA;
          return (b.id || 0) - (a.id || 0);
        });

        heroCompilationGames = sortedGames.slice(0, 8);
      }

      // 2. The remaining compilations populate top shelves and initial bottom shelves
      const remaining = validDetails.filter((d) => d.id !== chosenHeroId);
      const initialTop = remaining.slice(0, 2);
      const initialBottom = remaining.slice(2, 4);

      initialTop.forEach((x) => seenCompilationIds.add(x.id));
      initialBottom.forEach((x) => seenCompilationIds.add(x.id));

      topCompilations = initialTop;
      bottomCompilations = initialBottom;
      stopGamePage = 1;
    } catch (e: any) {
      console.warn('Failed to load initial StopGame compilations:', e);
    } finally {
      isStopGameLoading = false;
    }
  }

  // Infinite Scroll: Load more StopGame compilations when scrolling down
  async function loadMoreCompilations() {
    if (isLoadingMore) return;
    try {
      isLoadingMore = true;
      let targetPage = stopGamePage + 1;
      if (targetPage > totalStopGamePages) {
        // Wrap around to random page
        targetPage = Math.floor(Math.random() * Math.min(5, totalStopGamePages)) + 1;
      }

      const res = await GetStopGameCompilations('best', targetPage);
      if (!res || !Array.isArray(res.items) || res.items.length === 0) return;
      stopGamePage = targetPage;

      // Filter out compilations we have already loaded
      const unseen = res.items.filter((item) => !seenCompilationIds.has(item.id));
      const candidates = shuffleArray(unseen.length > 0 ? unseen : res.items).slice(0, 3);
      candidates.forEach((c) => seenCompilationIds.add(c.id));

      const details = await Promise.all(
        candidates.map((c) => GetStopGameCompilationDetail(c.id, false).catch(() => null))
      );

      const valid = details.filter((d): d is CompilationDetail => !!(d && Array.isArray(d.games) && d.games.length > 0));
      if (valid.length > 0) {
        bottomCompilations = [...bottomCompilations, ...valid];
      }
    } catch (e: any) {
      console.warn('Failed to load more StopGame compilations:', e);
    } finally {
      isLoadingMore = false;
    }
  }

  // Scroll Container Listener for Infinite Scroll
  function handleMainScroll(e: Event) {
    const target = e.currentTarget as HTMLElement;
    if (!target) return;
    const { scrollTop, scrollHeight, clientHeight } = target;
    // Trigger load when 400px before bottom
    if (scrollHeight - (scrollTop + clientHeight) < 400) {
      loadMoreCompilations();
    }
  }

  onMount(() => {
    loadInitialCompilations();
    startHeroTimer();

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (selectedGame) {
          selectedGame = null;
          e.stopPropagation();
        }
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      stopHeroTimer();
      window.removeEventListener('keydown', handleKeyDown);
    };
  });

  onDestroy(() => {
    stopHeroTimer();
  });
</script>

<div class="flex-1 flex flex-col h-full bg-gradient-to-b from-[#0F1218] to-[#0B0F1E] text-white select-none overflow-hidden font-['Manrope','Outfit',sans-serif]">
  {#if selectedGame}
    <!-- ======================================================== -->
    <!-- DRILLDOWN: FULL GAME DETAIL VIEW                         -->
    <!-- ======================================================== -->
    <div class="flex-1 flex flex-col h-full overflow-hidden">
      <!-- Breadcrumb Bar in Ducke style -->
      <header class="px-6 py-3 bg-[#0F1218] flex items-center justify-between gap-4 flex-shrink-0 z-20">
        <button
          type="button"
          onclick={() => (selectedGame = null)}
          class="flex items-center gap-2 px-3.5 py-1.5 rounded-[12px] bg-gradient-to-b from-white/[0.04] to-white/[0.02] hover:bg-white/[0.06] text-xs font-light text-white/80 hover:text-white transition-all cursor-pointer"
        >
          <ArrowLeft size={14} weight="regular" />
          <span>Назад к Витрине</span>
        </button>

        <div class="flex items-center gap-2 text-xs text-white/50 font-light">
          <span class="w-1.5 h-1.5 rounded-full bg-[#4C7DFF]"></span>
          <span class="truncate max-w-[320px] text-white/80">{getDisplayTitle(selectedGame)}</span>
        </div>
      </header>

      <!-- GameDetailView Body -->
      <div class="flex-1 overflow-hidden">
        <GameDetailView
          game={selectedGame}
          {downloadPath}
          {onStartDownload}
          {onSelectFolder}
          {onUpdateDownloadPath}
        />
      </div>
    </div>
  {:else}
    <!-- ======================================================== -->
    <!-- MAIN STOREFRONT VIEW (INFINITE SCROLLING SHELVES)        -->
    <!-- ======================================================== -->
    <div
      onscroll={handleMainScroll}
      class="flex-1 overflow-y-auto custom-scrollbar relative"
    >
      <!-- ======================================================== -->
      <!-- AMBIENT TOP BACKDROP (BLURRED SCREENSHOT EXTENDING DOWN) -->
      <!-- ======================================================== -->
      <div
        class="absolute top-0 left-0 right-0 h-[1400px] sm:h-[1600px] lg:h-[1800px] pointer-events-none overflow-hidden select-none z-0"
        style="mask-image: linear-gradient(to bottom, rgba(0,0,0,1) 0%, rgba(0,0,0,0.9) 45%, rgba(0,0,0,0.4) 80%, rgba(0,0,0,0) 100%); -webkit-mask-image: linear-gradient(to bottom, rgba(0,0,0,1) 0%, rgba(0,0,0,0.9) 45%, rgba(0,0,0,0.4) 80%, rgba(0,0,0,0) 100%);"
      >
        {#if bgLayerA}
          <img
            src={bgLayerA}
            alt=""
            class="absolute inset-0 w-full h-full object-cover scale-130 filter blur-[70px] sm:blur-[100px] lg:blur-[120px] brightness-[0.70] sm:brightness-[0.75] saturate-[1.35] transition-opacity duration-1000 ease-in-out {activeLayer === 'A' ? 'opacity-100' : 'opacity-0'}"
          />
        {/if}
        {#if bgLayerB}
          <img
            src={bgLayerB}
            alt=""
            class="absolute inset-0 w-full h-full object-cover scale-130 filter blur-[70px] sm:blur-[100px] lg:blur-[120px] brightness-[0.70] sm:brightness-[0.75] saturate-[1.35] transition-opacity duration-1000 ease-in-out {activeLayer === 'B' ? 'opacity-100' : 'opacity-0'}"
          />
        {/if}
        <!-- Downward progressive gradient into deep page background -->
        <div class="absolute inset-0 bg-gradient-to-b from-black/15 via-[#0F1218]/35 via-45% via-[#0F1218]/80 via-80% to-[#0F1218]"></div>
        <!-- Subtle horizontal edge vignettes -->
        <div class="absolute inset-0 bg-gradient-to-r from-[#0F1218]/60 via-transparent to-[#0F1218]/60"></div>
      </div>

      <div class="max-w-[1440px] mx-auto p-4 sm:p-6 lg:p-8 space-y-8 relative z-10">

        {#if (isLoading || isStopGameLoading) && games.length === 0}
          {@render storefrontSkeleton()}
        {:else if games.length === 0}
          <div class="p-16 text-center text-xs text-white/40 space-y-3">
            <GameController size={32} weight="regular" class="mx-auto text-white/30" />
            <p class="font-light text-sm text-white/60">Каталог игр пуст</p>
            <p class="text-xs text-white/40 max-w-sm mx-auto">Подключите источник торрентов или FTP сервер в настройках, чтобы наполнить витрину играми.</p>
          </div>
        {:else}
          <!-- ====================================================== -->
          <!-- 1. HERO FEATURED & RECOMMENDED CAROUSEL (STEAM HERO)   -->
          <!-- ====================================================== -->
          {#if heroGames.length > 0}
            {@const hero = heroGames[currentHeroIndex]}
          {@const cachedScreens = heroScreensCache[hero.id]}
          {@const rawScreens = (cachedScreens && cachedScreens.length > 0)
            ? cachedScreens
            : (Array.isArray(hero.screenshots) && hero.screenshots.length > 0 ? hero.screenshots : [])}
          {@const heroScreens = rawScreens.slice(0, 4)}
          {@const activeHeroImage = (heroPreviewIndex > 0 && heroScreens[heroPreviewIndex - 1])
            ? heroScreens[heroPreviewIndex - 1]
            : (heroScreens[0] || hero.backgroundImage || hero.headerImage)}

          <section
            aria-label="Главное и рекомендуемое"
            class="relative space-y-3"
            onmouseenter={() => (isHeroHovered = true)}
            onmouseleave={() => {
              isHeroHovered = false;
              heroPreviewIndex = 0;
            }}
          >
            <!-- Hero Header Title Row -->
            <div class="flex items-center justify-between px-1">
              <div class="flex items-center gap-2 min-w-0">
                <span class="text-[12px] font-light text-white/[0.50] uppercase tracking-wider shrink-0">Главное и рекомендуемое</span>
                {#if heroCompilation}
                  <span class="text-[11px] text-white/30 hidden sm:inline">•</span>
                  <span class="text-[11px] font-normal text-white/80 bg-white/[0.05] border border-white/10 px-2.5 py-0.5 rounded-full truncate max-w-[260px] sm:max-w-[440px]">
                    {cleanCompilationTitle(heroCompilation.title)}
                  </span>
                {/if}
              </div>
              <!-- Carousel Indicators -->
              <div class="flex items-center gap-1.5">
                {#each heroGames as _, idx}
                  <button
                    type="button"
                    aria-label="Слайд {idx + 1}"
                    onclick={() => {
                      currentHeroIndex = idx;
                      heroPreviewIndex = 0;
                    }}
                    class="h-[3px] rounded-full transition-all cursor-pointer {currentHeroIndex === idx ? 'w-5 bg-white' : 'w-2 bg-white/20 hover:bg-white/40'}"
                  ></button>
                {/each}
              </div>
            </div>

            <!-- Hero Bento Container: 2-Column Steam Layout in Ducke Bento Styling -->
            <div class="grid grid-cols-12 gap-3 sm:gap-4 lg:gap-5 items-stretch">
              
              <!-- Left Column (~67%): Large Screen Viewport with Navigation -->
              <div
                class="col-span-12 lg:col-span-8 relative min-h-[260px] sm:min-h-[340px] lg:min-h-[420px] rounded-[24px] overflow-hidden bg-black/60 cursor-pointer group flex items-center justify-center select-none shadow-lg"
                onclick={() => (selectedGame = hero)}
                role="button"
                tabindex="0"
                onkeydown={(e) => e.key === 'Enter' && (selectedGame = hero)}
              >
                {#if activeHeroImage}
                  <img
                    src={activeHeroImage}
                    alt={getDisplayTitle(hero)}
                    style="filter: saturate(0.9) brightness(0.95);"
                    class="w-full h-full object-cover object-center transition-transform duration-300 group-hover:scale-[1.01]"
                  />
                {/if}

                <!-- Dark vignette gradient -->
                <div class="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent pointer-events-none"></div>

                <!-- Navigation Arrow: Prev -->
                <button
                  type="button"
                  aria-label="Предыдущая игра"
                  onclick={(e) => {
                    e.stopPropagation();
                    prevHero();
                  }}
                  class="absolute left-3 top-1/2 -translate-y-1/2 p-2.5 rounded-full bg-black/70 hover:bg-black/90 text-white opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer active:scale-95 z-20"
                >
                  <CaretLeft size={20} weight="bold" />
                </button>

                <!-- Navigation Arrow: Next -->
                <button
                  type="button"
                  aria-label="Следующая игра"
                  onclick={(e) => {
                    e.stopPropagation();
                    nextHero();
                  }}
                  class="absolute right-3 top-1/2 -translate-y-1/2 p-2.5 rounded-full bg-black/70 hover:bg-black/90 text-white opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer active:scale-95 z-20"
                >
                  <CaretRight size={20} weight="bold" />
                </button>

                <!-- Bottom Label overlay -->
                <div class="absolute bottom-4 left-4 right-4 flex items-end justify-between gap-3 pointer-events-none z-10">
                  <div class="min-w-0">
                    <h2 class="text-xl sm:text-2xl lg:text-3xl font-normal text-white tracking-tight drop-shadow truncate">
                      {getDisplayTitle(hero)}
                    </h2>
                    {#if hero.genres && hero.genres.length > 0}
                      <div class="flex items-center gap-2 mt-1 text-[12px] font-light text-white/70 drop-shadow">
                        <span>{hero.genres.slice(0, 3).join(' • ')}</span>
                        {#if hero.releaseDate}
                          <span>•</span>
                          <span>{hero.releaseDate}</span>
                        {/if}
                      </div>
                    {/if}
                  </div>
                </div>
              </div>

              <!-- Right Column (~33%): Capsule, Specs, 4 Screenshots Grid & Action Deck -->
              <div class="col-span-12 lg:col-span-4 flex flex-col justify-between p-4 sm:p-5 rounded-[24px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] gap-3">
                <div class="space-y-3">
                  <!-- Header Capsule / Landscape Art (Steam 460x215) -->
                  {#if hero.headerImage || hero.capsuleImage}
                    <div class="w-full aspect-[460/215] rounded-[18px] overflow-hidden bg-black/40 flex-shrink-0">
                      <img
                        src={hero.headerImage || hero.capsuleImage}
                        alt={getDisplayTitle(hero)}
                        style="filter: saturate(0.85) brightness(0.92);"
                        class="w-full h-full object-cover"
                      />
                    </div>
                  {/if}

                  <!-- Short Synopsis / Description -->
                  {#if hero.shortDescription}
                    <p class="text-xs text-white/[0.70] leading-relaxed font-light line-clamp-3">
                      {hero.shortDescription}
                    </p>
                  {/if}

                  <!-- 4 Mini Screenshots Grid (Steam Hover Switch) -->
                  <div class="space-y-1.5 pt-1">
                    <div class="flex items-center justify-between">
                      <span class="text-[11px] font-light text-white/[0.42] uppercase tracking-wider">Скриншоты:</span>
                      {#if heroScreens.length > 0}
                        <span class="text-[10px] font-mono text-white/[0.30]">{heroPreviewIndex === 0 ? 1 : heroPreviewIndex} / {heroScreens.length}</span>
                      {/if}
                    </div>
                    <div class="grid grid-cols-2 gap-2">
                      {#if heroScreens.length > 0}
                        {#each heroScreens as sUrl, sIdx}
                          <button
                            type="button"
                            onmouseenter={() => (heroPreviewIndex = sIdx + 1)}
                            class="relative aspect-[16/9] rounded-[12px] overflow-hidden bg-white/[0.02] transition-all cursor-pointer {heroPreviewIndex === sIdx + 1 || (heroPreviewIndex === 0 && sIdx === 0) ? 'ring-2 ring-white/80 opacity-100 shadow-md' : 'opacity-60 hover:opacity-100'}"
                          >
                            <img src={sUrl} alt="Screen {sIdx + 1}" class="w-full h-full object-cover" />
                          </button>
                        {/each}
                      {:else}
                        {#each [1, 2, 3, 4] as _}
                          <div class="aspect-[16/9] rounded-[12px] bg-white/[0.03] animate-pulse"></div>
                        {/each}
                      {/if}
                    </div>
                  </div>

                  <!-- Specs Table in Ducke format -->
                  <div class="divide-y divide-white/[0.04] text-xs pt-1">
                    {#if hero.reviewScoreDesc || hero.reviewPercent}
                      <div class="flex items-center justify-between py-1.5">
                        <span class="text-[11px] font-light text-white/[0.50] uppercase tracking-wider">Обзоры:</span>
                        <span class="font-normal {(hero.reviewPercent || 0) >= 70 ? 'text-[#66c0f4]' : 'text-amber-400'}">
                          {hero.reviewScoreDesc || `${hero.reviewPercent}% положительных`}
                          {#if hero.totalReviews}
                            <span class="text-[11px] font-light text-white/[0.42] ml-1">({hero.totalReviews.toLocaleString()})</span>
                          {/if}
                        </span>
                      </div>
                    {/if}

                    {#if hero.sizeDisplay}
                      <div class="flex items-center justify-between py-1.5">
                        <span class="text-[11px] font-light text-white/[0.50] uppercase tracking-wider">Размер:</span>
                        <span class="font-mono text-white/80 font-light">{hero.sizeDisplay}</span>
                      </div>
                    {/if}
                  </div>
                </div>

                <!-- Action Button in Ducke Style -->
                <div class="pt-2 flex items-center justify-between gap-3">
                  <button
                    type="button"
                    onclick={() => (selectedGame = hero)}
                    class="flex-1 h-11 px-6 rounded-[14px] bg-white text-black font-semibold text-xs uppercase tracking-wider hover:bg-white/90 active:scale-[0.98] transition-all cursor-pointer flex items-center justify-center gap-2"
                  >
                    <span>Подробнее об игре</span>
                  </button>

                  <button
                    type="button"
                    onclick={() => onStartDownload(hero.id, downloadPath)}
                    class="h-11 px-4 rounded-[14px] bg-white/[0.04] hover:bg-white/[0.08] text-white transition-all cursor-pointer flex items-center justify-center"
                    title="Скачать"
                  >
                    <Download size={16} weight="regular" />
                  </button>
                </div>
              </div>

            </div>
          </section>
        {/if}

        <!-- ====================================================== -->
        <!-- 2. TOP STOPGAME COMPILATIONS (BEFORE STEAM SHELVES)    -->
        <!-- ====================================================== -->
        {#each deduplicatedData.topShelves as compShelf (compShelf.id)}
          {@render stopGameShelf(compShelf)}
        {/each}

        <!-- ====================================================== -->
        <!-- 3. TOP RATED GAMES SHELF (DEDUPLICATED)                -->
        <!-- ====================================================== -->
        {#if deduplicatedData.topRated.length > 0}
          <section aria-label="Игры с высоким рейтингом" class="space-y-3">
            <div class="flex items-center justify-between px-1">
              <div class="flex items-center gap-2">
                <span class="text-[12px] font-light text-white/[0.50] uppercase tracking-wider">Высокий рейтинг в Steam</span>
                <span class="text-[12px] font-light text-white/[0.30]">• Оценки сообщества от 80%</span>
              </div>
              <div class="flex items-center gap-1.5">
                <button
                  type="button"
                  aria-label="Прокрутить назад"
                  onclick={() => scrollShelf('shelf-toprated', 'left')}
                  class="w-7 h-7 rounded-[10px] bg-white/[0.03] hover:bg-white/[0.08] text-white/70 hover:text-white flex items-center justify-center cursor-pointer transition-colors"
                >
                  <CaretLeft size={16} weight="regular" />
                </button>
                <button
                  type="button"
                  aria-label="Прокрутить вперед"
                  onclick={() => scrollShelf('shelf-toprated', 'right')}
                  class="w-7 h-7 rounded-[10px] bg-white/[0.03] hover:bg-white/[0.08] text-white/70 hover:text-white flex items-center justify-center cursor-pointer transition-colors"
                >
                  <CaretRight size={16} weight="regular" />
                </button>
              </div>
            </div>

            <!-- Horizontal Immersive Poster Cards -->
            <div
              id="shelf-toprated"
              class="flex gap-4 overflow-x-auto no-scrollbar scroll-smooth pb-2 pt-1"
            >
              {#each deduplicatedData.topRated as g (g.id)}
                {@render gamePosterCard({
                  game: g,
                  title: getDisplayTitle(g),
                  topCategory: g.genres && g.genres.length > 0 ? g.genres[0] : 'Steam',
                  year: g.releaseDate?.match(/\b(19\d\d|20\d\d)\b/)?.[1],
                  size: g.sizeDisplay,
                  score: g.reviewPercent ? `${g.reviewPercent}%` : undefined,
                  image: g.capsuleImage || g.headerImage || g.backgroundImage,
                  inLibrary: true,
                  onClick: () => (selectedGame = g)
                })}
              {/each}
            </div>
          </section>
        {/if}

        <!-- ====================================================== -->
        <!-- 4. RECENTLY ADDED & UPDATED SHELF (DEDUPLICATED)       -->
        <!-- ====================================================== -->
        {#if deduplicatedData.recent.length > 0}
          <section aria-label="Новинки и недавние добавления" class="space-y-3">
            <div class="flex items-center justify-between px-1">
              <div class="flex items-center gap-2">
                <span class="text-[12px] font-light text-white/[0.50] uppercase tracking-wider">Новинки каталога</span>
                <span class="text-[12px] font-light text-white/[0.30]">• Недавно добавленные релизы</span>
              </div>
              <div class="flex items-center gap-1.5">
                <button
                  type="button"
                  aria-label="Прокрутить назад"
                  onclick={() => scrollShelf('shelf-recent', 'left')}
                  class="w-7 h-7 rounded-[10px] bg-white/[0.03] hover:bg-white/[0.08] text-white/70 hover:text-white flex items-center justify-center cursor-pointer transition-colors"
                >
                  <CaretLeft size={16} weight="regular" />
                </button>
                <button
                  type="button"
                  aria-label="Прокрутить вперед"
                  onclick={() => scrollShelf('shelf-recent', 'right')}
                  class="w-7 h-7 rounded-[10px] bg-white/[0.03] hover:bg-white/[0.08] text-white/70 hover:text-white flex items-center justify-center cursor-pointer transition-colors"
                >
                  <CaretRight size={16} weight="regular" />
                </button>
              </div>
            </div>

            <!-- Horizontal Immersive Poster Cards -->
            <div
              id="shelf-recent"
              class="flex gap-4 overflow-x-auto no-scrollbar scroll-smooth pb-2 pt-1"
            >
              {#each deduplicatedData.recent as g (g.id)}
                {@render gamePosterCard({
                  game: g,
                  title: getDisplayTitle(g),
                  topCategory: g.genres && g.genres.length > 0 ? g.genres[0] : 'Релиз',
                  year: g.releaseDate?.match(/\b(19\d\d|20\d\d)\b/)?.[1] || g.uploadDate,
                  size: g.sizeDisplay,
                  image: g.capsuleImage || g.headerImage || g.backgroundImage,
                  inLibrary: true,
                  onClick: () => (selectedGame = g)
                })}
              {/each}
            </div>
          </section>
        {/if}

        <!-- ====================================================== -->
        <!-- 5. STEAM TABBED TOP-10 LIST WIDGET                     -->
        <!-- ====================================================== -->
        <section aria-label="Популярные категории" class="space-y-3 pt-2">
          <!-- Ducke-styled Tab Headers (Pill chips like in Sidebar) -->
          <div class="flex items-center gap-1.5 overflow-x-auto no-scrollbar py-0.5 px-1">
            <button
              type="button"
              onclick={() => (activeTabList = 'topRated')}
              class="px-3 py-1.5 rounded-[12px] text-[11px] font-medium transition-all cursor-pointer whitespace-nowrap {activeTabList === 'topRated' ? 'bg-white/[0.12] text-white' : 'bg-white/[0.025] text-white/50 hover:text-white hover:bg-white/[0.05]'}"
            >
              Лидеры рейтинга
            </button>
            <button
              type="button"
              onclick={() => (activeTabList = 'recent')}
              class="px-3 py-1.5 rounded-[12px] text-[11px] font-medium transition-all cursor-pointer whitespace-nowrap {activeTabList === 'recent' ? 'bg-white/[0.12] text-white' : 'bg-white/[0.025] text-white/50 hover:text-white hover:bg-white/[0.05]'}"
            >
              Новинки
            </button>
            <button
              type="button"
              onclick={() => (activeTabList = 'rpg')}
              class="px-3 py-1.5 rounded-[12px] text-[11px] font-medium transition-all cursor-pointer whitespace-nowrap {activeTabList === 'rpg' ? 'bg-white/[0.12] text-white' : 'bg-white/[0.025] text-white/50 hover:text-white hover:bg-white/[0.05]'}"
            >
              Ролевые игры (RPG)
            </button>
            <button
              type="button"
              onclick={() => (activeTabList = 'indie')}
              class="px-3 py-1.5 rounded-[12px] text-[11px] font-medium transition-all cursor-pointer whitespace-nowrap {activeTabList === 'indie' ? 'bg-white/[0.12] text-white' : 'bg-white/[0.025] text-white/50 hover:text-white hover:bg-white/[0.05]'}"
            >
              Инди-хиты
            </button>
          </div>

          <!-- Compact Rows List in Ducke Card Style (h-64px rows) -->
          <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
            {#each currentTabListGames as g (g.id)}
              {@const thumb = g.capsuleImage || g.headerImage || g.iconUrl}
              <div
                role="button"
                tabindex="0"
                onclick={() => (selectedGame = g)}
                onkeydown={(e) => e.key === 'Enter' && (selectedGame = g)}
                class="group relative h-[64px] min-h-[64px] rounded-[18px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] hover:bg-white/[0.05] hover:-translate-y-[1px] px-3 py-2 flex items-center justify-between gap-3 transition-all duration-[160ms] cursor-pointer select-none"
              >
                <div class="flex items-center gap-3 min-w-0">
                  <div class="w-[48px] h-[48px] rounded-[14px] overflow-hidden bg-white/[0.02] flex-shrink-0 flex items-center justify-center">
                    {#if thumb}
                      <img
                        src={thumb}
                        alt={getDisplayTitle(g)}
                        style="filter: saturate(0.8) brightness(0.88);"
                        class="w-full h-full object-cover"
                      />
                    {:else}
                      <div class="w-full h-full flex items-center justify-center text-white/30">
                        <GameController size={18} weight="regular" />
                      </div>
                    {/if}
                  </div>

                  <div class="min-w-0 flex flex-col justify-center gap-[2px]">
                    {#if g.genres && g.genres.length > 0}
                      <div class="text-[11px] font-light text-white/[0.42] leading-tight truncate">
                        {g.genres.slice(0, 2).join(' • ')}
                      </div>
                    {/if}
                    <h4 class="text-[13px] font-normal text-white/[0.92] truncate group-hover:text-white">
                      {getDisplayTitle(g)}
                    </h4>
                  </div>
                </div>

                <div class="w-[84px] min-w-[84px] flex-shrink-0 flex items-center justify-end text-right gap-2">
                  {#if g.reviewPercent}
                    <span class="text-[11px] font-mono text-[#66c0f4]">
                      {g.reviewPercent}%
                    </span>
                  {/if}
                  <span class="text-[11px] font-mono text-white/[0.42]">
                    {g.sizeDisplay || ''}
                  </span>
                </div>
              </div>
            {/each}
          </div>
        </section>

        <!-- ====================================================== -->
        <!-- 6. INFINITE SCROLL: FURTHER STOPGAME SHELVES BELOW     -->
        <!-- ====================================================== -->
        {#each deduplicatedData.bottomShelves as compShelf (compShelf.id)}
          {@render stopGameShelf(compShelf)}
        {/each}

        <!-- Bottom Loading Indicator when fetching more shelves -->
        {#if isLoadingMore}
          <div class="flex items-center justify-center gap-2 py-6 text-xs text-white/50 font-light">
            <ArrowsClockwise size={16} class="animate-spin text-[#4C7DFF]" />
            <span>Загрузка следующих подборок...</span>
          </div>
        {/if}

        {/if}
      </div>
    </div>
  {/if}
</div>

<!-- ===================================================================== -->
<!-- SNIPPET 1: STOPGAME SHELF ROW                                         -->
<!-- ===================================================================== -->
{#snippet stopGameShelf(compShelf: { id: string; title: string; games: CompilationGame[] })}
  <section aria-label={cleanCompilationTitle(compShelf.title)} class="space-y-3">
    <div class="flex items-center justify-between px-1">
      <div class="flex items-center gap-2 min-w-0">
        <span class="text-[12px] font-light text-white/[0.50] uppercase tracking-wider truncate">
          {cleanCompilationTitle(compShelf.title)}
        </span>
      </div>
      <div class="flex items-center gap-1.5 flex-shrink-0">
        <button
          type="button"
          aria-label="Прокрутить назад"
          onclick={() => scrollShelf(`shelf-sg-${compShelf.id}`, 'left')}
          class="w-7 h-7 rounded-[10px] bg-white/[0.03] hover:bg-white/[0.08] text-white/70 hover:text-white flex items-center justify-center cursor-pointer transition-colors"
        >
          <CaretLeft size={16} weight="regular" />
        </button>
        <button
          type="button"
          aria-label="Прокрутить вперед"
          onclick={() => scrollShelf(`shelf-sg-${compShelf.id}`, 'right')}
          class="w-7 h-7 rounded-[10px] bg-white/[0.03] hover:bg-white/[0.08] text-white/70 hover:text-white flex items-center justify-center cursor-pointer transition-colors"
        >
          <CaretRight size={16} weight="regular" />
        </button>
      </div>
    </div>

    <!-- Horizontal Immersive Poster Cards in Reference Design -->
    <div
      id={`shelf-sg-${compShelf.id}`}
      class="flex gap-4 overflow-x-auto no-scrollbar scroll-smooth pb-2 pt-1"
    >
      {#each compShelf.games as g (g.stopGameId || g.title)}
        {@const posterSrc = g.duckeGame?.capsuleImage || g.posterUrl || g.duckeGame?.headerImage}
        {@render gamePosterCard({
          game: g.duckeGame,
          title: g.title,
          topCategory: g.duckeGame?.genres && g.duckeGame.genres.length > 0 ? g.duckeGame.genres[0] : 'Ducke',
          year: g.year,
          size: g.duckeGame?.sizeDisplay,
          image: posterSrc,
          onClick: () => {
            if (g.duckeGame) {
              selectedGame = g.duckeGame;
            }
          }
        })}
      {/each}
    </div>
  </section>
{/snippet}

<!-- ===================================================================== -->
<!-- SNIPPET 2: EDITORIAL IMMERSIVE POSTER CARD (REFERENCE DESIGN)         -->
<!-- ===================================================================== -->
{#snippet gamePosterCard(props: {
  game?: GameEntity | null;
  title: string;
  topCategory?: string;
  year?: string | number;
  size?: string;
  score?: string;
  image?: string;
  onClick: () => void;
})}
  <div
    role="button"
    tabindex="0"
    onclick={props.onClick}
    onkeydown={(e) => e.key === 'Enter' && props.onClick()}
    class="group relative flex-shrink-0 w-[190px] md:w-[220px] aspect-[1/1.42] rounded-[24px] overflow-hidden cursor-pointer select-none transition-all duration-[240ms] ease-out hover:scale-[1.02] hover:-translate-y-1 bg-black/40 shadow-xl border border-white/[0.06] hover:border-white/20 text-left"
  >
    <!-- Full-bleed background poster image -->
    {#if props.image}
      <img
        src={props.image}
        alt={props.title}
        loading="lazy"
        decoding="async"
        class="absolute inset-0 w-full h-full object-cover transition-transform duration-500 ease-out group-hover:scale-105 brightness-[0.88] contrast-[1.05]"
        onerror={(e) => {
          const target = e.currentTarget as HTMLImageElement;
          target.src = 'https://images.stopgame.ru/games/logos/empty.jpg';
        }}
      />
    {:else}
      <div class="absolute inset-0 w-full h-full flex items-center justify-center bg-[#131622] text-white/30">
        <GameController size={40} weight="regular" />
      </div>
    {/if}

    <!-- Resting title vignette (legible when NOT hovered) -->
    <div class="absolute bottom-0 inset-x-0 p-3.5 bg-gradient-to-t from-black/90 via-black/45 to-transparent transition-opacity duration-200 group-hover:opacity-0 pointer-events-none">
      <h4 class="text-xs sm:text-[13px] font-bold text-white truncate drop-shadow">{props.title}</h4>
      {#if props.year || props.topCategory}
        <span class="text-[10px] text-white/60 font-light truncate block mt-0.5">{props.year ? `${props.year} • ` : ''}{props.topCategory || ''}</span>
      {/if}
    </div>

    <!-- Cinematic dark vignette gradient overlay: appears on hover -->
    <div class="absolute inset-0 bg-gradient-to-t from-black/95 via-black/50 to-black/80 pointer-events-none opacity-0 group-hover:opacity-100 transition-opacity duration-300"></div>

    <!-- Card Content Layer: hidden by default, smoothly reveals on hover -->
    <div class="relative z-10 w-full h-full p-4 flex flex-col justify-between pointer-events-none opacity-0 group-hover:opacity-100 transition-all duration-300 transform scale-95 group-hover:scale-100">
      
      <!-- Top Row: Editorial Meta (e.g., "2025 | Action" or Year / Genre) -->
      <div class="flex items-center justify-between gap-2">
        <div class="text-[12px] font-normal tracking-wide text-white/80 drop-shadow">
          {#if props.year && props.topCategory}
            <span>{props.year} <span class="opacity-50 mx-0.5">|</span> {props.topCategory}</span>
          {:else if props.year}
            <span>{props.year}</span>
          {:else if props.topCategory}
            <span>{props.topCategory}</span>
          {:else}
            <span>Ducke</span>
          {/if}
        </div>
      </div>

      <!-- Center / Mid-Bottom: Bold Editorial Title + Micro Line Accent -->
      <div class="space-y-1.5 my-auto text-center px-1">
        <h3 class="text-base md:text-lg font-bold text-white tracking-tight leading-snug drop-shadow-md line-clamp-2">
          {props.title}
        </h3>
        
        <!-- Minimal white line dash (as in reference Nike card) -->
        <div class="w-4 h-[2px] bg-white/80 rounded-full mx-auto my-1 opacity-80 group-hover:w-7 transition-all duration-300"></div>

        <!-- Description or sublabel -->
        {#if props.size}
          <p class="text-[11px] font-light text-white/70 line-clamp-1 font-mono">
            {props.size}
          </p>
        {:else if props.score}
          <p class="text-[11px] font-mono text-[#66c0f4]">
            {props.score}
          </p>
        {/if}
      </div>

      <!-- Bottom Interactive Action Callout -->
      <div class="flex flex-col items-center justify-center text-center gap-0.5 pt-1 text-white/90">
        <CaretLeft size={12} weight="bold" class="rotate-90 text-white/70 group-hover:-translate-y-0.5 transition-transform" />
        <span class="text-[11px] font-medium tracking-wide">
          Открыть
        </span>
      </div>

    </div>
  </div>
{/snippet}

<!-- ===================================================================== -->
<!-- SNIPPET 3: STOREFRONT SKELETON (MATCHING GAMELISTSIDEBAR DESIGN)      -->
<!-- ===================================================================== -->
{#snippet storefrontSkeleton()}
  <div class="space-y-8 select-none pointer-events-none">
    
    <!-- 1. Hero Skeleton (Bento Layout) -->
    <section class="space-y-3">
      <div class="flex items-center justify-between px-1">
        <div class="flex items-center gap-2">
          <div class="h-3.5 w-44 rounded-[6px] bg-white/[0.05] animate-pulse"></div>
          <span class="text-[11px] text-white/30 hidden sm:inline">•</span>
          <div class="h-4 w-36 rounded-full bg-white/[0.03] border border-white/5 animate-pulse hidden sm:block"></div>
        </div>
        <div class="flex items-center gap-1.5">
          {#each Array(6) as _, i}
            <div class="h-[3px] rounded-full bg-white/10 {i === 0 ? 'w-5 bg-white/30' : 'w-2'}"></div>
          {/each}
        </div>
      </div>

      <div class="grid grid-cols-12 gap-3 sm:gap-4 lg:gap-5 items-stretch">
        <!-- Left Hero Viewport (~67%) -->
        <div class="col-span-12 lg:col-span-8 min-h-[260px] sm:min-h-[340px] lg:min-h-[420px] rounded-[24px] bg-gradient-to-b from-white/[0.03] to-white/[0.015] border border-white/[0.04] p-6 sm:p-8 flex flex-col justify-end gap-3 shadow-lg relative overflow-hidden">
          <div class="h-8 w-2/3 max-w-[360px] rounded-[10px] bg-white/[0.06] animate-pulse"></div>
          <div class="h-4 w-1/3 max-w-[200px] rounded-[6px] bg-white/[0.03] animate-pulse"></div>
          <div class="flex items-center gap-2 pt-2">
            <div class="h-6 w-20 rounded-[8px] bg-white/[0.04] animate-pulse"></div>
            <div class="h-6 w-24 rounded-[8px] bg-white/[0.04] animate-pulse"></div>
            <div class="h-6 w-16 rounded-[8px] bg-white/[0.04] animate-pulse"></div>
          </div>
        </div>

        <!-- Right Hero Sidebar (~33%) -->
        <div class="col-span-12 lg:col-span-4 rounded-[24px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] border border-white/[0.04] p-5 sm:p-6 flex flex-col justify-between space-y-4 shadow-lg">
          <div class="space-y-3.5">
            <div class="space-y-1.5">
              <div class="h-6 w-3/4 rounded-[8px] bg-white/[0.06] animate-pulse"></div>
              <div class="h-3.5 w-full rounded bg-white/[0.03] animate-pulse"></div>
              <div class="h-3.5 w-4/5 rounded bg-white/[0.03] animate-pulse"></div>
            </div>

            <!-- 2x2 Screenshots Skeleton Grid -->
            <div class="grid grid-cols-2 gap-2 pt-1">
              {#each Array(4) as _}
                <div class="aspect-[16/9] rounded-[12px] bg-white/[0.03] animate-pulse"></div>
              {/each}
            </div>

            <!-- Specs Lines -->
            <div class="divide-y divide-white/[0.04] text-xs pt-1">
              <div class="flex items-center justify-between py-1.5">
                <div class="h-3 w-16 rounded bg-white/[0.03] animate-pulse"></div>
                <div class="h-3 w-28 rounded bg-white/[0.04] animate-pulse"></div>
              </div>
              <div class="flex items-center justify-between py-1.5">
                <div class="h-3 w-14 rounded bg-white/[0.03] animate-pulse"></div>
                <div class="h-3 w-20 rounded bg-white/[0.04] animate-pulse"></div>
              </div>
            </div>
          </div>

          <!-- Action Button Skeleton -->
          <div class="pt-2 flex items-center gap-3">
            <div class="flex-1 h-11 rounded-[14px] bg-white/[0.05] animate-pulse"></div>
            <div class="w-11 h-11 rounded-[14px] bg-white/[0.03] animate-pulse"></div>
          </div>
        </div>
      </div>
    </section>

    <!-- 2. Shelf Skeleton 1: "Высокий рейтинг в Steam" -->
    <section class="space-y-3">
      <div class="flex items-center justify-between px-1">
        <div class="flex items-center gap-2">
          <div class="h-3.5 w-48 rounded-[6px] bg-white/[0.05] animate-pulse"></div>
          <div class="h-3 w-32 rounded bg-white/[0.025] hidden sm:block"></div>
        </div>
        <div class="flex items-center gap-1.5">
          <div class="w-7 h-7 rounded-[10px] bg-white/[0.02]"></div>
          <div class="w-7 h-7 rounded-[10px] bg-white/[0.02]"></div>
        </div>
      </div>

      <div class="flex gap-4 overflow-hidden pb-2 pt-1">
        {#each Array(7) as _, i}
          {@const opacity = (Math.max(0.08, 1 - Math.pow(i / 6, 1.25) * 0.92)).toFixed(3)}
          <div
            class="w-[170px] sm:w-[190px] flex-shrink-0 rounded-[18px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] p-2 space-y-2.5 border border-white/[0.03] transition-opacity duration-300"
            style="animation-delay: {i * 50}ms; opacity: {opacity};"
          >
            <div class="aspect-[2/3] w-full rounded-[14px] bg-white/[0.03] animate-pulse"></div>
            <div class="space-y-1.5 px-1 pb-1">
              <div class="h-3.5 w-4/5 rounded bg-white/[0.05] animate-pulse"></div>
              <div class="h-2.5 w-1/2 rounded bg-white/[0.03] animate-pulse"></div>
              <div class="pt-1 flex items-center justify-between">
                <div class="h-3 w-14 rounded bg-white/[0.03] animate-pulse"></div>
                <div class="h-3 w-10 rounded bg-white/[0.03] animate-pulse"></div>
              </div>
            </div>
          </div>
        {/each}
      </div>
    </section>

    <!-- 3. Shelf Skeleton 2: "Новинки каталога" -->
    <section class="space-y-3">
      <div class="flex items-center justify-between px-1">
        <div class="flex items-center gap-2">
          <div class="h-3.5 w-40 rounded-[6px] bg-white/[0.05] animate-pulse"></div>
          <div class="h-3 w-36 rounded bg-white/[0.025] hidden sm:block"></div>
        </div>
        <div class="flex items-center gap-1.5">
          <div class="w-7 h-7 rounded-[10px] bg-white/[0.02]"></div>
          <div class="w-7 h-7 rounded-[10px] bg-white/[0.02]"></div>
        </div>
      </div>

      <div class="flex gap-4 overflow-hidden pb-2 pt-1">
        {#each Array(7) as _, i}
          {@const opacity = (Math.max(0.08, 1 - Math.pow(i / 6, 1.25) * 0.92)).toFixed(3)}
          <div
            class="w-[170px] sm:w-[190px] flex-shrink-0 rounded-[18px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] p-2 space-y-2.5 border border-white/[0.03] transition-opacity duration-300"
            style="animation-delay: {i * 50 + 80}ms; opacity: {opacity};"
          >
            <div class="aspect-[2/3] w-full rounded-[14px] bg-white/[0.03] animate-pulse"></div>
            <div class="space-y-1.5 px-1 pb-1">
              <div class="h-3.5 w-3/4 rounded bg-white/[0.05] animate-pulse"></div>
              <div class="h-2.5 w-2/5 rounded bg-white/[0.03] animate-pulse"></div>
              <div class="pt-1 flex items-center justify-between">
                <div class="h-3 w-12 rounded bg-white/[0.03] animate-pulse"></div>
                <div class="h-3 w-12 rounded bg-white/[0.03] animate-pulse"></div>
              </div>
            </div>
          </div>
        {/each}
      </div>
    </section>

    <!-- 4. Tabbed Top-10 List Skeleton (identical style to GameListSidebar) -->
    <section class="space-y-3 pt-2">
      <div class="flex items-center gap-1.5 overflow-x-auto no-scrollbar py-0.5 px-1">
        <div class="h-8 w-28 rounded-[12px] bg-white/[0.08] animate-pulse"></div>
        <div class="h-8 w-24 rounded-[12px] bg-white/[0.025]"></div>
        <div class="h-8 w-36 rounded-[12px] bg-white/[0.025]"></div>
        <div class="h-8 w-20 rounded-[12px] bg-white/[0.025]"></div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-2 pt-1">
        {#each Array(6) as _, i}
          {@const opacity = (Math.max(0.12, 1 - Math.pow(i / 5, 1.25) * 0.88)).toFixed(3)}
          <div
            class="h-[64px] rounded-[18px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] p-2 flex items-center gap-3 border border-white/[0.03] transition-opacity duration-300"
            style="animation-delay: {i * 40}ms; opacity: {opacity};"
          >
            <div class="w-[48px] h-[48px] rounded-[14px] bg-white/[0.03] animate-pulse flex-shrink-0"></div>
            <div class="flex-1 space-y-1.5 min-w-0">
              <div class="h-2 w-16 rounded bg-white/[0.03] animate-pulse"></div>
              <div class="h-3 w-3/4 rounded bg-white/[0.05] animate-pulse"></div>
            </div>
            <div class="w-14 h-3.5 rounded bg-white/[0.03] animate-pulse mr-2"></div>
          </div>
        {/each}
      </div>
    </section>

  </div>
{/snippet}

<style>
  .custom-scrollbar::-webkit-scrollbar {
    width: 6px;
    height: 6px;
  }
  .custom-scrollbar::-webkit-scrollbar-track {
    background: transparent;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.1);
    border-radius: 9999px;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb:hover {
    background: rgba(255, 255, 255, 0.2);
  }
  .no-scrollbar::-webkit-scrollbar {
    display: none;
  }
  .no-scrollbar {
    -ms-overflow-style: none;
    scrollbar-width: none;
  }
</style>
