/* eslint-disable @typescript-eslint/no-unused-expressions */

import { ethers, waffle } from "hardhat"
import { expect } from "chai"
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import { randomBeaconDeployment, constants } from "./fixtures"
import type { RandomBeaconStub, SortitionPool, StakingStub } from "../typechain"

const fixture = async () => randomBeaconDeployment()

describe("RandomBeacon - Pool", () => {
  let operator: SignerWithAddress
  let randomBeacon: RandomBeaconStub
  let sortitionPool: SortitionPool
  let stakingStub: StakingStub

  // prettier-ignore
  before(async () => {
    [operator] = await ethers.getSigners()
  })

  beforeEach("load test fixture", async () => {
    const contracts = await waffle.loadFixture(fixture)

    randomBeacon = contracts.randomBeacon as RandomBeaconStub
    sortitionPool = contracts.sortitionPool as SortitionPool
    stakingStub = contracts.stakingStub as StakingStub
  })

  describe("registerOperator", () => {
    beforeEach(async () => {
      await stakingStub.setStake(operator.address, constants.minimumStake)
    })

    context("when the operator is not registered yet", () => {
      beforeEach(async () => {
        await randomBeacon.connect(operator).registerOperator()
      })

      it("should register the operator", async () => {
        expect(await sortitionPool.isOperatorInPool(operator.address)).to.be
          .true
      })
    })

    context("when the operator is already registered", () => {
      beforeEach(async () => {
        await randomBeacon.connect(operator).registerOperator()
      })

      it("should revert", async () => {
        await expect(
          randomBeacon.connect(operator).registerOperator()
        ).to.be.revertedWith("Operator is already registered")
      })
    })
  })

  describe("updateOperatorStatus", () => {
    beforeEach(async () => {
      // Operator is registered.
      await stakingStub.setStake(operator.address, constants.minimumStake)
      await randomBeacon.connect(operator).registerOperator()

      // Simulate the operator became ineligible.
      await stakingStub.setStake(operator.address, 0)

      await randomBeacon.connect(operator).updateOperatorStatus()
    })

    context("when status update removes operator from sortition pool", () => {
      it("should remove operator from the pool", async () => {
        expect(await sortitionPool.isOperatorInPool(operator.address)).to.be
          .false
      })
    })
  })

  describe("isOperatorEligible", () => {
    context("when the operator is eligible to join the sortition pool", () => {
      beforeEach(async () => {
        await stakingStub.setStake(operator.address, constants.minimumStake)
      })

      it("should return true", async () => {
        await expect(await randomBeacon.isOperatorEligible(operator.address)).to
          .be.true
      })
    })

    context(
      "when the operator is not eligible to join the sortition pool",
      () => {
        beforeEach(async () => {
          await stakingStub.setStake(
            operator.address,
            constants.minimumStake.sub(1)
          )
        })

        it("should return false", async () => {
          await expect(await randomBeacon.isOperatorEligible(operator.address))
            .to.be.false
        })
      }
    )
  })
})
