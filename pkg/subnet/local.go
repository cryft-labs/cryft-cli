// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/mod/semver"

	"github.com/MetalBlockchain/coreth/params"
	"github.com/cryft-labs/cryft-cli/pkg/application"
	"github.com/cryft-labs/cryft-cli/pkg/binutils"
	"github.com/cryft-labs/cryft-cli/pkg/constants"
	"github.com/cryft-labs/cryft-cli/pkg/key"
	"github.com/cryft-labs/cryft-cli/pkg/localnetworkinterface"
	"github.com/cryft-labs/cryft-cli/pkg/models"
	"github.com/cryft-labs/cryft-cli/pkg/utils"
	"github.com/cryft-labs/cryft-cli/pkg/ux"
	"github.com/cryft-labs/cryft-cli/pkg/vm"
	"github.com/MetalBlockchain/metal-network-runner/client"
	"github.com/MetalBlockchain/metal-network-runner/rpcpb"
	"github.com/MetalBlockchain/metal-network-runner/server"
	anrutils "github.com/MetalBlockchain/metal-network-runner/utils"
	"github.com/MetalBlockchain/metalgo/api/info"
	"github.com/MetalBlockchain/metalgo/config"
	"github.com/MetalBlockchain/metalgo/genesis"
	"github.com/MetalBlockchain/metalgo/ids"
	"github.com/MetalBlockchain/metalgo/utils/crypto/keychain"
	"github.com/MetalBlockchain/metalgo/utils/logging"
	"github.com/MetalBlockchain/metalgo/utils/set"
	"github.com/MetalBlockchain/metalgo/utils/storage"
	"github.com/MetalBlockchain/metalgo/vms/components/avax"
	"github.com/MetalBlockchain/metalgo/vms/components/verify"
	"github.com/MetalBlockchain/metalgo/vms/platformvm"
	"github.com/MetalBlockchain/metalgo/vms/platformvm/reward"
	"github.com/MetalBlockchain/metalgo/vms/platformvm/signer"
	"github.com/MetalBlockchain/metalgo/vms/platformvm/txs"
	"github.com/MetalBlockchain/metalgo/vms/secp256k1fx"
	"github.com/MetalBlockchain/metalgo/wallet/subnet/primary"
	"github.com/MetalBlockchain/metalgo/wallet/subnet/primary/common"
	"github.com/shubhamdubey02/subnet-evm/core"
	"go.uber.org/zap"
)

type LocalDeployer struct {
	procChecker        binutils.ProcessChecker
	binChecker         binutils.BinaryChecker
	getClientFunc      getGRPCClientFunc
	binaryDownloader   binutils.PluginBinaryDownloader
	app                *application.Avalanche
	backendStartedHere bool
	setDefaultSnapshot setDefaultSnapshotFunc
	avagoVersion       string
	avagoBinaryPath    string
	vmBin              string
}

// uses either avagoVersion or avagoBinaryPath
func NewLocalDeployer(
	app *application.Avalanche,
	avagoVersion string,
	avagoBinaryPath string,
	vmBin string,
) *LocalDeployer {
	return &LocalDeployer{
		procChecker:        binutils.NewProcessChecker(),
		binChecker:         binutils.NewBinaryChecker(),
		getClientFunc:      binutils.NewGRPCClient,
		binaryDownloader:   binutils.NewPluginBinaryDownloader(app),
		app:                app,
		setDefaultSnapshot: SetDefaultSnapshot,
		avagoVersion:       avagoVersion,
		avagoBinaryPath:    avagoBinaryPath,
		vmBin:              vmBin,
	}
}

type getGRPCClientFunc func(...binutils.GRPCClientOpOption) (client.Client, error)

type setDefaultSnapshotFunc func(string, bool, string, bool) (bool, error)

type DeployInfo struct {
	SubnetID                   ids.ID
	BlockchainID               ids.ID
	TeleporterMessengerAddress string
	TeleporterRegistryAddress  string
}

// DeployToLocalNetwork does the heavy lifting:
// * it checks the gRPC is running, if not, it starts it
// * kicks off the actual deployment
func (d *LocalDeployer) DeployToLocalNetwork(chain string, chainGenesis []byte, genesisPath string, subnetIDStr string) (*DeployInfo, error) {
	if err := d.StartServer(); err != nil {
		return nil, err
	}
	return d.doDeploy(chain, chainGenesis, genesisPath, subnetIDStr)
}

func getAssetID(wallet primary.Wallet, tokenName string, tokenSymbol string, maxSupply uint64) (ids.ID, error) {
	xWallet := wallet.X()
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			genesis.EWOQKey.PublicKey().Address(),
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultWalletCreationTimeout)
	subnetAssetTx, err := xWallet.IssueCreateAssetTx(
		tokenName,
		tokenSymbol,
		9, // denomination for UI purposes only in explorer
		map[uint32][]verify.State{
			0: {
				&secp256k1fx.TransferOutput{
					Amt:          maxSupply,
					OutputOwners: *owner,
				},
			},
		},
		common.WithContext(ctx),
	)
	defer cancel()
	if err != nil {
		return ids.Empty, err
	}
	return subnetAssetTx.ID(), nil
}

