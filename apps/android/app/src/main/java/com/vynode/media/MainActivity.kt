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
        is AppScreen.ServerPicker -> ServerPickerScreen(screen,controller::selectGlobalServer,controller::chooseGlobalServer,controller::advancedConnect,tv)
        is AppScreen.ServerInfo -> ServerInfoScreen(screen,controller::returnHome,tv)
        AppScreen.Connect -> ConnectScreen(tv, controller::connect)
        is AppScreen.ConfirmInsecure -> ConfirmInsecure(screen.endpoint) { controller.connect(screen.endpoint, true) }
        is AppScreen.Pair -> PairScreen(screen)
        is AppScreen.Home -> if(tv) TvShell("HOME",controller){content,rail->VyNodeHome(true,screen.rows.map { row -> HomeRow(row.id,row.title,row.items.map(::HomeCard)) },controller::open,screen.focusId,controller::openSearch,embeddedTvShell=true,initialRequester=content,railRequester=rail)} else VyNodeHome(false, screen.rows.map { row -> HomeRow(row.id, row.title, row.items.map(::HomeCard)) }, controller::open, screen.focusId, controller::openSearch,controller::openDownloads,controller::openAccount)
        is AppScreen.Movie -> MovieScreen(screen, controller, tv)
        is AppScreen.Playing -> PlayerScreen(screen, controller, tv)
        is AppScreen.Show -> ShowScreen(screen, controller::playFromShow, controller::startOver, controller::download, controller::returnFromDetail, tv)
        is AppScreen.Search -> if(tv) TvShell("SEARCH",controller){content,rail->SearchScreen(screen,controller,true,content,rail)} else SearchScreen(screen, controller, false)
        is AppScreen.Library -> if(tv) TvShell(screen.kind,controller){content,rail->TvLibraryScreen(screen,controller,content,rail)} else TvLibraryScreen(screen,controller)
        is AppScreen.Offline -> if(tv) TvShell("DOWNLOADS",controller){content,rail->OfflineScreen(screen.downloads,controller::playOffline,content,rail,controller::returnHome)} else OfflineScreen(screen.downloads, controller::playOffline,back=controller::returnHome)
        is AppScreen.Account -> if(tv) TvShell("ACCOUNT",controller){content,rail->TvAccountScreen(screen,controller,true,content,rail)} else TvAccountScreen(screen,controller,false,back=controller::returnHome)
        is AppScreen.LocalPlaying -> OfflinePlayerScreen(screen.download, controller)
        is AppScreen.Error -> MessageScreen(screen.message, controller::retry)
        is AppScreen.IdentityMismatch -> MessageScreen("This address now identifies as ${screen.received.serverName}. Credentials were not sent.", controller::retry)
    }
}

