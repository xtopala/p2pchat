package main

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tls "github.com/libp2p/go-libp2p-tls"
	yamux "github.com/libp2p/go-libp2p-yamux"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
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

	// setup a P2P host

	// bootstrap the Kad-DHT

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

	var kaddht *dht.IpfsDHT
	// routing configuration with KadDHT
	routing := libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		// TODO: configure KadDHT
		return kaddht, err
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

	return node, kaddht
}
