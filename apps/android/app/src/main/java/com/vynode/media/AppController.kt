package com.vynode.media

import android.content.Context
import com.vynode.media.auth.SecureTokenStore
import com.vynode.media.auth.SessionCoordinator
import com.vynode.media.data.ClientDatabase
import com.vynode.media.data.ServerEntity
import com.vynode.media.data.DownloadEntity
import com.vynode.media.offline.DownloadWorker
import androidx.work.*
import com.vynode.media.data.ProgressEntity
import com.vynode.media.data.SyncStateEntity
import org.json.JSONArray
import org.json.JSONObject
import java.time.Instant
import java.util.UUID
import com.vynode.media.network.*
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import com.vynode.media.playback.DeviceCapabilities

sealed interface AppScreen {
    data object Connect : AppScreen
    data class ConfirmInsecure(val endpoint: String) : AppScreen
    data class Pair(val server: ServerIdentity, val request: PairingRequest) : AppScreen
    data class Home(val server: ServerIdentity, val rows: List<ApiHomeRow>, val focusId: String? = null) : AppScreen
    data class Movie(val server: ServerIdentity, val movie: ApiMovie, val progress: ApiProgress?) : AppScreen
    data class Playing(val server: ServerIdentity, val item: ApiHomeItem, val session: PlaybackSession) : AppScreen
    data class Show(val server: ServerIdentity, val show: ApiShow, val message: String? = null) : AppScreen
    data class Search(val query: String = "", val results: ApiSearch? = null) : AppScreen
    data class Offline(val downloads: List<DownloadEntity>) : AppScreen
    data class LocalPlaying(val download: DownloadEntity) : AppScreen
    data class Error(val message: String) : AppScreen
    data class IdentityMismatch(val expected: String, val received: ServerIdentity) : AppScreen
}

class AppController(context: Context, private val tv: Boolean) {
    private val appContext = context.applicationContext
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private val database = ClientDatabase.open(context)
    private val tokens = SecureTokenStore(context)
    private val mutable = MutableStateFlow<AppScreen>(AppScreen.Connect)
    val screen: StateFlow<AppScreen> = mutable
    private var api: VyNodeApi? = null
    private var coordinator: SessionCoordinator? = null
    private var currentServer: ServerIdentity? = null
    private var homeFocusId: String? = null
    private var detailItem: ApiHomeItem? = null
    private var beforePlayer: AppScreen? = null
    private var detailOrigin: AppScreen? = null

    init { scope.launch { restore() } }

    fun connect(rawEndpoint: String, allowInsecure: Boolean = false) {
        val endpoint = rawEndpoint.trim().trimEnd('/')
        if (endpoint.startsWith("http://") && !allowInsecure) { mutable.value = AppScreen.ConfirmInsecure(endpoint); return }
        scope.launch { runCatching { beginPairing(endpoint) }.onFailure { mutable.value = AppScreen.Error(it.message ?: "Server unavailable") } }
    }

    fun retry() { mutable.value = AppScreen.Connect }

    private suspend fun beginPairing(endpoint: String) {
        val next = VyNodeApi(endpoint)
        val identity = next.identity()
        api = next
        val request = next.createPairing(android.os.Build.MODEL, if (tv) "Android TV" else "Android")
        mutable.value = AppScreen.Pair(identity, request)
        poll(identity, request)
    }

    private suspend fun poll(identity: ServerIdentity, request: PairingRequest) {
        while (currentCoroutineContext().isActive) {
            delay(3_000)
            val next = api?.pairingState(request.id) ?: return
            when (next.status) {
                "APPROVED" -> {
                    val session = api!!.exchange(request)
                    val c = SessionCoordinator(identity.instanceId, tokens, api!!)
                    c.establish(session); coordinator = c; api!!.accessToken = session.accessToken
                    database.dao().saveServer(ServerEntity(identity.instanceId, identity.serverName, api!!.baseUrl, api!!.baseUrl.startsWith("http://"), System.currentTimeMillis()))
                    loadHome(identity); return
                }
                "DENIED", "EXPIRED" -> { mutable.value = AppScreen.Error("Pairing ${next.status.lowercase()}"); return }
            }
        }
    }

