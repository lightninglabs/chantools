package ltconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/channeldb"
)

// LTRecoveryConfig holds all Lightning Terminal recovery configuration
type LTRecoveryConfig struct {
	Description string `json:"description"`
	Version     string `json:"version"`
	LightningTerminal LightningTerminalConfig `json:"lightning_terminal"`
	Testing     TestingConfig `json:"testing"`
}

// LightningTerminalConfig holds Lightning Terminal specific configuration
type LightningTerminalConfig struct {
	Keys           KeysConfig           `json:"keys"`
	Channel        ChannelConfig        `json:"channel"`
	Tapscript      TapscriptConfig      `json:"tapscript"`
	AuxiliaryLeaves AuxiliaryLeavesConfig `json:"auxiliary_leaves"`
}

// KeysConfig holds key-related configuration
type KeysConfig struct {
	ActualInternalKey     string `json:"actual_internal_key"`
	RemoteRevocationBase  string `json:"remote_revocation_base"`
	RemoteFundingKey      string `json:"remote_funding_key"`
}

// ChannelConfig holds channel-specific configuration
type ChannelConfig struct {
	Type      uint32   `json:"type"`
	CSVDelays []uint16 `json:"csv_delays"`
	KeyIndex  uint32   `json:"key_index"`
	Balance   uint64   `json:"balance"`
}

// TapscriptConfig holds tapscript testing configuration
type TapscriptConfig struct {
	ActualRoot     string             `json:"actual_root"`
	TestScenarios  []TapscriptScenario `json:"test_scenarios"`
}

// TapscriptScenario represents a tapscript root test scenario
type TapscriptScenario struct {
	Name string `json:"name"`
	Root string `json:"root"`
}

// AuxiliaryLeavesConfig holds auxiliary leaf configuration
type AuxiliaryLeavesConfig struct {
	TargetAuxHash   string           `json:"target_aux_hash"`
	AssetScenarios  []AssetScenario  `json:"asset_scenarios"`
}

// AssetScenario represents an asset commitment scenario
type AssetScenario struct {
	Version  int    `json:"version"`
	Name     string `json:"name"`
	RootHash string `json:"root_hash"`
	RootSum  uint64 `json:"root_sum"`
}

// TestingConfig holds general testing configuration
type TestingConfig struct {
	MaxHTLCIndex    uint64   `json:"max_htlc_index"`
	MaxKeysToTest   uint32   `json:"max_keys_to_test"`
	ChannelTypes    []interface{} `json:"channel_types"`
	DummyKey        string   `json:"dummy_key"`
}

// Global configuration instance
var Config *LTRecoveryConfig

