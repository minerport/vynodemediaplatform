package com.vynode.media.auth

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** Refresh tokens are encrypted by a non-exportable Android Keystore key. */
interface TokenStore { fun read(serverId: String): String?; fun replace(serverId: String, refreshToken: String); fun clear(serverId: String) }

class SecureTokenStore(context: Context) : TokenStore {
    private val preferences = context.getSharedPreferences("vynode_secure_session", Context.MODE_PRIVATE)
    private val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }

    @Synchronized override fun read(serverId: String): String? {
        val value = preferences.getString(serverId, null) ?: return null
        val bytes = android.util.Base64.decode(value, android.util.Base64.NO_WRAP)
        val iv = bytes.copyOfRange(0, 12)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, iv))
        return cipher.doFinal(bytes.copyOfRange(12, bytes.size)).decodeToString()
    }

    /** Synchronous commit is required before a newly rotated access token is published. */
    @Synchronized override fun replace(serverId: String, refreshToken: String) {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, key())
        val encrypted = cipher.iv + cipher.doFinal(refreshToken.encodeToByteArray())
        check(preferences.edit().putString(serverId, android.util.Base64.encodeToString(encrypted, android.util.Base64.NO_WRAP)).commit())
    }

    @Synchronized override fun clear(serverId: String) { check(preferences.edit().remove(serverId).commit()) }

    private fun key(): SecretKey {
        (keyStore.getKey(KEY_ALIAS, null) as? SecretKey)?.let { return it }
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").run {
            init(KeyGenParameterSpec.Builder(KEY_ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM).setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE).build())
            generateKey()
        }
    }
    private companion object { const val KEY_ALIAS = "vynode_refresh_v1" }
}
