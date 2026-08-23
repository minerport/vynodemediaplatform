package com.vynode.media.auth

import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import java.util.concurrent.atomic.AtomicInteger

class SessionCoordinatorTest {
    private class MemoryStore(var token: String? = "refresh-1", var fail: Boolean = false) : TokenStore {
        override fun read(serverId: String) = token
        override fun replace(serverId: String, refreshToken: String) { if (fail) error("disk full"); token = refreshToken }
        override fun clear(serverId: String) { token = null }
    }

    @Test fun concurrentFailuresRotateExactlyOnce() = runTest {
        val calls = AtomicInteger()
        val store = MemoryStore()
        val coordinator = SessionCoordinator("concurrent-server", store) { calls.incrementAndGet(); RotatedSession("access-2", "refresh-2") }
        val results = (1..20).map { async { coordinator.refresh("access-1") } }.awaitAll()
        assertEquals(1, calls.get())
        assertEquals(setOf("access-2"), results.toSet())
        assertEquals("refresh-2", store.token)
    }

    @Test fun rotatedAccessIsNotPublishedWhenDurableCommitFails() = runTest {
        val store = MemoryStore(fail = true)
        val coordinator = SessionCoordinator("failure-server", store) { RotatedSession("access-2", "refresh-2") }
        runCatching { coordinator.refresh("access-1") }
        assertNull(coordinator.currentAccessToken())
        assertEquals("refresh-1", store.token)
    }

    @Test fun foregroundAndWorkerCoordinatorsShareOneRotation() = runTest {
        val calls = AtomicInteger(); val store = MemoryStore()
        val transport = RefreshTransport { calls.incrementAndGet(); RotatedSession("access-shared", "refresh-shared") }
        val foreground = SessionCoordinator("shared-server", store, transport)
        val worker = SessionCoordinator("shared-server", store, transport)
        val results = listOf(async { foreground.refresh("access-old") }, async { worker.refresh("access-old") }).awaitAll()
        assertEquals(1, calls.get())
        assertEquals(setOf("access-shared"), results.toSet())
    }
}
