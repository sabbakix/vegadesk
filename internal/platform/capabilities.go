// Package platform probes what management subsystems are actually present on
// this host. This is the single mechanism that keeps vegadesk distro-agnostic:
// features check Capabilities and render "not available on this host" instead
// of assuming systemd/docker/ufw/NetworkManager everywhere.
package platform

import "os"

// InitSystem identifies the running init system, so far as it matters for
// service management.
type InitSystem string

const (
	InitSystemd InitSystem = "systemd"
	InitOther   InitSystem = "other" // sysvinit, OpenRC, runit, ... not yet supported
)

// FirewallTool identifies which firewall frontend is available, preferring
// the friendliest one when more than one is installed.
type FirewallTool string

const (
	FirewallUFW       FirewallTool = "ufw"
	FirewallFirewalld FirewallTool = "firewalld"
	FirewallNftables  FirewallTool = "nftables"
	FirewallIptables  FirewallTool = "iptables"
	FirewallNone      FirewallTool = "none"
)

// NetManager identifies which network configuration layer is available.
type NetManager string

const (
	NetManagerNM  NetManager = "networkmanager"
	NetManagerRaw NetManager = "iproute" // plain `ip`, read/basic-config only
)

// Capabilities is a point-in-time snapshot of what this host can do. It is
// resolved once at startup; none of these facts change while vegadesk runs.
type Capabilities struct {
	Init InitSystem

	HasDocker bool
	HasPodman bool

	HasLibvirt bool // virsh on PATH (implies a libvirt client is usable)

	Firewall FirewallTool

	NetManager  NetManager
	HasNmcli    bool
	HasJournald bool // journalctl present -> structured log browsing available
}

// LookPath is overridable in tests; production code uses execx.LookPath but
// platform must not import execx's exec-running half just to check PATH, so
// it takes the probe function as a parameter to Detect instead.
type PathChecker func(name string) bool

// Detect probes the host. checker is normally execx.LookPath; passed in so
// this package stays trivially unit-testable without shelling out.
func Detect(checker PathChecker) Capabilities {
	c := Capabilities{}

	if _, err := os.Stat("/run/systemd/system"); err == nil && checker("systemctl") {
		c.Init = InitSystemd
	} else {
		c.Init = InitOther
	}

	c.HasDocker = checker("docker")
	c.HasPodman = checker("podman")
	c.HasLibvirt = checker("virsh")

	switch {
	case checker("ufw"):
		c.Firewall = FirewallUFW
	case checker("firewall-cmd"):
		c.Firewall = FirewallFirewalld
	case checker("nft"):
		c.Firewall = FirewallNftables
	case checker("iptables"):
		c.Firewall = FirewallIptables
	default:
		c.Firewall = FirewallNone
	}

	c.HasNmcli = checker("nmcli")
	if c.HasNmcli {
		c.NetManager = NetManagerNM
	} else {
		c.NetManager = NetManagerRaw
	}

	c.HasJournald = checker("journalctl")

	return c
}

// ContainerRuntimeName returns the preferred runtime name for display and
// for internal/collectors/containers to pick an implementation, or ""
// when neither is present.
func (c Capabilities) ContainerRuntimeName() string {
	switch {
	case c.HasDocker:
		return "docker"
	case c.HasPodman:
		return "podman"
	default:
		return ""
	}
}
