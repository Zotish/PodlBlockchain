// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract MoonbaseAlphaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Moonbase Alpha Bridge Token", "LQD", 8) {}
}

contract MoonbaseAlphaTokenLock is LQDTokenLockVault {}
