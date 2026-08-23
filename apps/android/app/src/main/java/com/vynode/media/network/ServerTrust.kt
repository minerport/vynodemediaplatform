package com.vynode.media.network

data class ServerIdentity(val instanceId: String, val serverName: String, val apiVersion: String)

sealed interface TrustDecision {
    data object Trusted : TrustDecision
    data class FirstConnection(val identity: ServerIdentity) : TrustDecision
    data class IdentityChanged(val expected: String, val received: ServerIdentity) : TrustDecision
}

object ServerTrust {
    fun evaluate(savedInstanceId: String?, received: ServerIdentity): TrustDecision = when {
        savedInstanceId == null -> TrustDecision.FirstConnection(received)
        savedInstanceId == received.instanceId -> TrustDecision.Trusted
        else -> TrustDecision.IdentityChanged(savedInstanceId, received)
    }
}
