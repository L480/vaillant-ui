package ebusd

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	Addr    string
	Timeout time.Duration

	mu   sync.Mutex
	conn net.Conn
	rd   *bufio.Reader
}

type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

func New(addr string) *Client { return &Client{Addr: addr, Timeout: 15 * time.Second} }

func quote(arg string) (string, error) {
	if strings.ContainsAny(arg, "\"\r\n\x00") {
		return "", fmt.Errorf("ungültige Zeichen im Argument")
	}
	if arg == "" || strings.ContainsAny(arg, " \t") {
		return `"` + arg + `"`, nil
	}
	return arg, nil
}

func (c *Client) Do(ctx context.Context, args ...string) (string, error) {
	parts := make([]string, len(args))
	for i, a := range args {
		q, err := quote(a)
		if err != nil {
			return "", err
		}
		parts[i] = q
	}
	line := strings.Join(parts, " ")

	c.mu.Lock()
	defer c.mu.Unlock()
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		out, err := c.roundTrip(ctx, line)
		if err == nil {
			return out, nil
		}
		lastErr = err
		c.closeLocked()
		if ctx.Err() != nil {
			break
		}
	}
	return "", fmt.Errorf("ebusd nicht erreichbar: %w", lastErr)
}

func (c *Client) roundTrip(ctx context.Context, line string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if c.conn == nil {
		d := net.Dialer{Timeout: 5 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", c.Addr)
		if err != nil {
			return "", err
		}
		c.conn, c.rd = conn, bufio.NewReader(conn)
	}
	deadline := time.Now().Add(c.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = c.conn.SetDeadline(deadline)
	if _, err := c.conn.Write([]byte(line + "\n")); err != nil {
		return "", err
	}
	// Every response is its result lines plus one empty line; an empty result is "\n\n".
	readLine := func() (string, error) {
		l, err := c.rd.ReadString('\n')
		return strings.TrimRight(l, "\r\n"), err
	}
	l, err := readLine()
	if err != nil {
		return "", err
	}
	if l == "" {
		t, err := readLine()
		if err != nil {
			return "", err
		}
		if t != "" {
			return "", fmt.Errorf("Antwortrahmen ungültig")
		}
		return "", nil
	}
	out := []string{l}
	for {
		l, err := readLine()
		if err != nil {
			return "", err
		}
		if l == "" {
			break
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n"), nil
}

func (c *Client) closeLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn, c.rd = nil, nil
	}
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
}

func check(out string) (string, error) {
	if strings.HasPrefix(out, "ERR:") {
		return "", &Error{Msg: strings.TrimSpace(strings.TrimPrefix(out, "ERR:"))}
	}
	return out, nil
}

// maxAge < 0 forces a bus read.
func (c *Client) Read(ctx context.Context, circuit, name string, maxAge int, input string) (string, error) {
	args := []string{"read"}
	if maxAge < 0 {
		args = append(args, "-f")
	} else {
		args = append(args, "-m", strconv.Itoa(maxAge))
	}
	args = append(args, "-c", circuit)
	if input != "" {
		args = append(args, "-i", input)
	}
	out, err := c.Do(ctx, append(args, name)...)
	if err != nil {
		return "", err
	}
	return check(out)
}

func (c *Client) Write(ctx context.Context, circuit, name, value string) error {
	out, err := c.Do(ctx, "write", "-c", circuit, name, value)
	if err != nil {
		return err
	}
	if _, err := check(out); err != nil {
		return err
	}
	if o := strings.TrimSpace(out); o != "done" && o != "" {
		return &Error{Msg: "unerwartete Antwort: " + o}
	}
	return nil
}

// WriteDefs returns "circuit.name" of every write message available at ebusd's current access level.
func (c *Client) WriteDefs(ctx context.Context) (map[string]bool, error) {
	out, err := c.Do(ctx, "find", "-w", "-f")
	if err != nil {
		return nil, err
	}
	if _, err := check(out); err != nil {
		return nil, err
	}
	defs := map[string]bool{}
	for _, l := range strings.Split(out, "\n") {
		f := strings.SplitN(l, ",", 4)
		if len(f) < 3 || !strings.Contains(f[0], "w") {
			continue
		}
		defs[f[1]+"."+f[2]] = true
	}
	return defs, nil
}

// ReadDefs returns circuit and name of every active read message (CSV order).
func (c *Client) ReadDefs(ctx context.Context) ([][2]string, error) {
	out, err := c.Do(ctx, "find", "-r", "-f")
	if err != nil {
		return nil, err
	}
	if _, err := check(out); err != nil {
		return nil, err
	}
	var defs [][2]string
	seen := map[string]bool{}
	for _, l := range strings.Split(out, "\n") {
		f := strings.SplitN(l, ",", 4)
		if len(f) < 3 || !strings.Contains(f[0], "r") || seen[f[1]+"."+f[2]] {
			continue
		}
		seen[f[1]+"."+f[2]] = true
		defs = append(defs, [2]string{f[1], f[2]})
	}
	return defs, nil
}

type Device struct {
	Address string `json:"address"`
	Circuit string `json:"circuit"`
	ID      string `json:"id"`
	SW      string `json:"sw"`
	HW      string `json:"hw"`
}

type Info struct {
	Version string   `json:"version"`
	Signal  bool     `json:"signal"`
	Devices []Device `json:"-"`
}

func (c *Client) Info(ctx context.Context) (Info, error) {
	out, err := c.Do(ctx, "info")
	if err != nil {
		return Info{}, err
	}
	return ParseInfo(out), nil
}

func ParseInfo(out string) Info {
	var inf Info
	for _, l := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(l, "version: "):
			v := strings.TrimPrefix(l, "version: ")
			v = strings.TrimPrefix(v, "ebusd ")
			if i := strings.Index(v, "."); i >= 0 {
				if j := strings.Index(v[i+1:], "."); j >= 0 {
					v = v[:i+1+j]
				}
			}
			inf.Version = v
		case strings.HasPrefix(l, "signal: "):
			inf.Signal = strings.TrimPrefix(l, "signal: ") == "acquired"
		case strings.HasPrefix(l, "address ") && strings.Contains(l, "scanned \""):
			d := Device{Address: l[8:10]}
			s := l[strings.Index(l, "scanned \"")+9:]
			s = s[:strings.Index(s, "\"")]
			for _, kv := range strings.Split(s, ";") {
				k, v, _ := strings.Cut(kv, "=")
				switch k {
				case "ID":
					d.ID = v
				case "SW":
					d.SW = v
				case "HW":
					d.HW = v
				}
			}
			if i := strings.Index(l, "loaded \"vaillant/"); i >= 0 {
				f := l[i+17:]
				f = f[:strings.Index(f, "\"")]
				switch {
				case strings.HasPrefix(f, "bai"), strings.Contains(f, ".bai."):
					d.Circuit = "bai"
				case strings.Contains(f, "700"):
					d.Circuit = "700"
				default:
					d.Circuit = strings.TrimSuffix(f, ".csv")
				}
			}
			inf.Devices = append(inf.Devices, d)
		}
	}
	return inf
}
