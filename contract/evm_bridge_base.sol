// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IERC20BridgeAsset {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}

/**
 * Shared wrapped LQD token used by EVM bridge deployments.
 * Relayer mints when LQD-side lock is confirmed; users burn to bridge back to LQD.
 */
contract LQDWrappedBridgeToken {
    string public name;
    string public symbol;
    uint8 public immutable decimals;

    address public owner;
    address public relayer;
    uint256 public totalSupply;

    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;
    mapping(bytes32 => bool) public processed;

    event Transfer(address indexed from, address indexed to, uint256 amount);
    event Approval(address indexed owner, address indexed spender, uint256 amount);
    event Mint(address indexed to, uint256 amount, bytes32 id);
    event Burn(address indexed from, uint256 amount, bytes32 id, string toLqd);
    event OwnerUpdated(address indexed newOwner);
    event RelayerUpdated(address indexed newRelayer);

    modifier onlyOwner() {
        require(msg.sender == owner, "owner only");
        _;
    }

    modifier onlyRelayer() {
        require(msg.sender == relayer, "relayer only");
        _;
    }

    constructor(string memory tokenName, string memory tokenSymbol, uint8 tokenDecimals) {
        owner = msg.sender;
        relayer = msg.sender;
        name = tokenName;
        symbol = tokenSymbol;
        decimals = tokenDecimals;
    }

    function setOwner(address newOwner) external onlyOwner {
        require(newOwner != address(0), "owner=0");
        owner = newOwner;
        emit OwnerUpdated(newOwner);
    }

    function setRelayer(address newRelayer) external onlyOwner {
        require(newRelayer != address(0), "relayer=0");
        relayer = newRelayer;
        emit RelayerUpdated(newRelayer);
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        _transfer(msg.sender, to, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        uint256 allowed = allowance[from][msg.sender];
        require(allowed >= amount, "allowance");
        if (allowed != type(uint256).max) {
            allowance[from][msg.sender] = allowed - amount;
            emit Approval(from, msg.sender, allowance[from][msg.sender]);
        }
        _transfer(from, to, amount);
        return true;
    }

    function mint(address to, uint256 amount, bytes32 id) external onlyRelayer {
        require(to != address(0), "to=0");
        require(amount > 0, "amount=0");
        require(!processed[id], "already processed");
        processed[id] = true;
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Transfer(address(0), to, amount);
        emit Mint(to, amount, id);
    }

    function burn(uint256 amount, string calldata toLqd) external {
        require(amount > 0, "amount=0");
        require(bytes(toLqd).length > 0, "toLqd empty");
        require(balanceOf[msg.sender] >= amount, "insufficient");
        balanceOf[msg.sender] -= amount;
        totalSupply -= amount;
        bytes32 id = keccak256(abi.encodePacked(block.chainid, address(this), msg.sender, amount, toLqd, block.number));
        emit Transfer(msg.sender, address(0), amount);
        emit Burn(msg.sender, amount, id, toLqd);
    }

    function _transfer(address from, address to, uint256 amount) internal {
        require(to != address(0), "to=0");
        require(balanceOf[from] >= amount, "insufficient");
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
    }
}

/**
 * External token vault used by EVM bridge deployments.
 * Users lock ERC20/BEP20/etc. tokens here; relayer releases after LQD-side burn.
 */
contract LQDTokenLockVault {
    address public owner;
    uint256 public nonce;
    mapping(bytes32 => bool) public processed;

    event Locked(address indexed token, address indexed from, uint256 amount, bytes32 id, string toLqd);
    event Released(address indexed token, address indexed to, uint256 amount, bytes32 id);
    event OwnerUpdated(address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "owner only");
        _;
    }

    constructor() {
        owner = msg.sender;
    }

    function setOwner(address newOwner) external onlyOwner {
        require(newOwner != address(0), "owner=0");
        owner = newOwner;
        emit OwnerUpdated(newOwner);
    }

    function lock(address token, uint256 amount, string calldata toLqd) external returns (bytes32) {
        require(token != address(0), "token=0");
        require(amount > 0, "amount=0");
        require(bytes(toLqd).length > 0, "toLqd empty");
        _safeTransferFrom(token, msg.sender, address(this), amount);
        bytes32 id = keccak256(abi.encodePacked(block.chainid, address(this), token, msg.sender, toLqd, amount, nonce++));
        emit Locked(token, msg.sender, amount, id, toLqd);
        return id;
    }

    function release(address token, address to, uint256 amount, bytes32 id) external onlyOwner {
        require(token != address(0), "token=0");
        require(to != address(0), "to=0");
        require(amount > 0, "amount=0");
        require(!processed[id], "already processed");
        processed[id] = true;
        _safeTransfer(token, to, amount);
        emit Released(token, to, amount, id);
    }

    function _safeTransfer(address token, address to, uint256 amount) private {
        (bool ok, bytes memory data) = token.call(abi.encodeCall(IERC20BridgeAsset.transfer, (to, amount)));
        require(ok && (data.length == 0 || abi.decode(data, (bool))), "transfer failed");
    }

    function _safeTransferFrom(address token, address from, address to, uint256 amount) private {
        (bool ok, bytes memory data) = token.call(abi.encodeCall(IERC20BridgeAsset.transferFrom, (from, to, amount)));
        require(ok && (data.length == 0 || abi.decode(data, (bool))), "transferFrom failed");
    }
}
