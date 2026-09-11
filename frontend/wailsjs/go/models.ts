export namespace config {
	
	export class TorrentSourceConfig {
	    id: string;
	    name: string;
	    url: string;
	    enabled: boolean;
	    itemCount: number;
	    lastSynced: number;
	
	    static createFrom(source: any = {}) {
	        return new TorrentSourceConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.enabled = source["enabled"];
	        this.itemCount = source["itemCount"];
	        this.lastSynced = source["lastSynced"];
	    }
	}
	export class ServerConfig {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    protocol: string;
	    user: string;
	    password?: string;
	    remoteDir: string;
	    isActive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ServerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.user = source["user"];
	        this.password = source["password"];
	        this.remoteDir = source["remoteDir"];
	        this.isActive = source["isActive"];
	    }
	}
	export class AppSettings {
	    downloadPath: string;
	    maxConcurrentFiles: number;
	    maxSpeedKBps: number;
	    theme: string;
	    steamDeckMode: boolean;
	    steamApiKey: string;
	    enableLogs: boolean;
	    activeServer?: ServerConfig;
	    savedServers: ServerConfig[];
	    torrentSources: TorrentSourceConfig[];
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadPath = source["downloadPath"];
	        this.maxConcurrentFiles = source["maxConcurrentFiles"];
	        this.maxSpeedKBps = source["maxSpeedKBps"];
	        this.theme = source["theme"];
	        this.steamDeckMode = source["steamDeckMode"];
	        this.steamApiKey = source["steamApiKey"];
	        this.enableLogs = source["enableLogs"];
	        this.activeServer = this.convertValues(source["activeServer"], ServerConfig);
	        this.savedServers = this.convertValues(source["savedServers"], ServerConfig);
	        this.torrentSources = this.convertValues(source["torrentSources"], TorrentSourceConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace database {
	
	export class DownloadRecord {
	    id: string;
	    gameId: number;
	    gameTitle: string;
	    remotePath: string;
	    localPath: string;
	    totalBytes: number;
	    downloadedBytes: number;
	    status: string;
	    errorMessage: string;
	    isTorrent: boolean;
	    magnetUri?: string;
	    createdAt: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.gameId = source["gameId"];
	        this.gameTitle = source["gameTitle"];
	        this.remotePath = source["remotePath"];
	        this.localPath = source["localPath"];
	        this.totalBytes = source["totalBytes"];
	        this.downloadedBytes = source["downloadedBytes"];
	        this.status = source["status"];
	        this.errorMessage = source["errorMessage"];
	        this.isTorrent = source["isTorrent"];
	        this.magnetUri = source["magnetUri"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class GameVariant {
	    id: number;
	    rawName: string;
	    cleanTitle: string;
	    sizeBytes: number;
	    sizeDisplay: string;
	    sourceType: string;
	    torrentSource?: string;
	    remotePath: string;
	    magnetUri?: string;
	    uploadDate?: string;
	    isDirectory: boolean;
	    steamAppId?: number;
	
	    static createFrom(source: any = {}) {
	        return new GameVariant(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.rawName = source["rawName"];
	        this.cleanTitle = source["cleanTitle"];
	        this.sizeBytes = source["sizeBytes"];
	        this.sizeDisplay = source["sizeDisplay"];
	        this.sourceType = source["sourceType"];
	        this.torrentSource = source["torrentSource"];
	        this.remotePath = source["remotePath"];
	        this.magnetUri = source["magnetUri"];
	        this.uploadDate = source["uploadDate"];
	        this.isDirectory = source["isDirectory"];
	        this.steamAppId = source["steamAppId"];
	    }
	}
	export class SteamMovie {
	    id: number;
	    name: string;
	    thumbnail: string;
	    mp4?: string;
	    webm?: string;
	    hls?: string;
	
	    static createFrom(source: any = {}) {
	        return new SteamMovie(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.thumbnail = source["thumbnail"];
	        this.mp4 = source["mp4"];
	        this.webm = source["webm"];
	        this.hls = source["hls"];
	    }
	}
	export class GameEntity {
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
	    iconUrl?: string;
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
	    sourceType?: string;
	    torrentSource?: string;
	    magnetUri?: string;
	    uploadDate?: string;
	    favoriteStatus?: string;
	    variants?: GameVariant[];
	
	    static createFrom(source: any = {}) {
	        return new GameEntity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.rawName = source["rawName"];
	        this.cleanTitle = source["cleanTitle"];
	        this.searchTitle = source["searchTitle"];
	        this.remotePath = source["remotePath"];
	        this.sizeBytes = source["sizeBytes"];
	        this.sizeDisplay = source["sizeDisplay"];
	        this.isDirectory = source["isDirectory"];
	        this.isCollection = source["isCollection"];
	        this.parentPath = source["parentPath"];
	        this.steamAppId = source["steamAppId"];
	        this.steamSynced = source["steamSynced"];
	        this.steamTitle = source["steamTitle"];
	        this.shortDescription = source["shortDescription"];
	        this.detailedDescription = source["detailedDescription"];
	        this.headerImage = source["headerImage"];
	        this.capsuleImage = source["capsuleImage"];
	        this.backgroundImage = source["backgroundImage"];
	        this.iconUrl = source["iconUrl"];
	        this.screenshots = source["screenshots"];
	        this.movies = this.convertValues(source["movies"], SteamMovie);
	        this.genres = source["genres"];
	        this.developers = source["developers"];
	        this.publishers = source["publishers"];
	        this.releaseDate = source["releaseDate"];
	        this.controllerSupport = source["controllerSupport"];
	        this.pcRequirements = source["pcRequirements"];
	        this.metacriticScore = source["metacriticScore"];
	        this.reviewScoreDesc = source["reviewScoreDesc"];
	        this.reviewPercent = source["reviewPercent"];
	        this.totalReviews = source["totalReviews"];
	        this.sourceType = source["sourceType"];
	        this.torrentSource = source["torrentSource"];
	        this.magnetUri = source["magnetUri"];
	        this.uploadDate = source["uploadDate"];
	        this.favoriteStatus = source["favoriteStatus"];
	        this.variants = this.convertValues(source["variants"], GameVariant);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FavoriteItem {
	    gameId: number;
	    status: string;
	    addedAt: string;
	    updatedAt: string;
	    game: GameEntity;
	
	    static createFrom(source: any = {}) {
	        return new FavoriteItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameId = source["gameId"];
	        this.status = source["status"];
	        this.addedAt = source["addedAt"];
	        this.updatedAt = source["updatedAt"];
	        this.game = this.convertValues(source["game"], GameEntity);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	

}

export namespace downloader {
	
	export class DownloadProgressEvent {
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
	    isTorrent: boolean;
	    magnetUri?: string;
	    torrentSeeds: number;
	    torrentPeers: number;
	
	    static createFrom(source: any = {}) {
	        return new DownloadProgressEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadId = source["downloadId"];
	        this.gameId = source["gameId"];
	        this.gameTitle = source["gameTitle"];
	        this.coverImage = source["coverImage"];
	        this.downloadedBytes = source["downloadedBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.progressPercent = source["progressPercent"];
	        this.speedBytesPerSec = source["speedBytesPerSec"];
	        this.speedDisplay = source["speedDisplay"];
	        this.etaSeconds = source["etaSeconds"];
	        this.etaDisplay = source["etaDisplay"];
	        this.status = source["status"];
	        this.currentFile = source["currentFile"];
	        this.fileIndex = source["fileIndex"];
	        this.totalFiles = source["totalFiles"];
	        this.localPath = source["localPath"];
	        this.errorMessage = source["errorMessage"];
	        this.isTorrent = source["isTorrent"];
	        this.magnetUri = source["magnetUri"];
	        this.torrentSeeds = source["torrentSeeds"];
	        this.torrentPeers = source["torrentPeers"];
	    }
	}
	export class StorageDriveInfo {
	    id: string;
	    label: string;
	    path: string;
	    type: string;
	    freeBytes: number;
	    totalBytes: number;
	    usedBytes: number;
	    duckeBytes: number;
	    freeGB: string;
	    totalGB: string;
	    usedGB: string;
	    duckeGB: string;
	    isDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StorageDriveInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.path = source["path"];
	        this.type = source["type"];
	        this.freeBytes = source["freeBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.usedBytes = source["usedBytes"];
	        this.duckeBytes = source["duckeBytes"];
	        this.freeGB = source["freeGB"];
	        this.totalGB = source["totalGB"];
	        this.usedGB = source["usedGB"];
	        this.duckeGB = source["duckeGB"];
	        this.isDefault = source["isDefault"];
	    }
	}

}

export namespace logger {
	
	export class LogEntry {
	    id: number;
	    timestamp: string;
	    level: string;
	    source: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.level = source["level"];
	        this.source = source["source"];
	        this.message = source["message"];
	    }
	}

}

export namespace main {
	
	export class AppInfo {
	    name: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	    }
	}
	export class ConnectionTestResult {
	    success: boolean;
	    protocolUsed: string;
	    errorMessage: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.protocolUsed = source["protocolUsed"];
	        this.errorMessage = source["errorMessage"];
	    }
	}
	export class DiskSpaceInfo {
	    freeBytes: number;
	    totalBytes: number;
	    freeGB: string;
	    totalGB: string;
	
	    static createFrom(source: any = {}) {
	        return new DiskSpaceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.freeBytes = source["freeBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.freeGB = source["freeGB"];
	        this.totalGB = source["totalGB"];
	    }
	}
	export class GamePageDetails {
	    game: database.GameEntity;
	    downloadStatus: string;
	    downloadProgress?: downloader.DownloadProgressEvent;
	    localPath?: string;
	    isInstalled: boolean;
	    logoUrl?: string;
	    bannerUrl?: string;
	    coverUrl?: string;
	    backgroundUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new GamePageDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game = this.convertValues(source["game"], database.GameEntity);
	        this.downloadStatus = source["downloadStatus"];
	        this.downloadProgress = this.convertValues(source["downloadProgress"], downloader.DownloadProgressEvent);
	        this.localPath = source["localPath"];
	        this.isInstalled = source["isInstalled"];
	        this.logoUrl = source["logoUrl"];
	        this.bannerUrl = source["bannerUrl"];
	        this.coverUrl = source["coverUrl"];
	        this.backgroundUrl = source["backgroundUrl"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SteamSearchResult {
	    id: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new SteamSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}

}

export namespace metadata {
	
	export class MetadataProgress {
	    isSyncing: boolean;
	    current: number;
	    total: number;
	    currentGame: string;
	
	    static createFrom(source: any = {}) {
	        return new MetadataProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isSyncing = source["isSyncing"];
	        this.current = source["current"];
	        this.total = source["total"];
	        this.currentGame = source["currentGame"];
	    }
	}
	export class SteamCandidate {
	    appId: number;
	    name: string;
	    tinyImage: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new SteamCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.name = source["name"];
	        this.tinyImage = source["tinyImage"];
	        this.score = source["score"];
	    }
	}

}

