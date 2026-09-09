import { sound } from './audio';

export type NavZone = 'sidebar' | 'list' | 'detail' | 'modal' | 'header' | 'grid';
type Direction = 'UP' | 'DOWN' | 'LEFT' | 'RIGHT';

export interface GamepadState {
  connected: boolean;
  id: string;
  activeZone: NavZone;
  focusedElementIndex: number;
}

export class GamepadEngine {
  private isRunning: boolean = false;
  private currentFocusedEl: HTMLElement | null = null;
  private lastButtonsState: boolean[] = [];
  private focusStack: HTMLElement[] = [];
  private activeZone: NavZone = 'list';
  private zoneMemory: Map<NavZone, HTMLElement> = new Map();

  // Console timing parameters
  private static readonly INITIAL_REPEAT_DELAY_MS = 360;
  private static readonly REPEAT_INTERVAL_MS = 110;
  private static readonly ANALOG_DEADZONE = 0.40;
  private static readonly SCROLL_DEADZONE = 0.22;

  // Directional hold tracking
  private heldDirection: Direction | null = null;
  private heldStartTime: number = 0;
  private lastStepTime: number = 0;

  // Controller tracking
  private lastActiveGamepadIndex: number = 0;
  private connectedGamepadIds: Set<string> = new Set();

  // Public Callbacks
  public onTabChange?: (direction: 'PREV' | 'NEXT') => void;
  public onBack?: () => void;
  public onSearch?: () => void;
  public onFilter?: () => void;
  public onControllerStateChange?: (connected: boolean, id: string) => void;
  public onZoneChange?: (zone: NavZone) => void;

  constructor() {
    if (typeof window === 'undefined') return;

    window.addEventListener('gamepadconnected', (e: GamepadEvent) => {
      console.log('[Ducke Gamepad] Connected:', e.gamepad.id, 'index:', e.gamepad.index);
      this.connectedGamepadIds.add(e.gamepad.id);
      this.lastActiveGamepadIndex = e.gamepad.index;
      this.onControllerStateChange?.(true, e.gamepad.id);

      // Auto-focus first visible item after connection
      setTimeout(() => {
        if (!this.currentFocusedEl || !document.body.contains(this.currentFocusedEl)) {
          this.focusFirstInZone('grid') || this.focusFirstInZone('list') || this.focusFirstInZone('detail');
        }
      }, 250);
    });

    window.addEventListener('gamepaddisconnected', (e: GamepadEvent) => {
      console.log('[Ducke Gamepad] Disconnected:', e.gamepad.id);
      this.connectedGamepadIds.delete(e.gamepad.id);

      const anyRemaining = this.getConnectedGamepads().length > 0;
      this.onControllerStateChange?.(anyRemaining, anyRemaining ? 'gamepad' : '');

      if (!anyRemaining && this.currentFocusedEl) {
        this.currentFocusedEl.classList.remove('gamepad-focused');
      }
    });

    window.addEventListener('app:modal-opened', () => {
      if (this.currentFocusedEl) {
        this.focusStack.push(this.currentFocusedEl);
      }
      setTimeout(() => this.focusFirstInZone('modal'), 60);
    });

    window.addEventListener('app:modal-closed', () => {
      const prev = this.focusStack.pop();
      if (prev && document.body.contains(prev) && this.isElementVisible(prev)) {
        this.setFocus(prev);
      } else {
        this.focusFirstInZone('detail') || this.focusFirstInZone('grid') || this.focusFirstInZone('list');
      }
    });
  }

  public start() {
    if (this.isRunning) return;
    this.isRunning = true;
    this.loop();
  }

  public stop() {
    this.isRunning = false;
  }

  private loop = () => {
    if (!this.isRunning) return;

    this.pollGamepad();
    requestAnimationFrame(this.loop);
  };

  private getConnectedGamepads(): Gamepad[] {
    const raw = navigator.getGamepads ? navigator.getGamepads() : [];
    const list: Gamepad[] = [];
    for (let i = 0; i < raw.length; i++) {
      const g = raw[i];
      if (g && g.connected) list.push(g);
    }
    return list;
  }

