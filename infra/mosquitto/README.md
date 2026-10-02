# Mosquitto

Local development uses `mosquitto.conf` with anonymous MQTT on port 1883.

Do not commit `passwd`. Generate a local password file only when you turn authentication on:

```bash
mosquitto_passwd -c infra/mosquitto/passwd <username>
```

Production requirements:

- anonymous access disabled
- TLS on port 8883
- per-device topic authorization
- device passwords are the one-time device secrets, never a shared admin password baked into firmware

Ingestion identifies a device from the topic (`projectId` + `deviceKey`) and drops unknown, mismatched, and disabled devices. The API subscriber cannot see the publisher password. When broker authentication is on, allow only username `device_key` and password `device_secret` to publish under that device's prefix. Set `MQTT_USERNAME` and `MQTT_PASSWORD` for the API client itself. Do not put those operator credentials in firmware.
