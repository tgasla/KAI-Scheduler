/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package feature_flags

import (
	"context"
	"fmt"
	"strings"

	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testContext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
)

func SetRestrictNodeScheduling(
	value *bool, testCtx *testContext.TestContext, ctx context.Context,
) error {
	return PatchSystemDeploymentFeatureFlags(
		ctx,
		testCtx,
		constant.SystemPodsNamespace,
		constant.SchedulerDeploymentName,
		constant.SchedulerContainerName,
		func(args []string) []string {
			if value != nil {
				return append(args, fmt.Sprintf("--restrict-node-scheduling=%t", *value))
			}
			for i, arg := range args {
				if strings.HasPrefix(arg, "--restrict-node-scheduling=") {
					return append(args[:i], args[i+1:]...)
				}
			}
			return args
		},
	)
}
