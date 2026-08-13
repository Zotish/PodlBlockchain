// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract MantleSepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Mantle Sepolia Bridge Token", "LQD", 8) {}
}

contract MantleSepoliaTokenLock is LQDTokenLockVault {}