  /**
   * Selects the active controller dynamically based on current user interaction.
   */
  private getActiveGamepad(): Gamepad | null {
    const gamepads = this.getConnectedGamepads();
    if (gamepads.length === 0) return null;

    // Check if any controller is currently issuing input
    for (const gp of gamepads) {
      const hasButton = gp.buttons.some((b) => b && (b.pressed || b.value > 0.3));
      const hasAxis = gp.axes.some((a) => Math.abs(a) > 0.30);
      if (hasButton || hasAxis) {
        this.lastActiveGamepadIndex = gp.index;
        return gp;
      }
    }

    // Fall back to remembered controller index
    const remembered = gamepads.find((g) => g.index === this.lastActiveGamepadIndex);
    if (remembered) return remembered;

    // Fall back to the first available controller
    return gamepads[0] || null;
  }

  private pollGamepad() {
    const gp = this.getActiveGamepad();
    if (!gp) {
      this.heldDirection = null;
      return;
    }

    const now = Date.now();

    // 1. Right Stick Analog Scrolling (Smooth vertical scroll with acceleration curve)
    const rightAxisY = gp.axes[3] || 0;
    if (Math.abs(rightAxisY) > GamepadEngine.SCROLL_DEADZONE) {
      const sign = rightAxisY > 0 ? 1 : -1;
      const normalized = (Math.abs(rightAxisY) - GamepadEngine.SCROLL_DEADZONE) / (1.0 - GamepadEngine.SCROLL_DEADZONE);
      const scrollSpeed = sign * Math.pow(normalized, 1.4) * 26;
      this.scrollActivePanel(scrollSpeed);
    }

    // 2. D-Pad & Left Stick Directional Navigation with Dominant Axis Resolution
    const axisX = gp.axes[0] || 0;
    const axisY = gp.axes[1] || 0;
    const absX = Math.abs(axisX);
    const absY = Math.abs(axisY);

    let stickDir: Direction | null = null;
    if (absX > GamepadEngine.ANALOG_DEADZONE || absY > GamepadEngine.ANALOG_DEADZONE) {
      if (absX > absY) {
        stickDir = axisX > 0 ? 'RIGHT' : 'LEFT';
      } else {
        stickDir = axisY > 0 ? 'DOWN' : 'UP';
      }
    }

    const dpadUp = gp.buttons[12]?.pressed || false;
    const dpadDown = gp.buttons[13]?.pressed || false;
    const dpadLeft = gp.buttons[14]?.pressed || false;
    const dpadRight = gp.buttons[15]?.pressed || false;

    let dpadDir: Direction | null = null;
    if (dpadUp) dpadDir = 'UP';
    else if (dpadDown) dpadDir = 'DOWN';
    else if (dpadLeft) dpadDir = 'LEFT';
    else if (dpadRight) dpadDir = 'RIGHT';

    const currentDir: Direction | null = dpadDir || stickDir;

    // Console-grade Repeat State Machine
    if (currentDir) {
      if (this.heldDirection !== currentDir) {
        // First immediate tap (0ms latency)
        this.heldDirection = currentDir;
        this.heldStartTime = now;
        this.lastStepTime = now;
        this.navigateSpatial(currentDir);
      } else {
        // Sustained hold: wait initial repeat delay, then tick at repeat interval
        const holdDuration = now - this.heldStartTime;
        if (holdDuration >= GamepadEngine.INITIAL_REPEAT_DELAY_MS) {
          if (now - this.lastStepTime >= GamepadEngine.REPEAT_INTERVAL_MS) {
            this.lastStepTime = now;
            this.navigateSpatial(currentDir);
          }
        }
      }
    } else {
      this.heldDirection = null;
      this.heldStartTime = 0;
      this.lastStepTime = 0;
    }

    // 3. Action Buttons
    // (A) Cross / A: Click / Activate
    this.handleButtonPress(gp, 0, () => {
      sound.playSelect();
      if (this.currentFocusedEl) {
        this.currentFocusedEl.click();

        if (this.currentFocusedEl instanceof HTMLInputElement || this.currentFocusedEl instanceof HTMLTextAreaElement) {
          const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
          if (app && app.TriggerSteamOSKeyboard) {
            app.TriggerSteamOSKeyboard().catch(() => {});
          }
        }
      }
    });

    // (B) Circle / B: Back / Close Modal / Exit Detail
    this.handleButtonPress(gp, 1, () => {
      sound.playBack();
      const modalActive = this.isModalActive();
      if (modalActive) {
        window.dispatchEvent(new CustomEvent('app:modal-close'));
      } else {
        const ev = new CustomEvent('app:go-back', { cancelable: true });
        const notHandled = window.dispatchEvent(ev);
        if (notHandled) {
          if (this.activeZone === 'detail') {
            this.focusFirstInZone('grid') || this.focusFirstInZone('list');
          } else if (this.activeZone === 'list' || this.activeZone === 'grid') {
            this.focusFirstInZone('sidebar') || this.focusFirstInZone('header');
          }
          this.onBack?.();
        }
      }
    });

    // (X) Square / X: Focus Search Input / Toggle Search
    this.handleButtonPress(gp, 2, () => {
      sound.playSelect();
      window.dispatchEvent(new CustomEvent('app:toggle-search'));
      this.focusSearchInput();
      this.onSearch?.();
    });

    // (Y) Triangle / Y: Focus Filter Selector / Toggle Filter
    this.handleButtonPress(gp, 3, () => {
      sound.playSelect();
      window.dispatchEvent(new CustomEvent('app:toggle-filter'));
      this.focusFilterSelect();
      this.onFilter?.();
    });

    // LB (4): Previous Sub-tab / Previous Media / Previous Top Tab
    this.handleButtonPress(gp, 4, () => {
      sound.playTab();
      if (this.isModalActive()) {
        window.dispatchEvent(new CustomEvent('app:gallery-prev'));
      } else {
        const ev = new CustomEvent('app:subtab-prev', { cancelable: true });
        const notCancelled = window.dispatchEvent(ev);
        if (notCancelled) {
          this.onTabChange?.('PREV');
        }
      }
    });

    // RB (5): Next Sub-tab / Next Media / Next Top Tab
    this.handleButtonPress(gp, 5, () => {
      sound.playTab();
      if (this.isModalActive()) {
        window.dispatchEvent(new CustomEvent('app:gallery-next'));
      } else {
        const ev = new CustomEvent('app:subtab-next', { cancelable: true });
        const notCancelled = window.dispatchEvent(ev);
        if (notCancelled) {
          this.onTabChange?.('NEXT');
        }
      }
    });

    // LT (6): Switch Zone Left
    this.handleButtonPress(gp, 6, () => {
      this.switchZoneRelative(-1);
    });

    // RT (7): Switch Zone Right
    this.handleButtonPress(gp, 7, () => {
      this.switchZoneRelative(1);
    });

    // Select / View (8): Search or Quick Action
    this.handleButtonPress(gp, 8, () => {
      sound.playSelect();
      window.dispatchEvent(new CustomEvent('app:toggle-search'));
      this.focusSearchInput();
    });

    // Start / Menu (9): Toggle Settings / Menu
    this.handleButtonPress(gp, 9, () => {
      sound.playTab();
      window.dispatchEvent(new CustomEvent('app:toggle-menu'));
    });
  }

