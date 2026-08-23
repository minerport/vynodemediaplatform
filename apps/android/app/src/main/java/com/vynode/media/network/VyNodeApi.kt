package com.vynode.media.network

import com.vynode.media.auth.RefreshTransport
import com.vynode.media.auth.RotatedSession
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.io.RandomAccessFile
import java.security.MessageDigest

data class PairingRequest(val id: String, val code: String, val challenge: String, val expiresAt: String)
data class PairingState(val status: String, val pollAfterSeconds: Long)
data class ApiHomeItem(val type: String, val id: String, val title: String, val subtitle: String?, val artworkId: String?)
data class ApiHomeRow(val id: String, val type: String, val title: String, val items: List<ApiHomeItem>)
data class ApiTrack(val id: String, val kind: String, val codec: String, val language: String?, val title: String?, val default: Boolean, val forced: Boolean, val commentary: Boolean) {
    val semanticLabel: String get() = listOfNotNull(language?.uppercase(), title, "Commentary".takeIf { commentary }, "Forced".takeIf { forced }).joinToString(" • ").ifBlank { codec.uppercase() }
}
data class ApiMarker(val type: String, val start: Double, val end: Double)
data class ApiQuality(val id: String, val label: String)
data class ApiNavigationItem(val id: String, val title: String)
data class ApiProgress(val position: Double, val duration: Double, val watched: Boolean)
data class PlaybackSession(val id: String, val mode: String, val mediaUrl: String?, val hlsUrl: String?, val subtitleUrl: String?, val resumePosition: Double,
    val audioTracks: List<ApiTrack>, val subtitleTracks: List<ApiTrack>, val markers: List<ApiMarker>, val qualities: List<ApiQuality>, val next: ApiNavigationItem?)
data class ApiEpisode(val id: String, val title: String, val season: Int, val number: Int, val available: Boolean, val progress: ApiProgress? = null)
data class ApiShow(val id: String, val title: String, val episodes: List<ApiEpisode>)
data class ApiMovie(val id: String, val title: String, val overview: String, val year: Int, val runtimeMinutes: Int, val artworkId: String?)
data class ApiSearch(val movies: List<ApiMovie>, val shows: List<ApiShow>)
data class ApiDownload(val id: String, val logicalId: String, val assetId: String, val status: String, val size: Long, val checksum: String)
data class ApiManifest(val title: String, val fileUrl: String, val download: ApiDownload, val artworkUrl: String?)
data class ApiChange(val type: String, val entityId: String)
data class ApiChanges(val cursor: Long, val changes: List<ApiChange>)

class ApiFailure(val status: Int, message: String) : Exception(message)

class VyNodeApi(endpoint: String, private val client: OkHttpClient = OkHttpClient()) : RefreshTransport {
    val baseUrl = endpoint.trim().trimEnd('/')
    @Volatile var accessToken: String? = null
    @Volatile var refreshTokenProvider: (() -> String?)? = null

    suspend fun identity(): ServerIdentity = getJson("/api/v1/connection-info").let {
        ServerIdentity(it.getString("serverId"), it.getString("serverName"), it.getString("apiVersion"))
    }

    suspend fun createPairing(deviceName: String, platform: String): PairingRequest = postJson("/api/v1/pairing/requests", JSONObject()
        .put("name", deviceName).put("clientName", "VyNode Android").put("clientVersion", "0.1.0")
        .put("platform", platform).put("platformVersion", android.os.Build.VERSION.RELEASE)).let {
        PairingRequest(it.getString("id"), it.getString("code"), it.getString("challenge"), it.getString("expiresAt"))
    }

    suspend fun pairingState(id: String): PairingState = getJson("/api/v1/pairing/requests/$id").let {
        PairingState(it.getString("status"), it.optLong("pollAfterSeconds", 3))
    }

    suspend fun exchange(pairing: PairingRequest): RotatedSession = postJson("/api/v1/pairing/requests/${pairing.id}/exchange", JSONObject().put("challenge", pairing.challenge), native = true).session()
	suspend fun exchangeConnect(assertion:String):RotatedSession=postJson("/api/v1/connect/exchange",JSONObject().put("assertion",assertion).put("device",JSONObject().put("name",android.os.Build.MODEL).put("clientName","VyNode Android").put("platform","ANDROID").put("platformVersion",android.os.Build.VERSION.RELEASE)),native=true).session()

    override suspend fun rotate(refreshToken: String): RotatedSession = postJson("/api/v1/auth/refresh", JSONObject().put("refreshToken", refreshToken), native = true).session()

    suspend fun home(): List<ApiHomeRow> = getJson("/api/v1/home", authenticated = true).optJSONArray("rows").orEmpty().mapObjects { row ->
        ApiHomeRow(row.optString("ID", row.optString("id")), row.optString("Type", row.optString("type")), row.optString("Title", row.optString("title")),
            row.optJSONArray("Items")?.mapObjects(::homeItem) ?: row.optJSONArray("items").orEmpty().mapObjects(::homeItem))
    }

