package ethereum

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/contract"
	"github.com/keep-network/keep-core/pkg/operator"
)

// Definitions of contract names.
const (
	WalletRegistryContractName = "WalletRegistry"
)

// TbtcChain represents a TBTC-specific chain handle.
type TbtcChain struct {
	*Chain

	walletRegistry *contract.WalletRegistry
}

// NewTbtcChain construct a new instance of the TBTC-specific Ethereum
// chain handle.
func newTbtcChain(
	config ethereum.Config,
	baseChain *Chain,
) (*TbtcChain, error) {
	walletRegistryAddress, err := config.ContractAddress(WalletRegistryContractName)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%v]",
			WalletRegistryContractName,
			err,
		)
	}

	walletRegistry, err :=
		contract.NewWalletRegistry(
			walletRegistryAddress,
			baseChain.chainID,
			baseChain.key,
			baseChain.client,
			baseChain.nonceManager,
			baseChain.miningWaiter,
			baseChain.blockCounter,
			baseChain.transactionMutex,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to WalletRegistry contract: [%v]",
			err,
		)
	}

	return &TbtcChain{
		Chain:          baseChain,
		walletRegistry: walletRegistry,
	}, nil
}

// IsRecognized checks whether the given operator is recognized by the TbtcChain
// as eligible to join the network. If the operator has a stake delegation or
// had a stake delegation in the past, it will be recognized.
func (tc *TbtcChain) IsRecognized(operatorPublicKey *operator.PublicKey) (bool, error) {
	operatorAddress, err := operatorPublicKeyToChainAddress(operatorPublicKey)
	if err != nil {
		return false, fmt.Errorf(
			"cannot convert from operator key to chain address: [%v]",
			err,
		)
	}

	stakingProvider, err := tc.walletRegistry.OperatorToStakingProvider(
		operatorAddress,
	)
	if err != nil {
		return false, fmt.Errorf(
			"failed to map operator [%v] to a staking provider: [%v]",
			operatorAddress,
			err,
		)
	}

	if (stakingProvider == common.Address{}) {
		return false, nil
	}

	// Check if the staking provider has an owner. This check ensures that there
	// is/was a stake delegation for the given staking provider.
	_, _, _, hasStakeDelegation, err := tc.Chain.RolesOf(
		chain.Address(stakingProvider.Hex()),
	)
	if err != nil {
		return false, fmt.Errorf(
			"failed to check stake delegation for staking provider [%v]: [%v]",
			stakingProvider,
			err,
		)
	}

	if !hasStakeDelegation {
		return false, nil
	}

	return true, nil
}
