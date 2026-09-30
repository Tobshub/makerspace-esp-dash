#include <Arduino.h>

// Phase 13 fills this sketch in. Copy credentials from the device setup
// wizard. Do not commit real values, and do not embed a broker admin password.

#define WIFI_SSID ""
#define WIFI_PASSWORD ""

#define MQTT_HOST ""
#define MQTT_PORT 1883

#define DEVICE_KEY ""
#define DEVICE_SECRET ""
#define PROJECT_ID ""

void setup() {
  Serial.begin(115200);
  Serial.println("esp32-basic skeleton — see docs/phases/13-esp32-starter.md");
}

void loop() {}
