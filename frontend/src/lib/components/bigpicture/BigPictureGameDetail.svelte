<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    ArrowLeft,
    Play,
    DownloadSimple as Download,
    Check,
    Star,
    Folder,
    FolderOpen,
    SpeakerHigh as Volume2,
    SpeakerSlash as VolumeX,
    Pause,
    CaretLeft as ChevronLeft,
    CaretRight as ChevronRight,
    CaretDown as ChevronDown,
    X,
    MagnifyingGlass as Search,
    ArrowsClockwise as RefreshCw,
    PencilSimple as Edit3,
    Disc,
    ArrowDown,
    FilmStrip as Film,
    Image as ImageIcon,
    UploadSimple,
    Info,
    Magnet,
    SteamLogo,
    ThumbsUp,
    ThumbsDown,
    ChatText,
    ArrowSquareOut as ExternalLink
  } from 'phosphor-svelte';
  import Hls from 'hls.js';
  import { sound } from '../../navigation/audio';
  import * as AppAPI from '../../../../wailsjs/go/main/App';
  import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime';
  import type {
    GameEntity,
    SteamMovie,
    SteamCandidateItem,
    GamePageDetails,
    SteamAnonymizedReview,
    SteamReviewsResponse
  } from '../../types/game';
  import { formatTorrentSourceName } from '../../utils/sourceFormatter';
  import { getDisplayTitle } from '../../utils/titleUtils';
  import { downloadsStore } from '../../stores/downloads.svelte';
  import { formatSteamReviewBBCode, formatReviewDate } from '../../utils/steamReviewFormatter';


  let {
    game = null as any,
    downloadPath = 'C:\\Ducke',
    onBack = () => {},
    onStartDownload = (gameId: number, targetPath: string) => {},
    onSelectFolder = async (): Promise<string> => '',
    onUpdateDownloadPath = (path: string) => {},
    activeDownloads = [] as any[]
  } = $props();

  let customDownloadPath = $state<string>('');
  let pageDetails = $state<GamePageDetails | null>(null);
  let isMounted = true;

  // Active game entity (enriched or prop)
  let activeGame = $derived(pageDetails?.game || game);

  // Logo load failure tracker
  let logoFailedMap = $state<Record<number, boolean>>({});

  // Steam candidate search modal state
  let isSteamModalOpen = $state<boolean>(false);
  let steamSearchTerm = $state<string>('');
  let steamCandidates = $state<SteamCandidateItem[]>([]);
  let isSearchingSteam = $state<boolean>(false);
  let directAppIdInput = $state<string>('');
  let isSavingSteam = $state<boolean>(false);

  // Game Description Modal state (Triggered by [X])
  let isDescriptionModalOpen = $state<boolean>(false);
  let descTab = $state<'about' | 'details' | 'specs' | 'variants' | 'reviews'>('about');
  let descScrollEl = $state<HTMLDivElement | null>(null);

  // Steam Reviews state (Big Picture)
  let steamReviews = $state<SteamAnonymizedReview[]>([]);
  let steamReviewsCursor = $state<string>('*');
  let steamReviewsHasMore = $state<boolean>(false);
  let steamReviewsTotal = $state<number>(0);
  let isLoadingReviews = $state<boolean>(false);
  let isLoadingMoreReviews = $state<boolean>(false);
  let reviewLanguage = $state<'russian' | 'all'>('russian');
  let lastReviewsAppId = $state<number>(0);

  function handleOpenSteamStore() {
    sound.playSelect();
    const appId = activeGame?.steamAppId;
    if (!appId || appId <= 0) return;
    const storeUrl = `https://store.steampowered.com/app/${appId}`;
    try {
      BrowserOpenURL(storeUrl);
    } catch {
      AppAPI.OpenURL(storeUrl);
    }
  }

  async function loadSteamReviews(reset = false) {
    const appId = activeGame?.steamAppId;
    if (!appId || appId <= 0) {
      steamReviews = [];
      steamReviewsHasMore = false;
      steamReviewsTotal = 0;
      return;
    }

    if (reset) {
      steamReviewsCursor = '*';
      isLoadingReviews = true;
    } else {
      isLoadingMoreReviews = true;
    }

    try {
      const res: SteamReviewsResponse = await AppAPI.GetSteamReviews(appId, reset ? '*' : steamReviewsCursor, reviewLanguage);
      if (res) {
        if (reset) {
          steamReviews = res.reviews || [];
        } else {
          const existingIds = new Set(steamReviews.map(r => r.id));
          const newOnes = (res.reviews || []).filter(r => !existingIds.has(r.id));
          steamReviews = [...steamReviews, ...newOnes];
        }
        steamReviewsCursor = res.cursor || '';
        steamReviewsHasMore = !!res.hasMore;
        steamReviewsTotal = res.totalReviews || steamReviews.length;
      }
    } catch (e) {
      console.error('Failed to load Steam reviews (BigPicture):', e);
    } finally {
      isLoadingReviews = false;
      isLoadingMoreReviews = false;
    }
  }

  function handleLanguageChange(lang: 'russian' | 'all') {
    sound.playSelect();
    if (reviewLanguage === lang) return;
    reviewLanguage = lang;
    loadSteamReviews(true);
  }

  $effect(() => {
    const currentAppId = activeGame?.steamAppId || 0;
    if (currentAppId > 0 && currentAppId !== lastReviewsAppId) {
      lastReviewsAppId = currentAppId;
      loadSteamReviews(true);
    } else if (currentAppId <= 0 && lastReviewsAppId !== 0) {
      lastReviewsAppId = 0;
      steamReviews = [];
      steamReviewsHasMore = false;
      steamReviewsTotal = 0;
    }
  });

  function openDescriptionModal() {
    sound.playSelect();
    isDescriptionModalOpen = true;
    window.dispatchEvent(new CustomEvent('app:modal-opened'));
  }

  function closeDescriptionModal() {
    sound.playBack();
    if (isDescriptionModalOpen) {
      isDescriptionModalOpen = false;
      window.dispatchEvent(new CustomEvent('app:modal-closed'));
    }
  }

  function cycleDescTab(delta: number) {
    const tabs: ('about' | 'details' | 'specs' | 'variants' | 'reviews')[] = ['about', 'details', 'specs'];
    if (activeVariantsList && activeVariantsList.length > 1) tabs.push('variants');
    if (activeGame?.steamAppId > 0) tabs.push('reviews');
    const curIdx = tabs.indexOf(descTab);
    const nextIdx = (curIdx + delta + tabs.length) % tabs.length;
    descTab = tabs[nextIdx];
    sound.playTab();
  }

  // Download Variant Wizard state
  let isDownloadWizardOpen = $state<boolean>(false);

  function openDownloadWizard() {
    sound.playSelect();
    if (activeVariantsList && activeVariantsList.length > 0 && !selectedVariantId) {
      selectedVariantId = activeVariantsList[0].id;
    }
    isDownloadWizardOpen = true;
    window.dispatchEvent(new CustomEvent('app:modal-opened'));
  }

  function closeDownloadWizard() {
    sound.playBack();
    if (isDownloadWizardOpen) {
      isDownloadWizardOpen = false;
      window.dispatchEvent(new CustomEvent('app:modal-closed'));
    }
  }

  function cycleWizardVariant(delta: number) {
    if (!activeVariantsList || activeVariantsList.length <= 1) return;
    const curId = selectedVariantId || activeVariantsList[0]?.id;
    const idx = activeVariantsList.findIndex((v: any) => v.id === curId);
    const nextIdx = (idx + delta + activeVariantsList.length) % activeVariantsList.length;
    selectedVariantId = activeVariantsList[nextIdx]?.id;
    sound.playMove();
  }

  function confirmDownloadVariant(variantId: number) {
    selectedVariantId = variantId;
    closeDownloadWizard();
    sound.playSelect();
    onStartDownload(variantId, customDownloadPath);
  }

  // Favorites backlog state
  let isFavoriteDropdownOpen = $state<boolean>(false);
  let favoriteDropdownContainerEl = $state<HTMLDivElement | null>(null);
  let favoriteDropdownTriggerEl = $state<HTMLButtonElement | null>(null);

  // Variant selector state
  let selectedVariantId = $state<number | null>(null);
  let isVariantDropdownOpen = $state<boolean>(false);
  let variantDropdownTriggerEl = $state<HTMLButtonElement | null>(null);
  let variantDropdownContainerEl = $state<HTMLDivElement | null>(null);

  let activeVariantsList = $derived(activeGame?.variants || []);
  let activeVariant = $derived.by(() => {
    if (!activeVariantsList || activeVariantsList.length === 0) return null;
    return activeVariantsList.find((v: any) => v.id === selectedVariantId) || activeVariantsList[0];
  });

  let downloadSourceInfo = $derived.by(() => {
    const v = activeVariant || activeGame || game;
    if (!v) return null;
    const isTorrent = v.sourceType === 'torrent' || !!v.magnetUri;
    const rawSrc = (v.torrentSource || '').trim();
    const resolvedName = formatTorrentSourceName(rawSrc);
    if (isTorrent) {
      return {
        type: 'torrent',
        name: resolvedName || 'Торрент',
        shortName: resolvedName || 'Торрент',
        badgeClass: 'bg-white/[0.04] border-white/10 text-[#cbd5e1]'
      };
    }
    return {
      type: 'ftp',
      name: 'FTP-сервер',
      shortName: 'FTP',
      badgeClass: 'bg-white/[0.04] border-white/10 text-[#cbd5e1]'
    };
  });

  let variantSeeds = $state<Record<number, { seeders: number; leechers: number; loading: boolean }>>({});
  let lastSeedsFetchedGameId = 0;

  function formatSeedsCount(seeds: number): string {
    return `${seeds}`;
  }

  async function loadTorrentSeedsForGame(g: GameEntity | null | undefined) {
    if (!g) return;
    const gid = g.id;

    const allVariants = (g.variants && g.variants.length > 0) ? g.variants : [g];
    const queries: { id: number; magnetUri: string }[] = [];

    for (const v of allVariants) {
      const magnet = v.magnetUri || (v.id === gid ? g.magnetUri : '') || '';
      const isTorrent = v.sourceType === 'torrent' || !!magnet;
      if (isTorrent && magnet) {
        queries.push({ id: v.id, magnetUri: magnet });
        if (!variantSeeds[v.id]) {
          variantSeeds[v.id] = { seeders: 0, leechers: 0, loading: true };
        }
      }
    }

    if (queries.length === 0) return;
    lastSeedsFetchedGameId = gid;

    try {
      const results = await AppAPI.GetTorrentSeedsBatch(queries as any);
      if (lastSeedsFetchedGameId === gid && results) {
        for (const [idStr, res] of Object.entries(results)) {
          const id = Number(idStr);
          variantSeeds[id] = {
            seeders: res.seeders || 0,
            leechers: res.leechers || 0,
            loading: false
          };
        }
      }
    } catch (err) {
      console.warn('[Seeds] Failed to fetch torrent seeds:', err);
      if (lastSeedsFetchedGameId === gid) {
        for (const q of queries) {
          if (variantSeeds[q.id]?.loading) {
            variantSeeds[q.id].loading = false;
          }
        }
      }
    }
  }

  $effect(() => {
    const g = activeGame || game;
    if (g?.id && g.id !== lastSeedsFetchedGameId) {
      loadTorrentSeedsForGame(g);
    }
  });

  let currentVariantSeedInfo = $derived.by(() => {
    const v = activeVariant || activeGame || game;
    if (!v) return null;
    const isTorrent = v.sourceType === 'torrent' || !!v.magnetUri;
    if (!isTorrent) return null;
    return variantSeeds[v.id] || null;
  });

  // Docked content tabs: 'about' | 'screenshots' | 'specs' | 'variants'
  let activeTab = $state<'about' | 'screenshots' | 'specs' | 'variants'>('about');

  // Media & Screenshot Lightbox
  let activeScreenshotPreview = $state<string | null>(null);
  let lightboxImage = $state<string | null>(null);
  let lightboxIndex = $state<number>(0);

  // Background Ambient Trailer & Theater Mode State
  const AMBIENT_TRAILER_DELAY_MS = 2000;
  let trailerTimer: any = null;
  let isAmbientTrailerActive = $state<boolean>(false);
  let isAmbientTimerElapsed = $state<boolean>(false);
  let isTheaterMode = $state<boolean>(false);
  let isVideoMuted = $state<boolean>(true);
  let isVideoPlaying = $state<boolean>(true);
  let videoHasRenderedFrame = $state<boolean>(false);
  let currentMovieIndex = $state<number>(0);
  let videoBgEl: HTMLVideoElement | null = $state(null);
  let theaterEnterTimestamp = 0;

  // Movie overrides & dynamic loading
  let movieOverrides = $state<Record<number, SteamMovie[]>>({});
  let enrichingGameIds = new Set<number>();
  let isEnrichingCurrentGame = $state<boolean>(false);

  // In-memory cache for fast back-and-forth switching
  const pageDetailsCache = new Map<number, GamePageDetails>();
  let lastGameId: number | null = null;
  let switchSequence = 0;

  $effect(() => {
    if (downloadPath) customDownloadPath = downloadPath;
  });

  let activeDownload = $derived.by(() => {
    const curGame = pageDetails?.game || game;
    if (!curGame) return null;
    return downloadsStore.getDownloadForGame(curGame);
  });

  let isDownloading = $derived(
    (activeDownload && (activeDownload.status === 'downloading' || activeDownload.status === 'queued' || activeDownload.status === 'scanning' || activeDownload.status === 'paused')) ||
    (pageDetails && (pageDetails.downloadStatus === 'downloading' || pageDetails.downloadStatus === 'queued' || pageDetails.downloadStatus === 'scanning' || pageDetails.downloadStatus === 'paused'))
  );

  let isCompleted = $derived(
    (activeDownload && activeDownload.status === 'completed') ||
    downloadsStore.isGameInstalled(pageDetails?.game || game) ||
    pageDetails?.isInstalled
  );

  let currentFavoriteStatus = $derived(activeGame?.favoriteStatus || '');

  // Fast URL sanitizer ensuring Akamai/Steam static CDN
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

  // Movie list derivation
  let movieList = $derived.by<SteamMovie[]>(() => {
    if (!game) return [];
    const ov = game.id ? movieOverrides[game.id] : null;
    if (ov && ov.length > 0) return ov;
    const pdm = pageDetails?.game?.movies;
    if (pdm && pdm.length > 0) return pdm;
    if (game.movies && game.movies.length > 0) return game.movies;
    return [];
  });

  function pickMovieSource(m: SteamMovie): string {
    if (!m) return '';
    // Prefer HLS (H.264 fMP4) which plays reliably via hls.js with native H.264 hardware decoding
    if (m.hls) return sanitizeMediaUrl(m.hls);
    // Direct MP4 fallback if not a broken steam placeholder
    if (m.mp4 && !m.mp4.includes('/apps/') && !m.mp4.includes('movie_max.mp4')) return sanitizeMediaUrl(m.mp4);
    if (m.webm) return sanitizeMediaUrl(m.webm);
    if (m.mp4) return sanitizeMediaUrl(m.mp4);
    return '';
  }

  let activeMovie = $derived.by(() => {
    if (movieList.length === 0) return null;
    const m = movieList[currentMovieIndex % movieList.length];
    if (!m) return null;
    const src = pickMovieSource(m);
    let thumb = sanitizeMediaUrl(m.thumbnail || '');
    return {
      ...m,
      src,
      thumbnail: thumb
    };
  });

  // Screenshots array
  let gameScreenshots = $derived.by<string[]>(() => {
    const sc = pageDetails?.game?.screenshots?.length
      ? pageDetails.game.screenshots
      : (game?.screenshots || []);
    if (!Array.isArray(sc)) return [];
    return sc
      .map((s: any) => (typeof s === 'string' ? s : s?.url))
      .filter((s: string) => typeof s === 'string' && s.trim() !== '')
      .map((s: string) => sanitizeMediaUrl(s));
  });

  export interface UnifiedMediaItem {
    type: 'video' | 'screenshot';
    title: string;
    url: string;
    thumb?: string;
    movie?: SteamMovie;
  }

  // Unified Media list combining trailers then screenshots
  let unifiedMediaList = $derived.by<UnifiedMediaItem[]>(() => {
    const list: UnifiedMediaItem[] = [];
    movieList.forEach((m, idx) => {
      const src = pickMovieSource(m);
      if (src) {
        list.push({
          type: 'video',
          title: m.name || `Трейлер ${idx + 1}`,
          url: src,
          thumb: sanitizeMediaUrl(m.thumbnail || ''),
          movie: m
        });
      }
    });
    gameScreenshots.forEach((sc, idx) => {
      list.push({
        type: 'screenshot',
        title: `Скриншот ${idx + 1}`,
        url: sc,
        thumb: sc
      });
    });
    return list;
  });

  let currentMediaIndex = $state<number>(0);
  let activeMedia = $derived.by<UnifiedMediaItem | null>(() => {
    if (unifiedMediaList.length === 0) return null;
    const idx = ((currentMediaIndex % unifiedMediaList.length) + unifiedMediaList.length) % unifiedMediaList.length;
    return unifiedMediaList[idx] || null;
  });

  function formatReviewsCount(count: number): string {
    if (!count || count <= 0) return '';
    return count.toLocaleString('ru-RU');
  }

  // Still artwork background
  let currentBackdropUrl = $derived.by(() => {
    if (activeScreenshotPreview) return activeScreenshotPreview;
    if (pageDetails?.backgroundUrl) return sanitizeMediaUrl(pageDetails.backgroundUrl);
    if (!game) return '';
    if (game.backgroundImage) return sanitizeMediaUrl(game.backgroundImage);
    if (game.headerImage) return sanitizeMediaUrl(game.headerImage);
    if (game.steamAppId && game.steamAppId > 0) {
      return `https://shared.steamstatic.com/store_item_assets/steam/apps/${game.steamAppId}/page_bg_generated_v6b.jpg`;
    }
    return '';
  });

  function getAppAPI(): any {
    if (typeof window !== 'undefined' && (window as any)?.go?.main?.App) {
      return (window as any).go.main.App;
    }
    return AppAPI;
  }

  // --------------------------------------------------------------------------
  // Video Player & Hls Logic
  // --------------------------------------------------------------------------
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
      if (err?.name === 'AbortError') return;
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
        enableWorker: false,
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

  function handleTrailerEnded() {
    if (isTheaterMode) {
      if (unifiedMediaList.length > 1) {
        nextMedia();
      } else if (videoBgEl) {
        videoBgEl.currentTime = 0;
        videoBgEl.play().catch(() => {});
      }
      return;
    }
    if (movieList.length <= 1) {
      if (videoBgEl) {
        videoBgEl.currentTime = 0;
        videoBgEl.play().catch(() => {});
      }
      return;
    }
    currentMovieIndex = (currentMovieIndex + 1) % movieList.length;
  }

  function syncMediaPlayback() {
    if (!isTheaterMode) return;
    const item = unifiedMediaList[currentMediaIndex];
    if (!item) return;

    if (item.type === 'video') {
      const mIdx = movieList.findIndex((m) => m === item.movie);
      if (mIdx !== -1) {
        currentMovieIndex = mIdx;
      }
      isVideoMuted = false;
      if (videoBgEl) {
        videoBgEl.muted = false;
      }
      loadMediaSource(item.url);
      safePlay();
    } else {
      // Viewing screenshot: pause and mute background video
      isVideoMuted = true;
      if (videoBgEl) {
        videoBgEl.muted = true;
      }
      safePause();
    }
  }

  function nextMedia() {
    if (unifiedMediaList.length <= 1) return;
    sound.playMove();
    currentMediaIndex = (currentMediaIndex + 1) % unifiedMediaList.length;
    syncMediaPlayback();
  }

  function prevMedia() {
    if (unifiedMediaList.length <= 1) return;
    sound.playMove();
    currentMediaIndex = (currentMediaIndex - 1 + unifiedMediaList.length) % unifiedMediaList.length;
    syncMediaPlayback();
  }

  function nextTrailer() {
    nextMedia();
  }

  function prevTrailer() {
    prevMedia();
  }

  function enterTheaterMode(targetIndex?: number) {
    if (unifiedMediaList.length === 0) return;
    if (typeof targetIndex === 'number' && targetIndex >= 0 && targetIndex < unifiedMediaList.length) {
      currentMediaIndex = targetIndex;
    } else {
      currentMediaIndex = 0;
    }
    theaterEnterTimestamp = Date.now();
    sound.playSelect();
    if (trailerTimer) clearTimeout(trailerTimer);
    isAmbientTimerElapsed = true;
    isAmbientTrailerActive = true;
    isTheaterMode = true;
    syncMediaPlayback();
    window.dispatchEvent(new CustomEvent('app:theater-mode', { detail: { active: true } }));
  }

  function exitTheaterMode() {
    if (Date.now() - theaterEnterTimestamp < 350) return;
    sound.playBack();
    isTheaterMode = false;
    isVideoMuted = true;
    if (videoBgEl) {
      videoBgEl.muted = true;
    }
    // Restore ambient trailer playback if movies exist
    if (movieList.length > 0) {
      const curMovie = movieList[currentMovieIndex % movieList.length];
      if (curMovie) {
        loadMediaSource(pickMovieSource(curMovie));
        safePlay();
      }
    } else {
      safePause();
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

  // --------------------------------------------------------------------------
  // Data Loading & Lifecycle
  // --------------------------------------------------------------------------
  $effect(() => {
    const curId = game?.id;
    if (!curId || !game) {
      pageDetails = null;
      lastGameId = null;
      return;
    }

    if (curId !== lastGameId) {
      lastGameId = curId;
      lastSeedsFetchedGameId = 0;
      const seq = ++switchSequence;

      // Reset interactive state
      currentMovieIndex = 0;
      activeScreenshotPreview = null;
      isVariantDropdownOpen = false;
      isFavoriteDropdownOpen = false;
      activeTab = 'about';

      if (game?.variants && game.variants.length > 0) {
        selectedVariantId = game.variants[0].id;
      } else {
        selectedVariantId = curId;
      }

      // Reset video
      if (trailerTimer) clearTimeout(trailerTimer);
      isAmbientTimerElapsed = false;
      isAmbientTrailerActive = false;
      if (isTheaterMode) {
        isTheaterMode = false;
        window.dispatchEvent(new CustomEvent('app:theater-mode', { detail: { active: false } }));
      }
      isVideoMuted = true;
      stopVideoPlayback();

      // Ambient timer
      trailerTimer = setTimeout(() => {
        if (isMounted && game?.id === curId) {
          isAmbientTimerElapsed = true;
        }
      }, AMBIENT_TRAILER_DELAY_MS);

      // Populate details
      if (pageDetailsCache.has(curId)) {
        pageDetails = pageDetailsCache.get(curId)!;
      } else {
        pageDetails = {
          game,
          downloadStatus: 'none',
          downloadProgress: undefined,
          localPath: '',
          isInstalled: false,
          logoUrl: game.iconUrl || '',
          bannerUrl: game.backgroundImage || game.headerImage || '',
          coverUrl: game.capsuleImage || '',
          backgroundUrl: game.backgroundImage || ''
        };
      }

      fetchGamePageDetails(curId, seq);
      resolveMoviesIfNeeded(game, seq);
      triggerPriorityEnrichmentIfNeeded(game, curId, seq);
    }
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
            downloadProgress: undefined,
            localPath: '',
            isInstalled: false,
            logoUrl: '',
            bannerUrl: '',
            coverUrl: '',
            backgroundUrl: ''
          };
        }
      }

      if (!isMounted || seq !== switchSequence) return;
      if (res) {
        pageDetailsCache.set(gameId, res);
        pageDetails = res;
        if (res.game) {
          loadTorrentSeedsForGame(res.game);
        }
      }
    } catch (err) {
      console.warn('[BigPictureGameDetail] Failed to get page details:', err);
    }
  }

  function resolveMoviesIfNeeded(g: GameEntity, seq: number) {
    if (!g.steamAppId || g.steamAppId <= 0) return;
    const currentMovies = movieOverrides[g.id] || g.movies || [];
    if (Array.isArray(currentMovies) && currentMovies.length > 0) return;

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

    const app = getAppAPI();
    if (app && typeof app.EnrichGameNow === 'function') {
      app.EnrichGameNow(curId).then((enriched: any) => {
        enrichingGameIds.delete(curId);
        if (isMounted && seq === switchSequence) {
          isEnrichingCurrentGame = false;
          if (enriched) {
            fetchGamePageDetails(curId, seq);
          }
        }
      }).catch(() => {
        enrichingGameIds.delete(curId);
        if (isMounted && seq === switchSequence) {
          isEnrichingCurrentGame = false;
        }
      });
    } else {
      isEnrichingCurrentGame = false;
    }
  }

  // --------------------------------------------------------------------------
  // User Actions
  // --------------------------------------------------------------------------
  async function handlePrimaryAction() {
    if (!activeGame) return;
    sound.playSelect();
    if (isCompleted) {
      try {
        const app = getAppAPI();
        if (app && typeof app.LaunchGameByGameID === 'function') {
          await app.LaunchGameByGameID(activeGame.id);
        } else if (app && typeof app.LaunchGame === 'function') {
          await app.LaunchGame(activeGame.folderName || '');
        }
      } catch (e) {
        console.error(e);
      }
    } else if (isDownloading) {
      // Already downloading
    } else {
      if (activeVariantsList && activeVariantsList.length > 1) {
        openDownloadWizard();
      } else {
        onStartDownload(selectedVariantId || activeGame.id, customDownloadPath);
      }
    }
  }

  async function handleOpenGameFolder() {
    if (!activeGame) return;
    sound.playSelect();
    try {
      const app = getAppAPI();
      if (app && typeof app.OpenGameFolder === 'function') {
        await app.OpenGameFolder(activeGame.id);
      }
    } catch (e) {
      console.error(e);
    }
  }

  async function handleBrowseFolder() {
    sound.playSelect();
    try {
      let selected = await onSelectFolder();
      if (selected) {
        if (/^[a-zA-Z]:\\?$/.test(selected)) {
          selected = `${selected[0].toUpperCase()}:\\Ducke`;
        }
        customDownloadPath = selected;
        onUpdateDownloadPath(selected);
      }
    } catch (e) {
      console.error(e);
    }
  }

  async function handleToggleFavoriteStatus(status: string) {
    if (!activeGame?.id) return;
    try {
      sound.playSelect();
      isFavoriteDropdownOpen = false;
      await AppAPI.SetFavoriteStatus(activeGame.id, status);
      activeGame.favoriteStatus = status;
      if (game) game.favoriteStatus = status;
    } catch (e) {
      console.error('Failed to set favorite status:', e);
    }
  }

  async function handleRemoveFavorite() {
    if (!activeGame?.id) return;
    try {
      sound.playFocus();
      isFavoriteDropdownOpen = false;
      await AppAPI.RemoveFromFavorites(activeGame.id);
      activeGame.favoriteStatus = '';
      if (game) game.favoriteStatus = '';
    } catch (e) {
      console.error('Failed to remove from favorites:', e);
    }
  }

  function switchTab(tab: 'about' | 'screenshots' | 'specs' | 'variants') {
    if (activeTab === tab) return;
    sound.playTab();
    activeTab = tab;
  }

  function cycleTab(delta: number) {
    const tabs: ('about' | 'screenshots' | 'specs' | 'variants')[] = ['about'];
    if (gameScreenshots.length > 0) tabs.push('screenshots');
    tabs.push('specs');
    if (activeVariantsList && activeVariantsList.length > 1) tabs.push('variants');

    const curIdx = tabs.indexOf(activeTab);
    const nextIdx = (curIdx + delta + tabs.length) % tabs.length;
    switchTab(tabs[nextIdx]);
  }

  // Lightbox
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

  // Steam candidate search modal
  async function openSteamModal() {
    sound.playSelect();
    const raw = activeGame?.steamTitle || activeGame?.cleanTitle || activeGame?.rawName || activeGame?.folderName || '';
    const cleaned = raw
      .replace(/\[.*?\]|\(.*?\)|[\{\}]/g, ' ')
      .replace(/\b(v\s*\d+([._\s]\d+)*|build\s*\d+|patch\s*\d+|update\s*\d+)\b/gi, ' ')
      .replace(/\b(19[7-9]\d|20[0-3]\d)\b/g, ' ')
      .replace(/\s+/g, ' ')
      .trim();
    steamSearchTerm = cleaned || raw;
    directAppIdInput = (activeGame?.steamAppId && activeGame.steamAppId > 0) ? String(activeGame.steamAppId) : '';
    steamCandidates = [];
    isSteamModalOpen = true;
    window.dispatchEvent(new CustomEvent('app:modal-opened'));
    await performSteamSearch();
  }

  function closeSteamModal() {
    sound.playBack();
    if (isSteamModalOpen) {
      isSteamModalOpen = false;
      window.dispatchEvent(new CustomEvent('app:modal-closed'));
    }
  }

  async function performSteamSearch() {
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
      if (activeGame) activeGame.steamAppId = appId;
      closeSteamModal();
      fetchGamePageDetails(game.id, ++switchSequence);
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
      if (activeGame) activeGame.steamAppId = 0;
      closeSteamModal();
      fetchGamePageDetails(game.id, ++switchSequence);
    } catch (e) {
      console.error(e);
    } finally {
      isSavingSteam = false;
    }
  }

  // Window pointer down dismiss
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

  // Keyboard and gamepad listener
  onMount(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (isSteamModalOpen) {
        if (e.key === 'Escape' || e.key === 'Backspace') {
          closeSteamModal();
          e.preventDefault();
          e.stopPropagation();
        }
        return;
      }

      if (isDescriptionModalOpen) {
        if (e.key === 'Escape' || e.key === 'Backspace' || e.key === 'x' || e.key === 'X') {
          closeDescriptionModal();
          e.preventDefault();
          e.stopPropagation();
        } else if (e.key === 'q' || e.key === 'Q' || e.key === 'ArrowLeft') {
          cycleDescTab(-1);
          e.preventDefault();
        } else if (e.key === 'e' || e.key === 'E' || e.key === 'ArrowRight') {
          cycleDescTab(1);
          e.preventDefault();
        } else if (e.key === 'ArrowUp') {
          descScrollEl?.scrollBy({ top: -180, behavior: 'smooth' });
          e.preventDefault();
        } else if (e.key === 'ArrowDown') {
          descScrollEl?.scrollBy({ top: 180, behavior: 'smooth' });
          e.preventDefault();
        }
        return;
      }

      if (isDownloadWizardOpen) {
        if (e.key === 'Escape' || e.key === 'Backspace') {
          closeDownloadWizard();
          e.preventDefault();
          e.stopPropagation();
        } else if (e.key === 'ArrowUp') {
          cycleWizardVariant(-1);
          e.preventDefault();
        } else if (e.key === 'ArrowDown') {
          cycleWizardVariant(1);
          e.preventDefault();
        } else if (e.key === 'Enter') {
          const vId = selectedVariantId || activeVariantsList[0]?.id;
          if (vId) confirmDownloadVariant(vId);
          e.preventDefault();
        } else if (e.key === 'x' || e.key === 'X') {
          handleBrowseFolder();
          e.preventDefault();
        }
        return;
      }

      if (isTheaterMode) {
        if (e.key === 'ArrowDown' || e.key === 'Escape' || e.key === 'Backspace') {
          exitTheaterMode();
          e.preventDefault();
          e.stopPropagation();
        } else if (e.key === ' ' || e.key === 'Enter') {
          togglePlayPause();
          e.preventDefault();
        } else if (e.key === 'm' || e.key === 'M') {
          toggleMute();
          e.preventDefault();
        } else if (e.key === 'ArrowRight') {
          nextMedia();
          e.preventDefault();
        } else if (e.key === 'ArrowLeft') {
          prevMedia();
          e.preventDefault();
        }
        return;
      }

      if (isFavoriteDropdownOpen) {
        if (e.key === 'Escape' || e.key === 'Backspace') {
          isFavoriteDropdownOpen = false;
          e.preventDefault();
          e.stopPropagation();
        }
        return;
      }

      // Root game detail Escape or Backspace exits to library
      if (e.key === 'Escape' || e.key === 'Backspace') {
        sound.playBack();
        onBack();
        e.preventDefault();
        e.stopPropagation();
        return;
      }

      if (e.key === 'x' || e.key === 'X') {
        openDescriptionModal();
        e.preventDefault();
        return;
      }

      if (e.key === 'y' || e.key === 'Y') {
        isFavoriteDropdownOpen = !isFavoriteDropdownOpen;
        e.preventDefault();
        return;
      }

      if (e.key === 'ArrowUp' && unifiedMediaList.length > 0) {
        enterTheaterMode(0);
        e.preventDefault();
      }
    };

    const handleGamepadDir = (e: CustomEvent) => {
      const dir = e.detail?.dir;
      if (!dir) return;

      if (isDescriptionModalOpen) {
        if (dir === 'DOWN') {
          descScrollEl?.scrollBy({ top: 180, behavior: 'smooth' });
        } else if (dir === 'UP') {
          descScrollEl?.scrollBy({ top: -180, behavior: 'smooth' });
        } else if (dir === 'LEFT') {
          cycleDescTab(-1);
        } else if (dir === 'RIGHT') {
          cycleDescTab(1);
        }
        return;
      }

      if (isTheaterMode) {
        if (dir === 'DOWN') {
          exitTheaterMode();
        } else if (dir === 'RIGHT') {
          nextMedia();
        } else if (dir === 'LEFT') {
          prevMedia();
        }
        return;
      }

      if (isDownloadWizardOpen) {
        if (dir === 'UP') {
          cycleWizardVariant(-1);
        } else if (dir === 'DOWN') {
          cycleWizardVariant(1);
        }
        return;
      }

      if (!isDownloadWizardOpen && !isSteamModalOpen && !isFavoriteDropdownOpen) {
        if (dir === 'UP') {
          const isCarouselFocused = document.activeElement?.closest('.media-carousel-container') !== null;
          if (!isCarouselFocused && unifiedMediaList.length > 0) {
            enterTheaterMode(0);
          }
        }
      }
    };

    const handleBtnX = (e: CustomEvent) => {
      if (isSteamModalOpen) return;
      if (isDescriptionModalOpen) {
        closeDescriptionModal();
        e.preventDefault();
        return;
      }
      if (isDownloadWizardOpen) {
        handleBrowseFolder();
        e.preventDefault();
        return;
      }
      if (isTheaterMode) {
        if (activeMedia?.type === 'video') {
          toggleMute();
        }
        e.preventDefault();
        return;
      }
      openDescriptionModal();
      e.preventDefault();
    };

    const handleBtnY = (e: CustomEvent) => {
      if (isTheaterMode || isDescriptionModalOpen || isDownloadWizardOpen) return;
      isFavoriteDropdownOpen = !isFavoriteDropdownOpen;
      e.preventDefault();
    };

    const handleGoBack = (e: CustomEvent) => {
      if (isSteamModalOpen) {
        closeSteamModal();
        e.preventDefault();
        e.stopImmediatePropagation();
      } else if (isDescriptionModalOpen) {
        closeDescriptionModal();
        e.preventDefault();
        e.stopImmediatePropagation();
      } else if (isDownloadWizardOpen) {
        closeDownloadWizard();
        e.preventDefault();
        e.stopImmediatePropagation();
      } else if (isTheaterMode) {
        exitTheaterMode();
        e.preventDefault();
        e.stopImmediatePropagation();
      } else if (isFavoriteDropdownOpen) {
        isFavoriteDropdownOpen = false;
        e.preventDefault();
        e.stopImmediatePropagation();
      } else {
        sound.playBack();
        onBack();
        e.preventDefault();
        e.stopImmediatePropagation();
      }
    };

    const handleModalClose = (e: Event) => {
      if (isSteamModalOpen) {
        closeSteamModal();
        e.preventDefault();
      } else if (isDescriptionModalOpen) {
        closeDescriptionModal();
        e.preventDefault();
      } else if (isDownloadWizardOpen) {
        closeDownloadWizard();
        e.preventDefault();
      } else if (isTheaterMode) {
        exitTheaterMode();
        e.preventDefault();
      }
    };

    const handleSubtabPrev = (e: Event) => {
      if (isDescriptionModalOpen) {
        cycleDescTab(-1);
        e.preventDefault();
      } else if (isTheaterMode) {
        prevMedia();
        e.preventDefault();
      }
    };

    const handleSubtabNext = (e: Event) => {
      if (isDescriptionModalOpen) {
        cycleDescTab(1);
        e.preventDefault();
      } else if (isTheaterMode) {
        nextMedia();
        e.preventDefault();
      }
    };

    const handleGalleryPrev = (e: Event) => {
      if (isDescriptionModalOpen) {
        cycleDescTab(-1);
        e.preventDefault();
      } else if (isTheaterMode) {
        prevMedia();
        e.preventDefault();
      }
    };

    const handleGalleryNext = (e: Event) => {
      if (isDescriptionModalOpen) {
        cycleDescTab(1);
        e.preventDefault();
      } else if (isTheaterMode) {
        nextMedia();
        e.preventDefault();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('pointerdown', handleWindowPointerDown, true);
    window.addEventListener('app:gamepad-dir', handleGamepadDir as EventListener);
    window.addEventListener('app:btn-x', handleBtnX as EventListener);
    window.addEventListener('app:btn-y', handleBtnY as EventListener);
    window.addEventListener('app:go-back', handleGoBack as EventListener, true);
    window.addEventListener('app:modal-close', handleModalClose as EventListener);
    window.addEventListener('app:subtab-prev', handleSubtabPrev as EventListener);
    window.addEventListener('app:subtab-next', handleSubtabNext as EventListener);
    window.addEventListener('app:gallery-prev', handleGalleryPrev as EventListener);
    window.addEventListener('app:gallery-next', handleGalleryNext as EventListener);

    return () => {
      isMounted = false;
      stopAndUnloadVideo();
      if (trailerTimer) clearTimeout(trailerTimer);
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('pointerdown', handleWindowPointerDown, true);
      window.removeEventListener('app:gamepad-dir', handleGamepadDir as EventListener);
      window.removeEventListener('app:btn-x', handleBtnX as EventListener);
      window.removeEventListener('app:btn-y', handleBtnY as EventListener);
      window.removeEventListener('app:go-back', handleGoBack as EventListener, true);
      window.removeEventListener('app:modal-close', handleModalClose as EventListener);
      window.removeEventListener('app:subtab-prev', handleSubtabPrev as EventListener);
      window.removeEventListener('app:subtab-next', handleSubtabNext as EventListener);
      window.removeEventListener('app:gallery-prev', handleGalleryPrev as EventListener);
      window.removeEventListener('app:gallery-next', handleGalleryNext as EventListener);
    };
  });

  onDestroy(() => {
    isMounted = false;
    if (isTheaterMode) {
      window.dispatchEvent(new CustomEvent('app:theater-mode', { detail: { active: false } }));
    }
    stopAndUnloadVideo();
    if (trailerTimer) clearTimeout(trailerTimer);
  });
