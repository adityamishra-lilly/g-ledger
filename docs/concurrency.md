# Concurrency

How ledgerd handles connections, why it is built this way, and what that costs.

> **Status:** the design argument below is settled. The benchmark that quantifies it
> (Phase 0 §6: RSS and p99 against connection count, `pooled` vs `unbounded`) has not
> been run yet. Numbers land in [Measurements](#measurements) when it has.

## The model

One goroutine per connection, reading and writing with the standard library's
`net.Conn`. A bounded admission pool caps how many connections are served at once;
past the cap, new connections are rejected immediately rather than queued.

Both halves of that are selectable at runtime from one binary via `--mode`
(`pooled` | `unbounded`), so the benchmark below is reproducible by a reviewer.

## Why bound concurrency at all

Goroutines are cheap, so the objection is fair: why cap them?

Because cheap is not free, and the cap is not really about the goroutine. Each
connection holds a read buffer, a write buffer, and an 8 KB minimum goroutine stack.
More importantly, unbounded admission converts overload into unbounded *queueing
delay* — work is accepted that cannot be served, latency climbs without limit, and
the server dies slowly and invisibly through GC pressure and tail latency rather than
failing fast.

A bounded pool turns overload into an explicit, immediate rejection. Admission is
acquired **non-blocking**, after `Accept()` returns: a blocking acquire would
reintroduce the exact queueing delay the bound exists to prevent.

### What the bound does *not* buy

`--max-conns` caps *admitted connections*, not goroutines — an admitted connection
still gets its own goroutine. So the bound is a **rejection policy**, not a
memory-per-idle-connection policy. It controls queueing delay and tail latency under
overload. It does not make 100k idle connections cheap.

The only thing that makes 100k idle connections cheap is an event loop. See below.

## Why not an event loop?

Rejected for now; revisited in Phase 9 as a measured alternative rather than an
unmeasured default.

### First, a framing correction

This is not "threads vs. event loop". **Go's `net.Conn` is already an event loop.**
A blocking `conn.Read` does not block an OS thread — the runtime netpoller registers
the file descriptor with `epoll` (Linux), `kqueue` (BSD), or IOCP (Windows) and parks
the goroutine until readiness wakes it. You get epoll's syscall efficiency while
writing sequential code.

The real choice is **who owns the scheduling**: Go's M:N scheduler, or a loop you
write yourself.

### What writing that loop would buy

- **Memory per idle connection.** The dominant cost here is the 8 KB minimum
  goroutine stack — roughly 800 MB of stacks at 100k connections, before buffers. A
  userspace loop holds only fd state, on the order of tens of MB.
- **GC mark cost.** The collector scans goroutine stacks. 100k of them is real mark
  work, and it surfaces in p99.9 specifically.
- **Batching.** Draining many ready descriptors per wakeup enables batched writes and
  fewer syscalls per reply.

### What it would cost

1. **`fsync` poisons the loop.** This is the decisive one. Phase 5's headline is
   group commit — batching concurrent writers into one `fsync`. In a callback-driven
   loop, an `fsync` on the loop thread stalls *every* connection on that loop. Fixing
   that requires a separate WAL writer thread, a completion queue, and thread-safe
   out-of-loop writes — reintroducing precisely the async plumbing the loop was meant
   to remove, and adding a cross-thread buffer handoff that is the hardest thing in
   this codebase to make race-clean. With a goroutine per connection, "block on the
   WAL, then reply" is four lines and `go test -race` can actually check it.
2. **ledgerd is not network-bound.** Group commit targets tens of thousands of
   durable writes per second. At that rate, epoll-vs-netpoller dispatch overhead is
   noise; the bottleneck is `fsync`, and after that, parser allocations. Userspace
   loops start paying off well past that, on workloads doing trivial work per request.
   Optimising transport dispatch first would be optimising the wrong layer.
3. **It would delete this document's own deliverable.** The output of Phase 0 is the
   `pooled`-vs-`unbounded` comparison below. With no per-connection goroutines, that
   comparison does not exist — and Phase 3's `sync.Pool` work loses the unpooled
   baseline that makes it interesting.
4. **Cross-platform development.** Development is on Windows with benchmarking in
   WSL2 and on a Linux host. A hand-rolled `epoll` loop is Linux-only; `io_uring` more
   so. Library event loops fall back to a goroutine-based emulation on Windows, which
   means testing a materially different code path from the one that ships.

Secondary, but not nothing: `SetReadDeadline` gives the three-deadline state machine
(idle / command / write) almost for free. On a raw loop, `epoll` has no per-descriptor
deadline and you own a timer wheel.

### What is done instead

- One goroutine per connection, never two. A separate writer goroutine would double
  stack cost for no gain at this scale.
- Handler call stacks kept shallow. Goroutine stacks start at 8 KB and grow by
  copying; a deep call chain or a large frame on the hot path silently multiplies
  per-connection memory.
- `GOGC` / `GOMEMLIMIT` tuning (Phase 3) as the main lever on stack-scan cost.

### When this gets reopened

Phase 9 revisits it as a second transport behind the same interface, selectable at
runtime the way `--mode` already is, published as a comparison. The triggers:

- RSS per idle connection makes the 100k level infeasible on the benchmark host, or
- profiling shows GC stack-scan among the top contributors to p99.9, or
- Phases 0–8 are complete and the netpoller-vs-`io_uring` comparison is the chosen
  stretch goal.

Whichever fires, the number that fired it gets recorded here.

## Measurements

_Not yet run._ Phase 0 §6 method: for each concurrency level in {1k, 10k, 50k, 100k}
and each mode in {`pooled`, `unbounded`}, hold that many connections idle for 60s,
then drive a fixed 5k req/sec echo workload for 60s.

To be recorded: RSS at idle and under load, goroutine count, accept rate during ramp,
p50/p99/p99.9 echo latency, GC pause distribution, failed connects. Two charts,
regenerated from committed CSV rather than hand-drawn: RSS against connection count,
and p99 against connection count, both modes on each.

Three things this section must state when it is filled in, whichever way the data
falls:

1. Measured cost per idle connection, in KB, for each mode.
2. The crossover point where bounding starts to win, if there is one in range.
3. **An explicit statement if `unbounded` wins at every level tested**, with an
   extrapolation of where it would stop winning and why. Goroutine-per-connection
   holds up well into the tens of thousands; that is the expected result at 1k–10k,
   and it is the honest one, not a failure.

### Environment

To be recorded per host before any numbers are taken — WSL2 and the Linux benchmark
box separately, never mixed:

| Setting | Note |
| --- | --- |
| `ulimit -n` | ≥ 4 × `--max-conns`, on server and load host both |
| `net.core.somaxconn` | Go derives the listen backlog from it |
| `net.ipv4.ip_local_port_range` | widened on the load host |
| `net.ipv4.tcp_tw_reuse` | as configured |
| Kernel / Go version / CPU / RAM | exact values |

One planning note: 100k connections from a single load host to a single
(destination IP, destination port) exceeds the ~64k ephemeral port space. Widening
`ip_local_port_range` reaches roughly 64k and no further. The 100k level needs
multiple destination ports, multiple source IPs, or a second load host.
