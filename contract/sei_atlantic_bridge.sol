// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract SeiAtlanticLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Sei Atlantic Bridge Token", "LQD", 8) {}
}

contract SeiAtlanticTokenLock is LQDTokenLockVault {}
