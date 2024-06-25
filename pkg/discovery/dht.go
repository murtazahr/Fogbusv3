package discovery

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p/core/network"
	"log"
	"time"

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
		if err := d.connectWithRetry(ctx, addr); err != nil {
			fmt.Printf("Failed to connect to bootstrap peer %s after retries: %v\n", addr, err)
			// Continue trying to connect to other peers
			continue
		}
		fmt.Printf("Connected to bootstrap peer: %s\n", addr)
	}
	return nil
}

func (d *DHT) connectWithRetry(ctx context.Context, addr string) error {
	backoff := time.Second
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		err := d.connectToPeer(ctx, addr)
		if err == nil {
			return nil
		}

		fmt.Printf("Attempt %d: Failed to connect to %s: %v\n", i+1, addr, err)

		select {
		case <-time.After(backoff):
			backoff *= 2 // exponential backoff
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("failed to connect after %d attempts", maxRetries)
}

func (d *DHT) connectToPeer(ctx context.Context, addr string) error {
	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return fmt.Errorf("invalid bootstrap peer address: %w", err)
	}

	peerinfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("failed to get peer info: %w", err)
	}

	// Check if already connected
	if d.host.Network().Connectedness(peerinfo.ID) == network.Connected {
		fmt.Printf("Already connected to peer: %s\n", addr)
		return nil
	}

	// Set a timeout for the connection attempt
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	log.Printf("Attempting to connect to peer: %s", peerinfo.ID)
	if err := d.host.Connect(ctxTimeout, *peerinfo); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	return nil
}
