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
