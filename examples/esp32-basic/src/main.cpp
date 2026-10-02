#include <Arduino.h>
#include <WiFi.h>
#include <WiFiClient.h>
#include <MQTTClient.h>
#include <ArduinoJson.h>
#include <math.h>
#include <time.h>

// Copy these from the device setup wizard. Wi-Fi values are your network.
// DEVICE_SECRET is this device's MQTT password, never a broker admin password.
// Leave the defaults empty in git.

#define WIFI_SSID ""
#define WIFI_PASSWORD ""

#define MQTT_HOST ""
#define MQTT_PORT 1883

#define DEVICE_KEY ""
#define DEVICE_SECRET ""
#define PROJECT_ID ""

static const char *kFirmware = "1.0.0";
static const uint32_t kTelemetryMs = 5000;
static const uint32_t kWifiAttemptMs = 15000;
static const uint32_t kBackoffStartMs = 1000;
static const uint32_t kBackoffMaxMs = 30000;
static const int kAckCache = 8;
static const int kInbox = 4;
static const int kMaxCommand = 256;

#if defined(LED_BUILTIN)
static const int kLedPin = LED_BUILTIN;
#else
static const int kLedPin = 2;
#endif

WiFiClient net;
MQTTClient mqtt(512);

char statusTopic[128];
char stateTopic[128];
char telemetryTopic[128];
char commandTopic[128];
char ackTopic[128];

struct CachedAck {
  char id[48];
  char body[160];
};

struct Pending {
  char body[kMaxCommand];
  int length;
};

CachedAck acks[kAckCache];
uint8_t ackCount = 0;
uint8_t ackNext = 0;
Pending inbox[kInbox];

bool ledOn = false;
bool wifiPending = false;
bool ntpStarted = false;
uint32_t wifiStartedAt = 0;
uint32_t backoffMs = kBackoffStartMs;
uint32_t nextTryAt = 0;
uint32_t lastTelemetryAt = 0;

bool configured() {
  return WIFI_SSID[0] != '\0' && MQTT_HOST[0] != '\0' && DEVICE_KEY[0] != '\0' && PROJECT_ID[0] != '\0';
}

void topic(char *out, size_t size, const char *leaf) {
  snprintf(out, size, "makerspace/v1/projects/%s/devices/%s/%s", PROJECT_ID, DEVICE_KEY, leaf);
}

void bumpBackoff() {
  nextTryAt = millis() + backoffMs;
  uint32_t next = backoffMs * 2;
  backoffMs = next > kBackoffMaxMs ? kBackoffMaxMs : next;
}

void resetBackoff() {
  backoffMs = kBackoffStartMs;
  nextTryAt = 0;
}

float round1(float value) { return roundf(value * 10.0f) / 10.0f; }

float sampleTemperature() {
  float seconds = millis() / 1000.0f;
  return 24.0f + 4.0f * sinf(seconds / 90.0f);
}

float sampleHumidity(float temperature) {
  float humidity = 68.0f - (temperature - 24.0f) * 1.5f;
  if (humidity < 35.0f) return 35.0f;
  if (humidity > 95.0f) return 95.0f;
  return humidity;
}

bool clockReady() { return time(nullptr) > 1700000000; }

void setLed(bool on) {
  ledOn = on;
  digitalWrite(kLedPin, on ? HIGH : LOW);
}

const char *cachedAck(const char *id) {
  for (uint8_t i = 0; i < ackCount; i++) {
    if (strcmp(acks[i].id, id) == 0) return acks[i].body;
  }
  return nullptr;
}

void rememberAck(const char *id, const char *body) {
  for (uint8_t i = 0; i < ackCount; i++) {
    if (strcmp(acks[i].id, id) == 0) {
      snprintf(acks[i].body, sizeof(acks[i].body), "%s", body);
      return;
    }
  }
  snprintf(acks[ackNext].id, sizeof(acks[ackNext].id), "%s", id);
  snprintf(acks[ackNext].body, sizeof(acks[ackNext].body), "%s", body);
  ackNext = (ackNext + 1) % kAckCache;
  if (ackCount < kAckCache) ackCount++;
}

bool publish(const char *topicName, const char *payload, bool retained, int qos) {
  if (!mqtt.publish(topicName, payload, retained, qos)) {
    Serial.printf("publish failed %s\n", topicName);
    return false;
  }
  return true;
}

void publishState() {
  char body[96];
  snprintf(body, sizeof(body), "{\"firmwareVersion\":\"%s\",\"led\":%s}", kFirmware, ledOn ? "true" : "false");
  publish(stateTopic, body, false, 1);
}

void publishTelemetry() {
  float temperature = sampleTemperature();
  JsonDocument doc;
  if (clockReady()) doc["timestamp"] = static_cast<int64_t>(time(nullptr)) * 1000;
  JsonObject metrics = doc["metrics"].to<JsonObject>();
  metrics["temperature"] = round1(temperature);
  metrics["humidity"] = round1(sampleHumidity(temperature));
  metrics["led"] = ledOn;
  char body[192];
  size_t length = serializeJson(doc, body, sizeof(body));
  if (length == 0 || length >= sizeof(body)) {
    Serial.println("telemetry encode failed");
    return;
  }
  publish(telemetryTopic, body, false, 0);
}

void announce() {
  if (!publish(statusTopic, "{\"status\":\"online\"}", true, 1)) return;
  publishState();
  publishTelemetry();
  lastTelemetryAt = millis();
}

