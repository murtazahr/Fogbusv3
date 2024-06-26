package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

type MDNS struct {
	service mdns.Service
	host    host.Host
}

type mdnsNotifee struct {
	host host.Host
}

func (m *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("Found peer: %s, connecting\n", pi.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.host.Connect(ctx, pi); err != nil {
		fmt.Printf("Error connecting to peer %s: %s\n", pi.ID, err)
	} else {
		fmt.Printf("Connected to peer: %s\n", pi.ID)
	}
}

func NewMDNS(h host.Host) (*MDNS, error) {
	notifee := &mdnsNotifee{host: h}
	service := mdns.NewMdnsService(h, "fog-computing", notifee)
	return &MDNS{
		service: service,
		host:    h,
	}, nil
}

func (m *MDNS) Start() error {
	return m.service.Start()
}

func (m *MDNS) Stop() {
	err := m.service.Close()
	if err != nil {
		fmt.Printf("Error closing MDNS service: %s", err)
		return
	}
}
