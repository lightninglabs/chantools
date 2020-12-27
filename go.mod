module github.com/guggero/chantools

require (
	git.schwanenlied.me/yawning/bsaes.git v0.0.0-20190320102049-26d1add596b6 // indirect
	github.com/Yawning/aez v0.0.0-20180408160647-ec7426b44926 // indirect
	github.com/btcsuite/btcd v0.21.0-beta.0.20201208033208-6bd4c64a54fa
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f
	github.com/btcsuite/btcutil v1.0.2
	github.com/btcsuite/btcutil/psbt v1.0.3-0.20200826194809-5f93e33af2b0
	github.com/btcsuite/btcwallet v0.11.1-0.20201207233335-415f37ff11a1
	github.com/btcsuite/btcwallet/walletdb v1.3.4
	github.com/coreos/bbolt v1.3.3
	github.com/davecgh/go-spew v1.1.1
	github.com/gohugoio/hugo v0.79.1 // indirect
	github.com/jessevdk/go-flags v1.4.0
	github.com/lightningnetwork/lnd v0.11.1-beta
	github.com/ltcsuite/ltcd v0.0.0-20191228044241-92166e412499 // indirect
	github.com/miekg/dns v1.1.26 // indirect
	github.com/spf13/cobra v1.1.1
	go.etcd.io/bbolt v1.3.5-0.20200615073812-232d8fc87f50
	golang.org/x/crypto v0.0.0-20200709230013-948cd5f35899
)

replace github.com/lightningnetwork/lnd => github.com/guggero/lnd v0.11.1-beta.rc5.0.20201214225837-5a6d8ff78da4

go 1.13
