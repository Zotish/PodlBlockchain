// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract StoryAeneidLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Story Aeneid Bridge Token", "LQD", 8) {}
}

contract StoryAeneidTokenLock is LQDTokenLockVault {}
