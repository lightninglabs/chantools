// Copyright (c) 2014-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package fasthd

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
)

const (
	HardenedKeyStart = 0x80000000 // 2^31
	keyLen           = 33
)

var (
	ErrInvalidChild = errors.New("the extended key at this index is invalid")
	ErrUnusableSeed = errors.New("unusable seed")
	masterKey       = []byte("Bitcoin seed")
)

type FastDerivation struct {
	key       []byte
	chainCode []byte
	version   []byte
	scratch   [keyLen + 4]byte
}

func (k *FastDerivation) PubKeyBytes() []byte {
	pkx, pky := btcec.S256().ScalarBaseMult(k.key)
	pubKey := btcec.PublicKey{Curve: btcec.S256(), X: pkx, Y: pky}
	return pubKey.SerializeCompressed()
}

func (k *FastDerivation) Child(i uint32) error {
	isChildHardened := i >= HardenedKeyStart
	if isChildHardened {
		copy(k.scratch[1:], k.key)
	} else {
		copy(k.scratch[:], k.PubKeyBytes())
	}
	binary.BigEndian.PutUint32(k.scratch[keyLen:], i)

	hmac512 := hmac.New(sha512.New, k.chainCode)
	_, _ = hmac512.Write(k.scratch[:])
	ilr := hmac512.Sum(nil)

	il := ilr[:len(ilr)/2]
	childChainCode := ilr[len(ilr)/2:]

	ilNum := new(big.Int).SetBytes(il)
	if ilNum.Cmp(btcec.S256().N) >= 0 || ilNum.Sign() == 0 {
		return ErrInvalidChild
	}

	keyNum := new(big.Int).SetBytes(k.key)
	ilNum.Add(ilNum, keyNum)
	ilNum.Mod(ilNum, btcec.S256().N)

	k.key = ilNum.Bytes()
	k.chainCode = childChainCode

	return nil
}

func (k *FastDerivation) ChildPath(path []uint32) error {
	for _, pathPart := range path {
		if err := k.Child(pathPart); err != nil {
			return err
		}
	}
	return nil
}

func NewFastDerivation(seed []byte, net *chaincfg.Params) (*FastDerivation, error) {
	// First take the HMAC-SHA512 of the master key and the seed data:
	//   I = HMAC-SHA512(Key = "Bitcoin seed", Data = S)
	hmac512 := hmac.New(sha512.New, masterKey)
	_, _ = hmac512.Write(seed)
	lr := hmac512.Sum(nil)

	// Split "I" into two 32-byte sequences Il and Ir where:
	//   Il = master secret key
	//   Ir = master chain code
	secretKey := lr[:len(lr)/2]
	chainCode := lr[len(lr)/2:]

	// Ensure the key in usable.
	secretKeyNum := new(big.Int).SetBytes(secretKey)
	if secretKeyNum.Cmp(btcec.S256().N) >= 0 || secretKeyNum.Sign() == 0 {
		return nil, ErrUnusableSeed
	}

	return &FastDerivation{
		key:       secretKey,
		chainCode: chainCode,
		version:   net.HDPrivateKeyID[:],
	}, nil
}
