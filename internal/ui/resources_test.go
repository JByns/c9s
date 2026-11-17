package ui

import (
	"testing"
	"time"

	"github.com/JByns/c9s/internal/rpc"
)

func TestScopeMatchNode(t *testing.T) {
	s := Scope{ClusterID: 35, NodeHost: "ct-jessie-node4", NodeIP: "10.10.16.8"}
	for v, want := range map[string]bool{
		"ct-jessie-node4":       true,
		"10.10.16.8":            true,
		"10.10.16.8:1433":       true,
		"10.10.16.80":           false,
		"ct-jessie-node5":       false,
		"ct-jessie-node4x:1433": false,
	} {
		if got := s.matchNode(v); got != want {
			t.Errorf("matchNode(%q) = %v, want %v", v, got, want)
		}
	}
	if !(Scope{}).matchNode("anything") {
		t.Error("an unscoped node filter must match everything")
	}
}

func TestHumanAge(t *testing.T) {
	ts := time.Now().Add(-(6*24*time.Hour + 8*time.Hour)).UTC().Format("2006-01-02T15:04:05.000Z")
	if got := humanAge(ts); got != "6d8h" {
		t.Errorf("humanAge(rfc3339) = %q", got)
	}
	epoch := time.Now().Add(-21 * time.Minute).Unix()
	if got := humanAge(rpc.Str(rpc.Map{"x": float64(epoch)}, "x")); got != "21m" {
		t.Errorf("humanAge(epoch) = %q", got)
	}
}

func TestDefaultBackupMethod(t *testing.T) {
	cases := []struct{ typ, vendor, want string }{
		{"GALERA", "mariadb", "mariabackupfull"},
		{"REPLICATION", "percona", "xtrabackupfull"},
		{"POSTGRESQL_SINGLE", "postgresql", "pg_basebackup"},
		{"MONGODB", "10gen", "mongodump"},
		{"MSSQL_AO_ASYNC", "microsoft", "mssqlfull"},
		{"REDIS_SHARDED", "redis", ""},
	}
	for _, c := range cases {
		if got := defaultBackupMethod(c.typ, c.vendor); got != c.want {
			t.Errorf("%s/%s: got %q want %q", c.typ, c.vendor, got, c.want)
		}
	}
}

func TestFindResourceAliases(t *testing.T) {
	for alias, want := range map[string]string{"cl": "clusters", "no": "nodes", "maint": "maintenance", "top": "top", "qu": "queries"} {
		if r := FindResource(alias); r == nil || r.Name != want {
			t.Errorf("FindResource(%q) wrong", alias)
		}
	}
}
