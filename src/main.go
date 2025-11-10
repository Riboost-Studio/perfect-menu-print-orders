// main.go
package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// detectLocalIP tries to find your main IPv4 address (non-loopback)
func detectLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String(), nil
		}
	}
	return "", fmt.Errorf("no local IPv4 address found")
}

// probe returns true if the port is open
func probe(ip string, port int) bool {
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func main() {
	localIP, err := detectLocalIP()
	if err != nil {
		panic(err)
	}
	fmt.Println("Local IP:", localIP)

	// assume /24 subnet
	parts := strings.Split(localIP, ".")
	if len(parts) != 4 {
		panic("unexpected IP format")
	}
	subnet := strings.Join(parts[:3], ".")
	fmt.Println("Scanning subnet:", subnet+".0/24")

	var wg sync.WaitGroup
	ipChan := make(chan string, 256)
	printers := make(chan string, 256)

	// worker goroutines
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range ipChan {
				if probe(ip, 631) || probe(ip, 9100) || probe(ip, 515) {
					printers <- ip
				}
			}
		}()
	}

	// enqueue IPs
	for i := 1; i <= 254; i++ {
		ip := fmt.Sprintf("%s.%d", subnet, i)
		if ip != localIP {
			ipChan <- ip
		}
	}
	close(ipChan)

	go func() {
		wg.Wait()
		close(printers)
	}()

	fmt.Println("Scanning... please wait (a few seconds)")
	found := false
	for ip := range printers {
		fmt.Printf("Possible printer found at %s\n", ip)
		found = true
	}

	if !found {
		fmt.Println("No printers detected.")
	}
	fmt.Println("Done.")
}
