package chansummary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jessevdk/go-flags"
	"io/ioutil"
)

const (
	defaultApiUrl = "https://blockstream.info/api"
)

type config struct {
	ApiUrl string `long:"apiurl" description:"API URL to use (must be esplora compatible)"`
}

type fileContent struct {
	Channels []*channel `json:"channels"`
}

func Main() error {
	var (
		err  error
		args []string
	)
	
	// Parse command line.
	config := &config{
		ApiUrl: defaultApiUrl,
	}
	if args, err = flags.Parse(config); err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("exactly one file argument needed")
	}
	file := args[0]
	
	// Read file and parse into channel.
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	channels := fileContent{}
	err = decoder.Decode(&channels)
	if err != nil {
		return err
	}

	return collectChanSummary(config, channels.Channels)
}
