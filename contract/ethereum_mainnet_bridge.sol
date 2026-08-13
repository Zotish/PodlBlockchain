// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract EthereumMainnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Ethereum Mainnet Bridge Token", "LQD", 8) {}
}

contract EthereumMainnetTokenLock is LQDTokenLockVault {}
