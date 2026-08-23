package com.vynode.media

import android.app.UiModeManager
import android.content.res.Configuration
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.input.key.*
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.zIndex
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.activity.compose.BackHandler
import androidx.media3.common.Player
import androidx.media3.common.text.CueGroup
import androidx.media3.ui.PlayerView
import com.vynode.media.network.ApiHomeItem
import com.vynode.media.playback.VyNodePlayer
import com.vynode.media.data.DownloadEntity
import com.vynode.media.network.ApiEpisode
import com.vynode.media.discovery.VyNodeDiscovery
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.vynode.media.connect.ConnectedServer

class MainActivity : ComponentActivity() {
    private var controller: AppController? = null
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val isTv = getSystemService(UiModeManager::class.java).currentModeType == Configuration.UI_MODE_TYPE_TELEVISION
        controller = AppController(this, isTv)
        setContent { VyNodeTheme { VyNodeApp(controller!!, isTv) } }
    }
    override fun onDestroy() { controller?.close(); super.onDestroy() }
}

data class HomeCard(val item: ApiHomeItem) { val id get() = item.id; val title get() = item.title }
data class HomeRow(val id: String, val title: String, val cards: List<HomeCard>)

private fun Modifier.remoteActivate(action: () -> Unit) = onPreviewKeyEvent { event ->
    if (event.key in listOf(Key.DirectionCenter, Key.Enter, Key.NumPadEnter) && event.type == KeyEventType.KeyUp) {
        action()
        true
    } else false
}

private fun formatPlayerTime(milliseconds: Long): String {
    val total=(milliseconds.coerceAtLeast(0)/1000)
    val hours=total/3600
    val minutes=(total%3600)/60
    val seconds=total%60
    return if(hours>0) "%d:%02d:%02d".format(hours,minutes,seconds) else "%02d:%02d".format(minutes,seconds)
}

@Composable fun VyNodeApp(controller: AppController, tv: Boolean) {
    when (val screen = controller.screen.collectAsStateWithLifecycle().value) {
        AppScreen.GlobalSignIn -> GlobalSignInScreen(tv,controller::globalLogin,controller::globalRegister,controller::beginTvGlobalSignIn,controller::advancedConnect)
        is AppScreen.GlobalDeviceCode -> GlobalDeviceCodeScreen(screen)
        is AppScreen.ServerPicker -> ServerPickerScreen(screen,controller::selectGlobalServer,controller::advancedConnect)
        AppScreen.Connect -> ConnectScreen(tv, controller::connect)
        is AppScreen.ConfirmInsecure -> ConfirmInsecure(screen.endpoint) { controller.connect(screen.endpoint, true) }
        is AppScreen.Pair -> PairScreen(screen)
        is AppScreen.Home -> VyNodeHome(tv, screen.rows.map { row -> HomeRow(row.id, row.title, row.items.map(::HomeCard)) }, controller::open, screen.focusId, controller::openSearch,controller::chooseGlobalServer,controller::globalLogout)
        is AppScreen.Movie -> MovieScreen(screen, controller)
        is AppScreen.Playing -> PlayerScreen(screen, controller)
        is AppScreen.Show -> ShowScreen(screen, controller::play, controller::startOver, controller::download, controller::returnFromDetail, tv)
        is AppScreen.Search -> SearchScreen(screen, controller)
        is AppScreen.Offline -> OfflineScreen(screen.downloads, controller::playOffline)
        is AppScreen.LocalPlaying -> OfflinePlayerScreen(screen.download, controller)
        is AppScreen.Error -> MessageScreen(screen.message, controller::retry)
        is AppScreen.IdentityMismatch -> MessageScreen("This address now identifies as ${screen.received.serverName}. Credentials were not sent.", controller::retry)
    }
}

