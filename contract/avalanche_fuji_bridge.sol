// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract AvalancheFujiLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Avalanche Fuji Bridge Token", "LQD", 8) {}
}

contract AvalancheFujiTokenLock is LQDTokenLockVault {}
