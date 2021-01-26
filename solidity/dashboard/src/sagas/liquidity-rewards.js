import {
  takeLatest,
  takeEvery,
  fork,
  call,
  put,
  select,
} from "redux-saga/effects"
import { submitButtonHelper, logError, getLPRewardsWrapper } from "./utils"
import { sendTransaction } from "./web3"
import { LiquidityRewardsFactory } from "../services/liquidity-rewards"
import { gt, percentageOf, eq } from "../utils/arithmetics.utils"
import { LIQUIDITY_REWARD_PAIRS } from "../constants/constants"
import { getWsUrl } from "../connectors/utils"
import { initializeWeb3, createLPRewardsContract } from "../contracts"
/** @typedef { import("../services/liquidity-rewards").LiquidityRewards} LiquidityRewards */
import BigNumber from "bignumber.js"
import { showMessage } from "../actions/messages"
import { messageType } from "../components/Message"
import React from "react"
import ViewYourLiquidityBalance from "../components/ViewYourLiquidityBalance"

function* fetchAllLiquidtyRewardsData(action) {
  const { address } = action.payload

  for (const [pairName, value] of Object.entries(LIQUIDITY_REWARD_PAIRS)) {
    yield fork(fetchLiquidityRewardsData, { name: pairName, ...value }, address)
  }
}

function* fetchLiquidityRewardsData(liquidityRewardPair, address) {
  /** @type LiquidityRewards */
  const LiquidityRewards = yield getLPRewardsWrapper(liquidityRewardPair)

  try {
    yield put({
      type: `liquidity_rewards/${liquidityRewardPair.name}_fetch_data_start`,
      payload: { liquidityRewardPairName: liquidityRewardPair.name },
    })

    // Fetching balance of liquidity token for a given uniswap pair deposited in
    // the `LPRewards` contract.
    const lpBalance = yield call(
      [LiquidityRewards, LiquidityRewards.stakedBalance],
      address
    )
    // Fetching balance of liquidity token for a given uniswap pair.
    const wrappedTokenBalance = yield call(
      [LiquidityRewards, LiquidityRewards.wrappedTokenBalance],
      address
    )
    let apy = Infinity
    // Fetching total deposited liqidity tokens in the `LPRewards` contract.
    const totalSupply = yield call([
      LiquidityRewards,
      LiquidityRewards.totalSupply,
    ])
    if (gt(totalSupply, 0)) {
      apy = yield call(
        [LiquidityRewards, LiquidityRewards.calculateAPY],
        totalSupply
      )
    }

    let reward = 0
    let shareOfPoolInPercent = 0
    if (gt(lpBalance, 0)) {
      // Fetching available reward balance from `LPRewards` contract.
      reward = yield call(
        [LiquidityRewards, LiquidityRewards.rewardBalance],
        address
      )
      // % of total pool in the `LPRewards` contract.
      shareOfPoolInPercent = percentageOf(lpBalance, totalSupply).toString()
    }

    yield put({
      type: `liquidity_rewards/${liquidityRewardPair.name}_fetch_data_success`,
      payload: {
        liquidityRewardPairName: liquidityRewardPair.name,
        apy,
        lpBalance,
        wrappedTokenBalance,
        reward,
        shareOfPoolInPercent,
      },
    })
  } catch (error) {
    yield* logError(
      `liquidity_rewards/${liquidityRewardPair.name}_fetch_data_failure`,
      error,
      { liquidityRewardPairName: liquidityRewardPair.name }
    )
  }
}

export function* watchFetchLiquidityRewardsData() {
  yield takeLatest(
    "liquidity_rewards/fetch_data_request",
    fetchAllLiquidtyRewardsData
  )
}

export function* watchLiquidityRewardsEarnedNotification() {
  for (const [pairName, value] of Object.entries(LIQUIDITY_REWARD_PAIRS)) {
    yield takeLatest(
      `liquidity_rewards/${pairName}_liquidity_rewards_earned_notification`,
      processLiquidityRewardEarnedNotification
    )
  }
}

