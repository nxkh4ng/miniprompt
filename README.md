# miniprompt

A lightweight Go library for building interactive command-line interfaces

## Goals (v0.1.0)

- Bốn prompt: Input, Confirm, Select, MultiSelect.
- Một core chung: đọc phím, vòng lặp và render không bị chép lại ở từng prompt.
- Đúng: hiển thị đúng với nội dung dài và terminal nhỏ.
- An toàn: khôi phục terminal khi submit, huỷ bằng Ctrl+C hoặc gặp lỗi.
- Dễ dự đoán: không âm thầm trả giá trị rỗng khi hết input.
- Dễ đọc: mỗi file có trách nhiệm rõ ràng; chỉ thêm abstraction khi có nhu cầu thật.
- Go 1.26+, ít dependency, chạy trên Linux, macOS và Windows.
