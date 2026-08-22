package sharing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/huin/goupnp/dcps/internetgateway1"
	"github.com/huin/goupnp/dcps/internetgateway2"
)

const DiscoveryService = "_vynode-media._tcp"

type advertisement interface{ Shutdown() error }
type advertiseFunc func(string, int, []string) (advertisement, error)

type upnpClient interface {
	AddPortMappingCtx(context.Context, string, uint16, string, uint16, string, bool, string, uint32) error
	DeletePortMappingCtx(context.Context, string, uint16, string) error
}
type gateway struct {
	client upnpClient
	host   string
	local  string
}
type discoverGatewayFunc func(context.Context) (gateway, error)

type Runtime struct {
	db               *sql.DB
	instanceID, name string
	port             int
	now              func() time.Time
	advertise        advertiseFunc
	discover         discoverGatewayFunc
	mu               sync.Mutex
	ad               advertisement
	mapped           *gateway
	mappedExternal   uint16
	wake, stop, done chan struct{}
	emit             func(context.Context, string, map[string]any, string)
}

func newRuntime(db *sql.DB, instanceID, name string, port int, emit func(context.Context, string, map[string]any, string)) *Runtime {
	r := &Runtime{db: db, instanceID: instanceID, name: name, port: port, now: time.Now, wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}), emit: emit}
	r.advertise = func(name string, port int, txt []string) (advertisement, error) {
		svc, err := mdns.NewMDNSService(name, DiscoveryService, "", "", port, nil, txt)
		if err != nil {
			return nil, err
		}
		return mdns.NewServer(&mdns.Config{Zone: svc})
	}
	r.discover = discoverGateway
	return r
}

func (r *Runtime) Start() { go r.loop() }
func (r *Runtime) Wake() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}
func (r *Runtime) Close() {
	select {
	case <-r.stop:
		return
	default:
		close(r.stop)
	}
	<-r.done
}
func (r *Runtime) loop() {
	defer close(r.done)
	ticker := time.NewTicker(30 * time.Second)
	cleanup := time.NewTicker(time.Minute)
	defer ticker.Stop()
	defer cleanup.Stop()
	r.reconcile(context.Background())
	for {
		select {
		case <-r.wake:
			r.reconcile(context.Background())
		case <-ticker.C:
			r.reconcile(context.Background())
		case <-cleanup.C:
			r.cleanup(context.Background())
		case <-r.stop:
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			r.stopDiscovery(ctx)
			r.stopMapping(ctx)
			cancel()
			return
		}
	}
}

func (r *Runtime) reconcile(ctx context.Context) {
	var discovery, mapping bool
	var external int
	if err := r.db.QueryRowContext(ctx, "SELECT discovery_enabled,port_mapping_enabled,port_mapping_external_port FROM remote_access_settings WHERE id=1").Scan(&discovery, &mapping, &external); err != nil {
		return
	}
	if discovery {
		r.startDiscovery(ctx)
	} else {
		r.stopDiscovery(ctx)
	}
	if mapping {
		r.ensureMapping(ctx, external)
	} else {
		r.stopMapping(ctx)
	}
}
func (r *Runtime) setDiscovery(ctx context.Context, state, summary string) {
	_, _ = r.db.ExecContext(ctx, "UPDATE remote_access_settings SET discovery_runtime_status=?,discovery_last_error=?,discovery_updated_at=? WHERE id=1", state, summary, stamp(r.now()))
	r.health(ctx, "network-discovery", "LAN_DISCOVERY", state == "ERROR", "LAN discovery is enabled but the runtime advertiser failed: "+summary)
}
func (r *Runtime) startDiscovery(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ad != nil {
		return
	}
	r.setDiscovery(ctx, "STARTING", "")
	short := r.instanceID
	if len(short) > 8 {
		short = short[:8]
	}
	name := strings.TrimSpace(r.name)
	if name == "" {
		name = "VyNode Media"
	}
	ad, err := r.advertise(name+"-"+short, r.port, []string{"id=" + r.instanceID, "api=v1", "secure=optional"})
	if err != nil {
		r.setDiscovery(ctx, "ERROR", safeError(err))
		r.event(ctx, "LAN_DISCOVERY_FAILED", map[string]any{"error": safeError(err)}, "lan-discovery-failed")
		return
	}
	r.ad = ad
	r.setDiscovery(ctx, "RUNNING", "")
	r.event(ctx, "LAN_DISCOVERY_STARTED", map[string]any{"service": DiscoveryService, "port": r.port}, "lan-discovery-started")
}
func (r *Runtime) stopDiscovery(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ad != nil {
		_ = r.ad.Shutdown()
		r.ad = nil
		r.event(ctx, "LAN_DISCOVERY_STOPPED", map[string]any{}, "lan-discovery-stopped")
	}
	r.setDiscovery(ctx, "DISABLED", "")
}

