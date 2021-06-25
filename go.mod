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
	github.com/frankban/quicktest v1.11.2 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/gogo/protobuf v1.2.1
	github.com/google/go-cmp v0.5.3 // indirect
	github.com/lightningnetwork/lnd v0.12.1-beta
	github.com/ltcsuite/ltcd v0.0.0-20191228044241-92166e412499 // indirect
	github.com/miekg/dns v1.1.26 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.6.1
	go.etcd.io/bbolt v1.3.5-0.20200615073812-232d8fc87f50
	golang.org/x/crypto v0.0.0-20200709230013-948cd5f35899
	golang.org/x/net v0.0.0-20200202094626-16171245cfb2 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/sys v0.0.0-20200501145240-bc7a7d42d5c3 // indirect
	golang.org/x/text v0.3.4 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/lightningnetwork/lnd => github.com/guggero/lnd v0.11.0-beta.rc4.0.20210609102733-8d0a492ec962

go 1.13