  private handleButtonPress(gp: Gamepad, index: number, callback: () => void) {
    const isPressed = gp.buttons[index]?.pressed || false;
    const wasPressed = this.lastButtonsState[index] || false;

    if (isPressed && !wasPressed) {
      callback();
    }

    this.lastButtonsState[index] = isPressed;
  }

  public isModalActive(): boolean {
    const modalItems = document.querySelectorAll<HTMLElement>('[data-nav-zone="modal"] [data-nav-item]');
    return modalItems.length > 0;
  }

  public getNavItems(zone?: NavZone): HTMLElement[] {
    if (this.isModalActive()) {
      return Array.from(
        document.querySelectorAll<HTMLElement>(
          '[data-nav-zone="modal"] [data-nav-item]:not([disabled]):not([style*="display: none"])'
        )
      ).filter((el) => this.isElementVisible(el));
    }

    let selector = '[data-nav-item]:not([disabled]):not([style*="display: none"])';
    if (zone) {
      selector = `[data-nav-zone="${zone}"] [data-nav-item]:not([disabled]):not([style*="display: none"]), [data-nav-zone="${zone}"][data-nav-item]:not([disabled]):not([style*="display: none"])`;
    }

    return Array.from(document.querySelectorAll<HTMLElement>(selector)).filter((el) => this.isElementVisible(el));
  }

