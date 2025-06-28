## 本地开发命令

```bash
go run .
# 参数都是可选的
go run . -c 100 -n 1000 -f config.json -t 20
```

## 运行命令

```bash
.\go-test.exe -c 100 -n 1000 -f config.json -t 20
```

## 命令行参数说明

```
-c 并发数
-n 总请求数
-f 配置文件
-t 超时时间，单位秒
```

## 配置文件示例

```json
[
  {
    "url": "http://localhost:8080",
    "method": "GET",
    "headers": {
      "Content-Type": "application/json"
    },
    "params": {
      "key": "value",
      "number": 456
    },
    "data": {
      "name": "张三",
      "age": 18
    }
  }
]
```
