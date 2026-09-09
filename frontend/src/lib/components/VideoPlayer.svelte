<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Hls from 'hls.js';
  import { Play, Pause, Volume2, VolumeX, Maximize2, Minimize2, AlertCircle, RefreshCw } from 'lucide-svelte';

  interface Props {
    src?: string;
    hls?: string;
    mp4?: string;
    webm?: string;
    poster?: string;
    title?: string;
    autoplay?: boolean;
    muted?: boolean;
    controls?: boolean;
    class?: string;
  }

  let {
    src = '',
    hls = '',
    mp4 = '',
    webm = '',
    poster = '',
    title = '',
    autoplay = false,
    muted = false,
    controls = true,
    class: className = ''
  }: Props = $props();

  let playerContainer = $state<HTMLDivElement | null>(null);
  let isFullscreen = $state(false);

  let videoElement: HTMLVideoElement | null = null;
  let hlsInstance: Hls | null = null;
  let isPlaying = $state(false);
  let isMuted = $state(false);
  let currentTime = $state(0);
  let duration = $state(0);
  let hasError = $state(false);
  let errorMessage = $state('');
  let isBuffering = $state(false);

  let lastLoadedSource = '';

  // Determine effective primary URL
  function computeTargetUrl(): string {
    let url = '';
    if (hls && hls.trim() !== '') url = hls.trim();
    else if (src && (src.includes('.m3u8') || src.includes('hls_264'))) url = src.trim();
    else if (mp4 && mp4.trim() !== '' && !mp4.includes('/apps/')) url = mp4.trim();
    else if (webm && webm.trim() !== '') url = webm.trim();
    else if (src && src.trim() !== '' && !src.includes('/apps/')) url = src.trim();

    if (url) {
      // Fastly CDN is fast, unblocked and reliable for Steam HLS/MP4 in all regions
      url = url.replace(/video\.akamai\.steamstatic\.com/gi, 'video.fastly.steamstatic.com');
      url = url.replace(/shared\.akamai\.steamstatic\.com/gi, 'video.fastly.steamstatic.com');
      url = url.replace(/^http:\/\//i, 'https://');
    }
    return url;
  }

  function destroyHls() {
    if (hlsInstance) {
      try {
        hlsInstance.stopLoad();
        hlsInstance.detachMedia();
        hlsInstance.destroy();
      } catch (e) {
        console.warn('HLS destroy notice:', e);
      }
      hlsInstance = null;
    }
  }

  function loadMediaSource(targetUrl: string) {
    if (!videoElement) return;
    if (!targetUrl) {
      destroyHls();
      videoElement.removeAttribute('src');
      videoElement.load();
      lastLoadedSource = '';
      isPlaying = false;
      isBuffering = false;
      return;
    }
    if (targetUrl === lastLoadedSource) return;

    lastLoadedSource = targetUrl;
    hasError = false;
    errorMessage = '';
    isBuffering = false;
    destroyHls();

    const isHlsStream = targetUrl.includes('.m3u8') || targetUrl.includes('hls_264');

    if (isHlsStream) {
      if (Hls.isSupported()) {
        const hls = new Hls({
          enableWorker: false, // Prevents WebKitGTK worker blob / cross-origin worker exceptions
          lowLatencyMode: false,
          backBufferLength: 30,
          maxBufferLength: 30,
          xhrSetup: (xhr) => {
            xhr.withCredentials = false;
          }
        });
        hlsInstance = hls;

        hls.loadSource(targetUrl);
        hls.attachMedia(videoElement);

        hls.on(Hls.Events.MANIFEST_PARSED, () => {
          hasError = false;
          isBuffering = false;
          if (autoplay) {
            videoElement?.play().catch(() => {});
          }
        });

        hls.on(Hls.Events.ERROR, (_event, data) => {
          if (data.fatal) {
            console.warn('[Hls Fatal Error]', data.type, data.details);
            switch (data.type) {
              case Hls.ErrorTypes.NETWORK_ERROR:
                hls.startLoad();
                break;
              case Hls.ErrorTypes.MEDIA_ERROR:
                hls.recoverMediaError();
                break;
              default:
                destroyHls();
                if (videoElement) {
                  videoElement.src = targetUrl;
                  videoElement.load();
                }
                break;
            }
          }
        });
      } else if (videoElement.canPlayType('application/vnd.apple.mpegurl')) {
        videoElement.src = targetUrl;
        if (autoplay) videoElement.play().catch(() => {});
      } else {
        videoElement.src = targetUrl;
      }
    } else {
      videoElement.src = targetUrl;
      if (autoplay) videoElement.play().catch(() => {});
    }
  }

  // Reactive effect strictly checking for target URL changes
  $effect(() => {
    const target = computeTargetUrl();
    if (videoElement && target !== lastLoadedSource) {
      loadMediaSource(target);
    }
  });

  onDestroy(() => {
    destroyHls();
  });

  function videoRefAction(node: HTMLVideoElement) {
    videoElement = node;
    node.muted = muted;
    isMuted = muted;

    const target = computeTargetUrl();
    if (target) {
      loadMediaSource(target);
    }

    return {
      destroy() {
        destroyHls();
        videoElement = null;
      }
    };
  }

  function togglePlay(e?: Event) {
    if (e) e.stopPropagation();
    if (!videoElement) return;

    const target = computeTargetUrl();
    if (!target) {
      console.warn('[Ducke VideoPlayer] No valid video stream URL to play');
      return;
    }

    if (!lastLoadedSource || lastLoadedSource !== target) {
      loadMediaSource(target);
    }

    if (videoElement.paused) {
      if (hlsInstance) {
        hlsInstance.startLoad();
      }
      const p = videoElement.play();
      if (p !== undefined) {
        p.then(() => {
          isPlaying = true;
          isBuffering = false;
        }).catch((err) => {
          console.warn('Standard play failed, retrying muted:', err);
          if (videoElement) {
            videoElement.muted = true;
            isMuted = true;
            videoElement.play().then(() => {
              isPlaying = true;
              isBuffering = false;
            }).catch((err2) => {
              console.error('Playback error:', err2);
              hasError = true;
              errorMessage = 'Не удалось воспроизвести видео';
              isBuffering = false;
            });
          }
        });
      }
    } else {
      videoElement.pause();
      isPlaying = false;
    }
  }

  function toggleMute(e?: Event) {
    if (e) e.stopPropagation();
    if (!videoElement) return;
    videoElement.muted = !videoElement.muted;
    isMuted = videoElement.muted;
  }

  onMount(() => {
    const handleFs = () => {
      isFullscreen = !!document.fullscreenElement;
    };
    document.addEventListener('fullscreenchange', handleFs);
    return () => {
      document.removeEventListener('fullscreenchange', handleFs);
    };
  });

  function handleFullscreen(e?: Event) {
    if (e) e.stopPropagation();
    const targetEl = playerContainer || videoElement;
    if (!targetEl) return;

    if (!document.fullscreenElement) {
      if (targetEl.requestFullscreen) {
        targetEl.requestFullscreen().catch((err) => {
          console.warn('[VideoPlayer] requestFullscreen fallback to video element:', err);
          videoElement?.requestFullscreen?.().catch(console.error);
        });
      } else if ((videoElement as any)?.webkitEnterFullscreen) {
        (videoElement as any).webkitEnterFullscreen();
      }
    } else {
      if (document.exitFullscreen) {
        document.exitFullscreen().catch(console.error);
      }
    }
  }
</script>

<div
  bind:this={playerContainer}
  class="relative w-full h-full bg-black flex items-center justify-center overflow-hidden group select-none {className}"
>
  <!-- HTML5 Video Element -->
  <video
    use:videoRefAction
    {poster}
    playsinline
    preload="auto"
    class="w-full h-full object-contain cursor-pointer"
    onclick={togglePlay}
    ondblclick={handleFullscreen}
    onplay={() => { isPlaying = true; isBuffering = false; }}
    onplaying={() => { isPlaying = true; isBuffering = false; }}
    onpause={() => { isPlaying = false; isBuffering = false; }}
    onwaiting={() => { if (isPlaying) isBuffering = true; }}
    oncanplay={() => { isBuffering = false; }}
    onseeking={() => { isBuffering = true; }}
    onseeked={() => { isBuffering = false; }}
    ontimeupdate={() => {
      if (videoElement) {
        currentTime = videoElement.currentTime;
        duration = videoElement.duration || 0;
        if (videoElement.currentTime > 0) isBuffering = false;
      }
    }}
    onerror={() => {
      const err = videoElement?.error;
      console.warn('[Ducke VideoPlayer] Native video error:', err?.code, err?.message);

      // Fallback 1: If HLS.js errored, try native GStreamer HTML5 playback directly
      if (hlsInstance) {
        destroyHls();
        if (videoElement && lastLoadedSource) {
          videoElement.src = lastLoadedSource;
          videoElement.load();
          return;
        }
      }

      // Fallback 2: If Fastly CDN failed, try Akamai fallback
      if (videoElement && lastLoadedSource && lastLoadedSource.includes('fastly.steamstatic.com')) {
        const akamaiUrl = lastLoadedSource.replace('video.fastly.steamstatic.com', 'video.akamai.steamstatic.com');
        lastLoadedSource = akamaiUrl;
        destroyHls();
        videoElement.src = akamaiUrl;
        videoElement.load();
        return;
      }

      hasError = true;
      errorMessage = 'Ошибка воспроизведения';
      isBuffering = false;
    }}
  >
    <track kind="captions" />
  </video>

  <!-- Buffering Spinner (Only shown when buffering during playback) -->
  {#if isBuffering && isPlaying}
    <div class="absolute inset-0 flex items-center justify-center bg-black/40 pointer-events-none z-20">
      <RefreshCw class="w-8 h-8 text-white animate-spin drop-shadow-md" />
    </div>
  {/if}

  <!-- Error State Overlay -->
  {#if hasError}
    <div class="absolute inset-0 flex flex-col items-center justify-center bg-black/90 p-4 text-center z-20 space-y-2">
      <AlertCircle class="w-8 h-8 text-rose-400" />
      <p class="text-xs font-semibold text-white">{errorMessage || 'Не удалось воспроизвести видео'}</p>
      <button
        type="button"
        class="px-3 py-1.5 rounded bg-white/10 hover:bg-white/20 text-xs text-white transition-colors cursor-pointer"
        onclick={() => {
          lastLoadedSource = '';
          const target = computeTargetUrl();
          if (target) loadMediaSource(target);
        }}
      >
        Повторить
      </button>
    </div>
  {/if}

  <!-- Center Big Play Button (Visible when paused) -->
  {#if !isPlaying && !hasError}
    <button
      type="button"
      class="absolute inset-0 flex items-center justify-center bg-black/25 hover:bg-black/35 transition-colors cursor-pointer z-10"
      onclick={togglePlay}
      aria-label="Воспроизвести"
    >
      <div class="w-14 h-14 rounded-full bg-black/80 border border-white/25 flex items-center justify-center text-white shadow-2xl hover:scale-110 transition-transform">
        <Play class="w-6 h-6 ml-0.5 fill-white" />
      </div>
    </button>
  {/if}

  <!-- Tactile SteamOS Control Bar (Hover & Focus) -->
  {#if controls && !hasError}
    <div class="absolute bottom-0 inset-x-0 bg-gradient-to-t from-black/90 via-black/40 to-transparent p-2.5 flex items-center justify-between gap-2.5 opacity-0 group-hover:opacity-100 focus-within:opacity-100 transition-opacity duration-150 z-30">
      <!-- Play/Pause -->
      <button
        type="button"
        data-nav-item
        class="p-1.5 rounded bg-white/10 hover:bg-white/20 text-white cursor-pointer transition-colors"
        onclick={togglePlay}
        title={isPlaying ? "Пауза" : "Воспроизведение"}
      >
        {#if isPlaying}
          <Pause class="w-4 h-4 fill-white" />
        {:else}
          <Play class="w-4 h-4 fill-white ml-0.5" />
        {/if}
      </button>

      <!-- Mute/Unmute -->
      <button
        type="button"
        data-nav-item
        class="p-1.5 rounded bg-white/10 hover:bg-white/20 text-white cursor-pointer transition-colors"
        onclick={toggleMute}
        title={isMuted ? "Включить звук" : "Выключить звук"}
      >
        {#if isMuted}
          <VolumeX class="w-4 h-4 text-rose-400" />
        {:else}
          <Volume2 class="w-4 h-4" />
        {/if}
      </button>

      <!-- Timeline Progress -->
      {#if duration > 0}
        <div class="flex-1 flex items-center gap-2 px-1">
          <input
            type="range"
            min="0"
            max={duration}
            step="0.1"
            value={currentTime}
            class="w-full h-1 bg-white/20 rounded appearance-none cursor-pointer accent-white"
            oninput={(e) => {
              const val = parseFloat((e.target as HTMLInputElement).value);
              if (videoElement) videoElement.currentTime = val;
            }}
          />
        </div>
      {:else}
        <div class="flex-1"></div>
      {/if}

      <!-- Fullscreen -->
      <button
        type="button"
        data-nav-item
        class="p-1.5 rounded bg-white/10 hover:bg-white/20 text-white cursor-pointer transition-colors"
        onclick={handleFullscreen}
        title={isFullscreen ? "Оконный режим (Esc)" : "Во весь экран (F)"}
      >
        {#if isFullscreen}
          <Minimize2 class="w-4 h-4" />
        {:else}
          <Maximize2 class="w-4 h-4" />
        {/if}
      </button>
    </div>
  {/if}
</div>