  public setFocus(el: HTMLElement | null) {
    if (this.currentFocusedEl) {
      this.currentFocusedEl.classList.remove('gamepad-focused');
    }

    this.currentFocusedEl = el;

    if (this.currentFocusedEl) {
      this.currentFocusedEl.classList.add('gamepad-focused');
      this.currentFocusedEl.focus({ preventScroll: true });
      sound.playFocus();

      // Determine active zone
      const zoneEl = this.currentFocusedEl.closest<HTMLElement>('[data-nav-zone]');
      const zone = (zoneEl?.getAttribute('data-nav-zone') as NavZone) || 'list';
      this.activeZone = zone;
      this.zoneMemory.set(zone, this.currentFocusedEl);
      this.onZoneChange?.(zone);

      // Smooth scroll target into view
      this.currentFocusedEl.scrollIntoView({
        block: zone === 'list' ? 'center' : 'nearest',
        inline: 'nearest',
        behavior: 'smooth'
      });
    }
  }

  public focusFirstInZone(zone: NavZone): boolean {
    const remembered = this.zoneMemory.get(zone);
    if (remembered && document.body.contains(remembered) && this.isElementVisible(remembered)) {
      this.setFocus(remembered);
      return true;
    }

    const items = this.getNavItems(zone);
    if (items.length > 0) {
      this.setFocus(items[0]);
      return true;
    }
    return false;
  }

  /**
   * Discovers all visible data-nav-zone elements dynamically on screen.
   */
  public getActiveZones(): NavZone[] {
    const preferredOrder: NavZone[] = ['header', 'sidebar', 'list', 'grid', 'detail'];
    const activeOnScreen = new Set<NavZone>();

    const zoneElements = document.querySelectorAll<HTMLElement>('[data-nav-zone]');
    for (const el of Array.from(zoneElements)) {
      const zone = el.getAttribute('data-nav-zone') as NavZone;
      if (!zone || zone === 'modal') continue;
      const rect = el.getBoundingClientRect();
      if (rect.width > 0 && rect.height > 0) {
        const hasItems = el.querySelector('[data-nav-item]') !== null;
        if (hasItems) {
          activeOnScreen.add(zone);
        }
      }
    }

    return preferredOrder.filter((z) => activeOnScreen.has(z));
  }

  private switchZoneRelative(delta: number) {
    if (this.isModalActive()) return;

    const availableZones = this.getActiveZones();
    if (availableZones.length <= 1) return;

    const currentIndex = availableZones.indexOf(this.activeZone);
    let nextIndex: number;
    if (currentIndex === -1) {
      nextIndex = delta > 0 ? 0 : availableZones.length - 1;
    } else {
      nextIndex = (currentIndex + delta + availableZones.length) % availableZones.length;
    }

    const targetZone = availableZones[nextIndex];
    if (targetZone) {
      this.focusFirstInZone(targetZone);
    }
  }

  private focusSearchInput() {
    const input = document.querySelector<HTMLInputElement>('input[placeholder*="Поиск"], [data-nav-search]');
    if (input) {
      this.setFocus(input);
      input.select();
      const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
      if (app && app.TriggerSteamOSKeyboard) {
        app.TriggerSteamOSKeyboard().catch(() => {});
      }
    }
  }

  private focusFilterSelect() {
    const select = document.querySelector<HTMLSelectElement>('select, [data-nav-filter]');
    if (select) {
      this.setFocus(select);
    }
  }

