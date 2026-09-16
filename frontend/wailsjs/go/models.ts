export namespace collections {
	
	export class CompilationGame {
	    stopGameId: string;
	    title: string;
	    url: string;
	    posterUrl: string;
	    stopGameScore: string;
	    inLibrary: boolean;
	    duckeGame?: database.GameEntity;
	
	    static createFrom(source: any = {}) {
	        return new CompilationGame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stopGameId = source["stopGameId"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.posterUrl = source["posterUrl"];
	        this.stopGameScore = source["stopGameScore"];
	        this.inLibrary = source["inLibrary"];
	        this.duckeGame = this.convertValues(source["duckeGame"], database.GameEntity);
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
	export class CompilationDetail {
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
	
	    static createFrom(source: any = {}) {
	        return new CompilationDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.authorName = source["authorName"];
	        this.authorAvatar = source["authorAvatar"];
	        this.authorUrl = source["authorUrl"];
	        this.gamesCount = source["gamesCount"];
	        this.rating = source["rating"];
	        this.description = source["description"];
	        this.lastUpdated = source["lastUpdated"];
	        this.matchedCount = source["matchedCount"];
	        this.games = this.convertValues(source["games"], CompilationGame);
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
	
	export class CompilationSummary {
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
	
	    static createFrom(source: any = {}) {
	        return new CompilationSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.authorName = source["authorName"];
	        this.authorAvatar = source["authorAvatar"];
	        this.authorUrl = source["authorUrl"];
	        this.gamesCount = source["gamesCount"];
	        this.commentsCount = source["commentsCount"];
	        this.rating = source["rating"];
	        this.description = source["description"];
	        this.previewImages = source["previewImages"];
	        this.matchedCount = source["matchedCount"];
	    }
	}
	export class CompilationsResponse {
	    items: CompilationSummary[];
	    totalPages: number;
	    currentPage: number;
	    sort: string;
	
	    static createFrom(source: any = {}) {
	        return new CompilationsResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], CompilationSummary);
	        this.totalPages = source["totalPages"];
	        this.currentPage = source["currentPage"];
	        this.sort = source["sort"];
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
	    remotePath?: string;
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
	    canonicalKey?: string;
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
	    tags?: string[];
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
	        this.canonicalKey = source["canonicalKey"];
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
	        this.tags = source["tags"];
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
	    customExePath: string;
	    launchArguments: string;
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
	        this.customExePath = source["customExePath"];
	        this.launchArguments = source["launchArguments"];
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

export namespace http {
	
	export class Response {
	    Status: string;
	    StatusCode: number;
	    Proto: string;
	    ProtoMajor: number;
	    ProtoMinor: number;
	    Header: Record<string, Array<string>>;
	    Body: any;
	    ContentLength: number;
	    TransferEncoding: string[];
	    Close: boolean;
	    Uncompressed: boolean;
	    Trailer: Record<string, Array<string>>;
	    Request?: Request;
	    TLS?: tls.ConnectionState;
	
	    static createFrom(source: any = {}) {
	        return new Response(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Status = source["Status"];
	        this.StatusCode = source["StatusCode"];
	        this.Proto = source["Proto"];
	        this.ProtoMajor = source["ProtoMajor"];
	        this.ProtoMinor = source["ProtoMinor"];
	        this.Header = source["Header"];
	        this.Body = source["Body"];
	        this.ContentLength = source["ContentLength"];
	        this.TransferEncoding = source["TransferEncoding"];
	        this.Close = source["Close"];
	        this.Uncompressed = source["Uncompressed"];
	        this.Trailer = source["Trailer"];
	        this.Request = this.convertValues(source["Request"], Request);
	        this.TLS = this.convertValues(source["TLS"], tls.ConnectionState);
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
	export class Request {
	    Method: string;
	    URL?: url.URL;
	    Proto: string;
	    ProtoMajor: number;
	    ProtoMinor: number;
	    Header: Record<string, Array<string>>;
	    Body: any;
	    ContentLength: number;
	    TransferEncoding: string[];
	    Close: boolean;
	    Host: string;
	    Form: Record<string, Array<string>>;
	    PostForm: Record<string, Array<string>>;
	    MultipartForm?: multipart.Form;
	    Trailer: Record<string, Array<string>>;
	    RemoteAddr: string;
	    RequestURI: string;
	    TLS?: tls.ConnectionState;
	    Response?: Response;
	    Pattern: string;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Method = source["Method"];
	        this.URL = this.convertValues(source["URL"], url.URL);
	        this.Proto = source["Proto"];
	        this.ProtoMajor = source["ProtoMajor"];
	        this.ProtoMinor = source["ProtoMinor"];
	        this.Header = source["Header"];
	        this.Body = source["Body"];
	        this.ContentLength = source["ContentLength"];
	        this.TransferEncoding = source["TransferEncoding"];
	        this.Close = source["Close"];
	        this.Host = source["Host"];
	        this.Form = source["Form"];
	        this.PostForm = source["PostForm"];
	        this.MultipartForm = this.convertValues(source["MultipartForm"], multipart.Form);
	        this.Trailer = source["Trailer"];
	        this.RemoteAddr = source["RemoteAddr"];
	        this.RequestURI = source["RequestURI"];
	        this.TLS = this.convertValues(source["TLS"], tls.ConnectionState);
	        this.Response = this.convertValues(source["Response"], Response);
	        this.Pattern = source["Pattern"];
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

export namespace multipart {
	
	export class FileHeader {
	    Filename: string;
	    Header: Record<string, Array<string>>;
	    Size: number;
	
	    static createFrom(source: any = {}) {
	        return new FileHeader(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Filename = source["Filename"];
	        this.Header = source["Header"];
	        this.Size = source["Size"];
	    }
	}
	export class Form {
	    Value: Record<string, Array<string>>;
	    File: Record<string, Array<FileHeader>>;
	
	    static createFrom(source: any = {}) {
	        return new Form(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Value = source["Value"];
	        this.File = this.convertValues(source["File"], Array<FileHeader>, true);
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

export namespace net {
	
	export class IPNet {
	    IP: number[];
	    Mask: number[];
	
	    static createFrom(source: any = {}) {
	        return new IPNet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.IP = source["IP"];
	        this.Mask = source["Mask"];
	    }
	}

}

export namespace pkix {
	
	export class AttributeTypeAndValue {
	    Type: number[];
	    Value: any;
	
	    static createFrom(source: any = {}) {
	        return new AttributeTypeAndValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Type = source["Type"];
	        this.Value = source["Value"];
	    }
	}
	export class Extension {
	    Id: number[];
	    Critical: boolean;
	    Value: number[];
	
	    static createFrom(source: any = {}) {
	        return new Extension(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Id = source["Id"];
	        this.Critical = source["Critical"];
	        this.Value = source["Value"];
	    }
	}
	export class Name {
	    Country: string[];
	    Organization: string[];
	    OrganizationalUnit: string[];
	    Locality: string[];
	    Province: string[];
	    StreetAddress: string[];
	    PostalCode: string[];
	    SerialNumber: string;
	    CommonName: string;
	    Names: AttributeTypeAndValue[];
	    ExtraNames: AttributeTypeAndValue[];
	
	    static createFrom(source: any = {}) {
	        return new Name(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Country = source["Country"];
	        this.Organization = source["Organization"];
	        this.OrganizationalUnit = source["OrganizationalUnit"];
	        this.Locality = source["Locality"];
	        this.Province = source["Province"];
	        this.StreetAddress = source["StreetAddress"];
	        this.PostalCode = source["PostalCode"];
	        this.SerialNumber = source["SerialNumber"];
	        this.CommonName = source["CommonName"];
	        this.Names = this.convertValues(source["Names"], AttributeTypeAndValue);
	        this.ExtraNames = this.convertValues(source["ExtraNames"], AttributeTypeAndValue);
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

export namespace tls {
	
	export class ConnectionState {
	    Version: number;
	    HandshakeComplete: boolean;
	    DidResume: boolean;
	    CipherSuite: number;
	    CurveID: number;
	    NegotiatedProtocol: string;
	    NegotiatedProtocolIsMutual: boolean;
	    ServerName: string;
	    PeerCertificates: x509.Certificate[];
	    VerifiedChains: x509.Certificate[][];
	    SignedCertificateTimestamps: number[][];
	    OCSPResponse: number[];
	    TLSUnique: number[];
	    ECHAccepted: boolean;
	    HelloRetryRequest: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Version = source["Version"];
	        this.HandshakeComplete = source["HandshakeComplete"];
	        this.DidResume = source["DidResume"];
	        this.CipherSuite = source["CipherSuite"];
	        this.CurveID = source["CurveID"];
	        this.NegotiatedProtocol = source["NegotiatedProtocol"];
	        this.NegotiatedProtocolIsMutual = source["NegotiatedProtocolIsMutual"];
	        this.ServerName = source["ServerName"];
	        this.PeerCertificates = this.convertValues(source["PeerCertificates"], x509.Certificate);
	        this.VerifiedChains = this.convertValues(source["VerifiedChains"], x509.Certificate);
	        this.SignedCertificateTimestamps = source["SignedCertificateTimestamps"];
	        this.OCSPResponse = source["OCSPResponse"];
	        this.TLSUnique = source["TLSUnique"];
	        this.ECHAccepted = source["ECHAccepted"];
	        this.HelloRetryRequest = source["HelloRetryRequest"];
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

export namespace url {
	
	export class Userinfo {
	
	
	    static createFrom(source: any = {}) {
	        return new Userinfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class URL {
	    Scheme: string;
	    Opaque: string;
	    // Go type: Userinfo
	    User?: any;
	    Host: string;
	    Path: string;
	    Fragment: string;
	    RawQuery: string;
	    RawPath: string;
	    RawFragment: string;
	    ForceQuery: boolean;
	    OmitHost: boolean;
	
	    static createFrom(source: any = {}) {
	        return new URL(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Scheme = source["Scheme"];
	        this.Opaque = source["Opaque"];
	        this.User = this.convertValues(source["User"], null);
	        this.Host = source["Host"];
	        this.Path = source["Path"];
	        this.Fragment = source["Fragment"];
	        this.RawQuery = source["RawQuery"];
	        this.RawPath = source["RawPath"];
	        this.RawFragment = source["RawFragment"];
	        this.ForceQuery = source["ForceQuery"];
	        this.OmitHost = source["OmitHost"];
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

export namespace x509 {
	
	export class PolicyMapping {
	    // Go type: OID
	    IssuerDomainPolicy: any;
	    // Go type: OID
	    SubjectDomainPolicy: any;
	
	    static createFrom(source: any = {}) {
	        return new PolicyMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.IssuerDomainPolicy = this.convertValues(source["IssuerDomainPolicy"], null);
	        this.SubjectDomainPolicy = this.convertValues(source["SubjectDomainPolicy"], null);
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
	export class OID {
	
	
	    static createFrom(source: any = {}) {
	        return new OID(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class Certificate {
	    Raw: number[];
	    RawTBSCertificate: number[];
	    RawSubjectPublicKeyInfo: number[];
	    RawSubject: number[];
	    RawIssuer: number[];
	    Signature: number[];
	    SignatureAlgorithm: number;
	    PublicKeyAlgorithm: number;
	    PublicKey: any;
	    Version: number;
	    // Go type: big
	    SerialNumber?: any;
	    Issuer: pkix.Name;
	    Subject: pkix.Name;
	    // Go type: time
	    NotBefore: any;
	    // Go type: time
	    NotAfter: any;
	    KeyUsage: number;
	    Extensions: pkix.Extension[];
	    ExtraExtensions: pkix.Extension[];
	    UnhandledCriticalExtensions: number[][];
	    ExtKeyUsage: number[];
	    UnknownExtKeyUsage: number[][];
	    BasicConstraintsValid: boolean;
	    IsCA: boolean;
	    MaxPathLen: number;
	    MaxPathLenZero: boolean;
	    SubjectKeyId: number[];
	    AuthorityKeyId: number[];
	    OCSPServer: string[];
	    IssuingCertificateURL: string[];
	    DNSNames: string[];
	    EmailAddresses: string[];
	    IPAddresses: number[][];
	    URIs: url.URL[];
	    PermittedDNSDomainsCritical: boolean;
	    PermittedDNSDomains: string[];
	    ExcludedDNSDomains: string[];
	    PermittedIPRanges: net.IPNet[];
	    ExcludedIPRanges: net.IPNet[];
	    PermittedEmailAddresses: string[];
	    ExcludedEmailAddresses: string[];
	    PermittedURIDomains: string[];
	    ExcludedURIDomains: string[];
	    CRLDistributionPoints: string[];
	    PolicyIdentifiers: number[][];
	    Policies: OID[];
	    InhibitAnyPolicy: number;
	    InhibitAnyPolicyZero: boolean;
	    InhibitPolicyMapping: number;
	    InhibitPolicyMappingZero: boolean;
	    RequireExplicitPolicy: number;
	    RequireExplicitPolicyZero: boolean;
	    PolicyMappings: PolicyMapping[];
	
	    static createFrom(source: any = {}) {
	        return new Certificate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Raw = source["Raw"];
	        this.RawTBSCertificate = source["RawTBSCertificate"];
	        this.RawSubjectPublicKeyInfo = source["RawSubjectPublicKeyInfo"];
	        this.RawSubject = source["RawSubject"];
	        this.RawIssuer = source["RawIssuer"];
	        this.Signature = source["Signature"];
	        this.SignatureAlgorithm = source["SignatureAlgorithm"];
	        this.PublicKeyAlgorithm = source["PublicKeyAlgorithm"];
	        this.PublicKey = source["PublicKey"];
	        this.Version = source["Version"];
	        this.SerialNumber = this.convertValues(source["SerialNumber"], null);
	        this.Issuer = this.convertValues(source["Issuer"], pkix.Name);
	        this.Subject = this.convertValues(source["Subject"], pkix.Name);
	        this.NotBefore = this.convertValues(source["NotBefore"], null);
	        this.NotAfter = this.convertValues(source["NotAfter"], null);
	        this.KeyUsage = source["KeyUsage"];
	        this.Extensions = this.convertValues(source["Extensions"], pkix.Extension);
	        this.ExtraExtensions = this.convertValues(source["ExtraExtensions"], pkix.Extension);
	        this.UnhandledCriticalExtensions = source["UnhandledCriticalExtensions"];
	        this.ExtKeyUsage = source["ExtKeyUsage"];
	        this.UnknownExtKeyUsage = source["UnknownExtKeyUsage"];
	        this.BasicConstraintsValid = source["BasicConstraintsValid"];
	        this.IsCA = source["IsCA"];
	        this.MaxPathLen = source["MaxPathLen"];
	        this.MaxPathLenZero = source["MaxPathLenZero"];
	        this.SubjectKeyId = source["SubjectKeyId"];
	        this.AuthorityKeyId = source["AuthorityKeyId"];
	        this.OCSPServer = source["OCSPServer"];
	        this.IssuingCertificateURL = source["IssuingCertificateURL"];
	        this.DNSNames = source["DNSNames"];
	        this.EmailAddresses = source["EmailAddresses"];
	        this.IPAddresses = source["IPAddresses"];
	        this.URIs = this.convertValues(source["URIs"], url.URL);
	        this.PermittedDNSDomainsCritical = source["PermittedDNSDomainsCritical"];
	        this.PermittedDNSDomains = source["PermittedDNSDomains"];
	        this.ExcludedDNSDomains = source["ExcludedDNSDomains"];
	        this.PermittedIPRanges = this.convertValues(source["PermittedIPRanges"], net.IPNet);
	        this.ExcludedIPRanges = this.convertValues(source["ExcludedIPRanges"], net.IPNet);
	        this.PermittedEmailAddresses = source["PermittedEmailAddresses"];
	        this.ExcludedEmailAddresses = source["ExcludedEmailAddresses"];
	        this.PermittedURIDomains = source["PermittedURIDomains"];
	        this.ExcludedURIDomains = source["ExcludedURIDomains"];
	        this.CRLDistributionPoints = source["CRLDistributionPoints"];
	        this.PolicyIdentifiers = source["PolicyIdentifiers"];
	        this.Policies = this.convertValues(source["Policies"], OID);
	        this.InhibitAnyPolicy = source["InhibitAnyPolicy"];
	        this.InhibitAnyPolicyZero = source["InhibitAnyPolicyZero"];
	        this.InhibitPolicyMapping = source["InhibitPolicyMapping"];
	        this.InhibitPolicyMappingZero = source["InhibitPolicyMappingZero"];
	        this.RequireExplicitPolicy = source["RequireExplicitPolicy"];
	        this.RequireExplicitPolicyZero = source["RequireExplicitPolicyZero"];
	        this.PolicyMappings = this.convertValues(source["PolicyMappings"], PolicyMapping);
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

