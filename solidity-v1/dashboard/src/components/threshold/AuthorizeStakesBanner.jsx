import Banner from "../Banner"
import * as Icons from "../Icons"
import React from "react"
import Chip from "../Chip"
import { LINK } from "../../constants/constants"
import OnlyIf from "../OnlyIf"
import { KEEP, ThresholdToken } from "../../utils/token.utils"
import BigNumber from "bignumber.js"
import { toThresholdTokenAmount } from "../../utils/stake-to-t.utils"

const AuthorizeStakesBanner = ({ numberOfStakesToAuthorize = 0 }) => {
  return (
    <Banner className="banner authorize-stakes-banner">
      <div className="banner__content-wrapper">
        <Banner.Icon icon={Icons.EarnThresholdTokens} />
        <div className="authorize-stakes-banner__content">
          <Banner.Title className="h3 text-white banner__title--font-weight-600">
            <p className="mb-1">
              Authorize your{" "}
              <OnlyIf condition={numberOfStakesToAuthorize > 0}>
                {numberOfStakesToAuthorize}
              </OnlyIf>{" "}
              stake
              <OnlyIf condition={numberOfStakesToAuthorize !== 1}>
                {"s"}
              </OnlyIf>{" "}
              below to get started staking on Threshold.
            </p>
          </Banner.Title>
          <Banner.Description>
            <div className={"flex row space-between"}>
              <div className={"authorize-stakes-banner__base-info"}>
                <ul>
                  <li className={"mb-1"}>
                    <Icons.Rewards
                      className={"authorize-stakes-banner__base-info-icon"}
                    />{" "}
                    Earn rewards on your KEEP stake with Threshold.
                  </li>
                  <li>
                    <Icons.Money
                      className={"authorize-stakes-banner__base-info-icon"}
                    />{" "}
                    Exchange rate is 1 KEEP ={" "}
                    {ThresholdToken.displayAmountWithSymbol(
                      toThresholdTokenAmount(KEEP.fromTokenUnit(1)),
                      3,
                      (amount) =>
                        new BigNumber(amount).toFormat(3, BigNumber.ROUND_DOWN)
                    )}
                    .
                  </li>
                </ul>
              </div>
              <div
                className={"flex row authorize-stakes-banner__pre-node-info"}
              >
                <Chip
                  size={"small"}
                  text={"NOTE"}
                  className={"authorize-stakes-banner__pre-node-info-chip"}
                />
                <div>
                  You will need to run a PRE node to qualify for rewards.{" "}
                  <a
                    href={LINK.setUpPRE}
                    rel="noopener noreferrer"
                    target="_blank"
                    className={"authorize-stakes-banner__pre-node-info-link"}
                  >
                    Set up PRE
                  </a>
                </div>
              </div>
            </div>
          </Banner.Description>
        </div>
      </div>
    </Banner>
  )
}

export default AuthorizeStakesBanner
