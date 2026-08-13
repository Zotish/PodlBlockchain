// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract BaseMainnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Base Mainnet Bridge Token", "LQD", 8) {}
}

contract BaseMainnetTokenLock is LQDTokenLockVault {}
