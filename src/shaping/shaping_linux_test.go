package shaping

import (
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/facebook/augmented-traffic-control/src/atc_thrift"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func TestCreatesRootQdisc(t *testing.T) {
	tearDown, link := setUpNetlinkTest(t)
	defer tearDown()

	// Setup root qdisc.
	check(t, setupRootQdisc(link), "couldn't create root qdisc")
	// Check that the root qdisc exists.
	qdiscs, err := netlink.QdiscList(link)
	check(t, err, "couldn't list qdiscs")

	if testing.Verbose() {
		test_cmd(t, "tc", "qdisc", "show", "dev", link.Attrs().Name)
		test_cmd(t, "tc", "class", "show", "dev", link.Attrs().Name)
		test_cmd(t, "tc", "filter", "show", "dev", link.Attrs().Name)
	}

	// FIXME Better asserts

	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc")
	}
	_, ok := qdiscs[0].(*netlink.Htb)
	if !ok {
		t.Fatal("Qdisc is the wrong type")
	}
}

func TestShapeOn(t *testing.T) {
	tearDown, link := setUpNetlinkTest(t)
	defer tearDown()

	check(t, setupRootQdisc(link), "couldn't create root qdisc")

	// Set up class + filters (ipv4/ipv6) using shape_on
	mark := int64(5)
	shaping := &atc_thrift.LinkShaping{
		Rate:       10,
		Delay:      &atc_thrift.Delay{Delay: 10, Jitter: 15, Correlation: 1},
		Reorder:    &atc_thrift.Reorder{Percentage: 6, Gap: 1, Correlation: 0.6},
		Loss:       &atc_thrift.Loss{Percentage: 10, Correlation: 6.3},
		Corruption: &atc_thrift.Corruption{Percentage: 3.4},
	}
	check(t, shape_on(mark, shaping, link), "could not enable shaping")

	classes, err := netlink.ClassList(
		link, netlink.MakeHandle(0x1, uint16(mark)),
	)
	check(t, err, "couldn't list classes")
	if len(classes) != 1 {
		t.Fatal("Failed to add class")
	}

	filters, err := netlink.FilterList(link, netlink.MakeHandle(0x1, 0))
	check(t, err, "couldn't list filters")

	if testing.Verbose() {
		test_cmd(t, "tc", "qdisc", "show", "dev", link.Attrs().Name)
		test_cmd(t, "tc", "class", "show", "dev", link.Attrs().Name)
		test_cmd(t, "tc", "filter", "show", "dev", link.Attrs().Name)
	}

	// FIXME Better asserts

	// We expect 2 filters to be setup: 1 for ipv4, 1 for ipv6.
	if len(filters) != 2 {
		t.Fatal("Failed to add filter")
	}
}

func TestShapeRate0(t *testing.T) {
	tearDown, link := setUpNetlinkTest(t)
	defer tearDown()

	check(t, setupRootQdisc(link), "couldn't create root qdisc")

	// Set up class + filters (ipv4/ipv6) using shape_on
	mark := int64(5)
	shaping := &atc_thrift.LinkShaping{}
	check(t, shape_on(mark, shaping, link), "could not enable shaping")

	classes, err := netlink.ClassList(
		link, netlink.MakeHandle(0x1, uint16(mark)),
	)
	check(t, err, "couldn't list classes")
	if len(classes) != 1 {
		t.Fatal("Failed to add class")
	}
	class := classes[0].(*netlink.HtbClass)

	// When a rate of 0 is given, we default to not limiting the traffic by
	// allocating the biggest rate we can. (Currently this is u32 only until
	// the netlink implementation is supporting 64 bits).

	if testing.Verbose() {
		test_cmd(t, "tc", "qdisc", "show", "dev", link.Attrs().Name)
		test_cmd(t, "tc", "class", "show", "dev", link.Attrs().Name)
		test_cmd(t, "tc", "filter", "show", "dev", link.Attrs().Name)
	}
	if class.Rate != math.MaxUint32 {
		t.Fatal("Failed to set unlimited rate.")
	}
}

func TestShapeOff(t *testing.T) {
	tearDown, link := setUpNetlinkTest(t)
	defer tearDown()

	check(t, setupRootQdisc(link), "couldn't create root qdisc")

	// Set up class + filters (ipv4/ipv6) using shape_on
	mark := int64(5)
	shaping := &atc_thrift.LinkShaping{
		Rate:  10,
		Delay: &atc_thrift.Delay{Delay: 10},
		Loss:  &atc_thrift.Loss{Percentage: 10},
	}
	check(t, shape_on(mark, shaping, link), "could not enable shaping")
	check(t, shape_off(mark, link), "could not disable shaping")

	filters, err := netlink.FilterList(link, netlink.MakeHandle(0x1, 0))
	check(t, err, "could not list filters")

	if testing.Verbose() {
		test_cmd(t, "tc", "qdisc", "show", "dev", link.Attrs().Name)
		test_cmd(t, "tc", "class", "show", "dev", link.Attrs().Name)
		test_cmd(t, "tc", "filter", "show", "dev", link.Attrs().Name)
	}

	// FIXME Better asserts

	// We expect 0 filters to be setup: 1 for ipv4, 1 for ipv6.
	if len(filters) != 0 {
		t.Fatal("Failed to delete filter")
	}
}

/**
*** Testing Utilities
**/

func setUpNetlinkTest(t *testing.T) (func(), netlink.Link) {
	if os.Getuid() != 0 {
		t.Skip("Skipped test because it requires root privileges")
	}

	// new temporary namespace so we don't pollute the host
	// lock thread since the namespace is thread local
	runtime.LockOSThread()
	ns, err := netns.New()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("Failed to create new network namespace: %v", err)
	}

	link := setUpDummyInterface(t)

	return func() {
		ns.Close()
		runtime.UnlockOSThread()
	}, link
}

func setUpDummyInterface(t *testing.T) netlink.Link {
	// Use a Dummy interface for testing as boot2docker 1.8.1 does not suport
	// Ifb interfaces
	if err := netlink.LinkAdd(
		&netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{Name: "foo"},
		}); err != nil {
		t.Fatal(err)
	}

	link, err := netlink.LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	return link
}

func check(t *testing.T, err error, s string) {
	if err != nil {
		t.Fatalf(s+": %v", err)
	}
}

func test_cmd(t *testing.T, cmd string, args ...string) {
	if cmdOut, err := exec.Command(cmd, args...).CombinedOutput(); err != nil {
		t.Fatalf("There was an error running cmd %v:\n%v\n", err, string(cmdOut))
	} else {
		t.Logf("run_cmd %q\n%s\n", cmd+" "+strings.Join(args, " "), string(cmdOut))
	}
}
