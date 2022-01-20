package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	host "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tls "github.com/libp2p/go-libp2p-tls"
	yamux "github.com/libp2p/go-libp2p-yamux"
	"github.com/libp2p/go-tcp-transport"
	"github.com/mr-tron/base58/base58"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const serviceName = "awesome/p2pchat"

type P2P struct {
	// host context layer
	Ctx context.Context

	// libp2p host
	Host host.Host

	// Kademlia DHT routing table
	KadDHT *dht.IpfsDHT

	// peer discovery service
	Discovery *discovery.RoutingDiscovery

	// PubSub handler
	PubSub *pubsub.PubSub
}

// Constructor for a new P2P object.

// Constructed libp2p host is secured with TLS encrypted transportation
// over a TCP transport connection using a Yamux Stream Multiplexer and
// usese a UPnP for the NAT traversal.

// On this host we bootstrap a Kademlia DHT using default peers offered by libp2p.
// Peer Discovery service is created from such DHT.
// The PubSub handler is created last on the host, using previously created Discover service.
func NewP2P() *P2P {
	ctx := context.Background()

	// setup a P2P node
	node, kadDHT := setupNode(ctx)

	logrus.Debugln("Created the P2P Node and Kademlia DHT")

	// bootstrap the Kad-DHT
	bootstrapDHT(ctx, node, kadDHT)

	logrus.Debugln("Bootstraped the Kademlia DHT and Connected to Bootstrap Peers")

	// create a peer discovery service
	routingDiscovery := discovery.NewRoutingDiscovery(kadDHT)

	logrus.Debugln("Peer Discovery service created")

	// create PubSub handler
	pubsub := setupPubSub(ctx, node, routingDiscovery)

	logrus.Debugln("PubSub handler created")

	return &P2P{
		Ctx:       ctx,
		Host:      node,
		KadDHT:    kadDHT,
		Discovery: routingDiscovery,
		PubSub:    pubsub,
	}
}

// Method of P2P that connects to service peers using
// the Advertise functionality of Peer Discovery Service
// to advertise the service and the discover all peers advertising the same.
// The peer discovery is handled by a go routine that will read peer addresses
// from a channel
func (p2p *P2P) AdvertiseConnect() {
	// advertise the availability of the service on this node
	ttl, err := p2p.Discovery.Advertise(p2p.Ctx, serviceName)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("P2P Discovery Advertise failed")
	}

	logrus.Debugln("PeerChat service advertised")

	// give time to propagate the advertisment
	time.Sleep(time.Second * 5)

	logrus.Debugln("Service Time-to-Live is %s", ttl)

	// find all that advertise the same
	peerchan, err := p2p.Discovery.FindPeers(p2p.Ctx, serviceName)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("P2P Discovery failed")
	}

	// conect peers as they are being discovered
	go handlePeerDiscovery(p2p.Host, peerchan)

	logrus.Traceln("Peer Connection Hander started")
}

// Method of P2P that connects to service peers using
// the Provide functionallity of the Kademlia FHT directly to
// announce the ability to provide the service and then discovers
// all peers that provide the same.
// The peer discovery is handled by a go routine that will read peer
// addresses from a channel
func (p2p *P2P) AnnounceConnect() {
	// generate Service CID
	cid := generateCID(serviceName)

	logrus.Traceln("Service CID generated")

	// announce that this host can provide the service CID
	if err := p2p.KadDHT.Provide(p2p.Ctx, cid, true); err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("Service CID Announce failed")
	}

	logrus.Debugln("PeerChat Service announced")
	// sleep to allow announcment to propagate
	time.Sleep(time.Second * 5)

	// find other providers for the service CID
	peerChan := p2p.KadDHT.FindProvidersAsync(p2p.Ctx, cid, 0)

	logrus.Traceln("PeerChat Service peers discovered")

	go handlePeerDiscovery(p2p.Host, peerChan)

	logrus.Debugln("Peer Connection Handler started")
}

// This one generates a CID object from a given string.
// SHA256 is used to hash the string and generate a Multihash.
// The Multihash is then base58 encoded and used to create the CID
func generateCID(name string) cid.Cid {
	// hash the service content ID
	hash := sha256.Sum256([]byte(name))
	// append the hash with the hashing codec ID for SHA2-256 (0x12),
	// the digest size (0x20) and the hash of the service content ID
	finalHash := append([]byte{0x12, 0x20}, hash[:]...)
	// encode the full hash to Base58
	b58 := base58.Encode(finalHash)

	// generate Multihash from the base58 string
	multiHash, err := multihash.FromB58String(string(b58))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("Service CID generation failed")
	}

	// generate a CID from the Multihash
	cidValue := cid.NewCidV1(12, multiHash)
	return cidValue
}

