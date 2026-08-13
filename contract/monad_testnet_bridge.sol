// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./evm_bridge_base.sol";

contract MonadTestnetLQDBridge is LQDWrappedBridgeToken {
    constructor() LQDWrappedBridgeToken("LQD Monad Testnet Bridge Token", "LQD", 8) {}
}

contract MonadTestnetTokenLock is LQDTokenLockVault {}
