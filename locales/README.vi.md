# ADB GUI 🚀

Một công cụ quản lý Android mạnh mẽ, hiện đại và độc lập được xây dựng bằng **Wails**, **React** và **Ant Design**.

> ✨ **Lưu ý**: Ứng dụng này là kết quả của quá trình **vibecoding** thuần túy.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey.svg)

## ✨ Tính năng

### 📱 Quản lý thiết bị
- **Danh sách thiết bị hợp nhất**: Quản lý liền mạch các thiết bị vật lý và không dây (hợp nhất USB/Wi-Fi) trong một chế độ xem hợp nhất.
- **Kết nối không dây**: Kết nối dễ dàng thông qua ghép nối IP/Cổng với hỗ trợ mDNS.
- **Lịch sử thiết bị**: Truy cập nhanh vào các thiết bị ngoại tuyến đã kết nối trước đó.
- **Ghim thiết bị**: Ghim thiết bị được sử dụng nhiều nhất để luôn ở đầu danh sách.
- **Làm mới tuần tự**: Cơ chế thăm dò tuần tự thông minh hơn mang lại giao diện ổn định, không bị nhấp nháy.

### 📦 Quản lý ứng dụng
- **Kiểm soát gói đầy đủ**: Cài đặt (Kéo & Thả), Gỡ cài đặt, Bật, Tắt, Buộc dừng, Xóa dữ liệu.
- **Quản lý APK**: Xuất APK đã cài đặt, Cài đặt hàng loạt.
- **Lọc thông minh**: Tìm kiếm và lọc theo ứng dụng hệ thống/người dùng.
- **Hành động nhanh**: Khởi chạy ứng dụng hoặc chuyển trực tiếp đến nhật ký của chúng.

### 🖥️ Phản chiếu màn hình (Scrcpy)
- **Hiệu suất cao**: Phản chiếu độ trễ thấp được hỗ trợ bởi Scrcpy.
- **Ghi màn hình**: Ghi nền độc lập, hỗ trợ ghi đồng thời nhiều thiết bị và truy cập thư mục bằng một cú nhấp chuột.
- **Chuyển tiếp âm thanh**: Truyền phát âm thanh thiết bị đến máy tính của bạn (Android 11+).
- **Tùy chỉnh**: Điều chỉnh Độ phân giải, Tốc độ bit, FPS và Codec (H.264/H.265).
- **Điều khiển**: Hỗ trợ cảm ứng đa điểm, Giữ màn hình bật, Chế độ tắt màn hình.

### 📂 Quản lý tệp
- **Trình khám phá đầy đủ tính năng**: Duyệt, Sao chép, Cắt, Dán, Đổi tên, Xóa và Tạo thư mục.
- **Kéo & Thả**: Tải tệp lên bằng cách kéo chúng vào cửa sổ.
- **Tải xuống**: Dễ dàng chuyển tệp từ thiết bị sang máy tính.
- **Xem trước**: Mở tệp trực tiếp trên máy chủ bằng các ứng dụng mặc định.

### 📜 Logcat nâng cao
- **Phát trực tuyến thời gian thực**: Trình xem nhật ký trực tiếp với điều khiển tự động cuộn.
- **Lọc mạnh mẽ**: Lọc theo Mức nhật ký, Thẻ, PID hoặc Regex tùy chỉnh.
- **Lọc trước (Pre-Filter)**: Khả năng đệm hiệu suất cao chỉ lưu trữ các nhật ký khớp với các quy tắc cụ thể.
- **Tập trung vào ứng dụng**: Tự động lọc nhật ký cho một ứng dụng cụ thể.

### 💻 ADB Shell
- **Bảng điều khiển tích hợp**: Chạy các lệnh ADB thô trực tiếp trong ứng dụng.
- **Lịch sử lệnh**: Truy cập nhanh vào các lệnh đã thực thi trước đó.

### 🔌 Khay hệ thống
- **Truy cập nhanh**: Kiểm soát phản chiếu và xem trạng thái thiết bị từ thanh menu/khay hệ thống.
- **Ghim thiết bị**: Ghim thiết bị chính của bạn để xuất hiện ở đầu danh sách và menu khay.
- **Chức năng khay**: Truy cập trực tiếp vào Logcat, Shell và Trình quản lý tệp cho các thiết bị đã ghim ngay từ khay.
- **Chỉ báo ghi âm**: Chỉ báo trực quan "chấm đỏ" trên khay khi đang ghi âm.
- **Hoạt động nền**: Giữ ứng dụng chạy trong nền để truy cập tức thì.

---

## 🛠️ Binary tích hợp sẵn

Ứng dụng này hoàn toàn độc lập. Nó bao gồm:
- **ADB** (Android Debug Bridge)
- Tệp thực thi **Scrcpy**
- **Scrcpy-server**

Khi khởi động, các tệp này được giải nén vào một thư mục tạm thời và được sử dụng tự động. Bạn không cần cấu hình PATH hệ thống.

---

## ⚠️ Lưu ý quan trọng cho người dùng Xiaomi/Poco/Redmi

Để bật **điều khiển bằng cảm ứng** trong Scrcpy, bạn phải:
1. Vào **Tùy chọn nhà phát triển**.
2. Bật **Gỡ lỗi USB**.
3. Bật **Gỡ lỗi USB (Cài đặt bảo mật)**.
   *(Lưu ý: Điều này yêu cầu thẻ SIM và đăng nhập tài khoản Mi trên hầu hết các thiết bị Xiaomi).*

---

## 🚀 Bắt đầu

### Điều kiện tiên quyết
- **Go** (v1.21)
- **Node.js** (v18 LTS)
- **Wails CLI** (v2.9.2)
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.2
  ```

### Phát triển
```bash
wails dev
```

### Biên dịch (Build)
```bash
wails build
```
Ứng dụng đã biên dịch sẽ có sẵn trong `build/bin`.

### Phát hành (Release)
Dự án này sử dụng GitHub Actions để tự động hóa việc biên dịch đa nền tảng. Để tạo một bản phát hành mới:
1. Gắn thẻ (tag) cho commit của bạn: `git tag v1.0.0`
2. Đẩy thẻ lên: `git push origin v1.0.0`
GitHub Action sẽ tự động biên dịch cho macOS, Windows và Linux, và tải các tệp lên trang Release.

---

## 🔧 Khắc phục sự cố

### macOS: "Ứng dụng bị hỏng và không thể mở được"
Nếu bạn tải xuống ứng dụng từ GitHub và gặp lỗi *"adbGUI.app bị hỏng và không thể mở được"*, điều này là do tính năng cách ly Gatekeeper của macOS.

Để khắc phục điều này, hãy chạy lệnh sau trong terminal của bạn:
```bash
sudo xattr -cr /path/to/adbGUI.app
```
*(Thay thế `/path/to/adbGUI.app` bằng đường dẫn thực tế đến ứng dụng đã tải xuống của bạn)*

> **Hoặc tự build:** Nếu bạn không muốn bỏ qua Gatekeeper, bạn có thể dễ dàng [build ứng dụng từ mã nguồn](#-bắt-đầu) ngay trên máy của mình. Chỉ mất vài phút thôi!

### Windows: "Windows đã bảo vệ PC của bạn"
Nếu bạn thấy cửa sổ SmartScreen màu xanh ngăn ứng dụng khởi động:
1. Nhấp vào **Thông tin thêm (More info)**.
2. Nhấp vào **Vẫn chạy (Run anyway)**.

---

## 📄 Giấy phép
Dự án này được cấp phép theo Giấy phép MIT.