    suspend fun startPlayback(type: String, id: String, capabilities: JSONObject, qualityId: String? = null, audioId: String? = null, subtitleId: String? = null, start: Double = 0.0, resume: Boolean = true): PlaybackSession =
        postJson("/api/v1/playback/sessions", JSONObject()
            .put("logicalType", type.uppercase()).put("logicalId", id).put("resume", resume)
            .put("capabilities", capabilities).apply { qualityId?.let { put("qualityId", it) }; audioId?.let { put("selectedAudioTrackId", it) }; subtitleId?.let { put("selectedSubtitleTrackId", it) }; if(start > 0) put("startPosition", start) }, authenticated = true).let {
            val version=it.optJSONObject("selectedVersion") ?: JSONObject()
            PlaybackSession(it.getString("id"), it.getJSONObject("decision").getString("mode"),
                it.optString("mediaUrl").takeIf(String::isNotBlank), it.optString("hlsUrl").takeIf(String::isNotBlank),
                it.optString("subtitleUrl").takeIf(String::isNotBlank), it.optDouble("resumePosition", 0.0),
                version.optJSONArray("audioTracks").orEmpty().mapObjects(::track), version.optJSONArray("subtitleTracks").orEmpty().mapObjects(::track),
                it.optJSONArray("markers").orEmpty().mapObjects { x -> ApiMarker(x.getString("type"),x.getDouble("start"),x.getDouble("end")) },
                it.optJSONArray("availableQualities").orEmpty().mapObjects { x -> ApiQuality(x.getString("id"),x.getString("label")) },
                it.optJSONObject("navigation")?.optJSONObject("next")?.let { x -> ApiNavigationItem(x.getString("logicalId"),x.getString("title")) })
        }

    suspend fun movie(id: String): ApiMovie = getJson("/api/v1/movies/$id", authenticated=true).movie()
    suspend fun progress(type: String, id: String): ApiProgress = getJson("/api/v1/playback/${type.uppercase()}/$id/progress", authenticated=true).let {
        ApiProgress(it.optDouble("position"), it.optDouble("duration"), it.optBoolean("watched"))
    }
    suspend fun startOver(type: String, id: String) = postJson("/api/v1/playback/${type.uppercase()}/$id/start-over", JSONObject(), authenticated=true)
    suspend fun search(query: String): ApiSearch = getJson("/api/v1/search?q=" + java.net.URLEncoder.encode(query,"UTF-8"), authenticated=true).let { root ->
        ApiSearch(root.optJSONArray("movies").orEmpty().mapObjects { it.movie() }, root.optJSONArray("shows").orEmpty().mapObjects { ApiShow(it.getString("id"),it.getString("title"),emptyList()) })
    }

    suspend fun show(id: String): ApiShow = getJson("/api/v1/shows/$id", authenticated = true).let { show ->
        val result=ApiShow(show.getString("id"), show.getString("title"), show.optJSONArray("seasons").orEmpty().mapObjects { season ->
            val seasonNumber = season.optInt("seasonNumber")
            season.optJSONArray("episodes").orEmpty().mapObjects { episode -> ApiEpisode(episode.getString("id"), episode.getString("title"), seasonNumber, episode.getInt("episodeNumber"), episode.optBoolean("available")) }
        }.flatten())
        result.copy(episodes=result.episodes.map { episode -> episode.copy(progress=runCatching { progress("EPISODE",episode.id) }.getOrNull()) })
    }

    suspend fun createDownload(type: String, id: String): ApiDownload = postJson("/api/v1/downloads",
        JSONObject().put("logicalType", type.uppercase()).put("logicalId", id).put("profileId", "ORIGINAL"), authenticated = true).download()

    suspend fun download(id: String): ApiDownload = getJson("/api/v1/downloads/$id", authenticated = true).download()
    suspend fun manifest(id: String): ApiManifest = getJson("/api/v1/downloads/$id/manifest", authenticated = true).let {
        ApiManifest(it.getString("title"), it.getString("fileUrl"), it.getJSONObject("download").download(), it.optJSONArray("artworkUrls")?.optString(0)?.takeIf(String::isNotBlank))
    }

    suspend fun transferArtwork(url: String, target: File) = withContext(Dispatchers.IO) {
        target.parentFile?.mkdirs()
        client.newCall(Request.Builder().url(if (url.startsWith("http")) url else baseUrl + url).authenticated().build()).execute().use { response ->
            if (!response.isSuccessful) throw ApiFailure(response.code, "Artwork request failed")
            target.outputStream().use { output -> response.body.byteStream().use { it.copyTo(output) } }
        }
    }

