/**
 * Utility functions for parsing and comparing dates across Ducke catalogs,
 * favorites, download histories, and game variants.
 */

/**
 * Parses date string or number into Unix epoch milliseconds.
 * Handles ISO-8601, SQLite DATETIME ('YYYY-MM-DD HH:MM:SS'), Unix epoch,
 * and Steam textual dates ('12 Oct, 2023', 'Oct 2023', '2023').
 */
export function parseDateToMs(dateStr?: string | number | null): number {
  if (dateStr === undefined || dateStr === null || dateStr === '') return 0;

  if (typeof dateStr === 'number') {
    // If it's in seconds (e.g. 10 digits), convert to milliseconds
    return dateStr < 1e11 ? dateStr * 1000 : dateStr;
  }

  const s = String(dateStr).trim();
  if (!s) return 0;

  // Handle SQLite standard format "YYYY-MM-DD HH:MM:SS" (stored in UTC)
  if (/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(s)) {
    const t = Date.parse(s.replace(' ', 'T') + 'Z');
    if (!isNaN(t)) return t;
  }

  // Handle standard ISO or RFC formats
  const parsed = Date.parse(s);
  if (!isNaN(parsed)) return parsed;

  // Fallback: extract 4-digit year (1970 - 2035)
  const yearMatch = s.match(/\b(19\d\d|20\d\d)\b/);
  if (yearMatch) {
    return new Date(parseInt(yearMatch[1], 10), 0, 1).getTime();
  }

  return 0;
}

/**
 * Returns the timestamp of when the game was added (or uploaded to tracker).
 * Combines uploadDate, createdAt, and fallback game id.
 */
export function getGameAddedTime(g: any): number {
  if (!g) return 0;
  const tUpload = parseDateToMs(g.uploadDate);
  const tCreated = parseDateToMs(g.createdAt);
  const maxT = Math.max(tUpload, tCreated);
  if (maxT > 0) return maxT;
  // Fallback to ID for sequential insert ordering
  return Number(g.id) || 0;
}

/**
 * Returns the timestamp of when the game was last updated/refreshed.
 * Combines updatedAt, uploadDate, createdAt, and fallback game id.
 */
export function getGameUpdatedTime(g: any): number {
  if (!g) return 0;
  const tUpdated = parseDateToMs(g.updatedAt);
  const tUpload = parseDateToMs(g.uploadDate);
  const tCreated = parseDateToMs(g.createdAt);
  const maxT = Math.max(tUpdated, tUpload, tCreated);
  if (maxT > 0) return maxT;
  return Number(g.id) || 0;
}

/**
 * Returns the timestamp or year of the Steam official release date.
 */
export function getGameReleaseTime(g: any): number {
  if (!g) return 0;
  if (g._releaseYear !== undefined && g._releaseYear !== null) {
    return new Date(g._releaseYear, 0, 1).getTime();
  }
  return parseDateToMs(g.releaseDate);
}
