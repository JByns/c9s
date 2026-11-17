// c9s is a terminal user interface client for ClusterControl. It runs on
// the ClusterControl controller and talks to the local cmon over RPC v2.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/JByns/c9s/internal/config"
	"github.com/JByns/c9s/internal/rpc"
	"github.com/JByns/c9s/internal/ui"
)

var version = "dev"

const usage = `c9s - terminal UI for ClusterControl

Run it on the ClusterControl controller:

  c9s [flags]                 open the UI
  c9s rpc <path> <op> [json]  send one raw RPC v2 call to cmon and print the reply
  c9s version

c9s connects to the cmon service on this machine. It takes the user from the
ClusterControl client config (~/.s9s/s9s.conf, then /etc/s9s.conf) and logs
in with the user's RSA key ~/.s9s/<user>.key. Without a key it asks for the
password (or reads $CC_PASSWORD).

Flags:
`

func main() {
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage); flag.PrintDefaults() }
	user := flag.String("user", "", "ClusterControl user (default: cmon_user from the client config)")
	key := flag.String("key", "", "RSA private key for login (default: ~/.s9s/<user>.key)")
	refresh := flag.Duration("refresh", 5*time.Second, "refresh interval")
	write := flag.Bool("write", false, "allow actions that change clusters")
	flag.Parse()

	if flag.Arg(0) == "version" {
		fmt.Println("c9s", version)
		return
	}

	local, err := config.Detect(*user, *key)
	if err != nil {
		fail(err)
	}
	client, err := connect(local)
	if err != nil {
		fail(err)
	}

	if flag.Arg(0) == "rpc" {
		rawCall(client, flag.Args()[1:])
		return
	}
	if flag.NArg() > 0 {
		fail(fmt.Errorf("unknown command %q (run c9s -h)", flag.Arg(0)))
	}

	ensureUTF8()
	app := ui.New(client, ui.Options{
		Refresh: *refresh, AllowWrite: *write, Version: version, Host: local.Host,
	})
	if err := app.Run(); err != nil {
		fail(err)
	}
}

// connect logs in to the local cmon with the user's key if there is one,
// else with a password.
func connect(l *config.Local) (*rpc.Client, error) {
	if l.User == "" {
		l.User = ask("ClusterControl user: ")
		if l.Key == "" {
			if l2, err := config.Detect(l.User, ""); err == nil {
				l.Key = l2.Key
			}
		}
	}
	password := os.Getenv("CC_PASSWORD")
	if l.Key == "" && password == "" {
		fmt.Printf("Password for %s: ", l.User)
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return nil, err
		}
		password = string(b)
	}
	client := rpc.New(l.Controller, l.User, password, true)
	var err error
	if l.Key != "" {
		err = client.LoginWithKey(l.Key)
	} else {
		err = client.Login()
	}
	if err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) {
			return nil, fmt.Errorf("cannot reach cmon at %s: is this the ClusterControl controller, and is cmon running? (systemctl status cmon)", l.Controller)
		}
		return nil, fmt.Errorf("login as %s failed: %w", l.User, err)
	}
	return client, nil
}

func rawCall(c *rpc.Client, args []string) {
	if len(args) < 2 {
		fail(fmt.Errorf("usage: c9s rpc <path> <operation> [json-params]"))
	}
	params := rpc.Map{}
	if len(args) > 2 {
		if err := json.Unmarshal([]byte(args[2]), &params); err != nil {
			fail(err)
		}
	}
	r, err := c.Call(args[0], args[1], params)
	out, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(out))
	if err != nil {
		fail(err)
	}
}

// ensureUTF8 makes the terminal library draw box lines and symbols. Over
// SSH the locale is often unset (POSIX), which makes it print "?" instead.
// Set C9S_ASCII=1 to keep the locale as it is.
func ensureUTF8() {
	if os.Getenv("C9S_ASCII") != "" {
		return
	}
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(k); v != "" {
			if u := strings.ToUpper(v); strings.Contains(u, "UTF-8") || strings.Contains(u, "UTF8") {
				return
			}
			break
		}
	}
	os.Setenv("LC_ALL", "C.UTF-8")
}

func ask(prompt string) string {
	fmt.Print(prompt)
	s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(s)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "c9s:", err)
	os.Exit(1)
}