@Composable private fun GlobalSignInScreen(tv:Boolean,login:(String,String)->Unit,register:(String,String,String)->Unit,tvCode:()->Unit,advanced:()->Unit){var username by remember{mutableStateOf("")};var displayName by remember{mutableStateOf("")};var password by remember{mutableStateOf("")};var creating by remember{mutableStateOf(false)};Column(Modifier.fillMaxSize().background(Brush.radialGradient(listOf(Color(0x55317767),VyNodeColor.Background),radius=1100f)).padding(if(tv)64.dp else 24.dp),verticalArrangement=Arrangement.Center,horizontalAlignment=androidx.compose.ui.Alignment.CenterHorizontally){Surface(Modifier.widthIn(max=620.dp),color=VyNodeColor.Surface,shape=VyNodeRadius.large){Column(Modifier.padding(if(tv)40.dp else 24.dp),verticalArrangement=Arrangement.spacedBy(16.dp)){Text("VYNODE MEDIA",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text("Welcome to VyNode",style=MaterialTheme.typography.headlineLarge);Text("Sign in once to find the servers linked to your account.",color=VyNodeColor.Muted);if(tv){VyNodePrimaryButton("Sign In to VyNode",onClick=tvCode)}else{OutlinedTextField(username,{username=it},label={Text("Username")},singleLine=true,modifier=Modifier.fillMaxWidth());if(creating)OutlinedTextField(displayName,{displayName=it},label={Text("Display name")},singleLine=true,modifier=Modifier.fillMaxWidth());OutlinedTextField(password,{password=it},label={Text("Password")},singleLine=true,visualTransformation=androidx.compose.ui.text.input.PasswordVisualTransformation(),modifier=Modifier.fillMaxWidth());VyNodePrimaryButton(if(creating)"Create VyNode Account" else "Sign In to VyNode",enabled=username.isNotBlank()&&password.isNotBlank()&&(!creating||displayName.isNotBlank())){if(creating)register(username,displayName,password)else login(username,password)};TextButton(onClick={creating=!creating}){Text(if(creating)"Already have an account? Sign in" else "Create Account")}};TextButton(onClick=advanced){Text("Advanced · Connect manually")}}}}}

@Composable private fun GlobalDeviceCodeScreen(screen:AppScreen.GlobalDeviceCode)=Column(Modifier.fillMaxSize().padding(64.dp),verticalArrangement=Arrangement.Center,horizontalAlignment=androidx.compose.ui.Alignment.CenterHorizontally){Text("Sign in to VyNode",style=MaterialTheme.typography.headlineLarge);Text("Open ${BuildConfig.CONNECT_BASE_URL}${screen.verificationPath} and enter",color=VyNodeColor.Muted);Text(screen.userCode,style=MaterialTheme.typography.displayLarge,color=VyNodeColor.Accent,modifier=Modifier.padding(24.dp));Text("Waiting for approval…")}

@Composable private fun ServerPickerScreen(screen:AppScreen.ServerPicker,select:(ConnectedServer)->Unit,advanced:()->Unit){Column(Modifier.fillMaxSize().padding(32.dp),verticalArrangement=Arrangement.spacedBy(18.dp)){Text("Linked Servers",style=MaterialTheme.typography.headlineLarge);screen.message?.let{Text(it,color=VyNodeColor.Muted)};screen.servers.forEach{server->OutlinedCard(onClick={select(server)},modifier=Modifier.fillMaxWidth()){Column(Modifier.padding(22.dp)){Text(server.name,style=MaterialTheme.typography.titleLarge);Text(server.relationship.lowercase().replaceFirstChar(Char::uppercase),color=VyNodeColor.Muted)}}};TextButton(onClick=advanced){Text("Advanced · Connect manually")}}}

@Composable private fun MovieScreen(screen: AppScreen.Movie, controller: AppController) {
    BackHandler { controller.returnFromDetail() }
    Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(VyNodeColor.Raised,VyNodeColor.Background),endY=900f))) {
        Column(Modifier.fillMaxWidth(.72f).align(androidx.compose.ui.Alignment.BottomStart).padding(48.dp), verticalArrangement=Arrangement.spacedBy(20.dp)) {
            Text("MOVIE", color=VyNodeColor.Accent, style=MaterialTheme.typography.labelLarge)
            Text(screen.movie.title, style=MaterialTheme.typography.displayLarge)
            Text(listOfNotNull(screen.movie.year.takeIf{it>0}?.toString(),screen.movie.runtimeMinutes.takeIf{it>0}?.let{"$it min"}).joinToString("  ·  "),color=VyNodeColor.Muted)
            Text(screen.movie.overview,style=MaterialTheme.typography.bodyLarge,color=VyNodeColor.Muted)
            Row(horizontalArrangement=Arrangement.spacedBy(12.dp)) {
                val item=ApiHomeItem("MOVIE",screen.movie.id,screen.movie.title,null,screen.movie.artworkId)
                val canResume=screen.progress?.let { !it.watched && it.position >= 30 && it.duration > it.position } == true
                VyNodePrimaryButton(if(canResume) "Resume ${formatPlayerTime((screen.progress!!.position*1000).toLong())}" else "Play") { controller.play(item) }
                if(canResume) VyNodeSecondaryButton("Start Over") { controller.startOver(item) }
                VyNodeSecondaryButton("Back") { controller.returnFromDetail() }
            }
        }
    }
}

