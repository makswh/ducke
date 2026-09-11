<script lang="ts">
  import { onMount } from 'svelte';
  import { Magnet, RefreshCw, Plus, Trash2, Folder, X } from 'lucide-svelte';
  import { sound } from '../navigation/audio';
  import * as AppAPI from '../../../wailsjs/go/main/App';

  interface TorrentSourceItem {
    id: string;
    name: string;
    url: string;
    enabled: boolean;
    itemCount: number;
    lastSynced: number;
  }

  let {
    torrentSources = [] as TorrentSourceItem[],
    onSourcesChanged = (sources: TorrentSourceItem[]) => {},
    isBigPicture = false
  } = $props();

  let newTorrentUrl = $state<string>('');
  let isAddingSource = $state<boolean>(false);
  let isSyncingTorrents = $state<boolean>(false);
  let torrentMessage = $state<{ type: 'success' | 'error'; text: string } | null>(null);

  async function refreshSettingsList() {
    try {
      const sources = await AppAPI.GetTorrentSources();
      if (Array.isArray(sources)) {
        onSourcesChanged(sources);
        return;
      }
      const updated = await AppAPI.GetSettings();
      if (updated && Array.isArray(updated.torrentSources)) {
        onSourcesChanged(updated.torrentSources);
      }
    } catch (e) {
      console.error('[TorrentSourcesManager] Failed to fetch settings:', e);
    }
  }

  onMount(async () => {
    await refreshSettingsList();
  });

  async function handleAddTorrentSource(urlToAdd?: string) {
    const targetUrl = (urlToAdd || newTorrentUrl).trim();
    if (!targetUrl) return;
    sound.playSelect();
    isAddingSource = true;
    torrentMessage = null;
    try {
      const src = await AppAPI.AddTorrentSource(targetUrl);
      newTorrentUrl = '';
      torrentMessage = { type: 'success', text: `Источник "${src.name}" успешно добавлен (${src.itemCount} игр)` };
      setTimeout(() => { torrentMessage = null; }, 5000);
      await refreshSettingsList();
    } catch (e: any) {
      torrentMessage = { type: 'error', text: `Ошибка добавления источника: ${e?.message || e}` };
    } finally {
      isAddingSource = false;
    }
  }

  async function handleImportFromFile() {
    sound.playSelect();
    isAddingSource = true;
    torrentMessage = null;
    try {
      const src = await AppAPI.ImportTorrentSourceFile();
      if (src && src.name) {
        torrentMessage = { type: 'success', text: `Источник "${src.name}" успешно добавлен (${src.itemCount} игр)` };
        setTimeout(() => { torrentMessage = null; }, 5000);
        await refreshSettingsList();
      }
    } catch (e: any) {
      if (e && !e.toString().includes('cancelled') && !e.toString().includes('closed')) {
        torrentMessage = { type: 'error', text: `Ошибка импорта: ${e?.message || e}` };
      }
    } finally {
      isAddingSource = false;
    }
  }

  async function handleToggleTorrentSource(id: string, currentEnabled: boolean) {
    sound.playSelect();
    try {
      await AppAPI.ToggleTorrentSource(id, !currentEnabled);
      await refreshSettingsList();
    } catch (e: any) {
      console.error('[TorrentSourcesManager] Failed to toggle source:', e);
    }
  }

  async function handleRemoveTorrentSource(id: string) {
    sound.playSelect();
    try {
      await AppAPI.RemoveTorrentSource(id);
      await refreshSettingsList();
    } catch (e: any) {
      console.error('[TorrentSourcesManager] Failed to remove source:', e);
    }
  }

  async function handleSyncAllTorrents() {
    if (isSyncingTorrents) return;
    sound.playSelect();
    isSyncingTorrents = true;
    torrentMessage = null;
    try {
      await AppAPI.SyncTorrentSources();
      torrentMessage = { type: 'success', text: 'Все активные источники торрентов синхронизированы' };
      setTimeout(() => { torrentMessage = null; }, 4000);
      await refreshSettingsList();
    } catch (e: any) {
      torrentMessage = { type: 'error', text: `Ошибка синхронизации: ${e?.message || e}` };
    } finally {
      isSyncingTorrents = false;
    }
  }