</script>

<div class="w-full h-full relative overflow-hidden bg-[#07080a] select-none text-white">

  <!-- 1. Fullscreen Cinematic Background Layer -->
  <div class="{isTheaterMode ? 'fixed inset-0 z-[90] w-screen h-screen bg-black pointer-events-auto' : 'absolute inset-0 z-0 pointer-events-none'} overflow-hidden">
    <!-- Static Backdrop Image -->
    {#if currentBackdropUrl && !isTheaterMode}
      <img
        src={currentBackdropUrl}
        alt=""
        class="w-full h-full object-cover object-center transition-all duration-700 ease-out filter blur-md scale-105 {videoHasRenderedFrame ? 'opacity-15' : 'opacity-35'}"
        decoding="async"
      />
    {/if}

    <!-- Live Fullscreen Background Video Trailer (Dimmed and blurred until [↑] / Theater Mode) -->
    <video
      bind:this={videoBgEl}
      poster={activeMovie?.thumbnail ? sanitizeMediaUrl(activeMovie.thumbnail) : ''}
      class="absolute inset-0 w-full h-full object-cover transition-all duration-700 ease-out {isTheaterMode && activeMedia?.type === 'video' ? 'opacity-100 blur-0 scale-100 brightness-100 z-10' : (!isTheaterMode && isAmbientTrailerActive && activeMovie?.src && videoHasRenderedFrame ? 'opacity-30 blur-md scale-105 brightness-75 z-10' : 'opacity-0 pointer-events-none')}"
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

    <!-- Fullscreen Screenshot (when active in Theater Mode) -->
    {#if isTheaterMode && activeMedia?.type === 'screenshot'}
      <img
        src={activeMedia.url}
        alt={activeMedia.title}
        class="absolute inset-0 w-full h-full object-contain bg-black transition-opacity duration-300 ease-out z-10"
      />
    {/if}

    <!-- Ambient vignette & falloff gradients -->
    {#if !isTheaterMode}
      <div class="absolute inset-0 bg-gradient-to-t from-[#07080a] via-[#07080a]/75 to-transparent transition-opacity duration-500 z-10 pointer-events-none"></div>
      <div class="absolute inset-0 bg-gradient-to-r from-[#07080a]/95 via-[#07080a]/60 to-transparent transition-opacity duration-500 z-10 pointer-events-none"></div>
    {/if}
  </div>

  <!-- 2. Main Fullscreen Interface Layer (100vw x 100vh, Zero Page Scroll) -->
  <div
    data-nav-zone="detail"
    class="relative z-10 w-full h-full flex flex-col justify-between p-6 sm:p-8 lg:p-10 overflow-hidden select-none {isTheaterMode ? 'hidden pointer-events-none' : ''}"
  >
    <!-- Top Bar: Navigation & Status -->
    <div class="flex items-center justify-between w-full flex-shrink-0 z-20">
      <button
        data-nav-item
        type="button"
        class="px-4 py-2 rounded bg-white/[0.08] hover:bg-white/[0.15] border border-white/10 text-xs font-bold text-white hover:border-white/30 flex items-center gap-2.5 cursor-pointer transition-all active:scale-95 focus:ring-1 focus:ring-white focus:outline-none shadow-md"
        onclick={() => {
          sound.playBack();
          onBack();
        }}
      >
        <ArrowLeft class="w-4 h-4" />
        <span>Библиотека</span>
        <span class="w-4 h-4 rounded bg-white/20 text-[10px] flex items-center justify-center font-bold">B</span>
      </button>

      <div class="flex items-center gap-2.5">
        {#if isEnrichingCurrentGame}
          <div class="flex items-center gap-2 px-3 py-1.5 rounded-sm bg-sky-500/10 border border-sky-500/20 text-sky-400 text-xs font-mono">
            <RefreshCw class="w-3.5 h-3.5 animate-spin" />
            <span>Поиск данных...</span>
          </div>
        {/if}

        <button
          data-nav-item
          type="button"
          class="px-3.5 py-1.5 rounded bg-white/[0.08] hover:bg-white/[0.15] border border-white/10 text-xs font-semibold text-[#cbd5e1] hover:text-white flex items-center gap-2 cursor-pointer transition-colors focus:ring-1 focus:ring-white focus:outline-none shadow-md"
          onclick={openSteamModal}
          title="Привязать Steam / Изменить метаданные"
        >
          <Edit3 class="w-3.5 h-3.5 text-sky-400" />
          <span>{activeGame?.steamAppId > 0 ? `Steam AppID: ${activeGame.steamAppId}` : (activeGame?.steamAppId < 0 ? `SteamGridDB: ${-activeGame.steamAppId}` : 'Привязать Steam')}</span>
        </button>
      </div>
    </div>

    <!-- Middle Hero Stage: Game Title/Logo, Meta Badges & Action Buttons -->
    {#if activeGame}
      <div class="max-w-4xl space-y-4 my-auto z-20">
        <!-- Official Steam Logo or Large Google Sans Title -->
        <div class="min-h-[60px] sm:min-h-[80px] flex items-end">
          {#if activeGame.steamAppId && !logoFailedMap[activeGame.steamAppId]}
            <img
              src="https://shared.steamstatic.com/store_item_assets/steam/apps/{activeGame.steamAppId}/logo.png"
              alt={activeGame.cleanTitle || activeGame.rawName}
              class="max-h-24 sm:max-h-28 md:max-h-32 max-w-[320px] sm:max-w-lg object-contain object-left-bottom drop-shadow-2xl transition-all duration-500"
              onerror={() => {
                if (activeGame?.steamAppId) logoFailedMap[activeGame.steamAppId] = true;
              }}
            />
          {:else}
            <h1 class="text-3xl sm:text-4xl md:text-5xl font-black text-white tracking-tight leading-none drop-shadow-2xl line-clamp-2">
              {activeGame.cleanTitle || activeGame.rawName || activeGame.folderName || 'Игра'}
            </h1>
          {/if}
        </div>

        <!-- 1. Status & Source Badges Strip -->
        <div class="flex flex-wrap items-center gap-2 pt-0.5">
          <span class="px-2 py-0.5 rounded bg-white/[0.08] text-[#8e95a2] text-[10px] font-black tracking-widest border border-white/10 uppercase font-mono">
            PC
          </span>

          {#if isCompleted}
            <span class="px-2.5 py-0.5 rounded bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs font-semibold flex items-center gap-1.5">
              <Check class="w-3.5 h-3.5 stroke-[2.5]" />
              <span>Установлено</span>
            </span>
          {:else if isDownloading}
            <span class="px-2.5 py-0.5 rounded bg-sky-500/10 border border-sky-500/20 text-sky-400 text-xs font-semibold flex items-center gap-1.5">
              <Download class="w-3.5 h-3.5 stroke-[2.5]" />
              <span>{Math.round(activeDownload?.progressPercent ?? (activeDownload as any)?.progress ?? pageDetails?.downloadProgress?.progressPercent ?? 0)}%</span>
            </span>
          {/if}

          {#if downloadSourceInfo}
            <span class="px-2.5 py-0.5 rounded border border-white/10 bg-white/[0.05] text-[#94a3b8] text-xs font-medium flex items-center gap-1.5">
              <span>{downloadSourceInfo.name}</span>
            </span>
          {/if}

          {#if currentFavoriteStatus}
            <span class="px-2.5 py-0.5 rounded bg-amber-500/10 border border-amber-500/20 text-amber-400 text-xs font-semibold flex items-center gap-1.5">
              <Star class="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
              <span>
                {#if currentFavoriteStatus === 'playing'}
                  Прохожу
                {:else if currentFavoriteStatus === 'completed'}
                  Пройдено
                {:else}
                  В планах
                {/if}
              </span>
            </span>
          {/if}
        </div>

        <!-- 2. Metadata Factline (No trailing bullets, clean typography) -->
        <div class="flex flex-wrap items-center gap-y-1 gap-x-2 text-xs text-[#8e95a2]">
          {#if activeGame.releaseDate || activeGame.steamReleaseDate}
            <span>{activeGame.releaseDate || activeGame.steamReleaseDate}</span>
          {/if}

          {#if (activeGame.releaseDate || activeGame.steamReleaseDate) && (activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr)}
            <span class="text-white/20 select-none">•</span>
          {/if}

          {#if activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr}
            <span class="font-mono text-[#cbd5e1]">{activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr}</span>
          {/if}

          {#if !isCompleted && currentVariantSeedInfo}
            {#if (activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr || activeGame.releaseDate || activeGame.steamReleaseDate)}
              <span class="text-white/20 select-none">•</span>
            {/if}
            <span class="inline-flex items-center gap-1 font-mono {currentVariantSeedInfo.seeders > 0 ? 'text-emerald-400 font-semibold' : 'text-[#8e95a2]'}">
              <UploadSimple class="w-3 h-3 stroke-[2.5]" />
              {#if currentVariantSeedInfo.loading}
                <span class="animate-pulse text-[#64748b]">...</span>
              {:else}
                <span>{formatSeedsCount(currentVariantSeedInfo.seeders)}</span>
              {/if}
            </span>
          {/if}

          {#if activeGame.reviewPercent}
            {#if (activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr || activeGame.releaseDate || activeGame.steamReleaseDate || (!isCompleted && currentVariantSeedInfo))}
              <span class="text-white/20 select-none">•</span>
            {/if}
            <span class="font-bold text-white flex items-center gap-1">
              <Star class="w-3.5 h-3.5 text-amber-400 fill-amber-400" />
              <span>{activeGame.reviewPercent}%</span>
            </span>
          {/if}

          {#if activeGame.genres && Array.isArray(activeGame.genres) && activeGame.genres.length > 0}
            {#if (activeGame.reviewPercent || activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr || activeGame.releaseDate || (!isCompleted && currentVariantSeedInfo))}
              <span class="text-white/20 select-none">•</span>
            {/if}
            <span class="text-[#8e95a2]">
              {activeGame.genres.slice(0, 3).join(', ')}
            </span>
          {/if}
        </div>

        <!-- 3. Short Synopsis Preview -->
        {#if activeGame.shortDescription}
          <p class="text-xs sm:text-sm text-[#cbd5e1]/90 leading-relaxed line-clamp-2 max-w-2xl font-normal drop-shadow-sm">
            {activeGame.shortDescription.replace(/<[^>]*>?/gm, '')}
          </p>
        {/if}

        <!-- 4. Tactile Action Buttons Row (Unified single-row Steam Deck layout) -->
        <div class="flex items-center gap-2.5 pt-1.5 flex-wrap">
          <!-- Primary Action Pill (A) -->
          <button
            data-nav-item
            type="button"
            class="h-11 px-6 rounded-md bg-white hover:bg-slate-100 text-black font-extrabold text-sm inline-flex items-center gap-2.5 transition-all cursor-pointer shadow-lg shadow-white/5 active:scale-[0.98] focus:ring-2 focus:ring-white focus:outline-none flex-shrink-0"
            onclick={handlePrimaryAction}
          >
            {#if isCompleted}
              <Play class="w-4 h-4 fill-black text-black" />
              <span>Играть</span>
            {:else if isDownloading}
              <Download class="w-4 h-4 stroke-[2.5]" />
              <span>{activeDownload?.status === 'paused' || pageDetails?.downloadStatus === 'paused' ? 'Приостановлено' : (activeDownload?.status === 'queued' || pageDetails?.downloadStatus === 'queued' ? 'В очереди...' : 'Скачивается...')}</span>
            {:else}
              <Download class="w-4 h-4 stroke-[2.5]" />
              <span>Скачать</span>
              {#if activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr}
                <span class="text-xs font-mono opacity-60 font-bold">• {activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr}</span>
              {/if}
            {/if}
            <span class="w-5 h-5 rounded-full bg-black/15 text-black text-[10px] font-black flex items-center justify-center ml-0.5">A</span>
          </button>

          <!-- Description Button (X) -->
          <button
            data-nav-item
            type="button"
            class="h-11 px-4 rounded-md bg-white/[0.08] hover:bg-white/[0.14] text-white font-bold text-xs inline-flex items-center gap-2 border border-white/10 hover:border-white/20 transition-all cursor-pointer active:scale-[0.98] focus:ring-2 focus:ring-white focus:outline-none flex-shrink-0"
            onclick={openDescriptionModal}
            title="Открыть описание игры [X]"
          >
            <Info class="w-4 h-4 text-sky-400" />
            <span>Описание</span>
            <span class="w-4 h-4 rounded-full bg-white/15 text-[#cbd5e1] text-[9px] font-bold flex items-center justify-center">X</span>
          </button>

          <!-- Versions or Folder Action -->
          {#if isCompleted}
            <button
              data-nav-item
              type="button"
              class="h-11 px-4 rounded-md bg-white/[0.08] hover:bg-white/[0.14] text-white font-bold text-xs inline-flex items-center gap-2 border border-white/10 hover:border-white/20 transition-all cursor-pointer active:scale-[0.98] focus:ring-2 focus:ring-white focus:outline-none flex-shrink-0"
              onclick={handleOpenGameFolder}
              title="Открыть папку с игрой"
            >
              <Folder class="w-4 h-4 fill-current text-[#cbd5e1]" />
              <span>Папка</span>
            </button>
          {:else if activeVariantsList && activeVariantsList.length > 1}
            <button
              data-nav-item
              type="button"
              class="h-11 px-4 rounded-md bg-white/[0.08] hover:bg-white/[0.14] text-white font-bold text-xs inline-flex items-center gap-2 border border-white/10 hover:border-white/20 transition-all cursor-pointer active:scale-[0.98] focus:ring-2 focus:ring-white focus:outline-none flex-shrink-0"
              onclick={openDownloadWizard}
              title="Выбрать версию ({activeVariantsList.length} доступно)"
            >
              <Disc class="w-4 h-4 text-sky-400" />
              <span>Версии ({activeVariantsList.length})</span>
            </button>
          {/if}

          <!-- Open Steam Store Page -->
          {#if activeGame?.steamAppId && activeGame.steamAppId > 0}
            <button
              data-nav-item
              type="button"
              class="h-11 px-4 rounded-md bg-white/[0.08] hover:bg-white/[0.14] text-white font-bold text-xs inline-flex items-center gap-2 border border-white/10 hover:border-white/20 transition-all cursor-pointer active:scale-[0.98] focus:ring-2 focus:ring-white focus:outline-none flex-shrink-0"
              onclick={handleOpenSteamStore}
              title="Открыть страницу игры в магазине Steam"
            >
              <SteamLogo size={16} weight="bold" class="text-sky-400" />
              <span>В Steam</span>
              <ExternalLink class="w-3.5 h-3.5 text-[#8e95a2]" />
            </button>
          {/if}

          <!-- Watch Media Fullscreen Button (↑) -->
          {#if unifiedMediaList.length > 0}
            <button
              data-nav-item
              type="button"
              class="h-11 px-4 rounded-md bg-white/[0.08] hover:bg-white/[0.14] text-white font-bold text-xs inline-flex items-center gap-2 border border-white/10 hover:border-white/20 transition-all cursor-pointer active:scale-[0.98] focus:ring-2 focus:ring-white focus:outline-none flex-shrink-0"
              onclick={() => enterTheaterMode(0)}
              title="Смотреть во весь экран [↑]"
            >
              {#if movieList.length > 0}
                <Volume2 class="w-4 h-4 text-sky-400" />
                <span>Трейлер</span>
              {:else}
                <ImageIcon class="w-4 h-4 text-sky-400" />
                <span>Медиа</span>
              {/if}
              <span class="w-4 h-4 rounded-full bg-white/15 text-[#cbd5e1] text-[9px] font-bold flex items-center justify-center font-mono">↑</span>
            </button>
          {/if}

          <!-- Favorite Toggle (Y) -->
          <div class="relative flex-shrink-0">
            <button
              bind:this={favoriteDropdownTriggerEl}
              data-nav-item
              type="button"
              class="h-11 px-3.5 rounded-md bg-white/[0.08] hover:bg-white/[0.14] text-white border border-white/10 hover:border-white/20 transition-all cursor-pointer active:scale-[0.98] focus:ring-2 focus:ring-white focus:outline-none inline-flex items-center gap-2"
              onclick={() => {
                isFavoriteDropdownOpen = !isFavoriteDropdownOpen;
              }}
              title="Статус в избранном [Y]"
            >
              <Star class="w-4 h-4 {currentFavoriteStatus ? 'text-amber-400 fill-amber-400' : 'text-white/40 fill-white/20'}" />
              <span class="w-4 h-4 rounded-full bg-white/15 text-[#cbd5e1] text-[9px] font-bold flex items-center justify-center">Y</span>
            </button>

            {#if isFavoriteDropdownOpen}
              <div
                bind:this={favoriteDropdownContainerEl}
                class="absolute left-0 bottom-full mb-3 z-50 w-52 rounded-md bg-[#0d1117] border border-white/15 shadow-2xl p-1.5 space-y-1"
              >
                <div class="px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-[#64748b]">
                  Статус игры
                </div>
                <button
                  data-nav-item
                  type="button"
                  class="w-full text-left flex items-center justify-between px-3 py-2 rounded-sm text-xs transition-colors cursor-pointer {currentFavoriteStatus === 'planned' ? 'bg-amber-500/20 text-amber-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                  onclick={() => handleToggleFavoriteStatus('planned')}
                >
                  <span>В планах</span>
                  {#if currentFavoriteStatus === 'planned'}
                    <Check class="w-3.5 h-3.5 text-amber-400 stroke-[2.5]" />
                  {/if}
                </button>

                <button
                  data-nav-item
                  type="button"
                  class="w-full text-left flex items-center justify-between px-3 py-2 rounded-sm text-xs transition-colors cursor-pointer {currentFavoriteStatus === 'playing' ? 'bg-amber-500/20 text-amber-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                  onclick={() => handleToggleFavoriteStatus('playing')}
                >
                  <span>Прохожу</span>
                  {#if currentFavoriteStatus === 'playing'}
                    <Check class="w-3.5 h-3.5 text-amber-400 stroke-[2.5]" />
                  {/if}
                </button>

                <button
                  data-nav-item
                  type="button"
                  class="w-full text-left flex items-center justify-between px-3 py-2 rounded-sm text-xs transition-colors cursor-pointer {currentFavoriteStatus === 'completed' ? 'bg-amber-500/20 text-amber-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                  onclick={() => handleToggleFavoriteStatus('completed')}
                >
                  <span>Пройдено</span>
                  {#if currentFavoriteStatus === 'completed'}
                    <Check class="w-3.5 h-3.5 text-amber-400 stroke-[2.5]" />
                  {/if}
                </button>

                {#if currentFavoriteStatus}
                  <button
                    data-nav-item
                    type="button"
                    class="w-full text-left flex items-center justify-between px-3 py-2 rounded-sm text-xs text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
                    onclick={() => handleToggleFavoriteStatus('')}
                  >
                    <span>Убрать из избранного</span>
                    <X class="w-3.5 h-3.5 text-rose-400" />
                  </button>
                {/if}
              </div>
            {/if}
          </div>
        </div>
      </div>
    {/if}

    <!-- Bottom Strip: Trailers & Screenshots Carousel -->
    {#if movieList.length > 0 || gameScreenshots.length > 0}
      <div class="w-full pt-3 pb-1 z-20 flex-shrink-0">
        <div class="flex items-center justify-between pb-2">
          <span class="text-xs font-bold uppercase tracking-wider text-[#8e95a2]">Медиа и скриншоты</span>
          <span class="text-xs text-[#64748b] font-medium hidden sm:inline">Нажмите [A] для полноэкранного просмотра</span>
        </div>
        <div class="media-carousel-container flex items-center gap-3 overflow-x-auto scrollbar-none py-1">
          <!-- Video Trailers -->
          {#each movieList as movie, mIdx}
            <button
              data-nav-item
              type="button"
              class="w-44 sm:w-52 aspect-video rounded-sm overflow-hidden border border-white/15 bg-black hover:border-sky-400 focus:border-sky-400 hover:scale-105 focus:scale-105 focus:outline-none transition-all flex-shrink-0 cursor-pointer relative group/thumb shadow-lg"
              onclick={() => enterTheaterMode(mIdx)}
              title={movie.name || `Трейлер ${mIdx + 1}`}
            >
              {#if movie.thumbnail}
                <img src={sanitizeMediaUrl(movie.thumbnail)} alt={movie.name || 'Трейлер'} class="w-full h-full object-cover" loading="lazy" />
              {:else}
                <div class="w-full h-full bg-[#0d1117] flex items-center justify-center">
                  <Film class="w-8 h-8 text-white/30" />
                </div>
              {/if}
              <div class="absolute inset-0 bg-black/40 group-hover/thumb:bg-black/20 transition-colors flex items-center justify-center">
                <div class="w-9 h-9 rounded-full bg-black/80 group-hover/thumb:bg-sky-500 text-white flex items-center justify-center border border-white/20 group-hover/thumb:border-transparent transition-all shadow-md">
                  <Play class="w-3.5 h-3.5 fill-white translate-x-0.5" />
                </div>
              </div>
              <div class="absolute bottom-1.5 inset-x-1.5 flex items-center justify-between pointer-events-none">
                <span class="px-2 py-0.5 rounded bg-black/80 text-[10px] font-bold text-white max-w-[120px] truncate border border-white/10">
                  {movie.name || `Трейлер ${mIdx + 1}`}
                </span>
                <span class="px-1.5 py-0.5 rounded bg-sky-500/80 text-[9px] font-bold uppercase tracking-wider text-black">
                  Видео
                </span>
              </div>
            </button>
          {/each}

          <!-- Screenshots -->
          {#each gameScreenshots as sc, idx}
            <button
              data-nav-item
              type="button"
              class="w-44 sm:w-52 aspect-video rounded-sm overflow-hidden border border-white/10 bg-black/60 hover:border-white hover:scale-105 focus:border-white focus:scale-105 focus:outline-none transition-all flex-shrink-0 cursor-pointer relative group/thumb shadow-lg"
              onclick={() => enterTheaterMode(movieList.length + idx)}
              onmouseenter={() => {
                activeScreenshotPreview = sc;
              }}
              onfocus={() => {
                activeScreenshotPreview = sc;
              }}
              title="Скриншот {idx + 1}"
            >
              <img src={sc} alt="" class="w-full h-full object-cover" loading="lazy" />
              <div class="absolute inset-0 bg-white/10 opacity-0 group-hover/thumb:opacity-100 transition-opacity"></div>
            </button>
          {/each}
        </div>
      </div>
    {/if}
  </div>

  <!-- 3. Description Overlay Screen (Opened by [X]) -->
  {#if isDescriptionModalOpen}
    <div
      data-nav-zone="modal"
      class="fixed inset-0 z-50 bg-black/90 backdrop-blur-sm flex items-center justify-center p-4 sm:p-8 animate-fade-in select-none"
    >
      <div class="w-full max-w-5xl h-[88vh] bg-[#07080a] border border-white/10 rounded-md flex flex-col overflow-hidden shadow-2xl">
        <!-- 1. Modal Top Bar: Game Title & Subtitle + Close Button -->
        <div class="px-6 sm:px-8 pt-5 pb-4 flex items-center justify-between flex-shrink-0 bg-[#0c0e14]">
          <div class="min-w-0 flex-1 pr-4">
            <h2 class="text-base sm:text-xl font-bold text-white tracking-tight leading-snug truncate">
              {getDisplayTitle(activeGame) || 'Об игре'}
            </h2>
            {#if activeGame?.developers?.length || activeGame?.releaseDate}
              <div class="flex items-center gap-2 text-xs text-[#8e95a2] mt-0.5 truncate">
                {#if activeGame?.developers?.length}
                  <span>{activeGame.developers.join(', ')}</span>
                {/if}
                {#if activeGame?.developers?.length && activeGame?.releaseDate}
                  <span class="text-white/20">•</span>
                {/if}
                {#if activeGame?.releaseDate}
                  <span>{activeGame.releaseDate}</span>
                {/if}
              </div>
            {/if}
          </div>

          <div class="flex items-center gap-3 flex-shrink-0">
            <button
              data-nav-item
              type="button"
              class="px-3.5 py-1.5 rounded-sm bg-white/10 hover:bg-white/20 border border-white/10 text-white text-xs font-bold flex items-center gap-2 cursor-pointer focus:ring-1 focus:ring-white focus:outline-none transition-colors"
              onclick={closeDescriptionModal}
            >
              <span>Закрыть</span>
              <span class="w-4 h-4 rounded-sm bg-white/20 text-[10px] flex items-center justify-center font-bold">B</span>
            </button>
          </div>
        </div>

        <!-- 2. SteamOS Horizontal Tab Rail (Baseline Tabs) -->
        <div class="px-6 sm:px-8 border-b border-white/10 flex items-center justify-between flex-shrink-0 bg-[#090b10] gap-4">
          <div class="flex items-center gap-1 sm:gap-2 flex-1 min-w-0 overflow-x-auto scrollbar-none">
            <!-- LB Bumper Hint -->
            <button
              type="button"
              class="hidden sm:inline-flex items-center justify-center px-1.5 py-0.5 rounded-sm bg-white/5 hover:bg-white/10 border border-white/10 font-mono text-[10px] font-bold text-[#8e95a2] hover:text-white mr-1 cursor-pointer transition-colors focus:outline-none flex-shrink-0"
              onclick={() => cycleDescTab(-1)}
              title="Предыдущая вкладка (LB / Q / ←)"
            >
              LB
            </button>

            <!-- Tab 1: Об игре -->
            <button
              data-nav-item
              type="button"
              class="relative whitespace-nowrap flex-shrink-0 px-3.5 sm:px-5 py-2.5 sm:py-3 text-xs sm:text-sm font-semibold tracking-wide transition-colors cursor-pointer focus:outline-none focus:text-white -mb-px {descTab === 'about' ? 'text-white font-bold border-b border-white' : 'text-[#8e95a2] hover:text-white border-b border-transparent'}"
              onclick={() => {
                descTab = 'about';
                sound.playTab();
              }}
            >
              Об игре
            </button>

            <!-- Tab 2: Сведения -->
            <button
              data-nav-item
              type="button"
              class="relative whitespace-nowrap flex-shrink-0 px-3.5 sm:px-5 py-2.5 sm:py-3 text-xs sm:text-sm font-semibold tracking-wide transition-colors cursor-pointer focus:outline-none focus:text-white -mb-px {descTab === 'details' ? 'text-white font-bold border-b border-white' : 'text-[#8e95a2] hover:text-white border-b border-transparent'}"
              onclick={() => {
                descTab = 'details';
                sound.playTab();
              }}
            >
              Сведения
            </button>

            <!-- Tab 3: Системные требования -->
            <button
              data-nav-item
              type="button"
              class="relative whitespace-nowrap flex-shrink-0 px-3.5 sm:px-5 py-2.5 sm:py-3 text-xs sm:text-sm font-semibold tracking-wide transition-colors cursor-pointer focus:outline-none focus:text-white -mb-px {descTab === 'specs' ? 'text-white font-bold border-b border-white' : 'text-[#8e95a2] hover:text-white border-b border-transparent'}"
              onclick={() => {
                descTab = 'specs';
                sound.playTab();
              }}
            >
              Системные требования
            </button>

            <!-- Tab 4: Все релизы (если > 1) -->
            {#if activeVariantsList && activeVariantsList.length > 1}
              <button
                data-nav-item
                type="button"
                class="relative whitespace-nowrap flex-shrink-0 px-3.5 sm:px-5 py-2.5 sm:py-3 text-xs sm:text-sm font-semibold tracking-wide transition-colors cursor-pointer focus:outline-none focus:text-white -mb-px {descTab === 'variants' ? 'text-white font-bold border-b border-white' : 'text-[#8e95a2] hover:text-white border-b border-transparent'}"
                onclick={() => {
                  descTab = 'variants';
                  sound.playTab();
                }}
              >
                Все релизы ({activeVariantsList.length})
              </button>
            {/if}

            <!-- Tab 5: Отзывы Steam -->
            {#if activeGame?.steamAppId && activeGame.steamAppId > 0}
              <button
                data-nav-item
                type="button"
                class="relative whitespace-nowrap flex-shrink-0 px-3.5 sm:px-5 py-2.5 sm:py-3 text-xs sm:text-sm font-semibold tracking-wide transition-colors cursor-pointer focus:outline-none focus:text-white -mb-px {descTab === 'reviews' ? 'text-white font-bold border-b border-white' : 'text-[#8e95a2] hover:text-white border-b border-transparent'}"
                onclick={() => {
                  descTab = 'reviews';
                  sound.playTab();
                }}
              >
                Отзывы Steam {#if steamReviewsTotal > 0}<span class="text-xs opacity-75 font-mono">({steamReviewsTotal})</span>{/if}
              </button>
            {/if}

            <!-- RB Bumper Hint -->
            <button
              type="button"
              class="hidden sm:inline-flex items-center justify-center px-1.5 py-0.5 rounded-sm bg-white/5 hover:bg-white/10 border border-white/10 font-mono text-[10px] font-bold text-[#8e95a2] hover:text-white ml-1 cursor-pointer transition-colors focus:outline-none flex-shrink-0"
              onclick={() => cycleDescTab(1)}
              title="Следующая вкладка (RB / E / →)"
            >
              RB
            </button>
          </div>

          <!-- Quick Navigation Tip -->
          <div class="hidden xl:flex items-center gap-1.5 text-[11px] text-[#64748b] flex-shrink-0">
            <span>Вкладки:</span>
            <span class="font-mono text-[10px] text-[#8e95a2]">LB / RB</span>
            <span class="mx-1 text-white/10">•</span>
            <span>Прокрутка:</span>
            <span class="font-mono text-[10px] text-[#8e95a2]">D-pad ↑ / ↓</span>
          </div>
        </div>

        <!-- Scrollable Description Body (No Sidebars, Clean Full-Width Steam Layout) -->
        <div
          bind:this={descScrollEl}
          class="flex-1 overflow-y-auto p-6 sm:p-10 scrollbar-none"
        >
          {#if descTab === 'about'}
            <div class="max-w-4xl mx-auto space-y-6">
              {#if activeGame?.detailedDescription}
                <div class="steam-html-content text-sm sm:text-base text-[#d1d5db] leading-relaxed">
                  {@html activeGame.detailedDescription}
                </div>
              {:else if activeGame?.shortDescription}
                <div class="text-sm sm:text-base text-[#d1d5db] leading-relaxed">
                  {@html activeGame.shortDescription}
                </div>
              {:else}
                <div class="text-sm text-[#64748b]">Описание отсутствует</div>
              {/if}

              {#if activeGame?.genres && activeGame.genres.length > 0}
                <div class="pt-6 border-t border-white/10 space-y-2">
                  <span class="text-xs font-bold uppercase tracking-wider text-[#8e95a2] block">Жанры и теги</span>
                  <div class="flex items-center gap-2 flex-wrap text-xs">
                    {#each activeGame.genres as g}
                      <span class="px-3 py-1 rounded-sm bg-white/5 border border-white/10 text-white font-medium">
                        {g}
                      </span>
                    {/each}
                  </div>
                </div>
              {/if}
            </div>

          {:else if descTab === 'details'}
            <div class="max-w-4xl mx-auto space-y-6">
              <!-- Review score card -->
              {#if activeGame?.reviewPercent || activeGame?.reviewScoreDesc}
                {@const isPositive = (activeGame.reviewPercent || 0) >= 70}
                {@const isMixed = (activeGame.reviewPercent || 0) >= 40 && (activeGame.reviewPercent || 0) < 70}
                <div class="p-5 rounded-md bg-[#0e1219] border border-white/10 flex items-center gap-4 shadow-lg">
                  <div class="w-14 h-14 rounded border flex items-center justify-center font-black text-lg {isPositive ? 'bg-sky-500/15 text-sky-400 border-sky-500/30' : isMixed ? 'bg-amber-500/15 text-amber-400 border-amber-500/30' : 'bg-red-500/15 text-red-400 border-red-500/30'}">
                    {activeGame.reviewPercent || 0}%
                  </div>
                  <div class="space-y-1">
                    <div class="text-base font-bold {isPositive ? 'text-sky-400' : isMixed ? 'text-amber-400' : 'text-red-400'}">
                      {activeGame.reviewScoreDesc || (isPositive ? 'Положительные' : isMixed ? 'Смешанные' : 'Отрицательные')}
                    </div>
                    {#if activeGame.totalReviews}
                      <div class="text-xs text-[#8e95a2]">
                        {formatReviewsCount(activeGame.totalReviews)} обзоров игроков в Steam
                      </div>
                    {/if}
                  </div>
                </div>
              {:else if activeGame?.metacriticScore}
                <div class="p-5 rounded-md bg-[#0e1219] border border-white/10 flex items-center gap-4 shadow-lg">
                  <div class="w-14 h-14 rounded border border-emerald-500/30 bg-emerald-500/15 text-emerald-400 flex items-center justify-center font-black text-lg">
                    {activeGame.metacriticScore}
                  </div>
                  <div class="space-y-1">
                    <div class="text-base font-bold text-emerald-400">Metacritic Score</div>
                    <div class="text-xs text-[#8e95a2]">Оценка ведущих мировых критиков</div>
                  </div>
                </div>
              {/if}

              <!-- Game Specs & Metadata Breakdown -->
              <div class="bg-[#0e1219] border border-white/10 rounded-md p-6 shadow-lg divide-y divide-white/[0.06] text-sm">
                <div class="pb-3.5 flex items-center justify-between gap-4">
                  <span class="text-[#8e95a2]">Разработчик</span>
                  <span class="font-bold text-white text-right">{activeGame?.developers?.join(', ') || '—'}</span>
                </div>
                <div class="py-3.5 flex items-center justify-between gap-4">
                  <span class="text-[#8e95a2]">Издатель</span>
                  <span class="font-bold text-white text-right">{activeGame?.publishers?.join(', ') || '—'}</span>
                </div>
                <div class="py-3.5 flex items-center justify-between gap-4">
                  <span class="text-[#8e95a2]">Дата выхода</span>
                  <span class="font-bold text-white">{activeGame?.releaseDate || activeGame?.steamReleaseDate || '—'}</span>
                </div>
                <div class="py-3.5 flex items-center justify-between gap-4">
                  <span class="text-[#8e95a2]">Поддержка геймпада</span>
                  <span class="font-bold {activeGame?.controllerSupport === 'full' ? 'text-sky-400' : 'text-[#cbd5e1]'}">
                    {activeGame?.controllerSupport === 'full' ? 'Полная поддержка геймпада' : 'Клавиатура / Мышь'}
                  </span>
                </div>
                <div class="py-3.5 flex items-center justify-between gap-4">
                  <span class="text-[#8e95a2]">Размер в хранилище</span>
                  <span class="font-mono font-bold text-white">{activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr || '—'}</span>
                </div>
                <div class="py-3.5 flex items-center justify-between gap-4">
                  <span class="text-[#8e95a2]">Источник</span>
                  <span class="font-bold text-white">{downloadSourceInfo?.name || '—'}</span>
                </div>
                <div class="py-3.5 flex items-center justify-between gap-4">
                  <span class="text-[#8e95a2]">Steam AppID</span>
                  <span class="font-mono text-[#cbd5e1]">{activeGame?.steamAppId > 0 ? activeGame.steamAppId : 'Не привязан'}</span>
                </div>
                <div class="pt-3.5 flex items-center justify-between gap-4">
                  <span class="text-[#8e95a2]">Папка загрузки</span>
                  <span class="font-mono text-[#cbd5e1] truncate max-w-lg">{pageDetails?.localPath || customDownloadPath || downloadPath}</span>
                </div>
              </div>
            </div>

          {:else if descTab === 'specs'}
            <div class="max-w-4xl mx-auto space-y-6">
              <span class="text-xs font-bold uppercase tracking-wider text-[#8e95a2] block">Системные требования</span>
              {#if activeGame?.pcRequirements}
                <div class="steam-html-content text-sm sm:text-base text-[#d1d5db] leading-relaxed bg-[#0e1219] border border-white/10 rounded-md p-6 shadow-lg">
                  {@html activeGame.pcRequirements}
                </div>
              {:else}
                <div class="p-6 rounded-md bg-[#0e1219] border border-white/10 text-sm text-[#64748b]">
                  Информация о системных требованиях отсутствует
                </div>
              {/if}
            </div>

          {:else if descTab === 'variants'}
            <div class="space-y-3 max-w-4xl">
              <div class="text-xs text-[#8e95a2] pb-2 font-medium">
                Доступно релизов: {activeVariantsList.length}. Нажмите «Скачать» для загрузки конкретной сборки.
              </div>
              {#each activeVariantsList as variant (variant.id)}
                {@const isSel = activeVariant?.id === variant.id}
                <div
                  class="w-full flex items-center justify-between p-4 rounded border {isSel ? 'bg-white/10 border-white/30 text-white' : 'bg-[#0d1017] border-white/[0.06] text-[#8e95a2]'}"
                >
                  <div class="min-w-0 flex-1 pr-4">
                    <div class="text-sm font-semibold text-white truncate">{variant.rawName}</div>
                    <div class="text-xs text-[#64748b] font-mono flex items-center gap-2.5 mt-1">
                      <span>Источник: {variant.sourceType === 'torrent' || variant.magnetUri ? (formatTorrentSourceName(variant.torrentSource) || 'Торрент') : 'FTP-сервер'}</span>
                      {#if variant.sourceType === 'torrent' || variant.magnetUri}
                        <span class="text-white/20">•</span>
                        {#if variantSeeds[variant.id]?.loading}
                          <span class="text-[#64748b] animate-pulse">сиды: ...</span>
                        {:else if variantSeeds[variant.id]}
                          {@const s = variantSeeds[variant.id].seeders}
                          <span class="inline-flex items-center gap-0.5 {s > 0 ? 'text-emerald-400 font-semibold' : 'text-[#64748b]'}">
                            <UploadSimple class="w-3.5 h-3.5 stroke-[2.5]" />
                            <span>{formatSeedsCount(s)}</span>
                          </span>
                        {/if}
                      {/if}
                    </div>
                  </div>

                  <div class="flex items-center gap-4 font-mono text-xs flex-shrink-0">
                    <span class="{isSel ? 'text-sky-400 font-bold' : 'text-[#8e95a2]'}">{variant.sizeDisplay}</span>
                    <button
                      data-nav-item
                      type="button"
                      class="px-3.5 py-1.5 rounded bg-white text-black font-bold text-xs hover:bg-slate-200 transition-colors cursor-pointer focus:ring-1 focus:ring-white focus:outline-none"
                      onclick={() => {
                        confirmDownloadVariant(variant.id);
                        closeDescriptionModal();
                      }}
                    >
                      Скачать
                    </button>
                  </div>
                </div>
              {/each}
            </div>

          {:else if descTab === 'reviews'}
            <div class="max-w-4xl mx-auto space-y-6">
              <!-- Top Controls: Header + Language switcher + Open in Steam -->
              <div class="flex flex-wrap items-center justify-between gap-4 pb-4 border-b border-white/10">
                <div class="flex items-center gap-3">
                  <ChatText class="w-5 h-5 text-sky-400" />
                  <span class="text-sm font-bold uppercase tracking-wider text-white">Отзывы сообщества Steam</span>
                  {#if steamReviewsTotal > 0}
                    <span class="text-xs text-[#8e95a2] font-mono">({formatReviewsCount(steamReviewsTotal)})</span>
                  {/if}
                </div>

                <div class="flex items-center gap-3">
                  <div class="inline-flex rounded bg-black/40 border border-white/10 p-1 text-xs">
                    <button
                      data-nav-item
                      type="button"
                      class="px-3 py-1.5 rounded font-medium cursor-pointer transition-colors focus:ring-1 focus:ring-white focus:outline-none {reviewLanguage === 'russian' ? 'bg-white/20 text-white font-bold' : 'text-[#8e95a2] hover:text-white'}"
                      onclick={() => handleLanguageChange('russian')}
                    >
                      Русские
                    </button>
                    <button
                      data-nav-item
                      type="button"
                      class="px-3 py-1.5 rounded font-medium cursor-pointer transition-colors focus:ring-1 focus:ring-white focus:outline-none {reviewLanguage === 'all' ? 'bg-white/20 text-white font-bold' : 'text-[#8e95a2] hover:text-white'}"
                      onclick={() => handleLanguageChange('all')}
                    >
                      Все языки
                    </button>
                  </div>

                  <button
                    data-nav-item
                    type="button"
                    class="px-3.5 py-2 rounded bg-white/10 hover:bg-white/20 text-white text-xs font-semibold flex items-center gap-1.5 border border-white/10 transition-colors cursor-pointer focus:ring-1 focus:ring-white focus:outline-none"
                    onclick={handleOpenSteamStore}
                    title="Открыть страницу игры в магазине Steam"
                  >
                    <SteamLogo size={14} weight="bold" />
                    <span>В Steam</span>
                    <ExternalLink class="w-3 h-3 text-[#8e95a2]" />
                  </button>
                </div>
              </div>

              <!-- Reviews Body -->
              {#if isLoadingReviews}
                <div class="space-y-4">
                  {#each [1, 2, 3] as _}
                    <div class="p-5 rounded-md bg-[#0e1219] border border-white/10 space-y-3 animate-pulse">
                      <div class="h-5 w-32 bg-white/10 rounded"></div>
                      <div class="space-y-2">
                        <div class="h-3.5 w-full bg-white/5 rounded"></div>
                        <div class="h-3.5 w-3/4 bg-white/5 rounded"></div>
                      </div>
                    </div>
                  {/each}
                </div>
              {:else if steamReviews.length === 0}
                <div class="p-8 rounded-md bg-[#0e1219] border border-white/10 text-center text-sm text-[#8e95a2] space-y-2">
                  <p>Отзывов не найдено{reviewLanguage === 'russian' ? ' на русском языке' : ''}.</p>
                  {#if reviewLanguage === 'russian'}
                    <button
                      data-nav-item
                      type="button"
                      class="text-sky-400 hover:text-sky-300 underline font-medium cursor-pointer"
                      onclick={() => handleLanguageChange('all')}
                    >
                      Показать отзывы на всех языках
                    </button>
                  {/if}
                </div>
              {:else}
                <div class="space-y-4">
                  {#each steamReviews as rev (rev.id)}
                    <div class="p-5 rounded-md bg-[#0e1219] border border-white/10 hover:border-white/20 transition-colors space-y-3 shadow-md">
                      <div class="flex flex-wrap items-center justify-between gap-2">
                        <div class="flex items-center gap-2.5">
                          {#if rev.votedUp}
                            <div class="inline-flex items-center gap-1.5 px-3 py-1 rounded bg-sky-500/15 border border-sky-500/30 text-sky-400 text-xs font-bold">
                              <ThumbsUp class="w-3.5 h-3.5" weight="fill" />
                              <span>Рекомендую</span>
                            </div>
                          {:else}
                            <div class="inline-flex items-center gap-1.5 px-3 py-1 rounded bg-rose-500/15 border border-rose-500/30 text-rose-400 text-xs font-bold">
                              <ThumbsDown class="w-3.5 h-3.5" weight="fill" />
                              <span>Не рекомендую</span>
                            </div>
                          {/if}

                          <span class="text-xs text-white font-mono font-medium">
                            {rev.playtimeHours} в игре
                          </span>

                          {#if rev.playtimeAtReview}
                            <span class="text-xs text-[#64748b]">
                              ({rev.playtimeAtReview} на момент отзыва)
                            </span>
                          {/if}
                        </div>

                        <span class="text-xs text-[#64748b]">
                          {formatReviewDate(rev.timestampCreated)}
                        </span>
                      </div>

                      <div class="text-sm text-[#d1d5db] leading-relaxed break-words">
                        {@html formatSteamReviewBBCode(rev.review)}
                      </div>

                      {#if rev.votesUp > 0 || rev.votesFunny > 0}
                        <div class="pt-2 border-t border-white/[0.06] flex items-center gap-3 text-xs text-[#8e95a2]">
                          {#if rev.votesUp > 0}
                            <span class="inline-flex items-center gap-1">
                              <ThumbsUp class="w-3 h-3 text-[#8e95a2]" />
                              <span>{rev.votesUp} посчитали полезным</span>
                            </span>
                          {/if}
                          {#if rev.votesFunny > 0}
                            <span>😄 {rev.votesFunny}</span>
                          {/if}
                        </div>
                      {/if}
                    </div>
                  {/each}
                </div>

                <!-- Load More Button -->
                {#if steamReviewsHasMore}
                  <div class="pt-4 text-center">
                    <button
                      data-nav-item
                      type="button"
                      disabled={isLoadingMoreReviews}
                      class="px-6 py-3 rounded bg-white/10 hover:bg-white/20 border border-white/15 text-xs font-bold text-white transition-all cursor-pointer inline-flex items-center gap-2 disabled:opacity-50 focus:ring-1 focus:ring-white focus:outline-none"
                      onclick={() => loadSteamReviews(false)}
                    >
                      {#if isLoadingMoreReviews}
                        <RefreshCw class="w-4 h-4 animate-spin text-sky-400" />
                        <span>Загрузка отзывов...</span>
                      {:else}
                        <ChevronDown class="w-4 h-4" />
                        <span>Показать ещё отзывы</span>
                      {/if}
                    </button>
                  </div>
                {/if}
              {/if}
            </div>
          {/if}
        </div>

        <!-- Footer Control Strip with clear Gamepad glyphs -->
        <div class="px-6 sm:px-8 py-3.5 bg-[#0c0e14] border-t border-white/10 flex items-center justify-between text-xs text-[#8e95a2] flex-shrink-0">
          <div class="flex items-center gap-5">
            <span class="flex items-center gap-1.5">
              <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-mono text-[10px]">D-Pad ↑ / ↓</span>
              <span>Прокрутка</span>
            </span>
            <span class="flex items-center gap-1.5">
              <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-mono text-[10px]">LB / RB</span>
              <span>Смена разделов</span>
            </span>
          </div>
          <div class="flex items-center gap-3">
            <span class="flex items-center gap-1.5">
              <span class="w-4 h-4 rounded-full bg-white/20 text-white font-bold text-[10px] flex items-center justify-center">B</span>
              <span>Назад к игре</span>
            </span>
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- 4. Download Variant Wizard Modal (PS5 Console Style, 80% screen, single-column, large typography) -->
  {#if isDownloadWizardOpen}
    <div
      data-nav-zone="modal"
      class="fixed inset-0 z-50 bg-black/90 backdrop-blur-md flex items-center justify-center p-4 sm:p-8 animate-fade-in select-none"
    >
      <div class="w-[82vw] max-w-6xl h-[80vh] min-h-[560px] bg-[#07080a] border border-white/10 rounded-md flex flex-col overflow-hidden shadow-2xl relative">
        <!-- Ambient Game Background Artwork (Subtle PS5 Hub immersion) -->
        {#if currentBackdropUrl}
          <div class="absolute inset-0 z-0 pointer-events-none overflow-hidden opacity-10">
            <img
              src={currentBackdropUrl}
              alt=""
              class="w-full h-full object-cover filter blur-sm scale-105"
            />
            <div class="absolute inset-0 bg-gradient-to-b from-[#07080a]/60 via-[#07080a]/90 to-[#07080a]"></div>
          </div>
        {/if}

        <div class="relative z-10 flex flex-col h-full">
          <!-- 1. Top Bar: Console Header & Close Button -->
          <div class="px-8 sm:px-10 pt-7 pb-5 flex items-start justify-between flex-shrink-0 border-b border-white/10 bg-[#0c0f16]/95">
            <div class="min-w-0 flex-1 pr-6">
              <div class="flex items-center gap-2.5 text-xs font-black uppercase font-mono tracking-widest text-[#8e95a2] mb-1.5">
                <Disc class="w-4 h-4 text-sky-400 flex-shrink-0" />
                <span>Мастер установки и выбор издания</span>
                <span class="text-white/20">•</span>
                <span>{activeVariantsList.length} {activeVariantsList.length === 1 ? 'релиз' : activeVariantsList.length < 5 ? 'релиза' : 'релизов'}</span>
              </div>
              <h2 class="text-2xl sm:text-3xl font-black text-white tracking-tight leading-tight truncate">
                {getDisplayTitle(activeGame) || 'Выбор издания игры'}
              </h2>
              <p class="text-sm sm:text-base text-[#8e95a2] mt-1 truncate">
                Выберите подходящий источник для скачивания на консоль
              </p>
            </div>

            <div class="flex items-center gap-3 flex-shrink-0 pt-1">
              <button
                data-nav-item
                type="button"
                class="px-4 py-2 rounded-sm bg-white/10 hover:bg-white/20 border border-white/10 text-white text-xs sm:text-sm font-bold flex items-center gap-2.5 cursor-pointer focus:ring-1 focus:ring-white focus:outline-none transition-colors"
                onclick={closeDownloadWizard}
              >
                <span>Закрыть</span>
                <span class="w-4 h-4 rounded-sm bg-white/20 text-[10px] flex items-center justify-center font-bold">B</span>
              </button>
            </div>
          </div>

          <!-- 2. Installation Destination Strip -->
          <div class="px-8 sm:px-10 py-4 bg-[#090c12]/95 border-b border-white/10 flex items-center justify-between gap-4 flex-shrink-0">
            <div class="flex items-center gap-3.5 min-w-0">
              <div class="w-10 h-10 rounded-sm bg-white/5 border border-white/10 flex items-center justify-center flex-shrink-0 text-sky-400">
                <FolderOpen class="w-5 h-5" />
              </div>
              <div class="min-w-0">
                <div class="text-[11px] font-mono font-bold uppercase tracking-wider text-[#8e95a2]">
                  Папка назначения
                </div>
                <div class="text-sm sm:text-base font-mono font-semibold text-white truncate">
                  {customDownloadPath || downloadPath}
                </div>
              </div>
            </div>

            <button
              data-nav-item
              type="button"
              class="px-4 py-2 rounded-sm bg-white/10 hover:bg-white/20 border border-white/10 text-white font-bold text-xs sm:text-sm flex items-center gap-2 flex-shrink-0 cursor-pointer focus:ring-1 focus:ring-white focus:outline-none transition-colors"
              onclick={handleBrowseFolder}
            >
              <span>Изменить путь</span>
              <span class="w-4 h-4 rounded-sm bg-white/20 text-[10px] flex items-center justify-center font-bold">X</span>
            </button>
          </div>

          <!-- 3. Scrollable Releases Cards List (No Sidebars, Large Console Cards) -->
          <div class="flex-1 overflow-y-auto px-8 sm:px-10 py-6 space-y-4 scrollbar-none">
            <div class="flex items-center justify-between pb-1">
              <span class="text-xs font-black uppercase tracking-widest text-[#8e95a2] font-mono">
                Доступные варианты загрузки
              </span>
              <span class="text-xs font-mono text-[#64748b]">
                D-pad ↑ / ↓ для выбора
              </span>
            </div>

            {#each activeVariantsList as variant, idx (variant.id)}
              {@const isSel = (selectedVariantId || activeVariant?.id) === variant.id}
              {@const isTorrent = variant.sourceType === 'torrent' || !!variant.magnetUri}
              {@const srcName = isTorrent ? (formatTorrentSourceName(variant.torrentSource) || 'Торрент') : 'FTP-сервер'}
              <button
                data-nav-item
                type="button"
                class="w-full text-left p-5 sm:p-6 rounded-md border transition-all cursor-pointer flex flex-col md:flex-row md:items-center justify-between gap-5 focus:outline-none {isSel ? 'bg-white/[0.08] border-white shadow-xl ring-1 ring-white' : 'bg-[#0c0f16]/90 border-white/10 hover:bg-white/[0.04] hover:border-white/25 text-[#cbd5e1]'}"
                onclick={() => {
                  selectedVariantId = variant.id;
                  confirmDownloadVariant(variant.id);
                }}
              >
                <!-- Left Info Column -->
                <div class="min-w-0 flex-1 space-y-2.5">
                  <div class="flex items-center gap-3">
                    <span class="px-2 py-0.5 rounded-sm bg-white/10 font-mono text-xs font-bold text-white/80">
                      #{idx + 1}
                    </span>
                    <div class="text-base sm:text-lg font-bold text-white tracking-tight leading-snug truncate">
                      {variant.rawName || getDisplayTitle(activeGame)}
                    </div>
                  </div>

                  <!-- Metadata Badges Row -->
                  <div class="flex items-center gap-2.5 flex-wrap text-xs sm:text-sm">
                    <!-- Source Pill -->
                    <span class="px-3 py-1 rounded-sm bg-white/10 border border-white/10 text-white font-medium flex items-center gap-1.5">
                      {#if isTorrent}
                        <Magnet class="w-3.5 h-3.5 text-sky-400 flex-shrink-0" />
                      {:else}
                        <Folder class="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />
                      {/if}
                      <span>{srcName}</span>
                    </span>

                    <!-- Torrent Seeds -->
                    {#if isTorrent}
                      {#if variantSeeds[variant.id]?.loading}
                        <span class="px-3 py-1 rounded-sm bg-white/5 border border-white/10 text-[#8e95a2] animate-pulse text-xs font-mono">
                          сиды: проверка...
                        </span>
                      {:else if variantSeeds[variant.id]}
                        {@const s = variantSeeds[variant.id].seeders}
                        {@const l = variantSeeds[variant.id].leechers}
                        <span class="px-3 py-1 rounded-sm border font-semibold flex items-center gap-1.5 {s > 0 ? 'bg-emerald-500/15 border-emerald-500/30 text-emerald-400' : 'bg-white/5 border-white/10 text-[#8e95a2]'}">
                          <UploadSimple class="w-3.5 h-3.5 stroke-[2.5]" />
                          <span>{formatSeedsCount(s)}</span>
                          {#if l > 0}
                            <span class="text-white/30">•</span>
                            <span class="text-[#8e95a2] text-xs font-normal">{l} пиров</span>
                          {/if}
                        </span>
                      {/if}
                    {/if}

                    <!-- Fast Download Ready Badge -->
                    {#if isSel}
                      <span class="px-2.5 py-1 rounded-sm bg-sky-500/20 border border-sky-500/30 text-sky-300 font-bold text-xs uppercase tracking-wider">
                        Выбрано
                      </span>
                    {/if}
                  </div>
                </div>

                <!-- Right Actions & Size Column -->
                <div class="flex items-center justify-between md:justify-end gap-5 flex-shrink-0 pt-2 md:pt-0 border-t md:border-t-0 border-white/10">
                  <!-- Size -->
                  <div class="text-right">
                    <div class="text-[11px] font-mono uppercase text-[#8e95a2] tracking-wider">Размер</div>
                    <div class="text-lg sm:text-xl font-mono font-black text-white">
                      {variant.sizeDisplay || '—'}
                    </div>
                  </div>

                  <!-- CTA Button -->
                  <div class="px-5 py-2.5 rounded-sm font-bold text-xs sm:text-sm flex items-center gap-2.5 transition-all shadow-sm {isSel ? 'bg-white text-black font-black' : 'bg-white/10 hover:bg-white/20 text-white border border-white/10'}">
                    <span>{isSel ? 'Скачать сейчас' : 'Выбрать'}</span>
                    <span class="w-5 h-5 rounded-sm {isSel ? 'bg-black/20 text-black' : 'bg-white/20 text-white'} text-xs flex items-center justify-center font-black font-mono">A</span>
                  </div>
                </div>
              </button>
            {/each}
          </div>

          <!-- 4. Console Bottom Bar with Controller Glyphs -->
          <div class="px-8 sm:px-10 py-4 bg-[#080a0f]/95 border-t border-white/10 flex items-center justify-between flex-shrink-0 text-xs sm:text-sm text-[#8e95a2]">
            <div class="flex items-center gap-5 sm:gap-7 flex-wrap">
              <span class="flex items-center gap-2">
                <span class="w-5 h-5 rounded-sm bg-white/20 text-white font-bold text-xs flex items-center justify-center font-mono">A</span>
                <span class="font-semibold text-white">Скачать выбранный релиз</span>
              </span>
              <span class="flex items-center gap-2">
                <span class="w-5 h-5 rounded-sm bg-white/20 text-white font-bold text-xs flex items-center justify-center font-mono">X</span>
                <span>Сменить папку установки</span>
              </span>
              <span class="hidden sm:flex items-center gap-2 text-[#64748b]">
                <span class="px-1.5 py-0.5 rounded-sm bg-white/10 text-[#cbd5e1] font-mono text-[10px]">↑ / ↓</span>
                <span>Навигация по списку</span>
              </span>
            </div>

            <button
              data-nav-item
              type="button"
              class="px-4 py-2 rounded-sm bg-white/10 hover:bg-white/20 text-white font-semibold cursor-pointer focus:ring-1 focus:ring-white focus:outline-none flex items-center gap-2 transition-colors"
              onclick={closeDownloadWizard}
            >
              <span>Отмена</span>
              <span class="w-4 h-4 rounded-sm bg-white/20 text-[10px] flex items-center justify-center font-bold">B</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- 5. Unified Media Viewer (Trailers + Screenshots Overlay HUD) -->
  {#if isTheaterMode}
    <div
      data-nav-zone="modal"
      class="fixed inset-0 z-[100] flex flex-col justify-between p-6 sm:p-8 pointer-events-none select-none animate-fade-in"
      role="presentation"
    >
      <!-- Top Bar: Title, Media Type Badge, Index & Close Button -->
      <div class="flex items-center justify-between pointer-events-none w-full">
        <div class="pointer-events-auto flex items-center gap-3 px-4 py-2.5 rounded bg-[#07080a]/95 text-white text-xs font-bold border border-white/10 shadow-2xl max-w-xl">
          {#if activeMedia?.type === 'video'}
            <Film class="w-4 h-4 text-sky-400 flex-shrink-0" />
            <span class="px-1.5 py-0.5 rounded bg-sky-500/20 text-sky-400 text-[10px] uppercase font-bold tracking-wider">Видео</span>
          {:else}
            <ImageIcon class="w-4 h-4 text-emerald-400 flex-shrink-0" />
            <span class="px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-400 text-[10px] uppercase font-bold tracking-wider">Скриншот</span>
          {/if}
          <span class="truncate">{activeMedia?.title || 'Медиа'}</span>
          {#if unifiedMediaList.length > 1}
            <span class="text-[11px] font-mono text-[#8e95a2] flex-shrink-0">
              ({currentMediaIndex + 1} / {unifiedMediaList.length})
            </span>
          {/if}
        </div>

        <button
          data-nav-item
          type="button"
          class="pointer-events-auto px-4 py-2 rounded bg-black/85 hover:bg-black text-white/90 hover:text-white text-xs font-semibold flex items-center gap-2.5 border border-white/10 cursor-pointer transition-all shadow-2xl active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
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

      <!-- Center: Navigation Arrows over Fullscreen Media -->
      <div class="pointer-events-none flex-1 flex items-center justify-between px-2 sm:px-4">
        {#if unifiedMediaList.length > 1}
          <button
            data-nav-item
            type="button"
            class="pointer-events-auto p-3.5 rounded-full bg-black/60 hover:bg-black text-white/80 hover:text-white border border-white/10 cursor-pointer transition-all active:scale-95 focus:ring-1 focus:ring-white focus:outline-none shadow-2xl backdrop-blur-sm"
            onclick={prevMedia}
            title="Предыдущий [←] или [LB]"
          >
            <ChevronLeft class="w-6 h-6" />
          </button>
          <button
            data-nav-item
            type="button"
            class="pointer-events-auto p-3.5 rounded-full bg-black/60 hover:bg-black text-white/80 hover:text-white border border-white/10 cursor-pointer transition-all active:scale-95 focus:ring-1 focus:ring-white focus:outline-none shadow-2xl backdrop-blur-sm"
            onclick={nextMedia}
            title="Следующий [→] или [RB]"
          >
            <ChevronRight class="w-6 h-6" />
          </button>
        {/if}
      </div>

      <!-- Bottom Bar: Playback Controls & Controller Navigation Hints -->
      <div class="flex items-center justify-between pointer-events-none w-full">
        <div class="pointer-events-auto flex items-center gap-2.5 bg-[#07080a]/95 p-2 rounded border border-white/10 shadow-2xl">
          {#if activeMedia?.type === 'video'}
            <button
              data-nav-item
              type="button"
              class="p-2.5 rounded-sm bg-white/10 hover:bg-white/20 text-white border border-white/10 cursor-pointer transition-all active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
              onclick={togglePlayPause}
              title="Пауза / Воспроизведение [Пробел] или [A]"
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
              class="p-2.5 rounded-sm bg-white/10 hover:bg-white/20 text-white border border-white/10 cursor-pointer transition-all active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
              onclick={toggleMute}
              title="Звук [M] или [X]"
            >
              {#if isVideoMuted}
                <VolumeX class="w-4 h-4 text-white/60" />
              {:else}
                <Volume2 class="w-4 h-4 text-white" />
              {/if}
            </button>
          {/if}

          {#if unifiedMediaList.length > 1}
            {#if activeMedia?.type === 'video'}
              <div class="h-4 w-[1px] bg-white/20 mx-0.5"></div>
            {/if}
            <button
              data-nav-item
              type="button"
              class="p-2.5 rounded-sm bg-white/10 hover:bg-white/20 text-white border border-white/10 cursor-pointer transition-all active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
              onclick={prevMedia}
              title="Предыдущий [←] или [LB]"
            >
              <ChevronLeft class="w-4 h-4" />
            </button>
            <button
              data-nav-item
              type="button"
              class="p-2.5 rounded-sm bg-white/10 hover:bg-white/20 text-white border border-white/10 cursor-pointer transition-all active:scale-95 focus:ring-1 focus:ring-white focus:outline-none"
              onclick={nextMedia}
              title="Следующий [→] или [RB]"
            >
              <ChevronRight class="w-4 h-4" />
            </button>
          {/if}
        </div>

        <!-- Gamepad Navigation Glyph Hints -->
        <div class="pointer-events-auto flex items-center gap-3 px-3.5 py-2 rounded bg-[#07080a]/95 text-xs text-[#8e95a2] border border-white/10 shadow-2xl">
          <span class="flex items-center gap-1.5">
            <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-mono text-[10px]">D-Pad ← / →</span>
            <span>Листать</span>
          </span>
          <span class="flex items-center gap-1.5">
            <span class="px-1.5 py-0.5 rounded bg-white/10 text-white font-mono text-[10px]">LB / RB</span>
            <span>Медиа</span>
          </span>
          <span class="flex items-center gap-1.5">
            <span class="w-4 h-4 rounded-full bg-white/20 text-white font-bold text-[10px] flex items-center justify-center">B</span>
            <span>Закрыть</span>
          </span>
        </div>
      </div>
    </div>
  {/if}

  <!-- 7. Steam Candidate Matching Modal -->
  {#if isSteamModalOpen}
    <div data-nav-zone="modal" class="fixed inset-0 z-50 bg-black/85 flex items-center justify-center p-6 animate-fade-in">
      <div class="w-full max-w-2xl bg-[#07080a] border border-white/10 rounded-md p-6 space-y-6 shadow-2xl max-h-[85vh] flex flex-col">
        
        <!-- Modal Header -->
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-base font-bold text-white">Привязка данных Steam</h2>
            <p class="text-xs text-[#8e95a2] mt-0.5">Выберите подходящую игру из Steam или введите AppID вручную</p>
          </div>
          <button
            data-nav-item
            class="p-2 rounded-sm text-[#8e95a2] hover:text-white hover:bg-white/10 cursor-pointer focus:ring-1 focus:ring-white focus:outline-none"
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
            class="flex-1 bg-[#11141c] text-white text-xs rounded px-4 py-3 border border-white/[0.08] focus:border-sky-400 focus:outline-none"
            onkeydown={(e) => {
              if (e.key === 'Enter') performSteamSearch();
            }}
          />
          <button
            data-nav-item
            disabled={isSearchingSteam}
            class="px-5 py-3 rounded bg-white/10 hover:bg-white/20 text-xs font-bold text-white flex items-center gap-2 cursor-pointer disabled:opacity-50 focus:ring-1 focus:ring-white focus:outline-none"
            onclick={() => performSteamSearch()}
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
                class="w-full flex items-center justify-between p-3 rounded-sm bg-[#0d1017] hover:bg-white/10 border border-white/[0.06] text-left cursor-pointer transition-colors focus:ring-1 focus:ring-white focus:outline-none"
                onclick={() => handleLinkAppId(cand.appId)}
              >
                <div class="flex items-center gap-3">
                  {#if cand.tinyImage}
                    <img src={cand.tinyImage} alt="" class="w-16 h-8 object-cover rounded-sm" />
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
              class="w-32 bg-[#11141c] text-white text-xs font-mono rounded px-3 py-2 border border-white/[0.08] focus:border-sky-400 focus:outline-none"
            />
            <button
              data-nav-item
              class="px-4 py-2 rounded bg-white/10 hover:bg-white/20 text-xs font-semibold text-white cursor-pointer focus:ring-1 focus:ring-white focus:outline-none"
              onclick={handleApplyDirectAppId}
            >
              Применить
            </button>
          </div>

          <div class="flex items-center gap-4">
            {#if activeGame?.steamAppId}
              <button
                data-nav-item
                class="text-xs text-rose-400 hover:underline cursor-pointer focus:ring-1 focus:ring-white focus:outline-none"
                onclick={handleUnlinkMetadata}
              >
                Отвязать метаданные
              </button>
            {/if}

            <span class="flex items-center gap-1.5 text-xs text-[#8e95a2]">
              <span class="w-4 h-4 rounded-full bg-white/20 text-white font-bold text-[10px] flex items-center justify-center">B</span>
              <span>Назад</span>
            </span>
          </div>
        </div>

      </div>
    </div>
  {/if}

</div>