@Composable private fun SearchScreen(screen: AppScreen.Search, controller: AppController) {
    var query by remember(screen.query){mutableStateOf(screen.query)}
    val focusManager = LocalFocusManager.current
    val resultRequester = remember { FocusRequester() }
    val movies = screen.results?.movies.orEmpty()
    val shows = screen.results?.shows.orEmpty()
    LaunchedEffect(screen.results) {
        if (movies.isNotEmpty() || shows.isNotEmpty()) resultRequester.requestFocus()
    }
    BackHandler { controller.returnHome() }
    LazyColumn(Modifier.fillMaxSize().padding(40.dp),verticalArrangement=Arrangement.spacedBy(14.dp)) {
        item { Text("Search",style=MaterialTheme.typography.headlineLarge); OutlinedTextField(query,{query=it},singleLine=true,label={Text("Title")},keyboardOptions=KeyboardOptions(imeAction=ImeAction.Search),keyboardActions=KeyboardActions(onSearch={if(query.isNotBlank()){focusManager.clearFocus();controller.search(query)}})); Button(onClick={focusManager.clearFocus();controller.search(query)},enabled=query.isNotBlank()){Text("Search")}}
        itemsIndexed(movies,key={_,movie->movie.id}) { index,movie -> OutlinedCard(onClick={controller.open(ApiHomeItem("MOVIE",movie.id,movie.title,null,movie.artworkId))},modifier=Modifier.fillMaxWidth().then(if(index==0) Modifier.focusRequester(resultRequester) else Modifier)){Text(movie.title,Modifier.padding(20.dp))} }
        itemsIndexed(shows,key={_,show->show.id}) { index,show -> OutlinedCard(onClick={controller.open(ApiHomeItem("SHOW",show.id,show.title,null,null))},modifier=Modifier.fillMaxWidth().then(if(movies.isEmpty()&&index==0) Modifier.focusRequester(resultRequester) else Modifier)){Text(show.title,Modifier.padding(20.dp))} }
        if(screen.results!=null && movies.isEmpty() && shows.isEmpty()) item { Text("No results") }
    }
}

@Composable private fun ShowScreen(screen: AppScreen.Show, play: (ApiHomeItem) -> Unit, startOver: (ApiHomeItem) -> Unit, download: (ApiEpisode) -> Unit, back: () -> Unit, tv: Boolean) {
    BackHandler { back() }
    LazyColumn(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(VyNodeColor.Raised,VyNodeColor.Background),endY=700f)).padding(horizontal=if(tv) 64.dp else 20.dp,vertical=32.dp), verticalArrangement=Arrangement.spacedBy(16.dp)) {
        item { Text("SERIES",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text(screen.show.title, style=if(tv) MaterialTheme.typography.displayLarge else MaterialTheme.typography.headlineLarge); screen.message?.let { Text(it, color=MaterialTheme.colorScheme.primary) };Text("Episodes",style=MaterialTheme.typography.titleLarge,modifier=Modifier.padding(top=20.dp)) }
        items(screen.show.episodes, key={it.id}) { episode ->
            OutlinedCard(Modifier.fillMaxWidth(),shape=VyNodeRadius.medium,colors=CardDefaults.outlinedCardColors(containerColor=VyNodeColor.Surface)) { Row(Modifier.fillMaxWidth().padding(18.dp), horizontalArrangement=Arrangement.SpaceBetween,verticalAlignment=androidx.compose.ui.Alignment.CenterVertically) {
                Column(verticalArrangement=Arrangement.spacedBy(4.dp)) { Text("S${episode.season.toString().padStart(2,'0')}E${episode.number.toString().padStart(2,'0')}",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge); Text(episode.title,style=MaterialTheme.typography.titleLarge) }
                Row(horizontalArrangement=Arrangement.spacedBy(10.dp)) {
                    val item=ApiHomeItem("EPISODE",episode.id,episode.title,null,null)
                    val canResume=episode.progress?.let { !it.watched && it.position >= 30 && it.duration > it.position } == true
                    VyNodePrimaryButton(if(canResume) "Resume ${formatPlayerTime((episode.progress!!.position*1000).toLong())}" else "Play",enabled=episode.available,onClick={play(item)})
                    if(canResume) VyNodeSecondaryButton("Start Over",enabled=episode.available,onClick={startOver(item)})
                    if (!tv) VyNodeSecondaryButton("Download",enabled=episode.available,onClick={download(episode)})
                }
            } }
        }
    }
}

