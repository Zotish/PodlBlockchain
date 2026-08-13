// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract EthereumSepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Ethereum Sepolia Bridge Token", "LQD", 8) {}
}

contract EthereumSepoliaTokenLock is LQDTokenLockVault {}
