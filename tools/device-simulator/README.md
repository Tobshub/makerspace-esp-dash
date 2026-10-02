# Device simulator

Stand-in for an ESP32. It connects to the local broker with the device key and one-time secret, then publishes greenhouse telemetry until you stop it.

```bash
go run . --device-key dev_0123456789abcdef0123 --secret 'shown-once' --project-id <uuid>
```

The process prints the device key and broker URL. It does not print the secret.

What it publishes:

- telemetry every 5 seconds (`--interval`): `temperature`, `humidity`, `soil_moisture`, `pump_active`
- retained status `{"status":"online"}` on connect, and `{"status":"offline"}` on exit
- state including `firmwareVersion` `sim-1.0.0` (override with `--firmware`)

It subscribes to the device command topic. `set_pump` with `{"enabled": true}` or `{"enabled": false}` is acknowledged on `commands/ack`. A repeated command id is acknowledged again and is not applied twice.

If the broker is down, the process keeps retrying. After a disconnect it subscribes and announces itself again.
