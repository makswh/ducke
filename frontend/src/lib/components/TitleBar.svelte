<script lang="ts">
  import { onMount } from 'svelte';
  import { Minus, Square, Copy, X, Tv } from 'lucide-svelte';
  import {
    WindowMinimise,
    WindowToggleMaximise,
    WindowIsMaximised,
    Quit
  } from '../../../wailsjs/runtime/runtime';

  let { onToggleBigPicture = () => {} } = $props();

  let isMaximised = $state<boolean>(false);

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

<!-- Custom Seamless Window Titlebar -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<header
  class="h-[38px] w-full bg-[#07080a] border-b border-white/[0.06] flex items-center justify-between select-none z-50 flex-shrink-0"
  style="--wails-draggable: drag;"
  ondblclick={handleToggleMaximise}
>
  <!-- Left: App Branding & Status -->
  <div class="flex items-center gap-2.5 px-3.5 pointer-events-none">
    <div class="w-5 h-5 rounded-md bg-white/[0.06] border border-white/10 flex items-center justify-center text-white font-mono font-bold text-[10px] leading-none">
      D
    </div>
    <span class="text-[11px] font-bold tracking-[0.16em] text-[#cbd5e1] uppercase font-mono">DUCKE</span>
  </div>

  <!-- Center: Draggable Area -->
  <div class="flex-1 h-full" style="--wails-draggable: drag;"></div>

  <!-- Right: Window Action Buttons -->
  <div class="flex items-center h-full gap-1 pr-1.5" style="--wails-draggable: no-drag;">
    <!-- Big Picture Console Mode Button -->
    <button
      type="button"
      class="h-6.5 px-2.5 rounded-md flex items-center gap-1.5 text-[11px] font-medium text-[#94a3b8] hover:text-white bg-white/[0.03] hover:bg-white/[0.08] border border-white/[0.06] transition-colors cursor-pointer mr-1"
      onclick={onToggleBigPicture}
      title="Режим Big Picture (Консольный вид)"
      aria-label="Режим Big Picture"
    >
      <Tv class="w-3.5 h-3.5 text-sky-400" />
      <span class="hidden sm:inline">Big Picture</span>
    </button>

    <div class="h-3.5 w-px bg-white/[0.08] mr-0.5"></div>

    <!-- Minimize -->
    <button
      type="button"
      class="h-7 w-8 rounded-md flex items-center justify-center text-[#8e95a2] hover:text-white hover:bg-white/[0.08] active:bg-white/[0.12] transition-colors cursor-pointer"
      onclick={handleMinimise}
      title="Свернуть"
      aria-label="Свернуть"
    >
      <Minus class="w-3.5 h-3.5 stroke-[2]" />
    </button>

    <!-- Maximize / Restore -->
    <button
      type="button"
      class="h-7 w-8 rounded-md flex items-center justify-center text-[#8e95a2] hover:text-white hover:bg-white/[0.08] active:bg-white/[0.12] transition-colors cursor-pointer"
      onclick={handleToggleMaximise}
      title={isMaximised ? "Восстановить" : "Развернуть"}
      aria-label={isMaximised ? "Восстановить" : "Развернуть"}
    >
      {#if isMaximised}
        <Copy class="w-3 h-3 stroke-[2] rotate-180" />
      {:else}
        <Square class="w-3 h-3 stroke-[2]" />
      {/if}
    </button>

    <!-- Close -->
    <button
      type="button"
      class="h-7 w-8 rounded-md flex items-center justify-center text-[#8e95a2] hover:text-white hover:bg-[#e81123] active:bg-[#c4101f] transition-colors cursor-pointer"
      onclick={handleClose}
      title="Закрыть"
      aria-label="Закрыть"
    >
      <X class="w-3.5 h-3.5 stroke-[2]" />
    </button>
  </div>
</header>
