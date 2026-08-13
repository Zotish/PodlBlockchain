// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract PolygonAmoyLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Polygon Amoy Bridge Token", "LQD", 8) {}
}

contract PolygonAmoyTokenLock is LQDTokenLockVault {}
