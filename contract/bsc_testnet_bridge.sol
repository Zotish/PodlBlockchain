// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract BscTestnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD BSC Testnet Bridge Token", "LQD", 8) {}
}

contract BscTestnetTokenLock is LQDTokenLockVault {}