func exportToPChain(wallet primary.Wallet, owner *secp256k1fx.OutputOwners, subnetAssetID ids.ID, maxSupply uint64) error {
	xWallet := wallet.X()
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultWalletCreationTimeout)

	_, err := xWallet.IssueExportTx(
		ids.Empty,
		[]*avax.TransferableOutput{
			{
				Asset: avax.Asset{
					ID: subnetAssetID,
				},
				Out: &secp256k1fx.TransferOutput{
					Amt:          maxSupply,
					OutputOwners: *owner,
				},
			},
		},
		common.WithContext(ctx),
	)
	defer cancel()
	return err
}

func importFromXChain(wallet primary.Wallet, owner *secp256k1fx.OutputOwners) error {
	xWallet := wallet.X()
	pWallet := wallet.P()
	xChainID := xWallet.Builder().Context().BlockchainID
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultWalletCreationTimeout)
	_, err := pWallet.IssueImportTx(
		xChainID,
		owner,
		common.WithContext(ctx),
	)
	defer cancel()
	return err
}

func IssueTransformSubnetTx(
	elasticSubnetConfig models.ElasticSubnetConfig,
	kc keychain.Keychain,
	subnetID ids.ID,
	tokenName string,
	tokenSymbol string,
	maxSupply uint64,
) (ids.ID, ids.ID, error) {
	ctx := context.Background()
	api := constants.LocalAPIEndpoint
	wallet, err := primary.MakeWallet(
		ctx,
		&primary.WalletConfig{
			URI:              api,
			AVAXKeychain:     kc,
			EthKeychain:      secp256k1fx.NewKeychain(),
			PChainTxsToFetch: set.Of(subnetID),
		},
	)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	subnetAssetID, err := getAssetID(wallet, tokenName, tokenSymbol, maxSupply)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			genesis.EWOQKey.PublicKey().Address(),
		},
	}
	err = exportToPChain(wallet, owner, subnetAssetID, maxSupply)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	err = importFromXChain(wallet, owner)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultConfirmTxTimeout)
	transformSubnetTx, err := wallet.P().IssueTransformSubnetTx(elasticSubnetConfig.SubnetID, subnetAssetID,
		elasticSubnetConfig.InitialSupply, elasticSubnetConfig.MaxSupply, elasticSubnetConfig.MinConsumptionRate,
		elasticSubnetConfig.MaxConsumptionRate, elasticSubnetConfig.MinValidatorStake, elasticSubnetConfig.MaxValidatorStake,
		elasticSubnetConfig.MinStakeDuration, elasticSubnetConfig.MaxStakeDuration, elasticSubnetConfig.MinDelegationFee,
		elasticSubnetConfig.MinDelegatorStake, elasticSubnetConfig.MaxValidatorWeightFactor, elasticSubnetConfig.UptimeRequirement,
		common.WithContext(ctx),
	)
	defer cancel()
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	return transformSubnetTx.ID(), subnetAssetID, err
}

func IssueAddPermissionlessValidatorTx(
	kc keychain.Keychain,
	subnetID ids.ID,
	nodeID ids.NodeID,
	stakeAmount uint64,
	assetID ids.ID,
	startTime uint64,
	endTime uint64,
) (ids.ID, error) {
	ctx := context.Background()
	api := constants.LocalAPIEndpoint
	wallet, err := primary.MakeWallet(
		ctx,
		&primary.WalletConfig{
			URI:              api,
			AVAXKeychain:     kc,
			EthKeychain:      secp256k1fx.NewKeychain(),
			PChainTxsToFetch: set.Of(subnetID),
		},
	)
	if err != nil {
		return ids.Empty, err
	}
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			genesis.EWOQKey.PublicKey().Address(),
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultConfirmTxTimeout)
	tx, err := wallet.P().IssueAddPermissionlessValidatorTx(
		&txs.SubnetValidator{
			Validator: txs.Validator{
				NodeID: nodeID,
				Start:  startTime,
				End:    endTime,
				Wght:   stakeAmount,
			},
			Subnet: subnetID,
		},
		&signer.Empty{},
		assetID,
		owner,
		owner,
		reward.PercentDenominator,
		common.WithContext(ctx),
	)
	defer cancel()
	if err != nil {
		return ids.Empty, err
	}
	return tx.ID(), err
}

