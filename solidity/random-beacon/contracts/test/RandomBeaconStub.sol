pragma solidity ^0.8.6;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@keep-network/sortition-pools/contracts/SortitionPool.sol";
import "../RandomBeacon.sol";
import "../libraries/Callback.sol";
import "../libraries/Groups.sol";
import {BeaconDkg as DKG} from "../libraries/BeaconDkg.sol";
import {BeaconDkgValidator as DKGValidator} from "../BeaconDkgValidator.sol";

contract RandomBeaconStub is RandomBeacon {
    constructor(
        SortitionPool _sortitionPool,
        IERC20 _tToken,
        IRandomBeaconStaking _staking,
        DKGValidator _dkgValidator
    ) RandomBeacon(_sortitionPool, _tToken, _staking, _dkgValidator) {}

    function getDkgData() external view returns (DKG.Data memory) {
        return dkg;
    }

    function getCallbackData() external view returns (Callback.Data memory) {
        return callback;
    }

    function roughlyAddGroup(
        bytes calldata groupPubKey,
        bytes32 groupMembersHash
    ) external {
        bytes32 groupPubKeyHash = keccak256(groupPubKey);

        Groups.Group memory group;
        group.groupPubKey = groupPubKey;
        group.membersHash = groupMembersHash;
        /* solhint-disable-next-line not-rely-on-time */
        group.registrationBlockNumber = block.number;

        groups.groupsData[groupPubKeyHash] = group;
        groups.groupsRegistry.push(groupPubKeyHash);
    }

    function groupLifetimeOf(bytes32 groupPubKeyHash)
        external
        view
        returns (uint256)
    {
        return
            groups.groupsData[groupPubKeyHash].registrationBlockNumber +
            groups.groupLifetime;
    }

    function roughlyTerminateGroup(uint64 groupId) public {
        groups.groupsData[groups.groupsRegistry[groupId]].terminated = true;
        // just add groupId without sorting for simplicity
        groups.activeTerminatedGroups.push(groupId);
    }

    function isGroupTerminated(uint64 groupId) external view returns (bool) {
        bytes32 groupPubKeyHash = groups.groupsRegistry[groupId];

        return groups.groupsData[groupPubKeyHash].terminated;
    }

    function publicDkgLockState() external {
        dkgLockState();
    }
}
