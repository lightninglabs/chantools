module github.com/guggero/chantools

require (
	git.schwanenlied.me/yawning/bsaes.git v0.0.0-20190320102049-26d1add596b6 // indirect
	github.com/Yawning/aez v0.0.0-20180408160647-ec7426b44926 // indirect
	github.com/btcsuite/btcd v0.22.0-beta.0.20211005184431-e3449998be39
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f
	github.com/btcsuite/btcutil v1.0.3-0.20210527170813-e2ba6805a890
	github.com/btcsuite/btcutil/psbt v1.0.3-0.20210527170813-e2ba6805a890
	github.com/btcsuite/btcwallet v0.13.0
	github.com/btcsuite/btcwallet/wallet/txrules v1.1.0
	github.com/btcsuite/btcwallet/walletdb v1.3.6-0.20210803004036-eebed51155ec
	github.com/coreos/bbolt v1.3.3
	github.com/davecgh/go-spew v1.1.1
	github.com/frankban/quicktest v1.11.2 // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/lightningnetwork/lnd v0.14.1-beta
	github.com/lightningnetwork/lnd/kvdb v1.2.1
	github.com/ltcsuite/ltcd v0.0.0-20191228044241-92166e412499 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.7.0
	go.etcd.io/bbolt v1.3.6
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
)

replace github.com/lightningnetwork/lnd => github.com/guggero/lnd v0.11.0-beta.rc4.0.20211213093010-1807124d9195

replace github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt v3.2.2+incompatible

go 1.16
