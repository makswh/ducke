<script lang="ts">
  import { onMount } from 'svelte';
  import { gamepad, type NavZone } from '../navigation/gamepad';

  let { isVisible = true } = $props();

  let activeZone = $state<NavZone>('list');

  onMount(() => {
    gamepad.onZoneChange = (zone) => {
      activeZone = zone;
    };
  });
</script>

{#if isVisible}
  <div class="fixed bottom-4 right-6 z-40 pointer-events-none hidden md:flex items-center gap-3 px-3.5 py-1.5 rounded-xl bg-[#0d0f14]/95 backdrop-blur-lg border border-white/[0.1] text-[11px] font-semibold text-[#9ca3af] shadow-2xl transition-all duration-300">
    
    <!-- Button A -->
    <div class="flex items-center gap-1.5">
      <span class="w-4 h-4 rounded-full bg-emerald-500 text-black flex items-center justify-center text-[10px] font-black shadow-sm">A</span>
      <span class="text-[#e2e8f0]">
        {#if activeZone === 'detail'}
          Действие
        {:else if activeZone === 'modal'}
          Выбрать
        {:else}
          Открыть
        {/if}
      </span>
    </div>

    <!-- Button B -->
    <div class="flex items-center gap-1.5">
      <span class="w-4 h-4 rounded-full bg-rose-500 text-white flex items-center justify-center text-[10px] font-black shadow-sm">B</span>
      <span class="text-[#e2e8f0]">
        {#if activeZone === 'modal'}
          Закрыть
        {:else if activeZone === 'detail'}
          К списку
        {:else}
          Назад
        {/if}
      </span>
    </div>

    <!-- Button X -->
    <div class="flex items-center gap-1.5">
      <span class="w-4 h-4 rounded-full bg-sky-400 text-black flex items-center justify-center text-[10px] font-black shadow-sm">X</span>
      <span class="text-[#e2e8f0]">Поиск</span>
    </div>

    <!-- Button Y -->
    <div class="flex items-center gap-1.5">
      <span class="w-4 h-4 rounded-full bg-amber-400 text-black flex items-center justify-center text-[10px] font-black shadow-sm">Y</span>
      <span class="text-[#e2e8f0]">Фильтры</span>
    </div>

    <!-- Bumpers LB/RB -->
    <div class="flex items-center gap-1.5 border-l border-white/[0.08] pl-2.5">
      <span class="px-1.5 py-0.5 rounded-md bg-white/[0.08] border border-white/[0.1] text-white text-[9px] font-bold">LB / RB</span>
      <span class="text-[#cbd5e1]">Вкладки</span>
    </div>

    <!-- Triggers LT/RT -->
    <div class="flex items-center gap-1.5 border-l border-white/[0.08] pl-2.5">
      <span class="px-1.5 py-0.5 rounded-md bg-white/[0.08] border border-white/[0.1] text-white text-[9px] font-bold">LT / RT</span>
      <span class="text-[#cbd5e1]">Секции</span>
    </div>

  </div>
{/if}
