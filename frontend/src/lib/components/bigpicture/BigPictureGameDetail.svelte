<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    ArrowLeft,
    Play,
    Download,
    Check,
    Star,
    Folder,
    FolderOpen,
    Volume2,
    VolumeX,
    Pause,
    ChevronLeft,
    ChevronRight,
    ChevronDown,
    X,
    Search,
    RefreshCw,
    Edit3,
    Disc,
    ArrowDown
  } from 'lucide-svelte';
  import Hls from 'hls.js';
  import { sound } from '../../navigation/audio';
  import * as AppAPI from '../../../../wailsjs/go/main/App';
  import type {
    GameEntity,
    SteamMovie,
    SteamCandidateItem,
    GamePageDetails
  } from '../../types/game';

  let {
    game = null as any,
    downloadPath = 'C:\\Ducke',
    onBack = () => {},
    onStartDownload = (gameId: number, targetPath: string) => {},
    onSelectFolder = async (): Promise<string> => '',
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
    const srcName = (v.torrentSource || '').trim();
    if (isTorrent) {
      return {
        type: 'torrent',
        name: srcName ? `Торрент (${srcName})` : 'Торрент',
        shortName: srcName || 'Торрент',
        badgeClass: 'bg-sky-500/15 border-sky-500/30 text-sky-400'
      };
    }
    return {
      type: 'ftp',
      name: 'FTP-сервер',
      shortName: 'FTP',
      badgeClass: 'bg-emerald-500/15 border-emerald-500/30 text-emerald-400'
    };
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
    if (m.hls) return sanitizeMediaUrl(m.hls);
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
    destroyHls();

    const isHls = targetUrl.includes('.m3u8') || targetUrl.includes('hls_264');

    if (isHls && Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: false,
        lowLatencyMode: false,
        backBufferLength: 10,
        maxBufferLength: 15,
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
          destroyHls();
          handleTrailerEnded();
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

  function stopVideoPlayback() {
    destroyHls();
    lastLoadedSource = '';
    safePause();
  }

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
  }

  function exitTheaterMode() {
    if (Date.now() - theaterEnterTimestamp < 350) return;
    sound.playBack();
    isTheaterMode = false;
    isVideoMuted = true;
    if (videoBgEl) {
      videoBgEl.muted = true;
    }
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
      isTheaterMode = false;
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
      onStartDownload(selectedVariantId || activeGame.id, customDownloadPath);
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
      const selected = await onSelectFolder();
      if (selected) {
        customDownloadPath = selected;
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
      if (isSteamModalOpen) return;

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
        }
        return;
      }

      if (e.key === 'ArrowUp' && movieList.length > 0) {
        enterTheaterMode();
        e.preventDefault();
      } else if (e.key === 'q' || e.key === 'Q') {
        cycleTab(-1);
      } else if (e.key === 'e' || e.key === 'E') {
        cycleTab(1);
      }
    };

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
      } else {
        if (dir === 'UP' && movieList.length > 0) {
          enterTheaterMode();
        }
      }
    };

    const handleGoBack = (e: CustomEvent) => {
      if (isTheaterMode) {
        exitTheaterMode();
        e.preventDefault();
      } else if (lightboxImage) {
        closeLightbox();
        e.preventDefault();
      } else if (isSteamModalOpen) {
        closeSteamModal();
        e.preventDefault();
      } else if (isVariantDropdownOpen) {
        isVariantDropdownOpen = false;
        e.preventDefault();
      } else if (isFavoriteDropdownOpen) {
        isFavoriteDropdownOpen = false;
        e.preventDefault();
      } else {
        onBack();
      }
    };

    const handleSubtabPrev = (e: Event) => {
      if (!isTheaterMode && !lightboxImage && !isSteamModalOpen) {
        cycleTab(-1);
        e.preventDefault();
      }
    };

    const handleSubtabNext = (e: Event) => {
      if (!isTheaterMode && !lightboxImage && !isSteamModalOpen) {
        cycleTab(1);
        e.preventDefault();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('pointerdown', handleWindowPointerDown, true);
    window.addEventListener('app:gamepad-dir', handleGamepadDir as EventListener);
    window.addEventListener('app:go-back', handleGoBack as EventListener);
    window.addEventListener('app:subtab-prev', handleSubtabPrev as EventListener);
    window.addEventListener('app:subtab-next', handleSubtabNext as EventListener);

    return () => {
      isMounted = false;
      destroyHls();
      if (trailerTimer) clearTimeout(trailerTimer);
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('pointerdown', handleWindowPointerDown, true);
      window.removeEventListener('app:gamepad-dir', handleGamepadDir as EventListener);
      window.removeEventListener('app:go-back', handleGoBack as EventListener);
      window.removeEventListener('app:subtab-prev', handleSubtabPrev as EventListener);
      window.removeEventListener('app:subtab-next', handleSubtabNext as EventListener);
    };
  });

  onDestroy(() => {
    isMounted = false;
    destroyHls();
    if (trailerTimer) clearTimeout(trailerTimer);
  });
