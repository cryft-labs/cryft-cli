// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/cryft-labs/cryft-cli/pkg/constants"
	"github.com/cryft-labs/cryft-cli/pkg/keychain"
	"github.com/cryft-labs/cryft-cli/pkg/models"
	"github.com/cryft-labs/cryft-cli/pkg/networkoptions"
	"github.com/cryft-labs/cryft-cli/pkg/prompts"
	"github.com/cryft-labs/cryft-cli/pkg/subnet"
	"github.com/cryft-labs/cryft-cli/pkg/txutils"
	"github.com/cryft-labs/cryft-cli/pkg/ux"
	"github.com/MetalBlockchain/metalgo/genesis"
	"github.com/MetalBlockchain/metalgo/ids"
	"github.com/MetalBlockchain/metalgo/vms/secp256k1fx"
	"github.com/spf13/cobra"
)

var removeValidatorSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Tahoe, networkoptions.Mainnet}

// avalanche subnet removeValidator
func newRemoveValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "removeValidator [subnetName]",
		Short: "Remove a permissioned validator from your subnet",
		Long: `The subnet removeValidator command stops a whitelisted, subnet network validator from
validating your deployed Subnet.

To remove the validator from the Subnet's allow list, provide the validator's unique NodeID. You can bypass
these prompts by providing the values with flags.`,
		SilenceUsage: true,
		RunE:         removeValidator,
		Args:         cobra.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, removeValidatorSupportedNetworkOptions)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji deploy only]")
	cmd.Flags().StringVar(&nodeIDStr, "nodeID", "", "set the NodeID of the validator to remove")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "control keys that will be used to authenticate the removeValidator tx")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "file path of the removeValidator tx")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	return cmd
}

func removeValidator(_ *cobra.Command, args []string) error {
	var (
		nodeID ids.NodeID
		err    error
	)

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		globalNetworkFlags,
		false,
		removeValidatorSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	if outputTxPath != "" {
		if _, err := os.Stat(outputTxPath); err == nil {
			return fmt.Errorf("outputTxPath %q already exists", outputTxPath)
		}
	}

	if len(ledgerAddresses) > 0 {
		useLedger = true
	}

	if useLedger && keyName != "" {
		return ErrMutuallyExlusiveKeyLedger
	}

	chains, err := ValidateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}
	subnetName := chains[0]

	switch network.Kind {
	case models.Local:
		return removeFromLocal(subnetName)
	case models.Tahoe:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetFujiKeyOrLedger(app.Prompt, constants.PayTxsFeesMsg, app.GetKeyDir())
			if err != nil {
				return err
			}
		}
	case models.Mainnet:
		useLedger = true
		if keyName != "" {
			return ErrStoredKeyOnMainnet
		}
	default:
		return errors.New("unsupported network")
	}

	// get keychain accesor
	fee := network.GenesisParams().TxFee
	kc, err := keychain.GetKeychain(app, false, useLedger, ledgerAddresses, keyName, network, fee)
	if err != nil {
		return err
	}

	network.HandlePublicNetworkSimulation()

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}
	transferSubnetOwnershipTxID := sc.Networks[network.Name()].TransferSubnetOwnershipTxID

	controlKeys, threshold, err := txutils.GetOwners(network, subnetID, transferSubnetOwnershipTxID)
	if err != nil {
		return err
	}

	// add control keys to the keychain whenever possible
	if err := kc.AddAddresses(controlKeys); err != nil {
		return err
	}

	kcKeys, err := kc.PChainFormattedStrAddresses()
	if err != nil {
		return err
	}

	// get keys for add validator tx signing
	if subnetAuthKeys != nil {
		if err := prompts.CheckSubnetAuthKeys(kcKeys, subnetAuthKeys, controlKeys, threshold); err != nil {
			return err
		}
	} else {
		subnetAuthKeys, err = prompts.GetSubnetAuthKeys(app.Prompt, kcKeys, controlKeys, threshold)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser("Your subnet auth keys for remove validator tx creation: %s", subnetAuthKeys)

	if nodeIDStr == "" {
		nodeID, err = PromptNodeID()
		if err != nil {
			return err
		}
	} else {
		nodeID, err = ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return err
		}
	}

	// check that this guy actually is a validator on the subnet
	isValidator, err := subnet.IsSubnetValidator(subnetID, nodeID, network)
	if err != nil {
		// just warn the user, don't fail
		ux.Logger.PrintToUser("failed to check if node is a validator on the subnet: %s", err)
	} else if !isValidator {
		// this is actually an error
		return fmt.Errorf("node %s is not a validator on subnet %s", nodeID, subnetID)
	}

	ux.Logger.PrintToUser("NodeID: %s", nodeID.String())
	ux.Logger.PrintToUser("Network: %s", network.Name())
	ux.Logger.PrintToUser("Inputs complete, issuing transaction to remove the specified validator...")

	deployer := subnet.NewPublicDeployer(app, kc, network)
	isFullySigned, tx, remainingSubnetAuthKeys, err := deployer.RemoveValidator(
		controlKeys,
		subnetAuthKeys,
		subnetID,
		transferSubnetOwnershipTxID,
		nodeID,
	)
	if err != nil {
		return err
	}
	if !isFullySigned {
		if err := SaveNotFullySignedTx(
			"Remove Validator",
			tx,
			subnetName,
			subnetAuthKeys,
			remainingSubnetAuthKeys,
			outputTxPath,
			false,
		); err != nil {
			return err
		}
	}

	return err
}

func removeFromLocal(subnetName string) error {
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	subnetID := sc.Networks[models.Local.String()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	// Get NodeIDs of all validators on the subnet
	validators, err := subnet.GetSubnetValidators(subnetID)
	if err != nil {
		return err
	}

	// construct list of validators to choose from
	validatorList := make([]string, len(validators))
	for i, v := range validators {
		validatorList[i] = v.NodeID.String()
	}

	if nodeIDStr == "" {
		nodeIDStr, err = app.Prompt.CaptureList("Choose a validator to remove", validatorList)
		if err != nil {
			return err
		}
	}

	// Convert NodeID string to NodeID type
	nodeID, err := ids.NodeIDFromString(nodeIDStr)
	if err != nil {
		return err
	}

	testKey := genesis.EWOQKey
	keyChain := secp256k1fx.NewKeychain(testKey)
	_, err = subnet.IssueRemoveSubnetValidatorTx(keyChain, subnetID, nodeID)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Validator removed")

	return nil
}
