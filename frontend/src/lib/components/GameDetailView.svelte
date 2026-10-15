<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { slide } from 'svelte/transition';
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
    Smiley,
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
  import { parseDateToMs } from '../utils/dateUtils';
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
      try { localStorage.setItem('ducke_last_launched_game_id', String(game.id)); } catch {}
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
  let secondaryVariantDropdownTriggerEl = $state<HTMLButtonElement | null>(null);
  let variantDropdownContainerEl = $state<HTMLDivElement | null>(null);
  let variantSearchQuery = $state<string>('');
  let variantSortBy = $state<'default' | 'date' | 'seeds' | 'size'>('default');

  let currentGame = $derived(pageDetails?.game || game);

  let displayedVariants = $derived.by(() => {
    const list = currentGame?.variants ? [...currentGame.variants] : [];
    if (list.length === 0) return [];

    let filtered = list;
    if (variantSearchQuery.trim()) {
      const q = variantSearchQuery.toLowerCase().trim();
      filtered = filtered.filter((v: GameVariant) =>
        (v.rawName && v.rawName.toLowerCase().includes(q)) ||
        (v.torrentSource && v.torrentSource.toLowerCase().includes(q)) ||
        (v.version && v.version.toLowerCase().includes(q))
      );
    }

    if (variantSortBy === 'date') {
      filtered.sort((a: GameVariant, b: GameVariant) => {
        const da = Math.max(parseDateToMs(a.uploadDate), parseDateToMs(a.createdAt), parseDateToMs(a.updatedAt));
        const db = Math.max(parseDateToMs(b.uploadDate), parseDateToMs(b.createdAt), parseDateToMs(b.updatedAt));
        if (da !== db) return db - da;
        return (b.id || 0) - (a.id || 0);
      });
    } else if (variantSortBy === 'seeds') {
      filtered.sort((a: GameVariant, b: GameVariant) => {
        const sa = variantSeeds[a.id]?.seeders ?? -1;
        const sb = variantSeeds[b.id]?.seeders ?? -1;
        return sb - sa;
      });
    } else if (variantSortBy === 'size') {
      filtered.sort((a: GameVariant, b: GameVariant) => (b.sizeBytes || 0) - (a.sizeBytes || 0));
    }
    return filtered;
  });

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
    const hasTorrent = v.sourceType === 'torrent' || !!v.magnetUri || (v.uris && v.uris.some((u: string) => u.startsWith('magnet:')));
    const hasDDL = v.sourceType === 'ddl' || (v.hosters && v.hosters.length > 0) || (v.uris && v.uris.some((u: string) => u.startsWith('http://') || u.startsWith('https://')));
    const rawSrc = (v.torrentSource || '').trim();
    const resolvedName = formatSourceName(rawSrc);
    if (hasTorrent) {
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
        hosters: v.hosters || [],
        badgeClass: 'bg-white/[0.04] border-white/10 text-[#cbd5e1]'
      };
    }
    if (hasDDL) {
      const hostersList = v.hosters && v.hosters.length > 0 ? v.hosters.join(', ') : 'Direct';
      return {
        type: 'ddl',
        name: resolvedName ? `${resolvedName} (${hostersList})` : `Файлообменники (${hostersList})`,
        shortName: 'DDL',
        hosters: v.hosters || [],
        badgeClass: 'bg-white/[0.04] border-white/10 text-[#cbd5e1]'
      };
    }
    return {
      type: 'ftp',
      name: 'FTP-сервер',
      shortName: 'FTP',
      hosters: [],
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
      if (
        !variantDropdownContainerEl?.contains(target) &&
        !variantDropdownTriggerEl?.contains(target) &&
        !secondaryVariantDropdownTriggerEl?.contains(target)
      ) {
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

  function getHeaderBannerUrl(g: GameEntity | null): string {
    if (!g) return '';
    if (g.steamAppId && g.steamAppId > 0) {
      const header = `https://shared.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/header.jpg`;
      if (!imageLoadFailed[header]) return header;
    }
    if (g.headerImage && !imageLoadFailed[g.headerImage]) return g.headerImage;
    if (pageDetails?.bannerUrl && !imageLoadFailed[pageDetails.bannerUrl]) return pageDetails.bannerUrl;
    if (g.capsuleImage && !imageLoadFailed[g.capsuleImage]) return g.capsuleImage;
    return getPrimaryCoverUrl(g);
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
    if (!count || count <= 0) return '0 обзоров';
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
    return `${count.toLocaleString('ru-RU')} ${word}`;
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
      try { localStorage.setItem('ducke_last_launched_game_id', String(game.id)); } catch {}
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
          } else if (event.status === 'cancelled' || event.status === 'failed') {
            pageDetails.isInstalled = false;
          }
        }
      }
    });

    const unsubDownloadsUpdated = EventsOn('downloads:updated', () => {
      if (!isMounted) return;
      const curId = game?.id;
      if (curId) {
        pageDetailsCache.delete(curId);
        fetchGamePageDetails(curId, switchSequence);
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
      if (typeof unsubDownloadsUpdated === 'function') unsubDownloadsUpdated();
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
    (!!pageDetails?.isInstalled && pageDetails?.downloadStatus !== 'cancelled' && pageDetails?.downloadStatus !== 'failed' && pageDetails?.downloadStatus !== 'downloading' && pageDetails?.downloadStatus !== 'queued' && pageDetails?.downloadStatus !== 'scanning' && pageDetails?.downloadStatus !== 'paused') ||
    pageDetails?.downloadStatus === 'completed'
  );

  let isGameInFavorites = $derived(
    !!(favoriteItem || effectiveFavoriteStatus)
  );
</script>

<div
  bind:this={scrollContainer}
  class="flex-1 flex flex-col h-full overflow-y-auto relative select-none bg-gradient-to-b from-[#0F1218] to-[#0B0F1E] panel-detail"
  style="--game-accent: {dynamicAccentColor}; --game-accent-text: {dynamicAccentTextColor};"
>
  {#if game}
    {#if isSwitching}
      <!-- Minimal Skeleton: hairline blocks, single shimmer sweep, no decorative cards -->
      <div class="relative z-10 w-full max-w-[1280px] mx-auto flex flex-col flex-1 p-4 sm:p-6 lg:p-8 space-y-5 pb-20 select-none" aria-hidden="true">

        <!-- Showcase island skeleton -->
        <div class="w-full space-y-4 mb-6">
          <div class="sk-line h-3.5 w-48"></div>
          <div class="sk-line h-8 w-80"></div>

          <div class="grid grid-cols-12 gap-3 sm:gap-4 lg:gap-5 items-stretch">
            <div class="col-span-8 2xl:col-span-8 space-y-3 flex flex-col justify-between">
              <div class="sk-block w-full flex-1 min-h-[220px] rounded-[18px]"></div>
              <div class="flex gap-2 flex-shrink-0">
                {#each [1,2,3,4] as _}
                  <div class="sk-block flex-shrink-0 w-28 aspect-video rounded-[12px]"></div>
                {/each}
              </div>
            </div>

            <div class="col-span-4 2xl:col-span-4 space-y-3.5">
              <div class="sk-block w-full aspect-[460/215] rounded-[16px]"></div>
              <div class="space-y-2">
                <div class="sk-line h-3.5 w-full"></div>
                <div class="sk-line h-3.5 w-5/6"></div>
                <div class="sk-line h-3.5 w-3/4"></div>
              </div>
              <div class="space-y-2 pt-2">
                <div class="sk-line h-3 w-1/2"></div>
                <div class="sk-line h-3 w-2/3"></div>
                <div class="sk-line h-3 w-1/3"></div>
              </div>
            </div>
          </div>

          <div class="sk-block h-16 w-full rounded-[16px]"></div>
        </div>

      </div>

    {:else}
      {@const g = pageDetails?.game || game}
      {@const bgUrl = getHeroBackgroundUrl(g)}
      {@const coverUrl = getPrimaryCoverUrl(g)}
      {@const hasCover = !!(coverUrl && !imageLoadFailed[coverUrl])}
      {@const logoUrl = pageDetails?.logoUrl}
      {@const hasLogo = !!(logoUrl && !imageLoadFailed[logoUrl] && !isHorizontalAsset(logoUrl))}
      {@const headerBannerUrl = getHeaderBannerUrl(g)}

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
        <div class="absolute inset-0 bg-gradient-to-b from-[#0F1218]/30 via-[#0F1218]/65 to-[#0B0F1E]"></div>
        <div class="absolute inset-0 bg-gradient-to-r from-[#0F1218]/75 via-transparent to-[#0F1218]/75"></div>
      </div>
    {/if}

    <!-- Main Stage Container -->
    <div class="relative z-10 w-full max-w-[1280px] mx-auto flex flex-col flex-1 p-4 sm:p-6 lg:p-8 space-y-5 pb-20">
      
      <!-- STEAM STORE TOP SHOWCASE BLOCK -->
      <div class="w-full space-y-4">
        
        <!-- Breadcrumbs & Quick Steam Link -->
        <div class="flex items-center justify-between gap-4 text-xs">
          <div class="flex items-center gap-1.5 text-[#8e95a2] truncate">
            <button
              type="button"
              class="hover:text-white cursor-pointer transition-colors"
              onclick={() => onSelectGenre('')}
            >
              Все игры
            </button>
            <span class="text-white/20 select-none">›</span>
            {#if g.genres && g.genres[0]}
              <button
                type="button"
                class="hover:text-white cursor-pointer transition-colors"
                onclick={() => onSelectGenre(g.genres[0])}
              >
                {g.genres[0]}
              </button>
              <span class="text-white/20 select-none">›</span>
            {/if}
            <span class="text-white font-medium truncate">{getDisplayTitle(g)}</span>
          </div>

          {#if g.steamAppId && g.steamAppId > 0}
            <button
              data-nav-item
              type="button"
              class="px-3 py-1.5 rounded-[10px] bg-white/[0.04] hover:bg-white/[0.08] active:bg-white/[0.12] text-xs font-medium text-[#cbd5e1] hover:text-white transition-colors cursor-pointer inline-flex items-center gap-1.5 flex-shrink-0"
              onclick={handleOpenSteamStore}
              title="Открыть страницу игры в магазине Steam"
            >
              <SteamLogo size={14} weight="bold" class="text-[#66c0f4]" />
              <span>В Steam</span>
              <ExternalLink class="w-3 h-3 text-[#8e95a2]" />
            </button>
          {/if}
        </div>

        <!-- Clean Normalized Game Title -->
        <div>
          <h1 class="text-2xl sm:text-3xl lg:text-4xl font-extrabold text-white tracking-tight leading-tight">
            {getDisplayTitle(g)}
          </h1>
        </div>

        <!-- 2-Column Showcase Grid: Left Media Player & Right Metadata Sidebar -->
        <div class="grid grid-cols-12 gap-3 sm:gap-4 lg:gap-5 items-stretch">
          
          <!-- LEFT COLUMN: Media Player & Thumbnails (~67%) -->
          <div class="col-span-8 2xl:col-span-8 flex flex-col justify-between min-w-0 space-y-3">
            <!-- Viewport -->
            <div
              bind:this={screenshotViewport}
              class="relative w-full flex-1 min-h-[220px] rounded-[18px] overflow-hidden bg-black/60 group/viewer flex items-center justify-center select-none"
            >
              {#if activeMedia?.type === 'video'}
                <VideoPlayer
                  hls={activeMedia.hls}
                  mp4={activeMedia.mp4}
                  webm={activeMedia.webm}
                  poster={activeMedia.thumbnail}
                  title={activeMedia.name}
                  autoplay={false}
                  controls={true}
                />
              {:else if activeMedia?.type === 'image'}
                <img
                  src={activeMedia.url}
                  alt={activeMedia.name}
                  class="w-full h-full object-contain cursor-pointer select-none"
                  ondblclick={toggleScreenshotFullscreen}
                />

                <!-- Prev / Next Navigation Arrows -->
                {#if mediaList.length > 1}
                  <button
                    type="button"
                    data-nav-item
                    class="absolute left-3 top-1/2 -translate-y-1/2 p-2.5 rounded-full bg-black/70 hover:bg-black/90 text-white opacity-0 group-hover/viewer:opacity-100 transition-opacity cursor-pointer active:scale-95 z-20"
                    onclick={prevMedia}
                    title="Предыдущее медиа [←]"
                  >
                    <ChevronLeft class="w-5 h-5 stroke-[2.5]" />
                  </button>

                  <button
                    type="button"
                    data-nav-item
                    class="absolute right-3 top-1/2 -translate-y-1/2 p-2.5 rounded-full bg-black/70 hover:bg-black/90 text-white opacity-0 group-hover/viewer:opacity-100 transition-opacity cursor-pointer active:scale-95 z-20"
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
                    class="p-2 rounded-[10px] bg-black/70 hover:bg-black/90 text-white cursor-pointer transition-colors"
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
                <div class="absolute bottom-3 left-3 px-3 py-1.5 rounded-[10px] bg-black/80 text-[11px] font-semibold text-white/90 pointer-events-none flex items-center gap-2">
                  <ImageIcon class="w-3.5 h-3.5 text-[#9ca3af]" />
                  <span>{activeMedia.name}</span>
                  <span class="text-[#6b7280] font-mono">({activeMediaIndex + 1} / {mediaList.length})</span>
                </div>
              {:else if headerBannerUrl}
                <img
                  src={headerBannerUrl}
                  alt={getDisplayTitle(g)}
                  class="w-full h-full object-cover"
                />
              {:else if hasCover}
                <img
                  src={coverUrl}
                  alt={getDisplayTitle(g)}
                  class="w-full h-full object-cover"
                />
              {:else}
                <div class="w-full h-full flex flex-col items-center justify-center p-6 text-center space-y-3">
                  <div class="w-14 h-14 rounded-[14px] bg-white/[0.03] flex items-center justify-center text-[#8e95a2]">
                    <Disc class="w-7 h-7 stroke-[1.5] text-[#8e95a2]" />
                  </div>
                  <div class="text-xs text-[#8e95a2]">
                    Нет медиаматериалов
                  </div>
                </div>
              {/if}
            </div>

            <!-- Horizontal Thumbnail Strip -->
            {#if mediaList.length > 1}
              <div class="flex items-center gap-2 overflow-x-auto p-1.5 -m-1.5 no-scrollbar flex-shrink-0">
                {#each mediaList as item, idx}
                  <button
                    data-nav-item
                    type="button"
                    class="relative flex-shrink-0 w-28 sm:w-32 aspect-video rounded-[12px] overflow-hidden transition-all cursor-pointer group/thumb text-left {activeMediaIndex === idx ? 'opacity-100 bg-white/10' : 'opacity-50 hover:opacity-90'}"
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
                        <div class="w-5 h-5 rounded-full bg-black/80 flex items-center justify-center text-white">
                          <Play class="w-2.5 h-2.5 ml-0.5 fill-white" />
                        </div>
                      </div>
                      <span class="absolute bottom-1.5 inset-x-1.5 text-[9px] font-bold text-white truncate px-1 bg-black/80 rounded flex items-center gap-1 z-10">
                        <Film class="w-2.5 h-2.5 text-amber-400" />
                        <span class="truncate">{item.name}</span>
                      </span>
                    {:else}
                      <span class="absolute bottom-1.5 right-1.5 text-[9px] font-mono font-bold text-white/90 px-1 py-0.5 bg-black/80 rounded z-10">
                        {idx + 1}
                      </span>
                    {/if}

                    <!-- Active outline border (inner overlay to guarantee no overflow clipping on any resolution/DPI) -->
                    {#if activeMediaIndex === idx}
                      <div class="absolute inset-0 rounded-[12px] border-2 border-white pointer-events-none z-20"></div>
                    {/if}
                  </button>
                {/each}
              </div>
            {/if}

          </div>

          <!-- RIGHT COLUMN: Capsule + Synopsis + Specs + Tags (~33%) -->
          <div class="col-span-4 2xl:col-span-4 flex flex-col gap-3 sm:gap-3.5 text-xs min-w-0">
            
            <!-- Capsule / Header Banner (Steam Landscape 460x215) -->
            {#if headerBannerUrl && mediaList.length > 0}
              <div class="w-full aspect-[460/215] rounded-[16px] overflow-hidden bg-black/40 flex-shrink-0">
                <img
                  src={headerBannerUrl}
                  alt={getDisplayTitle(g)}
                  class="w-full h-full object-cover"
                  onerror={() => {
                    imageLoadFailed[headerBannerUrl] = true;
                  }}
                />
              </div>
            {/if}

            <!-- Short Synopsis -->
            {#if g.shortDescription}
              <div class="text-xs text-[#cbd5e1] leading-relaxed font-light line-clamp-3 sm:line-clamp-4">
                {@html g.shortDescription}
              </div>
            {/if}

            <!-- Metadata Specs Table (Steam Store Style) -->
            <div class="space-y-1.5 pt-2 border-t border-white/[0.04]">
              
              <!-- Steam Community Reviews -->
              {#if (g.totalReviews && g.totalReviews > 0) || (g.reviewPercent && g.reviewPercent > 0)}
                {@const isPositive = (g.reviewPercent || 0) >= 70}
                {@const isMixed = (g.reviewPercent || 0) >= 40 && (g.reviewPercent || 0) < 70}
                <div class="flex items-baseline justify-between gap-2 py-0.5">
                  <span class="text-[11px] uppercase tracking-wider text-[#8e95a2] flex-shrink-0">Все обзоры:</span>
                  <span class="text-right font-medium min-w-0 {isPositive ? 'text-[#66c0f4]' : isMixed ? 'text-amber-400' : 'text-red-400'}">
                    {g.reviewScoreDesc || (isPositive ? 'Положительные' : isMixed ? 'Смешанные' : 'Отрицательные')}
                    {#if g.totalReviews}
                      <span class="text-[11px] font-normal text-[#8e95a2] ml-1">({formatReviewsCount(g.totalReviews)})</span>
                    {/if}
                  </span>
                </div>
              {:else if g.metacriticScore && g.metacriticScore > 0}
                <div class="flex items-baseline justify-between gap-2 py-0.5">
                  <span class="text-[11px] uppercase tracking-wider text-[#8e95a2] flex-shrink-0">Metacritic:</span>
                  <span class="text-right font-bold text-amber-400">{g.metacriticScore} / 100</span>
                </div>
              {/if}

              <!-- Release Date -->
              {#if g.releaseDate}
                <div class="flex items-baseline justify-between gap-2 py-0.5">
                  <span class="text-[11px] uppercase tracking-wider text-[#8e95a2] flex-shrink-0">Дата выхода:</span>
                  <span class="text-right text-white font-medium">{g.releaseDate}</span>
                </div>
              {/if}

              <!-- Developer -->
              {#if g.developers && g.developers.length > 0}
                <div class="flex items-baseline justify-between gap-2 py-0.5">
                  <span class="text-[11px] uppercase tracking-wider text-[#8e95a2] flex-shrink-0">Разработчик:</span>
                  <span class="text-right text-[#66c0f4] font-medium truncate max-w-[170px]" title={g.developers.join(', ')}>
                    {g.developers.join(', ')}
                  </span>
                </div>
              {/if}

              <!-- Publisher -->
              {#if g.publishers && g.publishers.length > 0}
                <div class="flex items-baseline justify-between gap-2 py-0.5">
                  <span class="text-[11px] uppercase tracking-wider text-[#8e95a2] flex-shrink-0">Издатель:</span>
                  <span class="text-right text-[#66c0f4] font-medium truncate max-w-[170px]" title={g.publishers.join(', ')}>
                    {g.publishers.join(', ')}
                  </span>
                </div>
              {/if}

              <!-- Source / Size -->
              {#if downloadSourceInfo || formatSizeDisplay(g)}
                <div class="flex items-baseline justify-between gap-2 py-0.5">
                  <span class="text-[11px] uppercase tracking-wider text-[#8e95a2] flex-shrink-0">Источник / Размер:</span>
                  <span class="text-right font-mono text-[#8e95a2]">
                    {#if downloadSourceInfo}{downloadSourceInfo.shortName}{/if}
                    {#if downloadSourceInfo && formatSizeDisplay(g)} • {/if}
                    {#if formatSizeDisplay(g)}<span class="text-white font-bold">{formatSizeDisplay(g)}</span>{/if}
                  </span>
                </div>
              {/if}
            </div>

            <!-- Popular User Tags -->
            {#if g.tags && g.tags.length > 0}
              <div class="space-y-1 pt-2 border-t border-white/[0.04]">
                <div class="text-[11px] text-[#8e95a2]">
                  Популярные метки:
                </div>
                <div class="flex items-center gap-1.5 flex-wrap">
                  {#each g.tags.slice(0, 4) as tag}
                    <button
                      data-nav-item
                      type="button"
                      class="px-2 py-0.5 rounded-[8px] bg-white/[0.04] hover:bg-white/[0.08] active:bg-white/[0.12] text-[11px] font-normal text-[#cbd5e1] hover:text-white transition-colors cursor-pointer"
                      onclick={() => onSelectTag(tag)}
                      title="Искать игры с меткой {tag}"
                    >
                      {tag}
                    </button>
                  {/each}
                </div>
              </div>
            {/if}

          </div>

        </div>

        <!-- ACTION DECK: Smart Download / Play / Folder Controls (FULL WIDTH) -->
        <div class="w-full p-4 sm:p-5 rounded-[16px] bg-white/[0.03] space-y-3.5">
              
              {#snippet favoriteButton()}
                <div class="relative inline-flex items-center">
                  <button
                    bind:this={favoriteDropdownTriggerEl}
                    data-nav-item
                    type="button"
                    class="h-11 inline-flex items-center gap-2 text-xs px-4 rounded-[14px] transition-all cursor-pointer bg-white/[0.04] hover:bg-white/[0.08] text-[#cbd5e1] hover:text-white"
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
                      class="absolute left-0 top-full mt-2 z-50 min-w-[200px] rounded-[16px] bg-[#0d1117] border border-white/10 shadow-2xl p-2 space-y-1 text-xs"
                    >
                      <div class="px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider text-[#6b7280]">
                        Статус в избранном
                      </div>
                      <button
                        type="button"
                        class="w-full text-left flex items-center gap-2 px-3 py-2 rounded-[10px] transition-all cursor-pointer {effectiveFavoriteStatus === 'planned' ? 'bg-sky-500/20 text-sky-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
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
                        class="w-full text-left flex items-center gap-2 px-3 py-2 rounded-[10px] transition-all cursor-pointer {effectiveFavoriteStatus === 'playing' ? 'bg-amber-500/20 text-amber-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
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
                        class="w-full text-left flex items-center gap-2 px-3 py-2 rounded-[10px] transition-all cursor-pointer {effectiveFavoriteStatus === 'completed' ? 'bg-emerald-500/20 text-emerald-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
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
                          class="w-full text-left flex items-center gap-2 px-3 py-2 rounded-[10px] text-rose-400 hover:bg-rose-500/10 transition-all cursor-pointer"
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
                    class="h-11 w-11 flex-shrink-0 flex items-center justify-center rounded-[14px] transition-all cursor-pointer bg-white/[0.04] hover:bg-white/[0.08] text-[#8e95a2] hover:text-white"
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
                    class="h-11 w-11 flex-shrink-0 flex items-center justify-center rounded-[14px] transition-all cursor-pointer bg-white/[0.04] hover:bg-white/[0.08] text-[#8e95a2] hover:text-white"
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
                    class="h-11 px-4 flex items-center justify-center gap-2 rounded-[14px] transition-all cursor-pointer text-xs font-medium {isGameInSteam ? 'bg-sky-500/15 text-sky-300 hover:bg-sky-500/25' : 'bg-white/[0.04] hover:bg-white/[0.08] text-[#8e95a2] hover:text-white'}"
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

              {#if isDownloadCompleted || pageDetails?.isInstalled}

                <!-- ALREADY INSTALLED / DOWNLOAD COMPLETED STATE: PLAY + GEAR + FOLDER + FAVORITE -->
                <div class="space-y-3">
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div class="flex flex-wrap items-center gap-2.5">
                      <button
                        data-nav-item
                        class="h-11 px-8 text-sm font-black rounded-[14px] flex items-center justify-center gap-2.5 cursor-pointer active:scale-95 transition-all hover:brightness-110 uppercase tracking-wider"
                        style="background-color: var(--game-accent); color: var(--game-accent-text);"
                        onclick={handlePlayButtonClick}
                      >
                        <Play class="w-4 h-4 fill-current stroke-[2]" />
                        <span>ИГРАТЬ</span>
                      </button>

                      {@render gearButton()}
                      {@render folderButton()}
                      <button
                        data-nav-item
                        type="button"
                        class="h-11 px-3.5 rounded-[14px] bg-white/[0.04] hover:bg-white/[0.08] text-[#8e95a2] hover:text-white border border-white/[0.06] hover:border-white/15 flex items-center justify-center transition-all cursor-pointer"
                        onclick={() => onStartDownload(selectedVariantId || g.id, customDownloadPath)}
                        title="Скачать заново / переустановить"
                      >
                        <RefreshCw class="w-4 h-4" />
                      </button>
                      {@render steamButton()}
                      {@render favoriteButton()}
                    </div>

                    <div class="flex items-center gap-2 text-xs font-mono">
                      <span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-[8px] bg-emerald-500/15 text-emerald-400 font-semibold">
                        <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                        <span>Установлено</span>
                      </span>
                      {#if formatSizeDisplay(g)}
                        <span class="text-[#8e95a2]">{formatSizeDisplay(g)}</span>
                      {/if}
                    </div>
                  </div>

                  <!-- Metadata & Folder Path Row -->
                  <div class="pt-3 border-t border-white/[0.04] flex flex-wrap items-center justify-between gap-3 text-xs text-[#8e95a2]">
                    {#if pageDetails?.localPath}
                      <div class="flex items-center gap-2 min-w-0">
                        <span class="text-[#64748b] text-[11px] uppercase tracking-wider font-semibold flex-shrink-0">Папка:</span>
                        <button
                          type="button"
                          class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-[8px] bg-white/[0.03] hover:bg-white/[0.07] font-mono text-xs text-[#cbd5e1] hover:text-white transition-colors cursor-pointer truncate max-w-xs sm:max-w-md group"
                          onclick={handleOpenFolder}
                          title="Нажмите, чтобы открыть папку в проводнике"
                        >
                          <Folder class="w-3.5 h-3.5 text-[#8e95a2] flex-shrink-0 group-hover:text-white transition-colors" />
                          <span class="truncate">{pageDetails.localPath}</span>
                          <span class="text-[10px] text-[#64748b] group-hover:text-sky-400 transition-colors ml-1 flex-shrink-0">Открыть</span>
                        </button>
                      </div>
                    {/if}

                    {#if (activeVariant?.rawName || g.rawName)}
                      <div class="inline-flex items-center gap-1.5 font-mono text-xs text-[#64748b] ml-auto">
                        <Disc class="w-3.5 h-3.5 text-[#64748b] flex-shrink-0" />
                        <span class="truncate max-w-[240px] sm:max-w-md" title={activeVariant?.rawName || g.rawName}>
                          {activeVariant?.rawName || g.rawName}
                        </span>
                      </div>
                    {/if}
                  </div>
                </div>

              {:else if isDownloading}
                <!-- ACTIVE DOWNLOADING / QUEUED STATE -->
                {@const prog = activeGameDownload || pageDetails?.downloadProgress}
                {@const dlStatus = activeGameDownload?.status || pageDetails?.downloadStatus}
                <div class="space-y-3">
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div class="flex items-center gap-2 text-sm font-bold text-white">
                      <Download class="w-4 h-4 text-[var(--game-accent)] animate-bounce" />
                      <span>{dlStatus === 'paused' ? 'Приостановлено' : (dlStatus === 'queued' ? 'В очереди загрузки...' : (dlStatus === 'scanning' ? 'Получение метаданных торрента...' : 'Скачивается...'))}</span>
                      <span class="font-mono text-base text-[var(--game-accent)] ml-1">{Math.round(prog?.progressPercent || 0)}%</span>
                    </div>

                    <div class="flex items-center gap-3">
                      {#if prog?.speedDisplay}
                        <div class="text-xs font-mono text-[#8e95a2]">
                          <span class="text-white font-semibold">{prog.speedDisplay}</span>
                          {#if prog.etaDisplay}
                            <span class="text-[#64748b] ml-1.5">({prog.etaDisplay})</span>
                          {/if}
                        </div>
                      {/if}
                      {@render favoriteButton()}
                    </div>
                  </div>

                  <!-- Progress Bar -->
                  <div class="w-full h-2 rounded-none bg-black/60 overflow-hidden">
                    <div
                      class="h-full rounded-none transition-all duration-300"
                      style="width: {Math.max(2, prog?.progressPercent || 0)}%; background-color: var(--game-accent);"
                    ></div>
                  </div>

                  <!-- Bottom Row: Path & Release Name -->
                  <div class="pt-3 border-t border-white/[0.04] flex flex-wrap items-center justify-between gap-3 text-xs text-[#8e95a2]">
                    <div class="flex items-center gap-2 min-w-0">
                      <span class="text-[#64748b] text-[11px] uppercase tracking-wider font-semibold flex-shrink-0">Папка:</span>
                      <span class="font-mono text-xs text-[#cbd5e1] truncate max-w-xs sm:max-w-md">{customDownloadPath}</span>
                    </div>

                    {#if (activeVariant?.rawName || g.rawName)}
                      <div class="inline-flex items-center gap-1.5 font-mono text-xs text-[#64748b] ml-auto">
                        <Disc class="w-3.5 h-3.5 text-[#64748b] flex-shrink-0" />
                        <span class="truncate max-w-[240px] sm:max-w-md" title={activeVariant?.rawName || g.rawName}>
                          {activeVariant?.rawName || g.rawName}
                        </span>
                      </div>
                    {/if}
                  </div>
                </div>

              {:else}
                <!-- NOT INSTALLED / READY TO DOWNLOAD STATE -->
                <div class="space-y-3">
                  <!-- Top Row: Actions on Left, Source & Seeds on Right -->
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div class="flex flex-wrap items-center gap-2.5">
                      {#if hasCustomExe}
                        <!-- ИГРАТЬ via custom exe -->
                        <button
                          data-nav-item
                          class="h-11 px-8 text-xs sm:text-sm font-black rounded-[14px] flex items-center gap-2.5 cursor-pointer transition-all hover:brightness-110 active:scale-[0.98] uppercase tracking-wider"
                          style="background-color: var(--game-accent); color: var(--game-accent-text);"
                          onclick={handleLaunchWithCustom}
                        >
                          <Play class="w-4 h-4 fill-current stroke-[2]" />
                          <span>ИГРАТЬ</span>
                        </button>
                        {@render gearButton()}
                        {@render folderButton()}
                        {@render steamButton()}
                        {@render favoriteButton()}
                      {:else}
                        <!-- Unified Split Download Button (Squircle 14px) -->
                        <div class="relative inline-flex items-stretch rounded-[14px] overflow-visible {isVariantDropdownOpen ? 'z-30' : ''}">
                          <button
                            data-nav-item
                            class="h-11 px-6 sm:px-7 text-xs sm:text-sm font-black flex items-center gap-2.5 cursor-pointer transition-all hover:brightness-110 active:scale-[0.98] uppercase tracking-wider {g.variants && g.variants.length > 1 ? 'rounded-l-[14px]' : 'rounded-[14px]'}"
                            style="background-color: var(--game-accent); color: var(--game-accent-text);"
                            onclick={() => onStartDownload(selectedVariantId || g.id, customDownloadPath)}
                          >
                            <Download class="w-4 h-4 stroke-[2.5]" />
                            <span>СКАЧАТЬ</span>
                            {#if downloadSourceInfo?.type === 'torrent'}
                              <span class="opacity-75 text-[10px] tracking-normal font-sans font-bold px-1.5 py-0.5 rounded bg-black/20">ТОРРЕНТ</span>
                            {:else if downloadSourceInfo?.type === 'ddl'}
                              <span class="opacity-75 text-[10px] tracking-normal font-sans font-bold px-1.5 py-0.5 rounded bg-black/20">DDL</span>
                            {/if}
                            <span class="opacity-35 font-normal">|</span>
                            <span class="font-mono text-xs font-bold tracking-normal">{activeVariant?.sizeDisplay || formatSizeDisplay(g)}</span>
                          </button>

                          {#if g.variants && g.variants.length > 1}
                            <button
                              bind:this={variantDropdownTriggerEl}
                              data-nav-item
                              type="button"
                              class="h-11 px-3 flex items-center justify-center border-l border-black/20 hover:brightness-110 active:scale-95 cursor-pointer transition-all rounded-r-[14px]"
                              style="background-color: var(--game-accent); color: var(--game-accent-text);"
                              onclick={(e) => {
                                e.stopPropagation();
                                isVariantDropdownOpen = !isVariantDropdownOpen;
                                if (isVariantDropdownOpen) {
                                  variantSearchQuery = '';
                                }
                              }}
                              title="Выбрать версию ({g.variants.length} доступно)"
                            >
                              <ChevronDown class="w-4 h-4 stroke-[2.5] transition-transform duration-200 {isVariantDropdownOpen ? 'rotate-180' : ''}" />
                            </button>
                          {/if}
                        </div>

                        {@render steamButton()}
                        {@render favoriteButton()}
                        {@render gearButton()}
                      {/if}
                    </div>

                    <!-- Right Side: Source & Seeds Badge -->
                    <div class="flex items-center gap-2.5 text-xs font-mono text-[#8e95a2]">
                      {#if downloadSourceInfo}
                        {#if downloadSourceInfo.type === 'torrent'}
                          <div class="inline-flex items-center gap-1.5">
                            <span class="px-2 py-0.5 rounded-[4px] bg-white/[0.04] border border-white/[0.06] text-[11px] text-[#cbd5e1] font-semibold">
                              {downloadSourceInfo.shortName}
                            </span>
                            {#if downloadSourceInfo.hosters && downloadSourceInfo.hosters.length > 0}
                              <span class="text-white/20 select-none">•</span>
                              <span class="text-[11px] text-[#8e95a2]">Зеркала: {downloadSourceInfo.hosters.join(', ')}</span>
                            {/if}
                          </div>
                        {:else if downloadSourceInfo.type === 'ddl' && downloadSourceInfo.hosters && downloadSourceInfo.hosters.length > 0}
                          <div class="inline-flex items-center gap-1.5 flex-wrap">
                            <span class="text-[#8e95a2] font-semibold">DDL</span>
                            {#each downloadSourceInfo.hosters as hoster}
                              <span class="px-1.5 py-0.5 rounded-[4px] bg-white/[0.04] border border-white/[0.06] text-[11px] text-[#cbd5e1]">
                                {hoster}
                              </span>
                            {/each}
                          </div>
                        {:else}
                          <span class="text-[#8e95a2]">{downloadSourceInfo.shortName}</span>
                        {/if}
                      {/if}
                      {#if currentVariantSeedInfo}
                        {#if downloadSourceInfo}
                          <span class="text-white/20 select-none">•</span>
                        {/if}
                        {#if currentVariantSeedInfo.loading}
                          <span class="inline-flex items-center gap-1 text-[11px] text-[#64748b] animate-pulse">
                            <UploadSimple class="w-3.5 h-3.5 stroke-[2.5]" />
                            <span>...</span>
                          </span>
                        {:else}
                          <span class="inline-flex items-center gap-1 text-[11px] {currentVariantSeedInfo.seeders > 0 ? 'text-emerald-400 font-semibold' : 'text-[#64748b]'}">
                            <UploadSimple class="w-3.5 h-3.5 stroke-[2.5]" />
                            <span>{formatSeedsCount(currentVariantSeedInfo.seeders)}</span>
                          </span>
                        {/if}
                      {/if}
                    </div>
                  </div>

                  <!-- Bottom Row: Save Path (interactive) + Release Version (interactive) -->
                  <div class="pt-3 border-t border-white/[0.04] flex flex-wrap items-center justify-between gap-3 text-xs text-[#8e95a2]">
                    <!-- Save Folder Button -->
                    <div class="flex items-center gap-2 min-w-0">
                      <span class="text-[#64748b] text-[11px] uppercase tracking-wider font-semibold flex-shrink-0">Папка:</span>
                      <button
                        type="button"
                        class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-[8px] bg-white/[0.03] hover:bg-white/[0.07] font-mono text-xs text-[#cbd5e1] hover:text-white transition-colors cursor-pointer truncate max-w-xs sm:max-w-md group"
                        onclick={handleBrowseFolder}
                        title="Нажмите, чтобы изменить папку для сохранения"
                      >
                        <Folder class="w-3.5 h-3.5 text-[#8e95a2] flex-shrink-0 group-hover:text-white transition-colors" />
                        <span class="truncate">{customDownloadPath}</span>
                        <span class="text-[10px] text-[#64748b] group-hover:text-sky-400 transition-colors ml-1 flex-shrink-0">Изменить</span>
                      </button>
                    </div>

                    <!-- Release Variant Badge / Dropdown -->
                    {#if (activeVariant?.rawName || g.rawName)}
                      <div class="flex items-center gap-2 min-w-0 flex-shrink-0">
                        {#if g.variants && g.variants.length > 1}
                          <span class="text-[#64748b] text-[11px] uppercase tracking-wider font-semibold">Версия:</span>
                          <button
                            bind:this={secondaryVariantDropdownTriggerEl}
                            type="button"
                            class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-[8px] bg-white/[0.03] hover:bg-white/[0.07] font-mono text-xs text-[#cbd5e1] hover:text-white transition-colors cursor-pointer truncate max-w-[200px] sm:max-w-xs group"
                            onclick={(e) => {
                              e.stopPropagation();
                              isVariantDropdownOpen = !isVariantDropdownOpen;
                              if (isVariantDropdownOpen) {
                                variantSearchQuery = '';
                              }
                            }}
                            title="Нажмите, чтобы сменить версию или репак"
                          >
                            <Disc class="w-3.5 h-3.5 text-[#8e95a2] flex-shrink-0 group-hover:text-white transition-colors" />
                            <span class="truncate">{activeVariant?.rawName || g.rawName}</span>
                            <ChevronDown class="w-3 h-3 text-[#8e95a2] flex-shrink-0 transition-transform duration-200 {isVariantDropdownOpen ? 'rotate-180' : ''}" />
                          </button>
                        {:else}
                          <div class="inline-flex items-center gap-1.5 font-mono text-xs text-[#64748b]" title={activeVariant?.rawName || g.rawName}>
                            <Disc class="w-3.5 h-3.5 text-[#64748b] flex-shrink-0" />
                            <span class="truncate max-w-[200px] sm:max-w-xs">{activeVariant?.rawName || g.rawName}</span>
                          </div>
                        {/if}
                      </div>
                    {/if}
                  </div>
                </div>
              {/if}

            </div>
      </div>

      <!-- FULL-WIDTH REPACK & VERSION SELECTOR WITH MICROANIMATION -->
      {#if isVariantDropdownOpen && g.variants && g.variants.length > 1}
        <div
          bind:this={variantDropdownContainerEl}
          transition:slide={{ duration: 250 }}
          class="w-full p-5 rounded-[18px] bg-white/[0.025] space-y-4"
        >
          <!-- Header: Title, Count Badge & Collapse Button -->
          <div class="flex items-center justify-between gap-3 pb-3 border-b border-white/[0.06]">
            <div class="flex items-center gap-2">
              <Disc class="w-4 h-4 text-[#8e95a2]" />
              <span class="text-xs font-bold uppercase tracking-wider text-white">Доступные издания и репаки</span>
              <span class="text-[10px] font-mono font-semibold text-[#8e95a2] bg-white/[0.04] px-2 py-0.5 rounded">
                {displayedVariants.length}{#if variantSearchQuery} / {g.variants.length}{/if}
              </span>
            </div>

            <button
              type="button"
              class="text-xs text-[#8e95a2] hover:text-white flex items-center gap-1.5 cursor-pointer transition-colors py-1 px-2.5 rounded hover:bg-white/[0.06]"
              onclick={() => (isVariantDropdownOpen = false)}
            >
              <span>Свернуть</span>
              <ChevronDown class="w-3.5 h-3.5 rotate-180" />
            </button>
          </div>

          <!-- Toolbar: Search & Sort Buttons (Aligned with Ducke System) -->
          <div class="flex flex-wrap items-center justify-between gap-3">
            <!-- Search Input -->
            <div class="relative flex-1 min-w-[200px] max-w-sm">
              <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7280] pointer-events-none" />
              <input
                type="text"
                bind:value={variantSearchQuery}
                placeholder="Поиск по названию или трекеру..."
                class="w-full bg-[#0d1117] text-[#ededed] placeholder-[#5a6170] text-xs font-medium rounded pl-9 pr-7 py-1.5 focus:outline-none transition-colors"
              />
              {#if variantSearchQuery}
                <button
                  type="button"
                  class="absolute right-2.5 top-1/2 -translate-y-1/2 text-[#6b7280] hover:text-white cursor-pointer"
                  onclick={() => (variantSearchQuery = '')}
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              {/if}
            </div>

            <!-- Sort Segment Switch -->
            <div class="flex items-center gap-2 text-xs flex-shrink-0">
              <span class="text-[10px] font-bold uppercase tracking-wider text-[#64748b] hidden sm:inline">Сортировка:</span>
              <div class="flex items-center bg-[#0d1117] rounded p-0.5">
                <button
                  type="button"
                  class="px-2.5 py-1 rounded-sm text-xs transition-colors cursor-pointer {variantSortBy === 'default' ? 'bg-white/10 text-white font-semibold' : 'text-[#8e95a2] hover:text-white'}"
                  onclick={() => (variantSortBy = 'default')}
                >
                  По умолчанию
                </button>
                <button
                  type="button"
                  class="px-2.5 py-1 rounded-sm text-xs transition-colors cursor-pointer {variantSortBy === 'date' ? 'bg-white/10 text-white font-semibold' : 'text-[#8e95a2] hover:text-white'}"
                  onclick={() => (variantSortBy = 'date')}
                  title="Сортировать по дате добавления"
                >
                  По дате
                </button>
                <button
                  type="button"
                  class="px-2.5 py-1 rounded-sm text-xs transition-colors cursor-pointer {variantSortBy === 'seeds' ? 'bg-white/10 text-white font-semibold' : 'text-[#8e95a2] hover:text-white'}"
                  onclick={() => (variantSortBy = 'seeds')}
                  title="Сортировать по количеству сидов"
                >
                  По сидам
                </button>
                <button
                  type="button"
                  class="px-2.5 py-1 rounded-sm text-xs transition-colors cursor-pointer {variantSortBy === 'size' ? 'bg-white/10 text-white font-semibold' : 'text-[#8e95a2] hover:text-white'}"
                  onclick={() => (variantSortBy = 'size')}
                  title="Сортировать по размеру"
                >
                  По размеру
                </button>
              </div>
            </div>
          </div>

          <!-- List: Structured Ducke-Style Cards with Proper Visual Hierarchy -->
          <div class="max-h-80 overflow-y-auto space-y-1.5 pr-1 no-scrollbar">
            {#if displayedVariants.length === 0}
              <div class="py-8 text-center text-xs text-[#64748b] flex flex-col items-center gap-1.5">
                <span>Релизов по запросу «{variantSearchQuery}» не найдено</span>
                <button
                  type="button"
                  class="text-[#8e95a2] hover:text-white hover:underline cursor-pointer"
                  onclick={() => (variantSearchQuery = '')}
                >
                  Сбросить поиск
                </button>
              </div>
            {:else}
              {#each displayedVariants as variant (variant.id)}
                {@const isSelected = (activeVariant?.id === variant.id)}
                {@const seedInfo = variantSeeds[variant.id]}
                {@const seedCount = seedInfo?.seeders ?? 0}
                {@const rawSrc = formatSourceName(variant.torrentSource)}

                <div
                  role="button"
                  tabindex="0"
                  class="group relative p-3 rounded cursor-pointer transition-all {isSelected ? 'bg-white/[0.08] before:absolute before:left-0 before:top-2 before:bottom-2 before:w-[3px] before:rounded-r before:bg-white' : 'bg-[#0d1117] hover:bg-[#11141c]'}"
                  onclick={() => {
                    selectedVariantId = variant.id;
                  }}
                  onkeydown={(e) => {
                    if (e.key === 'Enter') {
                      selectedVariantId = variant.id;
                    }
                  }}
                  title={variant.rawName}
                >
                  <div class="flex items-center justify-between gap-4">
                    <!-- Left: Disc icon + Title on top, metadata below -->
                    <div class="flex items-start gap-3 min-w-0 flex-1">
                      <div class="w-8 h-8 rounded bg-white/[0.04] flex items-center justify-center flex-shrink-0 mt-0.5 transition-colors">
                        <Disc class="w-4 h-4 {isSelected ? 'text-white' : 'text-[#8e95a2] group-hover:text-white'} transition-colors" />
                      </div>

                      <div class="min-w-0 flex-1">
                        <!-- Main Title -->
                        <h4 class="text-xs sm:text-[13px] font-semibold leading-snug truncate {isSelected ? 'text-white' : 'text-[#ededed] group-hover:text-white'} transition-colors">
                          {variant.rawName}
                        </h4>

                        <!-- Secondary Subtitle: Source & Format -->
                        <div class="flex items-center gap-2 mt-1 text-[11px] text-[#8e95a2]">
                          <span class="truncate">{variant.sourceType === 'torrent' || variant.magnetUri ? `Источник: ${rawSrc || 'Торрент'}` : 'Прямая загрузка'}</span>
                        </div>
                      </div>
                    </div>

                    <!-- Right: Seeds, Size and Button -->
                    <div class="flex items-center gap-3 sm:gap-4 flex-shrink-0">
                      <!-- Seeds: Clean icon + number, no border or bg -->
                      {#if variant.sourceType === 'torrent' || variant.magnetUri}
                        {#if seedInfo?.loading}
                          <span class="inline-flex items-center gap-1 text-[11px] font-mono text-[#64748b] animate-pulse">
                            <UploadSimple class="w-3.5 h-3.5 stroke-[2.5]" />
                            <span>...</span>
                          </span>
                        {:else if seedInfo}
                          <span class="inline-flex items-center gap-1 font-mono text-xs {seedCount >= 10 ? 'text-emerald-400 font-semibold' : (seedCount > 0 ? 'text-amber-400' : 'text-[#64748b]')}">
                            <UploadSimple class="w-3.5 h-3.5 stroke-[2.5]" />
                            <span>{formatSeedsCount(seedCount)}</span>
                          </span>
                        {/if}
                      {/if}

                      <!-- Size -->
                      <span class="font-mono text-xs font-semibold px-2 py-0.5 rounded bg-black/40 {isSelected ? 'text-white' : 'text-[#cbd5e1]'} text-right">
                        {variant.sizeDisplay}
                      </span>

                      <!-- Select button / badge -->
                      {#if isSelected}
                        <span class="px-2.5 py-1 rounded bg-white/15 text-xs font-bold text-white flex items-center gap-1.5">
                          <Check class="w-3.5 h-3.5 stroke-[3] text-emerald-400" />
                          <span>Выбрано</span>
                        </span>
                      {:else}
                        <span class="px-2.5 py-1 rounded bg-white/[0.04] group-hover:bg-white/10 text-xs font-medium text-[#8e95a2] group-hover:text-white transition-colors">
                          Выбрать
                        </span>
                      {/if}
                    </div>
                  </div>
                </div>
              {/each}
            {/if}
          </div>
        </div>
      {/if}

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
          <div class="pointer-events-auto w-full max-w-md bg-[#0d1117] rounded-[16px] flex flex-col overflow-hidden">
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
                        class="text-left flex items-center justify-between px-3 py-2 rounded-sm text-xs font-mono transition-colors cursor-pointer {isSelected ? 'bg-white/10 text-white font-bold' : 'bg-white/[0.03] hover:bg-white/[0.07] text-[#cbd5e1]'}"
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
                    class="flex-1 min-w-0 bg-[#07080a] text-[#ededed] placeholder-[#5a6170] text-xs font-mono rounded px-3 py-2.5 focus:outline-none transition-colors"
                  />
                  <button
                    type="button"
                    title="Выбрать файл на диске"
                    class="flex-shrink-0 w-9 h-9 flex items-center justify-center rounded bg-white/[0.05] hover:bg-white/10 text-[#94a3b8] hover:text-white transition-colors cursor-pointer"
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
                  class="w-full bg-[#07080a] text-[#ededed] placeholder-[#5a6170] text-xs font-mono rounded px-3 py-2.5 focus:outline-none transition-colors"
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
                  class="h-9 px-4 flex items-center justify-center gap-2 rounded text-xs font-bold transition-all cursor-pointer hover:brightness-110 active:scale-95"
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
                    ? 'bg-emerald-500/15 text-emerald-300'
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
                class="h-9 px-3.5 rounded text-xs font-medium text-[#8e95a2] hover:text-white bg-white/[0.04] hover:bg-white/[0.08] transition-colors cursor-pointer"
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
        <div class="space-y-3">
          <h3 class="text-xs font-bold uppercase tracking-wider text-[#8e95a2] flex items-center gap-2">
            <FileText class="w-3.5 h-3.5 text-[#8e95a2]" />
            <span>Об этой игре</span>
          </h3>

          {#if g.detailedDescription}
            <div class="steam-html-content space-y-3 text-sm text-[#cbd5e1] leading-relaxed [&_img]:w-full [&_img]:max-w-full [&_img]:h-auto [&_img]:rounded-none [&_img]:my-3 [&_img]:block [&_img]:border-none [&_img]:shadow-none">
              {@html g.detailedDescription}
            </div>
          {:else}
            <p class="text-xs text-[#6b7280]">
              Подробные сведения о релизе и файлах доступны в разделе «Спецификации релиза».
            </p>
          {/if}
        </div>
      {/snippet}

      {#snippet genresCard()}
        {#if g.genres && g.genres.length > 0}
          <div class="space-y-2 text-xs">
            <div class="text-[11px] font-bold uppercase tracking-wider text-[#8e95a2] flex items-center gap-1.5">
              <Tag class="w-3.5 h-3.5 text-[#8e95a2]" />
              <span>Жанры</span>
            </div>
            <div class="flex items-center gap-1.5 flex-wrap">
              {#each g.genres as genre}
                <button
                  data-nav-item
                  type="button"
                  class="inline-flex items-center px-3 py-1 rounded-[10px] bg-white/[0.04] hover:bg-white/[0.08] active:bg-white/[0.12] text-xs font-medium text-[#cbd5e1] hover:text-white transition-all cursor-pointer"
                  onclick={() => onSelectGenre(genre)}
                  title="Фильтровать по жанру {genre}"
                >
                  <span>{genre}</span>
                </button>
              {/each}
            </div>
          </div>
        {/if}
      {/snippet}

      {#snippet gameSpecsCard()}
        <div class="space-y-3 text-xs">
          <h4 class="font-bold uppercase text-[#8e95a2] flex items-center gap-2 text-[11px] tracking-wider">
            <Monitor class="w-3.5 h-3.5 text-[#8e95a2]" />
            <span>Спецификации и сведения</span>
          </h4>
          <div class="divide-y divide-white/[0.04] text-[#9ca3af]">
            <div class="flex items-center justify-between py-2.5">
              <span class="text-[#8e95a2]">Платформа</span>
              <span class="text-white font-medium flex items-center gap-1.5">
                <Gamepad2 class="w-3.5 h-3.5 text-[#8e95a2]" />
                <span>PC (Windows)</span>
              </span>
            </div>

            <div class="flex items-center justify-between py-2.5 gap-3">
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

            <div class="flex items-center justify-between py-2.5">
              <span class="text-[#8e95a2]">Формат релиза</span>
              <span class="text-white">{g.isDirectory ? 'Папка с файлами' : 'Архив'}</span>
            </div>

            {#if g.remotePath}
              <div class="flex items-center justify-between py-2.5 gap-3">
                <span class="text-[#8e95a2] flex-shrink-0">Путь на сервере</span>
                <div class="flex items-center gap-1.5 min-w-0 justify-end">
                  <span class="font-mono text-white text-xs truncate max-w-[200px] select-all text-right" title={g.remotePath}>{g.remotePath}</span>
                  <button
                    type="button"
                    class="p-1 rounded hover:bg-white/10 text-[#8e95a2] hover:text-white transition-colors flex-shrink-0 cursor-pointer"
                    onclick={() => copyText(g.remotePath, 'Путь скопирован')}
                    title="Скопировать путь"
                  >
                    <Copy class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            {/if}

            <div class="flex items-center justify-between py-2.5 gap-2">
              <span class="text-[#8e95a2] flex-shrink-0 whitespace-nowrap">Steam AppID</span>
              <button
                data-nav-item
                type="button"
                class="h-7 px-2.5 rounded-[8px] bg-white/[0.04] hover:bg-white/[0.08] text-[#cbd5e1] hover:text-white transition-colors cursor-pointer text-xs whitespace-nowrap inline-flex items-center gap-1.5"
                onclick={() => openSteamModal(g)}
                title="Найти в Steam / Изменить метаданные"
              >
                <span class="font-mono">{g.steamAppId > 0 ? g.steamAppId : (g.steamAppId < 0 ? `SGDB: ${-g.steamAppId}` : 'Привязать')}</span>
                <Edit3 class="w-3 h-3 text-[#8e95a2] flex-shrink-0" />
              </button>
            </div>
          </div>
        </div>
      {/snippet}

      {#snippet requirementsCard()}
        <div class="space-y-2.5 text-xs">
          <h4 class="font-bold uppercase text-[#8e95a2] flex items-center gap-2 text-[11px] tracking-wider">
            <Cpu class="w-3.5 h-3.5 text-[#8e95a2]" />
            <span>Системные требования</span>
          </h4>
          <div class="steam-html-content text-xs text-[#9ca3af] leading-relaxed [&_ul]:my-2 [&_ul]:pl-4 [&_li]:mb-1 [&_strong]:text-white/90 [&_h2]:text-xs [&_h2]:font-bold [&_h2]:text-white/90">
            {@html g.pcRequirements}
          </div>
        </div>
      {/snippet}

      {#snippet steamReviewsSection()}
        {#if g.steamAppId && g.steamAppId > 0}
          <div class="space-y-4 pt-2">
            <!-- Section Header -->
            <div class="flex flex-wrap items-center justify-between gap-3 pb-1">
              <div class="flex items-center gap-2.5">
                <div class="w-7 h-7 rounded-[8px] bg-sky-500/10 flex items-center justify-center text-sky-400">
                  <ChatText class="w-4 h-4" />
                </div>
                <div class="flex items-baseline gap-2">
                  <h3 class="text-xs font-bold text-white uppercase tracking-wider">Отзывы сообщества Steam</h3>
                  {#if steamReviewsTotal > 0}
                    <span class="text-xs text-[#8e95a2]">({steamReviewsTotal.toLocaleString('ru-RU')})</span>
                  {/if}
                </div>
              </div>

              <div class="flex items-center gap-2">
                <!-- Language Selector -->
                <div class="inline-flex rounded-[10px] bg-white/[0.03] border border-white/[0.06] p-0.5 text-xs">
                  <button
                    type="button"
                    class="px-2.5 py-1 rounded-[8px] transition-colors cursor-pointer text-xs {reviewLanguage === 'russian' ? 'bg-white/[0.08] text-white font-medium' : 'text-[#8e95a2] hover:text-white'}"
                    onclick={() => handleLanguageChange('russian')}
                  >
                    Русские
                  </button>
                  <button
                    type="button"
                    class="px-2.5 py-1 rounded-[8px] transition-colors cursor-pointer text-xs {reviewLanguage === 'all' ? 'bg-white/[0.08] text-white font-medium' : 'text-[#8e95a2] hover:text-white'}"
                    onclick={() => handleLanguageChange('all')}
                  >
                    Все языки
                  </button>
                </div>

                <!-- Open in Steam button -->
                <button
                  data-nav-item
                  type="button"
                  class="hidden sm:inline-flex items-center gap-1.5 text-xs text-[#8e95a2] hover:text-white px-3 py-1.5 rounded-[10px] bg-white/[0.03] border border-white/[0.06] hover:bg-white/[0.07] transition-all cursor-pointer"
                  onclick={handleOpenSteamStore}
                  title="Открыть страницу игры в магазине Steam"
                >
                  <SteamLogo size={14} weight="bold" />
                  <span>В Steam</span>
                  <ExternalLink class="w-3.5 h-3.5 text-[#8e95a2]" />
                </button>
              </div>
            </div>

            <!-- Reviews Content -->
            {#if isLoadingReviews}
              <!-- Loading skeletons matching card design -->
              <div class="space-y-3">
                {#each [1, 2, 3] as _}
                  <div class="p-4 sm:p-5 rounded-[14px] bg-[#0d1117] border border-white/[0.06] space-y-3 animate-pulse">
                    <div class="flex items-center justify-between">
                      <div class="flex items-center gap-2.5">
                        <div class="h-6 w-24 bg-white/10 rounded-[8px]"></div>
                        <div class="h-4 w-20 bg-white/10 rounded-[6px]"></div>
                      </div>
                      <div class="h-3.5 w-24 bg-white/10 rounded-[6px]"></div>
                    </div>
                    <div class="space-y-2 pt-1">
                      <div class="h-3 w-full bg-white/5 rounded-[4px]"></div>
                      <div class="h-3 w-5/6 bg-white/5 rounded-[4px]"></div>
                      <div class="h-3 w-2/3 bg-white/5 rounded-[4px]"></div>
                    </div>
                  </div>
                {/each}
              </div>
            {:else if steamReviews.length === 0}
              <div class="p-8 rounded-[14px] bg-[#0d1117] border border-white/[0.06] text-center text-xs text-[#8e95a2] space-y-2">
                <p>Отзывов не найдено{reviewLanguage === 'russian' ? ' на русском языке' : ''}.</p>
                {#if reviewLanguage === 'russian'}
                  <button
                    type="button"
                    class="text-sky-400 hover:text-sky-300 hover:underline cursor-pointer transition-colors"
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
                  
                  <div class="p-4 sm:p-5 rounded-[14px] bg-[#0d1117] border border-white/[0.06] hover:border-white/10 transition-colors duration-150 space-y-3">
                    <!-- Top Row: Recommendation + Playtime + Date -->
                    <div class="flex flex-wrap items-center justify-between gap-2.5">
                      <div class="flex flex-wrap items-center gap-2.5">
                        {#if rev.votedUp}
                          <div class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-[8px] bg-sky-500/15 text-sky-400 text-xs font-semibold">
                            <ThumbsUp class="w-3.5 h-3.5" weight="fill" />
                            <span>Рекомендую</span>
                          </div>
                        {:else}
                          <div class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-[8px] bg-rose-500/15 text-rose-400 text-xs font-semibold">
                            <ThumbsDown class="w-3.5 h-3.5" weight="fill" />
                            <span>Не рекомендую</span>
                          </div>
                        {/if}

                        <span class="text-xs text-white/90 font-medium">
                          {rev.playtimeHours} всего
                        </span>

                        {#if rev.playtimeAtReview}
                          <span class="text-[11px] text-[#8e95a2] hidden sm:inline">
                            ({rev.playtimeAtReview} на момент отзыва)
                          </span>
                        {/if}
                      </div>

                      <div class="text-[11px] text-[#8e95a2]">
                        {formatReviewDate(rev.timestampCreated)}
                      </div>
                    </div>

                    <!-- Review Text (anonymized, formatted BBCode) -->
                    <div class="relative">
                      <div
                        class="text-xs sm:text-[13px] text-[#cbd5e1] leading-relaxed break-words font-normal {isLong && !isExpanded ? 'max-h-36 sm:max-h-40 overflow-hidden' : ''}"
                      >
                        {@html formatSteamReviewBBCode(rev.review)}
                      </div>

                      {#if isLong && !isExpanded}
                        <div class="absolute bottom-0 left-0 right-0 h-10 bg-gradient-to-t from-[#0d1117] via-[#0d1117]/80 to-transparent pointer-events-none"></div>
                      {/if}
                    </div>

                    <!-- Bottom Row: Helpful votes count & Expand button -->
                    <div class="flex items-center justify-between gap-3 pt-2 border-t border-white/[0.04] text-[11px] text-[#8e95a2]">
                      <div class="flex items-center gap-2">
                        {#if rev.votesUp > 0}
                          <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-[6px] bg-white/[0.03] border border-white/[0.05] text-[#8e95a2]">
                            <ThumbsUp class="w-3 h-3 text-[#8e95a2]" />
                            <span>{rev.votesUp.toLocaleString('ru-RU')} посчитали полезным</span>
                          </span>
                        {/if}
                        {#if rev.votesFunny > 0}
                          <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-[6px] bg-white/[0.03] border border-white/[0.05] text-[#8e95a2]">
                            <Smiley class="w-3.5 h-3.5 text-[#8e95a2]" />
                            <span>{rev.votesFunny.toLocaleString('ru-RU')}</span>
                          </span>
                        {/if}
                      </div>

                      {#if isLong}
                        <button
                          type="button"
                          class="inline-flex items-center gap-1 text-xs text-sky-400 hover:text-sky-300 font-medium transition-colors cursor-pointer ml-auto"
                          onclick={() => toggleReviewExpand(rev.id)}
                        >
                          <span>{isExpanded ? 'Свернуть' : 'Читать полностью'}</span>
                          <ChevronDown class="w-3.5 h-3.5 transition-transform duration-200 {isExpanded ? 'rotate-180' : ''}" />
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
                    class="px-5 py-2.5 rounded-[12px] bg-white/[0.03] hover:bg-white/[0.07] border border-white/[0.06] text-xs font-medium text-white/80 hover:text-white transition-all cursor-pointer inline-flex items-center gap-2 disabled:opacity-50"
                    onclick={() => loadSteamReviews(false)}
                  >
                    {#if isLoadingMoreReviews}
                      <RefreshCw class="w-3.5 h-3.5 animate-spin text-sky-400" />
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

      <!-- BOTTOM SECTION: Content Area: 2-Column Responsive Side-by-Side Layout on all screens -->
      <div class="space-y-4 pt-1">
        
        {#if hasSteamMetadata}
          <!-- 2-Column Responsive Side-by-Side Layout -->
          <div class="grid grid-cols-12 gap-4 sm:gap-5 lg:gap-6 items-start">
            <div class="col-span-8 2xl:col-span-8 space-y-4 min-w-0">
              {@render mediaAndDescription()}
              {@render steamReviewsSection()}
            </div>
            <div class="col-span-4 2xl:col-span-4 p-4 sm:p-5 rounded-[16px] bg-white/[0.03] space-y-4 min-w-0">
              {#if g.genres && g.genres.length > 0}
                {@render genresCard()}
                <div class="h-px bg-white/[0.04]"></div>
              {/if}
              {@render gameSpecsCard()}
              {#if g.pcRequirements}
                <div class="h-px bg-white/[0.04]"></div>
                {@render requirementsCard()}
              {/if}
            </div>
          </div>

        {:else}
          <!-- RAW / UNENRICHED RELEASES COMPACT VIEW -->
          {@const btih = extractBtih(activeVariant?.magnetUri || g.magnetUri || g.remotePath)}
          {@const isTorrent = (activeVariant?.sourceType || g.sourceType) === 'torrent' || !!(activeVariant?.magnetUri || g.magnetUri || btih)}

          <div class="grid grid-cols-2 gap-4 sm:gap-5">
            
            <!-- CARD 1: Release Specifications -->
            <div class="space-y-3 text-xs">
              <h4 class="font-bold uppercase text-[#8e95a2] flex items-center gap-2 text-[11px] tracking-wider">
                <Server class="w-3.5 h-3.5 text-[#8e95a2]" />
                <span>Сведения о релизе</span>
              </h4>

              <div class="divide-y divide-white/[0.04] text-[#9ca3af]">
                
                <!-- Source -->
                <div class="flex items-center justify-between py-2">
                  <span class="text-[#8e95a2]">Источник</span>
                  <span class="font-medium text-white flex items-center gap-1.5">
                    <HardDrive class="w-3.5 h-3.5 text-[#8e95a2]" />
                    <span>{downloadSourceInfo?.name || (isTorrent ? 'Торрент' : 'FTP-сервер')}</span>
                  </span>
                </div>

                <!-- Raw Release Name -->
                <div class="py-2 space-y-1.5">
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
                  <div class="font-mono text-white text-[11px] bg-black/40 rounded-[12px] p-3 break-words select-all leading-relaxed">
                    {activeVariant?.rawName || g.rawName || getDisplayTitle(g)}
                  </div>
                </div>

                <!-- Size -->
                <div class="flex items-center justify-between py-2">
                  <span class="text-[#8e95a2]">Размер данных</span>
                  <span class="font-bold text-white font-mono">{activeVariant?.sizeDisplay || formatSizeDisplay(g) || '—'}</span>
                </div>

                <!-- Protocol / Hash / Path -->
                {#if isTorrent}
                  <div class="flex items-center justify-between py-2">
                    <span class="text-[#8e95a2]">Тип передачи</span>
                    <span class="text-[#cbd5e1] font-mono">BitTorrent (P2P)</span>
                  </div>

                  {#if btih}
                    <div class="flex items-center justify-between py-2 gap-3">
                      <span class="text-[#8e95a2] flex-shrink-0">Хэш (BTIH)</span>
                      <div class="flex items-center gap-2 min-w-0">
                        <span class="font-mono text-white text-[11px] truncate select-all" title={btih}>
                          {btih}
                        </span>
                        <button
                          type="button"
                          class="p-1.5 rounded-[8px] hover:bg-white/10 text-[#8e95a2] hover:text-white transition-colors flex-shrink-0 cursor-pointer"
                          onclick={() => copyText(activeVariant?.magnetUri || g.magnetUri || `magnet:?xt=urn:btih:${btih}`, 'Magnet-ссылка скопирована')}
                          title="Скопировать magnet-ссылку"
                        >
                          <Copy class="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </div>
                  {/if}
                {:else}
                  <div class="flex items-center justify-between py-2 gap-3">
                    <span class="text-[#8e95a2] flex-shrink-0">Путь на сервере</span>
                    <span class="font-mono text-white truncate max-w-[240px] select-all text-right" title={g.remotePath}>{g.remotePath || '—'}</span>
                  </div>
                  <div class="flex items-center justify-between py-2">
                    <span class="text-[#8e95a2]">Формат</span>
                    <span class="text-white">{g.isDirectory ? 'Папка с файлами' : 'Архив'}</span>
                  </div>
                {/if}

              </div>
            </div>

            <!-- CARD 2: Save Settings & Steam Binding -->
            <div class="space-y-3 text-xs flex flex-col justify-between">
              <div class="space-y-3">
                <h4 class="font-bold uppercase text-[#8e95a2] flex items-center gap-2 text-[11px] tracking-wider">
                  <Folder class="w-3.5 h-3.5 text-[#8e95a2]" />
                  <span>Параметры сохранения и интеграции</span>
                </h4>

                <div class="divide-y divide-white/[0.04] text-[#9ca3af]">
                  
                  <!-- Download Destination -->
                  <div class="flex items-center justify-between py-2 gap-3">
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
                  <div class="flex items-center justify-between py-2">
                    <span class="text-[#8e95a2]">Статус</span>
                    <span class="font-semibold {pageDetails?.isInstalled ? 'text-emerald-400' : 'text-white'}">
                      {pageDetails?.isInstalled ? 'Установлено' : (isDownloading ? 'Скачивается' : 'Готово к скачиванию')}
                    </span>
                  </div>

                  <!-- Steam Metadata Binding -->
                  <div class="flex items-center justify-between py-2 gap-3">
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
                        class="px-3.5 py-2 rounded-[12px] bg-white/[0.06] hover:bg-white/[0.12] text-xs font-medium text-white transition-colors flex items-center gap-1.5 cursor-pointer"
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
              <div class="py-2.5 text-[11px] text-[#8e95a2] leading-relaxed">
                <span class="font-medium text-white/80">Интеграция:</span>
                Привязка к Steam автоматически добавит официальное описание, постер, скриншоты, жанры, дату релиза и системные требования.
              </div>

            </div>

          </div>

          <!-- Toast Copy Feedback -->
          {#if copiedTextFeedback}
            <div class="fixed bottom-6 right-6 z-50 px-4 py-2.5 rounded-[14px] bg-[#11141c] text-white text-xs font-medium flex items-center gap-2">
              <Check class="w-3.5 h-3.5 text-emerald-400" />
              <span>{copiedTextFeedback}</span>
            </div>
          {/if}

          <!-- Toast Launch Error Feedback -->
          {#if launchErrorFeedback}
            <div class="fixed bottom-6 right-6 z-50 px-4 py-2.5 rounded-[14px] bg-[#1c1114] text-rose-200 text-xs font-medium flex items-center gap-2">
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
        <div class="bg-[#10131a] rounded-[20px] w-full max-w-2xl overflow-hidden flex flex-col max-h-[85vh]">
          <!-- Modal Header -->
          <div class="p-5 border-b border-white/[0.06] flex items-center justify-between bg-black/40">
            <div class="space-y-0.5">
              <div class="flex items-center gap-2">
                <Link2 class="w-4 h-4 text-[var(--game-accent)]" />
                <h3 class="text-sm font-bold uppercase text-white tracking-wider">Привязка метаданных Steam</h3>
              </div>
              <p class="text-xs text-[#8e95a2] truncate max-w-md">{getDisplayTitle(g)}</p>
            </div>

            <button
              data-nav-item
              class="text-[#8e95a2] hover:text-white p-2 rounded-[10px] hover:bg-white/5 transition-colors cursor-pointer"
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
                  <Search class="w-4 h-4 text-[#6b7280] absolute left-3.5 top-1/2 -translate-y-1/2 pointer-events-none" />
                  <input
                    data-nav-item
                    type="text"
                    bind:value={steamSearchTerm}
                    placeholder="Введите название игры..."
                    class="w-full bg-white/[0.03] text-white text-xs rounded-[12px] pl-10 pr-4 py-2.5 focus:outline-none"
                  />
                </div>

                <button
                  data-nav-item
                  type="submit"
                  disabled={isSearchingSteam}
                  class="px-5 py-2.5 text-xs font-bold text-black rounded-[12px] cursor-pointer flex items-center gap-1.5 disabled:opacity-50"
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
                <div class="p-6 rounded-[14px] bg-black/30 text-center text-xs text-[#6b7280] space-y-1">
                  <p>Ничего не найдено по данному запросу.</p>
                  <p class="text-[11px] text-[#4b5563]">Попробуйте сократить название или указать AppID вручную ниже.</p>
                </div>
              {:else}
                <div class="space-y-2 max-h-56 overflow-y-auto pr-1">
                  {#each steamCandidates as candidate}
                    {@const isCurrent = g.steamAppId === candidate.appId}
                    {@const matchPercent = Math.round(candidate.score * 100)}
                    
                    <div class="p-3 rounded-[14px] {isCurrent ? 'bg-white/[0.08]' : 'bg-black/40 hover:bg-black/60'} flex items-center justify-between gap-3 transition-colors">
                      <div class="flex items-center gap-3 min-w-0">
                        {#if candidate.tinyImage}
                          <img src={candidate.tinyImage} alt="" class="w-12 h-6 object-cover rounded-[8px] flex-shrink-0 bg-black" />
                        {:else}
                          <div class="w-12 h-6 rounded-[8px] bg-white/5 flex items-center justify-center flex-shrink-0">
                            <Disc class="w-3.5 h-3.5 text-[#6b7280]" />
                          </div>
                        {/if}

                        <div class="min-w-0 space-y-0.5">
                          <div class="flex items-center gap-2">
                            <span class="text-xs font-semibold text-white truncate">{candidate.name}</span>
                            <span
                              class="text-[10px] font-mono font-bold px-1.5 py-0.5 rounded-[6px]"
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
                        class="px-3.5 py-1.5 text-xs font-bold rounded-[10px] cursor-pointer flex items-center gap-1 flex-shrink-0 transition-transform active:scale-95 disabled:opacity-50 {isCurrent ? 'bg-white/10 text-[#9ca3af]' : ''}"
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
                  class="flex-1 bg-white/[0.03] text-white text-xs font-mono rounded-[12px] px-4 py-2.5 border border-white/[0.08] focus:border-[var(--game-accent)] focus:outline-none"
                />
                <button
                  data-nav-item
                  disabled={isSavingSteam || !directAppIdInput.trim()}
                  class="btn-secondary px-4 py-2.5 text-xs font-bold rounded-[12px] cursor-pointer disabled:opacity-50"
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
              class="btn-secondary px-5 py-2 rounded-[12px] text-xs"
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
