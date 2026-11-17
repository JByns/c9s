// Package rpc is a minimal client for the ClusterControl controller (cmon)
// RPC v2 JSON API.
//
// Every call is an HTTP POST of a JSON object with an "operation" key to a
// path like /v2/clusters/. Authentication returns a "cmon-sid" session cookie
// that the cookie jar sends back on every later request.
package rpc

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"sync"
	"time"
)

// Map is a decoded JSON object.
type Map = map[string]any

// Client talks to one cmon controller.
type Client struct {
	BaseURL  string
	User     string
	password string
	keyPath  string

	http *http.Client
	mu   sync.Mutex
}

// New builds a client. insecure skips TLS verification, which a default
// controller install needs because cmon ships a self-signed certificate.
func New(baseURL, user, password string, insecure bool) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		User:     user,
		password: password,
		http: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure}, //nolint:gosec
			},
		},
	}
}

// Login authenticates with user name and password (operation
// authenticateWithPassword on /v2/auth).
func (c *Client) Login() error {
	_, err := c.post("/v2/auth", Map{
		"operation": "authenticateWithPassword",
		"user_name": c.User,
		"password":  c.password,
	})
	return err
}

// LoginWithKey authenticates with an RSA key: the controller
// sends a challenge and the client signs it with the user's RSA private key
// (RSA-SHA256, PKCS#1 v1.5, base64). No password is needed.
func (c *Client) LoginWithKey(keyPath string) error {
	pemData, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(pemData)
	if block == nil {
		return fmt.Errorf("%s: not a PEM key", keyPath)
	}
	var key *rsa.PrivateKey
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = k
	} else if k8, err8 := x509.ParsePKCS8PrivateKey(block.Bytes); err8 == nil {
		if rk, ok := k8.(*rsa.PrivateKey); ok {
			key = rk
		}
	}
	if key == nil {
		return fmt.Errorf("%s: not an RSA private key", keyPath)
	}

	reply, err := c.post("/v2/auth", Map{"operation": "authenticate", "username": c.User})
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(Str(reply, "challenge")))
	sig, err := rsa.SignPKCS1v15(nil, key, crypto.SHA256, digest[:])
	if err != nil {
		return err
	}
	_, err = c.post("/v2/auth", Map{
		"operation": "authenticateResponse",
		"signature": base64.StdEncoding.EncodeToString(sig),
	})
	if err == nil {
		c.keyPath = keyPath
	}
	return err
}

// relogin repeats whichever login method worked first.
func (c *Client) relogin() error {
	if c.keyPath != "" {
		return c.LoginWithKey(c.keyPath)
	}
	return c.Login()
}

// Call sends one operation and returns the reply. If the session expired it
// logs in again and retries once.
func (c *Client) Call(path, operation string, params Map) (Map, error) {
	req := Map{"operation": operation}
	for k, v := range params {
		req[k] = v
	}
	reply, err := c.post(path, req)
	if err != nil && isAuthError(err) {
		if lerr := c.relogin(); lerr == nil {
			return c.post(path, req)
		}
	}
	return reply, err
}

// ReplyError is a reply whose request_status is not "Ok".
type ReplyError struct {
	Status  string
	Message string
}

func (e *ReplyError) Error() string { return e.Status + ": " + e.Message }

func isAuthError(err error) bool {
	re, ok := err.(*ReplyError)
	return ok && (re.Status == "AuthRequired" || re.Status == "AccessDenied")
}

func (c *Client) post(path string, body Map) (Map, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Post(c.BaseURL+path, "application/json", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var reply Map
	if err := json.Unmarshal(data, &reply); err != nil {
		return nil, fmt.Errorf("HTTP %d, reply is not JSON: %.200s", resp.StatusCode, data)
	}
	if st := Str(reply, "request_status"); st != "" && st != "Ok" {
		return reply, &ReplyError{Status: st, Message: Str(reply, "error_string")}
	}
	return reply, nil
}

// Str reads a value at a dotted path ("a.b.c") as display text.
func Str(m Map, path string) string {
	v := Get(m, path)
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%.1f", t)
	case bool:
		return fmt.Sprintf("%t", t)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// Get reads a value at a dotted path.
func Get(m Map, path string) any {
	var cur any = m
	for _, k := range strings.Split(path, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

// List reads a list of objects from a reply key.
func List(m Map, key string) []Map {
	raw, _ := m[key].([]any)
	out := make([]Map, 0, len(raw))
	for _, r := range raw {
		if mm, ok := r.(map[string]any); ok {
			out = append(out, mm)
		}
	}
	return out
}
