<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Search,
    Download,
    Check,
    Gamepad2,
    HardDrive,
    X
  } from 'lucide-svelte';
  import { sound } from '../../navigation/audio';
  import { gamepad } from '../../navigation/gamepad';

  let {
    games = [] as any[],
    activeDownloads = [] as any[],
    searchQuery = $bindable(''),
    onSelectGame = (game: any) => {},
    isSearchOpen = false,
    onCloseSearch = () => {}
  } = $props();

  type FilterType = 'all' | 'downloading' | 'has_steam';
  let activeFilter = $state<FilterType>('all');
  let searchInputEl = $state<HTMLInputElement | null>(null);

  // Virtualization State
  let scrollContainer = $state<HTMLDivElement | null>(null);
  let scrollTop = $state<number>(0);
  let containerWidth = $state<number>(1280);
  let containerHeight = $state<number>(800);
  let measuredCardHeight = $state<number>(0);

  const GAP = 24; // gap-6 = 1.5rem = 24px
  const OVERSCAN_ROWS = 3; // 3 rows buffer above and below for smooth gamepad and analog stick scrolling

  let filteredGames = $derived.by(() => {
    let list = games || [];

    // Filter by category
    if (activeFilter === 'downloading') {
      const activeIds = new Set((activeDownloads || []).map((d) => d && d.gameId));
      list = list.filter((g) => activeIds.has(g.id));
    } else if (activeFilter === 'has_steam') {
      list = list.filter((g) => (g.steamAppId && g.steamAppId !== 0) || g.capsuleImage || g.steamSynced);
    }

    // Filter by search query
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim();
      list = list.filter((g) => {
        const title = (g.cleanTitle || g.displayTitle || g.folderName || '').toLowerCase();
        const steamTitle = (g.steamTitle || '').toLowerCase();
        return title.includes(q) || steamTitle.includes(q);
      });
    }

    // Default sort by date added to server (newest first)
    return list.slice().sort((a, b) => (b.id || 0) - (a.id || 0));
  });

  // Calculate dynamic column count based on container width matching Tailwind breakpoints
  let columnCount = $derived.by(() => {
    if (containerWidth >= 1536) return 7;
    if (containerWidth >= 1280) return 6;
    if (containerWidth >= 1024) return 5;
    if (containerWidth >= 768) return 4;
    if (containerWidth >= 640) return 3;
    return 2;
  });

  // Calculate card & row dimensions
  let availWidth = $derived(Math.max(100, containerWidth - 64)); // 32px padding left + 32px right
  let cardWidth = $derived(Math.max(80, (availWidth - (columnCount - 1) * GAP) / columnCount));
  let estimatedCardHeight = $derived(Math.round(cardWidth * 1.5 + 56)); // 2:3 aspect ratio + footer title/badges
  let cardHeight = $derived(measuredCardHeight > 0 ? measuredCardHeight : estimatedCardHeight);
  let rowTotalHeight = $derived(cardHeight + GAP);

  // Virtualization slicing
  let totalItems = $derived(filteredGames.length);
  let totalRows = $derived(Math.ceil(totalItems / columnCount));
  let totalGridHeight = $derived(totalRows > 0 ? totalRows * rowTotalHeight - GAP : 0);

  let startRow = $derived.by(() => {
    if (totalRows === 0) return 0;
    const r = Math.floor(scrollTop / rowTotalHeight);
    return Math.max(0, r - OVERSCAN_ROWS);
  });

  let endRow = $derived.by(() => {
    if (totalRows === 0) return 0;
    const r = Math.ceil((scrollTop + containerHeight) / rowTotalHeight);
    return Math.min(Math.max(0, totalRows - 1), r + OVERSCAN_ROWS);
  });

  let startIndex = $derived(startRow * columnCount);
  let endIndex = $derived(Math.min(totalItems, (endRow + 1) * columnCount));

  let visibleGames = $derived.by(() => {
    return filteredGames.slice(startIndex, endIndex);
  });

  let offsetY = $derived(startRow * rowTotalHeight);

  function handleScroll() {
    if (scrollContainer) {
      scrollTop = scrollContainer.scrollTop;
    }
  }

  // Reset scroll when filter or search changes
  $effect(() => {
    const _ = [activeFilter, searchQuery];
    if (scrollContainer) {
      scrollContainer.scrollTop = 0;
      scrollTop = 0;
    }
  });

  onMount(() => {
    if (!scrollContainer) return;
    containerWidth = scrollContainer.clientWidth || 1280;
    containerHeight = scrollContainer.clientHeight || 800;

    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.target === scrollContainer) {
          containerWidth = scrollContainer.clientWidth;
          containerHeight = scrollContainer.clientHeight;
        }
      }
    });
    ro.observe(scrollContainer);

    return () => ro.disconnect();
  });

  function measureCard(node: HTMLElement) {
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (node.offsetHeight > 0) {
          measuredCardHeight = node.offsetHeight;
        }
      }
    });
    ro.observe(node);
    if (node.offsetHeight > 0) {
      measuredCardHeight = node.offsetHeight;
    }
    return {
      destroy() {
        ro.disconnect();
      }
    };
  }

  $effect(() => {
    if (isSearchOpen && searchInputEl) {
      setTimeout(() => {
        searchInputEl?.focus();
        searchInputEl?.select();
      }, 50);
    }
  });

  let imageFailedMap = $state<Record<number | string, boolean>>({});

  function getInitials(title: string): string {
    if (!title) return 'D';
    const clean = title.replace(/[^a-zA-Zа-яА-Я0-9\s]/g, '').trim();
    const words = clean.split(/\s+/).filter(Boolean);
    if (words.length >= 2) {
      return (words[0][0] + words[1][0]).toUpperCase();
    }
    return clean.slice(0, 2).toUpperCase() || 'D';
  }

  function isHorizontalAsset(url: string): boolean {
    if (!url) return false;
    return /capsule_231x87|capsule_616x353|capsule_467x181|header\.jpg|header_alt/i.test(url);
  }

  // Steam Deck 2:3 vertical poster URL priority
  function getVerticalCover(g: any): string {
    if (!g) return '';
    if (g.capsuleImage && !isHorizontalAsset(g.capsuleImage)) {
      return g.capsuleImage;
    }
    if (g.steamAppId && g.steamAppId > 0) {
      return `https://shared.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/library_600x900.jpg`;
    }
    if (g.libraryCover && !isHorizontalAsset(g.libraryCover)) return g.libraryCover;
    if (g.coverUrl && !isHorizontalAsset(g.coverUrl)) return g.coverUrl;
    return '';
  }

  const fetchingSGDB = new Set<string>();

  async function handleImageError(e: Event, game: any) {
    const target = e.currentTarget as HTMLImageElement;
    if (!target) return;
    const appId = game.steamAppId;
    const currentSrc = target.src;
    const gameKey = String(game.id || game.folderName || '');

    if (appId && appId > 0) {
      if (currentSrc.includes('shared.steamstatic.com')) {
        target.src = `https://steamcdn-a.akamaihd.net/steam/apps/${appId}/library_600x900.jpg`;
        return;
      }
      if (currentSrc.includes('steamcdn-a.akamaihd.net')) {
        target.src = `https://cdn.cloudflare.steamstatic.com/steam/apps/${appId}/library_600x900.jpg`;
        return;
      }
      if (currentSrc.includes('cdn.cloudflare.steamstatic.com')) {
        target.src = `https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/${appId}/library_600x900_2x.jpg`;
        return;
      }
    }

    // Check if we have a valid vertical cover already stored
    if (game.capsuleImage && !isHorizontalAsset(game.capsuleImage) && target.src !== game.capsuleImage) {
      target.src = game.capsuleImage;
      return;
    }

    // On-the-fly fetch from SteamGridDB for missing portrait cover
    const term = game.cleanTitle || game.displayTitle || game.steamTitle || game.folderName || '';
    if (term && !fetchingSGDB.has(gameKey)) {
      fetchingSGDB.add(gameKey);
      try {
        const appApi = (window as any)?.go?.main?.App;
        if (appApi && typeof appApi.ResolveGameCover === 'function') {
          const sgdbCover = await appApi.ResolveGameCover(game.id || 0, term, game.steamAppId || 0);
          if (sgdbCover) {
            game.capsuleImage = sgdbCover;
            target.src = sgdbCover;
            return;
          }
        }
      } catch {
        // Normal fallback when portrait cover is not available on SteamGridDB
      }
    }

    // Strictly portrait: do NOT fall back to landscape banners; render clean Steam Deck card
    imageFailedMap[gameKey] = true;
  }