func IssueAddPermissionlessDelegatorTx(
	kc keychain.Keychain,
	subnetID ids.ID,
	nodeID ids.NodeID,
	stakeAmount uint64,
	assetID ids.ID,
	startTime uint64,
	endTime uint64,
) (ids.ID, error) {
	ctx := context.Background()
	api := constants.LocalAPIEndpoint
	wallet, err := primary.MakeWallet(
		ctx,
		&primary.WalletConfig{
			URI:              api,
			AVAXKeychain:     kc,
			EthKeychain:      secp256k1fx.NewKeychain(),
			PChainTxsToFetch: set.Of(subnetID),
		},
	)
	if err != nil {
		return ids.Empty, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultConfirmTxTimeout)
	tx, err := wallet.P().IssueAddPermissionlessDelegatorTx(
		&txs.SubnetValidator{
			Validator: txs.Validator{
				NodeID: nodeID,
				Start:  startTime,
				End:    endTime,
				Wght:   stakeAmount,
			},
			Subnet: subnetID,
		},
		assetID,
		&secp256k1fx.OutputOwners{},
		common.WithContext(ctx),
	)
	defer cancel()
	if err != nil {
		return ids.Empty, err
	}
	return tx.ID(), err
}

func (d *LocalDeployer) StartServer() error {
	isRunning, err := d.procChecker.IsServerProcessRunning(d.app)
	if err != nil {
		return fmt.Errorf("failed querying if server process is running: %w", err)
	}
	if !isRunning {
		d.app.Log.Debug("gRPC server is not running")
		if err := binutils.StartServerProcess(d.app); err != nil {
			return fmt.Errorf("failed starting gRPC server process: %w", err)
		}
		d.backendStartedHere = true
	}
	return nil
}

func GetCurrentSupply(subnetID ids.ID) error {
	api := constants.LocalAPIEndpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	_, _, err := pClient.GetCurrentSupply(ctx, subnetID)
	return err
}

// BackendStartedHere returns true if the backend was started by this run,
// or false if it found it there already
func (d *LocalDeployer) BackendStartedHere() bool {
	return d.backendStartedHere
}

