// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract BlastSepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Blast Sepolia Bridge Token", "LQD", 8) {}
}

contract BlastSepoliaTokenLock is LQDTokenLockVault {}
