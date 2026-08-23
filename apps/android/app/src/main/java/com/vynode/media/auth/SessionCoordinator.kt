package com.vynode.media.auth

import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.util.concurrent.ConcurrentHashMap

data class RotatedSession(val accessToken: String, val refreshToken: String)
fun interface RefreshTransport { suspend fun rotate(refreshToken: String): RotatedSession }

/** One refresh can be in flight per active server; concurrent 401s share its result. */
class SessionCoordinator(
    private val serverId: String,
    private val store: TokenStore,
    private val transport: RefreshTransport,
) {
    // Coordinators are created by both the foreground controller and
    // WorkManager. They must share one lock per server or a background download
    // could race the UI and make rotated-refresh reuse look like token theft.
    private val state = refreshStates.computeIfAbsent(serverId) { RefreshState() }
    private val mutex = state.mutex

    fun currentAccessToken(): String? = state.accessToken

    suspend fun refresh(failedToken: String?): String = mutex.withLock {
        state.accessToken?.takeIf { failedToken == null || it != failedToken }?.let { return it }
        val old = store.read(serverId) ?: error("Session is not available")
        val rotated = transport.rotate(old)
        store.replace(serverId, rotated.refreshToken)
        state.accessToken = rotated.accessToken
        rotated.accessToken
    }

    fun establish(session: RotatedSession) {
        store.replace(serverId, session.refreshToken)
        state.accessToken = session.accessToken
    }

    fun revoke() { state.accessToken = null; store.clear(serverId); refreshStates.remove(serverId, state) }

    private class RefreshState(val mutex: Mutex = Mutex(), @Volatile var accessToken: String? = null)
    private companion object { val refreshStates = ConcurrentHashMap<String, RefreshState>() }
}
