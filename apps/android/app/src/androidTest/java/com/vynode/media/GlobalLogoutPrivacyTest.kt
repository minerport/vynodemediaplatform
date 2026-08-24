package com.vynode.media

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.vynode.media.auth.SecureTokenStore
import com.vynode.media.data.ClientDatabase
import com.vynode.media.data.DownloadEntity
import com.vynode.media.data.GlobalAccountEntity
import com.vynode.media.data.ProgressEntity
import com.vynode.media.data.ServerEntity
import com.vynode.media.data.SyncStateEntity
import java.io.File
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class GlobalLogoutPrivacyTest {
    @Test
    fun logoutDeletesAccountAStateBeforeAccountSwitchOrReauthentication() {
        runBlocking {
        val context = ApplicationProvider.getApplicationContext<Context>()
        context.deleteDatabase("vynode-client.db")
        context.getSharedPreferences("vynode_secure_session", Context.MODE_PRIVATE).edit().clear().commit()
        val offlineDir = context.filesDir.resolve("offline/server-a").apply { mkdirs() }
        val media = File(offlineDir, "download-a.mp4").apply { writeBytes(ByteArray(4096) { it.toByte() }) }
        val artwork = File(offlineDir, "download-a-artwork").apply { writeBytes(byteArrayOf(1, 2, 3)) }
        val db = ClientDatabase.open(context)
        val dao = db.dao()
        dao.saveGlobalAccount(GlobalAccountEntity("account-a", "account-a", "Account A", true, 1))
        dao.saveServer(ServerEntity("server-a", "Server A", "http://127.0.0.1:1", true, 1, "account-a"))
        dao.saveDownload(DownloadEntity("server-a", "download-a", "movie-a", "Private movie", "asset-a", media.length(), "sha", media.absolutePath, "READY", "/art", artwork.absolutePath))
        dao.saveSyncState(SyncStateEntity("server-a", 42, "epoch-a", 9))
        dao.enqueue(ProgressEntity(serverId="server-a", eventId="event-a", sequence=1, logicalType="MOVIE", logicalId="movie-a", positionSeconds=3.0, durationSeconds=20.0, occurredAt="2026-08-23T00:00:00Z", action=null))
        val tokens = SecureTokenStore(context)
        tokens.replace("server-a", "local-refresh-secret")
        tokens.replace("__vynode_global__", "global-refresh-secret")
        db.close()

        val controller = AppController(context, tv = false)
        controller.globalLogout()
        repeat(100) {
            if (controller.screen.value == AppScreen.GlobalSignIn && tokens.read("server-a") == null) return@repeat
            delay(50)
        }

        val verified = ClientDatabase.open(context)
        assertEquals(AppScreen.GlobalSignIn, controller.screen.value)
        assertNull(tokens.read("server-a"))
        assertNull(tokens.read("__vynode_global__"))
        assertTrue(verified.dao().downloadsForServer("server-a").isEmpty())
        assertTrue(verified.dao().pendingProgress("server-a").isEmpty())
        assertNull(verified.dao().syncState("server-a"))
        assertFalse(media.exists())
        assertFalse(artwork.exists())
        assertFalse(offlineDir.exists())

        verified.dao().saveGlobalAccount(GlobalAccountEntity("account-b", "account-b", "Account B", true, 2))
        assertTrue(verified.dao().downloadsForServer("server-a").isEmpty())
        assertTrue(verified.dao().pendingProgress("server-a").isEmpty())
        assertNull(verified.dao().syncState("server-a"))
        verified.dao().deactivateGlobalAccounts()
        verified.dao().saveGlobalAccount(GlobalAccountEntity("account-a", "account-a", "Account A", true, 3))
        assertTrue(verified.dao().downloadsForServer("server-a").isEmpty())
        assertFalse(media.exists())
        verified.close()
        controller.close()
        context.deleteDatabase("vynode-client.db")
        Unit
        }
    }
}
