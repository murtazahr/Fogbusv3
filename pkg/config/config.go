package config

import (
	"flag"
	"strings"
)

type NodeType string

const (
	IoTNode   NodeType = "iot"
	FogNode   NodeType = "fog"
	CloudNode NodeType = "cloud"
)

type Config struct {
	NodeType       NodeType
	ListenAddr     string
	BootstrapPeers []string
}

func ParseFlags() *Config {
	cfg := &Config{}

	nodeType := flag.String("type", "fog", "Node type: iot, fog, or cloud")
	listenAddr := flag.String("listen", "/ip4/0.0.0.0/tcp/0", "Listen address")
	bootstrapPeers := flag.String("bootstrap", "", "Comma-separated list of bootstrap peer multiaddresses")

	flag.Parse()

	cfg.NodeType = NodeType(*nodeType)
	cfg.ListenAddr = *listenAddr

	if *bootstrapPeers != "" {
		cfg.BootstrapPeers = strings.Split(*bootstrapPeers, ",")
	}

	return cfg
}
