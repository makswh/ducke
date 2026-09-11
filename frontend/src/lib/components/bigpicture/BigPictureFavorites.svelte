<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Bookmark,
    Search,
    Download,
    Gamepad2,
    Check,
    Clock,
    CheckCircle2,
    Trash2,
    X,
    Star
  } from 'lucide-svelte';
  import { sound } from '../../navigation/audio';
  import {
    GetFavorites,
    SetFavoriteStatus,
    RemoveFromFavorites
  } from '../../../../wailsjs/go/main/App';
  import { EventsOn } from '../../../../wailsjs/runtime/runtime';
  import type { GameEntity } from '../../types/game';

  let {
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
        const g = f.game;
        if (!g) return false;
        const title = (g.cleanTitle || g.displayTitle || g.folderName || '').toLowerCase();
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
      case 'playing': return 'bg-sky-500 text-black font-black';
      case 'completed': return 'bg-emerald-500 text-black font-black';
      case 'planned': return 'bg-amber-400 text-black font-black';
      default: return 'bg-white/20 text-white font-bold';
    }
  }

  function getCleanTitle(game: any): string {
    if (!game) return '';
    if (game.steamTitle && !/^Steam App \d+$/i.test(game.steamTitle)) {
      return game.steamTitle;
    }
    return game.cleanTitle || game.displayTitle || game.folderName || '';
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
    unsubFavorites = EventsOn('favorites:updated', () => {
      if (isMounted) loadFavorites();
    });
  });

  onDestroy(() => {
    isMounted = false;
    if (typeof unsubFavorites === 'function') unsubFavorites();
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
          <X class="w-3.5 h-3.5" />
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
          <Bookmark class="w-8 h-8" />
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
        {#each filteredFavorites as item (item.gameId)}
          {@const game = item.game}
          {@const cover = game?.capsuleImage || game?.headerImage || game?.backgroundImage}
          {@const isDownloading = (activeDownloads || []).some((d: any) => d && d.gameId === item.gameId && d.status === 'downloading')}

          <button
            data-nav-item
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
                />
              {:else}
                <div class="w-full h-full flex flex-col items-center justify-between p-4 text-center bg-gradient-to-b from-[#181d28] via-[#10141d] to-[#0a0c12]">
                  <div class="w-full flex justify-end">
                    <Gamepad2 class="w-4 h-4 text-white/20" />
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
              <div class="absolute top-2.5 left-2.5 px-2 py-0.5 rounded-md text-[9px] uppercase tracking-wider z-20 shadow-md {getStatusBadgeStyle(item.status)}">
                {getStatusLabel(item.status)}
              </div>

              <!-- Downloading Badge Overlay Top Right -->
              {#if isDownloading}
                <div class="absolute top-2.5 right-2.5 px-2 py-1 rounded-lg bg-sky-500 text-black text-[9px] font-black tracking-wider flex items-center gap-1 shadow-lg z-20">
                  <Download class="w-3 h-3 animate-bounce" />
                  <span>СКАЧИВАЕТСЯ</span>
                </div>
              {/if}

              <!-- Review score bottom left -->
              {#if game?.reviewPercent && game.reviewPercent > 0}
                <div class="absolute bottom-2 left-2 px-1.5 py-0.5 rounded-md bg-black/80 backdrop-blur-md text-[10px] font-mono font-bold flex items-center gap-1 border border-white/10 z-20 {game.reviewPercent >= 70 ? 'text-sky-400' : 'text-[#94a3b8]'}">
                  <span>★ {game.reviewPercent}%</span>
                </div>
              {/if}

              <!-- Size badge bottom right -->
              {#if game?.sizeDisplay}
                <div class="absolute bottom-2 right-2 px-1.5 py-0.5 rounded-md bg-black/80 backdrop-blur-md text-[10px] font-mono text-white/90 border border-white/10 z-20">
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
