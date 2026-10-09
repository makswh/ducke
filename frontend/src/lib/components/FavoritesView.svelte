<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    BookmarkSimple,
    SquaresFour
  } from 'phosphor-svelte';
  import type { GameEntity } from '../types/game';
  import GameDetailView from './GameDetailView.svelte';
  import GameListSidebar, { type SortOptionItem, type StatusOptionItem } from './GameListSidebar.svelte';
  import { GetFavorites, SetFavoriteLaunchConfig, SelectGameExeFile, LaunchGameWithCustomConfig } from '../../../wailsjs/go/main/App';
  import { EventsOn } from '../../../wailsjs/runtime/runtime';
  import { getDisplayTitle } from '../utils/titleUtils';
  import { parseDateToMs } from '../utils/dateUtils';

  let {
    downloadPath = 'C:\\Ducke',
    onStartDownload = (gameId: number, targetPath: string) => {},
    onSelectFolder = () => Promise.resolve(''),
    onOpenCatalog = () => {},
    onUpdateDownloadPath = (path: string) => {}
  } = $props();

  let favorites = $state<any[]>([]);
  let isLoading = $state<boolean>(true);
  let selectedTab = $state<string>('all');
  let searchQuery = $state<string>('');
  let selectedSort = $state<string>('updated_desc');
  let selectedGameId = $state<number | null>(null);

  const favoriteSortOptions: SortOptionItem[] = [
    { id: 'updated_desc', label: 'По обновлению' },
    { id: 'added_desc', label: 'По добавлению' },
    { id: 'rating', label: 'По оценке Steam' },
    { id: 'name', label: 'По названию (А — Я)' },
    { id: 'size', label: 'По размеру' }
  ];

  async function loadFavorites() {
    try {
      isLoading = true;
      const res = await GetFavorites();
      favorites = Array.isArray(res) ? res : [];
      if (favorites.length > 0 && selectedGameId === null) {
        selectedGameId = favorites[0]?.game?.id || null;
      }
    } catch (e) {
      console.error('Failed to load favorites:', e);
    } finally {
      isLoading = false;
    }
  }

  let unsubFavorites: any = null;
  let unsubEnriched: any = null;

  onMount(() => {
    loadFavorites();
    unsubFavorites = EventsOn('favorites:updated', () => {
      loadFavorites();
    });
    unsubEnriched = EventsOn('game:enriched', (enrichedGame: any) => {
      if (!enrichedGame) return;
      favorites = favorites.map((item) => {
        const g = item.game;
        if (!g) return item;
        const matchesId = item.gameId === enrichedGame.id || g.id === enrichedGame.id;
        const matchesSteam = enrichedGame.steamAppId && g.steamAppId === enrichedGame.steamAppId;
        const matchesCanonical = enrichedGame.canonicalKey && g.canonicalKey && enrichedGame.canonicalKey === g.canonicalKey;
        if (matchesId || matchesSteam || matchesCanonical) {
          return {
            ...item,
            game: {
              ...g,
              ...enrichedGame,
              favoriteStatus: item.status || g.favoriteStatus
            }
          };
        }
        return item;
      });
    });
  });

  onDestroy(() => {
    if (unsubFavorites) unsubFavorites();
    if (unsubEnriched) unsubEnriched();
  });

  // Filter and sort items
  let filteredItems = $derived.by(() => {
    let list = (favorites || []).slice();

    // Tab filter
    if (selectedTab !== 'all') {
      list = list.filter((item) => item.status === selectedTab);
    }

    // Search filter
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim();
      list = list.filter((item) => {
        const title = getDisplayTitle(item.game).toLowerCase();
        const raw = (item.game?.rawName || '').toLowerCase();
        return title.includes(q) || raw.includes(q);
      });
    }

    // Sort
    if (selectedSort === 'updated_desc') {
      list.sort((a, b) => parseDateToMs(b.updatedAt || b.addedAt) - parseDateToMs(a.updatedAt || a.addedAt));
    } else if (selectedSort === 'added_desc') {
      list.sort((a, b) => parseDateToMs(b.addedAt || b.updatedAt) - parseDateToMs(a.addedAt || a.updatedAt));
    } else if (selectedSort === 'rating') {
      list.sort((a, b) => (b.game?.reviewPercent || 0) - (a.game?.reviewPercent || 0));
    } else if (selectedSort === 'name') {
      list.sort((a, b) => getDisplayTitle(a.game).localeCompare(getDisplayTitle(b.game)));
    } else if (selectedSort === 'size') {
      list.sort((a, b) => (b.game?.sizeBytes || 0) - (a.game?.sizeBytes || 0));
    }

    return list;
  });

  // Counts for tabs
  let counts = $derived.by(() => {
    const res = { all: 0, planned: 0, playing: 0, completed: 0 };
    for (const item of favorites || []) {
      res.all++;
      if (item.status === 'planned') res.planned++;
      else if (item.status === 'playing') res.playing++;
      else if (item.status === 'completed') res.completed++;
    }
    return res;
  });

  let statusFilterOptions = $derived<StatusOptionItem[]>([
    { id: 'all', label: 'Все', count: counts.all },
    { id: 'planned', label: 'В планах', count: counts.planned },
    { id: 'playing', label: 'Прохожу', count: counts.playing },
    { id: 'completed', label: 'Прошел', count: counts.completed }
  ]);

  let favoriteGames = $derived.by<GameEntity[]>(() => {
    return filteredItems.map((item) => {
      const g = item.game || {};
      return {
        ...g,
        favoriteStatus: item.status || g.favoriteStatus
      };
    });
  });

  let selectedGame = $derived.by(() => {
    if (!selectedGameId) return filteredItems[0]?.game || null;
    const found = filteredItems.find((item) =>
      item.game?.id === selectedGameId ||
      (item.game?.variants && item.game.variants.some((v: any) => v.id === selectedGameId))
    );
    return found ? found.game : (filteredItems[0]?.game || null);
  });

  let selectedItem = $derived.by(() => {
    if (!selectedGameId) return filteredItems[0] || null;
    const found = filteredItems.find((item) =>
      item.game?.id === selectedGameId ||
      (item.game?.variants && item.game.variants.some((v: any) => v.id === selectedGameId))
    );
    return found || (filteredItems[0] || null);
  });

  // Launch config local state — synced when selected item changes
  let configExePath = $state<string>('');
  let configLaunchArgs = $state<string>('');
  let isSaving = $state<boolean>(false);
  let saveSuccess = $state<boolean>(false);

  // Sync fields when selected item changes
  $effect(() => {
    if (selectedItem) {
      configExePath = selectedItem.customExePath || '';
      configLaunchArgs = selectedItem.launchArguments || '';
    }
  });

  async function browseExeFile() {
    const path = await SelectGameExeFile(configExePath || '');
    if (path) configExePath = path;
  }

  async function saveLaunchConfig() {
    if (!selectedGame) return;
    isSaving = true;
    saveSuccess = false;
    try {
      await SetFavoriteLaunchConfig(selectedGame.id, configExePath.trim(), configLaunchArgs.trim());
      saveSuccess = true;
      setTimeout(() => (saveSuccess = false), 2000);
    } catch (e) {
      console.error('Failed to save launch config:', e);
    } finally {
      isSaving = false;
    }
  }

  async function launchWithCustomConfig() {
    if (!selectedGame) return;
    await LaunchGameWithCustomConfig(selectedGame.id);
  }

