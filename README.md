# p2pchat
Making a simple terminal based chat app wiht libp2p.
This chat should allow users to jump between different chat rooms. Also, users are allowed to change their usernames at any point they choose to do so.

## Usage
Application can be invoked without any flags, it then joins the default *loby* room as a *anon* user.
We can modify this by passing ``-user`` and ``-room`` flags.

The method of peer discovery can also be modified by using the ``-discover`` flag. Valid flag values are *announce* and *advertise*. The application default is *advertise*.

Application runtime can be modified to user different loglevels using the ``-log`` flag. Valid values are *trace*, *debug*, *info*, *warn* and *error*. The application default is *info*.

Application can be istalled with
```
go install .
```

and then to run it use
```
p2pchat -username X -room Y
```
Or, we could just run it like
``` 
go run . -username X -room Y
```

## Future

Would love to try out and implement:
- [x] Kademlia DHT for peer discovery and routing
- [x] TLS encryption
- [x] Peer active discovery
- [x] YAMUX stream multiplexing
- [x] NAT traversal
- [x] AutoRelay
- [ ] Support for QUIC transport
- [ ] Use Protocol buffers for message endcoding
- [ ] Chat Room notifications
- [ ] Password protected Chat Rooms  
- [ ] Support other PubSub routers (FloodSub, RandomSub, EpiSub)

