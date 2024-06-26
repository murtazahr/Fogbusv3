package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type SmartContract struct {
	contractapi.Contract
}

type NodeInfo struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Address string `json:"address"`
}

type AppInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

const BootstrapPeersKey = "BootstrapPeersList"

func (s *SmartContract) RegisterNode(ctx contractapi.TransactionContextInterface, nodeInfoJSON string) error {
	var nodeInfo NodeInfo
	err := json.Unmarshal([]byte(nodeInfoJSON), &nodeInfo)
	if err != nil {
		return err
	}

	nodeInfoBytes, err := json.Marshal(nodeInfo)
	if err != nil {
		return err
	}

	err = ctx.GetStub().PutState(nodeInfo.ID, nodeInfoBytes)
	if err != nil {
		return err
	}

	// Add node to bootstrap peers list if it's a Fog or Cloud node
	if nodeInfo.Type == "fog" || nodeInfo.Type == "cloud" {
		return s.addBootstrapPeer(ctx, nodeInfo.Address)
	}

	return nil
}

func (s *SmartContract) DeployApplication(ctx contractapi.TransactionContextInterface, appInfoJSON string) error {
	var appInfo AppInfo
	err := json.Unmarshal([]byte(appInfoJSON), &appInfo)
	if err != nil {
		return err
	}

	appInfoBytes, err := json.Marshal(appInfo)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(appInfo.ID, appInfoBytes)
}

func (s *SmartContract) GetBootstrapPeers(ctx contractapi.TransactionContextInterface) ([]string, error) {
	bootstrapPeersBytes, err := ctx.GetStub().GetState(BootstrapPeersKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read bootstrap peers: %v", err)
	}

	var bootstrapPeers []string
	if bootstrapPeersBytes != nil {
		err = json.Unmarshal(bootstrapPeersBytes, &bootstrapPeers)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal bootstrap peers: %v", err)
		}
	}

	return bootstrapPeers, nil
}

func (s *SmartContract) addBootstrapPeer(ctx contractapi.TransactionContextInterface, peerAddress string) error {
	bootstrapPeers, err := s.GetBootstrapPeers(ctx)
	if err != nil {
		return err
	}

	// Check if peer already exists
	for _, peer := range bootstrapPeers {
		if peer == peerAddress {
			return nil
		}
	}

	bootstrapPeers = append(bootstrapPeers, peerAddress)
	bootstrapPeersBytes, err := json.Marshal(bootstrapPeers)
	if err != nil {
		return fmt.Errorf("failed to marshal bootstrap peers: %v", err)
	}

	return ctx.GetStub().PutState(BootstrapPeersKey, bootstrapPeersBytes)
}

func (s *SmartContract) RemoveBootstrapPeer(ctx contractapi.TransactionContextInterface, peerAddress string) error {
	bootstrapPeers, err := s.GetBootstrapPeers(ctx)
	if err != nil {
		return err
	}

	for i, peer := range bootstrapPeers {
		if peer == peerAddress {
			bootstrapPeers = append(bootstrapPeers[:i], bootstrapPeers[i+1:]...)
			break
		}
	}

	bootstrapPeersBytes, err := json.Marshal(bootstrapPeers)
	if err != nil {
		return fmt.Errorf("failed to marshal bootstrap peers: %v", err)
	}

	return ctx.GetStub().PutState(BootstrapPeersKey, bootstrapPeersBytes)
}

func main() {
	chaincode, err := contractapi.NewChaincode(&SmartContract{})
	if err != nil {
		fmt.Printf("Error creating chaincode: %s", err.Error())
		return
	}

	if err := chaincode.Start(); err != nil {
		fmt.Printf("Error starting chaincode: %s", err.Error())
	}
}
