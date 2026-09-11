import type { GameEntity, GameVariant } from '../types/game';

function isPlaceholderTitle(title: string | undefined | null): boolean {
  if (!title) return true;
  const t = title.trim();
  return t === '' || /^Steam App \d+$/i.test(t);
}

/**
 * Normalizes title for deduplication: lowercase, strips tags, brackets, versions, articles, junk.
 */
export function cleanCanonicalKey(raw: string | undefined | null): string {
  if (!raw) return '';
  let s = raw.toLowerCase();

  // Strip brackets: [repack], (2020), {pc}, etc.
  s = s.replace(/\[.*?\]|\(.*?\)|[\{\}]/g, ' ');

  // Strip dates: 2020, 2015-2025, 2008-2010
  s = s.replace(/\b\d{4}(?:[-/.]\d{4})?\b/g, ' ');

  // Strip versions: vv.1.0.7.0, v1.2.0.43, v.1.0.1868/1.50, ver.1.2, build 1234, patch 2, update 5
  s = s.replace(/\b(v+[._\s]*\d+([._\s\-/]\d+)*[a-z]?|ver[._\s]*\d+|build\s*\d+|patch\s*\d+|update\s*\d*|hotfix)\b/gi, ' ');

  // Strip editions: complete edition, deluxe edition, repack, etc.
  s = s.replace(/\b(deluxe(\s+edition)?|ultimate(\s+edition)?|goty(\s+edition)?|game\s+of\s+the\s+year(\s+edition)?|collector('s)?\s+edition|remastered|enhanced(\s+edition)?|gold\s+edition|director('s)?\s+cut|complete(\s+edition)?|definitive(\s+edition)?|special\s+edition|anniversary(\s+edition)?|repack|portable|multi\d*|selective\s+download|unpacked|rip|steamrip|bundle|bonus|digital|legacy)\b/gi, ' ');

  // Strip release groups & junk words
  const junkWords = new Set([
    'the', 'a', 'an', 'repack', 'репак', 'rip', 'рип', 'steamrip', 'xatab', 'хатаб',
    'fitgirl', 'dodi', 'decepticon', 'механики', 'choptik', 'elamigos', 'codex',
    'empress', 'gog', 'rus', 'eng', 'linux', 'win', 'windows', 'mac', 'macos', 'pc',
    'native', 'portable', 'unpacked', 'лицензия', 'пиратка', 'сборка', 'папка', 'игры',
    'таблетка', 'вшита', 'русификатор', 'озвучка', 'текст', 'от', 'by', 'версия',
    'dlc', 'dlcs', 'fix', 'hotfix', 'pack', 'bundle', 'bonus', 'update', 'patch',
    'v', 'vv', 'ver', 'legacy'
  ]);

  // Replace punctuation with spaces
  s = s.replace(/[^a-z0-9а-яё]/gi, ' ');

  const words = s.split(/\s+/).filter((w) => {
    if (!w) return false;
    if (junkWords.has(w)) return false;
    const num = parseInt(w, 10);
    if (!isNaN(num) && num >= 1970 && num <= 2035) return false;
    return true;
  });

  return words.join(' ').trim();
}

function scoreGame(g: GameEntity): number {
  let score = 0;
  if (g.steamAppId && g.steamAppId !== 0) score += 100;
  if (g.steamSynced) score += 50;
  if (g.headerImage) score += 50;
  if (g.shortDescription) score += 30;
  if (g.screenshots && g.screenshots.length > 0) score += 20;
  if (g.iconUrl) score += 10;
  return score;
}

function toVariant(g: GameEntity): GameVariant {
  return {
    id: g.id,
    rawName: g.rawName || g.cleanTitle || '',
    cleanTitle: g.cleanTitle || '',
    sizeBytes: g.sizeBytes || 0,
    sizeDisplay: g.sizeDisplay || '',
    sourceType: g.sourceType || 'torrent',
    torrentSource: g.torrentSource,
    remotePath: g.remotePath || '',
    magnetUri: g.magnetUri,
    uploadDate: g.uploadDate,
    isDirectory: !!g.isDirectory,
    steamAppId: g.steamAppId,
  };
}

/**
 * Deduplicates a list of GameEntity objects, collapsing different releases / repacks of the
 * same game into a single primary card and aggregating all versions into primary.variants.
 */
