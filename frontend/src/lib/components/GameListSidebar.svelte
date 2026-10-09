<script lang="ts">
  import { onMount, type Snippet } from 'svelte';
  import {
    HardDrive,
    MagnifyingGlass,
    Sliders,
    ArrowsDownUp,
    CaretDown,
    X,
    SquaresFour as Layers,
    ArrowDown
  } from 'phosphor-svelte';
  import type { GameEntity } from '../types/game';
  import { getDisplayTitle } from '../utils/titleUtils';
  import { getGameAddedTime, getGameUpdatedTime } from '../utils/dateUtils';
  import { downloadsStore } from '../stores/downloads.svelte';

  export interface SortOptionItem {
    id: string;
    label: string;
  }

  export interface StatusOptionItem {
    id: string;
    label: string;
    count?: number;
  }

  let {
    title = 'Библиотека',
    games = [] as GameEntity[],
    selectedGameId = $bindable<number | null>(null),
    onSelectGame = (game: GameEntity) => {},
    searchQuery = $bindable<string>(''),
    searchPlaceholder = 'Поиск по названию, тегам...',
    selectedSort = $bindable<string>('added_desc'),
    sortOptions = [
      { id: 'added_desc', label: 'По дате добавления' },
      { id: 'updated_desc', label: 'По дате обновления' },
      { id: 'date_desc', label: 'По дате выхода' },
      { id: 'popular_desc', label: 'Популярные' },
      { id: 'rating_desc', label: 'Оценка Steam' },
      { id: 'name', label: 'По названию (А — Я)' },
      { id: 'size_desc', label: 'По размеру (большие)' },
      { id: 'size_asc', label: 'По размеру (легкие)' }
    ] as SortOptionItem[],
    onSortChange = (sort: string) => {},
    hasFilters = false,
    activeFilterCount = 0,
    onOpenFilters = () => {},
    statusFilterOptions = [] as StatusOptionItem[],
    selectedStatus = $bindable<string>('all'),
    onSelectStatus = (status: string) => {},
    recentGame = null as GameEntity | null,
    isLoading = false,
    loadingStatusText = '',
    filterPanelSnippet,
    filterChipsSnippet
  }: {
    title?: string;
    games: GameEntity[];
    selectedGameId: number | null;
    onSelectGame?: (game: GameEntity) => void;
    searchQuery?: string;
    searchPlaceholder?: string;
    selectedSort?: string;
    sortOptions?: SortOptionItem[];
    onSortChange?: (sort: string) => void;
    hasFilters?: boolean;
    activeFilterCount?: number;
    onOpenFilters?: () => void;
    statusFilterOptions?: StatusOptionItem[];
    selectedStatus?: string;
    onSelectStatus?: (status: string) => void;
    recentGame?: GameEntity | null;
    isLoading?: boolean;
    loadingStatusText?: string;
    filterPanelSnippet?: Snippet;
    filterChipsSnippet?: Snippet;
  } = $props();

  let isSearchOpen = $state<boolean>(false);
  let isStatusDropdownOpen = $state<boolean>(false);
  let imageLoadFailed = $state<Record<string, boolean>>({});

  // Virtualization constants
  const ITEM_HEIGHT = 64;
  const GAP = 6;
  const ITEM_SLOT = ITEM_HEIGHT + GAP;
  const HEADER_FIRST_HEIGHT = 36;
  const HEADER_HEIGHT = 42;

  interface VirtualRow {
    key: string;
    type: 'header' | 'game';
    label?: string;
    game?: GameEntity;
    top: number;
    height: number;
  }

  let listContainer = $state<HTMLDivElement | null>(null);
  let rulerTrackEl = $state<HTMLDivElement | null>(null);
  let scrollTop = $state<number>(0);
  let containerHeight = $state<number>(600);
  let scrollMarkerTop = $state<number>(0);
  let isDraggingRuler = $state<boolean>(false);

  function getSortLabel(val: string): string {
    const found = sortOptions.find((o) => o.id === val);
    return found ? found.label : val;
  }

  function formatSizeDisplay(game: GameEntity | null): string {
    if (!game) return '';
    if (game.sizeBytes && game.sizeBytes > 0) {
      const gb = game.sizeBytes / (1024 * 1024 * 1024);
      if (gb >= 1) return `${gb.toFixed(1)} GB`;
      const mb = game.sizeBytes / (1024 * 1024);
      return `${mb.toFixed(0)} MB`;
    }
    if (game.sizeDisplay && game.sizeDisplay.trim() !== '' && game.sizeDisplay.toLowerCase() !== 'unknown') {
      return game.sizeDisplay.replace(/(\d+(?:\.\d+)?)\s*([A-Za-zА-Яа-я]+)/, '$1 $2');
    }
    return '';
  }

  function getGameDownloadProgress(gameId: number) {
    const realDl = downloadsStore.safeActiveDownloads.find((d) => d.gameId === gameId);
    if (realDl) return { isDownloading: true, percent: realDl.progressPercent, status: realDl.status };
    return null;
  }

  function getStatusLabel(status?: string): string {
    if (status === 'playing') return 'Прохожу';
    if (status === 'completed') return 'Прошел';
    if (status === 'planned') return 'В планах';
    return '';
  }

  function getGameSubtitle(game: GameEntity): string {
    const statusLabel = getStatusLabel(game.favoriteStatus);
    const genre = (game.isCollection ? 'Сборник' : (game.genres && game.genres.length > 0 ? game.genres[0] : '')).trim();
    let year = '';
    if (game.releaseDate) {
      const ym = game.releaseDate.match(/\b(19\d\d|20\d\d)\b/);
      if (ym) year = ym[1];
    }
    const parts = [statusLabel, genre, year].filter(Boolean);
    return parts.join(' • ');
  }

  function updateScrollPosition() {
    if (!listContainer) return;
    scrollTop = listContainer.scrollTop;
    const scrollHeight = listContainer.scrollHeight;
    const clientHeight = listContainer.clientHeight;
    const maxScrollDist = scrollHeight - clientHeight;
    const trackHeight = rulerTrackEl?.clientHeight || Math.max(100, (containerHeight > 50 ? containerHeight : 600) - 48);
    const effectiveTrack = Math.max(1, trackHeight - 8);
    if (maxScrollDist > 0) {
      scrollMarkerTop = Math.max(0, Math.min(effectiveTrack, (scrollTop / maxScrollDist) * effectiveTrack));
    } else {
      scrollMarkerTop = 0;
    }
  }

  function handleScroll() {
    if (!listContainer) return;
    updateScrollPosition();
  }

  function handleRulerScrollTo(clientY: number) {
    if (!rulerTrackEl || !listContainer) return;
    const rect = rulerTrackEl.getBoundingClientRect();
    const effectiveTrack = Math.max(1, rect.height - 8);
    const offsetY = Math.max(0, Math.min(effectiveTrack, clientY - rect.top));
    const ratio = offsetY / effectiveTrack;
    const maxScrollDist = listContainer.scrollHeight - listContainer.clientHeight;
    if (maxScrollDist > 0) {
      listContainer.scrollTop = ratio * maxScrollDist;
      updateScrollPosition();
    }
  }

  function handleRulerWheel(e: WheelEvent) {
    if (!listContainer) return;
    listContainer.scrollTop += e.deltaY;
  }

  function handleRulerPointerDown(e: PointerEvent) {
    if (e.button !== 0) return;
    e.preventDefault();
    e.stopPropagation();
    isDraggingRuler = true;

    const target = e.currentTarget as HTMLElement;
    try {
      target.setPointerCapture(e.pointerId);
    } catch {}

    handleRulerScrollTo(e.clientY);

    const onPointerMove = (moveEvent: PointerEvent) => {
      if (!isDraggingRuler) return;
      moveEvent.preventDefault();
      handleRulerScrollTo(moveEvent.clientY);
    };

    const onPointerUp = (upEvent: PointerEvent) => {
      isDraggingRuler = false;
      try {
        target.releasePointerCapture(upEvent.pointerId);
      } catch {}
      window.removeEventListener('pointermove', onPointerMove);
      window.removeEventListener('pointerup', onPointerUp);
    };

    window.addEventListener('pointermove', onPointerMove);
    window.addEventListener('pointerup', onPointerUp);
  }

  // Precomputed coordinates for virtualized rows
  let virtualItems = $derived.by<VirtualRow[]>(() => {
    if (games.length === 0) return [];

    const rows: VirtualRow[] = [];
    let currentTop = 0;

    const isDateSort = selectedSort === 'added_desc' || selectedSort === 'updated_desc';

    if (!isDateSort) {
      for (let i = 0; i < games.length; i++) {
        const g = games[i];
        rows.push({
          key: `game-${g.id}`,
          type: 'game',
          game: g,
          top: currentTop,
          height: ITEM_HEIGHT
        });
        currentTop += ITEM_SLOT;
      }
      return rows;
    }

    const now = Date.now();
    const oneDay = 24 * 60 * 60 * 1000;
    const sevenDays = 7 * oneDay;

    const todayList: GameEntity[] = [];
    const thisWeekList: GameEntity[] = [];
    const earlierList: GameEntity[] = [];

    for (const g of games) {
      const t = selectedSort === 'updated_desc' ? getGameUpdatedTime(g) : getGameAddedTime(g);
      if (t > 1e11) {
        const diff = now - t;
        if (diff >= 0 && diff < oneDay) {
          todayList.push(g);
          continue;
        } else if (diff >= 0 && diff < sevenDays) {
          thisWeekList.push(g);
          continue;
        }
      }
      earlierList.push(g);
    }

    let isFirstGroup = true;

    if (todayList.length > 0) {
      const hHeight = isFirstGroup ? HEADER_FIRST_HEIGHT : HEADER_HEIGHT;
      rows.push({
        key: 'header-today',
        type: 'header',
        label: 'Сегодня',
        top: currentTop,
        height: hHeight
      });
      currentTop += hHeight;
      isFirstGroup = false;

      for (const g of todayList) {
        rows.push({
          key: `game-${g.id}`,
          type: 'game',
          game: g,
          top: currentTop,
          height: ITEM_HEIGHT
        });
        currentTop += ITEM_SLOT;
      }
    }

    if (thisWeekList.length > 0) {
      const hHeight = isFirstGroup ? HEADER_FIRST_HEIGHT : HEADER_HEIGHT;
      rows.push({
        key: 'header-thisweek',
        type: 'header',
        label: 'На этой неделе',
        top: currentTop,
        height: hHeight
      });
      currentTop += hHeight;
      isFirstGroup = false;

      for (const g of thisWeekList) {
        rows.push({
          key: `game-${g.id}`,
          type: 'game',
          game: g,
          top: currentTop,
          height: ITEM_HEIGHT
        });
        currentTop += ITEM_SLOT;
      }
    }

    if (earlierList.length > 0) {
      if (todayList.length > 0 || thisWeekList.length > 0) {
        const hHeight = isFirstGroup ? HEADER_FIRST_HEIGHT : HEADER_HEIGHT;
        rows.push({
          key: 'header-earlier',
          type: 'header',
          label: 'Раньше',
          top: currentTop,
          height: hHeight
        });
        currentTop += hHeight;
        isFirstGroup = false;
      }

      for (const g of earlierList) {
        rows.push({
          key: `game-${g.id}`,
          type: 'game',
          game: g,
          top: currentTop,
          height: ITEM_HEIGHT
        });
        currentTop += ITEM_SLOT;
      }
    }

    return rows;
  });

  let totalListHeight = $derived.by(() => {
    if (virtualItems.length === 0) return 0;
    const last = virtualItems[virtualItems.length - 1];
    return last.top + last.height + 40;
  });

  let rulerTickCount = $derived.by(() => {
    return Math.max(8, Math.floor((containerHeight - 40) / 16));
  });

  let visibleItems = $derived.by<VirtualRow[]>(() => {
    const total = virtualItems.length;
    if (total === 0) return [];

    const effectiveHeight = containerHeight > 50 ? containerHeight : 700;
    const viewportTop = Math.max(0, scrollTop - 350);
    const viewportBottom = scrollTop + effectiveHeight + 350;

    let low = 0;
    let high = total - 1;
    let startIdx = 0;

    while (low <= high) {
      const mid = (low + high) >> 1;
      const it = virtualItems[mid];
      if (it.top + it.height >= viewportTop) {
        startIdx = mid;
        high = mid - 1;
      } else {
        low = mid + 1;
      }
    }

    let endIdx = startIdx;
    while (endIdx < total && virtualItems[endIdx].top <= viewportBottom) {
      endIdx++;
    }

    const clampedStart = Math.max(0, startIdx - 2);
    const clampedEnd = Math.min(total, endIdx + 2);

    return virtualItems.slice(clampedStart, clampedEnd);
  });

  let prevSearch = $state<string>('');
  let prevSort = $state<string>(selectedSort);
  let prevStatus = $state<string>(selectedStatus);

  // Reset scroll position only when search, sort, or status filter actually changes
  $effect(() => {
    if (searchQuery !== prevSearch || selectedSort !== prevSort || selectedStatus !== prevStatus) {
      prevSearch = searchQuery;
      prevSort = selectedSort;
      prevStatus = selectedStatus;
      if (listContainer) {
        listContainer.scrollTop = 0;
      }
    }
  });

  export function ensureGameVisible(gameId: number) {
    if (!listContainer || virtualItems.length === 0) return;
    const vItem = virtualItems.find((it) => it.type === 'game' && it.game?.id === gameId);
    if (!vItem) return;
    const itemTop = vItem.top;
    const itemBottom = itemTop + vItem.height;
    const curTop = listContainer.scrollTop;
    const curBottom = curTop + containerHeight;

    if (itemTop < curTop) {
      listContainer.scrollTop = itemTop;
    } else if (itemBottom > curBottom) {
      listContainer.scrollTop = itemBottom - containerHeight + 10;
    }
  }

  onMount(() => {
    if (!listContainer) return;
    containerHeight = listContainer.clientHeight || 600;
    updateScrollPosition();

    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.target === listContainer) {
          containerHeight = listContainer.clientHeight || 600;
        }
      }
      updateScrollPosition();
    });
    ro.observe(listContainer);
    if (rulerTrackEl) ro.observe(rulerTrackEl);

    return () => ro.disconnect();
  });