func (r *Runtime) ensureMapping(ctx context.Context, configured int) {
	external := configured
	if external == 0 {
		external = r.port
	}
	if external < 1 || external > 65535 || r.port < 1 || r.port > 65535 {
		r.mappingState(ctx, "FAILED", external, "invalid port", nil)
		return
	}
	r.mu.Lock()
	if r.mapped != nil && r.mappedExternal == uint16(external) {
		var expiry string
		_ = r.db.QueryRowContext(ctx, "SELECT COALESCE(lease_expires_at,'') FROM port_mappings WHERE protocol='UPNP' AND owned=1 LIMIT 1").Scan(&expiry)
		if t, err := time.Parse(time.RFC3339Nano, expiry); err == nil && t.After(r.now().Add(10*time.Minute)) {
			r.mu.Unlock()
			return
		}
		g := *r.mapped
		r.mu.Unlock()
		r.mappingState(ctx, "RENEWING", external, "", &g)
		r.addMapping(ctx, &g, uint16(external))
		return
	}
	r.mu.Unlock()
	r.mappingState(ctx, "DISCOVERING", external, "", nil)
	lookup, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	g, err := r.discover(lookup)
	if err != nil {
		r.mappingState(ctx, "FAILED", external, safeError(err), nil)
		r.event(ctx, "PORT_MAPPING_FAILED", map[string]any{"protocol": "UPNP", "error": safeError(err)}, "port-mapping-failed")
		return
	}
	r.addMapping(ctx, &g, uint16(external))
}
func (r *Runtime) addMapping(ctx context.Context, g *gateway, external uint16) {
	lease := uint32(1800)
	if err := g.client.AddPortMappingCtx(ctx, "", external, "TCP", uint16(r.port), g.local, true, "VyNode Media "+shortID(r.instanceID), lease); err != nil {
		r.mappingState(ctx, "FAILED", int(external), safeError(err), g)
		r.event(ctx, "PORT_MAPPING_FAILED", map[string]any{"protocol": "UPNP", "error": safeError(err)}, "port-mapping-failed")
		return
	}
	r.mu.Lock()
	r.mapped = g
	r.mappedExternal = external
	r.mu.Unlock()
	r.mappingState(ctx, "MAPPED", int(external), "", g)
	r.event(ctx, "PORT_MAPPING_ESTABLISHED", map[string]any{"protocol": "UPNP", "internalPort": r.port, "externalPort": external}, "port-mapping-established")
}
func (r *Runtime) stopMapping(ctx context.Context) {
	r.mu.Lock()
	g, external := r.mapped, r.mappedExternal
	r.mapped = nil
	r.mappedExternal = 0
	r.mu.Unlock()
	if g != nil {
		_ = g.client.DeletePortMappingCtx(ctx, "", external, "TCP")
	}
	_, _ = r.db.ExecContext(ctx, "DELETE FROM port_mappings WHERE protocol='UPNP' AND owned=1")
	_, _ = r.db.ExecContext(ctx, `INSERT INTO port_mappings(id,protocol,state,internal_port,owned,updated_at) VALUES('upnp','UPNP','DISABLED',?,0,?) ON CONFLICT(id) DO UPDATE SET state='DISABLED',external_port=NULL,gateway=NULL,lease_expires_at=NULL,owned=0,last_error=NULL,updated_at=excluded.updated_at`, r.port, stamp(r.now()))
}
func (r *Runtime) mappingState(ctx context.Context, state string, external int, summary string, g *gateway) {
	gatewayHost := ""
	owned := 0
	var expiry any
	if g != nil {
		gatewayHost = g.host
	}
	if state == "MAPPED" {
		owned = 1
		expiry = stamp(r.now().Add(30 * time.Minute))
	}
	_, _ = r.db.ExecContext(ctx, `INSERT INTO port_mappings(id,protocol,state,internal_port,external_port,gateway,lease_expires_at,owned,last_error,updated_at) VALUES('upnp','UPNP',?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET state=excluded.state,internal_port=excluded.internal_port,external_port=excluded.external_port,gateway=excluded.gateway,lease_expires_at=excluded.lease_expires_at,owned=excluded.owned,last_error=excluded.last_error,updated_at=excluded.updated_at`, state, r.port, external, nullString(gatewayHost), expiry, owned, nullString(summary), stamp(r.now()))
	r.health(ctx, "network-port-mapping", "PORT_MAPPING", state == "FAILED", "Automatic UPnP port mapping failed: "+summary)
}