</script>

<div class="w-full h-full flex flex-col relative overflow-hidden bg-[#07080a] select-none text-white">

  <!-- 1. Fullscreen Cinematic Background Layer: Still Art + Live Ambient Trailer Video -->
  <div class="absolute inset-0 z-0 overflow-hidden pointer-events-none">
    <!-- Static Backdrop Image -->
    {#if currentBackdropUrl}
      <img
        src={currentBackdropUrl}
        alt=""
        class="w-full h-full object-cover object-center transition-opacity duration-700 ease-out {isAmbientTrailerActive && activeMovie ? 'opacity-0' : 'opacity-60'}"
        decoding="async"
      />
    {/if}

    <!-- Live Fullscreen Background Video Trailer -->
    <video
      bind:this={videoBgEl}
      poster={activeMovie?.thumbnail ? sanitizeMediaUrl(activeMovie.thumbnail) : ''}
      class="absolute inset-0 w-full h-full object-cover transition-opacity duration-700 ease-out {isAmbientTrailerActive && activeMovie?.src ? 'opacity-100' : 'opacity-0 pointer-events-none'}"
      muted={isVideoMuted}
      playsinline
      preload="auto"
      onended={handleTrailerEnded}
    ></video>

    <!-- Ambient vignette & falloff gradients -->
    <div class="absolute inset-0 bg-gradient-to-t from-[#07080a] via-[#07080a]/70 to-transparent transition-opacity duration-500 {isTheaterMode ? 'opacity-20' : 'opacity-100'}"></div>
    <div class="absolute inset-0 bg-gradient-to-r from-[#07080a]/95 via-[#07080a]/50 to-transparent transition-opacity duration-500 {isTheaterMode ? 'opacity-15' : 'opacity-100'}"></div>
  </div>

  <!-- 2. Sticky Top Bar (Back Button & Steam Link) -->
  <div class="relative z-10 px-8 sm:px-12 lg:px-16 pt-5 pb-2 flex items-center justify-between transition-all duration-300 {isTheaterMode ? 'opacity-0 pointer-events-none -translate-y-4' : 'opacity-100 translate-y-0'}">
    <button
      data-nav-item
      type="button"
      class="px-4 py-2 rounded-xl bg-black/60 hover:bg-black/80 backdrop-blur-md border border-white/10 text-xs font-bold text-white hover:border-white/30 flex items-center gap-2.5 cursor-pointer transition-all active:scale-95 focus:ring-2 focus:ring-white focus:outline-none shadow-lg"
      onclick={() => {
        sound.playBack();
        onBack();
      }}
    >
      <ArrowLeft class="w-4 h-4" />
      <span>Назад к библиотеке</span>
      <span class="w-4 h-4 rounded bg-white/20 text-[10px] flex items-center justify-center font-bold">B</span>
    </button>

    <div class="flex items-center gap-2">
      {#if isEnrichingCurrentGame}
        <div class="flex items-center gap-2 px-3 py-1.5 rounded-xl bg-sky-500/10 border border-sky-500/20 text-sky-400 text-xs font-mono animate-pulse">
          <RefreshCw class="w-3.5 h-3.5 animate-spin" />
          <span>Поиск данных...</span>
        </div>
      {/if}

      <button
        data-nav-item
        type="button"
        class="px-3.5 py-1.5 rounded-xl bg-black/60 hover:bg-black/80 backdrop-blur-md border border-white/10 text-xs font-semibold text-[#cbd5e1] hover:text-white flex items-center gap-2 cursor-pointer transition-colors focus:ring-2 focus:ring-white focus:outline-none shadow-lg"
        onclick={openSteamModal}
        title="Привязать Steam / Изменить метаданные"
      >
        <Edit3 class="w-3.5 h-3.5 text-sky-400" />
        <span>{activeGame?.steamAppId > 0 ? `Steam AppID: ${activeGame.steamAppId}` : (activeGame?.steamAppId < 0 ? `SteamGridDB: ${-activeGame.steamAppId}` : 'Привязать Steam')}</span>
      </button>
    </div>
  </div>

  <!-- 3. Upper Dashboard Hero Stage -->
  <div
    data-nav-zone="detail"
    class="relative z-10 flex-1 min-h-0 flex flex-col justify-end px-8 sm:px-12 lg:px-16 pt-2 pb-5 overflow-hidden transition-all duration-500 ease-out {isTheaterMode ? 'opacity-0 pointer-events-none -translate-y-4' : 'opacity-100 translate-y-0'}"
  >
    {#if activeGame}
      <div class="w-full flex flex-col lg:flex-row items-end justify-between gap-8 lg:gap-14">

        <!-- Left Hero: Official Game Logo, Badges, Synopsis & Tactile Action Buttons -->
        <div class="flex-1 min-w-0 flex flex-col justify-end space-y-3.5 max-w-2xl">
          
          <!-- Official Steam Logo or Bold Title -->
          <div class="min-h-[80px] sm:min-h-[110px] flex items-end">
            {#if activeGame.steamAppId && !logoFailedMap[activeGame.steamAppId]}
              <img
                src="https://shared.steamstatic.com/store_item_assets/steam/apps/{activeGame.steamAppId}/logo.png"
                alt={activeGame.cleanTitle || activeGame.rawName}
                class="max-h-24 sm:max-h-32 md:max-h-36 max-w-[320px] sm:max-w-lg object-contain object-left-bottom filter drop-shadow-[0_12px_24px_rgba(0,0,0,0.8)] transition-all duration-500"
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

          <!-- Console Meta Chips Line -->
          <div class="flex flex-wrap items-center gap-2">
            <span class="px-2 py-0.5 rounded bg-white/10 text-white text-[10px] font-black tracking-widest border border-white/20 uppercase">
              PC
            </span>

            {#if isCompleted}
              <span class="px-2.5 py-0.5 rounded-full bg-emerald-500/20 border border-emerald-500/30 text-emerald-400 text-xs font-bold flex items-center gap-1.5">
                <Check class="w-3.5 h-3.5 stroke-[3]" />
                <span>Установлено</span>
              </span>
            {:else if isDownloading}
              <span class="px-2.5 py-0.5 rounded-full bg-sky-500/20 border border-sky-500/30 text-sky-400 text-xs font-bold flex items-center gap-1.5">
                <Download class="w-3.5 h-3.5 stroke-[2.5]" />
                <span>{Math.round(activeDownload?.progress || pageDetails?.downloadProgress?.progressPercent || 0)}%</span>
              </span>
            {/if}

            {#if downloadSourceInfo}
              <span class="px-2.5 py-0.5 rounded-full border text-xs font-bold flex items-center gap-1.5 {downloadSourceInfo.badgeClass}">
                <span>Источник: {downloadSourceInfo.name}</span>
              </span>
            {/if}

            {#if currentFavoriteStatus}
              <span class="px-2.5 py-0.5 rounded-full bg-amber-500/20 border border-amber-500/30 text-amber-400 text-xs font-bold flex items-center gap-1.5">
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

            {#if activeGame.releaseDate || activeGame.steamReleaseDate}
              <span class="text-xs font-semibold text-[#8e95a2]">
                {activeGame.releaseDate || activeGame.steamReleaseDate}
              </span>
              <span class="text-white/20">•</span>
            {/if}

            {#if activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr}
              <span class="text-xs font-semibold text-[#8e95a2]">
                {activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr}
              </span>
              <span class="text-white/20">•</span>
            {/if}

            {#if activeGame.reviewPercent}
              <span class="text-xs font-bold text-white flex items-center gap-1">
                <Star class="w-3.5 h-3.5 text-amber-400 fill-amber-400" />
                <span>{activeGame.reviewPercent}%</span>
              </span>
              <span class="text-white/20">•</span>
            {/if}

            {#if activeGame.genres && Array.isArray(activeGame.genres) && activeGame.genres.length > 0}
              <span class="text-xs text-[#8e95a2]">
                {activeGame.genres.slice(0, 2).join(', ')}
              </span>
            {/if}
          </div>

          <!-- Synopsis / Short Description -->
          {#if activeGame.shortDescription}
            <p class="text-xs sm:text-sm text-[#cbd5e1] leading-relaxed line-clamp-2 max-w-xl font-normal drop-shadow-sm">
              {activeGame.shortDescription.replace(/<[^>]*>?/gm, '')}
            </p>
          {/if}

          <!-- Tactical Action Buttons -->
          <div class="flex items-center gap-3 pt-1">
            <!-- Primary Action Pill (A) -->
            <button
              data-nav-item
              type="button"
              class="px-7 py-3 rounded-full bg-white text-black font-extrabold text-sm flex items-center gap-3 hover:bg-slate-100 transition-all cursor-pointer shadow-2xl active:scale-95 focus:ring-4 focus:ring-white/40 focus:outline-none"
              onclick={handlePrimaryAction}
            >
              {#if isCompleted}
                <Play class="w-4 h-4 fill-black text-black" />
                <span>Играть</span>
              {:else if isDownloading}
                <Download class="w-4 h-4 stroke-[2.5]" />
                <span>Скачивается</span>
              {:else}
                <Download class="w-4 h-4 stroke-[2.5]" />
                <span>Загрузить</span>
              {/if}
              <span class="w-5 h-5 rounded-full bg-black/15 text-black text-[10px] font-black flex items-center justify-center">A</span>
            </button>

            <!-- Secondary Action (Folder / Versions) (X) -->
            {#if isCompleted}
              <button
                data-nav-item
                type="button"
                class="px-4 py-3 rounded-full bg-white/10 hover:bg-white/20 text-white font-bold text-xs flex items-center gap-2 border border-white/15 transition-all cursor-pointer active:scale-95 focus:ring-4 focus:ring-white/40 focus:outline-none"
                onclick={handleOpenGameFolder}
                title="Открыть папку с игрой"
              >
                <Folder class="w-4 h-4 fill-current text-[#cbd5e1]" />
                <span>Папка</span>
                <span class="w-4 h-4 rounded-full bg-white/15 text-[#cbd5e1] text-[9px] font-bold flex items-center justify-center">X</span>
              </button>
            {:else if activeVariantsList && activeVariantsList.length > 1}
              <div class="relative">
                <button
                  bind:this={variantDropdownTriggerEl}
                  data-nav-item
                  type="button"
                  class="px-4 py-3 rounded-full bg-white/10 hover:bg-white/20 text-white font-bold text-xs flex items-center gap-2 border border-white/15 transition-all cursor-pointer active:scale-95 focus:ring-4 focus:ring-white/40 focus:outline-none"
                  onclick={() => {
                    isVariantDropdownOpen = !isVariantDropdownOpen;
                  }}
                  title="Выбрать версию игры ({activeVariantsList.length} доступно)"
                >
                  <Disc class="w-4 h-4 text-sky-400" />
                  <span>Версия ({activeVariantsList.length})</span>
                  <span class="w-4 h-4 rounded-full bg-white/15 text-[#cbd5e1] text-[9px] font-bold flex items-center justify-center">X</span>
                </button>

                {#if isVariantDropdownOpen}
                  <div
                    bind:this={variantDropdownContainerEl}
                    class="absolute left-0 bottom-full mb-3 z-50 w-80 max-h-60 overflow-y-auto overscroll-contain rounded-2xl bg-[#0d1117] border border-white/10 shadow-2xl p-2 space-y-1 backdrop-blur-md"
                  >
                    <div class="px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-[#64748b]">
                      Доступные релизы ({activeVariantsList.length})
                    </div>
                    {#each activeVariantsList as variant (variant.id)}
                      {@const isSel = activeVariant?.id === variant.id}
                      <button
                        data-nav-item
                        type="button"
                        class="w-full text-left flex items-center justify-between gap-3 px-3 py-2.5 rounded-xl text-xs transition-colors cursor-pointer {isSel ? 'bg-white/15 text-white font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                        onclick={(e) => {
                          e.stopPropagation();
                          selectedVariantId = variant.id;
                          isVariantDropdownOpen = false;
                        }}
                      >
                        <div class="min-w-0 flex-1 pointer-events-none">
                          <div class="truncate text-white text-xs">{variant.rawName}</div>
                          <div class="text-[10px] text-[#64748b] font-mono">
                            Источник: {variant.sourceType === 'torrent' ? (variant.torrentSource ? `Торрент (${variant.torrentSource})` : 'Торрент') : 'FTP-сервер'}
                          </div>
                        </div>
                        <div class="flex items-center gap-1.5 flex-shrink-0 font-mono text-[11px] pointer-events-none {isSel ? 'text-sky-400 font-bold' : 'text-[#64748b]'}">
                          <span>{variant.sizeDisplay}</span>
                          {#if isSel}
                            <Check class="w-3.5 h-3.5 stroke-[2.5]" />
                          {/if}
                        </div>
                      </button>
                    {/each}
                  </div>
                {/if}
              </div>
            {:else}
              <button
                data-nav-item
                type="button"
                class="px-4 py-3 rounded-full bg-white/10 hover:bg-white/20 text-white font-bold text-xs flex items-center gap-2 border border-white/15 transition-all cursor-pointer active:scale-95 focus:ring-4 focus:ring-white/40 focus:outline-none"
                onclick={handleBrowseFolder}
                title="Выбрать папку для сохранения"
              >
                <FolderOpen class="w-4 h-4 text-[#cbd5e1]" />
                <span>Папка</span>
                <span class="w-4 h-4 rounded-full bg-white/15 text-[#cbd5e1] text-[9px] font-bold flex items-center justify-center">X</span>
              </button>
            {/if}

            <!-- Favorite Toggle Dropdown (Y) -->
            <div class="relative">
              <button
                bind:this={favoriteDropdownTriggerEl}
                data-nav-item
                type="button"
                class="p-3 rounded-full bg-white/10 hover:bg-white/20 text-white border border-white/15 transition-all cursor-pointer active:scale-95 focus:ring-4 focus:ring-white/40 focus:outline-none flex items-center gap-1.5"
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
                  class="absolute left-0 bottom-full mb-3 z-50 w-52 rounded-2xl bg-[#0d1117] border border-white/10 shadow-2xl p-1.5 space-y-1 backdrop-blur-md"
                >
                  <div class="px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-[#64748b]">
                    Статус игры
                  </div>
                  <button
                    data-nav-item
                    type="button"
                    class="w-full text-left flex items-center justify-between px-3 py-2 rounded-xl text-xs transition-colors cursor-pointer {currentFavoriteStatus === 'planned' ? 'bg-amber-500/20 text-amber-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                    onclick={() => handleToggleFavoriteStatus('planned')}
                  >
                    <span>В планах</span>
                    {#if currentFavoriteStatus === 'planned'}
                      <Check class="w-3.5 h-3.5 text-amber-400" />
                    {/if}
                  </button>

                  <button
                    data-nav-item
                    type="button"
                    class="w-full text-left flex items-center justify-between px-3 py-2 rounded-xl text-xs transition-colors cursor-pointer {currentFavoriteStatus === 'playing' ? 'bg-amber-500/20 text-amber-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                    onclick={() => handleToggleFavoriteStatus('playing')}
                  >
                    <span>Прохожу</span>
                    {#if currentFavoriteStatus === 'playing'}
                      <Check class="w-3.5 h-3.5 text-amber-400" />
                    {/if}
                  </button>

                  <button
                    data-nav-item
                    type="button"
                    class="w-full text-left flex items-center justify-between px-3 py-2 rounded-xl text-xs transition-colors cursor-pointer {currentFavoriteStatus === 'completed' ? 'bg-amber-500/20 text-amber-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                    onclick={() => handleToggleFavoriteStatus('completed')}
                  >
                    <span>Пройдено</span>
                    {#if currentFavoriteStatus === 'completed'}
                      <Check class="w-3.5 h-3.5 text-amber-400" />
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

            <!-- Watch Trailer Fullscreen Button (↑) -->
            {#if movieList.length > 0}
              <button
                data-nav-item
                type="button"
                class="px-4 py-3 rounded-full bg-white/10 hover:bg-white/20 text-white font-semibold text-xs flex items-center gap-2 border border-white/15 transition-all cursor-pointer active:scale-95 focus:ring-4 focus:ring-white/40 focus:outline-none"
                onclick={enterTheaterMode}
                title="Смотреть трейлер во весь экран [↑]"
              >
                <Volume2 class="w-4 h-4 text-sky-400" />
                <span>Трейлер</span>
                <span class="px-1.5 py-0.5 rounded bg-white/15 text-[10px] font-mono">↑</span>
              </button>
            {/if}

          </div>

          <!-- Download Source Indicator -->
          {#if downloadSourceInfo && !isCompleted}
            <div class="flex flex-wrap items-center gap-2 pt-1 text-xs text-[#8e95a2]">
              <span class="text-[#64748b]">Источник скачивания:</span>
              <span class="font-bold px-2 py-0.5 rounded-md border {downloadSourceInfo.badgeClass}">
                {downloadSourceInfo.name}
              </span>
              {#if (activeVariant?.rawName || activeGame?.rawName)}
                <span class="text-white/20">•</span>
                <span class="text-[#8e95a2] font-mono text-[11px] truncate max-w-md select-all" title={activeVariant?.rawName || activeGame?.rawName}>
                  {activeVariant?.rawName || activeGame?.rawName}
                </span>
              {/if}
            </div>
          {/if}

        </div>

        <!-- Right Side: Screenshots Carousel Strip -->
        {#if gameScreenshots.length > 0}
          <div class="w-full lg:w-[440px] xl:w-[480px] flex-shrink-0 flex flex-col justify-end">
            <div class="flex items-center gap-2.5 overflow-x-auto scrollbar-none py-1">
              {#each gameScreenshots as sc, idx}
                <button
                  data-nav-item
                  type="button"
                  class="w-20 sm:w-24 aspect-video rounded-xl overflow-hidden border border-white/15 bg-black/50 hover:border-white hover:scale-105 focus:border-white focus:scale-105 focus:outline-none transition-all flex-shrink-0 cursor-pointer relative group/thumb shadow-lg"
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
    {/if}
  </div>

  <!-- 4. Bottom Docked Tab Bar & Single Game Content Panel -->
  <div
    data-nav-zone="grid"
    class="relative z-20 w-full flex-shrink-0 bg-[#07080a] border-t border-white/[0.08] pt-2 pb-4 transition-opacity duration-300 ease-out {isTheaterMode ? 'opacity-0 pointer-events-none translate-y-6' : 'opacity-100 translate-y-0'}"
  >
    
    <!-- Category Filter Switcher Bar -->
    <div class="px-8 sm:px-12 lg:px-16 flex items-center justify-between pb-2.5">
      <div class="flex items-center gap-1.5 sm:gap-2">
        <button
          data-nav-item
          type="button"
          class="px-3.5 py-1 rounded-full text-xs font-semibold transition-colors cursor-pointer focus:outline-none {activeTab === 'about' ? 'bg-white/20 text-white border border-white/30' : 'text-[#8e95a2] hover:text-white border border-transparent'}"
          onclick={() => switchTab('about')}
        >
          Об игре
        </button>

        {#if gameScreenshots.length > 0}
          <button
            data-nav-item
            type="button"
            class="px-3.5 py-1 rounded-full text-xs font-semibold transition-colors cursor-pointer focus:outline-none {activeTab === 'screenshots' ? 'bg-white/20 text-white border border-white/30' : 'text-[#8e95a2] hover:text-white border border-transparent'}"
            onclick={() => switchTab('screenshots')}
          >
            Скриншоты ({gameScreenshots.length})
          </button>
        {/if}

        <button
          data-nav-item
          type="button"
          class="px-3.5 py-1 rounded-full text-xs font-semibold transition-colors cursor-pointer focus:outline-none {activeTab === 'specs' ? 'bg-white/20 text-white border border-white/30' : 'text-[#8e95a2] hover:text-white border border-transparent'}"
          onclick={() => switchTab('specs')}
        >
          Требования и инфо
        </button>

        {#if activeVariantsList && activeVariantsList.length > 1}
          <button
            data-nav-item
            type="button"
            class="px-3.5 py-1 rounded-full text-xs font-semibold transition-colors cursor-pointer focus:outline-none {activeTab === 'variants' ? 'bg-white/20 text-white border border-white/30' : 'text-[#8e95a2] hover:text-white border border-transparent'}"
            onclick={() => switchTab('variants')}
          >
            Версии ({activeVariantsList.length})
          </button>
        {/if}
      </div>

      <!-- Quick Gamepad bumper hint -->
      <div class="hidden sm:flex items-center gap-1.5 text-[11px] text-[#64748b] font-medium">
        <span class="px-1.5 py-0.5 rounded bg-white/10 font-mono text-[10px] text-white">LB</span>
        <span class="px-1.5 py-0.5 rounded bg-white/10 font-mono text-[10px] text-white">RB</span>
        <span>Смена вкладки</span>
      </div>
    </div>

    <!-- Active Tab Docked Content Panel -->
    <div class="px-8 sm:px-12 lg:px-16">
      <div class="w-full max-h-[30vh] sm:max-h-[34vh] overflow-y-auto rounded-2xl bg-[#0c0e14]/90 border border-white/[0.06] p-5 sm:p-6 shadow-inner text-sm scrollbar-thin">
        
        <!-- Tab 1: Detailed Overview -->
        {#if activeTab === 'about'}
          <div class="space-y-4">
            {#if activeGame?.detailedDescription}
              <div class="steam-html-content text-xs sm:text-sm text-[#cbd5e1] leading-relaxed">
                {@html activeGame.detailedDescription}
              </div>
            {:else if activeGame?.shortDescription}
              <div class="text-xs sm:text-sm text-[#cbd5e1] leading-relaxed">
                {@html activeGame.shortDescription}
              </div>
            {:else}
              <div class="text-xs text-[#64748b]">Описание отсутствует</div>
            {/if}

            {#if activeGame?.genres && activeGame.genres.length > 0}
              <div class="pt-3 border-t border-white/[0.06] flex items-center gap-2 flex-wrap text-xs">
                <span class="text-[#8e95a2] font-semibold">Жанры:</span>
                {#each activeGame.genres as g}
                  <span class="px-2.5 py-0.5 rounded-lg bg-white/[0.05] border border-white/[0.08] text-[#cbd5e1]">
                    {g}
                  </span>
                {/each}
              </div>
            {/if}
          </div>

        <!-- Tab 2: Screenshots Grid -->
        {:else if activeTab === 'screenshots'}
          <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
            {#each gameScreenshots as sc, idx}
              <button
                data-nav-item
                type="button"
                class="aspect-video rounded-xl overflow-hidden border border-white/15 bg-black/60 hover:border-white focus:border-white focus:outline-none transition-all cursor-pointer relative group/thumb shadow-md"
                onclick={() => openLightbox(idx)}
                title="Скриншот {idx + 1}"
              >
                <img src={sc} alt="" class="w-full h-full object-cover" />
                <div class="absolute inset-0 bg-white/10 opacity-0 group-hover/thumb:opacity-100 transition-opacity"></div>
                <span class="absolute bottom-1.5 right-1.5 px-1.5 py-0.5 rounded bg-black/70 text-[10px] font-mono text-white/80">
                  {idx + 1}
                </span>
              </button>
            {/each}
          </div>

        <!-- Tab 3: System Requirements & Specs -->
        {:else if activeTab === 'specs'}
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <!-- Left: Technical Specs Table -->
            <div class="space-y-2.5 text-xs divide-y divide-white/[0.04]">
              <div class="pt-1 flex items-center justify-between">
                <span class="text-[#8e95a2]">Разработчик</span>
                <span class="font-bold text-white text-right">{activeGame?.developers?.join(', ') || '—'}</span>
              </div>
              <div class="pt-2 flex items-center justify-between">
                <span class="text-[#8e95a2]">Издатель</span>
                <span class="font-bold text-white text-right">{activeGame?.publishers?.join(', ') || '—'}</span>
              </div>
              <div class="pt-2 flex items-center justify-between">
                <span class="text-[#8e95a2]">Дата выхода</span>
                <span class="font-bold text-white">{activeGame?.releaseDate || activeGame?.steamReleaseDate || '—'}</span>
              </div>
              <div class="pt-2 flex items-center justify-between">
                <span class="text-[#8e95a2]">Управление</span>
                <span class="font-bold text-sky-400">{activeGame?.controllerSupport === 'full' ? 'Полная поддержка геймпада' : 'Клавиатура / Мышь'}</span>
              </div>
              <div class="pt-2 flex items-center justify-between">
                <span class="text-[#8e95a2]">Размер в хранилище</span>
                <span class="font-mono font-bold text-white">{activeVariant?.sizeDisplay || activeGame?.sizeDisplay || activeGame?.sizeStr || '—'}</span>
              </div>
              <div class="pt-2 flex items-center justify-between">
                <span class="text-[#8e95a2]">Steam AppID</span>
                <span class="font-mono text-white">{activeGame?.steamAppId > 0 ? activeGame.steamAppId : 'Не привязан'}</span>
              </div>
              <div class="pt-2 flex items-center justify-between">
                <span class="text-[#8e95a2]">Источник скачивания</span>
                <span class="font-bold text-white">{downloadSourceInfo?.name || '—'}</span>
              </div>
              <div class="pt-2 flex items-center justify-between">
                <span class="text-[#8e95a2]">Путь сохранения</span>
                <span class="font-mono text-[#cbd5e1] truncate max-w-[280px]">{pageDetails?.localPath || customDownloadPath}</span>
              </div>
            </div>

            <!-- Right: PC Requirements -->
            <div class="space-y-2 text-xs">
              <span class="text-[#8e95a2] font-bold uppercase tracking-wider block">Системные требования</span>
              {#if activeGame?.pcRequirements}
                <div class="steam-html-content text-[#cbd5e1] leading-relaxed">
                  {@html activeGame.pcRequirements}
                </div>
              {:else}
                <div class="text-[#64748b]">Информация о системных требованиях отсутствует</div>
              {/if}
            </div>
          </div>

        <!-- Tab 4: Release Variants List -->
        {:else if activeTab === 'variants'}
          <div class="space-y-2">
            {#each activeVariantsList as variant (variant.id)}
              {@const isSel = activeVariant?.id === variant.id}
              <button
                data-nav-item
                type="button"
                class="w-full flex items-center justify-between p-3 rounded-xl border text-left cursor-pointer transition-colors focus:ring-2 focus:ring-white focus:outline-none {isSel ? 'bg-white/15 border-white/30 text-white font-bold' : 'bg-black/40 border-white/[0.06] text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                onclick={() => {
                  sound.playSelect();
                  selectedVariantId = variant.id;
                }}
              >
                <div class="min-w-0 flex-1">
                  <div class="text-xs font-semibold text-white truncate">{variant.rawName}</div>
                  <div class="text-[10px] text-[#64748b] font-mono mt-0.5">
                    Источник: {variant.sourceType === 'torrent' ? (variant.torrentSource ? `Торрент (${variant.torrentSource})` : 'Торрент') : 'FTP-сервер'}
                  </div>
                </div>

                <div class="flex items-center gap-3 font-mono text-xs">
                  <span class="{isSel ? 'text-sky-400 font-bold' : 'text-[#8e95a2]'}">{variant.sizeDisplay}</span>
                  {#if isSel}
                    <span class="px-2 py-0.5 rounded bg-sky-500/20 text-sky-400 text-[10px] font-bold">Выбрано</span>
                  {:else}
                    <span class="text-xs text-white/40">Выбрать [A]</span>
                  {/if}
                </div>
              </button>
            {/each}
          </div>
        {/if}

      </div>
    </div>

  </div>

  <!-- 5. Theater Mode Overlay HUD (Clean cinematic player, zero distracting badges) -->
  {#if isTheaterMode}
    <div
      data-nav-zone="modal"
      class="fixed inset-0 z-40 flex flex-col justify-between p-6 pointer-events-auto select-none"
      onclick={(e) => {
        if (e.target === e.currentTarget) {
          exitTheaterMode();
        }
      }}
      role="button"
      tabindex="-1"
      onkeydown={(e) => { if (e.key === 'Escape') exitTheaterMode(); }}
    >
      <!-- Top Right: Minimal discreet back badge -->
      <div class="flex items-center justify-end">
        <button
          data-nav-item
          type="button"
          class="px-3.5 py-1.5 rounded-xl bg-black/60 hover:bg-black/80 text-white/80 hover:text-white text-xs font-semibold flex items-center gap-2 border border-white/10 backdrop-blur-md cursor-pointer transition-colors shadow-lg"
          onclick={(e) => {
            e.stopPropagation();
            exitTheaterMode();
          }}
          title="Вернуться [↓] или [B]"
        >
          <ArrowDown class="w-3.5 h-3.5" />
          <span>Назад</span>
          <span class="w-4 h-4 rounded bg-white/20 text-[10px] flex items-center justify-center font-bold">B</span>
        </button>
      </div>

      <!-- Bottom: Subtle unobtrusive bar -->
      <div class="flex items-center justify-between pointer-events-none" role="presentation">
        <div class="pointer-events-auto flex items-center gap-2">
          <button
            data-nav-item
            type="button"
            class="p-2 rounded-xl bg-black/60 hover:bg-black/80 text-white/80 hover:text-white border border-white/10 backdrop-blur-md cursor-pointer transition-colors"
            onclick={togglePlayPause}
            title="Пауза / Воспроизведение [Space]"
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
            class="p-2 rounded-xl bg-black/60 hover:bg-black/80 text-white/80 hover:text-white border border-white/10 backdrop-blur-md cursor-pointer transition-colors"
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
            <button
              data-nav-item
              type="button"
              class="p-2 rounded-xl bg-black/60 hover:bg-black/80 text-white/80 hover:text-white border border-white/10 backdrop-blur-md cursor-pointer transition-colors"
              onclick={prevTrailer}
              title="Предыдущий [←]"
            >
              <ChevronLeft class="w-4 h-4" />
            </button>
            <button
              data-nav-item
              type="button"
              class="p-2 rounded-xl bg-black/60 hover:bg-black/80 text-white/80 hover:text-white border border-white/10 backdrop-blur-md cursor-pointer transition-colors"
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

  <!-- 6. Fullscreen Screenshot Lightbox Modal -->
  {#if lightboxImage}
    <div
      data-nav-zone="modal"
      class="fixed inset-0 z-50 bg-black/95 backdrop-blur-md flex flex-col items-center justify-center p-4 select-none"
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
        class="absolute top-6 right-6 p-2.5 rounded-full bg-white/10 hover:bg-white/20 text-white cursor-pointer transition-colors z-20 focus:ring-2 focus:ring-white/40 focus:outline-none"
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
          class="absolute left-6 top-1/2 -translate-y-1/2 p-3.5 rounded-full bg-white/10 hover:bg-white/20 text-white cursor-pointer transition-colors z-20 focus:ring-2 focus:ring-white/40 focus:outline-none"
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
          class="absolute right-6 top-1/2 -translate-y-1/2 p-3.5 rounded-full bg-white/10 hover:bg-white/20 text-white cursor-pointer transition-colors z-20 focus:ring-2 focus:ring-white/40 focus:outline-none"
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
        class="max-w-6xl max-h-[85vh] rounded-3xl overflow-hidden border border-white/20 shadow-2xl bg-black"
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
      <div class="absolute bottom-6 px-3.5 py-1 rounded-full bg-white/10 text-xs font-mono text-[#cbd5e1] border border-white/15">
        {lightboxIndex + 1} / {gameScreenshots.length}
      </div>
    </div>
  {/if}

  <!-- 7. Steam Candidate Matching Modal -->
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

          {#if activeGame?.steamAppId}
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

</div>