// This one is used to generate p2p configuration options and
// to create libp2p node object for the given context
func setupNode(ctx context.Context) (host.Host, *dht.IpfsDHT) {
	// host identity options
	pvtkey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	identity := libp2p.Identity(pvtkey)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("P2P Identity configuration generation failed")
	}

	logrus.Traceln("P2P Indentity configuration generated")

	// TLS secured TCP transport
	tlsTransport, err := tls.New(pvtkey)
	security := libp2p.Security(tls.ID, tlsTransport)
	transport := libp2p.Transport(tcp.NewTCPTransport)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("P2P Security and Transport configuration generation failed")
	}

	logrus.Traceln("P2P Security and Transport configuration generated")

	// host listener address
	mulAddr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	listener := libp2p.ListenAddrs(mulAddr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("P2P Address Listener configuration generation failed")
	}

	logrus.Traceln("P2P Address Listener configuration generated")

	// stream multiplexer and connection manager
	muxer := libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport)
	conn := libp2p.ConnectionManager(connmgr.NewConnManager(100, 400, time.Minute))

	// NAT traversal and relay options
	nat := libp2p.NATPortMap()
	relay := libp2p.EnableAutoRelay()

	logrus.Traceln("P2P Stream Multiplexer and Connection Manager configurations generated")

	var kadDHT *dht.IpfsDHT
	// routing configuration with KadDHT
	routing := libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		kadDHT = setupKadDHT(ctx, h)
		return kadDHT, err
	})

	logrus.Traceln("P2P Routing configuration generated")

	opts := libp2p.ChainOptions(identity, listener, security, transport, muxer, conn, nat, routing, relay)

	// create a new libp2p node with created options
	node, err := libp2p.New(ctx, opts)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("P2P Node generation failed")
	}

	return node, kadDHT
}

// This one generates a Kademlia DHT object
func setupKadDHT(ctx context.Context, nodeHost host.Host) *dht.IpfsDHT {
	// DHT server mode option
	dhtMode := dht.Mode(dht.ModeServer)
	// retrive the list of default bootstrap peer addresses form libp2p
	bootstraps := dht.GetDefaultBootstrapPeerAddrInfos()
	// DHT bootstrap peers option
	dhtPeers := dht.BootstrapPeers(bootstraps...)

	logrus.Trace("DHT Configuration generated")

	// start a Kademlia DHT on the node in server mode
	kadDHT, err := dht.New(ctx, nodeHost, dhtMode, dhtPeers)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("Kademlia DHT creation failed")
	}

	return kadDHT
}

// This bootstraps a given Kademlia DHT to satisfy the IPFS router interface
// and connects to all bootstrap peers provided by libp2p
func bootstrapDHT(ctx context.Context, nodeHost host.Host, kadDHT *dht.IpfsDHT) {
	if err := kadDHT.Bootstrap(ctx); err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("Kademlia bootstrap failed")
	}

	logrus.Trace("Kademlia DHT is in Bootstrap Mode")

	g := new(errgroup.Group)
	// counters for the number of bootstrap peers
	var connectedBootPeers int
	var totalBootPeers int

	// iterate over the default bootstrap peers provided by libp2p
	for _, peerAddr := range dht.DefaultBootstrapPeers {
		// peer address information
		peerInfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)

		// connect to each bootstrap peer
		g.Go(func() error {
			err := nodeHost.Connect(ctx, *peerInfo)
			if err != nil {
				// increment the total bootstrap peer count
				totalBootPeers++
			} else {
				// increment the connected and total bootstrap peer count
				connectedBootPeers++
				totalBootPeers++
			}

			return err
		})
	}

	if err := g.Wait(); err == nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("Connecting to Bootstrap node failed")
	}

	logrus.Debugf("Connected to %d out of %d Bootstrap Peers", connectedBootPeers, totalBootPeers)
}

// This one generates a PubSub handler object
func setupPubSub(ctx context.Context, nodeHost host.Host, routingDiscovery *discovery.RoutingDiscovery) *pubsub.PubSub {
	// new PubSub service which uses a GossipSub router
	pubSubHandler, err := pubsub.NewGossipSub(ctx, nodeHost, pubsub.WithDiscovery(routingDiscovery))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
			"type":  "GossipSub",
		}).Fatalln("PubSub Handler creation failed")
	}

	return pubSubHandler
}

// This one connects the given node to all peers received from
// a channel of peer address information
func handlePeerDiscovery(nodeHost host.Host, peerchan <-chan peer.AddrInfo) {
	for peer := range peerchan {
		if peer.ID == nodeHost.ID() {
			continue
		}

		nodeHost.Connect(context.Background(), peer)
	}
}
