package discovery

import (
	"Fogbusv3/pkg/blockchain"
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
		options = append(options, dht.Mode(dht.ModeServer))
	}

	kadDHT, err := dht.New(ctx, h, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	return &DHT{
		kadDHT: kadDHT,
		host:   h,
	}, nil
}

func (d *DHT) Start(ctx context.Context) error {
	return nil
}

func (d *DHT) Bootstrap(ctx context.Context) error {
	return d.kadDHT.Bootstrap(ctx)
}

func (d *DHT) Stop() error {
	return d.kadDHT.Close()
}

func (d *DHT) UpdateBootstrapPeers(ctx context.Context, fabricClient *blockchain.FabricClient) error {
	bootstrapPeers, err := fabricClient.GetBootstrapPeers()
	if err != nil {
		return fmt.Errorf("failed to get bootstrap peers from blockchain: %v", err)
	}

	var peerInfos []peer.AddrInfo
	var unreachablePeers []string
	for _, addr := range bootstrapPeers {
		maddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			fmt.Printf("Invalid bootstrap peer address %s: %v\n", addr, err)
			unreachablePeers = append(unreachablePeers, addr)
			continue
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			fmt.Printf("Failed to get peer info from address %s: %v\n", addr, err)
			unreachablePeers = append(unreachablePeers, addr)
			continue
		}
		if err := d.host.Connect(ctx, *peerInfo); err != nil {
			fmt.Printf("Failed to connect to bootstrap peer %s: %v\n", addr, err)
			unreachablePeers = append(unreachablePeers, addr)
			continue
		}
		peerInfos = append(peerInfos, *peerInfo)
	}

	if err := d.kadDHT.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %v", err)
	}

	if len(unreachablePeers) > 0 {
		for _, p := range unreachablePeers {
			if err := fabricClient.RemoveBootstrapPeer(p); err != nil {
				fmt.Printf("Failed to remove unreachable bootstrap p %s: %v\n", p, err)
			}
		}
	}

	return nil
}
