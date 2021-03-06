/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package context

import (
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	sdkConfig "github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config/lookup"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	protocolApi "github.com/trustbloc/sidetree-core-go/pkg/api/protocol"
	"github.com/trustbloc/sidetree-core-go/pkg/batch"
	"github.com/trustbloc/sidetree-fabric/pkg/context/blockchain"
	"github.com/trustbloc/sidetree-fabric/pkg/context/cas"
	"github.com/trustbloc/sidetree-fabric/pkg/context/protocol"
)

const (
	keyProtocolFile = "protocol.file"
	keyConfigFile   = "config.file"

	defaultConfigFile   = "config.yaml"
	defaultProtocolFile = "protocol.json"
)

var logger = logrus.New()

// SidetreeContext implements 'Fabric' version of Sidetree node context
type SidetreeContext struct {
	protocolClient   protocolApi.Client
	casClient        batch.CASClient
	blockchainClient batch.BlockchainClient
}

// New creates new Sidetree context
func New(cfg *viper.Viper, channelProvider context.ChannelProvider) (*SidetreeContext, error) {

	pc, err := getProtocolClient(cfg)
	if err != nil {
		logger.Errorf("Failed to load protocol: %s", err.Error())
		return nil, err
	}

	ctx := &SidetreeContext{
		protocolClient:   pc,
		casClient:        cas.New(channelProvider),
		blockchainClient: blockchain.New(channelProvider),
	}

	return ctx, nil
}

// GetChannelProvider creates new channel provider
func GetChannelProvider(cfg *viper.Viper) (context.ChannelProvider, error) {

	configProvider := getConfigProvider(cfg)
	sdk, err := fabsdk.New(configProvider)
	if err != nil {
		logger.Errorf("Failed to create SDK: %s", err.Error())
		return nil, err
	}

	sidetreeCfg, err := getSidetreeConfig(configProvider)
	if err != nil {
		logger.Errorf("Failed to load 'sidetree' configuration: %s", err.Error())
		return nil, err
	}

	chProvider := sdk.ChannelContext(sidetreeCfg.Channel, fabsdk.WithUser(sidetreeCfg.User))
	logger.Debugf("Created channel provider for %s with user %s", sidetreeCfg.Channel, sidetreeCfg.User)

	return chProvider, nil
}

func getProtocolClient(cfg *viper.Viper) (*protocol.Client, error) {

	protocolConfigFile := defaultProtocolFile
	if cfg.IsSet(keyProtocolFile) {
		protocolConfigFile = cfg.GetString(keyProtocolFile)
	}

	return protocol.New(protocolConfigFile)
}

func getConfigProvider(cfg *viper.Viper) core.ConfigProvider {
	cfgFile := defaultConfigFile
	if cfg.IsSet(keyConfigFile) {
		cfgFile = cfg.GetString(keyConfigFile)
	}

	return sdkConfig.FromFile(cfgFile)
}

func getSidetreeConfig(configProvider core.ConfigProvider) (*sidetreeConfig, error) {

	configBackend, err := configProvider()
	if err != nil {
		return nil, err
	}

	cfgLookup := lookup.New(configBackend...)

	const key = "sidetree"
	if _, ok := cfgLookup.Lookup(key); !ok {
		return nil, errors.New("sidetree configuration key not found")
	}

	var sidetreeCfg sidetreeConfig
	if err = cfgLookup.UnmarshalKey(key, &sidetreeCfg); err != nil {
		return nil, err
	}

	return &sidetreeCfg, nil
}

// Protocol returns protocol client
func (m *SidetreeContext) Protocol() protocolApi.Client {
	return m.protocolClient
}

// Blockchain returns blockchain client
func (m *SidetreeContext) Blockchain() batch.BlockchainClient {
	return m.blockchainClient
}

// CAS returns content addressable storage client
func (m *SidetreeContext) CAS() batch.CASClient {
	return m.casClient
}

//sidetreeConfig defines 'fabric' channel used for recording Sidetree transaction
// and channel user for performing transactions on that channel
type sidetreeConfig struct {
	Channel string
	User    string
}
