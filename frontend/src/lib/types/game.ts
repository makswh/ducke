export interface SteamMovie {
  id: number;
  name: string;
  thumbnail: string;
  mp4?: string;
  webm?: string;
  hls?: string;
}

export interface GameEntity {
  id: number;
  rawName: string;
  cleanTitle: string;
  searchTitle: string;
  remotePath: string;
  sizeBytes: number;
  sizeDisplay: string;
  isDirectory: boolean;
  isCollection: boolean;
  parentPath: string;
  steamAppId: number;
  steamSynced: boolean;
  steamTitle?: string;
  shortDescription?: string;
  detailedDescription?: string;
  headerImage?: string;
  capsuleImage?: string;
  backgroundImage?: string;
  screenshots?: string[];
  movies?: SteamMovie[];
  genres?: string[];
  developers?: string[];
  publishers?: string[];
  releaseDate?: string;
  controllerSupport?: string;
  pcRequirements?: string;
  metacriticScore?: number;
  reviewScoreDesc?: string;
  reviewPercent?: number;
  totalReviews?: number;
}

export type MediaItem =
  | { type: 'video'; id: number; name: string; thumbnail: string; mp4?: string; webm?: string; hls?: string; url?: string }
  | { type: 'image'; id: number; name: string; url: string };

export interface SteamCandidateItem {
  appId: number;
  name: string;
  tinyImage: string;
  score: number;
}

export interface DownloadProgressEvent {
  downloadId: string;
  gameId: number;
  gameTitle: string;
  coverImage?: string;
  downloadedBytes: number;
  totalBytes: number;
  progressPercent: number;
  speedBytesPerSec: number;
  speedDisplay: string;
  etaSeconds: number;
  etaDisplay: string;
  status: string;
  currentFile: string;
  fileIndex: number;
  totalFiles: number;
  localPath: string;
  errorMessage?: string;
}

export interface GamePageDetails {
  game: GameEntity;
  downloadStatus: string;
  downloadProgress?: DownloadProgressEvent;
  localPath?: string;
  isInstalled: boolean;
  logoUrl?: string;
  bannerUrl?: string;
  coverUrl?: string;
  backgroundUrl?: string;
}
