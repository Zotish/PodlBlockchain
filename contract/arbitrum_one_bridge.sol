// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract ArbitrumOneLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Arbitrum One Bridge Token", "LQD", 8) {}
}

contract ArbitrumOneTokenLock is LQDTokenLockVault {}
