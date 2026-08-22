package sharing

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/mdns"
)

func TestRealMDNSAdvertisementDiscovery(t *testing.T) {
	if os.Getenv("VYNODE_MDNS_INTEGRATION") != "1" {
		t.Skip("set VYNODE_MDNS_INTEGRATION=1 on a multicast-capable network")
	}
	instance := "VyNodeIntegration-" + shortID(time.Now().UTC().Format("150405.000000"))
	svc, err := mdns.NewMDNSService(instance, DiscoveryService, "", "", 18096, nil, []string{"id=mdns-integration", "api=v1", "secure=optional"})
	if err != nil {
		t.Fatal(err)
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: svc})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown()
	time.Sleep(200 * time.Millisecond)
	entries := make(chan *mdns.ServiceEntry, 16)
	done := make(chan struct{})
	found := false
	go func() {
		for e := range entries {
			if strings.HasPrefix(e.Name, instance+".") && e.Port == 18096 {
				for _, txt := range e.InfoFields {
					if txt == "id=mdns-integration" {
						found = true
					}
				}
			}
		}
		close(done)
	}()
	if err = mdns.Lookup(DiscoveryService, entries); err != nil {
		close(entries)
		<-done
		t.Fatal(err)
	}
	close(entries)
	<-done
	if !found {
		t.Fatal("VyNode mDNS service was not discovered")
	}
}
