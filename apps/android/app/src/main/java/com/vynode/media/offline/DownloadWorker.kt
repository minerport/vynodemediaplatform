package com.vynode.media.offline

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.Data
import androidx.work.WorkerParameters
import com.vynode.media.auth.SecureTokenStore
import com.vynode.media.auth.SessionCoordinator
import com.vynode.media.data.ClientDatabase
import com.vynode.media.data.DownloadEntity
import com.vynode.media.network.ServerTrust
import com.vynode.media.network.TrustDecision
import com.vynode.media.network.VyNodeApi
import kotlinx.coroutines.delay

class DownloadWorker(context: Context, parameters: WorkerParameters) : CoroutineWorker(context, parameters) {
    override suspend fun doWork(): Result {
        val serverId = inputData.getString(SERVER) ?: return Result.failure()
        val downloadId = inputData.getString(DOWNLOAD) ?: return Result.failure()
        val database = ClientDatabase.open(applicationContext)
        val server = database.dao().server(serverId) ?: return Result.failure()
        val api = VyNodeApi(server.endpoint)
        val identity = runCatching { api.identity() }.getOrElse { return Result.retry() }
        if (ServerTrust.evaluate(serverId, identity) is TrustDecision.IdentityChanged) return Result.failure()
        api.accessToken = runCatching { SessionCoordinator(serverId, SecureTokenStore(applicationContext), api).refresh(null) }.getOrElse { return Result.retry() }
        var download = runCatching { api.download(downloadId) }.getOrElse { return Result.retry() }
        var attempts = 0
        while (download.status != "READY" && attempts++ < 300) {
            if (download.status in setOf("FAILED", "CANCELED", "REVOKED")) return Result.failure()
            delay(1_000); download = runCatching { api.download(downloadId) }.getOrElse { return Result.retry() }
        }
        if (download.status != "READY") return Result.retry()
        val manifest = runCatching { api.manifest(downloadId) }.getOrElse { return Result.retry() }
        val target = applicationContext.filesDir.resolve("offline/$serverId/$downloadId.mp4")
        return runCatching {
            api.transfer(manifest.fileUrl, target, manifest.download.size, manifest.download.checksum) { bytes ->
                setProgressAsync(Data.Builder().putLong(BYTES, bytes).putLong(TOTAL, manifest.download.size).build())
            }
            val artwork = manifest.artworkUrl?.let { url ->
                applicationContext.filesDir.resolve("offline/$serverId/$downloadId-artwork").let { target ->
                    runCatching { api.transferArtwork(url, target) }.getOrNull()?.let { target.absolutePath }
                }
            }
            database.dao().saveDownload(DownloadEntity(serverId, downloadId, download.logicalId, manifest.title,
                download.assetId, download.size, download.checksum, target.absolutePath, "READY", manifest.artworkUrl, artwork))
            Result.success()
        }.getOrElse { Result.retry() }
    }

    companion object {
        const val SERVER = "serverId"; const val DOWNLOAD = "downloadId"; const val BYTES = "bytes"; const val TOTAL = "total"
    }
}
