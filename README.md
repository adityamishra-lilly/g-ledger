# g-ledger

`ledgerd` — an in-memory double-entry ledger with a write-ahead log, served over the
Redis RESP protocol.

> Early development. Phase 0 (transport) is in progress; the headline throughput,
> latency, and durability claims go here once they have been measured.

## Design decisions

Each entry states the alternative that was rejected and why. This list grows as the
phases land.

**Goroutine-per-connection, not a userspace event loop.**
Rejected: a hand-rolled `epoll` loop or a library event loop (`gnet`, `evio`),
and `io_uring`. Go's `net.Conn` is already backed by an event loop — the runtime
netpoller parks the goroutine on `epoll`/`kqueue`/IOCP — so the choice is who owns
scheduling, not whether polling happens. Owning it ourselves would buy lower memory
per idle connection and less GC stack-scan work, but it makes Phase 5's `fsync` block
every connection sharing the loop, and ledgerd is bound by `fsync` and parser
allocations long before transport dispatch. Deferred to Phase 9, where it is built as
a second transport and published as a measurement rather than assumed.
Full argument, including the cost this accepts: [`docs/concurrency.md`](docs/concurrency.md).
