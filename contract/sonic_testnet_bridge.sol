// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract SonicTestnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Sonic Testnet Bridge Token", "LQD", 8) {}
}

contract SonicTestnetTokenLock is LQDTokenLockVault {}
