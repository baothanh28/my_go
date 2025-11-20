# Push Notification – Workflow Summary (with Device ID)

## 1. Client collects Expo Push Token & Device ID
- App gọi `Notifications.getExpoPushTokenAsync()` để lấy `expoPushToken`.
- App lấy `deviceId` (ví dụ dùng: `Application.androidId`, `expo-device`, hoặc tự sinh UUID).
- Gửi cả `expoPushToken` + `deviceId` lên Backend.

### Example payload
```json
{
  "userId": 123,
  "deviceId": "device-abc-xyz-123",
  "expoPushToken": "ExponentPushToken[xxxxxx]"
}
2. Backend stores user devices & tokens
Backend lưu mỗi thiết bị của user dưới dạng một bản ghi riêng.

Suggested table design
Field	Description
id	Primary key
user_id	User reference
device_id	Unique device identifier
expo_push_token	Expo push token
platform	ios/android
last_seen_at	Last time the device synced
created_at	Timestamp


5. Client receives notification
Client xử lý 3 trạng thái:
Foreground
Background
App bị kill (tùy OS)

Listener:
Notifications.addNotificationReceivedListener
Notifications.addNotificationResponseReceivedListener
