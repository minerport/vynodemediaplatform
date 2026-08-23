package com.vynode.media.playback

import android.content.Context
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.Player
import androidx.media3.common.C
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.hls.HlsMediaSource
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import android.media.MediaCodecList
import android.os.Build
import org.json.JSONArray
import org.json.JSONObject

object DeviceCapabilities {
    fun detect(): JSONObject {
        val decoders = MediaCodecList(MediaCodecList.ALL_CODECS).codecInfos.filterNot { it.isEncoder }
        val codecs = decoders.flatMap { it.supportedTypes.asIterable() }.map(String::lowercase).toSet()
        val maximumVideoWidth = decoders.flatMap { info ->
            info.supportedTypes.filter { it.startsWith("video/") }.mapNotNull { type ->
                runCatching { info.getCapabilitiesForType(type).videoCapabilities?.supportedWidths?.upper }.getOrNull()
            }
        }.maxOrNull()
        val maximumVideoHeight = decoders.flatMap { info ->
            info.supportedTypes.filter { it.startsWith("video/") }.mapNotNull { type ->
                runCatching { info.getCapabilitiesForType(type).videoCapabilities?.supportedHeights?.upper }.getOrNull()
            }
        }.maxOrNull()
        val maximumAudioChannels = decoders.flatMap { info ->
            info.supportedTypes.filter { it.startsWith("audio/") }.mapNotNull { type ->
                runCatching { info.getCapabilitiesForType(type).audioCapabilities?.maxInputChannelCount }.getOrNull()
            }
        }.maxOrNull()?.coerceAtMost(16)
        fun has(mime: String) = mime in codecs
        return JSONObject().put("schemaVersion", 1).put("clientName", "VyNode Android")
            .put("clientVersion", "0.1.0").put("platform", "Android").put("platformVersion", Build.VERSION.RELEASE)
            // MediaCodec reports elementary stream support, not container support.
            // Keep this list conservative and limited to containers Media3 can
            // direct-play consistently across the supported Android baseline.
            .put("deviceModel", Build.MODEL).put("supportedContainers", JSONArray(listOf("mp4", "webm", "mpegts")))
            .put("supportedVideoCodecs", JSONArray(buildList { if (has("video/avc")) add("h264"); if (has("video/hevc")) add("hevc"); if (has("video/x-vnd.on2.vp9")) add("vp9"); if (has("video/av01")) add("av1") }))
            .put("supportedAudioCodecs", JSONArray(buildList { if (has("audio/mp4a-latm")) add("aac"); if (has("audio/mpeg")) add("mp3"); if (has("audio/opus")) add("opus"); if (has("audio/ac3")) add("ac3"); if (has("audio/eac3")) add("eac3") }))
            .put("subtitleFormats", JSONArray(listOf("srt", "webvtt", "ass")))
            .apply { maximumVideoWidth?.let { put("maximumVideoWidth", it) }; maximumVideoHeight?.let { put("maximumVideoHeight", it) }; maximumAudioChannels?.let { put("maximumAudioChannels", it) } }
            .put("directPlaySupport", true).put("fragmentedMp4Support", true)
    }
}

@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
class VyNodePlayer(private val context: Context, private val token: () -> String?) {
    val player: ExoPlayer = ExoPlayer.Builder(context).build()

    fun play(url: String, hls: Boolean, positionMs: Long = 0, subtitleUrl: String? = null) {
        val item = MediaItem.Builder().setUri(url).apply {
            if (hls) setMimeType(MimeTypes.APPLICATION_M3U8)
            if (!subtitleUrl.isNullOrBlank()) {
                setSubtitleConfigurations(
                    listOf(
                        MediaItem.SubtitleConfiguration.Builder(android.net.Uri.parse(subtitleUrl))
                            // Playback subtitle endpoints normalize supported text
                            // sidecars to WebVTT, regardless of the source codec.
                            .setMimeType(MimeTypes.TEXT_VTT)
                            .setLanguage("und")
                            .setLabel("Selected subtitles")
                            .setSelectionFlags(C.SELECTION_FLAG_DEFAULT)
                            .build()
                    )
                )
            }
        }.build()
        if (url.startsWith("file:") || url.startsWith("content:")) {
            player.setMediaItem(item)
            player.prepare(); player.seekTo(positionMs); player.playWhenReady = true
            return
        }
        val headers = token()?.let { mapOf("Authorization" to "Bearer $it") } ?: emptyMap()
        val factory = DefaultHttpDataSource.Factory().setDefaultRequestProperties(headers)
        // The same authenticated data source is used for the primary media,
        // HLS children, and externally served subtitle tracks.
        player.setMediaSource(DefaultMediaSourceFactory(context).setDataSourceFactory(factory).createMediaSource(item))
        player.prepare(); player.seekTo(positionMs); player.playWhenReady = true
    }

    fun seekToMarkerEnd(endSeconds: Double) = player.seekTo((endSeconds * 1000).toLong())
    fun release() = player.release()
    fun toggle() {
        if (player.playWhenReady) player.pause() else player.play()
    }
    fun seekBy(deltaMs: Long) = player.seekTo((player.currentPosition + deltaMs).coerceAtLeast(0))
}
