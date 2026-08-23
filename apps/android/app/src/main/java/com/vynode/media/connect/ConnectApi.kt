package com.vynode.media.connect

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject

data class GlobalAccount(val id: String, val username: String, val displayName: String)
data class GlobalTokens(val accessToken: String, val refreshToken: String, val account: GlobalAccount)
data class ConnectedServer(val id: String, val name: String, val relationship: String, val endpoints: List<String>)
data class DeviceAuthorization(val deviceCode: String, val userCode: String, val verificationPath: String, val pollAfterSeconds: Int)

/** Typed, app-wide Connect client shared by phone, tablet, and TV. Media requests never pass through it. */
class ConnectApi(private val baseUrl: String, private val http: OkHttpClient = OkHttpClient()) {
    suspend fun login(username: String, password: String, deviceName: String): GlobalTokens = post("/api/v1/account/login", JSONObject().put("username", username).put("password", password).put("deviceName", deviceName).put("platform", "ANDROID")).tokens()
    suspend fun register(username: String, displayName: String, password: String, deviceName: String): GlobalTokens = post("/api/v1/account/register", JSONObject().put("username", username).put("displayName", displayName).put("password", password).put("deviceName", deviceName).put("platform", "ANDROID")).tokens()
    suspend fun refresh(refreshToken: String): GlobalTokens = post("/api/v1/account/refresh", JSONObject().put("refreshToken", refreshToken)).tokens()
    suspend fun deviceCode(deviceName: String): DeviceAuthorization { val v=post("/api/v1/device-codes",JSONObject().put("deviceName",deviceName).put("platform","ANDROID_TV"));return DeviceAuthorization(v.getString("deviceCode"),v.getString("userCode"),v.getString("verificationPath"),v.getInt("pollAfterSeconds")) }
	suspend fun exchangeDeviceCode(deviceCode:String):GlobalTokens=post("/api/v1/device-codes/exchange",JSONObject().put("deviceCode",deviceCode)).tokens()
    suspend fun assertion(accessToken: String, serverId: String): String = post("/api/v1/servers/$serverId/assertion",JSONObject(),accessToken).getString("assertion")
	suspend fun servers(accessToken:String):List<ConnectedServer>{val values=get("/api/v1/servers",accessToken);return (0 until values.length()).map{index->val v=values.getJSONObject(index);ConnectedServer(v.getString("id"),v.getString("name"),v.getString("relationship"),(0 until v.getJSONArray("endpoints").length()).map{v.getJSONArray("endpoints").getJSONObject(it).getString("url")})}}
	private suspend fun get(path:String,bearer:String)=withContext(Dispatchers.IO){val request=Request.Builder().url(baseUrl.trimEnd('/')+path).header("X-VyNode-Client","native").header("Authorization","Bearer $bearer").build();http.newCall(request).execute().use{response->val raw=response.body.string();if(!response.isSuccessful)throw ConnectException(response.code);org.json.JSONArray(raw)}}
    private suspend fun post(path:String,body:JSONObject,bearer:String?=null)=withContext(Dispatchers.IO){val request=Request.Builder().url(baseUrl.trimEnd('/')+path).header("X-VyNode-Client","native").post(body.toString().toRequestBody(JSON)).apply{if(bearer!=null)header("Authorization","Bearer $bearer")}.build();http.newCall(request).execute().use{response->val raw=response.body.string();if(!response.isSuccessful)throw ConnectException(response.code);JSONObject(raw)}}
    private fun JSONObject.tokens():GlobalTokens{val a=getJSONObject("account");return GlobalTokens(getString("accessToken"),getString("refreshToken"),GlobalAccount(a.getString("id"),a.getString("username"),a.getString("displayName")))}
    private companion object { val JSON="application/json".toMediaType() }
}
class ConnectException(val status:Int):Exception("Connect request failed ($status)")