</script>

<div data-nav-zone="grid" class="flex-1 flex flex-col h-full overflow-hidden bg-[#07080a] text-white select-none relative">
  
  <!-- Top Ribbon: Filters & Game Count -->
  <div class="px-8 pt-6 pb-4 flex items-center justify-between flex-shrink-0 z-10 border-b border-white/[0.04]">
    <div class="flex items-center gap-2">
      <button
        data-nav-item
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all cursor-pointer {activeFilter === 'all' ? 'bg-white/15 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          activeFilter = 'all';
        }}
      >
        Все игры ({games.length})
      </button>

      <button
        data-nav-item
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all cursor-pointer {activeFilter === 'has_steam' ? 'bg-white/15 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          activeFilter = 'has_steam';
        }}
      >
        Steam данные
      </button>

      <button
        data-nav-item
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all cursor-pointer flex items-center gap-1.5 {activeFilter === 'downloading' ? 'bg-white/15 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          activeFilter = 'downloading';
        }}
      >
        <span>В загрузках</span>
        {#if (activeDownloads || []).length > 0}
          <span class="px-1.5 py-0.2 rounded-full text-[10px] bg-sky-500 text-black font-black">
            {(activeDownloads || []).length}
          </span>
        {/if}
      </button>
    </div>

    <!-- Active Search Indicator -->
    {#if searchQuery}
      <div class="flex items-center gap-2 px-3 py-1.5 rounded-xl bg-sky-500/10 border border-sky-500/30 text-sky-400 text-xs">
        <span>Поиск: "{searchQuery}"</span>
        <button
          class="hover:text-white cursor-pointer"
          onclick={() => {
            searchQuery = '';
          }}
        >
          <X class="w-3.5 h-3.5" />
        </button>
      </div>
    {/if}
  </div>

  <!-- Search Modal Overlay (when X is pressed) -->
  {#if isSearchOpen}
    <div data-nav-zone="modal" class="absolute inset-x-0 top-0 z-50 bg-[#07080a]/95 backdrop-blur-xl border-b border-white/10 p-6 flex items-center justify-center animate-fade-in shadow-2xl">
      <div class="w-full max-w-2xl flex items-center gap-3">
        <Search class="w-5 h-5 text-sky-400 flex-shrink-0" />
        <input
          data-nav-item
          bind:this={searchInputEl}
          type="text"
          bind:value={searchQuery}
          placeholder="Поиск игр в библиотеке..."
          class="flex-1 bg-transparent text-white text-base font-semibold focus:outline-none placeholder-[#64748b]"
          onkeydown={(e) => {
            if (e.key === 'Escape' || e.key === 'Enter') {
              onCloseSearch();
            }
          }}
        />
        <button
          data-nav-item
          class="px-4 py-2 rounded-xl text-xs font-bold bg-white/10 text-white hover:bg-white/20 cursor-pointer"
          onclick={onCloseSearch}
        >
          Готово
        </button>
      </div>
    </div>
  {/if}

  <!-- Main 10-foot Console Posters Grid with Virtualization -->
  <div
    bind:this={scrollContainer}
    onscroll={handleScroll}
    class="flex-1 overflow-y-auto p-8 pt-6 relative focus:outline-none"
  >
    {#if filteredGames.length === 0}
      <div class="h-64 flex flex-col items-center justify-center text-center space-y-2 text-[#8e95a2]">
        <p class="text-sm font-semibold">Игры не найдены</p>
        {#if searchQuery}
          <p class="text-xs text-[#64748b]">Попробуйте изменить поисковый запрос</p>
        {/if}
      </div>
    {:else}
      <!-- Total virtual height spacer -->
      <div style="height: {totalGridHeight}px; position: relative; width: 100%;">
        <!-- Rendered visible items slice shifted via translateY -->
        <div
          style="transform: translateY({offsetY}px); will-change: transform; grid-template-columns: repeat({columnCount}, minmax(0, 1fr));"
          class="grid gap-6 w-full"
        >
          {#each visibleGames as game, idx (game.id || game.folderName)}
            {@const gameKey = game.id || game.folderName}
            {@const cover = getVerticalCover(game)}
            {@const isDownloading = (activeDownloads || []).some((d) => d && d.gameId === game.id && d.status === 'downloading')}
            {@const isFailed = !!imageFailedMap[gameKey]}

            <button
              data-nav-item
              use:measureCard
              class="group relative flex flex-col rounded-2xl overflow-hidden text-left cursor-pointer transition-all duration-200 focus:scale-105 focus:ring-2 focus:ring-white focus:outline-none focus:z-20 hover:scale-103 bg-[#0d1017] border-0"
              onclick={() => {
                sound.playSelect();
                onSelectGame(game);
              }}
            >
              <!-- Poster Aspect 2:3 -->
              <div class="relative aspect-[2/3] w-full bg-[#11141c] overflow-hidden">
                {#if cover && !isFailed}
                  <img
                    src={cover}
                    alt={game.cleanTitle || game.folderName}
                    referrerpolicy="no-referrer"
                    class="w-full h-full object-cover transition-transform duration-300 group-hover:scale-103"
                    onerror={(e) => handleImageError(e, game)}
                  />
                {:else}
                  <!-- Rich Console Monogram Placeholder -->
                  <div class="w-full h-full flex flex-col items-center justify-between p-4 text-center bg-gradient-to-b from-[#181d28] via-[#10141d] to-[#0a0c12] relative overflow-hidden">
                    <div class="w-full flex justify-end">
                      <Gamepad2 class="w-4 h-4 text-white/20" />
                    </div>

                    <div class="my-auto flex flex-col items-center space-y-2">
                      <div class="w-12 h-12 rounded-2xl bg-white/[0.06] border border-white/10 flex items-center justify-center text-base font-black text-white/80 shadow-inner">
                        {getInitials(game.cleanTitle || game.folderName)}
                      </div>
                      <span class="text-xs font-bold text-[#e2e8f0] line-clamp-3 leading-snug px-1">
                        {game.cleanTitle || game.folderName}
                      </span>
                    </div>

                    <div class="w-full text-center">
                      <span class="text-[9px] font-mono uppercase tracking-widest text-[#64748b]">Не привязано</span>
                    </div>
                  </div>
                {/if}

                <!-- Downloading / Queued Badge Overlay -->
                {#if isDownloading}
                  <div class="absolute top-2.5 right-2.5 px-2 py-1 rounded-lg bg-sky-500 text-black text-[10px] font-black tracking-wider flex items-center gap-1 shadow-lg z-20">
                    <Download class="w-3 h-3 animate-bounce" />
                    <span>СКАЧИВАЕТСЯ</span>
                  </div>
                {/if}

                <!-- Size badge bottom right -->
                {#if game.sizeDisplay || game.sizeStr}
                  <div class="absolute bottom-2 right-2 px-1.5 py-0.5 rounded-md bg-black/80 backdrop-blur-md text-[10px] font-mono text-white/90 border border-white/10 z-20">
                    {game.sizeDisplay || game.sizeStr}
                  </div>
                {/if}
              </div>

              <!-- Title & Steam Rating -->
              <div class="p-3 bg-[#07080a] flex flex-col space-y-1">
                <span class="text-xs font-bold text-white truncate group-hover:text-sky-400 transition-colors">
                  {game.cleanTitle || game.displayTitle || game.folderName}
                </span>

                {#if (game.genres && game.genres.length > 0) || game.steamGenres}
                  <span class="text-[10px] text-[#64748b] truncate">
                    {(game.genres || game.steamGenres || []).slice(0, 2).join(' • ')}
                  </span>
                {/if}
              </div>
            </button>
          {/each}
        </div>
      </div>
    {/if}
  </div>
</div>
