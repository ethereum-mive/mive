package params

import "github.com/ethereum/go-ethereum/common"

const (
	DefaultFeeReductionDenominator = 50       // Bounds the reduction amount the various fees may have in Mive.
	DefaultBlockGasLimitMultiplier = 100      // Bounds the maximum gas limit a Mive block may have.
	DefaultMinBlockGasLimit        = 30000000 // Minimum gas limit for a Mive block.
)

var (
	// DefaultBeaconAddress is the default beacon address, which has suffix "315e" (a variant of "mive").
	DefaultBeaconAddress = common.HexToAddress("0x000000000000000000000000000000000000315e")

	// BeneficiaryAddress is the address that will receive tx fees.
	// TODO
	BeneficiaryAddress = common.HexToAddress("0x0000000000000000000000000000000000000000")
)
