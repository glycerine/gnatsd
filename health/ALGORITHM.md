# The ALLCALL leased-leader election algorithm.

Jason E. Aten

February 2017

definitions, with example values.
-----------

Let heartBeat = 1 sec. This is how frequently
we will assess cluster health by
sending out an allcall ping.

Let leaseTime = 10 sec. This is how long a leader's lease
lasts for. A failed leader won't be replaced until its
lease (+maxClockSkew) has expired. The lease
also lets leaders efficiently serve reads
without doing quorum checks.

Let maxClockSkew = 1 sec. The maxClockSkew
is a bound on how far out of sync our local
clocks may drift.

givens
--------

* Given: Let each server have a numeric integer rank, that is distinct
and unique to that server. If necessary an extremely long
true random number is used to break ties between server ranks, so
that we may assert, with probability 1, that all ranks are distinct integers,
and that each server, absent an in-force lease, can be put into a strict total order.

* Rule: The lower the rank is preferred for being the leader.

* Ordering by lease time then rank: we order the pair (leaseExpires, rank) first
by largest leaseExpires time, then by lower rank. If
both leases have expired, then lease time is not considered as
a part of the ordering, and rank alone determines the
new leader.

ALLCALL Algorithm phases
===========================

### I. Init phase

When a server joins the cluster,
it does not issue allcalls (a ping of all members)
until after leaseTime + maxClockSkew time has elapsed.

During init, the server does, however,
accept and respond to allcalls() from
other cluster members. The allcall() ping will contain
the current (lease, leader-rank) and leader-id
according to the issuer of the allcall(). Every
recipient of the allcall updates her local information
of who she thinks the leader is, so long as the
received information is monotone in the (lease, leader-rank)
ordering; so a later unexpired lease will replace an
earlier unexpired lease, and if both are expired then
the lower rank will replace the larger rank as winner
of the current leader role.

### II. regular ping phase

After a server has finished its Init phase, it
enters its ping phase, where is stays until it
is shut down.

During ping phase, the server continues to accept and respond
to allcall requests from other servers. Now in addition,
the server also issues its own allcall() pings every
heartBeat seconds.

### III. Election and Lease determination

Election and leasing are computed locally, by each
node, one heartBeat after receiving any replies
from the allcall(). The localnode who issued
the allcall sorts the respondents, including
itself, and if all leases have expired, it
determines who is the new leader and marks
their lease as starting from now. Lease
and leader computation is done locally
and independently on each server.
Once in ping phase, this new determination
is broadcast at the next heartbeat when
the local node issues an allcall().

If a node receives an allcall with a leader
claim, and that leader claim has a shorter
lease expirtaion time than the existing
leader, the new proposed leader is rejected
in favor of the current leader. This
favors continuity of leadership until
the end of the current leaders term, and
is a product of the sort order
described above where we sort 
candidates by lease time then rank.

## Properties of the allcall

The allcalls() are heard by all active cluster members, and
contain the sender's computed result of who the current leader is,
and replies answer back with the recipient's own rank and id. Each
recipient of an allcall() replies to all cluster members.
Both the sending and the replying to the allcall are
broadcasts that are published to a well known topic.

## Safety/Convergence: ALLCALL converges to one leader

Suppose two nodes are partitioned and so both are leaders on
their own side of the network. Then suppose the network
is joined again, so the two leaders are brought together
by a healing of the network, or by adding a new link
between the networks. The two nodes exchange Ids and lease
times, and the node with the shorter valid lease time
adopts the node with the longer lease as leader,
since that is the sort order. The adoption takes
effect as soon as the loser's current lease expires.
Hence the two leader situation persists for at
most one lease term after the network join.

## Liveness: a leader will be chosen

Given the total order among nodes, exactly one
will be lowest rank and thus be the preferred
leader at the end of the any current leader's
lease, even if the current lease holder
has failed. Hence, with at least one live
node, the system can run for at most one
lease term before electing a leader.


## commentary

ALLCALL does not guarantee that there will
never be more than one leader. Availability
in the face of network partition is
desirable in many cases, and ALLCALL is
appropriate for these. This is congruent
with Nats design as an always-on system.
ALLCALL does not guarantee that a
leader will always be present, but
with live nodes it does provide
that the cluster will have a leader
after one lease term + maxClockSkew
has expired.

By design, ALLCALL functions well
in a cluster with any number of nodes.
One and two nodes, or an even number
of nodes, will work just fine.

Compared to quorum based elections
like raft and paxos, where an odd
number of at least three
nodes is required to make progress,
this can be very desirable.

ALLCALL is appropriate for AP,
rather than CP, style systems, where
availability is more important
than having a single writer. When
writes are idempotent or deduplicated
downstream, this is typically preferred.
It is better for availability and
uptime to run an always-on leadership
system.

implementation
------------

ALLCALL is implented on top of
the Nats system, see the health/
subdirectory of

https://github.com/nats-io/gnatsd

