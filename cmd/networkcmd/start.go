// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cryft-labs/cryft-cli/pkg/binutils"
	"github.com/cryft-labs/cryft-cli/pkg/constants"
	"github.com/cryft-labs/cryft-cli/pkg/models"
	"github.com/cryft-labs/cryft-cli/pkg/subnet"
	"github.com/cryft-labs/cryft-cli/pkg/utils"
	"github.com/cryft-labs/cryft-cli/pkg/ux"
	"github.com/cryft-labs/cryft-cli/pkg/vm"
	"github.com/MetalBlockchain/metal-network-runner/client"
	"github.com/MetalBlockchain/metal-network-runner/server"
	anrutils "github.com/MetalBlockchain/metal-network-runner/utils"
	"github.com/spf13/cobra"
)

var (
	userProvidedAvagoVersion string
	snapshotName             string
	avagoBinaryPath          string
)

const (
	latest  = "latest"
	jsonExt = ".json"
)

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts a local network",
		Long: `The network start command starts a local, multi-node Avalanche network on your machine.

By default, the command loads the default snapshot. If you provide the --snapshot-name
flag, the network loads that snapshot instead. The command fails if the local network is
already running.`,

		RunE:         StartNetwork,
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&userProvidedAvagoVersion, "metalgo-version", latest, "use this version of metalgo (ex: v1.17.12)")
	cmd.Flags().StringVar(&avagoBinaryPath, "metalgo-path", "", "use this avalanchego binary path")
	cmd.Flags().StringVar(&snapshotName, "snapshot-name", constants.DefaultSnapshotName, "name of snapshot to use to start the network from")

	return cmd
}

func StartNetwork(*cobra.Command, []string) error {
	var (
		err          error
		avagoVersion string
	)
	if avagoBinaryPath == "" {
		avagoVersion, err = determineAvagoVersion(userProvidedAvagoVersion)
		if err != nil {
			return err
		}
	}
	sd := subnet.NewLocalDeployer(app, avagoVersion, avagoBinaryPath, "")

	if err := sd.StartServer(); err != nil {
		return err
	}

	needsRestart, avalancheGoBinPath, err := sd.SetupLocalEnv()
	if err != nil {
		return err
	}

	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}

	ctx, cancel := utils.GetANRContext()
	defer cancel()

	bootstrapped, err := checkNetworkIsAlreadyBootstrapped(ctx, cli)
	if err != nil {
		return err
	}

	if bootstrapped {
		if !needsRestart {
			ux.Logger.PrintToUser("Network has already been booted.")
			return nil
		}
		if _, err := cli.Stop(ctx); err != nil {
			return err
		}
		if err := app.ResetPluginsDir(); err != nil {
			return err
		}
	}

	var startMsg string
	if snapshotName == constants.DefaultSnapshotName {
		startMsg = "Starting previously deployed and stopped snapshot"
	} else {
		startMsg = fmt.Sprintf("Starting previously deployed and stopped snapshot %s...", snapshotName)
	}
	ux.Logger.PrintToUser(startMsg)

	outputDirPrefix := filepath.Join(app.GetRunDir(), "network")
	outputDir, err := anrutils.MkDirWithTimestamp(outputDirPrefix)
	if err != nil {
		return err
	}

	pluginDir := app.GetPluginsDir()

	loadSnapshotOpts := []client.OpOption{
		client.WithExecPath(avalancheGoBinPath),
		client.WithRootDataDir(outputDir),
		client.WithReassignPortsIfUsed(true),
		client.WithPluginDir(pluginDir),
	}

	// load global node configs if they exist
	configStr, err := app.Conf.LoadNodeConfig()
	if err != nil {
		return err
	}
	if configStr != "" {
		loadSnapshotOpts = append(loadSnapshotOpts, client.WithGlobalNodeConfig(configStr))
	}

	ux.Logger.PrintToUser("Booting Network. Wait until healthy...")
	resp, err := cli.LoadSnapshot(
		ctx,
		snapshotName,
		loadSnapshotOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to start network with the persisted snapshot: %w", err)
	}

	ux.Logger.PrintToUser("Node logs directory: %s/node<i>/logs", resp.ClusterInfo.RootDataDir)
	ux.Logger.PrintToUser("Network ready to use.")

	if subnet.HasEndpoints(resp.ClusterInfo) {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Local network node endpoints:")
		if err := ux.PrintEndpointTables(resp.ClusterInfo); err != nil {
			return err
		}
	}

	relayerStoredConfigPath := filepath.Join(app.GetAWMRelayerSnapshotConfsDir(), snapshotName+jsonExt)
	if utils.FileExists(relayerStoredConfigPath) {
		relayerConfigPath := app.GetAWMRelayerConfigPath()
		if err := binutils.CopyFile(relayerStoredConfigPath, relayerConfigPath); err != nil {
			return err
		}
		ux.Logger.PrintToUser("")
	}

	storedExtraLocalNetowkrDataPath := filepath.Join(app.GetExtraLocalNetworkSnapshotsDir(), snapshotName+jsonExt)
	if utils.FileExists(storedExtraLocalNetowkrDataPath) {
		extraLocalNetworkDataPath := app.GetExtraLocalNetworkDataPath()
		if err := binutils.CopyFile(storedExtraLocalNetowkrDataPath, extraLocalNetworkDataPath); err != nil {
			return err
		}
	}

	return nil
}

func determineAvagoVersion(userProvidedAvagoVersion string) (string, error) {
	// a specific user provided version should override this calculation, so just return
	if userProvidedAvagoVersion != latest {
		return userProvidedAvagoVersion, nil
	}

	// Need to determine which subnets have been deployed
	locallyDeployedSubnets, err := subnet.GetLocallyDeployedSubnetsFromFile(app)
	if err != nil {
		return "", err
	}

	// if no subnets have been deployed, use latest
	if len(locallyDeployedSubnets) == 0 {
		return latest, nil
	}

	currentRPCVersion := -1

	// For each deployed subnet, check RPC versions
	for _, deployedSubnet := range locallyDeployedSubnets {
		sc, err := app.LoadSidecar(deployedSubnet)
		if err != nil {
			return "", err
		}

		// if you have a custom vm, you must provide the version explicitly
		// if you upgrade from subnet-evm to a custom vm, the RPC version will be 0
		if sc.VM == models.CustomVM || sc.Networks[models.Local.String()].RPCVersion == 0 {
			continue
		}

		if currentRPCVersion == -1 {
			currentRPCVersion = sc.Networks[models.Local.String()].RPCVersion
		}

		if sc.Networks[models.Local.String()].RPCVersion != currentRPCVersion {
			return "", fmt.Errorf(
				"RPC version mismatch. Expected %d, got %d for Subnet %s. Upgrade all subnets to the same RPC version to launch the network",
				currentRPCVersion,
				sc.RPCVersion,
				sc.Name,
			)
		}
	}

	// If currentRPCVersion == -1, then only custom subnets have been deployed, the user must provide the version explicitly if not latest
	if currentRPCVersion == -1 {
		ux.Logger.PrintToUser("No Subnet RPC version found. Using latest AvalancheGo version")
		return latest, nil
	}

	return vm.GetLatestAvalancheGoByProtocolVersion(
		app,
		currentRPCVersion,
		constants.AvalancheGoCompatibilityURL,
	)
}

func checkNetworkIsAlreadyBootstrapped(ctx context.Context, cli client.Client) (bool, error) {
	_, err := cli.Status(ctx)
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			return false, nil
		}
		return false, fmt.Errorf("failed trying to get network status: %w", err)
	}
	return true, nil
}