    private suspend fun restore() {
        val saved = database.dao().lastServer() ?: return
        val next = VyNodeApi(saved.endpoint)
        val identity = runCatching { next.identity() }.getOrElse {
            val local = database.dao().readyDownloads(saved.serverId)
            if (local.isNotEmpty()) mutable.value = AppScreen.Offline(local)
            return
        }
        when (val trust = ServerTrust.evaluate(saved.serverId, identity)) {
            is TrustDecision.IdentityChanged -> { mutable.value = AppScreen.IdentityMismatch(trust.expected, trust.received); return }
            else -> Unit
        }
        val c = SessionCoordinator(saved.serverId, tokens, next)
        val access = runCatching { withContext(Dispatchers.IO) { c.refresh(null) } }.getOrElse { return }
        coordinator = c; api = next; next.accessToken = access
        syncPending(saved.serverId)
        loadHome(identity)
    }

    private suspend fun loadHome(identity: ServerIdentity) {
        currentServer = identity
        val rows = api!!.home()
        mutable.value = AppScreen.Home(identity, rows, homeFocusId)
    }

    fun play(item: ApiHomeItem, qualityId: String? = null, audioId: String? = null, subtitleId: String? = null, startSeconds: Double = 0.0, resume: Boolean = true) {
        if(mutable.value !is AppScreen.Playing) beforePlayer=mutable.value
        scope.launch {
            runCatching { api!!.startPlayback(item.type, item.id, DeviceCapabilities.detect(), qualityId, audioId, subtitleId, startSeconds, resume) }
                .onSuccess { detailItem=item; mutable.value = AppScreen.Playing(currentServer!!, item, it) }
                .onFailure { mutable.value = AppScreen.Error(it.message ?: "Playback could not start") }
        }
    }

    fun open(item: ApiHomeItem) {
        homeFocusId=item.id
        detailOrigin = (mutable.value as? AppScreen.Search)
        if (item.type.equals("SHOW", true)) {
            scope.launch { runCatching { api!!.show(item.id) }.onSuccess { mutable.value = AppScreen.Show(currentServer!!, it) }
                .onFailure { mutable.value = AppScreen.Error(it.message ?: "Show could not be loaded") } }
        } else if(item.type.equals("MOVIE",true)) {
            scope.launch { runCatching { api!!.movie(item.id) to runCatching { api!!.progress("MOVIE", item.id) }.getOrNull() }.onSuccess { (movie, progress) -> detailItem=item; mutable.value=AppScreen.Movie(currentServer!!,movie,progress) }
                .onFailure { mutable.value=AppScreen.Error(it.message ?: "Movie could not be loaded") } }
        } else play(item)
    }

    fun openSearch() { mutable.value=AppScreen.Search() }
    fun startOver(item: ApiHomeItem) {
        scope.launch {
            runCatching { api!!.startOver(item.type, item.id) }
                .onSuccess { play(item, resume = false) }
                .onFailure { mutable.value = AppScreen.Error(it.message ?: "Playback could not restart") }
        }
    }
    fun search(query:String) { scope.launch { runCatching { api!!.search(query) }.onSuccess { mutable.value=AppScreen.Search(query,it) }.onFailure { mutable.value=AppScreen.Error(it.message ?: "Search failed") } } }

    fun download(episode: ApiEpisode) {
        scope.launch {
            runCatching { api!!.createDownload("EPISODE", episode.id) }.onSuccess { download ->
                val request = OneTimeWorkRequestBuilder<DownloadWorker>().setInputData(workDataOf(DownloadWorker.SERVER to currentServer!!.instanceId, DownloadWorker.DOWNLOAD to download.id)).build()
                WorkManager.getInstance(appContext).enqueueUniqueWork("download-${download.id}", ExistingWorkPolicy.KEEP, request)
                val screen = mutable.value as? AppScreen.Show
                if (screen != null) mutable.value = screen.copy(message = "${episode.title} queued for offline download")
            }.onFailure { val screen=mutable.value as? AppScreen.Show; if(screen!=null) mutable.value=screen.copy(message=it.message ?: "Download failed") }
        }
    }

    fun playOffline(download: DownloadEntity) { mutable.value = AppScreen.LocalPlaying(download) }
    fun returnOffline() { scope.launch { database.dao().lastServer()?.let { mutable.value = AppScreen.Offline(database.dao().readyDownloads(it.serverId)) } } }
    fun returnHome() { scope.launch { currentServer?.let { loadHome(it) } } }
    fun returnFromDetail() { detailOrigin?.let { origin -> detailOrigin=null; mutable.value=origin } ?: returnHome() }
    fun returnDetail() { beforePlayer?.let { mutable.value=it } ?: returnHome() }

