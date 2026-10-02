# ESP32 basic example

Starter firmware for a fresh ESP32. The sketch is a skeleton until Phase 13.

The finished example will:

1. connect to Wi-Fi and reconnect with backoff
2. connect to MQTT as `device_key` / `device_secret`
3. set a Last Will on the status topic, retained, payload `{"status":"offline"}`
4. publish retained `{"status":"online"}` on connect, then state including `firmwareVersion`
5. publish telemetry as `{"metrics":{"temperature":28.4}}` (`timestamp` is optional Unix milliseconds)
6. subscribe to the command topic
7. handle `set_led` and send acknowledgements

Topics:

```text
makerspace/v1/projects/{projectId}/devices/{deviceKey}/telemetry
makerspace/v1/projects/{projectId}/devices/{deviceKey}/state
makerspace/v1/projects/{projectId}/devices/{deviceKey}/status
makerspace/v1/projects/{projectId}/devices/{deviceKey}/commands
makerspace/v1/projects/{projectId}/devices/{deviceKey}/commands/ack
```

Flash with PlatformIO once the sketch is implemented:

```bash
cd examples/esp32-basic
pio run -t upload
```
