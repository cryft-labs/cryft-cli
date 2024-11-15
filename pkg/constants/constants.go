// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import (
	"time"
)

const (
	DefaultPerms755        = 0o755
	WriteReadReadPerms     = 0o644
	WriteReadUserOnlyPerms = 0o600

	UbuntuVersionLTS = "20.04"

	BaseDirName = ".metal-cli"
	LogDir      = "logs"

	ServerRunFile      = "gRPCserver.run"
	AvalancheCliBinDir = "bin"
	RunDir             = "runs"
	ServicesDir        = "services"

	SuffixSeparator              = "_"
	SidecarFileName              = "sidecar.json"
	GenesisFileName              = "genesis.json"
	ElasticSubnetConfigFileName  = "elastic_subnet_config.json"
	SidecarSuffix                = SuffixSeparator + SidecarFileName
	GenesisSuffix                = SuffixSeparator + GenesisFileName
	NodeFileName                 = "node.json"
	NodePrometheusConfigFileName = "prometheus.yml"
	NodeCloudConfigFileName      = "node_cloud_config.json"
	AnsibleDir                   = "ansible"
	AnsibleHostInventoryFileName = "hosts"
	StopAWSNode                  = "stop-aws-node"
	CreateAWSNode                = "create-aws-node"
	GetAWSNodeIP                 = "get-aws-node-ip"
	ClustersConfigFileName       = "cluster_config.json"
	ClustersConfigVersion        = "1"
	StakerCertFileName           = "staker.crt"
	StakerKeyFileName            = "staker.key"
	BLSKeyFileName               = "signer.key"
	SidecarVersion               = "1.4.0"

	MaxLogFileSize   = 4
	MaxNumOfLogFiles = 5
	RetainOldFiles   = 0 // retain all old log files

	CloudOperationTimeout = 2 * time.Minute

	ANRRequestTimeout      = 3 * time.Minute
	APIRequestTimeout      = 30 * time.Second
	APIRequestLargeTimeout = 2 * time.Minute
	FastGRPCDialTimeout    = 100 * time.Millisecond

	SSHServerStartTimeout       = 1 * time.Minute
	SSHScriptTimeout            = 2 * time.Minute
	SSHLongRunningScriptTimeout = 10 * time.Minute
	SSHDirOpsTimeout            = 10 * time.Second
	SSHFileOpsTimeout           = 100 * time.Second
	SSHPOSTTimeout              = 10 * time.Second
	SSHSleepBetweenChecks       = 1 * time.Second
	SSHShell                    = "/bin/bash"
	AWSVolumeTypeGP3            = "gp3"
	AWSVolumeTypeIO1            = "io1"
	AWSVolumeTypeIO2            = "io2"
	AWSGP3DefaultIOPS           = 3000
	AWSGP3DefaultThroughput     = 125
	SimulatePublicNetwork       = "SIMULATE_PUBLIC_NETWORK"

	TahoeAPIEndpoint   = "https://tahoe.metalblockchain.org"
	MainnetAPIEndpoint = "https://api.metalblockchain.org"

	// this depends on bootstrap snapshot
	LocalAPIEndpoint = "http://127.0.0.1:9650"
	LocalNetworkID   = 1337

	DevnetAPIEndpoint = ""
	DevnetNetworkID   = 1338

	DefaultTokenName = "Test Token"

	DefaultTokenSymbol = "TEST"

	HealthCheckInterval = 100 * time.Millisecond

	// it's unlikely anyone would want to name a snapshot `default`
	// but let's add some more entropy
	SnapshotsDirName = "snapshots"

	DefaultSnapshotName = "default-1654102510"

	Cortina17Version = "v1.10.17"

	BootstrapSnapshotRawBranch = "https://github.com/cryft-labs/cryft-cli/raw/main/"

	CurrentBootstrapNamePath = "currentBootstrapName.txt"

	AssetsDir = "assets/"

	BootstrapSnapshotArchiveName = "bootstrapSnapshot.tar.gz"
	BootstrapSnapshotLocalPath   = AssetsDir + BootstrapSnapshotArchiveName
	BootstrapSnapshotURL         = BootstrapSnapshotRawBranch + BootstrapSnapshotLocalPath
	BootstrapSnapshotSHA256URL   = BootstrapSnapshotRawBranch + AssetsDir + "sha256sum.txt"

	BootstrapSnapshotSingleNodeArchiveName = "bootstrapSnapshotSingleNode.tar.gz"
	BootstrapSnapshotSingleNodeLocalPath   = AssetsDir + BootstrapSnapshotSingleNodeArchiveName
	BootstrapSnapshotSingleNodeURL         = BootstrapSnapshotRawBranch + BootstrapSnapshotSingleNodeLocalPath
	BootstrapSnapshotSingleNodeSHA256URL   = BootstrapSnapshotRawBranch + AssetsDir + "sha256sumSingleNode.txt"

	BootstrapSnapshotPreCortina17ArchiveName = "bootstrapSnapshot.PreCortina17.tar.gz"
	BootstrapSnapshotPreCortina17LocalPath   = AssetsDir + BootstrapSnapshotPreCortina17ArchiveName
	BootstrapSnapshotPreCortina17URL         = BootstrapSnapshotRawBranch + BootstrapSnapshotPreCortina17LocalPath
	BootstrapSnapshotPreCortina17SHA256URL   = BootstrapSnapshotRawBranch + AssetsDir + "sha256sum.PreCortina17.txt"

	BootstrapSnapshotSingleNodePreCortina17ArchiveName = "bootstrapSnapshotSingleNode.PreCortina17.tar.gz"
	BootstrapSnapshotSingleNodePreCortina17LocalPath   = AssetsDir + BootstrapSnapshotSingleNodePreCortina17ArchiveName
	BootstrapSnapshotSingleNodePreCortina17URL         = BootstrapSnapshotRawBranch + BootstrapSnapshotSingleNodePreCortina17LocalPath
	BootstrapSnapshotSingleNodePreCortina17SHA256URL   = BootstrapSnapshotRawBranch + AssetsDir + "sha256sumSingleNode.PreCortina17.txt"

	ExtraLocalNetworkDataFilename     = "extra-local-network-data.json"
	ExtraLocalNetworkDataSnapshotsDir = "extra-local-network-data"

	CliInstallationURL         = "https://raw.githubusercontent.com/MetalBlockchain/metal-cli/main/scripts/install.sh"
	ExpectedCliInstallErr      = "resource temporarily unavailable"
	EIPLimitErr                = "AddressLimitExceeded"
	ErrCreatingAWSNode         = "failed to create AWS Node"
	ErrCreatingGCPNode         = "failed to create GCP Node"
	ErrReleasingGCPStaticIP    = "failed to release gcp static ip"
	KeyDir                     = "key"
	KeySuffix                  = ".pk"
	YAMLSuffix                 = ".yml"
	CustomGrafanaDashboardJSON = "custom.json"
	Enable                     = "enable"

	Disable = "disable"

	TimeParseLayout             = "2006-01-02 15:04:05"
	MinStakeWeight              = 1
	DefaultStakeWeight          = 20
	AVAXSymbol                  = "METAL"
	DefaultFujiStakeDuration    = "48h"
	DefaultMainnetStakeDuration = "336h"
	// The absolute minimum is 25 seconds, but set to 1 minute to allow for
	// time to go through the command
	DevnetStakingStartLeadTime                   = 30 * time.Second
	StakingStartLeadTime                         = 5 * time.Minute
	StakingMinimumLeadTime                       = 25 * time.Second
	PrimaryNetworkValidatingStartLeadTimeNodeCmd = 20 * time.Second
	PrimaryNetworkValidatingStartLeadTime        = 1 * time.Minute
	AWSCloudServerRunningState                   = "running"
	AvalancheCLISuffix                           = "-metal-cli"
	AWSDefaultCredential                         = "default"
	GCPDefaultImageProvider                      = "ubuntu-os-cloud"
	GCPImageFilter                               = "family=ubuntu-2004* AND architecture=x86_64"
	GCPEnvVar                                    = "GOOGLE_APPLICATION_CREDENTIALS"
	GCPDefaultAuthKeyPath                        = "~/.config/gcloud/application_default_credentials.json"
	CertSuffix                                   = "-kp.pem"
	AWSSecurityGroupSuffix                       = "-sg"
	ExportSubnetSuffix                           = "-export.dat"
	SSHTCPPort                                   = 22
	AvalanchegoAPIPort                           = 9650
	AvalanchegoP2PPort                           = 9651
	AvalanchegoGrafanaPort                       = 3000
	AvalanchegoLokiPort                          = 23101
	CloudServerStorageSize                       = 1000
	MonitoringCloudServerStorageSize             = 50
	OutboundPort                                 = 0
	// Set this one to true while testing changes that alter CLI execution on cloud nodes
	// Disable it for releases to save cluster creation time
	EnableSetupCLIFromSource           = false
	SetupCLIFromSourceBranch           = "main"
	BuildEnvGolangVersion              = "1.22.1"
	IsHealthyJSONFile                  = "isHealthy.json"
	IsBootstrappedJSONFile             = "isBootstrapped.json"
	AvalancheGoVersionJSONFile         = "avalancheGoVersion.json"
	SubnetSyncJSONFile                 = "isSubnetSynced.json"
	AnsibleInventoryDir                = "inventories"
	AnsibleTempInventoryDir            = "temp_inventories"
	AnsibleStatusDir                   = "status"
	AnsibleInventoryFlag               = "-i"
	AnsibleExtraArgsIdentitiesOnlyFlag = "--ssh-extra-args='-o IdentitiesOnly=yes'"
	AnsibleSSHShellParams              = "-o IdentitiesOnly=yes -o StrictHostKeyChecking=no"
	AnsibleSSHUseAgentParams           = "-o StrictHostKeyChecking=no"
	AnsibleExtraVarsFlag               = "--extra-vars"

	ConfigAPMCredentialsFileKey   = "credentials-file"
	ConfigAPMAdminAPIEndpointKey  = "admin-api-endpoint"
	ConfigNodeConfigKey           = "node-config"
	ConfigMetricsEnabledKey       = "MetricsEnabled"
	ConfigAuthorizeCloudAccessKey = "AuthorizeCloudAccess"
	ConfigSingleNodeEnabledKey    = "SingleNodeEnabled"
	OldConfigFileName             = ".metal-cli.json"
	OldMetricsConfigFileName      = ".metal-cli/config"
	DefaultConfigFileName         = ".metal-cli/config.json"
	DefaultNodeType               = "default"
	AWSCloudService               = "Amazon Web Services"
	GCPCloudService               = "Google Cloud Platform"
	AWSDefaultInstanceType        = "c5.2xlarge"
	GCPDefaultInstanceType        = "e2-standard-8"
	AnsibleSSHUser                = "ubuntu"
	AWSNodeAnsiblePrefix          = "aws_node"
	GCPNodeAnsiblePrefix          = "gcp_node"
	CustomVMDir                   = "vms"
	ClusterYAMLFileName           = "clusterInfo.yaml"
	GCPStaticIPPrefix             = "static-ip"
	AvaLabsOrg                    = "MetalBlockchain"
	AvalancheGoRepoName           = "metalgo"
	SubnetEVMRepoName             = "subnet-evm"
	CliRepoName                   = "metal-cli"
	TeleporterRepoName            = "teleporter"
	AWMRelayerRepoName            = "awm-relayer"
	SubnetEVMReleaseURL           = "https://github.com/shubhamdubey02/subnet-evm/releases/download/%s/%s"
	SubnetEVMArchive              = "subnet-evm_%s_linux_amd64.tar.gz"
	CloudNodeConfigBasePath       = "/home/ubuntu/.metalgo/"
	CloudNodeSubnetEvmBinaryPath  = "/home/ubuntu/.metalgo/plugins/%s"
	CloudNodeStakingPath          = "/home/ubuntu/.metalgo/staking/"
	CloudNodeConfigPath           = "/home/ubuntu/.metalgo/configs/"
	CloudNodePrometheusConfigPath = "/etc/prometheus/prometheus.yml"
	CloudNodeCLIConfigBasePath    = "/home/ubuntu/.metal-cli/"
	AvalanchegoMonitoringPort     = 9090
	AvalanchegoMachineMetricsPort = 9100
	MonitoringDir                 = "monitoring"
	LoadTestDir                   = "loadtest"
	DashboardsDir                 = "dashboards"
	NodeConfigJSONFile            = "node.json"
	IPAddressSuffix               = "/32"
	AvalancheGoInstallDir         = "metalgo"
	SubnetEVMInstallDir           = "subnet-evm"
	AWMRelayerInstallDir          = "awm-relayer"
	TeleporterInstallDir          = "teleporter"
	AWMRelayerBin                 = "awm-relayer"
	AWMRelayerConfigFilename      = "awm-relayer-config.json"
	AWMRelayerStorageDir          = "awm-relayer-storage"
	AWMRelayerLogFilename         = "awm-relayer.log"
	AWMRelayerRunFilename         = "awm-relayer-process.json"

	AWMRelayerSnapshotConfsDir = "relayer-confs"

	TeleporterKeyName = "cli-teleporter-deployer"
	AWMRelayerKeyName = "cli-awm-relayer"

	AWMRelayerMetricsPort = 9091

	SubnetEVMBin = "subnet-evm"

	DefaultNodeRunURL = "http://127.0.0.1:9650"

	APMDir                = ".apm"
	APMLogName            = "apm.log"
	DefaultAvaLabsPackage = "MetalBlockchain/metal-plugins-core"
	APMPluginDir          = "apm_plugins"

	// #nosec G101
	GithubAPITokenEnvVarName = "METAL_CLI_GITHUB_TOKEN"

	ReposDir                   = "repos"
	SubnetDir                  = "subnets"
	NodesDir                   = "nodes"
	VMDir                      = "vms"
	ChainConfigDir             = "chains"
	AVMKeyName                 = "avm"
	EVMKeyName                 = "evm"
	PlatformKeyName            = "platform"
	SubnetType                 = "subnet type"
	PrecompileType             = "precompile type"
	CustomAirdrop              = "custom-airdrop"
	NumberOfAirdrops           = "airdrop-addresses"
	SubnetConfigFileName       = "subnet.json"
	ChainConfigFileName        = "chain.json"
	PerNodeChainConfigFileName = "per-node-chain.json"
	NodeConfigFileName         = "node-config.json"

	GitRepoCommitName  = "Avalanche-CLI"
	GitRepoCommitEmail = "info@avax.network"
	AvaLabsMaintainers = "ava-labs"

	UpgradeBytesFileName      = "upgrade.json"
	UpgradeBytesLockExtension = ".lock"
	NotAvailableLabel         = "Not available"
	BackendCmd                = "avalanche-cli-backend"

	AvalancheGoVersionUnknown            = "n/a"
	AvalancheGoCompatibilityVersionAdded = "v1.9.2"
	AvalancheGoCompatibilityURL          = "https://raw.githubusercontent.com/MetalBlockchain/metalgo/master/version/compatibility.json"
	SubnetEVMRPCCompatibilityURL         = "https://raw.githubusercontent.com/shubhamdubey02/subnet-evm/master/compatibility.json"

	YesLabel = "Yes"
	NoLabel  = "No"

	SubnetIDLabel     = "SubnetID: "
	BlockchainIDLabel = "BlockchainID: "

	PluginDir = "plugins"

	Network                      = "network"
	MultiSig                     = "multi-sig"
	SkipUpdateFlag               = "skip-update-check"
	LastFileName                 = ".last_actions.json"
	APIRole                      = "API"
	ValidatorRole                = "Validator"
	MonitorRole                  = "Monitor"
	AWMRelayerRole               = "Relayer"
	LoadTestRole                 = "LoadTest"
	DefaultWalletCreationTimeout = 5 * time.Second

	DefaultConfirmTxTimeout = 20 * time.Second

	PayTxsFeesMsg = "pay transaction fees"

	CodespaceNameEnvVar = "CODESPACE_NAME"

	// E2E
	E2ENetworkPrefix        = "172.18.0"
	E2EClusterName          = "e2e"
	E2EDocker               = "docker"
	E2EDockerComposeFile    = "/tmp/avalanche-cli-docker-compose.yml"
	E2EDebugAvalanchegoPath = "E2E_AVALANCHEGO_PATH"
	GitExtension            = ".git"
)
