package mive

import (
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
)

// handlerConfig is the collection of initialization parameters to create a full
// node network handler.
type handlerConfig struct {
}

type handler struct {
	ethClient *ethclient.Client

	database ethdb.Database
}

// newHandler returns a handler for all Mive chain management protocol.
func newHandler(config *handlerConfig) (*handler, error) {
	return &handler{}, nil
}

func (h *handler) Start() {
}

func (h *handler) Stop() {
}
