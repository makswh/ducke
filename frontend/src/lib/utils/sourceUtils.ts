import type { GameEntity } from '../types/game';

export type SourceFilterType = 'all' | 'ddl' | 'torrent' | 'ftp';

export interface SourceFilterOption {
  id: SourceFilterType;
  label: string;
  shortLabel: string;
}

export const SOURCE_FILTER_OPTIONS: SourceFilterOption[] = [
  { id: 'all', label: 'Все источники', shortLabel: 'Все' },
  { id: 'ddl', label: 'Прямая загрузка (DDL)', shortLabel: 'DDL' },
  { id: 'torrent', label: 'Торрент', shortLabel: 'Торрент' },
  { id: 'ftp', label: 'FTP-сервер', shortLabel: 'FTP' }
];

export function hasGameMagnet(game: any): boolean {
  if (!game) return false;
  if (game.sourceType === 'torrent') return true;
  if (game.magnetUri) return true;
  if (Array.isArray(game.uris) && game.uris.some((u: string) => typeof u === 'string' && u.startsWith('magnet:'))) {
    return true;
  }
  return false;
}

export function isGameDDL(game: any): boolean {
  if (!game) return false;
  if (game.sourceType === 'ddl') return true;
  if (Array.isArray(game.hosters) && game.hosters.length > 0) return true;
  if (Array.isArray(game.uris) && game.uris.some((u: string) => typeof u === 'string' && (u.startsWith('http://') || u.startsWith('https://')))) {
    return true;
  }
  return false;
}

export function isGameTorrent(game: any): boolean {
  return hasGameMagnet(game);
}

export function isGameFTP(game: any): boolean {
  if (!game) return false;
  return !isGameTorrent(game) && !isGameDDL(game);
}

export function getPrimarySourceType(game: any): 'ddl' | 'torrent' | 'ftp' {
  if (isGameTorrent(game)) return 'torrent';
  if (isGameDDL(game)) return 'ddl';
  return 'ftp';
}

/**
 * Checks if a game matches the source filter, inspecting both the primary record
 * and any alternative release variants (e.g. game has both torrent and DDL releases).
 */
export function matchesSourceFilter(game: any, filter: SourceFilterType): boolean {
  if (!filter || filter === 'all') return true;
  if (!game) return false;

  // 1. Check primary game entity
  if (filter === 'ddl' && isGameDDL(game)) return true;
  if (filter === 'torrent' && isGameTorrent(game)) return true;
  if (filter === 'ftp' && isGameFTP(game)) return true;

  // 2. Check variants
  if (Array.isArray(game.variants) && game.variants.length > 0) {
    for (const v of game.variants) {
      if (filter === 'ddl' && isGameDDL(v)) return true;
      if (filter === 'torrent' && isGameTorrent(v)) return true;
      if (filter === 'ftp' && isGameFTP(v)) return true;
    }
  }

  return false;
}

/**
 * Checks if a download history record matches the source filter
 */
export function matchesDownloadRecordSource(record: any, filter: SourceFilterType): boolean {
  if (!filter || filter === 'all') return true;
  if (!record) return false;
  if (filter === 'ddl') return !!record.isDdl;
  if (filter === 'torrent') return !record.isDdl && !!record.isTorrent;
  if (filter === 'ftp') return !record.isDdl && !record.isTorrent;
  return true;
}
