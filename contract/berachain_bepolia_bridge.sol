// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract BerachainBepoliaLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Berachain Bepolia Bridge Token", "LQD", 8) {}
}

contract BerachainBepoliaTokenLock is LQDTokenLockVault {}
