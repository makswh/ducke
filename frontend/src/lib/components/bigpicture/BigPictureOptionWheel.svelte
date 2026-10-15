<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import type { ComponentType } from 'svelte';
  import { sound } from '../../navigation/audio';
  import { gamepad } from '../../navigation/gamepad';

  export interface WheelOption {
    id: string;
    label: string;
    icon?: ComponentType<any>;
    danger?: boolean;
    onSelect: () => void;
  }

  let props = $props<{
    items: WheelOption[];
    title?: string;
    subtitle?: string;
    defaultSelected?: number;
    side?: 'left' | 'right';
    fontSize?: number;
    spacing?: number;
    curve?: number;
    tilt?: number;
    blur?: number;
    fade?: number;
    minOpacity?: number;
    smoothing?: number;
    loop?: boolean;
    draggable?: boolean;
    onClose: () => void;
  }>();

  let items = $derived(props.items || []);
  let title = $derived(props.title || 'Действия');
  let subtitle = $derived(props.subtitle || 'Опции игры');
  let side = $derived(props.side || 'left');
  let fontSize = $derived(props.fontSize ?? 1.3);
  let spacing = $derived(props.spacing ?? 2.8);
  let curve = $derived(props.curve ?? 1.0);
  let tilt = $derived(props.tilt ?? 7.5);
  let blur = $derived(props.blur ?? 1.5);
  let fade = $derived(props.fade ?? 0.35);
  let minOpacity = $derived(props.minOpacity ?? 0.12);
  let smoothing = $derived(props.smoothing ?? 140);
  let loop = $derived(props.loop ?? true);
  let draggable = $derived(props.draggable ?? true);
  const onClose = () => props.onClose?.();

  let rootRef: HTMLDivElement | null = null;
  let itemEls: HTMLElement[] = [];

  function registerItem(node: HTMLElement, idx: number) {
    itemEls[idx] = node;
    return {
      update(newIdx: number) {
        if (newIdx !== idx) {
          delete itemEls[idx];
          idx = newIdx;
          itemEls[idx] = node;
        }
      },
      destroy() {
        delete itemEls[idx];
      }
    };
  }

  let selectedIndex = $state<number>(0);
  let isDragging = $state<boolean>(false);

  // Animation & Physics state (plain non-reactive refs for guaranteed 60/120 FPS performance)
  let pos = 0;
  let target = 0;
  let rafId: number | null = null;
  let lastTime = 0;
  let wheelTimer: any = null;
  let dragStart: { y: number; start: number; id: number } | null = null;
  let dragMoved = false;

  function getRowH(): number {
    const remPx = typeof window !== 'undefined' ? parseFloat(getComputedStyle(document.documentElement).fontSize) || 16 : 16;
    return Math.max(fontSize * spacing * remPx, 56);
  }

  function runFrame(now: number) {
    const dt = Math.min((now - lastTime) / 1000, 0.05);
    lastTime = now;
    const tau = Math.max(smoothing, 1) / 1000;
    const k = 1 - Math.exp(-dt / tau);

    let next = pos + (target - pos) * k;
    const settled = Math.abs(target - next) < 0.001;
    if (settled) next = target;
    pos = next;

    const n = items.length;
    const mirror = side === 'right' ? -1 : 1;
    const tiltRad = (tilt * Math.PI) / 180;
    const rowH = getRowH();
    const R = tiltRad > 0.0005 ? rowH / tiltRad : 0;

    for (let i = 0; i < n; i++) {
      const el = itemEls[i];
      if (!el) continue;
      let d = i - next;
      if (loop && n > 1) {
        d = ((d % n) + n) % n;
        if (d > n / 2) d -= n;
      }
      const dist = Math.abs(d);
      let x = 0;
      let y = d * rowH;
      let rot = 0;
      if (R > 0) {
        const ang = Math.max(-Math.PI / 2, Math.min(Math.PI / 2, d * tiltRad));
        y = R * Math.sin(ang);
        x = -mirror * R * (1 - Math.cos(ang)) * curve;
        rot = (mirror * ang * 180) / Math.PI;
      }

      el.style.transform = `translate3d(${x.toFixed(2)}px, calc(${y.toFixed(2)}px - 50%), 0) rotate(${rot.toFixed(3)}deg)`;
      el.style.opacity = String(Math.max(minOpacity, 1 - dist * fade));
      el.style.filter = blur > 0 ? `blur(${(dist * blur).toFixed(2)}px)` : 'none';
      el.style.setProperty('--ow-p', Math.max(0, 1 - Math.min(dist, 1)).toFixed(4));
    }

    if (!settled) {
      rafId = requestAnimationFrame(runFrame);
    } else {
      rafId = null;
    }
  }

  function startLoop() {
    if (rafId !== null) {
      cancelAnimationFrame(rafId);
    }
    lastTime = performance.now();
    rafId = requestAnimationFrame(runFrame);
  }

  function applyTarget(value: number, snap: boolean) {
    const n = items.length;
    if (n === 0) return;
    let v = value;
    if (!loop) v = Math.min(Math.max(v, 0), Math.max(n - 1, 0));
    if (snap) v = Math.round(v);
    target = v;

    const idx = ((Math.round(v) % n) + n) % n;
    if (idx !== selectedIndex) {
      selectedIndex = idx;
      sound.playMove();
      if (itemEls[idx]) {
        gamepad.setFocus(itemEls[idx]);
      }
    }
    startLoop();
  }

  let lastStepTime = 0;
  const STEP_COOLDOWN_MS = 135;

  function step(delta: number) {
    const now = performance.now();
    if (now - lastStepTime < STEP_COOLDOWN_MS) {
      return;
    }
    lastStepTime = now;
    const direction = delta > 0 ? 1 : -1;
    applyTarget(Math.round(target) + direction, true);
  }

  function activateSelected() {
    if (items.length === 0) return;
    const cur = items[selectedIndex];
    if (cur && typeof cur.onSelect === 'function') {
      sound.playSelect();
      onClose();
      cur.onSelect();
    }
  }

  let lastWheelTime = 0;
  function handleWheel(e: WheelEvent) {
    e.preventDefault();
    const now = performance.now();
    if (now - lastWheelTime < 110) return;
    const delta = e.deltaY;
    if (Math.abs(delta) < 6) return;
    lastWheelTime = now;
    step(delta > 0 ? 1 : -1);
  }

  function handlePointerDown(e: PointerEvent) {
    if (!draggable) return;
    dragStart = { y: e.clientY, start: target, id: e.pointerId };
    dragMoved = false;
    isDragging = true;
  }

  function handlePointerMove(e: PointerEvent) {
    if (!dragStart) return;
    const rowH = getRowH();
    const dy = e.clientY - dragStart.y;
    if (!dragMoved && Math.abs(dy) > 4) {
      dragMoved = true;
      rootRef?.setPointerCapture(dragStart.id);
    }
    if (dragMoved) {
      applyTarget(dragStart.start - dy / rowH, false);
    }
  }

  function handlePointerEnd() {
    if (!dragStart) return;
    dragStart = null;
    isDragging = false;
    if (dragMoved) {
      applyTarget(target, true);
    }
  }

  function handleItemClick(index: number) {
    if (dragMoved) return;
    if (index === selectedIndex) {
      activateSelected();
    } else {
      const n = items.length;
      let d = index - (((Math.round(target) % n) + n) % n);
      if (loop && n > 1) {
        if (d > n / 2) d -= n;
        else if (d < -n / 2) d += n;
      }
      applyTarget(target + d, true);
    }
  }

  onMount(() => {
    // Notify gamepad engine of modal open so focusStack saves previous element
    window.dispatchEvent(new CustomEvent('app:modal-opened'));

    const initial = Math.max(0, Math.min(props.defaultSelected || 0, items.length - 1));
    pos = initial;
    target = initial;
    selectedIndex = initial;
    // Initial grace period so opening key/stick deflection doesn't trigger instant step
    lastStepTime = performance.now() + 100;

    // Immediately trigger layout on next frame once elements are in DOM
    requestAnimationFrame(() => {
      startLoop();
      if (itemEls[initial]) {
        gamepad.setFocus(itemEls[initial]);
      }
    });

    const handleKeyDown = (e: KeyboardEvent) => {
      if (
        e.key === 'Escape' ||
        e.key === 'Backspace' ||
        e.key === 'b' ||
        e.key === 'B' ||
        e.key === 'и' ||
        e.key === 'И' ||
        e.code === 'KeyB' ||
        e.key === 'x' ||
        e.key === 'X' ||
        e.key === 'o' ||
        e.key === 'O' ||
        e.key === 'ContextMenu'
      ) {
        e.preventDefault();
        e.stopPropagation();
        e.stopImmediatePropagation();
        sound.playBack();
        onClose();
        return;
      }

      if (e.key === 'ArrowUp' || e.key === 'w' || e.key === 'W' || e.key === 'ц' || e.key === 'Ц') {
        e.preventDefault();
        e.stopPropagation();
        e.stopImmediatePropagation();
        step(-1);
        return;
      }

      if (e.key === 'ArrowDown' || e.key === 's' || e.key === 'S' || e.key === 'ы' || e.key === 'Ы') {
        e.preventDefault();
        e.stopPropagation();
        e.stopImmediatePropagation();
        step(1);
        return;
      }

      // Absorb horizontal navigation so shelves don't scroll underneath
      if (
        e.key === 'ArrowLeft' ||
        e.key === 'ArrowRight' ||
        e.key === 'a' ||
        e.key === 'A' ||
        e.key === 'd' ||
        e.key === 'D' ||
        e.key === 'ф' ||
        e.key === 'Ф' ||
        e.key === 'в' ||
        e.key === 'В'
      ) {
        e.preventDefault();
        e.stopPropagation();
        e.stopImmediatePropagation();
        return;
      }

      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        e.stopPropagation();
        e.stopImmediatePropagation();
        activateSelected();
        return;
      }
    };

    const handleGamepadDir = (e: CustomEvent) => {
      e.preventDefault();
      e.stopImmediatePropagation();
      const dir = e.detail?.dir;
      if (dir === 'UP') {
        step(-1);
      } else if (dir === 'DOWN') {
        step(1);
      }
    };

    const handleBtnA = (e: Event) => {
      e.preventDefault();
      e.stopImmediatePropagation();
      activateSelected();
    };

    const handleBtnB = (e: Event) => {
      e.preventDefault();
      e.stopImmediatePropagation();
      sound.playBack();
      onClose();
    };

    const handleBtnX = (e: Event) => {
      e.preventDefault();
      e.stopImmediatePropagation();
      sound.playBack();
      onClose();
    };

    const handleModalClose = (e: Event) => {
      e.preventDefault();
      e.stopImmediatePropagation();
      sound.playBack();
      onClose();
    };

    const handleGoBack = (e: Event) => {
      e.preventDefault();
      e.stopImmediatePropagation();
      sound.playBack();
      onClose();
    };

    window.addEventListener('keydown', handleKeyDown, true);
    window.addEventListener('app:gamepad-dir', handleGamepadDir as EventListener, true);
    window.addEventListener('app:btn-a', handleBtnA as EventListener, true);
    window.addEventListener('app:btn-b', handleBtnB as EventListener, true);
    window.addEventListener('app:btn-x', handleBtnX as EventListener, true);
    window.addEventListener('app:modal-close', handleModalClose as EventListener, true);
    window.addEventListener('app:go-back', handleGoBack as EventListener, true);

    return () => {
      window.removeEventListener('keydown', handleKeyDown, true);
      window.removeEventListener('app:gamepad-dir', handleGamepadDir as EventListener, true);
      window.removeEventListener('app:btn-a', handleBtnA as EventListener, true);
      window.removeEventListener('app:btn-b', handleBtnB as EventListener, true);
      window.removeEventListener('app:btn-x', handleBtnX as EventListener, true);
      window.removeEventListener('app:modal-close', handleModalClose as EventListener, true);
      window.removeEventListener('app:go-back', handleGoBack as EventListener, true);
      if (wheelTimer) clearTimeout(wheelTimer);
      if (rafId !== null) {
        cancelAnimationFrame(rafId);
        rafId = null;
      }
      // Notify gamepad engine modal closed to restore focus
      window.dispatchEvent(new CustomEvent('app:modal-closed'));
    };
  });
