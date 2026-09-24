<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    GameController as Gamepad2,
    HardDrive,
    MagnifyingGlass as Search,
    X,
    Folder,
    DownloadSimple as Download,
    Star,
    Calendar,
    User,
    SquaresFour as Layers,
    Buildings as Building2,
    Tag,
    Tag as Tags,
    Monitor,
    Play,
    HardDrives as Server,
    Disc,
    Cpu,
    CheckCircle as CheckCircle2,
    FolderOpen,
    FileText,
    ShieldCheck,
    PencilSimple as Edit3,
    ArrowsClockwise as RefreshCw,
    Link as Link2,
    LinkBreak as Unlink,
    ArrowSquareOut as ExternalLink,
    Check,
    CaretLeft as ChevronLeft,
    CaretRight as ChevronRight,
    FilmStrip as Film,
    Image as ImageIcon,
    ArrowsOut as Maximize2,
    ArrowsIn as Minimize2,
    CaretDown as ChevronDown,
    BookmarkSimple as Bookmark,
    BookmarkSimple as BookmarkCheck,
    Clock,
    Trash as Trash2,
    Copy,
    Gear as Settings,
    SteamLogo,
    UploadSimple,
    ThumbsUp,
    ThumbsDown,
    ChatText
  } from 'phosphor-svelte';
  import VideoPlayer from './VideoPlayer.svelte';
  import type {
    GameEntity,
    GameVariant,
    SteamMovie,
    MediaItem,
    SteamCandidateItem,
    GamePageDetails,
    SteamAnonymizedReview,
    SteamReviewsResponse
  } from '../types/game';
  import { EventsOn, BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
  import { SetFavoriteStatus, RemoveFromFavorites, SetFavoriteLaunchConfig, SelectGameExeFile, LaunchGameWithCustomConfig, AddGameToSteam, CheckGameInSteam, RemoveGameFromSteam, GetTorrentSeedsBatch, GetSteamReviews, OpenURL } from '../../../wailsjs/go/main/App';
  import { isPlaceholderTitle, cleanTorrentTitle, getDisplayTitle } from '../utils/titleUtils';
  import { downloadsStore } from '../stores/downloads.svelte';
  import { formatSteamReviewBBCode, formatReviewDate } from '../utils/steamReviewFormatter';


  let {
    game = $bindable(null as GameEntity | null),
    downloadPath = '',
    isLoading = false,
    loadingStatusText = '',
    onStartDownload = (gameId: number, targetPath: string) => {},
    onSelectFolder = async (): Promise<string> => '',
    onSelectGenre = (genre: string) => {},
    onSelectTag = (tag: string) => {},
    onUpdateDownloadPath = (path: string) => {},
    favoriteItem = null as any
  } = $props();

  // Launch config modal state
  let isLaunchConfigOpen = $state<boolean>(false);
  let lcExePath = $state<string>('');
  let lcLaunchArgs = $state<string>('');
  let lcSaving = $state<boolean>(false);
  let lcSaveSuccess = $state<boolean>(false);
  let lcCandidates = $state<string[]>([]);
  let isScanningCandidates = $state<boolean>(false);

  // Synced local custom launch config (reacts even if favoriteItem is not passed as prop)
  let currentCustomExePath = $state<string>('');
  let currentLaunchArgs = $state<string>('');

  // Sync modal fields when favoriteItem or game changes
  $effect(() => {
    if (favoriteItem) {
      lcExePath = favoriteItem.customExePath || '';
      lcLaunchArgs = favoriteItem.launchArguments || '';
      currentCustomExePath = favoriteItem.customExePath || '';
      currentLaunchArgs = favoriteItem.launchArguments || '';
    } else if (game?.id) {
      const targetGame = pageDetails?.game || game;
      if (targetGame?.favoriteStatus) {
        const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
        if (app && app.GetFavoriteLaunchConfig) {
          app.GetFavoriteLaunchConfig(targetGame.id).then((cfg: any) => {
            if (cfg && cfg.exePath) {
              lcExePath = cfg.exePath || '';
              lcLaunchArgs = cfg.launchArgs || '';
              currentCustomExePath = cfg.exePath || '';
              currentLaunchArgs = cfg.launchArgs || '';
            } else {
              currentCustomExePath = '';
              currentLaunchArgs = '';
            }
          }).catch(() => {});
        }
      } else {
        lcExePath = '';
        lcLaunchArgs = '';
        currentCustomExePath = '';
        currentLaunchArgs = '';
      }
    }
  });

  let hasCustomExe = $derived(!!(currentCustomExePath?.trim() || favoriteItem?.customExePath?.trim()));

  async function loadExeCandidates() {
    const curId = game?.id;
    if (!curId) return;
    isScanningCandidates = true;
    try {
      const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
      if (app && app.FindGameExecutables) {
        const found = await app.FindGameExecutables(curId);
        if (Array.isArray(found)) {
          lcCandidates = found;
          if (!lcExePath && found.length === 1) {
            lcExePath = found[0];
          }
        }
      }
    } catch (e) {
      console.error('[GameDetailView] Failed to find executables:', e);
    } finally {
      isScanningCandidates = false;
    }
  }

  function openLaunchConfigModal() {
    isLaunchConfigOpen = true;
    loadExeCandidates();
  }

  async function handleBrowseExe() {
    const defaultDir = pageDetails?.localPath || '';
    const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
    let path = '';
    if (app && app.SelectGameExeFile) {
      path = await app.SelectGameExeFile(defaultDir);
    } else {
      path = await SelectGameExeFile();
    }
    if (path) lcExePath = path;
  }

  async function handleSaveLaunchConfig(andLaunch = false) {
    if (!game) return;
    lcSaving = true;
    lcSaveSuccess = false;
    try {
      await SetFavoriteLaunchConfig(game.id, lcExePath.trim(), lcLaunchArgs.trim());
      currentCustomExePath = lcExePath.trim();
      currentLaunchArgs = lcLaunchArgs.trim();
      if (favoriteItem) {
        favoriteItem.customExePath = lcExePath.trim();
        favoriteItem.launchArguments = lcLaunchArgs.trim();
      }
      lcSaveSuccess = true;
      if (andLaunch) {
        isLaunchConfigOpen = false;
        await handleLaunchWithCustom();
      } else {
        setTimeout(() => {
          lcSaveSuccess = false;
          if (lcExePath.trim()) isLaunchConfigOpen = false;
        }, 800);
      }
    } catch (e) {
      console.error('[GameDetailView] Failed to save launch config:', e);
    } finally {
      lcSaving = false;
    }
  }

  let launchErrorFeedback = $state<string | null>(null);
  let launchErrorTimeout: any = null;

  function showLaunchError(msg: string) {
    launchErrorFeedback = msg;
    if (launchErrorTimeout) clearTimeout(launchErrorTimeout);
    launchErrorTimeout = setTimeout(() => {
      launchErrorFeedback = null;
    }, 4500);
  }

  async function handleLaunchWithCustom() {
    if (!game) return;
    try {
      await LaunchGameWithCustomConfig(game.id);
    } catch (err: any) {
      console.error('[GameDetailView] Failed to launch with custom config:', err);
      const errMsg = err?.message || String(err);
      showLaunchError(`Ошибка запуска: ${errMsg}`);
    }
  }

  async function handlePlayButtonClick() {
    if (isGameInFavorites) {
      if (!hasCustomExe) {
        openLaunchConfigModal();
        return;
      }
      await handleLaunchWithCustom();
      return;
    }

    if (hasCustomExe) {
      await handleLaunchWithCustom();
      return;
    }

    await handleLaunchGame();
  }

  // Steam integration state
  let isGameInSteam = $state<boolean>(false);
  let isAddingToSteam = $state<boolean>(false);

  async function checkSteamStatus() {
    const curGame = pageDetails?.game || game;
    if (!curGame?.id) {
      isGameInSteam = false;
      return;
    }
    try {
      isGameInSteam = await CheckGameInSteam(curGame.id);
    } catch {
      isGameInSteam = false;
    }
  }

  $effect(() => {
    const _id = game?.id;
    const _exe = currentCustomExePath;
    checkSteamStatus();
  });

  async function handleAddToSteam() {
    const curGame = pageDetails?.game || game;
    if (!curGame?.id) return;

    if (!hasCustomExe && !pageDetails?.localPath && !pageDetails?.isInstalled) {
      openLaunchConfigModal();
      showLaunchError('Сначала укажите исполняемый файл (.exe) игры в настройках');
      return;
    }

    isAddingToSteam = true;
    try {
      const res = await AddGameToSteam(curGame.id);
      isGameInSteam = true;
      if (res && res.message) {
        copiedTextFeedback = res.message;
        if (copiedTimeout) clearTimeout(copiedTimeout);
        copiedTimeout = setTimeout(() => {
          copiedTextFeedback = null;
        }, 5500);
      }
    } catch (err: any) {
      console.error('[GameDetailView] Failed to add game to Steam:', err);
      const errMsg = err?.message || String(err);
      if (errMsg.includes('.exe')) {
        openLaunchConfigModal();
      }
      showLaunchError(errMsg);
    } finally {
      isAddingToSteam = false;
    }
  }

  async function handleRemoveFromSteam() {
    const curGame = pageDetails?.game || game;
    if (!curGame?.id) return;
    try {
      await RemoveGameFromSteam(curGame.id);
      isGameInSteam = false;
      copiedTextFeedback = 'Ярлык удален из библиотеки Steam';
      if (copiedTimeout) clearTimeout(copiedTimeout);
      copiedTimeout = setTimeout(() => {
        copiedTextFeedback = null;
      }, 4000);
    } catch (err: any) {
      console.error('[GameDetailView] Failed to remove game from Steam:', err);
      showLaunchError(`Ошибка: ${err?.message || String(err)}`);
    }
  }



  const STEAM_DEFAULT_ACCENT = '#66c0f4';

  let customDownloadPath = $state<string>('');
  let pageDetails = $state<GamePageDetails | null>(null);
  let isLoadingDetails = $state<boolean>(false);
  let activeTab = $state<'description' | 'info' | 'requirements' | 'specs' | 'reviews'>('description');
  let activeMediaIndex = $state<number>(0);

  // Steam Reviews state
  let steamReviews = $state<SteamAnonymizedReview[]>([]);
  let steamReviewsCursor = $state<string>('*');
  let steamReviewsHasMore = $state<boolean>(false);
  let steamReviewsTotal = $state<number>(0);
  let isLoadingReviews = $state<boolean>(false);
  let isLoadingMoreReviews = $state<boolean>(false);
  let reviewLanguage = $state<'russian' | 'all'>('russian');
  let expandedReviewIds = $state<Record<string, boolean>>({});
  let lastLoadedReviewsAppId = $state<number>(0);

  function toggleReviewExpand(id: string) {
    expandedReviewIds[id] = !expandedReviewIds[id];
  }

  function handleOpenSteamStore() {
    const curGame = pageDetails?.game || game;
    const appId = curGame?.steamAppId;
    if (!appId || appId <= 0) return;
    const storeUrl = `https://store.steampowered.com/app/${appId}`;
    try {
      BrowserOpenURL(storeUrl);
    } catch {
      OpenURL(storeUrl);
    }
  }

  async function loadSteamReviews(reset = false) {
    const curGame = pageDetails?.game || game;
    const appId = curGame?.steamAppId;
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
      const res: SteamReviewsResponse = await GetSteamReviews(appId, reset ? '*' : steamReviewsCursor, reviewLanguage);
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
      console.error('Failed to load Steam reviews:', e);
    } finally {
      isLoadingReviews = false;
      isLoadingMoreReviews = false;
    }
  }

  function handleLanguageChange(lang: 'russian' | 'all') {
    if (reviewLanguage === lang) return;
    reviewLanguage = lang;
    loadSteamReviews(true);
  }

  $effect(() => {
    const curGame = pageDetails?.game || game;
    const currentAppId = curGame?.steamAppId || 0;
    if (currentAppId > 0 && currentAppId !== lastLoadedReviewsAppId) {
      lastLoadedReviewsAppId = currentAppId;
      loadSteamReviews(true);
    } else if (currentAppId <= 0 && lastLoadedReviewsAppId !== 0) {
      lastLoadedReviewsAppId = 0;
      steamReviews = [];
      steamReviewsHasMore = false;
      steamReviewsTotal = 0;
    }
  });

  let imageLoadFailed = $state<Record<string, boolean>>({});
  let dynamicAccentColor = $state<string>(STEAM_DEFAULT_ACCENT);
  let coverColorCache = new Map<string, string>();
  let enrichingGameIds = new Set<number>();
  let isEnrichingCurrentGame = $state<boolean>(false);
  let movieOverrides = $state<Record<number, SteamMovie[]>>({});
  let screenshotViewport = $state<HTMLDivElement | null>(null);
  let isScreenshotFullscreen = $state<boolean>(false);
  let coverAspectRatio = $state<string | null>(null);
  let selectedVariantId = $state<number | null>(null);
  let isVariantDropdownOpen = $state<boolean>(false);
  let variantDropdownTriggerEl = $state<HTMLButtonElement | null>(null);
  let variantDropdownContainerEl = $state<HTMLDivElement | null>(null);

  // Favorites backlog state
  let isFavoriteDropdownOpen = $state<boolean>(false);
  let favoriteDropdownContainerEl = $state<HTMLDivElement | null>(null);
  let favoriteDropdownTriggerEl = $state<HTMLButtonElement | null>(null);

  let torrentSourcesMap = $state<Record<string, string>>({});
  let copiedTextFeedback = $state<string | null>(null);
  let copiedTimeout: any = null;

  async function loadTorrentSourcesMap() {
    try {
      const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
      if (app && app.GetTorrentSources) {
        const sources = await app.GetTorrentSources();
        if (Array.isArray(sources)) {
          const m: Record<string, string> = {};
          for (const s of sources) {
            if (s && s.id && s.name) {
              m[s.id] = s.name;
            }
          }
          torrentSourcesMap = m;
        }
      }
    } catch {}
  }

  function cleanSourceDisplayName(name: string): string {
    if (!name) return '';
    const firstPart = name.split('|')[0].trim();
    return firstPart || name;
  }

  function formatSourceName(rawSource: string | undefined): string {
    if (!rawSource) return '';
    const s = rawSource.trim();
    if (torrentSourcesMap[s]) {
      return cleanSourceDisplayName(torrentSourcesMap[s]);
    }
    if (s.startsWith('tsrc_')) {
      return 'Каталог торрентов';
    }
    return cleanSourceDisplayName(s);
  }

  function extractBtih(uriOrPath: string | undefined): string {
    if (!uriOrPath) return '';
    const m = uriOrPath.match(/urn:btih:([a-zA-Z0-9]{32,40})/i);
    if (m && m[1]) return m[1].toUpperCase();
    return '';
  }

  async function copyText(text: string, label: string) {
    if (!text) return;
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
      } else {
        const ta = document.createElement('textarea');
        ta.value = text;
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
      }
      copiedTextFeedback = label;
      clearTimeout(copiedTimeout);
      copiedTimeout = setTimeout(() => {
        copiedTextFeedback = null;
      }, 2000);
    } catch (err) {
      console.error('Failed to copy text:', err);
    }
  }

  let localFavoriteStatus = $state<string | null>(null);

  $effect(() => {
    const _ = game?.id;
    localFavoriteStatus = null;
  });

  let effectiveFavoriteStatus = $derived(
    localFavoriteStatus !== null
      ? localFavoriteStatus
      : (pageDetails?.game?.favoriteStatus || game?.favoriteStatus || '')
  );

  async function handleToggleFavoriteStatus(status: string) {
    const targetGame = pageDetails?.game || game;
    if (!targetGame?.id) return;
    try {
      isFavoriteDropdownOpen = false;
      await SetFavoriteStatus(targetGame.id, status);
      localFavoriteStatus = status;
      if (pageDetails?.game) {
        pageDetails.game.favoriteStatus = status;
      }
    } catch (e) {
      console.error('Failed to set favorite status:', e);
    }
  }

  async function handleRemoveFavorite() {
    const targetGame = pageDetails?.game || game;
    if (!targetGame?.id) return;
    try {
      isFavoriteDropdownOpen = false;
      await RemoveFromFavorites(targetGame.id);
      localFavoriteStatus = '';
      if (pageDetails?.game) {
        pageDetails.game.favoriteStatus = '';
      }
    } catch (e) {
      console.error('Failed to remove from favorites:', e);
    }
  }

  let activeVariant = $derived.by(() => {
    const vg = pageDetails?.game || game;
    if (!vg?.variants || vg.variants.length === 0) return null;
    return vg.variants.find((v) => v.id === selectedVariantId) || vg.variants[0];
  });

  let downloadSourceInfo = $derived.by(() => {
    const vg = pageDetails?.game || game;
    const v = activeVariant || vg;
    if (!v) return null;
    const isTorrent = v.sourceType === 'torrent' || !!v.magnetUri;
    const rawSrc = (v.torrentSource || '').trim();
    const resolvedName = formatSourceName(rawSrc);
    if (isTorrent) {
      let displayName = 'Торрент';
      if (resolvedName) {
        if (resolvedName.toLowerCase().startsWith('торрент')) {
          displayName = resolvedName;
        } else if (resolvedName.includes('(') && resolvedName.endsWith(')')) {
          displayName = `Торрент: ${resolvedName}`;
        } else {
          displayName = `Торрент (${resolvedName})`;
        }
      }
      return {
        type: 'torrent',
        name: displayName,
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
    const mod10 = seeds % 10;
    const mod100 = seeds % 100;
    if (mod100 >= 11 && mod100 <= 19) {
      return `${seeds} сидов`;
    }
    if (mod10 === 1) {
      return `${seeds} сид`;
    }
    if (mod10 >= 2 && mod10 <= 4) {
      return `${seeds} сида`;
    }
    return `${seeds} сидов`;
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
      const results = await GetTorrentSeedsBatch(queries as any);
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
    const vg = pageDetails?.game || game;
    if (vg?.id && vg.id !== lastSeedsFetchedGameId) {
      loadTorrentSeedsForGame(vg);
    }
  });

  let currentVariantSeedInfo = $derived.by(() => {
    const vg = pageDetails?.game || game;
    const v = activeVariant || vg;
    if (!v) return null;
    const isTorrent = v.sourceType === 'torrent' || !!v.magnetUri;
    if (!isTorrent) return null;
    return variantSeeds[v.id] || null;
  });

  $effect(() => {
    const vg = pageDetails?.game || game;
    if (vg?.variants && vg.variants.length > 0) {
      if (!selectedVariantId || !vg.variants.some((v) => v.id === selectedVariantId)) {
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

  // Steam metadata search modal state
  let isSteamModalOpen = $state<boolean>(false);
  let steamSearchTerm = $state<string>('');
  let steamCandidates = $state<SteamCandidateItem[]>([]);
  let isSearchingSteam = $state<boolean>(false);
  let directAppIdInput = $state<string>('');
  let isSavingSteam = $state<boolean>(false);

  $effect(() => {
    if (downloadPath) {
      customDownloadPath = downloadPath;
    }
  });

  // Calculate contrast text color for accent
  function getContrastTextColor(colorStr: string): string {
    let r = 102, g = 192, b = 244;
    if (!colorStr) return '#000000';
    if (colorStr.startsWith('#')) {
      const hex = colorStr.replace('#', '');
      if (hex.length === 3) {
        r = parseInt(hex[0] + hex[0], 16);
        g = parseInt(hex[1] + hex[1], 16);
        b = parseInt(hex[2] + hex[2], 16);
      } else if (hex.length >= 6) {
        r = parseInt(hex.slice(0, 2), 16);
        g = parseInt(hex.slice(2, 4), 16);
        b = parseInt(hex.slice(4, 6), 16);
      }
    } else if (colorStr.startsWith('rgb')) {
      const matches = colorStr.match(/\d+/g);
      if (matches && matches.length >= 3) {
        r = parseInt(matches[0], 10);
        g = parseInt(matches[1], 10);
        b = parseInt(matches[2], 10);
      }
    }
    const yiq = (r * 299 + g * 587 + b * 114) / 1000;
    return yiq >= 135 ? '#000000' : '#ffffff';
  }

  let dynamicAccentTextColor = $derived.by(() => getContrastTextColor(dynamicAccentColor));

  // Determine if cover is landscape
  let isCoverLandscape = $derived.by(() => {
    if (coverAspectRatio) {
      const parts = coverAspectRatio.split('/');
      if (parts.length === 2) {
        const w = parseFloat(parts[0]);
        const h = parseFloat(parts[1]);
        if (w && h && (w / h) > 1.15) return true;
      }
    }
    return false;
  });

  // --------------------------------------------------------------------------
  // Game Switch & Data Loading Architecture
  // --------------------------------------------------------------------------
  let lastGameId: number | null = null;
  let isSwitching = $state<boolean>(false);
  let switchSequence = 0;
  let switchTimeout: any = null;
  let scrollContainer = $state<HTMLDivElement | null>(null);
  let isMounted = true;

  // In-memory cache for fast back-and-forth switching between recently viewed games
  const pageDetailsCache = new Map<number, GamePageDetails>();

  onDestroy(() => {
    isMounted = false;
    clearTimeout(switchTimeout);
  });

  function createInitialPageDetails(g: GameEntity): GamePageDetails {
    return {
      game: g,
      downloadStatus: 'none',
      downloadProgress: null,
      localPath: '',
      isInstalled: false,
      logoUrl: '',
      bannerUrl: g.backgroundImage || g.headerImage || '',
      coverUrl: g.capsuleImage || '',
      backgroundUrl: g.backgroundImage || '',
      media: []
    };
  }

  // Orchestrated game data switch whenever the selected game changes
  $effect(() => {
    const curId = game?.id;
    if (!curId || !game) {
      pageDetails = null;
      isSwitching = false;
      lastGameId = null;
      return;
    }

    if (curId !== lastGameId) {
      const prevId = lastGameId;
      lastGameId = curId;
      lastSeedsFetchedGameId = 0;
      const seq = ++switchSequence;

      // 1. Reset all interactive view state for the new game immediately
      activeMediaIndex = 0;
      activeTab = 'description';
      coverAspectRatio = null;

      // Retain previously selected variant if present in the new variants list (e.g. after release merge)
      if (game.variants && game.variants.length > 0) {
        if (selectedVariantId && game.variants.some((v) => v.id === selectedVariantId)) {
          // Keep current selectedVariantId
        } else if (prevId && game.variants.some((v) => v.id === prevId)) {
          selectedVariantId = prevId;
        } else {
          selectedVariantId = game.variants[0].id;
        }
      } else {
        selectedVariantId = curId;
      }
      isVariantDropdownOpen = false;
      isFavoriteDropdownOpen = false;

      // 2. Reset scroll position to top
      if (scrollContainer) {
        scrollContainer.scrollTop = 0;
      }

      // 3. Populate immediate baseline data (from cache or game entity) to avoid showing stale data
      if (pageDetailsCache.has(curId)) {
        pageDetails = pageDetailsCache.get(curId)!;
        isSwitching = false;
        isLoadingDetails = false;
      } else {
        pageDetails = createInitialPageDetails(game);
        // Show lightweight transition skeleton only if the game has no rich metadata yet
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

      // 4. Launch coordinated background loaders for this game
      fetchGamePageDetails(curId, seq);
      resolveAccentColor(game, seq);
      resolveMoviesIfNeeded(game, seq);
      triggerPriorityEnrichmentIfNeeded(game, curId, seq);
    }
  });

  async function fetchGamePageDetails(gameId: number, seq: number) {
    if (!isMounted) return;
    isLoadingDetails = true;

    try {
      const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
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

      // Verify sequence token and mount state before applying
      if (!isMounted || seq !== switchSequence) return;

      if (res) {
        pageDetailsCache.set(gameId, res);
        pageDetails = res;
        if (res.game) {
          loadTorrentSeedsForGame(res.game);
        }
      }
    } catch (err) {
      console.error('[GameDetailView] Failed to get game page details:', err);
    } finally {
      if (isMounted && seq === switchSequence) {
        isLoadingDetails = false;
        isSwitching = false;
      }
    }
  }

  function resolveAccentColor(g: GameEntity, seq: number) {
    const coverUrl = getPrimaryCoverUrl(g);
    if (!coverUrl) {
      dynamicAccentColor = STEAM_DEFAULT_ACCENT;
      return;
    }

    if (coverColorCache.has(coverUrl)) {
      dynamicAccentColor = coverColorCache.get(coverUrl)!;
      return;
    }

    const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
    if (app && app.ExtractDominantColor) {
      app.ExtractDominantColor(coverUrl).then((color: string) => {
        if (isMounted && seq === switchSequence && color) {
          dynamicAccentColor = color;
          coverColorCache.set(coverUrl, color);
        }
      }).catch(() => {});
    }
  }

  function resolveMoviesIfNeeded(g: GameEntity, seq: number) {
    if (!g.steamAppId || g.steamAppId <= 0) return;

    const currentMovies = movieOverrides[g.id] || g.movies || [];
    const hasValidHls = currentMovies.some((m) => m.hls && m.hls.trim() !== '');
    if (hasValidHls) return;

    const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
    if (app && app.GetGameMovies) {
      app.GetGameMovies(g.steamAppId).then((freshMovies: SteamMovie[]) => {
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

    const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
    if (app && app.EnrichGameNow) {
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



  function formatSizeDisplay(g: GameEntity | null): string {
    if (!g) return '';
    if (g.sizeDisplay && g.sizeDisplay.trim() !== '' && g.sizeDisplay.toLowerCase() !== 'unknown') {
      return g.sizeDisplay;
    }
    if (g.sizeBytes && g.sizeBytes > 0) {
      const gb = g.sizeBytes / (1024 * 1024 * 1024);
      if (gb >= 1) return `${gb.toFixed(1)} GB`;
      const mb = g.sizeBytes / (1024 * 1024);
      return `${mb.toFixed(0)} MB`;
    }
    return '';
  }

  function isHorizontalAsset(url: string): boolean {
    if (!url) return false;
    return /capsule_231x87|capsule_616x353|capsule_467x181|header\.jpg|header_alt/i.test(url);
  }

  function getPrimaryCoverUrl(g: GameEntity | null): string {
    if (!g) return '';
    if (pageDetails?.coverUrl && !imageLoadFailed[pageDetails.coverUrl]) {
      return pageDetails.coverUrl;
    }
    if (g.capsuleImage && !isHorizontalAsset(g.capsuleImage) && !imageLoadFailed[g.capsuleImage]) {
      return g.capsuleImage;
    }
    if (g.steamAppId && g.steamAppId > 0) {
      const libUrl = `https://shared.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/library_600x900.jpg`;
      if (!imageLoadFailed[libUrl]) return libUrl;
      const akamaiUrl = `https://steamcdn-a.akamaihd.net/steam/apps/${g.steamAppId}/library_600x900.jpg`;
      if (!imageLoadFailed[akamaiUrl]) return akamaiUrl;
    }
    if (g.capsuleImage && !imageLoadFailed[g.capsuleImage]) return g.capsuleImage;
    if (g.headerImage && !imageLoadFailed[g.headerImage]) return g.headerImage;
    return '';
  }

  function getHeroBackgroundUrl(g: GameEntity | null): string {
    if (!g) return '';
    if (pageDetails?.backgroundUrl && !imageLoadFailed[pageDetails.backgroundUrl]) {
      return pageDetails.backgroundUrl;
    }
    if (g.backgroundImage && !imageLoadFailed[g.backgroundImage]) {
      return g.backgroundImage;
    }
    if (g.steamAppId && g.steamAppId > 0) {
      const v6b = `https://shared.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/page_bg_generated_v6b.jpg`;
      if (!imageLoadFailed[v6b]) return v6b;
      const pageBg = `https://shared.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/page.bg.jpg`;
      if (!imageLoadFailed[pageBg]) return pageBg;
    }
    if (g.screenshots && g.screenshots.length > 0 && !imageLoadFailed[g.screenshots[0]]) {
      return g.screenshots[0];
    }
    return '';
  }

  // Unified Media List
  let mediaList = $derived.by<MediaItem[]>(() => {
    if (!game) return [];
    const items: MediaItem[] = [];

    const effectiveMovies = (game.id && movieOverrides[game.id]) || game.movies;
    if (effectiveMovies && effectiveMovies.length > 0) {
      for (const m of effectiveMovies) {
        let hls = m.hls || '';
        let mp4 = m.mp4 || '';
        let webm = m.webm || '';
        let thumb = m.thumbnail || '';

        if (hls) {
          hls = hls.replace(/^http:\/\//i, 'https://')
                   .replace(/video\.fastly\.steamstatic\.com/gi, 'video.akamai.steamstatic.com');
        }
        if (mp4) {
          mp4 = mp4.replace(/^http:\/\//i, 'https://')
                   .replace(/video\.fastly\.steamstatic\.com/gi, 'video.akamai.steamstatic.com');
          if (mp4.includes('/apps/') && mp4.includes('movie_max.mp4')) mp4 = '';
        }
        if (webm) {
          webm = webm.replace(/^http:\/\//i, 'https://')
                     .replace(/video\.fastly\.steamstatic\.com/gi, 'video.akamai.steamstatic.com');
        }
        if (thumb) {
          thumb = thumb.trim().replace(/^http:\/\//i, 'https://');
          if (thumb.startsWith('//')) {
            thumb = 'https:' + thumb;
          } else if (!thumb.startsWith('https://')) {
            thumb = `https://shared.steamstatic.com/store_item_assets/steam/apps/${game.steamAppId || ''}/${thumb.replace(/^\//, '')}`;
          }
          thumb = thumb.replace(/shared\.fastly\.steamstatic\.com/gi, 'shared.steamstatic.com')
                       .replace(/shared\.akamai\.steamstatic\.com/gi, 'shared.steamstatic.com');
        }

        if (hls || mp4 || webm || thumb) {
          items.push({
            type: 'video',
            id: m.id || items.length,
            name: m.name || 'Трейлер',
            thumbnail: thumb,
            mp4,
            webm,
            hls
          });
        }
      }
    }

    const currentScreenshots = (pageDetails?.game?.screenshots && pageDetails.game.screenshots.length > 0)
      ? pageDetails.game.screenshots
      : (game.screenshots || []);

    if (currentScreenshots && currentScreenshots.length > 0) {
      for (let i = 0; i < currentScreenshots.length; i++) {
        const rawUrl = typeof currentScreenshots[i] === 'string' ? currentScreenshots[i] : (currentScreenshots[i] as any)?.url;
        if (rawUrl) {
          items.push({
            type: 'image',
            id: i,
            name: `Скриншот ${i + 1}`,
            url: rawUrl.replace(/^http:\/\//i, 'https://')
          });
        }
      }
    }

    return items;
  });

  let activeMedia = $derived.by<MediaItem | null>(() => {
    if (!mediaList || mediaList.length === 0) return null;
    const idx = Math.min(Math.max(0, activeMediaIndex), mediaList.length - 1);
    return mediaList[idx];
  });

  function nextMedia() {
    if (mediaList.length === 0) return;
    activeMediaIndex = (activeMediaIndex + 1) % mediaList.length;
  }

  function prevMedia() {
    if (mediaList.length === 0) return;
    activeMediaIndex = (activeMediaIndex - 1 + mediaList.length) % mediaList.length;
  }

  function toggleScreenshotFullscreen() {
    if (!screenshotViewport) return;
    if (!document.fullscreenElement) {
      screenshotViewport.requestFullscreen?.().catch(console.error);
    } else {
      document.exitFullscreen?.().catch(console.error);
    }
  }

  function onCoverLoaded(img: HTMLImageElement) {
    if (img && img.naturalWidth > 0 && img.naturalHeight > 0) {
      coverAspectRatio = `${img.naturalWidth} / ${img.naturalHeight}`;
    }
    extractColorFromImgElement(img);
  }

  async function extractColorFromImgElement(img: HTMLImageElement) {
    if (!img) return;
    const src = img.src;
    if (!src) return;

    if (coverColorCache.has(src)) {
      dynamicAccentColor = coverColorCache.get(src)!;
      return;
    }

    try {
      const canvas = document.createElement('canvas');
      const ctx = canvas.getContext('2d');
      if (!ctx) throw new Error('no canvas ctx');
      canvas.width = 48;
      canvas.height = 48;
      ctx.drawImage(img, 0, 0, 48, 48);
      const data = ctx.getImageData(0, 0, 48, 48).data;

      let bestColor = STEAM_DEFAULT_ACCENT;
      let maxScore = -1;
      let rSum = 0, gSum = 0, bSum = 0, count = 0;

      for (let i = 0; i < data.length; i += 16) {
        const r = data[i];
        const g = data[i + 1];
        const b = data[i + 2];
        const a = data[i + 3];

        if (a < 180) continue;
        const brightness = (r * 299 + g * 587 + b * 114) / 1000;
        if (brightness < 35 || brightness > 235) continue;

        const max = Math.max(r, g, b);
        const min = Math.min(r, g, b);
        const delta = max - min;
        const saturation = max === 0 ? 0 : delta / max;

        const score = saturation * 2.5 + (1 - Math.abs(brightness - 140) / 140);
        if (score > maxScore && saturation > 0.22) {
          maxScore = score;
          bestColor = `rgb(${r}, ${g}, ${b})`;
        }
        rSum += r;
        gSum += g;
        bSum += b;
        count++;
      }

      let chosen = STEAM_DEFAULT_ACCENT;
      if (maxScore > 0.5) {
        chosen = bestColor;
      } else if (count > 0) {
        chosen = `rgb(${Math.round(rSum / count)}, ${Math.round(gSum / count)}, ${Math.round(bSum / count)})`;
      }

      dynamicAccentColor = chosen;
      coverColorCache.set(src, chosen);
    } catch {
      try {
        const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
        if (app && app.ExtractDominantColor) {
          const color = await app.ExtractDominantColor(src);
          if (color) {
            dynamicAccentColor = color;
            coverColorCache.set(src, color);
          }
        }
      } catch {
        dynamicAccentColor = STEAM_DEFAULT_ACCENT;
      }
    }
  }

  function formatReviewsCount(count: number): string {
    if (!count || count <= 0) return 'Отзывы Steam';
    const n = Math.abs(count) % 100;
    const n1 = n % 10;
    let word = 'обзоров';
    if (n > 10 && n < 20) {
      word = 'обзоров';
    } else if (n1 > 1 && n1 < 5) {
      word = 'обзора';
    } else if (n1 === 1) {
      word = 'обзор';
    }
    return `${count.toLocaleString('ru-RU')} ${word} в Steam`;
  }

  async function handleBrowseFolder() {
    let selected = await onSelectFolder();
    if (selected) {
      if (/^[a-zA-Z]:\\?$/.test(selected)) {
        selected = `${selected[0].toUpperCase()}:\\Ducke`;
      }
      customDownloadPath = selected;
      onUpdateDownloadPath(selected);
    }
  }

  async function handleLaunchGame() {
    if (!game) return;
    try {
      const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
      if (app && app.LaunchGameByGameID) {
        await app.LaunchGameByGameID(game.id);
      } else if (app && app.OpenGameFolder) {
        await app.OpenGameFolder(game.id);
      }
    } catch (err: any) {
      console.error('[GameDetailView] Failed to launch game:', err);
      const errMsg = err?.message || String(err);
      showLaunchError(`Ошибка запуска: ${errMsg}`);
    }
  }

  async function handleOpenFolder() {
    if (!game) return;
    try {
      const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
      if (app && app.OpenGameFolder) {
        await app.OpenGameFolder(game.id);
      }
    } catch (err) {
      console.error('[GameDetailView] Failed to open game folder:', err);
    }
  }

  // Steam candidate search modal helpers
  function closeSteamModal() {
    if (isSteamModalOpen) {
      isSteamModalOpen = false;
      window.dispatchEvent(new CustomEvent('app:modal-closed'));
    }
  }

  async function openSteamModal(g: GameEntity) {
    const raw = g.steamTitle || (g as any).searchTitle || (g as any).cleanTitle || getDisplayTitle(g) || '';
    const cleaned = raw
      .replace(/\[.*?\]|\(.*?\)|[\{\}]/g, ' ')
      .replace(/\b(v\s*\d+([._\s]\d+)*|build\s*\d+|patch\s*\d+|update\s*\d+)\b/gi, ' ')
      .replace(/\b(19[7-9]\d|20[0-3]\d)\b/g, ' ')
      .replace(/\s+/g, ' ')
      .trim();
    steamSearchTerm = cleaned || raw;
    directAppIdInput = (g.steamAppId && g.steamAppId > 0) ? String(g.steamAppId) : '';
    steamCandidates = [];
    isSteamModalOpen = true;
    window.dispatchEvent(new CustomEvent('app:modal-opened'));
    await performSteamSearch(true);
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
      const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
      if (app && app.SearchSteamCandidates) {
        const results = await app.SearchSteamCandidates(cleanQuery);
        steamCandidates = (results as SteamCandidateItem[]) || [];

        // Automatically link if exact 100% (or score >= 0.95) match found on initial auto-search
        if (autoLinkExact && steamCandidates.length > 0 && steamCandidates[0].score >= 0.95 && (!game?.steamAppId || game.steamAppId <= 0)) {
          await handleLinkAppId(steamCandidates[0].appId);
          return;
        }
      } else {
        steamCandidates = [];
      }
    } catch (err) {
      console.error('Failed to search Steam:', err);
      steamCandidates = [];
    } finally {
      isSearchingSteam = false;
    }
  }

  async function handleLinkAppId(appId: number) {
    if (!game) return;
    isSavingSteam = true;
    try {
      const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
      if (app && app.UpdateSteamAppID) {
        await app.UpdateSteamAppID(game.id, appId);
      }
      closeSteamModal();
      switchSequence++;
      fetchGamePageDetails(game.id, switchSequence);
    } catch (err) {
      console.error('Failed to link steam AppID:', err);
    } finally {
      isSavingSteam = false;
    }
  }

  async function handleApplyDirectAppId() {
    if (!game) return;
    const cleanId = directAppIdInput.trim().replace(/\D/g, '');
    const num = parseInt(cleanId, 10);
    if (!isNaN(num) && num > 0) {
      await handleLinkAppId(num);
    }
  }

  async function handleUnlinkMetadata() {
    if (!game) return;
    isSavingSteam = true;
    try {
      const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
      if (app && app.ResetGameMetadata) {
        await app.ResetGameMetadata(game.id);
      } else if (app && app.UpdateSteamAppID) {
        await app.UpdateSteamAppID(game.id, 0);
      }
      isSteamModalOpen = false;
      switchSequence++;
      fetchGamePageDetails(game.id, switchSequence);
    } catch (err) {
      console.error('Failed to unlink metadata:', err);
    } finally {
      isSavingSteam = false;
    }
  }

  function handleWindowKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape' && isVariantDropdownOpen) {
      e.preventDefault();
      e.stopPropagation();
      isVariantDropdownOpen = false;
      return;
    }
    if (e.target && (e.target as HTMLElement).tagName === 'INPUT') return;
    if (e.key === 'ArrowRight' || e.key === 'KeyD') {
      nextMedia();
    } else if (e.key === 'ArrowLeft' || e.key === 'KeyA') {
      prevMedia();
    }
  }

  let availableSubTabs = $derived.by<('description' | 'info' | 'requirements' | 'specs')[]>(() => {
    const g = pageDetails?.game || game;
    const tabs: ('description' | 'info' | 'requirements' | 'specs')[] = ['description', 'info'];
    if (g?.pcRequirements) {
      tabs.push('requirements');
    }
    tabs.push('specs');
    return tabs;
  });

  function cycleSubTab(direction: 'PREV' | 'NEXT') {
    const tabs = availableSubTabs;
    const idx = tabs.indexOf(activeTab);
    if (idx === -1) {
      activeTab = tabs[0];
      return;
    }
    if (direction === 'NEXT') {
      activeTab = tabs[(idx + 1) % tabs.length];
    } else {
      activeTab = tabs[(idx - 1 + tabs.length) % tabs.length];
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleWindowKeyDown);
    window.addEventListener('pointerdown', handleWindowPointerDown, true);

    const onFsChange = () => {
      isScreenshotFullscreen = !!document.fullscreenElement;
    };
    document.addEventListener('fullscreenchange', onFsChange);

    const onGalleryPrev = () => prevMedia();
    const onGalleryNext = () => nextMedia();
    const onSubTabPrev = (e: Event) => {
      e.preventDefault();
      cycleSubTab('PREV');
    };
    const onSubTabNext = (e: Event) => {
      e.preventDefault();
      cycleSubTab('NEXT');
    };
    const onModalClose = () => {
      if (document.fullscreenElement) {
        document.exitFullscreen?.().catch(console.error);
      } else if (isSteamModalOpen) {
        closeSteamModal();
      } else if (isVariantDropdownOpen) {
        isVariantDropdownOpen = false;
      }
    };

    window.addEventListener('app:gallery-prev', onGalleryPrev);
    window.addEventListener('app:gallery-next', onGalleryNext);
    window.addEventListener('app:subtab-prev', onSubTabPrev);
    window.addEventListener('app:subtab-next', onSubTabNext);
    window.addEventListener('app:modal-close', onModalClose);

    // Runtime events
    const unsubProgress = EventsOn('download:progress', (event: any) => {
      if (!isMounted || !event) return;
      const cur = pageDetails?.game || game;
      if (cur) {
        const matches = event.gameId === cur.id || (cur.variants && cur.variants.some((v: any) => v.id === event.gameId));
        if (matches && pageDetails) {
          pageDetails.downloadStatus = event.status;
          pageDetails.downloadProgress = event;
          if (event.status === 'completed') {
            pageDetails.isInstalled = true;
          }
        }
      }
    });

    const unsubReviews = EventsOn('game:reviews-updated', (event: any) => {
      if (!isMounted) return;
      if (!event || !game) return;
      if (event.gameId === game.id || (event.steamAppId > 0 && event.steamAppId === game.steamAppId)) {
        if (pageDetails?.game) {
          pageDetails.game.reviewScoreDesc = event.reviewScoreDesc;
          pageDetails.game.reviewPercent = event.reviewPercent;
          pageDetails.game.totalReviews = event.totalReviews;
        }
      }
    });

    const unsubEnriched = EventsOn('game:enriched', (enrichedGame: any) => {
      if (!isMounted || !enrichedGame) return;
      const targetId = game?.id;
      if (
        targetId &&
        (enrichedGame.id === targetId ||
         (enrichedGame.steamAppId > 0 && enrichedGame.steamAppId === game?.steamAppId) ||
         (game?.variants && game.variants.some((v: any) => v.id === enrichedGame.id)))
      ) {
        enrichingGameIds.delete(targetId);
        enrichingGameIds.delete(enrichedGame.id);
        isEnrichingCurrentGame = false;
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

    loadTorrentSourcesMap();
    const unsubTorrents = EventsOn('torrents:updated', () => {
      loadTorrentSourcesMap();
    });

    return () => {
      isMounted = false;
      clearTimeout(switchTimeout);
      clearTimeout(copiedTimeout);
      window.removeEventListener('keydown', handleWindowKeyDown);
      window.removeEventListener('pointerdown', handleWindowPointerDown, true);
      document.removeEventListener('fullscreenchange', onFsChange);
      window.removeEventListener('app:gallery-prev', onGalleryPrev);
      window.removeEventListener('app:gallery-next', onGalleryNext);
      window.removeEventListener('app:subtab-prev', onSubTabPrev);
      window.removeEventListener('app:subtab-next', onSubTabNext);
      window.removeEventListener('app:modal-close', onModalClose);
      if (typeof unsubProgress === 'function') unsubProgress();
      if (typeof unsubReviews === 'function') unsubReviews();
      if (typeof unsubEnriched === 'function') unsubEnriched();
      if (typeof unsubTorrents === 'function') unsubTorrents();
    };
  });

  let hasSteamMetadata = $derived.by(() => {
    const g = pageDetails?.game || game;
    if (!g) return false;
    return !!(
      (g.screenshots && g.screenshots.length > 0) ||
      (g.movies && g.movies.length > 0) ||
      g.shortDescription ||
      g.detailedDescription ||
      g.pcRequirements ||
      (g.genres && g.genres.length > 0) ||
      (g.developers && g.developers.length > 0)
    );
  });

  let activeGameDownload = $derived.by(() => {
    const targetGame = pageDetails?.game || game;
    if (!targetGame) return null;
    return downloadsStore.getDownloadForGame(targetGame);
  });

  let isDownloading = $derived(
    (activeGameDownload && (activeGameDownload.status === 'downloading' || activeGameDownload.status === 'queued' || activeGameDownload.status === 'scanning' || activeGameDownload.status === 'paused')) ||
    pageDetails?.downloadStatus === 'downloading' ||
    pageDetails?.downloadStatus === 'queued' ||
    pageDetails?.downloadStatus === 'scanning' ||
    pageDetails?.downloadStatus === 'paused'
  );

  let isDownloadCompleted = $derived(
    (activeGameDownload && activeGameDownload.status === 'completed') ||
    downloadsStore.isGameInstalled(pageDetails?.game || game) ||
    !!(pageDetails?.isInstalled || pageDetails?.downloadStatus === 'completed')
  );

  let isGameInFavorites = $derived(
    !!(favoriteItem || effectiveFavoriteStatus)
  );
</script>

<div
  bind:this={scrollContainer}
  class="flex-1 flex flex-col h-full overflow-y-auto relative select-none bg-[#090a0d] panel-detail"
  style="--game-accent: {dynamicAccentColor}; --game-accent-text: {dynamicAccentTextColor};"
>
  {#if game}
    {#if isSwitching}
      <!-- Minimal Skeleton: hairline blocks, single shimmer sweep, no decorative cards -->
      <div class="relative z-10 w-full max-w-[1280px] mx-auto flex flex-col flex-1 p-6 lg:p-10 pb-28 select-none" aria-hidden="true">

        <!-- Hero row: cover placeholder + info lines -->
        <div class="grid grid-cols-1 md:grid-cols-12 gap-8 items-start mb-10">

          <!-- Cover: plain rectangle, no border/icon -->
          <div class="md:col-span-4 lg:col-span-3 flex justify-center md:justify-start">
            <div class="w-full max-w-[220px] aspect-[2/3] rounded sk-block"></div>
          </div>

          <!-- Info column: lines only -->
          <div class="md:col-span-8 lg:col-span-9 pt-2 flex flex-col gap-5">
            <!-- Title -->
            <div class="space-y-2.5">
              <div class="sk-line h-7 w-2/3"></div>
              <div class="sk-line h-3.5 w-1/4"></div>
            </div>

            <!-- Synopsis -->
            <div class="space-y-2 pt-1">
              <div class="sk-line h-3.5 w-full max-w-xl"></div>
              <div class="sk-line h-3.5 w-5/6 max-w-lg"></div>
              <div class="sk-line h-3.5 w-3/4 max-w-md"></div>
            </div>

            <!-- Action area -->
            <div class="flex items-center gap-3 pt-2">
              <div class="sk-block h-9 w-28 rounded"></div>
              <div class="sk-block h-9 w-20 rounded"></div>
            </div>
          </div>
        </div>

        <!-- Tabs bar -->
        <div class="border-b border-white/[0.05] pb-3 mb-6 flex gap-8">
          <div class="sk-line h-4 w-20"></div>
          <div class="sk-line h-4 w-28 opacity-50"></div>
          <div class="sk-line h-4 w-28 opacity-30"></div>
        </div>

        <!-- Media viewport -->
        <div class="sk-block w-full aspect-video rounded mb-3"></div>

        <!-- Thumbnail strip -->
        <div class="flex gap-2 mb-8">
          {#each [1,2,3,4] as _}
            <div class="sk-block flex-shrink-0 w-32 aspect-video rounded"></div>
          {/each}
        </div>

        <!-- Description lines -->
        <div class="space-y-2.5 max-w-2xl">
          <div class="sk-line h-3.5 w-full"></div>
          <div class="sk-line h-3.5 w-11/12"></div>
          <div class="sk-line h-3.5 w-5/6"></div>
          <div class="sk-line h-3.5 w-3/4 opacity-60"></div>
          <div class="sk-line h-3.5 w-2/3 opacity-40"></div>
        </div>

      </div>

    {:else}
      {@const g = pageDetails?.game || game}
      {@const bgUrl = getHeroBackgroundUrl(g)}
      {@const coverUrl = getPrimaryCoverUrl(g)}
      {@const hasCover = !!(coverUrl && !imageLoadFailed[coverUrl])}
      {@const logoUrl = pageDetails?.logoUrl}
      {@const hasLogo = !!(logoUrl && !imageLoadFailed[logoUrl] && !isHorizontalAsset(logoUrl))}

      <!-- Atmospheric Steam Backdrop with Dark Scrim for High-Contrast Readability -->
      {#if bgUrl}
      <div
        class="absolute top-0 left-0 right-0 h-[650px] overflow-hidden pointer-events-none z-0 select-none"
        style="mask-image: linear-gradient(to bottom, rgba(0,0,0,1) 0%, rgba(0,0,0,0.85) 30%, rgba(0,0,0,0.2) 70%, rgba(0,0,0,0) 95%); -webkit-mask-image: linear-gradient(to bottom, rgba(0,0,0,1) 0%, rgba(0,0,0,0.85) 30%, rgba(0,0,0,0.2) 70%, rgba(0,0,0,0) 95%);"
      >
        <img
          src={bgUrl}
          alt=""
          referrerpolicy="no-referrer"
          class="w-full h-full object-cover object-top brightness-75 contrast-105 opacity-85"
          onerror={() => {
            imageLoadFailed[bgUrl] = true;
          }}
        />
        <div class="absolute inset-0 bg-gradient-to-b from-[#090a0d]/30 via-[#090a0d]/65 to-[#090a0d]"></div>
        <div class="absolute inset-0 bg-gradient-to-r from-[#090a0d]/75 via-transparent to-[#090a0d]/75"></div>
      </div>
    {/if}

    <!-- Main Stage Container -->
    <div class="relative z-10 w-full max-w-[1280px] mx-auto flex flex-col flex-1 p-6 lg:p-10 space-y-8 pb-28">
      
      <!-- TOP HERO SECTION -->
      <div class="grid grid-cols-1 md:grid-cols-12 gap-8 items-start">
        
        <!-- COVER / ART COLUMN -->
        <div class="{isCoverLandscape ? 'md:col-span-5 lg:col-span-5' : 'md:col-span-4 lg:col-span-3'} flex justify-center md:justify-start">
          {#if hasCover}
            <div
              class="relative rounded overflow-hidden bg-[#07080a] group w-full {isCoverLandscape ? '' : 'max-w-[280px]'} transition-all duration-300 flex items-center justify-center border border-white/[0.08]"
              style={coverAspectRatio ? `aspect-ratio: ${coverAspectRatio};` : (isCoverLandscape ? 'aspect-ratio: 16/9;' : 'aspect-ratio: 2/3;')}
            >
              <img
                src={coverUrl}
                alt={g.cleanTitle}
                referrerpolicy="no-referrer"
                class="w-full h-full object-cover transition-transform duration-300 group-hover:scale-102"
                onload={(e) => onCoverLoaded(e.currentTarget as HTMLImageElement)}
                onerror={() => {
                  imageLoadFailed[coverUrl] = true;
                }}
              />
              <div class="absolute inset-0 bg-gradient-to-tr from-transparent via-white/[0.03] to-white/[0.08] pointer-events-none z-20"></div>
            </div>
          {:else}
            <!-- Ascetic Tactile Poster Frame for Games Pending Cover / Steam Enrichment -->
            <div
              class="relative rounded w-full max-w-[280px] aspect-[2/3] bg-[#07080a] border border-white/[0.08] flex flex-col items-center justify-between p-6 text-center shadow-inner"
            >
              <div class="flex-1 flex flex-col items-center justify-center w-full space-y-3">
                <div class="w-14 h-14 rounded bg-white/[0.03] border border-white/[0.06] flex items-center justify-center text-[#8e95a2]">
                  <Disc class="w-7 h-7 stroke-[1.5] text-[#8e95a2]" />
                </div>
                <div class="text-xs font-semibold text-white/70 line-clamp-3 px-1 leading-snug">
                  {getDisplayTitle(g)}
                </div>
              </div>

              <div class="pt-3 w-full border-t border-white/[0.04]">
                {#if isEnrichingCurrentGame}
                  <div class="inline-flex items-center gap-1.5 text-[11px] text-[#8e95a2]">
                    <RefreshCw class="w-3 h-3 animate-spin text-[#8e95a2]" />
                    <span>Поиск обложки...</span>
                  </div>
                {:else}
                  <button
                    data-nav-item
                    type="button"
                    class="w-full py-2 px-3 rounded bg-white/[0.04] hover:bg-white/[0.08] text-xs font-medium text-[#cbd5e1] hover:text-white border border-white/[0.08] hover:border-white/20 transition-colors cursor-pointer flex items-center justify-center gap-1.5"
                    onclick={() => openSteamModal(g)}
                  >
                    <Search class="w-3.5 h-3.5 text-[#8e95a2]" />
                    <span>Найти в Steam</span>
                  </button>
                {/if}
              </div>
            </div>
          {/if}
        </div>

        <!-- DETAILS & ACTION COLUMN -->
        <div class="{isCoverLandscape ? 'md:col-span-7 lg:col-span-7' : 'md:col-span-8 lg:col-span-9'} space-y-5">
          
          <!-- LOGO OR TITLE + REVIEWS / METACRITIC -->
          <div class="flex flex-col sm:flex-row items-start justify-between gap-4 border-b border-white/[0.06] pb-4">
            
            <!-- Logo / Title Display -->
            <div class="space-y-1 min-w-0">
              {#if hasLogo}
                <!-- Transparent Game Logo (SteamGridDB / Steam) -->
                <div class="py-1">
                  <img
                    src={logoUrl}
                    alt={getDisplayTitle(g)}
                    class="max-h-36 sm:max-h-44 md:max-h-48 max-w-[440px] sm:max-w-[580px] object-contain object-left select-none filter drop-shadow-md"
                    onerror={() => {
                      if (logoUrl) {
                        imageLoadFailed[logoUrl] = true;
                        if (pageDetails) pageDetails.logoUrl = '';
                      }
                    }}
                  />
                </div>
              {:else}
                <!-- High-contrast crisp title text -->
                <h1 class="text-2xl sm:text-3xl lg:text-4xl font-extrabold text-white tracking-tight uppercase leading-tight">
                  {getDisplayTitle(g)}
                </h1>
              {/if}
            </div>

            <!-- Review / Metacritic Score -->
            {#if (g.totalReviews && g.totalReviews > 0) || (g.reviewPercent && g.reviewPercent > 0)}
              {@const isPositive = (g.reviewPercent || 0) >= 70}
              {@const isMixed = (g.reviewPercent || 0) >= 40 && (g.reviewPercent || 0) < 70}
              <div class="flex items-center gap-3 bg-[#11141c] border border-white/10 p-2.5 px-3.5 rounded flex-shrink-0">
                <div
                  class="w-11 h-11 rounded border flex items-center justify-center font-black text-sm {isPositive ? 'bg-[#66c0f4]/15 text-[#66c0f4] border-[#66c0f4]/40' : isMixed ? 'bg-amber-500/15 text-amber-400 border-amber-500/40' : 'bg-red-500/15 text-red-400 border-red-500/40'}"
                >
                  {g.reviewPercent || 0}%
                </div>
                <div class="space-y-0.5">
                  <div class="text-xs font-bold {isPositive ? 'text-[#66c0f4]' : isMixed ? 'text-amber-400' : 'text-red-400'} leading-tight">
                    {g.reviewScoreDesc || (isPositive ? 'Положительные' : isMixed ? 'Смешанные' : 'Отрицательные')}
                  </div>
                  <div class="text-[10px] font-medium tracking-wide text-[#8e95a2]">
                    {formatReviewsCount(g.totalReviews || 0)}
                  </div>
                </div>
              </div>
            {:else if g.metacriticScore && g.metacriticScore > 0}
              <div class="flex items-center gap-3 bg-[#11141c] border border-white/10 p-2.5 px-3.5 rounded flex-shrink-0">
                <div
                  class="w-11 h-11 rounded border flex items-center justify-center font-black text-sm"
                  style="border-color: var(--game-accent); color: var(--game-accent); background-color: color-mix(in srgb, var(--game-accent) 15%, transparent);"
                >
                  {g.metacriticScore}
                </div>
                <div class="space-y-0.5">
                  <div class="flex items-center text-amber-400 text-xs tracking-widest">★ ★ ★ ★ ★</div>
                  <div class="text-[10px] font-black uppercase tracking-wider text-[#9ca3af]">METACRITIC</div>
                </div>
              </div>
            {/if}
          </div>

          <!-- Discreet Steam Enrichment / Metadata status -->
          {#if isEnrichingCurrentGame}
            <div class="flex items-center gap-2 text-xs text-[#8e95a2] py-0.5">
              <RefreshCw class="w-3.5 h-3.5 animate-spin text-[#8e95a2] flex-shrink-0" />
              <span>Синхронизация данных с базой Steam...</span>
            </div>
          {:else if !hasSteamMetadata}
            <div class="flex items-center gap-2 text-xs text-[#6b7280] py-0.5">
              <span>Данные Steam не синхронизированы.</span>
              <button
                data-nav-item
                type="button"
                class="text-[#cbd5e1] hover:text-white hover:underline inline-flex items-center gap-1 cursor-pointer font-medium"
                onclick={() => openSteamModal(g)}
              >
                <Edit3 class="w-3 h-3" />
                <span>Привязать вручную</span>
              </button>
            </div>
          {/if}

          <!-- Short Synopsis -->
          {#if g.shortDescription}
            <div class="text-sm sm:text-base text-[#cbd5e1] leading-relaxed font-normal">
              {@html g.shortDescription}
            </div>
          {/if}

          <!-- ACTION DECK: Smart Download / Play / Folder Controls -->
          <div class="space-y-3 pt-2">
            
            {#snippet favoriteButton()}
              <div class="relative inline-flex items-center">
                <button
                  bind:this={favoriteDropdownTriggerEl}
                  data-nav-item
                  type="button"
                  class="h-11 inline-flex items-center gap-2 text-xs px-3.5 rounded transition-colors cursor-pointer border border-white/10 bg-white/[0.04] hover:bg-white/[0.08] text-[#cbd5e1] hover:text-white"
                  onclick={(e) => {
                    e.stopPropagation();
                    isFavoriteDropdownOpen = !isFavoriteDropdownOpen;
                  }}
                  title="Добавить в избранное / статус прохождения"
                >
                  {#if effectiveFavoriteStatus === 'playing'}
                    <Gamepad2 class="w-3.5 h-3.5 text-amber-400" />
                    <span class="font-medium text-[#ededed]">Прохожу</span>
                  {:else if effectiveFavoriteStatus === 'completed'}
                    <CheckCircle2 class="w-3.5 h-3.5 text-emerald-400" />
                    <span class="font-medium text-[#ededed]">Прошел</span>
                  {:else if effectiveFavoriteStatus === 'planned'}
                    <BookmarkCheck class="w-3.5 h-3.5 text-sky-400" />
                    <span class="font-medium text-[#ededed]">В планах</span>
                  {:else}
                    <Bookmark class="w-3.5 h-3.5 text-[#8e95a2]" />
                    <span class="text-[#8e95a2]">В избранное</span>
                  {/if}
                  <ChevronDown class="w-3.5 h-3.5 text-[#8e95a2] transition-transform duration-200 {isFavoriteDropdownOpen ? 'rotate-180' : ''}" />
                </button>

                {#if isFavoriteDropdownOpen}
                  <div
                    bind:this={favoriteDropdownContainerEl}
                    class="absolute left-0 top-full mt-2 z-50 min-w-[190px] rounded bg-[#0d1117] border border-white/10 shadow-2xl p-1.5 space-y-1 text-xs"
                  >
                    <div class="px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider text-[#6b7280]">
                      Статус в избранном
                    </div>
                    <button
                      type="button"
                      class="w-full text-left flex items-center gap-2 px-2.5 py-2 rounded-sm transition-colors cursor-pointer {effectiveFavoriteStatus === 'planned' ? 'bg-sky-500/20 text-sky-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                      onclick={() => handleToggleFavoriteStatus('planned')}
                    >
                      <Clock class="w-3.5 h-3.5 text-sky-400" />
                      <span>В планах</span>
                      {#if effectiveFavoriteStatus === 'planned'}
                        <Check class="w-3.5 h-3.5 ml-auto text-sky-400" />
                      {/if}
                    </button>
                    <button
                      type="button"
                      class="w-full text-left flex items-center gap-2 px-2.5 py-2 rounded-sm transition-colors cursor-pointer {effectiveFavoriteStatus === 'playing' ? 'bg-amber-500/20 text-amber-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                      onclick={() => handleToggleFavoriteStatus('playing')}
                    >
                      <Gamepad2 class="w-3.5 h-3.5 text-amber-400" />
                      <span>Прохожу</span>
                      {#if effectiveFavoriteStatus === 'playing'}
                        <Check class="w-3.5 h-3.5 ml-auto text-amber-400" />
                      {/if}
                    </button>
                    <button
                      type="button"
                      class="w-full text-left flex items-center gap-2 px-2.5 py-2 rounded-sm transition-colors cursor-pointer {effectiveFavoriteStatus === 'completed' ? 'bg-emerald-500/20 text-emerald-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                      onclick={() => handleToggleFavoriteStatus('completed')}
                    >
                      <CheckCircle2 class="w-3.5 h-3.5 text-emerald-400" />
                      <span>Прошел</span>
                      {#if effectiveFavoriteStatus === 'completed'}
                        <Check class="w-3.5 h-3.5 ml-auto text-emerald-400" />
                      {/if}
                    </button>

                    {#if effectiveFavoriteStatus}
                      <div class="h-px bg-white/[0.06] my-1"></div>
                      <button
                        type="button"
                        class="w-full text-left flex items-center gap-2 px-2.5 py-2 rounded-sm text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
                        onclick={handleRemoveFavorite}
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                        <span>Удалить из избранного</span>
                      </button>
                    {/if}
                  </div>
                {/if}
              </div>
            {/snippet}

            {#snippet gearButton()}
              {#if isGameInFavorites}
                <button
                  data-nav-item
                  type="button"
                  title={hasCustomExe ? "Настройки запуска (настроено)" : "Настройки запуска (укажите .exe файл)"}
                  class="h-11 w-11 flex-shrink-0 flex items-center justify-center rounded transition-all cursor-pointer border border-white/10 bg-white/[0.04] hover:bg-white/[0.08] text-[#8e95a2] hover:text-white"
                  onclick={openLaunchConfigModal}
                >
                  <Settings class="w-4 h-4" />
                </button>
              {/if}
            {/snippet}

            {#snippet folderButton()}
              {#if pageDetails?.localPath || pageDetails?.isInstalled}
                <button
                  data-nav-item
                  type="button"
                  title="Открыть папку с игрой"
                  class="h-11 w-11 flex-shrink-0 flex items-center justify-center rounded transition-all cursor-pointer border border-white/10 bg-white/[0.04] hover:bg-white/[0.08] text-[#8e95a2] hover:text-white"
                  onclick={handleOpenFolder}
                >
                  <FolderOpen class="w-4 h-4" />
                </button>
              {/if}
            {/snippet}

            {#snippet steamButton()}
              {#if isGameInFavorites}
                <button
                  data-nav-item
                  type="button"
                  title={isGameInSteam ? "Игра добавлена в Steam (нажмите для повторной синхронизации)" : "Добавить игру со всеми обложками в библиотеку Steam"}
                  class="h-11 px-3.5 flex items-center justify-center gap-2 rounded transition-all cursor-pointer border text-xs font-medium {isGameInSteam ? 'bg-sky-500/15 border-sky-500/30 text-sky-300 hover:bg-sky-500/25' : 'bg-white/[0.04] hover:bg-white/[0.08] border-white/10 text-[#8e95a2] hover:text-white'}"
                  onclick={handleAddToSteam}
                  disabled={isAddingToSteam}
                >
                  {#if isAddingToSteam}
                    <RefreshCw class="w-3.5 h-3.5 animate-spin text-sky-400 flex-shrink-0" />
                    <span>В Steam...</span>
                  {:else if isGameInSteam}
                    <Check class="w-3.5 h-3.5 text-sky-400 flex-shrink-0" />
                    <span>В Steam</span>
                  {:else}
                    <SteamLogo size={16} weight="bold" class="flex-shrink-0" />
                    <span>В Steam</span>
                  {/if}
                </button>
              {/if}
            {/snippet}

            {#snippet openSteamStoreButton()}
              {#if g.steamAppId && g.steamAppId > 0}
                <button
                  data-nav-item
                  type="button"
                  title="Открыть страницу игры в магазине Steam"
                  class="h-11 px-3.5 flex items-center justify-center gap-2 rounded transition-all cursor-pointer border text-xs font-medium bg-white/[0.04] hover:bg-white/[0.08] active:bg-white/[0.12] border-white/10 text-[#cbd5e1] hover:text-white"
                  onclick={handleOpenSteamStore}
                >
                  <SteamLogo size={16} weight="bold" class="flex-shrink-0 text-[#66c0f4]" />
                  <span>В Steam</span>
                  <ExternalLink class="w-3.5 h-3.5 text-[#8e95a2] flex-shrink-0" />
                </button>
              {/if}
            {/snippet}

            {#if isDownloadCompleted || pageDetails?.isInstalled}

              <!-- ALREADY INSTALLED / DOWNLOAD COMPLETED STATE: PLAY + GEAR + FOLDER + FAVORITE -->
              <div class="space-y-2">
                <div class="flex flex-wrap items-center gap-2.5">
                  <button
                    data-nav-item
                    class="h-11 px-8 text-sm font-black rounded flex items-center justify-center gap-2.5 cursor-pointer active:scale-95 transition-all hover:brightness-110 shadow-lg uppercase tracking-wider"
                    style="background-color: var(--game-accent); color: var(--game-accent-text);"
                    onclick={handlePlayButtonClick}
                  >
                    <Play class="w-4 h-4 fill-current stroke-[2]" />
                    <span>ИГРАТЬ</span>
                  </button>

                  {@render gearButton()}
                  {@render folderButton()}
                  {@render steamButton()}
                  {@render openSteamStoreButton()}
                  {@render favoriteButton()}
                </div>

                <!-- Minimalist Sleek Metadata Line: Folder Path • Size • Original Release -->
                <div class="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-xs text-[#8e95a2] pt-0.5">
                  {#if pageDetails?.localPath}
                    <button
                      type="button"
                      class="inline-flex items-center gap-1.5 font-mono text-[#94a3b8] hover:text-white transition-colors cursor-pointer hover:underline"
                      onclick={handleOpenFolder}
                      title="Нажмите, чтобы открыть папку в проводнике"
                    >
                      <Folder class="w-3.5 h-3.5 text-[#6b7280]" />
                      <span class="truncate max-w-md">{pageDetails.localPath}</span>
                    </button>
                  {/if}

                  {#if formatSizeDisplay(g)}
                    {#if pageDetails?.localPath}
                      <span class="text-white/20 select-none">•</span>
                    {/if}
                    <span class="font-mono text-[#8e95a2]">{formatSizeDisplay(g)}</span>
                  {/if}

                  {#if (activeVariant?.rawName || g.rawName)}
                    <span class="text-white/20 select-none">•</span>
                    <span class="font-mono text-[#64748b] truncate max-w-md" title={activeVariant?.rawName || g.rawName}>
                      {activeVariant?.rawName || g.rawName}
                    </span>
                  {/if}
                </div>
              </div>

            {:else if isDownloading}
              <!-- ACTIVE DOWNLOADING / QUEUED STATE -->
              {@const prog = activeGameDownload || pageDetails?.downloadProgress}
              {@const dlStatus = activeGameDownload?.status || pageDetails?.downloadStatus}
              <div class="space-y-3 max-w-md bg-[#07080a] p-4 rounded border border-white/[0.06]">
                <div class="flex items-center justify-between text-xs font-bold text-white">
                  <div class="flex items-center gap-2">
                    <Download class="w-4 h-4 text-[var(--game-accent)]" />
                    <span>{dlStatus === 'paused' ? 'Приостановлено' : (dlStatus === 'queued' ? 'В очереди загрузки...' : (dlStatus === 'scanning' ? 'Получение метаданных торрента...' : 'Скачивается...'))}</span>
                  </div>
                  <span class="font-mono text-[var(--game-accent)]">{Math.round(prog?.progressPercent || 0)}%</span>
                </div>

                <!-- Progress Bar -->
                <div class="w-full h-2 rounded-none bg-black/60 overflow-hidden border border-white/5">
                  <div
                    class="h-full rounded-none transition-all duration-300"
                    style="width: {Math.max(2, prog?.progressPercent || 0)}%; background-color: var(--game-accent);"
                  ></div>
                </div>

                <div class="flex items-center justify-between pt-1">
                  {#if prog?.speedDisplay}
                    <div class="text-[11px] font-mono text-[#8e95a2]">
                      <span>{prog.speedDisplay}</span>
                      {#if prog.etaDisplay}
                        <span class="ml-2">({prog.etaDisplay})</span>
                      {/if}
                    </div>
                  {/if}
                  <div class="flex items-center gap-2">
                    {@render openSteamStoreButton()}
                    {@render favoriteButton()}
                  </div>
                </div>
              </div>

              {#if (activeVariant?.rawName || g.rawName)}
                <div class="flex items-center gap-2 text-[11px] font-mono text-[#64748b]">
                  <Disc class="w-3.5 h-3.5 text-[#8e95a2] flex-shrink-0" />
                  <span class="text-[#64748b] flex-shrink-0">Оригинальный релиз:</span>
                  <span
                    class="text-[#cbd5e1] font-medium truncate max-w-2xl select-all"
                    title={activeVariant?.rawName || g.rawName}
                  >
                    {activeVariant?.rawName || g.rawName}
                  </span>
                </div>
              {/if}

            {:else}
              <!-- NOT INSTALLED / READY TO DOWNLOAD STATE -->
              <div class="space-y-2">
                <div class="flex flex-wrap items-center gap-2.5">
                  {#if hasCustomExe}
                    <!-- ИГРАТЬ via custom exe -->
                    <button
                      data-nav-item
                      class="h-11 px-8 text-xs sm:text-sm font-black rounded flex items-center gap-2.5 cursor-pointer transition-all hover:brightness-110 active:scale-[0.98] uppercase tracking-wider shadow-lg"
                      style="background-color: var(--game-accent); color: var(--game-accent-text);"
                      onclick={handleLaunchWithCustom}
                    >
                      <Play class="w-4 h-4 fill-current stroke-[2]" />
                      <span>ИГРАТЬ</span>
                    </button>
                    {@render gearButton()}
                    {@render folderButton()}
                    {@render steamButton()}
                    {@render openSteamStoreButton()}
                    {@render favoriteButton()}
                  {:else}
                    <!-- Unified Split Download Button -->
                    <div class="relative inline-flex items-stretch rounded shadow-lg border border-white/10 overflow-visible {isVariantDropdownOpen ? 'z-30' : ''}">
                      <button
                        data-nav-item
                        class="h-11 px-7 text-xs sm:text-sm font-black flex items-center gap-2.5 cursor-pointer transition-all hover:brightness-110 active:scale-[0.98] uppercase tracking-wider {g.variants && g.variants.length > 1 ? 'rounded-l' : 'rounded'}"
                        style="background-color: var(--game-accent); color: var(--game-accent-text);"
                        onclick={() => onStartDownload(selectedVariantId || g.id, customDownloadPath)}
                      >
                        <Download class="w-4 h-4 stroke-[2.5]" />
                        <span>СКАЧАТЬ</span>
                        <span class="opacity-35 font-normal">|</span>
                        <span class="font-mono text-xs font-bold tracking-normal">{activeVariant?.sizeDisplay || formatSizeDisplay(g)}</span>
                      </button>

                      {#if g.variants && g.variants.length > 1}
                        <button
                          bind:this={variantDropdownTriggerEl}
                          data-nav-item
                          type="button"
                          class="h-11 px-3 flex items-center justify-center border-l border-black/20 hover:brightness-110 active:scale-95 cursor-pointer transition-all rounded-r"
                          style="background-color: var(--game-accent); color: var(--game-accent-text);"
                          onclick={(e) => {
                            e.stopPropagation();
                            isVariantDropdownOpen = !isVariantDropdownOpen;
                          }}
                          title="Выбрать версию ({g.variants.length} доступно)"
                        >
                          <ChevronDown class="w-4 h-4 stroke-[2.5] transition-transform duration-200 {isVariantDropdownOpen ? 'rotate-180' : ''}" />
                        </button>

                        {#if isVariantDropdownOpen}
                          <div
                            bind:this={variantDropdownContainerEl}
                            class="absolute left-0 top-full mt-2 z-50 min-w-[340px] sm:min-w-[420px] max-w-[500px] max-h-72 overflow-y-auto overscroll-contain rounded bg-[#0d1117] border border-white/10 shadow-2xl p-1.5 space-y-1"
                          >
                            <div class="px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider text-[#6b7280]">
                              Выбор версии для скачивания ({g.variants.length})
                            </div>
                            {#each g.variants as variant (variant.id)}
                              {@const isSelected = (activeVariant?.id === variant.id)}
                              <button
                                type="button"
                                class="w-full text-left flex items-center justify-between gap-3 px-3 py-2.5 rounded-sm text-xs transition-colors cursor-pointer {isSelected ? 'bg-white/10 text-white font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                                onclick={(e) => {
                                  e.stopPropagation();
                                  selectedVariantId = variant.id;
                                  isVariantDropdownOpen = false;
                                }}
                              >
                                <div class="min-w-0 flex-1 pointer-events-none">
                                  <div class="truncate text-white text-xs">{variant.rawName}</div>
                                  <div class="flex items-center gap-2 text-[10px] text-[#6b7280] font-mono mt-0.5">
                                    <span>Источник: {variant.sourceType === 'torrent' || variant.magnetUri ? `Торрент (${formatSourceName(variant.torrentSource) || 'Каталог'})` : 'FTP-сервер'}</span>
                                    {#if variant.sourceType === 'torrent' || variant.magnetUri}
                                      {#if variantSeeds[variant.id]?.loading}
                                        <span class="text-white/20">•</span>
                                        <span class="text-[#64748b] animate-pulse">сиды: ...</span>
                                      {:else if variantSeeds[variant.id]}
                                        {@const s = variantSeeds[variant.id].seeders}
                                        <span class="text-white/20">•</span>
                                        <span class="inline-flex items-center gap-0.5 {s > 0 ? 'text-emerald-400 font-semibold' : 'text-[#64748b]'}">
                                          <UploadSimple class="w-3 h-3 stroke-[2.5]" />
                                          <span>{formatSeedsCount(s)}</span>
                                        </span>
                                      {/if}
                                    {/if}
                                  </div>
                                </div>
                                <div class="flex items-center gap-2 flex-shrink-0 font-mono text-[11px] pointer-events-none {isSelected ? 'text-[var(--game-accent)] font-bold' : 'text-[#6b7280]'}">
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

                    {@render steamButton()}
                    {@render openSteamStoreButton()}
                    {@render favoriteButton()}
                    {@render gearButton()}
                  {/if}
                </div>

                <!-- Minimalist Sleek Metadata Line: Folder Path • Source • Seeders • Release Name -->
                <div class="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-xs text-[#8e95a2] pt-0.5">
                  <button
                    type="button"
                    class="inline-flex items-center gap-1.5 font-mono text-[#94a3b8] hover:text-white transition-colors cursor-pointer hover:underline"
                    onclick={handleBrowseFolder}
                    title="Нажмите, чтобы изменить папку для сохранения"
                  >
                    <Folder class="w-3.5 h-3.5 text-[#6b7280]" />
                    <span class="truncate max-w-xs">{customDownloadPath}</span>
                  </button>

                  {#if downloadSourceInfo}
                    <span class="text-white/20 select-none">•</span>
                    <span class="text-[#8e95a2]">{downloadSourceInfo.shortName}</span>
                  {/if}

                  {#if currentVariantSeedInfo}
                    <span class="text-white/20 select-none">•</span>
                    {#if currentVariantSeedInfo.loading}
                      <span class="text-[#64748b] font-mono text-[11px] animate-pulse">поиск сидов...</span>
                    {:else}
                      <span class="inline-flex items-center gap-1 font-mono text-[11px] {currentVariantSeedInfo.seeders > 0 ? 'text-emerald-400 font-semibold' : 'text-[#64748b]'}">
                        <UploadSimple class="w-3.5 h-3.5 stroke-[2.5]" />
                        <span>{formatSeedsCount(currentVariantSeedInfo.seeders)}</span>
                      </span>
                    {/if}
                  {/if}

                  {#if (activeVariant?.rawName || g.rawName)}
                    <span class="text-white/20 select-none">•</span>
                    <span
                      class="font-mono text-[#64748b] truncate max-w-md"
                      title={activeVariant?.rawName || g.rawName}
                    >
                      {activeVariant?.rawName || g.rawName}
                    </span>
                  {/if}
                </div>
              </div>
            {/if}

          </div>

        </div>
      </div>

      <!-- Launch Config Modal -->
      {#if isLaunchConfigOpen && (favoriteItem || isGameInFavorites)}
        <!-- Backdrop -->
        <button
          type="button"
          aria-label="Закрыть настройки запуска"
          class="fixed inset-0 z-50 bg-black/70 cursor-default border-none p-0 m-0 w-full h-full"
          onclick={() => (isLaunchConfigOpen = false)}
        ></button>

        <!-- Modal Panel -->
        <div class="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none">
          <div class="pointer-events-auto w-full max-w-md bg-[#0d1117] border border-white/10 rounded-md shadow-2xl flex flex-col overflow-hidden">
            <!-- Header -->
            <div class="flex items-center justify-between px-5 py-4 border-b border-white/[0.06]">
              <div class="flex items-center gap-2.5 min-w-0">
                <Settings class="w-4 h-4 text-[#94a3b8] flex-shrink-0" />
                <span class="text-sm font-bold text-white truncate">
                  Настройки запуска — {getDisplayTitle(g)}
                </span>
              </div>
              <button
                type="button"
                class="w-7 h-7 flex-shrink-0 flex items-center justify-center rounded-sm text-[#6b7280] hover:text-white hover:bg-white/10 transition-colors cursor-pointer"
                onclick={() => (isLaunchConfigOpen = false)}
              >
                <X class="w-4 h-4" />
              </button>
            </div>

            <!-- Body -->
            <div class="px-5 py-4 space-y-4">
              <!-- Auto-detected Exe Candidates (if any found) -->
              {#if isScanningCandidates}
                <div class="flex items-center gap-2 text-xs text-[#8e95a2] py-1">
                  <RefreshCw class="w-3.5 h-3.5 animate-spin text-[#8e95a2]" />
                  <span>Поиск исполняемых файлов в папке игры...</span>
                </div>
              {:else if lcCandidates && lcCandidates.length > 0}
                <div class="space-y-1.5">
                  <div class="text-[11px] font-semibold uppercase tracking-wider text-[#8e95a2] flex items-center justify-between">
                    <span>Обнаруженные файлы игры ({lcCandidates.length})</span>
                    <span class="text-[10px] text-[#64748b] font-normal lowercase">нажмите для выбора</span>
                  </div>
                  <div class="flex flex-col gap-1 max-h-32 overflow-y-auto pr-1">
                    {#each lcCandidates as cand}
                      {@const fileName = cand.split(/[/\\]/).pop()}
                      {@const isSelected = lcExePath === cand}
                      <button
                        type="button"
                        class="text-left flex items-center justify-between px-3 py-2 rounded-sm text-xs font-mono transition-colors cursor-pointer {isSelected ? 'bg-white/10 text-white font-bold border border-white/20' : 'bg-white/[0.03] hover:bg-white/[0.07] text-[#cbd5e1] border border-white/[0.05]'}"
                        onclick={() => { lcExePath = cand; }}
                      >
                        <span class="truncate">{fileName}</span>
                        {#if isSelected}
                          <Check class="w-3.5 h-3.5 text-emerald-400 flex-shrink-0 ml-2" />
                        {/if}
                      </button>
                    {/each}
                  </div>
                </div>
              {/if}

              <!-- Exe Path -->
              <div class="space-y-1.5">
                <label for="launch-config-exe-input" class="text-[11px] font-semibold uppercase tracking-wider text-[#8e95a2]">
                  Исполняемый файл (.exe)
                </label>
                <div class="flex items-center gap-2">
                  <input
                    id="launch-config-exe-input"
                    type="text"
                    bind:value={lcExePath}
                    placeholder="C:\Games\game.exe"
                    spellcheck="false"
                    class="flex-1 min-w-0 bg-[#07080a] text-[#ededed] placeholder-[#5a6170] text-xs font-mono rounded px-3 py-2.5 border border-white/[0.08] focus:border-white/25 focus:outline-none transition-colors"
                  />
                  <button
                    type="button"
                    title="Выбрать файл на диске"
                    class="flex-shrink-0 w-9 h-9 flex items-center justify-center rounded bg-white/[0.05] hover:bg-white/10 border border-white/[0.08] text-[#94a3b8] hover:text-white transition-colors cursor-pointer"
                    onclick={handleBrowseExe}
                  >
                    <Folder class="w-4 h-4" />
                  </button>
                </div>
                {#if lcExePath}
                  <p class="text-[10px] text-[#6b7280] font-mono truncate">{lcExePath}</p>
                {/if}
              </div>

              <!-- Launch Args -->
              <div class="space-y-1.5">
                <label for="launch-config-args-input" class="text-[11px] font-semibold uppercase tracking-wider text-[#8e95a2]">
                  Параметры запуска
                  <span class="normal-case font-normal text-[#6b7280]">(необязательно)</span>
                </label>
                <input
                  id="launch-config-args-input"
                  type="text"
                  bind:value={lcLaunchArgs}
                  placeholder="-dx12 -fullscreen -windowed"
                  spellcheck="false"
                  class="w-full bg-[#07080a] text-[#ededed] placeholder-[#5a6170] text-xs font-mono rounded px-3 py-2.5 border border-white/[0.08] focus:border-white/25 focus:outline-none transition-colors"
                />
              </div>

              <!-- Note -->
              {#if !lcExePath}
                <p class="text-[11px] text-[#6b7280] leading-relaxed">
                  Укажите путь к исполняемому файлу игры или выберите его из обнаруженных выше.
                </p>
              {/if}
            </div>

            <!-- Footer -->
            <div class="flex items-center gap-2.5 px-5 py-4 border-t border-white/[0.06]">
              {#if lcExePath.trim()}
                <button
                  type="button"
                  disabled={lcSaving}
                  class="h-9 px-4 flex items-center justify-center gap-2 rounded text-xs font-bold transition-all cursor-pointer shadow-md hover:brightness-110 active:scale-95"
                  style="background-color: var(--game-accent); color: var(--game-accent-text);"
                  onclick={() => handleSaveLaunchConfig(true)}
                >
                  <Play class="w-3.5 h-3.5 fill-current" />
                  <span>Сохранить и играть</span>
                </button>
              {/if}

              <button
                type="button"
                disabled={lcSaving}
                class="flex-1 h-9 flex items-center justify-center gap-2 rounded text-xs font-bold transition-colors cursor-pointer
                  {lcSaveSuccess
                    ? 'bg-emerald-500/15 border border-emerald-500/30 text-emerald-300'
                    : 'bg-white text-slate-950 hover:bg-white/90 active:scale-[0.98]'}"
                onclick={() => handleSaveLaunchConfig(false)}
              >
                {#if lcSaveSuccess}
                  <Check class="w-4 h-4" />
                  <span>Сохранено</span>
                {:else}
                  <span>{lcSaving ? 'Сохранение...' : 'Сохранить'}</span>
                {/if}
              </button>

              <button
                type="button"
                class="h-9 px-3.5 rounded text-xs font-medium text-[#8e95a2] hover:text-white bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] transition-colors cursor-pointer"
                onclick={() => (isLaunchConfigOpen = false)}
              >
                Отмена
              </button>
            </div>
          </div>
        </div>
      {/if}

      <!-- SNIPPETS FOR CONTENT SECTIONS -->
      {#snippet mediaAndDescription()}
        <div class="space-y-6">
          <!-- UNIFIED MEDIA SHOWCASE -->
          {#if mediaList.length > 0 && activeMedia}
            <div class="space-y-3">
              <!-- Main Viewport (16:9) -->
              <div
                bind:this={screenshotViewport}
                class="relative w-full aspect-video rounded overflow-hidden bg-black border border-white/10 group/viewer flex items-center justify-center"
              >
                {#if activeMedia.type === 'video'}
                  <VideoPlayer
                    hls={activeMedia.hls}
                    mp4={activeMedia.mp4}
                    webm={activeMedia.webm}
                    poster={activeMedia.thumbnail}
                    title={activeMedia.name}
                    autoplay={false}
                    controls={true}
                  />
                {:else}
                  <img
                    src={activeMedia.url}
                    alt={activeMedia.name}
                    class="w-full h-full object-contain cursor-pointer select-none"
                    ondblclick={toggleScreenshotFullscreen}
                  />

                  <!-- Previous / Next Media Navigation -->
                  {#if mediaList.length > 1}
                    <button
                      type="button"
                      data-nav-item
                      class="absolute left-3 top-1/2 -translate-y-1/2 p-2.5 rounded-full bg-black/60 hover:bg-black/90 text-white border border-white/10 opacity-0 group-hover/viewer:opacity-100 transition-opacity cursor-pointer active:scale-95 z-20"
                      onclick={prevMedia}
                      title="Предыдущее медиа [←]"
                    >
                      <ChevronLeft class="w-5 h-5 stroke-[2.5]" />
                    </button>

                    <button
                      type="button"
                      data-nav-item
                      class="absolute right-3 top-1/2 -translate-y-1/2 p-2.5 rounded-full bg-black/60 hover:bg-black/90 text-white border border-white/10 opacity-0 group-hover/viewer:opacity-100 transition-opacity cursor-pointer active:scale-95 z-20"
                      onclick={nextMedia}
                      title="Следующее медиа [→]"
                    >
                      <ChevronRight class="w-5 h-5 stroke-[2.5]" />
                    </button>
                  {/if}

                  <!-- Fullscreen Button -->
                  <div class="absolute top-3 right-3 flex items-center gap-2 opacity-0 group-hover/viewer:opacity-100 transition-opacity z-20">
                    <button
                      type="button"
                      data-nav-item
                      class="p-2 rounded-sm bg-black/70 hover:bg-black/90 text-white border border-white/15 cursor-pointer transition-colors"
                      onclick={toggleScreenshotFullscreen}
                      title={isScreenshotFullscreen ? "Выйти из полноэкранного режима" : "Во весь экран"}
                    >
                      {#if isScreenshotFullscreen}
                        <Minimize2 class="w-4 h-4" />
                      {:else}
                        <Maximize2 class="w-4 h-4" />
                      {/if}
                    </button>
                  </div>

                  <!-- Bottom-Left Label -->
                  <div class="absolute bottom-3 left-3 px-2.5 py-1 rounded-md bg-[#07080a]/90 border border-white/10 text-[11px] font-semibold text-white/90 pointer-events-none flex items-center gap-2">
                    <ImageIcon class="w-3.5 h-3.5 text-[#9ca3af]" />
                    <span>{activeMedia.name}</span>
                    <span class="text-[#6b7280] font-mono">({activeMediaIndex + 1} / {mediaList.length})</span>
                  </div>
                {/if}
              </div>

              <!-- Unified Thumbnails Strip -->
              {#if mediaList.length > 1}
                <div class="flex items-center gap-2.5 overflow-x-auto pb-1 no-scrollbar">
                  {#each mediaList as item, idx}
                    <button
                      data-nav-item
                      type="button"
                      class="relative flex-shrink-0 w-32 sm:w-36 aspect-video rounded overflow-hidden border transition-all cursor-pointer group/thumb text-left {activeMediaIndex === idx ? 'border-white ring-2 ring-white/25 opacity-100' : 'border-white/10 opacity-60 hover:opacity-100 hover:border-white/40'}"
                      onclick={() => (activeMediaIndex = idx)}
                      title={item.name}
                    >
                      <img
                        src={item.type === 'video' ? item.thumbnail : item.url}
                        alt={item.name}
                        class="w-full h-full object-cover"
                      />

                      {#if item.type === 'video'}
                        <div class="absolute inset-0 bg-black/35 flex items-center justify-center group-hover/thumb:bg-black/15 transition-colors">
                          <div class="w-6 h-6 rounded-full bg-black/80 flex items-center justify-center text-white border border-white/20 shadow-md">
                            <Play class="w-3 h-3 ml-0.5 fill-white" />
                          </div>
                        </div>
                        <span class="absolute bottom-1 inset-x-1 text-[9px] font-bold text-white truncate px-1 bg-black/75 rounded flex items-center gap-1">
                          <Film class="w-2.5 h-2.5 text-amber-400" />
                          <span class="truncate">{item.name}</span>
                        </span>
                      {:else}
                        <span class="absolute bottom-1 right-1 text-[9px] font-mono font-bold text-white/90 px-1 py-0.5 bg-black/75 rounded border border-white/10">
                          {idx + 1}
                        </span>
                      {/if}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}

          <!-- Detailed Overview Text -->
          <div class="space-y-4">
            {#if g.detailedDescription}
              <div class="steam-html-content space-y-3">
                {@html g.detailedDescription}
              </div>
            {:else}
              <p class="text-xs text-[#6b7280]">
                Подробные сведения о релизе и файлах доступны в разделе «Спецификации релиза».
              </p>
            {/if}
          </div>
        </div>
      {/snippet}

      {#snippet genresAndTagsCard()}
        {@const hasGenres = g.genres && g.genres.length > 0}
        {@const hasTags = g.tags && g.tags.length > 0}
        {#if hasGenres || hasTags}
          <div class="p-5 rounded bg-[#07080a] border border-white/[0.06] space-y-4 text-xs">
            {#if hasGenres}
              <div class="space-y-2">
                <div class="text-[11px] font-bold uppercase tracking-wider text-[#8e95a2] flex items-center gap-1.5">
                  <Tag class="w-3.5 h-3.5 text-[#8e95a2]" />
                  <span>Жанры</span>
                </div>
                <div class="flex items-center gap-1.5 flex-wrap">
                  {#each g.genres as genre}
                    <button
                      data-nav-item
                      type="button"
                      class="inline-flex items-center px-2.5 py-1 rounded-sm bg-white/[0.04] hover:bg-white/[0.08] active:bg-white/[0.12] border border-white/[0.06] hover:border-white/15 text-xs font-medium text-[#cbd5e1] hover:text-white transition-colors cursor-pointer"
                      onclick={() => onSelectGenre(genre)}
                      title="Фильтровать по жанру {genre}"
                    >
                      <span>{genre}</span>
                    </button>
                  {/each}
                </div>
              </div>
            {/if}

            {#if hasGenres && hasTags}
              <div class="h-px bg-white/[0.04]"></div>
            {/if}

            {#if hasTags}
              <div class="space-y-2">
                <div class="text-[11px] font-bold uppercase tracking-wider text-[#8e95a2] flex items-center gap-1.5">
                  <Tags class="w-3.5 h-3.5 text-[#8e95a2]" />
                  <span>Популярные метки</span>
                </div>
                <div class="flex items-center gap-1.5 flex-wrap">
                  {#each g.tags as tag}
                    <button
                      data-nav-item
                      type="button"
                      class="inline-flex items-center px-2 py-0.5 rounded-md bg-white/[0.02] hover:bg-white/[0.06] active:bg-white/[0.1] border border-white/[0.04] hover:border-white/10 text-[11px] font-normal text-[#94a3b8] hover:text-white transition-colors cursor-pointer"
                      onclick={() => onSelectTag(tag)}
                      title="Искать игры с меткой {tag}"
                    >
                      <span>{tag}</span>
                    </button>
                  {/each}
                </div>
              </div>
            {/if}
          </div>
        {/if}
      {/snippet}

      {#snippet gameInfoCard()}
        <div class="p-5 rounded bg-[#07080a] border border-white/[0.06] space-y-3 text-xs">
          <h4 class="font-bold uppercase text-[#8e95a2] flex items-center gap-2 text-[11px] tracking-wider">
            <Monitor class="w-3.5 h-3.5 text-[#8e95a2]" />
            <span>Сведения об игре</span>
          </h4>
          <div class="divide-y divide-white/[0.04] text-[#9ca3af]">
            <div class="flex items-center justify-between py-2">
              <span class="text-[#8e95a2]">Платформа</span>
              <span class="text-white font-medium flex items-center gap-1.5">
                <Gamepad2 class="w-3.5 h-3.5 text-[#8e95a2]" />
                <span>PC (Windows)</span>
              </span>
            </div>

            {#if g.releaseDate}
              <div class="flex items-center justify-between py-2">
                <span class="text-[#8e95a2]">Дата выхода</span>
                <span class="text-white font-medium flex items-center gap-1.5">
                  <Calendar class="w-3.5 h-3.5 text-[#8e95a2]" />
                  <span>{g.releaseDate}</span>
                </span>
              </div>
            {/if}

            {#if g.developers && g.developers.length > 0}
              <div class="flex items-center justify-between py-2 gap-3">
                <span class="text-[#8e95a2] flex-shrink-0">Разработчик</span>
                <span class="text-white font-medium truncate max-w-[220px] text-right" title={g.developers.join(', ')}>
                  {g.developers.join(', ')}
                </span>
              </div>
            {/if}

            {#if g.publishers && g.publishers.length > 0}
              <div class="flex items-center justify-between py-2 gap-3">
                <span class="text-[#8e95a2] flex-shrink-0">Издатель</span>
                <span class="text-white font-medium truncate max-w-[220px] text-right" title={g.publishers.join(', ')}>
                  {g.publishers.join(', ')}
                </span>
              </div>
            {/if}

            {#if downloadSourceInfo}
              <div class="flex items-center justify-between py-2 gap-3">
                <span class="text-[#8e95a2] flex-shrink-0">Источник</span>
                <span
                  class="font-medium text-right truncate max-w-[230px] {downloadSourceInfo.type === 'torrent' ? 'text-sky-400' : 'text-emerald-400'}"
                  title={downloadSourceInfo.name}
                >
                  {downloadSourceInfo.name}
                </span>
              </div>
            {/if}

            <div class="flex items-center justify-between py-2 gap-3">
              <span class="text-[#8e95a2] flex-shrink-0">Управление</span>
              <span class="text-white font-medium flex items-center gap-1.5 whitespace-nowrap">
                {#if g.controllerSupport === 'full'}
                  <Gamepad2 class="w-3.5 h-3.5 text-emerald-400" />
                  <span>Геймпад (Полная)</span>
                {:else}
                  <Monitor class="w-3.5 h-3.5 text-[#8e95a2]" />
                  <span>Клавиатура и мышь</span>
                {/if}
              </span>
            </div>

            <div class="flex items-center justify-between py-2 gap-2">
              <span class="text-[#8e95a2] flex-shrink-0 whitespace-nowrap">Steam AppID</span>
              <div class="flex items-center gap-1.5 flex-shrink-0">
                {#if g.steamAppId > 0}
                  <button
                    data-nav-item
                    type="button"
                    class="h-7 px-2.5 rounded-sm bg-sky-500/10 hover:bg-sky-500/20 border border-sky-500/30 text-sky-300 transition-colors cursor-pointer text-xs whitespace-nowrap inline-flex items-center gap-1.5"
                    onclick={handleOpenSteamStore}
                    title="Открыть страницу игры в магазине Steam"
                  >
                    <SteamLogo size={13} weight="bold" class="flex-shrink-0" />
                    <span>В Steam</span>
                    <ExternalLink class="w-3 h-3 text-sky-400/80 flex-shrink-0" />
                  </button>
                {/if}
                <button
                  data-nav-item
                  type="button"
                  class="h-7 px-2.5 rounded-sm bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] hover:border-white/15 text-[#cbd5e1] hover:text-white transition-colors cursor-pointer text-xs whitespace-nowrap inline-flex items-center gap-1.5"
                  onclick={() => openSteamModal(g)}
                  title="Найти в Steam / Изменить метаданные"
                >
                  <span class="font-mono">{g.steamAppId > 0 ? g.steamAppId : (g.steamAppId < 0 ? `SGDB: ${-g.steamAppId}` : 'Привязать')}</span>
                  <Edit3 class="w-3 h-3 text-[#8e95a2] flex-shrink-0" />
                </button>
              </div>
            </div>
          </div>
        </div>
      {/snippet}

      {#snippet requirementsCard()}
        <div class="p-5 rounded bg-[#07080a] border border-white/[0.06] space-y-3.5 text-xs">
          <h4 class="font-bold uppercase text-[#8e95a2] flex items-center gap-2 text-[11px] tracking-wider">
            <Cpu class="w-3.5 h-3.5 text-[#8e95a2]" />
            <span>Системные требования</span>
          </h4>
          <div class="steam-html-content text-xs text-[#9ca3af] leading-relaxed">
            {@html g.pcRequirements}
          </div>
        </div>
      {/snippet}

      {#snippet specsCard()}
        <div class="space-y-5">
          <div class="p-5 rounded bg-[#07080a] border border-white/[0.06] space-y-3 text-xs">
            <h4 class="font-bold uppercase text-[#8e95a2] flex items-center gap-2 text-[11px] tracking-wider">
              <Server class="w-3.5 h-3.5 text-[#8e95a2]" />
              <span>Хранилище репозитория</span>
            </h4>
            <div class="divide-y divide-white/[0.04] text-[#9ca3af]">
              {#if (activeVariant?.rawName || g.rawName)}
                <div class="flex items-center justify-between py-2 gap-3">
                  <span class="text-[#8e95a2] flex-shrink-0">Оригинальный релиз</span>
                  <span class="font-mono text-white truncate max-w-[240px] select-all text-right" title={activeVariant?.rawName || g.rawName}>
                    {activeVariant?.rawName || g.rawName}
                  </span>
                </div>
              {/if}
              <div class="flex items-center justify-between py-2 gap-3">
                <span class="text-[#8e95a2] flex-shrink-0">Путь на сервере</span>
                <span class="font-mono text-white truncate max-w-[240px] select-all text-right" title={g.remotePath}>{g.remotePath}</span>
              </div>
              <div class="flex items-center justify-between py-2">
                <span class="text-[#8e95a2]">Общий размер данных</span>
                <span class="font-bold text-white font-mono">{g.sizeDisplay || '—'}</span>
              </div>
              <div class="flex items-center justify-between py-2">
                <span class="text-[#8e95a2]">Формат релиза</span>
                <span class="text-white">{g.isDirectory ? 'Папка с файлами' : 'Архив'}</span>
              </div>
            </div>
          </div>

          <div class="p-5 rounded bg-[#07080a] border border-white/[0.06] space-y-3 text-xs">
            <h4 class="font-bold uppercase text-[#8e95a2] flex items-center gap-2 text-[11px] tracking-wider">
              <Folder class="w-3.5 h-3.5 text-[#8e95a2]" />
              <span>Локальная конфигурация</span>
            </h4>
            <div class="divide-y divide-white/[0.04] text-[#9ca3af]">
              <div class="flex items-center justify-between py-2 gap-3">
                <span class="text-[#8e95a2] flex-shrink-0">Директория сохранения</span>
                <span class="font-mono text-white truncate max-w-[240px] text-right" title={customDownloadPath}>{customDownloadPath}</span>
              </div>
              <div class="flex items-center justify-between py-2">
                <span class="text-[#8e95a2]">Статус установки</span>
                <span class="font-semibold {pageDetails?.isInstalled ? 'text-emerald-400' : 'text-[#8e95a2]'}">
                  {pageDetails?.isInstalled ? 'Установлено' : 'Не установлено'}
                </span>
              </div>
              <div class="flex items-center justify-between py-2">
                <span class="text-[#8e95a2]">Поддержка контроллера</span>
                <span class="font-medium text-[#cbd5e1]">{g.controllerSupport === 'full' ? 'Полная (XInput/DirectInput)' : 'Клавиатура / Мышь'}</span>
              </div>
            </div>
          </div>
        </div>
      {/snippet}

      {#snippet steamReviewsSection()}
        {#if g.steamAppId && g.steamAppId > 0}
          <div class="space-y-4 pt-2">
            <!-- Section Header -->
            <div class="flex items-center justify-between gap-3 border-b border-white/[0.08] pb-3">
              <div class="flex items-center gap-2.5">
                <ChatText class="w-4 h-4 text-[#66c0f4]" />
                <h3 class="text-sm font-bold text-white uppercase tracking-wider">Отзывы сообщества Steam</h3>
                {#if steamReviewsTotal > 0}
                  <span class="text-xs text-[#8e95a2] font-mono">({formatReviewsCount(steamReviewsTotal)})</span>
                {/if}
              </div>

              <div class="flex items-center gap-2">
                <!-- Language Selector -->
                <div class="inline-flex rounded bg-[#0d1117] border border-white/10 p-0.5 text-[11px]">
                  <button
                    type="button"
                    class="px-2.5 py-1 rounded transition-colors cursor-pointer font-medium {reviewLanguage === 'russian' ? 'bg-white/15 text-white font-bold' : 'text-[#8e95a2] hover:text-white'}"
                    onclick={() => handleLanguageChange('russian')}
                  >
                    Русские
                  </button>
                  <button
                    type="button"
                    class="px-2.5 py-1 rounded transition-colors cursor-pointer font-medium {reviewLanguage === 'all' ? 'bg-white/15 text-white font-bold' : 'text-[#8e95a2] hover:text-white'}"
                    onclick={() => handleLanguageChange('all')}
                  >
                    Все языки
                  </button>
                </div>

                <!-- Open in Steam button -->
                <button
                  data-nav-item
                  type="button"
                  class="hidden sm:inline-flex items-center gap-1.5 text-xs text-[#8e95a2] hover:text-[#66c0f4] px-2.5 py-1 rounded bg-white/[0.03] hover:bg-white/[0.06] border border-white/[0.06] transition-colors cursor-pointer"
                  onclick={handleOpenSteamStore}
                  title="Открыть страницу игры в магазине Steam"
                >
                  <SteamLogo size={14} weight="bold" />
                  <span>В Steam</span>
                  <ExternalLink class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <!-- Reviews Content -->
            {#if isLoadingReviews}
              <!-- Loading skeletons -->
              <div class="space-y-3">
                {#each [1, 2, 3] as _}
                  <div class="p-4 rounded bg-[#0d1117]/80 border border-white/[0.06] space-y-3 animate-pulse">
                    <div class="flex items-center justify-between">
                      <div class="h-5 w-28 bg-white/10 rounded"></div>
                      <div class="h-4 w-20 bg-white/10 rounded"></div>
                    </div>
                    <div class="space-y-1.5">
                      <div class="h-3.5 w-full bg-white/5 rounded"></div>
                      <div class="h-3.5 w-4/5 bg-white/5 rounded"></div>
                      <div class="h-3.5 w-2/3 bg-white/5 rounded"></div>
                    </div>
                  </div>
                {/each}
              </div>
            {:else if steamReviews.length === 0}
              <div class="p-6 rounded bg-[#0d1117]/60 border border-white/[0.06] text-center text-xs text-[#8e95a2] space-y-1.5">
                <p>Отзывов не найдено{reviewLanguage === 'russian' ? ' на русском языке' : ''}.</p>
                {#if reviewLanguage === 'russian'}
                  <button
                    type="button"
                    class="text-sky-400 hover:text-sky-300 hover:underline cursor-pointer"
                    onclick={() => handleLanguageChange('all')}
                  >
                    Попробовать показать отзывы на всех языках
                  </button>
                {/if}
              </div>
            {:else}
              <div class="space-y-3">
                {#each steamReviews as rev (rev.id)}
                  {@const isExpanded = !!expandedReviewIds[rev.id]}
                  {@const isLong = rev.review.length > 340 || rev.review.split('\n').length > 5}
                  
                  <div class="p-4 rounded bg-[#0d1117] border border-white/[0.07] hover:border-white/15 transition-colors space-y-3">
                    <!-- Top Row: Recommendation + Playtime + Date -->
                    <div class="flex flex-wrap items-center justify-between gap-2">
                      <div class="flex flex-wrap items-center gap-2">
                        {#if rev.votedUp}
                          <div class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded bg-[#66c0f4]/10 border border-[#66c0f4]/30 text-[#66c0f4] text-xs font-bold">
                            <ThumbsUp class="w-3.5 h-3.5" weight="fill" />
                            <span>Рекомендую</span>
                          </div>
                        {:else}
                          <div class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded bg-rose-500/10 border border-rose-500/30 text-rose-400 text-xs font-bold">
                            <ThumbsDown class="w-3.5 h-3.5" weight="fill" />
                            <span>Не рекомендую</span>
                          </div>
                        {/if}

                        <span class="text-xs text-[#cbd5e1] font-medium font-mono">
                          {rev.playtimeHours} в игре
                        </span>

                        {#if rev.playtimeAtReview}
                          <span class="text-[11px] text-[#64748b] hidden sm:inline">
                            ({rev.playtimeAtReview} на момент отзыва)
                          </span>
                        {/if}
                      </div>

                      <div class="text-[11px] text-[#64748b]">
                        {formatReviewDate(rev.timestampCreated)}
                      </div>
                    </div>

                    <!-- Review Text (anonymized, formatted BBCode) -->
                    <div class="relative">
                      <div
                        class="text-xs text-[#cbd5e1] leading-relaxed break-words {isLong && !isExpanded ? 'max-h-28 overflow-hidden' : ''}"
                      >
                        {@html formatSteamReviewBBCode(rev.review)}
                      </div>

                      {#if isLong && !isExpanded}
                        <div class="absolute bottom-0 left-0 right-0 h-10 bg-gradient-to-t from-[#0d1117] to-transparent pointer-events-none"></div>
                      {/if}
                    </div>

                    <!-- Bottom Row: Helpful votes count & Expand button -->
                    <div class="flex items-center justify-between gap-2 pt-1 border-t border-white/[0.04] text-[11px] text-[#8e95a2]">
                      <div class="flex items-center gap-3">
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

                      {#if isLong}
                        <button
                          type="button"
                          class="text-xs font-semibold text-[#8e95a2] hover:text-white transition-colors cursor-pointer hover:underline"
                          onclick={() => toggleReviewExpand(rev.id)}
                        >
                          {isExpanded ? 'Свернуть' : 'Читать полностью'}
                        </button>
                      {/if}
                    </div>
                  </div>
                {/each}
              </div>

              <!-- Load More Button -->
              {#if steamReviewsHasMore}
                <div class="pt-2 text-center">
                  <button
                    data-nav-item
                    type="button"
                    disabled={isLoadingMoreReviews}
                    class="px-5 py-2.5 rounded bg-white/[0.04] hover:bg-white/[0.08] border border-white/10 hover:border-white/20 text-xs font-bold text-[#cbd5e1] hover:text-white transition-all cursor-pointer inline-flex items-center gap-2 disabled:opacity-50"
                    onclick={() => loadSteamReviews(false)}
                  >
                    {#if isLoadingMoreReviews}
                      <RefreshCw class="w-3.5 h-3.5 animate-spin text-[#66c0f4]" />
                      <span>Загрузка отзывов...</span>
                    {:else}
                      <ChevronDown class="w-3.5 h-3.5" />
                      <span>Показать ещё отзывы</span>
                    {/if}
                  </button>
                </div>
              {/if}
            {/if}
          </div>
        {/if}
      {/snippet}

      <!-- BOTTOM SECTION: Content Area (Tabs on small screens, 2-column on wide screens) -->
      <div class="space-y-6 pt-4">
        
        {#if hasSteamMetadata}
          <!-- Navigation Tabs: Visible only on smaller screens (<xl) -->
          <div class="xl:hidden w-full border-b border-white/[0.08] flex items-center gap-2 sm:gap-6 overflow-x-auto no-scrollbar">
            <button
              data-nav-item
              class="relative py-3.5 px-1 text-xs font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer flex-shrink-0 {activeTab === 'description' ? 'text-white' : 'text-[#8e95a2] hover:text-[#d1d5db]'}"
              onclick={() => (activeTab = 'description')}
            >
              <span>Описание</span>
              {#if activeTab === 'description'}
                <span
                  class="absolute bottom-0 left-0 right-0 h-[2px] rounded-full transition-all duration-300"
                  style="background-color: var(--game-accent);"
                ></span>
              {/if}
            </button>

            <button
              data-nav-item
              class="relative py-3.5 px-1 text-xs font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer flex-shrink-0 {activeTab === 'info' ? 'text-white' : 'text-[#8e95a2] hover:text-[#d1d5db]'}"
              onclick={() => (activeTab = 'info')}
            >
              <span>Об игре</span>
              {#if activeTab === 'info'}
                <span
                  class="absolute bottom-0 left-0 right-0 h-[2px] rounded-full transition-all duration-300"
                  style="background-color: var(--game-accent);"
                ></span>
              {/if}
            </button>

            {#if g.pcRequirements}
              <button
                data-nav-item
                class="relative py-3.5 px-1 text-xs font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer flex-shrink-0 {activeTab === 'requirements' ? 'text-white' : 'text-[#8e95a2] hover:text-[#d1d5db]'}"
                onclick={() => (activeTab = 'requirements')}
              >
                <span>Системные требования</span>
                {#if activeTab === 'requirements'}
                  <span
                    class="absolute bottom-0 left-0 right-0 h-[2px] rounded-full transition-all duration-300"
                    style="background-color: var(--game-accent);"
                  ></span>
                {/if}
              </button>
            {/if}

            <button
              data-nav-item
              class="relative py-3.5 px-1 text-xs font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer flex-shrink-0 {activeTab === 'specs' ? 'text-white' : 'text-[#8e95a2] hover:text-[#d1d5db]'}"
              onclick={() => (activeTab = 'specs')}
            >
              <span>Спецификации релиза</span>
              {#if activeTab === 'specs'}
                <span
                  class="absolute bottom-0 left-0 right-0 h-[2px] rounded-full transition-all duration-300"
                  style="background-color: var(--game-accent);"
                ></span>
              {/if}
            </button>

            {#if g.steamAppId && g.steamAppId > 0}
              <button
                data-nav-item
                class="relative py-3.5 px-1 text-xs font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer flex-shrink-0 {activeTab === 'reviews' ? 'text-white' : 'text-[#8e95a2] hover:text-[#d1d5db]'}"
                onclick={() => (activeTab = 'reviews')}
              >
                <span>Отзывы</span>
                {#if steamReviewsTotal > 0}
                  <span class="text-[10px] text-[#8e95a2] font-mono ml-1">({steamReviewsTotal})</span>
                {/if}
                {#if activeTab === 'reviews'}
                  <span
                    class="absolute bottom-0 left-0 right-0 h-[2px] rounded-full transition-all duration-300"
                    style="background-color: var(--game-accent);"
                  ></span>
                {/if}
              </button>
            {/if}
          </div>

          <!-- Small screens view (<xl): Tab-switched -->
          <div class="xl:hidden space-y-5">
            {#if activeTab === 'description'}
              {@render mediaAndDescription()}
            {:else if activeTab === 'info'}
              <div class="space-y-5">
                {@render genresAndTagsCard()}
                {@render gameInfoCard()}
              </div>
            {:else if activeTab === 'requirements' && g.pcRequirements}
              {@render requirementsCard()}
            {:else if activeTab === 'specs'}
              {@render specsCard()}
            {:else if activeTab === 'reviews'}
              {@render steamReviewsSection()}
            {/if}
          </div>

          <!-- Large screens view (>=xl): 2-Column Responsive Side-by-Side Layout -->
          <div class="hidden xl:grid xl:grid-cols-12 xl:gap-8 xl:items-start">
            <div class="xl:col-span-7 2xl:col-span-8 space-y-6 min-w-0">
              {@render mediaAndDescription()}
              {@render steamReviewsSection()}
            </div>
            <div class="xl:col-span-5 2xl:col-span-4 space-y-5 min-w-0">
              {@render genresAndTagsCard()}
              {@render gameInfoCard()}
              {#if g.pcRequirements}
                {@render requirementsCard()}
              {/if}
              {@render specsCard()}
            </div>
          </div>

        {:else}
          <!-- RAW / UNENRICHED RELEASES COMPACT VIEW -->
          {@const btih = extractBtih(activeVariant?.magnetUri || g.magnetUri || g.remotePath)}
          {@const isTorrent = (activeVariant?.sourceType || g.sourceType) === 'torrent' || !!(activeVariant?.magnetUri || g.magnetUri || btih)}

          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            
            <!-- CARD 1: Release Specifications -->
            <div class="p-5 rounded bg-[#07080a] border border-white/[0.06] space-y-4 text-xs">
              <h4 class="font-bold uppercase text-[#8e95a2] flex items-center gap-2 text-[11px] tracking-wider">
                <Server class="w-3.5 h-3.5 text-[#8e95a2]" />
                <span>Сведения о релизе</span>
              </h4>

              <div class="divide-y divide-white/[0.04] text-[#9ca3af]">
                
                <!-- Source -->
                <div class="flex items-center justify-between py-2.5">
                  <span class="text-[#8e95a2]">Источник</span>
                  <span class="font-medium text-white flex items-center gap-1.5">
                    <HardDrive class="w-3.5 h-3.5 text-[#8e95a2]" />
                    <span>{downloadSourceInfo?.name || (isTorrent ? 'Торрент' : 'FTP-сервер')}</span>
                  </span>
                </div>

                <!-- Raw Release Name -->
                <div class="py-2.5 space-y-1.5">
                  <div class="flex items-center justify-between">
                    <span class="text-[#8e95a2]">Оригинальный релиз</span>
                    <button
                      type="button"
                      class="text-[11px] text-[#8e95a2] hover:text-white transition-colors cursor-pointer flex items-center gap-1"
                      onclick={() => copyText(activeVariant?.rawName || g.rawName || '', 'Имя релиза скопировано')}
                      title="Скопировать оригинальное название"
                    >
                      <Copy class="w-3 h-3" />
                      <span>Копировать</span>
                    </button>
                  </div>
                  <div class="font-mono text-white text-[11px] bg-black/40 border border-white/5 rounded p-2.5 break-words select-all leading-relaxed">
                    {activeVariant?.rawName || g.rawName || getDisplayTitle(g)}
                  </div>
                </div>

                <!-- Size -->
                <div class="flex items-center justify-between py-2.5">
                  <span class="text-[#8e95a2]">Размер данных</span>
                  <span class="font-bold text-white font-mono">{activeVariant?.sizeDisplay || formatSizeDisplay(g) || '—'}</span>
                </div>

                <!-- Protocol / Hash / Path -->
                {#if isTorrent}
                  <div class="flex items-center justify-between py-2.5">
                    <span class="text-[#8e95a2]">Тип передачи</span>
                    <span class="text-[#cbd5e1] font-mono">BitTorrent (P2P)</span>
                  </div>

                  {#if btih}
                    <div class="flex items-center justify-between py-2.5 gap-3">
                      <span class="text-[#8e95a2] flex-shrink-0">Хэш (BTIH)</span>
                      <div class="flex items-center gap-2 min-w-0">
                        <span class="font-mono text-white text-[11px] truncate select-all" title={btih}>
                          {btih}
                        </span>
                        <button
                          type="button"
                          class="p-1 rounded hover:bg-white/10 text-[#8e95a2] hover:text-white transition-colors flex-shrink-0 cursor-pointer"
                          onclick={() => copyText(activeVariant?.magnetUri || g.magnetUri || `magnet:?xt=urn:btih:${btih}`, 'Magnet-ссылка скопирована')}
                          title="Скопировать magnet-ссылку"
                        >
                          <Copy class="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </div>
                  {/if}
                {:else}
                  <div class="flex items-center justify-between py-2.5 gap-3">
                    <span class="text-[#8e95a2] flex-shrink-0">Путь на сервере</span>
                    <span class="font-mono text-white truncate max-w-[240px] select-all text-right" title={g.remotePath}>{g.remotePath || '—'}</span>
                  </div>
                  <div class="flex items-center justify-between py-2.5">
                    <span class="text-[#8e95a2]">Формат</span>
                    <span class="text-white">{g.isDirectory ? 'Папка с файлами' : 'Архив'}</span>
                  </div>
                {/if}

              </div>
            </div>

            <!-- CARD 2: Save Settings & Steam Binding -->
            <div class="p-5 rounded bg-[#07080a] border border-white/[0.06] space-y-4 text-xs flex flex-col justify-between">
              <div class="space-y-4">
                <h4 class="font-bold uppercase text-[#8e95a2] flex items-center gap-2 text-[11px] tracking-wider">
                  <Folder class="w-3.5 h-3.5 text-[#8e95a2]" />
                  <span>Параметры сохранения и интеграции</span>
                </h4>

                <div class="divide-y divide-white/[0.04] text-[#9ca3af]">
                  
                  <!-- Download Destination -->
                  <div class="flex items-center justify-between py-2.5 gap-3">
                    <span class="text-[#8e95a2] flex-shrink-0">Папка сохранения</span>
                    <div class="flex items-center gap-2 min-w-0 justify-end">
                      <span class="font-mono text-white truncate max-w-[200px]" title={customDownloadPath}>{customDownloadPath}</span>
                      <button
                        type="button"
                        class="text-[var(--game-accent)] hover:underline text-[11px] flex-shrink-0 cursor-pointer"
                        onclick={handleBrowseFolder}
                      >
                        Изменить
                      </button>
                    </div>
                  </div>

                  <!-- Install / Ready Status -->
                  <div class="flex items-center justify-between py-2.5">
                    <span class="text-[#8e95a2]">Статус</span>
                    <span class="font-semibold {pageDetails?.isInstalled ? 'text-emerald-400' : 'text-white'}">
                      {pageDetails?.isInstalled ? 'Установлено' : (isDownloading ? 'Скачивается' : 'Готово к скачиванию')}
                    </span>
                  </div>

                  <!-- Steam Metadata Binding -->
                  <div class="flex items-center justify-between py-2.5 gap-3">
                    <span class="text-[#8e95a2]">Метаданные Steam</span>
                    {#if isEnrichingCurrentGame}
                      <span class="inline-flex items-center gap-1.5 text-[#8e95a2] text-xs font-mono">
                        <RefreshCw class="w-3 h-3 animate-spin text-[#8e95a2]" />
                        <span>Автопоиск...</span>
                      </span>
                    {:else}
                      <button
                        data-nav-item
                        type="button"
                        class="px-3 py-1.5 rounded bg-white/[0.06] hover:bg-white/[0.12] text-xs font-medium text-white border border-white/10 transition-colors flex items-center gap-1.5 cursor-pointer"
                        onclick={() => openSteamModal(g)}
                        title="Найти игру в Steam и привязать обложку, скриншоты и описание"
                      >
                        <Search class="w-3.5 h-3.5 text-[#8e95a2]" />
                        <span>Привязать вручную</span>
                      </button>
                    {/if}
                  </div>

                </div>
              </div>

              <!-- Subtle Info Note -->
              <div class="p-3.5 rounded bg-white/[0.02] border border-white/[0.04] text-[11px] text-[#8e95a2] leading-relaxed">
                <span class="font-medium text-white/80">Интеграция:</span>
                Привязка к Steam автоматически добавит официальное описание, постер, скриншоты, жанры, дату релиза и системные требования.
              </div>

            </div>

          </div>

          <!-- Toast Copy Feedback -->
          {#if copiedTextFeedback}
            <div class="fixed bottom-6 right-6 z-50 px-4 py-2 rounded bg-[#11141c] border border-white/15 text-white text-xs font-medium shadow-2xl flex items-center gap-2">
              <Check class="w-3.5 h-3.5 text-emerald-400" />
              <span>{copiedTextFeedback}</span>
            </div>
          {/if}

          <!-- Toast Launch Error Feedback -->
          {#if launchErrorFeedback}
            <div class="fixed bottom-6 right-6 z-50 px-4 py-2 rounded bg-[#1c1114] border border-rose-500/30 text-rose-200 text-xs font-medium shadow-2xl flex items-center gap-2">
              <X class="w-3.5 h-3.5 text-rose-400 flex-shrink-0" />
              <span>{launchErrorFeedback}</span>
            </div>
          {/if}
        {/if}

      </div>

    </div>

    <!-- STEAM METADATA MATCHER & APPID EDITOR MODAL -->
    {#if isSteamModalOpen}
      <div data-nav-zone="modal" class="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 sm:p-6">
        <div class="bg-[#10131a] border border-white/10 rounded-md w-full max-w-2xl overflow-hidden flex flex-col max-h-[85vh]">
          <!-- Modal Header -->
          <div class="p-5 border-b border-white/[0.08] flex items-center justify-between bg-black/40">
            <div class="space-y-0.5">
              <div class="flex items-center gap-2">
                <Link2 class="w-4 h-4 text-[var(--game-accent)]" />
                <h3 class="text-sm font-bold uppercase text-white tracking-wider">Привязка метаданных Steam</h3>
              </div>
              <p class="text-xs text-[#8e95a2] truncate max-w-md">{getDisplayTitle(g)}</p>
            </div>

            <button
              data-nav-item
              class="text-[#8e95a2] hover:text-white p-1.5 rounded-sm hover:bg-white/5 transition-colors cursor-pointer"
              onclick={() => (isSteamModalOpen = false)}
            >
              <X class="w-5 h-5" />
            </button>
          </div>

          <!-- Modal Body -->
          <div class="p-5 space-y-5 overflow-y-auto flex-1">
            <!-- Search Bar -->
            <div class="space-y-2">
              <span class="block text-[11px] font-bold uppercase tracking-wider text-[#9ca3af]">Поиск в каталоге Steam</span>
              <form
                onsubmit={(e) => {
                  e.preventDefault();
                  performSteamSearch();
                }}
                class="flex items-center gap-2"
              >
                <div class="relative flex-1">
                  <Search class="w-4 h-4 text-[#6b7280] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
                  <input
                    data-nav-item
                    type="text"
                    bind:value={steamSearchTerm}
                    placeholder="Введите название игры..."
                    class="w-full bg-[#090a0d] text-white text-xs rounded pl-9 pr-4 py-2.5 border border-white/[0.08] focus:border-[var(--game-accent)] focus:outline-none"
                  />
                </div>

                <button
                  data-nav-item
                  type="submit"
                  disabled={isSearchingSteam}
                  class="px-5 py-2.5 text-xs font-bold text-black rounded cursor-pointer flex items-center gap-1.5 disabled:opacity-50"
                  style="background-color: var(--game-accent);"
                >
                  {#if isSearchingSteam}
                    <RefreshCw class="w-3.5 h-3.5 animate-spin" />
                    <span>Поиск...</span>
                  {:else}
                    <Search class="w-3.5 h-3.5" />
                    <span>Искать</span>
                  {/if}
                </button>
              </form>
            </div>

            <!-- Candidates List -->
            <div class="space-y-2">
              <div class="flex items-center justify-between text-[11px] font-bold uppercase tracking-wider text-[#9ca3af]">
                <span>Найденные варианты в Steam</span>
                {#if steamCandidates.length > 0}
                  <span>{steamCandidates.length} совпадений</span>
                {/if}
              </div>

              {#if isSearchingSteam}
                <div class="p-8 text-center text-xs text-[#6b7280] flex items-center justify-center gap-2">
                  <RefreshCw class="w-4 h-4 animate-spin text-[var(--game-accent)]" />
                  <span>Поиск подходящих игр в Steam...</span>
                </div>
              {:else if steamCandidates.length === 0}
                <div class="p-6 rounded bg-black/30 border border-white/[0.05] text-center text-xs text-[#6b7280] space-y-1">
                  <p>Ничего не найдено по данному запросу.</p>
                  <p class="text-[11px] text-[#4b5563]">Попробуйте сократить название или указать AppID вручную ниже.</p>
                </div>
              {:else}
                <div class="space-y-2 max-h-56 overflow-y-auto pr-1">
                  {#each steamCandidates as candidate}
                    {@const isCurrent = g.steamAppId === candidate.appId}
                    {@const matchPercent = Math.round(candidate.score * 100)}
                    
                    <div class="p-2.5 rounded bg-black/40 border {isCurrent ? 'border-[var(--game-accent)]' : 'border-white/[0.06] hover:border-white/15'} flex items-center justify-between gap-3 transition-colors">
                      <div class="flex items-center gap-3 min-w-0">
                        {#if candidate.tinyImage}
                          <img src={candidate.tinyImage} alt="" class="w-12 h-6 object-cover rounded flex-shrink-0 bg-black" />
                        {:else}
                          <div class="w-12 h-6 rounded bg-white/5 flex items-center justify-center flex-shrink-0">
                            <Disc class="w-3.5 h-3.5 text-[#6b7280]" />
                          </div>
                        {/if}

                        <div class="min-w-0 space-y-0.5">
                          <div class="flex items-center gap-2">
                            <span class="text-xs font-semibold text-white truncate">{candidate.name}</span>
                            <span
                              class="text-[10px] font-mono font-bold px-1.5 py-0.2 rounded"
                              style="color: var(--game-accent); background-color: color-mix(in srgb, var(--game-accent) 15%, transparent);"
                            >
                              {matchPercent}%
                            </span>
                          </div>
                          <span class="text-[10px] font-mono text-[#64748b]">AppID: {candidate.appId}</span>
                        </div>
                      </div>

                      <button
                        data-nav-item
                        disabled={isSavingSteam || isCurrent}
                        class="px-3.5 py-1.5 text-xs font-bold rounded cursor-pointer flex items-center gap-1 flex-shrink-0 transition-transform active:scale-95 disabled:opacity-50 {isCurrent ? 'bg-white/10 text-[#9ca3af]' : ''}"
                        style={isCurrent ? '' : 'background-color: var(--game-accent); color: var(--game-accent-text);'}
                        onclick={() => handleLinkAppId(candidate.appId)}
                      >
                        {#if isCurrent}
                          <Check class="w-3.5 h-3.5" />
                          <span>Привязано</span>
                        {:else}
                          <Link2 class="w-3.5 h-3.5" />
                          <span>Привязать</span>
                        {/if}
                      </button>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>

            <!-- Direct AppID Manual Input -->
            <div class="pt-3 border-t border-white/[0.06] space-y-2">
              <span class="block text-[11px] font-bold uppercase tracking-wider text-[#9ca3af]">Или укажите Steam AppID напрямую</span>
              <div class="flex items-center gap-2">
                <input
                  data-nav-item
                  type="text"
                  bind:value={directAppIdInput}
                  placeholder="Например, 33230 для Assassin's Creed 2"
                  class="flex-1 bg-[#090a0d] text-white text-xs font-mono rounded px-4 py-2.5 border border-white/[0.08] focus:border-[var(--game-accent)] focus:outline-none"
                />
                <button
                  data-nav-item
                  disabled={isSavingSteam || !directAppIdInput.trim()}
                  class="btn-secondary px-4 py-2.5 text-xs font-bold rounded cursor-pointer disabled:opacity-50"
                  onclick={handleApplyDirectAppId}
                >
                  Применить AppID
                </button>
              </div>
            </div>

          </div>

          <!-- Modal Footer -->
          <div class="p-4 px-5 border-t border-white/[0.08] flex items-center justify-between bg-black/40 text-xs">
            <div>
              {#if g.steamAppId && g.steamAppId !== 0}
                <button
                  data-nav-item
                  disabled={isSavingSteam}
                  class="text-rose-400 hover:text-rose-300 font-semibold flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
                  onclick={handleUnlinkMetadata}
                >
                  <Unlink class="w-3.5 h-3.5" />
                  <span>Отвязать текущие метаданные</span>
                </button>
              {/if}
            </div>

            <button
              data-nav-item
              class="btn-secondary px-5 py-2 rounded text-xs"
              onclick={() => (isSteamModalOpen = false)}
            >
              Закрыть
            </button>
          </div>
        </div>
      </div>
    {/if}
  {/if}
{:else}
    <!-- Empty Placeholder / Loading State -->
    <div class="flex-1 flex flex-col items-center justify-center text-[#5a6170] space-y-3">
      {#if isLoading}
        <!-- Minimal loading state: wordmark + slim progress pulse -->
        <div class="flex-1 flex flex-col items-center justify-center gap-6 select-none">
          <!-- Ducke wordmark / logo placeholder -->
          <div class="flex flex-col items-center gap-5">
            <!-- Ultra-thin animated progress bar -->
            <div class="w-32 h-px bg-white/[0.06] rounded-full overflow-hidden relative">
              <div class="absolute inset-y-0 left-0 w-1/3 bg-white/30 rounded-full animate-[loading-sweep_1.4s_ease-in-out_infinite]"></div>
            </div>
            <!-- Muted status text -->
            <p class="text-[11px] text-[#525a6c] font-mono tracking-wider uppercase">
              {loadingStatusText || 'Загрузка библиотеки...'}
            </p>
          </div>
        </div>
      {:else}
        <Layers class="w-10 h-10 stroke-[1.5]" />
        <p class="text-xs font-medium">Выберите игру из библиотеки слева</p>
      {/if}
    </div>
  {/if}
</div>

<style>
  @property --game-accent {
    syntax: '<color>';
    inherits: true;
    initial-value: #66c0f4;
  }

  .panel-detail {
    transition: --game-accent 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  }
</style>
