// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract AvalancheCLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Avalanche C-Chain Bridge Token", "LQD", 8) {}
}

contract AvalancheCTokenLock is LQDTokenLockVault {}
