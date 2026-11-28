// Package fake is an in-memory ebusd TCP server for tests and offline E2E runs.
package fake

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
)

type Server struct {
	mu     sync.Mutex
	Values map[string]string // "circuit.name" -> raw value
	Writes []string          // "circuit.name=value"
	// Writable limits find -w and write to these keys; nil allows every key in Values.
	Writable map[string]bool
	// OnWrite runs under the server lock after a write stored the value.
	OnWrite func(values map[string]string, key, old, new string)
	ln      net.Listener
}

func (s *Server) Get(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Values[key]
}

func (s *Server) WriteLog() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.Writes...)
}

func New(values map[string]string) *Server { return &Server{Values: values} }

func (s *Server) Start(addr string) (string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	s.ln = ln
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return ln.Addr().String(), nil
}

func (s *Server) Close() { s.ln.Close() }

func tokenize(line string) []string {
	var out []string
	var cur strings.Builder
	inQ, has := false, false
	for _, r := range line {
		switch {
		case r == '"':
			inQ, has = !inQ, true
		case (r == ' ' || r == '\t') && !inQ:
			if has {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if has {
		out = append(out, cur.String())
	}
	return out
}

func (s *Server) serve(c net.Conn) {
	defer c.Close()
	rd := bufio.NewReader(c)
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			return
		}
		fmt.Fprintf(c, "%s\n\n", s.handle(tokenize(strings.TrimSpace(line))))
	}
}

func (s *Server) handle(args []string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(args) == 0 {
		return "ERR: command not found"
	}
	var circuit, input string
	var rest []string
	readOnly := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-c":
			i++
			circuit = args[i]
		case "-i":
			i++
			input = args[i]
		case "-m":
			i++
		case "-r":
			readOnly = true
		case "-f", "-w", "-a":
		default:
			rest = append(rest, args[i])
		}
	}
	switch args[0] {
	case "info":
		return "version: ebusd 26.1.26.1\nsignal: acquired\n" +
			`address 08: slave #11, scanned "MF=Vaillant;ID=BAI00;SW=0405;HW=9602", loaded "vaillant/bai.0010015600.inc" ([HW=9602]), "vaillant/08.bai.csv"` + "\n" +
			`address 15: slave #2, scanned "MF=Vaillant;ID=70000;SW=0614;HW=6903", loaded "vaillant/15.700.csv"`
	case "find":
		var lines []string
		for k := range s.Values {
			c, n, _ := strings.Cut(k, ".")
			if readOnly {
				if !strings.Contains(k, "#") {
					lines = append(lines, fmt.Sprintf("r,%s,%s,,,,,,,", c, n))
				}
				continue
			}
			if strings.Contains(k, "#") || s.Writable != nil && !s.Writable[k] {
				continue
			}
			lines = append(lines, fmt.Sprintf("w,%s,%s,,,,,,,", c, n))
		}
		return strings.Join(lines, "\n")
	case "read":
		if len(rest) != 1 {
			return "ERR: invalid arguments"
		}
		key := circuit + "." + rest[0]
		if input != "" {
			key += "#" + input
		}
		v, ok := s.Values[key]
		if !ok {
			return "ERR: element not found"
		}
		return v
	case "write":
		if len(rest) != 2 {
			return "ERR: invalid arguments"
		}
		key := circuit + "." + rest[0]
		if _, ok := s.Values[key]; !ok || s.Writable != nil && !s.Writable[key] {
			return "ERR: element not found"
		}
		old := s.Values[key]
		s.Values[key] = rest[1]
		s.Writes = append(s.Writes, key+"="+rest[1])
		if s.OnWrite != nil {
			s.OnWrite(s.Values, key, old, rest[1])
		}
		if strings.Contains(rest[0], "Timer.") {
			return "empty"
		}
		return "done"
	}
	return "ERR: command not found"
}
