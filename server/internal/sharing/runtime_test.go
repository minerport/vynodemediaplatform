package sharing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vynode/media/server/internal/auth"
)

type fakeAd struct{ stopped int }

func (f *fakeAd) Shutdown() error { f.stopped++; return nil }

type fakeUPnP struct {
	adds, deletes     int
	addErr, deleteErr error
}

func (f *fakeUPnP) AddPortMappingCtx(context.Context, string, uint16, string, uint16, string, bool, string, uint32) error {
	f.adds++
	return f.addErr
}
func (f *fakeUPnP) DeletePortMappingCtx(context.Context, string, uint16, string) error {
	f.deletes++
	return f.deleteErr
}

func TestDiscoveryLifecycleAndSafeTXT(t *testing.T) {
	s, _ := fixture(t)
	ctx := context.Background()
	ad := &fakeAd{}
	var gotName string
	var gotPort int
	var gotTXT []string
	r := newRuntime(s.db, "server-id", "Family Server", 8096, nil)
	r.advertise = func(name string, port int, txt []string) (advertisement, error) {
		gotName = name
		gotPort = port
		gotTXT = append([]string{}, txt...)
		return ad, nil
	}
	r.startDiscovery(ctx)
	if gotName != "Family Server-server-i" || gotPort != 8096 || len(gotTXT) != 3 || gotTXT[0] != "id=server-id" {
		t.Fatalf("advertisement %q %d %#v", gotName, gotPort, gotTXT)
	}
	var status string
	_ = s.db.QueryRow("SELECT discovery_runtime_status FROM remote_access_settings WHERE id=1").Scan(&status)
	if status != "RUNNING" {
		t.Fatal(status)
	}
	r.stopDiscovery(ctx)
	if ad.stopped != 1 {
		t.Fatalf("shutdowns=%d", ad.stopped)
	}
	_ = s.db.QueryRow("SELECT discovery_runtime_status FROM remote_access_settings WHERE id=1").Scan(&status)
	if status != "DISABLED" {
		t.Fatal(status)
	}
}

func TestDiscoveryFailureDoesNotFailRuntime(t *testing.T) {
	s, _ := fixture(t)
	r := newRuntime(s.db, "id", "server", 8096, nil)
	r.advertise = func(string, int, []string) (advertisement, error) { return nil, errors.New("multicast unavailable") }
	r.startDiscovery(context.Background())
	var state string
	_ = s.db.QueryRow("SELECT discovery_runtime_status FROM remote_access_settings WHERE id=1").Scan(&state)
	if state != "ERROR" {
		t.Fatal(state)
	}
}

func TestUPnPOwnedMappingRenewalFailureAndCleanup(t *testing.T) {
	s, _ := fixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	client := &fakeUPnP{}
	r := newRuntime(s.db, "server-id", "server", 8096, nil)
	r.now = func() time.Time { return now }
	r.discover = func(context.Context) (gateway, error) {
		return gateway{client: client, host: "192.0.2.1", local: "192.0.2.2"}, nil
	}
	r.ensureMapping(ctx, 18096)
	if client.adds != 1 {
		t.Fatalf("adds=%d", client.adds)
	}
	var state string
	var owned int
	_ = s.db.QueryRow("SELECT state,owned FROM port_mappings WHERE id='upnp'").Scan(&state, &owned)
	if state != "MAPPED" || owned != 1 {
		t.Fatalf("%s %d", state, owned)
	}
	now = now.Add(25 * time.Minute)
	r.ensureMapping(ctx, 18096)
	if client.adds != 2 {
		t.Fatalf("renew adds=%d", client.adds)
	}
	client.addErr = errors.New("router conflict")
	now = now.Add(25 * time.Minute)
	r.ensureMapping(ctx, 18096)
	_ = s.db.QueryRow("SELECT state FROM port_mappings WHERE id='upnp'").Scan(&state)
	if state != "FAILED" {
		t.Fatal(state)
	}
	r.stopMapping(ctx)
	if client.deletes != 1 {
		t.Fatalf("deletes=%d", client.deletes)
	}
	_ = s.db.QueryRow("SELECT state,owned FROM port_mappings WHERE id='upnp'").Scan(&state, &owned)
	if state != "DISABLED" || owned != 0 {
		t.Fatalf("%s %d", state, owned)
	}
}

func TestUPnPDiscoveryFailureAndExpiredRecordCleanup(t *testing.T) {
	s, p := fixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	r := newRuntime(s.db, "id", "server", 8096, nil)
	r.now = func() time.Time { return now }
	r.discover = func(context.Context) (gateway, error) { return gateway{}, errors.New("malformed gateway response") }
	r.ensureMapping(ctx, 0)
	var state string
	_ = s.db.QueryRow("SELECT state FROM port_mappings WHERE id='upnp'").Scan(&state)
	if state != "FAILED" {
		t.Fatal(state)
	}
	inv, _, e := s.CreateInvite(ctx, p, "", auth.RoleUser, nil, time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	pair, e := s.CreatePairing(ctx, auth.DeviceInput{Name: "TV", ClientName: "Test", Platform: "tv"})
	if e != nil {
		t.Fatal(e)
	}
	now = now.Add(8 * time.Hour)
	r.cleanup(ctx)
	var inviteStatus, pairStatus, tokenHash, challenge string
	_ = s.db.QueryRow("SELECT status,token_hash FROM user_invitations WHERE id=?", inv.ID).Scan(&inviteStatus, &tokenHash)
	_ = s.db.QueryRow("SELECT status,challenge_hash FROM pairing_requests WHERE id=?", pair.ID).Scan(&pairStatus, &challenge)
	if inviteStatus != "EXPIRED" || pairStatus != "EXPIRED" || tokenHash[:8] != "expired:" || challenge[:18] != "expired-challenge:" {
		t.Fatalf("%s %s %s %s", inviteStatus, pairStatus, tokenHash, challenge)
	}
}
