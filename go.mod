module github.com/guggero/chantools

require (
	git.schwanenlied.me/yawning/bsaes.git v0.0.0-20190320102049-26d1add596b6 // indirect
	github.com/Yawning/aez v0.0.0-20180408160647-ec7426b44926 // indirect
	github.com/btcsuite/btcd v0.21.0-beta.0.20210513141527-ee5896bad5be
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f
	github.com/btcsuite/btcutil v1.0.3-0.20210527170813-e2ba6805a890
	github.com/btcsuite/btcutil/psbt v1.0.3-0.20210527170813-e2ba6805a890
	github.com/btcsuite/btcwallet v0.12.1-0.20210519225359-6ab9b615576f
	github.com/btcsuite/btcwallet/walletdb v1.3.5
	github.com/coreos/bbolt v1.3.3
	github.com/davecgh/go-spew v1.1.1
	github.com/frankban/quicktest v1.11.2 // indirect
	github.com/gogo/protobuf v1.2.1
	github.com/google/go-cmp v0.5.3 // indirect
	github.com/lightningnetwork/lnd v0.13.1-beta
	github.com/ltcsuite/ltcd v0.0.0-20191228044241-92166e412499 // indirect
	github.com/miekg/dns v1.1.26 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.7.0
	go.etcd.io/bbolt v1.3.5-0.20200615073812-232d8fc87f50
	golang.org/x/crypto v0.0.0-20200709230013-948cd5f35899
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/text v0.3.4 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/lightningnetwork/lnd => github.com/guggero/lnd v0.11.0-beta.rc4.0.20210726083410-a74ce5305eaa

replace github.com/lightningnetwork/lnd/kvdb => github.com/guggero/lnd/kvdb v0.0.0-20210726083410-a74ce5305eaa

// Fix incompatibility of etcd go.mod package.
// See https://github.com/etcd-io/etcd/issues/11154
replace go.etcd.io/etcd => go.etcd.io/etcd v0.5.0-alpha.5.0.20201125193152-8a03d2e9614b

go 1.13
