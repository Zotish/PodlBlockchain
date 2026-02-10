// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * Minimal bridge token + mint/burn gateway.
 * Deployer becomes relayer (can be changed).
 * This contract is for BSC testnet.
 */
contract LQDBridge {
    string public name = "LQD";
    string public symbol = "LQD";
    uint8 public decimals = 8;

    address public relayer;
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(bytes32 => bool) public processed;

    event Mint(address indexed to, uint256 amount, bytes32 id);
    event Burn(address indexed from, uint256 amount, bytes32 id, string toLqd);
    event RelayerUpdated(address indexed newRelayer);

    modifier onlyRelayer() {
        require(msg.sender == relayer, "relayer only");
        _;
    }

    constructor() {
        relayer = msg.sender;
    }

    function setRelayer(address r) external onlyRelayer {
        relayer = r;
        emit RelayerUpdated(r);
    }

    function mint(address to, uint256 amount, bytes32 id) external onlyRelayer {
        require(!processed[id], "already processed");
        processed[id] = true;
        balanceOf[to] += amount;
        totalSupply += amount;
        emit Mint(to, amount, id);
    }

    function burn(uint256 amount, string calldata toLqd) external {
        require(balanceOf[msg.sender] >= amount, "insufficient");
        balanceOf[msg.sender] -= amount;
        totalSupply -= amount;
        bytes32 id = keccak256(abi.encodePacked(msg.sender, amount, toLqd, block.number));
        emit Burn(msg.sender, amount, id, toLqd);
    }
}
