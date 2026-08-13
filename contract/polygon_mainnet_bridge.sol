// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract PolygonMainnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Polygon Mainnet Bridge Token", "LQD", 8) {}
}

contract PolygonMainnetTokenLock is LQDTokenLockVault {}
