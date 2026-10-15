import type { GameEntity } from '../types/game';
import { getDisplayTitle } from './titleUtils';

/**
 * Filter games that have valid visuals (capsule, header, or background)
 */
export function hasValidCover(game: GameEntity): boolean {
  return !!(
    (game.capsuleImage && game.capsuleImage.startsWith('http')) ||
    (game.headerImage && game.headerImage.startsWith('http')) ||
    (game.backgroundImage && game.backgroundImage.startsWith('http'))
  );
}

/**
 * Normalizes title for deduplication comparison
 */
export function normalizeTitleKey(title: string): string {
  if (!title) return '';
  return title
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, '')
    .trim();
}

/**
 * Featured games for the top Hero carousel.
 * Requires high-resolution header/capsule, screenshots, and preferably good reviews.
 */
export function getHeroFeaturedGames(games: GameEntity[], limit: number = 8): GameEntity[] {
  if (!games || games.length === 0) return [];

  const candidates = games.filter((g) => {
    if (!g) return false;
    const hasImage = !!(
      (g.headerImage && g.headerImage.startsWith('http')) ||
      (g.backgroundImage && g.backgroundImage.startsWith('http'))
    );
    const hasScreens = Array.isArray(g.screenshots) && g.screenshots.length >= 2;
    const hasTitle = !!getDisplayTitle(g);
    return hasImage && (hasScreens || g.shortDescription) && hasTitle;
  });

  const seenKeys = new Set<string>();
  const seenIds = new Set<number>();
  const result: GameEntity[] = [];

  const sorted = candidates.slice().sort((a, b) => {
    const revA = (a.reviewPercent || 0) * Math.log10(Math.max(10, a.totalReviews || 1));
    const revB = (b.reviewPercent || 0) * Math.log10(Math.max(10, b.totalReviews || 1));
    if (revB !== revA) return revB - revA;

    const dateA = a.uploadDate || a.createdAt || '';
    const dateB = b.uploadDate || b.createdAt || '';
    return dateB.localeCompare(dateA);
  });

  for (const g of sorted) {
    if (!g || seenIds.has(g.id)) continue;
    const key = normalizeTitleKey(getDisplayTitle(g));
    if (key && seenKeys.has(key)) continue;

    seenIds.add(g.id);
    if (key) seenKeys.add(key);
    result.push(g);
    if (result.length >= limit) break;
  }

  return result;
}

/**
 * Top rated games (Steam reviews >= 80% or high score), excluding games already shown.
 */
export function getTopRatedGames(
  games: GameEntity[],
  limit: number = 18,
  seenIds: Set<number> = new Set(),
  seenKeys: Set<string> = new Set()
): GameEntity[] {
  if (!games || games.length === 0) return [];

  const candidates = games
    .filter((g) => {
      if (!g || seenIds.has(g.id)) return false;
      const key = normalizeTitleKey(getDisplayTitle(g));
      if (key && seenKeys.has(key)) return false;

      const pct = g.reviewPercent || 0;
      const count = g.totalReviews || 0;
      return pct >= 80 && count >= 50 && hasValidCover(g);
    })
    .sort((a, b) => {
      const scoreA = (a.reviewPercent || 0) * 1000 + Math.min(500, a.totalReviews || 0);
      const scoreB = (b.reviewPercent || 0) * 1000 + Math.min(500, b.totalReviews || 0);
      return scoreB - scoreA;
    });

  const result: GameEntity[] = [];
  for (const g of candidates) {
    if (seenIds.has(g.id)) continue;
    const key = normalizeTitleKey(getDisplayTitle(g));
    if (key && seenKeys.has(key)) continue;

    result.push(g);
    seenIds.add(g.id);
    if (key) seenKeys.add(key);
    if (result.length >= limit) break;
  }

  return result;
}

/**
 * Recently added/updated games, excluding games already shown.
 */
export function getRecentGames(
  games: GameEntity[],
  limit: number = 18,
  seenIds: Set<number> = new Set(),
  seenKeys: Set<string> = new Set()
): GameEntity[] {
  if (!games || games.length === 0) return [];

  const candidates = games
    .filter((g) => {
      if (!g || seenIds.has(g.id) || !hasValidCover(g)) return false;
      const key = normalizeTitleKey(getDisplayTitle(g));
      if (key && seenKeys.has(key)) return false;
      return true;
    })
    .slice()
    .sort((a, b) => {
      const dateA = a.uploadDate || a.createdAt || '';
      const dateB = b.uploadDate || b.createdAt || '';
      if (dateA && dateB && dateA !== dateB) {
        return dateB.localeCompare(dateA);
      }
      return (b.id || 0) - (a.id || 0);
    });

  const result: GameEntity[] = [];
  for (const g of candidates) {
    if (seenIds.has(g.id)) continue;
    const key = normalizeTitleKey(getDisplayTitle(g));
    if (key && seenKeys.has(key)) continue;

    result.push(g);
    seenIds.add(g.id);
    if (key) seenKeys.add(key);
    if (result.length >= limit) break;
  }

  return result;
}

/**
 * Filter games by specific genre or tag keywords with deduplication
 */
export function getGamesByGenre(
  games: GameEntity[],
  keywords: string[],
  limit: number = 18,
  seenIds: Set<number> = new Set(),
  seenKeys: Set<string> = new Set()
): GameEntity[] {
  if (!games || games.length === 0 || !keywords || keywords.length === 0) return [];

  const lowerKeywords = keywords.map((k) => k.toLowerCase());

  const candidates = games.filter((g) => {
    if (!g || seenIds.has(g.id) || !hasValidCover(g)) return false;
    const key = normalizeTitleKey(getDisplayTitle(g));
    if (key && seenKeys.has(key)) return false;

    const genres = Array.isArray(g.genres) ? g.genres.map((x) => x.toLowerCase()) : [];
    const tags = Array.isArray(g.tags) ? g.tags.map((x) => x.toLowerCase()) : [];
    const combined = [...genres, ...tags];

    return lowerKeywords.some((kw) => combined.some((item) => item.includes(kw)));
  });

  const result: GameEntity[] = [];
  for (const g of candidates) {
    if (seenIds.has(g.id)) continue;
    const key = normalizeTitleKey(getDisplayTitle(g));
    if (key && seenKeys.has(key)) continue;

    result.push(g);
    seenIds.add(g.id);
    if (key) seenKeys.add(key);
    if (result.length >= limit) break;
  }

  return result;
}

export function getIndieHighlights(
  games: GameEntity[],
  limit: number = 18,
  seenIds?: Set<number>,
  seenKeys?: Set<string>
): GameEntity[] {
  return getGamesByGenre(games, ['indie', 'инди'], limit, seenIds, seenKeys);
}

export function getRpgHighlights(
  games: GameEntity[],
  limit: number = 18,
  seenIds?: Set<number>,
  seenKeys?: Set<string>
): GameEntity[] {
  return getGamesByGenre(games, ['rpg', 'ролевая', 'ролевые'], limit, seenIds, seenKeys);
}

export function getActionHighlights(
  games: GameEntity[],
  limit: number = 18,
  seenIds?: Set<number>,
  seenKeys?: Set<string>
): GameEntity[] {
  return getGamesByGenre(games, ['action', 'экшен', 'боевик', 'shooter', 'шутер'], limit, seenIds, seenKeys);
}

export function getStrategyHighlights(
  games: GameEntity[],
  limit: number = 18,
  seenIds?: Set<number>,
  seenKeys?: Set<string>
): GameEntity[] {
  return getGamesByGenre(games, ['strategy', 'стратегия'], limit, seenIds, seenKeys);
}
