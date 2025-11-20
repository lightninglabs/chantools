module github.com/lightninglabs/chantools

go 1.24.0

require (
	github.com/btcsuite/btcd v0.24.3-0.20250318170759-4f4ea81776d6
	github.com/btcsuite/btcd/btcec/v2 v2.3.5
	github.com/btcsuite/btcd/btcutil v1.1.5
	github.com/btcsuite/btcd/btcutil/psbt v1.1.10
	github.com/btcsuite/btcd/chaincfg/chainhash v1.1.0
	github.com/btcsuite/btclog v0.0.0-20241003133417-09c4e92e319c // indirect
	github.com/btcsuite/btcwallet v0.16.13
	github.com/btcsuite/btcwallet/wallet/txrules v1.2.2 // indirect
	github.com/btcsuite/btcwallet/walletdb v1.5.1
	github.com/coreos/bbolt v1.3.3
	github.com/davecgh/go-spew v1.1.1
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/hasura/go-graphql-client v0.9.1
	github.com/lightninglabs/loop v0.31.1-beta.0.20250527112532-ca8610e68e18
	github.com/lightninglabs/loop/swapserverrpc v1.0.14 // indirect
	github.com/lightninglabs/pool v0.6.6-beta
	// The current version of lnd we are compatible with, mostly affects the
	// commands that touch the channel DB and has an impact on the DB schema.
	// NOTE: When updating this version, make sure to also update the string in
	// cmd/chantools/root.go and run "make docs".
	github.com/lightningnetwork/lnd v0.19.0-beta
	github.com/lightningnetwork/lnd/kvdb v1.4.16
	github.com/lightningnetwork/lnd/queue v1.1.1
	github.com/lightningnetwork/lnd/ticker v1.1.1
	github.com/lightningnetwork/lnd/tor v1.1.6
	github.com/spf13/cobra v1.10.1
	github.com/stretchr/testify v1.11.1
	go.etcd.io/bbolt v1.3.11
	golang.org/x/crypto v0.45.0
	golang.org/x/oauth2 v0.30.0
)

require (
	github.com/btcsuite/btclog/v2 v2.0.1-0.20250110154127-3ae4bf1cb318
	github.com/lightningnetwork/lnd/fn/v2 v2.0.8
	github.com/tv42/zbase32 v0.0.0-20220222190657-f76a9fc892fa
	google.golang.org/protobuf v1.36.8
)

