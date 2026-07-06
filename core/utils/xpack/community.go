//go:build !xpack && !enterprise

package xpack

import "github.com/2Panel-dev/2Panel/core/utils/xpack/helper"

var AuthProvider = helper.NewIAuthProvider()

var MultiNodeProvider = helper.NewIMultiNodeProvider()
