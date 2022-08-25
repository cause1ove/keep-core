package config

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-log"
	ethereumCommon "github.com/keep-network/keep-common/pkg/chain/ethereum"

	"github.com/keep-network/keep-core/pkg/beacon/registry"
	"github.com/keep-network/keep-core/pkg/diagnostics"
	"github.com/keep-network/keep-core/pkg/metrics"
	"github.com/keep-network/keep-core/pkg/net/libp2p"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
)

var logger = log.Logger("keep-config")

const (
	// #nosec G101 (look for hardcoded credentials)
	// This line doesn't contain any credentials.
	// It's just the name of the environment variable.
	EthereumPasswordEnvVariable = "KEEP_ETHEREUM_PASSWORD"

	// LogLevelEnvVariable can be used to define logging configuration.
	LogLevelEnvVariable = "LOG_LEVEL"
)

// Config is the top level config structure.
type Config struct {
	Ethereum    ethereumCommon.Config
	LibP2P      libp2p.Config `mapstructure:"network"`
	Storage     registry.Config
	Metrics     metrics.Config
	Diagnostics diagnostics.Config
	Tbtc        tbtc.Config
}

// Bind the flags to the viper configuration. Viper reads configuration from
// command-line flags, environment variables and config file.
func bindFlags(flagSet *pflag.FlagSet) error {
	if err := viper.BindPFlags(flagSet); err != nil {
		return err
	}
	return nil
}

// Resolve ethereum network based on the set boolean flags.
func (c *Config) resolveEthereumNetwork(flagSet *pflag.FlagSet) error {
	var err error

	c.Ethereum.Network, err = func() (ethereumCommon.Network,error) {
		if ok, err := flagSet.GetBool(ethereumCommon.Goerli.String()); ok {
			return ethereumCommon.Goerli, err
		}
		if ok, err := flagSet.GetBool(ethereumCommon.Developer.String()); ok {
			return ethereumCommon.Developer, err
		}

		return ethereumCommon.Mainnet, nil
	}()

	return err
}

// ReadConfig reads in the configuration file at `configFilePath` and flags defined in
// the `flagSet`.
func (c *Config) ReadConfig(configFilePath string, flagSet *pflag.FlagSet, categories ...Category) error {
	initializeContractAddressesAliases()

	if flagSet != nil {
		if err := bindFlags(flagSet); err != nil {
			return fmt.Errorf("unable to bind the flags: [%w]", err)
		}

		if  err := c.resolveEthereumNetwork(flagSet); err != nil {
			return fmt.Errorf("unable to resolve ethereum network: [%w]", err)
		}
	}

	// Read configuration from a file if the config file path is set.
	if configFilePath != "" {
		if err := readConfigFile(configFilePath); err != nil {
			return fmt.Errorf(
				"unable to load config (file: [%s]): [%w]",
				configFilePath,
				err,
			)
		}
	}

	// Unmarshal config based on loaded config file and command-line flags.
	if err := unmarshalConfig(c); err != nil {
		return fmt.Errorf("unable to unmarshal config: %w", err)
	}

	// Resolve contracts addresses.
	c.resolveContractsAddresses()

	// Validate configuration.
	if err := validateConfig(c, categories...); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Don't use viper.BindEnv for password reading as it's too sensitive value
	// to read it with an external library.
	if c.Ethereum.Account.KeyFilePassword == "" {
		c.Ethereum.Account.KeyFilePassword = os.Getenv(EthereumPasswordEnvVariable)
	}

	if strings.TrimSpace(c.Ethereum.Account.KeyFilePassword) == "" {
		var (
			password string
			err      error
		)
		fmt.Printf(
			"Ethereum Account Password has to be set for the configured Ethereum Key File.\n"+
				"Please set %s environment variable, or set it in the config file, or provide it in the prompt below.\n",
			EthereumPasswordEnvVariable,
		)

		for strings.TrimSpace(password) == "" {
			if password, err = readPassword("Enter Ethereum Account Password: "); err != nil {
				return err
			}
		}

		c.Ethereum.Account.KeyFilePassword = password
	}

	return nil
}

func validateConfig(config *Config, categories ...Category) error {
	var result *multierror.Error

	for _, category := range categories {
		switch category {
		case Ethereum:
			if config.Ethereum.URL == "" {
				result = multierror.Append(result, fmt.Errorf(
					"missing value for ethereum.url; see ethereum section in configuration",
				))
			}

			if config.Ethereum.Account.KeyFile == "" {
				result = multierror.Append(result, fmt.Errorf(
					"missing value for ethereum.keyFile; see ethereum section in configuration",
				))
			}
		case Network:
			if config.LibP2P.Port == 0 {
				result = multierror.Append(result, fmt.Errorf(
					"missing value for network.port; see network section in configuration",
				))
			}
		case Storage:
			if config.Storage.DataDir == "" {
				result = multierror.Append(result, fmt.Errorf(
					"missing value for storage.dataDir; see storage section in configuration",
				))
			}
		}
	}

	return result.ErrorOrNil()
}

// ReadEthereumConfig reads in the configuration from a file specified by `--config`
// flag and other flags provided in the `flagSet` and returns its contained Ethereum
// config, or an error if something fails.
//
// This is the same as invoking ReadConfig and reading the Ethereum property
// from the returned config, but is available for external functions that expect
// to interact solely with Ethereum and are therefore independent of the rest of
// the config structure.
func ReadEthereumConfig(flagSet *pflag.FlagSet) (ethereumCommon.Config, error) {
	config := Config{}

	configPath, err := flagSet.GetString("config")
	if err != nil {
		return ethereumCommon.Config{},
			fmt.Errorf(
				"failed to read config file path from command flag: %w",
				err,
			)
	}

	if err := config.ReadConfig(configPath, flagSet, Ethereum); err != nil {
		return ethereumCommon.Config{}, err
	}

	return config.Ethereum, nil
}

// readConfigFile uses viper to read configuration from a config file. The config file
// is not mandatory, if the path is
func readConfigFile(configFilePath string) error {
	// Read configuration from a file, located in `configFilePath`.
	viper.SetConfigFile(configFilePath)

	// Read configuration.
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf(
			"failed to read configuration from file [%s]: %w",
			configFilePath,
			err,
		)
	}

	return nil
}

// unmarshalConfig unmarshals config with viper from config file and command-line
// flags into a struct.
func unmarshalConfig(config *Config) error {
	if err := viper.Unmarshal(
		config,
		viper.DecodeHook(
			mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
				mapstructure.StringToSliceHookFunc(","),
				mapstructure.TextUnmarshallerHookFunc(),
			),
		),
	); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	return nil
}

// readPassword prompts a user to enter a password. The read password uses
// the system password reading call that helps to prevent key loggers from
// capturing the password.
func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Print("\n")
	if err != nil {
		return "", fmt.Errorf("unable to read password, error [%s]", err)
	}

	return strings.TrimSpace(string(bytePassword)), nil
}
