package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/guggero/chantools/btc/fasthd"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/aezeed"
	"github.com/lightningnetwork/lnd/keychain"
)

var (
	nodeKeyDerivationPath = "m/1017'/%d'/%d'/0/0"
)

type vanityGenCommand struct {
	Prefix  string `long:"prefix" description:"Hex encoded prefix to find in node public key."`
	Threads int    `long:"threads" description:"Number of parallel threads." default:"4"`
}

func (c *vanityGenCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	prefixBytes, err := hex.DecodeString(c.Prefix)
	if err != nil {
		return fmt.Errorf("hex decoding of prefix failed: %v", err)
	}

	if len(prefixBytes) < 2 {
		return fmt.Errorf("prefix must be at least 2 bytes")
	}
	if !(prefixBytes[0] == 0x02 || prefixBytes[0] == 0x03) {
		return fmt.Errorf("prefix must start with 02 or 03 because " +
			"it's an EC public key")
	}

	path, err := lnd.ParsePath(fmt.Sprintf(
		nodeKeyDerivationPath, chainParams.HDCoinType,
		keychain.KeyFamilyNodeKey,
	))
	if err != nil {
		return err
	}

	numBits := ((len(prefixBytes) - 1) * 8) + 1
	numTries := math.Pow(2, float64(numBits))
	fmt.Printf("Running vanitygen on %d threads. Prefix bit length is %d, "+
		"expecting to approach\nprobability p=1.0 after %s seeds.\n",
		c.Threads, numBits, format(int64(numTries)))
	runtime.GOMAXPROCS(c.Threads)
	var (
		mtx         sync.Mutex
		globalCount uint64
		abort       = make(chan struct{})
		start       = time.Now()
	)

	for i := 0; i < c.Threads; i++ {
		go func() {
			var (
				entropy [16]byte
				count   uint64
			)
			for {
				select {
				case <-abort:
					return
				default:
				}

				if _, err := rand.Read(entropy[:]); err != nil {
					log.Error(err)
				}
				rootKey, err := fasthd.NewFastDerivation(
					entropy[:], chainParams,
				)
				if err != nil {
					log.Error(err)
				}
				err = rootKey.ChildPath(path)
				if err != nil {
					log.Error(err)
				}
				pubKeyBytes := rootKey.PubKeyBytes()

				if bytes.HasPrefix(
					pubKeyBytes, prefixBytes,
				) {
					seed, err := aezeed.New(
						aezeed.CipherSeedVersion,
						&entropy, time.Now(),
					)
					if err != nil {
						log.Error(err)
					}
					mnemonic, err := seed.ToMnemonic(nil)
					if err != nil {
						log.Error(err)
					}
					fmt.Printf("\nLooking for %x, found "+
						"pubkey: %x\nwith seed: %v\n",
						prefixBytes, pubKeyBytes,
						mnemonic)

					close(abort)
					return
				}

				if count > 0 && count%100 == 0 {
					mtx.Lock()
					globalCount += count
					count = 0
					mtx.Unlock()
				}
				count++
			}
		}()
	}

	lastCount := uint64(0)
	for {
		select {
		case <-abort:
			return nil
		case <-time.After(1 * time.Second):
			mtx.Lock()
			currentCount := globalCount
			mtx.Unlock()

			msg := fmt.Sprintf("Tested %sk seeds, p=%.5f, "+
				"speed=%dk/s, elapsed=%v",
				format(int64(currentCount/1000)),
				float64(currentCount)/numTries,
				(currentCount-lastCount)/1000,
				time.Since(start).Truncate(time.Second),
			)
			fmt.Printf("\r%-80s", msg)

			lastCount = currentCount
		}
	}
}

func format(n int64) string {
	in := strconv.FormatInt(n, 10)
	numOfDigits := len(in)
	if n < 0 {
		numOfDigits-- // First character is the - sign (not a digit)
	}
	numOfCommas := (numOfDigits - 1) / 3

	out := make([]byte, len(in)+numOfCommas)
	if n < 0 {
		in, out[0] = in[1:], '-'
	}

	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			return string(out)
		}
		if k++; k == 3 {
			j, k = j-1, 0
			out[j] = ','
		}
	}
}