</script>

<!-- 1. Fullscreen Backdrop with cinematic dimming and smooth fade-in -->
<div
  class="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex items-center select-none bp-wheel-backdrop"
  data-nav-zone="modal"
  onclick={(e) => {
    if (e.target === e.currentTarget) {
      sound.playBack();
      onClose();
    }
  }}
  role="presentation"
>
  <!-- Left Radial Dark Vignette to accent the wheel -->
  <div class="absolute inset-0 bg-gradient-to-r from-black/90 via-black/50 to-transparent pointer-events-none"></div>

  <!-- 2. Wheel Stage Container -->
  <div class="relative z-10 w-full max-w-2xl h-full flex flex-col justify-between py-10 px-8 sm:px-14 lg:px-20">
    
    <!-- Top Title Header -->
    <div class="space-y-1 bp-wheel-header">
      <span class="text-[11px] font-black uppercase tracking-widest text-[#4C7DFF] font-mono">
        {subtitle}
      </span>
      <h2 class="text-2xl sm:text-3xl font-black text-white tracking-tight line-clamp-1">
        {title}
      </h2>
    </div>

    <!-- Center Wheel Stage -->
    <div class="relative w-full h-[480px] my-auto flex items-center">
      <!-- Physical Center Focus Cradle -->
      <div class="absolute inset-x-0 top-1/2 -translate-y-1/2 h-14 rounded-[16px] bg-white/[0.08] border border-white/20 shadow-2xl pointer-events-none flex items-center justify-between px-5 z-0">
        <!-- Active Pill Accent on left -->
        <div class="w-1.5 h-6 rounded-full bg-[#4C7DFF]"></div>

        <!-- Tactile A Button Prompt on right -->
        <div class="flex items-center gap-1.5 px-2.5 py-1 rounded-[8px] bg-white/10 text-white text-[11px] font-bold">
          <span class="w-4 h-4 rounded-full bg-white text-black text-[9px] font-black flex items-center justify-center">A</span>
          <span>Применить</span>
        </div>
      </div>

      <!-- The Kinetic 3D OptionWheel Area -->
      <div
        bind:this={rootRef}
        role="listbox"
        tabindex="0"
        aria-label="Option Wheel"
        class="relative w-full h-full overflow-hidden cursor-grab touch-none outline-none {isDragging ? 'cursor-grabbing' : ''}"
        onwheel={handleWheel}
        onpointerdown={handlePointerDown}
        onpointermove={handlePointerMove}
        onpointerup={handlePointerEnd}
        onpointercancel={handlePointerEnd}
      >
        {#each items as item, index (item.id || index)}
          <button
            use:registerItem={index}
            role="option"
            type="button"
            data-nav-item
            aria-selected={selectedIndex === index}
            class="bp-wheel-option absolute top-1/2 left-7 sm:left-9 flex items-center gap-3.5 px-4 py-2.5 rounded-[12px] whitespace-nowrap cursor-pointer transition-colors will-change-transform z-10 {selectedIndex === index ? 'text-white font-bold text-base sm:text-lg' : 'text-[#8e95a2] font-semibold text-sm sm:text-base'}"
            style="font-size: {fontSize}rem;"
            onclick={() => handleItemClick(index)}
          >
            {#if item.icon}
              {@const Icon = item.icon}
              <Icon
                class="w-5 h-5 flex-shrink-0 transition-transform {item.danger ? 'text-rose-400' : (selectedIndex === index ? 'text-[#4C7DFF] scale-110' : 'text-[#8e95a2]')}"
              />
            {/if}
            <span class="truncate">{item.label}</span>
          </button>
        {/each}
      </div>
    </div>

    <!-- Bottom Tactile Action HUD Bar -->
    <div class="flex items-center gap-6 pt-4 border-t border-white/[0.08] text-xs font-semibold text-[#8e95a2] bp-wheel-footer">
      <div class="flex items-center gap-1.5">
        <span class="w-5 h-5 rounded-full bg-emerald-500 text-black font-black text-[10px] flex items-center justify-center shadow-sm">A</span>
        <span class="text-white">Выбрать</span>
      </div>
      <div class="flex items-center gap-1.5">
        <span class="w-5 h-5 rounded-full bg-rose-500 text-white font-black text-[10px] flex items-center justify-center shadow-sm">B</span>
        <span class="text-[#cbd5e1]">Закрыть</span>
      </div>
      <div class="flex items-center gap-1.5">
        <span class="px-2 py-0.5 rounded-[6px] bg-white/[0.08] border border-white/10 text-white font-mono text-[10px] font-bold">↑ / ↓</span>
        <span class="text-[#cbd5e1]">Вращение</span>
      </div>
    </div>

  </div>
</div>

<style>
  @keyframes bpWheelBackdropFade {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  @keyframes bpWheelSlideIn {
    from {
      opacity: 0;
      transform: translateX(-35px);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }

  :global(.bp-wheel-backdrop) {
    animation: bpWheelBackdropFade 0.22s cubic-bezier(0.16, 1, 0.3, 1) forwards;
  }

  :global(.bp-wheel-header),
  :global(.bp-wheel-footer) {
    animation: bpWheelSlideIn 0.28s cubic-bezier(0.16, 1, 0.3, 1) forwards;
  }

  :global(.bp-wheel-option) {
    transform-origin: left center !important;
  }

  :global(.bp-wheel-option.gamepad-focused) {
    box-shadow: none !important;
    border-color: transparent !important;
  }
</style>
