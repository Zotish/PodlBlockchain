package constantset

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
	BLOCKCHAIN_DB_PATH      = "5001/evodb"
	BLOCKCHAIN_KEY          = "blockchain_key"
	MaxBlockGas             = 8000000
	ChainID                 = uint(139)
	MaxBlockSize            = 2 * 1024 * 1024
	MaxTxPoolSize           = 10000
	MaxTxsPerAccount        = 100
	BaseFeeUpdateBlock      = 10
	InitialBaseFee          = 1_000_000_000
	MinBaseFee              = 500_000_000
	MaxBaseFee              = 10_000_000_000
	BaseFeeChangeDenom      = 8
	RecentBlocksForTxCount  = 5
	TransactionTTL          = 3600
	ReplacementFeeBump      = 10
	GasLimitAdjustmentSpeed = 1024
	ContractCallGas         = 50000       // Gas for contract calls
	ContractDeployGas       = 100000      // Gas for contract deployment
	MaxContractSize         = 1024 * 1024 // 1MB max contract size
)
