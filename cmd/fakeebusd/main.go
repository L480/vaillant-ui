// Command fakeebusd serves recorded ebusd values for offline E2E tests.
package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/L480/vaillant-ui/internal/ebusd/fake"
)

func main() {
	addr := flag.String("listen", "127.0.0.1:8888", "")
	values := flag.String("values", "e2e/fixtures/values.tsv", "circuit<TAB>name<TAB>raw")
	writable := flag.String("writable", "e2e/fixtures/writable.txt", "circuit,name per line")
	flag.Parse()

	vals := map[string]string{}
	f, err := os.Open(*values)
	if err != nil {
		log.Fatal(err)
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p := strings.SplitN(sc.Text(), "\t", 3)
		if len(p) == 3 {
			vals[p[0]+"."+p[1]] = p[2]
		}
	}
	w := map[string]bool{}
	b, err := os.ReadFile(*writable)
	if err != nil {
		log.Fatal(err)
	}
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		w[strings.Replace(l, ",", ".", 1)] = true
	}
	s := fake.New(vals)
	s.Writable = w
	a, err := s.Start(*addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("fake ebusd on %s: %d values, %d writable", a, len(vals), len(w))
	select {}
}
