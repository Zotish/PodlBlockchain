// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract OptimismSepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Optimism Sepolia Bridge Token", "LQD", 8) {}
}

contract OptimismSepoliaTokenLock is LQDTokenLockVault {}
