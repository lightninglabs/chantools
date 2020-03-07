module github.com/guggero/chantools

require (
	git.schwanenlied.me/yawning/bsaes.git v0.0.0-20190320102049-26d1add596b6 // indirect
	github.com/Yawning/aez v0.0.0-20180408160647-ec7426b44926 // indirect
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f
	github.com/btcsuite/btcutil v0.0.0-20191219182022-e17c9730c422
	github.com/btcsuite/btcwallet v0.11.1-0.20200219004649-ae9416ad7623
	github.com/btcsuite/btcwallet/walletdb v1.2.0
	github.com/davecgh/go-spew v1.1.1
	github.com/golang/protobuf v1.3.2 // indirect
	github.com/jessevdk/go-flags v1.4.0
	github.com/lightningnetwork/lnd v0.8.0-beta-rc3.0.20191224233846-f289a39c1a00
	github.com/ltcsuite/ltcd v0.0.0-20191228044241-92166e412499 // indirect
	github.com/miekg/dns v1.1.26 // indirect
	golang.org/x/crypto v0.0.0-20191227163750-53104e6ec876
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553 // indirect
	golang.org/x/sys v0.0.0-20191224085550-c709ea063b76 // indirect
	gopkg.in/yaml.v2 v2.2.3 // indirect
)

replace github.com/lightningnetwork/lnd => github.com/guggero/lnd v0.9.0-beta-rc1.0.20200307101759-2650bff06031

go 1.13
