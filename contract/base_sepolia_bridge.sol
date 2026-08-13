// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract BaseSepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Base Sepolia Bridge Token", "LQD", 8) {}
}

contract BaseSepoliaTokenLock is LQDTokenLockVault {}
