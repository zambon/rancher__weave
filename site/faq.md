# FAQ

## Troubleshooting

### My conatiner is not running, and when I do a "docker logs my-container" the
last line shows a "unable to find interface ethwe" error.

When a container is started via our Docker API proxy (which is the default way we
recommend to use Weave), we replace the regular entry-point with a small program
that waits until the Weave network interface is ready, then runs the real
entry-point.

If this small program finds the Weave interface is not there at all, then it will
say "Unable to find interface ethwe". This text will be in the logs of your
container, because our little program is running there, ahead of your real program.
And if it doesn't find the interface then it will give up and not run the real
program.


## IP addressing

### Should I give an IP address to all my containers?

Since version 0.11, Weave Net has a mechanism that automatically assigns IP addresses
for you. This feature, called IPAM, is documented [here](http://docs.weave.works/weave/latest_release/ipam.html).

### Can I foce a specific IP address for a container?

If you want to start containers with a mixture of automatically-allocated addresses
and manually-chosen addresses, and have the containers communicate with each other,
you can choose a -iprange that is smaller than -ipsubnet. Refer to the 
[IPAM documentation](http://docs.weave.works/weave/latest_release/ipam.html) for
more details.

### Can I use docker –bridge to allocate Weave IP addresses?

Several users have adopted the technique of telling Docker to use Weave’s network bridge,
since Docker already allocates IP addresses.

Docker (as of this writing) is inherently single-host, so the administrator has to
give each host a different range of addresses to work with, but if you take some
large space like 10.2.0.0/16 and divide it up into 256 ranges of 256 addresses,
that’ll get you quite a long way.

However, insisting on controlling the bridge that Docker uses limits adoption. What if
everybody did that? Some people will have specific needs for that bridge, and we want
those people to have the option to use Weave. Statically carving up the address space
limits the flexibility:

    * How do you add an extra 50 hosts, if you’ve already shared out all the space?
    * What if you want to allocate addresses in several different subnets, spread across
    multiple hosts?
    * What if the numbers of containers running on each host turn out to be highly skewed,
    so you run out of addresses on one host while still having plenty available on another?

Moreover, manually tracking all those subnets limits the scale.  For two or three
hosts it’s no problem, but for two or three hundred it doesn’t sound like fun at all.
You’d want to automate the task of allocating them. Which is where we came in...

## Service discovery

### Can I load balance containers?

When using Weave DNS, it is permissible to register multiple containers with
the same name. Weave DNS picks one address at random on each request. This
provides a basic load balancing capability.

### What happens when a container dies?

Weave DNS removes the addresses of any container that dies. This offers a simple way to
implement redundancy. E.g. if in our example we stop one of the pingme containers and
re-run the ping tests, eventually (within ~30s at most, since that is the weaveDNS cache
expiry time) we will only be hitting the address of the container that is still alive.

## Integration

<!---
### How does Weave integrate with Kubernetes / Mesos / my favorite orchestration framework?
-->

TODO


## Performance

### How fast is Weave?

Weave Net is designed to work everywhere including over the open internet and
through firewalls, without configuration or code changes. To do this, Weave Net
switches from kernel space to use space and back again.  Multiple context switches
of this kind can, for some specific types of applications, have a user-observable
performance impact.  But for most applications there should be no visible
performance impact.

### Is there any kernel space optimization?

Weave Net has a [kernel space data path](http://blog.weave.works/2015/06/12/weave-fast-datapath/)
optimisation due to become part of the product very soon.  This will work at
kernel speeds like other data centre backbones.  But even without this optimisation,
Weave Net is plenty fast enough for most applications.

In benchmarking it, remember that Weave Net is application centric - one
network does not need to share traffic with other networks.  So you cannot
compare Weave Net ‘like for like’ with a ‘big SDN’ backbone.  Also note that
in most user-facing applications, Weave Net will not be the bottleneck.  For
example Weave Net might handle webserver to database traffic. If you need more
load, in many cases you can add more nodes.


## Security

### What do you recommend for confidentiality, integrity and authenticity with Weave Net?

Weave communicates via TCP and UDP on a well known port, so you may use
whatever is appropriate to your requirements – for example an IPsec VPN
for inter-DC traffic, or VPC/private network inside a datacentre. For cases
when this is not convenient, Weave Net includes a secure, performant authenticated
encryption mechanism which you can use in conjunction with or as an
alternative to other security technologies. The rest of the FAQ pertains
to this feature, which is implemented with Daniel J. Bernstein’s NaCl library.

### Why did you choose NaCl?

In keeping with our ease-of-use philosophy, the cryptography in Weave is intended
to satisfy a particular user requirement: strong, out of the box security without
complex set up or the need to wade through configuration of cipher suite negotiation,
certificate generation or any of the other things needed to properly secure an IPsec
or TLS installation.

In reality Weave’s requirements on a cryptographic protocol are comparatively simple,
mainly as a consequence of the fact that we control the software deployed at both ends
of the session. Many features common to IPsec and TLS (such as cipher suite negotiation)
are not required, and since such complex functionality has historically proven to be a
source of protocol design flaws we weren’t keen to include them in Weave.

Finally, there is the matter of robust implementations. Both IPsec and TLS implementations
have suffered from a litany of side channel attacks, protocol downgrade vulnerabilities
and other bugs. NaCl has been designed and implemented by professional cryptographers to
avoid these mistakes, and make state-of-the-art authenticated encryption available to
implementers via an API that is highly resistant to misuse. This is why we chose NaCl.

### I heard that Weave’s handshake is plaintext!

It is, but so too are those of protocols such as (D)TLS. The following information
is transmitted in the clear by each peer when a new connection is established:

* Name
* Nickname
* UID
* Connection ID
* Protocol Version
* Public Key

This minimal set of non-confidential information is required for Weave’s peer discovery
mechanism to work efficiently in the presence of encrypted sessions. By comparison, TLS
with client authentication involves sending both server and client certificates
containing distinguished names, serial numbers and public keys across the network
in plaintext.

### I heard that Weave uses an unhashed passphrase (facepalm)

Salting and hashing should be used whenever a service does not require access to
the plaintext of a credential – for example when storing user passwords in a database.
In other cases, such as the private key of a TLS server, access to the plaintext is
essential and as a matter of standard practice (hardware security modules excepted)
this material is kept in unencrypted form so that services can restart without operator
intervention. You should think of the Weave passphrase in the same way, and protect
and distribute it in a similar fashion. Weave never transmits your passphrase over
the network.

### How is the passphrase used?

Every time a new control plane TCP connection is established between a pair of Weave
peers the following steps occur:

Both peers use NaCl to generate a new Curve25519 keypair, the public half of which
is transmitted to the other peer. Each peer uses NaCl to combine their private key
with the received (unauthenticated) public key of the remote peer to derive a
shared secret which is unique to that particular session. Each peer derives an
ephemeral session key by computing the SHA256 hash of the shared secret
concatenated with the passphrase. If the passphrase is not identical on both
peers, the derived session keys will be different, and the connection will be
terminated; the same is true if an active attacker attempts to substitute public
keys. This scheme has the following properties:

* Offline dictionary attack resistance. The passphrase is never transmitted across
  the network, so cannot be sniffed and subjected to a brute force attack
* Forward secrecy. Every session between peers uses a pair of freshly generated
  Curve25519 keypairs, which are then used to derive a shared secret; the
  passphrase is then mixed into this secret with a 256-bit SHA hash. If the
  passphrase is disclosed, it is not possible to compromise historic traffic
  as the ephemeral private halves of the keypairs are not known
* Known-key security. If a session key between peers is compromised, it is not
  possible to obtain the passphrase nor read traffic between other peers
* Online dictionary attack resistance. An active attacker can make only one
  guess at the passphrase per protocol interaction; connection acceptance is
  rate limited to thwart brute force attacks (nevertheless, as with any service
  it is good practice to monitor your logs for probing behaviour)

### How do I choose a passphrase?

You are strongly encouraged to generate a high entropy passphrase from a random
source to protect against probing attacks; see here for details.

### Is Weave vulnerable to stripping/protocol downgrade attacks?

If a Weave peer is configured to use encryption, it won’t accept or make
connections to peers that aren’t. Consequently if you MITM a pair of Weave
peers and strip out either of the public keys from the handshake (as you
might similarly filter STARTTLS capability from an SMTP connection) the
connection will not establish and you will get an error in the logs.

### Replay attacks?

When crypto is enabled Weave automatically uses a sliding receive window of
the same type as IPsec and DTLS, implemented efficiently using a bitset. It
is sized to permit reordering of recent UDP traffic whilst maintaining
perfectly accurate duplicate rejection.

Incidentally, replay protection in DTLS and IPsec is optional.

### Side channel attacks?

Weave’s crypto is a very thin wrapper around the Go NaCl library contributed
by Adam Langley and others, which is in turn a direct port of Bernstein et
al’s original C/assembler library of the same name. This code has been
systematically constructed to eliminate timing side channels and padding
oracles of the kind that have plagued TLS and IPsec for years.

### Why does your UDP encryption utilise only eight of the twenty-four NaCl nonce bytes?

NaCl supports 192-bit nonces so that randomly generated nonces have
negligible risk of collision under the birthday paradox. Weave Net’s
strategy for guaranteeing that a given key/nonce pair is never reused
does not rely on chance:

* We use a monotonically increasing message sequence number between
  each pair of peers (NaCl’s nonces don’t need to be unpredictable,
  merely distinct) which is reset at the start of each session
* Every session between peers uses a new key

This means we guarantee never to reuse a key/nonce pair until our
message sequence number wraps. We chose 64 bits as a reasonable compromise
between per packet overhead and wrap protection – gigabit ethernet can
transmit approximately 1.5 million minimum sized UDP packets per second,
at which rate it would take a 64-bit counter just shy of 390 thousand
years to wrap.

### I think I've found a security problem in Weave. What should I do?

If you think you've found a security problem with Weave, please send a message
to help@weave.works. Your message will be forwarded to the Weave team
and it will not be publicly readable.

