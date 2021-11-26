/* eslint-disable no-await-in-loop */

import { ethers } from "hardhat"
import type { BigNumber, ContractTransaction } from "ethers"
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { RandomBeacon, SortitionPool } from "../../typechain"
import { Operator } from "./operators"
// eslint-disable-next-line import/no-cycle
import { selectGroup } from "./groups"

export interface DkgResult {
  submitterMemberIndex: number
  groupPubKey: string
  misbehavedMembersIndices: number[]
  signatures: string
  signingMembersIndices: number[]
  members: number[]
}

export const noMisbehaved = []

export async function genesis(
  randomBeacon: RandomBeacon
): Promise<[ContractTransaction, BigNumber]> {
  const tx = await randomBeacon.genesis()

  const expectedSeed = ethers.BigNumber.from(
    ethers.utils.keccak256(
      ethers.utils.solidityPack(
        ["uint256", "uint256"],
        [await randomBeacon.genesisSeed(), tx.blockNumber]
      )
    )
  )

  return [tx, expectedSeed]
}

// Sign and submit a correct DKG result which cannot be challenged because used
// signers belong to an actual group selected by the sortition pool for given
// seed.
export async function signAndSubmitCorrectDkgResult(
  randomBeacon: RandomBeacon,
  groupPublicKey: string,
  seed: BigNumber,
  startBlock: number,
  misbehavedIndices: number[],
  submitterIndex = 1,
  numberOfSignatures = 33
): Promise<{
  transaction: ContractTransaction
  dkgResult: DkgResult
  dkgResultHash: string
  members: number[]
}> {
  return signAndSubmitArbitraryDkgResult(
    randomBeacon,
    groupPublicKey,
    await selectGroup(randomBeacon, seed),
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
  randomBeacon: RandomBeacon,
  groupPublicKey: string,
  signers: Operator[],
  startBlock: number,
  misbehavedIndices: number[],
  submitterIndex = 1,
  numberOfSignatures = 33
): Promise<{
  transaction: ContractTransaction
  dkgResult: DkgResult
  dkgResultHash: string
  members: number[]
}> {
  const { members, signingMembersIndices, signaturesBytes } =
    await signDkgResult(
      signers,
      groupPublicKey,
      misbehavedIndices,
      startBlock,
      numberOfSignatures
    )

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

  const transaction = await randomBeacon
    .connect(await ethers.getSigner(signers[submitterIndex - 1].address))
    .submitDkgResult(dkgResult)

  return { transaction, dkgResult, dkgResultHash, members }
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

export async function getDkgResultSubmitterSigner(
  randomBeacon: RandomBeacon,
  dkgResult: DkgResult
): Promise<SignerWithAddress> {
  const sortitionPool = (await ethers.getContractAt(
    "SortitionPool",
    await randomBeacon.sortitionPool()
  )) as SortitionPool

  const submitterMember = await sortitionPool.getIDOperator(
    dkgResult.members[dkgResult.submitterMemberIndex - 1]
  )

  return ethers.getSigner(submitterMember)
}
