package blockchain

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
)

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

// FabricClient provides methods to interact with the deployed chaincode
type FabricClient struct {
	setup *FabricSetup
}

func NewFabricClient(setup *FabricSetup) *FabricClient {
	return &FabricClient{setup: setup}
}

func (fc *FabricClient) getContract() (*gateway.Contract, error) {
	network, err := fc.setup.Gateway.GetNetwork(fc.setup.Config.ChannelName)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}
	return network.GetContract(fc.setup.Config.ChainCodeID), nil
}

func (fc *FabricClient) RegisterNode(nodeInfo NodeInfo) error {
	contract, err := fc.getContract()
	if err != nil {
		return err
	}

	nodeInfoBytes, err := json.Marshal(nodeInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal node info: %w", err)
	}

	_, err = contract.SubmitTransaction("RegisterNode", string(nodeInfoBytes))
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	return nil
}

func (fc *FabricClient) DeployApplication(appInfo AppInfo) error {
	contract, _ := fc.getContract()

	appInfoBytes, err := json.Marshal(appInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal app info: %w", err)
	}

	_, err = contract.SubmitTransaction("DeployApplication", string(appInfoBytes))
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	return nil
}

func (fc *FabricClient) GetBootstrapPeers() ([]string, error) {
	contract, _ := fc.getContract()

	result, err := contract.EvaluateTransaction("GetBootstrapPeers")
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate transaction: %w", err)
	}

	var bootstrapPeers []string
	err = json.Unmarshal(result, &bootstrapPeers)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal bootstrap peers: %w", err)
	}

	return bootstrapPeers, nil
}

func (fc *FabricClient) RemoveBootstrapPeer(peerAddress string) error {
	contract, _ := fc.getContract()

	_, err := contract.SubmitTransaction("RemoveBootstrapPeer", peerAddress)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	return nil
}
