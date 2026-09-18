<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { BookmarkSimple, X, GameController, DownloadSimple as Download } from 'phosphor-svelte';
  import { sound } from '../../navigation/audio';
  import {
    GetFavorites,
    SetFavoriteStatus,
    RemoveFromFavorites
  } from '../../../../wailsjs/go/main/App';
  import { EventsOn } from '../../../../wailsjs/runtime/runtime';
  import type { GameEntity } from '../../types/game';
  import { getDisplayTitle } from '../../utils/titleUtils';

  let {
    games = [] as any[],
    torrentGames = [] as any[],
    activeDownloads = [] as any[],
    onSelectGame = (game: GameEntity) => {},
    onGoToCatalog = () => {}
  } = $props();

  type FavoriteTab = 'all' | 'planned' | 'playing' | 'completed';
  let selectedTab = $state<FavoriteTab>('all');
  let searchQuery = $state<string>('');
  let favorites = $state<any[]>([]);
  let isLoading = $state<boolean>(true);
  let isMounted = true;
  let unsubFavorites: any = null;
  let unsubEnriched: any = null;

  let brokenCovers = $state<Record<string, boolean>>({});

  function sanitizeMediaUrl(raw: string | undefined | null): string {
    if (!raw) return '';
    let url = raw.trim().replace(/^http:\/\//i, 'https://');
    url = url.replace(/video\.fastly\.steamstatic\.com/gi, 'video.akamai.steamstatic.com');
    url = url.replace(/shared\.fastly\.steamstatic\.com/gi, 'shared.steamstatic.com');
    url = url.replace(/cdn\.fastly\.steamstatic\.com/gi, 'cdn.akamai.steamstatic.com');
    if (url.startsWith('//')) {
      url = 'https:' + url;
    }
    return url;
  }

  // Resolves the best working cover image for a game, gracefully falling back if an asset returns 404
  function getGameCover(item: any): string {
    if (!item) return '';
    const candidates = [
      item.capsuleImage,
      item.headerImage,
      item.backgroundImage,
      ...(Array.isArray(item.screenshots) && item.screenshots.length > 0 ? [item.screenshots[0]] : [])
    ];
    for (const raw of candidates) {
      if (!raw || typeof raw !== 'string') continue;
      const url = sanitizeMediaUrl(raw);
      if (url && !brokenCovers[url]) {
        return url;
      }
    }
    return '';
  }

  // Fast single-pass catalog lookup for favorites enrichment
  let catalogLookup = $derived.by(() => {
    if (!Array.isArray(favorites) || favorites.length === 0) {
      return { byId: new Map<number, any>(), byAppId: new Map<number, any>(), byTitle: new Map<string, any>() };
    }

    const idSet = new Set<number>();
    const appIdSet = new Set<number>();
    const titleSet = new Set<string>();

    for (const item of favorites) {
      if (!item) continue;
      const g = item.game || (item.cleanTitle ? item : {});
      const gId = Number(g.id || item.gameId || item.id || 0);
      const sId = Number(g.steamAppId || item.steamAppId || 0);
      const title = (g.cleanTitle || item.cleanTitle || g.rawName || g.steamTitle || '').toLowerCase().trim();
      if (gId) idSet.add(gId);
      if (sId) appIdSet.add(sId);
      if (title) titleSet.add(title);
    }

    const byId = new Map<number, any>();
    const byAppId = new Map<number, any>();
    const byTitle = new Map<string, any>();

    const scanCatalog = (list: any[]) => {
      for (const g of list || []) {
        if (!g) continue;
        const gId = Number(g.id);
        const sId = Number(g.steamAppId);
        const t = (g.cleanTitle || '').toLowerCase().trim();
        if (gId && idSet.has(gId) && !byId.has(gId)) byId.set(gId, g);
        if (sId && appIdSet.has(sId) && !byAppId.has(sId)) byAppId.set(sId, g);
        if (t && titleSet.has(t) && !byTitle.has(t)) byTitle.set(t, g);
      }
    };

    scanCatalog(games);
    scanCatalog(torrentGames);

    return { byId, byAppId, byTitle };
  });

  function resolveFavoriteGame(f: any): GameEntity {
    const g = f?.game || (f?.cleanTitle ? f : {});
    const gameId = Number(g.id || f?.gameId || f?.id || 0);
    const steamAppId = Number(g.steamAppId || f?.steamAppId || 0);
    const cleanTitle = getDisplayTitle(g) || f?.cleanTitle || g.rawName || '';

    const { byId, byAppId, byTitle } = catalogLookup;
    const match = (gameId ? byId.get(gameId) : null) ||
                  (steamAppId ? byAppId.get(steamAppId) : null) ||
                  (cleanTitle ? byTitle.get(cleanTitle.toLowerCase().trim()) : null);

    const cap = sanitizeMediaUrl(g.capsuleImage || match?.capsuleImage || g.headerImage || match?.headerImage || g.backgroundImage || match?.backgroundImage || '');
    const hdr = sanitizeMediaUrl(g.headerImage || match?.headerImage || g.capsuleImage || match?.capsuleImage || g.backgroundImage || match?.backgroundImage || '');
    const bg = sanitizeMediaUrl(g.backgroundImage || match?.backgroundImage || g.headerImage || match?.headerImage || g.capsuleImage || match?.capsuleImage || '');

    return {
      ...(match || {}),
      ...g,
      id: gameId || match?.id || 0,
      steamAppId: steamAppId || match?.steamAppId || 0,
      cleanTitle: cleanTitle || match?.cleanTitle || 'Игра',
      rawName: g.rawName || f?.rawName || match?.rawName || cleanTitle,
      capsuleImage: cap,
      headerImage: hdr,
      backgroundImage: bg,
      shortDescription: g.shortDescription || match?.shortDescription || '',
      detailedDescription: g.detailedDescription || match?.detailedDescription || '',
      genres: g.genres || match?.genres || '',
      releaseDate: g.releaseDate || match?.releaseDate || '',
      developer: g.developer || match?.developer || '',
      publisher: g.publisher || match?.publisher || '',
      reviewPercent: g.reviewPercent ?? match?.reviewPercent ?? 0,
      reviewScore: g.reviewScore ?? match?.reviewScore ?? 0,
      reviewCount: g.reviewCount ?? match?.reviewCount ?? 0,
      sizeBytes: g.sizeBytes || match?.sizeBytes || 0,
      sizeDisplay: g.sizeDisplay || match?.sizeDisplay || '',
      movies: (g.movies && g.movies.length > 0) ? g.movies : (match?.movies || []),
      screenshots: (g.screenshots && g.screenshots.length > 0) ? g.screenshots : (match?.screenshots || []),
      sourceType: g.sourceType || match?.sourceType || 'favorite',
      favoriteStatus: f?.status || g.favoriteStatus || 'planned',
      isFavorite: true
    };
  }

  async function loadFavorites() {
    try {
      isLoading = true;
      const res = await GetFavorites();
      if (isMounted) {
        favorites = Array.isArray(res) ? res : [];
      }
    } catch (e) {
      console.error('Failed to load favorites in Big Picture:', e);
    } finally {
      if (isMounted) {
        isLoading = false;
      }
    }
  }

  async function handleSetStatus(gameId: number, status: string) {
    sound.playSelect();
    try {
      await SetFavoriteStatus(gameId, status);
      await loadFavorites();
    } catch (e) {
      console.error('Failed to set status:', e);
    }
  }

  async function handleRemove(gameId: number) {
    sound.playBack();
    try {
      await RemoveFromFavorites(gameId);
      await loadFavorites();
    } catch (e) {
      console.error('Failed to remove favorite:', e);
    }
  }

  let counts = $derived.by(() => {
    const all = favorites.length;
    const planned = favorites.filter((f) => f.status === 'planned').length;
    const playing = favorites.filter((f) => f.status === 'playing').length;
    const completed = favorites.filter((f) => f.status === 'completed').length;
    return { all, planned, playing, completed };
  });

  let filteredFavorites = $derived.by(() => {
    let list = favorites || [];
    if (selectedTab !== 'all') {
      list = list.filter((f) => f.status === selectedTab);
    }
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim();
      list = list.filter((f) => {
        const g = resolveFavoriteGame(f);
        const title = (g.cleanTitle || g.rawName || '').toLowerCase();
        const steamTitle = (g.steamTitle || '').toLowerCase();
        return title.includes(q) || steamTitle.includes(q);
      });
    }
    return list;
  });

  function getStatusLabel(status: string): string {
    switch (status) {
      case 'playing': return 'Прохожу';
      case 'completed': return 'Пройдено';
      case 'planned': return 'В планах';
      default: return 'В избранном';
    }
  }

  function getStatusBadgeStyle(status: string): string {
    switch (status) {
      case 'playing': return 'bg-[#07080a]/90 text-sky-400 border border-sky-500/30 font-bold';
      case 'completed': return 'bg-[#07080a]/90 text-emerald-400 border border-emerald-500/30 font-bold';
      case 'planned': return 'bg-[#07080a]/90 text-amber-400 border border-amber-500/30 font-bold';
      default: return 'bg-[#07080a]/90 text-white border border-white/20 font-bold';
    }
  }

  import { gamepad } from '../../navigation/gamepad';

  function getCleanTitle(game: any): string {
    return getDisplayTitle(game);
  }

  function getInitials(title: string): string {
    if (!title) return 'G';
    const clean = title.replace(/\[.*?\]|\(.*?\)/g, '').trim();
    const parts = clean.split(/\s+/).filter(Boolean);
    if (parts.length >= 2) {
      return (parts[0][0] + parts[1][0]).toUpperCase();
    }
    return clean.slice(0, 2).toUpperCase();
  }

  onMount(() => {
    isMounted = true;
    loadFavorites();

    const handleBtnY = (e: CustomEvent) => {
      const activeEl = document.activeElement as HTMLElement | null;
      if (!activeEl) return;
      const gameIdStr = activeEl.getAttribute('data-game-id');
      if (gameIdStr) {
        const gameId = Number(gameIdStr);
        const item = favorites.find((f) => Number(f.gameId || f.id) === gameId);
        if (item) {
          e.preventDefault();
          const nextStatusMap: Record<string, string> = {
            'planned': 'playing',
            'playing': 'completed',
            'completed': ''
          };
          const next = nextStatusMap[item.status] || 'planned';
          if (next) {
            handleSetStatus(gameId, next);
          } else {
            handleRemove(gameId);
          }
        }
      }
    };

    window.addEventListener('app:btn-y', handleBtnY as EventListener);

    unsubFavorites = EventsOn('favorites:updated', () => {
      if (isMounted) loadFavorites();
    });
    unsubEnriched = EventsOn('game:enriched', (enrichedGame: any) => {
      if (!enrichedGame || !isMounted) return;
      favorites = favorites.map((item) => {
        const g = item.game || item;
        const matchesId = item.gameId === enrichedGame.id || g.id === enrichedGame.id;
        const matchesSteam = enrichedGame.steamAppId && (g.steamAppId === enrichedGame.steamAppId || item.steamAppId === enrichedGame.steamAppId);
        const matchesCanonical = enrichedGame.canonicalKey && g.canonicalKey && enrichedGame.canonicalKey === g.canonicalKey;
        if (matchesId || matchesSteam || matchesCanonical) {
          return {
            ...item,
            game: {
              ...(item.game || {}),
              ...enrichedGame,
              favoriteStatus: item.status || g.favoriteStatus
            }
          };
        }
        return item;
      });
    });

    setTimeout(() => {
      gamepad.retryFocusZone('grid');
    }, 150);

    return () => {
      window.removeEventListener('app:btn-y', handleBtnY as EventListener);
    };
  });

  onDestroy(() => {
    isMounted = false;
    if (typeof unsubFavorites === 'function') unsubFavorites();
    if (typeof unsubEnriched === 'function') unsubEnriched();
  });
