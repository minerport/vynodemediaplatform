package com.vynode.media.network

import org.junit.Assert.assertTrue
import org.junit.Test

class ServerTrustTest {
    private val server = ServerIdentity("stable-a", "Home", "v1")
    @Test fun firstConnectionRequiresTrust() = assertTrue(ServerTrust.evaluate(null, server) is TrustDecision.FirstConnection)
    @Test fun stableIdentityIsTrusted() = assertTrue(ServerTrust.evaluate("stable-a", server) is TrustDecision.Trusted)
    @Test fun changedIdentityBlocksCredentialSubmission() = assertTrue(ServerTrust.evaluate("stable-b", server) is TrustDecision.IdentityChanged)
}
