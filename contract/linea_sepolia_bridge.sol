// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract LineaSepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Linea Sepolia Bridge Token", "LQD", 8) {}
}

contract LineaSepoliaTokenLock is LQDTokenLockVault {}
