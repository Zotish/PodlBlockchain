// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract BscMainnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD BSC Mainnet Bridge Token", "LQD", 8) {}
}

contract BscMainnetTokenLock is LQDTokenLockVault {}
