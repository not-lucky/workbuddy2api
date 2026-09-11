package pool

import (
	"testing"
	"time"

	"workbuddy2api/internal/auth"
)

func TestPickHealthyByRegion(t *testing.T) {
	p := New("")
	p.Add(&auth.Auth{UID: "u-cn", Domain: "www.codebuddy.cn"})
	p.Add(&auth.Auth{UID: "u-global", Domain: "www.workbuddy.ai"})
	if got := p.PickHealthyByRegion(auth.RegionCN); got == nil || got.UID != "u-cn" {
		t.Errorf("cn=%+v want u-cn", got)
	}
	if got := p.PickHealthyByRegion(auth.RegionGlobal); got == nil || got.UID != "u-global" {
		t.Errorf("global=%+v want u-global", got)
	}
}

func TestPickHealthyByRegionSkipsCooling(t *testing.T) {
	p := New("")
	p.Add(&auth.Auth{UID: "u-cn", Domain: ""})
	p.Add(&auth.Auth{UID: "u-global", Domain: "www.workbuddy.ai"})
	p.Cooldown("u-global", CoolSoft, time.Hour, "test")
	if got := p.PickHealthyByRegion(auth.RegionGlobal); got != nil {
		t.Errorf("global=%+v want nil (cooling)", got)
	}
	if got := p.PickHealthyByRegion(auth.RegionCN); got == nil {
		t.Error("cn must still be available")
	}
}

func TestPickHealthyByRegionDeterministic(t *testing.T) {
	p := New("")
	p.Add(&auth.Auth{UID: "u-b", Domain: "www.workbuddy.ai"})
	p.Add(&auth.Auth{UID: "u-a", Domain: "www.workbuddy.ai"})
	for i := 0; i < 10; i++ {
		if got := p.PickHealthyByRegion(auth.RegionGlobal); got == nil || got.UID != "u-a" {
			t.Fatalf("got=%+v want deterministic u-a", got)
		}
	}
}

func TestHasRegionAccounts(t *testing.T) {
	p := New("")
	if p.HasRegionAccounts(auth.RegionCN) || p.HasRegionAccounts(auth.RegionGlobal) {
		t.Fatal("empty pool must have no regions")
	}
	p.Add(&auth.Auth{UID: "u1", Domain: "www.workbuddy.ai"})
	if !p.HasRegionAccounts(auth.RegionGlobal) || p.HasRegionAccounts(auth.RegionCN) {
		t.Error("only global present")
	}
	p.Disable("u1", "test")
	if p.HasRegionAccounts(auth.RegionGlobal) {
		t.Error("disabled-only realm must not count")
	}
}
