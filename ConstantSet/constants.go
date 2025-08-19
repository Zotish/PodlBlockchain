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
	BLOCKCHAIN_DB_PATH      = "5000/evodb"
	BLOCKCHAIN_KEY          = "blockchain_key"
	MaxBlockGas             = 8000000   // Adjust as needed
	ChainID                 = uint(137) // Mainnet chain ID - change per network
	MaxBlockSize            = 2 * 1024 * 1024
	MaxTxPoolSize           = 10000
	MaxTxsPerAccount        = 100
	BaseFeeUpdateBlock      = 10             // Bl
	InitialBaseFee          = 1_000_000_000  // 1 Gwei in wei
	MinBaseFee              = 500_000_000    // 0.5 Gwei minimum
	MaxBaseFee              = 10_000_000_000 // 10 Gwei cap
	BaseFeeChangeDenom      = 8              // 1/8 = 12.5% max change
	RecentBlocksForTxCount  = 5              // Blocks to consider for sender tx count
	TransactionTTL          = 3600           // 1 hour in seconds
	ReplacementFeeBump      = 10             // 10% fee bump required for replacement
	GasLimitAdjustmentSpeed = 1024           // How quickly gas limit adjusts
)
