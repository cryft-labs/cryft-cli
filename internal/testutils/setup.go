// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package testutils

import (
	"io"
	"testing"

	"github.com/cryft-labs/cryft-cli/pkg/application"
	"github.com/cryft-labs/cryft-cli/pkg/config"
	"github.com/cryft-labs/cryft-cli/pkg/ux"
	"github.com/MetalBlockchain/metalgo/utils/logging"
	"github.com/stretchr/testify/require"
)

func SetupTest(t *testing.T) *require.Assertions {
	// use io.Discard to not print anything
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	return require.New(t)
}

func SetupTestInTempDir(t *testing.T) *application.Avalanche {
	testDir := t.TempDir()

	app := application.New()
	app.Setup(testDir, logging.NoLog{}, &config.Config{}, nil, nil)
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	return app
}
