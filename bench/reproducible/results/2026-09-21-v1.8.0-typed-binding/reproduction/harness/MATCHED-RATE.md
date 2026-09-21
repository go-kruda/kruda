# Matched-rate latency protocol

- Derive one offered rate per profile as 50% of the slowest fresh median capacity, rounded down to 1,000 requests/second.
- Run Kruda, Fiber, and Actix for four rounds per profile: 24 measured samples total. Pair order is balanced 2/2 inside each profile and position bias is reversed between profiles.
- Use the pinned patched wrk2 binary with four threads, 256 connections, a 30-second invocation, corrected HDR latency, and per-thread post-calibration completion windows.
- Accept a sample only when the sum of per-thread post-calibration rates is 99%-101% of the offered rate, all errors are zero, every thread calibrated once, and the histogram/window count difference does not exceed the 256 concurrent connections that can straddle the two snapshots.
- Report p50, p90, and p99. Treat differences near wrk2's approximately 1 ms precision as unresolved.

Full-invocation `Requests/sec` includes calibration and is retained only as a diagnostic; it is not the achieved-rate gate.