// LoadConfig loads the Lightning Terminal recovery configuration from file
func LoadConfig(configPath string) error {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	config := &LTRecoveryConfig{}
	if err := json.Unmarshal(configData, config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Process computed values
	if err := processComputedValues(config); err != nil {
		return fmt.Errorf("failed to process computed values: %w", err)
	}

	Config = config
	return nil
}

// processComputedValues computes dynamic values in the configuration
func processComputedValues(config *LTRecoveryConfig) error {
	// Compute taproot assets marker
	taprootAssetsMarker := sha256.Sum256([]byte("taproot-assets"))
	
	// Update tapscript scenarios with computed values
	for i, scenario := range config.LightningTerminal.Tapscript.TestScenarios {
		switch scenario.Name {
		case "taproot_assets_marker":
			config.LightningTerminal.Tapscript.TestScenarios[i].Root = hex.EncodeToString(taprootAssetsMarker[:])
		}
	}
	
	// Update asset scenarios with computed values
	for i, scenario := range config.LightningTerminal.AuxiliaryLeaves.AssetScenarios {
		switch scenario.Name {
		case "single-asset":
			rootHash := sha256.Sum256([]byte("single-asset-root"))
			config.LightningTerminal.AuxiliaryLeaves.AssetScenarios[i].RootHash = hex.EncodeToString(rootHash[:])
		case "keyed-commitment":
			keyIndexBytes := []byte{byte(config.LightningTerminal.Channel.KeyIndex)}
			rootHash := sha256.Sum256(append([]byte("asset-root-"), keyIndexBytes...))
			config.LightningTerminal.AuxiliaryLeaves.AssetScenarios[i].RootHash = hex.EncodeToString(rootHash[:])
		case "csv-keyed-commitment":
			csvDelay := config.LightningTerminal.Channel.CSVDelays[0] // Use first CSV delay
			csvBytes := []byte{byte(csvDelay), byte(csvDelay >> 8)}
			rootHash := sha256.Sum256(append([]byte("csv-asset-"), csvBytes...))
			config.LightningTerminal.AuxiliaryLeaves.AssetScenarios[i].RootHash = hex.EncodeToString(rootHash[:])
		}
	}
	
	return nil
}

// GetActualInternalKey returns the parsed actual internal key
func (c *LTRecoveryConfig) GetActualInternalKey() (*btcec.PublicKey, error) {
	keyBytes, err := hex.DecodeString(c.LightningTerminal.Keys.ActualInternalKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode actual internal key: %w", err)
	}
	return btcec.ParsePubKey(keyBytes)
}

// GetRemoteRevocationBase returns the parsed remote revocation base key
func (c *LTRecoveryConfig) GetRemoteRevocationBase() (*btcec.PublicKey, error) {
	keyBytes, err := hex.DecodeString(c.LightningTerminal.Keys.RemoteRevocationBase)
	if err != nil {
		return nil, fmt.Errorf("failed to decode remote revocation base key: %w", err)
	}
	return btcec.ParsePubKey(keyBytes)
}

// GetRemoteFundingKey returns the parsed remote funding key
func (c *LTRecoveryConfig) GetRemoteFundingKey() (*btcec.PublicKey, error) {
	keyBytes, err := hex.DecodeString(c.LightningTerminal.Keys.RemoteFundingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode remote funding key: %w", err)
	}
	return btcec.ParsePubKey(keyBytes)
}

// GetDummyKey returns the parsed dummy key
func (c *LTRecoveryConfig) GetDummyKey() (*btcec.PublicKey, error) {
	keyBytes, err := hex.DecodeString(c.Testing.DummyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode dummy key: %w", err)
	}
	return btcec.ParsePubKey(keyBytes)
}

// GetActualTapscriptRoot returns the parsed actual tapscript root
func (c *LTRecoveryConfig) GetActualTapscriptRoot() ([]byte, error) {
	return hex.DecodeString(c.LightningTerminal.Tapscript.ActualRoot)
}

// GetTargetAuxHash returns the parsed target auxiliary hash
func (c *LTRecoveryConfig) GetTargetAuxHash() ([]byte, error) {
	return hex.DecodeString(c.LightningTerminal.AuxiliaryLeaves.TargetAuxHash)
}

// GetChannelTypes returns the parsed channel types
func (c *LTRecoveryConfig) GetChannelTypes() ([]channeldb.ChannelType, error) {
	var channelTypes []channeldb.ChannelType
	
	for _, ct := range c.Testing.ChannelTypes {
		switch v := ct.(type) {
		case float64:
			// JSON numbers are float64 by default
			channelTypes = append(channelTypes, channeldb.ChannelType(uint32(v)))
		case string:
			// Parse string representations of channel type combinations
			channelType, err := parseChannelTypeString(v)
			if err != nil {
				return nil, fmt.Errorf("failed to parse channel type '%s': %w", v, err)
			}
			channelTypes = append(channelTypes, channelType)
		}
	}
	
	return channelTypes, nil
}

// parseChannelTypeString parses channel type string representations
func parseChannelTypeString(s string) (channeldb.ChannelType, error) {
	// Handle combinations like "SimpleTaprootFeatureBit | TapscriptRootBit"
	parts := strings.Split(s, "|")
	var result channeldb.ChannelType
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "SimpleTaprootFeatureBit":
			result |= channeldb.SimpleTaprootFeatureBit
		case "TapscriptRootBit":
			result |= channeldb.TapscriptRootBit
		case "AnchorOutputsBit":
			result |= channeldb.AnchorOutputsBit
		case "ZeroHtlcTxFeeBit":
			result |= channeldb.ZeroHtlcTxFeeBit
		default:
			// Try to parse as a number
			if num, err := strconv.ParseUint(part, 10, 32); err == nil {
				result |= channeldb.ChannelType(num)
			} else {
				return 0, fmt.Errorf("unknown channel type: %s", part)
			}
		}
	}
	
	return result, nil
}

// GetTapscriptScenarios returns parsed tapscript scenarios
func (c *LTRecoveryConfig) GetTapscriptScenarios() ([]TapscriptTestScenario, error) {
	var scenarios []TapscriptTestScenario
	
	for _, scenario := range c.LightningTerminal.Tapscript.TestScenarios {
		var root []byte
		var err error
		
		if scenario.Root != "" {
			root, err = hex.DecodeString(scenario.Root)
			if err != nil {
				return nil, fmt.Errorf("failed to decode tapscript root for %s: %w", scenario.Name, err)
			}
		}
		
		scenarios = append(scenarios, TapscriptTestScenario{
			Name: scenario.Name,
			Root: root,
		})
	}
	
	return scenarios, nil
}

// TapscriptTestScenario represents a processed tapscript test scenario
type TapscriptTestScenario struct {
	Name string
	Root []byte
}

// GetAssetScenarios returns parsed asset scenarios
func (c *LTRecoveryConfig) GetAssetScenarios() ([]AssetTestScenario, error) {
	var scenarios []AssetTestScenario
	
	for _, scenario := range c.LightningTerminal.AuxiliaryLeaves.AssetScenarios {
		var rootHash [32]byte
		if scenario.RootHash != "" {
			rootBytes, err := hex.DecodeString(scenario.RootHash)
			if err != nil {
				return nil, fmt.Errorf("failed to decode root hash for %s: %w", scenario.Name, err)
			}
			copy(rootHash[:], rootBytes)
		}
		
		scenarios = append(scenarios, AssetTestScenario{
			Version:  scenario.Version,
			Name:     scenario.Name,
			RootHash: rootHash,
			RootSum:  scenario.RootSum,
		})
	}
	
	return scenarios, nil
}

// AssetTestScenario represents a processed asset test scenario
type AssetTestScenario struct {
	Version  int
	Name     string
	RootHash [32]byte
	RootSum  uint64
}