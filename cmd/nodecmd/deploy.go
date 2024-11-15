// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"

	"github.com/cryft-labs/cryft-cli/cmd/subnetcmd"
	"github.com/cryft-labs/cryft-cli/pkg/ansible"
	"github.com/cryft-labs/cryft-cli/pkg/models"
	"github.com/cryft-labs/cryft-cli/pkg/networkoptions"
	"github.com/cryft-labs/cryft-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var (
	subnetOnly  bool
	avoidChecks bool
)

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Deploy a subnet into a devnet cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node devnet deploy command deploys a subnet into a devnet cluster, creating subnet and blockchain txs for it.
It saves the deploy info both locally and remotely.
`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         deploySubnet,
	}
	cmd.Flags().BoolVar(&subnetOnly, "subnet-only", false, "only create a subnet")
	cmd.Flags().BoolVar(&avoidChecks, "no-checks", false, "do not check for healthy status or rpc compatibility of nodes against subnet")
	return cmd
}

func deploySubnet(cmd *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := args[1]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	if clustersConfig.Clusters[clusterName].Network.Kind != models.Devnet {
		return fmt.Errorf("node deploy command must be applied to devnet clusters")
	}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	defer disconnectHosts(hosts)
	if !avoidChecks {
		if err := checkHostsAreHealthy(hosts); err != nil {
			return err
		}
		if err := checkHostsAreRPCCompatible(hosts, subnetName); err != nil {
			return err
		}
	}
	networkFlags := networkoptions.NetworkFlags{
		ClusterName: clusterName,
	}
	keyNameParam := ""
	useLedgerParam := false
	useEwoqParam := true
	sameControlKey := true

	if err := subnetcmd.CallDeploy(
		cmd,
		subnetOnly,
		subnetName,
		networkFlags,
		keyNameParam,
		useLedgerParam,
		useEwoqParam,
		sameControlKey,
	); err != nil {
		return err
	}
	if subnetOnly {
		ux.Logger.PrintToUser("Subnet successfully created!")
	} else {
		ux.Logger.PrintToUser("Blockchain successfully created!")
	}
	return nil
}
