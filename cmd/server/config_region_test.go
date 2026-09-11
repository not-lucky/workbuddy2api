package main

import (
	"os"
	"path/filepath"
	"testing"

	"workbuddy2api/internal/upstream"
)

func TestUpstreamBasesDefault(t *testing.T) {
	c := Default()
	if err := c.normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if c.Upstream.ChatBaseCN != upstream.DefaultChatBaseCN {
		t.Errorf("chat_base_cn=%q want %q", c.Upstream.ChatBaseCN, upstream.DefaultChatBaseCN)
	}
	if c.Upstream.BillingBaseCN != upstream.DefaultBillingBaseCN {
		t.Errorf("billing_base_cn=%q want %q", c.Upstream.BillingBaseCN, upstream.DefaultBillingBaseCN)
	}
	if c.Upstream.ChatBaseGlobal != upstream.DefaultChatBaseGlobal {
		t.Errorf("chat_base_global=%q want %q", c.Upstream.ChatBaseGlobal, upstream.DefaultChatBaseGlobal)
	}
	if c.Upstream.BillingBaseGlobal != upstream.DefaultBillingBaseGlobal {
		t.Errorf("billing_base_global=%q want %q", c.Upstream.BillingBaseGlobal, upstream.DefaultBillingBaseGlobal)
	}
}

func TestUpstreamBasesFromFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "c.json")
	os.WriteFile(fp, []byte(`{"upstream":{"chat_base_global":"https://chat-g.example","billing_base_global":"https://bill-g.example"}}`), 0o600)
	c, err := Load(fp)
	if err != nil {
		t.Fatal(err)
	}
	if c.Upstream.ChatBaseGlobal != "https://chat-g.example" {
		t.Errorf("chat_base_global=%q", c.Upstream.ChatBaseGlobal)
	}
	if c.Upstream.BillingBaseGlobal != "https://bill-g.example" {
		t.Errorf("billing_base_global=%q", c.Upstream.BillingBaseGlobal)
	}
	if c.Upstream.ChatBaseCN != upstream.DefaultChatBaseCN {
		t.Errorf("chat_base_cn=%q want CN default (file must not clobber)", c.Upstream.ChatBaseCN)
	}
}

func TestUpstreamBasesEnvOverride(t *testing.T) {
	t.Setenv("WB2A_CHAT_BASE_GLOBAL", "https://env-chat-g.example")
	t.Setenv("WB2A_BILLING_BASE_CN", "https://env-bill-cn.example")
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Upstream.ChatBaseGlobal != "https://env-chat-g.example" {
		t.Errorf("chat_base_global=%q want env", c.Upstream.ChatBaseGlobal)
	}
	if c.Upstream.BillingBaseCN != "https://env-bill-cn.example" {
		t.Errorf("billing_base_cn=%q want env", c.Upstream.BillingBaseCN)
	}
}
