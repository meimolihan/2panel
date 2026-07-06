//go:build !xpack && !enterprise

package xpack

import "github.com/2Panel-dev/2Panel/agent/utils/xpack/helper"

var MultiNodeProvider = helper.NewIMultiNodeProvider()
