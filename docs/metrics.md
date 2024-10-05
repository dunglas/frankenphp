# Metrics

When [Caddy metrics](https://caddyserver.com/docs/metrics) are enabled, FrankenPHP exposes the following metrics:

* `frankenphp_[worker]_total_workers`: The total number of workers.
* `frankenphp_[worker]_busy_workers`: The number of workers currently processing a request.
* `frankenphp_[worker]_worker_request_time`: The time spent processing requests by all workers.
* `frankenphp_[worker]_worker_request_count`: The number of requests processed by all workers.
* `frankenphp_[worker]_ready_workers`: The number of workers that have called `frankenphp_handle_request` at least once.
* `frankenphp_[worker]_worker_crashes`: The number of times a worker has unexpectedly terminated.
* `frankenphp_[worker]_worker_restarts`: The number of times a worker has been deliberately restarted.
* `frankenphp_total_threads`: The total number of PHP threads.
* `frankenphp_busy_threads`: The number of PHP threads currently processing a request (running workers always consume a thread).

For worker metrics, the `[worker]` placeholder is replaced by the worker script path in the Caddyfile.
