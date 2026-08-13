// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract HyperliquidTestnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Hyperliquid Testnet Bridge Token", "LQD", 8) {}
}

contract HyperliquidTestnetTokenLock is LQDTokenLockVault {}
