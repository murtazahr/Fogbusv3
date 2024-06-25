package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"Fogbusv3/pkg/config"
	"Fogbusv3/pkg/node"
)

func main() {
	cfg := config.ParseFlags()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, err := node.NewNode(ctx, cfg)
	if err != nil {
		fmt.Printf("Failed to create node: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Node created. Type: %s, ID: %s\n", cfg.NodeType, n.ID())
	fmt.Printf("Addresses: %v\n", n.Addrs())

	if err := n.Start(ctx); err != nil {
		fmt.Printf("Failed to start node: %v\n", err)
		os.Exit(1)
	}

	// Wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("Received signal, shutting down...")

	if err := n.Stop(); err != nil {
		fmt.Printf("Error during shutdown: %v\n", err)
	}
}