require (
	4d63.com/gocheckcompilerdirectives v1.3.0 // indirect
	4d63.com/gochecknoglobals v0.2.2 // indirect
	codeberg.org/chavacava/garif v0.2.0 // indirect
	dario.cat/mergo v1.0.1 // indirect
	dev.gaijin.team/go/exhaustruct/v4 v4.0.0 // indirect
	dev.gaijin.team/go/golib v0.6.0 // indirect
	github.com/4meepo/tagalign v1.4.3 // indirect
	github.com/Abirdcfly/dupword v0.1.7 // indirect
	github.com/AdminBenni/iota-mixing v1.0.0 // indirect
	github.com/AlwxSin/noinlineerr v1.0.5 // indirect
	github.com/Antonboom/errname v1.1.1 // indirect
	github.com/Antonboom/nilnil v1.1.1 // indirect
	github.com/Antonboom/testifylint v1.6.4 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/Djarvur/go-err113 v0.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/MirrexOne/unqueryvet v1.2.1 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/OpenPeeDeeP/depguard/v2 v2.2.1 // indirect
	github.com/Yawning/aez v0.0.0-20211027044916-e49e68abd344 // indirect
	github.com/aead/chacha20 v0.0.0-20180709150244-8b13a72661da // indirect
	github.com/aead/siphash v1.0.1 // indirect
	github.com/alecthomas/chroma/v2 v2.20.0 // indirect
	github.com/alecthomas/go-check-sumtype v0.3.1 // indirect
	github.com/alexkohler/nakedret/v2 v2.0.6 // indirect
	github.com/alexkohler/prealloc v1.0.0 // indirect
	github.com/alfatraining/structtag v1.0.0 // indirect
	github.com/alingse/asasalint v0.0.11 // indirect
	github.com/alingse/nilnesserr v0.2.0 // indirect
	github.com/ashanbrown/forbidigo/v2 v2.3.0 // indirect
	github.com/ashanbrown/makezero/v2 v2.1.0 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bkielbasa/cyclop v1.2.3 // indirect
	github.com/blizzy78/varnamelen v0.8.0 // indirect
	github.com/bombsimon/wsl/v4 v4.7.0 // indirect
	github.com/bombsimon/wsl/v5 v5.3.0 // indirect
	github.com/breml/bidichk v0.3.3 // indirect
	github.com/breml/errchkjson v0.4.1 // indirect
	github.com/btcsuite/btcwallet/wallet/txauthor v1.3.5 // indirect
	github.com/btcsuite/btcwallet/wallet/txsizes v1.2.5 // indirect
	github.com/btcsuite/btcwallet/wtxmgr v1.5.6 // indirect
	github.com/btcsuite/go-socks v0.0.0-20170105172521-4720035b7bfd // indirect
	github.com/btcsuite/websocket v0.0.0-20150119174127-31079b680792 // indirect
	github.com/btcsuite/winsvc v1.0.0 // indirect
	github.com/butuzov/ireturn v0.4.0 // indirect
	github.com/butuzov/mirror v1.3.0 // indirect
	github.com/catenacyber/perfsprint v0.10.0 // indirect
	github.com/ccojocar/zxcvbn-go v1.0.4 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charithe/durationcheck v0.0.11 // indirect
	github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc // indirect
	github.com/charmbracelet/lipgloss v1.1.0 // indirect
	github.com/charmbracelet/x/ansi v0.8.0 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13-0.20250311204145-2c3ea96c31dd // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/ckaznocha/intrange v0.3.1 // indirect
	github.com/containerd/continuity v0.3.0 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/curioswitch/go-reassign v0.3.0 // indirect
	github.com/daixiang0/gci v0.13.7 // indirect
	github.com/dave/dst v0.27.3 // indirect
	github.com/decred/dcrd/crypto/blake256 v1.0.1 // indirect
	github.com/decred/dcrd/lru v1.1.2 // indirect
	github.com/denis-tingaikin/go-header v0.5.0 // indirect
	github.com/dlclark/regexp2 v1.11.5 // indirect
	github.com/docker/cli v28.0.1+incompatible // indirect
	github.com/docker/docker v28.0.1+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ettle/strcase v0.2.0 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/fatih/structtag v1.2.0 // indirect
	github.com/fergusstrange/embedded-postgres v1.25.0 // indirect
	github.com/firefart/nonamedreturns v1.0.6 // indirect
	github.com/fortytw2/leaktest v1.3.0 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/fzipp/gocyclo v0.6.0 // indirect
	github.com/ghostiam/protogetter v0.3.17 // indirect
	github.com/go-critic/go-critic v0.14.2 // indirect
	github.com/go-errors/errors v1.0.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-macaroon-bakery/macaroonpb v1.0.0 // indirect
	github.com/go-toolsmith/astcast v1.1.0 // indirect
	github.com/go-toolsmith/astcopy v1.1.0 // indirect
	github.com/go-toolsmith/astequal v1.2.0 // indirect
	github.com/go-toolsmith/astfmt v1.1.0 // indirect
	github.com/go-toolsmith/astp v1.1.0 // indirect
	github.com/go-toolsmith/strparse v1.1.0 // indirect
	github.com/go-toolsmith/typep v1.1.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/go-xmlfmt/xmlfmt v1.1.3 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/godoc-lint/godoc-lint v0.10.1 // indirect
	github.com/gofrs/flock v0.13.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang-migrate/migrate/v4 v4.17.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/golangci/asciicheck v0.5.0 // indirect
	github.com/golangci/dupl v0.0.0-20250308024227-f665c8d69b32 // indirect
	github.com/golangci/go-printf-func-name v0.1.1 // indirect
	github.com/golangci/gofmt v0.0.0-20250106114630-d62b90e6713d // indirect
	github.com/golangci/golangci-lint/v2 v2.6.2 // indirect
	github.com/golangci/golines v0.0.0-20250217134842-442fd0091d95 // indirect
	github.com/golangci/misspell v0.7.0 // indirect
	github.com/golangci/plugin-module-register v0.1.2 // indirect
	github.com/golangci/revgrep v0.8.0 // indirect
	github.com/golangci/swaggoswag v0.0.0-20250504205917-77f2aca3143e // indirect
	github.com/golangci/unconvert v0.0.0-20250410112200-a129a6e6413e // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gordonklaus/ineffassign v0.2.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/gostaticanalysis/analysisutil v0.7.1 // indirect
	github.com/gostaticanalysis/comment v1.5.0 // indirect
	github.com/gostaticanalysis/forcetypeassert v0.2.0 // indirect
	github.com/gostaticanalysis/nilerr v0.1.2 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-immutable-radix/v2 v2.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hexops/gotextdiff v1.0.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.14.3 // indirect
	github.com/jackc/pgerrcode v0.0.0-20240316143900-6e2875d9b438 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.3 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgtype v1.14.0 // indirect
	github.com/jackc/pgx/v4 v4.18.2 // indirect
	github.com/jackc/pgx/v5 v5.6.0 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/jessevdk/go-flags v1.4.0 // indirect
	github.com/jgautheron/goconst v1.8.2 // indirect
	github.com/jingyugao/rowserrcheck v1.1.1 // indirect
	github.com/jjti/go-spancheck v0.6.5 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/jrick/logrotate v1.1.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/julz/importas v0.2.0 // indirect
	github.com/karamaru-alpha/copyloopvar v1.2.2 // indirect
	github.com/kisielk/errcheck v1.9.0 // indirect
	github.com/kkHAIKE/contextcheck v1.1.6 // indirect
	github.com/kkdai/bstream v1.0.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/kulti/thelper v0.7.1 // indirect
	github.com/kunwardeep/paralleltest v1.0.15 // indirect
	github.com/lasiar/canonicalheader v1.1.2 // indirect
	github.com/ldez/exptostd v0.4.5 // indirect
	github.com/ldez/gomoddirectives v0.7.1 // indirect
	github.com/ldez/grignotin v0.10.1 // indirect
	github.com/ldez/tagliatelle v0.7.2 // indirect
	github.com/ldez/usetesting v0.5.0 // indirect
	github.com/leonklingele/grouper v1.1.2 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/lightninglabs/gozmq v0.0.0-20191113021534-d20a764486bf // indirect
	github.com/lightninglabs/lndclient v0.19.0-7 // indirect
	github.com/lightninglabs/neutrino v0.16.1 // indirect
	github.com/lightninglabs/neutrino/cache v1.1.2 // indirect
	github.com/lightninglabs/pool/auctioneerrpc v1.1.3 // indirect
	github.com/lightningnetwork/lightning-onion v1.2.1-0.20240712235311-98bd56499dfb // indirect
	github.com/lightningnetwork/lnd/clock v1.1.1 // indirect
	github.com/lightningnetwork/lnd/healthcheck v1.2.6 // indirect
	github.com/lightningnetwork/lnd/sqldb v1.0.9 // indirect
	github.com/lightningnetwork/lnd/tlv v1.3.1 // indirect
	github.com/ltcsuite/ltcd v0.0.0-20191228044241-92166e412499 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/macabu/inamedparam v0.2.0 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/manuelarte/embeddedstructfieldcheck v0.4.0 // indirect
	github.com/manuelarte/funcorder v0.5.0 // indirect
	github.com/maratori/testableexamples v1.0.1 // indirect
	github.com/maratori/testpackage v1.1.2 // indirect
	github.com/matoous/godox v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mgechev/revive v1.12.0 // indirect
	github.com/miekg/dns v1.1.50 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/user v0.3.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/moricho/tparallel v0.3.2 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/nakabonne/nestif v0.3.1 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/nishanths/exhaustive v0.12.0 // indirect
	github.com/nishanths/predeclared v0.2.2 // indirect
	github.com/nunnatsa/ginkgolinter v0.21.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/opencontainers/runc v1.2.8 // indirect
	github.com/ory/dockertest/v3 v3.10.0 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/polyfloyd/go-errorlint v1.8.0 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/quasilyte/go-ruleguard v0.4.5 // indirect
	github.com/quasilyte/go-ruleguard/dsl v0.3.23 // indirect
	github.com/quasilyte/gogrep v0.5.0 // indirect
	github.com/quasilyte/regex/syntax v0.0.0-20210819130434-b3f0c404a727 // indirect
	github.com/quasilyte/stdinfo v0.0.0-20220114132959-f7386bf02567 // indirect
	github.com/raeperd/recvcheck v0.2.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rinchsan/gosimports v0.3.8 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/fastuuid v1.2.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryancurrah/gomodguard v1.4.1 // indirect
	github.com/ryanrolds/sqlclosecheck v0.5.1 // indirect
	github.com/sanposhiho/wastedassign/v2 v2.1.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/sashamelentyev/interfacebloat v1.1.0 // indirect
	github.com/sashamelentyev/usestdlibvars v1.29.0 // indirect
	github.com/securego/gosec/v2 v2.22.10 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/sivchari/containedctx v1.0.3 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/sonatard/noctx v0.4.0 // indirect
	github.com/sourcegraph/go-diff v0.7.0 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.12.0 // indirect
	github.com/ssgreg/nlreturn/v2 v2.2.1 // indirect
	github.com/stbenjam/no-sprintf-host-port v0.2.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/subosito/gotenv v1.4.1 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7 // indirect
	github.com/tetafro/godot v1.5.4 // indirect
	github.com/timakin/bodyclose v0.0.0-20241222091800-1db5c5ca4d67 // indirect
	github.com/timonwong/loggercheck v0.11.0 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20201229170055-e5319fda7802 // indirect
	github.com/tomarrell/wrapcheck/v2 v2.11.0 // indirect
	github.com/tommy-muehle/go-mnd/v2 v2.5.1 // indirect
	github.com/ultraware/funlen v0.2.0 // indirect
	github.com/ultraware/whitespace v0.2.0 // indirect
	github.com/uudashr/gocognit v1.2.0 // indirect
	github.com/uudashr/iface v1.4.1 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/xen0n/gosmopolitan v1.3.0 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yagipy/maintidx v1.0.0 // indirect
	github.com/yeya24/promlinter v0.3.0 // indirect
	github.com/ykadowak/zerologlint v0.1.5 // indirect
	gitlab.com/bosi/decorder v0.4.2 // indirect
	gitlab.com/yawning/bsaes.git v0.0.0-20190805113838-0a714cd429ec // indirect
	go-simpler.org/musttag v0.14.0 // indirect
	go-simpler.org/sloglint v0.11.1 // indirect
	go.augendre.info/arangolint v0.3.1 // indirect
	go.augendre.info/fatcontext v0.9.0 // indirect
	go.etcd.io/etcd/api/v3 v3.5.12 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.12 // indirect
	go.etcd.io/etcd/client/v2 v2.305.12 // indirect
	go.etcd.io/etcd/client/v3 v3.5.12 // indirect
	go.etcd.io/etcd/pkg/v3 v3.5.12 // indirect
	go.etcd.io/etcd/raft/v3 v3.5.12 // indirect
	go.etcd.io/etcd/server/v3 v3.5.12 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.29.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.20.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/mock v0.4.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20240909161429-701f63a606c0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20251023183803-a4bb9ffd2546 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/term v0.37.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	google.golang.org/genproto v0.0.0-20240213162025-012b6fc9bca9 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250818200422-3122310a409c // indirect
	google.golang.org/grpc v1.75.0 // indirect
	gopkg.in/errgo.v1 v1.0.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/macaroon-bakery.v2 v2.3.0 // indirect
	gopkg.in/macaroon.v2 v2.1.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	honnef.co/go/tools v0.6.1 // indirect
	modernc.org/libc v1.55.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
	modernc.org/sqlite v1.34.5 // indirect
	mvdan.cc/gofumpt v0.9.2 // indirect
	mvdan.cc/unparam v0.0.0-20251027182757-5beb8c8f8f15 // indirect
	nhooyr.io/websocket v1.8.7 // indirect
	pgregory.net/rapid v1.2.0 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

tool (
	github.com/golangci/golangci-lint/v2/cmd/golangci-lint
	github.com/rinchsan/gosimports/cmd/gosimports
)

// We want to format raw bytes as hex instead of base64. The forked version
// allows us to specify that as an option. This is required for the
// taproot-assets dependency to function properly.
replace google.golang.org/protobuf => github.com/lightninglabs/protobuf-go-hex-display v1.33.0-hex-display

replace github.com/btcsuite/btcd/btcutil/psbt => github.com/guggero/btcd/btcutil/psbt v0.0.0-20250512051801-35a277511ec5
