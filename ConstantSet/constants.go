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
	BLOCKCHAIN_DB_PATH      = "5000/evodb"
	BLOCKCHAIN_KEY          = "blockchain_key"
	MaxBlockGas             = 8000000
	ChainID                 = uint(139)
	MaxBlockSize            = 2 * 1024 * 1024
	MaxTxPoolSize           = 100000000
	MaxTxsPerAccount        = 100
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
	GasPrice                = 1              // Gas price (1 LQD per gas unit)
	BaseDeployGas           = uint64(200000) // Base gas for deployment
	DeployGasPerByte        = uint64(25)     // Gas per byte of contract payload

	Leader             = 40
	ValidatorReward    = 20
	Liquidity_provider = 40
)
