package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	//"sort"

	"github.com/tatsushid/go-fastping"
)

type response struct {
	addr *net.IPAddr
	rtt  time.Duration
}

func main() {
	var useUDP bool
	flag.BoolVar(&useUDP, "udp", false, "use non-privileged datagram-oriented UDP as ICMP endpoints")
	flag.BoolVar(&useUDP, "u", false, "use non-privileged datagram-oriented UDP as ICMP endpoints (shorthand)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  %s [options] hostname(192.168.10.) [source]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	hostname := flag.Arg(0)
	if len(hostname) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	source := ""
	if flag.NArg() > 1 {
		source = flag.Arg(1)
	}

	success := make(chan string)
	for i := 1; i < 255; i++ {
		go sender(hostname+strconv.Itoa(i), source, useUDP, success)
	}

	result := []string{}
	for i := 1; i < 255; i++ {
		r := <-success
		if len(r) > 1 {
			result = append(result, r)
		}
	}

	//sort.Strings(result)

	for i, ip := range result {
		fmt.Println(i+1, " : ", ip)
	}

    time.Sleep(500 * time.Millisecond)
}

func sender(hostname, source string, useUDP bool, success chan string) {

	p := fastping.NewPinger()
	if useUDP {
		p.Network("udp")
	}

	if source != "" {
		p.Source(source)
	}

	netProto := "ip4:icmp"
	if strings.Index(hostname, ":") != -1 {
		netProto = "ip6:ipv6-icmp"
	}
	ra, err := net.ResolveIPAddr(netProto, hostname)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	results := make(map[string]*response)
	results[ra.String()] = nil
	p.AddIPAddr(ra)

	onRecv, onIdle := make(chan *response), make(chan bool)
	p.OnRecv = func(addr *net.IPAddr, t time.Duration) {
		onRecv <- &response{addr: addr, rtt: t}
	}
	p.OnIdle = func() {
		onIdle <- true
	}

	p.MaxRTT = time.Second
	p.RunLoop()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	select {
	case <-c:
		fmt.Println("get interrupted")
	case res := <-onRecv:
		if _, ok := results[res.addr.String()]; ok {
			results[res.addr.String()] = res
			success <- hostname
		}
	case <-onIdle:
		for host, r := range results {
			if r == nil {
				//fmt.Printf("%s : unreachable %v\n", host, time.Now())
				success <- ""
			} else {
				//fmt.Printf("%s : %v %v\n", host, r.rtt, time.Now())
				success <- ""
			}
			results[host] = nil
		}
	case <-p.Done():
		if err = p.Err(); err != nil {
			fmt.Println("Ping failed:", err)
		}
	}
	signal.Stop(c)
	p.Stop()
}
