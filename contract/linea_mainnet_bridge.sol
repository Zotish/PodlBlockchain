// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract LineaMainnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Linea Mainnet Bridge Token", "LQD", 8) {}
}

contract LineaMainnetTokenLock is LQDTokenLockVault {}
