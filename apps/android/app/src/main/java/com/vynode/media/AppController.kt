package com.vynode.media

import android.content.Context
import com.vynode.media.auth.SecureTokenStore
import com.vynode.media.auth.SessionCoordinator
import com.vynode.media.auth.RotatedSession
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
import com.vynode.media.connect.ConnectApi
import com.vynode.media.connect.ConnectedServer
import com.vynode.media.connect.ConnectException
import com.vynode.media.data.GlobalAccountEntity

sealed interface AppScreen {
    data object GlobalSignIn : AppScreen
    data class ServerPicker(val servers:List<ConnectedServer>,val message:String?=null,val currentServerId:String?=null):AppScreen
    data class ServerInfo(val serverName:String):AppScreen
	data class GlobalDeviceCode(val userCode:String,val verificationPath:String):AppScreen
    data object Connect : AppScreen
    data class ConfirmInsecure(val endpoint: String) : AppScreen
    data class Pair(val server: ServerIdentity, val request: PairingRequest) : AppScreen
    data class Home(val server: ServerIdentity, val rows: List<ApiHomeRow>, val focusId: String? = null) : AppScreen
    data class Movie(val server: ServerIdentity, val movie: ApiMovie, val progress: ApiProgress?) : AppScreen
    data class Playing(val server: ServerIdentity, val item: ApiHomeItem, val session: PlaybackSession) : AppScreen
    data class Show(val server: ServerIdentity, val show: ApiShow, val message: String? = null, val focusId:String?=null) : AppScreen
    data class Search(val query: String = "", val results: ApiSearch? = null) : AppScreen
    data class Library(val kind: String, val results: ApiSearch) : AppScreen
    data class Offline(val downloads: List<DownloadEntity>) : AppScreen
    data class Account(val accountName: String, val serverName: String) : AppScreen
    data class LocalPlaying(val download: DownloadEntity) : AppScreen
    data class Error(val message: String) : AppScreen
    data class IdentityMismatch(val expected: String, val received: ServerIdentity) : AppScreen
}

class AppController(context: Context, private val tv: Boolean) {
    private val appContext = context.applicationContext
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private val database = ClientDatabase.open(context)
    private val tokens = SecureTokenStore(context)
    private val mutable = MutableStateFlow<AppScreen>(AppScreen.GlobalSignIn)
    val screen: StateFlow<AppScreen> = mutable
    private var api: VyNodeApi? = null
    private var coordinator: SessionCoordinator? = null
    private var currentServer: ServerIdentity? = null
    private var homeFocusId: String? = null
    private var detailItem: ApiHomeItem? = null
    private var beforePlayer: AppScreen? = null
    private var detailOrigin: AppScreen? = null
	private val connectApi=ConnectApi(BuildConfig.CONNECT_BASE_URL)
	private val globalSession=SessionCoordinator(GLOBAL_TOKEN_KEY,tokens){refresh->connectApi.refresh(refresh).let{RotatedSession(it.accessToken,it.refreshToken)}}

    init { scope.launch { restore() } }

    fun connect(rawEndpoint: String, allowInsecure: Boolean = false) {
        val endpoint = rawEndpoint.trim().trimEnd('/')
        if (endpoint.startsWith("http://") && !allowInsecure) { mutable.value = AppScreen.ConfirmInsecure(endpoint); return }
        scope.launch { runCatching { beginPairing(endpoint) }.onFailure { mutable.value = AppScreen.Error(it.message ?: "Server unavailable") } }
    }

