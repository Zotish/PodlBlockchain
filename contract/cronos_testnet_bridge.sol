// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract CronosTestnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Cronos Testnet Bridge Token", "LQD", 8) {}
}

contract CronosTestnetTokenLock is LQDTokenLockVault {}
