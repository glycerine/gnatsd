# go-sliding-window

[Docs: https://godoc.org/github.com/glycerine/go-sliding-window](https://godoc.org/github.com/glycerine/go-sliding-window)


In a picture:

~~~

    /------------------ swp flow-controls these 2 end points -------------\
    |                                                                     |
    V                                                                     V
publisher ---tcp-->  gnatsd  ---tcp-->  nats go-client lib -in-proc-> subscriber
   (a)                 (b)                      (c)                      (d)


legend:
(a) is your go pub process
(b) is the gnatsd server process [ref 3]
(c) nats-client lib [ref 4]
(d) your go subscriber

The problem solved here:
the (c) internal buffers can overflow if (a) is a fast publisher.

Note: (c) and (d) are compiled together into your go subscriber.
~~~

### executive summary

The problem
-----------
Referring to the figure above, when the publisher (a) is faster than the subscriber (d),
the buffers in (c) can overflow. Parts (c) and (d) are compiled
into your subscriber process.

This overflow can happen *even though* the sub links
from (a)->(b) and (b)->(c) are individually
flow-controlled, because there is no end-to-end
feedback that includes (d).

The solution
------------

swp provides flow-control between (a) and (d), giving your
publisher feedback about how much your subscriber can handle.

Implication: nats can be used for *both* control-plane and data delivery.

Note: swp is *optional*, and is layered on top of nats.
It does not change the nats service at all. It simply provides
additional guarantees between two endpoints connected
over nats. Of course other subscribers listening to the same
publisher will see the same rate of events that (d) does.

### description

Package swp implements the same Sliding Window Protocol that
TCP uses for flow-control and reliable, ordered delivery.

The Nats event bus (https://nats.io/) is a
software model of a hardware multicast
switch. Nats provides multicast, but no guarantees of delivery
and no flow-control. This works fine as long as your
downstream read/subscribe capacity is larger than your
publishing rate. If you don't know nats,
[reference 1](https://www.youtube.com/watch?v=5GcAgMPECxE)
is a great introduction.

If your nats publisher ever produces
faster than your subscriber can keep up, you may overrun
your buffers and drop messages. If your sender is local
and replaying a disk file of traffic over nats, you are
guaranteed to exhaust even the largest of the internal
nats client buffers. In addition you may wish the enforced
order of delivery (even with dropped messages), which
swp provides.


### discussion

swp was built to provide flow-control and reliable, ordered
delivery on top of the nats event bus. It reproduces the
TCP sliding window and flow-control mechanism in a
Session between two nats clients. It provides flow
control between exactly two nats endpoints; in many
cases this is sufficient to allow all subscribers to
keep up. If you have a wide variation in consumer
performance, establish the rate-controlling
swp Session between your producer and your slowest consumer.

There is also a Session.RegisterAsap() API that can be
used to obtain possibly-out-of-order and possibly-duplicated
but as-soon-as-possible delivery (similar to that which
nats give you natively), while retaining the
flow-control required to avoid client-buffer overrun.
This can be used in tandem with the main always-ordered-and-lossless
API if so desired.

### API documentation

[Docs: https://godoc.org/github.com/glycerine/go-sliding-window](https://godoc.org/github.com/glycerine/go-sliding-window)

## notes

An implementation of the sliding window protocol (SWP) in Go.

This algorithm is the same one that TCP uses for reliability,
ordering, and flow-control.

Reference: pp118-120, Computer Networks: A Systems Approach
  by Peterson and Davie, Morgan Kaufmann Publishers, 1996.
  For flow control implementation details see section 6.2.4
  "Sliding Window Revisited", pp296-299.

Per Peterson and Davie, the SWP has three benefits:

 * SWP reliably delivers messages across an unreliable link. SWP accomplishes this by acknowledging messages, and automatically resending messages that do not get acknowledged within a timeout.

 * SWP preserves the order in which messages are transmitted and received, by attaching sequence numbers and holding off on delivery until ordered delivery is obtained.

 * SWP can provide flow control. Overly fast senders can be throttled by slower receivers. We implement this here; it was the main motivation for `swp` development.

### status

Working and useful. The library was test-driven and features a network simulator that simulates packet reordering, duplication, and loss. We pass tests with 20% packet loss easily, and we pass tests for lock-step flow control down to one message only in-flight at a time (just as a verification of flow control; do not actually run this way in production if you want performance and/or packet-reordering support!). Round-trip time estimation and smoothing from the observed data-to-ack round trips has been implemented to set retry-deadlines, similar to how TCP operates.

In short, the library is ready, working, and quite useful.

### next steps

What could be improved: the network simulator could be improved by adding a chaos monkey mode that is even more aggressive about re-ordering and duplicating packets.

### credits

Author: Jason E. Aten, Ph.D.

License: MIT

### references

[1] https://www.youtube.com/watch?v=5GcAgMPECxE

[2] https://github.com/nats-io/gnatsd/issues/217

[3] https://github.com/nats-io/gnatsd

[4] https://github.com/nats-io/nats