    fun retry() { mutable.value = AppScreen.GlobalSignIn }
	fun advancedConnect(){mutable.value=AppScreen.Connect}
	fun globalLogin(username:String,password:String){scope.launch{runCatching{connectApi.login(username,password,android.os.Build.MODEL)}.onSuccess{session->acceptGlobalSession(session);loadGlobalServers(session.account.id)}.onFailure{mutable.value=AppScreen.Error(it.message?:"VyNode sign-in failed")}}}
	fun globalRegister(username:String,displayName:String,password:String){scope.launch{runCatching{connectApi.register(username,displayName,password,android.os.Build.MODEL)}.onSuccess{session->acceptGlobalSession(session);loadGlobalServers(session.account.id)}.onFailure{mutable.value=AppScreen.Error(it.message?:"VyNode account creation failed")}}}
	fun beginTvGlobalSignIn(){scope.launch{runCatching{connectApi.deviceCode(android.os.Build.MODEL)}.onSuccess{code->mutable.value=AppScreen.GlobalDeviceCode(code.userCode,code.verificationPath);while(currentCoroutineContext().isActive){delay(code.pollAfterSeconds*1000L);try{val session=connectApi.exchangeDeviceCode(code.deviceCode);acceptGlobalSession(session);loadGlobalServers(session.account.id);return@launch}catch(e:ConnectException){if(e.status==410){mutable.value=AppScreen.Error(when(e.code){"device_denied"->"Device authorization was denied";"device_expired"->"Device authorization expired";else->"Device authorization is no longer available"});return@launch};if(e.status!=409)throw e}}}.onFailure{mutable.value=AppScreen.Error(it.message?:"Device authorization failed")}}}
	fun selectGlobalServer(server:ConnectedServer){scope.launch{runCatching{connectToGlobalServer(server,database.dao().activeGlobalAccount()?.accountId)}.onFailure{mutable.value=AppScreen.Error(it.message?:"Server unavailable")}}}
	fun chooseGlobalServer(){scope.launch{val account=database.dao().activeGlobalAccount();if(account==null){currentServer?.let{mutable.value=AppScreen.ServerInfo(it.serverName)}?:run{mutable.value=AppScreen.Error("No server is connected")};return@launch};runCatching{val values=withGlobalAccess{connectApi.servers(it)};mutable.value=AppScreen.ServerPicker(values,if(values.isEmpty())"No VyNode servers are linked to your account yet." else null,currentServer?.instanceId)}.onFailure{mutable.value=AppScreen.Error(it.message?:"VyNode Connect is unavailable")}}}
	fun globalLogout(){scope.launch{val account=database.dao().activeGlobalAccount();account?.let{database.dao().serversForGlobalAccount(it.accountId).forEach{server->clearGlobalServerState(server.serverId)}};globalSession.revoke();coordinator?.revoke();coordinator=null;api=null;currentServer=null;database.dao().deactivateGlobalAccounts();mutable.value=AppScreen.GlobalSignIn}}
	private suspend fun clearGlobalServerState(serverId:String){
		database.dao().downloadsForServer(serverId).forEach{download->
			WorkManager.getInstance(appContext).cancelUniqueWork("download-${download.downloadId}")
			listOfNotNull(download.localFile,download.localArtwork).forEach{path->runCatching{java.io.File(path).delete()}}
		}
		appContext.filesDir.resolve("offline/$serverId").deleteRecursively()
		database.dao().deleteDownloads(serverId)
		database.dao().deleteProgress(serverId)
		database.dao().deleteSyncState(serverId)
		tokens.clear(serverId)
	}
	private suspend fun loadGlobalServers(accountId:String){val values=withGlobalAccess{connectApi.servers(it)};if(values.isEmpty()){mutable.value=AppScreen.ServerPicker(emptyList(),"No VyNode servers are linked to your account yet.")}else if(values.size==1){connectToGlobalServer(values.first(),accountId)}else mutable.value=AppScreen.ServerPicker(values)}
	private suspend fun connectToGlobalServer(server:ConnectedServer,accountId:String?){val endpoint=server.endpoints.firstOrNull()?:throw IllegalStateException("No reachable endpoint is registered");val next=VyNodeApi(endpoint);val identity=next.identity();if(identity.instanceId!=server.id)throw IllegalStateException("Server identity did not match the linked server");val assertion=withGlobalAccess{connectApi.assertion(it,server.id)};val session=next.exchangeConnect(assertion);tokens.replace(server.id,session.refreshToken);val c=SessionCoordinator(server.id,tokens,next);c.establish(session);coordinator=c;api=next;next.accessToken=session.accessToken;database.dao().saveServer(ServerEntity(server.id,server.name,endpoint,endpoint.startsWith("http://"),System.currentTimeMillis(),accountId));loadHome(identity)}
	private suspend fun acceptGlobalSession(session:com.vynode.media.connect.GlobalTokens){globalSession.establish(RotatedSession(session.accessToken,session.refreshToken));database.dao().deactivateGlobalAccounts();database.dao().saveGlobalAccount(GlobalAccountEntity(session.account.id,session.account.username,session.account.displayName,true,System.currentTimeMillis()))}
	private suspend fun <T> withGlobalAccess(call:suspend(String)->T):T{val first=globalSession.currentAccessToken()?:globalSession.refresh(null);return try{call(first)}catch(e:ConnectException){if(e.status!=401)throw e;call(globalSession.refresh(first))}}

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
		val saved = database.dao().lastServer() ?: run{restoreGlobal();return}
		if(saved.globalAccountId!=null && database.dao().activeGlobalAccount()?.accountId!=saved.globalAccountId){restoreGlobal();return}
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
	private suspend fun restoreGlobal(){val account=database.dao().activeGlobalAccount()?:return;if(tokens.read(GLOBAL_TOKEN_KEY)==null)return;runCatching{globalSession.refresh(null);loadGlobalServers(account.accountId)}}

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
        detailOrigin = mutable.value.takeIf { it is AppScreen.Search || it is AppScreen.Library }
        if (item.type.equals("SHOW", true)) {
            scope.launch { runCatching { api!!.show(item.id) }.onSuccess { mutable.value = AppScreen.Show(currentServer!!, it) }
                .onFailure { mutable.value = AppScreen.Error(it.message ?: "Show could not be loaded") } }
        } else if(item.type.equals("MOVIE",true)) {
            scope.launch { runCatching { api!!.movie(item.id) to runCatching { api!!.progress("MOVIE", item.id) }.getOrNull() }.onSuccess { (movie, progress) -> detailItem=item; mutable.value=AppScreen.Movie(currentServer!!,movie,progress) }
                .onFailure { mutable.value=AppScreen.Error(it.message ?: "Movie could not be loaded") } }
        } else play(item)
    }

    fun openSearch() { mutable.value=AppScreen.Search() }
    fun openLibrary(kind:String) { scope.launch { runCatching { api!!.search("") }.onSuccess { mutable.value=AppScreen.Library(kind.uppercase(),it) }.onFailure { mutable.value=AppScreen.Error(it.message ?: "Library could not be loaded") } } }
    fun openDownloads() { scope.launch { val server=currentServer ?: return@launch; mutable.value=AppScreen.Offline(database.dao().readyDownloads(server.instanceId)) } }
    fun openAccount() { scope.launch { val account=database.dao().activeGlobalAccount(); mutable.value=AppScreen.Account(account?.displayName ?: account?.username ?: "Local account",currentServer?.serverName ?: "No server") } }
    fun startOver(item: ApiHomeItem) {
        scope.launch {
            runCatching { api!!.startOver(item.type, item.id) }
                .onSuccess { play(item, resume = false) }
                .onFailure { mutable.value = AppScreen.Error(it.message ?: "Playback could not restart") }
        }
    }
    fun playFromShow(item:ApiHomeItem){val current=mutable.value as? AppScreen.Show;if(current!=null)mutable.value=current.copy(focusId=item.id);play(item)}
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
	private companion object{const val GLOBAL_TOKEN_KEY="__vynode_global__"}
}
