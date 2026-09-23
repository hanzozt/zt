package getzt

import (
	"fmt"

	c "github.com/hanzozt/zt/v2/zt/constants"
)

func InstallZiti(targetVersion, targetOS, targetArch, binDir string, verbose bool) error {
	fmt.Println("Attempting to install '" + c.ZT + "' version: " + targetVersion)
	return FindVersionAndInstallGitHubRelease(
		c.HanzoZTOrg, c.ZT, c.ZT, targetOS, targetArch, binDir, targetVersion, verbose)
}

func InstallZrok(targetVersion, targetOS, targetArch, binDir string, verbose bool) error {
	fmt.Println("Attempting to install '" + c.ZROK + "' version: " + targetVersion)
	return FindVersionAndInstallGitHubRelease(
		c.HanzoZTOrg, c.ZROK, c.ZROK, targetOS, targetArch, binDir, targetVersion, verbose)
}

func InstallCaddy(targetVersion, targetOS, targetArch, binDir string, verbose bool) error {
	fmt.Println("Attempting to install '" + c.Caddy + "' version: " + targetVersion)
	return FindVersionAndInstallGitHubRelease(
		c.CaddyOrg, c.Caddy, c.Caddy, targetOS, targetArch, binDir, targetVersion, verbose)
}
