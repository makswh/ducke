<script lang="ts">
  import { onMount } from 'svelte';
  import { X, GameController, Command } from 'phosphor-svelte';

  let { isOpen = false, onClose = () => {} } = $props();

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape' && isOpen) {
      onClose();
      e.stopPropagation();
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  });
</script>

{#if isOpen}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 select-none animate-fade-in"
    onclick={(e) => {
      if (e.target === e.currentTarget) onClose();
    }}
    role="presentation"
  >
    <div
      class="w-full max-w-xl bg-[#0d1117] border border-white/10 rounded-[18px] shadow-2xl overflow-hidden flex flex-col text-slate-200"
      role="dialog"
      aria-modal="true"
    >
      <!-- Modal Header -->
      <div class="px-6 py-4 border-b border-white/[0.08] flex items-center justify-between bg-white/[0.02]">
        <div class="flex items-center gap-2.5">
          <Command size={18} weight="bold" class="text-sky-400" />
          <h2 class="text-xs font-bold uppercase tracking-wider text-white">Горячие клавиши и управление</h2>
        </div>
        <button
          type="button"
          class="w-8 h-8 rounded-[8px] flex items-center justify-center text-[#8e95a2] hover:text-white hover:bg-white/10 transition-colors cursor-pointer"
          onclick={onClose}
          aria-label="Закрыть"
        >
          <X size={15} />
        </button>
      </div>

      <!-- Modal Body -->
      <div class="p-6 space-y-6 max-h-[75vh] overflow-y-auto font-sans text-xs">
        <!-- Section 1: Keyboard -->
        <div class="space-y-3">
          <div class="flex items-center gap-2 text-white font-semibold text-xs pb-1 border-b border-white/[0.06]">
            <span>Клавиатура</span>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[12px]">
            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Режим Big Picture</span>
              <kbd class="px-2 py-0.5 rounded-[6px] bg-white/10 text-white font-mono text-[11px] font-bold border border-white/10">F11</kbd>
            </div>

            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Обновить каталоги</span>
              <kbd class="px-2 py-0.5 rounded-[6px] bg-white/10 text-white font-mono text-[11px] font-bold border border-white/10">F5</kbd>
            </div>

            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Настройки Ducke</span>
              <kbd class="px-2 py-0.5 rounded-[6px] bg-white/10 text-white font-mono text-[11px] font-bold border border-white/10">Ctrl + ,</kbd>
            </div>

            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Поиск в каталоге</span>
              <kbd class="px-2 py-0.5 rounded-[6px] bg-white/10 text-white font-mono text-[11px] font-bold border border-white/10">Ctrl + F</kbd>
            </div>

            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Назад / Закрыть</span>
              <kbd class="px-2 py-0.5 rounded-[6px] bg-white/10 text-white font-mono text-[11px] font-bold border border-white/10">Esc</kbd>
            </div>

            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Выход из Ducke</span>
              <kbd class="px-2 py-0.5 rounded-[6px] bg-white/10 text-white font-mono text-[11px] font-bold border border-white/10">Alt + F4</kbd>
            </div>
          </div>
        </div>

        <!-- Section 2: Gamepad -->
        <div class="space-y-3">
          <div class="flex items-center gap-2 text-white font-semibold text-xs pb-1 border-b border-white/[0.06]">
            <GameController size={15} class="text-sky-400" />
            <span>Геймпад (Xbox / Steam Deck / PlayStation)</span>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[12px]">
            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Выбрать / Открыть</span>
              <div class="flex items-center gap-1.5">
                <span class="w-4 h-4 rounded-full bg-emerald-500 text-black font-black text-[10px] flex items-center justify-center">A</span>
                <span class="text-[#64748b] text-[10px]">(✕)</span>
              </div>
            </div>

            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Назад / Отмена</span>
              <div class="flex items-center gap-1.5">
                <span class="w-4 h-4 rounded-full bg-rose-500 text-white font-black text-[10px] flex items-center justify-center">B</span>
                <span class="text-[#64748b] text-[10px]">(○)</span>
              </div>
            </div>

            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Опции / Поиск</span>
              <div class="flex items-center gap-1.5">
                <span class="w-4 h-4 rounded-full bg-sky-400 text-black font-black text-[10px] flex items-center justify-center">X</span>
                <span class="text-[#64748b] text-[10px]">(□)</span>
              </div>
            </div>

            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Избранное / Фильтры</span>
              <div class="flex items-center gap-1.5">
                <span class="w-4 h-4 rounded-full bg-amber-400 text-black font-black text-[10px] flex items-center justify-center">Y</span>
                <span class="text-[#64748b] text-[10px]">(△)</span>
              </div>
            </div>

            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Переключение вкладок</span>
              <span class="px-2 py-0.5 rounded-[6px] bg-white/10 text-white font-mono text-[10px] font-bold border border-white/10">LB / RB</span>
            </div>

            <div class="flex items-center justify-between p-2 rounded-[10px] bg-white/[0.03] border border-white/[0.04]">
              <span class="text-[#cbd5e1]">Быстрая прокрутка страниц</span>
              <span class="px-2 py-0.5 rounded-[6px] bg-white/10 text-white font-mono text-[10px] font-bold border border-white/10">LT / RT</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="px-6 py-3 border-t border-white/[0.08] flex items-center justify-end bg-white/[0.02]">
        <button
          type="button"
          class="px-4 py-2 rounded-[10px] bg-white/10 hover:bg-white/15 text-white text-xs font-semibold border border-white/10 cursor-pointer transition-colors"
          onclick={onClose}
        >
          Понятно
        </button>
      </div>
    </div>
  </div>
{/if}
