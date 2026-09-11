<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    ArrowLeft,
    Download,
    Play,
    Calendar,
    Building2,
    Tag,
    Star,
    Gamepad2,
    Monitor,
    Check,
    Folder,
    FolderOpen,
    Edit3,
    Search,
    RefreshCw,
    X,
    Cpu,
    ChevronLeft,
    ChevronRight,
    Film,
    Image as ImageIcon,
    AlertCircle,
    Info,
    HardDrive,
    Maximize2,
    Minimize2,
    CheckCircle2,
    ChevronUp,
    ChevronDown,
    Disc,
    Bookmark
  } from 'lucide-svelte';
  import VideoPlayer from '../VideoPlayer.svelte';
  import { sound } from '../../navigation/audio';
  import * as AppAPI from '../../../../wailsjs/go/main/App';
  import type {
    GameEntity,
    SteamMovie,
    MediaItem,
    SteamCandidateItem,
    GamePageDetails
  } from '../../types/game';
  import { EventsOn } from '../../../../wailsjs/runtime/runtime';

  let {
    game = null as any,
    downloadPath = 'C:\\Ducke',
    onBack = () => {},
    onStartDownload = (gameId: number, targetPath: string) => {},
    onSelectFolder = async (): Promise<string> => '',
    activeDownloads = [] as any[]
  } = $props();

  let customDownloadPath = $state<string>('');
  let activeMediaIndex = $state<number>(0);
  let screenshotViewport = $state<HTMLDivElement | null>(null);
  let isScreenshotFullscreen = $state(false);
  let pageDetails = $state<GamePageDetails | null>(null);
  let imageLoadFailed = $state<Record<string, boolean>>({});

  // Steam candidate search modal state
  let isSteamModalOpen = $state<boolean>(false);
  let steamSearchTerm = $state<string>('');
  let steamCandidates = $state<SteamCandidateItem[]>([]);
  let isSearchingSteam = $state<boolean>(false);
  let directAppIdInput = $state<string>('');
  let isSavingSteam = $state<boolean>(false);

  // Favorites backlog state
  let isFavoriteDropdownOpen = $state<boolean>(false);
  let favoriteDropdownContainerEl = $state<HTMLDivElement | null>(null);
  let favoriteDropdownTriggerEl = $state<HTMLButtonElement | null>(null);

  async function handleToggleFavoriteStatus(status: string) {
    const targetGame = pageDetails?.game || game;
    if (!targetGame?.id) return;
    try {
      sound.playSelect();
      isFavoriteDropdownOpen = false;
      await AppAPI.SetFavoriteStatus(targetGame.id, status);
      targetGame.favoriteStatus = status;
      if (game) game.favoriteStatus = status;
    } catch (e) {
      console.error('Failed to set favorite status:', e);
    }
  }

  async function handleRemoveFavorite() {
    const targetGame = pageDetails?.game || game;
    if (!targetGame?.id) return;
    try {
      sound.playFocus();
      isFavoriteDropdownOpen = false;
      await AppAPI.RemoveFromFavorites(targetGame.id);
      targetGame.favoriteStatus = '';
      if (game) game.favoriteStatus = '';
    } catch (e) {
      console.error('Failed to remove from favorites:', e);
    }
  }

  let currentFavoriteStatus = $derived((pageDetails?.game || game)?.favoriteStatus || '');

  $effect(() => {
    if (downloadPath) customDownloadPath = downloadPath;
  });

  let activeDownload = $derived.by(() => {
    if (!game) return null;
    return (activeDownloads || []).find((d) => d && d.gameId === game.id);
  });

  let isDownloading = $derived(
    (activeDownload && (activeDownload.status === 'downloading' || activeDownload.status === 'queued' || activeDownload.status === 'scanning')) ||
    (pageDetails && (pageDetails.downloadStatus === 'downloading' || pageDetails.downloadStatus === 'queued' || pageDetails.downloadStatus === 'scanning'))
  );
  let isCompleted = $derived(
    (activeDownload && activeDownload.status === 'completed') ||
    pageDetails?.isInstalled
  );

  let movieOverrides = $state<Record<number, SteamMovie[]>>({});
  let enrichingGameIds = new Set<number>();
  let selectedVariantId = $state<number | null>(null);
  let isVariantDropdownOpen = $state<boolean>(false);
  let variantDropdownTriggerEl = $state<HTMLButtonElement | null>(null);
  let variantDropdownContainerEl = $state<HTMLDivElement | null>(null);

  let activeVariantsList = $derived((pageDetails?.game || game)?.variants || []);

  let activeVariant = $derived.by(() => {
    const vg = pageDetails?.game || game;
    if (!vg?.variants || vg.variants.length === 0) return null;
    return vg.variants.find((v: any) => v.id === selectedVariantId) || vg.variants[0];
  });

  $effect(() => {
    const vg = pageDetails?.game || game;
    if (vg?.variants && vg.variants.length > 0) {
      if (!selectedVariantId || !vg.variants.some((v: any) => v.id === selectedVariantId)) {
        selectedVariantId = vg.variants[0].id;
      }
    }
  });

  function handleWindowPointerDown(e: PointerEvent) {
    const target = e.target as Node | null;
    if (!target) return;
    if (isVariantDropdownOpen) {
      if (!variantDropdownContainerEl?.contains(target) && !variantDropdownTriggerEl?.contains(target)) {
        isVariantDropdownOpen = false;
      }
    }
    if (isFavoriteDropdownOpen) {
      if (!favoriteDropdownContainerEl?.contains(target) && !favoriteDropdownTriggerEl?.contains(target)) {
        isFavoriteDropdownOpen = false;
      }
    }
  }

  // --------------------------------------------------------------------------
  // Game Switch & Data Loading Architecture (Big Picture)
  // --------------------------------------------------------------------------
  let lastGameId: number | null = null;
  let isSwitching = $state<boolean>(false);
  let switchSequence = 0;
  let switchTimeout: any = null;
  let scrollContainer = $state<HTMLDivElement | null>(null);
  let isMounted = true;

  // In-memory cache for fast back-and-forth switching
  const pageDetailsCache = new Map<number, GamePageDetails>();

  onDestroy(() => {
    isMounted = false;
    clearTimeout(switchTimeout);
  });

  function getAppAPI(): any {
    if (typeof window !== 'undefined' && (window as any)?.go?.main?.App) {
      return (window as any).go.main.App;
    }
    return AppAPI;
  }

  function createInitialPageDetails(g: GameEntity): GamePageDetails {
    return {
      game: g,
      downloadStatus: 'none',
      downloadProgress: null,
      localPath: '',
      isInstalled: false,
      logoUrl: g.iconUrl || '',
      bannerUrl: g.backgroundImage || g.headerImage || '',
      coverUrl: g.capsuleImage || '',
      backgroundUrl: g.backgroundImage || '',
      media: []
    };
  }

  $effect(() => {
    const curId = game?.id;
    if (!curId || !game) {
      pageDetails = null;
      isSwitching = false;
      lastGameId = null;
      return;
    }

    if (curId !== lastGameId) {
      lastGameId = curId;
      const seq = ++switchSequence;

      // 1. Reset interactive state immediately
      activeMediaIndex = 0;
      selectedVariantId = game?.variants && game.variants.length > 0 ? game.variants[0].id : curId;
      isVariantDropdownOpen = false;
      isFavoriteDropdownOpen = false;

      // 2. Reset scroll position to top
      if (scrollContainer) {
        scrollContainer.scrollTop = 0;
      }

      // 3. Populate immediate baseline data (from cache or game entity)
      if (pageDetailsCache.has(curId)) {
        pageDetails = pageDetailsCache.get(curId)!;
        isSwitching = false;
      } else {
        pageDetails = createInitialPageDetails(game);
        const hasRichData = !!(game.shortDescription || game.detailedDescription || (game.screenshots && game.screenshots.length > 0));
        if (!hasRichData) {
          isSwitching = true;
          clearTimeout(switchTimeout);
          switchTimeout = setTimeout(() => {
            if (isMounted && seq === switchSequence) {
              isSwitching = false;
            }
          }, 150);
        } else {
          isSwitching = false;
        }
      }

      // 4. Launch coordinated background loaders
      fetchGamePageDetails(curId, seq);
      resolveMoviesIfNeeded(game, seq);
      triggerPriorityEnrichmentIfNeeded(game, curId, seq);
    }
  });

  async function fetchGamePageDetails(gameId: number, seq: number) {
    if (!isMounted) return;

    try {
      const app = getAppAPI();
      if (!app) return;

      let res: GamePageDetails | null = null;
      if (typeof app.GetGamePageDetails === 'function') {
        res = await app.GetGamePageDetails(gameId);
      } else if (typeof app.GetGameDetails === 'function') {
        const details = await app.GetGameDetails(gameId);
        if (details) {
          res = {
            game: details,
            downloadStatus: 'none',
            downloadProgress: null,
            localPath: '',
            isInstalled: false,
            logoUrl: '',
            bannerUrl: '',
            coverUrl: '',
            backgroundUrl: '',
            media: []
          };
        }
      }

      if (!isMounted || seq !== switchSequence) return;

      if (res) {
        pageDetailsCache.set(gameId, res);
        pageDetails = res;
      }
    } catch (err) {
      console.warn('[BigPictureGameDetail] Failed to get page details:', err);
    } finally {
      if (isMounted && seq === switchSequence) {
        isSwitching = false;
      }
    }
  }

  function resolveMoviesIfNeeded(g: GameEntity, seq: number) {
    if (!g.steamAppId || g.steamAppId <= 0) return;

    const currentMovies = movieOverrides[g.id] || g.movies || [];
    const hasValidHls = Array.isArray(currentMovies) && currentMovies.some((m: any) => m.hls && m.hls.trim() !== '');
    if (hasValidHls) return;

    const app = getAppAPI();
    if (app && typeof app.GetGameMovies === 'function') {
      app.GetGameMovies(g.steamAppId).then((freshMovies: any) => {
        if (isMounted && seq === switchSequence && freshMovies && freshMovies.length > 0) {
          movieOverrides[g.id] = freshMovies;
        }
      }).catch(() => {});
    }
  }

  function triggerPriorityEnrichmentIfNeeded(g: GameEntity, curId: number, seq: number) {
    if (enrichingGameIds.has(curId)) return;
    const hasRich = !!(
      (g.screenshots && g.screenshots.length > 0) ||
      (g.movies && g.movies.length > 0) ||
      g.shortDescription ||
      g.detailedDescription ||
      (g.genres && g.genres.length > 0)
    );
    if (hasRich) return;

    enrichingGameIds.add(curId);
    const app = getAppAPI();
    if (app && typeof app.EnrichGameNow === 'function') {
      app.EnrichGameNow(curId).then((enriched: any) => {
        if (isMounted && seq === switchSequence && enriched) {
          fetchGamePageDetails(curId, seq);
        }
      }).catch(() => {});
    }
  }

  function sanitizeImageUrl(url: string | undefined): string {
    if (!url) return '';
    let res = url.replace(/^http:\/\//i, 'https://');
    res = res.replace(/shared\.akamai\.steamstatic\.com/gi, 'shared.fastly.steamstatic.com');
    res = res.replace(/cdn\.cloudflare\.steamstatic\.com/gi, 'shared.fastly.steamstatic.com');
    return res;
  }

  function sanitizeVideoUrl(url: string | undefined): string {
    if (!url) return '';
    let res = url.replace(/^http:\/\//i, 'https://');
    res = res.replace(/video\.akamai\.steamstatic\.com/gi, 'video.fastly.steamstatic.com');
    res = res.replace(/shared\.akamai\.steamstatic\.com/gi, 'video.fastly.steamstatic.com');
    return res;
  }

  // Background artwork
  let backdropUrl = $derived.by(() => {
    if (pageDetails?.backgroundUrl && !imageLoadFailed[pageDetails.backgroundUrl]) {
      return sanitizeImageUrl(pageDetails.backgroundUrl);
    }
    if (!game) return '';
    if (game.backgroundImage) return sanitizeImageUrl(game.backgroundImage);
    if (game.headerImage) return sanitizeImageUrl(game.headerImage);
    if (game.steamAppId && game.steamAppId > 0) return `https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/${game.steamAppId}/page_bg_generated_v6b.jpg`;
    return '';
  });

  function isHorizontalAsset(url: string): boolean {
    if (!url) return false;
    return /capsule_231x87|capsule_616x353|capsule_467x181|header\.jpg|header_alt/i.test(url);
  }

  // Vertical Cover image (2:3 aspect)
  let coverUrl = $derived.by(() => {
    if (pageDetails?.coverUrl && !imageLoadFailed[pageDetails.coverUrl]) {
      return sanitizeImageUrl(pageDetails.coverUrl);
    }
    if (!game) return '';
    if (game.capsuleImage && !isHorizontalAsset(game.capsuleImage)) {
      return sanitizeImageUrl(game.capsuleImage);
    }
    if (game.steamAppId && game.steamAppId > 0) {
      return `https://shared.steamstatic.com/store_item_assets/steam/apps/${game.steamAppId}/library_600x900.jpg`;
    }
    if (game.libraryCover && !isHorizontalAsset(game.libraryCover)) return sanitizeImageUrl(game.libraryCover);
    if (game.coverUrl && !isHorizontalAsset(game.coverUrl)) return sanitizeImageUrl(game.coverUrl);
    return '';
  });

  let isCoverFailed = $state<boolean>(false);
  let isResolvingSGDB = false;

  $effect(() => {
    if (coverUrl) {
      isCoverFailed = false;
    }
  });

  async function handleCoverError(e: Event) {
    const target = e.currentTarget as HTMLImageElement;
    if (!target || !game) return;
    const appId = game.steamAppId;
    const currentSrc = target.src;

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

    if (game.capsuleImage && !isHorizontalAsset(game.capsuleImage) && target.src !== sanitizeImageUrl(game.capsuleImage)) {
      target.src = sanitizeImageUrl(game.capsuleImage);
      return;
    }

    const term = game.cleanTitle || game.displayTitle || game.steamTitle || game.folderName || '';
    if (term && !isResolvingSGDB) {
      isResolvingSGDB = true;
      try {
        const app = getAppAPI();
        if (app && typeof app.ResolveGameCover === 'function') {
          const sgdbCover = await app.ResolveGameCover(game.id || 0, term, game.steamAppId || 0);
          if (sgdbCover) {
            game.capsuleImage = sgdbCover;
            target.src = sanitizeImageUrl(sgdbCover);
            return;
          }
        }
      } catch {
        // Normal fallback when cover is not available on SteamGridDB
      }
    }

    isCoverFailed = true;
  }

  // Unified Media List (Trailers + Screenshots)
  let mediaList = $derived.by<MediaItem[]>(() => {
    if (!game) return [];
    const list: MediaItem[] = [];

    const effectiveMovies = (game.id && movieOverrides[game.id]) || game.movies;
    if (Array.isArray(effectiveMovies)) {
      effectiveMovies.forEach((m: SteamMovie) => {
        let hls = m.hls ? sanitizeVideoUrl(m.hls) : '';
        let mp4 = m.mp4 ? sanitizeVideoUrl(m.mp4) : '';
        let webm = m.webm ? sanitizeVideoUrl(m.webm) : '';
        let thumb = m.thumbnail ? sanitizeImageUrl(m.thumbnail) : '';

        if (mp4.includes('/apps/') && mp4.includes('movie_max.mp4')) {
          mp4 = '';
        }

        const primaryUrl = hls || mp4 || webm;

        if (primaryUrl || thumb) {
          list.push({
            type: 'video',
            id: m.id || list.length,
            name: m.name || 'Трейлер',
            url: primaryUrl,
            thumbnail: thumb,
            hls,
            webm,
            mp4
          });
        }
      });
    }

    if (Array.isArray(game.screenshots)) {
      game.screenshots.forEach((s: any, idx: number) => {
        const url = sanitizeImageUrl(typeof s === 'string' ? s : s?.url);
        if (url) {
          list.push({
            type: 'image',
            id: idx,
            name: `Скриншот ${idx + 1}`,
            url
          });
        }
      });
    }

    return list;
  });

  let activeMedia = $derived.by<MediaItem | null>(() => {
    if (!mediaList || mediaList.length === 0) return null;
    const idx = Math.min(Math.max(0, activeMediaIndex), mediaList.length - 1);
    return mediaList[idx];
  });

  function nextMedia() {
    if (mediaList.length === 0) return;
    sound.playFocus();
    activeMediaIndex = (activeMediaIndex + 1) % mediaList.length;
  }

  function prevMedia() {
    if (mediaList.length === 0) return;
    sound.playFocus();
    activeMediaIndex = (activeMediaIndex - 1 + mediaList.length) % mediaList.length;
  }

  function toggleMediaFullscreen() {
    if (!screenshotViewport) return;
    sound.playSelect();
    if (!document.fullscreenElement) {
      screenshotViewport.requestFullscreen?.().catch(console.error);
    } else {
      document.exitFullscreen?.().catch(console.error);
    }
  }

  function toggleScreenshotFullscreen() {
    toggleMediaFullscreen();
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape' && (isVariantDropdownOpen || isFavoriteDropdownOpen)) {
      e.preventDefault();
      e.stopPropagation();
      isVariantDropdownOpen = false;
      isFavoriteDropdownOpen = false;
      return;
    }
    if (isSteamModalOpen) return;
    if (e.key === 'ArrowRight' || e.key === 'KeyD') {
      nextMedia();
    } else if (e.key === 'ArrowLeft' || e.key === 'KeyA') {
      prevMedia();
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('pointerdown', handleWindowPointerDown, true);
    const onFsChange = () => {
      isScreenshotFullscreen = !!document.fullscreenElement;
    };
    document.addEventListener('fullscreenchange', onFsChange);

    const onPrev = () => prevMedia();
    const onNext = () => nextMedia();
    const onSubTabPrev = (e: Event) => {
      if (mediaList.length > 1) {
        e.preventDefault();
        prevMedia();
      }
    };
    const onSubTabNext = (e: Event) => {
      if (mediaList.length > 1) {
        e.preventDefault();
        nextMedia();
      }
    };
    const onClose = () => {
      if (document.fullscreenElement) {
        document.exitFullscreen?.().catch(console.error);
      } else if (isSteamModalOpen) {
        closeSteamModal();
      } else if (isVariantDropdownOpen) {
        isVariantDropdownOpen = false;
      }
    };
    const onGoBack = (e: Event) => {
      if (document.fullscreenElement) {
        document.exitFullscreen?.().catch(console.error);
        e.preventDefault();
        e.stopPropagation();
      } else if (isSteamModalOpen) {
        closeSteamModal();
        e.preventDefault();
        e.stopPropagation();
      } else if (isVariantDropdownOpen) {
        isVariantDropdownOpen = false;
        e.preventDefault();
        e.stopPropagation();
      }
    };

    window.addEventListener('app:gallery-prev', onPrev);
    window.addEventListener('app:gallery-next', onNext);
    window.addEventListener('app:subtab-prev', onSubTabPrev);
    window.addEventListener('app:subtab-next', onSubTabNext);
    window.addEventListener('app:modal-close', onClose);
    window.addEventListener('app:go-back', onGoBack);

    const unsubReviews = EventsOn('game:reviews-updated', (event: any) => {
      if (!event || !game) return;
      if (event.gameId === game.id || (event.steamAppId > 0 && event.steamAppId === game.steamAppId)) {
        if (pageDetails?.game) {
          pageDetails.game.reviewScoreDesc = event.reviewScoreDesc;
          pageDetails.game.reviewPercent = event.reviewPercent;
          pageDetails.game.totalReviews = event.totalReviews;
        }
        game.reviewScoreDesc = event.reviewScoreDesc;
        game.reviewPercent = event.reviewPercent;
        game.totalReviews = event.totalReviews;
      }
    });

    const unsubEnriched = EventsOn('game:enriched', (enrichedGame: any) => {
      if (!isMounted || !enrichedGame) return;
      const targetId = game?.id;
      if (targetId && (enrichedGame.id === targetId || (enrichedGame.steamAppId > 0 && enrichedGame.steamAppId === game?.steamAppId))) {
        if (pageDetails) {
          pageDetails.game = { ...pageDetails.game, ...enrichedGame };
          pageDetailsCache.set(targetId, pageDetails);
        }
        if (game) {
          game = { ...game, ...enrichedGame };
        }
        resolveAccentColor(enrichedGame, switchSequence);
        resolveMoviesIfNeeded(enrichedGame, switchSequence);
      }
    });

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('pointerdown', handleWindowPointerDown, true);
      document.removeEventListener('fullscreenchange', onFsChange);
      window.removeEventListener('app:gallery-prev', onPrev);
      window.removeEventListener('app:gallery-next', onNext);
      window.removeEventListener('app:subtab-prev', onSubTabPrev);
      window.removeEventListener('app:subtab-next', onSubTabNext);
      window.removeEventListener('app:modal-close', onClose);
      window.removeEventListener('app:go-back', onGoBack);
      clearTimeout(switchTimeout);
      if (typeof unsubReviews === 'function') unsubReviews();
      if (typeof unsubEnriched === 'function') unsubEnriched();
    };
  });

  async function handleBrowseFolder() {
    sound.playFocus();
    try {
      const selected = await onSelectFolder();
      if (selected) {
        customDownloadPath = selected;
      }
    } catch (e) {
      console.error(e);
    }
  }

  function handleDownload() {
    if (!game) return;
    sound.playSelect();
    onStartDownload(selectedVariantId || game.id, customDownloadPath);
  }

  async function handleLaunch() {
    if (!game) return;
    sound.playSelect();
    try {
      const app = getAppAPI();
      if (app && typeof app.LaunchGameByGameID === 'function') {
        await app.LaunchGameByGameID(game.id);
      } else if (app && typeof app.LaunchGame === 'function') {
        await app.LaunchGame(game.folderName || '');
      }
    } catch (e) {
      console.error(e);
    }
  }

  async function handleOpenGameFolder() {
    if (!game) return;
    sound.playSelect();
    try {
      const app = getAppAPI();
      if (app && typeof app.OpenGameFolder === 'function') {
        await app.OpenGameFolder(game.id);
      }
    } catch (e) {
      console.error(e);
    }
  }

  function scrollToTop() {
    sound.playSelect();
    if (scrollContainer) {
      scrollContainer.scrollTo({ top: 0, behavior: 'smooth' });
    }
  }

  // Steam Modal Management
  async function openSteamModal() {
    sound.playSelect();
    const raw = game?.steamTitle || (game as any)?.searchTitle || game?.cleanTitle || game?.displayTitle || game?.folderName || '';
    const cleaned = raw
      .replace(/\[.*?\]|\(.*?\)|[\{\}]/g, ' ')
      .replace(/\b(v\s*\d+([._\s]\d+)*|build\s*\d+|patch\s*\d+|update\s*\d+)\b/gi, ' ')
      .replace(/\b(19[7-9]\d|20[0-3]\d)\b/g, ' ')
      .replace(/\s+/g, ' ')
      .trim();
    steamSearchTerm = cleaned || raw;
    directAppIdInput = (game?.steamAppId && game.steamAppId > 0) ? String(game.steamAppId) : '';
    steamCandidates = [];
    isSteamModalOpen = true;
    window.dispatchEvent(new CustomEvent('app:modal-opened'));
    await performSteamSearch(true);
  }

  function closeSteamModal() {
    sound.playBack();
    if (isSteamModalOpen) {
      isSteamModalOpen = false;
      window.dispatchEvent(new CustomEvent('app:modal-closed'));
    }
  }

  async function performSteamSearch(autoLinkExact = false) {
    const cleanQuery = (steamSearchTerm || '')
      .replace(/^[\{\[\(]\s*(linux|win|windows|mac|macos|pc|gog|steam|portable|repack|native|unpack|unpacked)\s*[\}\]\)]\s*/gi, '')
      .replace(/\s*[\{\[\(]\s*(linux|win|windows|mac|macos|pc|gog|steam|portable|repack|native|unpack|unpacked)\s*[\}\]\)]$/gi, '')
      .replace(/[\{\}]/g, '')
      .trim();

    if (!cleanQuery) return;
    isSearchingSteam = true;
    try {
      const app = getAppAPI();
      if (app && typeof app.SearchSteamCandidates === 'function') {
        const res = await app.SearchSteamCandidates(cleanQuery);
        steamCandidates = res || [];

        // Automatically link if exact 100% (or score >= 0.95) match found on initial auto-search
        if (autoLinkExact && steamCandidates.length > 0 && steamCandidates[0].score >= 0.95 && (!game?.steamAppId || game.steamAppId <= 0)) {
          await handleLinkAppId(steamCandidates[0].appId);
          return;
        }
      }
    } catch (e) {
      console.warn('Steam search failed:', e);
      steamCandidates = [];
    } finally {
      isSearchingSteam = false;
    }
  }

  async function handleLinkAppId(appId: number) {
    if (!game) return;
    isSavingSteam = true;
    try {
      const app = getAppAPI();
      if (app && typeof app.UpdateSteamAppID === 'function') {
        await app.UpdateSteamAppID(game.id, appId);
      }
      game.steamAppId = appId;
      closeSteamModal();
      loadGameDetails(game.id);
    } catch (e) {
      console.error(e);
    } finally {
      isSavingSteam = false;
    }
  }

  async function handleApplyDirectAppId() {
    const clean = directAppIdInput.trim().replace(/\D/g, '');
    const num = parseInt(clean, 10);
    if (!isNaN(num) && num > 0) {
      await handleLinkAppId(num);
    }
  }

  async function handleUnlinkMetadata() {
    if (!game) return;
    isSavingSteam = true;
    try {
      const app = getAppAPI();
      if (app && typeof app.ResetGameMetadata === 'function') {
        await app.ResetGameMetadata(game.id);
      } else if (app && typeof app.UpdateSteamAppID === 'function') {
        await app.UpdateSteamAppID(game.id, 0);
      }
      game.steamAppId = 0;
      closeSteamModal();
      loadGameDetails(game.id);
    } catch (e) {
      console.error(e);
    } finally {
      isSavingSteam = false;
    }
  }

  let logoUrl = $derived(pageDetails?.logoUrl);
  let hasLogo = $derived(logoUrl && !imageLoadFailed[logoUrl]);
  let activeGame = $derived(pageDetails?.game || game);