@Composable private fun OfflineScreen(downloads: List<DownloadEntity>, play: (DownloadEntity) -> Unit) = LazyColumn(Modifier.fillMaxSize().padding(32.dp), verticalArrangement=Arrangement.spacedBy(14.dp)) {
    item { Text("Offline", style=MaterialTheme.typography.headlineLarge); Text("Downloaded media on this device") }
    items(downloads, key={it.downloadId}) { item -> OutlinedCard(onClick={play(item)}, modifier=Modifier.fillMaxWidth()) {
        Row(Modifier.padding(24.dp),horizontalArrangement=Arrangement.spacedBy(16.dp),verticalAlignment=androidx.compose.ui.Alignment.CenterVertically) {
            item.localArtwork?.let { path -> remember(path) { android.graphics.BitmapFactory.decodeFile(path) }?.let { bitmap -> Image(bitmap.asImageBitmap(),null,Modifier.size(72.dp)) } }
            Text(item.title, style=MaterialTheme.typography.titleLarge)
        }
    } }
}

@Composable private fun OfflinePlayerScreen(download: DownloadEntity, controller: AppController) {
    val context=androidx.compose.ui.platform.LocalContext.current
    val engine=remember(download.downloadId){VyNodePlayer(context){null}}
    LaunchedEffect(download.downloadId){engine.play(java.io.File(download.localFile!!).toURI().toString(),false); while(isActive){delay(15_000);controller.reportOffline(download,engine.player.currentPosition,engine.player.duration)} }
    DisposableEffect(engine){onDispose{controller.reportOffline(download,engine.player.currentPosition,engine.player.duration,engine.player.duration>0 && engine.player.currentPosition/engine.player.duration.toDouble()>=.9);engine.release()}}
    BackHandler { controller.returnOffline() }
    Box(Modifier.fillMaxSize()) { AndroidView(factory={PlayerView(it).apply{player=engine.player;useController=true}},modifier=Modifier.fillMaxSize()); Text("Offline • ${download.title}",Modifier.padding(20.dp).zIndex(2f)) }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable private fun ConnectScreen(tv: Boolean, connect: (String) -> Unit) {
    var endpoint by remember { mutableStateOf("") }
    val context = androidx.compose.ui.platform.LocalContext.current
    val focusManager = LocalFocusManager.current
    val keyboard = LocalSoftwareKeyboardController.current
    val discovery = remember { VyNodeDiscovery(context) }
    val discovered by discovery.servers.collectAsStateWithLifecycle()
    DisposableEffect(discovery) { discovery.start(); onDispose { discovery.stop() } }
    LaunchedEffect(Unit) { delay(100);focusManager.clearFocus(force=true);keyboard?.hide() }
    Column(Modifier.fillMaxSize().background(Brush.radialGradient(listOf(Color(0x55317767),VyNodeColor.Background),radius=1100f)).padding(if(tv) 64.dp else 24.dp), verticalArrangement=Arrangement.Center,horizontalAlignment=androidx.compose.ui.Alignment.CenterHorizontally) {
      Surface(Modifier.widthIn(max=if(tv) 680.dp else 520.dp),color=VyNodeColor.Surface,shape=VyNodeRadius.large,shadowElevation=16.dp){Column(Modifier.padding(if(tv) 40.dp else 24.dp)){
        Text("VYNODE MEDIA",style=MaterialTheme.typography.labelLarge,color=VyNodeColor.Accent)
        Spacer(Modifier.height(8.dp));Text("Connect manually", style=MaterialTheme.typography.headlineLarge)
        Text("Choose a discovered VyNode server or enter a trusted address.",color=VyNodeColor.Muted)
        Spacer(Modifier.height(20.dp))
        OutlinedTextField(endpoint, { endpoint=it }, label={Text("Server address")}, singleLine=true, modifier=Modifier.fillMaxWidth())
        discovered.forEach { server ->
            val host = if (server.host.contains(':')) "[${server.host}]" else server.host
            TextButton(onClick={ endpoint = "http://$host:${server.port}" }) { Text("${server.name} • $host:${server.port}") }
        }
        Spacer(Modifier.height(16.dp)); VyNodePrimaryButton("Continue",enabled=endpoint.isNotBlank(),onClick={connect(endpoint)})
        Spacer(Modifier.height(10.dp)); Text("Your server identity is verified before saved credentials are sent.")
      }}
    }
}

@Composable private fun ConfirmInsecure(endpoint: String, continueAction: () -> Unit) = Column(Modifier.fillMaxSize().padding(40.dp), verticalArrangement=Arrangement.Center) {
    Text("Insecure local connection", style=MaterialTheme.typography.headlineLarge); Spacer(Modifier.height(16.dp))
    Text("$endpoint uses HTTP. Anyone on this network may be able to observe traffic. Use only on a trusted local network.")
    Spacer(Modifier.height(20.dp)); Button(onClick=continueAction) { Text("Trust this local connection") }
}

@Composable private fun PairScreen(screen: AppScreen.Pair) = Column(Modifier.fillMaxSize().background(Brush.radialGradient(listOf(Color(0x55317767),VyNodeColor.Background),radius=1100f)).padding(40.dp), verticalArrangement=Arrangement.Center,horizontalAlignment=androidx.compose.ui.Alignment.CenterHorizontally) {
    Text("PAIR WITH",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text(screen.server.serverName, style=MaterialTheme.typography.headlineLarge); Spacer(Modifier.height(20.dp)); Text("Approve this device from your VyNode administration screen",color=VyNodeColor.Muted)
    Text(screen.request.code, style=MaterialTheme.typography.displayLarge, color=MaterialTheme.colorScheme.primary,modifier=Modifier.padding(vertical=24.dp))
    Text("Waiting for approval…",color=VyNodeColor.Muted)
}

@Composable private fun MessageScreen(message: String, retry: () -> Unit) = Column(Modifier.fillMaxSize().padding(40.dp), verticalArrangement=Arrangement.Center) {
    Text(message, style=MaterialTheme.typography.headlineSmall); Spacer(Modifier.height(20.dp)); Button(onClick=retry) { Text("Choose Server") }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable fun VyNodeHome(tv: Boolean, rows: List<HomeRow>, play: (ApiHomeItem) -> Unit = {}, focusId:String?=null, search:()->Unit={},servers:()->Unit={},logout:()->Unit={}) {
    val searchRequester=remember{FocusRequester()}
    Scaffold(containerColor=VyNodeColor.Background,topBar={ TopAppBar(colors=TopAppBarDefaults.topAppBarColors(containerColor=VyNodeColor.Background.copy(alpha=.94f)),title={Column{Text("VyNode",style=MaterialTheme.typography.titleLarge);Text("MEDIA",style=MaterialTheme.typography.labelSmall,color=VyNodeColor.Accent)}},actions={VyNodeSecondaryButton("Servers",onClick=servers);VyNodeSecondaryButton("Sign out",onClick=logout);VyNodeSecondaryButton("Search",modifier=Modifier.padding(end=if(tv) 32.dp else 12.dp).focusRequester(searchRequester),onClick=search)}) }) { padding ->
        LazyColumn(Modifier.fillMaxSize().padding(padding).background(Brush.radialGradient(listOf(Color(0x332A7164),Color.Transparent),radius=1300f)).padding(horizontal=if(tv) 64.dp else 16.dp), verticalArrangement=Arrangement.spacedBy(if(tv) 32.dp else 22.dp)) {
            itemsIndexed(rows, key={_,row->row.id}) { rowIndex,row ->
                Column(verticalArrangement=Arrangement.spacedBy(10.dp)) {
                    Text(row.title, style=if(tv) MaterialTheme.typography.headlineMedium else MaterialTheme.typography.titleLarge)
                    Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).focusGroup().padding(vertical=6.dp), horizontalArrangement=Arrangement.spacedBy(if(tv) 18.dp else 12.dp)) {
                        row.cards.forEachIndexed { cardIndex,card -> key(card.id) { MediaCard(card, tv, card.id==focusId || (focusId==null && rowIndex==0 && cardIndex==0),searchRequester) { play(card.item) } } }
                    }
                }
            }
        }
    }
}

@Composable private fun MediaCard(card: HomeCard, tv: Boolean, restoreFocus:Boolean=false, upRequester:FocusRequester, play: () -> Unit) {
    var focused by remember { mutableStateOf(false) }
    val requester=remember{FocusRequester()}
    LaunchedEffect(restoreFocus){if(restoreFocus) runCatching{requester.requestFocus()}}
    OutlinedCard(onClick=play, modifier=Modifier.width(if(tv) 210.dp else 150.dp).height(if(tv) 300.dp else 220.dp).scale(if(focused&&tv) 1.035f else 1f)
        .testTag("media-${card.id}").focusRequester(requester).focusProperties { up=upRequester }.onFocusChanged { focused=it.isFocused }
        .onPreviewKeyEvent { event ->
            when {
                event.key == Key.DirectionUp && event.type == KeyEventType.KeyDown -> { upRequester.requestFocus(); true }
                event.key in listOf(Key.DirectionCenter, Key.Enter, Key.NumPadEnter) && event.type == KeyEventType.KeyUp -> { play(); true }
                else -> false
            }
        }, shape=VyNodeRadius.medium,colors=CardDefaults.outlinedCardColors(containerColor=if(focused) VyNodeColor.Raised else VyNodeColor.Surface),border=if(focused) BorderStroke(4.dp, VyNodeColor.Focus) else BorderStroke(1.dp, VyNodeColor.Border)) {
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color.Transparent,VyNodeColor.Background),startY=140f)).padding(16.dp)) { Text(card.title, style=MaterialTheme.typography.titleMedium,modifier=Modifier.align(androidx.compose.ui.Alignment.BottomStart),maxLines=2) }
    }
}

