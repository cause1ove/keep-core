import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer } = await getNamedAccounts()
  const { to1e18 } = helpers.number

  const POOL_WEIGHT_DIVISOR = to1e18(1) // TODO: Update value

  const TokenStaking = await deployments.get("TokenStaking")
  const T = await deployments.get("T")

  const SortitionPool = await deployments.deploy("SortitionPool", {
    from: deployer,
    args: [TokenStaking.address, T.address, POOL_WEIGHT_DIVISOR],
    log: true,
  })

  if (hre.network.tags.tenderly) {
    await hre.tenderly.verify({
      name: "SortitionPool",
      address: SortitionPool.address,
    })
  }
}

export default func

func.tags = ["SortitionPool"]
// TokenStaking and T deployments are expected to be resolved from
// @threshold-network/solidity-contracts
func.dependencies = ["TokenStaking", "T"]
