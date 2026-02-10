package constantset

import "time"

var (
	AddressPre              = "0x"
	BlockHexPrefix          = "0x"
	StatusPending           = "pending"
	StatusSuccess           = "succsess"
	StatusFailed            = "failed"
	Decimals                = 8
	MinGas                  = 21000
	GasPerByte              = 68
	GasContractCall         = 20000
	BLOCKCHAIN_DB_PATH      = "5002/evodb"
	BLOCKCHAIN_KEY          = "blockchain_key"
	MaxBlockGas             = 40000000
	ChainID                 = uint(139)
	MaxBlockSize            = 2 * 1024 * 1024
	MaxTxPoolSize           = 100000000
	MaxTxsPerAccount        = 20000
	BaseFeeUpdateBlock      = 10
	InitialBaseFee          = 10
	MinBaseFee              = 10
	MaxBaseFee              = 10_000_000_000
	BaseFeeChangeDenom      = 8
	RecentBlocksForTxCount  = 5
	TransactionTTL          = 100 * time.Now().Year()
	ReplacementFeeBump      = 10
	GasLimitAdjustmentSpeed = 1024
	ContractCallGas         = 50000
	ContractDeployGas       = 100000
	MaxContractSize         = 1024 * 1024
	GasPrice                = 1
	BaseDeployGas           = uint64(200000)
	DeployGasPerByte        = uint64(25)
	LiquidityPoolAddress    = "0x1111111111111111111111111111111111111111"
	BridgeEscrowAddress     = "0x00000000000000000000000000000000000000b1"

	Leader             = 40
	ValidatorReward    = 20
	Liquidity_provider = 40
)
