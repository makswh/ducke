/**
 * Title utilities for Ducke
 * Resolves clean, human-readable display titles from Steam metadata or cleaned release names.
 */

export function isPlaceholderTitle(title: string | null | undefined): boolean {
  if (!title) return true;
  const t = title.trim();
  return t === '' || /^Steam App \d+$/i.test(t);
}

/**
 * Strips release junk, scene tags, versions, years, repackers, languages, and size markers
 * from raw torrent or crawler names.
 */
export function cleanTorrentTitle(rawTitle: string): string {
  if (!rawTitle) return '';
  let s = rawTitle.trim();

  // 1. Spaced dot abbreviations like "S T A L K E R" -> "S.T.A.L.K.E.R."
  s = s.replace(/\b([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\b/g, '$1.$2.$3.$4.$5.$6.$7.');
  s = s.replace(/\b([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\b/g, '$1.$2.$3.$4.$5.$6.');
  s = s.replace(/\b([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\b/g, '$1.$2.$3.$4.$5.');
  s = s.replace(/\b([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\b/g, '$1.$2.$3.$4.');
  s = s.replace(/\b([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\b/g, '$1.$2.$3.');

  // 2. Strip all square brackets [ ... ] (e.g. [DL], [В разработке], [P], [RUS / ENG], [v1.2])
  s = s.replace(/\[[^\]]*\]/g, ' ');

  // 3. Strip years in parentheses (2019) or (2003-2020)
  s = s.replace(/\(\s*\d{4}(?:\s*[-–—/]\s*\d{4})?\s*\)/g, ' ');

  // 4. Strip release notes in parentheses (repack by ...)
  s = s.replace(/\((?:repack|rip|версия|от|by|portable|gog|pc|[\d.,\s/\\+-]+)[^\)]*\)/gi, ' ');

  // 5. Dual titles e.g. "TITLE / НАЗВАНИЕ" or "TITLE | НАЗВАНИЕ" (done after stripping brackets)
  if (s.includes(' / ')) {
    const parts = s.split(' / ').map((p) => p.trim()).filter(Boolean);
    if (parts.length >= 2 && parts[0].length >= 3) {
      s = parts[0];
    }
  } else if (s.includes(' | ')) {
    const parts = s.split(' | ').map((p) => p.trim()).filter(Boolean);
    if (parts.length >= 2 && parts[0].length >= 3) {
      s = parts[0];
    }
  }

  // 6. Strip version markers (v 1 3 3, build 1234, etc.)
  s = s.replace(/\b(?:v\s*\d+([._\s]\d+)*|build\s*\d+|patch\s*\d+|update\s*\d*|hotfix)\b/gi, ' ');

  // 7. Strip standalone release words (RePack, portable, etc.)
  s = s.replace(/\b(repack|репак|rip|рип|portable|unpacked|steamrip|gog|xatab|fitgirl|dodi|decepticon|elamigos|codex|empress|multi\d*)\b/gi, ' ');

  // 8. Strip trailing platform markers (PC, Linux, Windows) and sizes
  s = s.replace(/\s*\b(pc|mac|linux|win|windows)\s*$/gi, ' ');
  s = s.replace(/(?:[\s\-_]+)?(?:\[|\()?(\d+([.,]\d+)?\s*(?:gb|mb|tb|гб|мб|тб|g|m|t))(?:\)|\])?$/gi, ' ');

  // 9. Strip curly braces and boundary separators
  s = s.replace(/[\{\}]/g, ' ');
  s = s.replace(/^[\s\-_:/\\|]+|[\s\-_:/\\|]+$/g, '');

  return s.replace(/\s+/g, ' ').trim();
}

/**
 * Returns the best display title for a game entity:
 * - Official Steam title if available (and not a placeholder)
 * - Otherwise, cleaned release / torrent / raw title
 */
export function getDisplayTitle(g: any): string {
  if (!g) return '';
  if (!isPlaceholderTitle(g.steamTitle)) {
    return g.steamTitle.trim();
  }
  const raw = (g.cleanTitle && g.cleanTitle.trim() !== '') ? g.cleanTitle : (g.rawName || '');
  return cleanTorrentTitle(raw);
}