@Composable private fun GlobalSignInScreen(tv:Boolean,login:(String,String)->Unit,register:(String,String,String)->Unit,tvCode:()->Unit,advanced:()->Unit){var username by remember{mutableStateOf("")};var displayName by remember{mutableStateOf("")};var password by remember{mutableStateOf("")};var creating by remember{mutableStateOf(false)};val signInRequester=remember{FocusRequester()};LaunchedEffect(tv){if(tv)runCatching{signInRequester.requestFocus()}};Column(Modifier.fillMaxSize().background(VyNodeColor.Background).padding(if(tv)64.dp else 24.dp),verticalArrangement=Arrangement.Center,horizontalAlignment=androidx.compose.ui.Alignment.CenterHorizontally){Surface(Modifier.widthIn(max=620.dp),color=VyNodeColor.Surface,shape=VyNodeRadius.large){Column(Modifier.padding(if(tv)40.dp else 24.dp),verticalArrangement=Arrangement.spacedBy(16.dp)){Text("VYNODE MEDIA",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text("Welcome to VyNode",style=MaterialTheme.typography.headlineLarge);Text("Sign in once to find the servers linked to your account.",color=VyNodeColor.Muted);if(tv){VyNodePrimaryButton("Sign In to VyNode",modifier=Modifier.focusRequester(signInRequester),onClick=tvCode)}else{OutlinedTextField(username,{username=it},label={Text("Username")},singleLine=true,modifier=Modifier.fillMaxWidth());if(creating)OutlinedTextField(displayName,{displayName=it},label={Text("Display name")},singleLine=true,modifier=Modifier.fillMaxWidth());OutlinedTextField(password,{password=it},label={Text("Password")},singleLine=true,visualTransformation=androidx.compose.ui.text.input.PasswordVisualTransformation(),modifier=Modifier.fillMaxWidth());VyNodePrimaryButton(if(creating)"Create VyNode Account" else "Sign In to VyNode",enabled=username.isNotBlank()&&password.isNotBlank()&&(!creating||displayName.isNotBlank())){if(creating)register(username,displayName,password)else login(username,password)};TextButton(onClick={creating=!creating}){Text(if(creating)"Already have an account? Sign in" else "Create Account")}};TextButton(onClick=advanced){Text("Advanced · Connect manually")}}}}}

@Composable private fun GlobalDeviceCodeScreen(screen:AppScreen.GlobalDeviceCode)=Column(Modifier.fillMaxSize().background(VyNodeColor.Background).padding(64.dp),verticalArrangement=Arrangement.Center,horizontalAlignment=androidx.compose.ui.Alignment.CenterHorizontally){Text("SIGN IN TO VYNODE",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text("Use a phone or computer",style=MaterialTheme.typography.displayLarge);Text("Open ${BuildConfig.CONNECT_BASE_URL}${screen.verificationPath} and enter this code",color=VyNodeColor.Muted,style=MaterialTheme.typography.titleLarge,modifier=Modifier.padding(top=14.dp));Text(screen.userCode,style=MaterialTheme.typography.displayLarge,color=VyNodeColor.Text,modifier=Modifier.padding(vertical=30.dp));Row(verticalAlignment=androidx.compose.ui.Alignment.CenterVertically,horizontalArrangement=Arrangement.spacedBy(14.dp)){CircularProgressIndicator(Modifier.size(26.dp),strokeWidth=3.dp);Text("Waiting for approval…",color=VyNodeColor.Muted)}}

@Composable private fun ServerPickerScreen(screen:AppScreen.ServerPicker,select:(ConnectedServer)->Unit,refresh:()->Unit,advanced:()->Unit,tv:Boolean){val requester=remember{FocusRequester()};val focusedIndex=screen.servers.indexOfFirst{it.id==screen.currentServerId}.takeIf{it>=0}?:0;LaunchedEffect(screen.servers,screen.currentServerId){runCatching{requester.requestFocus()}};Column(Modifier.fillMaxSize().background(VyNodeColor.Background).padding(if(tv)64.dp else 32.dp),verticalArrangement=Arrangement.spacedBy(18.dp)){Text("YOUR VYNODE ACCOUNT",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text(if(screen.servers.isEmpty())"No linked servers" else "Choose a server",style=if(tv)MaterialTheme.typography.displayLarge else MaterialTheme.typography.headlineLarge);screen.message?.let{Text(it,color=VyNodeColor.Muted,style=MaterialTheme.typography.titleMedium)};screen.servers.forEachIndexed{index,server->var focused by remember{mutableStateOf(false)};val current=server.id==screen.currentServerId;OutlinedCard(onClick={select(server)},modifier=Modifier.widthIn(max=if(tv)720.dp else 560.dp).fillMaxWidth().then(if(index==focusedIndex)Modifier.focusRequester(requester)else Modifier).onFocusChanged{focused=it.isFocused},border=BorderStroke(if(focused)4.dp else 1.dp,if(focused)VyNodeColor.Focus else VyNodeColor.Border),colors=CardDefaults.outlinedCardColors(containerColor=if(focused)VyNodeColor.Raised else VyNodeColor.Surface)){Column(Modifier.padding(if(tv)26.dp else 22.dp)){Text(server.name,style=MaterialTheme.typography.titleLarge);Text(listOf(server.relationship.lowercase().replaceFirstChar(Char::uppercase)).plus(if(current)"Current" else "").filter{it.isNotBlank()}.joinToString(" · "),color=VyNodeColor.Muted)}}};if(screen.servers.isEmpty()){Text("Link or accept an invitation from your phone or computer, then return here.",color=VyNodeColor.Muted);VyNodePrimaryButton("Refresh linked servers",modifier=Modifier.focusRequester(requester),onClick=refresh)};TextButton(onClick=advanced){Text("Advanced · Connect manually")}}}
@Composable private fun ServerInfoScreen(screen:AppScreen.ServerInfo,back:()->Unit,tv:Boolean){BackHandler{back()};Column(Modifier.fillMaxSize().background(VyNodeColor.Background).padding(64.dp),verticalArrangement=Arrangement.Center){Text("CURRENT SERVER",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text(screen.serverName,style=MaterialTheme.typography.displayLarge);Text(if(tv)"This TV is paired directly with this server." else "This device is paired directly with this server.",color=VyNodeColor.Muted,style=MaterialTheme.typography.titleLarge,modifier=Modifier.padding(vertical=16.dp));VyNodePrimaryButton("Back to Home",onClick=back)}}

@Composable private fun MovieScreen(screen: AppScreen.Movie, controller: AppController, tv: Boolean) {
    val playRequester=remember{FocusRequester()};LaunchedEffect(screen.movie.id){if(tv)runCatching{playRequester.requestFocus()}}
    BackHandler { controller.returnFromDetail() }
    Box(Modifier.fillMaxSize().background(VyNodeColor.Background)) {
        val detailModifier=if(tv) Modifier.fillMaxWidth(.72f).align(androidx.compose.ui.Alignment.BottomStart).padding(64.dp) else Modifier.fillMaxWidth().align(androidx.compose.ui.Alignment.TopStart).padding(horizontal=20.dp)
        Column(detailModifier, verticalArrangement=Arrangement.spacedBy(if(tv) 20.dp else 14.dp)) {
            if(!tv) Spacer(Modifier.height(220.dp))
            Text("MOVIE", color=VyNodeColor.Accent, style=MaterialTheme.typography.labelLarge)
            Text(screen.movie.title, style=if(tv) MaterialTheme.typography.displayLarge else MaterialTheme.typography.headlineLarge,maxLines=3)
            Text(listOfNotNull(screen.movie.year.takeIf{it>0}?.toString(),screen.movie.runtimeMinutes.takeIf{it>0}?.let{"$it min"}).joinToString("  ·  "),color=VyNodeColor.Muted)
            Text(screen.movie.overview,style=MaterialTheme.typography.bodyLarge,color=VyNodeColor.Muted)
            Row(Modifier.horizontalScroll(rememberScrollState()),horizontalArrangement=Arrangement.spacedBy(10.dp)) {
                val item=ApiHomeItem("MOVIE",screen.movie.id,screen.movie.title,null,screen.movie.artworkId)
                val canResume=screen.progress?.let { !it.watched && it.position >= 30 && it.duration > it.position } == true
                VyNodePrimaryButton(if(canResume) "Resume ${formatPlayerTime((screen.progress!!.position*1000).toLong())}" else "Play",modifier=Modifier.focusRequester(playRequester)) { controller.play(item) }
                if(canResume) VyNodeSecondaryButton("Start Over") { controller.startOver(item) }
                if(tv) VyNodeSecondaryButton("Back") { controller.returnFromDetail() }
            }
        }
    }
}

@Composable private fun SearchScreen(screen: AppScreen.Search, controller: AppController, tv:Boolean, initialRequester:FocusRequester?=null, railRequester:FocusRequester?=null) {
    var query by remember(screen.query){mutableStateOf(screen.query)}
    val focusManager = LocalFocusManager.current
    val resultRequester = remember { FocusRequester() }
    val movies = screen.results?.movies.orEmpty()
    val shows = screen.results?.shows.orEmpty()
    LaunchedEffect(screen.results) {
        if (movies.isNotEmpty() || shows.isNotEmpty()) resultRequester.requestFocus() else initialRequester?.requestFocus()
    }
    BackHandler { controller.returnHome() }
    if(tv){Column(Modifier.fillMaxSize().padding(horizontal=48.dp,vertical=36.dp),verticalArrangement=Arrangement.spacedBy(18.dp)){Text("SEARCH YOUR MEDIA",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text("Search",style=MaterialTheme.typography.displayLarge);Row(Modifier.fillMaxWidth(),horizontalArrangement=Arrangement.spacedBy(12.dp),verticalAlignment=androidx.compose.ui.Alignment.CenterVertically){OutlinedTextField(query,{query=it},singleLine=true,placeholder={Text("Movies and shows")},keyboardOptions=KeyboardOptions(imeAction=ImeAction.Search),keyboardActions=KeyboardActions(onSearch={if(query.isNotBlank()){focusManager.clearFocus();controller.search(query)}}),modifier=Modifier.weight(1f).then(if(initialRequester!=null)Modifier.focusRequester(initialRequester).focusProperties{left=railRequester?:FocusRequester.Default}else Modifier));VyNodePrimaryButton("Search",enabled=query.isNotBlank()){focusManager.clearFocus();controller.search(query)}};LazyColumn(Modifier.fillMaxWidth().weight(1f),verticalArrangement=Arrangement.spacedBy(16.dp)){if(movies.isNotEmpty())item{Text("Movies",style=MaterialTheme.typography.titleLarge);LazyRow(horizontalArrangement=Arrangement.spacedBy(18.dp)){itemsIndexed(movies,key={_,it->it.id}){index,movie->SearchResultCard(movie.title,true,Modifier.then(if(index==0)Modifier.focusRequester(resultRequester)else Modifier)){controller.open(ApiHomeItem("MOVIE",movie.id,movie.title,null,movie.artworkId))}}}};if(shows.isNotEmpty())item{Text("Shows",style=MaterialTheme.typography.titleLarge);LazyRow(horizontalArrangement=Arrangement.spacedBy(18.dp)){itemsIndexed(shows,key={_,it->it.id}){index,show->SearchResultCard(show.title,true,Modifier.then(if(movies.isEmpty()&&index==0)Modifier.focusRequester(resultRequester)else Modifier)){controller.open(ApiHomeItem("SHOW",show.id,show.title,null,null))}}}};if(screen.results!=null&&movies.isEmpty()&&shows.isEmpty())item{Text("No results for “${screen.query}”",style=MaterialTheme.typography.titleLarge);Text("Check the title or try a shorter search.",color=VyNodeColor.Muted)}}};return}
    LazyColumn(Modifier.fillMaxSize().padding(horizontal=if(tv)64.dp else 20.dp,vertical=if(tv)48.dp else 24.dp),verticalArrangement=Arrangement.spacedBy(if(tv)24.dp else 16.dp)) {
        item { Text("SEARCH YOUR MEDIA",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text("Search",style=if(tv)MaterialTheme.typography.displayLarge else MaterialTheme.typography.headlineLarge);Row(Modifier.fillMaxWidth(),horizontalArrangement=Arrangement.spacedBy(10.dp),verticalAlignment=androidx.compose.ui.Alignment.CenterVertically){ OutlinedTextField(query,{query=it},singleLine=true,placeholder={Text("Movies and shows")},keyboardOptions=KeyboardOptions(imeAction=ImeAction.Search),keyboardActions=KeyboardActions(onSearch={if(query.isNotBlank()){focusManager.clearFocus();controller.search(query)}}),modifier=Modifier.weight(1f).then(if(initialRequester!=null)Modifier.focusRequester(initialRequester).focusProperties{left=railRequester?:FocusRequester.Default}else Modifier)); VyNodePrimaryButton("Search",enabled=query.isNotBlank()){focusManager.clearFocus();controller.search(query)}}}
        if(movies.isNotEmpty()) item { Text("Movies",style=MaterialTheme.typography.titleLarge);LazyRow(horizontalArrangement=Arrangement.spacedBy(if(tv)18.dp else 12.dp)){itemsIndexed(movies,key={_,it->it.id}){index,movie->SearchResultCard(movie.title,tv,Modifier.then(if(index==0)Modifier.focusRequester(resultRequester)else Modifier)){controller.open(ApiHomeItem("MOVIE",movie.id,movie.title,null,movie.artworkId))}}} }
        if(shows.isNotEmpty()) item { Text("Shows",style=MaterialTheme.typography.titleLarge);LazyRow(horizontalArrangement=Arrangement.spacedBy(if(tv)18.dp else 12.dp)){itemsIndexed(shows,key={_,it->it.id}){index,show->SearchResultCard(show.title,tv,Modifier.then(if(movies.isEmpty()&&index==0)Modifier.focusRequester(resultRequester)else Modifier)){controller.open(ApiHomeItem("SHOW",show.id,show.title,null,null))}}} }
        if(screen.results!=null && movies.isEmpty() && shows.isEmpty()) item { Column(Modifier.padding(vertical=48.dp),verticalArrangement=Arrangement.spacedBy(6.dp)){Text("No results for “${screen.query}”",style=MaterialTheme.typography.titleLarge);Text("Check the title or try a shorter search.",color=VyNodeColor.Muted)} }
    }
}

@Composable private fun TvShell(selected:String,controller:AppController,content:@Composable (FocusRequester,FocusRequester)->Unit){
    val railRequester=remember{FocusRequester()};val contentRequester=remember{FocusRequester()};var railFocused by remember{mutableStateOf(false)}
    LaunchedEffect(selected){delay(140);runCatching{contentRequester.requestFocus()}}
    BackHandler { if(railFocused) runCatching{contentRequester.requestFocus()} else runCatching{railRequester.requestFocus()} }
    Box(Modifier.fillMaxSize().background(VyNodeColor.Background)){
        Box(Modifier.fillMaxSize().padding(start=92.dp)){content(contentRequester,railRequester)}
        if(railFocused)Box(Modifier.fillMaxSize().background(Color.Black.copy(alpha=.46f)).zIndex(15f))
        Surface(Modifier.fillMaxHeight().width(if(railFocused)220.dp else 84.dp).zIndex(20f).onFocusChanged{railFocused=it.hasFocus},color=VyNodeColor.Background.copy(alpha=.98f),shadowElevation=if(railFocused)20.dp else 0.dp){
            Column(Modifier.fillMaxSize().padding(horizontal=12.dp,vertical=28.dp).focusGroup(),verticalArrangement=Arrangement.spacedBy(7.dp)){
                Row(Modifier.height(44.dp).padding(horizontal=10.dp),verticalAlignment=androidx.compose.ui.Alignment.CenterVertically){Text("V",color=VyNodeColor.Accent,style=MaterialTheme.typography.titleLarge);if(railFocused){Spacer(Modifier.width(12.dp));Text("VyNode",style=MaterialTheme.typography.titleLarge)}}
                Spacer(Modifier.height(18.dp))
                TvRailButton("HOME","⌂","Home",selected,railFocused,railRequester,contentRequester,controller::returnHome)
                TvRailButton("MOVIES","▣","Movies",selected,railFocused,null,contentRequester,{controller.openLibrary("MOVIES")})
                TvRailButton("SHOWS","▤","Shows",selected,railFocused,null,contentRequester,{controller.openLibrary("SHOWS")})
                TvRailButton("SEARCH","⌕","Search",selected,railFocused,null,contentRequester,controller::openSearch)
                TvRailButton("DOWNLOADS","↓","Downloads",selected,railFocused,null,contentRequester,controller::openDownloads)
                Spacer(Modifier.weight(1f))
                TvRailButton("SERVER","◈","Server",selected,railFocused,null,contentRequester,controller::chooseGlobalServer)
                TvRailButton("ACCOUNT","●","Account",selected,railFocused,null,contentRequester,controller::openAccount)
            }
        }
    }
}

@Composable private fun TvRailButton(id:String,glyph:String,label:String,selected:String,expanded:Boolean,requester:FocusRequester?,contentRequester:FocusRequester,onClick:()->Unit){
    var focused by remember{mutableStateOf(false)}
    OutlinedButton(onClick=onClick,modifier=Modifier.fillMaxWidth().height(50.dp).then(if(requester!=null)Modifier.focusRequester(requester)else Modifier).focusProperties{right=contentRequester}.onFocusChanged{focused=it.isFocused},shape=VyNodeRadius.medium,border=if(focused)BorderStroke(3.dp,VyNodeColor.Focus)else BorderStroke(1.dp,Color.Transparent),colors=ButtonDefaults.outlinedButtonColors(containerColor=when{focused->VyNodeColor.Raised;id==selected->VyNodeColor.Selected;else->Color.Transparent},contentColor=when{id==selected->VyNodeColor.AccentStrong;else->VyNodeColor.Text})){Text(glyph,style=MaterialTheme.typography.titleLarge);if(expanded){Spacer(Modifier.width(12.dp));Text(label,Modifier.weight(1f),style=MaterialTheme.typography.titleMedium)}}
}

@Composable private fun SearchResultCard(title:String,tv:Boolean,modifier:Modifier=Modifier,onClick:()->Unit){var focused by remember{mutableStateOf(false)};OutlinedCard(onClick=onClick,modifier=modifier.width(if(tv)210.dp else 150.dp).height(if(tv)300.dp else 220.dp).scale(if(tv&&focused)1.035f else 1f).onFocusChanged{focused=it.isFocused},shape=VyNodeRadius.medium,colors=CardDefaults.outlinedCardColors(containerColor=if(focused)VyNodeColor.Raised else VyNodeColor.Surface),border=BorderStroke(if(focused)4.dp else 1.dp,if(focused)VyNodeColor.Focus else VyNodeColor.Border)){Box(Modifier.fillMaxSize().background(if(focused)VyNodeColor.Raised else VyNodeColor.Surface)){Text(title,Modifier.align(androidx.compose.ui.Alignment.BottomStart).padding(16.dp),style=MaterialTheme.typography.titleMedium,maxLines=2)}}}

@Composable private fun TvLibraryScreen(screen:AppScreen.Library,controller:AppController,initialRequester:FocusRequester?=null,railRequester:FocusRequester?=null){
    val values=if(screen.kind=="MOVIES")screen.results.movies.map{ApiHomeItem("MOVIE",it.id,it.title,null,it.artworkId)}else screen.results.shows.map{ApiHomeItem("SHOW",it.id,it.title,null,null)}
    BackHandler{controller.returnHome()}
    LazyColumn(Modifier.fillMaxSize().padding(horizontal=48.dp,vertical=40.dp),verticalArrangement=Arrangement.spacedBy(22.dp)){item{Text("YOUR LIBRARY",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text(if(screen.kind=="MOVIES")"Movies" else "Shows",style=MaterialTheme.typography.displayLarge)};if(values.isEmpty())item{Text("No ${screen.kind.lowercase()} yet",color=VyNodeColor.Muted,style=MaterialTheme.typography.titleLarge)}else item{Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).focusGroup().padding(8.dp),horizontalArrangement=Arrangement.spacedBy(20.dp)){values.forEachIndexed{index,item->SearchResultCard(item.title,true,Modifier.then(if(index==0&&initialRequester!=null)Modifier.focusRequester(initialRequester).focusProperties{left=railRequester?:FocusRequester.Default}else Modifier)){controller.open(item)}}}}}
}

@Composable private fun TvAccountScreen(screen:AppScreen.Account,controller:AppController,tv:Boolean,initialRequester:FocusRequester?=null,railRequester:FocusRequester?=null,back:(()->Unit)?=null){if(back!=null)BackHandler{back()};Column(Modifier.fillMaxSize().padding(if(tv)64.dp else 24.dp),verticalArrangement=Arrangement.Center){Text("ACCOUNT",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text(screen.accountName,style=if(tv)MaterialTheme.typography.displayLarge else MaterialTheme.typography.headlineLarge,maxLines=2);Text("Connected to ${screen.serverName}",color=VyNodeColor.Muted,style=if(tv)MaterialTheme.typography.titleLarge else MaterialTheme.typography.titleMedium,modifier=Modifier.padding(vertical=18.dp));Row(Modifier.horizontalScroll(rememberScrollState()),horizontalArrangement=Arrangement.spacedBy(14.dp)){VyNodePrimaryButton("Switch Server",modifier=Modifier.then(if(initialRequester!=null)Modifier.focusRequester(initialRequester).focusProperties{left=railRequester?:FocusRequester.Default}else Modifier),onClick=controller::chooseGlobalServer);VyNodeSecondaryButton("Sign Out",onClick=controller::globalLogout)}}}

@Composable private fun ShowScreen(screen: AppScreen.Show, play: (ApiHomeItem) -> Unit, startOver: (ApiHomeItem) -> Unit, download: (ApiEpisode) -> Unit, back: () -> Unit, tv: Boolean) {
    if(tv){TvShowScreen(screen,play,back);return}
    BackHandler { back() }
    val seasons=remember(screen.show.episodes){screen.show.episodes.map{it.season}.distinct().sorted()}
    var selectedSeason by remember(screen.show.id){mutableStateOf(seasons.firstOrNull()?:0)}
    val episodes=screen.show.episodes.filter{it.season==selectedSeason}
    val firstPlayable=episodes.firstOrNull{it.available}
    LazyColumn(Modifier.fillMaxSize().background(VyNodeColor.Background).padding(horizontal=20.dp,vertical=32.dp),verticalArrangement=Arrangement.spacedBy(16.dp)){
        item{
            Text("SERIES",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge)
            Text(screen.show.title,style=MaterialTheme.typography.headlineLarge,maxLines=2)
            screen.show.overview?.takeIf{it.isNotBlank()}?.let{Text(it,color=VyNodeColor.Muted,maxLines=4,modifier=Modifier.padding(top=10.dp))}
            firstPlayable?.let{episode->
                val item=ApiHomeItem("EPISODE",episode.id,episode.title,null,null)
                val canResume=episode.progress?.let{!it.watched&&it.position>=30&&it.duration>it.position}==true
                Row(Modifier.padding(top=16.dp),horizontalArrangement=Arrangement.spacedBy(10.dp)){
                    VyNodePrimaryButton(if(canResume)"Resume" else "Play",onClick={play(item)})
                    if(canResume)VyNodeSecondaryButton("Start Over",onClick={startOver(item)})
                }
            }
            screen.message?.let{Text(it,color=MaterialTheme.colorScheme.primary,modifier=Modifier.padding(top=8.dp))}
        }
        item{
            Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),horizontalArrangement=Arrangement.spacedBy(8.dp)){
                seasons.forEach{season->
                    val label=if(season==0)"Specials" else "Season $season"
                    if(season==selectedSeason)VyNodePrimaryButton(label,onClick={}) else VyNodeSecondaryButton(label,onClick={selectedSeason=season})
                }
            }
        }
        items(episodes,key={it.id}){episode->
            val item=ApiHomeItem("EPISODE",episode.id,episode.title,null,null)
            OutlinedCard(onClick={if(episode.available){{play(item)}}else null},modifier=Modifier.fillMaxWidth(),shape=VyNodeRadius.medium,colors=CardDefaults.outlinedCardColors(containerColor=VyNodeColor.Surface),border=BorderStroke(1.dp,VyNodeColor.Border)){
                Column(Modifier.fillMaxWidth().padding(18.dp),verticalArrangement=Arrangement.spacedBy(8.dp)){
                    Text("S${episode.season.toString().padStart(2,'0')}E${episode.number.toString().padStart(2,'0')}",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge)
                    Text(episode.title,style=MaterialTheme.typography.titleLarge,maxLines=2)
                    episode.overview?.takeIf{it.isNotBlank()}?.let{Text(it,color=VyNodeColor.Muted,maxLines=2)}
                    Row(Modifier.fillMaxWidth(),horizontalArrangement=Arrangement.SpaceBetween,verticalAlignment=androidx.compose.ui.Alignment.CenterVertically){
                        Text(episode.runtimeMinutes?.let{"$it min"}?:"Play episode",color=VyNodeColor.Muted)
                        VyNodeSecondaryButton("Download",enabled=episode.available,onClick={download(episode)})
                    }
                }
            }
        }
    }
}

@Composable private fun OfflineScreen(downloads: List<DownloadEntity>, play: (DownloadEntity) -> Unit,initialRequester:FocusRequester?=null,railRequester:FocusRequester?=null,back:(()->Unit)?=null) {if(back!=null)BackHandler{back()};LazyColumn(Modifier.fillMaxSize().padding(32.dp), verticalArrangement=Arrangement.spacedBy(14.dp)) {
    item { Text("Downloads", style=MaterialTheme.typography.headlineLarge); Text("Available offline on this device",color=VyNodeColor.Muted) }
    if(downloads.isEmpty())item{Column(verticalArrangement=Arrangement.spacedBy(10.dp)){Text("No downloads yet",style=MaterialTheme.typography.titleLarge,modifier=Modifier.padding(top=28.dp));Text("Download a movie or episode to watch without a connection.",color=VyNodeColor.Muted);if(back!=null)VyNodePrimaryButton("Browse Home",modifier=Modifier.then(if(initialRequester!=null)Modifier.focusRequester(initialRequester).focusProperties{left=railRequester?:FocusRequester.Default}else Modifier),onClick=back)}}
    itemsIndexed(downloads, key={_,it->it.downloadId}) { index,item -> OutlinedCard(onClick={play(item)}, modifier=Modifier.fillMaxWidth().then(if(index==0&&initialRequester!=null)Modifier.focusRequester(initialRequester).focusProperties{left=railRequester?:FocusRequester.Default}else Modifier)) {
        Row(Modifier.padding(24.dp),horizontalArrangement=Arrangement.spacedBy(16.dp),verticalAlignment=androidx.compose.ui.Alignment.CenterVertically) {
            item.localArtwork?.let { path -> remember(path) { android.graphics.BitmapFactory.decodeFile(path) }?.let { bitmap -> Image(bitmap.asImageBitmap(),null,Modifier.size(72.dp)) } }
            Text(item.title, style=MaterialTheme.typography.titleLarge)
        }
    } }}
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
    Column(Modifier.fillMaxSize().background(VyNodeColor.Background).padding(if(tv) 64.dp else 24.dp), verticalArrangement=Arrangement.Center,horizontalAlignment=androidx.compose.ui.Alignment.CenterHorizontally) {
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

@Composable private fun PairScreen(screen: AppScreen.Pair) = Column(Modifier.fillMaxSize().background(VyNodeColor.Background).padding(40.dp), verticalArrangement=Arrangement.Center,horizontalAlignment=androidx.compose.ui.Alignment.CenterHorizontally) {
    Text("PAIR WITH",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text(screen.server.serverName, style=MaterialTheme.typography.headlineLarge); Spacer(Modifier.height(20.dp)); Text("Approve this device from your VyNode administration screen",color=VyNodeColor.Muted)
    Text(screen.request.code, style=MaterialTheme.typography.displayLarge, color=MaterialTheme.colorScheme.primary,modifier=Modifier.padding(vertical=24.dp))
    Text("Waiting for approval…",color=VyNodeColor.Muted)
}

@Composable private fun MessageScreen(message: String, retry: () -> Unit) = Column(Modifier.fillMaxSize().padding(40.dp), verticalArrangement=Arrangement.Center) {
    Text(message, style=MaterialTheme.typography.headlineSmall); Spacer(Modifier.height(20.dp)); VyNodePrimaryButton("Try Again", onClick=retry)
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable fun VyNodeHome(tv: Boolean, rows: List<HomeRow>, play: (ApiHomeItem) -> Unit = {}, focusId:String?=null, search:()->Unit={},downloads:()->Unit={},account:()->Unit={},embeddedTvShell:Boolean=false,initialRequester:FocusRequester?=null,railRequester:FocusRequester?=null) {
    val featureRequester=initialRequester?:remember{FocusRequester()}
    val feature=rows.firstOrNull()?.cards?.firstOrNull()
    Scaffold(containerColor=VyNodeColor.Background,topBar={ if(!embeddedTvShell)TopAppBar(colors=TopAppBarDefaults.topAppBarColors(containerColor=VyNodeColor.Background.copy(alpha=.94f)),title={Column{Text("VyNode",style=MaterialTheme.typography.titleLarge);Text("MEDIA",style=MaterialTheme.typography.labelSmall,color=VyNodeColor.Accent)}},actions={VyNodeSecondaryButton("Search",modifier=Modifier.focusRequester(featureRequester),onClick=search);VyNodeSecondaryButton("Downloads",onClick=downloads);TextButton(onClick=account,modifier=Modifier.padding(end=4.dp)){Text("Account",color=VyNodeColor.Muted)}}) }) { padding ->
        Column(Modifier.fillMaxSize().padding(padding).background(VyNodeColor.Background).padding(horizontal=if(tv)40.dp else 16.dp,vertical=if(embeddedTvShell)28.dp else 0.dp)){
          feature?.let { card -> HomeFeature(card,tv,featureRequester,railRequester){play(card.item)} }
          Spacer(Modifier.height(if(tv)28.dp else 20.dp))
          LazyColumn(Modifier.fillMaxWidth().weight(1f), verticalArrangement=Arrangement.spacedBy(if(tv) 32.dp else 22.dp)) {
            itemsIndexed(rows, key={_,row->row.id}) { rowIndex,row ->
                Column(verticalArrangement=Arrangement.spacedBy(10.dp)) {
                    Text(row.title, style=if(tv) MaterialTheme.typography.headlineMedium else MaterialTheme.typography.titleLarge)
                    Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).focusGroup().padding(vertical=6.dp), horizontalArrangement=Arrangement.spacedBy(if(tv) 18.dp else 12.dp)) {
                        row.cards.forEachIndexed { cardIndex,card -> key(card.id) { MediaCard(card, tv, card.id==focusId,featureRequester,if(cardIndex==0)railRequester else null) { play(card.item) } } }
                    }
                }
            }
          }
        }
    }
}

@Composable private fun TvShowScreen(screen:AppScreen.Show,play:(ApiHomeItem)->Unit,back:()->Unit){
    BackHandler{back()};val seasons=remember(screen.show.episodes){screen.show.episodes.map{it.season}.distinct().sorted()};var selectedSeason by remember(screen.show.id){mutableIntStateOf(seasons.firstOrNull{season->screen.show.episodes.any{it.season==season&&it.id==screen.focusId}}?:seasons.firstOrNull()?:0)};val episodes=screen.show.episodes.filter{it.season==selectedSeason};val playRequester=remember{FocusRequester()};val seasonRequester=remember{FocusRequester()}
    LaunchedEffect(screen.show.id){runCatching{playRequester.requestFocus()}}
    Column(Modifier.fillMaxSize().background(VyNodeColor.Background).padding(horizontal=64.dp,vertical=32.dp),verticalArrangement=Arrangement.spacedBy(10.dp)){
        Text("SERIES",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text(screen.show.title,style=MaterialTheme.typography.headlineLarge,maxLines=2)
        Text(listOfNotNull(screen.show.year.takeIf{it>0}?.toString(),screen.show.rating.takeIf{it>0}?.let{"Rated %.1f".format(it)},screen.show.genres.takeIf{it.isNotEmpty()}?.joinToString(" · ")).joinToString("  ·  "),color=VyNodeColor.Muted,style=MaterialTheme.typography.titleMedium)
        if(screen.show.overview.isNotBlank())Text(screen.show.overview,Modifier.fillMaxWidth(.72f),color=VyNodeColor.Muted,style=MaterialTheme.typography.bodyLarge,maxLines=2)
        screen.show.episodes.firstOrNull{it.available}?.let{episode->val canResume=episode.progress?.let{!it.watched&&it.position>=30&&it.duration>it.position}==true;VyNodePrimaryButton(if(canResume)"Resume ${formatPlayerTime((episode.progress!!.position*1000).toLong())}" else "Play",modifier=Modifier.focusRequester(playRequester).focusProperties{down=seasonRequester}){play(ApiHomeItem("EPISODE",episode.id,episode.title,null,null))}}
        Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).focusGroup(),horizontalArrangement=Arrangement.spacedBy(10.dp)){seasons.forEachIndexed{index,season->VyNodeSecondaryButton(if(season==0)"Specials" else "Season $season",modifier=Modifier.then(if(index==0)Modifier.focusRequester(seasonRequester).focusProperties{up=playRequester}else Modifier),onClick={selectedSeason=season})}}
        LazyRow(Modifier.fillMaxWidth().height(190.dp),horizontalArrangement=Arrangement.spacedBy(18.dp)){items(episodes,key={it.id}){episode->TvEpisodeCard(episode,episode.id==screen.focusId){play(ApiHomeItem("EPISODE",episode.id,episode.title,null,null))}}}
    }
}

@Composable private fun TvEpisodeCard(episode:ApiEpisode,restoreFocus:Boolean,onClick:()->Unit){var focused by remember{mutableStateOf(false)};val requester=remember{FocusRequester()};LaunchedEffect(restoreFocus){if(restoreFocus)runCatching{requester.requestFocus()}};OutlinedCard(onClick=onClick,enabled=episode.available,modifier=Modifier.width(340.dp).height(180.dp).focusRequester(requester).scale(if(focused)1.03f else 1f).onFocusChanged{focused=it.isFocused},shape=VyNodeRadius.medium,border=BorderStroke(if(focused)4.dp else 1.dp,if(focused)VyNodeColor.Focus else VyNodeColor.Border),colors=CardDefaults.outlinedCardColors(containerColor=if(focused)VyNodeColor.Raised else VyNodeColor.Surface)){Column(Modifier.fillMaxSize().padding(18.dp),verticalArrangement=Arrangement.spacedBy(6.dp)){Text("S${episode.season.toString().padStart(2,'0')}E${episode.number.toString().padStart(2,'0')}",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge);Text(episode.title,style=MaterialTheme.typography.headlineSmall,maxLines=1);if(episode.overview.isNotBlank())Text(episode.overview,color=VyNodeColor.Muted,maxLines=1);Spacer(Modifier.weight(1f));Text(if(episode.progress?.watched==true)"Watched" else episode.runtimeMinutes.takeIf{it>0}?.let{"$it min"}?:"Play",color=VyNodeColor.Muted)}}}

@Composable private fun HomeFeature(card:HomeCard,tv:Boolean,requester:FocusRequester,railRequester:FocusRequester?,play:()->Unit){
    LaunchedEffect(card.id){if(tv)runCatching{requester.requestFocus()}}
    Box(Modifier.fillMaxWidth().height(if(tv) 230.dp else 220.dp).clip(VyNodeRadius.large).background(VyNodeColor.Surface)){
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color.Transparent,VyNodeColor.Background.copy(alpha=.58f)))))
        Column(Modifier.align(androidx.compose.ui.Alignment.BottomStart).padding(if(tv) 32.dp else 20.dp),verticalArrangement=Arrangement.spacedBy(if(tv) 12.dp else 8.dp)){
            Text("FROM YOUR LIBRARY",color=VyNodeColor.Accent,style=MaterialTheme.typography.labelLarge)
            Text(card.title,style=if(tv) MaterialTheme.typography.displayLarge else MaterialTheme.typography.headlineLarge,maxLines=2)
            VyNodePrimaryButton(if(card.item.type=="SHOW") "View show" else "Play",modifier=Modifier.focusRequester(requester).then(if(railRequester!=null)Modifier.focusProperties{left=railRequester}else Modifier),onClick=play)
        }
    }
}

@Composable private fun MediaCard(card: HomeCard, tv: Boolean, restoreFocus:Boolean=false, upRequester:FocusRequester, railRequester:FocusRequester?=null, play: () -> Unit) {
    var focused by remember { mutableStateOf(false) }
    val requester=remember{FocusRequester()}
    LaunchedEffect(restoreFocus){if(restoreFocus) runCatching{requester.requestFocus()}}
    OutlinedCard(onClick=play, modifier=Modifier.width(if(tv) 210.dp else 150.dp).height(if(tv) 300.dp else 220.dp).scale(if(focused&&tv) 1.035f else 1f)
        .testTag("media-${card.id}").focusRequester(requester).focusProperties { up=upRequester;if(railRequester!=null)left=railRequester }.onFocusChanged { focused=it.isFocused }
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
@Composable private fun PlayerScreen(screen: AppScreen.Playing, controller: AppController, tv: Boolean) {
    val context = androidx.compose.ui.platform.LocalContext.current
    val engine = remember(screen.session.id) { VyNodePlayer(context, controller::accessToken) }
    var position by remember { mutableLongStateOf(0L) }
    var duration by remember { mutableLongStateOf(0L) }
    var isPlaying by remember { mutableStateOf(false) }
    var playWhenReady by remember { mutableStateOf(false) }
    var subtitleText by remember(screen.session.id) { mutableStateOf("") }
    var menu by remember { mutableStateOf<String?>(null) }
    var menuReturnRequester by remember { mutableStateOf<FocusRequester?>(null) }
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
    val showUpNext=!autoplayCanceled && screen.session.next!=null && activeMarker==null && duration>0 && upNextRemaining in 0..10
    val contextualActionRequester=when {
        showUpNext -> upNextRequester
        activeMarker != null -> markerRequester
        else -> FocusRequester.Cancel
    }
    Box(Modifier.fillMaxSize().background(Color.Black)) {
        AndroidView(factory={ PlayerView(it).apply {
            player=engine.player
            useController=false
            subtitleView?.visibility=android.view.View.GONE
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
                Row(if(tv)Modifier.fillMaxWidth() else Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),horizontalArrangement=if(tv)Arrangement.Center else Arrangement.Start,verticalAlignment=androidx.compose.ui.Alignment.CenterVertically) {
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
                    if(screen.session.qualities.isNotEmpty()) VyNodeSecondaryButton("Quality",modifier=Modifier.focusRequester(qualityRequester).focusProperties { left=forwardRequester;right=if(screen.session.audioTracks.isNotEmpty()) audioRequester else subtitleRequester },onClick={menuReturnRequester=qualityRequester;menu="quality"})
                    if(screen.session.audioTracks.isNotEmpty()){Spacer(Modifier.width(8.dp));VyNodeSecondaryButton("Audio",modifier=Modifier.focusRequester(audioRequester).focusProperties { left=if(screen.session.qualities.isNotEmpty()) qualityRequester else forwardRequester;right=subtitleRequester },onClick={menuReturnRequester=audioRequester;menu="audio"})}
                    if(screen.session.subtitleTracks.isNotEmpty()){Spacer(Modifier.width(8.dp));VyNodeSecondaryButton("Subtitles",modifier=Modifier.focusRequester(subtitleRequester).focusProperties { left=if(screen.session.audioTracks.isNotEmpty()) audioRequester else if(screen.session.qualities.isNotEmpty()) qualityRequester else forwardRequester;up=contextualActionRequester },onClick={menuReturnRequester=subtitleRequester;menu="subtitle"})}
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
        activeMarker?.let { marker -> val skip={engine.seekToMarkerEnd(marker.end);runCatching{playerRequester.requestFocus()};Unit};VyNodePrimaryButton(if(marker.type=="INTRO") "Skip Intro" else "Skip Credits",modifier=Modifier.align(androidx.compose.ui.Alignment.BottomEnd).padding(end=48.dp,bottom=176.dp).zIndex(4f).focusRequester(markerRequester).focusProperties { down=subtitleRequester }.remoteActivate(skip),onClick=skip) }
        screen.session.next?.let { next ->
            if(showUpNext) Surface(Modifier.align(androidx.compose.ui.Alignment.BottomEnd).padding(end=48.dp,bottom=176.dp).widthIn(max=440.dp).zIndex(4f),color=VyNodeColor.Overlay,shape=VyNodeRadius.large,shadowElevation=16.dp){
                Column(Modifier.padding(20.dp),verticalArrangement=Arrangement.spacedBy(8.dp)){
                    Text("Up Next: ${next.title} in $upNextRemaining")
                    Row(horizontalArrangement=Arrangement.spacedBy(8.dp)){
                        val playNow={ controller.changePlayback(screen.session,ApiHomeItem("EPISODE",next.id,next.title,null,null),positionMs=position,durationMs=duration,startMs=0) }
                        VyNodePrimaryButton("Play Now",modifier=Modifier.focusRequester(upNextRequester).remoteActivate(playNow),onClick=playNow)
                        val cancel={ autoplayCanceled=true;runCatching{playerRequester.requestFocus()};Unit }
                        VyNodeSecondaryButton("Cancel",modifier=Modifier.remoteActivate(cancel),onClick=cancel)
                    }
                }
            }
        }
        LaunchedEffect(menu) { if (menu != null) { delay(80);runCatching { menuRequester.requestFocus() } } else menuReturnRequester?.let{delay(80);runCatching{it.requestFocus()};menuReturnRequester=null} }
        menu?.let { kind ->
            Surface(Modifier.align(androidx.compose.ui.Alignment.CenterEnd).padding(48.dp).widthIn(min=260.dp,max=380.dp).zIndex(5f),color=VyNodeColor.Overlay,shape=VyNodeRadius.large,shadowElevation=18.dp) {
                Column(Modifier.padding(20.dp), verticalArrangement=Arrangement.spacedBy(8.dp)) {
                    Text(kind.replaceFirstChar { it.uppercase() })
                    if (kind == "quality") screen.session.qualities.forEachIndexed { index, quality ->
                        val select = { menu=null; controller.changePlayback(screen.session,screen.item,position,duration,qualityId=quality.id) }
                        val selected=quality.id==screen.session.selectedQualityId
                        val focus=selected || (screen.session.selectedQualityId==null&&index==0)
                        if(selected)VyNodePrimaryButton(quality.label,modifier=(if(focus)Modifier.focusRequester(menuRequester)else Modifier).remoteActivate(select),onClick=select)
                        else VyNodeSecondaryButton(quality.label,modifier=(if(focus)Modifier.focusRequester(menuRequester)else Modifier).remoteActivate(select),onClick=select)
                    }
                    if (kind == "audio") screen.session.audioTracks.forEachIndexed { index, track ->
                        val select = { menu=null; controller.changePlayback(screen.session,screen.item,position,duration,audioId=track.id) }
                        val selected=track.id==screen.session.selectedAudioId || (screen.session.selectedAudioId==null&&track.default)
                        val focus=selected || (screen.session.selectedAudioId==null&&screen.session.audioTracks.none{it.default}&&index==0)
                        if(selected)VyNodePrimaryButton(track.semanticLabel,modifier=(if(focus)Modifier.focusRequester(menuRequester)else Modifier).remoteActivate(select),onClick=select)
                        else VyNodeSecondaryButton(track.semanticLabel,modifier=(if(focus)Modifier.focusRequester(menuRequester)else Modifier).remoteActivate(select),onClick=select)
                    }
                    if (kind == "subtitle") {
                        val disable = { menu=null; controller.changePlayback(screen.session,screen.item,position,duration,subtitleId="") }
                        if(screen.session.selectedSubtitleId==null)VyNodePrimaryButton("Off",modifier=Modifier.focusRequester(menuRequester).remoteActivate(disable),onClick=disable)
                        else VyNodeSecondaryButton("Off",modifier=Modifier.remoteActivate(disable),onClick=disable)
                        screen.session.subtitleTracks.forEach { track ->
                            val select = { menu=null; controller.changePlayback(screen.session,screen.item,position,duration,subtitleId=track.id) }
                            val selected=track.id==screen.session.selectedSubtitleId
                            if(selected)VyNodePrimaryButton(track.semanticLabel,modifier=Modifier.focusRequester(menuRequester).remoteActivate(select),onClick=select)
                            else VyNodeSecondaryButton(track.semanticLabel,modifier=Modifier.remoteActivate(select),onClick=select)
                        }
                    }
                }
            }
        }
    }
}
