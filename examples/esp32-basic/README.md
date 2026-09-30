# ESP32 basic example

Starter firmware for a fresh ESP32. The sketch is a skeleton until Phase 13.

The finished example will:

1. connect to Wi-Fi and reconnect with backoff
2. connect to MQTT as `device_key` / `device_secret`
3. set a Last Will offline message
4. publish online state and sample telemetry
5. subscribe to the command topic
6. handle `set_led` and send acknowledgements

Topics:

```text
makerspace/v1/projects/{projectId}/devices/{deviceKey}/telemetry
makerspace/v1/projects/{projectId}/devices/{deviceKey}/state
makerspace/v1/projects/{projectId}/devices/{deviceKey}/commands
makerspace/v1/projects/{projectId}/devices/{deviceKey}/commands/ack
```

Flash with PlatformIO once the sketch is implemented:

```bash
cd examples/esp32-basic
pio run -t upload
```
