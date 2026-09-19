package main

import (
	"fmt"

	"vegadesk/internal/app"
	containerrt "vegadesk/internal/collectors/containers"
	containerfeature "vegadesk/internal/features/containers"
	firewallfeature "vegadesk/internal/features/firewall"
	logsfeature "vegadesk/internal/features/logs"
	"vegadesk/internal/features/networking"
	"vegadesk/internal/features/overview"
	"vegadesk/internal/features/processes"
	"vegadesk/internal/features/services"
	"vegadesk/internal/features/storage"
	usersfeature "vegadesk/internal/features/users"
	"vegadesk/internal/features/vms"
	"vegadesk/internal/platform"
	"vegadesk/internal/privilege"
	"vegadesk/internal/ui/theme"
)

// registerSections wires every planned section into the sidebar, gating
// availability on the platform capability probe. Sections not yet built in
// this phase show a Placeholder rather than being omitted, so the sidebar
// always reflects the full intended scope.
func registerSections(root *app.Model, caps platform.Capabilities, priv privilege.Status, styles theme.Styles) {
	root.AddSection("Overview", overview.New(styles), true, "")
	root.AddSection("Processes", processes.New(styles), true, "")

	root.AddSection("Storage", storage.New(styles, priv), true, "")

	servicesAvailable := caps.Init == platform.InitSystemd
	root.AddSection("Services", services.New(styles, priv), servicesAvailable,
		"no supported init system detected (systemd required in this build)")

	if rt := containerrt.Detect(caps.HasDocker, caps.HasPodman); rt != nil {
		m := containerfeature.New(styles, rt)
		root.AddSection(m.Title(), m, true, "")
	} else {
		root.AddSection("Containers", app.NewPlaceholder("Containers", "not built yet"), false,
			"no container runtime found (looked for docker, podman)")
	}

	root.AddSection("Virtual Machines", vms.New(styles), caps.HasLibvirt,
		"libvirt not found (virsh not on PATH)")

	root.AddSection("Networking", networking.New(styles), true, "")

	root.AddSection("Logs", logsfeature.New(styles), caps.HasJournald,
		"journald not found (journalctl not on PATH)")

	root.AddSection("Users", usersfeature.New(styles, priv), true, "")

	if caps.Firewall == platform.FirewallUFW {
		m := firewallfeature.New(styles, priv)
		root.AddSection(m.Title(), m, true, "")
	} else {
		reason := "no supported firewall frontend found (looked for ufw, firewalld, nft, iptables)"
		if caps.Firewall != platform.FirewallNone {
			reason = fmt.Sprintf("detected %s, but only ufw is supported in this build", caps.Firewall)
		}
		root.AddSection("Firewall", app.NewPlaceholder("Firewall", "not built yet"), false, reason)
	}
}
