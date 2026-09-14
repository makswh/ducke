import type { GameEntity } from './game';

export interface CompilationSummary {
  id: string;
  title: string;
  url: string;
  authorName: string;
  authorAvatar: string;
  authorUrl: string;
  gamesCount: number;
  commentsCount: number;
  rating: string;
  description: string;
  previewImages: string[];
  matchedCount: number;
}

export interface CompilationsResponse {
  items: CompilationSummary[];
  totalPages: number;
  currentPage: number;
  sort: string;
}

export interface CompilationGame {
  stopGameId: string;
  title: string;
  url: string;
  posterUrl: string;
  stopGameScore: string;
  inLibrary: boolean;
  duckeGame?: GameEntity;
}

export interface CompilationDetail {
  id: string;
  title: string;
  authorName: string;
  authorAvatar: string;
  authorUrl: string;
  gamesCount: number;
  rating: string;
  description: string;
  lastUpdated: string;
  matchedCount: number;
  games: CompilationGame[];
}