</script>

<div data-nav-zone="favorites" class="flex-1 flex h-full overflow-hidden bg-gradient-to-b from-[#0F1218] to-[#0B0F1E] text-white">
  <!-- 1. LEFT SIDEBAR: Reusable Dark Tactile Game List Sidebar -->
  <GameListSidebar
    title="Избранное"
    games={favoriteGames}
    bind:selectedGameId
    bind:searchQuery
    searchPlaceholder="Поиск в избранном..."
    bind:selectedSort
    sortOptions={favoriteSortOptions}
    statusFilterOptions={statusFilterOptions}
    bind:selectedStatus={selectedTab}
    isLoading={isLoading}
    loadingStatusText="Загрузка избранного..."
  />

  <!-- 2. RIGHT DETAIL PANE: Full Window Height -->
  <div class="flex-1 flex overflow-hidden min-w-0 h-full">
    {#if isLoading}
      <div class="flex-1 flex items-center justify-center p-8 text-xs text-[#8e95a2]">
        <span>Загрузка...</span>
      </div>
    {:else if favorites.length === 0}
      <!-- Empty State -->
      <div class="flex-1 flex flex-col items-center justify-center p-8 text-center space-y-4 select-none">
        <div class="w-14 h-14 rounded-[16px] bg-white/[0.04] border border-white/10 flex items-center justify-center text-[#64748b]">
          <BookmarkSimple size={28} weight="duotone" class="text-[#64748b]" />
        </div>
        <div class="space-y-1 max-w-sm">
          <h3 class="text-sm font-bold text-white">Список избранного пуст</h3>
          <p class="text-xs text-[#8e95a2]">
            Добавляйте понравившиеся игры в «В планах», «Прохожу» или «Прошел» со страницы любой игры каталога.
          </p>
        </div>
        <button
          data-nav-item
          type="button"
          class="px-5 py-2.5 rounded-[14px] bg-white text-slate-950 text-xs font-bold hover:bg-white/90 transition-transform active:scale-95 cursor-pointer flex items-center gap-2"
          onclick={onOpenCatalog}
        >
          <SquaresFour size={16} weight="bold" />
          <span>Перейти в каталог</span>
        </button>
      </div>
    {:else if selectedGame}
      <GameDetailView
        game={selectedGame}
        {downloadPath}
        {onStartDownload}
        {onSelectFolder}
        {onUpdateDownloadPath}
        favoriteItem={selectedItem}
      />
    {:else}
      <div class="flex-1 flex items-center justify-center text-xs text-[#8e95a2]">
        <span>Выберите игру из списка</span>
      </div>
    {/if}
  </div>
</div>

