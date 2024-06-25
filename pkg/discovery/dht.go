package discovery

import (
	"context"
	"fmt"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type DHT struct {
	kadDHT *dht.IpfsDHT
	host   host.Host
}

func NewDHT(ctx context.Context, h host.Host, bootstrapPeers []string) (*DHT, error) {
	var options []dht.Option

	if len(bootstrapPeers) == 0 {
		// If no bootstrap peers, run in server mode
		options = append(options, dht.Mode(dht.ModeServer))
	}

	kadDHT, err := dht.New(ctx, h, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	d := &DHT{
		kadDHT: kadDHT,
		host:   h,
	}

	return d, nil
}

func (d *DHT) Start(ctx context.Context) error {
	if err := d.kadDHT.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	return nil
}

func (d *DHT) Stop() error {
	return d.kadDHT.Close()
}

func (d *DHT) ConnectToBootstrapPeers(ctx context.Context, bootstrapPeers []string) error {
	if len(bootstrapPeers) == 0 {
		fmt.Println("No bootstrap peers provided. Running as a bootstrap node.")
		return nil
	}

	for _, addr := range bootstrapPeers {
		maddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return fmt.Errorf("invalid bootstrap peer address: %w", err)
		}
		peerinfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			return fmt.Errorf("failed to get peer info: %w", err)
		}
		if err := d.host.Connect(ctx, *peerinfo); err != nil {
			fmt.Printf("Failed to connect to bootstrap peer %s: %v\n", addr, err)
			// Continue trying to connect to other peers
			continue
		}
		fmt.Printf("Connected to bootstrap peer: %s\n", addr)
	}
	return nil
}
