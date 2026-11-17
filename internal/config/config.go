// Package config works out how c9s reaches the cmon service on the
// ClusterControl controller it runs on.
//
// c9s is meant to be started on the controller itself. It connects only to
// the local cmon: the controller address comes from the ClusterControl client
// config (~/.s9s/s9s.conf, then /etc/s9s.conf) or defaults to
// https://127.0.0.1:9501, and it must point at this machine.
//
// Login uses the user's RSA key (~/.s9s/<user>.key) when it exists, so no
// password is needed.
package config

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// DefaultController is cmon's RPC v2 address on the controller.
const DefaultController = "https://127.0.0.1:9501"

// Local describes the connection to this controller's cmon.
type Local struct {
	Controller string // RPC v2 URL, always on this machine
	User       string // ClusterControl user, "" if not known
	Key        string // RSA key path, "" for password login
	Host       string // short host name of this controller, shown in the UI
	Source     string // client config file used, "" if none
}

// Detect builds the local connection. user and key override the client config.
func Detect(user, key string) (*Local, error) {
	l := &Local{Controller: DefaultController, Host: shortHostname()}
	if kv, src := clientConfig(); kv != nil {
		l.Source = src
		l.User = kv["cmon_user"]
		switch {
		case kv["controller"] != "":
			l.Controller = kv["controller"]
		case kv["controller_host_name"] != "":
			port := kv["controller_port"]
			if port == "" {
				port = "9501"
			}
			l.Controller = "https://" + kv["controller_host_name"] + ":" + port
		}
	}
	// C9S_CONTROLLER is for development (the mock controller); it must still
	// be on this machine.
	if v := os.Getenv("C9S_CONTROLLER"); v != "" {
		l.Controller = v
	}
	if !strings.Contains(l.Controller, "://") {
		l.Controller = "https://" + l.Controller
	}
	if !IsLocal(l.Controller) {
		return nil, fmt.Errorf("controller %s is not this machine: c9s runs on the ClusterControl controller (ssh to it and run c9s there)", l.Controller)
	}
	if user != "" {
		l.User = user
	}
	l.Key = keyPath(l.User, key)
	return l, nil
}

// IsLocal reports whether the URL's host is this machine: a loopback
// address, this host's name, or one of its interface addresses.
func IsLocal(controller string) bool {
	u, err := url.Parse(controller)
	if err != nil {
		return false
	}
	h := u.Hostname()
	if h == "" || h == "localhost" {
		return true
	}
	if hn, err := os.Hostname(); err == nil && (strings.EqualFold(h, hn) || strings.EqualFold(h, strings.Split(hn, ".")[0])) {
		return true
	}
	ips := []net.IP{net.ParseIP(h)}
	if ips[0] == nil {
		ips = nil
		addrs, _ := net.LookupHost(h)
		for _, a := range addrs {
			ips = append(ips, net.ParseIP(a))
		}
	}
	local := map[string]bool{}
	if ifs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range ifs {
			if n, ok := a.(*net.IPNet); ok {
				local[n.IP.String()] = true
			}
		}
	}
	for _, ip := range ips {
		if ip != nil && (ip.IsLoopback() || local[ip.String()]) {
			return true
		}
	}
	return false
}

// keyPath returns the RSA key to log in with, or "" to use a password.
func keyPath(user, key string) string {
	if key != "" {
		return expand(key)
	}
	if user == "" {
		return ""
	}
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".s9s", user+".key")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// clientConfig reads the first ClusterControl client config file that exists.
func clientConfig() (map[string]string, string) {
	home, _ := os.UserHomeDir()
	for _, p := range []string{filepath.Join(home, ".s9s", "s9s.conf"), "/etc/s9s.conf"} {
		if kv, err := readIni(p); err == nil {
			return kv, p
		}
	}
	return nil, ""
}

func shortHostname() string {
	hn, err := os.Hostname()
	if err != nil || hn == "" {
		return "cc"
	}
	return strings.Split(hn, ".")[0]
}

func expand(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// readIni reads key = value lines (sections are ignored, comments skipped).
func readIni(path string) (map[string]string, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	kv := map[string]string{}
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' || line[0] == ';' || line[0] == '[' {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			kv[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return kv, sc.Err()
}
