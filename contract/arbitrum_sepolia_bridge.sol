// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract ArbitrumSepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Arbitrum Sepolia Bridge Token", "LQD", 8) {}
}

contract ArbitrumSepoliaTokenLock is LQDTokenLockVault {}
