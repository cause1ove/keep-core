// SPDX-License-Identifier: MIT
//
// ▓▓▌ ▓▓ ▐▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▄
// ▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
//   ▓▓▓▓▓▓    ▓▓▓▓▓▓▓▀    ▐▓▓▓▓▓▓    ▐▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
//   ▓▓▓▓▓▓▄▄▓▓▓▓▓▓▓▀      ▐▓▓▓▓▓▓▄▄▄▄         ▓▓▓▓▓▓▄▄▄▄         ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
//   ▓▓▓▓▓▓▓▓▓▓▓▓▓▀        ▐▓▓▓▓▓▓▓▓▓▓         ▓▓▓▓▓▓▓▓▓▓         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
//   ▓▓▓▓▓▓▀▀▓▓▓▓▓▓▄       ▐▓▓▓▓▓▓▀▀▀▀         ▓▓▓▓▓▓▀▀▀▀         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▀
//   ▓▓▓▓▓▓   ▀▓▓▓▓▓▓▄     ▐▓▓▓▓▓▓     ▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌
// ▓▓▓▓▓▓▓▓▓▓ █▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
// ▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
//
//

// Initial version copied from Keep Network ECDSA Wallets:
// https://github.com/keep-network/keep-core/blob/5286ebc8ff99b2aa6569f9dd97fd6995f25b4630/solidity/ecdsa/contracts/libraries/EcdsaAuthorization.sol
//
// With the following differences:
// - functions' visibility was changed to public/external to deploy it as a linked
//   library.
// - documentation was updated to be more generic.

pragma solidity ^0.8.9;

import "@keep-network/sortition-pools/contracts/SortitionPool.sol";
import "@threshold-network/solidity-contracts/contracts/staking/IStaking.sol";