export function deduplicateGames(list: GameEntity[]): GameEntity[] {
  if (!list || list.length <= 1) return list || [];

  const n = list.length;
  const parent = Array.from({ length: n }, (_, i) => i);

  function find(i: number): number {
    if (parent[i] === i) return i;
    parent[i] = find(parent[i]);
    return parent[i];
  }

  function union(i: number, j: number) {
    const rootI = find(i);
    const rootJ = find(j);
    if (rootI !== rootJ) {
      parent[rootI] = rootJ;
    }
  }

  const keyToRoot = new Map<string, number>();

  for (let i = 0; i < n; i++) {
    const g = list[i];
    if (!g) continue;

    const keys: string[] = [];

    if (g.steamAppId && g.steamAppId !== 0) {
      keys.push(`appid:${g.steamAppId}`);
    }

    if (g.steamTitle && !isPlaceholderTitle(g.steamTitle)) {
      const stk = cleanCanonicalKey(g.steamTitle);
      if (stk) keys.push(`steam:${stk}`);
    }

    const kClean = cleanCanonicalKey(g.cleanTitle);
    if (kClean) keys.push(`title:${kClean}`);

    const kSearch = cleanCanonicalKey(g.searchTitle);
    if (kSearch && kSearch !== kClean) keys.push(`title:${kSearch}`);

    for (const k of keys) {
      const root = keyToRoot.get(k);
      if (root !== undefined) {
        union(i, root);
      } else {
        keyToRoot.set(k, i);
      }
    }
  }

  // Group games by connected component root
  const groups = new Map<number, GameEntity[]>();
  const groupOrder: number[] = [];

  for (let i = 0; i < n; i++) {
    const g = list[i];
    if (!g) continue;
    const root = find(i);
    if (!groups.has(root)) {
      groups.set(root, []);
      groupOrder.push(root);
    }
    groups.get(root)!.push(g);
  }

  const result: GameEntity[] = [];

  for (const root of groupOrder) {
    const items = groups.get(root)!;
    if (items.length === 0) continue;

    // Pick best primary game
    let bestIdx = 0;
    let bestScore = scoreGame(items[0]);
    for (let idx = 1; idx < items.length; idx++) {
      const sc = scoreGame(items[idx]);
      if (sc > bestScore || (sc === bestScore && items[idx].id > items[bestIdx].id)) {
        bestScore = sc;
        bestIdx = idx;
      }
    }

    // Clone primary
    const primary: GameEntity = { ...items[bestIdx] };

    // Inherit missing metadata from siblings
    for (const item of items) {
      if ((!primary.steamAppId || primary.steamAppId === 0) && item.steamAppId && item.steamAppId !== 0) {
        primary.steamAppId = item.steamAppId;
        primary.steamSynced = item.steamSynced;
        primary.steamTitle = item.steamTitle;
        primary.shortDescription = item.shortDescription;
        primary.detailedDescription = item.detailedDescription;
        primary.headerImage = item.headerImage;
        primary.capsuleImage = item.capsuleImage;
        primary.backgroundImage = item.backgroundImage;
        primary.iconUrl = item.iconUrl;
        primary.screenshots = item.screenshots;
        primary.movies = item.movies;
        primary.genres = item.genres;
        primary.developers = item.developers;
        primary.publishers = item.publishers;
        primary.releaseDate = item.releaseDate;
        primary.controllerSupport = item.controllerSupport;
        primary.pcRequirements = item.pcRequirements;
        primary.metacriticScore = item.metacriticScore;
        primary.reviewScoreDesc = item.reviewScoreDesc;
        primary.reviewPercent = item.reviewPercent;
        primary.totalReviews = item.totalReviews;
      }
      if (!primary.iconUrl && item.iconUrl) primary.iconUrl = item.iconUrl;
      if (!primary.headerImage && item.headerImage) primary.headerImage = item.headerImage;
      if (!primary.capsuleImage && item.capsuleImage) primary.capsuleImage = item.capsuleImage;
      if (!primary.backgroundImage && item.backgroundImage) primary.backgroundImage = item.backgroundImage;
    }

    if (isPlaceholderTitle(primary.steamTitle)) {
      primary.steamTitle = '';
    }

    // Collect all variants from all items in this group
    const seenVariants = new Set<string>();
    const variants: GameVariant[] = [];

    const addVariant = (v: GameVariant) => {
      const vKey = `${v.id}|${(v.rawName || '').trim()}|${v.sizeBytes || 0}`;
      if (seenVariants.has(vKey)) return;
      seenVariants.add(vKey);
      variants.push(v);
    };

    for (const item of items) {
      if (item.variants && item.variants.length > 0) {
        for (const v of item.variants) {
          addVariant(v);
        }
      } else {
        addVariant(toVariant(item));
      }
    }

    primary.variants = variants;
    result.push(primary);
  }

  return result;
}
