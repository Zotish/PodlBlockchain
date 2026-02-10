// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

interface IERC20 {
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function transfer(address to, uint256 amount) external returns (bool);
}

// Lock contract for bridging BEP20 tokens to LQD chain.
contract BscTokenLock {
    address public owner;
    uint256 public nonce;

    event Locked(address indexed token, address indexed from, uint256 amount, bytes32 id, string toLqd);
    event Released(address indexed token, address indexed to, uint256 amount, bytes32 id);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    constructor() {
        owner = msg.sender;
    }

    function lock(address token, uint256 amount, string calldata toLqd) external returns (bytes32) {
        require(amount > 0, "amount=0");
        require(bytes(toLqd).length > 0, "toLqd empty");
        require(IERC20(token).transferFrom(msg.sender, address(this), amount), "transferFrom failed");
        bytes32 id = keccak256(abi.encodePacked(token, msg.sender, toLqd, amount, nonce++));
        emit Locked(token, msg.sender, amount, id, toLqd);
        return id;
    }

    function release(address token, address to, uint256 amount, bytes32 id) external onlyOwner {
        require(IERC20(token).transfer(to, amount), "transfer failed");
        emit Released(token, to, amount, id);
    }
}
