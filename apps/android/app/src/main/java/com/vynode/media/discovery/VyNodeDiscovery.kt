package com.vynode.media.discovery

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

data class DiscoveredServer(val name: String, val host: String, val port: Int)

class VyNodeDiscovery(context: Context) : NsdManager.DiscoveryListener {
    private val manager = context.getSystemService(NsdManager::class.java)
    private val mutable = MutableStateFlow<List<DiscoveredServer>>(emptyList())
    val servers: StateFlow<List<DiscoveredServer>> = mutable
    private var active = false

    fun start() { if (!active) { active = true; manager.discoverServices("_vynode-media._tcp", NsdManager.PROTOCOL_DNS_SD, this) } }
    fun stop() { if (active) { active = false; runCatching { manager.stopServiceDiscovery(this) } } }
    override fun onServiceFound(service: NsdServiceInfo) { manager.resolveService(service, object : NsdManager.ResolveListener {
        override fun onResolveFailed(serviceInfo: NsdServiceInfo, errorCode: Int) = Unit
        override fun onServiceResolved(info: NsdServiceInfo) { mutable.value = (mutable.value + DiscoveredServer(info.serviceName, info.host.hostAddress ?: return, info.port)).distinctBy { it.host to it.port } }
    }) }
    override fun onServiceLost(service: NsdServiceInfo) { mutable.value = mutable.value.filterNot { it.name == service.serviceName } }
    override fun onDiscoveryStarted(type: String) = Unit
    override fun onDiscoveryStopped(type: String) { active = false }
    override fun onStartDiscoveryFailed(type: String, errorCode: Int) { active = false }
    override fun onStopDiscoveryFailed(type: String, errorCode: Int) { active = false }
}
