package itest

import (
	"os"

	"github.com/btcsuite/btclog/v2"
)

var log btclog.Logger

//nolint:gochecknoinits
func init() {
	logger := btclog.NewSLogger(btclog.NewDefaultHandler(os.Stdout))
	logger.SetLevel(btclog.LevelTrace)
	log = logger.SubSystem("ITEST")
}