// Called from mqtt.loop. Copy the command and acknowledge it from loop() so a
// QoS 1 publish does not run inside the callback.
void onMessage(MQTTClient *, char *, char bytes[], int length) {
  if (length <= 0 || length >= kMaxCommand) return;
  for (int i = 0; i < kInbox; i++) {
    if (inbox[i].length != 0) continue;
    memcpy(inbox[i].body, bytes, length);
    inbox[i].body[length] = '\0';
    inbox[i].length = length;
    return;
  }
  Serial.println("command dropped");
}

bool validId(const char *id) {
  if (id == nullptr || id[0] == '\0' || strlen(id) >= sizeof(acks[0].id)) return false;
  for (const char *p = id; *p != '\0'; p++) {
    char c = *p;
    bool ok = (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_';
    if (!ok) return false;
  }
  return true;
}

void handleCommand(const char *body) {
  JsonDocument doc;
  if (deserializeJson(doc, body) != DeserializationError::Ok) return;
  const char *id = doc["id"];
  if (!validId(id)) return;

  if (const char *previous = cachedAck(id)) {
    publish(ackTopic, previous, false, 1);
    Serial.printf("command %s duplicate\n", id);
    return;
  }

  const char *name = doc["command"] | "";
  bool success = false;
  const char *error = "unknown command";
  if (strcmp(name, "set_led") == 0) {
    JsonVariant enabled = doc["payload"]["enabled"];
    if (!enabled.is<bool>()) {
      error = "enabled required";
    } else {
      setLed(enabled.as<bool>());
      success = true;
      error = "";
    }
  }

  char ack[160];
  if (success) {
    snprintf(ack, sizeof(ack), "{\"id\":\"%s\",\"success\":true,\"state\":{\"led\":%s}}", id, ledOn ? "true" : "false");
  } else {
    snprintf(ack, sizeof(ack), "{\"id\":\"%s\",\"success\":false,\"error\":\"%s\"}", id, error);
  }
  rememberAck(id, ack);
  if (!publish(ackTopic, ack, false, 1)) return;
  Serial.printf("command %s %s\n", id, success ? "ok" : error);
  if (success) publishState();
}

void drainCommands() {
  for (int i = 0; i < kInbox; i++) {
    if (inbox[i].length == 0) continue;
    handleCommand(inbox[i].body);
    inbox[i].length = 0;
  }
}

bool ensureWifi() {
  bool online = WiFi.status() == WL_CONNECTED;
  if (online && wifiPending) nextTryAt = 0;
  if (online) {
    wifiPending = false;
    if (!ntpStarted) {
      configTime(0, 0, "pool.ntp.org");
      ntpStarted = true;
    }
    return true;
  }
  if (mqtt.connected()) mqtt.disconnect();
  uint32_t now = millis();
  if (!wifiPending) {
    if (now < nextTryAt) return false;
    Serial.printf("wifi connecting %s\n", WIFI_SSID);
    WiFi.mode(WIFI_STA);
    WiFi.setAutoReconnect(true);
    WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
    wifiPending = true;
    wifiStartedAt = now;
    return false;
  }
  if (now - wifiStartedAt > kWifiAttemptMs) {
    Serial.println("wifi timed out");
    WiFi.disconnect(false);
    wifiPending = false;
    bumpBackoff();
  }
  return false;
}

bool ensureMqtt() {
  if (mqtt.connected()) return true;
  uint32_t now = millis();
  if (now < nextTryAt) return false;
  Serial.printf("mqtt connecting %s as %s\n", MQTT_HOST, DEVICE_KEY);
  mqtt.setWill(statusTopic, "{\"status\":\"offline\"}", true, 1);
  if (!mqtt.connect(DEVICE_KEY, DEVICE_KEY, DEVICE_SECRET)) {
    Serial.println("mqtt connect failed");
    bumpBackoff();
    return false;
  }
  if (!mqtt.subscribe(commandTopic, 1)) {
    Serial.println("mqtt subscribe failed");
    mqtt.disconnect();
    bumpBackoff();
    return false;
  }
  resetBackoff();
  Serial.println("mqtt connected");
  announce();
  return true;
}

void setup() {
  pinMode(kLedPin, OUTPUT);
  digitalWrite(kLedPin, LOW);
  Serial.begin(115200);
  delay(200);
  if (!configured()) {
    Serial.println("Paste Wi-Fi and device credentials from the setup wizard into src/main.cpp, then flash again.");
    return;
  }
  topic(statusTopic, sizeof(statusTopic), "status");
  topic(stateTopic, sizeof(stateTopic), "state");
  topic(telemetryTopic, sizeof(telemetryTopic), "telemetry");
  topic(commandTopic, sizeof(commandTopic), "commands");
  topic(ackTopic, sizeof(ackTopic), "commands/ack");
  mqtt.begin(MQTT_HOST, MQTT_PORT, net);
  mqtt.onMessageAdvanced(onMessage);
  mqtt.setTimeout(3000);
  mqtt.setKeepAlive(30);
  mqtt.setCleanSession(true);
  Serial.printf("esp32-basic %s\n", kFirmware);
}

void loop() {
  if (!configured()) {
    delay(1000);
    return;
  }
  if (!ensureWifi()) {
    delay(50);
    return;
  }
  if (!ensureMqtt()) {
    delay(50);
    return;
  }
  mqtt.loop();
  drainCommands();
  uint32_t now = millis();
  if (now - lastTelemetryAt >= kTelemetryMs) {
    publishTelemetry();
    lastTelemetryAt = now;
  }
}