</script>

<!-- SIDEBAR: Master Game List (Dark Tactile Control Panel Style) -->
<aside
  data-nav-zone="list"
  class="panel-master relative w-[360px] min-w-[360px] max-w-[360px] flex flex-col flex-shrink-0 h-full select-none bg-gradient-to-b from-[#0F1218] to-[#0B0F1E] font-['Manrope','Outfit',sans-serif]"
>
  <!-- Bento Grid Controls Header -->
  <div class="p-3 pb-2 flex flex-col gap-[6px] flex-shrink-0">
    <!-- Bento Row 1: Summary Tile + Action Buttons (56x56) -->
    <div class="flex items-stretch gap-[6px] w-full h-[56px]">
      <!-- Summary Tile (flex-1) -->
      <div class="flex-1 min-w-0 h-[56px] rounded-[18px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] px-3.5 flex flex-col justify-center">
        <span class="text-[12px] font-light text-white/[0.50] leading-none">{title}</span>
        <span class="text-[20px] font-normal text-white/[0.92] leading-tight mt-0.5 truncate">{games.length} игр</span>
      </div>

      <!-- Search Action Tile (56x56) -->
      <button
        type="button"
        data-nav-item
        data-nav-search
        onclick={() => (isSearchOpen = !isSearchOpen)}
        class="w-[56px] h-[56px] flex-shrink-0 rounded-[18px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] flex items-center justify-center text-white/70 hover:text-white hover:bg-white/[0.05] hover:-translate-y-[1px] transition-all duration-[160ms] cursor-pointer {isSearchOpen || searchQuery ? 'bg-white/[0.08] text-white' : ''}"
        title="Поиск"
      >
        <MagnifyingGlass size={20} weight="regular" />
      </button>

      <!-- Filter Action Tile (56x56, if filters enabled) -->
      {#if hasFilters}
        <button
          type="button"
          data-nav-item
          data-nav-filter
          onclick={onOpenFilters}
          class="w-[56px] h-[56px] flex-shrink-0 rounded-[18px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] flex items-center justify-center relative text-white/70 hover:text-white hover:bg-white/[0.05] hover:-translate-y-[1px] transition-all duration-[160ms] cursor-pointer {activeFilterCount > 0 ? 'bg-white/[0.08] text-white' : ''}"
          title="Фильтры"
        >
          <Sliders size={20} weight="regular" />
          {#if activeFilterCount > 0}
            <span class="absolute top-2.5 right-2.5 w-1.5 h-1.5 rounded-full bg-[#4C7DFF]"></span>
          {/if}
        </button>
      {/if}

      <!-- Sort Action Tile (56x56) -->
      <div class="w-[56px] h-[56px] flex-shrink-0 rounded-[18px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] flex items-center justify-center relative hover:bg-white/[0.05] hover:-translate-y-[1px] transition-all duration-[160ms] cursor-pointer">
        <ArrowsDownUp size={20} weight="regular" class="text-white/70 pointer-events-none" />
        <select
          data-nav-item
          bind:value={selectedSort}
          onchange={(e) => onSortChange((e.target as HTMLSelectElement).value)}
          class="absolute inset-0 opacity-0 cursor-pointer w-full h-full"
          title="Сортировка"
        >
          {#each sortOptions as opt}
            <option value={opt.id} class="bg-[#0F1218] text-white">{opt.label}</option>
          {/each}
        </select>
      </div>
    </div>

    <!-- Optional Status Chips Row (e.g. for Favorites: Все, Планы, Прохожу, Прошел) -->
    {#if statusFilterOptions.length > 0}
      <div class="flex items-center gap-1.5 overflow-x-auto no-scrollbar py-0.5">
        {#each statusFilterOptions as st}
          <button
            type="button"
            onclick={() => {
              selectedStatus = st.id;
              onSelectStatus(st.id);
            }}
            class="px-3 py-1.5 rounded-[12px] text-[11px] font-medium transition-all cursor-pointer whitespace-nowrap {selectedStatus === st.id ? 'bg-white/[0.12] text-white' : 'bg-white/[0.025] text-white/50 hover:text-white hover:bg-white/[0.05]'}"
          >
            <span>{st.label}</span>
            {#if st.count !== undefined}
              <span class="ml-1 text-[10px] font-mono opacity-70">({st.count})</span>
            {/if}
          </button>
        {/each}
      </div>
    {/if}

    <!-- Bento Row 2: Sort Tile (48px, label left, value + chevron right) -->
    <div class="h-[48px] w-full rounded-[18px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] px-3.5 flex items-center justify-between relative cursor-pointer hover:bg-white/[0.05] transition-all duration-[160ms]">
      <span class="text-[12px] font-light text-white/[0.50] whitespace-nowrap">Сортировка</span>
      <div class="flex items-center gap-1.5 min-w-0 pl-2">
        <span class="text-[13px] font-normal text-white/[0.92] truncate">{getSortLabel(selectedSort)}</span>
        <CaretDown size={14} weight="regular" class="text-white/40 flex-shrink-0 pointer-events-none" />
      </div>
      <select
        bind:value={selectedSort}
        onchange={(e) => onSortChange((e.target as HTMLSelectElement).value)}
        class="absolute inset-0 opacity-0 cursor-pointer w-full h-full"
      >
        {#each sortOptions as opt}
          <option value={opt.id} class="bg-[#0F1218] text-white">{opt.label}</option>
        {/each}
      </select>
    </div>

    <!-- Bento Row 3: Search Input (when toggled or has search query) -->
    {#if isSearchOpen || searchQuery}
      <div class="relative flex items-center h-10 rounded-[18px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] px-3.5 transition-all">
        <MagnifyingGlass size={15} weight="regular" class="text-white/40 mr-2 flex-shrink-0 pointer-events-none" />
        <input
          type="text"
          bind:value={searchQuery}
          placeholder={searchPlaceholder}
          class="w-full bg-transparent text-white placeholder-white/30 text-xs font-light focus:outline-none"
        />
        {#if searchQuery}
          <button
            type="button"
            class="w-5 h-5 rounded-[6px] flex items-center justify-center text-white/40 hover:text-white hover:bg-white/10 cursor-pointer transition-colors"
            onclick={() => (searchQuery = '')}
          >
            <X size={12} weight="regular" />
          </button>
        {/if}
      </div>
    {/if}

    <!-- Bento Row 4: «НЕДАВНО ЗАПУЩЕНО» (if present and not searching) -->
    {#if !searchQuery && recentGame}
      {@const isRecentSelected = selectedGameId === recentGame.id}
      {@const recentDl = getGameDownloadProgress(recentGame.id)}
      {@const recentArt = recentGame.headerImage || recentGame.capsuleImage || recentGame.iconUrl}
      <div
        data-nav-item
        role="button"
        tabindex="0"
        class="group relative h-[96px] overflow-hidden rounded-[18px] p-2.5 px-3 flex items-center gap-3 transition-all duration-[160ms] ease-out cursor-pointer select-none bg-[#4C7DFF]/[0.06] hover:bg-[#4C7DFF]/[0.09] hover:-translate-y-[1px]"
        onclick={() => {
          selectedGameId = recentGame.id;
          onSelectGame(recentGame);
        }}
        onkeydown={(e) => {
          if (e.key === 'Enter') {
            selectedGameId = recentGame.id;
            onSelectGame(recentGame);
          }
        }}
      >
        <div class="w-[2px] h-[32px] bg-[#4C7DFF] rounded-full flex-shrink-0 -ml-1"></div>

        <div class="w-[72px] h-[72px] rounded-[16px] overflow-hidden flex-shrink-0 flex items-center justify-center bg-white/[0.02]">
          {#if recentArt && !imageLoadFailed[recentArt]}
            <img
              src={recentArt}
              alt=""
              decoding="async"
              style="filter: saturate(0.8) brightness(0.88);"
              class="w-full h-full object-cover"
              onerror={() => {
                if (recentArt) {
                  imageLoadFailed[recentArt] = true;
                }
              }}
            />
          {:else}
            <div class="w-full h-full rounded-[16px] bg-white/[0.03]"></div>
          {/if}
        </div>

        <div class="flex-1 min-w-0 flex flex-col justify-center gap-[2px]">
          <div class="text-[11px] font-light text-white/[0.50] leading-tight normal-case tracking-normal">
            Недавно запущено
          </div>
          <div
            class="text-white/[0.92] font-normal break-words"
            style="display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; overflow: hidden; font-size: 14px; line-height: 1.25;"
          >
            {getDisplayTitle(recentGame)}
          </div>
          {#if getGameSubtitle(recentGame)}
            <div class="text-[12px] font-light text-white/[0.50] leading-tight truncate">
              {getGameSubtitle(recentGame)}
            </div>
          {/if}
        </div>

        <div class="w-[72px] min-w-[72px] flex items-center justify-end text-right mr-4 flex-shrink-0 gap-1">
          {#if recentDl && recentDl.isDownloading}
            <ArrowDown size={14} weight="regular" class="text-white/70 flex-shrink-0" />
            <span class="text-[12px] font-normal text-white tabular-nums whitespace-nowrap">
              {Math.round(recentDl.percent)}%
            </span>
          {:else if formatSizeDisplay(recentGame)}
            <span class="text-[12px] font-light text-white/[0.42] tabular-nums whitespace-nowrap">
              {formatSizeDisplay(recentGame)}
            </span>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Optional Filter Chips Tray -->
    {#if filterChipsSnippet}
      {@render filterChipsSnippet()}
    {/if}
  </div>

  <!-- Optional Offcanvas Filter Panel Slot -->
  {#if filterPanelSnippet}
    {@render filterPanelSnippet()}
  {/if}

  <!-- Master Game List Items with Tactile Ruler Gauge & Bottom Fade -->
  <div class="relative flex-1 min-h-0">
    <!-- Functional Tactile Vertical Ruler Gauge (Shifted to the very right edge of the sidebar) -->
    <div
      bind:this={rulerTrackEl}
      role="scrollbar"
      aria-orientation="vertical"
      aria-controls="master-game-list"
      aria-valuenow={Math.round(scrollTop)}
      aria-valuemin={0}
      aria-valuemax={listContainer ? Math.max(0, listContainer.scrollHeight - listContainer.clientHeight) : 100}
      tabindex="-1"
      onpointerdown={handleRulerPointerDown}
      onwheel={handleRulerWheel}
      class="group/ruler absolute right-0 top-2 bottom-10 w-[24px] z-30 select-none flex items-stretch justify-end pr-[2px] {isDraggingRuler ? 'cursor-grabbing' : 'cursor-pointer'}"
      title="Шкала прокрутки: нажмите или перетащите для навигации"
    >
      <div class="w-[10px] h-full flex flex-col justify-between items-end py-1 pointer-events-none">
        {#each Array(rulerTickCount) as _, i}
          {#if (i + 1) % 5 === 0}
            <div class="w-[9px] h-[1px] bg-white/[0.18] transition-colors group-hover/ruler:bg-white/[0.28]"></div>
          {:else}
            <div class="w-[5px] h-[1px] bg-white/[0.10] transition-colors group-hover/ruler:bg-white/[0.18]"></div>
          {/if}
        {/each}
      </div>

      <div
        class="absolute right-[2px] w-[14px] rounded-full pointer-events-none transition-all duration-75 {isDraggingRuler ? 'h-[5px] bg-[#5E8BFF]' : 'h-[3px] bg-[#4C7DFF] group-hover/ruler:bg-[#5E8BFF] group-hover/ruler:h-[5px]'}"
        style="top: {scrollMarkerTop}px;"
      ></div>
    </div>

    <!-- Bottom Fade (64px linear-gradient to #0B0F1E) -->
    <div
      class="absolute bottom-0 left-0 right-0 h-[64px] pointer-events-none z-10"
      style="background: linear-gradient(to bottom, transparent, #0B0F1E);"
    ></div>

    <!-- Scrollable Virtual List (Native scrollbar completely hidden, pr-6 so cards never overlap ruler) -->
    <div
      id="master-game-list"
      bind:this={listContainer}
      onscroll={handleScroll}
      style="scrollbar-width: none; -ms-overflow-style: none;"
      class="h-full overflow-y-auto p-2 pr-6 select-none no-scrollbar"
    >
      {#if isLoading && games.length === 0}
        <div class="px-3 py-2 mb-1 text-[11px] text-white/40 font-light tracking-wider truncate">
          {loadingStatusText || 'Загрузка...'}
        </div>
        <div class="flex flex-col gap-[6px] w-full pointer-events-none select-none">
          {#each Array(10) as _, i}
            {@const opacity = (Math.max(0.04, 1 - Math.pow(i / 9, 1.25) * 0.96)).toFixed(3)}
            <div
              class="h-[64px] rounded-[18px] bg-gradient-to-b from-white/[0.03] to-white/[0.02] p-2 flex items-center gap-3 transition-opacity duration-300"
              style="animation-delay: {i * 60}ms; opacity: {opacity};"
            >
              <div class="w-[48px] h-[48px] rounded-[14px] bg-white/[0.03]"></div>
              <div class="flex-1 space-y-1.5">
                <div class="h-2 w-16 rounded bg-white/[0.03]"></div>
                <div class="h-3 w-3/4 rounded bg-white/[0.05]"></div>
              </div>
            </div>
          {/each}
        </div>
      {:else if games.length === 0}
        <div class="p-8 text-center text-xs text-white/40 space-y-2">
          <Layers class="w-6 h-6 mx-auto text-white/30" />
          <p class="font-light">Игр не найдено</p>
        </div>
      {:else}
        <!-- Fully Virtualized Game List with Absolute Positioning (renders only visible rows) -->
        <div
          style="height: {totalListHeight}px; position: relative; width: 100%;"
          class="select-none"
        >
          {#each visibleItems as item (item.key)}
            {#if item.type === 'header'}
              <div
                style="position: absolute; top: {item.top}px; left: 0; right: 0; height: {item.height}px;"
                class="text-[12px] font-light text-white/[0.50] pl-3 flex items-end pb-2 select-none pointer-events-none"
              >
                {item.label}
              </div>
            {:else if item.game}
              <div
                style="position: absolute; top: {item.top}px; left: 0; right: 0; height: 64px;"
              >
                {@render gameCard(item.game)}
              </div>
            {/if}
          {/each}
        </div>
      {/if}
    </div>
  </div>
</aside>

{#snippet gameCard(game: GameEntity)}
  {@const isSelected = selectedGameId === game.id}
  {@const dl = getGameDownloadProgress(game.id)}

  <div
    data-nav-item
    role="button"
    tabindex="0"
    class="group relative h-[64px] min-h-[64px] max-h-[64px] overflow-hidden rounded-[18px] cursor-pointer transition-all duration-[160ms] ease-out select-none outline-none focus:outline-none focus-visible:outline-none {isSelected
      ? 'bg-white/[0.08] text-white'
      : 'bg-gradient-to-b from-white/[0.03] to-white/[0.02] hover:bg-white/[0.05] hover:-translate-y-[1px]'}"
    onclick={() => {
      selectedGameId = game.id;
      onSelectGame(game);
    }}
    onkeydown={(e) => {
      if (e.key === 'Enter') {
        selectedGameId = game.id;
        onSelectGame(game);
      }
    }}
  >
    <!-- Downloading Progress Layer (Fill 0.07, right border line 1px 0.18) -->
    {#if dl && dl.isDownloading}
      <div
        class="absolute inset-y-0 left-0 bg-white/[0.07] rounded-l-[18px] rounded-r-none pointer-events-none transition-all duration-300 overflow-hidden"
        style="width: {Math.max(0, Math.min(100, dl.percent))}%;"
      >
        <div class="absolute right-0 top-0 bottom-0 w-[1px] bg-white/[0.18] pointer-events-none"></div>
      </div>
    {/if}

    <!-- Card Content (align-items:center, padding: 10px 12px) -->
    <div class="relative z-10 px-3 py-2.5 h-full flex items-center gap-3">
      <!-- 48px Icon with 14px radius (filter: saturate(0.8) brightness(0.88)) -->
      <div class="w-[48px] h-[48px] rounded-[14px] overflow-hidden flex-shrink-0 flex items-center justify-center bg-white/[0.02]">
        {#if game.iconUrl && !imageLoadFailed[game.iconUrl]}
          <img
            src={game.iconUrl}
            alt=""
            decoding="async"
            style="filter: saturate(0.8) brightness(0.88);"
            class="w-full h-full object-cover"
            onerror={() => {
              if (game.iconUrl) {
                imageLoadFailed[game.iconUrl] = true;
              }
            }}
          />
        {:else if (game.headerImage || game.capsuleImage) && !imageLoadFailed[game.headerImage || game.capsuleImage || '']}
          {@const fallbackArt = game.headerImage || game.capsuleImage || ''}
          <img
            src={fallbackArt}
            alt=""
            decoding="async"
            style="filter: saturate(0.8) brightness(0.88);"
            class="w-full h-full object-cover"
            onerror={() => {
              imageLoadFailed[fallbackArt] = true;
            }}
          />
        {:else}
          <div class="w-full h-full rounded-[14px] bg-white/[0.03]"></div>
        {/if}
      </div>

      <!-- Text Block (min-width 0, flex 1, gap 2px between subtitle and title) -->
      <div class="flex-1 min-w-0 flex flex-col justify-center gap-[2px]">
        {#if getGameSubtitle(game)}
          <div class="text-[12px] font-light text-white/[0.50] leading-tight normal-case tracking-normal truncate">
            {getGameSubtitle(game)}
          </div>
        {/if}
        <div
          class="text-white/[0.92] font-normal break-words {isSelected ? 'text-white' : 'hover:text-white'}"
          style="display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; overflow: hidden; font-size: 14px; line-height: 1.25;"
        >
          {getDisplayTitle(game)}
        </div>
      </div>

      <!-- Right Column: 72px without compression, right-aligned, mr-4, tabular-nums -->
      <div class="w-[72px] min-w-[72px] max-w-[72px] flex-shrink-0 flex items-center justify-end text-right mr-4 gap-1">
        {#if dl && dl.isDownloading}
          <ArrowDown size={14} weight="regular" class="text-white/70 flex-shrink-0" />
          <span class="text-[12px] font-normal text-white tabular-nums whitespace-nowrap">
            {Math.round(dl.percent)}%
          </span>
        {:else if formatSizeDisplay(game)}
          <span class="text-[12px] font-light text-white/[0.42] tabular-nums whitespace-nowrap">
            {formatSizeDisplay(game)}
          </span>
        {/if}
      </div>
    </div>
  </div>
{/snippet}

<style>
  :global(.no-scrollbar::-webkit-scrollbar) {
    display: none !important;
    width: 0 !important;
    height: 0 !important;
  }
  :global(.no-scrollbar) {
    -ms-overflow-style: none !important;
    scrollbar-width: none !important;
  }
</style>
