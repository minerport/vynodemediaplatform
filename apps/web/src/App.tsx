import {
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import Hls from "./vendor/hls.min.mjs";
import {
  api,
  browserCapabilities,
  type AuditEvent,
  type ContinueItem,
  type Library,
  type MediaFile,
  type Movie,
  type PlaybackSession,
  type PlaybackPreferences,
  type PlaybackVersion,
  type ScanJob,
  type Session,
  type Show,
  type SystemInfo,
  type Unmatched,
  type User,
} from "./api";
type Page = string;
let activePlaybackContext = "";
const requested = () => {
  const path = location.pathname.slice(1);
  return path === "settings" ? "account" : path || "home";
};
export function App() {
  const [page, setPage] = useState<Page>(requested()),
    [user, setUser] = useState<User | null>(null),
    [info, setInfo] = useState<SystemInfo | null>(null),
    [loading, setLoading] = useState(true);
  const go = (p: Page) => {
    history.pushState({}, "", `/${p}`);
    setPage(p);
  };
  useEffect(() => {
    api
      .status()
      .then(async (s) => {
        if (s.setupRequired) {
          go("setup");
          return;
        }
        try {
          const u = await api.refresh();
          setUser(u);
          setInfo(await api.info());
          if (requested() === "setup" || requested() === "login") go("home");
        } catch {
          if (!requested().startsWith("invite/")) go("login");
        }
      })
      .finally(() => setLoading(false));
  }, []);
  if (loading)
    return (
      <main>
        <section className="panel loading">
          <div />
          <div />
        </section>
      </main>
    );
  if (page === "setup")
    return (
      <Setup
        done={(u) => {
          setUser(u);
          api.info().then(setInfo);
          go("home");
        }}
      />
    );
  if(page.startsWith("invite/"))return <InviteAccept token={page.slice(7)} done={u=>{setUser(u);api.info().then(setInfo);go("home")}}/>;
  if (page === "login" || !user)
    return (
      <Login
        done={(u) => {
          setUser(u);
          api.info().then(setInfo);
          go("home");
        }}
      />
    );
  const admin = user.role !== "USER";
  if (
    (page === "users" ||
      page === "audit" ||
      page === "libraries" ||
      page === "metadata" ||
      page === "automation" || page === "admin/sharing" || page === "admin/remote-access" ||
      page === "streams" || page === "admin/dashboard" || page === "admin/analytics" || page === "admin/health" || page === "admin/jobs" || page === "admin/notifications") &&
    !admin
  ) {
    history.replaceState({}, "", "/home");
    return <Home info={info} user={user} />;
  }
  const watch = page.match(/^watch\/(movie|episode)\/([^/]+)$/);
  const content = watch ? (
    <Player
      type={watch[1].toUpperCase() as "MOVIE" | "EPISODE"}
      id={watch[2]}
      go={go}
    />
  ) : page === "account" ? (
    <Account user={user} info={info} />
  ) : page === "security/sessions" ? (
    <Sessions />
  ) : page === "downloads" ? (
    <DownloadsPage />
  ) : page === "libraries" ? (
    <Libraries />
  ) : page === "users" ? (
    <Users />
  ) : page === "audit" ? (
    <Audit />
  ) : page === "streams" ? (
    <ActiveStreams />
  ) : page === "admin/dashboard" ? (
    <AdminDashboardPage go={go}/>
  ) : page === "admin/analytics" ? (
    <AnalyticsPage />
  ) : page === "admin/health" ? (
    <HealthPage />
  ) : page === "admin/jobs" ? (
    <JobsPage />
  ) : page === "admin/notifications" ? (
    <NotificationsPage />
  ) : page === "admin/sharing" ? (
    <SharingPage />
  ) : page === "admin/remote-access" ? (
    <RemoteAccessPage />
  ) : page === "movies" ? (
    <Movies go={go} />
  ) : page.startsWith("movies/") ? (
    <MovieDetail id={page.split("/")[1]} go={go} />
  ) : page === "shows" ? (
    <Shows go={go} />
  ) : page.startsWith("shows/") ? (
    <ShowDetail id={page.split("/")[1]} go={go} />
  ) : page === "metadata" ? (
    <MetadataAdmin />
  ) : page === "automation" ? (
    <AutomationAdmin />
  ) : page === "collections" ? (
    <CollectionsPage go={go} admin={admin}/>
  ) : page.startsWith("collections/") ? (
    <CollectionPage id={page.split("/")[1]} go={go} admin={admin}/>
  ) : page === "playlists" ? (
    <PlaylistsPage />
  ) : page === "watchlist" || page === "favorites" ? (
    <PersonalPage kind={page as "watchlist"|"favorites"} go={go}/>
  ) : page === "settings/home" ? (
    <HomeSettings />
  ) : (
    <Home info={info} user={user} />
  );
  if (watch) return content;
  const pageTitle = page.startsWith("admin/")
    ? "Server administration"
    : page === "settings/home"
      ? "Home customization"
      : page.split("/")[0].replace(/(^|-)\w/g, (value) => value.replace("-", " ").toUpperCase());
  return (
    <div className="app-shell">
      <aside>
        <div className="brand">
          <div className="mark">V</div><span>VyNode</span><small>MEDIA</small>
        </div>
        <nav>
          <p className="nav-section-label">Browse</p>
          <Nav go={go} page={page} target="home">
            Home
          </Nav>
          <Nav go={go} page={page} target="movies">
            Movies
          </Nav>
          <Nav go={go} page={page} target="shows">
            Shows
          </Nav>
          <Nav go={go} page={page} target="collections">Collections</Nav>
          <Nav go={go} page={page} target="playlists">Playlists</Nav>
          <Nav go={go} page={page} target="watchlist">Watchlist</Nav>
          <Nav go={go} page={page} target="favorites">Favorites</Nav>
          <Nav go={go} page={page} target="downloads">Downloads</Nav>
          {admin && (
            <>
              <p className="nav-section-label">Administration</p>
              <Nav go={go} page={page} target="admin/dashboard">Dashboard</Nav>
              <Nav go={go} page={page} target="admin/analytics">Analytics</Nav>
              <Nav go={go} page={page} target="admin/health">Health</Nav>
              <Nav go={go} page={page} target="admin/jobs">Jobs</Nav>
              <Nav go={go} page={page} target="admin/notifications">Webhooks</Nav>
              <Nav go={go} page={page} target="admin/sharing">Sharing</Nav>
              {user.role==="OWNER"&&<Nav go={go} page={page} target="admin/remote-access">Remote Access</Nav>}
              <Nav go={go} page={page} target="libraries">
                Libraries
              </Nav>
              <Nav go={go} page={page} target="metadata">
                Metadata
              </Nav>
              <Nav go={go} page={page} target="streams">
                Active Streams
              </Nav>
              <Nav go={go} page={page} target="automation">
                Automation
              </Nav>
            </>
          )}
          <p className="nav-section-label">Settings</p>
          <Nav go={go} page={page} target="settings/home">Home rows</Nav>
          <Nav go={go} page={page} target="account">
            Account
          </Nav>
          <Nav go={go} page={page} target="security/sessions">
            Sessions
          </Nav>
          {admin && (
            <>
              <Nav go={go} page={page} target="users">
                Users
              </Nav>
              <Nav go={go} page={page} target="audit">
                Audit
              </Nav>
            </>
          )}
          <button className="nav-logout"
            onClick={() =>
              api.logout().finally(() => {
                setUser(null);
                go("login");
              })
            }
          >
            Logout
          </button>
        </nav>
      </aside>
      <main>
        <header>
          <button className="mobile-brand" onClick={()=>go("home")} aria-label="Go to Home"><span className="mark">V</span></button>
          <div className="header-context">
            <p className="eyebrow">{pageTitle}</p>
            <h1>{user.displayName}</h1>
          </div>
          <button className="header-search" onClick={()=>go("movies")} aria-label="Browse media">Browse</button>
          <div className="connection online">
            <i />
            {info?.serverName}
          </div>
        </header>
        {admin && (
          <label className="mobile-admin-select">
            Admin view
            <select
              value={page.startsWith("admin/") || page === "streams" ? page : "admin/dashboard"}
              onChange={(e) => go(e.target.value)}
            >
              <option value="admin/dashboard">Dashboard</option>
              <option value="streams">Active Streams</option>
              <option value="admin/analytics">Analytics</option>
              <option value="admin/health">Health</option>
              <option value="admin/jobs">Jobs</option>
              <option value="admin/notifications">Webhooks</option>
              <option value="admin/sharing">Sharing</option>
              {user.role==="OWNER"&&<option value="admin/remote-access">Remote Access</option>}
            </select>
          </label>
        )}
        {content}
        <nav className="mobile-nav" aria-label="Primary navigation">
          <Nav go={go} page={page} target="home">Home</Nav>
          <Nav go={go} page={page} target="movies">Movies</Nav>
          <Nav go={go} page={page} target="shows">Shows</Nav>
          <Nav go={go} page={page} target="downloads">Downloads</Nav>
          <Nav go={go} page={page} target="account">Account</Nav>
        </nav>
      </main>
    </div>
  );
}
function Nav({
  go,
  page,
  target,
  children,
}: {
  go: (p: Page) => void;
  page: Page;
  target: Page;
  children: ReactNode;
}) {
  return (
    <button
      className={page === target ? "active" : ""}
      onClick={() => go(target)}
    >
      {children}
    </button>
  );
}
function Home({ info, user }: { info: SystemInfo | null; user: User }) {
  const [rows,setRows]=useState<import("./api").HomeRow[]>([]);
  useEffect(()=>{api.home().then(x=>setRows(x.rows??[]))},[]);
  return (
    <section className="content">
      <div className="hero">
        <p className="eyebrow">YOUR MEDIA</p>
        <h2>{info?.serverName}</h2>
        <p>Welcome back, {user.displayName}.</p>
      </div>
      {!rows.length && (
        <div className="empty">
          <div className="empty-icon">V</div>
          <h2>Your library is ready</h2>
          <p>Identified movies and shows will appear here.</p>
        </div>
      )}
      {rows.map(r=><section key={r.ID} className="home-row"><h2>{r.Title}</h2><div className="poster-grid">{r.Items.map((x,i)=><CurationCard key={`${x.Type}-${x.ID}-${i}`} item={x}/>)}</div>{r.SeeAll&&<a href={r.SeeAll}>See all</a>}</section>)}
    </section>
  );
}
function Card({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main>
      <section className="panel auth-card">
        <div className="mark">V</div>
        <h1>{title}</h1>
        {children}
      </section>
    </main>
  );
}
function Setup({ done }: { done: (u: User) => void }) {
  const [e, busy, error] = useSubmit(
    async (d) =>
      api.setup({
        serverName: d.get("server"),
        displayName: d.get("display"),
        username: d.get("username"),
        password: d.get("password"),
        device: {
          name: "Web browser",
          clientName: "VyNode Web",
          platform: "web",
        },
      }),
    done,
  );
  return (
    <Card title="Create your server">
      <Form submit={e} busy={busy} error={error} setup />
    </Card>
  );
}
function Login({ done }: { done: (u: User) => void }) {
  const [e, busy, error] = useSubmit(
    (d) => api.login(String(d.get("username")), String(d.get("password"))),
    done,
  );
  return (
    <Card title="Sign in">
      <Form submit={e} busy={busy} error={error} />
    </Card>
  );
}
function useSubmit(
  work: (d: FormData) => Promise<User>,
  done: (u: User) => void,
) {
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const d = new FormData(e.currentTarget);
    if (d.get("confirm") && d.get("confirm") !== d.get("password")) {
      setError("Passwords do not match.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      done(await work(d));
    } catch (x) {
      setError(x instanceof Error ? x.message : "Request failed");
    } finally {
      setBusy(false);
    }
  };
  return [submit, busy, error] as const;
}
function Form({
  submit,
  busy,
  error,
  setup = false,
}: {
  submit: (e: FormEvent<HTMLFormElement>) => void;
  busy: boolean;
  error?: string;
  setup?: boolean;
}) {
  return (
    <form onSubmit={submit}>
      {setup && (
        <>
          <label>
            Server name
            <input name="server" required maxLength={100} />
          </label>
          <label>
            Display name
            <input
              name="display"
              autoComplete="name"
              required
              maxLength={100}
            />
          </label>
        </>
      )}
      <label>
        Username
        <input
          name="username"
          autoComplete="username"
          required
          pattern="[A-Za-z0-9][A-Za-z0-9._-]{2,63}"
        />
      </label>
      <label>
        Password
        <input
          name="password"
          type="password"
          autoComplete={setup ? "new-password" : "current-password"}
          minLength={setup ? 10 : 1}
          maxLength={256}
          required
        />
      </label>
      {setup && (
        <label>
          Confirm password
          <input
            name="confirm"
            type="password"
            autoComplete="new-password"
            required
          />
        </label>
      )}
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <button disabled={busy}>
        {busy ? "Working…" : setup ? "Create owner" : "Sign in"}
      </button>
    </form>
  );
}
function Account({ user, info }: { user: User; info: SystemInfo | null }) {
  const [msg, setMsg] = useState(""),
    [busy, setBusy] = useState(false),
    [preferences,setPreferences]=useState<PlaybackPreferences|null>(null);
  useEffect(()=>{api.playbackPreferences().then(setPreferences)},[]);
  async function change(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (busy) return;
    const form = e.currentTarget,
      d = new FormData(form);
    if (d.get("next") !== d.get("confirm")) {
      setMsg("Passwords do not match.");
      return;
    }
    setBusy(true);
    setMsg("");
    try {
      await api.password(String(d.get("current")), String(d.get("next")));
      form.reset();
      setMsg("Password changed. Other sessions were revoked.");
    } catch (x) {
      setMsg(x instanceof Error ? x.message : "Unable to change password");
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="panel">
      <h2>Account</h2>
      <dl className="vertical">
        <div>
          <dt>Display name</dt>
          <dd>{user.displayName}</dd>
        </div>
        <div>
          <dt>Username</dt>
          <dd>@{user.username}</dd>
        </div>
        <div>
          <dt>Role</dt>
          <dd>{user.role}</dd>
        </div>
        <div>
          <dt>Created</dt>
          <dd>{new Date(user.createdAt).toLocaleString()}</dd>
        </div>
        <div>
          <dt>Server</dt>
          <dd>{info?.serverName}</dd>
        </div>
      </dl>
      <form onSubmit={change}>
        <h2>Change password</h2>
        <label>
          Current password
          <input name="current" type="password" required />
        </label>
        <label>
          New password
          <input
            name="next"
            type="password"
            minLength={10}
            maxLength={256}
            required
          />
        </label>
        <label>
          Confirm new password
          <input name="confirm" type="password" required />
        </label>
        <button disabled={busy}>
          {busy ? "Changing…" : "Change password"}
        </button>
        {msg && <p role="status">{msg}</p>}
      </form>
      {preferences&&<form onSubmit={e=>{e.preventDefault();setBusy(true);api.setPlaybackPreferences(preferences).then(p=>{setPreferences(p);setMsg("Playback preferences saved.")}).catch(x=>setMsg(x.message)).finally(()=>setBusy(false))}}>
        <h2>Playback</h2>
        <label>Preferred audio languages<input aria-label="Preferred audio languages" value={preferences.preferredAudioLanguages.join(", ")} onChange={e=>setPreferences({...preferences,preferredAudioLanguages:e.target.value.split(",").map(x=>x.trim()).filter(Boolean)})}/><small>Ordered language codes, for example: en, ja, es</small></label>
        <label>Preferred subtitle languages<input aria-label="Preferred subtitle languages" value={preferences.preferredSubtitleLanguages.join(", ")} onChange={e=>setPreferences({...preferences,preferredSubtitleLanguages:e.target.value.split(",").map(x=>x.trim()).filter(Boolean)})}/></label>
        <label>Subtitles<select aria-label="Subtitle mode" value={preferences.subtitleMode} onChange={e=>setPreferences({...preferences,subtitleMode:e.target.value as PlaybackPreferences["subtitleMode"]})}><option value="OFF">Off</option><option value="ALWAYS">Always</option><option value="WHEN_AUDIO_NOT_PREFERRED">When audio is not preferred</option><option value="FORCED_ONLY">Forced only</option></select></label>
        <label><input type="checkbox" checked={preferences.avoidCommentary} onChange={e=>setPreferences({...preferences,avoidCommentary:e.target.checked})}/> Avoid commentary unless explicitly selected</label>
        <label><input type="checkbox" checked={preferences.preferHearingImpaired} onChange={e=>setPreferences({...preferences,preferHearingImpaired:e.target.checked})}/> Prefer hearing-impaired subtitles</label>
        <label><input type="checkbox" checked={preferences.autoplayNextEpisode} onChange={e=>setPreferences({...preferences,autoplayNextEpisode:e.target.checked})}/> Autoplay next episode</label>
        <label>Home streaming quality<select value={preferences.localQualityId} onChange={e=>setPreferences({...preferences,localQualityId:e.target.value})}><option value="auto">Auto</option><option value="original">Original</option><option value="1080p">Up to 1080p</option><option value="720p">Up to 720p</option><option value="480p">Up to 480p</option></select></label>
        <label>Remote streaming quality<select value={preferences.remoteQualityId} onChange={e=>setPreferences({...preferences,remoteQualityId:e.target.value})}><option value="auto">Auto</option><option value="original">Original</option><option value="1080p">Up to 1080p</option><option value="720p">Up to 720p</option><option value="480p">Up to 480p</option></select></label>
        <button disabled={busy}>Save playback preferences</button>
      </form>}
      <PairDevice />
    </section>
  );
}

function InviteAccept({token,done}:{token:string;done:(u:User)=>void}){
  const [invite,setInvite]=useState<{serverName:string;invitation:import("./api").Invitation}|null>(null),[error,setError]=useState("");
  useEffect(()=>{api.inspectInvitation(token).then(setInvite).catch(e=>setError(e.message))},[token]);
  async function submit(e:FormEvent<HTMLFormElement>){e.preventDefault();const f=e.currentTarget,d=new FormData(f);setError("");try{const u=await api.acceptInvitation({token,username:d.get("username"),displayName:d.get("display"),password:d.get("password"),device:{name:"Web browser",clientName:"VyNode Web",platform:navigator.platform||"web"}});done(u)}catch(x){setError(x instanceof Error?x.message:"Unable to accept invitation")}}
  return <main className="auth"><section className="auth-card"><div className="mark large">V</div><h1>{invite?.serverName||"VyNode Media"}</h1>{invite?<><p>You were invited as <strong>{invite.invitation.role}</strong> with access to {invite.invitation.libraries.map(x=>x.libraryName).join(", ")||"no libraries"}.</p><form onSubmit={submit}><label>Display name<input name="display" required maxLength={100}/></label><label>Username<input name="username" required pattern="[A-Za-z0-9][A-Za-z0-9._-]{2,63}"/></label><label>Password<input name="password" type="password" minLength={10} required/></label><button>Create account</button></form></>:!error&&<p>Checking invitation…</p>}{error&&<p className="form-error" role="alert">{error}</p>}</section></main>
}

function PairDevice(){const [code,setCode]=useState(""),[request,setRequest]=useState<import("./api").PairingRequest|null>(null),[message,setMessage]=useState("");async function find(e:FormEvent){e.preventDefault();setMessage("");try{setRequest(await api.lookupPairing(code))}catch(x){setMessage(x instanceof Error?x.message:"Pairing request unavailable")}}return <section><h2>Pair a device</h2><p>Enter the short code shown by your TV or native client. Approval creates a normal revocable device session.</p><form onSubmit={find}><label>Pairing code<input value={code} onChange={e=>setCode(e.target.value.toUpperCase())} placeholder="ABCD-7KQ2" maxLength={9} required/></label><button>Find device</button></form>{request&&<div className="list-row"><div><strong>{request.deviceName}</strong><p>{request.clientName} · {request.platform}<br/>Requested {new Date(request.requestedAt).toLocaleString()}</p></div><div><button onClick={()=>api.approvePairing(request.id).then(()=>{setMessage("Device approved. It may now complete normal session setup.");setRequest(null)})}>Approve</button><button className="danger" onClick={()=>api.denyPairing(request.id).then(()=>setRequest(null))}>Deny</button></div></div>}{message&&<p role="status">{message}</p>}</section>}

function SharingPage(){const [users,setUsers]=useState<User[]>([]),[libraries,setLibraries]=useState<Library[]>([]),[invitations,setInvitations]=useState<import("./api").Invitation[]>([]),[selected,setSelected]=useState<Record<string,string[]>>({}),[download,setDownload]=useState<Record<string,string[]>>({}),[link,setLink]=useState(""),[message,setMessage]=useState("");const load=()=>Promise.all([api.users(),api.libraries(),api.invitations()]).then(async([u,l,i])=>{setUsers(u.users);setLibraries(l.libraries);setInvitations(i.invitations);const grants=await Promise.all(u.users.map(async x=>[x.id,(await api.userGrants(x.id)).grants] as const));setSelected(Object.fromEntries(grants.map(([id,g])=>[id,g.filter(x=>x.permissions.includes("VIEW")).map(x=>x.libraryId)])));setDownload(Object.fromEntries(grants.map(([id,g])=>[id,g.filter(x=>x.permissions.includes("DOWNLOAD")).map(x=>x.libraryId)])))});useEffect(()=>{load()},[]);const toggle=(user:string,library:string)=>setSelected(v=>({...v,[user]:(v[user]||[]).includes(library)?v[user].filter(x=>x!==library):[...(v[user]||[]),library]}));const toggleDownload=(user:string,library:string)=>setDownload(v=>({...v,[user]:(v[user]||[]).includes(library)?v[user].filter(x=>x!==library):[...(v[user]||[]),library]}));async function invite(e:FormEvent<HTMLFormElement>){e.preventDefault();const f=e.currentTarget,d=new FormData(f),ids=d.getAll("library").map(String),downloadIds=d.getAll("download").map(String);const x=await api.createInvitation({identifier:d.get("identifier"),role:d.get("role"),expiresInHours:Number(d.get("expiration")),libraries:ids.map(libraryId=>({libraryId,permissions:["VIEW","PLAY",...(downloadIds.includes(libraryId)?["DOWNLOAD" as const]:[])]}))});setLink(location.origin+x.invitePath);setMessage("This invite link is shown once. Create a new invitation if you lose it.");f.reset();load()}return <section className="content admin-console"><div className="admin-heading"><div><p className="eyebrow">ACCESS POLICY</p><h2>Users and sharing</h2></div></div><div className="ops-grid"><section><h3>Library access</h3>{users.filter(u=>u.role==="USER").map(u=><div key={u.id} className="sharing-user"><strong>{u.displayName}</strong><small>@{u.username} · role and media access are independent</small>{libraries.map(l=><div key={l.id}><label><input type="checkbox" checked={(selected[u.id]||[]).includes(l.id)} onChange={()=>toggle(u.id,l.id)}/> View/play {l.name}</label><label><input type="checkbox" disabled={!(selected[u.id]||[]).includes(l.id)} checked={(download[u.id]||[]).includes(l.id)} onChange={()=>toggleDownload(u.id,l.id)}/> Allow offline download</label></div>)}<button onClick={()=>api.setUserGrants(u.id,(selected[u.id]||[]).map(libraryId=>({libraryId,permissions:["VIEW","PLAY",...((download[u.id]||[]).includes(libraryId)?["DOWNLOAD" as const]:[])]}))).then(()=>setMessage(`Access updated for ${u.displayName}.`))}>Apply access</button></div>)}</section><section><h3>Invite user</h3><form onSubmit={invite}><label>Identifier (optional)<input name="identifier" maxLength={100}/></label><label>Role<select name="role"><option value="USER">User</option><option value="ADMIN">Admin</option></select></label><label>Expires<select name="expiration"><option value="1">1 hour</option><option value="24">24 hours</option><option value="168">7 days</option></select></label>{libraries.map(l=><div key={l.id}><label><input type="checkbox" name="library" value={l.id}/> {l.name}</label><label><input type="checkbox" name="download" value={l.id}/> Include DOWNLOAD permission</label></div>)}<button>Create invitation</button></form>{link&&<div className="one-time-secret"><label>One-time invite link<input readOnly value={link}/></label><button onClick={()=>navigator.clipboard.writeText(link)}>Copy invite link</button></div>}{message&&<p role="status">{message}</p>}</section><section><h3>Invitations</h3>{invitations.map(x=><div className="event-row" key={x.id}><b>{x.status}</b><span>{x.identifier||"Unlabeled"} · {x.role} · {x.libraries.map(l=>l.libraryName).join(", ")||"No libraries"}</span><time>Expires {new Date(x.expiresAt).toLocaleString()}</time>{x.status==="PENDING"&&<button onClick={()=>api.revokeInvitation(x.id).then(load)}>Revoke</button>}</div>)}</section></div></section>}

function RemoteAccessPage(){const [x,setX]=useState<import("./api").RemoteSettings|null>(null),[message,setMessage]=useState("");useEffect(()=>{api.remoteAccess().then(setX)},[]);if(!x)return <section className="panel loading"><div/><div/></section>;return <section className="content admin-console"><div className="admin-heading"><div><p className="eyebrow">LOCAL-FIRST NETWORKING</p><h2>Remote Access</h2></div></div><div className="ops-grid"><section><h3>Connection policy</h3><label><input type="checkbox" checked={x.discoveryEnabled} onChange={e=>setX({...x,discoveryEnabled:e.target.checked})}/> LAN Discovery</label><small>Runtime: {x.discoveryStatus}{x.discoveryLastError?` · ${x.discoveryLastError}`:""}</small><label><input type="checkbox" checked={x.reverseProxyEnabled} onChange={e=>setX({...x,reverseProxyEnabled:e.target.checked})}/> Reverse proxy enabled</label><label>Manual remote URL<input value={x.manualRemoteUrl||""} placeholder="https://media.example.com" onChange={e=>setX({...x,manualRemoteUrl:e.target.value})}/></label><label><input type="checkbox" checked={x.insecureRemoteAllowed} onChange={e=>setX({...x,insecureRemoteAllowed:e.target.checked})}/> Explicitly allow insecure HTTP remote URL</label>{x.manualRemoteUrl?.startsWith("http:")&&<p className="form-error">Remote credentials and media would travel over an insecure connection.</p>}<button onClick={()=>api.saveRemoteAccess(x).then(v=>{setX(v);setMessage("Remote access settings saved.")}).catch(e=>setMessage(e.message))}>Save settings</button></section><section><h3>Trusted networks</h3><label>Trusted proxy CIDRs<textarea value={x.trustedProxyCidrs.join("\n")} onChange={e=>setX({...x,trustedProxyCidrs:e.target.value.split(/\s+/).filter(Boolean)})}/></label><small>Forwarded headers are accepted only from these explicit proxy networks.</small><label>Local network CIDRs<textarea value={x.localNetworkCidrs.join("\n")} onChange={e=>setX({...x,localNetworkCidrs:e.target.value.split(/\s+/).filter(Boolean)})}/></label><small>Classification affects warnings and playback quality, never authentication.</small></section><section><h3>Diagnostics</h3><table><tbody><tr><th>Manual endpoint</th><td>{x.manualStatus}</td></tr><tr><th>TLS</th><td>{x.manualRemoteUrl?.startsWith("https:")?"HTTPS configured":x.manualRemoteUrl?"INSECURE":"Not configured"}</td></tr><tr><th>LAN discovery</th><td>{x.discoveryStatus}</td></tr><tr><th>Automatic mapping</th><td>{x.portMappingProtocol||"UPNP"} · {x.portMappingStatus}</td></tr>{x.portMappingLeaseExpiresAt&&<tr><th>Mapping lease</th><td>{new Date(x.portMappingLeaseExpiresAt).toLocaleString()}</td></tr>}<tr><th>External reachability</th><td>Unverified externally</td></tr></tbody></table><label><input type="checkbox" checked={x.portMappingEnabled} onChange={e=>setX({...x,portMappingEnabled:e.target.checked})}/> Enable automatic port mapping</label><label>External TCP port (0 = listener port)<input type="number" min="0" max="65535" value={x.portMappingExternalPort||0} onChange={e=>setX({...x,portMappingExternalPort:Number(e.target.value)})}/></label><small>UPnP IGD is opt-in and maps only VyNode's configured listener. A successful mapping does not prove external reachability.</small>{x.portMappingLastError&&<p className="form-error">{x.portMappingLastError}</p>}{message&&<p role="status">{message}</p>}</section></div></section>}
function DownloadsPage(){const [items,setItems]=useState<import("./api").OfflineDownload[]>([]),[settings,setSettings]=useState<import("./api").OfflineSettings|null>(null),[message,setMessage]=useState("");const load=()=>{api.offlineDownloads().then(x=>setItems(x.downloads)).catch(e=>setMessage(e.message));api.offlineSettings().then(setSettings).catch(()=>{})};useEffect(()=>{load()},[]);return <section className="content"><div className="admin-heading"><div><p className="eyebrow">NATIVE DEVICE SYNC</p><h2>Offline downloads</h2></div></div>{settings&&<section className="panel"><h3>Server download cache</h3><p>{(settings.cacheBytes/1073741824).toFixed(2)} GiB used · {settings.readyAssets} ready · {settings.preparingAssets} preparing</p><label>Cache quota in GiB (0 = unlimited)<input type="number" min="0" step="1" value={Math.round(settings.cacheQuotaBytes/1073741824)} onChange={e=>setSettings({...settings,cacheQuotaBytes:Number(e.target.value)*1073741824})}/></label><button onClick={()=>api.saveOfflineSettings(settings.cacheQuotaBytes).then(setSettings)}>Save cache quota</button></section>}<section className="panel"><p>This page manages server assignments for the current paired/native device. Media is stored by native clients, not in browser storage.</p>{message&&<p role="status">{message}</p>}{items.length===0&&!message&&<p>No offline downloads are assigned to this device.</p>}{items.map(x=><div className="list-row" key={x.id}><div><strong>{x.logicalType} · {x.profileId}</strong><p>{x.mode} · {x.status} · {x.sizeBytes?`${(x.sizeBytes/1048576).toFixed(1)} MB`:"preparing"}</p><small>{x.checksumSha256?`SHA-256 ${x.checksumSha256.slice(0,16)}…`:Math.round(x.progress*100)+"% prepared"}</small></div><button onClick={()=>api.removeOfflineDownload(x.id).then(load)}>Remove</button></div>)}</section></section>}

function Sessions() {
  const [items, setItems] = useState<Session[]>([]),
    [msg, setMsg] = useState("");
  const load = () => api.sessions().then((x) => setItems(x.sessions));
  useEffect(() => {
    load();
  }, []);
  return (
    <section className="panel">
      <h2>Active sessions</h2>
      {items.map((s) => (
        <div className="list-row" key={s.id}>
          <div>
            <strong>{s.deviceName}</strong>
            <p>
              {s.clientName || "VyNode client"} · {s.platform}
              <br />
              Created {new Date(s.createdAt).toLocaleString()} · Active{" "}
              {new Date(s.lastActivityAt).toLocaleString()}
            </p>
          </div>
          {s.current ? (
            <span>Current</span>
          ) : (
            <button
              onClick={() => {
                if (confirm(`Revoke ${s.deviceName}?`))
                  api.revoke(s.id).then(load);
              }}
            >
              Revoke
            </button>
          )}
        </div>
      ))}
      <button
        onClick={() =>
          api.logoutOthers().then(() => {
            setMsg("Other sessions logged out.");
            load();
          })
        }
      >
        Log out all other sessions
      </button>
      {msg && <p role="status">{msg}</p>}
    </section>
  );
}
function Users() {
  const [items, setItems] = useState<User[]>([]);
  const load = () => api.users().then((x) => setItems(x.users));
  useEffect(() => {
    load();
  }, []);
  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const d = new FormData(e.currentTarget);
    await api.createUser({
      displayName: d.get("display"),
      username: d.get("username"),
      password: d.get("password"),
      role: d.get("role"),
    });
    e.currentTarget.reset();
    load();
  }
  return (
    <section className="panel">
      <h2>Users</h2>
      {items.map((u) => (
        <div className="list-row" key={u.id}>
          <div>
            <strong>{u.displayName}</strong>
            <p>
              @{u.username} · {u.role} · {u.status}
            </p>
          </div>
          {u.role !== "OWNER" && (
            <button
              onClick={() =>
                api.setEnabled(u.id, u.status !== "ACTIVE").then(load)
              }
            >
              {u.status === "ACTIVE" ? "Disable" : "Enable"}
            </button>
          )}
        </div>
      ))}
      <form onSubmit={create}>
        <h2>Add user</h2>
        <label>
          Display name
          <input name="display" required />
        </label>
        <label>
          Username
          <input name="username" required />
        </label>
        <label>
          Initial password
          <input name="password" type="password" minLength={10} required />
        </label>
        <label>
          Role
          <select name="role">
            <option>USER</option>
            <option>ADMIN</option>
          </select>
        </label>
        <button>Add user</button>
      </form>
    </section>
  );
}
function Audit() {
  const [items, setItems] = useState<AuditEvent[]>([]),
    [offset, setOffset] = useState(0);
  useEffect(() => {
    api.audit(offset).then((x) => setItems(x.events));
  }, [offset]);
  return (
    <section className="panel">
      <h2>Security audit</h2>
      {items.map((e) => (
        <div className="list-row" key={e.id}>
          <div>
            <strong>{e.event}</strong>
            <p>
              {new Date(e.timestamp).toLocaleString()} · Actor{" "}
              {e.actorUserId || "system"} ·{" "}
              {String(e.metadata.outcome || "recorded")}
            </p>
          </div>
        </div>
      ))}
      <div>
        <button
          disabled={!offset}
          onClick={() => setOffset(Math.max(0, offset - 25))}
        >
          Previous
        </button>{" "}
        <button
          disabled={items.length < 25}
          onClick={() => setOffset(offset + 25)}
        >
          Next
        </button>
      </div>
    </section>
  );
}

function Libraries() {
  const [items, setItems] = useState<Library[]>([]),
    [selected, setSelected] = useState<Library | null>(null),
    [files, setFiles] = useState<MediaFile[]>([]),
    [detail, setDetail] = useState<MediaFile | null>(null),
    [job, setJob] = useState<ScanJob | null>(null),
    [message, setMessage] = useState("");
  const load = () => api.libraries().then((x) => setItems(x.libraries));
  useEffect(() => {
    load();
  }, []);
  useEffect(() => {
    if (!job || !["QUEUED", "RUNNING"].includes(job.state)) return;
    const timer = setInterval(
      () =>
        api.scanStatus(job.libraryId, job.id).then((x) => {
          setJob(x);
          if (!["QUEUED", "RUNNING"].includes(x.state)) {
            load();
            api.items(x.libraryId).then((v) => setFiles(v.items));
          }
        }),
      750,
    );
    return () => clearInterval(timer);
  }, [job]);
  async function open(x: Library) {
    const full = await api.library(x.id);
    setSelected(full);
    setDetail(null);
    setFiles((await api.items(x.id)).items);
  }
  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget,
      d = new FormData(form),
      paths = String(d.get("paths"))
        .split(/\r?\n/)
        .map((x) => x.trim())
        .filter(Boolean);
    setMessage("Validating sources…");
    try {
      for (const path of paths) await api.validateSource(path);
      const x = await api.createLibrary({
        name: d.get("name"),
        type: d.get("type"),
        sources: paths,
      });
      form.reset();
      setMessage("Library created.");
      await load();
      open(x);
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Unable to create library",
      );
    }
  }
  async function add(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!selected) return;
    const form = e.currentTarget,
      path = String(new FormData(form).get("source"));
    try {
      await api.validateSource(path, selected.id);
      await api.addSource(selected.id, path);
      form.reset();
      open(selected);
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Unable to add source",
      );
    }
  }
  async function start() {
    if (!selected) return;
    try {
      const x = await api.scan(selected.id);
      setJob(x);
      setMessage("Scan started.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Unable to scan");
    }
  }
  async function remove() {
    if (
      !selected ||
      !confirm(
        `Remove ${selected.name} from VyNode? Media files will not be deleted.`,
      )
    )
      return;
    await api.deleteLibrary(selected.id);
    setSelected(null);
    setFiles([]);
    setMessage("Library removed. Files were left untouched.");
    load();
  }
  if (detail)
    return <MediaTechnical file={detail} back={() => setDetail(null)} />;
  return (
    <section className="panel">
      <h2>Libraries</h2>
      {!items.length && (
        <p>
          No libraries yet. Add a library to inventory physical media files.
        </p>
      )}
      {items.map((x) => (
        <button className="list-row" key={x.id} onClick={() => open(x)}>
          <span>
            <strong>{x.name}</strong>
            <small>
              {x.type} · {x.availableCount || 0} available ·{" "}
              {x.missingCount || 0} missing
            </small>
          </span>
        </button>
      ))}
      {selected && (
        <>
          <h2>{selected.name}</h2>
          {selected.sources?.map((x) => (
            <div className="list-row" key={x.id}>
              <span>
                {x.configuredPath} · {x.lastScanStatus || "NEVER"}
              </span>
              <button
                onClick={() =>
                  confirm(
                    "Remove this source from VyNode? Files remain untouched.",
                  ) &&
                  api.removeSource(selected.id, x.id).then(() => open(selected))
                }
              >
                Remove source
              </button>
            </div>
          ))}
          <form onSubmit={add}>
            <label>
              Add source
              <input
                name="source"
                required
                placeholder="/media/movies2 or E:\Movies"
              />
            </label>
            <button>Add source</button>
          </form>
          <button
            onClick={start}
            disabled={!!job && ["QUEUED", "RUNNING"].includes(job.state)}
          >
            Scan library
          </button>
          {job && ["QUEUED", "RUNNING"].includes(job.state) && (
            <button
              onClick={() =>
                api
                  .cancelScan(job.libraryId, job.id)
                  .then(() => setJob({ ...job, state: "CANCELED" }))
              }
            >
              Cancel scan
            </button>
          )}
          <button onClick={remove}>Remove library</button>
          {job && (
            <p role="status">
              {job.state}: {job.filesProbed} probed, {job.filesAdded} added,{" "}
              {job.filesUpdated} updated, {job.filesUnchanged} unchanged,{" "}
              {job.filesFailed} failed{" "}
              {job.currentRelativePath && `— ${job.currentRelativePath}`}
            </p>
          )}
          <h3>Physical inventory</h3>
          {files.map((f) => (
            <button
              className="list-row"
              key={f.id}
              onClick={() => api.mediaFile(f.id).then(setDetail)}
            >
              <span>
                <strong>{f.fileName}</strong>
                <small>
                  {f.relativePath} · {formatBytes(f.sizeBytes)} ·{" "}
                  {f.resolutionClass || "Unknown video"} · {f.availability} ·{" "}
                  {f.probeStatus}
                </small>
              </span>
            </button>
          ))}
        </>
      )}
      <form onSubmit={create}>
        <h2>Add Library</h2>
        <label>
          Type
          <select name="type">
            <option value="MOVIES">Movies</option>
            <option value="TV">TV</option>
          </select>
        </label>
        <label>
          Name
          <input name="name" required maxLength={100} />
        </label>
        <label>
          Server folder paths, one per line
          <textarea
            name="paths"
            required
            placeholder={"/media/movies\n/media/movies2"}
          />
        </label>
        <button>Create Library</button>
      </form>
      {message && <p role="status">{message}</p>}
    </section>
  );
}
function MediaTechnical({ file, back }: { file: MediaFile; back: () => void }) {
  return (
    <section className="panel">
      <button onClick={back}>Back to inventory</button>
      <h2>{file.fileName}</h2>
      <dl className="vertical">
        <div>
          <dt>Relative path</dt>
          <dd>{file.relativePath}</dd>
        </div>
        <div>
          <dt>Size</dt>
          <dd>{formatBytes(file.sizeBytes)}</dd>
        </div>
        <div>
          <dt>Container</dt>
          <dd>{file.containerFormat || "Unknown"}</dd>
        </div>
        <div>
          <dt>Duration</dt>
          <dd>{file.durationSeconds?.toFixed(2) || "Unknown"} seconds</dd>
        </div>
        <div>
          <dt>Resolution / HDR</dt>
          <dd>
            {file.resolutionClass || "Unknown"} / {file.hdrClass || "Unknown"}
          </dd>
        </div>
      </dl>
      <h3>Streams</h3>
      {file.streams?.map((s) => (
        <div className="list-row" key={s.id}>
          <div>
            <strong>
              {s.type} {s.index}: {s.codec}
            </strong>
            <p>
              {s.profile} {s.width && `${s.width}×${s.height}`}{" "}
              {s.channels && `${s.channels} channels`} {s.language}
            </p>
          </div>
        </div>
      ))}
      <h3>Preliminary filename hints</h3>
      <p>
        Unmatched observation only: {file.candidateTitle || "None"}{" "}
        {file.candidateYear || ""}{" "}
        {file.seasonNumber ? `S${file.seasonNumber}E${file.episodeStart}` : ""}
      </p>
    </section>
  );
}
function formatBytes(v: number) {
  if (v < 1024) return `${v} B`;
  if (v < 1024 ** 2) return `${(v / 1024).toFixed(1)} KiB`;
  if (v < 1024 ** 3) return `${(v / 1024 ** 2).toFixed(1)} MiB`;
  return `${(v / 1024 ** 3).toFixed(2)} GiB`;
}

