// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract MetisSepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Metis Sepolia Bridge Token", "LQD", 8) {}
}

contract MetisSepoliaTokenLock is LQDTokenLockVault {}