func (r *Runtime) health(ctx context.Context, id, category string, active bool, description string) {
	now := stamp(r.now())
	if active {
		_, _ = r.db.ExecContext(ctx, `INSERT INTO health_issues(id,category,severity,reference_type,reference_id,description,status,first_detected_at,last_detected_at) VALUES(?,?,?,?,?,?,'OPEN',?,?) ON CONFLICT(category,reference_type,reference_id) DO UPDATE SET description=excluded.description,status=CASE WHEN health_issues.status='IGNORED' THEN 'IGNORED' ELSE 'OPEN' END,last_detected_at=excluded.last_detected_at,resolved_at=NULL`, id, category, "WARNING", "network", id, description, now, now)
		return
	}
	_, _ = r.db.ExecContext(ctx, "UPDATE health_issues SET status='RESOLVED',resolved_at=?,last_detected_at=? WHERE category=? AND reference_type='network' AND reference_id=? AND status='OPEN'", now, now, category, id)
}
func (r *Runtime) cleanup(ctx context.Context) {
	now := stamp(r.now())
	_, _ = r.db.ExecContext(ctx, "UPDATE user_invitations SET status='EXPIRED',token_hash='expired:'||id WHERE status='PENDING' AND expires_at<=?", now)
	_, _ = r.db.ExecContext(ctx, "UPDATE pairing_requests SET status='EXPIRED',code_hash='expired-code:'||id,challenge_hash='expired-challenge:'||id WHERE status IN ('PENDING','APPROVED') AND expires_at<=?", now)
}
func (r *Runtime) event(ctx context.Context, kind string, payload map[string]any, dedupe string) {
	if r.emit != nil {
		r.emit(ctx, kind, payload, dedupe)
	}
}
func safeError(err error) string {
	if err == nil {
		return ""
	}
	s := strings.TrimSpace(err.Error())
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
func shortID(v string) string {
	if len(v) > 8 {
		return v[:8]
	}
	return v
}
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func discoverGateway(ctx context.Context) (gateway, error) {
	type candidate struct {
		client upnpClient
		host   string
	}
	candidates := []candidate{}
	if cs, _, _ := internetgateway2.NewWANIPConnection2ClientsCtx(ctx); len(cs) > 0 {
		for _, c := range cs {
			candidates = append(candidates, candidate{c, c.ServiceClient.Location.Hostname()})
		}
	}
	if cs, _, _ := internetgateway2.NewWANIPConnection1ClientsCtx(ctx); len(cs) > 0 {
		for _, c := range cs {
			candidates = append(candidates, candidate{c, c.ServiceClient.Location.Hostname()})
		}
	}
	if cs, _, _ := internetgateway1.NewWANIPConnection1ClientsCtx(ctx); len(cs) > 0 {
		for _, c := range cs {
			candidates = append(candidates, candidate{c, c.ServiceClient.Location.Hostname()})
		}
	}
	if len(candidates) == 0 {
		return gateway{}, errors.New("no UPnP IGD gateway discovered")
	}
	host := candidates[0].host
	local, err := localFor(host)
	if err != nil {
		return gateway{}, err
	}
	return gateway{client: candidates[0].client, host: host, local: local}, nil
}
func localFor(host string) (string, error) {
	if host == "" {
		return "", errors.New("gateway address unavailable")
	}
	c, err := net.DialTimeout("udp", net.JoinHostPort(host, "1900"), time.Second)
	if err != nil {
		return "", err
	}
	defer c.Close()
	a, ok := c.LocalAddr().(*net.UDPAddr)
	if !ok || a.IP == nil {
		return "", fmt.Errorf("local address unavailable for %s", host)
	}
	return a.IP.String(), nil
}
func listenerPort(address string) (int, error) {
	_, raw, e := net.SplitHostPort(address)
	if e != nil {
		return 0, e
	}
	p, e := strconv.Atoi(raw)
	if e != nil || p < 1 || p > 65535 {
		return 0, errors.New("invalid listener port")
	}
	return p, nil
}
