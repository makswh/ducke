import { EventsOn } from '../../../wailsjs/runtime/runtime';

let torrentSourcesMap: Record<string, string> = {};
let isListening = false;

export async function loadTorrentSourcesMap(): Promise<Record<string, string>> {
  try {
    const app = typeof window !== 'undefined' ? (window as any)?.go?.main?.App : null;
    if (app && app.GetTorrentSources) {
      const sources = await app.GetTorrentSources();
      if (Array.isArray(sources)) {
        const m: Record<string, string> = {};
        for (const s of sources) {
          if (s && s.id && s.name) {
            m[s.id] = s.name;
          }
        }
        torrentSourcesMap = m;
      }
    }
  } catch {}

  if (!isListening && typeof window !== 'undefined') {
    isListening = true;
    try {
      EventsOn('torrents:updated', () => {
        loadTorrentSourcesMap();
      });
    } catch {}
  }

  return torrentSourcesMap;
}

if (typeof window !== 'undefined') {
  loadTorrentSourcesMap();
}

export function cleanSourceDisplayName(name: string | undefined): string {
  if (!name) return '';
  let s = name.split('|')[0].trim();
  const m = s.match(/\(([^)]+)\)/);
  if (m && m[1] && (m[1].includes('Торрент') || m[1].includes('Игры') || m[1].includes('Games'))) {
    s = m[1].trim();
  }
  s = s.replace(/\s*(?:[-–—|/]\s*)?(?:win\s*)?(?:игры|games)\s*$/i, '').trim();
  return s || name;
}

export function formatTorrentSourceName(rawSource: string | undefined): string {
  if (!rawSource) return '';
  const s = rawSource.trim();
  if (torrentSourcesMap[s]) {
    return cleanSourceDisplayName(torrentSourcesMap[s]);
  }
  if (s.startsWith('tsrc_')) {
    return 'Каталог торрентов';
  }
  return cleanSourceDisplayName(s);
}
