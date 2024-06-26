package blockchain

import (
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"

	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
)

type FabricConfig struct {
	NetworkName string `yaml:"networkName"`
	ChannelName string `yaml:"channelName"`
	ChainCodeID string `yaml:"chaincodeID"`
	MSPID       string `yaml:"mspID"`
	User        struct {
		Name   string `yaml:"name"`
		Secret string `yaml:"secret"`
	} `yaml:"user"`
	ConnectionProfile string `yaml:"connectionProfile"`
	CAURL             string `yaml:"caURL"`
	CAName            string `yaml:"caName"`
}

type FabricSetup struct {
	Config  FabricConfig
	SDK     *fabsdk.FabricSDK
	Gateway *gateway.Gateway
}

func NewFabricSetup(configPath string) (*FabricSetup, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	setup := &FabricSetup{
		Config: *cfg,
	}

	err = setup.Initialize()
	if err != nil {
		return nil, err
	}

	return setup, nil
}

func (setup *FabricSetup) Initialize() error {
	sdk, err := fabsdk.New(config.FromFile(filepath.Clean(setup.Config.ConnectionProfile)))
	if err != nil {
		return fmt.Errorf("failed to create SDK: %w", err)
	}
	setup.SDK = sdk

	wallet, err := gateway.NewFileSystemWallet("wallet")
	if err != nil {
		return fmt.Errorf("failed to create wallet: %w", err)
	}

	if !wallet.Exists(setup.Config.User.Name) {
		err = setup.enrollUser(wallet)
		if err != nil {
			return fmt.Errorf("failed to enroll user: %w", err)
		}
	}

	gw, err := gateway.Connect(
		gateway.WithSDK(sdk),
		gateway.WithIdentity(wallet, setup.Config.User.Name),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to gateway: %w", err)
	}

	setup.Gateway = gw
	return nil
}

func (setup *FabricSetup) enrollUser(wallet *gateway.Wallet) error {
	ctx := setup.SDK.Context()
	mspClient, err := msp.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to create MSP client: %w", err)
	}

	signIdentity, err := mspClient.GetSigningIdentity(setup.Config.User.Name)
	if errors.Is(err, msp.ErrUserNotFound) {
		enrollmentSecret, err := mspClient.Register(&msp.RegistrationRequest{
			Name:   setup.Config.User.Name,
			Secret: setup.Config.User.Secret,
			Type:   "client",
		})
		if err != nil {
			return fmt.Errorf("failed to register user: %w", err)
		}

		err = mspClient.Enroll(setup.Config.User.Name, msp.WithSecret(enrollmentSecret))
		if err != nil {
			return fmt.Errorf("failed to enroll user: %w", err)
		}

		signIdentity, err = mspClient.GetSigningIdentity(setup.Config.User.Name)
		if err != nil {
			return fmt.Errorf("failed to get signing identity: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to get signing identity: %w", err)
	}

	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: signIdentity.EnrollmentCertificate()})
	keyPem, err := privateKeyToPEM(signIdentity.PrivateKey())
	if err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}

	identity := gateway.NewX509Identity(setup.Config.MSPID, string(certPem), string(keyPem))
	return wallet.Put(setup.Config.User.Name, identity)
}

func privateKeyToPEM(privateKey core.Key) ([]byte, error) {
	raw, err := privateKey.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw private key bytes: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}), nil
}

func LoadConfig(configPath string) (*FabricConfig, error) {
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cnfg file: %w", err)
	}

	var cnfg struct {
		Fabric FabricConfig `yaml:"fabric"`
	}

	err = yaml.Unmarshal(configFile, &cnfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cnfg: %w", err)
	}

	return &cnfg.Fabric, nil
}