    suspend fun transfer(fileUrl: String, target: File, expectedSize: Long, expectedSha256: String, progress: (Long) -> Unit) = withContext(Dispatchers.IO) {
        target.parentFile?.mkdirs()
        var offset = if (target.exists()) target.length() else 0L
        if (offset > expectedSize) { target.delete(); offset = 0 }
        val request = Request.Builder().url(if (fileUrl.startsWith("http")) fileUrl else baseUrl + fileUrl).authenticated()
            .apply { if (offset > 0) header("Range", "bytes=$offset-") }.build()
        client.newCall(request).execute().use { response ->
            if (!(response.code == 200 && offset == 0L) && response.code != 206) throw ApiFailure(response.code, "Download Range request failed")
            RandomAccessFile(target, "rw").use { output -> output.seek(offset); response.body.byteStream().use { input ->
                val buffer = ByteArray(128 * 1024); var read: Int
                while (input.read(buffer).also { read = it } >= 0) { output.write(buffer, 0, read); offset += read; progress(offset) }
            } }
        }
        check(target.length() == expectedSize) { "Downloaded file length did not match the manifest" }
        val digest = MessageDigest.getInstance("SHA-256"); target.inputStream().use { input -> val buffer=ByteArray(128*1024); var n:Int; while(input.read(buffer).also{n=it}>0) digest.update(buffer,0,n) }
        check(digest.digest().joinToString("") { "%02x".format(it) }.equals(expectedSha256, true)) { "Downloaded file checksum did not match the manifest" }
    }

    suspend fun pushProgress(events: JSONArray) = postJson("/api/v1/sync/push", JSONObject().put("progressEvents", events).put("inventory", JSONArray()), authenticated = true)
    suspend fun pullChanges(cursor: Long): ApiChanges = getJson("/api/v1/sync/changes?cursor=$cursor&limit=100", authenticated = true).let { root ->
        ApiChanges(root.getLong("cursor"), root.optJSONArray("changes").orEmpty().mapObjects { ApiChange(it.getString("type"), it.getString("entityId")) })
    }

    suspend fun updatePlayback(id: String, state: String, positionSeconds: Double, durationSeconds: Double) =
        patchJson("/api/v1/playback/sessions/$id", JSONObject().put("state", state)
            .put("position", positionSeconds).put("duration", durationSeconds))

    suspend fun stopPlayback(id: String) = execute(Request.Builder().url(baseUrl + "/api/v1/playback/sessions/$id")
        .delete().authenticated().build())

    private fun homeItem(x: JSONObject) = ApiHomeItem(x.optString("Type", x.optString("type")), x.optString("ID", x.optString("id")), x.optString("Title", x.optString("title")), x.optNullable("Subtitle", "subtitle"), x.optNullable("ArtworkID", "artworkId"))
    private fun track(x: JSONObject) = ApiTrack(x.getString("id"),x.getString("kind"),x.getString("codec"),x.optString("language").takeIf(String::isNotBlank),x.optString("title").takeIf(String::isNotBlank),x.optBoolean("default"),x.optBoolean("forced"),x.optBoolean("commentary"))
    private fun JSONObject.movie() = ApiMovie(getString("id"),getString("title"),optString("overview"),optInt("year"),optInt("runtimeMinutes"),optString("artworkId").takeIf(String::isNotBlank))

    private suspend fun getJson(path: String, authenticated: Boolean = false) = execute(Request.Builder().url(baseUrl + path).apply { if (authenticated) accessToken?.let { header("Authorization", "Bearer $it") } }.build())
    private suspend fun postJson(path: String, body: JSONObject, native: Boolean = false, authenticated: Boolean = false) = execute(Request.Builder().url(baseUrl + path).post(body.toString().toRequestBody(JSON)).apply { if (native) header("X-VyNode-Client", "native"); if (authenticated) authenticated() }.build())
    private suspend fun patchJson(path: String, body: JSONObject) = execute(Request.Builder().url(baseUrl + path).patch(body.toString().toRequestBody(JSON)).authenticated().build())
    private fun Request.Builder.authenticated() = apply { accessToken?.let { header("Authorization", "Bearer $it") } }
    private suspend fun execute(request: Request): JSONObject = withContext(Dispatchers.IO) {
        client.newCall(request).execute().use { response ->
            val text = response.body.string()
            if (!response.isSuccessful) throw ApiFailure(response.code, runCatching { JSONObject(text).getJSONObject("error").getString("message") }.getOrDefault("Server request failed"))
            if (text.isBlank()) JSONObject() else JSONObject(text)
        }
    }
    private fun JSONObject.session() = RotatedSession(getString("accessToken"), getString("refreshToken"))
    private fun JSONObject.download() = ApiDownload(getString("id"), getString("logicalId"), getString("assetId"), getString("status"), optLong("sizeBytes"), optString("checksumSha256"))
    private fun JSONObject.optNullable(primary: String, fallback: String) = optString(primary, optString(fallback)).takeIf { it.isNotBlank() }
    private fun JSONArray?.orEmpty() = this ?: JSONArray()
    private fun <T> JSONArray.mapObjects(transform: (JSONObject) -> T) = (0 until length()).map { transform(getJSONObject(it)) }
    private companion object { val JSON = "application/json; charset=utf-8".toMediaType() }
}