</script>

<div data-nav-zone="grid" class="flex-1 flex flex-col h-full overflow-hidden bg-[#07080a] text-white select-none relative">
  <!-- Top Ribbon: Backlog Status Tabs & Counts -->
  <div class="px-8 pt-6 pb-4 flex items-center justify-between flex-shrink-0 z-10 border-b border-white/[0.04]">
    <div class="flex items-center gap-2">
      <!-- Tab: All -->
      <button
        data-nav-item
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all cursor-pointer flex items-center gap-1.5 {selectedTab === 'all' ? 'bg-white/15 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          selectedTab = 'all';
        }}
      >
        <span>Все</span>
        <span class="px-1.5 py-0.2 rounded-full text-[10px] bg-white/10 font-mono">
          {counts.all}
        </span>
      </button>

      <!-- Tab: Planned -->
      <button
        data-nav-item
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all cursor-pointer flex items-center gap-1.5 {selectedTab === 'planned' ? 'bg-white/15 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          selectedTab = 'planned';
        }}
      >
        <span>В планах</span>
        {#if counts.planned > 0}
          <span class="px-1.5 py-0.2 rounded-full text-[10px] bg-amber-400 text-black font-black font-mono">
            {counts.planned}
          </span>
        {/if}
      </button>

      <!-- Tab: Playing -->
      <button
        data-nav-item
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all cursor-pointer flex items-center gap-1.5 {selectedTab === 'playing' ? 'bg-white/15 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          selectedTab = 'playing';
        }}
      >
        <span>Прохожу</span>
        {#if counts.playing > 0}
          <span class="px-1.5 py-0.2 rounded-full text-[10px] bg-sky-500 text-black font-black font-mono">
            {counts.playing}
          </span>
        {/if}
      </button>

      <!-- Tab: Completed -->
      <button
        data-nav-item
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all cursor-pointer flex items-center gap-1.5 {selectedTab === 'completed' ? 'bg-white/15 text-white shadow-sm' : 'text-[#8e95a2] hover:text-white hover:bg-white/[0.04]'}"
        onclick={() => {
          sound.playFocus();
          selectedTab = 'completed';
        }}
      >
        <span>Пройдено</span>
        {#if counts.completed > 0}
          <span class="px-1.5 py-0.2 rounded-full text-[10px] bg-emerald-500 text-black font-black font-mono">
            {counts.completed}
          </span>
        {/if}
      </button>
    </div>

    <!-- Active Search Filter Indicator -->
    {#if searchQuery}
      <div class="flex items-center gap-2 px-3 py-1.5 rounded-xl bg-sky-500/10 border border-sky-500/30 text-sky-400 text-xs">
        <span>Поиск: "{searchQuery}"</span>
        <button
          class="hover:text-white cursor-pointer"
          onclick={() => {
            searchQuery = '';
          }}
        >
          <X size={14} weight="bold" />
        </button>
      </div>
    {/if}
  </div>

  <!-- Main 10-foot Console Posters Grid -->
  <div class="flex-1 overflow-y-auto p-8 pt-6 relative focus:outline-none">
    {#if isLoading && favorites.length === 0}
      <div class="h-64 flex flex-col items-center justify-center text-center space-y-3 text-[#8e95a2]">
        <div class="w-10 h-10 rounded-full border-2 border-sky-400 border-t-transparent animate-spin"></div>
        <p class="text-sm font-semibold text-white">Загрузка бэклога...</p>
      </div>
    {:else if filteredFavorites.length === 0}
      <div class="h-80 flex flex-col items-center justify-center text-center space-y-4 text-[#8e95a2]">
        <div class="w-16 h-16 rounded-2xl bg-white/[0.04] border border-white/10 flex items-center justify-center text-white/30 shadow-inner">
          <BookmarkSimple size={32} weight="duotone" class="text-white/30" />
        </div>
        <div class="space-y-1">
          <p class="text-base font-bold text-white">
            {#if selectedTab === 'planned'}
              Нет игр в планах
            {:else if selectedTab === 'playing'}
              Нет игр в процессе прохождения
            {:else if selectedTab === 'completed'}
              Нет пройденных игр
            {:else}
              Список избранного пуст
            {/if}
          </p>
          <p class="text-xs text-[#64748b] max-w-sm">
            Добавляйте любые игры в бэклог прямо на странице игры с помощью кнопки «В избранное»
          </p>
        </div>
        <button
          data-nav-item
          class="px-6 py-2.5 rounded-xl bg-white/10 text-white hover:bg-white/20 text-xs font-bold transition-all cursor-pointer shadow-md active:scale-95"
          onclick={() => {
            sound.playSelect();
            onGoToCatalog();
          }}
        >
          Перейти в каталог [A]
        </button>
      </div>
    {:else}
      <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-7 gap-6 w-full">
        {#each filteredFavorites as item, idx (item?.gameId || item?.id || idx)}
          {@const game = resolveFavoriteGame(item)}
          {@const cover = getGameCover(game)}
          {@const isDownloading = (activeDownloads || []).some((d: any) => d && d.gameId === item.gameId && d.status === 'downloading')}

          <button
            data-nav-item
            data-game-id={item.gameId || item.id || (game?.id)}
            class="group relative flex flex-col rounded-2xl overflow-hidden text-left cursor-pointer transition-all duration-200 focus:scale-105 focus:ring-2 focus:ring-white focus:outline-none focus:z-20 hover:scale-103 bg-[#0d1017] border border-white/[0.05]"
            onclick={() => {
              if (game) {
                sound.playSelect();
                onSelectGame(game);
              }
            }}
          >
            <!-- Poster Aspect 2:3 -->
            <div class="relative aspect-[2/3] w-full bg-[#11141c] overflow-hidden">
              {#if cover}
                <img
                  src={cover}
                  alt={getCleanTitle(game)}
                  referrerpolicy="no-referrer"
                  class="w-full h-full object-cover transition-transform duration-300 group-hover:scale-103"
                  onerror={() => {
                    brokenCovers[cover] = true;
                  }}
                />
              {:else}
                <div class="w-full h-full flex flex-col items-center justify-between p-4 text-center bg-gradient-to-b from-[#181d28] via-[#10141d] to-[#0a0c12]">
                  <div class="w-full flex justify-end">
                    <GameController size={16} weight="regular" class="text-white/20" />
                  </div>
                  <div class="my-auto flex flex-col items-center space-y-2">
                    <div class="w-12 h-12 rounded-2xl bg-white/[0.06] border border-white/10 flex items-center justify-center text-base font-black text-white/80 shadow-inner">
                      {getInitials(getCleanTitle(game))}
                    </div>
                    <span class="text-xs font-bold text-[#e2e8f0] line-clamp-3 leading-snug px-1">
                      {getCleanTitle(game)}
                    </span>
                  </div>
                </div>
              {/if}

              <!-- Status Badge Overlay Top Left -->
              {#if selectedTab === 'all'}
                <div class="absolute top-2.5 left-2.5 px-2 py-0.5 rounded-md text-[9px] uppercase tracking-wider z-20 shadow-md {getStatusBadgeStyle(item.status)}">
                  {getStatusLabel(item.status)}
                </div>
              {/if}

              <!-- Downloading Badge Overlay Top Right -->
              {#if isDownloading}
                <div class="absolute top-2.5 right-2.5 px-2 py-0.5 rounded-md bg-[#07080a]/90 border border-sky-500/40 text-sky-400 text-[9px] font-bold tracking-wider flex items-center gap-1 shadow-md z-20">
                  <Download class="w-3 h-3" />
                  <span>СКАЧИВАЕТСЯ</span>
                </div>
              {/if}

              <!-- Review score bottom left -->
              {#if game?.reviewPercent && game.reviewPercent > 0}
                <div class="absolute bottom-2 left-2 px-1.5 py-0.5 rounded-md bg-[#07080a]/95 text-[10px] font-mono font-bold flex items-center gap-1 border border-white/10 z-20 {game.reviewPercent >= 70 ? 'text-sky-400' : 'text-[#94a3b8]'}">
                  <span>★ {game.reviewPercent}%</span>
                </div>
              {/if}

              <!-- Size badge bottom right -->
              {#if game?.sizeDisplay}
                <div class="absolute bottom-2 right-2 px-1.5 py-0.5 rounded-md bg-[#07080a]/95 text-[10px] font-mono text-white/90 border border-white/10 z-20">
                  {game.sizeDisplay}
                </div>
              {/if}
            </div>

            <!-- Card Bottom Title -->
            <div class="p-3 bg-[#07080a] flex flex-col space-y-1">
              <span class="text-xs font-bold text-white truncate group-hover:text-sky-400 transition-colors">
                {getCleanTitle(game)}
              </span>
              {#if game?.genres && game.genres.length > 0}
                <span class="text-[10px] text-[#64748b] truncate">
                  {game.genres.slice(0, 2).join(' • ')}
                </span>
              {/if}
            </div>
          </button>
        {/each}
      </div>
    {/if}
  </div>
</div>