function* processLiquidityRewardEarnedNotification(action) {
  const displayMessage = action.payload.displayMessage

  const liquidityRewardPairName = action.payload.liquidityRewardPairName
  const liquidityRewardPair = LIQUIDITY_REWARD_PAIRS[liquidityRewardPairName]

  /** @type LiquidityRewards */
  const LiquidityRewards = yield getLPRewardsWrapper(liquidityRewardPair)
  const { liquidityRewards } = yield select()
  const lastNotificationRewardAmount = new BigNumber(
    liquidityRewards[
      liquidityRewardPairName
    ].liquidityRewardsEarnedNotification.lastNotificationRewardAmount
  )
  const currentReward = yield call(
    [LiquidityRewards, LiquidityRewards.rewardBalance],
    action.payload.address,
  )
  // show the notification if the rewardBalance from LPRewardsContract is greater
  // than the reward amount that was last time the notification was displayed
  if (gt(currentReward, lastNotificationRewardAmount)) {
    if (displayMessage) {
      yield put(
        showMessage({
          type: messageType.REWARD,
          title: `[${liquidityRewardPairName}] You've earned rewards!`,
          sticky: true,
          content: <ViewYourLiquidityBalance />,
          classes: {
            bannerDescription: "m-0",
          },
        })
      )
    }
  }

  // save last notification reward amount for future comparisons
  yield put({
    type: `liquidity_rewards/${liquidityRewardPairName}_update_last_reward_amount`,
    payload: {
      liquidityRewardPairName,
      lastNotificationRewardAmount: currentReward,
    },
  })
}

function* stakeTokens(action) {
  const { contractName, address, amount, pool } = action.payload

  /** @type LiquidityRewards */
  const LiquidityRewards = yield getLPRewardsWrapper({ contractName, pool })

  const approvedAmount = yield call(
    [LiquidityRewards, LiquidityRewards.wrappedTokenAllowance],
    address,
    LiquidityRewards.LPRewardsContractAddress
  )

  if (!eq(amount, approvedAmount)) {
    yield call(sendTransaction, {
      payload: {
        contract: LiquidityRewards.wrappedToken,
        methodName: "approve",
        args: [LiquidityRewards.LPRewardsContractAddress, amount],
      },
    })
  }

  yield call(sendTransaction, {
    payload: {
      contract: LiquidityRewards.LPRewardsContract,
      methodName: "stake",
      args: [amount],
    },
  })
}

function* stakeTokensWorker(action) {
  yield call(submitButtonHelper, stakeTokens, action)
}

export function* watchStakeTokens() {
  yield takeEvery("liquidity_rewards/stake_tokens", stakeTokensWorker)
}

function* fetchAllLiquidityRewardsAPY(action) {
  for (const [pairName, value] of Object.entries(LIQUIDITY_REWARD_PAIRS)) {
    yield fork(fetchLiquidityRewardsAPY, { name: pairName, ...value })
  }
}

function* fetchLiquidityRewardsAPY(liquidityRewardPair) {
  try {
    yield put({
      type: `liquidity_rewards/${liquidityRewardPair.name}_fetch_apy_start`,
      payload: { liquidityRewardPairName: liquidityRewardPair.name },
    })
    const web3 = initializeWeb3(getWsUrl())
    const LPRewardsContract = yield call(
      createLPRewardsContract,
      web3,
      liquidityRewardPair.contractName
    )

    /** @type LiquidityRewards */
    const LiquidityRewards = yield call(
      [LiquidityRewardsFactory, LiquidityRewardsFactory.initialize],
      liquidityRewardPair.pool,
      LPRewardsContract,
      web3
    )

    let apy = Infinity
    const totalSupply = yield call([
      LiquidityRewards,
      LiquidityRewards.totalSupply,
    ])
    if (gt(totalSupply, 0)) {
      apy = yield call(
        [LiquidityRewards, LiquidityRewards.calculateAPY],
        totalSupply
      )
    }

    yield put({
      type: `liquidity_rewards/${liquidityRewardPair.name}_fetch_apy_success`,
      payload: { liquidityRewardPairName: liquidityRewardPair.name, apy },
    })
  } catch (error) {
    yield* logError(
      `liquidity_rewards/${liquidityRewardPair.name}_fetch_apy_failure`,
      error,
      { liquidityRewardPairName: liquidityRewardPair.name }
    )
  }
}

export function* watchFetchLiquidityRewardsAPY() {
  yield takeLatest(
    "liquidity_rewards/fetch_apy_request",
    fetchAllLiquidityRewardsAPY
  )
}