// doDeploy the actual deployment to the network runner
// steps:
//   - checks if the network has been started
//   - install all needed plugin binaries, for the the new VM, and the already deployed VMs
//   - either starts a network from the default snapshot if not started,
//     or restarts the already available network while preserving state
//   - waits completion of operation
//   - get from the network an available subnet ID to be used in blockchain creation
//   - deploy a new blockchain for the given VM ID, genesis, and available subnet ID
//   - waits completion of operation
//   - show status
func (d *LocalDeployer) doDeploy(chain string, chainGenesis []byte, genesisPath string, subnetIDStr string) (*DeployInfo, error) {
	needsRestart, avalancheGoBinPath, err := d.SetupLocalEnv()
	if err != nil {
		return nil, err
	}

	backendLogFile, err := binutils.GetBackendLogFile(d.app)
	var backendLogDir string
	if err == nil {
		// TODO should we do something if there _was_ an error?
		backendLogDir = filepath.Dir(backendLogFile)
	}

	cli, err := d.getClientFunc()
	if err != nil {
		return nil, fmt.Errorf("error creating gRPC Client: %w", err)
	}
	defer cli.Close()

	runDir := d.app.GetRunDir()

	ctx, cancel := utils.GetANRContext()
	defer cancel()

	// loading sidecar before it's needed so we catch any error early
	sc, err := d.app.LoadSidecar(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to load sidecar: %w", err)
	}

	// check for network status
	networkBooted := true
	clusterInfo, err := WaitForHealthy(ctx, cli)
	rootDir := clusterInfo.GetRootDataDir()
	if err != nil {
		if !server.IsServerError(err, server.ErrNotBootstrapped) {
			FindErrorLogs(rootDir, backendLogDir)
			return nil, fmt.Errorf("failed to query network health: %w", err)
		} else {
			networkBooted = false
		}
	}

	chainVMID, err := anrutils.VMID(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}
	d.app.Log.Debug("this VM will get ID", zap.String("vm-id", chainVMID.String()))

	if networkBooted && needsRestart {
		ux.Logger.PrintToUser("Restarting the network...")
		if _, err := cli.Stop(ctx); err != nil {
			return nil, fmt.Errorf("failed to stop network: %w", err)
		}
		if err := d.app.ResetPluginsDir(); err != nil {
			return nil, fmt.Errorf("failed to reset plugins dir: %w", err)
		}
		networkBooted = false
	}

	if !networkBooted {
		if err := d.startNetwork(ctx, cli, avalancheGoBinPath, runDir); err != nil {
			FindErrorLogs(rootDir, backendLogDir)
			return nil, err
		}
	}

	// latest check for rpc compatibility
	statusChecker := localnetworkinterface.NewStatusChecker()
	_, avagoRPCVersion, _, err := statusChecker.GetCurrentNetworkVersion()
	if err != nil {
		return nil, err
	}
	if avagoRPCVersion != sc.RPCVersion {
		if !networkBooted {
			_, _ = cli.Stop(ctx)
		}
		return nil, fmt.Errorf(
			"the avalanchego deployment uses rpc version %d but your subnet has version %d and is not compatible",
			avagoRPCVersion,
			sc.RPCVersion,
		)
	}

	// get VM info
	clusterInfo, err = WaitForHealthy(ctx, cli)
	if err != nil {
		FindErrorLogs(clusterInfo.GetRootDataDir(), backendLogDir)
		return nil, fmt.Errorf("failed to query network health: %w", err)
	}
	rootDir = clusterInfo.GetRootDataDir()

	if alreadyDeployed(chainVMID, clusterInfo) {
		return nil, fmt.Errorf("subnet %s has already been deployed", chain)
	}

	numBlockchains := len(clusterInfo.CustomChains)

	subnetIDs := maps.Keys(clusterInfo.Subnets)

	// in order to make subnet deploy faster, a set of validated subnet IDs is preloaded
	// in the bootstrap snapshot
	// we select one to be used for creating the next blockchain, for that we use the
	// number of currently created blockchains as the index to select the next subnet ID,
	// so we get incremental selection
	sort.Strings(subnetIDs)
	if len(subnetIDs) == 0 {
		return nil, errors.New("the network has not preloaded subnet IDs")
	}

	// If not set via argument, deploy to the next available subnet
	if subnetIDStr == "" {
		subnetIDStr = subnetIDs[numBlockchains%len(subnetIDs)]
	}

	// if a chainConfig has been configured
	var (
		chainConfig            string
		chainConfigFile        = filepath.Join(d.app.GetSubnetDir(), chain, constants.ChainConfigFileName)
		perNodeChainConfig     string
		perNodeChainConfigFile = filepath.Join(d.app.GetSubnetDir(), chain, constants.PerNodeChainConfigFileName)
		subnetConfig           string
		subnetConfigFile       = filepath.Join(d.app.GetSubnetDir(), chain, constants.SubnetConfigFileName)
	)
	if _, err := os.Stat(chainConfigFile); err == nil {
		// currently the ANR only accepts the file as a path, not its content
		chainConfig = chainConfigFile
	}
	if _, err := os.Stat(perNodeChainConfigFile); err == nil {
		perNodeChainConfig = perNodeChainConfigFile
	}
	if _, err := os.Stat(subnetConfigFile); err == nil {
		subnetConfig = subnetConfigFile
	}

	// install the plugin binary for the new VM
	if err := d.installPlugin(chainVMID, d.vmBin); err != nil {
		return nil, err
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Deploying Blockchain. Wait until network acknowledges...")

	// create a new blockchain on the already started network, associated to
	// the given VM ID, genesis, and available subnet ID
	blockchainSpecs := []*rpcpb.BlockchainSpec{
		{
			VmName:   chain,
			Genesis:  genesisPath,
			SubnetId: &subnetIDStr,
			SubnetSpec: &rpcpb.SubnetSpec{
				SubnetConfig: subnetConfig,
			},
			ChainConfig:        chainConfig,
			BlockchainAlias:    chain,
			PerNodeChainConfig: perNodeChainConfig,
		},
	}
	deployBlockchainsInfo, err := cli.CreateBlockchains(
		ctx,
		blockchainSpecs,
	)
	if err != nil {
		FindErrorLogs(rootDir, backendLogDir)
		pluginRemoveErr := d.removeInstalledPlugin(chainVMID)
		if pluginRemoveErr != nil {
			ux.Logger.PrintToUser("Failed to remove plugin binary: %s", pluginRemoveErr)
		}
		return nil, fmt.Errorf("failed to deploy blockchain: %w", err)
	}
	rootDir = clusterInfo.GetRootDataDir()

	d.app.Log.Debug(deployBlockchainsInfo.String())

	clusterInfo, err = WaitForHealthy(ctx, cli)
	if err != nil {
		FindErrorLogs(rootDir, backendLogDir)
		pluginRemoveErr := d.removeInstalledPlugin(chainVMID)
		if pluginRemoveErr != nil {
			ux.Logger.PrintToUser("Failed to remove plugin binary: %s", pluginRemoveErr)
		}
		return nil, fmt.Errorf("failed to query network health: %w", err)
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Blockchain ready to use. Local network node endpoints:")
	if err := ux.PrintEndpointTables(clusterInfo); err != nil {
		return nil, err
	}

	endpoint := GetFirstEndpoint(clusterInfo, chain)
	ux.Logger.PrintToUser("Browser Extension connection details (any node URL from above works):")
	rpcURL := endpoint[strings.LastIndex(endpoint, "http"):]
	ux.Logger.PrintToUser("RPC URL:           %s", rpcURL)
	codespaceURL, err := utils.GetCodespaceURL(rpcURL)
	if err != nil {
		return nil, err
	}
	if codespaceURL != "" {
		ux.Logger.PrintToUser("Codespace RPC URL: %s", codespaceURL)
	}

	if sc.VM == models.SubnetEvm {
		_, subnetAirdropAddress, subnetAirdropPrivKey, err := GetSubnetAirdropKeyInfo(d.app, chain)
		if err != nil {
			ux.Logger.PrintToUser("failure loading subnet airdrop info: %s", err)
		}
		if err := d.printExtraEvmInfo(chain, chainGenesis, subnetAirdropAddress, subnetAirdropPrivKey); err != nil {
			// not supposed to happen due to genesis pre validation
			return nil, nil
		}
	}

	// we can safely ignore errors here as the subnets have already been generated
	subnetID, _ := ids.FromString(subnetIDStr)
	var blockchainID ids.ID
	for _, info := range clusterInfo.CustomChains {
		if info.VmId == chainVMID.String() {
			blockchainID, _ = ids.FromString(info.ChainId)
		}
	}
	return &DeployInfo{
		SubnetID:     subnetID,
		BlockchainID: blockchainID,
	}, nil
}

func (d *LocalDeployer) printExtraEvmInfo(
	chain string,
	chainGenesis []byte,
	subnetAirdropAddress string,
	subnetAirdropPrivKey string,
) error {
	var evmGenesis core.Genesis
	if err := json.Unmarshal(chainGenesis, &evmGenesis); err != nil {
		return fmt.Errorf("failed to unmarshall genesis: %w", err)
	}
	for address := range evmGenesis.Alloc {
		amount := evmGenesis.Alloc[address].Balance
		formattedAmount := new(big.Int).Div(amount, big.NewInt(params.Ether))
		switch address.Hex() {
		case vm.PrefundedEwoqAddress.Hex():
			ux.Logger.PrintToUser("Funded address:    %s with %s (10^18) - private key: %s", address, formattedAmount.String(), vm.PrefundedEwoqPrivate)
		case subnetAirdropAddress:
			ux.Logger.PrintToUser("Funded address:    %s with %s (10^18) - private key: %s", address, formattedAmount.String(), subnetAirdropPrivKey)
		}
	}
	ux.Logger.PrintToUser("Network name:      %s", chain)
	ux.Logger.PrintToUser("Chain ID:          %s", evmGenesis.Config.ChainID)
	ux.Logger.PrintToUser("Currency Symbol:   %s", d.app.GetTokenSymbol(chain))
	return nil
}

// SetupLocalEnv also does some heavy lifting:
// * sets up default snapshot if not installed
// * checks if avalanchego is installed in the local binary path
// * if not, it downloads it and installs it (os - and archive dependent)
// * returns the location of the avalanchego path
func (d *LocalDeployer) SetupLocalEnv() (bool, string, error) {
	avagoVersion := ""
	avalancheGoBinPath := ""
	if d.avagoBinaryPath != "" {
		avalancheGoBinPath = d.avagoBinaryPath
		// get avago version from binary
		out, err := exec.Command(avalancheGoBinPath, "--"+config.VersionKey).Output()
		if err != nil {
			return false, "", err
		}
		fullVersion := string(out)
		splittedFullVersion := strings.Split(fullVersion, " ")
		if len(splittedFullVersion) == 0 {
			return false, "", fmt.Errorf("invalid avalanchego version: %q", fullVersion)
		}
		version := splittedFullVersion[0]
		splittedVersion := strings.Split(version, "/")
		if len(splittedVersion) != 2 {
			return false, "", fmt.Errorf("invalid avalanchego version: %q", fullVersion)
		}
		avagoVersion = "v" + splittedVersion[1]
	} else {
		var (
			avagoDir string
			err      error
		)
		avagoVersion, avagoDir, err = d.setupLocalEnv()
		if err != nil {
			return false, "", fmt.Errorf("failed setting up local environment: %w", err)
		}
		avalancheGoBinPath = filepath.Join(avagoDir, "metalgo")
	}

	configSingleNodeEnabled := d.app.Conf.GetConfigBoolValue(constants.ConfigSingleNodeEnabledKey)
	needsRestart, err := d.setDefaultSnapshot(d.app.GetSnapshotsDir(), false, avagoVersion, configSingleNodeEnabled)
	if err != nil {
		return false, "", fmt.Errorf("failed setting up snapshots: %w", err)
	}

	pluginDir := d.app.GetPluginsDir()

	if err := os.MkdirAll(pluginDir, constants.DefaultPerms755); err != nil {
		return false, "", fmt.Errorf("could not create pluginDir %s", pluginDir)
	}

	exists, err := storage.FolderExists(pluginDir)
	if !exists || err != nil {
		return false, "", fmt.Errorf("evaluated pluginDir to be %s but it does not exist", pluginDir)
	}

	// TODO: we need some better version management here
	// * compare latest to local version
	// * decide if force update or give user choice
	exists, err = storage.FileExists(avalancheGoBinPath)
	if !exists || err != nil {
		return false, "", fmt.Errorf(
			"evaluated avalancheGoBinPath to be %s but it does not exist", avalancheGoBinPath)
	}

	return needsRestart, avalancheGoBinPath, nil
}

func (d *LocalDeployer) setupLocalEnv() (string, string, error) {
	return binutils.SetupAvalanchego(d.app, d.avagoVersion)
}

// WaitForHealthy polls continuously until the network is ready to be used
func WaitForHealthy(
	ctx context.Context,
	cli client.Client,
) (*rpcpb.ClusterInfo, error) {
	cancel := make(chan struct{})
	defer close(cancel)
	go ux.PrintWait(cancel)
	resp, err := cli.WaitForHealthy(ctx)
	if err != nil {
		return nil, err
	}
	return resp.ClusterInfo, nil
}

// GetFirstEndpoint get a human readable endpoint for the given chain
func GetFirstEndpoint(clusterInfo *rpcpb.ClusterInfo, chain string) string {
	var endpoint string
	for _, nodeInfo := range clusterInfo.NodeInfos {
		for blockchainID, chainInfo := range clusterInfo.CustomChains {
			if chainInfo.ChainName == chain && nodeInfo.Name == clusterInfo.NodeNames[0] {
				endpoint = fmt.Sprintf("Endpoint at node %s for blockchain %q with VM ID %q: %s/ext/bc/%s/rpc", nodeInfo.Name, blockchainID, chainInfo.VmId, nodeInfo.GetUri(), blockchainID)
			}
		}
	}
	return endpoint
}

// HasEndpoints returns true if cluster info contains custom blockchains
func HasEndpoints(clusterInfo *rpcpb.ClusterInfo) bool {
	return len(clusterInfo.CustomChains) > 0
}

// return true if vm has already been deployed
func alreadyDeployed(chainVMID ids.ID, clusterInfo *rpcpb.ClusterInfo) bool {
	if clusterInfo != nil {
		for _, chainInfo := range clusterInfo.CustomChains {
			if chainInfo.VmId == chainVMID.String() {
				return true
			}
		}
	}
	return false
}

// get list of all needed plugins and install them
func (d *LocalDeployer) installPlugin(
	vmID ids.ID,
	vmBin string,
) error {
	return d.binaryDownloader.InstallVM(vmID.String(), vmBin)
}

// get list of all needed plugins and install them
func (d *LocalDeployer) removeInstalledPlugin(
	vmID ids.ID,
) error {
	return d.binaryDownloader.RemoveVM(vmID.String())
}

func getSnapshotLocs(isSingleNode bool, isPreCortina17 bool) (string, string, string, string) {
	bootstrapSnapshotArchiveName := ""
	url := ""
	shaSumURL := ""
	pathInShaSum := ""
	if isSingleNode {
		if isPreCortina17 {
			bootstrapSnapshotArchiveName = constants.BootstrapSnapshotSingleNodePreCortina17ArchiveName
			url = constants.BootstrapSnapshotSingleNodePreCortina17URL
			shaSumURL = constants.BootstrapSnapshotSingleNodePreCortina17SHA256URL
			pathInShaSum = constants.BootstrapSnapshotSingleNodePreCortina17LocalPath
		} else {
			bootstrapSnapshotArchiveName = constants.BootstrapSnapshotSingleNodeArchiveName
			url = constants.BootstrapSnapshotSingleNodeURL
			shaSumURL = constants.BootstrapSnapshotSingleNodeSHA256URL
			pathInShaSum = constants.BootstrapSnapshotSingleNodeLocalPath
		}
	} else {
		if isPreCortina17 {
			bootstrapSnapshotArchiveName = constants.BootstrapSnapshotPreCortina17ArchiveName
			url = constants.BootstrapSnapshotPreCortina17URL
			shaSumURL = constants.BootstrapSnapshotPreCortina17SHA256URL
			pathInShaSum = constants.BootstrapSnapshotPreCortina17LocalPath
		} else {
			bootstrapSnapshotArchiveName = constants.BootstrapSnapshotArchiveName
			url = constants.BootstrapSnapshotURL
			shaSumURL = constants.BootstrapSnapshotSHA256URL
			pathInShaSum = constants.BootstrapSnapshotLocalPath
		}
	}
	return bootstrapSnapshotArchiveName, url, shaSumURL, pathInShaSum
}

func getExpectedDefaultSnapshotSHA256Sum(isSingleNode bool, isPreCortina17 bool) (string, error) {
	_, _, url, path := getSnapshotLocs(isSingleNode, isPreCortina17)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed downloading sha256 sums: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed downloading sha256 sums: unexpected http status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	sha256FileBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed downloading sha256 sums: %w", err)
	}
	expectedSum, err := utils.SearchSHA256File(sha256FileBytes, path)
	if err != nil {
		return "", fmt.Errorf("failed obtaining snapshot sha256 sum: %w", err)
	}
	return expectedSum, nil
}

// Initialize default snapshot with bootstrap snapshot archive
// If force flag is set to true, overwrite the default snapshot if it exists
func SetDefaultSnapshot(snapshotsDir string, resetCurrentSnapshot bool, avagoVersion string, isSingleNode bool) (bool, error) {
	var isPreCortina17 bool
	if avagoVersion != "" {
		isPreCortina17 = semver.Compare(avagoVersion, constants.Cortina17Version) < 0
	}
	bootstrapSnapshotArchiveName, url, _, _ := getSnapshotLocs(isSingleNode, isPreCortina17)
	currentBootstrapNamePath := filepath.Join(snapshotsDir, constants.CurrentBootstrapNamePath)
	exists, err := storage.FileExists(currentBootstrapNamePath)
	if err != nil {
		return false, err
	}
	if exists {
		currentBootstrapNameBytes, err := os.ReadFile(currentBootstrapNamePath)
		if err != nil {
			return false, err
		}
		currentBootstrapName := string(currentBootstrapNameBytes)
		if currentBootstrapName != bootstrapSnapshotArchiveName {
			// there is a snapshot image change.
			resetCurrentSnapshot = true
		}
	} else {
		// we have no ref of currently used snapshot image
		resetCurrentSnapshot = true
	}
	bootstrapSnapshotArchivePath := filepath.Join(snapshotsDir, bootstrapSnapshotArchiveName)
	defaultSnapshotPath := filepath.Join(snapshotsDir, "anr-snapshot-"+constants.DefaultSnapshotName)
	defaultSnapshotInUse := false
	if _, err := os.Stat(defaultSnapshotPath); err == nil {
		defaultSnapshotInUse = true
	}
	// will download either if file not exists or if sha256 sum is not the same
	downloadSnapshot := false
	if _, err := os.Stat(bootstrapSnapshotArchivePath); os.IsNotExist(err) {
		downloadSnapshot = true
	} else {
		gotSum, err := utils.GetSHA256FromDisk(bootstrapSnapshotArchivePath)
		if err != nil {
			return false, err
		}
		expectedSum, err := getExpectedDefaultSnapshotSHA256Sum(isSingleNode, isPreCortina17)
		if err != nil {
			ux.Logger.PrintToUser("Warning: failure verifying that the local snapshot is the latest one: %s", err)
		} else if gotSum != expectedSum {
			downloadSnapshot = true
		}
	}
	if downloadSnapshot {
		resp, err := http.Get(url)
		if err != nil {
			return false, fmt.Errorf("failed downloading bootstrap snapshot: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("failed downloading bootstrap snapshot: unexpected http status code: %d", resp.StatusCode)
		}
		defer resp.Body.Close()
		bootstrapSnapshotBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("failed downloading bootstrap snapshot: %w", err)
		}
		if err := os.WriteFile(bootstrapSnapshotArchivePath, bootstrapSnapshotBytes, constants.WriteReadReadPerms); err != nil {
			return false, fmt.Errorf("failed writing down bootstrap snapshot: %w", err)
		}
		if defaultSnapshotInUse {
			ux.Logger.PrintToUser(logging.Yellow.Wrap("A new network snapshot image is available. Replacing the current one."))
		}
		resetCurrentSnapshot = true
	}
	if resetCurrentSnapshot {
		if err := os.RemoveAll(defaultSnapshotPath); err != nil {
			return false, fmt.Errorf("failed removing default snapshot: %w", err)
		}
		bootstrapSnapshotBytes, err := os.ReadFile(bootstrapSnapshotArchivePath)
		if err != nil {
			return false, fmt.Errorf("failed reading bootstrap snapshot: %w", err)
		}
		if err := binutils.InstallArchive("tar.gz", bootstrapSnapshotBytes, snapshotsDir); err != nil {
			return false, fmt.Errorf("failed installing bootstrap snapshot: %w", err)
		}
		if err := os.WriteFile(currentBootstrapNamePath, []byte(bootstrapSnapshotArchiveName), constants.DefaultPerms755); err != nil {
			return false, err
		}
	}
	return resetCurrentSnapshot, nil
}

// start the network
func (d *LocalDeployer) startNetwork(
	ctx context.Context,
	cli client.Client,
	avalancheGoBinPath string,
	runDir string,
) error {
	loadSnapshotOpts := []client.OpOption{
		client.WithExecPath(avalancheGoBinPath),
		client.WithRootDataDir(runDir),
		client.WithReassignPortsIfUsed(true),
		client.WithPluginDir(d.app.GetPluginsDir()),
	}

	// load global node configs if they exist
	configStr, err := d.app.Conf.LoadNodeConfig()
	if err != nil {
		return nil
	}
	if configStr != "" {
		loadSnapshotOpts = append(loadSnapshotOpts, client.WithGlobalNodeConfig(configStr))
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Booting Network. Wait until healthy...")
	resp, err := cli.LoadSnapshot(
		ctx,
		constants.DefaultSnapshotName,
		loadSnapshotOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to start network :%w", err)
	}
	ux.Logger.PrintToUser("Node logs directory: %s/node<i>/logs", resp.ClusterInfo.RootDataDir)
	ux.Logger.PrintToUser("Network ready to use.")
	return nil
}

// Returns an error if the server cannot be contacted. You may want to ignore this error.
func GetLocallyDeployedSubnets() (map[string]struct{}, error) {
	deployedNames := map[string]struct{}{}
	// if the server can not be contacted, or there is a problem with the query,
	// DO NOT FAIL, just print No for deployed status
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	resp, err := cli.Status(ctx)
	if err != nil {
		return nil, err
	}

	for _, chain := range resp.GetClusterInfo().CustomChains {
		deployedNames[chain.ChainName] = struct{}{}
	}

	return deployedNames, nil
}

func IssueRemoveSubnetValidatorTx(kc keychain.Keychain, subnetID ids.ID, nodeID ids.NodeID) (ids.ID, error) {
	ctx := context.Background()
	api := constants.LocalAPIEndpoint
	wallet, err := primary.MakeWallet(
		ctx,
		&primary.WalletConfig{
			URI:              api,
			AVAXKeychain:     kc,
			EthKeychain:      secp256k1fx.NewKeychain(),
			PChainTxsToFetch: set.Of(subnetID),
		},
	)
	if err != nil {
		return ids.Empty, err
	}

	tx, err := wallet.P().IssueRemoveSubnetValidatorTx(nodeID, subnetID)
	return tx.ID(), err
}

func GetSubnetValidators(subnetID ids.ID) ([]platformvm.ClientPermissionlessValidator, error) {
	api := constants.LocalAPIEndpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()

	return pClient.GetCurrentValidators(ctx, subnetID, nil)
}

func CheckNodeIsInSubnetValidators(subnetID ids.ID, nodeID string) (bool, error) {
	api := constants.LocalAPIEndpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()

	vals, err := pClient.GetCurrentValidators(ctx, subnetID, nil)
	if err != nil {
		return false, err
	}
	for _, v := range vals {
		if v.NodeID.String() == nodeID {
			return true, nil
		}
	}
	return false, nil
}

type ExtraLocalNetworkData struct {
	CChainTeleporterMessengerAddress string
	CChainTeleporterRegistryAddress  string
}

func GetExtraLocalNetworkData(app *application.Avalanche) (*ExtraLocalNetworkData, error) {
	bs, err := os.ReadFile(app.GetExtraLocalNetworkDataPath())
	if err != nil {
		return nil, err
	}
	extraLocalNetworkData := ExtraLocalNetworkData{}
	if err := json.Unmarshal(bs, &extraLocalNetworkData); err != nil {
		return nil, err
	}
	return &extraLocalNetworkData, nil
}

func WriteExtraLocalNetworkData(app *application.Avalanche, cchainTeleporterMessengerAddress string, cchainTeleporterRegistryAddress string) error {
	extraLocalNetworkDataPath := app.GetExtraLocalNetworkDataPath()
	extraLocalNetworkData := ExtraLocalNetworkData{
		CChainTeleporterMessengerAddress: cchainTeleporterMessengerAddress,
		CChainTeleporterRegistryAddress:  cchainTeleporterRegistryAddress,
	}
	bs, err := json.Marshal(&extraLocalNetworkData)
	if err != nil {
		return err
	}
	return os.WriteFile(extraLocalNetworkDataPath, bs, constants.WriteReadReadPerms)
}

func GetChainID(network models.Network, chainName string) (ids.ID, error) {
	client := info.NewClient(network.Endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	return client.GetBlockchainID(ctx, chainName)
}

func GetChainIDs(network models.Network, chainName string) (string, string, error) {
	pClient := platformvm.NewClient(network.Endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	blockChains, err := pClient.GetBlockchains(ctx)
	if err != nil {
		return "", "", err
	}
	if chain := utils.Find(blockChains, func(e platformvm.APIBlockchain) bool { return e.Name == chainName }); chain != nil {
		return chain.SubnetID.String(), chain.ID.String(), nil
	}
	return "", "", fmt.Errorf("%s not found on primary network blockchains", chainName)
}

func GetSubnetAirdropKeyInfo(app *application.Avalanche, subnetName string) (string, string, string, error) {
	keyName := vm.GetSubnetAirdropKeyName(subnetName)
	keyPath := app.GetKeyPath(keyName)
	if utils.FileExists(keyPath) {
		k, err := key.LoadSoft(models.NewLocalNetwork().ID, keyPath)
		if err != nil {
			return "", "", "", err
		}
		return keyName, k.C(), hex.EncodeToString(k.Raw()), nil
	}
	return "", "", "", nil
}
