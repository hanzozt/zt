// Package listener keeps the controller's configuration serving the client API
// alone on a second port.
//
// A client that signs in with a certificate cannot cross a TLS-terminating
// proxy, so it reaches the controller by a name of its own, passed through
// untouched. That name must not carry the management API: it would put the
// whole of the controller on an address with no proxy in front of it. The
// controller's config is written once, at first boot, with one listener that
// binds every API; Ensure adds the second, narrower one, and every boot
// converges it.
package listener

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Name is the listener Ensure keeps.
const Name = "client"

// binding is the one API the listener serves: sign-in, enrollment, services and
// sessions for an identity. The management and fabric APIs stay on the first
// listener.
const binding = "edge-client"

// Ensure makes the web section of the config at path hold a listener named Name
// that binds port on the same interface as the first listener, is advertised at
// host on the same port as the first listener, serves only the client API, and
// takes its identity and options from the first listener. The listener is
// replaced when it differs and left alone when it does not. The first rewrite
// keeps the original as path+".orig".
//
// It reports whether the file was written.
func Ensure(path, port, host string) (bool, error) {
	if port == "" || host == "" {
		return false, fmt.Errorf("a listener needs a port and a host, got %q and %q", port, host)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return false, fmt.Errorf("%s: not a mapping", path)
	}
	web := value(doc.Content[0], "web")
	if web == nil || web.Kind != yaml.SequenceNode {
		return false, fmt.Errorf("%s: no web listeners", path)
	}

	var first *yaml.Node
	at := -1
	for i, l := range web.Content {
		if text(value(l, "name")) == Name {
			at = i
		} else if first == nil {
			first = l
		}
	}
	if first == nil {
		return false, fmt.Errorf("%s: no listener to take the identity from", path)
	}

	want, err := narrow(first, port, host)
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if at >= 0 {
		same, err := equal(web.Content[at], want)
		if err != nil {
			return false, err
		}
		if same {
			return false, nil
		}
		web.Content[at] = want
	} else {
		web.Content = append(web.Content, want)
	}

	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return false, err
	}
	if err := enc.Close(); err != nil {
		return false, err
	}
	if _, err := os.Stat(path + ".orig"); os.IsNotExist(err) {
		if err := os.WriteFile(path+".orig", raw, 0o600); err != nil {
			return false, err
		}
	}
	return true, replace(path, out.Bytes())
}

// narrow is a copy of the all-APIs listener that binds and advertises the
// second address and serves the client API alone.
func narrow(first *yaml.Node, port, host string) (*yaml.Node, error) {
	l := clone(first)
	set(l, "name", scalar(Name))

	points := value(first, "bindPoints")
	if points == nil || len(points.Content) == 0 {
		return nil, fmt.Errorf("the first listener has no bindPoints")
	}
	bindHost, _, err := net.SplitHostPort(text(value(points.Content[0], "interface")))
	if err != nil {
		return nil, fmt.Errorf("the first listener's interface: %w", err)
	}
	_, advertised, err := net.SplitHostPort(text(value(points.Content[0], "address")))
	if err != nil {
		return nil, fmt.Errorf("the first listener's address: %w", err)
	}
	set(l, "bindPoints", sequence(mapping(
		"interface", scalar(net.JoinHostPort(bindHost, port)),
		"address", scalar(net.JoinHostPort(host, advertised)),
	)))
	set(l, "apis", sequence(mapping("binding", scalar(binding), "options", &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle})))
	return l, nil
}

func equal(a, b *yaml.Node) (bool, error) {
	x, err := yaml.Marshal(a)
	if err != nil {
		return false, err
	}
	y, err := yaml.Marshal(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(x, y), nil
}

// replace writes data to path through a temporary file, so a reader sees the
// old config or the new one and never half of either.
func replace(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func value(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func set(m *yaml.Node, key string, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = v
			return
		}
	}
	m.Content = append(m.Content, scalar(key), v)
}

func text(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}

func scalar(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }

func sequence(items ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Content: items}
}

func mapping(kv ...any) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i+1 < len(kv); i += 2 {
		m.Content = append(m.Content, scalar(kv[i].(string)), kv[i+1].(*yaml.Node))
	}
	return m
}

// clone copies a node without its comments: they describe the first listener.
func clone(n *yaml.Node) *yaml.Node {
	c := &yaml.Node{Kind: n.Kind, Style: n.Style, Tag: n.Tag, Value: n.Value}
	for _, k := range n.Content {
		c.Content = append(c.Content, clone(k))
	}
	return c
}
