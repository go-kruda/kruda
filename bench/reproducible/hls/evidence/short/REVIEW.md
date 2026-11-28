# Independent review

GPT‑6.1 Sol xhigh independently checked the design and replayed all 30 arms against raw logs and samples. Verified binary selection, schedule, gate outcomes, affinity, attempt/status/byte/origin accounting, delivered rate, and CPU, system CPU and RSS reductions for proxy, load and origin. The independently reproduced report (prior to the labeling-only edit below) had SHA-256 34f2189529d3b1365b3205f38ccd2d1859777cc6ebde3b133d70fd88ea7b86e8.

Verdict: PASS with scope notes. The Wing high04 run completed all scored responses and failed the aggregate origin-hit gate by five. Keep this run as failed and describe the aggregate discrepancy without attributing an exact warm-up cause. High-load Fiber and Actix misses came from the driver in-flight cap. At lower load, all arms delivered the intended work. Wing uses lower CPU and has lower p99 than Fiber in every matched block. Wing and Actix have close p99 and CPU, with Actix lower RSS. Max observed CPU endpoint offset 43.187 ms and max sample interval 203.003 ms.

Review wording changes: made the Wing-minus-Actix p99 direction explicit and labeled the misses as scored-request misses. The harness was untouched during the experiment. Publishable feature copy is limited to eligible fixed-length HLS and byte-range streaming. Run comparisons are experimental and apply to these configured builds.
