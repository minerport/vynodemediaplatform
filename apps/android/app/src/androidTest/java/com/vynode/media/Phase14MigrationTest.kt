package com.vynode.media

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import com.vynode.media.data.ClientDatabase
import java.io.File
import java.security.MessageDigest
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class Phase14MigrationTest {
    @Test
    fun phase13StateIsPreserved() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        val name = "phase14-migration.db"
        val mediaBytes = InstrumentationRegistry.getInstrumentation().context.assets
            .open("phase13-offline.mp4").use { it.readBytes() }
        val mediaSha256 = mediaBytes.sha256()
        val artworkBytes = byteArrayOf(9, 8, 7, 6)
        val media = File(context.filesDir, "phase13-download.mp4").apply { writeBytes(mediaBytes) }
        val artwork = File(context.filesDir, "phase13-artwork.jpg").apply { writeBytes(artworkBytes) }
        context.deleteDatabase(name)
        context.openOrCreateDatabase(name, Context.MODE_PRIVATE, null).apply {
            execSQL("CREATE TABLE servers (serverId TEXT NOT NULL,name TEXT NOT NULL,endpoint TEXT NOT NULL,insecureHttp INTEGER NOT NULL,lastConnectedAt INTEGER,PRIMARY KEY(serverId))")
            execSQL("CREATE TABLE downloads (serverId TEXT NOT NULL,downloadId TEXT NOT NULL,logicalId TEXT NOT NULL,title TEXT NOT NULL,assetId TEXT NOT NULL,sizeBytes INTEGER NOT NULL,checksum TEXT NOT NULL,localFile TEXT,state TEXT NOT NULL,artworkUrl TEXT,localArtwork TEXT,PRIMARY KEY(serverId,downloadId))")
            execSQL("CREATE TABLE sync_state (serverId TEXT NOT NULL,cursor INTEGER NOT NULL,installationEpoch TEXT NOT NULL,nextSequence INTEGER NOT NULL,PRIMARY KEY(serverId))")
            execSQL("CREATE TABLE progress_queue (rowId INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,serverId TEXT NOT NULL,eventId TEXT NOT NULL,sequence INTEGER NOT NULL,logicalType TEXT NOT NULL,logicalId TEXT NOT NULL,positionSeconds REAL NOT NULL,durationSeconds REAL NOT NULL,occurredAt TEXT NOT NULL,action TEXT)")
            execSQL("CREATE UNIQUE INDEX index_progress_queue_serverId_eventId ON progress_queue(serverId,eventId)")
            execSQL("INSERT INTO servers VALUES('server-a','A','http://127.0.0.1:1',1,123)")
            execSQL("INSERT INTO downloads VALUES(?,?,?,?,?,?,?,?,?,?,?)", arrayOf("server-a", "download-a", "movie-a", "Movie", "asset", mediaBytes.size, mediaSha256, media.absolutePath, "READY", "/art", artwork.absolutePath))
            execSQL("INSERT INTO sync_state VALUES('server-a',77,'epoch',88)")
            execSQL("INSERT INTO progress_queue(serverId,eventId,sequence,logicalType,logicalId,positionSeconds,durationSeconds,occurredAt,action) VALUES('server-a','event',9,'MOVIE','movie-a',12,100,'2026-01-01T00:00:00Z',NULL)")
            version = 2
            close()
        }

        val db = ClientDatabase.open(context, name)
        db.openHelper.writableDatabase.query("SELECT COUNT(*) FROM servers WHERE serverId='server-a' AND globalAccountId IS NULL").use {
            it.moveToFirst()
            assertEquals(1, it.getInt(0))
        }
        db.openHelper.writableDatabase.query("SELECT (SELECT COUNT(*) FROM downloads)+(SELECT COUNT(*) FROM sync_state)+(SELECT COUNT(*) FROM progress_queue)+(SELECT COUNT(*) FROM global_accounts)").use {
            it.moveToFirst()
            assertEquals(3, it.getInt(0))
        }
        assertArrayEquals(mediaBytes, media.readBytes())
        assertEquals(mediaSha256, media.readBytes().sha256())
        assertEquals(mediaBytes.size.toLong(), media.length())
        assertArrayEquals(artworkBytes, artwork.readBytes())
        assertLocalPlaybackAndSeek(context, media)
        db.close()
        context.deleteDatabase(name)
        media.delete()
        artwork.delete()
    }

    private fun assertLocalPlaybackAndSeek(context: Context, media: File) {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val ready = CountDownLatch(1)
        lateinit var player: ExoPlayer
        instrumentation.runOnMainSync {
            player = ExoPlayer.Builder(context).build().apply {
                addListener(object : Player.Listener {
                    override fun onPlaybackStateChanged(state: Int) {
                        if (state == Player.STATE_READY) ready.countDown()
                    }
                })
                setMediaItem(MediaItem.fromUri(media.toURI().toString()))
                prepare()
                play()
            }
        }
        assertTrue("Media3 did not prepare the migrated local MP4", ready.await(10, TimeUnit.SECONDS))
        Thread.sleep(1_000)
        var advanced = 0L
        instrumentation.runOnMainSync { advanced = player.currentPosition }
        assertTrue("Local playback position did not advance", advanced > 250)
        instrumentation.runOnMainSync { player.seekTo(3_000) }
        Thread.sleep(500)
        var sought = 0L
        instrumentation.runOnMainSync { sought = player.currentPosition; player.release() }
        assertTrue("Media3 did not seek in the migrated local MP4", sought >= 2_500)
    }

    private fun ByteArray.sha256() = MessageDigest.getInstance("SHA-256")
        .digest(this).joinToString("") { "%02x".format(it) }
}
