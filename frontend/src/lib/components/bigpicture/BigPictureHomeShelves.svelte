<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Play,
    DownloadSimple as Download,
    Folder,
    Star,
    Check,
    GameController as Gamepad2,
    FilmStrip as Film,
    Image as ImageIcon,
    CaretLeft as ChevronLeft,
    CaretRight as ChevronRight,
    X,
    ArrowsOut as Maximize2,
    SpeakerHigh as Volume2,
    SpeakerSlash as VolumeX,
    HardDrive,
    Info,
    Calendar,
    SquaresFour as Layers,
    Clock,
    Pause,
    DotsThree as MoreHorizontal,
    ArrowDown,
    ArrowsClockwise as RefreshCw
  } from 'phosphor-svelte';
  import Hls from 'hls.js';
  import { sound } from '../../navigation/audio';
  import { deduplicateGames, cleanCanonicalKey } from '../../utils/gameDeduplication';
  import {
    GetFavorites,
    SetFavoriteStatus,
    RemoveFromFavorites,
    GetDownloadHistory,
    LaunchGameByGameID,
    OpenGameFolder,
    GetGameDetails,
    GetGameMovies,
    EnrichGameNow
  } from '../../../../wailsjs/go/main/App';
  import { EventsOn } from '../../../../wailsjs/runtime/runtime';
  import type { GameEntity, SteamMovie } from '../../types/game';
  import { formatTorrentSourceName } from '../../utils/sourceFormatter';

  let {
    games = [] as any[],
    torrentGames = [] as any[],
    activeDownloads = [] as any[],
    downloadHistory: propDownloadHistory = [] as any[],
    hasFtpServers = false,
    hasTorrentSources = false,
    downloadPath = 'C:\\Ducke',
    onSelectGame = (game: any) => {},
    onStartDownload = (gameId: number, targetPath: string) => {},
    onGoToCatalog = () => {},
    onGoToTorrents = () => {},
    onGoToFavorites = () => {},
    onGoToDownloads = () => {}
  } = $props();

  let favorites = $state<any[]>([]);
  let localDownloadHistory = $state<any[]>([]);
  let downloadHistory = $derived(
    (propDownloadHistory && propDownloadHistory.length > 0) ? propDownloadHistory : localDownloadHistory
  );
  let brokenCovers = $state<Record<string, boolean>>({});
  let selectedFilter = $state<'all' | 'installed' | 'favorites' | 'catalog' | 'torrents'>('all');
  let focusedIndex = $state<number>(0);
  let isMounted = true;

  // Virtual horizontal window constants
  const WINDOW_BEFORE = 8;
  const WINDOW_AFTER = 20;
  const CARD_STEP_PX = 152; // 136px width + 16px gap

  // Background Ambient Trailer & Theater Mode State
  const AMBIENT_TRAILER_DELAY_MS = 2500;
  let trailerTimer: any = null;
  let isAmbientTrailerActive = $state<boolean>(false);
  let isTheaterMode = $state<boolean>(false);
  let isVideoMuted = $state<boolean>(true);
  let isVideoPlaying = $state<boolean>(true);
  let videoHasRenderedFrame = $state<boolean>(false);
  let currentMovieIndex = $state<number>(0);
  let videoBgEl: HTMLVideoElement | null = $state(null);

  let logoFailedMap = $state<Record<number, boolean>>({});
  let activeScreenshotPreview = $state<string | null>(null);

  // Lightbox modal state
  let lightboxImage = $state<string | null>(null);
  let lightboxIndex = $state<number>(0);

  // Strip container ref
  let stripContainerEl: HTMLDivElement | null = $state(null);
  let cardRefs = $state<Record<number, HTMLButtonElement | null>>({});

  async function loadData() {
    try {
      const [favRes, histRes] = await Promise.all([
        GetFavorites().catch(() => []),
        GetDownloadHistory().catch(() => [])
      ]);
      if (isMounted) {
        favorites = Array.isArray(favRes) ? favRes : [];
        localDownloadHistory = Array.isArray(histRes) ? histRes : [];
      }
    } catch (e) {
      console.error('Failed to load shelves data:', e);
    }
  }

  onMount(() => {
    loadData();

    const unsubFav = EventsOn('favorites:updated', () => {
      if (isMounted) loadData();
    });

    const unsubEnriched = EventsOn('game:enriched', (enrichedGame: any) => {
      if (!enrichedGame || !isMounted) return;
      if (enrichedGame.id) {
        enrichedOverrides[enrichedGame.id] = enrichedGame;
        const cur = focusedGame;
        if (cur && (cur.id === enrichedGame.id || (cur.steamAppId && cur.steamAppId === enrichedGame.steamAppId))) {
          if (enrichedGame.movies || enrichedGame.screenshots) {
            const entry = {
              movies: (enrichedGame.movies && enrichedGame.movies.length > 0) ? enrichedGame.movies : (currentDetails?.movies || []),
              screenshots: (enrichedGame.screenshots && enrichedGame.screenshots.length > 0) ? enrichedGame.screenshots : (currentDetails?.screenshots || [])
            };
            detailsCache.set(cur.id, entry);
            currentDetails = entry;
          }
        }
      }
    });

    const handleKeyDown = (e: KeyboardEvent) => {
      if (lightboxImage) {
        if (e.key === 'Escape' || e.key === 'Backspace') {
          closeLightbox();
          e.preventDefault();
        } else if (e.key === 'ArrowLeft') {
          prevLightboxImage();
          e.preventDefault();
        } else if (e.key === 'ArrowRight') {
          nextLightboxImage();
          e.preventDefault();
        }
        return;
      }

      // When in Theater Mode
      if (isTheaterMode) {
        if (e.key === 'ArrowDown' || e.key === 'Escape' || e.key === 'Backspace') {
          exitTheaterMode();
          e.preventDefault();
        } else if (e.key === ' ' || e.key === 'Enter') {
          togglePlayPause();
          e.preventDefault();
        } else if (e.key === 'm' || e.key === 'M') {
          toggleMute();
          e.preventDefault();
        } else if (e.key === 'ArrowRight') {
          nextTrailer();
          e.preventDefault();
        } else if (e.key === 'ArrowLeft') {
          prevTrailer();
          e.preventDefault();
        } else if (e.key === 'ArrowUp') {
          e.preventDefault();
        }
        return;
      }

      // Normal Dashboard Mode
      if (e.key === 'ArrowUp') {
        if (movieList.length > 0) {
          enterTheaterMode();
          e.preventDefault();
        }
      } else if (e.key === 'ArrowLeft') {
        moveFocus(-1);
        e.preventDefault();
      } else if (e.key === 'ArrowRight') {
        moveFocus(1);
        e.preventDefault();
      }
    };

    // Controller Directional Navigation listener
    const handleGamepadDir = (e: CustomEvent) => {
      const dir = e.detail?.dir;
      if (!dir) return;

      if (isTheaterMode) {
        if (dir === 'DOWN') {
          exitTheaterMode();
        } else if (dir === 'RIGHT') {
          nextTrailer();
        } else if (dir === 'LEFT') {
          prevTrailer();
        }
      }
    };

    const handleBtnX = (e: CustomEvent) => {
      if (isTheaterMode) {
        toggleMute();
        e.preventDefault();
        return;
      }
      handlePrimaryAction();
      e.preventDefault();
    };

    const handleBtnY = (e: CustomEvent) => {
      if (isTheaterMode) return;
      handleToggleFavorite();
      e.preventDefault();
    };

    const handleSubtabPrev = (e: CustomEvent) => {
      if (isTheaterMode) {
        prevTrailer();
        e.preventDefault();
      } else if (lightboxImage) {
        cycleLightbox(-1);
        e.preventDefault();
      }
    };

    const handleSubtabNext = (e: CustomEvent) => {
      if (isTheaterMode) {
        nextTrailer();
        e.preventDefault();
      } else if (lightboxImage) {
        cycleLightbox(1);
        e.preventDefault();
      }
    };

    const handleGoBack = (e: CustomEvent) => {
      if (lightboxImage) {
        closeLightbox();
        e.preventDefault();
        e.stopImmediatePropagation();
      } else if (isTheaterMode) {
        exitTheaterMode();
        e.preventDefault();
        e.stopImmediatePropagation();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('app:gamepad-dir', handleGamepadDir as EventListener);
    window.addEventListener('app:btn-x', handleBtnX as EventListener);
    window.addEventListener('app:btn-y', handleBtnY as EventListener);
    window.addEventListener('app:subtab-prev', handleSubtabPrev as EventListener);
    window.addEventListener('app:subtab-next', handleSubtabNext as EventListener);
    window.addEventListener('app:go-back', handleGoBack as EventListener, true);

    return () => {
      isMounted = false;
      if (isTheaterMode) {
        window.dispatchEvent(new CustomEvent('app:theater-mode', { detail: { active: false } }));
      }
      stopAndUnloadVideo();
      if (typeof unsubFav === 'function') unsubFav();
      if (typeof unsubEnriched === 'function') unsubEnriched();
      if (trailerTimer) clearTimeout(trailerTimer);
      if (detailsTimer) clearTimeout(detailsTimer);
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('app:gamepad-dir', handleGamepadDir as EventListener);
      window.removeEventListener('app:btn-x', handleBtnX as EventListener);
      window.removeEventListener('app:btn-y', handleBtnY as EventListener);
      window.removeEventListener('app:subtab-prev', handleSubtabPrev as EventListener);
      window.removeEventListener('app:subtab-next', handleSubtabNext as EventListener);
      window.removeEventListener('app:go-back', handleGoBack as EventListener, true);
    };
  });

  // Fast URL sanitizer ensuring Akamai/Steam static CDN (bypassing Fastly blockages in CIS)
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

  // Resolves the best working cover image for a game, gracefully falling back to header/background if an asset returns 404
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

  // Normalized list of favorites as full GameEntity objects with single-pass catalog enrichment
  let normalizedFavorites = $derived.by<any[]>(() => {
    if (!Array.isArray(favorites) || favorites.length === 0) return [];

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

    const matchesById = new Map<number, any>();
    const matchesByAppId = new Map<number, any>();
    const matchesByTitle = new Map<string, any>();

    const scanCatalog = (list: any[]) => {
      for (const g of list || []) {
        if (!g) continue;
        const gId = Number(g.id);
        const sId = Number(g.steamAppId);
        const t = (g.cleanTitle || '').toLowerCase().trim();
        if (gId && idSet.has(gId) && !matchesById.has(gId)) matchesById.set(gId, g);
        if (sId && appIdSet.has(sId) && !matchesByAppId.has(sId)) matchesByAppId.set(sId, g);
        if (t && titleSet.has(t) && !matchesByTitle.has(t)) matchesByTitle.set(t, g);
      }
    };

    scanCatalog(games);
    scanCatalog(torrentGames);

    return favorites.map((item: any) => {
      if (!item) return null;
      const g = item.game || (item.cleanTitle ? item : {});
      const gameId = Number(g.id || item.gameId || item.id || 0);
      const steamAppId = Number(g.steamAppId || item.steamAppId || 0);
      const cleanTitle = g.cleanTitle || item.cleanTitle || g.rawName || g.steamTitle || '';

      const match = (gameId ? matchesById.get(gameId) : null) ||
                    (steamAppId ? matchesByAppId.get(steamAppId) : null) ||
                    (cleanTitle ? matchesByTitle.get(cleanTitle.toLowerCase().trim()) : null);

      const cap = sanitizeMediaUrl(g.capsuleImage || match?.capsuleImage || g.headerImage || match?.headerImage || g.backgroundImage || match?.backgroundImage || '');
      const hdr = sanitizeMediaUrl(g.headerImage || match?.headerImage || g.capsuleImage || match?.capsuleImage || g.backgroundImage || match?.backgroundImage || '');
      const bg = sanitizeMediaUrl(g.backgroundImage || match?.backgroundImage || g.headerImage || match?.headerImage || g.capsuleImage || match?.capsuleImage || '');

      return {
        ...(match || {}),
        ...g,
        id: gameId || match?.id || 0,
        steamAppId: steamAppId || match?.steamAppId || 0,
        cleanTitle: cleanTitle || match?.cleanTitle || 'Игра',
        rawName: g.rawName || item.rawName || match?.rawName || cleanTitle,
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
        favoriteStatus: item.status || g.favoriteStatus || 'planned',
        isFavorite: true
      };
    }).filter(Boolean);
  });

  // Lookup for installed game IDs and titles
  let installedData = $derived.by(() => {
    const ids = new Set<number>();
    const titles = new Set<string>();
    for (const h of downloadHistory || []) {
      if (h && (h.status === 'completed' || h.isInstalled)) {
        if (h.gameId) ids.add(Number(h.gameId));
        if (h.steamAppId) ids.add(Number(h.steamAppId));
        if (h.gameTitle) titles.add(h.gameTitle.toLowerCase().trim());
      }
    }
    return { ids, titles };
  });

  // Active downloading map
  let activeDownloadingMap = $derived.by(() => {
    const map = new Map<number, any>();
    for (const d of activeDownloads || []) {
      if (d && (d.status === 'downloading' || d.status === 'queued') && d.gameId) {
        map.set(Number(d.gameId), d);
      }
    }
    return map;
  });

  // Favorite game IDs and titles
  let favoriteData = $derived.by(() => {
    const ids = new Set<number>();
    const titles = new Set<string>();
    for (const f of normalizedFavorites) {
      if (!f) continue;
      if (f.id) ids.add(Number(f.id));
      if (f.steamAppId) ids.add(Number(f.steamAppId));
      if (f.cleanTitle) titles.add(f.cleanTitle.toLowerCase().trim());
      if (f.steamTitle && !/^Steam App \d+$/i.test(f.steamTitle)) {
        titles.add(f.steamTitle.toLowerCase().trim());
      }
      if (f.variants && Array.isArray(f.variants)) {
        for (const v of f.variants) {
          if (v.id) ids.add(Number(v.id));
          if (v.steamAppId) ids.add(Number(v.steamAppId));
          if (v.cleanTitle) titles.add(v.cleanTitle.toLowerCase().trim());
        }
      }
    }
    for (const raw of favorites || []) {
      if (!raw) continue;
      const rId = Number(raw.gameId || raw.id || raw.game?.id || 0);
      if (rId) ids.add(rId);
      const sId = Number(raw.steamAppId || raw.game?.steamAppId || 0);
      if (sId) ids.add(sId);
      const rTitle = raw.cleanTitle || raw.game?.cleanTitle || raw.game?.rawName || '';
      if (rTitle) titles.add(rTitle.toLowerCase().trim());
    }
    return { ids, titles };
  });

  function checkGameInstalled(g: any): boolean {
    if (!g) return false;
    const { ids, titles } = installedData;
    const gId = Number(g.id);
    if (ids.has(gId)) return true;
    if (g.steamAppId && ids.has(Number(g.steamAppId))) return true;
    if (activeDownloadingMap.has(gId)) return true;
    if (g.cleanTitle && titles.has(g.cleanTitle.toLowerCase().trim())) return true;
    if (g.variants && Array.isArray(g.variants)) {
      for (const v of g.variants) {
        const vId = Number(v.id);
        if (ids.has(vId) || activeDownloadingMap.has(vId)) return true;
        if (v.steamAppId && ids.has(Number(v.steamAppId))) return true;
        if (v.cleanTitle && titles.has(v.cleanTitle.toLowerCase().trim())) return true;
      }
    }
    return false;
  }

  function checkGameFavorite(g: any): boolean {
    if (!g) return false;
    const { ids, titles } = favoriteData;
    const gId = Number(g.id);
    if (ids.has(gId)) return true;
    if (g.steamAppId && ids.has(Number(g.steamAppId))) return true;
    if (g.cleanTitle && titles.has(g.cleanTitle.toLowerCase().trim())) return true;
    if (g.variants && Array.isArray(g.variants)) {
      for (const v of g.variants) {
        const vId = Number(v.id);
        if (ids.has(vId)) return true;
        if (v.steamAppId && ids.has(Number(v.steamAppId))) return true;
        if (v.cleanTitle && titles.has(v.cleanTitle.toLowerCase().trim())) return true;
      }
    }
    return false;
  }

  function getCanonicalKey(g: any): string {
    if (!g) return '';
    if (g.steamAppId && Number(g.steamAppId) > 0) return `appid:${g.steamAppId}`;
    const key = g.canonicalKey || cleanCanonicalKey(g.cleanTitle || g.rawName || g.steamTitle || '');
    if (key) return `title:${key}`;
    return `id:${g.id}`;
  }

  // Memoized catalog pool deduplication to avoid running 20k deduplication on every tick
  let cachedCatalogPool: any[] = [];
  let lastGamesRef: any[] | null = null;
  let lastTorrentsRef: any[] | null = null;

  function getDeduplicatedCatalog(): any[] {
    if (games === lastGamesRef && torrentGames === lastTorrentsRef && cachedCatalogPool.length > 0) {
      return cachedCatalogPool;
    }
    const gList = Array.isArray(games) ? games : [];
    const tList = Array.isArray(torrentGames) ? torrentGames : [];
    cachedCatalogPool = deduplicateGames(gList.concat(tList));
    lastGamesRef = games;
    lastTorrentsRef = torrentGames;
    return cachedCatalogPool;
  }

  // Base list of all deduplicated games (FTP + Torrents + Favorites)
  let baseAllGames = $derived.by(() => {
    const catalog = getDeduplicatedCatalog();
    if (normalizedFavorites.length === 0) return catalog;

    const existingKeys = new Set<string>();
    for (const g of catalog) {
      const k = getCanonicalKey(g);
      if (k) existingKeys.add(k);
    }

    const extraFavs: any[] = [];
    for (const f of normalizedFavorites) {
      if (!f) continue;
      const k = getCanonicalKey(f);
      if (k && !existingKeys.has(k)) {
        existingKeys.add(k);
        extraFavs.push(f);
      }
    }

    if (extraFavs.length === 0) return catalog;
    return [...extraFavs, ...catalog];
  });

  // Filtered games for the single bottom strip
  let filteredGames = $derived.by(() => {
    const all = baseAllGames;

    if (selectedFilter === 'installed') {
      const list = all.filter((g) => checkGameInstalled(g));
      const deduplicated: any[] = [];
      const seenKeys = new Set<string>();
      for (const g of list) {
        const key = getCanonicalKey(g);
        if (key && !seenKeys.has(key)) {
          seenKeys.add(key);
          deduplicated.push(g);
        }
      }
      if (deduplicated.length > 0) return deduplicated;

      // Fallback: synthesize directly from completed download history
      const histList: any[] = [];
      for (const h of downloadHistory || []) {
        if (h && (h.status === 'completed' || h.isInstalled)) {
          const gId = Number(h.gameId || 0);
          const sId = Number(h.steamAppId || 0);
          const title = cleanCanonicalKey(h.gameTitle || '');
          const key = sId ? `appid:${sId}` : (title ? `title:${title}` : `id:${gId}`);
          if (key && !seenKeys.has(key)) {
            seenKeys.add(key);
            const found = all.find(g => (gId && Number(g.id) === gId) || (sId && Number(g.steamAppId) === sId) || (title && cleanCanonicalKey(g.cleanTitle) === title));
            if (found) {
              histList.push(found);
            } else {
              histList.push({
                id: gId || Date.now(),
                cleanTitle: h.gameTitle || 'Установленная игра',
                rawName: h.gameTitle || '',
                isInstalled: true,
                sourceType: 'installed'
              });
            }
          }
        }
      }
      return histList;
    }

    if (selectedFilter === 'favorites') {
      const list = all.filter((g) => checkGameFavorite(g));
      const deduplicated: any[] = [];
      const seenKeys = new Set<string>();
      for (const g of list) {
        const key = getCanonicalKey(g);
        if (key && !seenKeys.has(key)) {
          seenKeys.add(key);
          deduplicated.push(g);
        }
      }
      if (deduplicated.length > 0) return deduplicated;

      for (const f of normalizedFavorites) {
        const key = getCanonicalKey(f);
        if (key && !seenKeys.has(key)) {
          seenKeys.add(key);
          deduplicated.push(f);
        }
      }
      return deduplicated;
    }

    if (selectedFilter === 'catalog') {
      const raw = (games || []).filter((g) => g && g.sourceType !== 'torrent');
      const deduplicated: any[] = [];
      const seenKeys = new Set<string>();
      for (const g of raw) {
        const key = getCanonicalKey(g);
        if (key && !seenKeys.has(key)) {
          seenKeys.add(key);
          deduplicated.push(g);
        }
      }
      return deduplicated;
    }

    if (selectedFilter === 'torrents') {
      const raw = (torrentGames || []).length > 0 ? torrentGames : all.filter((g) => g && g.sourceType === 'torrent');
      const deduplicated: any[] = [];
      const seenKeys = new Set<string>();
      for (const g of raw) {
        const key = getCanonicalKey(g);
        if (key && !seenKeys.has(key)) {
          seenKeys.add(key);
          deduplicated.push(g);
        }
      }
      return deduplicated;
    }

    // Default 'all': installed/downloading first, then favorites, then rest
    const installedList: any[] = [];
    const favoritesList: any[] = [];
    const othersList: any[] = [];
    const seenKeys = new Set<string>();

    for (const g of all) {
      const key = getCanonicalKey(g);
      if (key && !seenKeys.has(key) && checkGameInstalled(g)) {
        installedList.push(g);
        seenKeys.add(key);
      }
    }
    for (const g of all) {
      const key = getCanonicalKey(g);
      if (key && !seenKeys.has(key) && checkGameFavorite(g)) {
        favoritesList.push(g);
        seenKeys.add(key);
      }
    }
    for (const g of all) {
      const key = getCanonicalKey(g);
      if (key && !seenKeys.has(key)) {
        othersList.push(g);
        seenKeys.add(key);
      }
    }

    return [...installedList, ...favoritesList, ...othersList];
  });

  let installedCount = $derived.by(() => {
    const fromBase = baseAllGames.filter((g) => checkGameInstalled(g));
    const seen = new Set<string>();
    for (const g of fromBase) {
      const k = getCanonicalKey(g);
      if (k) seen.add(k);
    }
    return seen.size;
  });

  let favoriteCount = $derived.by(() => {
    const fromBase = baseAllGames.filter((g) => checkGameFavorite(g));
    const seen = new Set<string>();
    for (const g of fromBase) {
      const k = getCanonicalKey(g);
      if (k) seen.add(k);
    }
    return seen.size;
  });

  // Virtual sliding window derivations
  let windowStart = $derived(Math.max(0, focusedIndex - WINDOW_BEFORE));
  let windowEnd = $derived(Math.min(filteredGames.length, focusedIndex + WINDOW_AFTER + 1));
  let visibleWindow = $derived(filteredGames.slice(windowStart, windowEnd));
  let spacerLeftWidth = $derived(windowStart * CARD_STEP_PX);
  let spacerRightWidth = $derived((filteredGames.length - windowEnd) * CARD_STEP_PX);

  // Keep focusedIndex bounded when filter changes
  $effect(() => {
    if (focusedIndex >= filteredGames.length) {
      focusedIndex = Math.max(0, filteredGames.length - 1);
    }
  });

  // Current focused game and dynamic priority enrichment overrides
  let focusedGame = $derived(filteredGames[focusedIndex] || null);
  let enrichedOverrides = $state<Record<number, any>>({});
  let effectiveFocusedGame = $derived.by(() => {
    if (!focusedGame) return null;
    const ov = enrichedOverrides[focusedGame.id];
    if (ov) {
      return { ...focusedGame, ...ov };
    }
    return focusedGame;
  });

  function isPlaceholderTitle(title: string | undefined | null): boolean {
    if (!title) return true;
    const t = title.trim();
    return t === '' || /^Steam App \d+$/i.test(t);
  }

  function cleanTitleDisplay(g: any): string {
    if (!g) return '';
    if (g.steamTitle && !isPlaceholderTitle(g.steamTitle)) {
      return g.steamTitle.trim();
    }
    const raw = g.cleanTitle || g.rawName || '';
    return raw
      .replace(/^[\{\[\(]\s*(linux|win|windows|mac|macos|pc|gog|steam|portable|repack|native|unpack|unpacked)\s*[\}\]\)]\s*/gi, '')
      .replace(/\s*[\{\[\(]\s*(linux|win|windows|mac|macos|pc|gog|steam|portable|repack|native|unpack|unpacked)\s*[\}\]\)]$/gi, '')
      .replace(/\[(?:dl|p|l|repack|portable|rip)\]/gi, '')
      .replace(/\[(?:rus|eng|multi\d*).*?\]/gi, '')
      .replace(/\(.*?\)/g, (m: string) => (m.match(/\b(19\d\d|20\d\d|rts|rpg|action|fps|tps)\b/i) ? '' : m))
      .replace(/\s+/g, ' ')
      .trim() || raw;
  }

  let displayTitle = $derived(cleanTitleDisplay(effectiveFocusedGame));

  // Priority enrichment state
  let enrichingGameIds = new Set<number>();
  let isEnrichingCurrentGame = $state<boolean>(false);

  function triggerPriorityEnrichmentIfNeeded(g: any, curId: number, seq: number) {
    const hasRich = !!(
      (g.screenshots && g.screenshots.length > 0) ||
      (g.movies && g.movies.length > 0) ||
      g.shortDescription ||
      g.detailedDescription ||
      (g.genres && g.genres.length > 0)
    );
    if (hasRich) {
      isEnrichingCurrentGame = false;
      return;
    }

    if (enrichingGameIds.has(curId)) {
      isEnrichingCurrentGame = true;
      return;
    }

    enrichingGameIds.add(curId);
    isEnrichingCurrentGame = true;

    EnrichGameNow(curId)
      .then((enriched: any) => {
        enrichingGameIds.delete(curId);
        if (!isMounted || seq !== detailsSeq) return;
        isEnrichingCurrentGame = false;
        if (enriched) {
          enrichedOverrides[curId] = enriched;
          const entry = {
            movies: (enriched.movies && enriched.movies.length > 0) ? enriched.movies : (currentDetails?.movies || []),
            screenshots: (enriched.screenshots && enriched.screenshots.length > 0) ? enriched.screenshots : (currentDetails?.screenshots || [])
          };
          detailsCache.set(curId, entry);
          currentDetails = entry;
        }
      })
      .catch(() => {
        enrichingGameIds.delete(curId);
        if (!isMounted || seq !== detailsSeq) {
          isEnrichingCurrentGame = false;
        }
      });
  }

  // Cache for full game details (movies, screenshots) loaded on focus
  const detailsCache = new Map<number, { movies: SteamMovie[]; screenshots: string[] }>();
  let currentDetails = $state<{ movies: SteamMovie[]; screenshots: string[] } | null>(null);
  let detailsSeq = 0;
  let detailsTimer: any = null;

  $effect(() => {
    const curGame = focusedGame;
    if (detailsTimer) clearTimeout(detailsTimer);

    if (!curGame) {
      currentDetails = null;
      isEnrichingCurrentGame = false;
      return;
    }

    const curId = curGame.id;
    if (detailsCache.has(curId)) {
      currentDetails = detailsCache.get(curId)!;
      const cached = detailsCache.get(curId)!;
      const combined = { ...curGame, ...(enrichedOverrides[curId] || {}), ...cached };
      triggerPriorityEnrichmentIfNeeded(combined, curId, detailsSeq);
      return;
    }

    // Debounce fetching details by 140ms to allow smooth rapid scrolling
    detailsTimer = setTimeout(() => {
      const seq = ++detailsSeq;
      GetGameDetails(curId)
        .then(async (details: any) => {
          if (!isMounted || seq !== detailsSeq) return;
          let movies: SteamMovie[] = (details && details.movies) || [];
          const screenshots: string[] = (details && details.screenshots) || [];

          if (details) {
            enrichedOverrides[curId] = { ...curGame, ...details };
          }

          if ((!movies || movies.length === 0) && curGame.steamAppId > 0) {
            try {
              const fresh = await GetGameMovies(curGame.steamAppId);
              if (fresh && fresh.length > 0) {
                movies = fresh;
              }
            } catch {}
          }

          if (!isMounted || seq !== detailsSeq) return;
          const entry = { movies: movies || [], screenshots: screenshots || [] };
          detailsCache.set(curId, entry);
          currentDetails = entry;

          const combined = { ...curGame, ...(details || {}), ...entry };
          triggerPriorityEnrichmentIfNeeded(combined, curId, seq);
        })
        .catch((err) => {
          console.warn('Failed to load focused game details:', err);
          if (isMounted && seq === detailsSeq) {
            triggerPriorityEnrichmentIfNeeded(curGame, curId, seq);
          }
        });
    }, 140);

    return () => {
      if (detailsTimer) clearTimeout(detailsTimer);
    };
  });

  // Available movies/trailers for focused game
  let movieList = $derived.by<SteamMovie[]>(() => {
    const movies = currentDetails?.movies?.length ? currentDetails.movies : (effectiveFocusedGame?.movies || []);
    if (!Array.isArray(movies)) return [];
    return movies.filter((m: any) => m && (m.mp4 || m.webm || m.hls));
  });

  // Use sanitizeMediaUrl for consistent URL resolution
  function cleanVideoUrl(rawUrl: string): string {
    return sanitizeMediaUrl(rawUrl);
  }

  // Pick HLS (H.264 fMP4) for reliable hardware decoding, fallback to direct MP4/WebM
  function pickMovieSource(m: SteamMovie): string {
    if (!m) return '';
    if (m.hls) {
      return cleanVideoUrl(m.hls);
    }
    if (m.mp4 && !m.mp4.includes('/apps/') && !m.mp4.includes('movie_max.mp4')) {
      return cleanVideoUrl(m.mp4);
    }
    if (m.webm) {
      return cleanVideoUrl(m.webm);
    }
    if (m.mp4) {
      return cleanVideoUrl(m.mp4);
    }
    return '';
  }

  // Current active trailer
  let activeMovie = $derived.by(() => {
    if (movieList.length === 0) return null;
    const m = movieList[currentMovieIndex % movieList.length];
    if (!m) return null;
    const src = pickMovieSource(m);
    let thumb = cleanVideoUrl(m.thumbnail || '');
    if (thumb.startsWith('//')) {
      thumb = 'https:' + thumb;
    }
    return {
      ...m,
      src,
      thumbnail: thumb
    };
  });

  let hlsInstance: Hls | null = null;
  let lastLoadedSource = '';
  let playPromise: Promise<void> | null = null;

  function destroyHls() {
    if (hlsInstance) {
      try {
        hlsInstance.stopLoad();
        hlsInstance.detachMedia();
        hlsInstance.destroy();
      } catch (e) {
        console.warn('HLS destroy notice:', e);
      }
      hlsInstance = null;
    }
  }

  async function safePlay() {
    if (!videoBgEl) return;
    try {
      if (hlsInstance) {
        hlsInstance.startLoad();
      }
      videoBgEl.muted = isVideoMuted;
      const p = videoBgEl.play();
      if (p !== undefined) {
        playPromise = p;
        await p;
        playPromise = null;
        isVideoPlaying = true;
      }
    } catch (err: any) {
      playPromise = null;
      if (err?.name === 'AbortError') {
        return;
      }
      if (err?.name === 'NotAllowedError') {
        if (videoBgEl) {
          videoBgEl.muted = true;
          isVideoMuted = true;
          try {
            const p2 = videoBgEl.play();
            if (p2 !== undefined) {
              playPromise = p2;
              await p2;
              playPromise = null;
              isVideoPlaying = true;
            }
          } catch {
            playPromise = null;
          }
        }
        return;
      }
      console.warn('[Video SafePlay notice]', err);
    }
  }

  async function safePause() {
    if (!videoBgEl) return;
    if (playPromise) {
      try {
        await playPromise;
      } catch {}
      playPromise = null;
    }
    if (videoBgEl && !videoBgEl.paused) {
      videoBgEl.pause();
      isVideoPlaying = false;
    }
  }

  function loadMediaSource(targetUrl: string) {
    if (!videoBgEl) return;
    if (!targetUrl) {
      destroyHls();
      safePause();
      lastLoadedSource = '';
      return;
    }
    if (targetUrl === lastLoadedSource) {
      if (isAmbientTrailerActive && videoBgEl.paused) {
        safePlay();
      }
      return;
    }

    lastLoadedSource = targetUrl;
    videoHasRenderedFrame = false;
    destroyHls();

    const isHls = targetUrl.includes('.m3u8') || targetUrl.includes('hls_264');

    if (isHls && Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: false, // Prevents Blob WebWorker SecurityError in WebView2/Wails!
        lowLatencyMode: false,
        backBufferLength: 30,
        maxBufferLength: 30,
        xhrSetup: (xhr) => {
          xhr.withCredentials = false;
        }
      });
      hlsInstance = hls;
      hls.loadSource(targetUrl);
      hls.attachMedia(videoBgEl);

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        if (videoBgEl && isAmbientTrailerActive) {
          safePlay();
        }
      });

      hls.on(Hls.Events.ERROR, (_evt, data) => {
        if (data.fatal) {
          console.warn('[Ambient Hls Fatal]', data.type, data.details);
          switch (data.type) {
            case Hls.ErrorTypes.NETWORK_ERROR:
              hls.startLoad();
              break;
            case Hls.ErrorTypes.MEDIA_ERROR:
              hls.recoverMediaError();
              break;
            default:
              destroyHls();
              videoHasRenderedFrame = false;
              handleTrailerEnded();
              break;
          }
        }
      });
    } else {
      videoBgEl.src = targetUrl;
      try {
        videoBgEl.load();
      } catch {}
      if (isAmbientTrailerActive) {
        safePlay();
      }
    }
  }

  function stopAndUnloadVideo() {
    destroyHls();
    if (videoBgEl) {
      try {
        videoBgEl.pause();
        videoBgEl.removeAttribute('src');
        videoBgEl.load();
      } catch {}
    }
    lastLoadedSource = '';
    videoHasRenderedFrame = false;
    isVideoPlaying = false;
  }

  function stopVideoPlayback() {
    stopAndUnloadVideo();
  }

  let isAmbientTimerElapsed = $state<boolean>(false);
  let lastFocusedGameId: number | null = null;
  let theaterEnterTimestamp = 0;

  // Ambient trailer delay timer (auto-starts after 2.5s on game change)
  $effect(() => {
    const curGame = focusedGame;
    const curId = curGame ? Number(curGame.id) : null;

    // Do nothing if it's still the exact same game!
    // This prevents re-running when favorites, download history, or details resolve asynchronously.
    if (curId === lastFocusedGameId) {
      return;
    }
    lastFocusedGameId = curId;

    if (trailerTimer) clearTimeout(trailerTimer);
    isAmbientTimerElapsed = false;
    isAmbientTrailerActive = false;
    isTheaterMode = false;
    isVideoMuted = true;
    currentMovieIndex = 0;
    activeScreenshotPreview = null;
    stopVideoPlayback();

    if (curGame) {
      trailerTimer = setTimeout(() => {
        if (isMounted && focusedGame?.id === curGame.id) {
          isAmbientTimerElapsed = true;
        }
      }, AMBIENT_TRAILER_DELAY_MS);
    }

    return () => {
      if (trailerTimer) clearTimeout(trailerTimer);
    };
  });

  $effect(() => {
    if (isAmbientTimerElapsed && movieList.length > 0 && !isAmbientTrailerActive) {
      isAmbientTrailerActive = true;
    }
  });

  $effect(() => {
    const curMovie = activeMovie;
    const isAmbient = isAmbientTrailerActive;

    if (!curMovie?.src || !isAmbient) {
      stopVideoPlayback();
      return;
    }

    loadMediaSource(curMovie.src);
  });

  $effect(() => {
    if (videoBgEl) {
      videoBgEl.muted = isVideoMuted;
    }
  });

  // Live status for focused game
  let isFocusedInstalled = $derived(effectiveFocusedGame ? checkGameInstalled(effectiveFocusedGame) : false);
  let focusedDownload = $derived(effectiveFocusedGame ? activeDownloadingMap.get(Number(effectiveFocusedGame.id)) : null);
  let isFocusedFavorite = $derived(effectiveFocusedGame ? checkGameFavorite(effectiveFocusedGame) : false);

  // Still artwork background
  let currentBackdropUrl = $derived.by(() => {
    if (activeScreenshotPreview) return activeScreenshotPreview;
    if (!effectiveFocusedGame) return '';
    const candidates = [
      effectiveFocusedGame.backgroundImage,
      effectiveFocusedGame.headerImage,
      effectiveFocusedGame.capsuleImage,
      ...(Array.isArray(effectiveFocusedGame.screenshots) && effectiveFocusedGame.screenshots.length > 0 ? [effectiveFocusedGame.screenshots[0]] : [])
    ];
    for (const raw of candidates) {
      if (!raw || typeof raw !== 'string') continue;
      const url = sanitizeMediaUrl(raw);
      if (url && !brokenCovers[url]) {
        return url;
      }
    }
    return '';
  });

  // Top screenshots
  let gameScreenshots = $derived.by<string[]>(() => {
    const sc = currentDetails?.screenshots?.length ? currentDetails.screenshots : (effectiveFocusedGame?.screenshots || []);
    if (!Array.isArray(sc)) return [];
    return sc.filter((s: string) => typeof s === 'string' && s.trim() !== '').slice(0, 8);
  });

  function enterTheaterMode() {
    if (movieList.length === 0) return;
    theaterEnterTimestamp = Date.now();
    sound.playSelect();
    if (trailerTimer) clearTimeout(trailerTimer);
    isAmbientTimerElapsed = true;
    isAmbientTrailerActive = true;
    isTheaterMode = true;
    isVideoMuted = false;
    if (videoBgEl) {
      videoBgEl.muted = false;
    }
    safePlay();
    window.dispatchEvent(new CustomEvent('app:theater-mode', { detail: { active: true } }));
  }

  function exitTheaterMode() {
    // Guard against immediate analog stick snap-back within 350ms
    if (Date.now() - theaterEnterTimestamp < 350) {
      return;
    }
    sound.playBack();
    isTheaterMode = false;
    isVideoMuted = true;
    if (videoBgEl) {
      videoBgEl.muted = true;
    }
    window.dispatchEvent(new CustomEvent('app:theater-mode', { detail: { active: false } }));
  }

  function togglePlayPause() {
    if (!videoBgEl) return;
    if (videoBgEl.paused) {
      videoBgEl.play().catch(() => {});
      isVideoPlaying = true;
    } else {
      videoBgEl.pause();
      isVideoPlaying = false;
    }
  }

  function toggleMute() {
    isVideoMuted = !isVideoMuted;
    if (videoBgEl) {
      videoBgEl.muted = isVideoMuted;
    }
  }

  // Loop to next trailer when current one finishes
  function handleTrailerEnded() {
    if (movieList.length <= 1) {
      if (videoBgEl) {
        videoBgEl.currentTime = 0;
        videoBgEl.play().catch(() => {});
      }
      return;
    }
    currentMovieIndex = (currentMovieIndex + 1) % movieList.length;
  }

  function nextTrailer() {
    if (movieList.length <= 1) return;
    sound.playMove();
    currentMovieIndex = (currentMovieIndex + 1) % movieList.length;
  }

  function prevTrailer() {
    if (movieList.length <= 1) return;
    sound.playMove();
    currentMovieIndex = (currentMovieIndex - 1 + movieList.length) % movieList.length;
  }

  function switchFilter(filter: 'all' | 'installed' | 'favorites' | 'catalog' | 'torrents') {
    if (selectedFilter === filter) return;
    sound.playTab();
    selectedFilter = filter;
    focusedIndex = 0;
    if (stripContainerEl) {
      stripContainerEl.scrollLeft = 0;
    }
    requestAnimationFrame(() => {
      scrollToCard(0);
    });
  }

  function moveFocus(delta: number) {
    if (filteredGames.length === 0) return;
    sound.playMove();
    const next = Math.max(0, Math.min(filteredGames.length - 1, focusedIndex + delta));
    if (next !== focusedIndex) {
      // Immediately stop video playback so card navigation runs at locked 60/120 FPS
      stopVideoPlayback();
      isAmbientTrailerActive = false;
      isAmbientTimerElapsed = false;
      if (trailerTimer) clearTimeout(trailerTimer);

      focusedIndex = next;
      scrollToCard(next);
    }
  }

  function setFocus(index: number) {
    if (index >= 0 && index < filteredGames.length) {
      if (focusedIndex !== index) {
        sound.playMove();
        stopVideoPlayback();
        isAmbientTrailerActive = false;
        isAmbientTimerElapsed = false;
        if (trailerTimer) clearTimeout(trailerTimer);

        focusedIndex = index;
      }
      scrollToCard(index);
    }
  }

  function scrollToCard(index: number) {
    requestAnimationFrame(() => {
      const card = cardRefs[index];
      if (card) {
        card.scrollIntoView({ behavior: 'auto', block: 'nearest', inline: 'center' });
      }
    });
  }

  async function handlePrimaryAction() {
    if (!effectiveFocusedGame) return;
    sound.playSelect();
    if (isFocusedInstalled) {
      await LaunchGameByGameID(effectiveFocusedGame.id);
    } else if (focusedDownload) {
      onGoToDownloads();
    } else {
      onSelectGame(effectiveFocusedGame);
    }
  }

  async function handleSecondaryAction() {
    if (!effectiveFocusedGame) return;
    sound.playSelect();
    if (isFocusedInstalled) {
      await OpenGameFolder(effectiveFocusedGame.id);
    } else {
      onStartDownload(effectiveFocusedGame.id, downloadPath);
    }
  }

  async function handleToggleFavorite() {
    if (!effectiveFocusedGame) return;
    sound.playFocus();
    const isFav = checkGameFavorite(effectiveFocusedGame);
    const targetId = Number(effectiveFocusedGame.id);
    if (isFav) {
      await RemoveFromFavorites(targetId);
      favorites = (favorites || []).filter((f: any) => {
        const fId = Number(f.gameId || f.id || f.game?.id);
        const fTitle = (f.cleanTitle || f.game?.cleanTitle || '').toLowerCase().trim();
        const curTitle = (displayTitle || '').toLowerCase().trim();
        return fId !== targetId && (fTitle === '' || curTitle === '' || fTitle !== curTitle);
      });
    } else {
      await SetFavoriteStatus(targetId, 'favorite');
      favorites = [
        ...(favorites || []),
        {
          gameId: targetId,
          status: 'favorite',
          game: effectiveFocusedGame
        }
      ];
    }
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

  function openLightbox(index: number) {
    sound.playSelect();
    lightboxIndex = index;
    lightboxImage = gameScreenshots[index] || null;
  }

  function closeLightbox() {
    sound.playBack();
    lightboxImage = null;
  }

  function prevLightboxImage() {
    if (gameScreenshots.length === 0) return;
    sound.playMove();
    lightboxIndex = (lightboxIndex - 1 + gameScreenshots.length) % gameScreenshots.length;
    lightboxImage = gameScreenshots[lightboxIndex];
  }

  function nextLightboxImage() {
    if (gameScreenshots.length === 0) return;
    sound.playMove();
    lightboxIndex = (lightboxIndex + 1) % gameScreenshots.length;
    lightboxImage = gameScreenshots[lightboxIndex];
  }
</script>

<div class="w-full h-full flex flex-col relative overflow-hidden bg-[#07080a] select-none text-white">

  <!-- 1. Fullscreen Cinematic Background Layer: Still Art + Live Ambient Trailer Video -->
  <div class="{isTheaterMode ? 'fixed inset-0 z-[90] w-screen h-screen bg-black pointer-events-auto' : 'absolute inset-0 z-0 pointer-events-none'} overflow-hidden">
    <!-- Static Backdrop Image — always shown as fallback, fades slightly when video is rendering -->
    {#if currentBackdropUrl}
      <img
        src={currentBackdropUrl}
        alt=""
        class="w-full h-full object-cover object-center transition-opacity duration-700 ease-out {isTheaterMode ? 'opacity-0 pointer-events-none' : (videoHasRenderedFrame ? 'opacity-20' : 'opacity-60')}"
        decoding="async"
      />
    {/if}

    <!-- Live Fullscreen Background Video Trailer (Always mounted for instant zero-lag playback) -->
    <video
      bind:this={videoBgEl}
      poster={activeMovie?.thumbnail ? sanitizeMediaUrl(activeMovie.thumbnail) : ''}
      class="absolute inset-0 w-full h-full object-cover transition-opacity duration-700 ease-out {isTheaterMode || (isAmbientTrailerActive && activeMovie?.src && videoHasRenderedFrame) ? 'opacity-100' : 'opacity-0 pointer-events-none'}"
      muted={isVideoMuted}
      playsinline
      preload="auto"
      onplaying={() => {
        isVideoPlaying = true;
        videoHasRenderedFrame = true;
      }}
      ontimeupdate={() => {
        if (videoBgEl && videoBgEl.currentTime > 0) {
          videoHasRenderedFrame = true;
        }
      }}
      onerror={() => {
        videoHasRenderedFrame = false;
        isVideoPlaying = false;
      }}
      onended={handleTrailerEnded}
    ></video>

    <!-- Ambient vignette & falloff gradients -->
    <div class="absolute inset-0 bg-gradient-to-t from-[#07080a] via-[#07080a]/70 to-transparent transition-opacity duration-500 {isTheaterMode ? 'opacity-0 pointer-events-none' : 'opacity-100'}"></div>
    <div class="absolute inset-0 bg-gradient-to-r from-[#07080a]/95 via-[#07080a]/50 to-transparent transition-opacity duration-500 {isTheaterMode ? 'opacity-0 pointer-events-none' : 'opacity-100'}"></div>
  </div>

  <!-- 2. Upper Dashboard Hero Stage (Smoothly fades out in Theater Mode) -->
  <div
    data-nav-zone="detail"
    class="relative z-10 flex-1 min-h-0 flex flex-col justify-end px-8 sm:px-12 lg:px-16 pt-6 pb-6 overflow-hidden transition-all duration-500 ease-out {isTheaterMode ? 'opacity-0 pointer-events-none -translate-y-4' : 'opacity-100 translate-y-0'}"
  >
    {#if effectiveFocusedGame}
      <div class="w-full flex flex-col lg:flex-row items-end justify-between gap-8 lg:gap-14">

        <!-- Left Hero: Official Game Logo, Badges, Synopsis & Tactile Action Buttons -->
        <div class="flex-1 min-w-0 flex flex-col justify-end space-y-4 max-w-2xl">
          
          <!-- Background Priority Search & Enriching Status Pill -->
          {#if isEnrichingCurrentGame}
            <div class="inline-flex items-center gap-2 px-2.5 py-1 rounded bg-white/[0.06] border border-white/10 text-sky-400 text-xs font-medium w-fit">
              <RefreshCw class="w-3.5 h-3.5 animate-spin flex-shrink-0" />
              <span>Очистка названия и поиск данных об игре...</span>
            </div>
          {/if}

          <!-- Official Steam Logo or Bold Title -->
          <div class="min-h-[90px] sm:min-h-[120px] flex items-end">
            {#if effectiveFocusedGame.steamAppId && !logoFailedMap[effectiveFocusedGame.steamAppId]}
              <img
                src="https://shared.steamstatic.com/store_item_assets/steam/apps/{effectiveFocusedGame.steamAppId}/logo.png"
                alt={displayTitle}
                class="max-h-24 sm:max-h-32 md:max-h-36 max-w-[320px] sm:max-w-lg object-contain object-left-bottom filter drop-shadow-[0_12px_24px_rgba(0,0,0,0.8)] transition-all duration-700 ease-out"
                onerror={() => {
                  logoFailedMap[effectiveFocusedGame.steamAppId] = true;
                }}
              />
            {:else}
              <h1 class="text-3xl sm:text-4xl md:text-5xl font-black text-white tracking-tight leading-none drop-shadow-2xl line-clamp-2 transition-all duration-500">
                {displayTitle}
              </h1>
            {/if}
          </div>

          <!-- Console Meta Chips Line -->
          <div class="flex flex-wrap items-center gap-2.5">
            <span class="px-2 py-0.5 rounded bg-white/10 text-white text-[10px] font-black tracking-widest border border-white/20 uppercase">
              PC
            </span>

            {#if isFocusedInstalled}
              <span class="px-2.5 py-0.5 rounded-md bg-white/[0.06] border border-white/10 text-emerald-400 text-xs font-semibold flex items-center gap-1.5">
                <Check class="w-3.5 h-3.5 stroke-[3]" />
                <span>Установлено</span>
              </span>
            {:else if focusedDownload}
              <span class="px-2.5 py-0.5 rounded-md bg-white/[0.06] border border-white/10 text-sky-400 text-xs font-semibold flex items-center gap-1.5">
                <Download class="w-3.5 h-3.5 stroke-[2.5]" />
                <span>{focusedDownload.progressPercent || 0}%</span>
              </span>
            {/if}

            {#if isFocusedFavorite}
              <span class="px-2.5 py-0.5 rounded-sm bg-white/[0.06] border border-white/10 text-amber-400 text-xs font-semibold flex items-center gap-1.5">
                <Star class="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
                <span>Избранное</span>
              </span>
            {/if}

            <span class="px-2.5 py-0.5 rounded-sm border border-white/10 bg-white/[0.06] text-[#cbd5e1] text-xs font-semibold flex items-center gap-1.5">
              <span>Источник: {effectiveFocusedGame.sourceType === 'torrent' || effectiveFocusedGame.magnetUri ? `Торрент (${formatTorrentSourceName(effectiveFocusedGame.torrentSource) || 'Каталог'})` : 'FTP-сервер'}</span>
            </span>

            {#if effectiveFocusedGame.releaseDate}
              <span class="text-xs font-semibold text-[#8e95a2]">
                {effectiveFocusedGame.releaseDate}
              </span>
              <span class="text-white/20">•</span>
            {/if}

            {#if effectiveFocusedGame.sizeDisplay}
              <span class="text-xs font-semibold text-[#8e95a2]">
                {effectiveFocusedGame.sizeDisplay}
              </span>
              <span class="text-white/20">•</span>
            {/if}

            {#if effectiveFocusedGame.reviewPercent}
              <span class="text-xs font-bold text-white flex items-center gap-1">
                <Star class="w-3.5 h-3.5 text-amber-400 fill-amber-400" />
                <span>{effectiveFocusedGame.reviewPercent}%</span>
              </span>
              <span class="text-white/20">•</span>
            {/if}

            {#if effectiveFocusedGame.genres && Array.isArray(effectiveFocusedGame.genres)}
              <span class="text-xs text-[#8e95a2]">
                {effectiveFocusedGame.genres.slice(0, 2).join(', ')}
              </span>
            {/if}
          </div>

          <!-- Synopsis / Short Description -->
          {#if effectiveFocusedGame.shortDescription}
            <p class="text-xs sm:text-sm text-[#cbd5e1] leading-relaxed line-clamp-2 max-w-xl font-normal drop-shadow-sm transition-all duration-700 ease-out">
              {effectiveFocusedGame.shortDescription}
            </p>
          {/if}

          <!-- Tactical Action Buttons -->
          <div class="flex items-center gap-3 pt-2">
            <!-- Primary Action Pill (A) -->
            <button
              data-nav-item
              type="button"
              class="px-7 py-3 rounded bg-white text-black font-extrabold text-sm flex items-center gap-3 hover:bg-slate-100 transition-all cursor-pointer shadow-2xl active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
              onclick={handlePrimaryAction}
            >
              {#if isFocusedInstalled}
                <Play class="w-4 h-4 fill-black text-black" />
                <span>Играть</span>
              {:else if focusedDownload}
                <Download class="w-4 h-4 stroke-[2.5]" />
                <span>В загрузки</span>
              {:else}
                <Info class="w-4 h-4 fill-black text-black" />
                <span>Страница игры</span>
              {/if}
              <span class="w-5 h-5 rounded-full bg-black/15 text-black text-[10px] font-black flex items-center justify-center">A</span>
            </button>

            <!-- Secondary Action (Folder / Download) (X) -->
            <button
              data-nav-item
              type="button"
              class="px-4 py-3 rounded bg-white/10 hover:bg-white/20 text-white font-bold text-xs flex items-center gap-2 border border-white/15 transition-all cursor-pointer active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
              onclick={handleSecondaryAction}
              title={isFocusedInstalled ? 'Открыть папку с игрой' : 'Начать скачивание игры'}
            >
              {#if isFocusedInstalled}
                <Folder class="w-4 h-4 fill-current text-[#cbd5e1]" />
                <span>Папка</span>
              {:else}
                <Download class="w-4 h-4 stroke-[2.5] text-[#cbd5e1]" />
                <span>Скачать</span>
              {/if}
              <span class="w-4 h-4 rounded-full bg-white/15 text-[#cbd5e1] text-[9px] font-bold flex items-center justify-center">X</span>
            </button>

            <!-- Favorite Toggle (Y) -->
            <button
              data-nav-item
              type="button"
              class="p-3 rounded bg-white/10 hover:bg-white/20 text-white border border-white/15 transition-all cursor-pointer active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
              onclick={handleToggleFavorite}
              title={isFocusedFavorite ? 'В избранном' : 'Добавить в избранное'}
            >
              <Star class="w-4 h-4 {isFocusedFavorite ? 'text-amber-400 fill-amber-400' : 'text-white/40 fill-white/20'}" />
            </button>

            <!-- Watch Trailer Fullscreen Button -->
            {#if movieList.length > 0}
              <button
                data-nav-item
                type="button"
                class="px-4 py-3 rounded bg-white/10 hover:bg-white/20 text-white font-semibold text-xs flex items-center gap-2 border border-white/15 transition-all cursor-pointer active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
                onclick={enterTheaterMode}
                title="Смотреть трейлер во весь экран [↑]"
              >
                <Volume2 class="w-4 h-4 fill-current text-sky-400" />
                <span>Трейлер</span>
                <span class="px-1.5 py-0.5 rounded bg-white/15 text-[10px] font-mono">↑</span>
              </button>
            {/if}

            <!-- More Options -->
            <button
              data-nav-item
              type="button"
              class="p-3 rounded bg-white/10 hover:bg-white/20 text-[#cbd5e1] hover:text-white border border-white/15 transition-all cursor-pointer active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
              onclick={() => onSelectGame(effectiveFocusedGame)}
              title="Все свойства и настройки"
            >
              <MoreHorizontal class="w-4 h-4" />
            </button>
          </div>

        </div>

        <!-- Right Side: Screenshots Carousel Strip -->
        {#if gameScreenshots.length > 0}
          <div class="w-full lg:w-[460px] xl:w-[500px] flex-shrink-0 flex flex-col justify-end">
            <div class="flex items-center gap-2.5 overflow-x-auto scrollbar-none py-1">
              {#each gameScreenshots as sc, idx}
                <button
                  data-nav-item
                  type="button"
                  class="w-20 sm:w-24 aspect-video rounded-sm overflow-hidden border border-white/15 bg-black/50 hover:border-white hover:scale-105 focus:border-white focus:scale-105 focus:outline-none transition-all flex-shrink-0 cursor-pointer relative group/thumb shadow-lg"
                  onclick={() => openLightbox(idx)}
                  onmouseenter={() => {
                    activeScreenshotPreview = sc;
                  }}
                  onfocus={() => {
                    activeScreenshotPreview = sc;
                  }}
                  title="Открыть скриншот"
                >
                  <img src={sc} alt="" class="w-full h-full object-cover" />
                  <div class="absolute inset-0 bg-white/15 opacity-0 group-hover/thumb:opacity-100 transition-opacity"></div>
                </button>
              {/each}
            </div>
          </div>
        {/if}

      </div>
    {:else}
      <div class="w-full h-full flex items-center justify-center text-[#64748b] text-sm">
        <span>Нет доступных игр</span>
      </div>
    {/if}
  </div>

  <!-- 3. Bottom Docked Horizontal Game Strip with Category Filters (Smoothly fades out in Theater Mode) -->
  <div
    data-nav-zone="grid"
    class="relative z-20 w-full flex-shrink-0 bg-[#07080a] border-t border-white/[0.08] pt-2 pb-3 transition-opacity duration-300 ease-out {isTheaterMode ? 'opacity-0 pointer-events-none translate-y-6' : 'opacity-100 translate-y-0'}"
  >
    
    <!-- Filter Switcher Bar -->
    <div class="px-8 sm:px-12 lg:px-16 flex items-center justify-between pb-2">
      <div class="flex items-center gap-1.5 sm:gap-2">
        <button
          data-nav-item
          type="button"
          class="px-3 py-1 rounded text-xs font-semibold transition-colors cursor-pointer focus:outline-none {selectedFilter === 'all' ? 'bg-white/20 text-white border border-white/30' : 'text-[#8e95a2] hover:text-white border border-transparent'}"
          onclick={() => {
            sound.playTab();
            selectedFilter = 'all';
          }}
        >
          Все ({baseAllGames.length})
        </button>

        <button
          data-nav-item
          type="button"
          class="px-3 py-1 rounded text-xs font-semibold transition-colors cursor-pointer focus:outline-none {selectedFilter === 'installed' ? 'bg-white/20 text-white border border-white/30' : 'text-[#8e95a2] hover:text-white border border-transparent'}"
          onclick={() => {
            sound.playTab();
            selectedFilter = 'installed';
          }}
        >
          Установленные ({installedCount})
        </button>

        <button
          data-nav-item
          type="button"
          class="px-3 py-1 rounded text-xs font-semibold transition-colors cursor-pointer focus:outline-none {selectedFilter === 'favorites' ? 'bg-white/20 text-white border border-white/30' : 'text-[#8e95a2] hover:text-white border border-transparent'}"
          onclick={() => {
            sound.playTab();
            selectedFilter = 'favorites';
          }}
        >
          Избранное ({favoriteCount})
        </button>

        {#if hasFtpServers}
          <button
            data-nav-item
            type="button"
            class="px-3 py-1 rounded text-xs font-semibold transition-colors cursor-pointer focus:outline-none {selectedFilter === 'catalog' ? 'bg-white/20 text-white border border-white/30' : 'text-[#8e95a2] hover:text-white border border-transparent'}"
            onclick={() => switchFilter('catalog')}
          >
            Каталог (FTP)
          </button>
        {/if}

        {#if hasTorrentSources}
          <button
            data-nav-item
            type="button"
            class="px-3 py-1 rounded text-xs font-semibold transition-colors cursor-pointer focus:outline-none {selectedFilter === 'torrents' ? 'bg-white/20 text-white border border-white/30' : 'text-[#8e95a2] hover:text-white border border-transparent'}"
            onclick={() => switchFilter('torrents')}
          >
            Торренты
          </button>
        {/if}
      </div>

      <div class="hidden sm:flex items-center gap-3 text-xs text-[#8e95a2]">
        {#if movieList.length > 0}
          <span class="inline-flex items-center gap-1.5 text-sky-400 font-medium">
            <Volume2 class="w-3.5 h-3.5" />
            <span>[↑] Трейлер</span>
          </span>
          <span class="text-white/20">•</span>
        {/if}
        <span>← → Выбор игры</span>
      </div>
    </div>

    <!-- Single Horizontal Carousel Strip with Virtual Sliding Window -->
    <div
      bind:this={stripContainerEl}
      class="flex items-center gap-4 overflow-x-auto px-8 sm:px-12 lg:px-16 pt-8 pb-8 scrollbar-none"
    >
      {#if filteredGames.length === 0}
        <div class="w-full py-8 text-center text-sm text-[#8e95a2]">
          Нет доступных игр в этом разделе
        </div>
      {:else}
        {#if spacerLeftWidth > 0}
          <div style="width: ${spacerLeftWidth}px; flex-shrink: 0;" aria-hidden="true"></div>
        {/if}

        {#each visibleWindow as rawG, vIdx (rawG?.id ? `${rawG.sourceType || 'g'}_${rawG.id}` : windowStart + vIdx)}
          {@const g = (rawG?.id && enrichedOverrides[rawG.id]) ? { ...rawG, ...enrichedOverrides[rawG.id] } : rawG}
          {@const idx = windowStart + vIdx}
          {@const isInst = checkGameInstalled(g)}
          {@const dl = activeDownloadingMap.get(Number(g.id))}
          {@const isFav = checkGameFavorite(g)}
          {@const isCardFocused = idx === focusedIndex}
          {@const coverUrl = getGameCover(g)}
          {@const categoryBorderClass = 
            isCardFocused
              ? (dl ? 'border border-white ring-1 ring-sky-400' : isInst ? 'border border-white ring-1 ring-emerald-500' : isFav ? 'border border-white ring-1 ring-amber-400' : 'border border-white ring-1 ring-white/40')
              : (dl ? 'border border-sky-400/80' : isInst ? 'border border-emerald-500/80' : isFav ? 'border border-amber-400/80' : 'border border-white/10')
          }

          <button
            bind:this={cardRefs[idx]}
            data-nav-item
            type="button"
            class="my-2 w-28 sm:w-32 md:w-36 aspect-[3/4] rounded overflow-hidden relative flex-shrink-0 cursor-pointer transition-all duration-150 text-left focus:outline-none {categoryBorderClass} {isCardFocused ? 'scale-105 shadow-2xl z-20 opacity-100' : 'opacity-75 hover:opacity-100 hover:scale-[1.02] bg-[#0d1117]'}"
            onclick={() => {
              sound.playSelect();
              if (focusedIndex !== idx) {
                setFocus(idx);
              }
              onSelectGame(g);
            }}
            onfocus={() => setFocus(idx)}
            onmouseenter={() => {
              if (focusedIndex !== idx) {
                setFocus(idx);
              }
            }}
          >
            <!-- Cover Poster with Automatic Error Fallback -->
            {#if coverUrl}
              <img
                src={coverUrl}
                alt={g.cleanTitle}
                class="w-full h-full object-cover"
                loading="lazy"
                decoding="async"
                onerror={() => {
                  brokenCovers[coverUrl] = true;
                }}
              />
            {:else}
              <div class="w-full h-full flex flex-col items-center justify-between p-3 bg-gradient-to-b from-[#181d28] via-[#10141d] to-[#07080a] text-center">
                <div class="w-full flex justify-end">
                  <Gamepad2 class="w-3.5 h-3.5 text-white/20" />
                </div>
                <div class="my-auto flex flex-col items-center gap-1.5 px-1">
                  <div class="w-9 h-9 rounded-sm bg-white/[0.06] border border-white/10 flex items-center justify-center text-xs font-black text-white/80">
                    {getInitials(g.cleanTitle)}
                  </div>
                  <span class="text-[11px] font-bold text-white line-clamp-2 leading-snug">{cleanTitleDisplay(g.cleanTitle)}</span>
                </div>
                <div class="h-2"></div>
              </div>
            {/if}

            <!-- Downloading bottom progress bar -->
            {#if dl && dl.progressPercent !== undefined}
              <div class="absolute bottom-0 inset-x-0 h-1 bg-black/60 pointer-events-none">
                <div class="h-full bg-sky-400" style="width: {dl.progressPercent}%"></div>
              </div>
            {/if}
          </button>
        {/each}

        {#if spacerRightWidth > 0}
          <div style="width: ${spacerRightWidth}px; flex-shrink: 0;" aria-hidden="true"></div>
        {/if}
      {/if}
    </div>

  </div>

  <!-- 4. Theater Mode Overlay HUD (Clean cinematic player, zero distracting badges) -->
  {#if isTheaterMode}
    <div
      data-nav-zone="modal"
      class="fixed inset-0 z-[100] flex flex-col justify-between p-8 pointer-events-none select-none"
      role="presentation"
    >
      <!-- Top Bar: Title & Back Button -->
      <div class="flex items-center justify-between pointer-events-none w-full">
        <div class="pointer-events-auto flex items-center gap-2.5 px-4 py-2.5 rounded bg-[#07080a]/95 text-white text-xs font-bold border border-white/15 shadow-2xl max-w-lg">
          <Film class="w-4 h-4 text-sky-400 flex-shrink-0" />
          <span class="truncate">{activeMovie?.name || 'Трейлер'}</span>
          {#if movieList.length > 1}
            <span class="text-[11px] font-mono text-[#8e95a2] flex-shrink-0">({currentMovieIndex + 1} / {movieList.length})</span>
          {/if}
        </div>

        <button
          data-nav-item
          type="button"
          class="pointer-events-auto px-4 py-2 rounded bg-black/85 hover:bg-black text-white/90 hover:text-white text-xs font-semibold flex items-center gap-2.5 border border-white/15 cursor-pointer transition-all shadow-2xl active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
          onclick={(e) => {
            e.stopPropagation();
            exitTheaterMode();
          }}
          title="Вернуться [Esc] или [B]"
        >
          <ArrowDown class="w-3.5 h-3.5" />
          <span>Назад</span>
          <span class="w-4 h-4 rounded bg-white/20 text-[10px] flex items-center justify-center font-bold">B</span>
        </button>
      </div>

      <!-- Bottom: Subtle unobtrusive bar -->
      <div class="flex items-center justify-between pointer-events-none w-full" role="presentation">
        <div class="pointer-events-auto flex items-center gap-2.5 bg-[#07080a]/95 p-2 rounded border border-white/15 shadow-2xl">
          <button
            data-nav-item
            type="button"
            class="p-2.5 rounded bg-white/10 hover:bg-white/20 text-white border border-white/15 cursor-pointer transition-all active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
            onclick={togglePlayPause}
            title="Пауза / Воспроизведение [Пробел]"
          >
            {#if isVideoPlaying}
              <Pause class="w-4 h-4 fill-current" />
            {:else}
              <Play class="w-4 h-4 fill-current" />
            {/if}
          </button>

          <button
            data-nav-item
            type="button"
            class="p-2.5 rounded bg-white/10 hover:bg-white/20 text-white border border-white/15 cursor-pointer transition-all active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
            onclick={toggleMute}
            title="Звук [M]"
          >
            {#if isVideoMuted}
              <VolumeX class="w-4 h-4 text-white/60" />
            {:else}
              <Volume2 class="w-4 h-4 text-white" />
            {/if}
          </button>

          {#if movieList.length > 1}
            <div class="h-4 w-[1px] bg-white/20 mx-0.5"></div>
            <button
              data-nav-item
              type="button"
              class="p-2.5 rounded bg-white/10 hover:bg-white/20 text-white border border-white/15 cursor-pointer transition-all active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
              onclick={prevTrailer}
              title="Предыдущий [←]"
            >
              <ChevronLeft class="w-4 h-4" />
            </button>
            <button
              data-nav-item
              type="button"
              class="p-2.5 rounded bg-white/10 hover:bg-white/20 text-white border border-white/15 cursor-pointer transition-all active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
              onclick={nextTrailer}
              title="Следующий [→]"
            >
              <ChevronRight class="w-4 h-4" />
            </button>
          {/if}
        </div>
      </div>
    </div>
  {/if}

  <!-- 5. Fullscreen Screenshot Lightbox Modal -->
  {#if lightboxImage}
    <div
      data-nav-zone="modal"
      class="fixed inset-0 z-50 bg-black/95 flex flex-col items-center justify-center p-4 select-none"
      onclick={closeLightbox}
      role="button"
      tabindex="-1"
      onkeydown={(e) => {
        if (e.key === 'Escape') closeLightbox();
      }}
    >
      <!-- Close Button -->
      <button
        data-nav-item
        type="button"
        class="absolute top-6 right-6 p-2.5 rounded bg-white/10 hover:bg-white/20 text-white cursor-pointer transition-colors z-20 focus:ring-1 focus:ring-white/40 focus:outline-none"
        onclick={(e) => {
          e.stopPropagation();
          closeLightbox();
        }}
        title="Закрыть [B]"
      >
        <X class="w-6 h-6" />
      </button>

      <!-- Navigation Arrows -->
      {#if gameScreenshots.length > 1}
        <button
          data-nav-item
          type="button"
          class="absolute left-6 top-1/2 -translate-y-1/2 p-3.5 rounded bg-white/10 hover:bg-white/20 text-white cursor-pointer transition-colors z-20 focus:ring-1 focus:ring-white/40 focus:outline-none"
          onclick={(e) => {
            e.stopPropagation();
            prevLightboxImage();
          }}
          title="Предыдущий [←]"
        >
          <ChevronLeft class="w-6 h-6" />
        </button>

        <button
          data-nav-item
          type="button"
          class="absolute right-6 top-1/2 -translate-y-1/2 p-3.5 rounded bg-white/10 hover:bg-white/20 text-white cursor-pointer transition-colors z-20 focus:ring-1 focus:ring-white/40 focus:outline-none"
          onclick={(e) => {
            e.stopPropagation();
            nextLightboxImage();
          }}
          title="Следующий [→]"
        >
          <ChevronRight class="w-6 h-6" />
        </button>
      {/if}

      <!-- Main Fullscreen Image -->
      <div
        class="max-w-6xl max-h-[85vh] rounded-md overflow-hidden border border-white/20 shadow-2xl bg-black"
        onclick={(e) => e.stopPropagation()}
        role="presentation"
      >
        <img
          src={lightboxImage}
          alt="Fullscreen Screenshot"
          class="w-full h-full object-contain max-h-[85vh]"
        />
      </div>

      <!-- Counter -->
      <div class="absolute bottom-6 px-3.5 py-1 rounded-sm bg-white/10 text-xs font-mono text-[#cbd5e1] border border-white/15">
        {lightboxIndex + 1} / {gameScreenshots.length}
      </div>
    </div>
  {/if}

</div>
