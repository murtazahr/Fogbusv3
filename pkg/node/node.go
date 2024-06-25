package node

import (
	"Fogbusv3/pkg/config"
	"Fogbusv3/pkg/discovery"
	"context"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type Node struct {
	host host.Host
	cfg  *config.Config
	dht  *discovery.DHT
	mdns *discovery.MDNS
}

func NewNode(ctx context.Context, cfg *config.Config) (*Node, error) {
	priv, _, err := crypto.GenerateKeyPair(
		crypto.Ed25519,
		-1,
	)
	if err != nil {
		log.Fatal(err)
	}

	pubKey := priv.GetPublic()
	peerID, _ := peer.IDFromPublicKey(pubKey)
	log.Printf("Peer ID: %s", peerID)

	sourceMultiAddr, err := multiaddr.NewMultiaddr(cfg.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid listen address: %w", err)
	}

	/*h, err := libp2p.New(
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(priv),
		libp2p.DefaultSecurity,
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
	)*/

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

	dht, err := discovery.NewDHT(ctx, h, cfg.BootstrapPeers)
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	var mdns *discovery.MDNS
	if cfg.NodeType != config.CloudNode {
		mdns, err = discovery.NewMDNS(h)
		if err != nil {
			return nil, fmt.Errorf("failed to create mDNS: %w", err)
		}
	}

	return &Node{
		host: h,
		cfg:  cfg,
		dht:  dht,
		mdns: mdns,
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

	if err := n.dht.ConnectToBootstrapPeers(ctx, n.cfg.BootstrapPeers); err != nil {
		return fmt.Errorf("failed to connect to bootstrap peers: %w", err)
	}

	if n.mdns != nil {
		if err := n.mdns.Start(); err != nil {
			return fmt.Errorf("failed to start mDNS: %w", err)
		}
	}

	return nil
}

func (n *Node) Stop() error {
	if n.mdns != nil {
		if err := n.mdns.Stop(); err != nil {
			return fmt.Errorf("failed to stop mDNS: %w", err)
		}
	}

	if err := n.dht.Stop(); err != nil {
		return fmt.Errorf("failed to stop DHT: %w", err)
	}

	return n.host.Close()
}
