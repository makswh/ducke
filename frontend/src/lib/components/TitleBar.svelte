<script lang="ts">
  import { onMount } from 'svelte';
  import { Minus, Square, Copy, X, TelevisionSimple } from 'phosphor-svelte';
  import {
    WindowMinimise,
    WindowToggleMaximise,
    WindowIsMaximised,
    Quit
  } from '../../../wailsjs/runtime/runtime';

  let {
    onToggleBigPicture = () => {},
    activeTab = 'catalog'
  } = $props();

  let isMaximised = $state<boolean>(false);

  function getTabTitle(tab: string) {
    switch (tab) {
      case 'catalog': return 'Каталог игр';
      case 'torrents': return 'Торренты';
      case 'collections': return 'Подборки';
      case 'favorites': return 'Избранное';
      case 'downloads': return 'Загрузки';
      case 'settings': return 'Настройки';
      default: return '';
    }
  }

  async function checkMaximised() {
    try {
      isMaximised = await WindowIsMaximised();
    } catch {
      // ignore
    }
  }

  function handleMinimise() {
    try {
      WindowMinimise();
    } catch (e) {
      console.error(e);
    }
  }

  function handleToggleMaximise() {
    try {
      WindowToggleMaximise();
      setTimeout(checkMaximised, 100);
    } catch (e) {
      console.error(e);
    }
  }

  function handleClose() {
    try {
      Quit();
    } catch (e) {
      console.error(e);
    }
  }

  onMount(() => {
    checkMaximised();
    window.addEventListener('resize', checkMaximised);
    return () => {
      window.removeEventListener('resize', checkMaximised);
    };
  });
</script>

<!-- Custom Seamless Window Titlebar (Modern 48px TopBar) -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<header
  class="h-12 w-full bg-[#0F1218] flex items-center justify-between select-none z-50 flex-shrink-0 px-3"
  style="--wails-draggable: drag;"
  ondblclick={handleToggleMaximise}
>
  <!-- Left: App Branding & Breadcrumb Status -->
  <div class="flex items-center gap-2.5 pointer-events-none">
    <div class="w-7 h-7 rounded-[10px] bg-white/[0.04] flex items-center justify-center overflow-hidden">
      <img src="/appicon.png" alt="Ducke" class="w-5 h-5 object-contain" />
    </div>
    <div class="flex items-center gap-1.5 font-mono text-[11px] tracking-wider uppercase">
      <span class="font-bold text-[#cbd5e1]">DUCKE</span>
      {#if getTabTitle(activeTab)}
        <span class="text-white/20">/</span>
        <span class="text-[#8e95a2] font-medium font-sans capitalize">{getTabTitle(activeTab)}</span>
      {/if}
    </div>
  </div>

  <!-- Center: Draggable Area -->
  <div class="flex-1 h-full" style="--wails-draggable: drag;"></div>

  <!-- Right: Window Action Buttons & Big Picture Switcher -->
  <div class="flex items-center h-full gap-1.5" style="--wails-draggable: no-drag;">
    <!-- Big Picture Console Mode Button (Modern Squircle Pill) -->
    <button
      type="button"
      class="h-8 px-3 rounded-[12px] flex items-center gap-2 text-[11px] font-medium text-[#94a3b8] hover:text-white bg-white/[0.03] hover:bg-white/[0.08] transition-all cursor-pointer mr-1"
      onclick={onToggleBigPicture}
      title="Режим Big Picture (Консольный вид)"
      aria-label="Режим Big Picture"
    >
      <TelevisionSimple size={15} weight="bold" class="text-sky-400" />
      <span class="hidden sm:inline">Big Picture</span>
    </button>

    <!-- Minimize (Squircle button) -->
    <button
      type="button"
      class="h-8 w-8 rounded-[8px] flex items-center justify-center text-[#8e95a2] hover:text-white hover:bg-white/[0.08] active:bg-white/[0.12] transition-colors cursor-pointer"
      onclick={handleMinimise}
      title="Свернуть"
      aria-label="Свернуть"
    >
      <Minus size={13} weight="bold" />
    </button>

    <!-- Maximize / Restore -->
    <button
      type="button"
      class="h-8 w-8 rounded-[8px] flex items-center justify-center text-[#8e95a2] hover:text-white hover:bg-white/[0.08] active:bg-white/[0.12] transition-colors cursor-pointer"
      onclick={handleToggleMaximise}
      title={isMaximised ? "Восстановить" : "Развернуть"}
      aria-label={isMaximised ? "Восстановить" : "Развернуть"}
    >
      {#if isMaximised}
        <Copy size={13} weight="regular" class="rotate-180" />
      {:else}
        <Square size={13} weight="regular" />
      {/if}
    </button>

    <!-- Close -->
    <button
      type="button"
      class="h-8 w-8 rounded-[8px] flex items-center justify-center text-[#8e95a2] hover:text-white hover:bg-[#e81123] active:bg-[#c4101f] transition-colors cursor-pointer"
      onclick={handleClose}
      title="Закрыть"
      aria-label="Закрыть"
    >
      <X size={13} weight="bold" />
    </button>
  </div>
</header>
