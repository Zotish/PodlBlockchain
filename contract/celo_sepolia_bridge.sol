// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract CeloSepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Celo Sepolia Bridge Token", "LQD", 8) {}
}

contract CeloSepoliaTokenLock is LQDTokenLockVault {}
