// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract HarmonyTestnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Harmony Testnet Bridge Token", "LQD", 8) {}
}

contract HarmonyTestnetTokenLock is LQDTokenLockVault {}
