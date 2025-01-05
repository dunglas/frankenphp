# Метрики

При включении [метрик Caddy](https://caddyserver.com/docs/metrics) FrankenPHP предоставляет следующие метрики:

- `frankenphp_[worker]_total_workers`: Общее количество Worker.
- `frankenphp_[worker]_busy_workers`: Количество Worker, которые в данный момент обрабатывают запрос.
- `frankenphp_[worker]_worker_request_time`: Время, затраченное всеми Worker на обработку запросов.
- `frankenphp_[worker]_worker_request_count`: Количество запросов, обработанных всеми Worker.
- `frankenphp_[worker]_ready_workers`: Количество Worker, которые вызвали `frankenphp_handle_request` хотя бы один раз.
- `frankenphp_[worker]_worker_crashes`: Количество случаев неожиданного завершения Worker.
- `frankenphp_[worker]_worker_restarts`: Количество случаев, когда Worker был перезапущен вручную.
- `frankenphp_total_threads`: Общее количество потоков PHP.
- `frankenphp_busy_threads`: Количество потоков PHP, которые в данный момент обрабатывают запрос (работающие Worker всегда используют поток).

Для метрик Worker плейсхолдер `[worker]` заменяется на путь к Worker-скрипту, указанному в Caddyfile.