  private scrollActivePanel(amount: number) {
    if (this.activeZone === 'detail') {
      const detailContainer = document.querySelector<HTMLElement>('[data-nav-zone="detail"]');
      if (detailContainer && detailContainer.scrollHeight > detailContainer.clientHeight + 10) {
        detailContainer.scrollTop += amount;
        return;
      }
    }

    if (this.currentFocusedEl) {
      let parent: HTMLElement | null = this.currentFocusedEl.parentElement;
      while (parent && parent !== document.body) {
        const overflowY = window.getComputedStyle(parent).overflowY;
        if ((overflowY === 'auto' || overflowY === 'scroll') && parent.scrollHeight > parent.clientHeight + 20) {
          parent.scrollTop += amount;
          return;
        }
        parent = parent.parentElement;
      }
    }
    const detailPanel = document.querySelector<HTMLElement>('.panel-detail, [data-nav-zone="detail"], main');
    if (detailPanel) {
      detailPanel.scrollTop += amount;
      return;
    }
    window.scrollBy({ top: amount });
  }

  private isElementVisible(el: HTMLElement): boolean {
    const rect = el.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) return false;
    const style = window.getComputedStyle(el);
    return style.visibility !== 'hidden' && style.display !== 'none' && style.opacity !== '0';
  }

  private navigateSpatial(direction: Direction) {
    const items = this.getNavItems();
    if (items.length === 0) return;

    // If nothing currently focused, focus element closest to viewport center
    if (!this.currentFocusedEl || !document.body.contains(this.currentFocusedEl)) {
      const viewCenterY = window.innerHeight / 2;
      const viewCenterX = window.innerWidth / 2;
      let bestItem: HTMLElement = items[0];
      let bestDist = Infinity;

      for (const item of items) {
        const rect = item.getBoundingClientRect();
        if (rect.bottom > 0 && rect.top < window.innerHeight && rect.right > 0 && rect.left < window.innerWidth) {
          const itemCenterX = rect.left + rect.width / 2;
          const itemCenterY = rect.top + rect.height / 2;
          const dist = Math.hypot(itemCenterX - viewCenterX, itemCenterY - viewCenterY);
          if (dist < bestDist) {
            bestDist = dist;
            bestItem = item;
          }
        }
      }

      this.setFocus(bestItem);
      return;
    }

    const currentRect = this.currentFocusedEl.getBoundingClientRect();
    const currentCenterX = currentRect.left + currentRect.width / 2;
    const currentCenterY = currentRect.top + currentRect.height / 2;

    let bestCandidate: HTMLElement | null = null;
    let minDistance = Infinity;

    for (const item of items) {
      if (item === this.currentFocusedEl) continue;

      const rect = item.getBoundingClientRect();
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;

      const dx = centerX - currentCenterX;
      const dy = centerY - currentCenterY;

      let isCandidate = false;

      switch (direction) {
        case 'UP':
          isCandidate = dy < -4;
          break;
        case 'DOWN':
          isCandidate = dy > 4;
          break;
        case 'LEFT':
          isCandidate = dx < -4;
          break;
        case 'RIGHT':
          isCandidate = dx > 4;
          break;
      }

      if (isCandidate) {
        let distance: number;
        // Directional axis weighting: heavily penalize perpendicular offsets to keep motion crisp in lines & columns
        if (direction === 'UP' || direction === 'DOWN') {
          distance = Math.abs(dy) + Math.abs(dx) * 2.4;
        } else {
          distance = Math.abs(dx) + Math.abs(dy) * 2.4;
        }

        // Zone affinity: prefer staying in the same zone if candidates are comparable
        const sameZone = item.closest('[data-nav-zone]') === this.currentFocusedEl.closest('[data-nav-zone]');
        if (sameZone) {
          distance *= 0.72;
        }

        if (distance < minDistance) {
          minDistance = distance;
          bestCandidate = item;
        }
      }
    }

    if (bestCandidate) {
      this.setFocus(bestCandidate);
    } else {
      // If at boundary in vertical direction, scroll the active panel smoothly
      if (direction === 'DOWN') {
        this.scrollActivePanel(220);
      } else if (direction === 'UP') {
        this.scrollActivePanel(-220);
      }
    }
  }
}

export const gamepad = new GamepadEngine();
