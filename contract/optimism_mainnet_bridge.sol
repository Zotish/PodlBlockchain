// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract OptimismMainnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Optimism Mainnet Bridge Token", "LQD", 8) {}
}

contract OptimismMainnetTokenLock is LQDTokenLockVault {}