</script>

<div class="space-y-6">
  <!-- Top Bar -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-white/[0.06] pb-4">
    <div>
      <h1 class="{isBigPicture ? 'text-xl' : 'text-base'} font-black uppercase tracking-wider text-white font-mono flex items-center gap-2.5">
        <Magnet class="{isBigPicture ? 'w-5 h-5' : 'w-4 h-4'} text-sky-400" />
        <span>Источники торрентов</span>
      </h1>
      <p class="{isBigPicture ? 'text-xs' : 'text-[11px]'} text-[#64748b] font-mono mt-0.5">
        Пользовательские каталоги раздач в формате JSON и синхронизация
      </p>
    </div>

    <button
      data-nav-item
      disabled={isSyncingTorrents || (torrentSources || []).length === 0}
      class="px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 active:bg-white/25 text-xs font-mono font-bold text-white cursor-pointer flex items-center gap-2 disabled:opacity-40 transition-colors self-start sm:self-auto focus:ring-2 focus:ring-white focus:outline-none"
      onclick={handleSyncAllTorrents}
    >
      <RefreshCw class="w-3.5 h-3.5 {isSyncingTorrents ? 'animate-spin text-sky-400' : ''}" />
      <span>{isSyncingTorrents ? 'Синхронизация...' : 'Синхронизировать все'}</span>
    </button>
  </div>

  <!-- Message Banner -->
  {#if torrentMessage}
    <div class="p-3.5 rounded-xl border text-xs font-mono flex items-center justify-between {torrentMessage.type === 'success' ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300' : 'bg-rose-500/10 border-rose-500/30 text-rose-300'}">
      <span>{torrentMessage.text}</span>
      <button onclick={() => (torrentMessage = null)} class="opacity-60 hover:opacity-100 p-0.5 cursor-pointer">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  {/if}

  <!-- Add Source Section -->
  <div class="space-y-3 pb-6 border-b border-white/[0.06]">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
      <div>
        <span class="text-xs text-[#cbd5e1] font-mono block font-bold">Добавить источник раздач</span>
        <span class="text-[11px] text-[#64748b] font-mono">
          Укажите прямую веб-ссылку (URL) или выберите локальный JSON-файл каталога
        </span>
      </div>

      <button
        data-nav-item
        type="button"
        disabled={isAddingSource}
        class="px-3.5 py-1.5 rounded-xl bg-white/5 hover:bg-white/10 active:bg-white/15 text-xs font-mono text-[#cbd5e1] hover:text-white border border-white/[0.06] transition-colors cursor-pointer flex items-center gap-2 self-start sm:self-auto disabled:opacity-50 focus:ring-2 focus:ring-white focus:outline-none"
        onclick={handleImportFromFile}
        title="Выбрать JSON-файл с диска"
      >
        <Folder class="w-3.5 h-3.5" />
        <span>Выбрать файл</span>
      </button>
    </div>

    <div class="flex items-center gap-2.5">
      <input
        data-nav-item
        type="text"
        bind:value={newTorrentUrl}
        placeholder="https://.../source.json"
        class="flex-1 bg-[#0c1017] border border-white/[0.08] px-3.5 py-2.5 text-xs font-mono text-white rounded-xl placeholder:text-white/20 focus:border-sky-400 focus:ring-2 focus:ring-white focus:outline-none"
        onkeydown={(e) => {
          if (e.key === 'Enter') handleAddTorrentSource();
        }}
      />
      <button
        data-nav-item
        disabled={isAddingSource || !newTorrentUrl.trim()}
        class="px-5 py-2.5 rounded-xl bg-sky-500 hover:bg-sky-400 active:bg-sky-600 disabled:opacity-40 text-slate-950 text-xs font-mono font-bold cursor-pointer flex items-center gap-2 transition-colors focus:ring-2 focus:ring-white focus:outline-none"
        onclick={() => handleAddTorrentSource()}
      >
        {#if isAddingSource}
          <RefreshCw class="w-3.5 h-3.5 animate-spin text-slate-950" />
          <span>Загрузка...</span>
        {:else}
          <Plus class="w-3.5 h-3.5" />
          <span>Добавить</span>
        {/if}
      </button>
    </div>
  </div>

  <!-- Connected Sources List -->
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <span class="text-xs font-mono font-bold uppercase tracking-wider text-[#8e95a2]">
        Подключенные каталоги ({(torrentSources || []).length})
      </span>
    </div>

    {#if (torrentSources || []).length === 0}
      <div class="p-8 rounded-2xl bg-[#090c12] border border-white/[0.04] text-center text-[#64748b] space-y-2">
        <Magnet class="w-8 h-8 stroke-1 mx-auto opacity-30 text-[#64748b]" />
        <p class="text-xs font-mono text-[#cbd5e1] font-bold">Источники не подключены</p>
        <p class="text-[11px] text-[#64748b] font-mono max-w-md mx-auto">
          Добавьте ссылку на внешний JSON-каталог или выберите локальный файл, чтобы загрузить список раздач.
        </p>
      </div>
    {:else}
      <div class="space-y-2">
        {#each (torrentSources || []) as src (src.id)}
          <div class="flex items-center justify-between p-3.5 rounded-xl bg-[#0c1017] border border-white/[0.06] hover:border-white/15 transition-colors">
            <div class="space-y-1 min-w-0 pr-4">
              <div class="flex items-center gap-2.5 flex-wrap">
                <span class="{isBigPicture ? 'text-sm' : 'text-xs'} font-bold text-white font-mono truncate">{src.name}</span>
                <span class="px-2 py-0.5 rounded text-[10px] font-mono font-bold {src.enabled ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/25' : 'bg-white/5 text-[#8e95a2] border border-white/5'}">
                  {src.enabled ? 'АКТИВЕН' : 'ОТКЛЮЧЕН'}
                </span>
                <span class="px-2 py-0.5 rounded text-[10px] font-mono bg-white/[0.06] text-[#8e95a2]">
                  {src.itemCount || 0} игр
                </span>
              </div>
              <div class="flex items-center gap-3 text-[11px] font-mono text-[#64748b]">
                <span class="truncate max-w-md">{src.url}</span>
                <span>•</span>
                <span>Синхр: {src.lastSynced ? new Date(src.lastSynced * 1000).toLocaleString() : 'Никогда'}</span>
              </div>
            </div>

            <div class="flex items-center gap-3 flex-shrink-0">
              <!-- Steam-styled Toggle Switch -->
              <button
                data-nav-item
                type="button"
                aria-label={src.enabled ? "Отключить источник" : "Включить источник"}
                title={src.enabled ? "Отключить источник" : "Включить источник"}
                class="w-12 h-6 rounded-full transition-colors cursor-pointer relative {src.enabled ? 'bg-sky-500' : 'bg-white/10'} focus:ring-2 focus:ring-white focus:outline-none"
                onclick={() => handleToggleTorrentSource(src.id, src.enabled)}
              >
                <span class="w-4 h-4 rounded-full bg-white transition-transform absolute top-1 {src.enabled ? 'left-7' : 'left-1'}"></span>
              </button>

              <!-- Delete Button -->
              <button
                data-nav-item
                type="button"
                class="p-2 rounded-xl text-[#8e95a2] hover:text-rose-400 hover:bg-rose-500/10 focus:ring-2 focus:ring-white focus:outline-none transition-colors cursor-pointer"
                onclick={() => handleRemoveTorrentSource(src.id)}
                title="Удалить источник"
              >
                <Trash2 class="w-4 h-4" />
              </button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>
