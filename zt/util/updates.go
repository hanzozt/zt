/*
	Copyright NetFoundry Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package util

import (
	"fmt"
	"os"
	"strings"

	"github.com/hanzozt/zt/v2/common/getzt"
	"github.com/hanzozt/zt/v2/zt/constants"

	"github.com/fatih/color"

	"github.com/blang/semver"
	"github.com/hanzozt/zt/v2/common/version"
	"github.com/michaelquigley/pfxlog"
)

func LogReleaseVersionCheck() {
	logger := pfxlog.Logger()
	if strings.ToLower(os.Getenv("ZT_CHECK_VERSION")) == "true" {
		logger.Debug("ZT_CHECK_VERSION is true. starting version check")
		developmentSemver, _ := semver.Parse("0.0.0")
		latestGithubRelease, err := getzt.GetHighestVersionGitHubReleaseInfo(constants.HanzoZTOrg, constants.ZT, false)
		if err != nil {
			logger.Debugf("failed to find latest GitHub version with error: %s", err)
			return // soft-fail version check if GitHub API is unavailable
		}
		// compose current build's semver as version string and semver object
		currentBuildVersion := version.GetVersion()
		currentBuildSemver, err := semver.ParseTolerant(currentBuildVersion) // ParseTolerant trims leading "v"
		if err != nil {
			logger.Warnf("failed to parse current build version as semver: '%s' with error: %s", version.GetVersion(), err)
			return
		}
		// ignore non-release builds and current release build
		if currentBuildSemver.EQ(developmentSemver) {
			logger.Debugf(
				"this build of zt is unreleased v%s",
				developmentSemver,
			)
		} else if latestGithubRelease.SemVer.GT(currentBuildSemver) {
			yellow := color.New(color.FgYellow).SprintFunc()
			green := color.New(color.FgGreen).SprintFunc()
			fmt.Fprintf(os.Stderr,
				`
*********************************************************************************

An updated version of Hanzo ZT (%s), is available to replace the running version (%s) from 
https://github.com/hanzozt/%s/releases/latest/

*********************************************************************************
`,
				green("v"+latestGithubRelease.SemVer.String()),
				yellow("v"+currentBuildSemver.String()),
				constants.ZT,
			)
			logger.Debugf(
				"this v%s build of Hanzo ZT is superseded by v%s",
				currentBuildSemver,
				latestGithubRelease,
			)
		} else if latestGithubRelease.SemVer.EQ(currentBuildSemver) {
			logger.Debugf(
				"this build of Hanzo ZT is the latest release v%s",
				currentBuildSemver,
			)
		}
	} else {
		logger.Debug("ZT_CHECK_VERSION is not 'true'. skipping version check")
	}
}
