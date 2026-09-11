<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Gamepad2,
    HardDrive,
    Search,
    X,
    Folder,
    Download,
    Star,
    Calendar,
    User,
    Layers,
    Building2,
    Tag,
    Monitor,
    Play,
    Server,
    Disc,
    Cpu,
    CheckCircle2,
    FolderOpen,
    FileText,
    ShieldCheck,
    Edit3,
    RefreshCw,
    Link2,
    Unlink,
    ExternalLink,
    Check,
    ChevronLeft,
    ChevronRight,
    Film,
    Image as ImageIcon,
    Maximize2,
    Minimize2,
    ChevronDown,
    Bookmark,
    BookmarkCheck,
    Clock,
    Trash2
  } from 'lucide-svelte';
  import VideoPlayer from './VideoPlayer.svelte';
  import type {
    GameEntity,
    GameVariant,
    SteamMovie,
    MediaItem,
    SteamCandidateItem,
    GamePageDetails
  } from '../types/game';
  import { EventsOn } from '../../../wailsjs/runtime/runtime';
  import { SetFavoriteStatus, RemoveFromFavorites } from '../../../wailsjs/go/main/App';

  let {
    game = null as GameEntity | null,
    downloadPath = '',
    isLoading = false,
    loadingStatusText = '',
    onStartDownload = (gameId: number, targetPath: string) => {},
    onSelectFolder = async (): Promise<string> => '',
    onSelectGenre = (genre: string) => {}
  } = $props();

  const STEAM_DEFAULT_ACCENT = '#66c0f4';

  let customDownloadPath = $state<string>('');
  let pageDetails = $state<GamePageDetails | null>(null);
  let isLoadingDetails = $state<boolean>(false);
  let activeTab = $state<'description' | 'requirements' | 'specs'>('description');
  let activeMediaIndex = $state<number>(0);
  let imageLoadFailed = $state<Record<string, boolean>>({});
  let dynamicAccentColor = $state<string>(STEAM_DEFAULT_ACCENT);
  let coverColorCache = new Map<string, string>();
  let enrichingGameIds = new Set<number>();
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

  async function handleToggleFavoriteStatus(status: string) {
    const targetGame = pageDetails?.game || game;
    if (!targetGame?.id) return;
    try {
      isFavoriteDropdownOpen = false;
      await SetFavoriteStatus(targetGame.id, status);
      targetGame.favoriteStatus = status;
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
      targetGame.favoriteStatus = '';
    } catch (e) {
      console.error('Failed to remove from favorites:', e);
    }
  }

  let activeVariant = $derived.by(() => {
    const vg = pageDetails?.game || game;
    if (!vg?.variants || vg.variants.length === 0) return null;
    return vg.variants.find((v) => v.id === selectedVariantId) || vg.variants[0];
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
    if (downloadPath && !customDownloadPath) {
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
      logoUrl: g.iconUrl || '',
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
      lastGameId = curId;
      const seq = ++switchSequence;

      // 1. Reset all interactive view state for the new game immediately
      activeMediaIndex = 0;
      activeTab = 'description';
      coverAspectRatio = null;
      selectedVariantId = game.variants && game.variants.length > 0 ? game.variants[0].id : curId;
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
    const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
    if (app && app.EnrichGameNow) {
      app.EnrichGameNow(curId).then((enriched: any) => {
        if (isMounted && seq === switchSequence && enriched) {
          fetchGamePageDetails(curId, seq);
        }
      }).catch(() => {});
    }
  }

  function isPlaceholderTitle(title: string | undefined | null): boolean {
    if (!title) return true;
    const t = title.trim();
    return t === '' || /^Steam App \d+$/i.test(t);
  }

  function getDisplayTitle(g: GameEntity | null): string {
    if (!g) return '';
    const raw = (!isPlaceholderTitle(g.steamTitle))
      ? g.steamTitle!
      : (g.cleanTitle && g.cleanTitle.trim() !== '' ? g.cleanTitle : (g.rawName || ''));
    return raw
      .replace(/^[\{\[\(]\s*(linux|win|windows|mac|macos|pc|gog|steam|portable|repack|native|unpack|unpacked)\s*[\}\]\)]\s*/gi, '')
      .replace(/\s*[\{\[\(]\s*(linux|win|windows|mac|macos|pc|gog|steam|portable|repack|native|unpack|unpacked)\s*[\}\]\)]$/gi, '')
      .replace(/[\{\}]/g, '')
      .replace(/[\s\-_]+(?:\[|\()?(\d+([.,]\d+)?\s*(?:gb|mb|tb|гб|мб|тб|g|m|t))(?:\)|\])?$/i, '')
      .replace(/(?:\[|\()?(\d+([.,]\d+)?\s*(?:gb|mb|tb|гб|мб|тб))(?:\)|\])?$/i, '')
      .trim();
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
      const v6b = `https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/page_bg_generated_v6b.jpg`;
      if (!imageLoadFailed[v6b]) return v6b;
      const pageBg = `https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/page.bg.jpg`;
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
                   .replace(/video\.akamai\.steamstatic\.com/gi, 'video.fastly.steamstatic.com');
        }
        if (mp4) {
          mp4 = mp4.replace(/^http:\/\//i, 'https://')
                   .replace(/video\.akamai\.steamstatic\.com/gi, 'video.fastly.steamstatic.com')
                   .replace(/shared\.akamai\.steamstatic\.com/gi, 'video.fastly.steamstatic.com');
          if (mp4.includes('/apps/') && mp4.includes('movie_max.mp4')) mp4 = '';
        }
        if (webm) {
          webm = webm.replace(/^http:\/\//i, 'https://')
                     .replace(/video\.akamai\.steamstatic\.com/gi, 'video.fastly.steamstatic.com')
                     .replace(/shared\.akamai\.steamstatic\.com/gi, 'video.fastly.steamstatic.com');
        }
        if (thumb) {
          thumb = thumb.replace(/^http:\/\//i, 'https://')
                       .replace(/shared\.akamai\.steamstatic\.com/gi, 'shared.fastly.steamstatic.com');
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

    if (game.screenshots && game.screenshots.length > 0) {
      for (let i = 0; i < game.screenshots.length; i++) {
        const rawUrl = typeof game.screenshots[i] === 'string' ? game.screenshots[i] : (game.screenshots[i] as any)?.url;
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
    const selected = await onSelectFolder();
    if (selected) {
      customDownloadPath = selected;
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
    } catch (err) {
      console.error('[GameDetailView] Failed to launch game:', err);
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
      loadGameDetails(game.id);
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
      loadGameDetails(game.id);
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

  const subTabs: ('description' | 'requirements' | 'specs')[] = ['description', 'requirements', 'specs'];
  function cycleSubTab(direction: 'PREV' | 'NEXT') {
    const idx = subTabs.indexOf(activeTab);
    if (direction === 'NEXT') {
      activeTab = subTabs[(idx + 1) % subTabs.length];
    } else {
      activeTab = subTabs[(idx - 1 + subTabs.length) % subTabs.length];
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
      if (!isMounted) return;
      if (event && game && event.gameId === game.id) {
        if (pageDetails) {
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
      isMounted = false;
      clearTimeout(switchTimeout);
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

  let isDownloading = $derived(
    pageDetails?.downloadStatus === 'downloading' ||
    pageDetails?.downloadStatus === 'queued' ||
    pageDetails?.downloadStatus === 'scanning'
  );
</script>

<div
  bind:this={scrollContainer}
  class="flex-1 flex flex-col h-full overflow-y-auto relative select-none bg-[#090a0d] panel-detail"
  style="--game-accent: {dynamicAccentColor}; --game-accent-text: {dynamicAccentTextColor};"
>
  {#if game}
    {#if isSwitching}
      <!-- Atmospheric Skeleton Backdrop -->
      <div class="absolute top-0 left-0 right-0 h-[650px] overflow-hidden pointer-events-none z-0 select-none">
        <div class="w-full h-full bg-white/[0.02] animate-pulse"></div>
        <div class="absolute inset-0 bg-gradient-to-b from-[#090a0d]/30 via-[#090a0d]/65 to-[#090a0d]"></div>
      </div>

      <!-- EXACT GAME PAGE SKELETON (Pixel-perfect replica of page layout) -->
      <div class="relative z-10 w-full max-w-[1280px] mx-auto flex flex-col flex-1 p-6 lg:p-10 space-y-8 pb-28">
        
        <!-- Top Hero Section Skeleton (12 Cols) -->
        <div class="grid grid-cols-1 md:grid-cols-12 gap-8 items-start">
          
          <!-- Left: Box-Art Poster Skeleton -->
          <div class="md:col-span-4 lg:col-span-3 flex justify-center md:justify-start">
            <div class="w-full max-w-[280px] aspect-[2/3] rounded-2xl bg-white/[0.04] border border-white/[0.08] overflow-hidden relative flex items-center justify-center animate-pulse shadow-lg">
              <div class="w-16 h-16 rounded-2xl bg-white/[0.05] border border-white/[0.06] flex items-center justify-center">
                <Gamepad2 class="w-7 h-7 text-white/20" />
              </div>
            </div>
          </div>

          <!-- Right: Info, Chips & Actions Skeleton -->
          <div class="md:col-span-8 lg:col-span-9 space-y-5">
            
            <!-- Title & Rating Header Skeleton -->
            <div class="flex flex-col sm:flex-row items-start justify-between gap-4 border-b border-white/[0.06] pb-4">
              <div class="space-y-2.5 w-full max-w-xl">
                <div class="h-9 sm:h-10 w-3/4 rounded-xl bg-white/[0.06] animate-pulse"></div>
                <div class="h-4 w-1/3 rounded-lg bg-white/[0.03] animate-pulse"></div>
              </div>

              <!-- Rating Score Card Skeleton -->
              <div class="flex items-center gap-3 bg-black/40 border border-white/[0.06] p-2.5 px-4 rounded-2xl flex-shrink-0 animate-pulse">
                <div class="w-11 h-11 rounded-xl bg-white/[0.06]"></div>
                <div class="space-y-1.5">
                  <div class="h-3.5 w-24 rounded bg-white/[0.05]"></div>
                  <div class="h-2.5 w-20 rounded bg-white/[0.03]"></div>
                </div>
              </div>
            </div>

            <!-- Minimalist Metadata Chips Skeleton -->
            <div class="flex flex-wrap items-center gap-2">
              <div class="h-7 w-16 rounded-xl bg-white/[0.04] border border-white/[0.06] animate-pulse"></div>
              <div class="h-7 w-28 rounded-xl bg-white/[0.04] border border-white/[0.06] animate-pulse"></div>
              <div class="h-7 w-24 rounded-xl bg-white/[0.04] border border-white/[0.06] animate-pulse"></div>
              <div class="h-7 w-20 rounded-xl bg-white/[0.04] border border-white/[0.06] animate-pulse"></div>
              <div class="h-7 w-36 rounded-xl bg-white/[0.04] border border-white/[0.06] animate-pulse"></div>
              <div class="h-7 w-24 rounded-xl bg-white/[0.04] border border-white/[0.06] animate-pulse"></div>
              <div class="h-7 w-20 rounded-xl bg-white/[0.04] border border-white/[0.06] animate-pulse"></div>
            </div>

            <!-- Short Synopsis Skeleton -->
            <div class="space-y-2 pt-1">
              <div class="h-4 w-full max-w-2xl rounded bg-white/[0.04] animate-pulse"></div>
              <div class="h-4 w-11/12 max-w-xl rounded bg-white/[0.04] animate-pulse"></div>
              <div class="h-4 w-4/5 max-w-lg rounded bg-white/[0.03] animate-pulse"></div>
            </div>

            <!-- Action Deck Skeleton -->
            <div class="space-y-3 pt-2">
              <div class="flex flex-wrap items-center gap-4">
                <div class="h-10 w-24 rounded-xl bg-white/[0.05] animate-pulse"></div>
                <div class="h-12 w-64 rounded-full bg-white/[0.08] border border-white/10 animate-pulse"></div>
                <div class="w-12 h-12 rounded-full bg-white/[0.04] border border-white/10 animate-pulse"></div>
              </div>

              <!-- Path Info Bar Skeleton -->
              <div class="h-8 w-72 rounded-xl bg-white/[0.02] border border-white/[0.04] animate-pulse"></div>
            </div>

          </div>
        </div>

        <!-- Bottom Tabs & Media Skeleton -->
        <div class="space-y-6 pt-4">
          <!-- Tabs Header Bar Skeleton -->
          <div class="w-full border-b border-white/[0.08] flex items-center gap-8 pb-3.5 pt-1">
            <div class="h-5 w-24 rounded bg-white/[0.08] animate-pulse"></div>
            <div class="h-5 w-36 rounded bg-white/[0.03] animate-pulse"></div>
            <div class="h-5 w-36 rounded bg-white/[0.03] animate-pulse"></div>
          </div>

          <!-- Unified Media Showcase Skeleton -->
          <div class="space-y-3">
            <!-- Main 16:9 Viewport Skeleton -->
            <div class="relative w-full aspect-video rounded-xl overflow-hidden bg-black/60 border border-white/10 flex items-center justify-center animate-pulse">
              <div class="w-14 h-14 rounded-full bg-white/[0.05] border border-white/10 flex items-center justify-center">
                <Play class="w-5 h-5 ml-0.5 text-white/20" />
              </div>
            </div>

            <!-- Thumbnails Strip Skeleton -->
            <div class="flex items-center gap-2.5 overflow-x-hidden pb-1">
              {#each [1, 2, 3, 4, 5] as _}
                <div class="w-32 sm:w-36 aspect-video rounded-lg bg-white/[0.04] border border-white/[0.08] flex-shrink-0 animate-pulse"></div>
              {/each}
            </div>
          </div>

          <!-- Description Paragraphs Skeleton -->
          <div class="space-y-3 pt-2">
            <div class="h-4 w-full rounded bg-white/[0.04] animate-pulse"></div>
            <div class="h-4 w-11/12 rounded bg-white/[0.04] animate-pulse"></div>
            <div class="h-4 w-5/6 rounded bg-white/[0.04] animate-pulse"></div>
            <div class="h-4 w-3/4 rounded bg-white/[0.03] animate-pulse"></div>
            <div class="h-4 w-2/3 rounded bg-white/[0.03] animate-pulse"></div>
          </div>
        </div>

      </div>

    {:else}
      {@const g = pageDetails?.game || game}
      {@const bgUrl = getHeroBackgroundUrl(g)}
      {@const coverUrl = getPrimaryCoverUrl(g)}
      {@const hasCover = !!(coverUrl && !imageLoadFailed[coverUrl])}
      {@const logoUrl = pageDetails?.logoUrl}
      {@const hasLogo = !!(logoUrl && !imageLoadFailed[logoUrl])}

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
        {#if hasCover}
          <div class="{isCoverLandscape ? 'md:col-span-5 lg:col-span-5' : 'md:col-span-4 lg:col-span-3'} flex justify-center md:justify-start">
            <div
              class="relative rounded-2xl overflow-hidden bg-[#0d1017] group w-full {isCoverLandscape ? '' : 'max-w-[280px]'} transition-all duration-300 flex items-center justify-center border border-white/10"
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
          </div>
        {/if}

        <!-- DETAILS & ACTION COLUMN -->
        <div class="{hasCover ? (isCoverLandscape ? 'md:col-span-7 lg:col-span-7' : 'md:col-span-8 lg:col-span-9') : 'md:col-span-12'} space-y-5">
          
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
                    class="max-h-24 sm:max-h-28 max-w-[340px] sm:max-w-[460px] object-contain object-left select-none filter drop-shadow-md"
                    onerror={() => {
                      if (logoUrl) imageLoadFailed[logoUrl] = true;
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
              <div class="flex items-center gap-3 bg-black/40 backdrop-blur-md border border-white/[0.08] p-2.5 px-4 rounded-2xl flex-shrink-0">
                <div
                  class="w-11 h-11 rounded-xl border flex items-center justify-center font-black text-sm {isPositive ? 'bg-[#66c0f4]/15 text-[#66c0f4] border-[#66c0f4]/40' : isMixed ? 'bg-amber-500/15 text-amber-400 border-amber-500/40' : 'bg-red-500/15 text-red-400 border-red-500/40'}"
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
              <div class="flex items-center gap-3 bg-black/40 backdrop-blur-md border border-white/[0.08] p-2.5 px-4 rounded-2xl flex-shrink-0">
                <div
                  class="w-11 h-11 rounded-xl border-2 flex items-center justify-center font-black text-sm"
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

          <!-- Minimalist Metadata Chips (Uniform neutral slate, ascetic styling) -->
          <div class="flex flex-wrap items-center gap-1.5 text-[11px]">
            <span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-white/[0.04] border border-white/[0.06] text-[#9ca3af]">
              <Gamepad2 class="w-3 h-3 text-[#9ca3af]" />
              <span class="font-medium">PC</span>
            </span>

            {#if g.releaseDate}
              <span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-white/[0.04] border border-white/[0.06] text-[#9ca3af]">
                <Calendar class="w-3 h-3 text-[#9ca3af]" />
                <span>{g.releaseDate}</span>
              </span>
            {/if}

            {#if g.genres && g.genres.length > 0}
              {#each g.genres as genre}
                <button
                  type="button"
                  class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-[#9ca3af] hover:text-white transition-colors cursor-pointer"
                  onclick={() => onSelectGenre(genre)}
                  title="Фильтровать по жанру {genre}"
                >
                  <Tag class="w-3 h-3 text-[var(--game-accent)]" />
                  <span>{genre}</span>
                </button>
              {/each}
            {/if}

            {#if g.developers && g.developers.length > 0}
              <span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-white/[0.04] border border-white/[0.06] text-[#9ca3af]">
                <Building2 class="w-3 h-3 text-[#9ca3af]" />
                <span class="truncate max-w-[200px]">{g.developers.join(', ')}</span>
              </span>
            {/if}

            <span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-white/[0.04] border border-white/[0.06] text-[#9ca3af]">
              {#if g.controllerSupport === 'full'}
                <Gamepad2 class="w-3 h-3 text-[#9ca3af]" />
                <span>Геймпад</span>
              {:else}
                <Monitor class="w-3 h-3 text-[#9ca3af]" />
                <span>Клавиатура</span>
              {/if}
            </span>

            <!-- AppID Matcher trigger button -->
            <button
              data-nav-item
              class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-[#9ca3af] hover:text-white transition-colors cursor-pointer"
              onclick={() => openSteamModal(g)}
              title="Найти в Steam / Изменить метаданные"
            >
              <span class="font-mono">{g.steamAppId > 0 ? `AppID: ${g.steamAppId}` : (g.steamAppId < 0 ? `SGDB: ${-g.steamAppId}` : '—')}</span>
              <Edit3 class="w-2.5 h-2.5 text-[#9ca3af] ml-0.5" />
            </button>
          </div>

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
                  class="inline-flex items-center gap-2 text-xs px-3.5 py-2.5 rounded-xl transition-colors cursor-pointer border {g.favoriteStatus ? 'bg-sky-500/15 text-sky-300 border-sky-500/30 hover:bg-sky-500/25' : 'text-[#8e95a2] hover:text-white bg-white/[0.04] hover:bg-white/[0.08] border-white/[0.06] hover:border-white/15'}"
                  onclick={(e) => {
                    e.stopPropagation();
                    isFavoriteDropdownOpen = !isFavoriteDropdownOpen;
                  }}
                  title="Добавить в избранное / статус прохождения"
                >
                  {#if g.favoriteStatus === 'playing'}
                    <Gamepad2 class="w-3.5 h-3.5 text-amber-400" />
                    <span class="font-medium text-amber-300">Прохожу</span>
                  {:else if g.favoriteStatus === 'completed'}
                    <CheckCircle2 class="w-3.5 h-3.5 text-emerald-400" />
                    <span class="font-medium text-emerald-300">Прошел</span>
                  {:else if g.favoriteStatus === 'planned'}
                    <BookmarkCheck class="w-3.5 h-3.5 text-sky-400" />
                    <span class="font-medium text-sky-300">В планах</span>
                  {:else}
                    <Bookmark class="w-3.5 h-3.5" />
                    <span>В избранное</span>
                  {/if}
                  <ChevronDown class="w-3.5 h-3.5 transition-transform duration-200 {isFavoriteDropdownOpen ? 'rotate-180' : ''}" />
                </button>

                {#if isFavoriteDropdownOpen}
                  <div
                    bind:this={favoriteDropdownContainerEl}
                    class="absolute left-0 top-full mt-2 z-50 min-w-[190px] rounded-xl bg-[#0d1117] border border-white/10 shadow-2xl p-1.5 space-y-1 backdrop-blur-md text-xs"
                  >
                    <div class="px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider text-[#6b7280]">
                      Статус в избранном
                    </div>
                    <button
                      type="button"
                      class="w-full text-left flex items-center gap-2 px-2.5 py-2 rounded-lg transition-colors cursor-pointer {g.favoriteStatus === 'planned' ? 'bg-sky-500/20 text-sky-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                      onclick={() => handleToggleFavoriteStatus('planned')}
                    >
                      <Clock class="w-3.5 h-3.5 text-sky-400" />
                      <span>В планах</span>
                      {#if g.favoriteStatus === 'planned'}
                        <Check class="w-3.5 h-3.5 ml-auto text-sky-400" />
                      {/if}
                    </button>
                    <button
                      type="button"
                      class="w-full text-left flex items-center gap-2 px-2.5 py-2 rounded-lg transition-colors cursor-pointer {g.favoriteStatus === 'playing' ? 'bg-amber-500/20 text-amber-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                      onclick={() => handleToggleFavoriteStatus('playing')}
                    >
                      <Gamepad2 class="w-3.5 h-3.5 text-amber-400" />
                      <span>Прохожу</span>
                      {#if g.favoriteStatus === 'playing'}
                        <Check class="w-3.5 h-3.5 ml-auto text-amber-400" />
                      {/if}
                    </button>
                    <button
                      type="button"
                      class="w-full text-left flex items-center gap-2 px-2.5 py-2 rounded-lg transition-colors cursor-pointer {g.favoriteStatus === 'completed' ? 'bg-emerald-500/20 text-emerald-300 font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                      onclick={() => handleToggleFavoriteStatus('completed')}
                    >
                      <CheckCircle2 class="w-3.5 h-3.5 text-emerald-400" />
                      <span>Прошел</span>
                      {#if g.favoriteStatus === 'completed'}
                        <Check class="w-3.5 h-3.5 ml-auto text-emerald-400" />
                      {/if}
                    </button>

                    {#if g.favoriteStatus}
                      <div class="h-px bg-white/[0.06] my-1"></div>
                      <button
                        type="button"
                        class="w-full text-left flex items-center gap-2 px-2.5 py-2 rounded-lg text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
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

            {#if pageDetails?.isInstalled}
              <!-- ALREADY INSTALLED STATE: PLAY + OPEN FOLDER -->
              <div class="flex flex-wrap items-center gap-4">
                <button
                  data-nav-item
                  class="px-9 py-3.5 text-sm font-black rounded-full flex items-center justify-center gap-3 cursor-pointer active:scale-95 transition-all hover:brightness-110 shadow-lg"
                  style="background-color: var(--game-accent); color: var(--game-accent-text);"
                  onclick={handleLaunchGame}
                >
                  <Play class="w-4 h-4 fill-current stroke-[2]" />
                  <span>ИГРАТЬ</span>
                </button>

                <button
                  data-nav-item
                  class="px-5 py-3.5 text-xs font-bold rounded-full bg-white/[0.08] hover:bg-white/[0.14] border border-white/10 text-white flex items-center gap-2 cursor-pointer transition-colors"
                  onclick={handleOpenFolder}
                >
                  <FolderOpen class="w-4 h-4 text-[var(--game-accent)]" />
                  <span>Папка с игрой</span>
                </button>

                {@render favoriteButton()}

                {#if formatSizeDisplay(g)}
                  <span class="text-sm font-mono text-[#8e95a2] px-3 py-1.5 rounded-lg bg-black/40 border border-white/[0.06]">
                    {formatSizeDisplay(g)}
                  </span>
                {/if}
              </div>

              {#if pageDetails.localPath}
                <div class="flex items-center gap-2 text-xs text-[#8e95a2] bg-white/[0.02] border border-white/[0.04] px-4 py-2 rounded-xl w-fit max-w-full">
                  <CheckCircle2 class="w-3.5 h-3.5 text-emerald-400 flex-shrink-0" />
                  <span class="text-[#6b7280] flex-shrink-0">Установлено:</span>
                  <span class="font-mono text-white truncate">{pageDetails.localPath}</span>
                </div>
              {/if}

            {:else if isDownloading}
              <!-- ACTIVE DOWNLOADING / QUEUED STATE -->
              {@const prog = pageDetails?.downloadProgress}
              <div class="space-y-3 max-w-md bg-[#11141c] p-4 rounded-2xl border border-white/10">
                <div class="flex items-center justify-between text-xs font-bold text-white">
                  <div class="flex items-center gap-2">
                    <Download class="w-4 h-4 text-[var(--game-accent)] animate-bounce" />
                    <span>{pageDetails?.downloadStatus === 'queued' ? 'В очереди загрузки...' : (pageDetails?.downloadStatus === 'scanning' ? 'Получение метаданных торрента...' : 'Скачивается...')}</span>
                  </div>
                  <span class="font-mono text-[var(--game-accent)]">{Math.round(prog?.progressPercent || 0)}%</span>
                </div>

                <!-- Progress Bar -->
                <div class="w-full h-2 rounded-full bg-black/60 overflow-hidden border border-white/5">
                  <div
                    class="h-full rounded-full transition-all duration-300"
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
                  {@render favoriteButton()}
                </div>
              </div>

            {:else}
              <!-- NOT INSTALLED / READY TO DOWNLOAD STATE -->
              <div class="space-y-2.5 pt-1">
                <div class="flex flex-wrap items-center gap-3">
                  <!-- Unified Split Download Button -->
                  <div class="relative inline-flex items-stretch rounded-xl shadow-lg border border-white/10 overflow-visible {isVariantDropdownOpen ? 'z-30' : ''}">
                    <button
                      data-nav-item
                      class="px-7 py-3 text-xs sm:text-sm font-black flex items-center gap-2.5 cursor-pointer transition-all hover:brightness-110 active:scale-[0.98] uppercase tracking-wider {g.variants && g.variants.length > 1 ? 'rounded-l-xl' : 'rounded-xl'}"
                      style="background-color: var(--game-accent); color: var(--game-accent-text);"
                      onclick={() => onStartDownload(selectedVariantId || g.id, customDownloadPath)}
                    >
                      <Download class="w-4 h-4 stroke-[2.5]" />
                      <span>СКАЧАТЬ В ХРАНИЛИЩЕ</span>
                      <span class="opacity-35 font-normal">|</span>
                      <span class="font-mono text-xs font-bold tracking-normal">{activeVariant?.sizeDisplay || formatSizeDisplay(g)}</span>
                    </button>

                    {#if g.variants && g.variants.length > 1}
                      <button
                        bind:this={variantDropdownTriggerEl}
                        data-nav-item
                        type="button"
                        class="px-3 flex items-center justify-center border-l border-black/20 hover:brightness-110 active:scale-95 cursor-pointer transition-all rounded-r-xl"
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
                          class="absolute left-0 top-full mt-2 z-50 min-w-[340px] sm:min-w-[420px] max-w-[500px] max-h-72 overflow-y-auto overscroll-contain rounded-xl bg-[#0d1117] border border-white/10 shadow-2xl p-1.5 space-y-1 backdrop-blur-md"
                        >
                          <div class="px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider text-[#6b7280]">
                            Выбор версии для скачивания ({g.variants.length})
                          </div>
                          {#each g.variants as variant (variant.id)}
                            {@const isSelected = (activeVariant?.id === variant.id)}
                            <button
                              type="button"
                              class="w-full text-left flex items-center justify-between gap-3 px-3 py-2.5 rounded-lg text-xs transition-colors cursor-pointer {isSelected ? 'bg-white/10 text-white font-bold' : 'text-[#8e95a2] hover:bg-white/5 hover:text-white'}"
                              onclick={(e) => {
                                e.stopPropagation();
                                selectedVariantId = variant.id;
                                isVariantDropdownOpen = false;
                              }}
                            >
                              <div class="min-w-0 flex-1 pointer-events-none">
                                <div class="truncate text-white text-xs">{variant.rawName}</div>
                                <div class="text-[10px] text-[#6b7280] font-mono">
                                  {variant.sourceType === 'torrent' ? 'Торрент' : 'FTP'}
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

                  <!-- Folder Destination Chip (Clickable to browse) -->
                  <button
                    data-nav-item
                    type="button"
                    class="inline-flex items-center gap-2 text-xs text-[#8e95a2] hover:text-white bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] hover:border-white/15 px-3.5 py-2.5 rounded-xl transition-colors cursor-pointer"
                    onclick={handleBrowseFolder}
                    title="Нажмите, чтобы изменить папку для сохранения"
                  >
                    <Folder class="w-3.5 h-3.5 text-[var(--game-accent)] flex-shrink-0" />
                    <span class="text-[#6b7280]">Папка:</span>
                    <span class="font-mono text-white truncate max-w-[220px] sm:max-w-[320px]">{customDownloadPath}</span>
                  </button>

                  <!-- Favorites Button -->
                  {@render favoriteButton()}
                </div>

                <!-- Release Subtitle when variants exist -->
                {#if g.variants && g.variants.length > 1}
                  <div class="flex items-center gap-2 text-[11px] font-mono text-[#64748b]">
                    <Disc class="w-3 h-3 text-[#8e95a2] flex-shrink-0" />
                    <span>Выбран релиз:</span>
                    <span class="text-[#cbd5e1] font-medium truncate max-w-lg">{activeVariant?.rawName || g.rawName}</span>
                    <button
                      type="button"
                      class="text-[var(--game-accent)] hover:underline cursor-pointer flex items-center gap-0.5"
                      onclick={(e) => {
                        e.stopPropagation();
                        isVariantDropdownOpen = true;
                      }}
                    >
                      <span>(сменить)</span>
                    </button>
                  </div>
                {/if}
              </div>
            {/if}

          </div>

        </div>
      </div>

      <!-- SNIPPETS FOR CONTENT SECTIONS -->
      {#snippet mediaAndDescription()}
        <div class="space-y-6">
          <!-- UNIFIED MEDIA SHOWCASE -->
          {#if mediaList.length > 0 && activeMedia}
            <div class="space-y-3">
              <!-- Main Viewport (16:9) -->
              <div
                bind:this={screenshotViewport}
                class="relative w-full aspect-video rounded-xl overflow-hidden bg-black border border-white/10 group/viewer flex items-center justify-center"
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
                      class="p-2 rounded-lg bg-black/70 hover:bg-black/90 text-white border border-white/15 cursor-pointer transition-colors"
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
                  <div class="absolute bottom-3 left-3 px-2.5 py-1 rounded-md bg-black/75 backdrop-blur-sm border border-white/10 text-[11px] font-semibold text-white/90 pointer-events-none flex items-center gap-2">
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
                      class="relative flex-shrink-0 w-32 sm:w-36 aspect-video rounded-lg overflow-hidden border transition-all cursor-pointer group/thumb text-left {activeMediaIndex === idx ? 'border-white ring-2 ring-white/25 opacity-100' : 'border-white/10 opacity-60 hover:opacity-100 hover:border-white/40'}"
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

      {#snippet requirementsCard()}
        <div class="p-6 rounded-2xl bg-[#11141a] border border-white/[0.06] space-y-3">
          <h4 class="font-bold uppercase text-white flex items-center gap-2 text-xs">
            <Cpu class="w-4 h-4 text-[var(--game-accent)]" />
            <span>Системные требования</span>
          </h4>
          <div class="steam-html-content text-xs text-[#9ca3af]">
            {@html g.pcRequirements}
          </div>
        </div>
      {/snippet}

      {#snippet specsCard()}
        <div class="space-y-4">
          <div class="p-6 rounded-2xl bg-[#11141a] border border-white/[0.06] space-y-3 text-xs">
            <h4 class="font-bold uppercase text-white flex items-center gap-2">
              <Server class="w-4 h-4 text-[var(--game-accent)]" />
              <span>Хранилище репозитория</span>
            </h4>
            <div class="divide-y divide-white/[0.04] space-y-2 text-[#9ca3af]">
              <div class="flex items-center justify-between pt-2">
                <span>Путь на сервере</span>
                <span class="font-mono text-white truncate max-w-[240px]" title={g.remotePath}>{g.remotePath}</span>
              </div>
              <div class="flex items-center justify-between pt-2">
                <span>Общий размер данных</span>
                <span class="font-bold text-white">{g.sizeDisplay || '—'}</span>
              </div>
              <div class="flex items-center justify-between pt-2">
                <span>Формат релиза</span>
                <span class="text-white">{g.isDirectory ? 'Папка с файлами' : 'Архив'}</span>
              </div>
            </div>
          </div>

          <div class="p-6 rounded-2xl bg-[#11141a] border border-white/[0.06] space-y-3 text-xs">
            <h4 class="font-bold uppercase text-white flex items-center gap-2">
              <Folder class="w-4 h-4 text-[var(--game-accent)]" />
              <span>Локальная конфигурация</span>
            </h4>
            <div class="divide-y divide-white/[0.04] space-y-2 text-[#9ca3af]">
              <div class="flex items-center justify-between pt-2">
                <span>Директория сохранения</span>
                <span class="font-mono text-white truncate max-w-[240px]">{customDownloadPath}</span>
              </div>
              <div class="flex items-center justify-between pt-2">
                <span>Статус установки</span>
                <span class="font-semibold {pageDetails?.isInstalled ? 'text-emerald-400' : 'text-[#8e95a2]'}">
                  {pageDetails?.isInstalled ? 'Установлено' : 'Не установлено'}
                </span>
              </div>
              <div class="flex items-center justify-between pt-2">
                <span>Поддержка контроллера</span>
                <span class="font-semibold text-[var(--game-accent)]">{g.controllerSupport === 'full' ? 'Полная (XInput/DirectInput)' : 'Клавиатура / Мышь'}</span>
              </div>
            </div>
          </div>
        </div>
      {/snippet}

      <!-- BOTTOM SECTION: Content Area (Tabs on small screens, 2-column on wide screens) -->
      <div class="space-y-6 pt-4">
        
        {#if hasSteamMetadata}
          <!-- Navigation Tabs: Visible only on smaller screens (<xl) -->
          <div class="xl:hidden w-full border-b border-white/[0.08] flex items-center gap-2 sm:gap-8 overflow-x-auto no-scrollbar">
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
          </div>

          <!-- Small screens view (<xl): Tab-switched -->
          <div class="xl:hidden space-y-6">
            {#if activeTab === 'description'}
              {@render mediaAndDescription()}
            {:else if activeTab === 'requirements' && g.pcRequirements}
              {@render requirementsCard()}
            {:else if activeTab === 'specs'}
              {@render specsCard()}
            {/if}
          </div>

          <!-- Large screens view (>=xl): 2-Column Responsive Side-by-Side Layout -->
          <div class="hidden xl:grid xl:grid-cols-12 xl:gap-8 xl:items-start">
            <div class="xl:col-span-7 2xl:col-span-8 space-y-6 min-w-0">
              {@render mediaAndDescription()}
            </div>
            <div class="xl:col-span-5 2xl:col-span-4 space-y-6 min-w-0">
              {#if g.pcRequirements}
                {@render requirementsCard()}
              {/if}
              {@render specsCard()}
            </div>
          </div>

        {:else}
          <!-- RAW / UNENRICHED RELEASES COMPACT VIEW -->
          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div class="p-6 rounded-2xl bg-[#11141a] border border-white/[0.06] space-y-3 text-xs">
              <h4 class="font-bold uppercase text-white flex items-center gap-2">
                <Server class="w-4 h-4 text-[var(--game-accent)]" />
                <span>Сведения о релизе в репозитории</span>
              </h4>
              <div class="divide-y divide-white/[0.04] space-y-2.5 text-[#9ca3af] pt-1">
                <div class="flex items-center justify-between pt-2">
                  <span>Имя объекта</span>
                  <span class="font-semibold text-white truncate max-w-[220px]">{getDisplayTitle(g)}</span>
                </div>
                <div class="flex items-center justify-between pt-2">
                  <span>Удаленный путь</span>
                  <span class="font-mono text-white truncate max-w-[220px]" title={g.remotePath}>{g.remotePath}</span>
                </div>
                <div class="flex items-center justify-between pt-2">
                  <span>Размер данных</span>
                  <span class="font-bold text-white">{formatSizeDisplay(g) || '—'}</span>
                </div>
                <div class="flex items-center justify-between pt-2">
                  <span>Тип содержимого</span>
                  <span class="text-white">{g.isDirectory ? 'Папка с файлами' : 'Файл / Архив'}</span>
                </div>
              </div>
            </div>

            <div class="p-6 rounded-2xl bg-[#11141a] border border-white/[0.06] space-y-3 text-xs">
              <h4 class="font-bold uppercase text-white flex items-center gap-2">
                <Folder class="w-4 h-4 text-[var(--game-accent)]" />
                <span>Параметры сохранения</span>
              </h4>
              <div class="divide-y divide-white/[0.04] space-y-2.5 text-[#9ca3af] pt-1">
                <div class="flex items-center justify-between pt-2">
                  <span>Целевая папка</span>
                  <span class="font-mono text-white truncate max-w-[220px]">{customDownloadPath}</span>
                </div>
                <div class="flex items-center justify-between pt-2">
                  <span>Статус</span>
                  <span class="font-semibold text-[var(--game-accent)]">
                    {pageDetails?.isInstalled ? 'Установлено' : 'Готово к загрузке'}
                  </span>
                </div>
                <div class="flex items-center justify-between pt-2">
                  <span>Метаданные Steam</span>
                  <button
                    data-nav-item
                    class="text-xs text-[var(--game-accent)] hover:underline flex items-center gap-1 cursor-pointer font-medium"
                    onclick={() => openSteamModal(g)}
                  >
                    <Edit3 class="w-3.5 h-3.5" />
                    <span>Привязать AppID вручную</span>
                  </button>
                </div>
              </div>
            </div>
          </div>
        {/if}

      </div>

    </div>

    <!-- STEAM METADATA MATCHER & APPID EDITOR MODAL -->
    {#if isSteamModalOpen}
      <div data-nav-zone="modal" class="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 sm:p-6">
        <div class="bg-[#10131a] border border-white/10 rounded-2xl w-full max-w-2xl overflow-hidden flex flex-col max-h-[85vh]">
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
              class="text-[#8e95a2] hover:text-white p-1.5 rounded-lg hover:bg-white/5 transition-colors cursor-pointer"
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
                    class="w-full bg-[#090a0d] text-white text-xs rounded-xl pl-9 pr-4 py-2.5 border border-white/[0.08] focus:border-[var(--game-accent)] focus:outline-none"
                  />
                </div>

                <button
                  data-nav-item
                  type="submit"
                  disabled={isSearchingSteam}
                  class="px-5 py-2.5 text-xs font-bold text-black rounded-xl cursor-pointer flex items-center gap-1.5 disabled:opacity-50"
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
                <div class="p-6 rounded-xl bg-black/30 border border-white/[0.05] text-center text-xs text-[#6b7280] space-y-1">
                  <p>Ничего не найдено по данному запросу.</p>
                  <p class="text-[11px] text-[#4b5563]">Попробуйте сократить название или указать AppID вручную ниже.</p>
                </div>
              {:else}
                <div class="space-y-2 max-h-56 overflow-y-auto pr-1">
                  {#each steamCandidates as candidate}
                    {@const isCurrent = g.steamAppId === candidate.appId}
                    {@const matchPercent = Math.round(candidate.score * 100)}
                    
                    <div class="p-2.5 rounded-xl bg-black/40 border {isCurrent ? 'border-[var(--game-accent)]' : 'border-white/[0.06] hover:border-white/15'} flex items-center justify-between gap-3 transition-colors">
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
                          <span class="text-[10px] font-mono text-[#6b7280]">AppID: {candidate.appId}</span>
                        </div>
                      </div>

                      <button
                        data-nav-item
                        disabled={isSavingSteam || isCurrent}
                        class="px-3.5 py-1.5 text-xs font-bold rounded-lg cursor-pointer flex items-center gap-1 flex-shrink-0 transition-transform active:scale-95 disabled:opacity-50 {isCurrent ? 'bg-white/10 text-[#9ca3af]' : ''}"
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
                  class="flex-1 bg-[#090a0d] text-white text-xs font-mono rounded-xl px-4 py-2.5 border border-white/[0.08] focus:border-[var(--game-accent)] focus:outline-none"
                />
                <button
                  data-nav-item
                  disabled={isSavingSteam || !directAppIdInput.trim()}
                  class="btn-secondary px-4 py-2.5 text-xs font-bold rounded-xl cursor-pointer disabled:opacity-50"
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
              class="btn-secondary px-5 py-2 rounded-xl text-xs"
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
        <div class="w-8 h-8 rounded-full border-2 border-sky-400 border-t-transparent animate-spin"></div>
        <p class="text-xs font-medium text-[#8e95a2]">{loadingStatusText || 'Загрузка библиотеки игр...'}</p>
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
