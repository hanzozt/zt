package getzt

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	c "github.com/hanzozt/zt/v2/zt/constants"
)

func InstallZitiEdgeTunnel(targetVersion, targetOS, targetArch, binDir string, verbose bool) error {
	var newVersion semver.Version

	if targetVersion != "" {
		newVersion = semver.MustParse(strings.TrimPrefix(targetVersion, "v"))
	} else {
		v, err := GetLatestGitHubReleaseVersion(c.HanzoZTOrg, c.ZT_EDGE_TUNNEL_GITHUB, verbose)
		if err != nil {
			return err
		}
		newVersion = v
	}

	fmt.Println("Attempting to install '" + c.ZT_EDGE_TUNNEL + "' version: " + newVersion.String())
	return FindVersionAndInstallGitHubRelease(
		c.HanzoZTOrg, c.ZT_EDGE_TUNNEL, c.ZT_EDGE_TUNNEL_GITHUB, targetOS, targetArch, binDir, newVersion.String(), verbose)
}