    fun changePlayback(session: PlaybackSession, item: ApiHomeItem, positionMs:Long, durationMs:Long, startMs:Long=positionMs, qualityId:String?=null, audioId:String?=null, subtitleId:String?=null) {
        scope.launch {
            runCatching { api?.updatePlayback(session.id,"STOPPED",positionMs/1000.0,durationMs.coerceAtLeast(0)/1000.0) }
            runCatching { api?.stopPlayback(session.id) }
            play(item,qualityId,audioId,subtitleId,startMs/1000.0)
        }
    }

    fun reportOffline(download: DownloadEntity, positionMs: Long, durationMs: Long, completed: Boolean = false) {
        scope.launch(Dispatchers.IO) {
            database.dao().enqueue(ProgressEntity(serverId=download.serverId, eventId=UUID.randomUUID().toString(), sequence=System.currentTimeMillis(),
                logicalType="EPISODE", logicalId=download.logicalId, positionSeconds=positionMs/1000.0,
                durationSeconds=durationMs.coerceAtLeast(0)/1000.0, occurredAt=Instant.now().toString(), action=if(completed) "WATCHED" else null))
        }
    }

    private suspend fun syncPending(serverId: String) {
        val pending=database.dao().pendingProgress(serverId)
        val epoch=appContext.getSharedPreferences("vynode_installation",Context.MODE_PRIVATE).let { prefs ->
            prefs.getString("epoch",null) ?: UUID.randomUUID().toString().also { check(prefs.edit().putString("epoch",it).commit()) }
        }
        val events=JSONArray(pending.map { event -> JSONObject().put("eventId",event.eventId).put("sequenceEpoch",epoch)
            .put("logicalType",event.logicalType).put("logicalId",event.logicalId).put("occurredAt",event.occurredAt)
            .put("deviceSequence",event.sequence).put("positionSeconds",event.positionSeconds).put("durationSeconds",event.durationSeconds)
            .put("watched",event.action=="WATCHED").apply { event.action?.let { put("explicitAction",it) } } })
        if(pending.isNotEmpty()) runCatching { api!!.pushProgress(events) }.onSuccess { database.dao().removeProgress(serverId,pending.map { it.eventId }) }
        // Presentation-only changes advance the sync cursor but deliberately do
        // not touch the device-bound media file or enqueue another transfer.
        val state=database.dao().syncState(serverId) ?: SyncStateEntity(serverId,0,epoch,System.currentTimeMillis())
        runCatching { api!!.pullChanges(state.cursor) }.onSuccess { delta ->
            delta.changes.filter { it.type == "METADATA_UPDATED" }.forEach { change ->
                database.dao().downloadForMedia(serverId, change.entityId)?.let { local ->
                    runCatching { api!!.manifest(local.downloadId) }.onSuccess { manifest ->
                        val artwork = manifest.artworkUrl?.let { url ->
                            appContext.filesDir.resolve("offline/$serverId/${local.downloadId}-artwork").let { target ->
                                runCatching { api!!.transferArtwork(url, target) }.getOrNull()?.let { target.absolutePath }
                            }
                        } ?: local.localArtwork
                        database.dao().saveDownload(local.copy(title=manifest.title, artworkUrl=manifest.artworkUrl, localArtwork=artwork))
                    }
                }
            }
            database.dao().saveSyncState(state.copy(cursor=delta.cursor))
        }
    }

    fun reportPlayback(sessionId: String, state: String, positionMs: Long, durationMs: Long) {
        scope.launch(Dispatchers.IO) { runCatching { api?.updatePlayback(sessionId, state, positionMs / 1000.0, durationMs.coerceAtLeast(0) / 1000.0) } }
    }

    fun stopPlayback(sessionId: String, positionMs: Long = 0, durationMs: Long = 0) {
        scope.launch {
            runCatching { api?.updatePlayback(sessionId,"STOPPED",positionMs/1000.0,durationMs.coerceAtLeast(0)/1000.0) }
            runCatching { api?.stopPlayback(sessionId) }
            returnDetail()
        }
    }

    fun mediaUrl(path: String) = if (path.startsWith("http")) path else api!!.baseUrl + path
    fun accessToken() = api?.accessToken

    fun close() { scope.cancel() }
}
