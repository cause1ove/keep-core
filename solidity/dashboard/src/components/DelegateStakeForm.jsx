import React, { useState } from "react"
import { SubmitButton } from "./Button"
import FormInput from "./FormInput"
import { withFormik, useFormikContext } from "formik"
import {
  validateAmountInRange,
  validateEthAddress,
  getErrorsObj,
} from "../forms/common-validators"
import { useCustomOnSubmitFormik } from "../hooks/useCustomOnSubmitFormik"
import { displayAmount, fromTokenUnit } from "../utils/token.utils"
import ProgressBar from "./ProgressBar"
import { colors } from "../constants/colors"
import {
  normalizeAmount,
  formatAmount as formatFormAmount,
} from "../forms/form.utils.js"
import { lte } from "../utils/arithmetics.utils"
import * as Icons from "./Icons"
import Tag from "./Tag"

const DelegateStakeForm = ({
  onSubmit,
  minStake,
  availableToStake,
  ...formikProps
}) => {
  const onSubmitBtn = useCustomOnSubmitFormik(onSubmit)
  const stakeTokensValue = fromTokenUnit(formikProps.values.stakeTokens)

  return (
    <form className="delegate-stake-form flex column">
      <TokensAmountField
        availableToStake={availableToStake}
        minStake={minStake}
        stakeTokensValue={stakeTokensValue}
      />
      <div className="address-fields-wrapper">
        <AddressField
          name="authorizerAddress"
          type="text"
          label="Authorizer Address"
          placeholder="0x0"
          icon={<Icons.AuthorizerFormIcon />}
          tooltipText="A role that approves operator contracts and slashing rules for operator misbehavior."
        />
        <AddressField
          name="operatorAddress"
          type="text"
          label="Operator Address"
          placeholder="0x0"
          icon={<Icons.OperatorFormIcon />}
          tooltipText="The operator address is tasked with participation in network operations, and represents the staker in most circumstances."
        />
        <AddressField
          name="beneficiaryAddress"
          type="text"
          label="Beneficiary Address"
          placeholder="0x0"
          icon={<Icons.BeneficiaryFormIcon />}
          tooltipText="The address to which rewards are sent that are generated by stake doing work on the network."
        />
      </div>
      <SubmitButton
        className="btn btn-primary btn-lg"
        type="submit"
        onSubmitAction={onSubmitBtn}
        withMessageActionIsPending={false}
        triggerManuallyFetch={true}
        disabled={!(formikProps.isValid && formikProps.dirty)}
      >
        delegate stake
      </SubmitButton>
    </form>
  )
}

const AddressField = ({ icon, ...formInputProps }) => {
  const [focused, setFocused] = useState(false)
  const { setFieldTouched, touched } = useFormikContext()
  const isTouched = focused || touched[formInputProps.name]

  const onFocus = () => {
    setFocused(true)
    if (
      formInputProps.name === "operatorAddress" &&
      !touched.authorizerAddress
    ) {
      setFieldTouched("authorizerAddress", true, false)
    } else if (
      formInputProps.name === "beneficiaryAddress" &&
      (!touched.authorizerAddress || !touched.operatorAddress)
    ) {
      setFieldTouched("authorizerAddress", true, false)
      setFieldTouched("operatorAddress", true, false)
    }
  }

  return (
    <div className={`address-field-wrapper${isTouched ? " touched" : ""}`}>
      <Icons.DashedLine />
      {icon}
      <FormInput {...formInputProps} onFocus={onFocus} />
    </div>
  )
}

const TokensAmountField = ({
  availableToStake,
  minStake,
  stakeTokensValue,
}) => {
  const { setFieldValue } = useFormikContext()

  const onAddonClick = () => {
    setFieldValue("stakeTokens", availableToStake)
  }
  return (
    <div className="token-amount-wrapper">
      <div className="token-amount-field">
        <FormInput
          name="stakeTokens"
          type="text"
          label="Token Amount"
          normalize={normalizeAmount}
          format={formatFormAmount}
          placeholder="0"
          instructionText={`The minimum stake is ${displayAmount(
            minStake
          )} KEEP`}
          leftIcon={<Icons.KeepOutline className="keep-outline--mint-100" />}
          inputAddon={<MaxStakeAddon onClick={onAddonClick} />}
        />
        <ProgressBar
          total={availableToStake}
          value={stakeTokensValue}
          color={colors.mint80}
          bgColor={colors.mint20}
        >
          <ProgressBar.Inline
            height={10}
            className="token-amount__progress-bar"
          />
        </ProgressBar>
        <div className="text-caption text-grey-60 text-right ml-a">
          {displayAmount(availableToStake)} KEEP available
        </div>
      </div>
    </div>
  )
}

const MaxStakeAddon = ({ onClick }) => {
  return <Tag IconComponent={Icons.Plus} text="Max Stake" onClick={onClick} />
}

const connectedWithFormik = withFormik({
  mapPropsToValues: () => ({
    beneficiaryAddress: "",
    stakeTokens: "",
    operatorAddress: "",
    authorizerAddress: "",
  }),
  validate: (values, props) => {
    const { beneficiaryAddress, operatorAddress, authorizerAddress } = values
    const errors = {}

    errors.stakeTokens = getStakeTokensError(props, values)
    errors.beneficiaryAddress = validateEthAddress(beneficiaryAddress)
    errors.operatorAddress = validateEthAddress(operatorAddress)
    errors.authorizerAddress = validateEthAddress(authorizerAddress)

    return getErrorsObj(errors)
  },
  displayName: "DelegateStakeForm",
})(DelegateStakeForm)

const getStakeTokensError = (props, { stakeTokens }) => {
  const { availableToStake, minStake } = props

  if (lte(availableToStake || 0, 0)) {
    return "Insufficient funds"
  } else {
    return validateAmountInRange(stakeTokens, availableToStake, minStake)
  }
}

export default connectedWithFormik
