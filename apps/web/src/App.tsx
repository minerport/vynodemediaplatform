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
          go("login");
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
      page === "automation" ||
      page === "streams") &&
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
  ) : page === "libraries" ? (
    <Libraries />
  ) : page === "users" ? (
    <Users />
  ) : page === "audit" ? (
    <Audit />
  ) : page === "streams" ? (
    <ActiveStreams />
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
  ) : (
    <Home info={info} user={user} />
  );
  if (watch) return content;
  return (
    <div className="app-shell">
      <aside>
        <div className="brand">
          <div className="mark">V</div>VyNode
        </div>
        <nav>
          <Nav go={go} page={page} target="home">
            Home
          </Nav>
          <Nav go={go} page={page} target="movies">
            Movies
          </Nav>
          <Nav go={go} page={page} target="shows">
            Shows
          </Nav>
          {admin && (
            <>
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
          <button
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
          <div>
            <p className="eyebrow">{user.role}</p>
            <h1>{user.displayName}</h1>
          </div>
          <div className="connection online">
            <i />
            {info?.serverName}
          </div>
        </header>
        {content}
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
  const [movies, setMovies] = useState<Movie[]>([]),
    [shows, setShows] = useState<Show[]>([]),
    [continuing, setContinuing] = useState<ContinueItem[]>([]);
  useEffect(() => {
    api.movies().then((x) => setMovies(x.movies.slice(0, 8)));
    api.shows().then((x) => setShows(x.shows.slice(0, 8)));
    api.continueWatching().then((x) => setContinuing(x.items));
  }, []);
  return (
    <section className="content">
      <div className="hero">
        <p className="eyebrow">YOUR MEDIA</p>
        <h2>{info?.serverName}</h2>
        <p>Welcome back, {user.displayName}.</p>
      </div>
      {continuing.length > 0 && <><h2>Continue Watching</h2><div className="continue-grid">{continuing.map(x=><div key={`${x.logicalType}-${x.logicalId}`} className="continue-card"><strong>{x.title}</strong><span>{formatTime(x.position)} of {formatTime(x.duration)} · {formatTime(x.duration-x.position)} remaining</span><progress value={x.progress} max={1}/><button onClick={()=>location.assign(`/watch/${x.logicalType.toLowerCase()}/${x.logicalId}`)}>Resume</button><button onClick={()=>api.dismissContinue(x.logicalType,x.logicalId).then(()=>setContinuing(items=>items.filter(i=>i!==x)))}>Remove</button></div>)}</div></>}
      {!movies.length && !shows.length && (
        <div className="empty">
          <div className="empty-icon">V</div>
          <h2>Your library is ready</h2>
          <p>Identified movies and shows will appear here.</p>
        </div>
      )}
      {movies.length > 0 && (
        <>
          <h2>Recently Added Movies</h2>
          <div className="poster-grid">
            {movies.map((x) => (
              <MediaCard
                key={x.id}
                kind="movies"
                id={x.id}
                title={x.title}
                year={x.year}
              />
            ))}
          </div>
        </>
      )}
      {shows.length > 0 && (
        <>
          <h2>Recently Added Shows</h2>
          <div className="poster-grid">
            {shows.map((x) => (
              <MediaCard
                key={x.id}
                kind="shows"
                id={x.id}
                title={x.title}
                year={x.year}
              />
            ))}
          </div>
        </>
      )}
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
    </section>
  );
}
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
            <strong>{x.title || x.logicalType} · {x.decision.mode.replaceAll("_", " ")}</strong>
            <p>
              {x.state} · {formatTime(x.position)} / {formatTime(x.duration)} ·{" "}
              {x.selectedVersion?.resolution} {x.selectedVersion?.videoCodec}
              {x.decision.mode !== "DIRECT_PLAY" && <> · Video {x.decision.plan.video.action} · Audio {x.decision.plan.audio.sourceCodec}{x.decision.plan.audio.targetCodec && ` → ${x.decision.plan.audio.targetCodec}`} · {x.decision.plan.container.target}</>}
              {x.decision.mode === "VIDEO_TRANSCODE" && <> · {x.decision.plan.video.sourceCodec?.toUpperCase()} → {x.decision.plan.video.targetCodec?.toUpperCase()} · {x.decision.plan.video.targetHeight}p · {x.decision.plan.backend?.actual || "SOFTWARE"}</>}
            </p>
          </div>
          <button onClick={() => api.adminStopPlayback(x.id).then(load)}>
            Stop
          </button>
        </div>
      ))}
    </section>
  );
}

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
              {e.available && <EpisodePlay id={e.id} go={go} />}
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
