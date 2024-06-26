package node

import (
	"Fogbusv3/pkg/blockchain"
	"Fogbusv3/pkg/config"
	"Fogbusv3/pkg/discovery"
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"time"
)

type Node struct {
	host   host.Host
	cfg    *config.Config
	dht    *discovery.DHT
	mdns   *discovery.MDNS
	fabric *blockchain.FabricClient
}

func NewNode(ctx context.Context, cfg *config.Config) (*Node, error) {
	priv, _, err := crypto.GenerateKeyPair(
		crypto.Ed25519,
		-1,
	)
	if err != nil {
		return nil, fmt.Errorf("error generating keys: %w", err)
	}

	sourceMultiAddr, err := multiaddr.NewMultiaddr(cfg.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid listen address: %w", err)
	}

	h, err := libp2p.New(
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(priv),
		libp2p.DefaultSecurity,
		libp2p.EnableRelay(),
		libp2p.NATPortMap(),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	var mdns *discovery.MDNS
	if cfg.NodeType != config.CloudNode {
		mdns, err = discovery.NewMDNS(h)
		if err != nil {
			return nil, fmt.Errorf("failed to create mDNS: %w", err)
		}
	}

	var dht *discovery.DHT
	if cfg.NodeType != config.IoTNode {
		dht, err = discovery.NewDHT(ctx, h, cfg.BootstrapPeers)
		if err != nil {
			return nil, fmt.Errorf("failed to create DHT: %w", err)
		}
	}

	fabricSetup, err := blockchain.NewFabricSetup(cfg.FabricConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to set up Fabric: %w", err)
	}
	fabricClient := blockchain.NewFabricClient(fabricSetup)

	return &Node{
		host:   h,
		cfg:    cfg,
		dht:    dht,
		mdns:   mdns,
		fabric: fabricClient,
	}, nil
}

func (n *Node) ID() peer.ID {
	return n.host.ID()
}

func (n *Node) Addrs() []string {
	var addrs []string
	for _, addr := range n.host.Addrs() {
		addrs = append(addrs, addr.String())
	}
	return addrs
}

func (n *Node) Start(ctx context.Context) error {
	if err := n.dht.Start(ctx); err != nil {
		return fmt.Errorf("failed to start DHT: %w", err)
	}

	if err := n.dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	if n.mdns != nil {
		if err := n.mdns.Start(); err != nil {
			return fmt.Errorf("failed to start mDNS: %w", err)
		}
	}

	// Register node on the blockchain
	nodeInfo := blockchain.NodeInfo{
		ID:      string(n.host.ID()),
		Type:    string(n.cfg.NodeType),
		Address: n.host.Addrs()[0].String(),
	}
	err := n.fabric.RegisterNode(nodeInfo)
	if err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// Start bootstrap peers update routine
	go n.updateBootstrapPeers(ctx)

	return nil
}

func (n *Node) Stop() error {
	if n.mdns != nil {
		n.mdns.Stop()
	}

	if err := n.dht.Stop(); err != nil {
		return fmt.Errorf("failed to stop DHT: %w", err)
	}

	// Remove this node from bootstrap peers if it's a Fog or Cloud node
	if n.cfg.NodeType == config.FogNode || n.cfg.NodeType == config.CloudNode {
		err := n.fabric.RemoveBootstrapPeer(n.host.Addrs()[0].String())
		if err != nil {
			fmt.Printf("Failed to remove bootstrap peer: %v\n", err)
		}
	}

	return n.host.Close()
}

func (n *Node) updateBootstrapPeers(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Update DHT with new bootstrap peers
			err := n.dht.UpdateBootstrapPeers(ctx, n.fabric)
			if err != nil {
				fmt.Printf("Failed to update bootstrap peers: %v\n", err)
			}
		}
	}
}