function Player({
  type,
  id,
  go,
}: {
  type: "MOVIE" | "EPISODE";
  id: string;
  go: (p: Page) => void;
}) {
  const video = useRef<HTMLVideoElement>(null),
    sessionRef = useRef<PlaybackSession | null>(null),
    generatedAbort = useRef<AbortController | null>(null),
    generatedURL = useRef(""),
    generatedEpoch = useRef(0),
    internalSeek = useRef(false),
    hlsRef = useRef<InstanceType<typeof Hls> | null>(null),
    resumeAfterStart = useRef(false),
    [session, setSession] = useState<PlaybackSession | null>(null),
    [versions, setVersions] = useState<PlaybackVersion[]>([]),
    [choice, setChoice] = useState(""),
    [audio, setAudio] = useState(""),
    [subtitle, setSubtitle] = useState(""),
    [quality, setQuality] = useState(""),
    [position, setPosition] = useState(0),
    [playing, setPlaying] = useState(false),
    [diagnostics,setDiagnostics]=useState(false),
    [upNext,setUpNext]=useState<{seconds:number;canceled:boolean}|null>(null),
    [message, setMessage] = useState("Preparing direct play…");
  const report = (state: string) => {
    const v = video.current,
      s = sessionRef.current;
    if (s && v)
      api
        .updatePlayback(
          s.id,
          state,
          v.currentTime,
          Number.isFinite(v.duration) ? v.duration : s.duration,
        )
        .catch(() => {});
  };
  async function attachGeneratedStream(s: PlaybackSession, startAt: number, resumePlayback = false) {
    const v = video.current;
    if (!v || !s.mediaUrl) return;
    const epoch = ++generatedEpoch.current;
    internalSeek.current = true;
    generatedAbort.current?.abort();
    generatedAbort.current = new AbortController();
    if (generatedURL.current) URL.revokeObjectURL(generatedURL.current);
    const mediaSource = new MediaSource();
    generatedURL.current = URL.createObjectURL(mediaSource);
    v.src = generatedURL.current;
    setMessage(startAt > 0 ? "Preparing seek…" : "Preparing stream…");
    await new Promise<void>((resolve, reject) => {
      mediaSource.addEventListener("sourceopen", () => {
        try {
          const mime = [
            'video/mp4; codecs="avc1.64001f, mp4a.40.2"',
            'video/mp4; codecs="avc1.42E01E, mp4a.40.2"',
            "video/mp4",
          ].find(MediaSource.isTypeSupported);
          if (!mime) throw new Error("This browser cannot decode the generated MP4 stream.");
          const source = mediaSource.addSourceBuffer(mime);
          source.mode = "sequence";
          source.timestampOffset = startAt;
          source.appendWindowStart = startAt;
          source.appendWindowEnd = Math.max(startAt + 0.001, s.duration);
          if (s.duration > 0) mediaSource.duration = s.duration;
          const queue: ArrayBuffer[] = [];
          let ended = false,
            started = false;
          const pump = () => {
            if (source.updating) return;
            const chunk = queue.shift();
            if (chunk) source.appendBuffer(chunk);
            else if (ended && mediaSource.readyState === "open") mediaSource.endOfStream();
          };
          source.addEventListener("updateend", () => {
            if (epoch !== generatedEpoch.current) return;
            if (!started && source.buffered.length) {
              started = true;
              internalSeek.current = true;
              v.currentTime = startAt;
              setTimeout(() => { internalSeek.current = false; }, 250);
              if (resumePlayback) v.play().then(() => { setMessage("Playing"); resolve(); }).catch(reject);
              else { setMessage("Ready"); resolve(); }
            }
            pump();
          });
          fetch(`${s.mediaUrl}?start=${startAt.toFixed(3)}`, {
            credentials: "same-origin",
            signal: generatedAbort.current?.signal,
          }).then(async response => {
            if (!response.ok || !response.body) throw new Error(`Stream returned ${response.status}`);
            const reader = response.body.getReader();
            for (;;) {
              const part = await reader.read();
              if (part.done) break;
              queue.push(new Uint8Array(part.value).buffer as ArrayBuffer);
              pump();
            }
            ended = true;
            pump();
          }).catch(error => {
            if (error.name !== "AbortError") reject(error);
          });
        } catch (error) { reject(error); }
      }, { once: true });
    });
  }
  function seekGenerated(target: number, resumePlayback: boolean) {
    const s = sessionRef.current;
    if (!s) return;
    report("PLAYING");
    internalSeek.current = true;
    attachGeneratedStream(s, target, resumePlayback)
      .catch((e) => setMessage(e.message));
  }
  function seekTo(target: number) {
    const v = video.current,
      s = sessionRef.current;
    if (!v || !s) return;
    const bounded = Math.max(0, Math.min(target, s.duration || target));
    setPosition(bounded);
    if (s.decision.mode === "DIRECT_PLAY"||s.decision.mode === "VIDEO_TRANSCODE") v.currentTime = bounded;
    else seekGenerated(bounded, !v.paused);
  }
  async function start(version = choice, resume = true, nextQuality = quality, startPosition = 0) {
    const currentSession = sessionRef.current,
      currentVideo = video.current;
    resumeAfterStart.current = Boolean(currentSession && currentVideo && !currentVideo.paused);
    if (currentSession && currentVideo) {
      await api.updatePlayback(
        currentSession.id,
        "STOPPED",
        currentVideo.currentTime,
        Number.isFinite(currentVideo.duration) ? currentVideo.duration : currentSession.duration,
      );
	  sessionRef.current = null;
	  hlsRef.current?.destroy();
	  hlsRef.current = null;
	  currentVideo.pause();
	  currentVideo.removeAttribute("src");
	  currentVideo.load();
    }
    setMessage("Preparing direct play…");
    const s = await api.startPlayback(type, id, version, resume, audio, subtitle, nextQuality, startPosition, activePlaybackContext);
    activePlaybackContext=s.playbackContextId||activePlaybackContext;
    sessionRef.current = s;
    setSession(s);
    setAudio(s.selectedAudioTrack?.id||"");
    setSubtitle(s.selectedSubtitleTrack?.id||"");
    setUpNext(null);
    if (s.decision.mode === "UNSUPPORTED") {
      setMessage(reasonMessage(s.decision.reasons));
      return;
    }
    setMessage(s.decision.mode === "DIRECT_PLAY" ? "Loading…" : "Preparing stream…");
  }
  useEffect(() => {
    const v=video.current;
    if (!session||!v) return;
    hlsRef.current?.destroy();hlsRef.current=null;
    if(session.decision.mode==="VIDEO_TRANSCODE"&&session.hlsUrl){
      if(Hls.isSupported()){const h=new Hls({enableWorker:true});hlsRef.current=h;h.loadSource(session.hlsUrl);h.attachMedia(v);h.on(Hls.Events.MANIFEST_PARSED,()=>{if(session.resumePosition>0)v.currentTime=session.resumePosition;if(resumeAfterStart.current)v.play().catch(error=>setMessage(error.message));else setMessage("Ready")});h.on(Hls.Events.ERROR,(_event:unknown,data:{fatal:boolean;details:string})=>{if(data.fatal)setMessage(`Playback error: ${data.details}`)});return()=>h.destroy()}
      if(v.canPlayType("application/vnd.apple.mpegurl")){v.src=session.hlsUrl;return}
      setMessage("HLS playback is unavailable in this browser.");return
    }
    if(!session.mediaUrl)return;
    if (session.decision.mode==="DIRECT_PLAY") {
      v.load();
      v.currentTime=session.resumePosition;
      if(resumeAfterStart.current)v.play().catch(error=>setMessage(error.message));
    } else attachGeneratedStream(session,session.resumePosition).catch(e=>setMessage(e.message));
  }, [session]);
  useEffect(()=>{if(!upNext||upNext.canceled||!session?.navigation?.autoplay||!session.navigation.next)return;const timer=setTimeout(()=>{if(upNext.seconds<=1){go(`watch/episode/${session.navigation!.next!.logicalId}`)}else setUpNext({...upNext,seconds:upNext.seconds-1})},1000);return()=>clearTimeout(timer)},[upNext,session]);
  useEffect(() => {
    api.playbackVersions(type, id).then((x) => setVersions(x.versions));
    start("", true).catch((e) => setMessage(e.message));
    const timer = setInterval(() => report("PLAYING"), 12000);
    const keys = (e: KeyboardEvent) => {
      if ((e.target as HTMLElement)?.matches("input,select,textarea")) return;
      const v = video.current;
      if (!v) return;
      if (e.code === "Space") {
        e.preventDefault();
        v.paused ? v.play() : v.pause();
      } else if (e.key === "ArrowLeft")
        v.currentTime = Math.max(0, v.currentTime - 10);
      else if (e.key === "ArrowRight")
        v.currentTime = Math.min(v.duration || Infinity, v.currentTime + 10);
      else if (e.key === "ArrowUp") v.volume = Math.min(1, v.volume + 0.1);
      else if (e.key === "ArrowDown") v.volume = Math.max(0, v.volume - 0.1);
      else if (e.key.toLowerCase() === "m") v.muted = !v.muted;
      else if (e.key.toLowerCase() === "f")
        v.requestFullscreen().catch(() => {});
    };
    addEventListener("keydown", keys);
    return () => {
      clearInterval(timer);
      removeEventListener("keydown", keys);
      report("STOPPED");
      generatedAbort.current?.abort();
      hlsRef.current?.destroy();
      if (generatedURL.current) URL.revokeObjectURL(generatedURL.current);
      if (video.current) {
        video.current.pause();
        video.current.removeAttribute("src");
        video.current.load();
      }
      sessionRef.current = null;
    };
  }, [type, id]);
  const activeMarker=session?.markers?.find(m=>position>=m.start&&position<m.end&&(m.type==="INTRO"||m.type==="RECAP"||m.type==="CREDITS"));
  const explanation=session?reasonMessage(session.decision.reasons):"";
  return (
    <main className="player-shell">
      <div className="player-top">
        <button onClick={async () => {if(sessionRef.current)await api.stopPlayback(sessionRef.current.id).catch(()=>{});activePlaybackContext="";go(type === "MOVIE" ? `movies/${id}` : "shows")}}>
          ← Back
        </button>
        <strong>VyNode · {session?.decision.mode.replaceAll("_", " ") || "Playback"}</strong>
      </div>
      {session?.decision.mode !== "UNSUPPORTED" && (session?.mediaUrl||session?.hlsUrl) ? (
        <video
          ref={video}
          src={session.decision.mode === "DIRECT_PLAY" ? session.mediaUrl : undefined}
          controls
          autoPlay
          playsInline
          onPlaying={() => {
            setPlaying(true);
            setMessage("Playing");
          }}
          onTimeUpdate={() => {
            setPosition(video.current?.currentTime || 0);
          }}
          onWaiting={() => setMessage("Buffering…")}
          onPause={() => {
            setPlaying(false);
            setMessage("Paused");
            report("PAUSED");
          }}
          onSeeking={() => {
            const v=video.current,s=sessionRef.current;
            if (!v||!s||s.decision.mode==="DIRECT_PLAY"||s.decision.mode==="VIDEO_TRANSCODE"||internalSeek.current) return;
            const target=v.currentTime,resumePlayback=!v.paused;
            seekGenerated(target,resumePlayback);
          }}
          onSeeked={() => {
            internalSeek.current = false;
            report(video.current?.paused ? "PAUSED" : "PLAYING");
          }}
          onEnded={async () => {
            setMessage("Completed");
            const s=sessionRef.current;if(s)await api.updatePlayback(s.id,"COMPLETED",video.current?.duration||s.duration,video.current?.duration||s.duration).catch(()=>{});
            if(s?.navigation?.next)setUpNext({seconds:s.navigation.countdownSeconds||10,canceled:false});
          }}
          onError={() => {
            setMessage("Playback error");
            report("ERROR");
          }}
        >
          {session.subtitleUrl && <track kind="subtitles" src={session.subtitleUrl} default />}
        </video>
      ) : (
        <div className="player-message" role="alert">
          {message}
        </div>
      )}
      <div className="player-controls">
        <span>{message}</span>
        <label>
          Version
          <select value={choice} onChange={(e) => setChoice(e.target.value)}>
            <option value="">Auto</option>
            {versions.map((v) => (
              <option key={v.id} value={v.id}>
                {[
                  v.resolution,
                  v.videoCodec.toUpperCase(),
                  v.container.toUpperCase(),
                  v.hdr,
                  v.label,
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </option>
            ))}
          </select>
        </label>
        <button onClick={() => start(choice, true)}>Play selection</button>
        {session?.availableQualities?.length ? <label>Quality<select aria-label="Quality" value={quality} onChange={e=>{const next=e.target.value,current=video.current?.currentTime||0;setQuality(next);start(choice,true,next,current).catch(error=>setMessage(error.message))}}><option value="">Auto</option>{session.availableQualities.map(q=><option key={q.id} value={q.id}>{q.label}</option>)}</select></label>:null}
        <button onClick={()=>{const v=video.current;if(v){if(v.paused)v.play().catch(e=>setMessage(e.message));else v.pause()}}}>{playing?"Pause":"Play"}</button>
        <button onClick={()=>seekTo(position-10)}>−10s</button>
        <button onClick={()=>seekTo(position+10)}>+10s</button>
        {activeMarker&&<button className="skip-control" onClick={()=>seekTo(activeMarker.end)}>Skip {activeMarker.type==="INTRO"?"Intro":activeMarker.type==="RECAP"?"Recap":"Credits"}</button>}
        <label>Playback position<input aria-label="Playback position" type="range" min="0" max={Math.max(1,session?.duration||1)} step="0.1" value={Math.min(position,session?.duration||position)} onChange={e=>seekTo(Number(e.target.value))} /></label>
        <label>Audio<select value={audio} onChange={e=>setAudio(e.target.value)}><option value="">Default</option>{(versions.find(v=>v.id===choice)||session?.selectedVersion)?.audioTracks?.map(t=><option key={t.id} value={t.id}>{t.language||"Unknown"} · {t.codec.toUpperCase()} {t.channels?`· ${t.channels}ch`:""}{t.commentary?" · Commentary":""}</option>)}</select></label>
        <label>Subtitles<select value={subtitle} onChange={e=>setSubtitle(e.target.value)}><option value="">Off</option>{(versions.find(v=>v.id===choice)||session?.selectedVersion)?.subtitleTracks?.filter(t=>t.usable).map(t=><option key={t.id} value={t.id}>{t.language||t.title||"Subtitle"} · {t.codec.toUpperCase()}</option>)}</select></label>
        {session && session.resumePosition > 0 && (
          <button onClick={() => api.startOver(type,id).then(()=>start(choice,false))}>Start Over</button>
        )}
        <button aria-expanded={diagnostics} onClick={()=>setDiagnostics(!diagnostics)}>Playback info</button>
      </div>
      {upNext&&session?.navigation?.next&&<aside className="up-next" aria-live="polite"><strong>Up Next: S{String(session.navigation.next.seasonNumber).padStart(2,"0")}E{String(session.navigation.next.episodeNumber).padStart(2,"0")} · {session.navigation.next.title}</strong>{session.navigation.autoplay&&!upNext.canceled&&<span>Playing in {upNext.seconds}s</span>}<button onClick={()=>go(`watch/episode/${session.navigation!.next!.logicalId}`)}>Play Now</button>{session.navigation.autoplay&&!upNext.canceled&&<button onClick={()=>setUpNext({...upNext,canceled:true})}>Cancel</button>}</aside>}
      {diagnostics&&session&&<aside className="playback-diagnostics"><h2>Playback information</h2><dl><div><dt>Mode</dt><dd>{session.decision.mode.replaceAll("_"," ")}</dd></div><div><dt>Why</dt><dd>{explanation}</dd></div><div><dt>Quality</dt><dd>{session.decision.plan.quality||"Original"}{session.decision.plan.video.targetBitrate?` · ${(session.decision.plan.video.targetBitrate/1_000_000).toFixed(1)} Mbps`:""}</dd></div><div><dt>Video</dt><dd>{session.selectedVersion.videoCodec.toUpperCase()} {session.selectedVersion.width}×{session.selectedVersion.height}{session.decision.plan.video.targetCodec?` → ${session.decision.plan.video.targetCodec.toUpperCase()} ${session.decision.plan.video.targetWidth}×${session.decision.plan.video.targetHeight}`:""}</dd></div><div><dt>Audio</dt><dd>{session.selectedAudioTrack?.language||"Unknown"} · {session.decision.plan.audio.action}{session.decision.plan.audio.targetCodec?` → ${session.decision.plan.audio.targetCodec.toUpperCase()}`:""}</dd></div><div><dt>Subtitles</dt><dd>{session.selectedSubtitleTrack?`${session.selectedSubtitleTrack.language||"Unknown"}${session.selectedSubtitleTrack.forced?" · Forced":""}`:"Off"}</dd></div><div><dt>Network</dt><dd>{session.networkContext||"Unknown"}{session.effectiveBandwidthLimit?` · ${(session.effectiveBandwidthLimit/1_000_000).toFixed(1)} Mbps limit`:""}</dd></div><div><dt>Backend</dt><dd>{session.decision.plan.backend?.actual||"Not transcoding"}</dd></div></dl><button onClick={()=>start(choice,true,quality,position).catch(e=>setMessage(e.message))}>Retry stream</button></aside>}
    </main>
  );
}
function reasonMessage(reasons: { code: string; value?: string }[]) {
  const first = reasons[0];
  if (!first)
    return "The original streams are compatible with this browser.";
  if (first.code === "MEDIA_UNAVAILABLE")
    return "The media source is currently unavailable.";
  if (first.code === "VIDEO_CODEC_UNSUPPORTED")
    return `This version uses ${first.value}, which this browser did not report as supported.`;
  if (first.code === "CONTAINER_UNSUPPORTED")
    return `This browser did not report support for the ${first.value} container.`;
  if(first.code==="SOURCE_CONTAINER_UNSUPPORTED")return `The video is being repackaged because this browser does not support the ${first.value} container.`;
  if(first.code==="AUDIO_CODEC_UNSUPPORTED")return `Audio is being converted because this browser does not support ${first.value?.toUpperCase()}.`;
  if(first.code==="BITRATE_LIMIT_EXCEEDED")return "Video is being transcoded to obey the effective streaming bitrate limit.";
  if(first.code==="RESOLUTION_LIMIT_EXCEEDED")return "Video is being transcoded to fit the selected resolution limit.";
  if(first.code==="TRANSCODE_CAPACITY_REACHED")return "The server is currently at its video transcode limit. Try again shortly.";
  return first.code.replaceAll("_"," ").toLowerCase();
}
function directlyPlayable(v: PlaybackVersion) {
  const c = browserCapabilities();
  return (
    v.available &&
    c.directPlaySupport &&
    c.supportedContainers.includes(v.container) &&
    c.supportedVideoCodecs.includes(v.videoCodec) &&
    v.audioCodecs.every((x) => c.supportedAudioCodecs.includes(x)) &&
    (!c.maximumVideoWidth || !v.width || v.width <= c.maximumVideoWidth) &&
    (!c.maximumVideoHeight || !v.height || v.height <= c.maximumVideoHeight) &&
    (!v.hdr || v.hdr === "SDR" || c.hdrCapabilities.includes(v.hdr))
  );
}
function EpisodePlay({ id, go }: { id: string; go: (p: Page) => void }) {
  const [playable, setPlayable] = useState<boolean | null>(null);
  useEffect(() => {
    api
      .playbackVersions("EPISODE", id)
      .then((x) => setPlayable(x.versions.some(directlyPlayable)))
      .catch(() => setPlayable(false));
  }, [id]);
  return playable ? (
    <button onClick={() => go(`watch/episode/${id}`)}>Play</button>
  ) : playable === false ? (
    <small>Direct Play unsupported</small>
  ) : (
    <small>Checking playback…</small>
  );
}
function formatTime(value: number) {
  const total = Math.max(0, Math.floor(value)),
    h = Math.floor(total / 3600),
    m = Math.floor((total % 3600) / 60),
    s = total % 60;
  return h
    ? `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`
    : `${m}:${String(s).padStart(2, "0")}`;
}
function ActiveStreams() {
  const [items, setItems] = useState<PlaybackSession[]>([]);
  const load = () => api.activePlayback().then((x) => setItems(x.sessions));
  useEffect(() => {
    load();
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, []);
  return (
    <section className="panel">
      <h2>Active Streams</h2>
      {!items.length && <p>No active playback sessions.</p>}
      {items.map((x) => (
        <div className="list-row" key={x.id}>
          <div>
            <strong>{x.userDisplayName || "Unknown user"} · {x.title || x.logicalType}</strong>
            <p>
              {x.clientName || "Unknown client"}{x.platform && ` on ${x.platform}`} · {x.networkContext || "Unknown network"} · Started {new Date(x.startedAt).toLocaleString()}<br/>
              {x.decision.mode.replaceAll("_", " ")} · {x.state} · {formatTime(x.position)} / {formatTime(x.duration)} ·{" "}
              {x.selectedVersion?.resolution} {x.selectedVersion?.videoCodec}
              {x.selectedAudioTrack?.language && <> · Audio {x.selectedAudioTrack.language}</>}
              {x.selectedSubtitleTrack ? <> · Subtitles {x.selectedSubtitleTrack.language || x.selectedSubtitleTrack.title || "selected"}</> : <> · Subtitles off</>}
              {x.decision.mode !== "DIRECT_PLAY" && <> · Video {x.decision.plan.video.action} · Audio {x.decision.plan.audio.sourceCodec}{x.decision.plan.audio.targetCodec && ` → ${x.decision.plan.audio.targetCodec}`} · {x.decision.plan.container.target}</>}
              {x.decision.mode === "VIDEO_TRANSCODE" && <> · {x.decision.plan.video.sourceCodec?.toUpperCase()} → {x.decision.plan.video.targetCodec?.toUpperCase()} · {x.decision.plan.video.targetHeight}p · {x.decision.plan.backend?.actual || "SOFTWARE"}</>}
            </p>
          </div>
          <button onClick={() => window.confirm("Stop this playback session?") && api.adminStopPlayback(x.id).then(load)}>
            Stop
          </button>
        </div>
      ))}
    </section>
  );
}

const bytes=(n:number)=>n?`${(n/1073741824).toFixed(1)} GB`:`—`;
function AdminDashboardPage({go}:{go:(p:string)=>void}){
  const [d,setD]=useState<import("./api").AdminDashboard|null>(null),[error,setError]=useState("");
  const load=()=>api.adminDashboard().then(setD).catch(e=>setError(e.message));
  useEffect(()=>{load();const t=setInterval(load,5000);return()=>clearInterval(t)},[]);
  if(error)return <section className="panel"><h2>Dashboard</h2><p className="form-error">{error}</p></section>;
  if(!d)return <section className="panel loading"><div/><div/></section>;
  const issues=Object.values(d.Health).reduce((a,b)=>a+b,0);
  return <section className="content admin-console"><div className="admin-heading"><div><p className="eyebrow">OPERATIONS</p><h2>{d.ServerName}</h2></div><button onClick={load}>Refresh</button></div>
    <div className="metric-grid"><button onClick={()=>go("streams")}><span>Active streams</span><strong>{d.Metrics.ActivePlaybackSessions}</strong><small>{d.Metrics.ActiveFFmpegProcesses} FFmpeg</small></button><button onClick={()=>go("admin/health")}><span>Needs attention</span><strong>{issues}</strong><small>{d.Health.ERROR||0} errors · {d.Health.WARNING||0} warnings</small></button><button onClick={()=>go("admin/jobs")}><span>Failed jobs</span><strong>{d.FailedJobs}</strong><small>Open job history</small></button><div><span>Uptime</span><strong>{Math.floor(d.Metrics.UptimeSeconds/3600)}h</strong><small>{d.Metrics.OperatingSystem} · {d.Metrics.Architecture}</small></div></div>
    <div className="ops-grid"><section><h3>Server</h3><table><tbody><tr><th>Version</th><td>{d.Version} ({d.Commit})</td></tr><tr><th>Database</th><td>{d.DatabaseType}</td></tr><tr><th>Memory</th><td>{d.Metrics.ProcessRSSBytes?`${bytes(d.Metrics.ProcessRSSBytes)} process RSS · `:""}{bytes(d.Metrics.GoSystemBytes)} Go reserved · {d.Metrics.GoRoutines} goroutines</td></tr><tr><th>System memory</th><td>{d.Metrics.SystemMemoryTotalBytes?`${bytes(d.Metrics.SystemMemoryAvailableBytes)} / ${bytes(d.Metrics.SystemMemoryTotalBytes)} available`:"Not supported on this platform"}</td></tr><tr><th>FFmpeg</th><td className="clip">{d.FFmpegVersion}</td></tr><tr><th>FFprobe</th><td className="clip">{d.FFprobeVersion}</td></tr></tbody></table></section>
    <section><h3>Library</h3><table><tbody><tr><th>Logical</th><td>{d.Libraries.Movies} movies · {d.Libraries.Shows} shows · {d.Libraries.Episodes} episodes</td></tr><tr><th>Physical</th><td>{d.Libraries.AvailableFiles}/{d.Libraries.PhysicalFiles} available</td></tr><tr><th>Missing</th><td>{d.Libraries.MissingFiles}</td></tr><tr><th>Unmatched</th><td>{d.Libraries.UnmatchedFiles}</td></tr><tr><th>Optimized</th><td>{d.Libraries.OptimizedVersions}</td></tr></tbody></table></section>
    <section><h3>Storage</h3>{d.Metrics.Disks.length?d.Metrics.Disks.map(x=><div className="storage-row" key={x.Label}><strong>{x.Label}</strong><progress value={x.UsedBytes} max={x.TotalBytes}/><span>{bytes(x.AvailableBytes)} free</span></div>):<p>No measurable configured filesystems.</p>}</section>
    <section><h3>Recent activity</h3>{d.RecentEvents.length?d.RecentEvents.map(x=><div className="event-row" key={x.ID}><b className={`severity ${x.Severity.toLowerCase()}`}>{x.Severity}</b><span>{x.Type.replaceAll("_"," ")}</span><time>{new Date(x.CreatedAt).toLocaleString()}</time></div>):<p>No operational events yet.</p>}</section></div></section>
}
function AnalyticsPage(){const [days,setDays]=useState(7),[x,setX]=useState<import("./api").PlaybackAnalytics|null>(null);useEffect(()=>{api.playbackAnalytics(days).then(setX)},[days]);return <section className="content admin-console"><div className="admin-heading"><h2>Playback analytics</h2><select value={days} onChange={e=>setDays(Number(e.target.value))}><option value="1">Today</option><option value="7">7 days</option><option value="30">30 days</option><option value="90">90 days</option></select></div>{x&&<><div className="metric-grid"><div><span>Plays</span><strong>{x.TotalPlays}</strong></div><div><span>Playback time</span><strong>{Math.round(x.PlaybackSeconds/3600)}h</strong></div><div><span>Users</span><strong>{x.UniqueUsers}</strong></div><div><span>Errors</span><strong>{x.PlaybackErrors}</strong></div></div><div className="ops-grid"><section><h3>Delivery modes</h3>{Object.entries(x.Modes).map(([k,n])=><div className="bar-row" key={k}><span>{k.replaceAll("_"," ")}</span><progress value={n} max={Math.max(1,x.TotalPlays)}/><b>{n}</b></div>)}</section><section><h3>Completion</h3><table><tbody><tr><th>Completed</th><td>{x.CompletionCount}</td></tr><tr><th>Movies</th><td>{x.MoviesPlayed}</td></tr><tr><th>Episodes</th><td>{x.EpisodesPlayed}</td></tr></tbody></table></section><section><h3>Top media</h3>{x.TopMedia.map(i=><div className="event-row" key={i.Key}><span>{i.Label}</span><b>{i.Count}</b></div>)}</section><section><h3>Top users</h3>{x.TopUsers.map(i=><div className="event-row" key={i.Key}><span>{i.Label}</span><b>{i.Count}</b></div>)}</section></div></>}</section>}
function HealthPage(){const [items,setItems]=useState<import("./api").HealthIssue[]>([]),[busy,setBusy]=useState(false);const load=()=>api.healthIssues().then(x=>setItems(x.issues));useEffect(()=>{load()},[]);const scan=()=>{setBusy(true);api.reevaluateHealth().then(x=>setItems(x.issues)).finally(()=>setBusy(false))};return <section className="content admin-console"><div className="admin-heading"><div><h2>Library health</h2><p>Detected local conditions, with source-offline suppression.</p></div><button disabled={busy} onClick={scan}>{busy?"Evaluating…":"Reevaluate health"}</button></div><div className="filter-line"><span>{items.filter(x=>x.Status==="OPEN").length} open</span><span>{items.filter(x=>x.Status==="IGNORED").length} ignored</span><span>{items.filter(x=>x.Status==="RESOLVED").length} resolved</span></div><div className="ops-table"><table><thead><tr><th>Severity</th><th>Category</th><th>Description</th><th>Status</th><th>Detected</th><th/></tr></thead><tbody>{items.map(x=><tr key={x.ID}><td><b className={`severity ${x.Severity.toLowerCase()}`}>{x.Severity}</b></td><td>{x.Category.replaceAll("_"," ")}</td><td>{x.Description}</td><td>{x.Status}</td><td>{new Date(x.LastDetectedAt).toLocaleString()}</td><td>{x.Status!=="RESOLVED"&&<button onClick={()=>api.setHealthIgnored(x.ID,x.Status!=="IGNORED").then(load)}>{x.Status==="IGNORED"?"Unignore":"Ignore"}</button>}</td></tr>)}</tbody></table></div>{!items.length&&<p>No health issues detected.</p>}</section>}
function JobsPage(){const [items,setItems]=useState<import("./api").AdminJob[]>([]);const load=()=>api.adminJobs().then(x=>setItems(x.jobs));useEffect(()=>{load();const t=setInterval(load,5000);return()=>clearInterval(t)},[]);return <section className="content admin-console"><div className="admin-heading"><h2>Background jobs</h2><button onClick={load}>Refresh</button></div><div className="ops-table"><table><thead><tr><th>Type</th><th>Target</th><th>Status</th><th>Progress</th><th>Started</th><th>Duration / error</th></tr></thead><tbody>{items.map(x=><tr key={`${x.Type}-${x.ID}`}><td>{x.Type}</td><td className="mono">{x.Target}</td><td>{x.State}</td><td><progress value={x.Progress} max="1"/> {Math.round(x.Progress*100)}%</td><td>{x.StartedAt?new Date(x.StartedAt).toLocaleString():"Queued"}</td><td>{x.Error||x.CompletedAt&&new Date(x.CompletedAt).toLocaleString()||"—"}</td></tr>)}</tbody></table></div>{!items.length&&<p>No background jobs.</p>}</section>}
function NotificationsPage(){const [items,setItems]=useState<import("./api").WebhookDestination[]>([]),[deliveries,setDeliveries]=useState<import("./api").WebhookDelivery[]>([]),[catalog,setCatalog]=useState<Record<string,{Type:string;Category:string;Severity:string}>>({}),[message,setMessage]=useState("");const load=()=>Promise.all([api.webhookDestinations().then(x=>setItems(x.destinations)),api.webhookDeliveries().then(x=>setDeliveries(x.deliveries)),api.webhookCatalog().then(x=>setCatalog(x.events))]);useEffect(()=>{load()},[]);const save=(e:FormEvent<HTMLFormElement>)=>{e.preventDefault();const f=e.currentTarget,d=new FormData(f),events=[...d.getAll("events")].map(String);api.saveWebhook({Name:String(d.get("name")),URL:String(d.get("url")),Enabled:true,AllowPrivateNetwork:d.get("private")==="on",AllowInsecureHTTP:d.get("http")==="on",MaxAttempts:Number(d.get("attempts")),EventTypes:events,Secret:String(d.get("secret"))}).then(()=>{f.reset();setMessage("Webhook saved.");load()}).catch(e=>setMessage(e.message))};const groups=Object.entries(catalog).reduce<Record<string,string[]>>((a,[k,v])=>{(a[v.Category]??=[]).push(k);return a},{});return <section className="content admin-console"><h2>Notifications & webhooks</h2><p>Provider-neutral, signed event delivery. Public HTTPS destinations are the safe default.</p><div className="ops-grid"><section><h3>Add webhook</h3><form onSubmit={save}><label>Name<input name="name" required/></label><label>Destination URL<input name="url" type="url" required placeholder="https://hooks.example.net/vynode"/></label><label>Signing secret<input name="secret" type="password" autoComplete="new-password"/></label><label>Attempts<select name="attempts" defaultValue="3"><option>1</option><option>2</option><option>3</option><option>4</option><option>5</option></select></label><label className="check"><input name="private" type="checkbox"/> Allow private/LAN destinations (SSRF-sensitive)</label><label className="check"><input name="http" type="checkbox"/> Allow insecure HTTP</label>{Object.entries(groups).map(([g,events])=><fieldset key={g}><legend>{g}</legend>{events.map(t=><label className="check" key={t}><input type="checkbox" name="events" value={t}/>{t.replaceAll("_"," ")}</label>)}</fieldset>)}<button>Save webhook</button><p>{message}</p></form></section><section><h3>Destinations</h3>{items.map(x=><div className="list-row" key={x.ID}><div><strong>{x.Name}</strong><p>{x.URL}<br/>{x.Enabled?"Enabled":"Disabled"} · {x.EventTypes.length} subscriptions · secret {x.HasSecret?"set":"not set"}</p></div><span><button onClick={()=>api.testWebhook(x.ID!).then(()=>{setMessage("Test queued.");setTimeout(load,1500)})}>Test</button><button onClick={()=>confirm(`Delete ${x.Name}?`)&&api.deleteWebhook(x.ID!).then(load)}>Delete</button></span></div>)}</section></div><h3>Delivery history</h3><div className="ops-table"><table><thead><tr><th>Event</th><th>Destination</th><th>Status</th><th>Attempts</th><th>HTTP</th><th>Time / diagnostic</th></tr></thead><tbody>{deliveries.map(x=><tr key={x.ID}><td>{x.EventType}</td><td>{x.DestinationName}</td><td>{x.Status}</td><td>{x.AttemptCount}</td><td>{x.LastHTTPStatus||"—"}</td><td>{new Date(x.CreatedAt).toLocaleString()} {x.LastError&&`· ${x.LastError}`}</td></tr>)}</tbody></table></div></section>}

function ArtworkImage({
  kind,
  id,
  type,
  title,
  className,
}: {
  kind: "movies" | "shows";
  id: string;
  type: "POSTER" | "BACKDROP";
  title: string;
  className?: string;
}) {
  const [src, setSrc] = useState("");
  useEffect(() => {
    let url = "";
    api
      .artwork(kind, id)
      .then(async (x) => {
        const selected = x.artwork.find(
          (a) => a.type === type && a.selected && a.cached,
        );
        if (selected) {
          url = URL.createObjectURL(await api.artworkBlob(selected.id));
          setSrc(url);
        }
      })
      .catch(() => setSrc(""));
    return () => {
      if (url) URL.revokeObjectURL(url);
    };
  }, [kind, id, type]);
  return src ? (
    <img
      className={className || "poster-image"}
      src={src}
      alt={`${title} ${type.toLowerCase()}`}
    />
  ) : (
    <div
      className={
        className === "backdrop-image"
          ? "backdrop-fallback"
          : "poster-placeholder"
      }
      aria-label={`${title} artwork unavailable`}
    >
      {type === "POSTER" ? title.slice(0, 1) : ""}
    </div>
  );
}
function MediaCard({
  kind,
  id,
  title,
  year,
  go,
}: {
  kind: "movies" | "shows";
  id: string;
  title: string;
  year?: number;
  go?: (p: Page) => void;
}) {
  return (
    <button
      className="media-card"
      onClick={() =>
        go ? go(`${kind}/${id}`) : location.assign(`/${kind}/${id}`)
      }
    >
      <ArtworkImage kind={kind} id={id} type="POSTER" title={title} />
      <strong>{title}</strong>
      <small>{year || "Year unknown"}</small>
    </button>
  );
}
function Movies({ go }: { go: (p: Page) => void }) {
  const [items, setItems] = useState<Movie[]>([]);
  useEffect(() => {
    api.movies().then((x) => setItems(x.movies));
  }, []);
  return (
    <section className="panel media-panel">
      <h2>Movies</h2>
      {!items.length && <p>No identified movies yet.</p>}
      <div className="poster-grid">
        {items.map((x) => (
          <MediaCard
            key={x.id}
            kind="movies"
            id={x.id}
            title={x.title}
            year={x.year}
            go={go}
          />
        ))}
      </div>
    </section>
  );
}
function MovieDetail({ id, go }: { id: string; go: (p: Page) => void }) {
  const [item, setItem] = useState<Movie | null>(null),
    [progress, setProgress] = useState<{
      position: number;
      watched: boolean;
    } | null>(null),
    [playable, setPlayable] = useState<boolean | null>(null);
  useEffect(() => {
    api.movie(id).then(setItem);
    api.progress("MOVIE", id).then(setProgress);
    api
      .playbackVersions("MOVIE", id)
      .then((x) => setPlayable(x.versions.some(directlyPlayable)))
      .catch(() => setPlayable(false));
  }, [id]);
  if (!item)
    return (
      <section className="panel loading">
        <div />
        <div />
      </section>
    );
  return (
    <section className="panel detail-panel">
      <ArtworkImage
        kind="movies"
        id={id}
        type="BACKDROP"
        title={item.title}
        className="backdrop-image"
      />
      <button onClick={() => go("movies")}>Back to movies</button>
      <div className="detail-grid">
        <ArtworkImage kind="movies" id={id} type="POSTER" title={item.title} />
        <div>
          <h2>
            {item.title} {item.year && `(${item.year})`}
          </h2>
          <CurationActions type="MOVIE" id={id}/>
          <p>
            {[
              item.runtimeMinutes && `${item.runtimeMinutes} min`,
              item.contentRating,
              item.rating && `TMDb ${item.rating.toFixed(1)}`,
            ]
              .filter(Boolean)
              .join(" · ")}
          </p>
          <p>{item.overview || "No overview available."}</p>
          <p>{item.genres?.join(" · ")}</p>
          {playable && (
            <button onClick={() => go(`watch/movie/${id}`)}>
              {progress && progress.position >= 30
                ? `Resume at ${formatTime(progress.position)}`
                : "Play"}
            </button>
          )}
          <DownloadAction type="MOVIE" id={id}/>
          {playable === false && (
            <p>
              This browser cannot directly play any available version yet.
              Transcoding support is coming in a later phase.
            </p>
          )}{" "}
          <button
            onClick={() =>
              api
                .markWatched("MOVIE", id, !progress?.watched)
                .then(() => api.progress("MOVIE", id).then(setProgress))
            }
          >
            {progress?.watched ? "Mark Unwatched" : "Mark Watched"}
          </button>
        </div>
      </div>
      <h3>Physical versions</h3>
      {item.versions?.map((v) => (
        <div className="list-row" key={v.id}>
          <span>
            {[v.resolution, v.codec, v.hdr, v.label]
              .filter(Boolean)
              .join(" · ") || "Media version"}
          </span>
        </div>
      ))}
      <ArtworkManager kind="movies" id={id} />
      <MarkerManager type="MOVIE" id={id}/>
      <OptimizationManager type="MOVIE" id={id} versions={item.versions??[]}/>
    </section>
  );
}
function Shows({ go }: { go: (p: Page) => void }) {
  const [items, setItems] = useState<Show[]>([]);
  useEffect(() => {
    api.shows().then((x) => setItems(x.shows));
  }, []);
  return (
    <section className="panel media-panel">
      <h2>Shows</h2>
      {!items.length && <p>No identified shows yet.</p>}
      <div className="poster-grid">
        {items.map((x) => (
          <MediaCard
            key={x.id}
            kind="shows"
            id={x.id}
            title={x.title}
            year={x.year}
            go={go}
          />
        ))}
      </div>
    </section>
  );
}
function ShowDetail({ id, go }: { id: string; go: (p: Page) => void }) {
  const [item, setItem] = useState<Show | null>(null);
  useEffect(() => {
    api.show(id).then(setItem);
  }, [id]);
  if (!item)
    return (
      <section className="panel loading">
        <div />
        <div />
      </section>
    );
  return (
    <section className="panel detail-panel">
      <ArtworkImage
        kind="shows"
        id={id}
        type="BACKDROP"
        title={item.title}
        className="backdrop-image"
      />
      <button onClick={() => go("shows")}>Back to shows</button>
      <div className="detail-grid">
        <ArtworkImage kind="shows" id={id} type="POSTER" title={item.title} />
        <div>
          <h2>{item.title}</h2>
          <CurationActions type="SHOW" id={id}/>
          <p>{item.overview || "No overview available."}</p>
        </div>
      </div>
      {item.seasons?.map((s) => (
        <section key={s.id}>
          <h3>
            {s.seasonNumber === 0 ? "Specials" : `Season ${s.seasonNumber}`}
          </h3>
          {s.episodes.map((e) => (
            <div className="list-row" key={e.id}>
              <div>
                <strong>
                  {e.episodeNumber}. {e.title}
                </strong>
                <p>
                  {e.airDate} · {e.available ? "Available" : "Unavailable"}
                  <br />
                  {e.overview}
                </p>
              </div>
              {e.available && <span><EpisodePlay id={e.id} go={go} /><DownloadAction type="EPISODE" id={e.id}/></span>}
              {e.available && <MarkerManager type="EPISODE" id={e.id}/>}
            </div>
          ))}
        </section>
      ))}
      <ArtworkManager kind="shows" id={id} />
      <AnalysisManager targetType="SHOW" targetId={id}/>
    </section>
  );
}
function DownloadAction({type,id}:{type:"MOVIE"|"EPISODE";id:string}){const [profile,setProfile]=useState("MEDIUM"),[message,setMessage]=useState("");return <span><select aria-label="Offline quality" value={profile} onChange={e=>setProfile(e.target.value)}><option value="ORIGINAL">Original</option><option value="HIGH">High 1080p</option><option value="MEDIUM">Medium 720p</option><option value="LOW">Low 480p</option></select><button onClick={()=>api.createOfflineDownload(type,id,profile).then(x=>setMessage(x.assetState==="READY"?"Ready to download on this device.":"Offline version is preparing.")).catch(e=>setMessage(e.message))}>Download</button>{message&&<small role="status">{message}</small>}</span>}
function MarkerManager({type,id}:{type:"MOVIE"|"EPISODE";id:string}){
  const [admin,setAdmin]=useState(false),[markers,setMarkers]=useState<import("./api").MediaMarker[]>([]),[message,setMessage]=useState("");
  const load=()=>api.markers(type,id).then(x=>setMarkers(x.markers ?? []));
  useEffect(()=>{api.me().then(u=>{if(u.role!=="USER"){setAdmin(true);load()}})},[type,id]);
  if(!admin)return null;
  return <details className="marker-manager"><summary>Manual playback markers</summary>{markers.map(m=><div className="list-row" key={m.id}><span>{m.type} · {formatTime(m.start)}–{formatTime(m.end)} · {m.source}</span><span><button onClick={()=>{const start=prompt("Marker start in seconds",String(m.start)),end=prompt("Marker end in seconds",String(m.end));if(start!==null&&end!==null)api.updateMarker(m.id,{...m,start:Number(start),end:Number(end)}).then(load)}}>Edit</button> <button onClick={()=>api.deleteMarker(m.id).then(load)}>Delete</button></span></div>)}<form onSubmit={e=>{e.preventDefault();const form=e.currentTarget,d=new FormData(form);api.saveMarker({logicalType:type,logicalId:id,type:String(d.get("type")) as import("./api").MediaMarker["type"],start:Number(d.get("start")),end:Number(d.get("end"))}).then(()=>{form.reset();setMessage("Marker saved.");load()}).catch(x=>setMessage(x.message))}}><label>Marker type<select name="type"><option value="INTRO">Intro</option><option value="RECAP">Recap</option><option value="CREDITS">Credits</option><option value="POST_CREDITS">Post-credits</option></select></label><label>Start time (seconds)<input name="start" type="number" min="0" step="0.1" required/></label><label>End time (seconds)<input name="end" type="number" min="0.1" step="0.1" required/></label><button>Add manual marker</button>{message&&<span role="status">{message}</span>}</form></details>
}

function AnalysisManager({targetType,targetId}:{targetType:string;targetId:string}){const [admin,setAdmin]=useState(false),[message,setMessage]=useState("");useEffect(()=>{api.me().then(x=>setAdmin(x.role!=="USER"))},[]);if(!admin)return null;return <section><h3>Automatic marker analysis</h3><button onClick={()=>api.analyzeMarkers(targetType,targetId).then(j=>setMessage(`Analysis ${j.State.toLowerCase()}.`)).catch(x=>setMessage(x.message))}>Reanalyze {targetType.toLowerCase()}</button>{message&&<span role="status">{message}</span>}</section>}
function OptimizationManager({type,id,versions}:{type:"MOVIE"|"EPISODE";id:string;versions:import("./api").MediaVersion[]}){const [admin,setAdmin]=useState(false),[profile,setProfile]=useState("mobile-720p"),[message,setMessage]=useState("");useEffect(()=>{api.me().then(x=>setAdmin(x.role!=="USER"))},[]);if(!admin||!versions.length)return null;return <section><h3>Optimized versions</h3><p>Creates a derivative in VyNode-controlled storage. The original is never changed.</p><label>Profile<select value={profile} onChange={e=>setProfile(e.target.value)}><option value="mobile-480p">Mobile 480p</option><option value="mobile-720p">Mobile 720p</option><option value="remote-1080p">Remote 1080p</option><option value="compatible-h264">Compatible H.264</option></select></label><button onClick={()=>api.optimize({logicalType:type,logicalId:id,sourceMediaFileId:versions[0].fileId,profile}).then(j=>setMessage(`Optimization ${j.State.toLowerCase()}.`)).catch(x=>setMessage(x.message))}>Optimize</button>{message&&<span role="status">{message}</span>}</section>}

function AutomationAdmin(){
  const [rules,setRules]=useState<import("./api").AutomationRule[]>([]),[jobs,setJobs]=useState<import("./api").IntelligenceJob[]>([]),[optimized,setOptimized]=useState<import("./api").OptimizedMedia[]>([]),[candidates,setCandidates]=useState<import("./api").MarkerCandidate[]>([]),[auto,setAuto]=useState(false),[message,setMessage]=useState("");
  const load=()=>Promise.all([api.automationRules(),api.intelligenceJobs(),api.optimizedMedia(),api.markerReview(),api.markerPolicy()]).then(([r,j,o,c,p])=>{setRules(r.rules??[]);setJobs(j.jobs??[]);setOptimized(o.items??[]);setCandidates(c.candidates??[]);setAuto(p.automaticallyActivateHighConfidence)});
  useEffect(()=>{load()},[]);
  return <section className="panel"><h2>Automation</h2><p>Local analysis and optimization run below active playback priority. Manual markers are always authoritative.</p><label><input type="checkbox" checked={auto} onChange={e=>api.setMarkerPolicy(e.target.checked).then(()=>{setAuto(e.target.checked);setMessage("Marker policy saved.")})}/> Automatically activate high-confidence markers</label>{message&&<p role="status">{message}</p>}
    <h3>Marker review</h3>{!candidates.length&&<p>No candidates need review.</p>}{candidates.map(c=><div className="list-row" key={c.ID}><span><strong>{c.Type}</strong> · {c.LogicalType} {c.LogicalID}<br/>{formatTime(c.Start)}–{formatTime(c.End)} · {c.ConfidenceClass} ({Math.round(c.Confidence*100)}%) · {c.Source}</span><span><button onClick={()=>api.reviewAutomaticMarker(c.ID,"ACCEPT").then(load)}>Accept</button> <button onClick={()=>{const start=prompt("Start seconds",String(c.Start)),end=prompt("End seconds",String(c.End));if(start&&end)api.reviewAutomaticMarker(c.ID,"ADJUST",Number(start),Number(end)).then(load)}}>Adjust</button> <button onClick={()=>api.reviewAutomaticMarker(c.ID,"REJECT").then(load)}>Reject</button></span></div>)}
    <h3>Rules</h3>{rules.map(r=><div className="list-row" key={r.ID}><span><strong>{r.Name}</strong> · {r.Trigger} · {r.Enabled?"Enabled":"Disabled"}<br/>{r.LastExecutionAt?`Last run ${r.LastExecutionAt}`:"Never run"}</span><span><button onClick={()=>api.dryRunAutomation(r).then(x=>setMessage(`Dry run: ${x.matches.length} matches, ${x.actionsExecuted} actions executed.`))}>Dry Run</button> <button onClick={()=>r.ID&&api.executeAutomation(r.ID).then(x=>{setMessage(`Executed: ${x.matches.length} matches, ${x.actionsExecuted} actions.`);load()})}>Run Now</button> <button onClick={()=>api.saveAutomationRule({...r,Enabled:!r.Enabled}).then(load)}>{r.Enabled?"Disable":"Enable"}</button> <button onClick={()=>r.ID&&api.deleteAutomationRule(r.ID).then(load)}>Delete</button></span></div>)}
    <form onSubmit={e=>{e.preventDefault();const form=e.currentTarget,d=new FormData(form);const codec=String(d.get("codec")),profile=String(d.get("profile")),trigger=String(d.get("trigger"));api.saveAutomationRule({Name:String(d.get("name")),Enabled:true,Trigger:trigger,Timezone:"UTC",Schedule:trigger==="SCHEDULE"?{Hour:Number(d.get("hour")),Minute:Number(d.get("minute"))}:undefined,Conditions:codec?[{Field:"codec",Operator:"EQUALS",Value:codec}]:[],Actions:[{Type:String(d.get("action")),Profile:profile||undefined}]}).then(()=>{form.reset();load()}).catch(x=>setMessage(x.message))}}><h3>Create structured rule</h3><label>Name<input name="name" required/></label><label>Trigger<select name="trigger"><option>MEDIA_IDENTIFIED</option><option>MEDIA_ADDED</option><option>METADATA_REFRESHED</option><option>SCAN_COMPLETED</option><option>SCHEDULE</option></select></label><label>Schedule hour (UTC)<input name="hour" type="number" min="0" max="23" defaultValue="0"/></label><label>Schedule minute<input name="minute" type="number" min="0" max="59" defaultValue="0"/></label><label>Video codec condition<input name="codec" placeholder="hevc"/></label><label>Action<select name="action"><option>CREATE_OPTIMIZED_VERSION</option><option>RUN_MARKER_ANALYSIS</option></select></label><label>Optimization profile<select name="profile"><option value="mobile-720p">Mobile 720p</option><option value="mobile-480p">Mobile 480p</option><option value="remote-1080p">Remote 1080p</option><option value="compatible-h264">Compatible H.264</option></select></label><button>Create rule</button></form>
    <h3>Background work</h3>{jobs.map(j=><div className="list-row" key={j.ID}><span>{j.Type} · {j.TargetType} {j.TargetID}<br/>{j.State} · {Math.round(j.Progress*100)}% {j.Error}</span></div>)}
    <h3>Managed optimized versions</h3>{!optimized.length&&<p>No optimized versions.</p>}{optimized.map(o=><div className="list-row" key={o.ID}><span>{o.LogicalType} {o.LogicalID} · {o.Profile}<br/>{o.Status} · {(o.SizeBytes/1048576).toFixed(1)} MB</span><button onClick={()=>confirm("Delete this optimized copy? The original will remain untouched.")&&api.deleteOptimized(o.ID).then(load)}>Delete copy</button></div>)}
  </section>
}

function CurationCard({item}:{item:import("./api").CurationItem}){const path=item.Type==="MOVIE"?`/movies/${item.ID}`:item.Type==="SHOW"?`/shows/${item.ID}`:`/watch/episode/${item.ID}`;return <a className="media-card" href={path}>{item.Type!=="EPISODE"?<ArtworkImage kind={item.Type==="MOVIE"?"movies":"shows"} id={item.ID} type="POSTER" title={item.Title}/>:<div className="poster-fallback">E</div>}<strong>{item.Title}</strong><span>{item.Subtitle||item.Year||item.Type}</span></a>}
function CurationActions({type,id}:{type:"MOVIE"|"SHOW";id:string}){const [watch,setWatch]=useState(false),[favorite,setFavorite]=useState(false);useEffect(()=>{Promise.all([api.personal("watchlist"),api.personal("favorites")]).then(([w,f])=>{setWatch(w.items.some(x=>x.Type===type&&x.ID===id));setFavorite(f.items.some(x=>x.Type===type&&x.ID===id))})},[type,id]);return <div><button onClick={()=>api.togglePersonal("watchlist",type,id,!watch).then(()=>setWatch(!watch))}>{watch?"Remove from Watchlist":"Add to Watchlist"}</button><button onClick={()=>api.togglePersonal("favorites",type,id,!favorite).then(()=>setFavorite(!favorite))}>{favorite?"Remove Favorite":"Favorite"}</button></div>}
function CollectionsPage({go,admin}:{go:(p:Page)=>void;admin:boolean}){const [manual,setManual]=useState<import("./api").Collection[]>([]),[smart,setSmart]=useState<import("./api").SmartCollection[]>([]),[message,setMessage]=useState("");const load=()=>Promise.all([api.collections(),api.smartCollections()]).then(([a,b])=>{setManual(a.collections??[]);setSmart(b.smartCollections??[])});useEffect(()=>{load()},[]);return <section className="content"><h2>Collections</h2><p>Manual collections preserve deliberate membership. Smart collections are dynamic saved queries.</p>{admin&&<><form onSubmit={e=>{e.preventDefault();const f=e.currentTarget,d=new FormData(f);api.saveCollection({Name:String(d.get("name")),Description:"",Scope:"SERVER_SHARED",Ordering:"CUSTOM"}).then(()=>{f.reset();load()}).catch(x=>setMessage(x.message))}}><h3>New manual collection</h3><input name="name" required placeholder="Collection name"/><button>Create</button></form><SmartBuilder after={load} message={setMessage}/></>}<h3>Manual</h3><div className="poster-grid">{manual.map(x=><button className="media-card" key={x.ID} onClick={()=>go(`collections/${x.ID}`)}>{x.ArtworkItemID?<ArtworkImage kind={x.ArtworkItemType==="MOVIE"?"movies":"shows"} id={x.ArtworkItemID} type="POSTER" title={x.Name}/>:<div className="poster-fallback">C</div>}<strong>{x.Name}</strong><span>Manual · {x.Scope.replace("_"," ")}</span></button>)}</div><h3>Dynamic</h3><div className="poster-grid">{smart.map(x=><button className="media-card" key={x.ID} onClick={()=>go(`collections/${x.ID}`)}><div className="poster-fallback">S</div><strong>{x.Name}</strong><span>Dynamic · {x.Scope.replace("_"," ")}</span></button>)}</div>{message&&<p role="status">{message}</p>}</section>}
function SmartBuilder({after,message}:{after:()=>void;message:(x:string)=>void}){const [preview,setPreview]=useState("");const build=(f:HTMLFormElement)=>{const d=new FormData(f);return {Name:String(d.get("name")),Description:"",Scope:"SERVER_SHARED" as const,RuleSchemaVersion:1,Rule:{logic:"ALL" as const,children:[{field:String(d.get("field")),operator:String(d.get("operator")),value:String(d.get("value"))}]},SortField:"title",SortDirection:"ASC" as const,Limit:50}};return <form onSubmit={e=>{e.preventDefault();const f=e.currentTarget;api.saveSmart(build(f)).then(()=>{f.reset();after()}).catch(x=>message(x.message))}}><h3>New smart collection</h3><input name="name" required placeholder="Collection name"/><select name="field"><option value="genre">Genre</option><option value="title">Title</option><option value="year">Year</option><option value="resolution">Resolution</option><option value="videoCodec">Video codec</option><option value="hdr">HDR</option><option value="availability">Availability</option></select><select name="operator"><option value="EQUALS">equals</option><option value="CONTAINS">contains</option><option value="GTE">at least</option></select><input name="value" required placeholder="Value"/><button type="button" onClick={e=>api.previewSmart(build(e.currentTarget.form!)).then(x=>setPreview(`${x.count} matches`)).catch(x=>message(x.message))}>Preview</button><button>Save dynamic collection</button>{preview&&<span>{preview}</span>}</form>}
function CollectionPage({id,go,admin}:{id:string;go:(p:Page)=>void;admin:boolean}){const [manual,setManual]=useState<import("./api").Collection|null>(null),[smart,setSmart]=useState<import("./api").SmartCollection|null>(null),[library,setLibrary]=useState<Array<{Type:string;ID:string;Title:string}>>([]),[selected,setSelected]=useState<string[]>([]);const load=()=>api.collection(id).then(setManual).catch(()=>api.smartCollection(id).then(setSmart));useEffect(()=>{load();if(admin)Promise.all([api.movies(),api.shows()]).then(([m,s])=>setLibrary([...m.movies.map(x=>({Type:"MOVIE",ID:x.id,Title:x.title})),...s.shows.map(x=>({Type:"SHOW",ID:x.id,Title:x.title}))]))},[id,admin]);const x=manual??smart;const move=(i:number,d:number)=>{if(!manual)return;const copy=[...manual.Items],j=i+d;if(j<0||j>=copy.length)return;[copy[i],copy[j]]=[copy[j],copy[i]];api.reorderCollection(id,copy.map(v=>`${v.Type}:${v.ID}`)).then(load)};return <section className="content"><button onClick={()=>go("collections")}>Back</button><h2>{x?.Name??"Collection"}</h2><p>{manual?"Manual collection":"Dynamic smart collection"}</p>{admin&&manual&&<><button onClick={()=>confirm("Delete this collection? Media will remain untouched.")&&api.deleteCollection(id).then(()=>go("collections"))}>Delete collection</button><details><summary>Add multiple library items</summary>{library.filter(v=>!manual.Items.some(i=>i.Type===v.Type&&i.ID===v.ID)).map(v=><label key={`${v.Type}-${v.ID}`}><input type="checkbox" checked={selected.includes(`${v.Type}:${v.ID}`)} onChange={e=>setSelected(a=>e.target.checked?[...a,`${v.Type}:${v.ID}`]:a.filter(k=>k!==`${v.Type}:${v.ID}`))}/>{v.Title} · {v.Type}</label>)}<button onClick={()=>api.addCollectionItems(id,selected.map(k=>{const [Type,ID]=k.split(":");return {Type,ID}})).then(()=>{setSelected([]);load()})}>Add selected</button></details></>}<div className="poster-grid">{x?.Items?.map((it,i)=><div key={`${it.Type}-${it.ID}-${i}`}><CurationCard item={it}/>{admin&&manual&&<><button onClick={()=>move(i,-1)}>Up</button><button onClick={()=>move(i,1)}>Down</button><button onClick={()=>api.removeCollectionItem(id,it.Type,it.ID).then(load)}>Remove</button></>}</div>)}</div></section>}
function PlaylistsPage(){const [items,setItems]=useState<import("./api").Playlist[]>([]),[active,setActive]=useState<import("./api").Playlist|null>(null);const load=()=>api.playlists().then(x=>setItems(x.playlists??[]));const open=(id:string)=>api.playlist(id).then(x=>setActive({...x,Items:x.Items??[]}));useEffect(()=>{load()},[]);const move=(i:number,d:number)=>{if(!active)return;const copy=[...(active.Items??[])],j=i+d;if(j<0||j>=copy.length)return;[copy[i],copy[j]]=[copy[j],copy[i]];api.reorderPlaylist(active.ID,copy.map(x=>x.ArtworkID)).then(()=>open(active.ID))};return <section className="content"><h2>Playlists</h2><p>Personal ordered Movie and Episode queues. Duplicate entries are allowed.</p><form onSubmit={e=>{e.preventDefault();const f=e.currentTarget,d=new FormData(f);api.savePlaylist({Name:String(d.get("name")),Description:""}).then(()=>{f.reset();load()})}}><input name="name" required placeholder="Playlist name"/><button>Create</button></form>{items.map(x=><div className="list-row" key={x.ID}><button onClick={()=>open(x.ID)}>{x.Name}</button><button onClick={()=>api.deletePlaylist(x.ID).then(()=>{setActive(null);load()})}>Delete</button></div>)}{active&&<section><h3>{active.Name}</h3><form onSubmit={e=>{e.preventDefault();const f=e.currentTarget,d=new FormData(f);api.addPlaylistItem(active.ID,String(d.get("type")),String(d.get("id"))).then(()=>{f.reset();open(active.ID)})}}><select name="type"><option>MOVIE</option><option>EPISODE</option></select><input name="id" required placeholder="Logical media ID"/><button>Add item</button></form>{(active.Items??[]).map((x,i)=><div className="list-row" key={x.ArtworkID}><span>{x.Title} · {x.Type}{x.Subtitle&&` · ${x.Subtitle}`}</span><span><button onClick={()=>location.assign(`/watch/${x.Type.toLowerCase()}/${x.ID}`)}>Play</button><button onClick={()=>move(i,-1)}>Up</button><button onClick={()=>move(i,1)}>Down</button><button onClick={()=>api.removePlaylistItem(active.ID,x.ArtworkID).then(()=>open(active.ID))}>Remove</button></span></div>)}</section>}</section>}
function PersonalPage({kind,go}:{kind:"watchlist"|"favorites";go:(p:Page)=>void}){const [items,setItems]=useState<import("./api").CurationItem[]>([]);useEffect(()=>{api.personal(kind).then(x=>setItems(x.items??[]))},[kind]);return <section className="content"><h2>{kind==="watchlist"?"My Watchlist":"Favorites"}</h2><div className="poster-grid">{items.map((x,i)=><div key={`${x.Type}-${x.ID}-${i}`}><CurationCard item={x}/><button onClick={()=>api.togglePersonal(kind,x.Type,x.ID,false).then(()=>setItems(v=>v.filter(y=>y!==x)))}>Remove</button></div>)}</div>{!items.length&&<p>Nothing here yet.</p>}<button onClick={()=>go("home")}>Home</button></section>}
function HomeSettings(){const [rows,setRows]=useState<import("./api").HomeRow[]>([]),[message,setMessage]=useState("");const load=()=>api.homeRows().then(x=>setRows(x.rows??[]));useEffect(()=>{load()},[]);const move=(i:number,d:number)=>{const copy=[...rows],j=i+d;if(j<0||j>=copy.length)return;[copy[i],copy[j]]=[copy[j],copy[i]];api.reorderHome(copy.map(x=>x.ID)).then(load)};return <section className="content"><h2>Home rows</h2><p>Each account has its own ordered layout. Empty rows stay configured here but are omitted from Home.</p>{rows.map((r,i)=><div className="list-row" key={r.ID}><span><strong>{r.Title}</strong><br/>{r.Type} · limit {r.Limit}</span><span><button aria-label={`Move ${r.Title} up`} onClick={()=>move(i,-1)}>Up</button><button aria-label={`Move ${r.Title} down`} onClick={()=>move(i,1)}>Down</button><button onClick={()=>api.saveHomeRow({...r,Enabled:!r.Enabled}).then(load)}>{r.Enabled?"Disable":"Enable"}</button><button onClick={()=>api.deleteHomeRow(r.ID).then(load)}>Remove</button></span></div>)}<form onSubmit={e=>{e.preventDefault();const f=e.currentTarget,d=new FormData(f),type=String(d.get("type"));api.saveHomeRow({Type:type,Title:String(d.get("title")),SourceID:["COLLECTION","SMART_COLLECTION","PLAYLIST"].includes(type)?String(d.get("source")):"",Enabled:true,Limit:Number(d.get("limit"))}).then(()=>{f.reset();load()}).catch(x=>setMessage(x.message))}}><h3>Add row</h3><select name="type"><option>WATCHLIST</option><option>FAVORITES</option><option>RECENTLY_ADDED_MOVIES</option><option>RECENTLY_ADDED_SHOWS</option><option>COLLECTION</option><option>SMART_COLLECTION</option><option>PLAYLIST</option></select><input name="title" required placeholder="Row title"/><input name="source" placeholder="Collection or playlist ID"/><select name="limit"><option>10</option><option>20</option><option>30</option></select><button>Add row</button></form>{message&&<p role="status">{message}</p>}</section>}

function ArtworkManager({
  kind,
  id,
}: {
  kind: "movies" | "shows";
  id: string;
}) {
  const [admin, setAdmin] = useState(false),
    [items, setItems] = useState<import("./api").Artwork[]>([]),
    [message, setMessage] = useState("");
  const load = () => api.artwork(kind, id).then((x) => setItems(x.artwork ?? []));
  useEffect(() => {
    api.me().then((u) => {
      if (u.role !== "USER") {
        setAdmin(true);
        load();
      }
    });
  }, [kind, id]);
  if (!admin) return null;
  return (
    <section className="artwork-manager">
      <h3>Artwork selection</h3>
      {items.map((x) => (
        <div className="list-row" key={x.id}>
          <span>
            {x.type} · {x.cached ? "Cached" : "Will cache when selected"}{" "}
            {x.manualSelection ? "· Manual" : ""}
          </span>
          <button
            disabled={x.selected}
            onClick={() =>
              api.selectArtwork(kind, id, x.id).then(() => {
                setMessage(`${x.type} selected and cached.`);
                load();
              })
            }
          >
            {x.selected ? "Selected" : "Select"}
          </button>
        </div>
      ))}
      {message && <p role="status">{message}</p>}
    </section>
  );
}
function MetadataAdmin() {
  const [items, setItems] = useState<Unmatched[]>([]),
    [provider, setProvider] = useState<{
      status: string;
      enabled: boolean;
      configured: boolean;
      language: string;
      region: string;
    } | null>(null),
    [message, setMessage] = useState("");
  const load = () =>
    Promise.all([
      api.unmatched().then((x) => setItems(x.items)),
      api.provider().then(setProvider),
    ]);
  useEffect(() => {
    load();
  }, []);
  async function configure(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget,
      d = new FormData(form);
    try {
      await api.configureProvider({
        enabled: d.get("enabled") === "on",
        token: d.get("token"),
        language: d.get("language"),
        region: d.get("region"),
      });
      form.reset();
      setMessage("Provider configuration saved; the token is not displayed.");
      load();
    } catch (x) {
      setMessage(x instanceof Error ? x.message : "Configuration failed");
    }
  }
  return (
    <section className="panel">
      <h2>Metadata provider</h2>
      <dl className="vertical">
        <div>
          <dt>Provider</dt>
          <dd>TMDb</dd>
        </div>
        <div>
          <dt>Status</dt>
          <dd>{provider?.status || "Loading"}</dd>
        </div>
        <div>
          <dt>Credential</dt>
          <dd>{provider?.configured ? "Configured" : "Not configured"}</dd>
        </div>
        <div>
          <dt>Preferences</dt>
          <dd>
            {provider?.language} · {provider?.region}
          </dd>
        </div>
      </dl>
      <form onSubmit={configure}>
        <label>
          <input
            name="enabled"
            type="checkbox"
            defaultChecked={provider?.enabled}
          />{" "}
          Provider enabled
        </label>
        <label>
          New TMDb read access token (leave blank to retain)
          <input name="token" type="password" autoComplete="new-password" />
        </label>
        <label>
          Language
          <input name="language" defaultValue={provider?.language || "en-US"} />
        </label>
        <label>
          Region
          <input name="region" defaultValue={provider?.region || "US"} />
        </label>
        <button>Save securely</button>
        <button
          type="button"
          onClick={() =>
            api
              .testProvider()
              .then(() => setMessage("Provider available."))
              .catch((x) => setMessage(x.message))
          }
        >
          Test connection
        </button>
      </form>
      <h2>Unmatched media</h2>
      {items.map((x) => (
        <MatchResolver key={x.fileId} item={x} done={load} />
      ))}
      {message && <p role="status">{message}</p>}
    </section>
  );
}
function MatchResolver({
  item,
  done,
}: {
  item: Unmatched;
  done: () => Promise<unknown>;
}) {
  const [candidates, setCandidates] = useState<Unmatched["candidates"]>(
      item.candidates || [],
    ),
    [selected, setSelected] = useState(""),
    [busy, setBusy] = useState(false),
    tv = item.episodeStart > 0;
  async function find() {
    setBusy(true);
    try {
      const found = await api.providerSearch(
        tv ? "SHOW" : "MOVIE",
        item.candidateTitle,
        item.candidateYear,
      );
      setCandidates(found.candidates);
      setSelected(found.candidates[0]?.providerId || "");
    } finally {
      setBusy(false);
    }
  }
  async function match() {
    if (!selected) return;
    await api.match(item.fileId, {
      type: tv ? "EPISODE" : "MOVIE",
      providerId: selected,
      season: item.seasonNumber,
      episodeStart: item.episodeStart,
      episodeEnd: item.episodeEnd,
    });
    await done();
  }
  return (
    <div className="match-review">
      <strong>{item.fileName}</strong>
      <p>
        {item.candidateTitle} {item.candidateYear || ""} · {item.state}{" "}
        {item.confidence} · score {item.score}
        {tv && (
          <>
            <br />
            Parsed S{item.seasonNumber} E{item.episodeStart}
            {item.episodeEnd > item.episodeStart && `–E${item.episodeEnd}`}
          </>
        )}
      </p>
      <button onClick={find} disabled={busy}>
        {busy ? "Searching…" : `Search ${tv ? "shows" : "movies"}`}
      </button>
      {candidates.length > 0 && (
        <>
          <label>
            Provider result
            <select
              value={selected}
              onChange={(e) => setSelected(e.target.value)}
            >
              <option value="">Choose a match</option>
              {candidates.map((c) => (
                <option key={c.providerId} value={c.providerId}>
                  {c.title} {c.year && `(${c.year})`}
                </option>
              ))}
            </select>
          </label>
          <button onClick={match} disabled={!selected}>
            Confirm association
          </button>
        </>
      )}
    </div>
  );
}
