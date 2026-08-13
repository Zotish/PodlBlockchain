// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract FantomTestnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Fantom Testnet Bridge Token", "LQD", 8) {}
}

contract FantomTestnetTokenLock is LQDTokenLockVault {}
