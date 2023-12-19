package mive

// Mive implements the Mive indexer and execution layer service.
type Mive struct {
}

func New() (*Mive, error) {
	return &Mive{}, nil
}
