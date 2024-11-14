// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"github.com/MetalBlockchain/metal-network-runner/utils"
	"github.com/MetalBlockchain/metalgo/ids"
)

type NetworkData struct {
	SubnetID                    ids.ID
	TransferSubnetOwnershipTxID ids.ID
	BlockchainID                ids.ID
	RPCVersion                  int
	TeleporterMessengerAddress  string
	TeleporterRegistryAddress   string
}

type PermissionlessValidators struct {
	TxID ids.ID
}
type ElasticSubnet struct {
	SubnetID    ids.ID
	AssetID     ids.ID
	PChainTXID  ids.ID
	TokenName   string
	TokenSymbol string
	Validators  map[string]PermissionlessValidators
	Txs         map[string]ids.ID
}

type Sidecar struct {
	Name                string
	VM                  VMType
	VMVersion           string
	RPCVersion          int
	Subnet              string
	TokenName           string
	TokenSymbol         string
	ChainID             string
	Version             string
	Networks            map[string]NetworkData
	ElasticSubnet       map[string]ElasticSubnet
	ImportedFromAPM     bool
	ImportedVMID        string
	CustomVMRepoURL     string
	CustomVMBranch      string
	CustomVMBuildScript string
	// Teleporter related
	TeleporterReady   bool
	TeleporterKey     string
	TeleporterVersion string
	RunRelayer        bool
	// SubnetEVM based VM's only
	SubnetEVMMainnetChainID uint
}

func (sc Sidecar) GetVMID() (string, error) {
	// get vmid
	var vmid string
	if sc.ImportedFromAPM {
		vmid = sc.ImportedVMID
	} else {
		chainVMID, err := utils.VMID(sc.Name)
		if err != nil {
			return "", err
		}
		vmid = chainVMID.String()
	}
	return vmid, nil
}