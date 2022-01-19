package main

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tls "github.com/libp2p/go-libp2p-tls"
	yamux "github.com/libp2p/go-libp2p-yamux"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type P2P struct {
	// host context layer
	Ctx context.Context

	// libp2p host
	Host host.Host

	// Kademlia DHT routing table
	KadDHT *dht.IpfsDHT

	// peer discovery service
	Discovery discovery.RoutingDiscovery

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

	// create PubSub handler

	return &P2P{
		Ctx: ctx,
	}
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
	connMng, err := connmgr.NewConnManager(100, 400, connmgr.WithSilencePeriod(time.Minute))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatalln("P2P Connection Manager configuration generation failed")
	}
	conn := libp2p.ConnectionManager(connMng)

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