@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
@Composable private fun PlayerScreen(screen: AppScreen.Playing, controller: AppController) {
    val context = androidx.compose.ui.platform.LocalContext.current
    val engine = remember(screen.session.id) { VyNodePlayer(context, controller::accessToken) }
    var position by remember { mutableLongStateOf(0L) }
    var duration by remember { mutableLongStateOf(0L) }
    var isPlaying by remember { mutableStateOf(false) }
    var playWhenReady by remember { mutableStateOf(false) }
    var subtitleText by remember(screen.session.id) { mutableStateOf("") }
    var menu by remember { mutableStateOf<String?>(null) }
    var autoplayCanceled by remember(screen.session.id) { mutableStateOf(false) }
    val markerRequester = remember { FocusRequester() }
    val menuRequester = remember { FocusRequester() }
    val upNextRequester = remember { FocusRequester() }
    val rewindRequester = remember { FocusRequester() }
    val playerRequester = remember { FocusRequester() }
    val forwardRequester = remember { FocusRequester() }
    val qualityRequester = remember { FocusRequester() }
    val audioRequester = remember { FocusRequester() }
    val subtitleRequester = remember { FocusRequester() }
    LaunchedEffect(screen.session.id) {
        val path = screen.session.hlsUrl ?: screen.session.mediaUrl ?: return@LaunchedEffect
        val subtitlePath = screen.session.subtitleUrl
        engine.play(controller.mediaUrl(path), screen.session.hlsUrl != null, (screen.session.resumePosition * 1000).toLong(),subtitlePath?.let(controller::mediaUrl))
        var reports=0
        while (isActive) { delay(250); position=engine.player.currentPosition;duration=engine.player.duration.coerceAtLeast(0);isPlaying=engine.player.isPlaying;playWhenReady=engine.player.playWhenReady;if(++reports%60==0) controller.reportPlayback(screen.session.id, if (engine.player.isPlaying) "PLAYING" else "PAUSED", position, engine.player.duration)
            val remaining=engine.player.duration-position
            if(!autoplayCanceled && screen.session.next!=null && engine.player.duration>0 && remaining in 0..500){ val n=screen.session.next; controller.changePlayback(screen.session,ApiHomeItem("EPISODE",n.id,n.title,null,null),positionMs=position,durationMs=duration,startMs=0); return@LaunchedEffect }
        }
    }
    LaunchedEffect(screen.session.id) { delay(350);runCatching { playerRequester.requestFocus() } }
    DisposableEffect(engine.player) {
        val listener = object : Player.Listener {
            override fun onCues(cueGroup: CueGroup) {
                subtitleText = cueGroup.cues.mapNotNull { it.text?.toString() }.joinToString("\n")
            }
        }
        engine.player.addListener(listener)
        onDispose { engine.player.removeListener(listener) }
    }
    DisposableEffect(engine) { onDispose { controller.reportPlayback(screen.session.id, "STOPPED", engine.player.currentPosition, engine.player.duration); engine.release() } }
    BackHandler { if(menu!=null) menu=null else controller.stopPlayback(screen.session.id,position,duration) }
    val activeMarker=screen.session.markers.firstOrNull { position/1000.0 >= it.start && position/1000.0 < it.end }
    val upNextRemaining=(duration-position)/1000
    val showUpNext=!autoplayCanceled && screen.session.next!=null && upNextRemaining in 0..10
    val contextualActionRequester=when {
        showUpNext -> upNextRequester
        activeMarker != null -> markerRequester
        else -> FocusRequester.Cancel
    }
    Box(Modifier.fillMaxSize().background(Color.Black)) {
        AndroidView(factory={ PlayerView(it).apply {
            player=engine.player
            useController=false
            subtitleView?.setBottomPaddingFraction(0.34f)
        } }, modifier=Modifier.fillMaxSize())
        Box(Modifier.fillMaxWidth().height(160.dp).background(Brush.verticalGradient(listOf(Color.Black.copy(alpha=.82f),Color.Transparent))).zIndex(2f))
        Surface(Modifier.padding(24.dp).zIndex(3f),color=VyNodeColor.Overlay,shape=VyNodeRadius.pill) { Text("${screen.item.title}  ·  ${screen.session.mode.replace('_',' ')}", Modifier.padding(horizontal=16.dp,vertical=10.dp),style=MaterialTheme.typography.labelLarge) }
        Column(Modifier.padding(16.dp).align(androidx.compose.ui.Alignment.TopEnd).zIndex(3f),horizontalAlignment=androidx.compose.ui.Alignment.End) {
            VyNodeSecondaryButton("Back",onClick={controller.stopPlayback(screen.session.id,position,duration)})
        }
        Surface(Modifier.align(androidx.compose.ui.Alignment.BottomCenter).fillMaxWidth().padding(24.dp).zIndex(3f),color=VyNodeColor.Overlay,shape=VyNodeRadius.large,shadowElevation=14.dp) {
            Column(Modifier.padding(horizontal=24.dp,vertical=16.dp),verticalArrangement=Arrangement.spacedBy(12.dp)) {
                Row(Modifier.fillMaxWidth(),horizontalArrangement=Arrangement.SpaceBetween){Text(formatPlayerTime(position),color=VyNodeColor.Text);Text(formatPlayerTime(duration),color=VyNodeColor.Muted)}
                LinearProgressIndicator(progress={if(duration>0) position.toFloat()/duration else 0f},modifier=Modifier.fillMaxWidth().height(4.dp).clip(VyNodeRadius.pill),color=VyNodeColor.Accent,trackColor=VyNodeColor.Border)
                Row(Modifier.fillMaxWidth(),horizontalArrangement=Arrangement.Center,verticalAlignment=androidx.compose.ui.Alignment.CenterVertically) {
                    val rewind={engine.player.seekTo((position-10_000).coerceAtLeast(0))}
                    VyNodeSecondaryButton("−10s",modifier=Modifier.focusRequester(rewindRequester).focusProperties { right=playerRequester }.remoteActivate(rewind),onClick=rewind)
                    Spacer(Modifier.width(12.dp))
                    val toggle={engine.toggle()}
                    VyNodePrimaryButton(if(playWhenReady) "Pause" else "Play",modifier=Modifier.focusRequester(playerRequester).focusProperties { left=rewindRequester;right=forwardRequester }.remoteActivate(toggle),onClick=toggle)
                    Spacer(Modifier.width(12.dp))
                    val firstMenuRequester=when { screen.session.qualities.isNotEmpty()->qualityRequester;screen.session.audioTracks.isNotEmpty()->audioRequester;else->subtitleRequester }
                    val forward={engine.player.seekTo((position+15_000).coerceAtMost(duration.coerceAtLeast(position+15_000)))}
                    VyNodeSecondaryButton("+15s",modifier=Modifier.focusRequester(forwardRequester).focusProperties { left=playerRequester;right=firstMenuRequester }.remoteActivate(forward),onClick=forward)
                    Spacer(Modifier.width(24.dp))
                    if(screen.session.qualities.isNotEmpty()) VyNodeSecondaryButton("Quality",modifier=Modifier.focusRequester(qualityRequester).focusProperties { left=forwardRequester;right=if(screen.session.audioTracks.isNotEmpty()) audioRequester else subtitleRequester },onClick={menu="quality"})
                    if(screen.session.audioTracks.isNotEmpty()){Spacer(Modifier.width(8.dp));VyNodeSecondaryButton("Audio",modifier=Modifier.focusRequester(audioRequester).focusProperties { left=if(screen.session.qualities.isNotEmpty()) qualityRequester else forwardRequester;right=subtitleRequester },onClick={menu="audio"})}
                    if(screen.session.subtitleTracks.isNotEmpty()){Spacer(Modifier.width(8.dp));VyNodeSecondaryButton("Subtitles",modifier=Modifier.focusRequester(subtitleRequester).focusProperties { left=if(screen.session.audioTracks.isNotEmpty()) audioRequester else if(screen.session.qualities.isNotEmpty()) qualityRequester else forwardRequester;up=contextualActionRequester },onClick={menu="subtitle"})}
                }
            }
        }
        if (subtitleText.isNotBlank()) Surface(
            Modifier.align(androidx.compose.ui.Alignment.BottomCenter).padding(start=96.dp,end=96.dp,bottom=330.dp).zIndex(4f),
            color=Color.Black.copy(alpha=.82f),
            shape=VyNodeRadius.medium
        ) {
            Text(subtitleText,Modifier.padding(horizontal=20.dp,vertical=10.dp),style=MaterialTheme.typography.titleLarge,color=Color.White)
        }
        activeMarker?.let { marker -> val skip={engine.seekToMarkerEnd(marker.end)};VyNodePrimaryButton(if(marker.type=="INTRO") "Skip Intro" else "Skip Credits",modifier=Modifier.align(androidx.compose.ui.Alignment.BottomEnd).padding(end=48.dp,bottom=176.dp).zIndex(4f).focusRequester(markerRequester).focusProperties { down=subtitleRequester }.remoteActivate(skip),onClick=skip) }
        screen.session.next?.let { next ->
            if(showUpNext) Surface(Modifier.align(androidx.compose.ui.Alignment.BottomEnd).padding(end=48.dp,bottom=176.dp).widthIn(max=440.dp).zIndex(4f),color=VyNodeColor.Overlay,shape=VyNodeRadius.large,shadowElevation=16.dp){
                Column(Modifier.padding(20.dp),verticalArrangement=Arrangement.spacedBy(8.dp)){
                    Text("Up Next: ${next.title} in $upNextRemaining")
                    Row(horizontalArrangement=Arrangement.spacedBy(8.dp)){
                        val playNow={ controller.changePlayback(screen.session,ApiHomeItem("EPISODE",next.id,next.title,null,null),positionMs=position,durationMs=duration,startMs=0) }
                        VyNodePrimaryButton("Play Now",modifier=Modifier.remoteActivate(playNow),onClick=playNow)
                        val cancel={ autoplayCanceled=true }
                        VyNodeSecondaryButton("Cancel",modifier=Modifier.focusRequester(upNextRequester).remoteActivate(cancel),onClick=cancel)
                    }
                }
            }
        }
        LaunchedEffect(menu) { if (menu != null) { delay(80);runCatching { menuRequester.requestFocus() } } }
        menu?.let { kind ->
            Surface(Modifier.align(androidx.compose.ui.Alignment.CenterEnd).padding(48.dp).widthIn(min=260.dp,max=380.dp).zIndex(5f),color=VyNodeColor.Overlay,shape=VyNodeRadius.large,shadowElevation=18.dp) {
                Column(Modifier.padding(20.dp), verticalArrangement=Arrangement.spacedBy(8.dp)) {
                    Text(kind.replaceFirstChar { it.uppercase() })
                    if (kind == "quality") screen.session.qualities.forEachIndexed { index, quality ->
                        val select = { menu=null; controller.changePlayback(screen.session,screen.item,position,duration,qualityId=quality.id) }
                        Button(onClick=select, modifier=(if(index==0) Modifier.focusRequester(menuRequester) else Modifier).remoteActivate(select)) { Text(quality.label) }
                    }
                    if (kind == "audio") screen.session.audioTracks.forEachIndexed { index, track ->
                        val select = { menu=null; controller.changePlayback(screen.session,screen.item,position,duration,audioId=track.id) }
                        Button(onClick=select, modifier=(if(index==0) Modifier.focusRequester(menuRequester) else Modifier).remoteActivate(select)) { Text(track.semanticLabel) }
                    }
                    if (kind == "subtitle") {
                        val disable = { menu=null; controller.changePlayback(screen.session,screen.item,position,duration,subtitleId="") }
                        Button(onClick=disable, modifier=Modifier.focusRequester(menuRequester).remoteActivate(disable)) { Text("Off") }
                        screen.session.subtitleTracks.forEach { track ->
                            val select = { menu=null; controller.changePlayback(screen.session,screen.item,position,duration,subtitleId=track.id) }
                            Button(onClick=select, modifier=Modifier.remoteActivate(select)) { Text(track.semanticLabel) }
                        }
                    }
                }
            }
        }
    }
}
