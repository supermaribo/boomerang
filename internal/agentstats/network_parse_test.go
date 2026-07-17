package agentstats

import "testing"

func TestParseProcRoute(t *testing.T) {
	data := []byte(`Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT                                                       
eth0	00000000	0101A8C0	0003	0	0	100	00000000	0	0	0                                                                               
eth0	0001A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0                                                                                 
wlan0	00000000	0101A8C0	0003	0	0	600	00000000	0	0	0                                                                             
`)
	if got := parseProcRoute(data); got != "eth0" {
		t.Fatalf("parseProcRoute = %q, want eth0 (lowest metric)", got)
	}
	if got := parseProcRoute([]byte("Iface Destination\nlo 00000000 00000000 0003 0 0 0 00000000 0 0 0\n")); got != "" {
		t.Fatalf("lo default route should be ignored, got %q", got)
	}
	if got := parseProcRoute(nil); got != "" {
		t.Fatalf("empty = %q", got)
	}
}

func TestParseProcRoute6(t *testing.T) {
	zero := "00000000000000000000000000000000"
	data := []byte(zero + " 00 " + zero + " 00 " + zero + " 00000064 00000000 00000000 00000003 eth0\n" +
		zero + " 00 " + zero + " 00 " + zero + " 00000258 00000000 00000000 00000003 wlan0\n" +
		"20010db8000000000000000000000000 40 " + zero + " 00 " + zero + " 00000100 00000000 00000000 00000001 eth0\n")
	if got := parseProcRoute6(data); got != "eth0" {
		t.Fatalf("parseProcRoute6 = %q, want eth0", got)
	}
	if got := parseProcRoute6(nil); got != "" {
		t.Fatalf("empty = %q", got)
	}
}

func TestParseNetDevCounters(t *testing.T) {
	data := []byte(`Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1234 1 0 0 0 0 0 0 5678 1 0 0 0 0 0 0
  eth0: 100000 10 0 0 0 0 0 0 200000 20 0 0 0 0 0 0
  ens18:999 1 0 0 0 0 0 0 888 1 0 0 0 0 0 0
`)
	rx, tx, ok := parseNetDevCounters(data, "eth0")
	if !ok || rx != 100000 || tx != 200000 {
		t.Fatalf("eth0 = %d/%d ok=%v", rx, tx, ok)
	}
	rx, tx, ok = parseNetDevCounters(data, "ens18")
	if !ok || rx != 999 || tx != 888 {
		t.Fatalf("ens18 = %d/%d ok=%v", rx, tx, ok)
	}
	if _, _, ok := parseNetDevCounters(data, "missing"); ok {
		t.Fatal("missing iface should fail")
	}
	if _, _, ok := parseNetDevCounters([]byte("garbage"), "eth0"); ok {
		t.Fatal("malformed should fail")
	}
	if _, _, ok := parseNetDevCounters(nil, "eth0"); ok {
		t.Fatal("nil should fail")
	}
}
