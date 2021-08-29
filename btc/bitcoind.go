package btc

import (
	"fmt"
	"io"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/lnd"
)

const (
	FormatCli          = "bitcoin-cli"
	FormatCliWatchOnly = "bitcoin-cli-watchonly"
	FormatImportwallet = "bitcoin-importwallet"
)

type KeyExporter interface {
	Header() string
	Format(*hdkeychain.ExtendedKey, *chaincfg.Params, string, uint32,
		uint32) (string, error)
	Trailer(uint32) string
}

// ParseFormat parses the given format name and returns its associated print
// function.
func ParseFormat(format string) KeyExporter {
	switch format {
	default:
		fallthrough

	case FormatCli:
		return &Cli{}

	case FormatCliWatchOnly:
		return &CliWatchOnly{}

	case FormatImportwallet:
		return &ImportWallet{}
	}
}

func ExportKeys(extendedKey *hdkeychain.ExtendedKey, strPaths []string,
	paths [][]uint32, params *chaincfg.Params, recoveryWindow,
	rescanFrom uint32, exporter KeyExporter, writer io.Writer) error {

	_, _ = fmt.Fprintf(
		writer, "# Wallet dump created by chantools on %s\n",
		time.Now().UTC(),
	)
	_, _ = fmt.Fprintf(writer, "%s\n", exporter.Header())
	for idx, strPath := range strPaths {
		path := paths[idx]

		// External branch first (<DerivationPath>/0/i).
		for i := uint32(0); i < recoveryWindow; i++ {
			path := append(path, 0, i)
			derivedKey, err := lnd.DeriveChildren(extendedKey, path)
			if err != nil {
				return err
			}
			result, err := exporter.Format(
				derivedKey, params, strPath, 0, i,
			)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(writer, "%s\n", result)
		}

		// Now the internal branch (<DerivationPath>/1/i).
		for i := uint32(0); i < recoveryWindow; i++ {
			path := append(path, 1, i)
			derivedKey, err := lnd.DeriveChildren(extendedKey, path)
			if err != nil {
				return err
			}
			result, err := exporter.Format(
				derivedKey, params, strPath, 1, i,
			)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(writer, "%s\n", result)
		}
	}

	_, _ = fmt.Fprintf(writer, "%s\n", exporter.Trailer(rescanFrom))
	return nil
}

func SeedBirthdayToBlock(params *chaincfg.Params,
	birthdayTimestamp time.Time) uint32 {

	var genesisTimestamp time.Time
	switch params.Name {
	case "mainnet":
		genesisTimestamp =
			chaincfg.MainNetParams.GenesisBlock.Header.Timestamp

	case "testnet3":
		genesisTimestamp =
			chaincfg.TestNet3Params.GenesisBlock.Header.Timestamp

	case "regtest", "simnet":
		return 0

	default:
		panic(fmt.Errorf("unimplemented network %v", params.Name))
	}

	// With the timestamps retrieved, we can estimate a block height by
	// taking the difference between them and dividing by the average block
	// time (10 minutes).
	return uint32(birthdayTimestamp.Sub(genesisTimestamp).Seconds() / 600)
}

type Cli struct{}

func (c *Cli) Header() string {
	return "# Paste the following lines into a command line window."
}

func (c *Cli) Format(hdKey *hdkeychain.ExtendedKey, params *chaincfg.Params,
	path string, branch, index uint32) (string, error) {

	privKey, err := hdKey.ECPrivKey()
	if err != nil {
		return "", fmt.Errorf("could not derive private key: %v", err)
	}
	wif, err := btcutil.NewWIF(privKey, params, true)
	if err != nil {
		return "", fmt.Errorf("could not encode WIF: %v", err)
	}
	flags := ""
	if params.Net == wire.TestNet || params.Net == wire.TestNet3 {
		flags = " -testnet"
	}
	return fmt.Sprintf("bitcoin-cli%s importprivkey %s \"%s/%d/%d/\" false",
		flags, wif.String(), path, branch, index), nil
}

func (c *Cli) Trailer(birthdayBlock uint32) string {
	return fmt.Sprintf("bitcoin-cli rescanblockchain %d\n", birthdayBlock)
}

type CliWatchOnly struct{}

func (c *CliWatchOnly) Header() string {
	return "# Paste the following lines into a command line window."
}

func (c *CliWatchOnly) Format(hdKey *hdkeychain.ExtendedKey,
	params *chaincfg.Params, path string, branch, index uint32) (string,
	error) {

	pubKey, err := hdKey.ECPubKey()
	if err != nil {
		return "", fmt.Errorf("could not derive private key: %v", err)
	}
	flags := ""
	if params.Net == wire.TestNet || params.Net == wire.TestNet3 {
		flags = " -testnet"
	}
	return fmt.Sprintf("bitcoin-cli%s importpubkey %x \"%s/%d/%d/\" false",
		flags, pubKey.SerializeCompressed(), path, branch, index), nil
}

func (c *CliWatchOnly) Trailer(birthdayBlock uint32) string {
	return fmt.Sprintf("bitcoin-cli rescanblockchain %d\n", birthdayBlock)
}

type ImportWallet struct{}

func (i *ImportWallet) Header() string {
	return "# Save this output to a file and use the importwallet " +
		"command of bitcoin core."
}

func (i *ImportWallet) Format(hdKey *hdkeychain.ExtendedKey,
	params *chaincfg.Params, path string, branch, index uint32) (string,
	error) {

	privKey, err := hdKey.ECPrivKey()
	if err != nil {
		return "", fmt.Errorf("could not derive private key: %v", err)
	}
	wif, err := btcutil.NewWIF(privKey, params, true)
	if err != nil {
		return "", fmt.Errorf("could not encode WIF: %v", err)
	}
	addrP2PKH, err := lnd.P2PKHAddr(privKey.PubKey(), params)
	if err != nil {
		return "", fmt.Errorf("could not create address: %v", err)
	}
	addrP2WKH, err := lnd.P2WKHAddr(privKey.PubKey(), params)
	if err != nil {
		return "", fmt.Errorf("could not create address: %v", err)
	}
	addrNP2WKH, err := lnd.NP2WKHAddr(privKey.PubKey(), params)
	if err != nil {
		return "", fmt.Errorf("could not create address: %v", err)
	}

	return fmt.Sprintf("%s 1970-01-01T00:00:01Z label=%s/%d/%d/ "+
		"# addr=%s,%s,%s", wif.String(), path, branch, index,
		addrP2PKH.EncodeAddress(), addrNP2WKH.EncodeAddress(),
		addrP2WKH.EncodeAddress(),
	), nil
}

func (i *ImportWallet) Trailer(_ uint32) string {
	return ""
}
