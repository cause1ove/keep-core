import {
  ContractsLoaded,
  getContractDeploymentBlockNumber,
  isCodeValid,
  Web3Loaded,
} from "../contracts"
import { getAllOperatorStakedEventsByAuthorizer } from "./token-staking.service"
import {
  AUTH_CONTRACTS_LABEL,
  TOKEN_GRANT_CONTRACT_NAME,
} from "../constants/constants"
import { Keep } from "../contracts"

const fetchThresholdAuthorizationData = async (address) => {
  if (!address) {
    return []
  }

  const web3 = await Web3Loaded
  const { eth } = web3
  const thresholdTokenStakingContractAddress =
    Keep.thresholdStakingContract.address
  const { stakingContract, grantContract } = await ContractsLoaded
  const keepOperatorStakedEvents = await getAllOperatorStakedEventsByAuthorizer(
    address
  )
  const authorizerOperators = keepOperatorStakedEvents.map(
    (_) => _.returnValues.operator
  )
  const authorizationData = []

  const keepToTStakedEvents =
    await Keep.keepToTStaking.getStakedEventsByOperator(authorizerOperators)

  const operatorsStakedToT = keepToTStakedEvents.reduce((map, _) => {
    map[_.returnValues.stakingProvider] = { ..._.returnValues }
    return map
  }, {})

  const tokenGrantStakingEvents = (
    await grantContract.getPastEvents("TokenGrantStaked", {
      fromBlock: await getContractDeploymentBlockNumber(
        TOKEN_GRANT_CONTRACT_NAME
      ),
      filter: { operator: authorizerOperators },
    })
  ).reduce((map, _) => {
    map[_.returnValues.operator] = { ..._.returnValues }
    return map
  }, {})

  // Fetch all authorizer operators
  for (let i = 0; i < keepOperatorStakedEvents.length; i++) {
    const { operator, beneficiary, authorizer } =
      keepOperatorStakedEvents[i].returnValues

    const { amount: stakeAmount, undelegatedAt } = await stakingContract.methods
      .getDelegationInfo(operator)
      .call()

    // If stake is undelegated we won't display it, because undelegated stakes
    // can't be staked to Threshold
    if (undelegatedAt !== "0") continue

    const isThresholdTokenStakingContractAuthorized =
      await stakingContract.methods
        .isAuthorizedForOperator(operator, thresholdTokenStakingContractAddress)
        .call()

    const owner = await Keep.keepToTStaking.resolveOwner(operator)
    const code = await eth.getCode(owner)

    const authorizerOperator = {
      owner: owner,
      authorizerAddress: authorizer,
      operatorAddress: operator,
      beneficiaryAddress: beneficiary,
      stakeAmount: stakeAmount,
      contract: {
        contractName: AUTH_CONTRACTS_LABEL.THRESHOLD_TOKEN_STAKING,
        operatorContractAddress: thresholdTokenStakingContractAddress,
        isAuthorized: isThresholdTokenStakingContractAuthorized,
      },
      isStakedToT: operatorsStakedToT.hasOwnProperty(operator),
      isFromGrant: tokenGrantStakingEvents.hasOwnProperty(operator),
      // Check if grantee is a contract. If it is then the stake from grant
      // can't be moved to T
      canBeMovedToT: !isCodeValid(code),
    }

    authorizationData.push(authorizerOperator)
  }

  return authorizationData
}

export const thresholdAuthorizationService = {
  fetchThresholdAuthorizationData,
}
