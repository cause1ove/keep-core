/* eslint-disable @typescript-eslint/no-use-before-define */
/* eslint-disable no-await-in-loop */

import { ethers } from "hardhat"

import type { BigNumber, BigNumberish, ContractTransaction } from "ethers"
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { SortitionPool, WalletFactory } from "../../typechain"
import { Operator } from "./operators"
// eslint-disable-next-line import/no-cycle
import { selectGroup } from "./groups"
import { firstEligibleIndex } from "./submission"
import { constants } from "../fixtures"

export interface DkgResult {
  submitterMemberIndex: number
  groupPubKey: string
  misbehavedMembersIndices: number[]
  signatures: string
  signingMembersIndices: number[]
  members: number[]
}

export const noMisbehaved = []

export function calculateDkgSeed(
  relayEntry: BigNumberish,
  blockNumber: BigNumberish
): BigNumber {
  return ethers.BigNumber.from(
    ethers.utils.keccak256(
      ethers.utils.solidityPack(
        ["uint256", "uint256"],
        [ethers.BigNumber.from(relayEntry), ethers.BigNumber.from(blockNumber)]
      )
    )
  )
}

// Sign and submit a correct DKG result which cannot be challenged because used
// signers belong to an actual group selected by the sortition pool for given
// seed.
export async function signAndSubmitCorrectDkgResult(
  walletFactory: WalletFactory,
  groupPublicKey: string,
  seed: BigNumber,
  startBlock: number,
  misbehavedIndices: number[],
  submitterIndex?: number,
  numberOfSignatures = 50
): Promise<{
  transaction: ContractTransaction
  dkgResult: DkgResult
  dkgResultHash: string
  members: number[]
  submitter: SignerWithAddress
}> {
  if (!submitterIndex) {
    // eslint-disable-next-line no-param-reassign
    submitterIndex = firstEligibleIndex(
      ethers.utils.keccak256(groupPublicKey),
      constants.groupSize
    )
  }

  const sortitionPool = (await ethers.getContractAt(
    "SortitionPool",
    await walletFactory.sortitionPool()
  )) as SortitionPool

  return signAndSubmitArbitraryDkgResult(
    walletFactory,
    groupPublicKey,
    await selectGroup(sortitionPool, seed),
    startBlock,
    misbehavedIndices,
    submitterIndex,
    numberOfSignatures
  )
}

// Sign and submit an arbitrary DKG result using given signers. Signers don't
// need to be part of the actual sortition pool group. This function is useful
// for preparing invalid or malicious results for testing purposes.
export async function signAndSubmitArbitraryDkgResult(
  walletFactory: WalletFactory,
  groupPublicKey: string,
  signers: Operator[],
  startBlock: number,
  misbehavedIndices: number[],
  submitterIndex?: number,
  numberOfSignatures = 33
): Promise<{
  transaction: ContractTransaction
  dkgResult: DkgResult
  dkgResultHash: string
  members: number[]
  submitter: SignerWithAddress
}> {
  const { members, signingMembersIndices, signaturesBytes } =
    await signDkgResult(
      signers,
      groupPublicKey,
      misbehavedIndices,
      startBlock,
      numberOfSignatures
    )

  if (!submitterIndex) {
    // eslint-disable-next-line no-param-reassign
    submitterIndex = firstEligibleIndex(ethers.utils.keccak256(groupPublicKey))
  }

  const dkgResult: DkgResult = {
    submitterMemberIndex: submitterIndex,
    groupPubKey: groupPublicKey,
    misbehavedMembersIndices: misbehavedIndices,
    signatures: signaturesBytes,
    signingMembersIndices,
    members,
  }

  const dkgResultHash = ethers.utils.keccak256(
    ethers.utils.defaultAbiCoder.encode(
      [
        "(uint256 submitterMemberIndex, bytes groupPubKey, uint8[] misbehavedMembersIndices, bytes signatures, uint256[] signingMembersIndices, uint32[] members)",
      ],
      [dkgResult]
    )
  )

  const submitter = await ethers.getSigner(signers[submitterIndex - 1].address)

  const transaction = await walletFactory
    .connect(submitter)
    .submitDkgResult(dkgResult)

  return { transaction, dkgResult, dkgResultHash, members, submitter }
}

export async function signDkgResult(
  signers: Operator[],
  groupPublicKey: string,
  misbehavedMembersIndices: number[],
  startBlock: number,
  numberOfSignatures: number
): Promise<{
  members: number[]
  signingMembersIndices: number[]
  signaturesBytes: string
}> {
  const resultHash = ethers.utils.solidityKeccak256(
    ["bytes", "uint8[]", "uint256"],
    [groupPublicKey, misbehavedMembersIndices, startBlock]
  )

  const members: number[] = []
  const signingMembersIndices: number[] = []
  const signatures: string[] = []
  for (let i = 0; i < signers.length; i++) {
    const { id, address } = signers[i]
    members.push(id)

    if (signatures.length === numberOfSignatures) {
      // eslint-disable-next-line no-continue
      continue
    }

    const signerIndex: number = i + 1

    signingMembersIndices.push(signerIndex)

    const ethersSigner = await ethers.getSigner(address)
    const signature = await ethersSigner.signMessage(
      ethers.utils.arrayify(resultHash)
    )

    signatures.push(signature)
  }

  const signaturesBytes: string = ethers.utils.hexConcat(signatures)

  return { members, signingMembersIndices, signaturesBytes }
}
