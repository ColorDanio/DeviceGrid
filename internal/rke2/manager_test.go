package rke2

import (
	"strings"
	"testing"
)

func TestBuildInstallScriptUsesConfiguredMirror(t *testing.T) {
	manager := &Manager{}
	script := manager.buildInstallScript("server", "token: test\n", ClusterConfig{
		InstallMirror: "https://mirror.example.test",
		Version:       "v1.31.5+rke2r1",
	})

	for _, want := range []string{
		"curl -sfL https://mirror.example.test/rke2/install.sh",
		"INSTALL_RKE2_VERSION=v1.31.5+rke2r1 INSTALL_RKE2_TYPE=server sh -",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}
