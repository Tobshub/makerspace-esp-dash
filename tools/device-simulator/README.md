# Device simulator

Stand-in for an ESP32. Phase 5 connects it to MQTT.

```bash
go run . --device-key dev_example --secret 'shown-once' --project-id <uuid>
```

The process prints the device key and broker URL. It does not print the secret.
