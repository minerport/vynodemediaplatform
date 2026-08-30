export type User = {
  id: string;
  username: string;
  displayName: string;
  role: "OWNER" | "ADMIN" | "USER";
  status: string;
  createdAt: string;
};
export type SystemInfo = {
  version: string;
  instanceId: string;
  serverName: string;
  databaseType: string;
  operatingSystem: string;
  architecture: string;
  uptimeSeconds: number;
};
export type Session = {
  id: string;
  deviceName: string;
  clientName: string;
  platform: string;
  createdAt: string;
  lastActivityAt: string;
  current: boolean;
};
export type LibraryGrant={libraryId:string;libraryName?:string;permissions:("VIEW"|"PLAY"|"DOWNLOAD")[]};
export type OfflineDownload={id:string;logicalType:string;logicalId:string;profileId:string;status:string;mode:string;assetState:string;sizeBytes:number;checksumSha256?:string;progress:number;createdAt:string};
export type OfflineSettings={cacheQuotaBytes:number;cacheBytes:number;readyAssets:number;preparingAssets:number};
export type Invitation={id:string;identifier?:string;role:"ADMIN"|"USER";status:string;createdBy:string;createdAt:string;expiresAt:string;acceptedAt?:string;libraries:LibraryGrant[]};
export type PairingRequest={id:string;code?:string;challenge?:string;status:string;deviceName:string;clientName:string;clientVersion?:string;platform:string;platformVersion?:string;requestedAt:string;expiresAt:string};
export type RemoteSettings={discoveryEnabled:boolean;discoveryStatus:string;discoveryLastError?:string;portMappingEnabled:boolean;portMappingProtocol?:string;portMappingStatus:string;portMappingExternalPort?:number;portMappingLeaseExpiresAt?:string;portMappingLastError?:string;reverseProxyEnabled:boolean;insecureRemoteAllowed:boolean;manualRemoteUrl?:string;manualStatus:string;trustedProxyCidrs:string[];localNetworkCidrs:string[];updatedAt:string};
export type AuditEvent = {
  id: number;
  event: string;
  actorUserId: string;
  targetType: string;
  targetId: string;
  timestamp: string;
  metadata: Record<string, unknown>;
};
let accessToken = "";
let refreshFlight: Promise<User> | null = null;
export type LibrarySource = {
  id: string;
  configuredPath: string;
  normalizedPath: string;
  lastScanStatus?: string;
  lastSuccessfulScanAt?: string;
  lastScanError?: string;
};
export type Library = {
  id: string;
  name: string;
  type: "MOVIES" | "TV";
  enabled: boolean;
  sources?: LibrarySource[];
  fileCount: number;
  availableCount: number;
  missingCount: number;
  probeFailureCount: number;
};
export type ScanJob = {
  id: string;
  libraryId: string;
  state: string;
  filesDiscovered: number;
  candidatesFound: number;
  filesProbed: number;
  filesAdded: number;
  filesUpdated: number;
  filesUnchanged: number;
  filesMissing: number;
  filesFailed: number;
  currentRelativePath?: string;
};
export type MediaStream = {
  id: string;
  index: number;
  type: string;
  codec: string;
  profile: string;
  width?: number;
  height?: number;
  channels?: number;
  language?: string;
  title?: string;
  forced: boolean;
  default: boolean;
  colorTransfer?: string;
};
export type MediaFile = {
  id: string;
  sourceId: string;
  relativePath: string;
  fileName: string;
  sizeBytes: number;
  modifiedAtNs: number;
  availability: string;
  probeStatus: string;
  probeError?: string;
  containerFormat?: string;
  durationSeconds?: number;
  bitrate?: number;
  resolutionClass?: string;
  hdrClass?: string;
  candidateTitle?: string;
  candidateYear?: number;
  seasonNumber?: number;
  episodeStart?: number;
  episodeEnd?: number;
  streams?: MediaStream[];
};
export type MediaVersion = {
  id: string;
  fileId: string;
  label: string;
  resolution: string;
  codec: string;
  hdr: string;
};
export type Movie = {
  id: string;
  title: string;
  year?: number;
  releaseDate?: string;
  runtimeMinutes?: number;
  overview: string;
  contentRating?: string;
  rating?: number;
  voteCount?: number;
  genres: string[];
  versions?: MediaVersion[];
};
export type Episode = {
  id: string;
  episodeNumber: number;
  title: string;
  overview: string;
  airDate: string;
  runtimeMinutes: number;
  available: boolean;
};
export type Season = {
  id: string;
  seasonNumber: number;
  title: string;
  overview: string;
  airDate: string;
  episodes: Episode[];
};
export type Show = {
  id: string;
  title: string;
  year?: number;
  firstAirDate?: string;
  overview: string;
  rating?: number;
  genres: string[];
  seasons?: Season[];
};
export type LocalSearch = { movies: Movie[]; shows: Show[] };
export type ProviderStatus = {
  enabled: boolean;
  configured: boolean;
  language: string;
  region: string;
  status: string;
};
export type Unmatched = {
  fileId: string;
  fileName: string;
  candidateTitle: string;
  candidateYear: number;
  seasonNumber: number;
  episodeStart: number;
  episodeEnd: number;
  state: string;
  score: number;
  confidence: string;
  candidates: Array<{
    providerId: string;
    title: string;
    year: number;
    overview?: string;
  }>;
};
export type Artwork = {
  id: string;
  entityType: string;
  entityId: string;
  type: "POSTER" | "BACKDROP" | "LOGO" | "SEASON_POSTER" | "EPISODE_STILL";
  selected: boolean;
  manualSelection: boolean;
  cached: boolean;
  mimeType?: string;
};
export type CapabilityProfile = {
  schemaVersion: 2;
  clientName: string;
  clientVersion: string;
  platform: string;
  supportedContainers: string[];
  supportedVideoCodecs: string[];
  supportedAudioCodecs: string[];
  maximumVideoWidth: number;
  maximumVideoHeight: number;
  maximumAudioChannels: number;
  hdrCapabilities: string[];
  subtitleFormats: string[];
  directPlaySupport: boolean;
  fragmentedMp4Support: boolean;
};
export type PlaybackReason = { code: string; value?: string };
export type PlaybackTrack = {
  id: string;
  kind: string;
  codec: string;
  language?: string;
  title?: string;
  channels?: number;
  default: boolean;
  commentary?: boolean;
  forced?: boolean;
  hearingImpaired?: boolean;
  usable: boolean;
  reason?: string;
  source?: string;
};
export type PlaybackVersion = {
  id: string;
  container: string;
  videoCodec: string;
  audioCodecs: string[];
  audioTracks?: PlaybackTrack[];
  subtitleTracks?: PlaybackTrack[];
  width?: number;
  height?: number;
  resolution?: string;
  hdr?: string;
  label?: string;
  available: boolean;
};
export type PlaybackSession = {
  id: string;
  title?: string;
  logicalType: "MOVIE" | "EPISODE";
  logicalId: string;
  selectedVersion: PlaybackVersion;
  decision: {
    mode: "DIRECT_PLAY" | "DIRECT_STREAM" | "AUDIO_TRANSCODE" | "VIDEO_TRANSCODE" | "UNSUPPORTED";
    reasons: PlaybackReason[];
    plan: {
      video: { action: string; sourceCodec?: string; targetCodec?: string; targetWidth?: number; targetHeight?: number; targetBitrate?: number; encoder?: string };
      audio: { action: string; sourceCodec?: string; targetCodec?: string };
      container: { source: string; target: string };
      quality?: string;
      backend?: {requested:string;actual:string;fallbackReason?:string};
    };
  };
  state: string;
  position: number;
  duration: number;
  resumePosition: number;
  startedAt: string;
  lastActivityAt: string;
  userDisplayName?: string;
  clientName?: string;
  platform?: string;
  mediaUrl?: string;
  hlsUrl?: string;
  availableQualities?: Array<{id:string;label:string;maxWidth:number;maxHeight:number;targetVideoBitrate:number}>;
  subtitleUrl?: string;
  selectedAudioTrack?: PlaybackTrack;
  selectedSubtitleTrack?: PlaybackTrack;
  markers?: MediaMarker[];
  playbackContextId?: string;
  networkContext?: "LOCAL"|"REMOTE";
  effectiveBandwidthLimit?: number;
  navigation?: {autoplay:boolean;countdownSeconds:number;previous?:NavigationItem;next?:NavigationItem};
};
export type NavigationItem={logicalId:string;showId:string;showTitle:string;title:string;seasonNumber:number;episodeNumber:number;available:boolean};
export type PlaybackPreferences={preferredAudioLanguages:string[];preferredSubtitleLanguages:string[];subtitleMode:"OFF"|"ALWAYS"|"WHEN_AUDIO_NOT_PREFERRED"|"FORCED_ONLY";autoplayNextEpisode:boolean;localQualityId:string;remoteQualityId:string;avoidCommentary:boolean;preferHearingImpaired:boolean};
export type MediaMarker={id:string;logicalType:"MOVIE"|"EPISODE";logicalId:string;type:"INTRO"|"RECAP"|"CREDITS"|"POST_CREDITS"|"CUSTOM";start:number;end:number;source:string;confidence?:number};
export type IntelligenceJob={ID:string;Type:string;TargetType:string;TargetID:string;State:string;Progress:number;Error?:string;CreatedAt:string};
export type OptimizedMedia={ID:string;SourceMediaFileID:string;DerivedMediaFileID:string;LogicalType:string;LogicalID:string;Profile:string;Status:string;CreatedAt:string;SizeBytes:number};
export type MarkerCandidate={ID:string;LogicalType:string;LogicalID:string;Type:string;Source:string;ReviewState:string;SourceIdentity:string;Start:number;End:number;Confidence:number;ConfidenceClass:"HIGH"|"MEDIUM"|"LOW"};
export type AutomationRule={ID?:string;Name:string;Enabled:boolean;Trigger:string;Timezone:string;Schedule?:{Hour:number;Minute:number};Conditions:{Field:string;Operator:string;Value:unknown}[];Actions:{Type:string;Profile?:string;CollectionID?:string}[];LastExecutionAt?:string};
export type CurationItem={Type:"MOVIE"|"SHOW"|"EPISODE";ID:string;Title:string;Subtitle:string;ArtworkID:string;Availability:string;Year:number;Rating:number;Position:number};
export type Collection={ID:string;Name:string;Description:string;SortTitle:string;Scope:"SERVER_SHARED"|"USER_PRIVATE";OwnerUserID:string;Ordering:string;ArtworkItemType:string;ArtworkItemID:string;Items:CurationItem[]};
export type RuleNode={logic?:"ALL"|"ANY"|"NOT";children?:RuleNode[];field?:string;operator?:string;value?:unknown};
export type SmartCollection={ID?:string;Name:string;Description:string;Scope:"SERVER_SHARED"|"USER_PRIVATE";OwnerUserID?:string;RuleSchemaVersion:number;Rule:RuleNode;SortField:string;SortDirection:"ASC"|"DESC";Limit:number;ArtworkItemType?:string;ArtworkItemID?:string;Items?:CurationItem[]};
export type Playlist={ID:string;Name:string;Description:string;Items:CurationItem[]};
export type HomeRow={ID:string;Type:string;Title:string;SourceID:string;Enabled:boolean;Position:number;Limit:number;Items:CurationItem[];SeeAll:string};
export type OpsEvent={ID:string;Type:string;Category:string;Severity:string;CreatedAt:string;Payload:Record<string,unknown>};
export type HealthIssue={ID:string;Category:string;Severity:string;ReferenceType:string;ReferenceID:string;Description:string;Status:string;FirstDetectedAt:string;LastDetectedAt:string};
export type DiskMetric={Label:string;Path:string;TotalBytes:number;UsedBytes:number;AvailableBytes:number};
export type AdminDashboard={Version:string;Commit:string;ServerName:string;InstanceID:string;DatabaseType:string;FFmpegVersion:string;FFprobeVersion:string;Metrics:{UptimeSeconds:number;OperatingSystem:string;Architecture:string;GoRoutines:number;GoHeapBytes:number;GoSystemBytes:number;ProcessRSSBytes:number;SystemMemoryTotalBytes:number;SystemMemoryAvailableBytes:number;Disks:DiskMetric[];ActivePlaybackSessions:number;ActiveFFmpegProcesses:number};Health:Record<string,number>;Libraries:{Movies:number;Shows:number;Episodes:number;PhysicalFiles:number;AvailableFiles:number;MissingFiles:number;UnmatchedFiles:number;OptimizedVersions:number;Resolution:Record<string,number>;VideoCodecs:Record<string,number>;HDR:Record<string,number>};FailedJobs:number;RecentEvents:OpsEvent[]};
export type PlaybackAnalytics={From:string;To:string;TotalPlays:number;UniqueUsers:number;MoviesPlayed:number;EpisodesPlayed:number;PlaybackErrors:number;CompletionCount:number;PlaybackSeconds:number;Modes:Record<string,number>;TopMedia:Array<{Key:string;Label:string;Count:number}>;TopUsers:Array<{Key:string;Label:string;Count:number}>};
export type AdminJob={ID:string;Type:string;Target:string;State:string;Progress:number;Priority:number;CreatedAt:string;StartedAt:string;CompletedAt:string;Error:string};
export type WebhookDestination={ID?:string;Name:string;URL:string;Enabled:boolean;AllowPrivateNetwork:boolean;AllowInsecureHTTP:boolean;HasSecret?:boolean;MaxAttempts:number;EventTypes:string[];Secret?:string};
export type WebhookDelivery={ID:string;EventID:string;EventType:string;DestinationID:string;DestinationName:string;Status:string;AttemptCount:number;LastHTTPStatus:number;LastError:string;CreatedAt:string};
export type ContinueItem = {
  logicalType: "MOVIE" | "EPISODE";
  logicalId: string;
  title: string;
  position: number;
  duration: number;
  progress: number;
  lastPlayedAt: string;
};
export function browserCapabilities(): CapabilityProfile {
  const video = document.createElement("video"),
    mp4 =
      video.canPlayType('video/mp4; codecs="avc1.42E01E, mp4a.40.2"') !== "",
    webm = video.canPlayType('video/webm; codecs="vp9, opus"') !== "";
  return {
    schemaVersion: 2,
    clientName: "VyNode Web",
    clientVersion: "0.1.0",
    platform: navigator.platform || "web",
    supportedContainers: [...(mp4 ? ["mp4"] : []), ...(webm ? ["webm"] : [])],
    supportedVideoCodecs: [
      ...(mp4 ? ["h264"] : []),
      ...(webm ? ["vp8", "vp9"] : []),
    ],
    supportedAudioCodecs: [
      ...(mp4 ? ["aac"] : []),
      ...(webm ? ["opus", "vorbis"] : []),
    ],
    maximumVideoWidth: Math.floor(
      (globalThis.screen?.width || globalThis.innerWidth || 1920) *
      (globalThis.devicePixelRatio || 1),
    ),
    maximumVideoHeight: Math.floor(
      (globalThis.screen?.height || globalThis.innerHeight || 1080) *
      (globalThis.devicePixelRatio || 1),
    ),
    maximumAudioChannels: 2,
    hdrCapabilities: [],
    subtitleFormats: ["webvtt"],
    directPlaySupport: mp4 || webm,
    fragmentedMp4Support: mp4,
  };
}
async function raw<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);
  const response = await fetch(path, {
    ...init,
    headers,
    credentials: "same-origin",
  });
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as {
      error?: { code?: string; message?: string };
    } | null;
    const error = new Error(
      body?.error?.message || `Server returned ${response.status}`,
    ) as Error & { status: number; code?: string };
    error.status = response.status;
    error.code = body?.error?.code;
    throw error;
  }
  return response.status === 204
    ? (undefined as T)
    : (response.json() as Promise<T>);
}
function accept(v: { accessToken: string; user: User }) {
  accessToken = v.accessToken;
  return v.user;
}
export function refresh(): Promise<User> {
  if (!refreshFlight)
    refreshFlight = raw<{ accessToken: string; user: User }>(
      "/api/v1/auth/refresh",
      { method: "POST", body: "{}" },
    )
      .then(accept)
      .finally(() => {
        refreshFlight = null;
      });
  return refreshFlight;
}
async function call<T>(path: string, init: RequestInit = {}): Promise<T> {
  try {
    return await raw<T>(path, init);
  } catch (reason) {
    if (
      (reason as { status?: number }).status !== 401 ||
      path.includes("/auth/")
    )
      throw reason;
    await refresh();
    return raw<T>(path, init);
  }
}
async function blob(path: string): Promise<Blob> {
  const run = async () => {
    const headers = new Headers();
    if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);
    const response = await fetch(path, { headers, credentials: "same-origin" });
    if (!response.ok) {
      const e = new Error(`Artwork returned ${response.status}`) as Error & {
        status: number;
      };
      e.status = response.status;
      throw e;
    }
    return response.blob();
  };
  try {
    return await run();
  } catch (reason) {
    if ((reason as { status?: number }).status !== 401) throw reason;
    await refresh();
    return run();
  }
}
export const api = {
  status: () => raw<{ setupRequired: boolean }>("/api/v1/setup/status"),
  connectExchange: (assertion: string) =>
    raw<{ accessToken: string; user: User }>("/api/v1/connect/exchange", {
      method: "POST",
      body: JSON.stringify({
        assertion,
        device: {
          name: "Web browser",
          clientName: "VyNode Web",
          platform: navigator.platform || "web",
        },
      }),
    }).then(accept),
  redeemConnectInvitation: (redemption: string) =>
    raw<{ provisioned: boolean }>("/api/v1/connect/invitations/redeem", {
      method: "POST",
      body: JSON.stringify({ redemption }),
    }),
  setup: (body: unknown) =>
    raw<{ accessToken: string; user: User }>("/api/v1/setup/owner", {
      method: "POST",
      body: JSON.stringify(body),
    }).then(accept),
  login: (username: string, password: string) =>
    raw<{ accessToken: string; user: User }>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({
        username,
        password,
        device: {
          name: "Web browser",
          clientName: "VyNode Web",
          platform: navigator.platform || "web",
        },
      }),
    }).then(accept),
  refresh,
  info: () => raw<SystemInfo>("/api/v1/system/info"),
  me: () => call<User>("/api/v1/account/me"),
  sessions: () => call<{ sessions: Session[] }>("/api/v1/account/sessions"),
  revoke: (id: string) =>
    call<void>(`/api/v1/account/sessions/${id}`, { method: "DELETE" }),
  logoutOthers: () =>
    call<void>("/api/v1/auth/logout-others", { method: "POST" }),
  password: (currentPassword: string, newPassword: string) =>
    call<void>("/api/v1/account/password", {
      method: "POST",
      body: JSON.stringify({ currentPassword, newPassword }),
    }),
  users: () => call<{ users: User[] }>("/api/v1/admin/users"),
  invitations:()=>call<{invitations:Invitation[]}>("/api/v1/admin/invitations"),
  createInvitation:(body:unknown)=>call<{invitation:Invitation;invitePath:string;shownOnce:boolean}>("/api/v1/admin/invitations",{method:"POST",body:JSON.stringify(body)}),
  revokeInvitation:(id:string)=>call<void>(`/api/v1/admin/invitations/${id}`,{method:"DELETE"}),
  userGrants:(id:string)=>call<{grants:LibraryGrant[]}>(`/api/v1/admin/users/${id}/library-access`),
  setUserGrants:(id:string,grants:LibraryGrant[])=>call<void>(`/api/v1/admin/users/${id}/library-access`,{method:"PUT",body:JSON.stringify({grants})}),
  inspectInvitation:(token:string)=>raw<{serverName:string;invitation:Invitation}>(`/api/v1/invitations/inspect?token=${encodeURIComponent(token)}`),
  acceptInvitation:(body:unknown)=>raw<{accessToken:string;user:User}>("/api/v1/invitations/accept",{method:"POST",body:JSON.stringify(body)}).then(accept),
  createPairing:(body:unknown)=>raw<PairingRequest>("/api/v1/pairing/requests",{method:"POST",headers:{"Content-Type":"application/json","X-VyNode-Client":"native"},body:JSON.stringify(body)}),
  lookupPairing:(code:string)=>call<PairingRequest>("/api/v1/account/pairing/lookup",{method:"POST",body:JSON.stringify({code})}),
  approvePairing:(id:string)=>call<void>(`/api/v1/account/pairing/${id}/approve`,{method:"POST"}),
  denyPairing:(id:string)=>call<void>(`/api/v1/account/pairing/${id}/deny`,{method:"POST"}),
  remoteAccess:()=>call<RemoteSettings>("/api/v1/admin/remote-access"),
  saveRemoteAccess:(body:RemoteSettings)=>call<RemoteSettings>("/api/v1/admin/remote-access",{method:"PUT",body:JSON.stringify(body)}),
  createUser: (body: unknown) =>
    call<User>("/api/v1/admin/users", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  setEnabled: (id: string, enabled: boolean) =>
    call<void>(`/api/v1/admin/users/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ enabled }),
    }),
  audit: (offset = 0) =>
    call<{ events: AuditEvent[]; limit: number; offset: number }>(
      `/api/v1/admin/audit?limit=25&offset=${offset}`,
    ),
  libraries: () => call<{ libraries: Library[] | null }>("/api/v1/libraries")
    .then((result) => ({ ...result, libraries: result.libraries ?? [] })),
  library: (id: string) => call<Library>(`/api/v1/libraries/${id}`),
  createLibrary: (body: unknown) =>
    call<Library>("/api/v1/libraries", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteLibrary: (id: string) =>
    call<void>(`/api/v1/libraries/${id}`, { method: "DELETE" }),
  validateSource: (path: string, libraryId = "") =>
    call<{ valid: boolean; normalizedPath: string }>(
      "/api/v1/libraries/sources/validate",
      { method: "POST", body: JSON.stringify({ path, libraryId }) },
    ),
  addSource: (libraryId: string, path: string) =>
    call(`/api/v1/libraries/${libraryId}/sources`, {
      method: "POST",
      body: JSON.stringify({ path }),
    }),
  removeSource: (libraryId: string, sourceId: string) =>
    call<void>(`/api/v1/libraries/${libraryId}/sources/${sourceId}`, {
      method: "DELETE",
    }),
  scan: (id: string) =>
    call<ScanJob>(`/api/v1/libraries/${id}/scan`, { method: "POST" }),
  scanStatus: (libraryId: string, jobId: string) =>
    call<ScanJob>(`/api/v1/libraries/${libraryId}/scans/${jobId}`),
  cancelScan: (libraryId: string, jobId: string) =>
    call<void>(`/api/v1/libraries/${libraryId}/scans/${jobId}`, {
      method: "DELETE",
    }),
  items: (id: string, offset = 0) =>
    call<{ items: MediaFile[] | null }>(
      `/api/v1/libraries/${id}/items?limit=50&offset=${offset}`,
    ).then((result) => ({ ...result, items: result.items ?? [] })),
  mediaFile: (id: string) => call<MediaFile>(`/api/v1/media/files/${id}`),
  movies: () => call<{ movies: Movie[] | null }>("/api/v1/movies")
    .then((result) => ({ ...result, movies: result.movies ?? [] })),
  search: (query: string) =>
    call<{ movies: Movie[] | null; shows: Show[] | null }>(
      `/api/v1/search?q=${encodeURIComponent(query)}`,
    ).then((result) => ({ movies: result.movies ?? [], shows: result.shows ?? [] })),
  movie: (id: string) => call<Movie>(`/api/v1/movies/${id}`),
  shows: () => call<{ shows: Show[] }>("/api/v1/shows"),
  show: (id: string) => call<Show>(`/api/v1/shows/${id}`),
  artwork: (kind: "movies" | "shows", id: string) =>
    call<{ artwork: Artwork[] }>(`/api/v1/${kind}/${id}/artwork`),
  artworkBlob: (id: string) => blob(`/api/v1/artwork/${id}/content`),
  selectArtwork: (kind: "movies" | "shows", entityId: string, id: string) =>
    call<void>(`/api/v1/${kind}/${entityId}/artwork/${id}/select`, {
      method: "POST",
    }),
  provider: () => call<ProviderStatus>("/api/v1/admin/metadata/provider"),
  configureProvider: (body: unknown) =>
    call<ProviderStatus>("/api/v1/admin/metadata/provider", {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  testProvider: () =>
    call("/api/v1/admin/metadata/provider/test", { method: "POST" }),
  unmatched: () =>
    call<{ items: Unmatched[] }>("/api/v1/admin/metadata/unmatched"),
  providerSearch: (type: string, q: string, year = 0) =>
    call<{ candidates: Unmatched["candidates"] }>(
      `/api/v1/admin/metadata/provider/search?type=${type}&q=${encodeURIComponent(q)}&year=${year}`,
    ),
  match: (fileId: string, body: unknown) =>
    call(`/api/v1/admin/metadata/files/${fileId}/match`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  unmatch: (fileId: string) =>
    call<void>(`/api/v1/admin/metadata/files/${fileId}/unmatch`, {
      method: "POST",
    }),
  playbackVersions: (type: "MOVIE" | "EPISODE", id: string) =>
    call<{ versions: PlaybackVersion[] }>(
      `/api/v1/playback/${type}/${id}/versions`,
    ),
  startPlayback: (
    logicalType: "MOVIE" | "EPISODE",
    logicalId: string,
    requestedVersionId = "",
    resume = true,
    selectedAudioTrackId = "",
    selectedSubtitleTrackId = "",
    qualityId = "",
    startPosition = 0,
    playbackContextId = "",
  ) =>
    call<PlaybackSession>("/api/v1/playback/sessions", {
      method: "POST",
      body: JSON.stringify({
        logicalType,
        logicalId,
        requestedVersionId,
        resume,
        selectedAudioTrackId,
        selectedSubtitleTrackId,
        qualityId,
        startPosition,
        playbackContextId,
        capabilities: browserCapabilities(),
      }),
    }),
  continueWatching: () =>
    call<{ items: ContinueItem[] }>("/api/v1/playback/continue-watching"),
  playbackPreferences:()=>call<PlaybackPreferences>("/api/v1/account/playback-preferences"),
  setPlaybackPreferences:(body:PlaybackPreferences)=>call<PlaybackPreferences>("/api/v1/account/playback-preferences",{method:"PATCH",body:JSON.stringify(body)}),
  markers:(type:"MOVIE"|"EPISODE",id:string)=>call<{markers:MediaMarker[]}>(`/api/v1/playback/${type}/${id}/markers`),
  saveMarker:(body:Partial<MediaMarker>)=>call<MediaMarker>("/api/v1/admin/media-markers",{method:"POST",body:JSON.stringify(body)}),
  updateMarker:(id:string,body:Partial<MediaMarker>)=>call<MediaMarker>(`/api/v1/admin/media-markers/${id}`,{method:"PATCH",body:JSON.stringify(body)}),
  deleteMarker:(id:string)=>call<void>(`/api/v1/admin/media-markers/${id}`,{method:"DELETE"}),
  dismissContinue:(type:"MOVIE"|"EPISODE",id:string)=>call<void>(`/api/v1/playback/continue-watching/items/${type}/${id}`,{method:"DELETE"}),
  startOver:(type:"MOVIE"|"EPISODE",id:string)=>call<void>(`/api/v1/playback/${type}/${id}/start-over`,{method:"POST"}),
  intelligenceJobs:()=>call<{jobs:IntelligenceJob[]}>("/api/v1/admin/background-jobs"),
  analyzeMarkers:(targetType:string,targetId:string)=>call<IntelligenceJob>("/api/v1/admin/marker-analysis",{method:"POST",body:JSON.stringify({targetType,targetId})}),
  markerReview:()=>call<{candidates:MarkerCandidate[]}>("/api/v1/admin/marker-review"),
  reviewAutomaticMarker:(id:string,action:string,start?:number,end?:number)=>call<void>(`/api/v1/admin/marker-review/${id}`,{method:"POST",body:JSON.stringify({action,start,end})}),
  markerPolicy:()=>call<{automaticallyActivateHighConfidence:boolean}>("/api/v1/admin/marker-policy"),
  setMarkerPolicy:(on:boolean)=>call<void>("/api/v1/admin/marker-policy",{method:"PUT",body:JSON.stringify({automaticallyActivateHighConfidence:on})}),
  optimize:(body:{logicalType:string;logicalId:string;sourceMediaFileId:string;profile:string})=>call<IntelligenceJob>("/api/v1/admin/optimizations",{method:"POST",body:JSON.stringify(body)}),
  optimizedMedia:()=>call<{items:OptimizedMedia[]}>("/api/v1/admin/optimizations"),
  deleteOptimized:(id:string)=>call<void>(`/api/v1/admin/optimizations/${id}`,{method:"DELETE"}),
  automationRules:()=>call<{rules:AutomationRule[]}>("/api/v1/admin/automation-rules"),
  saveAutomationRule:(body:AutomationRule)=>call<AutomationRule>(body.ID?`/api/v1/admin/automation-rules/${body.ID}`:"/api/v1/admin/automation-rules",{method:body.ID?"PUT":"POST",body:JSON.stringify(body)}),
  deleteAutomationRule:(id:string)=>call<void>(`/api/v1/admin/automation-rules/${id}`,{method:"DELETE"}),
  dryRunAutomation:(body:AutomationRule)=>call<{matches:string[];actionsExecuted:number}>("/api/v1/admin/automation-rules/dry-run",{method:"POST",body:JSON.stringify(body)}),
  executeAutomation:(id:string)=>call<{matches:string[];actionsExecuted:number}>(`/api/v1/admin/automation-rules/${id}/execute`,{method:"POST"}),
  home:()=>call<{rows:HomeRow[]}>("/api/v1/home"),
  homeRows:()=>call<{rows:HomeRow[]}>("/api/v1/account/home-rows"),
  saveHomeRow:(x:Partial<HomeRow>)=>call<HomeRow>(x.ID?`/api/v1/account/home-rows/${x.ID}`:"/api/v1/account/home-rows",{method:x.ID?"PATCH":"POST",body:JSON.stringify(x)}),
  deleteHomeRow:(id:string)=>call<void>(`/api/v1/account/home-rows/${id}`,{method:"DELETE"}),
  reorderHome:(IDs:string[])=>call<void>("/api/v1/account/home-rows/order",{method:"PUT",body:JSON.stringify({IDs})}),
  collections:()=>call<{collections:Collection[]}>("/api/v1/collections"),
  collection:(id:string)=>call<Collection>(`/api/v1/collections/${id}`),
  saveCollection:(x:Partial<Collection>)=>call<Collection>(x.ID?`/api/v1/collections/${x.ID}`:"/api/v1/collections",{method:x.ID?"PATCH":"POST",body:JSON.stringify(x)}),
  deleteCollection:(id:string)=>call<void>(`/api/v1/collections/${id}`,{method:"DELETE"}),
  addCollectionItems:(id:string,Items:Array<{Type:string;ID:string}>)=>call<void>(`/api/v1/collections/${id}/items`,{method:"POST",body:JSON.stringify({Items})}),
  removeCollectionItem:(id:string,type:string,itemId:string)=>call<void>(`/api/v1/collections/${id}/items/${type}/${itemId}`,{method:"DELETE"}),
  reorderCollection:(id:string,IDs:string[])=>call<void>(`/api/v1/collections/${id}/order`,{method:"PUT",body:JSON.stringify({IDs})}),
  smartCollections:()=>call<{smartCollections:SmartCollection[]}>("/api/v1/smart-collections"),
  smartCollection:(id:string)=>call<SmartCollection>(`/api/v1/smart-collections/${id}`),
  saveSmart:(x:SmartCollection)=>call<SmartCollection>(x.ID?`/api/v1/smart-collections/${x.ID}`:"/api/v1/smart-collections",{method:x.ID?"PATCH":"POST",body:JSON.stringify(x)}),
  previewSmart:(x:SmartCollection)=>call<{count:number;items:CurationItem[]}>("/api/v1/smart-collections/preview",{method:"POST",body:JSON.stringify(x)}),
  playlists:()=>call<{playlists:Playlist[]}>("/api/v1/playlists"),
  playlist:(id:string)=>call<Playlist>(`/api/v1/playlists/${id}`),
  savePlaylist:(x:Partial<Playlist>)=>call<Playlist>(x.ID?`/api/v1/playlists/${x.ID}`:"/api/v1/playlists",{method:x.ID?"PATCH":"POST",body:JSON.stringify(x)}),
  deletePlaylist:(id:string)=>call<void>(`/api/v1/playlists/${id}`,{method:"DELETE"}),
  addPlaylistItem:(id:string,Type:string,ID:string)=>call<CurationItem>(`/api/v1/playlists/${id}/items`,{method:"POST",body:JSON.stringify({Type,ID})}),
  removePlaylistItem:(id:string,entryId:string)=>call<void>(`/api/v1/playlists/${id}/items/${entryId}`,{method:"DELETE"}),
  reorderPlaylist:(id:string,IDs:string[])=>call<void>(`/api/v1/playlists/${id}/order`,{method:"PUT",body:JSON.stringify({IDs})}),
  personal:(kind:"watchlist"|"favorites")=>call<{items:CurationItem[]}>(`/api/v1/${kind}`),
  togglePersonal:(kind:"watchlist"|"favorites",type:string,id:string,on:boolean)=>call<void>(`/api/v1/${kind}/${type}/${id}`,{method:on?"PUT":"DELETE"}),
  updatePlayback: (
    id: string,
    state: string,
    position: number,
    duration: number,
  ) =>
    call<void>(`/api/v1/playback/sessions/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ state, position, duration }),
    }),
  stopPlayback: (id: string) =>
    call<void>(`/api/v1/playback/sessions/${id}`, { method: "DELETE" }),
  progress: (type: "MOVIE" | "EPISODE", id: string) =>
    call<{ position: number; duration: number; watched: boolean }>(
      `/api/v1/playback/${type}/${id}/progress`,
    ),
  markWatched: (type: "MOVIE" | "EPISODE", id: string, watched: boolean) =>
    call<void>(`/api/v1/playback/${type}/${id}/watched`, {
      method: "PUT",
      body: JSON.stringify({ watched }),
    }),
  activePlayback: () =>
    call<{ sessions: PlaybackSession[] }>("/api/v1/admin/playback/sessions"),
  adminDashboard:()=>call<AdminDashboard>("/api/v1/admin/dashboard"),
  playbackAnalytics:(days=7)=>{const to=new Date(),from=new Date(to.getTime()-days*86400000);return call<PlaybackAnalytics>(`/api/v1/admin/analytics/playback?from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`)},
  personalHistory:(days=30)=>{const to=new Date(),from=new Date(to.getTime()-days*86400000);return call<PlaybackAnalytics>(`/api/v1/account/playback-history/summary?from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`)},
  adminJobs:()=>call<{jobs:AdminJob[]}>("/api/v1/admin/jobs?limit=100"),
  healthIssues:(status="")=>call<{issues:HealthIssue[]}>(`/api/v1/admin/health/issues${status?`?status=${status}`:""}`),
  reevaluateHealth:()=>call<{issues:HealthIssue[]}>("/api/v1/admin/health/reevaluate",{method:"POST"}),
  setHealthIgnored:(id:string,on:boolean)=>call<void>(`/api/v1/admin/health/issues/${id}/ignored`,{method:on?"PUT":"DELETE"}),
  operationalEvents:()=>call<{events:OpsEvent[]}>("/api/v1/admin/operational-events?limit=50"),
  webhookCatalog:()=>call<{events:Record<string,{Type:string;Category:string;Severity:string}>}>("/api/v1/admin/notifications/catalog"),
  webhookDestinations:()=>call<{destinations:WebhookDestination[]}>("/api/v1/admin/notifications/destinations"),
  saveWebhook:(x:WebhookDestination)=>call<WebhookDestination>(x.ID?`/api/v1/admin/notifications/destinations/${x.ID}`:"/api/v1/admin/notifications/destinations",{method:x.ID?"PATCH":"POST",body:JSON.stringify(x)}),
  deleteWebhook:(id:string)=>call<void>(`/api/v1/admin/notifications/destinations/${id}`,{method:"DELETE"}),
  testWebhook:(id:string)=>call<void>(`/api/v1/admin/notifications/destinations/${id}/test`,{method:"POST"}),
  webhookDeliveries:()=>call<{deliveries:WebhookDelivery[]}>("/api/v1/admin/notifications/deliveries?limit=50"),
  adminStopPlayback: (id: string) =>
    call<void>(`/api/v1/admin/playback/sessions/${id}`, { method: "DELETE" }),
  offlineDownloads:()=>call<{downloads:OfflineDownload[]}>("/api/v1/downloads"),
  offlineSettings:()=>call<OfflineSettings>("/api/v1/admin/download-settings"),
  saveOfflineSettings:(cacheQuotaBytes:number)=>call<OfflineSettings>("/api/v1/admin/download-settings",{method:"PUT",body:JSON.stringify({cacheQuotaBytes})}),
  createOfflineDownload:(logicalType:"MOVIE"|"EPISODE",logicalId:string,profileId:string)=>call<OfflineDownload>("/api/v1/downloads",{method:"POST",body:JSON.stringify({logicalType,logicalId,profileId})}),
  removeOfflineDownload:(id:string)=>call<void>(`/api/v1/downloads/${id}`,{method:"DELETE"}),
  logout: () =>
    call<void>("/api/v1/auth/logout", { method: "POST" }).finally(() => {
      accessToken = "";
    }),
};
