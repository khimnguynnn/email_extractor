# Email Extractor - Tính năng mới: Crawl từ file URLs

## Tổng quan

Tính năng mới cho phép bạn crawl danh sách URLs từ file thay vì crawl tự động từ một URL duy nhất. Điều này hữu ích khi bạn đã có sẵn danh sách URLs cần crawl.

**⚠️ Quan trọng**: Khi sử dụng flag `-f`, chương trình sẽ **crawl toàn bộ URLs trong file** mà không có giới hạn về số lượng URLs hoặc emails, nhưng **có giới hạn số goroutines** để tránh quá tải hệ thống.

## Cách sử dụng

### 1. Tạo file chứa URLs

Tạo file text với mỗi URL trên một dòng:

```txt
https://example.com
https://httpbin.org
https://jsonplaceholder.typicode.com
https://httpstat.us
```

### 2. Chạy lệnh với flag `-f`

```bash
# Crawl toàn bộ URLs trong file (mặc định 50 workers)
email_extractor -f urls.txt

# Với số workers tùy chỉnh
email_extractor -f urls.txt -max-workers=100

# Với các tùy chọn khác
email_extractor -f urls.txt -out=emails.txt -timeout=30000 -sleep=1000 -max-workers=25

# Crawl tuần tự thay vì song song
email_extractor -f urls.txt -parallel=false

# Với timeout và sleep
email_extractor -f urls.txt -timeout=5000 -sleep=1000 -max-workers=10
```

## Các tùy chọn quan trọng

- `-f`: File chứa danh sách URLs (một URL trên một dòng) - **KHÔNG có giới hạn**
- `-max-workers`: Số lượng goroutines tối đa khi crawl song song (mặc định: 50)
- `-out`: File output cho emails (mặc định: emails.txt)
- `-parallel`: Crawl song song (mặc định: true)
- `-timeout`: Timeout cho mỗi request (mặc định: 10000ms)
- `-sleep`: Thời gian sleep giữa các request (mặc định: 0ms)

**Lưu ý**: 
- Các tùy chọn `-limit-urls` và `-limit-emails` **KHÔNG áp dụng** khi sử dụng `-f`
- `-max-workers` chỉ áp dụng khi `-parallel=true`

## Đặc điểm của tính năng mới

1. **Crawl toàn bộ URLs trong file**: Không có giới hạn về số lượng URLs
2. **Không có giới hạn emails**: Trích xuất tất cả emails tìm được
3. **Worker pool**: Sử dụng worker pool để kiểm soát số goroutines
4. **Lưu emails real-time**: Emails được lưu vào file ngay khi tìm thấy
5. **Kiểm tra trùng lặp**: Tự động bỏ qua URLs trùng lặp
6. **Tương thích ngược**: Vẫn giữ nguyên tính năng crawl từ URL duy nhất với giới hạn

## Real-time Email Saving

### **Tính năng mới**: Emails được lưu real-time
- Emails được lưu vào file **ngay khi tìm thấy**
- Không cần chờ hoàn thành tất cả URLs
- An toàn khi dừng chương trình bằng Ctrl+C
- Không bị mất dữ liệu đã crawl

### **Cách hoạt động**:
1. Khi crawl một URL và tìm thấy emails
2. Emails được lưu ngay lập tức vào file output
3. Tiếp tục crawl URL tiếp theo
4. Nếu dừng chương trình, emails đã lưu vẫn còn trong file

### **Lợi ích**:
- **An toàn**: Không mất dữ liệu khi dừng giữa chừng
- **Theo dõi tiến trình**: Có thể xem file output trong khi crawl
- **Tiết kiệm thời gian**: Không cần chờ hoàn thành mới có kết quả
- **Ổn định**: Tránh mất dữ liệu do lỗi hoặc crash

## Worker Pool System

### **Cách hoạt động**:
- Tạo một số lượng cố định workers (goroutines)
- Mỗi worker lấy URL từ queue và crawl
- Khi hoàn thành, worker lấy URL tiếp theo
- Tất cả URLs được xử lý tuần tự qua worker pool

### **Lợi ích**:
- **Kiểm soát tài nguyên**: Không tạo quá nhiều goroutines
- **Tránh quá tải**: Giới hạn số requests đồng thời
- **Hiệu suất tốt**: Vẫn có lợi ích của concurrency
- **Ổn định**: Không làm crash hệ thống

### **Khuyến nghị số workers**:
- **10-25 workers**: Cho server yếu hoặc cần crawl chậm
- **50 workers**: Mặc định, cân bằng tốt
- **100 workers**: Cho server mạnh và cần crawl nhanh
- **200+ workers**: Chỉ khi thực sự cần thiết

## Ví dụ sử dụng

### File `urls.txt`:
```txt
https://example.com
https://httpbin.org
https://jsonplaceholder.typicode.com
```

### Lệnh chạy:
```bash
email_extractor -f urls.txt -out=found_emails.txt -max-workers=25
```

### Kết quả:
- Crawl tất cả 3 URLs trong file
- Sử dụng tối đa 25 workers
- Không crawl thêm URLs nào khác
- Lưu tất cả emails tìm được vào file `found_emails.txt`
- Không có giới hạn về số lượng

## So sánh với tính năng cũ

| Tính năng | Cũ (-url) | Mới (-f) |
|-----------|-----------|----------|
| Input | 1 URL | File chứa nhiều URLs |
| Crawl depth | Theo độ sâu | Chỉ URLs trong file |
| Tự động tìm links | Có | Không |
| Giới hạn URLs | Có (mặc định: 1000) | **KHÔNG** |
| Giới hạn emails | Có (mặc định: 1000) | **KHÔNG** |
| Số goroutines | Không giới hạn | Giới hạn bởi `-max-workers` |
| Kiểm soát chính xác | Không | Có |
| Phù hợp cho | Khám phá website | Danh sách URLs cụ thể |

## Lưu ý quan trọng

⚠️ **Cảnh báo**: Khi sử dụng `-f` với file có nhiều URLs (ví dụ: hàng triệu URLs), chương trình sẽ:
- Chạy cho đến khi hoàn thành tất cả URLs
- Có thể mất rất nhiều thời gian
- Sử dụng worker pool để kiểm soát tài nguyên
- Có thể bị rate limit từ server

**Khuyến nghị**: 
- Sử dụng `-max-workers=25-50` cho file lớn
- Sử dụng `-sleep=1000` hoặc cao hơn để tránh bị block
- Sử dụng `-timeout=30000` hoặc cao hơn cho các trang web chậm
- Theo dõi tiến trình và có thể dừng bằng Ctrl+C nếu cần
