// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract ZkSyncSepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD zkSync Sepolia Bridge Token", "LQD", 8) {}
}

contract ZkSyncSepoliaTokenLock is LQDTokenLockVault {}
