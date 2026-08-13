// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract ScrollSepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Scroll Sepolia Bridge Token", "LQD", 8) {}
}

contract ScrollSepoliaTokenLock is LQDTokenLockVault {}
