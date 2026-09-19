package platform

import "testing"

func TestDetectContainerRuntimePreference(t *testing.T) {
	checker := func(name string) bool {
		return name == "docker" || name == "podman" || name == "systemctl"
	}
	c := Detect(checker)
	if got := c.ContainerRuntimeName(); got != "docker" {
		t.Errorf("with both present, want docker preferred, got %q", got)
	}
}

func TestDetectFirewallPreference(t *testing.T) {
	checker := func(name string) bool {
		return name == "firewall-cmd" || name == "nft" || name == "iptables"
	}
	c := Detect(checker)
	if c.Firewall != FirewallFirewalld {
		t.Errorf("Firewall = %v, want firewalld preferred over nft/iptables", c.Firewall)
	}
}

func TestDetectNoneAvailable(t *testing.T) {
	c := Detect(func(string) bool { return false })
	if c.ContainerRuntimeName() != "" {
		t.Errorf("expected no container runtime, got %q", c.ContainerRuntimeName())
	}
	if c.Firewall != FirewallNone {
		t.Errorf("Firewall = %v, want none", c.Firewall)
	}
	if c.NetManager != NetManagerRaw {
		t.Errorf("NetManager = %v, want iproute fallback", c.NetManager)
	}
}