/// @notice Library managing the state of stake authorizations for the operator
///         contract and the presence of operators in the sortition
///         pool based on the stake authorized for them.
library BeaconAuthorization {
    struct Parameters {
        // The minimum authorization required by the beacon so that
        // operator can join the sortition pool and do the work.
        uint96 minimumAuthorization;
        // Authorization decrease delay in seconds between the time
        // authorization decrease is requested and the time the authorization
        // decrease can be approved. It is always the same value, no matter if
        // authorization decrease amount is small, significant, or if it is
        // a decrease to zero.
        uint64 authorizationDecreaseDelay;
    }

    struct AuthorizationDecrease {
        uint96 decreasingBy; // amount
        uint64 decreasingAt; // timestamp
    }

    struct Data {
        Parameters parameters;
        mapping(address => address) stakingProviderToOperator;
        mapping(address => address) operatorToStakingProvider;
        mapping(address => AuthorizationDecrease) pendingDecreases;
    }

    event OperatorRegistered(
        address indexed stakingProvider,
        address indexed operator
    );

    event AuthorizationIncreased(
        address indexed stakingProvider,
        uint96 fromAmount,
        uint96 toAmount
    );

    event AuthorizationDecreaseRequested(
        address indexed stakingProvider,
        uint96 fromAmount,
        uint96 toAmount,
        uint64 decreasingAt
    );

    event AuthorizationDecreaseApproved(address indexed stakingProvider);

    event InvoluntaryAuthorizationDecreaseFailed(
        address indexed stakingProvider,
        uint96 fromAmount,
        uint96 toAmount
    );

    event OperatorJoinedSortitionPool(
        address indexed stakingProvider,
        address indexed operator
    );

    event OperatorStatusUpdated(
        address indexed stakingProvider,
        address indexed operator
    );

    /// @notice Sets the minimum authorization for the beacon. Without
    ///         at least the minimum authorization, staking provider is not
    ///         eligible to join and operate in the network.
    function setMinimumAuthorization(
        Data storage self,
        uint96 _minimumAuthorization
    ) external {
        self.parameters.minimumAuthorization = _minimumAuthorization;
    }

    /// @notice Sets the authorization decrease delay. It is the time in seconds
    ///         that needs to pass between the time authorization decrease is
    ///         requested and the time the authorization decrease can be
    ///         approved, no matter the authorization decrease amount.
    function setAuthorizationDecreaseDelay(
        Data storage self,
        uint64 _authorizationDecreaseDelay
    ) external {
        self
            .parameters
            .authorizationDecreaseDelay = _authorizationDecreaseDelay;
    }

    /// @notice Used by staking provider to set operator address that will
    ///         operate a node. The given staking provider can set operator
    ///         address only one time. The operator address can not be changed
    ///         and must be unique. Reverts if the operator is already set for
    ///         the staking provider or if the operator address is already in
    ///         use. Reverts if there is a pending authorization decrease for
    ///         the staking provider.
    function registerOperator(Data storage self, address operator) public {
        address stakingProvider = msg.sender;

        require(operator != address(0), "Operator can not be zero address");
        require(
            self.stakingProviderToOperator[stakingProvider] == address(0),
            "Operator already set for the staking provider"
        );
        require(
            self.operatorToStakingProvider[operator] == address(0),
            "Operator address already in use"
        );

        // Authorization request for a staking provider who has not yet
        // registered their operator can be approved immediately.
        // We need to make sure that the approval happens before operator
        // is registered to do not let the operator join the sortition pool
        // with an unresolved authorization decrease request that can be
        // approved at any point.
        AuthorizationDecrease storage decrease = self.pendingDecreases[
            stakingProvider
        ];
        require(
            decrease.decreasingAt == 0,
            "There is a pending authorization decrease request"
        );

        emit OperatorRegistered(stakingProvider, operator);

        self.stakingProviderToOperator[stakingProvider] = operator;
        self.operatorToStakingProvider[operator] = stakingProvider;
    }

    /// @notice Used by T staking contract to inform the beacon that the
    ///         authorized stake amount for the given staking provider increased.
    ///
    ///         Reverts if the authorization amount is below the minimum.
    ///
    ///         The function is not updating the sortition pool. Sortition pool
    ///         state needs to be updated by the operator with a call to
    ///         `joinSortitionPool` or `updateOperatorStatus`.
    ///
    /// @dev Should only be callable by T staking contract.
    function authorizationIncreased(
        Data storage self,
        address stakingProvider,
        uint96 fromAmount,
        uint96 toAmount
    ) external {
        require(
            toAmount >= self.parameters.minimumAuthorization,
            "Authorization below the minimum"
        );

        // Note that this function does not require the operator address to be
        // set for the given staking provider. This allows the stake owner
        // who is also an authorizer to increase the authorization before the
        // staking provider sets the operator. This allows delegating stake
        // and increasing authorization immediately one after another without
        // having to wait for the staking provider to do their part.

        emit AuthorizationIncreased(stakingProvider, fromAmount, toAmount);
    }

    /// @notice Used by T staking contract to inform the beacon that the
    ///         authorization decrease for the given staking provider has been
    ///         requested.
    ///
    ///         Reverts if the amount after deauthorization would be non-zero
    ///         and lower than the minimum authorization.
    ///
    ///         If the operator is not known (`registerOperator` was not called)
    ///         it lets to `approveAuthorizationDecrease` immediately. If the
    ///         operator is known (`registerOperator` was called), the operator
    ///         needs to update state of the sortition pool with a call to
    ///         `joinSortitionPool` or `updateOperatorStatus`. After the
    ///         sortition pool state is in sync, authorization decrease delay
    ///         starts.
    ///
    ///         After authorization decrease delay passes, authorization
    ///         decrease request needs to be approved with a call to
    ///         `approveAuthorizationDecrease` function.
    ///
    ///         If there is a pending authorization decrease request, it is
    ///         overwritten.
    ///
    /// @dev Should only be callable by T staking contract.
    function authorizationDecreaseRequested(
        Data storage self,
        address stakingProvider,
        uint96 fromAmount,
        uint96 toAmount
    ) public {
        require(
            toAmount == 0 || toAmount >= self.parameters.minimumAuthorization,
            "Authorization amount should be 0 or above the minimum"
        );

        address operator = self.stakingProviderToOperator[stakingProvider];

        uint64 decreasingAt;

        if (operator == address(0)) {
            // Operator is not known. It means `registerOperator` was not
            // called yet, and there is no chance the operator could
            // call `joinSortitionPool`. We can let to approve authorization
            // decrease immediately because that operator was never in the
            // sortition pool.

            // solhint-disable-next-line not-rely-on-time
            decreasingAt = uint64(block.timestamp);
        } else {
            // Operator is known. It means that this operator is or was in
            // the sortition pool. Before authorization decrease delay starts,
            // the operator needs to update the state of the sortition pool
            // with a call to `joinSortitionPool` or `updateOperatorStatus`.
            // For now, we set `decreasingAt` as "never decreasing" and let
            // it be updated by `joinSortitionPool` or `updateOperatorStatus`
            // once we know the sortition pool is in sync.

            // solhint-disable-next-line not-rely-on-time
            decreasingAt = type(uint64).max;
        }

        uint96 decreasingBy = fromAmount - toAmount;

        self.pendingDecreases[stakingProvider] = AuthorizationDecrease(
            decreasingBy,
            decreasingAt
        );

        emit AuthorizationDecreaseRequested(
            stakingProvider,
            fromAmount,
            toAmount,
            decreasingAt
        );
    }

    /// @notice Approves the previously registered authorization decrease
    ///         request. Reverts if authorization decrease delay have not passed
    ///         yet or if the authorization decrease was not requested for the
    ///         given staking provider.
    function approveAuthorizationDecrease(
        Data storage self,
        IStaking tokenStaking,
        address stakingProvider
    ) public {
        AuthorizationDecrease storage decrease = self.pendingDecreases[
            stakingProvider
        ];
        require(
            decrease.decreasingAt > 0,
            "Authorization decrease not requested"
        );
        require(
            decrease.decreasingAt != type(uint64).max,
            "Authorization decrease request not activated"
        );
        require(
            // solhint-disable-next-line not-rely-on-time
            block.timestamp >= decrease.decreasingAt,
            "Authorization decrease delay not passed"
        );

        emit AuthorizationDecreaseApproved(stakingProvider);

        // slither-disable-next-line unused-return
        tokenStaking.approveAuthorizationDecrease(stakingProvider);
        delete self.pendingDecreases[stakingProvider];
    }

    /// @notice Used by T staking contract to inform the beacon the
    ///         authorization has been decreased for the given staking provider
    ///         involuntarily, as a result of slashing.
    ///
    ///         If the operator is not known (`registerOperator` was not called)
    ///         the function does nothing. The operator was never in a sortition
    ///         pool so there is nothing to update.
    ///
    ///         If the operator is known, sortition pool is unlocked, and the
    ///         operator is in the sortition pool, the sortition pool state is
    ///         updated. If the sortition pool is locked, update needs to be
    ///         postponed. Every other staker is incentivized to call
    ///         `updateOperatorStatus` for the problematic operator to increase
    ///         their own rewards in the pool.
    ///
    /// @dev Should only be callable by T staking contract.
    function involuntaryAuthorizationDecrease(
        Data storage self,
        IStaking tokenStaking,
        SortitionPool sortitionPool,
        address stakingProvider,
        uint96 fromAmount,
        uint96 toAmount
    ) external {
        address operator = self.stakingProviderToOperator[stakingProvider];

        if (operator == address(0)) {
            // Operator is not known. It means `registerOperator` was not
            // called yet, and there is no chance the operator could
            // call `joinSortitionPool`. We can just ignore this update because
            // operator was never in the sortition pool.
            return;
        } else {
            // Operator is known. It means that this operator is or was in the
            // sortition pool and the sortition pool may need to be updated.
            //
            // If the sortition pool is not locked and the operator is in the
            // sortition pool, we are updating it.
            //
            // To keep stakes synchronized between applications when staking
            // providers are slashed, without the risk of running out of gas,
            // the staking contract queues up slashings and let users process
            // the transactions. When an application slashes one or more staking
            // providers, it adds them to the slashing queue on the staking
            // contract. A queue entry contains the staking provider’s address
            // and the amount they are due to be slashed.
            //
            // When there is at least one staking provider in the slashing
            // queue, any account can submit a transaction processing one or
            // more staking providers' slashings, and collecting a reward for
            // doing so. A queued slashing is processed by updating the staking
            // provider’s stake to the post-slashing amount, updating authorized
            // amount for each affected application, and notifying all affected
            // applications that the staking provider’s authorized stake has
            // been reduced due to slashing.
            //
            // The entire idea is that the process transaction is expensive
            // because each application needs to be updated, so the reward for
            // the processor is hefty and comes from the slashed tokens.
            // Practically, it means that if the sortition pool is unlocked, and
            // can be updated, it should be updated because we already paid
            // someone for updating it.
            //
            // If the sortition pool is locked, update needs to wait. Other
            // sortition pool members are incentivized to call
            // `updateOperatorStatus` for the problematic operator because they
            // will increase their rewards this way.
            if (sortitionPool.isOperatorInPool(operator)) {
                if (sortitionPool.isLocked()) {
                    emit InvoluntaryAuthorizationDecreaseFailed(
                        stakingProvider,
                        fromAmount,
                        toAmount
                    );
                } else {
                    updateOperatorStatus(
                        self,
                        tokenStaking,
                        sortitionPool,
                        operator
                    );
                }
            }
        }
    }

    /// @notice Lets the operator join the sortition pool. The operator address
    ///         must be known - before calling this function, it has to be
    ///         appointed by the staking provider by calling `registerOperator`.
    ///         Also, the operator must have the minimum authorization required
    ///         by the beacon. Function reverts if there is no minimum stake
    ///         authorized or if the operator is not known. If there was an
    ///         authorization decrease requested, it is activated by starting
    ///         the authorization decrease delay.
    function joinSortitionPool(
        Data storage self,
        IStaking tokenStaking,
        SortitionPool sortitionPool
    ) public {
        address operator = msg.sender;

        address stakingProvider = self.operatorToStakingProvider[operator];
        require(stakingProvider != address(0), "Unknown operator");

        AuthorizationDecrease storage decrease = self.pendingDecreases[
            stakingProvider
        ];

        uint96 _eligibleStake = eligibleStake(
            self,
            tokenStaking,
            stakingProvider,
            decrease.decreasingBy
        );

        require(_eligibleStake != 0, "Authorization below the minimum");

        emit OperatorJoinedSortitionPool(stakingProvider, operator);

        sortitionPool.insertOperator(operator, _eligibleStake);

        // If there is a pending authorization decrease request, activate it.
        // At this point, the sortition pool state is up to date so the
        // authorization decrease delay can start counting.
        if (decrease.decreasingAt == type(uint64).max) {
            decrease.decreasingAt =
                // solhint-disable-next-line not-rely-on-time
                uint64(block.timestamp) +
                self.parameters.authorizationDecreaseDelay;
        }
    }

    /// @notice Updates status of the operator in the sortition pool. If there
    ///         was an authorization decrease requested, it is activated by
    ///         starting the authorization decrease delay.
    ///         Function reverts if the operator is not known.
    function updateOperatorStatus(
        Data storage self,
        IStaking tokenStaking,
        SortitionPool sortitionPool,
        address operator
    ) public {
        address stakingProvider = self.operatorToStakingProvider[operator];
        require(stakingProvider != address(0), "Unknown operator");

        AuthorizationDecrease storage decrease = self.pendingDecreases[
            stakingProvider
        ];

        emit OperatorStatusUpdated(stakingProvider, operator);

        if (sortitionPool.isOperatorInPool(operator)) {
            uint96 _eligibleStake = eligibleStake(
                self,
                tokenStaking,
                stakingProvider,
                decrease.decreasingBy
            );

            sortitionPool.updateOperatorStatus(operator, _eligibleStake);
        }

        // If there is a pending authorization decrease request, activate it.
        // At this point, the sortition pool state is up to date so the
        // authorization decrease delay can start counting.
        if (decrease.decreasingAt == type(uint64).max) {
            decrease.decreasingAt =
                // solhint-disable-next-line not-rely-on-time
                uint64(block.timestamp) +
                self.parameters.authorizationDecreaseDelay;
        }
    }

    /// @notice Checks if the operator's authorized stake is in sync with
    ///         operator's weight in the sortition pool.
    ///         If the operator is not in the sortition pool and their
    ///         authorized stake is non-zero, function returns false.
    function isOperatorUpToDate(
        Data storage self,
        IStaking tokenStaking,
        SortitionPool sortitionPool,
        address operator
    ) external view returns (bool) {
        address stakingProvider = self.operatorToStakingProvider[operator];
        require(stakingProvider != address(0), "Unknown operator");

        AuthorizationDecrease storage decrease = self.pendingDecreases[
            stakingProvider
        ];

        uint96 _eligibleStake = eligibleStake(
            self,
            tokenStaking,
            stakingProvider,
            decrease.decreasingBy
        );

        if (!sortitionPool.isOperatorInPool(operator)) {
            return _eligibleStake == 0;
        } else {
            return sortitionPool.isOperatorUpToDate(operator, _eligibleStake);
        }
    }

    /// @notice Returns the current value of the staking provider's eligible
    ///         stake. Eligible stake is defined as the currently authorized
    ///         stake minus the pending authorization decrease. Eligible stake
    ///         is what is used for operator's weight in the pool. If the
    ///         authorized stake minus the pending authorization decrease is
    ///         below the minimum authorization, eligible stake is 0.
    /// @dev This function can be exposed to the public in contrast to the
    ///      second variant accepting `decreasingBy` as a parameter.
    function eligibleStake(
        Data storage self,
        IStaking tokenStaking,
        address stakingProvider
    ) external view returns (uint96) {
        return
            eligibleStake(
                self,
                tokenStaking,
                stakingProvider,
                pendingAuthorizationDecrease(self, stakingProvider)
            );
    }

    /// @notice Returns the current value of the staking provider's eligible
    ///         stake. Eligible stake is defined as the currently authorized
    ///         stake minus the pending authorization decrease. Eligible stake
    ///         is what is used for operator's weight in the pool. If the
    ///         authorized stake minus the pending authorization decrease is
    ///         below the minimum authorization, eligible stake is 0.
    /// @dev This function is not intended to be exposes to the public.
    ///      `decreasingBy` must be fetched from `pendingDecreases` mapping and
    ///      it is passed as a parameter to optimize gas usage of functions that
    ///      call `eligibleStake` and need to use `AuthorizationDecrease`
    ///      fetched from `pendingDecreases` for some additional logic.
    function eligibleStake(
        Data storage self,
        IStaking tokenStaking,
        address stakingProvider,
        uint96 decreasingBy
    ) public view returns (uint96) {
        uint96 authorizedStake = tokenStaking.authorizedStake(
            stakingProvider,
            address(this)
        );

        uint96 _eligibleStake = authorizedStake > decreasingBy
            ? authorizedStake - decreasingBy
            : 0;

        if (_eligibleStake < self.parameters.minimumAuthorization) {
            return 0;
        } else {
            return _eligibleStake;
        }
    }

    /// @notice Returns the amount of stake that is pending authorization
    ///         decrease for the given staking provider. If no authorization
    ///         decrease has been requested, returns zero.
    function pendingAuthorizationDecrease(
        Data storage self,
        address stakingProvider
    ) public view returns (uint96) {
        AuthorizationDecrease storage decrease = self.pendingDecreases[
            stakingProvider
        ];

        return decrease.decreasingBy;
    }

    /// @notice Returns the remaining time in seconds that needs to pass before
    ///         the requested authorization decrease can be approved.
    ///         If the sortition pool state was not updated yet by the operator
    ///         after requesting the authorization decrease, returns
    ///         `type(uint64).max`.
    function remainingAuthorizationDecreaseDelay(
        Data storage self,
        address stakingProvider
    ) external view returns (uint64) {
        AuthorizationDecrease storage decrease = self.pendingDecreases[
            stakingProvider
        ];

        if (decrease.decreasingAt == type(uint64).max) {
            return type(uint64).max;
        }

        // solhint-disable-next-line not-rely-on-time
        uint64 _now = uint64(block.timestamp);
        return _now > decrease.decreasingAt ? 0 : decrease.decreasingAt - _now;
    }
}
