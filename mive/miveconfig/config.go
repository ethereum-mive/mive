package miveconfig

// Config contains configuration options for the Mive protocol.
type Config struct {
	EthRpcUrl string

	// State scheme represents the scheme used to store ethereum states and trie
	// nodes on top. It can be 'hash', 'path', or none which means use the scheme
	// consistent with persistent state.
	StateScheme string `toml:",omitempty"`

	// Database options
	DatabaseHandles int `toml:"-"`
	DatabaseCache   int
	DatabaseFreezer string

	// Enables tracking of SHA3 preimages in the VM
	EnablePreimageRecording bool
}
