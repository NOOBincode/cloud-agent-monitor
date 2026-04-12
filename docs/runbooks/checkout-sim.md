# Runbook: checkout-sim 告警

## CheckoutHigh5xxRate

1. 在 Grafana 打开 Loki，查询：`{compose_service="checkout-sim"}`（与当前 Promtail 配置一致）。
2. 确认是否为预期演练：若刚运行故障注入，属正常；否则检查应用日志。
3. 恢复：在仓库根目录执行（`fault-injector` 为一次性任务，勿用 `exec`）：
   ```bash
   docker compose -f deploy/compose/docker-compose.yml --profile inject run --rm fault-injector /scripts/inject.sh reset
   ```
   或手动：
   ```bash
   curl -sS -X POST "http://localhost:18080/internal/chaos/reset" -H "X-Chaos-Token: dev-chaos-token"
   ```

## CheckoutLatencyP99High

1. Grafana → Prometheus → 查看 `checkout_request_duration_seconds_bucket`。
2. 若与 `latency_ms` 注入一致，按上节 `reset`。
3. 若无注入，检查节点 CPU/磁盘与是否有其他压测容器。