</script>

<div bind:this={scrollContainer} data-nav-zone="detail" class="flex-1 flex flex-col h-full overflow-y-auto bg-[#07080a] text-white select-none relative">
  {#if game}
    {#if isSwitching}
      <!-- Atmospheric Fullscreen Backdrop Skeleton -->
      <div class="fixed inset-0 pointer-events-none z-0 overflow-hidden">
        <div class="w-full h-full bg-white/[0.02] animate-pulse"></div>
        <div class="absolute inset-0 bg-gradient-to-t from-[#07080a] via-[#07080a]/85 to-transparent"></div>
        <div class="absolute inset-0 bg-gradient-to-r from-[#07080a] via-[#07080a]/70 to-transparent"></div>
      </div>

      <!-- Sticky Top Navigation Bar Skeleton -->
      <div class="relative z-10 px-8 lg:px-14 pt-6 pb-2 flex items-center justify-between">
        <div class="h-9 w-48 rounded-xl bg-white/[0.05] border border-white/10 animate-pulse"></div>
        <div class="h-9 w-40 rounded-xl bg-white/[0.05] border border-white/10 animate-pulse"></div>
      </div>

      <!-- Main Game Content Skeleton -->
      <div class="relative z-10 px-8 lg:px-14 py-4 w-full space-y-10">
        
        <!-- Hero Header Section Skeleton -->
        <div class="flex flex-col lg:flex-row gap-8 lg:gap-10 items-start w-full">
          <!-- Left: Game Poster / Cover Skeleton (2:3 Aspect) -->
          <div class="w-64 sm:w-72 2xl:w-80 aspect-[2/3] rounded-2xl bg-[#0d1017] flex-shrink-0 shadow-2xl border border-white/[0.08] relative overflow-hidden flex items-center justify-center animate-pulse">
            <div class="w-16 h-16 rounded-2xl bg-white/[0.05] border border-white/10 flex items-center justify-center">
              <Gamepad2 class="w-8 h-8 text-white/20" />
            </div>
          </div>

          <!-- Right: Info, Chips & Actions Skeleton -->
          <div class="flex-1 min-w-0 space-y-5">
            <!-- Game Logo / Title Skeleton -->
            <div class="space-y-2 py-1">
              <div class="h-10 sm:h-12 w-2/3 max-w-lg rounded-2xl bg-white/[0.06] animate-pulse"></div>
              <div class="h-4 w-1/3 max-w-xs rounded-lg bg-white/[0.03] animate-pulse"></div>
            </div>

            <!-- Metadata Chips Skeleton (7 chips matching the real blocks) -->
            <div class="flex flex-wrap items-center gap-2.5 text-xs">
              <div class="h-8 w-16 rounded-xl bg-white/[0.05] border border-white/[0.08] animate-pulse"></div>
              <div class="h-8 w-28 rounded-xl bg-white/[0.05] border border-white/[0.08] animate-pulse"></div>
              <div class="h-8 w-32 rounded-xl bg-white/[0.05] border border-white/[0.08] animate-pulse"></div>
              <div class="h-8 w-28 rounded-xl bg-white/[0.05] border border-white/[0.08] animate-pulse"></div>
              <div class="h-8 w-36 rounded-xl bg-white/[0.05] border border-white/[0.08] animate-pulse"></div>
              <div class="h-8 w-32 rounded-xl bg-sky-500/10 border border-sky-500/20 animate-pulse"></div>
              <div class="h-8 w-20 rounded-xl bg-white/[0.05] border border-white/[0.08] animate-pulse"></div>
            </div>

            <!-- Short Description / Synopsis Skeleton (3 lines) -->
            <div class="space-y-2.5 pt-1 max-w-4xl">
              <div class="h-4 w-full rounded bg-white/[0.04] animate-pulse"></div>
              <div class="h-4 w-5/6 rounded bg-white/[0.04] animate-pulse"></div>
              <div class="h-4 w-2/3 rounded bg-white/[0.03] animate-pulse"></div>
            </div>

            <!-- Download & Action Buttons Skeleton -->
            <div class="pt-2 space-y-3">
              <div class="flex flex-wrap items-center gap-3">
                <div class="h-14 w-64 rounded-2xl bg-white/[0.08] border border-white/10 animate-pulse"></div>
                <div class="h-14 w-28 rounded-2xl bg-white/[0.05] border border-white/10 animate-pulse"></div>
              </div>
              <!-- Destination Path Info Skeleton -->
              <div class="h-4 w-64 rounded bg-white/[0.03] animate-pulse"></div>
            </div>
          </div>
        </div>

        <!-- Unified Media Showcase Skeleton -->
        <div class="space-y-4 pt-4 border-t border-white/[0.06] w-full">
          <!-- Main 16:9 Viewport Skeleton -->
          <div class="relative w-full aspect-video max-h-[60vh] rounded-2xl overflow-hidden bg-black/60 border border-white/10 flex items-center justify-center shadow-xl animate-pulse">
            <div class="w-16 h-16 rounded-full bg-white/[0.05] border border-white/10 flex items-center justify-center">
              <Play class="w-6 h-6 ml-0.5 text-white/20" />
            </div>
          </div>

          <!-- Horizontal Thumbnails Strip Skeleton -->
          <div class="flex items-center gap-3 overflow-x-hidden pb-2 w-full">
            {#each [1, 2, 3, 4, 5, 6] as _}
              <div class="flex-shrink-0 w-36 sm:w-44 aspect-video rounded-xl bg-white/[0.04] border border-white/10 animate-pulse"></div>
            {/each}
          </div>
        </div>

        <!-- 2-Column Balanced Content Grid Skeleton -->
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-10 pt-4 border-t border-white/[0.06] w-full">
          <!-- Left Column: Detailed Description Skeleton -->
          <div class="lg:col-span-2 space-y-6">
            <div class="h-4 w-24 rounded bg-white/[0.08] animate-pulse"></div>
            <div class="space-y-3 pt-2">
              <div class="h-4 w-full rounded bg-white/[0.04] animate-pulse"></div>
              <div class="h-4 w-11/12 rounded bg-white/[0.04] animate-pulse"></div>
              <div class="h-4 w-5/6 rounded bg-white/[0.04] animate-pulse"></div>
              <div class="h-4 w-4/5 rounded bg-white/[0.03] animate-pulse"></div>
              <div class="h-4 w-3/4 rounded bg-white/[0.03] animate-pulse"></div>
              <div class="h-4 w-2/3 rounded bg-white/[0.03] animate-pulse"></div>
            </div>
          </div>

          <!-- Right Column: Specs & Requirements Skeleton -->
          <div class="space-y-6">
            <div class="space-y-4 bg-[#0c0e14] p-6 rounded-2xl border border-white/[0.06] animate-pulse">
              <div class="h-4 w-28 rounded bg-white/[0.06]"></div>
              <div class="space-y-3 pt-2">
                {#each [1, 2, 3, 4, 5, 6] as _}
                  <div class="flex items-center justify-between pt-2 border-t border-white/[0.04]">
                    <div class="h-3.5 w-20 rounded bg-white/[0.04]"></div>
                    <div class="h-3.5 w-28 rounded bg-white/[0.05]"></div>
                  </div>
                {/each}
              </div>
            </div>
          </div>
        </div>

      </div>
    {:else}
      <!-- Atmospheric Fullscreen Backdrop -->
      {#if backdropUrl}
    <div class="fixed inset-0 pointer-events-none z-0 overflow-hidden">
      <img
        src={backdropUrl}
        alt=""
        referrerpolicy="no-referrer"
        class="w-full h-full object-cover brightness-[0.38] contrast-[1.08] scale-105 transition-all duration-700"
      />
      <div class="absolute inset-0 bg-gradient-to-t from-[#07080a] via-[#07080a]/85 to-transparent"></div>
      <div class="absolute inset-0 bg-gradient-to-r from-[#07080a] via-[#07080a]/70 to-transparent"></div>
    </div>
  {/if}

  <!-- Sticky Top Navigation Bar -->
  <div class="relative z-10 px-8 lg:px-14 pt-6 pb-2 flex items-center justify-between">
    <button
      data-nav-item
      class="px-4 py-2 rounded-xl bg-black/60 backdrop-blur-md border border-white/10 text-xs font-bold text-white hover:bg-white/10 flex items-center gap-2 cursor-pointer transition-transform active:scale-95 focus:ring-2 focus:ring-white focus:outline-none"
      onclick={() => {
        sound.playBack();
        onBack();
      }}
    >
      <ArrowLeft class="w-4 h-4" />
      <span>Назад к библиотеке [B]</span>
    </button>

    <!-- Steam AppID Link Action -->
    <button
      data-nav-item
      class="px-4 py-2 rounded-xl bg-black/60 backdrop-blur-md border border-white/10 text-xs font-semibold text-[#cbd5e1] hover:text-white hover:bg-white/10 flex items-center gap-2 cursor-pointer transition-colors focus:ring-2 focus:ring-white focus:outline-none"
      onclick={openSteamModal}
      title="Привязать Steam / Изменить метаданные"
    >
      <Edit3 class="w-3.5 h-3.5 text-sky-400" />
      <span>{game.steamAppId > 0 ? `Steam AppID: ${game.steamAppId}` : (game.steamAppId < 0 ? `SteamGridDB: ${-game.steamAppId}` : 'Не привязан')}</span>
    </button>
  </div>

  <!-- Main Game Content -->
  <div class="relative z-10 px-8 lg:px-14 py-4 w-full space-y-10">
    
    <!-- Hero Header Section -->
    <div class="flex flex-col lg:flex-row gap-8 lg:gap-10 items-start w-full">
      
      <!-- Left: Game Poster / Cover (2:3 Aspect) -->
      <div class="w-64 sm:w-72 2xl:w-80 aspect-[2/3] rounded-2xl overflow-hidden bg-[#0d1017] flex-shrink-0 shadow-2xl border border-white/[0.08] relative">
        {#if coverUrl && !isCoverFailed}
          <img
            src={coverUrl}
            alt={game.cleanTitle || game.folderName}
            referrerpolicy="no-referrer"
            class="w-full h-full object-cover"
            onerror={handleCoverError}
          />
        {:else}
          <div class="w-full h-full flex flex-col items-center justify-between p-6 text-center bg-gradient-to-b from-[#181d28] via-[#10141d] to-[#0a0c12] relative overflow-hidden">
            <div class="w-full flex justify-end">
              <Gamepad2 class="w-5 h-5 text-white/20" />
            </div>

            <div class="my-auto flex flex-col items-center space-y-3">
              <div class="w-16 h-16 rounded-2xl bg-white/[0.06] border border-white/10 flex items-center justify-center text-xl font-black text-white/80 shadow-inner">
                {(game.cleanTitle || game.folderName || 'D').slice(0, 2).toUpperCase()}
              </div>
              <span class="text-sm font-bold text-[#e2e8f0] line-clamp-3 leading-snug px-2">
                {game.cleanTitle || game.folderName}
              </span>
            </div>

            <div class="w-full text-center">
              <span class="text-[10px] font-mono uppercase tracking-widest text-[#64748b]">
                {game.steamAppId ? 'Обложка отсутствует' : 'Не привязано'}
              </span>
            </div>
          </div>
        {/if}
      </div>

      <!-- Right: Info, Chips & Actions -->
      <div class="flex-1 min-w-0 space-y-5">
        
        <!-- Game Logo or Title -->
        {#if hasLogo}
          <div class="py-1">
            <img
              src={logoUrl}
              alt={activeGame?.cleanTitle || activeGame?.folderName}
              class="max-h-24 sm:max-h-32 max-w-[420px] sm:max-w-[560px] object-contain object-left select-none filter drop-shadow-lg"
              onerror={() => {
                if (logoUrl) imageLoadFailed[logoUrl] = true;
              }}
            />
          </div>
        {:else}
          <h1 class="text-3xl sm:text-4xl lg:text-5xl font-black tracking-tight text-white drop-shadow-md leading-tight">
            {(activeGame?.steamTitle && !/^Steam App \d+$/i.test(activeGame.steamTitle)) ? activeGame.steamTitle : (activeGame?.cleanTitle || activeGame?.displayTitle || activeGame?.folderName)}
          </h1>
        {/if}

        <!-- Metadata Chips (Ascetic Slate) -->
        <div class="flex flex-wrap items-center gap-2.5 text-xs">
          <!-- PC Badge -->
          <div class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-white/[0.06] border border-white/[0.08] text-[#cbd5e1]">
            <Monitor class="w-3.5 h-3.5 text-sky-400" />
            <span class="font-bold">PC</span>
          </div>

          <!-- Release Date -->
          {#if activeGame?.releaseDate || activeGame?.steamReleaseDate}
            <div class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-white/[0.06] border border-white/[0.08] text-[#cbd5e1]">
              <Calendar class="w-3.5 h-3.5 text-sky-400" />
              <span>{activeGame?.releaseDate || activeGame?.steamReleaseDate}</span>
            </div>
          {/if}

          <!-- Genres -->
          {#if (activeGame?.genres && activeGame.genres.length > 0) || activeGame?.steamGenres}
            <div class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-white/[0.06] border border-white/[0.08] text-[#cbd5e1]">
              <Tag class="w-3.5 h-3.5 text-sky-400" />
              <span>{(activeGame.genres || activeGame.steamGenres).slice(0, 4).join(', ')}</span>
            </div>
          {/if}

          <!-- Developers -->
          {#if activeGame?.developers && activeGame.developers.length > 0}
            <div class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-white/[0.06] border border-white/[0.08] text-[#cbd5e1]">
              <Building2 class="w-3.5 h-3.5 text-sky-400" />
              <span>{activeGame.developers.join(', ')}</span>
            </div>
          {/if}

          <!-- Controller Support -->
          <div class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-white/[0.06] border border-white/[0.08] text-[#cbd5e1]">
            <Gamepad2 class="w-3.5 h-3.5 text-sky-400" />
            <span>{activeGame?.controllerSupport === 'full' ? 'Геймпад (Полная)' : 'Клавиатура / Мышь'}</span>
          </div>

          <!-- Steam Rating / Metacritic -->
          {#if (activeGame?.totalReviews && activeGame.totalReviews > 0) || (activeGame?.reviewPercent && activeGame.reviewPercent > 0)}
            <div class="flex items-center gap-2 px-3.5 py-1.5 rounded-xl bg-sky-500/15 border border-sky-500/30 text-sky-300 font-bold">
              <span class="text-white font-black">{activeGame.reviewPercent || 0}%</span>
              <span>{activeGame.reviewScoreDesc || 'Положительные'}</span>
              {#if activeGame.totalReviews}
                <span class="text-[#94a3b8] font-normal text-[11px]">({activeGame.totalReviews.toLocaleString('ru-RU')} обзоров в Steam)</span>
              {/if}
            </div>
          {:else if activeGame?.metacriticScore || activeGame?.steamScore}
            <div class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-emerald-500/15 border border-emerald-500/30 text-emerald-400 font-bold">
              <Star class="w-3.5 h-3.5 fill-current" />
              <span>Metacritic: {activeGame.metacriticScore || activeGame.steamScore}</span>
            </div>
          {/if}

          <!-- Size -->
          {#if activeGame?.sizeDisplay || activeGame?.sizeStr}
            <div class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-white/[0.06] border border-white/[0.08] text-white font-mono font-bold">
              <Folder class="w-3.5 h-3.5 text-sky-400" />
              <span>{activeGame?.sizeDisplay || activeGame?.sizeStr}</span>
            </div>
          {/if}
        </div>

        <!-- Short Description / Synopsis -->
        {#if game.shortDescription}
          <p class="text-sm text-[#cbd5e1] leading-relaxed max-w-4xl">
            {@html game.shortDescription}
          </p>
        {/if}

        <!-- Prominent Download & Path Actions -->
        <div class="pt-2 space-y-3">
          <div class="flex flex-wrap items-center gap-3">
            {#if isCompleted}
              <button
                data-nav-item
                class="px-9 py-4 rounded-2xl bg-emerald-400 hover:bg-emerald-300 text-black font-black text-sm flex items-center gap-3 cursor-pointer shadow-2xl active:scale-98 transition-transform focus:ring-2 focus:ring-white focus:outline-none"
                onclick={handleLaunch}
              >
                <Play class="w-5 h-5 fill-black stroke-[2]" />
                <span>ИГРАТЬ [A]</span>
              </button>

              <button
                data-nav-item
                class="p-4 rounded-2xl bg-white/10 hover:bg-white/20 text-white text-xs font-semibold flex items-center gap-2 cursor-pointer transition-colors focus:ring-2 focus:ring-white focus:outline-none"
                onclick={handleOpenGameFolder}
                title="Открыть папку с установленной игрой"
              >
                <FolderOpen class="w-4 h-4 text-emerald-400" />
                <span>Папка с игрой</span>
              </button>
            {:else if isDownloading}
              <div class="px-8 py-4 rounded-2xl bg-sky-500 text-black font-black text-sm flex items-center gap-3 shadow-2xl">
                <Download class="w-5 h-5 animate-bounce stroke-[3]" />
                <span>СКАЧИВАЕТСЯ ({Math.round(activeDownload?.progress || pageDetails?.downloadProgress?.progressPercent || 0)}%)</span>
              </div>
            {:else}
              <div class="relative inline-flex items-stretch rounded-2xl shadow-2xl overflow-visible {isVariantDropdownOpen ? 'z-30' : ''}">
                <button
                  data-nav-item
                  class="px-8 py-4 bg-white text-black hover:bg-sky-400 transition-all active:scale-[0.98] text-sm font-black tracking-wide flex items-center gap-3 cursor-pointer focus:ring-2 focus:ring-white focus:outline-none {activeVariantsList && activeVariantsList.length > 1 ? 'rounded-l-2xl' : 'rounded-2xl'}"
                  onclick={handleDownload}
                >
                  <Download class="w-5 h-5 stroke-[3]" />
                  <span>СКАЧАТЬ В ХРАНИЛИЩЕ [A]</span>
                  <span class="opacity-40 font-normal">|</span>
                  <span class="font-mono text-xs font-bold tracking-normal">{activeVariant?.sizeDisplay || game.sizeDisplay}</span>
                </button>

                {#if activeVariantsList && activeVariantsList.length > 1}
                  <button
                    bind:this={variantDropdownTriggerEl}
                    data-nav-item
                    type="button"
                    class="px-4 flex items-center justify-center bg-white text-black hover:bg-sky-400 border-l border-black/15 transition-all active:scale-95 cursor-pointer rounded-r-2xl focus:ring-2 focus:ring-white focus:outline-none"
                    onclick={(e) => {
                      e.stopPropagation();
                      isVariantDropdownOpen = !isVariantDropdownOpen;
                    }}
                    title="Выбрать версию ({activeVariantsList.length} доступно)"
                  >
                    <ChevronDown class="w-4 h-4 stroke-[2.5] transition-transform duration-200 {isVariantDropdownOpen ? 'rotate-180' : ''}" />
                  </button>

                  {#if isVariantDropdownOpen}
                    <div
                      bind:this={variantDropdownContainerEl}
                      class="absolute left-0 bottom-full mb-3 z-50 w-[360px] sm:w-[420px] max-h-72 overflow-y-auto overscroll-contain rounded-2xl bg-[#0d1117] border border-white/10 shadow-2xl p-2 space-y-1 backdrop-blur-md"
                    >
                      <div class="px-3 py-1.5 text-[11px] font-bold uppercase tracking-wider text-[#64748b]">
                        Выбор версии ({activeVariantsList.length})
                      </div>
                      {#each activeVariantsList as variant (variant.id)}
                        {@const isSelected = (activeVariant?.id === variant.id)}
                        <button
                          data-nav-item
                          type="button"
                          class="w-full text-left flex items-center justify-between gap-3 px-3 py-2.5 rounded-xl text-xs transition-colors cursor-pointer {isSelected ? 'bg-white/15 text-white font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                          onclick={(e) => {
                            e.stopPropagation();
                            selectedVariantId = variant.id;
                            isVariantDropdownOpen = false;
                          }}
                        >
                          <div class="min-w-0 flex-1 pointer-events-none">
                            <div class="truncate text-white text-xs">{variant.rawName}</div>
                            <div class="text-[10px] text-[#64748b] font-mono">
                              {variant.sourceType === 'torrent' ? 'Торрент' : 'FTP'}
                            </div>
                          </div>
                          <div class="flex items-center gap-2 flex-shrink-0 font-mono text-[11px] pointer-events-none {isSelected ? 'text-sky-400 font-bold' : 'text-[#64748b]'}">
                            <span>{variant.sizeDisplay}</span>
                            {#if isSelected}
                              <Check class="w-3.5 h-3.5 stroke-[2.5]" />
                            {/if}
                          </div>
                        </button>
                      {/each}
                    </div>
                  {/if}
                {/if}
              </div>

              <button
                data-nav-item
                class="p-4 rounded-2xl bg-white/10 hover:bg-white/20 text-white text-xs font-semibold flex items-center gap-2 cursor-pointer transition-colors focus:ring-2 focus:ring-white focus:outline-none"
                onclick={handleBrowseFolder}
                title="Изменить папку для сохранения"
              >
                <FolderOpen class="w-4 h-4 text-sky-400" />
                <span>Папка</span>
              </button>
            {/if}

            <!-- Favorites Backlog Dropdown Button -->
            <div class="relative">
              <button
                bind:this={favoriteDropdownTriggerEl}
                data-nav-item
                type="button"
                class="p-4 rounded-2xl border text-xs font-semibold flex items-center gap-2 cursor-pointer transition-colors focus:ring-2 focus:ring-white focus:outline-none {currentFavoriteStatus ? 'bg-sky-500/15 text-sky-300 border-sky-500/30 hover:bg-sky-500/25' : 'bg-white/10 hover:bg-white/20 text-white border-white/5'}"
                onclick={() => {
                  sound.playFocus();
                  isFavoriteDropdownOpen = !isFavoriteDropdownOpen;
                }}
                title="Статус в избранном"
              >
                <Bookmark class="w-4 h-4 {currentFavoriteStatus ? 'text-sky-400 fill-sky-400/40' : 'text-[#8e95a2]'}" />
                <span>
                  {#if currentFavoriteStatus === 'playing'}
                    Прохожу
                  {:else if currentFavoriteStatus === 'completed'}
                    Пройдено
                  {:else if currentFavoriteStatus === 'planned'}
                    В планах
                  {:else}
                    В избранное
                  {/if}
                </span>
                <ChevronDown class="w-3.5 h-3.5 transition-transform duration-200 {isFavoriteDropdownOpen ? 'rotate-180' : ''}" />
              </button>

              {#if isFavoriteDropdownOpen}
                <div
                  bind:this={favoriteDropdownContainerEl}
                  class="absolute left-0 bottom-full mb-3 z-50 w-56 rounded-2xl bg-[#0d1117] border border-white/10 shadow-2xl p-1.5 space-y-1 backdrop-blur-md"
                >
                  <div class="px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-[#64748b]">
                    Статус прохождения
                  </div>
                  <button
                    data-nav-item
                    type="button"
                    class="w-full text-left flex items-center justify-between px-3 py-2 rounded-xl text-xs transition-colors cursor-pointer {currentFavoriteStatus === 'planned' ? 'bg-sky-500/20 text-sky-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                    onclick={() => handleToggleFavoriteStatus('planned')}
                  >
                    <span>В планах</span>
                    {#if currentFavoriteStatus === 'planned'}
                      <Check class="w-3.5 h-3.5 text-sky-400" />
                    {/if}
                  </button>

                  <button
                    data-nav-item
                    type="button"
                    class="w-full text-left flex items-center justify-between px-3 py-2 rounded-xl text-xs transition-colors cursor-pointer {currentFavoriteStatus === 'playing' ? 'bg-sky-500/20 text-sky-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                    onclick={() => handleToggleFavoriteStatus('playing')}
                  >
                    <span>Прохожу</span>
                    {#if currentFavoriteStatus === 'playing'}
                      <Check class="w-3.5 h-3.5 text-sky-400" />
                    {/if}
                  </button>

                  <button
                    data-nav-item
                    type="button"
                    class="w-full text-left flex items-center justify-between px-3 py-2 rounded-xl text-xs transition-colors cursor-pointer {currentFavoriteStatus === 'completed' ? 'bg-sky-500/20 text-sky-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                    onclick={() => handleToggleFavoriteStatus('completed')}
                  >
                    <span>Пройдено</span>
                    {#if currentFavoriteStatus === 'completed'}
                      <Check class="w-3.5 h-3.5 text-sky-400" />
                    {/if}
                  </button>

                  {#if currentFavoriteStatus}
                    <div class="pt-1 mt-1 border-t border-white/5">
                      <button
                        data-nav-item
                        type="button"
                        class="w-full text-left flex items-center gap-2 px-3 py-2 rounded-xl text-xs text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
                        onclick={handleRemoveFavorite}
                      >
                        <X class="w-3.5 h-3.5" />
                        <span>Удалить из избранного</span>
                      </button>
                    </div>
                  {/if}
                </div>
              {/if}
            </div>
          </div>

          <!-- Destination Path Info -->
          <div class="text-[11px] font-mono text-[#8e95a2] flex items-center gap-1.5">
            <span class="text-[#64748b]">{pageDetails?.isInstalled ? 'Установлено в:' : 'Сохранение в:'}</span>
            <span class="text-white truncate">{pageDetails?.localPath || customDownloadPath}</span>
          </div>
        </div>

      </div>
    </div>

    <!-- Unified Media Showcase (Trailers & Screenshots Unified) -->
    {#if mediaList.length > 0 && activeMedia}
      <div class="space-y-4 pt-4 border-t border-white/[0.06] w-full">
        <!-- Main 16:9 Viewport -->
        <div
          bind:this={screenshotViewport}
          class="relative w-full aspect-video {isScreenshotFullscreen ? 'max-h-full h-full rounded-none border-0' : 'max-h-[60vh] rounded-2xl border border-white/10'} overflow-hidden bg-black group/viewer flex items-center justify-center shadow-xl"
        >
          {#if activeMedia.type === 'video'}
            <VideoPlayer
              hls={activeMedia.hls}
              mp4={activeMedia.mp4}
              webm={activeMedia.webm}
              src={activeMedia.url}
              poster={activeMedia.thumbnail}
              title={activeMedia.name}
              autoplay={false}
              controls={true}
            />
          {:else}
            <button
              type="button"
              class="w-full h-full flex items-center justify-center cursor-pointer border-0 bg-transparent p-0 focus:outline-none"
              onclick={toggleMediaFullscreen}
              title="Открыть на весь экран"
            >
              <img
                src={activeMedia.url}
                alt={activeMedia.name}
                class="w-full h-full object-contain select-none"
              />
            </button>

            <!-- Navigation Buttons -->
            {#if mediaList.length > 1}
              <button
                type="button"
                data-nav-item
                class="absolute left-4 top-1/2 -translate-y-1/2 p-3 rounded-full bg-black/60 hover:bg-black/90 text-white border border-white/10 opacity-0 group-hover/viewer:opacity-100 transition-opacity cursor-pointer active:scale-95 z-20 focus:ring-2 focus:ring-white focus:outline-none"
                onclick={prevMedia}
                title="Предыдущее медиа [← / LB]"
              >
                <ChevronLeft class="w-6 h-6 stroke-[2.5]" />
              </button>

              <button
                type="button"
                data-nav-item
                class="absolute right-4 top-1/2 -translate-y-1/2 p-3 rounded-full bg-black/60 hover:bg-black/90 text-white border border-white/10 opacity-0 group-hover/viewer:opacity-100 transition-opacity cursor-pointer active:scale-95 z-20 focus:ring-2 focus:ring-white focus:outline-none"
                onclick={nextMedia}
                title="Следующее медиа [→ / RB]"
              >
                <ChevronRight class="w-6 h-6 stroke-[2.5]" />
              </button>
            {/if}

            <!-- Top-Right Fullscreen Button -->
            <div class="absolute top-4 right-4 flex items-center gap-2 opacity-0 group-hover/viewer:opacity-100 transition-opacity z-20">
              <button
                type="button"
                data-nav-item
                class="p-2.5 rounded-xl bg-black/70 hover:bg-black/90 text-white border border-white/15 cursor-pointer transition-colors focus:ring-2 focus:ring-white focus:outline-none"
                onclick={toggleMediaFullscreen}
                title={isScreenshotFullscreen ? "Выйти из полноэкранного режима" : "Во весь экран"}
              >
                {#if isScreenshotFullscreen}
                  <Minimize2 class="w-5 h-5" />
                {:else}
                  <Maximize2 class="w-5 h-5" />
                {/if}
              </button>
            </div>

            <!-- Bottom-Left Label -->
            <div class="absolute bottom-4 left-4 px-3 py-1.5 rounded-lg bg-black/75 backdrop-blur-sm border border-white/10 text-xs font-semibold text-white/90 pointer-events-none flex items-center gap-2">
              <ImageIcon class="w-4 h-4 text-[#9ca3af]" />
              <span>{activeMedia.name}</span>
              <span class="text-[#6b7280] font-mono">({activeMediaIndex + 1} / {mediaList.length})</span>
            </div>
          {/if}
        </div>

        <!-- Horizontal Thumbnails Strip -->
        {#if mediaList.length > 1}
          <div class="flex items-center gap-3 overflow-x-auto overflow-y-hidden pb-2 no-scrollbar w-full">
            {#each mediaList as item, idx}
              <button
                data-nav-item
                type="button"
                class="relative flex-shrink-0 w-36 sm:w-44 aspect-video rounded-xl overflow-hidden border transition-all cursor-pointer group/thumb text-left focus:ring-2 focus:ring-white focus:outline-none {activeMediaIndex === idx ? 'border-white ring-2 ring-white/25 opacity-100' : 'border-white/10 opacity-60 hover:opacity-100 hover:border-white/40'}"
                onclick={() => {
                  if (activeMediaIndex === idx) {
                    toggleMediaFullscreen();
                  } else {
                    activeMediaIndex = idx;
                    sound.playFocus();
                  }
                }}
                title={item.name}
              >
                <img
                  src={item.type === 'video' ? item.thumbnail : item.url}
                  alt={item.name}
                  class="w-full h-full object-cover"
                />

                {#if item.type === 'video'}
                  <div class="absolute inset-0 bg-black/35 flex items-center justify-center group-hover/thumb:bg-black/15 transition-colors">
                    <div class="w-7 h-7 rounded-full bg-black/80 flex items-center justify-center text-white border border-white/20 shadow-md">
                      <Play class="w-3.5 h-3.5 ml-0.5 fill-white" />
                    </div>
                  </div>
                  <span class="absolute bottom-1.5 inset-x-1.5 text-[10px] font-bold text-white truncate px-1.5 py-0.5 bg-black/75 rounded flex items-center gap-1">
                    <Film class="w-3 h-3 text-amber-400" />
                    <span class="truncate">{item.name}</span>
                  </span>
                {:else}
                  <span class="absolute bottom-1.5 right-1.5 text-[10px] font-mono font-bold text-white/90 px-1.5 py-0.5 bg-black/75 rounded border border-white/10">
                    {idx + 1}
                  </span>
                {/if}
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    <!-- 2-Column Balanced Content Grid (Description on Left, Specs & Requirements on Right) -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-10 pt-4 border-t border-white/[0.06] w-full">
      
      <!-- Left Column: Detailed Description -->
      <div
        data-nav-item
        tabindex="-1"
        role="region"
        aria-label="Об игре"
        class="nav-content-card lg:col-span-2 space-y-6 p-6 rounded-2xl bg-[#0c0e14]/70 border border-white/[0.06] transition-colors focus:outline-none"
      >
        <h2 class="text-sm font-bold uppercase tracking-wider text-white flex items-center gap-2">
          <Info class="w-4 h-4 text-sky-400" />
          <span>Об игре</span>
        </h2>
        
        {#if game.detailedDescription}
          <div class="steam-html-content text-sm text-[#cbd5e1] leading-relaxed w-full">
            {@html game.detailedDescription}
          </div>
        {:else if game.shortDescription}
          <div class="text-sm text-[#cbd5e1] leading-relaxed">
            {@html game.shortDescription}
          </div>
        {:else}
          <div class="text-xs text-[#64748b]">Описание отсутствует</div>
        {/if}
      </div>

      <!-- Right Column: Specs & Requirements -->
      <div class="space-y-6">
        
        <!-- Quick Game Info Card -->
        <div
          data-nav-item
          tabindex="-1"
          role="region"
          aria-label="Информация"
          class="nav-content-card space-y-4 bg-[#0c0e14] p-6 rounded-2xl border border-white/[0.06] transition-colors focus:outline-none"
        >
          <h3 class="text-xs font-bold uppercase tracking-wider text-[#8e95a2] flex items-center gap-2">
            <Info class="w-3.5 h-3.5 text-sky-400" />
            <span>Информация</span>
          </h3>

          <div class="space-y-3 text-xs divide-y divide-white/[0.04]">
            <div class="pt-2 flex items-center justify-between">
              <span class="text-[#8e95a2]">Статус</span>
              <span class="font-bold {pageDetails?.isInstalled ? 'text-emerald-400' : 'text-[#94a3b8]'}">
                {pageDetails?.isInstalled ? 'Установлено' : 'Не установлено'}
              </span>
            </div>

            {#if game.developers && game.developers.length > 0}
              <div class="pt-2 flex items-center justify-between">
                <span class="text-[#8e95a2]">Разработчик</span>
                <span class="font-bold text-white text-right">{game.developers.join(', ')}</span>
              </div>
            {/if}

            {#if game.publishers && game.publishers.length > 0}
              <div class="pt-2 flex items-center justify-between">
                <span class="text-[#8e95a2]">Издатель</span>
                <span class="font-bold text-white text-right">{game.publishers.join(', ')}</span>
              </div>
            {/if}

            {#if game.releaseDate || game.steamReleaseDate}
              <div class="pt-2 flex items-center justify-between">
                <span class="text-[#8e95a2]">Дата выхода</span>
                <span class="font-bold text-white">{game.releaseDate || game.steamReleaseDate}</span>
              </div>
            {/if}

            <div class="pt-2 flex items-center justify-between">
              <span class="text-[#8e95a2]">Контроллер</span>
              <span class="font-bold text-sky-400">{game.controllerSupport === 'full' ? 'Полная поддержка' : 'Клавиатура'}</span>
            </div>

            {#if game.sizeDisplay || game.sizeStr}
              <div class="pt-2 flex items-center justify-between">
                <span class="text-[#8e95a2]">Размер</span>
                <span class="font-mono font-bold text-white">{game.sizeDisplay || game.sizeStr}</span>
              </div>
            {/if}

            <div class="pt-2 flex items-center justify-between">
              <span class="text-[#8e95a2]">{game.steamAppId < 0 ? 'SteamGridDB ID' : 'Steam AppID'}</span>
              <span class="font-mono text-white">{game.steamAppId > 0 ? game.steamAppId : (game.steamAppId < 0 ? -game.steamAppId : '—')}</span>
            </div>
          </div>
        </div>

        <!-- System Requirements Card -->
        {#if game.pcRequirements}
          <div
            data-nav-item
            tabindex="-1"
            role="region"
            aria-label="Системные требования"
            class="nav-content-card space-y-4 bg-[#0c0e14] p-6 rounded-2xl border border-white/[0.06] transition-colors focus:outline-none"
          >
            <h3 class="text-xs font-bold uppercase tracking-wider text-[#8e95a2] flex items-center gap-2">
              <Cpu class="w-3.5 h-3.5 text-sky-400" />
              <span>Системные требования</span>
            </h3>
            <div class="steam-html-content text-xs text-[#cbd5e1] leading-relaxed">
              {@html game.pcRequirements}
            </div>
          </div>
        {/if}

      </div>
    </div>

    <!-- Bottom Back to Top Navigation Anchor -->
    <div class="pt-4 pb-12 flex justify-center">
      <button
        data-nav-item
        type="button"
        class="px-6 py-2.5 rounded-xl bg-white/[0.06] hover:bg-white/10 text-xs font-bold text-white border border-white/10 flex items-center gap-2 cursor-pointer focus:ring-2 focus:ring-white focus:outline-none transition-colors"
        onclick={scrollToTop}
      >
        <ChevronUp class="w-4 h-4" />
        <span>Наверх страницы [A]</span>
      </button>
    </div>

  </div>
  {/if}
{/if}
</div>

<!-- Steam Candidate Matching Modal -->
{#if isSteamModalOpen}
  <div data-nav-zone="modal" class="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex items-center justify-center p-6 animate-fade-in">
    <div class="w-full max-w-2xl bg-[#07080a] border border-white/10 rounded-2xl p-6 space-y-6 shadow-2xl max-h-[85vh] flex flex-col">
      
      <!-- Modal Header -->
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-base font-bold text-white">Привязка данных Steam</h2>
          <p class="text-xs text-[#8e95a2] mt-0.5">Выберите подходящую игру из Steam или введите AppID вручную</p>
        </div>
        <button
          data-nav-item
          class="p-2 rounded-lg text-[#8e95a2] hover:text-white hover:bg-white/10 cursor-pointer focus:ring-2 focus:ring-white focus:outline-none"
          onclick={closeSteamModal}
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Search Input -->
      <div class="flex items-center gap-2">
        <input
          data-nav-item
          type="text"
          bind:value={steamSearchTerm}
          placeholder="Поиск в базе Steam..."
          class="flex-1 bg-[#11141c] text-white text-xs rounded-xl px-4 py-3 border border-white/[0.08] focus:border-sky-400 focus:outline-none"
          onkeydown={(e) => {
            if (e.key === 'Enter') performSteamSearch();
          }}
        />
        <button
          data-nav-item
          disabled={isSearchingSteam}
          class="px-5 py-3 rounded-xl bg-white/10 hover:bg-white/20 text-xs font-bold text-white flex items-center gap-2 cursor-pointer disabled:opacity-50 focus:ring-2 focus:ring-white focus:outline-none"
          onclick={performSteamSearch}
        >
          {#if isSearchingSteam}
            <RefreshCw class="w-3.5 h-3.5 animate-spin" />
          {:else}
            <Search class="w-3.5 h-3.5" />
          {/if}
          <span>Поиск</span>
        </button>
      </div>

      <!-- Candidates List -->
      <div class="flex-1 overflow-y-auto space-y-2 max-h-64 pr-1">
        {#if isSearchingSteam}
          <div class="py-8 text-center text-xs text-[#8e95a2]">Поиск вариантов в Steam...</div>
        {:else if steamCandidates.length === 0}
          <div class="py-8 text-center text-xs text-[#8e95a2]">Совпадений не найдено</div>
        {:else}
          {#each steamCandidates as cand}
            <button
              data-nav-item
              class="w-full flex items-center justify-between p-3 rounded-xl bg-[#0d1017] hover:bg-white/10 border border-white/[0.06] text-left cursor-pointer transition-colors focus:ring-2 focus:ring-white focus:outline-none"
              onclick={() => handleLinkAppId(cand.appId)}
            >
              <div class="flex items-center gap-3">
                {#if cand.tinyImage}
                  <img src={cand.tinyImage} alt="" class="w-16 h-8 object-cover rounded" />
                {/if}
                <div>
                  <div class="text-xs font-bold text-white">{cand.name}</div>
                  <div class="text-[10px] text-[#8e95a2] font-mono">AppID: {cand.appId}</div>
                </div>
              </div>

              <span class="text-xs font-bold text-sky-400">Выбрать [A]</span>
            </button>
          {/each}
        {/if}
      </div>

      <!-- Direct AppID & Unlink Controls -->
      <div class="pt-4 border-t border-white/[0.06] flex items-center justify-between gap-4">
        <div class="flex items-center gap-2">
          <input
            data-nav-item
            type="text"
            bind:value={directAppIdInput}
            placeholder="Точный AppID"
            class="w-32 bg-[#11141c] text-white text-xs font-mono rounded-xl px-3 py-2 border border-white/[0.08] focus:border-sky-400 focus:outline-none"
          />
          <button
            data-nav-item
            class="px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 text-xs font-semibold text-white cursor-pointer focus:ring-2 focus:ring-white focus:outline-none"
            onclick={handleApplyDirectAppId}
          >
            Применить
          </button>
        </div>

        {#if game.steamAppId}
          <button
            data-nav-item
            class="text-xs text-rose-400 hover:underline cursor-pointer focus:ring-2 focus:ring-white focus:outline-none"
            onclick={handleUnlinkMetadata}
          >
            Отвязать метаданные
          </button>
        {/if}
      </div>

    </div>
  </div>
{/if}