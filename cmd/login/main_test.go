package main

import (
	"path/filepath"
	"strings"
	"testing"

	"workbuddy2api/internal/upstream"
)

func TestParseRegion(t *testing.T) {
	if got, err := parseRegion(""); err != nil || got != "cn" {
		t.Errorf("empty must default cn: %q %v", got, err)
	}
	if got, err := parseRegion("cn"); err != nil || got != "cn" {
		t.Errorf("cn: %q %v", got, err)
	}
	if got, err := parseRegion("global"); err != nil || got != "global" {
		t.Errorf("global: %q %v", got, err)
	}
	if _, err := parseRegion("eu"); err == nil {
		t.Error("invalid region must fail (silent misroute)")
	}
}

func TestBases(t *testing.T) {
	base, origin, _ := bases("cn")
	if base != upstream.DefaultChatBaseCN || origin != "https://www.codebuddy.cn" {
		t.Errorf("cn bases=%q %q", base, origin)
	}
	base, origin, _ = bases("global")
	if base != upstream.DefaultChatBaseGlobal || origin != upstream.DefaultBillingBaseGlobal {
		t.Errorf("global bases=%q %q", base, origin)
	}
}

func TestStateFilesArePerRegion(t *testing.T) {
	_, _, cnFile := bases("cn")
	_, _, gFile := bases("global")
	if cnFile == gFile {
		t.Error("cn/global must use distinct state files (concurrent logins)")
	}
	if !strings.Contains(cnFile, "cn") || !strings.Contains(gFile, "global") {
		t.Errorf("state files must be region-tagged: %q %q", cnFile, gFile)
	}
	if filepath.IsAbs(cnFile) != true {
		t.Errorf("state file must be absolute (TempDir-based): %q", cnFile)
	}
}

func TestStateDirEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WB2A_STATE_DIR", dir)
	_, _, f := bases("cn")
	if filepath.Dir(f) != dir {
		t.Errorf("WB2A_STATE_DIR must relocate state file: %q", f)
	}
}
