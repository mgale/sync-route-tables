package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DavidGamba/go-getoptions"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"
)

/*
Workflow
-- On Startup
- Get a list of docker bridges and their subnets
- Sync routes for those subnets to desired route table

-- While running
- Monitor main route table for ALL bridge changes and push updates to desired route table
- All route changes on bridge interfaces only are synced over

- Route deletes are handled automatically by the OS across route tables, no work required
- Docker network internal setting does not matter, the routes are still added
and access is controlled by Iptables.
- The kernel notifications for interface changes and route updates happend before the
networks are available via docker network ls. This means we can't query docker for
network information immediately when the routes are being added.
*/

var version string = "0.0.1"
var managedRouteTable int
var allBridges bool

var dockerCli *client.Client

var done chan struct{}
var routeUpdates chan netlink.RouteUpdate

//setupCloseHandler creates a 'listener' on a new goroutine which will notify the
//program if it receives an interrupt from the OS. We then handle this by calling
//our clean up procedure and exiting the program.
func setupCloseHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("\r- Ctrl+C pressed in Terminal")
		close(done)
	}()
}

//addRoute handles adding the new route to the destination route table
func addRoute(route netlink.Route) {

	if route.Dst == nil {
		// Skip trying to add routes that do not contain a destination CIDR
		return
	}

	changeHandle, err := netlink.NewHandle()
	if err != nil {
		log.Printf("error getting change handle: %v\n", err)
		return
	}

	myLink, err := netlink.LinkByIndex(route.LinkIndex)
	if err != nil {
		log.Printf("error getting link handle: %v\n", err)
		return
	}
	interfaceName := myLink.Attrs().Name

	newRoute := netlink.Route{
		Dst:        route.Dst,
		Table:      managedRouteTable,
		LinkIndex:  route.LinkIndex,
		ILinkIndex: route.ILinkIndex,
		Protocol:   route.Protocol,
		Scope:      route.Scope,
		Src:        route.Src,
	}

	log.Printf("Adding new route: Name: %s, DestIP: %v\n",
		interfaceName,
		newRoute.Dst.String(),
	)

	err = changeHandle.RouteAdd(&newRoute)

	if err != nil {
		// Route already exists
		if fmt.Sprint(err) == "file exists" {
			return
		} else {
			// On error we print a message but keep functioning
			log.Println("error changing route:", err)
		}
	}
}

func handleRouteUpdates(routeUpdates chan netlink.RouteUpdate) {

	maxRetry := 10
	retryTime := time.Second * 1

	log.Println("Listening for route updates ...")
	for routeUpdate := range routeUpdates {
		route := routeUpdate.Route
		if syscall.RTM_NEWROUTE != routeUpdate.Type {
			log.Println("Skipping route update, route is not being added:", route.Dst.IP.String())
			continue
		}

		if net.ParseIP(route.Dst.IP.String()).To4() == nil {
			log.Println("Skipping non-IPv4 entry:", route.Dst.IP.String())
			continue
		}

		//Skip changes to non-main route tables
		if route.Table != syscall.RT_TABLE_MAIN {
			continue
		}

		myLink, _ := netlink.LinkByIndex(route.LinkIndex)
		interfaceName := myLink.Attrs().Name

		if myLink.Type() != "bridge" {
			continue
		}

		log.Println("Change on interface:", interfaceName)

		syncRoute := false

		if allBridges {
			log.Println("Syncing route change")
			syncRoute = true
		} else {
			for i := 0; i < maxRetry; i++ {
				dockerBridges := getDockerNetworks()
				if val, ok := dockerBridges[interfaceName]; ok {
					log.Println("Found matching docker network:", val)
					syncRoute = true
					break
				}
				time.Sleep(time.Duration(retryTime))
			}
		}

		if syncRoute {
			addRoute(route)
		}
	}
	log.Println("No longer listening for route updates.")
}

//getDockerNetworks returns a map of all docker networks with their bridge name
//and network name
func getDockerNetworks() map[string]string {
	dockerBridges := make(map[string]string)

	networks, err := dockerCli.NetworkList(context.Background(), types.NetworkListOptions{})
	if err != nil {
		log.Printf("failed to connect to docker instance: %v\n", err)
		return dockerBridges
	}
	log.Println("##########################################")
	log.Println("Getting docker networks ...")
	for _, network := range networks {
		if network.Options["com.docker.network.bridge.default_bridge"] == "true" {
			log.Println("Skipping default docker bridge")
			//continue
		}
		if network.Driver == "bridge" {
			// Create linux bridge name
			log.Println("Adding network info for:", network.Name)
			bridgeName := fmt.Sprintf("br-%s", network.ID[:12])
			dockerBridges[bridgeName] = network.Name
		}
	}
	log.Println("Completed getting docker networks")
	log.Println("##########################################")
	return dockerBridges
}

//addDockerBridgeRoutes gets the routes for an existing
//docker bridge and calls the addRoute function
func addDockerBridgeRoutes(bridgeName string) {
	intLink, _ := netlink.LinkByName(bridgeName)
	netHandle, err := netlink.NewHandle()

	if err != nil {
		log.Printf("error getting handle: %v\n", err)
	}

	routes, err := netHandle.RouteList(intLink, netlink.FAMILY_V4)

	if err != nil {
		log.Printf("error getting routes: %v\n", err)
	}

	log.Println("Routes on:", bridgeName)
	for _, route := range routes {
		addRoute(route)
	}
}

//addAllDockerBridgeRoutes loops over the internal map of
//docker networks and adds initial entries on startup.
func addAllDockerBridgeRoutes(dockerBridges map[string]string) {
	log.Println("Syncing all routes over ...")
	for k := range dockerBridges {
		addDockerBridgeRoutes(k)
	}
}

func main() {

	opt := getoptions.New()
	opt.Bool("help", false, opt.Alias("?"))
	opt.Bool("version", false, opt.Alias("V"))
	opt.IntVar(&managedRouteTable, "managed-rt", 0, opt.Required(), opt.Description("Route table to push changes into"))
	opt.BoolVar(&allBridges, "all-bridges", false, opt.Description("Sync all changes, otherwised only docker networks are synced"))

	_, err := opt.Parse(os.Args[1:])
	if opt.Called("help") {
		fmt.Println(opt.Help())
		os.Exit(1)
	}
	if opt.Called("version") {
		fmt.Println(version)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	log.Println("starting ...")

	dockerCli, err = client.NewClientWithOpts()

	if err != nil {
		log.Printf("failed to connect to docker instance: %v\n", err)
	}

	setupCloseHandler()
	addAllDockerBridgeRoutes(getDockerNetworks())

	done = make(chan struct{})

	// Route monitoring
	routeUpdates = make(chan netlink.RouteUpdate)
	go handleRouteUpdates(routeUpdates)
	netlink.RouteSubscribe(routeUpdates, done)

	<-done

}
