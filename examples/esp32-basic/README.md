# ESP32 basic example

Flashable starter for a fresh ESP32. It connects to Wi-Fi, authenticates to MQTT as the device, publishes sample telemetry, and answers `set_led`.

## What it does

1. Connects to Wi-Fi and retries with exponential backoff, up to 30 seconds.
2. Connects to MQTT as `DEVICE_KEY` / `DEVICE_SECRET`. That secret is the device password from the setup wizard, not a broker admin password.
3. Sets a retained Last Will on the status topic: `{"status":"offline"}` at QoS 1.
4. On connect, publishes retained `{"status":"online"}`, then state `{"firmwareVersion":"1.0.0","led":false}` at QoS 1, and subscribes to commands at QoS 1.
5. Every 5 seconds, publishes telemetry at QoS 0. `timestamp` is Unix milliseconds once NTP has set the clock. Until then the field is omitted and the server uses the arrival time.
6. Handles `set_led` with `{"enabled": true}` or `{"enabled": false}` and acknowledges it at QoS 1.
7. Remembers the last 8 command ids. A repeated id is acknowledged again and does not change the LED twice.

Topics:

```text
makerspace/v1/projects/{projectId}/devices/{deviceKey}/telemetry
makerspace/v1/projects/{projectId}/devices/{deviceKey}/state
makerspace/v1/projects/{projectId}/devices/{deviceKey}/status
makerspace/v1/projects/{projectId}/devices/{deviceKey}/commands
makerspace/v1/projects/{projectId}/devices/{deviceKey}/commands/ack
```

The onboard LED follows `set_led` (active high, GPIO `LED_BUILTIN`, usually 2). If the LED looks inverted, the board drives it active low.

## Flash

You need [PlatformIO](https://platformio.org/) and a USB cable. The sketch uses the `256dpi/MQTT` library so acknowledgements and state can be published at QoS 1. Register a device in the dashboard and copy the wizard snippet into `src/main.cpp`. Fill in `WIFI_SSID` and `WIFI_PASSWORD` yourself. Do not commit those values.

```bash
cd examples/esp32-basic
pio run -t upload
pio device monitor
```

The serial monitor is 115200 baud. It prints the device key and broker host. It does not print the secret.

A down Wi-Fi network or broker does not stop the loop. The sketch waits, then tries again with a longer delay. After a reconnect it subscribes and announces itself again.

## Try a command

On the project Controls page, add a toggle:

- command: `set_led`
- on payload: `{"enabled": true}`
- off payload: `{"enabled": false}`

Send it to this device. The dashboard should move from Sending to Sent, then Acknowledged, and the device page should show firmware `1.0.0`. The Events page shows the command and the acknowledgement.

The same command can be published by hand:

```bash
mosquitto_pub -h localhost -t "makerspace/v1/projects/<project-uuid>/devices/dev_.../commands" \
  -q 1 \
  -m '{"id":"cmd_1","command":"set_led","payload":{"enabled":true}}'
```

Use a new `id` for a new action. Sending `cmd_1` again returns the first acknowledgement and leaves the LED where it is.
